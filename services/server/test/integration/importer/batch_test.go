// SPDX-License-Identifier: Apache-2.0

package importer_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// countImported returns the number of import_items rows of the source and of channels named name.
func (w *iw) countImported(source, name string) (items, channels int) {
	w.t.Helper()
	return w.count(`SELECT count(*) FROM import_items WHERE source_id = $1`, source),
		w.count(`SELECT count(*) FROM channels WHERE lower(name) = lower($1)`, name)
}

// serverStats returns stats.server of the source.
func (w *iw) serverStats(id string) map[string]any {
	w.t.Helper()
	s := w.get(id)
	m, _ := s.Stats["server"].(map[string]any)
	return m
}

// TestApplyBatch_RepeatSameTargets (plan acceptance, 22 7.2): a structure batch twice gives the
// same targets; the second time appends no event and writes no row.
func TestApplyBatch_RepeatSameTargets(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.running()
	seq := w.maxSeq()
	first := byKey(w.mustBatch(id, structure()...))
	types := map[string]int{}
	for _, e := range w.events(seq) {
		types[e.Type]++
	}
	if types["space.created"] != 1 || types["channel.created"] != 3 || types["import.progress"] != 1 || types["user.updated"] != 2 {
		t.Errorf("events of the first batch %v", types)
	}
	for key, want := range map[string]string{
		"space:" + catNew: "posted", "channel:" + chText: "posted", "channel:" + chForum: "posted", "channel:" + chHere: "posted",
		"forum_tag:" + chForum + ":9": "posted", "user:" + userAlex: "posted", "user:" + userBot: "posted", "user:" + userSam: "posted",
		"user:" + userHook: "posted", "user:" + userOtherB: "skipped",
	} {
		if it := first[key]; it.State != want || (want == "posted") != (it.TargetID != "") {
			t.Errorf("%s = %+v, want %s", key, it, want)
		}
	}
	if first["user:"+userAlex].TargetID != w.alex.PrincipalID || first["user:"+userBot].TargetID != w.lotse.PrincipalID {
		t.Errorf("mapped people %+v %+v", first["user:"+userAlex], first["user:"+userBot])
	}
	items, channels := w.countImported(id, "tickets")
	seq = w.maxSeq()
	second := byKey(w.mustBatch(id, structure()...))
	for key, it := range first {
		if second[key].TargetID != it.TargetID || second[key].ID != it.ID || second[key].State != it.State {
			t.Errorf("%s: %+v then %+v", key, it, second[key])
		}
	}
	if ev := w.events(seq); len(ev) != 0 {
		t.Errorf("a repeated batch appended %+v", ev)
	}
	if i, c := w.countImported(id, "tickets"); i != items || c != channels || c != 1 {
		t.Errorf("rows %d %d, want %d %d", i, c, items, channels)
	}
	stats := w.serverStats(id)
	if ch, _ := stats["channel"].(map[string]any); ch["posted"] != float64(3) {
		t.Errorf("stats %v", stats)
	}
	var page struct {
		Items      []item `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	w.must(w.robin, http.MethodGet, "/imports/"+id+"/items?state=skipped", nil, nil, http.StatusOK).JSON(t, &page)
	if len(page.Items) != 1 || page.Items[0].OriginKey != "user:"+userOtherB {
		t.Errorf("skipped items %+v", page.Items)
	}
	w.must(w.robin, http.MethodGet, "/imports/"+id+"/items?limit=4", nil, nil, http.StatusOK).JSON(t, &page)
	if len(page.Items) != 4 || page.NextCursor == "" {
		t.Fatalf("first page %+v", page)
	}
	seen := len(page.Items)
	for page.NextCursor != "" {
		cur := page.NextCursor
		page.NextCursor = ""
		w.must(w.robin, http.MethodGet, "/imports/"+id+"/items?limit=4&cursor="+cur, nil, nil, http.StatusOK).JSON(t, &page)
		seen += len(page.Items)
	}
	if seen != len(first) {
		t.Errorf("paged %d items, want %d", seen, len(first))
	}
}

// TestApplyBatch_StoredThen502 (plan acceptance, 22 16 "Stored, then error"): the answer of a
// committed batch is lost (the harness has no hook into the response writer, so the client drops
// the answer it got, which is what a 502 after the commit leaves it with); the retry returns the
// same targets and the counters count nothing twice.
func TestApplyBatch_StoredThen502(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.running()
	_ = w.batch(id, structure()...) // the answer is lost
	stored := w.serverStats(id)
	retry := byKey(w.mustBatch(id, structure()...))
	var rows int
	for key, it := range retry {
		var target string
		if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT coalesce(target_id::text, '') FROM import_items WHERE source_id = $1 AND origin_key = $2`,
			id, key).Scan(&target); err != nil {
			t.Fatal(err)
		}
		if target != it.TargetID {
			t.Errorf("%s: stored %s, answered %s", key, target, it.TargetID)
		}
		rows++
	}
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1`, id); n != rows {
		t.Errorf("%d items, want %d", n, rows)
	}
	b1, _ := json.Marshal(stored)
	b2, _ := json.Marshal(w.serverStats(id))
	if string(b1) != string(b2) {
		t.Errorf("stats changed by the retry: %s then %s", b1, b2)
	}
}

// TestApplyBatch_ConcurrentSameKey (22 7.4): two batches with the same keys in parallel give one
// target per key.
func TestApplyBatch_ConcurrentSameKey(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.running()
	items := structure()[:3]
	var wg sync.WaitGroup
	results := make([][]item, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := w.robin.Do(w.ctx, harness.Request{Method: http.MethodPost, Path: "/imports/" + id + "/batches", Body: map[string]any{"items": items}})
			if r.Status != http.StatusOK {
				t.Errorf("batch %d = %d %s", i, r.Status, r.Body)
				return
			}
			var b struct {
				Items []item `json:"items"`
			}
			_ = json.Unmarshal(r.Body, &b)
			results[i] = b.Items
		}(i)
	}
	wg.Wait()
	for k := range items {
		if results[0][k].TargetID == "" || results[0][k].TargetID != results[1][k].TargetID {
			t.Errorf("item %d: %+v and %+v", k, results[0][k], results[1][k])
		}
	}
	if _, c := w.countImported(id, "tickets"); c != 1 {
		t.Errorf("%d channels named tickets", c)
	}
}

// TestApplyBatch_SourceChanged (22 7.2 item 4): a different content_sha256 answers source_changed,
// counts it and changes nothing else.
func TestApplyBatch_SourceChanged(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.running()
	space := obj("space:"+catNew, "space", map[string]any{"name": "Imported", "sort_key": "a0"})
	space["content_sha256"] = strings.Repeat("ab", 32)
	first := w.mustBatch(id, space)[0]
	space["content_sha256"] = strings.Repeat("cd", 32)
	seq := w.maxSeq()
	again := w.mustBatch(id, space)[0]
	if !again.SourceChanged || again.TargetID != first.TargetID || again.ID != first.ID {
		t.Errorf("first %+v, again %+v", first, again)
	}
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND content_sha256 = decode($2, 'hex')`, id, strings.Repeat("ab", 32)); n != 1 {
		t.Errorf("the stored hash changed")
	}
	if w.serverStats(id)["source_changed"] != float64(1) {
		t.Errorf("stats %v", w.serverStats(id))
	}
	if ev := w.events(seq); len(ev) != 1 || ev[0].Type != "import.progress" {
		t.Errorf("events %+v", ev)
	}
	space["content_sha256"] = "xyz"
	p := w.wantProblem(w.batch(id, space), http.StatusBadRequest, "validation_failed", "")
	if !p.hasError("/items/0/content_sha256", "invalid") {
		t.Errorf("%+v", p.Errors)
	}
}

