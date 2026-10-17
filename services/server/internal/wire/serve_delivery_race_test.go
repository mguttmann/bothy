// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// TestGateway_TokenRevokeRacingAMessageCommit: a message transaction reads permission_version in the
// sync phase hook (after the event_seq lock), then a token.revoke (which writes no event) commits,
// then the message transaction commits. The message event carries the old rights version, so the
// delivery snapshot of the revoked token must still not serve it: the message committed after the
// revocation never reaches the revoked credential. A trigger that waits on an advisory lock
// models the commit latency of the message transaction after its pv read.
func TestGateway_TokenRevokeRacingAMessageCommit(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	space := w.newSpace("Engineering")
	support := w.newChannel(space, "support")
	agent, tok, params := w.agentWithToken()
	w.grant(agent, "agent", "channel", support.id)
	c, _ := w.ready(t, params, wstest.WithoutHeartbeat())
	readUntil(t, c, message(w.post(support.thread, "before")))

	marker := ids.New().String()
	w.sql(`CREATE FUNCTION m165_hold() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.client_msg_id::text = '` + marker + `' THEN
				PERFORM pg_advisory_lock_shared(4242);
				PERFORM pg_advisory_unlock_shared(4242);
			END IF;
			RETURN NEW;
		END $$`)
	w.sql(`CREATE TRIGGER m165_hold BEFORE INSERT ON event_log FOR EACH ROW EXECUTE FUNCTION m165_hold()`)

	ctx := context.Background()
	pool := w.db.Pool(t, pg.RoleOwner)
	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_lock(4242)`); err != nil {
		t.Fatal(err)
	}
	posted := make(chan int, 1)
	go func() {
		code, _ := w.rest(w.base, w.owner, http.MethodPost, "/api/v1/threads/"+support.thread+"/messages",
			map[string]any{"client_msg_id": marker, "content_md": "racing the revocation"})
		posted <- code
	}()
	// Wait until the message transaction waits in the trigger, after its pv read.
	deadline := time.Now().Add(20 * time.Second)
	for {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the message transaction never reached the trigger")
		}
		time.Sleep(20 * time.Millisecond)
	}
	var pvBefore int64
	_ = pool.QueryRow(ctx, `SELECT value FROM sequence_counters WHERE name = 'permission_version'`).Scan(&pvBefore)
	w.must(w.owner, http.MethodDelete, "/api/v1/tokens/"+tok, nil)
	revokedAt := time.Now()
	var pvAfter int64
	_ = pool.QueryRow(ctx, `SELECT value FROM sequence_counters WHERE name = 'permission_version'`).Scan(&pvAfter)
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_unlock(4242)`); err != nil {
		t.Fatal(err)
	}
	if code := <-posted; code < 200 || code > 299 {
		t.Fatalf("post = %d", code)
	}
	var evPV int64
	if err := pool.QueryRow(ctx, `SELECT permission_version FROM event_log WHERE client_msg_id = $1`, marker).Scan(&evPV); err != nil {
		t.Fatal(err)
	}
	t.Logf("permission_version before DELETE %d, after DELETE %d, carried by the racing message %d", pvBefore, pvAfter, evPV)

	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for {
		op, d, err := c.Next(rctx)
		var ce *wstest.CloseError
		if errors.As(err, &ce) {
			t.Logf("closed with %d %q, %s after the DELETE returned, without the racing message", ce.Code, ce.Reason, time.Since(revokedAt))
			return
		}
		if err != nil {
			t.Logf("no close and no racing message within the window: %v", err)
			return
		}
		if op == "dispatch" && bytes.Contains(d, []byte(marker)) {
			t.Fatalf("the connection of token %s received message %s (pv %d) %s after DELETE /tokens returned (pv %d)",
				tok, marker, evPV, time.Since(revokedAt), pvAfter)
		}
	}
}
