// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/importer/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestPersonalDataExport (M7-06, 20 section 5.2): importer writes nothing into the package, also
// for the person who created an import.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	alex, _ := importFixture(t)
	p := postgres.NewPersonalData()
	w := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: alex}, w); err != nil {
		t.Fatal(err)
	}
	if len(w.Sections) != 0 || len(w.Lines) != 0 || len(w.Contents) != 0 {
		t.Errorf("importer exported %v %v %d", w.Sections, w.Lines, len(w.Contents))
	}
	if p.Module() != "importer" || !slices.Equal(p.Tables(), []string{"import_sources", "import_items"}) {
		t.Errorf("module %s, tables %v", p.Module(), p.Tables())
	}
}

// TestPersonalDataErase (M7-06, 20 sections 2 and 4.4): import sources and items stay until
// retention.import_after_completion, so no step changes them, in both content modes and on the
// repeated run after a crash.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	alex, snap := importFixture(t)
	before := snap()
	p := postgres.NewPersonalData()
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		for range 2 {
			for _, step := range personaldata.ParticipantSteps() {
				if err := p.Erase(context.Background(), step, personaldata.Erasure{Subject: alex, RequestID: ids.New(), Content: mode}); err != nil {
					t.Fatalf("%s %s: %v", mode, step, err)
				}
			}
		}
	}
	if after := snap(); after != before {
		t.Errorf("erase changed the tables of importer:\nbefore %s\nafter  %s", before, after)
	}
}

// importFixture creates Alex with an import source and one item and returns Alex and a function
// that reads both tables as canonical JSON text.
func importFixture(t *testing.T) (ids.ID, func() string) {
	t.Helper()
	ctx := context.Background()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	alex, source := ids.New(), ids.New()
	for _, s := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, []any{alex}},
		{`INSERT INTO users (principal_id, email) VALUES ($1, 'alex@example.com')`, []any{alex}},
		{`INSERT INTO import_sources (id, kind, name, config, created_by) VALUES ($1, 'archive', 'Archive', '{}', $2)`, []any{source, alex}},
		{`INSERT INTO import_items (id, source_id, origin_key, item_kind) VALUES ($1, $2, 'user:1234', 'user')`, []any{ids.New(), source}},
	} {
		if _, err := owner.Exec(ctx, s.sql, s.args...); err != nil {
			t.Fatalf("%s: %v", s.sql, err)
		}
	}
	return alex, func() string {
		var s string
		if err := owner.QueryRow(ctx, `SELECT (SELECT jsonb_agg(to_jsonb(s)) FROM import_sources s)::text || (SELECT jsonb_agg(to_jsonb(i)) FROM import_items i)::text`).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
}
