// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/importer/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestRetentionPurge (20 4.1 class import; audit findings fn-import-migration-5 and srv-import-5):
// the participant of retention.import removes, as the application role, the items and then the row
// of a source that is completed or failed and ended before the cutoff, in batches, and reports
// more while a batch was full. A source that ended at or after the cutoff, a source in another
// state and a source without an end keep every row. A failed statement removes nothing.
func TestRetentionPurge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	p := postgres.NewRetention(db.Pool(t, pg.RoleApp))
	if p.Class() != retention.ClassImport || !slices.Equal(p.Tables(), []string{"import_items", "import_sources"}) {
		t.Fatalf("participant of class %s over %v", p.Class(), p.Tables())
	}

	robin := ids.New()
	if _, err := owner.Exec(ctx, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'robin', 'Robin')`, robin); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	const itemsPerSource = 5
	source := func(state string, finished *time.Time) ids.ID {
		t.Helper()
		id := ids.New()
		if _, err := owner.Exec(ctx, `INSERT INTO import_sources (id, kind, name, config, state, created_by, finished_at)
			VALUES ($1, 'discord', 'Old', '{}', $2, $3, $4)`, id, state, robin, finished); err != nil {
			t.Fatal(err)
		}
		for i := range itemsPerSource {
			if _, err := owner.Exec(ctx, `INSERT INTO import_items (id, source_id, origin_key, item_kind, state)
				VALUES ($1, $2, $3, 'user', 'posted')`, ids.New(), id, "user:"+strconv.Itoa(1000+i)); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	before, at := cutoff.Add(-time.Second), cutoff
	completed, failed := source("completed", &before), source("failed", &before)
	kept := map[string]ids.ID{
		"completed at the cutoff":             source("completed", &at),
		"running":                             source("running", nil),
		"paused":                              source("paused", nil),
		"draft":                               source("draft", nil),
		"running with a stray end time":       source("running", &before),
		"completed without an end time":       source("completed", nil),
		"failed at the cutoff":                source("failed", &at),
		"paused with a stray end time before": source("paused", &before),
	}
	rows := func(id ids.ID) (items int, source bool) {
		t.Helper()
		if err := owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM import_items WHERE source_id = $1),
			EXISTS (SELECT 1 FROM import_sources WHERE id = $1)`, id).Scan(&items, &source); err != nil {
			t.Fatal(err)
		}
		return items, source
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if n, more, err := p.Purge(cancelled, cutoff, 2); err == nil || n != 0 || more {
		t.Fatalf("Purge with a cancelled context = %d, %v, %v; want an error and nothing removed", n, more, err)
	}
	if items, src := rows(completed); items != itemsPerSource || !src {
		t.Fatalf("the failed call removed rows: %d items, source %v", items, src)
	}

	// Batches of two: the ten items of the two ended sources take five calls, whose last one also
	// removes the two source rows; the call after it finds nothing.
	total, calls := 0, 0
	for {
		n, more, err := p.Purge(ctx, cutoff, 2)
		if err != nil {
			t.Fatal(err)
		}
		total += n
		calls++
		if !more {
			break
		}
		if calls > 20 {
			t.Fatal("Purge keeps reporting more")
		}
	}
	if want := 2*itemsPerSource + 2; total != want {
		t.Errorf("Purge removed %d rows in %d calls, want %d", total, calls, want)
	}
	for name, id := range map[string]ids.ID{"completed before the cutoff": completed, "failed before the cutoff": failed} {
		if items, src := rows(id); items != 0 || src {
			t.Errorf("%s: %d items and source row %v left", name, items, src)
		}
	}
	for name, id := range kept {
		if items, src := rows(id); items != itemsPerSource || !src {
			t.Errorf("%s: %d items and source row %v, want every row kept", name, items, src)
		}
	}
	if n, more, err := p.Purge(ctx, cutoff, 2); err != nil || n != 0 || more {
		t.Errorf("a further Purge = %d, %v, %v", n, more, err)
	}
}
