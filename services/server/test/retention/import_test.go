// SPDX-License-Identifier: Apache-2.0

package retention_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// importSource is one import source of TestRetention_ImportAfterCompletion.
type importSource struct {
	id      string
	items   int
	message string
}

// importSource creates a source of the Discord server guild as the owner, starts it and writes a
// space, a channel led here, an author and one message into the main thread of the channel
// through the import interface (22 8); name is the name of the new space. It returns the source,
// the number of its items and the imported message.
func (w *world) importSource(guild, name string) importSource {
	w.t.Helper()
	cat, ch, user, msg := guild+"1", guild+"2", guild+"3", guild+"4"
	create := func(n string) map[string]any { return map[string]any{"create": map[string]any{"name": n}} }
	var src struct {
		ID string `json:"id"`
	}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/imports", Body: map[string]any{
		"kind": "discord", "name": name, "config": map[string]any{
			"version": 1, "guild_id": guild,
			"defaults": map[string]any{"unmapped_human": "placeholder", "unmapped_bot": "skip", "uncategorized_space": cat},
			"spaces":   []any{map[string]any{"discord_category": cat, "target": create(name)}},
			"channels": []any{map[string]any{"discord_channel": ch, "target": create("general"), "lead": "bothy"}},
			"users":    []any{},
		}}}, http.StatusCreated).JSON(w.t, &src)
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/imports/" + src.ID + "/start"}, http.StatusAccepted)
	item := func(key, kind string, data map[string]any) map[string]any {
		return map[string]any{"origin_key": key, "item_kind": kind, "data": data}
	}
	structure := []any{
		item("space:"+cat, "space", map[string]any{"name": name, "sort_key": "a0"}),
		item("channel:"+ch, "channel", map[string]any{"space_origin_key": "space:" + cat, "name": "general", "kind": "text", "sort_key": "a0"}),
		item("user:"+user, "user", map[string]any{"discord_kind": "user", "username": "sam" + guild}),
	}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/imports/" + src.ID + "/batches",
		Body: map[string]any{"items": structure}}, http.StatusOK)
	var res struct {
		Items []struct {
			TargetID string `json:"target_id"`
		} `json:"items"`
	}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/imports/" + src.ID + "/batches", Body: map[string]any{
		"thread_origin_key": "channel:" + ch,
		"items": []any{item("message:"+msg, "message", map[string]any{"author_origin_key": "user:" + user, "content_md": "canary-import-" + guild,
			"created_at": "2025-03-01T08:00:00.000Z"})}}}, http.StatusOK).JSON(w.t, &res)
	if len(res.Items) != 1 || res.Items[0].TargetID == "" {
		w.t.Fatalf("the message batch answered %+v", res)
	}
	return importSource{id: src.ID, items: len(structure) + 1, message: res.Items[0].TargetID}
}

// end ends the source with op (complete or cancel) and moves its end to at.
func (w *world) end(s importSource, op string, at time.Time) {
	w.t.Helper()
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/imports/" + s.id + "/" + op}, http.StatusOK)
	w.exec(`UPDATE import_sources SET finished_at = $2 WHERE id = $1`, s.id, at)
}

// TestRetention_ImportAfterCompletion (20 4.1 class import, 22 13.5 and 14; audit findings
// fn-import-migration-5 and srv-import-5): the job retention.import removes the items and then the
// row of a source that was completed or cancelled more than retention.import_after_completion ago,
// and nothing else: a source that ended inside the period and a running source keep every row.
// The imported messages stay as they are and only lose the reference to their item; the removed
// source answers 404.
func TestRetention_ImportAfterCompletion(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	now := w.clk.Now()
	period, err := config.ParseDuration(w.s.ConfigValue("retention.import_after_completion"))
	if err != nil || period <= 0 {
		t.Fatalf("retention.import_after_completion = %q: %v", w.s.ConfigValue("retention.import_after_completion"), err)
	}
	w.mu.Lock()
	regs := w.reg.Of(retention.ClassImport)
	w.mu.Unlock()
	if len(regs) != 1 || regs[0].Period != period || !slices.Equal(regs[0].Participant.Tables(), []string{"import_items", "import_sources"}) {
		t.Fatalf("the server registers %+v for the class import, want one participant of import_items and import_sources with the period %s", regs, period)
	}

	completed, cancelled := w.importSource("9001", "Completed long ago"), w.importSource("9002", "Cancelled long ago")
	recent, running := w.importSource("9003", "Completed lately"), w.importSource("9004", "Still running")
	w.end(completed, "complete", now.Add(-period-time.Second))
	w.end(cancelled, "cancel", now.Add(-period-time.Second))
	w.end(recent, "complete", now.Add(-period+time.Second))
	items := func(s importSource) int {
		return w.count(`SELECT count(*)::int FROM import_items WHERE source_id = $1`, s.id)
	}
	for _, s := range []importSource{completed, cancelled, recent, running} {
		if n := items(s); n != s.items {
			t.Fatalf("source %s has %d items before the job, want %d", s.id, n, s.items)
		}
	}

	// Batches of two rows (world.run): each source takes several batches.
	if n, want := w.run(retention.ClassImport, now), completed.items+cancelled.items+2; n != want {
		t.Errorf("retention.import removed %d rows, want %d (the items and the rows of the two expired sources)", n, want)
	}
	for _, c := range []struct {
		name string
		s    importSource
		kept bool
	}{{"completed before the period", completed, false}, {"cancelled before the period", cancelled, false},
		{"completed inside the period", recent, true}, {"running", running, true}} {
		wantItems := 0
		if c.kept {
			wantItems = c.s.items
		}
		if n := items(c.s); n != wantItems {
			t.Errorf("%s: %d items after the job, want %d", c.name, n, wantItems)
		}
		if got := w.exists("import_sources", c.s.id); got != c.kept {
			t.Errorf("%s: source row present = %v, want %v", c.name, got, c.kept)
		}
		ref := "import_item_id IS NULL"
		if c.kept {
			ref = "import_item_id IS NOT NULL"
		}
		if n := w.count(`SELECT count(*)::int FROM messages WHERE id = $1 AND origin = 'import' AND deleted_at IS NULL
			AND content_md LIKE 'canary-import-%' AND `+ref, c.s.message); n != 1 {
			t.Errorf("%s: the imported message changed or its item reference is not as expected (%s)", c.name, ref)
		}
		status := http.StatusNotFound
		if c.kept {
			status = http.StatusOK
		}
		harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodGet, Path: "/imports/" + c.s.id}, status)
	}
	if n := w.run(retention.ClassImport, now); n != 0 {
		t.Errorf("a second run removed %d rows", n)
	}
	// The source that ended inside the period goes once the period has passed for it too.
	if n, want := w.run(retention.ClassImport, now.Add(2*time.Second)), recent.items+1; n != want {
		t.Errorf("after the period of the later source the job removed %d rows, want %d", n, want)
	}
	if w.exists("import_sources", recent.id) || !w.exists("import_sources", running.id) || items(running) != running.items {
		t.Error("the later source stayed, or the running source lost rows")
	}
}
