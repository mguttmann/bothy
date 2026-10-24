// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// measured is one metric of stats_minutes summed over its rows.
type measured struct{ count, sum, good int64 }

// measuredRows sums stats_minutes per metric, dimension and channel (text, "" for NULL).
func measuredRows(t *testing.T, w *dlWorld) map[[3]string]measured {
	t.Helper()
	r, err := w.gwWorld.owner.Query(context.Background(), `SELECT metric, dimension, coalesce(channel_id::text, ''), sum(count), sum(sum), sum(good)
		FROM stats_minutes GROUP BY 1, 2, 3`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out := map[[3]string]measured{}
	for r.Next() {
		var k [3]string
		var m measured
		if err := r.Scan(&k[0], &k[1], &k[2], &m.count, &m.sum, &m.good); err != nil {
			t.Fatal(err)
		}
		out[k] = m
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestMeasure_ServeEndToEnd drives the own measurement (M6-13, VZ-T01, VZ-T04, VZ-T07) through a
// started api process on PostgreSQL 18: REST answers, a gateway connection that receives its own
// message live, a reconnect with catch-up and one with a cursor ahead of the head, a forum thread,
// and a message created while another session holds stats_minutes in ACCESS EXCLUSIVE mode. The
// domain request returns while the table is locked (no measuring point writes to the database);
// after the lock is gone one flush of the process writes the rows: messages_created per channel
// and origin, threads_created, sli.api_availability without a 5xx, sli.reconnect, catchups,
// resyncs with cursor_ahead and sli.delivery_latency, good never above count. No row holds the
// person's id or the canary text of the message.
func TestMeasure_ServeEndToEnd(t *testing.T) {
	t.Parallel()
	got := make(chan *wire.Measure, 1)
	gw := newGatewayWorldRole(t, wire.RoleAPI, wire.Deps{StatsMeasure: func(m *wire.Measure) { got <- m }})
	gw.partitions(t)
	owner := gw.newPerson("alex", identity.DeviceStateActive)
	gw.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'test')`, ids.New().String(), owner.user.String())
	w := &dlWorld{gwWorld: gw, owner: owner}
	me := testkit.Receive(t, "the own measurement of the process", got)
	ctx := context.Background()

	space := w.newSpace("Engineering")
	support := w.newChannel(space, "support")
	if code, b := w.rest(w.base, owner, http.MethodGet, "/api/v1/threads/"+ids.New().String(), nil); code != http.StatusNotFound {
		t.Fatalf("unknown thread = %d %s", code, b)
	}
	const canary = "canary-measure-7f3a"
	c, r := w.ready(t, w.params(owner))
	readUntil(t, c, message(w.post(support.thread, canary)))
	c.Close()

	second := w.addSession(owner)
	c2 := w.dial(t)
	p := w.params(second)
	cur := r.HeadSeq
	p.Cursor, p.SyncEpoch = &cur, r.SyncEpoch.String()
	idCtx, cancel := context.WithTimeout(ctx, testkit.Patience)
	defer cancel()
	if _, err := c2.Identify(idCtx, p); err != nil {
		t.Fatalf("identify with cursor: %v", err)
	}
	readUntil(t, c2, func(f frame) bool { return f.op == "catchup.complete" })
	c2.Close()

	third := w.addSession(owner)
	c3 := w.dial(t)
	p = w.params(third)
	ahead := r.HeadSeq + 1000000
	p.Cursor, p.SyncEpoch = &ahead, r.SyncEpoch.String()
	if _, err := c3.Identify(idCtx, p); err != nil {
		t.Fatalf("identify with a cursor ahead: %v", err)
	}
	readUntil(t, c3, func(f frame) bool { return f.op == "resync_required" })
	c3.Close()

	forum := w.must(owner, http.MethodPost, "/api/v1/spaces/"+space+"/channels", map[string]any{"name": "bugs", "kind": "forum"})
	if len(forum) > 36 {
		forum = forum[:36]
	}
	w.must(owner, http.MethodPost, "/api/v1/channels/"+forum+"/threads", map[string]any{"title": "Export",
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": "first"}})

	// While another session holds stats_minutes exclusively, a message is created all the same.
	lock, err := w.gwWorld.owner.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.Exec(ctx, `LOCK TABLE stats_minutes IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	w.post(support.thread, "while locked")
	if err := lock.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := me.Flush.Flush(ctx); err != nil {
		t.Fatalf("flush after the lock: %v", err)
	}

	rows := measuredRows(t, w)
	for k, m := range rows {
		if m.good > m.count {
			t.Errorf("%v: good %d above count %d", k, m.good, m.count)
		}
		if k[2] != "" && k[2] != support.id && k[2] != forum {
			t.Errorf("%v: a channel outside the scenario", k)
		}
	}
	want := map[[3]string]int64{
		{"messages_created", "human", support.id}: 2,
		{"messages_created", "human", forum}:      1,
		{"threads_created", "", forum}:            1,
		{"catchups", "", ""}:                      1,
		{"resyncs", "cursor_ahead", ""}:           1,
	}
	for k, n := range want {
		if rows[k].count != n {
			t.Errorf("%v: count %d, want %d (rows %v)", k, rows[k].count, n, rows)
		}
	}
	if a := rows[[3]string{"sli.api_availability", "", ""}]; a.count < 6 || a.good != a.count {
		t.Errorf("sli.api_availability %+v, want at least 6 events, all good", a)
	}
	if a := rows[[3]string{"sli.reconnect", "", ""}]; a.count < 3 {
		t.Errorf("sli.reconnect %+v, want the three connections", a)
	}
	if a := rows[[3]string{"sli.delivery_latency", "", ""}]; a.count < 1 {
		t.Errorf("sli.delivery_latency %+v, want the live message", a)
	}
	for _, needle := range []string{owner.user.String(), second.device.String(), canary} {
		var n int
		if err := w.gwWorld.owner.QueryRow(ctx, `SELECT count(*) FROM stats_minutes s WHERE s::text LIKE '%' || $1 || '%'`, needle).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%d rows of stats_minutes hold a person, device or content value", n)
		}
	}
}
