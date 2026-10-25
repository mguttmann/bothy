// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// instanceFixture creates Alex as the last changer of a setting and of a flag whose rollout names
// Alex, and returns Alex and a function that reads both tables as canonical JSON text.
func instanceFixture(t *testing.T) (ids.ID, func() string) {
	t.Helper()
	ctx := context.Background()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	alex := ids.New()
	for _, s := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, []any{alex}},
		{`INSERT INTO users (principal_id, email) VALUES ($1, 'alex@example.com')`, []any{alex}},
		{`INSERT INTO instance_settings (key, value, updated_by) VALUES ('example.setting', '1', $1)
			ON CONFLICT (key) DO UPDATE SET updated_by = EXCLUDED.updated_by`, []any{alex}},
		{`INSERT INTO feature_flags (key, enabled, rollout, updated_by) VALUES ('example.flag', false, jsonb_build_object('user_ids', jsonb_build_array($1::text)), $2)`, []any{alex.String(), alex}},
	} {
		if _, err := owner.Exec(ctx, s.sql, s.args...); err != nil {
			t.Fatalf("%s: %v", s.sql, err)
		}
	}
	return alex, func() string {
		var s string
		if err := owner.QueryRow(ctx, `SELECT (SELECT jsonb_agg(to_jsonb(s) ORDER BY s.key) FROM instance_settings s)::text
			|| (SELECT jsonb_agg(to_jsonb(f) ORDER BY f.key) FROM feature_flags f)::text`).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
}

// TestPersonalDataExport (M7-06, 20 section 5.2): instance writes nothing into the package, also
// for the last changer of a setting and a person in a rollout list.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	alex, _ := instanceFixture(t)
	p := postgres.NewPersonalData()
	w := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: alex}, w); err != nil {
		t.Fatal(err)
	}
	if len(w.Sections) != 0 || len(w.Lines) != 0 || len(w.Contents) != 0 {
		t.Errorf("instance exported %v %v %d", w.Sections, w.Lines, len(w.Contents))
	}
	if p.Module() != "instance" || !slices.Equal(p.Tables(), []string{"instance_settings", "feature_flags"}) {
		t.Errorf("module %s, tables %v", p.Module(), p.Tables())
	}
}

// TestPersonalDataErase (M7-06, 20 sections 2 and 4.4): updated_by and the rollout list keep the
// pseudonymous ID until the next change, so no step changes the tables, in both content modes and
// on the repeated run after a crash.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	alex, snap := instanceFixture(t)
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
		t.Errorf("erase changed the tables of instance:\nbefore %s\nafter  %s", before, after)
	}
}
