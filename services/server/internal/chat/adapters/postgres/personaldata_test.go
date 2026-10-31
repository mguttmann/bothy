// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// sidebarSchema and sidebarChannelsSchema are the schemas of the sections sidebar and
// sidebar_channels of personal.json, from the root of the repository (six levels above this
// package).
var (
	sidebarSchema = filepath.Join("..", "..", "..", "..", "..", "..", "docs", "api", "gdpr-export", "v1", "sections",
		"personal.sidebar.schema.json")
	sidebarChannelsSchema = filepath.Join("..", "..", "..", "..", "..", "..", "docs", "api", "gdpr-export", "v1", "sections",
		"personal.sidebar_channels.schema.json")
)

// personalWorld is a database with Alex (the subject) and Sam, two spaces and the sidebar rows of
// both: Alex has a row for each space (one with a personal key, a channel order and the device of
// the write), Sam one; and a forum with a channel row of each (M4-51).
type personalWorld struct {
	pool             *pg.Pool
	alex, sam        ids.ID
	support, finance ids.ID
	forum, post      ids.ID
	// count returns the sidebar rows of user: space rows plus channel rows.
	count func(user ids.ID) int
}

// newPersonalWorld builds the rows of personalWorld as the database owner.
func newPersonalWorld(t *testing.T) personalWorld {
	t.Helper()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := super.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	w := personalWorld{pool: db.Pool(t, pg.RoleApp), alex: ids.New(), sam: ids.New(), support: ids.New(), finance: ids.New(),
		forum: ids.New(), post: ids.New()}
	for _, p := range []struct {
		id     ids.ID
		handle string
	}{{w.alex, "alex"}, {w.sam, "sam"}} {
		exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, p.id, p.handle)
		exec(`INSERT INTO users (principal_id, email) VALUES ($1, $2)`, p.id, p.handle+"@example.com")
	}
	device := ids.New()
	exec(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Laptop', 'web', 'active')`, device, w.alex)
	exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a', $2), ($3, 'Finance', 'b', $2)`,
		w.support, w.sam, w.finance)
	exec(`INSERT INTO user_space_states (user_id, space_id, collapsed, sort_key, channel_order, updated_by_device_id)
		VALUES ($1, $2, true, 'k', ARRAY[$4::uuid], $3)`, w.alex, w.support, device, w.forum)
	exec(`INSERT INTO user_space_states (user_id, space_id) VALUES ($1, $2), ($3, $2)`, w.alex, w.finance, w.sam)
	exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'Projects', 'a', 20, $3)`, w.forum, w.support, w.sam)
	exec(`INSERT INTO user_channel_states (user_id, channel_id, thread_order, updated_by_device_id) VALUES ($1, $2, ARRAY[$3::uuid], $4)`,
		w.alex, w.forum, w.post, device)
	exec(`INSERT INTO user_channel_states (user_id, channel_id) VALUES ($1, $2)`, w.sam, w.forum)
	w.count = func(user ids.ID) int {
		t.Helper()
		var n int
		if err := super.QueryRow(ctx, `SELECT (SELECT count(*) FROM user_space_states WHERE user_id = $1)::int
			+ (SELECT count(*) FROM user_channel_states WHERE user_id = $1)::int`, user).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	return w
}

// validateSchema checks raw against the JSON Schema at path with format assertions (uuid,
// date-time).
func validateSchema(t *testing.T, path string, raw []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile(abs)
	if err != nil {
		t.Fatalf("schema %s: %v", path, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("%s does not hold %s: %v", path, raw, err)
	}
}

// TestPersonalDataExport (M7-06, 20 section 5.2): the section sidebar of personal.json holds
// exactly Alex's two rows with the documented members (no version, no row of Sam), matches its
// schema, and the participant writes nothing else.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	w := newPersonalWorld(t)
	p := postgres.NewPersonalData(w.pool)
	if p.Module() != "chat" {
		t.Fatalf("Module() = %q", p.Module())
	}
	want := []string{"spaces", "user_space_states", "user_channel_states", "channels", "dm_participants", "threads", "thread_tags"}
	if got := p.Tables(); !slices.Equal(got, want) {
		t.Fatalf("Tables() = %v, want %v", got, want)
	}
	mw := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: w.alex}, mw); err != nil {
		t.Fatal(err)
	}
	if len(mw.Sections) != 1 || len(mw.Sections[personaldata.FilePersonal]) != 2 || len(mw.Lines) != 0 || len(mw.Contents) != 0 {
		t.Fatalf("the export wrote more than the sections sidebar and sidebar_channels: %v %v", mw.Sections, mw.Lines)
	}
	raw := mw.Sections[personaldata.FilePersonal]["sidebar"]
	validateSchema(t, sidebarSchema, raw)
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("sidebar rows %s, want Alex's two", raw)
	}
	byID := map[string]map[string]any{}
	for _, r := range rows {
		byID[r["space_id"].(string)] = r
	}
	s, f := byID[w.support.String()], byID[w.finance.String()]
	if s == nil || f == nil {
		t.Fatalf("sidebar rows %s", raw)
	}
	if order, _ := s["channel_order"].([]any); s["collapsed"] != true || s["sort_key"] != "k" || s["updated_by_device_id"] == nil ||
		len(order) != 1 || order[0] != w.forum.String() {
		t.Errorf("support row %v", s)
	}
	if _, ok := f["sort_key"]; ok || f["collapsed"] != false {
		t.Errorf("finance row %v: an empty key must be absent", f)
	}
	if _, ok := f["channel_order"]; ok {
		t.Errorf("finance row %v: the default channel order must be absent", f)
	}
	// M4-51: Alex's channel row, never Sam's.
	rawChans := mw.Sections[personaldata.FilePersonal]["sidebar_channels"]
	validateSchema(t, sidebarChannelsSchema, rawChans)
	var chans []map[string]any
	if err := json.Unmarshal(rawChans, &chans); err != nil {
		t.Fatal(err)
	}
	if order, _ := chans[0]["thread_order"].([]any); len(chans) != 1 || chans[0]["channel_id"] != w.forum.String() || len(order) != 1 ||
		order[0] != w.post.String() || chans[0]["updated_by_device_id"] == nil {
		t.Fatalf("sidebar_channels %s", rawChans)
	}

	// A person without rows gets an empty array, never null.
	empty := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, empty); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"sidebar", "sidebar_channels"} {
		if got := string(empty.Sections[personaldata.FilePersonal][key]); got != "[]" {
			t.Errorf("%s of a person without rows = %s, want []", key, got)
		}
	}
}

// TestPersonalDataErase (M7-06, 20 section 4.4 item 3): the steps 1 to 6 in order, twice (the
// second run is the resume after a crash), delete Alex's sidebar rows (spaces and channels, M4-51)
// in StepPersonal and nothing before it; Sam's rows and the spaces stay, in both content modes.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			w := newPersonalWorld(t)
			p := postgres.NewPersonalData(w.pool)
			e := personaldata.Erasure{Subject: w.alex, RequestID: ids.New(), Content: mode}
			for run := range 2 {
				for _, step := range personaldata.ParticipantSteps() {
					if err := p.Erase(context.Background(), step, e); err != nil {
						t.Fatalf("run %d step %s: %v", run, step, err)
					}
					want := 3
					if run > 0 || step >= personaldata.StepPersonal {
						want = 0
					}
					if got := w.count(w.alex); got != want {
						t.Fatalf("run %d after step %s: Alex has %d sidebar rows, want %d", run, step, got, want)
					}
				}
			}
			if got := w.count(w.sam); got != 2 {
				t.Errorf("Sam's sidebar rows: %d, want 2", got)
			}
		})
	}
}
