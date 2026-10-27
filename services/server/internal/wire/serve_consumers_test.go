// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"net/http"
	"syscall"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// wakeBackends is the number of LISTEN connections of the worker consumers on db.
const wakeBackends = `SELECT count(*) FROM pg_stat_activity
	WHERE application_name = 'bothy-consumer-wake' AND datname = current_database()`

// seenHandler records the seqs a worker consumer handled.
type seenHandler struct{ seen chan int64 }

func (h seenHandler) HandleEvent(_ context.Context, ev sync.Event) error {
	select {
	case h.seen <- ev.Seq:
	default:
	}
	return nil
}

// insertEvent appends one event as the superuser at the next seq of event_seq, in the counter
// transaction as the server's sync hook does (a running server appends events of its own), with
// partitions, notifies and returns its seq.
func insertEvent(t *testing.T, db *pgtest.DB) int64 {
	t.Helper()
	super := db.SuperConn(t)
	ctx := context.Background()
	tx, err := super.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var seq int64
	if err := tx.QueryRow(ctx, "UPDATE sequence_counters SET value = value + 1 WHERE name = 'event_seq' RETURNING value").Scan(&seq); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT sync_event_log_ensure_partitions(1000, 1, $1::bigint)", seq); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO event_log (seq, id, type, resource_kind, receive_action, payload, permission_version)
		VALUES ($1::bigint, gen_random_uuid(), 'settings.updated', 'user', 'settings.read', '{}'::jsonb, 0)`, seq); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT pg_notify('event_log', $1::bigint::text)", seq); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return seq
}

// TestInsertEventTakesTheNextSeq: a running server appends events of its own (for example the
// status probe of the roles all and worker, status.updated), so insertEvent must take the next
// seq of event_seq as the server does, not a fixed one the server's event may already hold (main
// run 40540821549: duplicate key of event_log seq 1 in TestServeWorkerRolesRunConsumers/all).
func TestInsertEventTakesTheNextSeq(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	super := db.SuperConn(t)
	ctx := context.Background()
	// The server's event first: seq 1 through event_seq, as the sync hook appends it.
	if _, err := super.Exec(ctx, "SELECT sync_event_log_ensure_partitions(1000, 1, 1)"); err != nil {
		t.Fatal(err)
	}
	if _, err := super.Exec(ctx, `WITH c AS (UPDATE sequence_counters SET value = value + 1 WHERE name = 'event_seq' RETURNING value)
		INSERT INTO event_log (seq, id, type, resource_kind, receive_action, payload, permission_version)
		SELECT value, gen_random_uuid(), 'settings.updated', 'user', 'settings.read', '{}'::jsonb, 0 FROM c`); err != nil {
		t.Fatal(err)
	}
	if got := insertEvent(t, db); got != 2 {
		t.Fatalf("insertEvent took seq %d, want 2 (the next of event_seq)", got)
	}
}

// TestServeWorkerRolesRunConsumers: worker and all insert the cursor row of a registered
// consumer at the head, open the wake connection and hand later events to the handler; the stop
// step event_consumers closes the connection. api registers nothing and opens no connection.
func TestServeWorkerRolesRunConsumers(t *testing.T) {
	t.Parallel()
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker, wire.RoleAll} {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()
			db := newServeDB(t)
			h := seenHandler{seen: make(chan int64, 10)}
			p := start(t, role, serveEnv(t, db), wire.Deps{ConsumerHandlers: map[sync.ConsumerName]sync.ConsumerHandler{sync.ConsumerWebhooks: h}})
			waitReadyz(t, p.waitInternal(t), http.StatusOK)
			if role == wire.RoleAPI {
				time.Sleep(300 * time.Millisecond)
				if n := countRows(t, db, wakeBackends); n != 0 {
					t.Errorf("api opened %d wake connections", n)
				}
				if n := countRows(t, db, "SELECT count(*) FROM event_consumer_cursors"); n != 0 {
					t.Errorf("api created %d cursor rows", n)
				}
			} else {
				// The head the cursor row starts at may already hold the server's own events.
				if n := countRows(t, db, `SELECT count(*) FROM event_consumer_cursors
					WHERE name = 'webhooks' AND last_seq <= (SELECT value FROM sequence_counters WHERE name = 'event_seq')`); n != 1 {
					t.Fatalf("cursor rows at the head = %d, want 1", n)
				}
				deadline := testkit.Deadline()
				for countRows(t, db, wakeBackends) != 1 {
					if time.Now().After(deadline) {
						t.Fatalf("%d wake connections, want 1", countRows(t, db, wakeBackends))
					}
					time.Sleep(50 * time.Millisecond)
				}
				want := insertEvent(t, db)
				timeout := time.After(testkit.Patience)
				// The server's own events may come first.
				for handled := false; !handled; {
					select {
					case seq := <-h.seen:
						handled = seq == want
					case <-timeout:
						t.Fatalf("the consumer did not handle the event %d; logs:\n%s", want, p.logs)
					}
				}
			}
			p.sigs <- syscall.SIGTERM
			if code := p.exit(t); code != lifecycle.ExitOK {
				t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
			}
			deadline := testkit.Deadline()
			for countRows(t, db, wakeBackends) != 0 {
				if time.Now().After(deadline) {
					t.Fatal("the wake connection is still open after the shutdown")
				}
				time.Sleep(50 * time.Millisecond)
			}
		})
	}
}

// TestConsumerLagGaugesInApiRole: role api starts no runner, yet a cursor behind the head shows
// in both gauges, read through the gauges of otelmetrics on the meter provider of the test.
func TestConsumerLagGaugesInApiRole(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	insertEvent(t, db)
	insertEvent(t, db)
	if n := countRows(t, db, "WITH i AS (INSERT INTO event_consumer_cursors (name, last_seq) VALUES ('push', 0) RETURNING 1) SELECT count(*) FROM i"); n != 1 {
		t.Fatal("cursor row not inserted")
	}
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	gauges := func(c sync.Consumers) error {
		_, err := otelmetrics.RegisterConsumerGauges(mp, c, nil)
		return err
	}
	p := start(t, wire.RoleAPI, serveEnv(t, db), wire.Deps{ConsumerGauges: gauges})
	waitReadyz(t, p.waitInternal(t), http.StatusOK)
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Gauge[int64]:
				for _, pt := range d.DataPoints {
					if v, _ := pt.Attributes.Value("bothy.consumer.name"); v.AsString() == "push" && pt.Value == 2 {
						found[m.Name] = true
					}
				}
			case metricdata.Gauge[float64]:
				for _, pt := range d.DataPoints {
					if v, _ := pt.Attributes.Value("bothy.consumer.name"); v.AsString() == "push" && pt.Value > 0 {
						found[m.Name] = true
					}
				}
			}
		}
	}
	if !found["bothy.event.consumer.lag"] || !found["bothy.event.consumer.pending_age"] {
		t.Fatalf("gauges in role api = %+v, want push with lag 2 and a pending age", rm.ScopeMetrics)
	}
	p.sigs <- syscall.SIGTERM
	if code := p.exit(t); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
	}
}
