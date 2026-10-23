// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// ttl is cluster.presence_ttl of the tests.
const ttl = 10 * time.Second

// env is one database with the store, a view on a fake clock and an owner pool for assertions.
type env struct {
	store *postgres.PresenceStore
	view  *postgres.PresenceView
	clk   *clock.Fake
	owner *pg.Pool
	app   *pg.Pool
}

func newEnv(t *testing.T) env {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	clk := clock.NewFake(time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))
	view, err := postgres.NewPresenceView(pool, ttl, clk)
	if err != nil {
		t.Fatal(err)
	}
	return env{store: postgres.NewPresenceStore(pool), view: view, clk: clk, owner: db.Pool(t, pg.RoleOwner), app: pool}
}

// count returns the rows of gateway_presence, of node when it is not zero.
func (e env) count(t *testing.T, node ids.ID) int {
	t.Helper()
	var n int
	q := `SELECT count(*) FROM gateway_presence WHERE $1::uuid IS NULL OR node_id = $1`
	var arg any
	if !node.IsZero() {
		arg = node.String()
	}
	if err := e.owner.QueryRow(context.Background(), q, arg).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// flush writes rows of node at the fake clock's now.
func (e env) flush(t *testing.T, node ids.ID, rows ...app.PresenceRow) {
	t.Helper()
	now := e.clk.Now()
	if err := e.store.Flush(context.Background(), node, rows, now, now.Add(-ttl)); err != nil {
		t.Fatal(err)
	}
}

// TestPresenceStore_FlushReplacesTheNodesRows: a flush upserts the rows of its node, deletes the
// node's rows whose connection is gone, and leaves the rows of another node alone.
func TestPresenceStore_FlushReplacesTheNodesRows(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	nodeA, nodeB := ids.New(), ids.New()
	alex, sam := ids.New(), ids.New()
	r1 := app.PresenceRow{SessionID: ids.New(), PrincipalID: alex, DeviceID: ids.New(), Focus: []ids.ID{ids.New(), ids.New()}}
	r2 := app.PresenceRow{SessionID: ids.New(), PrincipalID: sam, DeviceID: ids.New(), Focus: []ids.ID{}, Idle: true}
	e.flush(t, nodeA, r1, r2)
	e.flush(t, nodeB, app.PresenceRow{SessionID: ids.New(), PrincipalID: sam, DeviceID: ids.New(), Focus: []ids.ID{}})
	if a, b := e.count(t, nodeA), e.count(t, nodeB); a != 2 || b != 1 {
		t.Fatalf("rows A %d, B %d", a, b)
	}
	r1.Idle, r1.Focus = true, nil
	e.flush(t, nodeA, r1)
	if a, b := e.count(t, nodeA), e.count(t, nodeB); a != 1 || b != 1 {
		t.Fatalf("after the end of Sam's connection: rows A %d, B %d", a, b)
	}
	got, err := e.view.Connections(context.Background(), []ids.ID{alex})
	if err != nil {
		t.Fatal(err)
	}
	if want := []presence.Connection{{DeviceID: r1.DeviceID, Focus: []ids.ID{}, Idle: true}}; !reflect.DeepEqual(got[alex], want) {
		t.Fatalf("got %+v, want %+v", got[alex], want)
	}
	e.flush(t, nodeA)
	if a := e.count(t, nodeA); a != 0 {
		t.Fatalf("an empty flush left %d rows", a)
	}
}

// TestPresenceStore_RowsComeBackAfterALoss: rows lost with the UNLOGGED table (a crash of the
// database) come back at the next flush (Design note 2).
func TestPresenceStore_RowsComeBackAfterALoss(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	node := ids.New()
	rows := []app.PresenceRow{{SessionID: ids.New(), PrincipalID: ids.New(), DeviceID: ids.New(), Focus: []ids.ID{}}}
	e.flush(t, node, rows...)
	if _, err := e.owner.Exec(context.Background(), `DELETE FROM gateway_presence`); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(time.Second)
	e.flush(t, node, rows...)
	if n := e.count(t, node); n != 1 {
		t.Fatalf("rows after the next flush: %d", n)
	}
}

// TestPresenceTTL: the rows of a node that stopped refreshing (a killed node) are ignored by the
// view after cluster.presence_ttl and deleted by the next flush of any node.
func TestPresenceTTL(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	killed, alive := ids.New(), ids.New()
	alex := ids.New()
	e.flush(t, killed, app.PresenceRow{SessionID: ids.New(), PrincipalID: alex, DeviceID: ids.New(), Focus: []ids.ID{}})
	e.clk.Advance(ttl)
	got, err := e.view.Connections(context.Background(), []ids.ID{alex})
	if err != nil || len(got[alex]) != 1 {
		t.Fatalf("at exactly the ttl: %+v %v", got, err)
	}
	e.clk.Advance(time.Millisecond)
	got, err = e.view.Connections(context.Background(), []ids.ID{alex})
	if err != nil || len(got) != 0 {
		t.Fatalf("after the ttl: %+v %v", got, err)
	}
	e.flush(t, alive)
	if n := e.count(t, killed); n != 0 {
		t.Fatalf("stale rows left: %d", n)
	}
}

// TestPresenceView_OneEntryPerDevice: two rows of one device (a replacement in flight across
// nodes) give one entry, the newest; people without rows are absent.
func TestPresenceView_OneEntryPerDevice(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex, robin, device := ids.New(), ids.New(), ids.New()
	focus := []ids.ID{ids.New()}
	e.flush(t, ids.New(), app.PresenceRow{SessionID: ids.New(), PrincipalID: alex, DeviceID: device, Focus: []ids.ID{}, Idle: true})
	e.clk.Advance(time.Second)
	e.flush(t, ids.New(), app.PresenceRow{SessionID: ids.New(), PrincipalID: alex, DeviceID: device, Focus: focus})
	got, err := e.view.Connections(context.Background(), []ids.ID{alex, robin})
	if err != nil {
		t.Fatal(err)
	}
	want := map[ids.ID][]presence.Connection{alex: {{DeviceID: device, Focus: focus}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestPresenceStore_DeleteNode: a clean shutdown deletes every row of its node only.
func TestPresenceStore_DeleteNode(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a, b := ids.New(), ids.New()
	e.flush(t, a, app.PresenceRow{SessionID: ids.New(), PrincipalID: ids.New(), DeviceID: ids.New(), Focus: []ids.ID{}})
	e.flush(t, b, app.PresenceRow{SessionID: ids.New(), PrincipalID: ids.New(), DeviceID: ids.New(), Focus: []ids.ID{}})
	if err := e.store.DeleteNode(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if e.count(t, a) != 0 || e.count(t, b) != 1 {
		t.Fatalf("rows A %d, B %d", e.count(t, a), e.count(t, b))
	}
}

// TestSignalSender_Notifies: a sent payload reaches a LISTEN ephemeral connection unchanged.
func TestSignalSender_Notifies(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	conn, err := e.owner.DialDedicated(ctx, "gateway-sender-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	if _, err := conn.Exec(ctx, "LISTEN ephemeral"); err != nil {
		t.Fatal(err)
	}
	payload := `{"kind":"gateway.consumer"}`
	if err := postgres.NewSignalSender(e.app).Send(ctx, []byte(payload)); err != nil {
		t.Fatal(err)
	}
	n, err := conn.WaitForNotification(ctx)
	if err != nil || n.Channel != "ephemeral" || n.Payload != payload {
		t.Fatalf("%+v %v", n, err)
	}
}
