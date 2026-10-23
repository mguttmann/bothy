// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// hintFrame is the part of a notification.created dispatch the test reads.
type hintFrame struct {
	Seq               *int64 `json:"seq"`
	PermissionVersion *int64 `json:"permission_version"`
	Type              string `json:"type"`
	Audience          string `json:"audience"`
	Actor             any    `json:"actor"`
	Resource          struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"resource"`
	Payload struct {
		NID      string `json:"nid"`
		Kind     string `json:"kind"`
		ThreadID string `json:"thread_id"`
		Quiet    bool   `json:"quiet"`
		Count    int    `json:"count"`
	} `json:"payload"`
}

// nextHint reads the frames of c until a notification.created arrives or the deadline passes.
func nextHint(t *testing.T, c *wstest.Client, within time.Duration) (hintFrame, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	for {
		op, d, err := c.Next(ctx)
		if err != nil {
			return hintFrame{}, false
		}
		if op != "dispatch" {
			continue
		}
		var f hintFrame
		if err := json.Unmarshal(d, &f); err != nil {
			t.Fatal(err)
		}
		if f.Type == "notification.created" {
			return f, true
		}
	}
}

// TestServeAll_DesktopHintOverGateway (M3-23, end to end in role all): a started process in role
// all with the real gateway, the consumer push and River. Sam follows the main thread of Support
// with level all and has a live desktop: a message of the Owner reaches Sam's desktop as one
// notification.created (no seq, no permission_version, no actor, audience self, resource the
// thread, kind message); after Sam focuses the thread (not idle) the next message gives none,
// and no notify.gateway_send is inserted for it.
func TestServeAll_DesktopHintOverGateway(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorldAs(t, wire.RoleAll, "BOTHY_CONSUMER__POLL_INTERVAL=50ms")
	space := w.newSpace("Support")
	support := w.newChannel(space, "support")
	sam := w.newPerson("sam", identity.DeviceStateActive)
	w.grant(sam.user, "user", "channel", support.id)
	if code, b := w.rest(w.base, sam, http.MethodPut, "/api/v1/me/notification-preferences/channel/"+support.id, map[string]any{"level": "all"}); code != http.StatusOK {
		t.Fatalf("putNotificationPreference = %d %s", code, b)
	}
	samC, _ := w.ready(t, w.params(sam))
	marker(t, samC)

	w.post(support.thread, "The export is back")
	f, ok := nextHint(t, samC, testkit.Patience)
	if !ok {
		t.Fatal("no notification.created reached Sam's live desktop")
	}
	if f.Seq != nil || f.PermissionVersion != nil || f.Actor != nil || f.Audience != "self" || f.Resource.Kind != "thread" ||
		f.Resource.ID != support.thread || f.Payload.Kind != "message" || f.Payload.ThreadID != support.thread || f.Payload.Count != 1 ||
		f.Payload.NID == "" {
		t.Fatalf("notification.created %+v", f)
	}

	if err := samC.Send("focus", map[string]any{"thread_ids": []string{support.thread}, "idle": false}); err != nil {
		t.Fatal(err)
	}
	marker(t, samC)
	var before int64
	w.scan(`SELECT count(*) FROM jobs.river_job WHERE kind = 'notify.gateway_send'`, &before)
	w.post(support.thread, "And the import too")
	var head int64
	w.scan(`SELECT max(seq) FROM event_log`, &head)
	testkit.Until(t, "the consumer push reaches the head of the log", func() bool {
		var cursor int64
		w.scan(`SELECT last_seq FROM event_consumer_cursors WHERE name = 'push'`, &cursor)
		return cursor >= head
	})
	var after int64
	w.scan(`SELECT count(*) FROM jobs.river_job WHERE kind = 'notify.gateway_send'`, &after)
	if after != before {
		t.Fatalf("%d gateway jobs for a thread in focus", after-before)
	}
	if f, ok := nextHint(t, samC, 2*time.Second); ok {
		t.Fatalf("a hint for a thread in focus: %+v", f)
	}
}

// scan reads the single integer of query as the schema owner into dst (0 for NULL).
func (w *gwWorld) scan(query string, dst *int64) {
	w.t.Helper()
	w.mu.Lock()
	if w.owner == nil {
		w.owner = w.db.Pool(w.t, pg.RoleOwner)
	}
	pool := w.owner
	w.mu.Unlock()
	var v *int64
	if err := pool.QueryRow(context.Background(), query).Scan(&v); err != nil {
		w.t.Fatal(err)
	}
	*dst = 0
	if v != nil {
		*dst = *v
	}
}