// TestApplyBatch_NotRunning: a draft and a paused source answer 409 import_not_running.
func TestApplyBatch_NotRunning(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.create()
	w.wantProblem(w.batch(id, structure()[:1]...), http.StatusConflict, "conflict", "import_not_running")
	w.wantProblem(w.do(w.robin, http.MethodPut, "/imports/"+id+"/cursor", map[string]any{"cursor": map[string]any{}}, nil),
		http.StatusConflict, "conflict", "import_not_running")
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/start", nil, nil, http.StatusAccepted)
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/pause", nil, nil, http.StatusOK)
	w.wantProblem(w.batch(id, structure()[:1]...), http.StatusConflict, "conflict", "import_not_running")
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1`, id); n != 0 {
		t.Errorf("%d items written", n)
	}
	w.wantProblem(w.batch("0192f4a0-7b1c-78d2-8a6a-ec465435b8a0", structure()[:1]...), http.StatusNotFound, "not_found", "")
}

// TestApplyBatch_TargetOutsideSource (05 4.12 import.apply): a channel the mapping does not name,
// an existing channel other than the mapped one and a tag of a channel not imported are refused
// and roll the whole batch back.
func TestApplyBatch_TargetOutsideSource(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.running()
	for _, bad := range []map[string]any{
		obj("channel:999", "channel", map[string]any{"name": "x", "kind": "text", "sort_key": "a0"}),
		obj("channel:"+chExisting, "channel", map[string]any{"existing_channel_id": w.existingSpace, "kind": "text", "sort_key": "a0"}),
		obj("forum_tag:"+chForum+":1", "forum_tag", map[string]any{"channel_origin_key": "channel:" + chForum, "name": "bug"}),
	} {
		r := w.batch(id, structure()[0], bad)
		w.wantProblem(r, http.StatusConflict, "conflict", "target_outside_source")
	}
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1`, id); n != 0 {
		t.Errorf("%d items written", n)
	}
	got := byKey(w.mustBatch(id, obj("space:"+catSkip, "space", map[string]any{"name": "Skipped", "sort_key": "a0"}),
		obj("channel:"+chSkip, "channel", map[string]any{"name": "skipped", "kind": "text", "sort_key": "a0"}),
		obj("space:"+catExisting, "space", map[string]any{"existing_space_id": w.existingSpace, "sort_key": "a0"}),
		obj("channel:"+chExisting, "channel", map[string]any{"existing_channel_id": w.existingChannel, "kind": "text", "sort_key": "a0"})))
	if got["space:"+catSkip].State != "skipped" || got["channel:"+chSkip].State != "skipped" ||
		got["space:"+catExisting].TargetID != w.existingSpace || got["channel:"+chExisting].TargetID != w.existingChannel {
		t.Errorf("skip and adopt %+v", got)
	}
	if _, ch, _ := w.channel(w.robin, w.existingChannel); ch["read_only"] != true {
		t.Errorf("an adopted channel with lead discord is read-only: %v", ch)
	}
}

