// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestPersonalDataExport (M7-06, 20 section 5.2): billing writes nothing into the package; the
// costs of the person's runs are in runs.jsonl (runs.cost_micros).
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	p := postgres.NewPersonalData()
	w := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, w); err != nil {
		t.Fatal(err)
	}
	if len(w.Sections) != 0 || len(w.Lines) != 0 || len(w.Contents) != 0 {
		t.Errorf("billing exported %v %v %d", w.Sections, w.Lines, len(w.Contents))
	}
	if p.Module() != "billing" || !slices.Equal(p.Tables(), []string{"cost_entries"}) {
		t.Errorf("module %s, tables %v", p.Module(), p.Tables())
	}
}

// TestPersonalDataErase (M7-06, 20 sections 2 and 4.4): cost entries stay until retention.cost,
// so no step changes them, in both content modes and on the repeated run after a crash.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := super.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	// The seed skips the foreign keys to runs, agents, threads and channels, as in the retention
	// test: the participant reads no other table.
	exec(`SET session_replication_role = replica`)
	exec(`INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, billing_mode, provider, model, amount_micros,
		source, occurred_at) VALUES ($1, $2, $2, $2, $2, 'personal', 'example', 'model-a', 10, 'computed', $3)`,
		ids.New(), ids.New(), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	exec(`SET session_replication_role = origin`)
	snap := func() string {
		var s string
		if err := super.QueryRow(ctx, `SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY c.id), '[]')::text FROM cost_entries c`).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	before := snap()
	p := postgres.NewPersonalData()
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		for range 2 {
			for _, step := range personaldata.ParticipantSteps() {
				if err := p.Erase(ctx, step, personaldata.Erasure{Subject: ids.New(), RequestID: ids.New(), Content: mode}); err != nil {
					t.Fatalf("%s %s: %v", mode, step, err)
				}
			}
		}
	}
	if after := snap(); after != before {
		t.Errorf("erase changed cost_entries:\nbefore %s\nafter  %s", before, after)
	}
}
