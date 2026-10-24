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

// TestMigration_PriceTable holds price_table against docs/spec/03-data-model.md section 4.9
// column by column, with the unit check, the key of created_by, the unique key, the exclusion
// constraint and the lookup index (M2-22 design note 2 a), and that btree_gist is installed. As
// bothy_app it then shows the rules: an overlapping validity of the same provider, model and unit
// fails with 23P01 while an adjacent one passes, and only valid_to can change (R-DM-23; ruling
// D-M2-22-1): every other column and the delete are refused.
func TestMigration_PriceTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	wantCols := []principalColumn{
		{"id", "uuid", false, ""},
		{"provider", "text", false, ""},
		{"model", "text", false, ""},
		{"unit", "text", false, ""},
		{"price_micros", "bigint", false, ""},
		{"currency", "text", false, "'USD'::text"},
		{"valid_from", tz, false, ""},
		{"valid_to", tz, true, ""},
		{"source_url", "text", false, ""},
		{"retrieved_at", tz, false, ""},
		{"created_by", "uuid", false, ""},
		{"created_at", tz, false, "now()"},
	}
	if got := principalColumns(t, conn, "price_table"); !reflect.DeepEqual(got, wantCols) {
		t.Errorf("price_table columns:\n got %+v\nwant %+v", got, wantCols)
	}
	cons := principalConstraints(t, conn, "price_table")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK ((unit = ANY (ARRAY['input_mtok'::text, 'output_mtok'::text, 'cache_read_mtok'::text, 'cache_write_mtok'::text, 'cache_write_1h_mtok'::text, 'request'::text])))",
		"EXCLUDE USING gist (provider WITH =, model WITH =, unit WITH =, tstzrange(valid_from, valid_to) WITH &&)",
		"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)",
		"UNIQUE (provider, model, unit, valid_from)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("price_table constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX price_table_lookup_idx ON public.price_table USING btree (provider, model, unit, valid_from DESC)",
		"CREATE INDEX price_table_no_overlap ON public.price_table USING gist (provider, model, unit, tstzrange(valid_from, valid_to))",
		"CREATE UNIQUE INDEX price_table_pkey ON public.price_table USING btree (id)",
		"CREATE UNIQUE INDEX price_table_provider_model_unit_valid_from_key ON public.price_table USING btree (provider, model, unit, valid_from)",
	}
	if got := indexes(t, conn, "price_table"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("price_table indexes:\n got %q\nwant %q", got, wantIdx)
	}
	var gist int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_extension WHERE extname = 'btree_gist'`).Scan(&gist); err != nil {
		t.Fatal(err)
	}
	if gist != 1 {
		t.Errorf("btree_gist installed %d times, want once", gist)
	}

	app := db.Pool(t, pg.RoleApp)
	const (
		person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a01"
		p1     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7b01"
		p2     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7b02"
		p3     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7b03"
		p4     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7b04"
		jan    = "2026-01-01T00:00:00Z"
		feb    = "2026-02-01T00:00:00Z"
		mar    = "2026-03-01T00:00:00Z"
	)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	ins := `INSERT INTO price_table (id, provider, model, unit, price_micros, valid_from, valid_to, source_url, retrieved_at, created_by)
		VALUES ($1, 'vendor', 'model-large', $2, 3000000, $3, $4, 'https://example.com/pricing', $3, $5)`
	mustExec(t, app, ins, p1, "input_mtok", jan, feb, person)
	var currency string
	var price int64
	if err := app.QueryRow(ctx, `SELECT currency, price_micros FROM price_table WHERE id = $1`, p1).Scan(&currency, &price); err != nil ||
		currency != "USD" || price != 3000000 {
		t.Fatalf("read as bothy_app: %s %d, %v", currency, price, err)
	}
	wantState(t, exec(app, ins, p2, "input_mtok", "2026-01-15T00:00:00Z", mar, person), "23P01", "an overlapping validity")
	wantState(t, exec(app, ins, p2, "input_mtok", "2025-12-01T00:00:00Z", nil, person), "23P01", "an open validity over a closed one")
	// [jan, feb) and [feb, ...) share no instant: the adjacent row passes.
	mustExec(t, app, ins, p2, "input_mtok", feb, nil, person)
	// Another unit overlaps freely.
	mustExec(t, app, ins, p3, "output_mtok", jan, nil, person)
	// An empty validity [jan, jan) overlaps nothing, so the unique key alone refuses it.
	wantState(t, exec(app, ins, p4, "input_mtok", jan, jan, person), "23505", "the same valid_from twice")
	wantCheckViolation(t, exec(app, ins, p4, "minutes", jan, nil, person), "an unknown unit")
	wantState(t, exec(app, ins, p4, "request", jan, nil, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a99"), "23503", "an unknown creator")

	// Only the end of a row changes (R-DM-23); a later end that would overlap the next row fails.
	mustExec(t, app, `UPDATE price_table SET valid_to = $2 WHERE id = $1`, p3, mar)
	wantState(t, exec(app, `UPDATE price_table SET valid_to = $2 WHERE id = $1`, p1, mar), "23P01", "an end into the next validity")
	for _, col := range []string{"id", "provider", "model", "unit", "price_micros", "currency", "valid_from", "source_url", "retrieved_at",
		"created_by", "created_at"} {
		wantState(t, exec(app, `UPDATE price_table SET `+col+` = `+col+` WHERE id = $1`, p1), "42501", "bothy_app updates "+col)
	}
	wantState(t, exec(app, `DELETE FROM price_table WHERE id = $1`, p1), "42501", "bothy_app deletes")
}

// TestMigration_CostEntries holds cost_entries against docs/spec/03-data-model.md section 4.9
// column by column, with both checks, every foreign key (among them agent_session_id ->
// agent_sessions, which M2-12 left to this ticket), the partial unique index of supersedes_id and
// the five indexes (M2-22 design note 2 b). As bothy_app it then inserts and reads an entry, and
// shows the rules: the checks, the keys, an entry superseded at most once, and no update or delete
// (append only; ruling D-M2-22-1).
func TestMigration_CostEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	wantCols := []principalColumn{
		{"id", "uuid", false, ""},
		{"run_id", "uuid", false, ""},
		{"agent_id", "uuid", false, ""},
		{"thread_id", "uuid", false, ""},
		{"channel_id", "uuid", false, ""},
		{"space_id", "uuid", true, ""},
		{"agent_session_id", "uuid", true, ""},
		{"billing_mode", "text", false, ""},
		{"provider", "text", false, ""},
		{"model", "text", false, ""},
		{"tokens_in", "bigint", false, "0"},
		{"tokens_out", "bigint", false, "0"},
		{"tokens_cache_read", "bigint", false, "0"},
		{"tokens_cache_write", "bigint", false, "0"},
		{"tokens_cache_write_1h", "bigint", false, "0"},
		{"amount_micros", "bigint", false, ""},
		{"currency", "text", false, "'USD'::text"},
		{"is_estimate", "boolean", false, "true"},
		{"source", "text", false, ""},
		{"occurred_at", tz, false, ""},
		{"reconciled_at", tz, true, ""},
		{"supersedes_id", "uuid", true, ""},
	}
	if got := principalColumns(t, conn, "cost_entries"); !reflect.DeepEqual(got, wantCols) {
		t.Errorf("cost_entries columns:\n got %+v\nwant %+v", got, wantCols)
	}
	cons := principalConstraints(t, conn, "cost_entries")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK ((billing_mode = ANY (ARRAY['personal'::text, 'organization'::text])))",
		"CHECK ((source = ANY (ARRAY['adapter_reported'::text, 'computed'::text, 'unpriced'::text, 'reconciled'::text])))",
		"FOREIGN KEY (agent_id) REFERENCES agents(principal_id) ON DELETE RESTRICT",
		"FOREIGN KEY (agent_session_id) REFERENCES agent_sessions(id) ON DELETE RESTRICT",
		"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE RESTRICT",
		"FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE RESTRICT",
		"FOREIGN KEY (space_id) REFERENCES spaces(id) ON DELETE RESTRICT",
		"FOREIGN KEY (supersedes_id) REFERENCES cost_entries(id) ON DELETE RESTRICT",
		"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("cost_entries constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX cost_entries_agent_occurred_idx ON public.cost_entries USING btree (agent_id, occurred_at)",
		"CREATE INDEX cost_entries_estimate_occurred_idx ON public.cost_entries USING btree (is_estimate, occurred_at) WHERE is_estimate",
		"CREATE INDEX cost_entries_run_idx ON public.cost_entries USING btree (run_id)",
		"CREATE INDEX cost_entries_space_occurred_idx ON public.cost_entries USING btree (space_id, occurred_at)",
		"CREATE INDEX cost_entries_thread_idx ON public.cost_entries USING btree (thread_id)",
		"CREATE UNIQUE INDEX cost_entries_pkey ON public.cost_entries USING btree (id)",
		"CREATE UNIQUE INDEX cost_entries_supersedes_key ON public.cost_entries USING btree (supersedes_id) WHERE (supersedes_id IS NOT NULL)",
	}
	if got := indexes(t, conn, "cost_entries"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("cost_entries indexes:\n got %q\nwant %q", got, wantIdx)
	}

	app := db.Pool(t, pg.RoleApp)
	const (
		person  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c01"
		agent   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c02"
		space   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c11"
		channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c21"
		main    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c31"
		run     = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c41"
		session = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c51"
		e1      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d01"
		e2      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d02"
		e3      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d03"
		e4      = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d04"
		unknown = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d99"
	)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'robin', 'Robin')`, agent)
	mustExec(t, app, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agent)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, main, channel, person)
	mustExec(t, app, `INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref) VALUES ($1, $2, $3, 'sess-a')`,
		session, main, agent)
	mustExec(t, app, `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, state, billing_mode_expected,
		invocation_jti, finished_at) VALUES ($1, $2, $3, 'message', $4, 'completed', 'organization', $1, now())`, run, main, agent, person)

	ins := `INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, space_id, agent_session_id, billing_mode,
		provider, model, amount_micros, source, occurred_at, supersedes_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'vendor', 'model-large', 1500, $9, now(), $10)`
	mustExec(t, app, ins, e1, run, agent, main, channel, space, session, "organization", "computed", nil)
	var tokens, cacheWrite1h int64
	var currency string
	var estimate bool
	if err := app.QueryRow(ctx, `SELECT tokens_in, tokens_cache_write_1h, currency, is_estimate FROM cost_entries WHERE id = $1`, e1).
		Scan(&tokens, &cacheWrite1h, &currency, &estimate); err != nil || tokens != 0 || cacheWrite1h != 0 || currency != "USD" || !estimate {
		t.Fatalf("read as bothy_app: %d %d %s %v, %v", tokens, cacheWrite1h, currency, estimate, err)
	}
	// A DM entry has no space, an entry of a run without a session no session.
	mustExec(t, app, ins, e2, run, agent, main, channel, nil, nil, "personal", "unpriced", nil)
	wantCheckViolation(t, exec(app, ins, e3, run, agent, main, channel, space, session, "guest", "computed", nil), "an unknown billing mode")
	wantCheckViolation(t, exec(app, ins, e3, run, agent, main, channel, space, session, "organization", "guessed", nil), "an unknown source")
	wantState(t, exec(app, ins, e3, run, agent, main, channel, space, unknown, "organization", "computed", nil), "23503", "an unknown session")
	wantState(t, exec(app, ins, e3, unknown, agent, main, channel, space, session, "organization", "computed", nil), "23503", "an unknown run")
	wantState(t, exec(app, ins, e3, run, agent, main, channel, space, session, "organization", "reconciled", unknown), "23503",
		"an unknown superseded entry")
	// An entry is superseded at most once.
	mustExec(t, app, ins, e3, run, agent, main, channel, space, session, "organization", "reconciled", e1)
	wantState(t, exec(app, ins, e4, run, agent, main, channel, space, session, "organization", "reconciled", e1), "23505",
		"a second correction of one entry")

	wantState(t, exec(app, `UPDATE cost_entries SET reconciled_at = now() WHERE id = $1`, e1), "42501", "bothy_app updates")
	wantState(t, exec(app, `DELETE FROM cost_entries WHERE id = $1`, e2), "42501", "bothy_app deletes")
	// The key of agent_session_id restricts: the session of a booking cannot go away.
	wantState(t, exec(db.Pool(t, pg.RoleOwner), `DELETE FROM agent_sessions WHERE id = $1`, session), "23001", "deleting a booked session")
}
