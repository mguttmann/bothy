// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMigration_StatsMinutes holds stats_minutes against docs/spec/03-data-model.md 4.15 column
// by column, with its checks, the unique key over all dimensions with NULLS NOT DISTINCT, the two
// indexes and the partitioning by range of bucket_start without a partition. As bothy_app it then
// shows the rules: a row needs a partition, which stats_minutes_ensure_partition creates once per
// UTC day; the additive upsert on the unique key adds into one row also with NULL channel, agent
// and currency; the checks refuse a bad minute, metric, currency or good; bothy_app cannot
// delete; stats_minutes_drop_expired drops exactly the partitions that end at the cutoff.
func TestMigration_StatsMinutes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	wantCols := []principalColumn{
		{"bucket_start", tz, false, ""},
		{"metric", "text", false, ""},
		{"channel_id", "uuid", true, ""},
		{"agent_id", "uuid", true, ""},
		{"dimension", "text", false, "''::text"},
		{"currency", "text", true, ""},
		{"count", "bigint", false, "0"},
		{"sum", "bigint", false, "0"},
		{"good", "bigint", false, "0"},
		{"updated_at", tz, false, "now()"},
	}
	if got := principalColumns(t, conn, "stats_minutes"); !reflect.DeepEqual(got, wantCols) {
		t.Errorf("stats_minutes columns:\n got %+v\nwant %+v", got, wantCols)
	}
	cons := principalConstraints(t, conn, "stats_minutes")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK (((count >= 0) AND (sum >= 0) AND (good >= 0)))",
		"CHECK (((currency IS NOT NULL) = (metric = 'cost'::text)))",
		"CHECK ((bucket_start = date_trunc('minute'::text, bucket_start)))",
		"CHECK ((good <= count))",
		"CHECK ((metric = ANY (ARRAY['messages_created'::text, 'threads_created'::text, 'runs_started'::text, 'runs_finished'::text, " +
			"'tool_calls'::text, 'approval_cards'::text, 'cost'::text, 'invocations_denied'::text, 'billing_mode_switches'::text, " +
			"'catchups'::text, 'resyncs'::text, 'slow_consumer_closes'::text, 'sli.api_availability'::text, 'sli.delivery_latency'::text, " +
			"'sli.reconnect'::text, 'sli.push_acceptance'::text, 'sli.run_start'::text, 'sli.turn_duration'::text])))",
		"CHECK (((metric ~~ 'sli.%'::text) OR (good = 0)))",
		"UNIQUE NULLS NOT DISTINCT (bucket_start, metric, channel_id, agent_id, dimension, currency)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("stats_minutes constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX stats_minutes_channel_bucket_idx ON ONLY public.stats_minutes USING btree (channel_id, bucket_start) WHERE (channel_id IS NOT NULL)",
		"CREATE INDEX stats_minutes_metric_bucket_idx ON ONLY public.stats_minutes USING btree (metric, bucket_start)",
		"CREATE UNIQUE INDEX stats_minutes_key ON ONLY public.stats_minutes USING btree (bucket_start, metric, channel_id, agent_id, dimension, currency) NULLS NOT DISTINCT",
	}
	if got := indexes(t, conn, "stats_minutes"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("stats_minutes indexes:\n got %q\nwant %q", got, wantIdx)
	}
	var strategy string
	var parts int
	if err := conn.QueryRow(ctx, `SELECT p.partstrat::text, (SELECT count(*) FROM pg_inherits WHERE inhparent = 'stats_minutes'::regclass)
		FROM pg_partitioned_table p WHERE p.partrelid = 'stats_minutes'::regclass`).Scan(&strategy, &parts); err != nil {
		t.Fatal(err)
	}
	if strategy != "r" || parts != 0 {
		t.Fatalf("partitioning %q with %d partitions, want range and none", strategy, parts)
	}

	app := db.Pool(t, pg.RoleApp)
	const (
		ch  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c01"
		ag  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c02"
		ins = `INSERT INTO stats_minutes (bucket_start, metric, channel_id, agent_id, dimension, currency, count, sum, good)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (bucket_start, metric, channel_id, agent_id, dimension, currency) DO UPDATE
			SET count = stats_minutes.count + excluded.count, sum = stats_minutes.sum + excluded.sum,
			    good = stats_minutes.good + excluded.good, updated_at = now()`
		minute = "2026-01-10T12:34:00Z"
	)
	wantCheckViolation(t, exec(app, ins, minute, "catchups", nil, nil, "", nil, 1, 0, 0), "a row without a partition")
	var created int
	for range 2 {
		if err := app.QueryRow(ctx, `SELECT stats_minutes_ensure_partition('2026-01-10')`).Scan(&created); err != nil {
			t.Fatal(err)
		}
	}
	if created != 0 {
		t.Fatal("the second call created the partition again")
	}
	for _, day := range []string{"2026-01-08", "2026-01-09", "2026-01-11"} {
		mustExec(t, app, `SELECT stats_minutes_ensure_partition($1)`, day)
	}
	// Two processes add into the same row, also with NULL channel, agent and currency.
	mustExec(t, app, ins, minute, "catchups", nil, nil, "", nil, 1, 250, 0)
	mustExec(t, app, ins, minute, "catchups", nil, nil, "", nil, 2, 750, 0)
	mustExec(t, app, ins, minute, "cost", ch, ag, "personal", "EUR", 1, 1500, 0)
	mustExec(t, app, ins, minute, "cost", ch, ag, "personal", "EUR", 1, 500, 0)
	mustExec(t, app, ins, minute, "sli.reconnect", nil, nil, "", nil, 3, 900, 2)
	var rows, count, sum int64
	if err := app.QueryRow(ctx, `SELECT count(*), sum(count), sum(sum) FROM stats_minutes WHERE metric = 'catchups'`).Scan(&rows, &count, &sum); err != nil ||
		rows != 1 || count != 3 || sum != 1000 {
		t.Fatalf("catchups: %d rows, count %d, sum %d (%v), want one row 3 and 1000", rows, count, sum, err)
	}
	if err := app.QueryRow(ctx, `SELECT count(*), sum(sum) FROM stats_minutes WHERE metric = 'cost'`).Scan(&rows, &sum); err != nil ||
		rows != 1 || sum != 2000 {
		t.Fatalf("cost: %d rows, sum %d (%v)", rows, sum, err)
	}
	wantCheckViolation(t, exec(app, ins, "2026-01-10T12:34:05Z", "catchups", nil, nil, "", nil, 1, 0, 0), "a bucket inside a minute")
	wantCheckViolation(t, exec(app, ins, minute, "messages_deleted", nil, nil, "", nil, 1, 0, 0), "an unknown metric")
	wantCheckViolation(t, exec(app, ins, minute, "cost", ch, ag, "personal", nil, 1, 0, 0), "a cost without currency")
	wantCheckViolation(t, exec(app, ins, minute, "catchups", nil, nil, "", "EUR", 1, 0, 0), "a currency outside cost")
	wantCheckViolation(t, exec(app, ins, minute, "catchups", nil, nil, "x", nil, 1, 0, 1), "good outside an sli")
	wantCheckViolation(t, exec(app, ins, minute, "sli.reconnect", nil, nil, "z", nil, 1, 0, 2), "good above count")
	wantCheckViolation(t, exec(app, ins, minute, "catchups", nil, nil, "y", nil, -1, 0, 0), "a negative count")
	wantState(t, exec(app, `DELETE FROM stats_minutes`), "42501", "bothy_app deletes")
	wantState(t, exec(app, `ALTER TABLE stats_minutes DETACH PARTITION stats_minutes_20260110`), "42501", "bothy_app detaches")

	// The cutoff 2026-01-10 00:00 drops the days that end at or before it: the 8th and the 9th.
	var dropped int
	if err := app.QueryRow(ctx, `SELECT stats_minutes_drop_expired('2026-01-10T00:00:00Z', interval '5 seconds')`).Scan(&dropped); err != nil {
		t.Fatal(err)
	}
	var left []string
	r, err := conn.Query(ctx, `SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = 'stats_minutes'::regclass ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	for r.Next() {
		var n string
		if err := r.Scan(&n); err != nil {
			t.Fatal(err)
		}
		left = append(left, n)
	}
	r.Close()
	if dropped != 2 || !slices.Equal(left, []string{"stats_minutes_20260110", "stats_minutes_20260111"}) {
		t.Fatalf("dropped %d, left %q", dropped, left)
	}
	var orphans int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_class WHERE relname IN ('stats_minutes_20260108', 'stats_minutes_20260109')`).Scan(&orphans); err != nil ||
		orphans != 0 {
		t.Fatalf("detached partitions still exist: %d (%v)", orphans, err)
	}
	wantState(t, exec(app, `SELECT stats_minutes_drop_expired(NULL, interval '1 second')`), "22023", "a NULL cutoff")
	wantState(t, exec(app, `SELECT stats_minutes_ensure_partition(NULL)`), "22023", "a NULL day")
}
