// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestPoolMetrics_States: bothy.db.pool.connections observes the pool statistics in the states
// idle, acquired and max.
func TestPoolMetrics_States(t *testing.T) {
	t.Parallel()
	metricstest.Strict(t)
	db := pgtest.New(t, pgtest.WithoutMigrations())
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 5
	pool := db.OpenPool(t, cfg)
	mp, reader := metricstest.Provider(t)
	reg, err := pool.RegisterMetrics(mp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Unregister() })
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	m := metricstest.Find(t, reader, "bothy.db.pool.connections")
	got := map[string]float64{}
	for _, p := range metricstest.Points(m) {
		if len(p.Attrs) != 1 {
			t.Errorf("attributes %v, want bothy.pool.state only", p.Attrs)
		}
		got[p.Attrs["bothy.pool.state"]] = p.Value
	}
	if got["acquired"] != 1 || got["max"] != 5 || len(got) != 3 {
		t.Errorf("pool connections %v, want acquired 1, max 5 and idle", got)
	}
}

// TestDBClientMetric_ClosedAttributes: db.client.operation.duration of the pool's own query
// tracer carries exactly db.operation.name from its closed list and db.collection.name from the
// tables of the migrations (other for any other table, absent without a table); the metric of
// otelpgx never leaves the process.
func TestDBClientMetric_ClosedAttributes(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	restore := pg.SetMeterProviderForTest(mp)
	defer restore()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	ctx := context.Background()
	for _, q := range []string{
		"SELECT count(*) FROM instance",
		"-- name: Counter :one\nSELECT value FROM public.sequence_counters WHERE name = 'event_seq'",
		"WITH c AS (SELECT 1 AS one) SELECT count(*) FROM \"principals\", c",
		"SELECT count(*) FROM pg_class",
		"SELECT 1",
	} {
		rows, err := pool.Query(ctx, q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		rows.Close()
	}
	m := metricstest.Find(t, reader, "db.client.operation.duration")
	type key struct{ op, table string }
	got := map[key]bool{}
	for _, p := range metricstest.Points(m) {
		for k := range p.Attrs {
			if k != "db.operation.name" && k != "db.collection.name" {
				t.Errorf("unexpected attribute %s=%s", k, p.Attrs[k])
			}
		}
		if op := p.Attrs["db.operation.name"]; op != metrics.Other && !slices.Contains(metrics.DBClientOperationDuration.Attrs[0].Values, op) {
			t.Errorf("db.operation.name %q outside the closed list", p.Attrs["db.operation.name"])
		}
		got[key{p.Attrs["db.operation.name"], p.Attrs["db.collection.name"]}] = true
	}
	for _, want := range []key{{"SELECT", "instance"}, {"SELECT", "sequence_counters"}, {"SELECT", "principals"}, {"SELECT", "other"}, {"SELECT", ""}} {
		if !got[want] {
			t.Errorf("no point %v; got %v", want, got)
		}
	}
	for _, mm := range metricstest.Collect(t, reader) {
		if mm.Name == "db.client.operation.errors" {
			t.Error("the otelpgx error counter left the process")
		}
	}
}