// TestApplyBatch_NameCollision (22 5.1 rule 1): a new space or channel whose name a live object
// the import did not create holds is 409 name_collision.
func TestApplyBatch_NameCollision(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.running()
	w.wantProblem(w.batch(id, obj("space:"+catCollide, "space", map[string]any{"name": "existing", "sort_key": "a0"})),
		http.StatusConflict, "conflict", "name_collision")
	w.mustBatch(id, obj("space:"+catExisting, "space", map[string]any{"existing_space_id": w.existingSpace, "sort_key": "a0"}))
	w.wantProblem(w.batch(id, obj("channel:"+chCollide, "channel", map[string]any{"space_origin_key": "space:" + catExisting, "name": "OLD",
		"kind": "text", "sort_key": "a5"})), http.StatusConflict, "conflict", "name_collision")
}

// TestApplyBatch_BatchShapes (M8-11 design note 1): thread, message and pin items need a batch
// with thread_origin_key, and structure items are refused in one; data outside the schema is
// validation_failed at its pointer.
func TestApplyBatch_BatchShapes(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.running()
	p := w.wantProblem(w.batch(id, obj("thread:1", "thread", map[string]any{}), obj("message:2", "message", map[string]any{}),
		obj("pin:3", "pin", map[string]any{})), http.StatusBadRequest, "validation_failed", "")
	for i := range 3 {
		if !p.hasError("/items/"+string(rune('0'+i))+"/item_kind", "invalid") {
			t.Errorf("item %d: %+v", i, p.Errors)
		}
	}
	r := w.do(w.robin, http.MethodPost, "/imports/"+id+"/batches", map[string]any{"thread_origin_key": "thread:1", "items": structure()[:1]}, nil)
	if p := w.wantProblem(r, http.StatusBadRequest, "validation_failed", ""); !p.hasError("/items/0/item_kind", "invalid") {
		t.Errorf("%+v", p.Errors)
	}
	p = w.wantProblem(w.batch(id, obj("space:1", "channel", map[string]any{"kind": "text", "sort_key": "a0", "name": "x", "read_only": true})),
		http.StatusBadRequest, "validation_failed", "")
	if !p.hasError("/items/0/origin_key", "kind_mismatch") || !p.hasError("/items/0/data/read_only", "unknown_member") {
		t.Errorf("%+v", p.Errors)
	}
	p = w.wantProblem(w.batch(id, obj("space:"+catNew, "space", map[string]any{"name": "Imported", "sort_key": "!"})),
		http.StatusBadRequest, "validation_failed", "")
	if !p.hasError("/items/0/data/sort_key", "invalid") {
		t.Errorf("%+v", p.Errors)
	}
}
