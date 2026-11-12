// SPDX-License-Identifier: Apache-2.0

package importer_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// largeThreads is the number of threads in the cursor of TestProgressOfLargeSource: the tool keeps
// two entries per thread in its cursor, so this many make it several times the default of
// sync.event_payload_max_bytes and stay below http.max_json_body_bytes.
const largeThreads = 3000

// toolCursor returns a cursor in the layout of the import tool (22 7.3) with n threads: the
// newest message per channel and thread, the mirrored state per thread and the run lease. The IDs
// are invented digit strings, shorter than the IDs of the source system.
func toolCursor(n int) map[string]any {
	threads, mirrored := map[string]any{}, map[string]any{}
	for i := range n {
		id := strconv.Itoa(1100000000000000 + i)
		threads[id] = strconv.Itoa(1200000000000000 + i)
		mirrored[id] = "o:0123456789abcdef"
	}
	return map[string]any{"version": 1, "run": map[string]any{"id": "r1", "renewed_at": "2026-10-14T09:00:00.000Z"},
		"channels": map[string]any{chText: "1200000000000000"}, "threads": threads, "mirrored": mirrored}
}

// progress is the newest import.progress of a source in the event log.
type progress struct {
	Truncated bool
	Bytes     int
	Import    struct {
		ID     string           `json:"id"`
		State  string           `json:"state"`
		Config map[string]any   `json:"config"`
		Stats  map[string]any   `json:"stats"`
		Cursor *json.RawMessage `json:"cursor"`
	}
}

// lastProgress reads the newest import.progress of the source id after seq; it fails when there is
// none.
func (w *iw) lastProgress(id string, after int64) progress {
	w.t.Helper()
	var p progress
	var payload []byte
	if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT truncated, payload::text FROM event_log
		WHERE type = 'import.progress' AND resource_id = $1 AND seq > $2 ORDER BY seq DESC LIMIT 1`, id, after).Scan(&p.Truncated, &payload); err != nil {
		w.t.Fatalf("no import.progress of the source after seq %d: %v", after, err)
	}
	p.Bytes = len(payload)
	var body struct {
		Import json.RawMessage `json:"import"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		w.t.Fatal(err)
	}
	if err := json.Unmarshal(body.Import, &p.Import); err != nil {
		w.t.Fatal(err)
	}
	return p
}

// TestProgressOfLargeSource (audit finding srv-import-2; 04 payload rule 2): import.progress carries
// the whole source while that fits sync.event_payload_max_bytes. A source whose cursor outgrew the
// limit (the tool adds two entries per thread and never removes one) still takes setImportCursor,
// a thread batch and a state change; their events carry the reduced payload (no cursor, an empty
// config, state and counters) with truncated set, and getImport returns the whole cursor. A cursor
// above http.max_json_body_bytes is 413 payload_too_large and leaves the stored one.
func TestProgressOfLargeSource(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	limit, err := strconv.Atoi(w.s.ConfigValue("sync.event_payload_max_bytes"))
	if err != nil {
		t.Fatal(err)
	}

	seq := w.maxSeq()
	w.must(w.robin, http.MethodPut, "/imports/"+id+"/cursor", map[string]any{"cursor": toolCursor(2)}, nil, http.StatusOK)
	if p := w.lastProgress(id, seq); p.Truncated || p.Import.Cursor == nil || len(p.Import.Config) == 0 {
		t.Fatalf("the event of a small source is not whole: truncated %v, cursor %v, config %v", p.Truncated, p.Import.Cursor != nil, p.Import.Config)
	}

	cursor := toolCursor(largeThreads)
	if b, _ := json.Marshal(cursor); len(b) < 2*limit {
		t.Fatalf("the cursor of the test has %d bytes, want more than twice the limit %d", len(b), limit)
	}
	seq = w.maxSeq()
	stats := map[string]any{"messages": 7}
	w.must(w.robin, http.MethodPut, "/imports/"+id+"/cursor", map[string]any{"cursor": cursor, "stats": stats}, nil, http.StatusOK)
	reduced := func(step, state string, after int64) {
		t.Helper()
		p := w.lastProgress(id, after)
		if !p.Truncated || p.Bytes >= limit {
			t.Errorf("%s: truncated %v with %d bytes, want a truncated payload below %d", step, p.Truncated, p.Bytes, limit)
		}
		if p.Import.Cursor != nil || len(p.Import.Config) != 0 {
			t.Errorf("%s: the reduced payload carries cursor %v or config %v", step, p.Import.Cursor != nil, p.Import.Config)
		}
		tool, _ := p.Import.Stats["tool"].(map[string]any)
		if p.Import.ID != id || p.Import.State != state || tool["messages"] != float64(7) {
			t.Errorf("%s: payload of %s in state %s with tool stats %v", step, p.Import.ID, p.Import.State, tool)
		}
	}
	reduced("setImportCursor", "running", seq)
	got := w.get(id)
	if threads, _ := got.Cursor["threads"].(map[string]any); len(threads) != largeThreads {
		t.Errorf("getImport returns %d threads of the cursor, want %d", len(threads), largeThreads)
	}
	if channels, _ := got.Config["channels"].([]any); len(channels) == 0 {
		t.Error("getImport returns no mapping")
	}

	seq = w.maxSeq()
	w.mustThreadBatch(id, "thread:6901", threadObj("6901", chForum, "Question", nil), msgObj("7901", userAlex, "first", tsStart, nil))
	reduced("applyImportBatch", "running", seq)

	seq = w.maxSeq()
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/pause", nil, nil, http.StatusOK)
	reduced("pauseImport", "paused", seq)
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/start", nil, nil, http.StatusAccepted)

	// The stored cursor is bounded by the body limit of the operation, with its own answer.
	body, err := strconv.Atoi(w.s.ConfigValue("http.max_json_body_bytes"))
	if err != nil {
		t.Fatal(err)
	}
	tooLarge := toolCursor(body / 60)
	if b, _ := json.Marshal(tooLarge); len(b) <= body {
		t.Fatalf("the cursor of the test has %d bytes, want more than http.max_json_body_bytes (%d)", len(b), body)
	}
	w.wantProblem(w.do(w.robin, http.MethodPut, "/imports/"+id+"/cursor", map[string]any{"cursor": tooLarge}, nil),
		http.StatusRequestEntityTooLarge, "payload_too_large", "")
	if threads, _ := w.get(id).Cursor["threads"].(map[string]any); len(threads) != largeThreads {
		t.Errorf("the refused cursor changed the stored one: %d threads, want %d", len(threads), largeThreads)
	}
}
