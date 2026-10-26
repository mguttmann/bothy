// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// pdFixture holds Alex and Sam, one app, Alex's uninstalled installation and Sam's active one.
type pdFixture struct {
	alex, app, alexInst  ids.ID
	installed, uninstall time.Time
}

func newPDFixture(t *testing.T, owner *pg.Pool) pdFixture {
	t.Helper()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(context.Background(), sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	f := pdFixture{
		alex: ids.New(), app: ids.New(), alexInst: ids.New(),
		installed: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), uninstall: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC),
	}
	sam := ids.New()
	for handle, id := range map[string]ids.ID{"alex": f.alex, "sam": sam} {
		exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, id, handle)
		exec(`INSERT INTO users (principal_id, email) VALUES ($1, $2)`, id, handle+"@example.com")
	}
	exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'app', 'tracker', 'Tracker')`, f.app)
	exec(`INSERT INTO apps (principal_id, slug, manifest, manifest_version, manifest_sha256) VALUES ($1, 'tracker', '{}', '1', '\x00')`, f.app)
	exec(`INSERT INTO app_installations (id, app_id, state, installed_by, installed_at, uninstalled_at, uninstalled_by)
		VALUES ($1, $2, 'uninstalled', $3, $4, $5, $6)`, f.alexInst, f.app, f.alex, f.installed, f.uninstall, sam)
	exec(`INSERT INTO app_installations (id, app_id, installed_by) VALUES ($1, $2, $3)`, ids.New(), f.app, sam)
	return f
}

// TestPersonalDataExport (M7-06, 20 section 5.2): access.json apps_installed holds Alex's
// installation with exactly the documented members and not Sam's; a person without an
// installation gets an empty array.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	f := newPDFixture(t, db.Pool(t, pg.RoleOwner))
	p := postgres.NewPersonalData(db.Pool(t, pg.RoleApp))
	w := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: f.alex}, w); err != nil {
		t.Fatal(err)
	}
	raw := w.Sections[personaldata.FileAccess]["apps_installed"]
	validateSection(t, "access.apps_installed.schema.json", raw)
	want := `[{"id":"` + f.alexInst.String() + `","app_id":"` + f.app.String() + `","state":"uninstalled","installed_at":"` +
		f.installed.Format(time.RFC3339) + `","uninstalled_at":"` + f.uninstall.Format(time.RFC3339) + `"}]`
	if string(raw) != want {
		t.Errorf("apps_installed =\n%s\nwant\n%s", raw, want)
	}
	if len(w.Sections) != 1 || len(w.Sections[personaldata.FileAccess]) != 1 || len(w.Lines) != 0 {
		t.Errorf("export wrote more than apps_installed: %v %v", w.Sections, w.Lines)
	}
	empty := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, empty); err != nil {
		t.Fatal(err)
	}
	if got := string(empty.Sections[personaldata.FileAccess]["apps_installed"]); got != "[]" {
		t.Errorf("apps_installed of a stranger = %s, want []", got)
	}
}

// TestPersonalDataErase (M7-06, 20 sections 2 and 4.4): apps, installations and webhooks stay
// with the pseudonymous ID of the person who made them, so no step changes a row of the module,
// in both content modes and on the repeated run after a crash.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	f := newPDFixture(t, owner)
	p := postgres.NewPersonalData(db.Pool(t, pg.RoleApp))
	before := pdSnapshot(t, owner, p.Tables())
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		for range 2 {
			for _, step := range personaldata.ParticipantSteps() {
				if err := p.Erase(context.Background(), step, personaldata.Erasure{Subject: f.alex, RequestID: ids.New(), Content: mode}); err != nil {
					t.Fatalf("%s %s: %v", mode, step, err)
				}
			}
		}
	}
	if after := pdSnapshot(t, owner, p.Tables()); !slices.Equal(after, before) {
		t.Errorf("erase changed the tables of apps:\nbefore %v\nafter  %v", before, after)
	}
	if want := []string{"apps", "app_installations", "webhook_subscriptions", "incoming_webhooks"}; !slices.Equal(p.Tables(), want) || p.Module() != "apps" {
		t.Errorf("module %s, tables %v", p.Module(), p.Tables())
	}
}

// validateSection validates raw against the section schema name of the export package.
func validateSection(t *testing.T, name string, raw json.RawMessage) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "..", "docs", "api", "gdpr-export", "v1", "sections", name))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	sch, err := c.Compile(path)
	if err != nil {
		t.Fatalf("schema %s: %v", name, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Errorf("%s: %s does not match: %v", name, raw, err)
	}
}

// pdSnapshot returns every row of tables as canonical JSON text, per table in a stable order.
func pdSnapshot(t *testing.T, q *pg.Pool, tables []string) []string {
	t.Helper()
	out := make([]string, 0, len(tables))
	for _, table := range tables {
		var s string
		if err := q.QueryRow(context.Background(),
			"SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text), '[]')::text FROM "+table+" t").Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, table+"="+s)
	}
	return out
}
