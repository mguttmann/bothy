// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// personalFixture holds the rows of the personal data tests: Alex is the subject, Sam another
// person with rows of the same kinds, Robin the creator of the space.
type personalFixture struct {
	e                      *env
	alex, sam              ids.ID
	thread, forum, channel ids.ID
	alexThreadDraft        ids.ID
	alexChannelDraft       ids.ID
	alexFile               ids.ID
}

// newPersonalFixture creates the people, a space with a forum channel and a thread, and for Alex
// and Sam a read state, a thread draft, a setting and an idempotency key; Alex also has a channel
// draft (a new forum post).
func newPersonalFixture(t *testing.T) *personalFixture {
	t.Helper()
	ctx := context.Background()
	e := newEnv(t, envOptions{})
	su := e.db.SuperConn(t)
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := su.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	f := &personalFixture{e: e, alex: ids.New(), sam: ids.New(), thread: ids.New(), forum: ids.New(),
		alexThreadDraft: ids.New(), alexChannelDraft: ids.New(), alexFile: ids.New()}
	robin, space := ids.New(), ids.New()
	f.channel = f.forum
	for handle, id := range map[string]ids.ID{"alex": f.alex, "sam": f.sam, "robin": robin} {
		exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, id.String(), handle)
		exec(`INSERT INTO users (principal_id, email) VALUES ($1, $2)`, id.String(), handle+"@example.com")
	}
	exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), robin.String())
	exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'ideas', 'a0', 20, $3)`, f.forum.String(), space.String(), robin.String())
	exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, f.thread.String(), f.forum.String(), robin.String())
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	for _, p := range []struct {
		who         ids.ID
		draft, text string
		seq         int
	}{{f.alex, f.alexThreadDraft.String(), "alex draft", 7}, {f.sam, ids.New().String(), "sam draft", 3}} {
		exec(`INSERT INTO read_states (user_id, thread_id, channel_id, last_read_seq, updated_at) VALUES ($1, $2, $3, $4, $5)`,
			p.who.String(), f.thread.String(), f.forum.String(), p.seq, at)
		files := []string{}
		if p.who == f.alex {
			files = append(files, f.alexFile.String())
		}
		exec(`INSERT INTO drafts (id, user_id, thread_id, channel_id, content_md, file_ids, updated_at) VALUES ($1, $2, $3, $4, $5, $6::uuid[], $7)`,
			p.draft, p.who.String(), f.thread.String(), f.forum.String(), p.text, files, at)
		exec(`INSERT INTO user_settings (user_id, key, value, updated_at) VALUES ($1, 'appearance.theme', '"dark"', $2)`, p.who.String(), at)
		exec(`INSERT INTO idempotency_keys (principal_id, key, action, request_sha256, state, response_status, response_body, expires_at)
			VALUES ($1, $2, 'settings.update', '\x01', 'completed', 200, '{"ok":true}', $3)`, p.who.String(), ids.New().String(), at.Add(time.Hour))
	}
	exec(`INSERT INTO drafts (id, user_id, thread_id, channel_id, content_md, updated_at) VALUES ($1, $2, NULL, $3, 'new post', $4)`,
		f.alexChannelDraft.String(), f.alex.String(), f.forum.String(), at)
	return f
}

// participant returns the participant of sync over the fixture.
func (f *personalFixture) participant(t *testing.T) *app.PersonalData {
	t.Helper()
	p, err := app.NewPersonalData(app.PersonalDataDeps{Rows: postgres.NewPersonalRepository(f.e.pool), UoW: f.e.uow, Events: f.e.appender, Clock: f.e.clock})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// personalSchema compiles docs/api/gdpr-export/v1/sections/personal.<key>.schema.json.
func personalSchema(t *testing.T, key string) *jsonschema.Schema {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	name := "personal." + key + ".schema.json"
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../../docs/api/gdpr-export/v1/sections", name)) //nolint:gosec // G304: a fixed schema below the repository root, not input.
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(name, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// section decodes the section key of personal.json, validates it against its schema and returns
// its objects.
func section(t *testing.T, w *personaldata.MemoryWriter, key string) []map[string]any {
	t.Helper()
	raw, ok := w.Sections[personaldata.FilePersonal][key]
	if !ok {
		t.Fatalf("section %s is missing", key)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := personalSchema(t, key).Validate(v); err != nil {
		t.Fatalf("section %s does not match its schema: %v\n%s", key, err, raw)
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// members returns the sorted member names of o.
func members(o map[string]any) []string {
	out := make([]string, 0, len(o))
	for k := range o {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestPersonalDataExport (M7-06): personal.json gets Alex's read states, drafts (with text and
// file ids) and settings with exactly the documented members, and nothing of Sam.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	f := newPersonalFixture(t)
	p := f.participant(t)
	if p.Module() != "sync" {
		t.Fatalf("Module %q", p.Module())
	}
	want := []string{"drafts", "event_log", "idempotency_keys", "read_states", "user_settings"}
	if got := slices.Sorted(slices.Values(p.Tables())); !slices.Equal(got, want) {
		t.Fatalf("Tables %v, want %v", got, want)
	}
	w := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: f.alex}, w); err != nil {
		t.Fatal(err)
	}
	if n := len(w.Sections[personaldata.FilePersonal]); n != 3 {
		t.Fatalf("personal.json has %d sections, want 3: %v", n, w.Sections[personaldata.FilePersonal])
	}
	rs := section(t, w, "read_states")
	if len(rs) != 1 || rs[0]["thread_id"] != f.thread.String() || rs[0]["last_read_seq"] != float64(7) {
		t.Fatalf("read_states %v", rs)
	}
	if got := members(rs[0]); !slices.Equal(got, []string{"channel_id", "last_read_seq", "thread_id", "updated_at", "version"}) {
		t.Fatalf("read state members %v", got)
	}
	drafts := section(t, w, "drafts")
	if len(drafts) != 2 {
		t.Fatalf("drafts %v", drafts)
	}
	byID := map[string]map[string]any{}
	for _, d := range drafts {
		byID[d["id"].(string)] = d
		if d["content_md"] == "sam draft" {
			t.Fatal("Sam's draft is in Alex's export")
		}
	}
	td, cd := byID[f.alexThreadDraft.String()], byID[f.alexChannelDraft.String()]
	if td == nil || td["content_md"] != "alex draft" || len(td["file_ids"].([]any)) != 1 || td["file_ids"].([]any)[0] != f.alexFile.String() {
		t.Fatalf("thread draft %v", td)
	}
	if got := members(td); !slices.Equal(got, []string{"channel_id", "content_md", "file_ids", "id", "thread_id", "updated_at", "version"}) {
		t.Fatalf("thread draft members %v", got)
	}
	if cd == nil || cd["content_md"] != "new post" || cd["thread_id"] != nil || len(cd["file_ids"].([]any)) != 0 {
		t.Fatalf("channel draft %v", cd)
	}
	settings := section(t, w, "settings")
	if len(settings) != 1 || settings[0]["key"] != "appearance.theme" || settings[0]["value"] != "dark" {
		t.Fatalf("settings %v", settings)
	}
	if got := members(settings[0]); !slices.Equal(got, []string{"key", "updated_at", "value", "version"}) {
		t.Fatalf("setting members %v", got)
	}

	// A person without rows gets three empty arrays, never null.
	empty := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, empty); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"read_states", "drafts", "settings"} {
		if got := string(empty.Sections[personaldata.FilePersonal][k]); got != "[]" {
			t.Fatalf("empty %s = %s", k, got)
		}
	}
}

// count returns the result of a counting query.
func (f *personalFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.e.db.SuperConn(t).QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// TestPersonalDataErase (M7-06): every step 1 to 6 runs twice in both content modes; step 3
// deletes Alex's read states, drafts, settings and idempotency keys, appends draft.deleted and
// settings.updated as the system once, and leaves Sam's rows untouched.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			f := newPersonalFixture(t)
			p := f.participant(t)
			ctx := context.Background()
			e := personaldata.Erasure{Subject: f.alex, RequestID: ids.New(), Content: mode}
			for run := range 2 {
				for _, step := range personaldata.ParticipantSteps() {
					if err := p.Erase(ctx, step, e); err != nil {
						t.Fatalf("run %d step %s: %v", run, step, err)
					}
				}
			}
			for table, col := range map[string]string{"read_states": "user_id", "drafts": "user_id", "user_settings": "user_id", "idempotency_keys": "principal_id"} {
				if n := f.count(t, "SELECT count(*) FROM "+table+" WHERE "+col+" = $1", f.alex.String()); n != 0 {
					t.Errorf("%s keeps %d rows of Alex", table, n)
				}
				if n := f.count(t, "SELECT count(*) FROM "+table+" WHERE "+col+" = $1", f.sam.String()); n != 1 {
					t.Errorf("%s has %d rows of Sam, want 1", table, n)
				}
			}
			if n := f.count(t, `SELECT count(*) FROM event_log WHERE type = 'draft.deleted' AND audience_principal_id = $1
				AND actor_kind = 'system' AND actor_principal_id IS NULL AND receive_action = 'self' AND channel_id = $2`, f.alex.String(), f.forum.String()); n != 2 {
				t.Errorf("draft.deleted events = %d, want 2", n)
			}
			if n := f.count(t, `SELECT count(*) FROM event_log WHERE type = 'settings.updated' AND audience_principal_id = $1
				AND actor_kind = 'system' AND actor_principal_id IS NULL AND resource_id = $1
				AND payload->'setting'->>'key' = 'appearance.theme' AND (payload->>'deleted')::boolean`, f.alex.String()); n != 1 {
				t.Errorf("settings.updated events = %d, want 1", n)
			}
			if n := f.count(t, `SELECT count(*) FROM event_log WHERE audience_principal_id = $1`, f.sam.String()); n != 0 {
				t.Errorf("%d events for Sam", n)
			}
		})
	}
}
