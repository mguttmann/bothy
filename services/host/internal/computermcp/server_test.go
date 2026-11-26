// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// wantTools are the tools of 27 7 with the members of their parameter schema (6.4 plus
// scale_id, D-CU-05-4) and the required ones.
var wantTools = map[string]struct{ props, required []string }{
	"list_desktops":   {},
	"open_session":    {props: []string{"desktop"}, required: []string{"desktop"}},
	"screenshot":      {},
	"zoom":            {props: []string{"region", "scale_id"}, required: []string{"region"}},
	"left_click":      {props: []string{"coordinate", "modifiers", "scale_id"}, required: []string{"coordinate"}},
	"right_click":     {props: []string{"coordinate", "modifiers", "scale_id"}, required: []string{"coordinate"}},
	"middle_click":    {props: []string{"coordinate", "modifiers", "scale_id"}, required: []string{"coordinate"}},
	"double_click":    {props: []string{"coordinate", "modifiers", "scale_id"}, required: []string{"coordinate"}},
	"triple_click":    {props: []string{"coordinate", "modifiers", "scale_id"}, required: []string{"coordinate"}},
	"mouse_move":      {props: []string{"coordinate", "scale_id"}, required: []string{"coordinate"}},
	"cursor_position": {},
	"scroll":          {props: []string{"amount", "coordinate", "direction", "scale_id"}, required: []string{"amount", "coordinate", "direction"}},
	"type":            {props: []string{"text"}, required: []string{"text"}},
	"key":             {props: []string{"keys", "repeat"}, required: []string{"keys"}},
	"wait":            {props: []string{"duration_ms"}, required: []string{"duration_ms"}},
	"batch":           {props: []string{"actions"}, required: []string{"actions"}},
	"done":            {},
}

func TestToolsListEqualsChapter27(t *testing.T) {
	r := newRig(t)
	r.initialize()
	resp := r.rpc("tools/list", map[string]any{})
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	var res struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			InputSchema struct {
				Type                 string                     `json:"type"`
				Properties           map[string]json.RawMessage `json:"properties"`
				Required             []string                   `json:"required"`
				AdditionalProperties *bool                      `json:"additionalProperties"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		want, ok := wantTools[tool.Name]
		if !ok {
			t.Errorf("tool %s is not in chapter 27 7", tool.Name)
			continue
		}
		var props []string
		for k := range tool.InputSchema.Properties {
			props = append(props, k)
		}
		sort.Strings(props)
		req := slices.Clone(tool.InputSchema.Required)
		sort.Strings(req)
		if tool.InputSchema.Type != "object" || !slices.Equal(props, want.props) || !slices.Equal(req, want.required) ||
			tool.InputSchema.AdditionalProperties == nil || *tool.InputSchema.AdditionalProperties || tool.Description == "" {
			t.Errorf("tool %s: type %q props %v required %v additional %v", tool.Name, tool.InputSchema.Type, props, req, tool.InputSchema.AdditionalProperties)
		}
	}
	if len(names) != len(wantTools) {
		t.Errorf("tools %v, want the %d of chapter 27 7", names, len(wantTools))
	}
	// The descriptions steer toward fewer model rounds (27 7): a batch with its wait, the host
	// ID of list_desktops, the first screenshot of open_session, keys instead of the pointer.
	desc := map[string]string{}
	for _, tool := range res.Tools {
		desc[tool.Name] = tool.Description
	}
	for name, phrase := range map[string]string{
		toolBatch:        "with the wait that follows it as the last item",
		toolListDesktops: "Pass the host ID to open_session",
		toolOpenSession:  "Returns the first screenshot",
		actKey:           "cmd+l (ctrl+l on Windows)",
		actType:          "type its full address",
	} {
		if !strings.Contains(desc[name], phrase) {
			t.Errorf("description of %s lacks %q", name, phrase)
		}
	}
	r.stop()
}

// TestConformance is CU-T19: list, open, image blocks with the data line, the default scale_id,
// zoom as PNG, cursor_position, done, and nothing after done.
func TestConformance(t *testing.T) {
	r := newRig(t)
	r.initialize()

	list := r.call("list_desktops", map[string]any{})
	if list.IsError || !strings.Contains(list.text(), tDesktopA) || !strings.Contains(list.text(), tHostA) {
		t.Fatalf("list_desktops: %+v", list)
	}
	if bad := r.call("screenshot", map[string]any{}); !bad.IsError || !strings.HasPrefix(bad.text(), "no_session") {
		t.Errorf("screenshot without a session: %+v", bad)
	}
	// open_session returns the first screenshot (27 7).
	opened := r.open()
	if img, mime := opened.image(t); mime != "image/jpeg" || !bytes.Equal(img, testImage(3000, 1)) ||
		!strings.Contains(opened.text(), "Computer session open on "+tDesktopA) || !strings.Contains(opened.text(), "scale_id 4") ||
		!strings.Contains(opened.text(), dataLine) {
		t.Fatalf("open_session: %q with a %q image of %d bytes", opened.text(), mime, len(img))
	}

	shot := r.call("screenshot", map[string]any{})
	img, mime := shot.image(t)
	if shot.IsError || mime != "image/jpeg" || !bytes.Equal(img, testImage(3000, 2)) {
		t.Fatalf("screenshot: error %v mime %q image of %d bytes", shot.IsError, mime, len(img))
	}
	if shot.Content[0].Type != "text" {
		t.Fatal("the first block is not text")
	}
	for _, want := range []string{"1280", "800", "scale_id 4", "Editor", "com.example.editor", dataLine} {
		if !strings.Contains(shot.text(), want) {
			t.Errorf("text block %q lacks %q", shot.text(), want)
		}
	}

	click := r.call("left_click", map[string]any{"coordinate": []int{10, 20}, "modifiers": []string{"shift"}})
	if click.IsError {
		t.Fatalf("left_click: %s", click.text())
	}
	r.host.mu.Lock()
	r.host.script = func(a frame.Action) []frame.Frame {
		switch a.Action {
		case "zoom":
			return okImage(a, 70000, frame.FormatPNG)
		case "cursor_position":
			rf, _ := frame.NewResult(frame.Result{Seq: a.Seq, Code: "ok", ScaleID: 4, App: frame.App{ID: "com.example.editor", Name: "Editor"},
				Cursor: &frame.Point{X: 11, Y: 22}})
			return []frame.Frame{rf}
		}
		return okImage(a, 3000, frame.FormatJPEG)
	}
	r.host.mu.Unlock()
	zoom := r.call("zoom", map[string]any{"region": []int{0, 0, 100, 50}, "scale_id": 4})
	if img, mime := zoom.image(t); zoom.IsError || mime != "image/png" || !bytes.Equal(img, testImage(70000, 4)) {
		t.Errorf("zoom: error %v mime %q %d bytes", zoom.IsError, mime, len(img))
	}
	cur := r.call("cursor_position", map[string]any{})
	if cur.IsError || !strings.Contains(cur.text(), "11") || !strings.Contains(cur.text(), "22") || len(cur.Content) != 1 {
		t.Errorf("cursor_position: %+v", cur)
	}

	got := r.host.received()
	if len(got) != 4 {
		t.Fatalf("host got %d actions", len(got))
	}
	for i, a := range got {
		if a.Seq != int64(i+2) { // seq 1 is the screenshot of open_session
			t.Errorf("action %d has seq %d", i, a.Seq)
		}
	}
	var args map[string]any
	_ = json.Unmarshal(got[1].Args, &args)
	if got[1].Action != "left_click" || args["scale_id"] != float64(4) || len(args) != 3 {
		t.Errorf("left_click args %s, want the scale_id of the last image", got[1].Args)
	}

	done := r.call("done", map[string]any{})
	if done.IsError || !r.host.gotClose() {
		t.Errorf("done: %+v, close %v", done, r.host.gotClose())
	}
	if after := r.call("screenshot", map[string]any{}); !after.IsError || !strings.HasPrefix(after.text(), "no_session") {
		t.Errorf("screenshot after done: %+v", after)
	}
	r.stop()
	if strings.Contains(r.logBuf.String(), "Editor") || !strings.Contains(r.logBuf.String(), "left_click") {
		t.Errorf("log %s", r.logBuf.String())
	}
}

// TestBatchStopsAtFirstFailure: the second item fails, the later ones are not executed and
// answer the fixed text, one image at the end.
func TestBatchStopsAtFirstFailure(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	r.host.mu.Lock()
	r.host.script = func(a frame.Action) []frame.Frame {
		if a.Action == "key" {
			return codeOnly(a, "invalid_action")
		}
		return okImage(a, 2000, frame.FormatJPEG)
	}
	r.host.mu.Unlock()
	res := r.call("batch", map[string]any{"actions": []any{
		map[string]any{"action": "screenshot"},
		map[string]any{"action": "key", "keys": "cmd+q"},
		map[string]any{"action": "type", "text": "hello Robin"},
		map[string]any{"action": "left_click", "coordinate": []int{1, 2}, "scale_id": 4},
	}})
	if !res.IsError {
		t.Error("batch with a failure is not an error")
	}
	if got := r.host.received(); len(got) != 2 || got[1].Action != "key" {
		t.Fatalf("host got %d actions", len(got))
	}
	text := res.text()
	if strings.Count(text, "Not executed: an earlier computer action in this turn failed.") != 2 || !strings.Contains(text, "invalid_action") {
		t.Errorf("batch text %q", text)
	}
	var images int
	for _, c := range res.Content {
		if c.Type == "image" {
			images++
		}
	}
	if img, _ := res.image(t); images != 1 || !bytes.Equal(img, testImage(2000, 2)) {
		t.Errorf("batch has %d images", images)
	}
	// A batch of successes returns only the last image.
	ok := r.call("batch", map[string]any{"actions": []any{map[string]any{"action": "screenshot"}, map[string]any{"action": "mouse_move", "coordinate": []int{5, 5}}}})
	if img, _ := ok.image(t); ok.IsError || !bytes.Equal(img, testImage(2000, 5)) || strings.Contains(ok.text(), "Not executed") {
		t.Errorf("batch of successes: %+v", ok.text())
	}
	r.stop()
}

// TestBatchQuickItems: every item of a batch but the last is sent with quick, so the host skips
// its settle wait and its image; the last item returns the one image (27 6.4, 7). A host whose
// message 2 does not name the feature gets no quick.
func TestBatchQuickItems(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	res := r.call("batch", map[string]any{"actions": []any{
		map[string]any{"action": "left_click", "coordinate": []int{1, 2}, "scale_id": 4},
		map[string]any{"action": "key", "keys": "cmd+l"},
		map[string]any{"action": "type", "text": "example.com"},
		map[string]any{"action": "key", "keys": "Return"},
		map[string]any{"action": "wait", "duration_ms": 1500},
	}})
	if res.IsError {
		t.Fatalf("batch: %s", res.text())
	}
	got := r.host.received()
	if len(got) != 5 {
		t.Fatalf("host got %d actions", len(got))
	}
	for i, a := range got {
		if a.Quick != (i < len(got)-1) {
			t.Errorf("item %d (%s) has quick %v", i+1, a.Action, a.Quick)
		}
	}
	if img, _ := res.image(t); !bytes.Equal(img, testImage(3000, 6)) || got[4].Seq != 6 { // seq 1 is the screenshot of open_session
		t.Errorf("batch image is not the one of the last item")
	}
	r.stop()

	old := newRig(t)
	old.host.features = nil
	old.initialize()
	old.open()
	if res := old.call("batch", map[string]any{"actions": []any{
		map[string]any{"action": "key", "keys": "cmd+l"},
		map[string]any{"action": "key", "keys": "Return"},
	}}); res.IsError {
		t.Fatalf("batch at an older host: %s", res.text())
	}
	for _, a := range old.host.received() {
		if a.Quick {
			t.Errorf("an older host got quick on %s", a.Action)
		}
	}
	old.stop()
}

// TestTypeSplitsLongText: a text longer than one frame becomes several type actions whose
// texts are valid UTF-8 and join to the text.
func TestTypeSplitsLongText(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	text := strings.Repeat("a", frame.MaxLen-60) + strings.Repeat("ä\U0001F600<&>", 3000)
	res := r.call("type", map[string]any{"text": text})
	if res.IsError {
		t.Fatalf("type: %s", res.text())
	}
	var joined strings.Builder
	r.host.mu.Lock()
	frames := slices.Clone(r.host.frames)
	r.host.mu.Unlock()
	if len(frames) < 2 {
		t.Fatalf("one type action for %d bytes", len(text))
	}
	for _, pt := range frames {
		if len(pt) > frame.MaxLen {
			t.Errorf("frame of %d bytes", len(pt))
		}
		f, err := frame.Decode(pt, frame.KindComputer, frame.FromDevice)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := f.ActionBody()
		var args struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(a.Args, &args); err != nil || a.Action != "type" {
			t.Fatalf("action %s %v", a.Action, err)
		}
		// Only the last part waits computer.settle and returns the image (27 7).
		if last := a.Seq == int64(len(frames))+1; a.Quick == last { // seq 1 is the screenshot of open_session
			t.Errorf("part %d of %d has quick %v", a.Seq, len(frames), a.Quick)
		}
		joined.WriteString(args.Text)
	}
	if joined.String() != text {
		t.Error("the parts do not join to the text")
	}
	r.stop()
}

func TestSplitTextBoundaries(t *testing.T) {
	for _, text := range []string{"", "a", strings.Repeat("\U0001F600", 40000), strings.Repeat(" <", 30000), strings.Repeat("x", 200000)} {
		parts := splitText(text)
		var b strings.Builder
		for _, p := range parts {
			f, err := frame.NewAction(frame.Action{Seq: 1 << 62, Action: "type", Args: mustArgs(t, p), Quick: true})
			if err != nil {
				t.Fatalf("part of %d bytes: %v", len(p), err)
			}
			if enc, err := frame.Encode(f); err != nil || len(enc) > frame.MaxLen {
				t.Fatalf("encoded part of %d bytes: %v", len(enc), err)
			}
			b.WriteString(p)
		}
		if b.String() != text {
			t.Errorf("parts of %d bytes do not join", len(text))
		}
	}
}

func mustArgs(t *testing.T, text string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestImageWithWrongBytesIsRefused: the sum of the data frames differs from bytes; the session
// ends and every later call answers stopped.
func TestImageWithWrongBytesIsRefused(t *testing.T) {
	for name, delta := range map[string]int{"short": -1, "long": 1} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			r.initialize()
			r.open()
			r.host.mu.Lock()
			r.host.script = func(a frame.Action) []frame.Frame {
				img := testImage(5000, 1)
				rf, _ := frame.NewResult(frame.Result{Seq: a.Seq, Code: "ok", Image: true, Width: 10, Height: 10, ScaleID: 1, App: frame.App{}})
				bf, _ := frame.NewImageBegin(frame.ImageBegin{Seq: a.Seq, Format: "jpeg", Width: 10, Height: 10, ScaleID: 1, Bytes: int64(len(img) + delta*-1)})
				return []frame.Frame{rf, bf, frame.ImageData(img), frame.ImageEnd()}
			}
			r.host.mu.Unlock()
			res := r.call("screenshot", map[string]any{})
			if !res.IsError || !strings.HasPrefix(res.text(), "stopped") {
				t.Errorf("screenshot with a wrong image: %+v", res.text())
			}
			if img, _ := res.image(t); img != nil {
				t.Error("a broken image reached the model")
			}
			if after := r.call("left_click", map[string]any{"coordinate": []int{1, 1}, "scale_id": 1}); !strings.HasPrefix(after.text(), "stopped") {
				t.Errorf("after the fault: %q", after.text())
			}
			r.stop()
		})
	}
}

// TestStoppedSession: cu_state stopped from the host ends the session locally.
func TestStoppedSession(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	r.host.mu.Lock()
	r.host.script = func(a frame.Action) []frame.Frame {
		st, _ := frame.NewState(frame.State{State: frame.StateStopped, Reason: "stopped_by_user"})
		return []frame.Frame{st}
	}
	r.host.mu.Unlock()
	res := r.call("left_click", map[string]any{"coordinate": []int{1, 1}, "scale_id": 1})
	if !res.IsError || !strings.HasPrefix(res.text(), "stopped") || !strings.Contains(res.text(), "stopped_by_user") {
		t.Errorf("click on a stopped session: %q", res.text())
	}
	if again := r.call("screenshot", map[string]any{}); !strings.HasPrefix(again.text(), "stopped") {
		t.Errorf("later call: %q", again.text())
	}
	if n := len(r.host.received()); n != 1 {
		t.Errorf("host got %d actions", n)
	}
	r.stop()
}

// TestPausedSessionPassesCodes: a paused session opens; the host's screen_locked reaches the
// model as the code.
func TestPausedSessionPassesCodes(t *testing.T) {
	r := newRig(t)
	r.host.mu.Lock()
	r.host.openState = frame.State{State: frame.StatePaused, Reason: "locked"}
	r.host.mu.Unlock()
	r.initialize()
	res := r.open() // no first screenshot while paused: one text block
	if !strings.Contains(res.text(), "paused") || len(res.Content) != 1 {
		t.Errorf("open text %q with %d blocks", res.text(), len(res.Content))
	}
	r.host.mu.Lock()
	r.host.script = func(a frame.Action) []frame.Frame { return codeOnly(a, "screen_locked") }
	r.host.mu.Unlock()
	if got := r.call("screenshot", map[string]any{}); !got.IsError || !strings.HasPrefix(got.text(), "screen_locked") {
		t.Errorf("screenshot while locked: %q", got.text())
	}
	r.stop()
}

// TestPinning is CU-T20.
func TestPinning(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	r.call("done", map[string]any{})
	info, err := os.Stat(r.pins)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("pin file %v %v", info, err)
	}
	var pins map[string]any
	b, _ := os.ReadFile(r.pins)
	if json.Unmarshal(b, &pins) != nil || !strings.Contains(string(b), tHostA) {
		t.Fatalf("pin file %s", b)
	}

	// Same generation, another key: host_key_mismatch before message 1.
	other := newFakeHost(t).static
	r.runner.set(func(fr *fakeRunner) { fr.keys[tHostA] = other })
	before := r.host.attachCount()
	res := r.call("open_session", map[string]any{"desktop": tHostA})
	if !res.IsError || !strings.HasPrefix(res.text(), "host_key_mismatch") {
		t.Fatalf("open with another key: %q", res.text())
	}
	if r.host.attachCount() != before {
		t.Error("the server got an attach after a key mismatch")
	}
	after, _ := os.ReadFile(r.pins)
	if !bytes.Equal(after, b) {
		t.Error("a mismatch changed the pin file")
	}

	// Higher generation, another key: replaced; the next open request says pin_changed.
	r.host.mu.Lock()
	r.host.static = other
	r.host.mu.Unlock()
	r.runner.set(func(fr *fakeRunner) { fr.gens[tHostA] = 2 })
	r.open()
	r.call("done", map[string]any{})
	if b2, _ := os.ReadFile(r.pins); bytes.Equal(b2, b) {
		t.Error("the pin was not replaced")
	}
	r.stop()

	r.start() // a new turn reads the pin file
	r.initialize()
	r.open()
	r.call("done", map[string]any{})
	r.open()
	var opens []bool
	for _, req := range r.runner.seen() {
		if req["op"] == "open" {
			opens = append(opens, req["pin_changed"].(bool))
		}
	}
	if !slices.Equal(opens, []bool{false, false, false, true, false}) {
		t.Errorf("pin_changed of the opens %v", opens)
	}
	r.stop()
}

// TestRefusalCodes maps the refusals of 27 12.4 and the request folder.
func TestRefusalCodes(t *testing.T) {
	cases := []struct {
		status       int
		code, reason string
		want         string
	}{
		{403, "forbidden", "computer_not_owner_run", "not_owner_run"},
		{403, "forbidden", "computer_tool_class", "not_owner_run"},
		{401, "invocation_token_invalid", "run_inactive", "not_owner_run"},
		{403, "forbidden", "computer_not_enabled", "not_enabled"},
		{404, "not_found", "", "not_enabled"},
		{409, "conflict", "computer_busy", "busy"},
		{409, "conflict", "computer_run_busy", "busy"},
		{409, "host_unavailable", "", "host_offline"},
		{0, "request_refused", "", "not_owner_run"},
	}
	r := newRig(t)
	r.initialize()
	for _, c := range cases {
		ans := map[string]any{"v": 1, "ok": false, "status": c.status, "code": c.code, "reason": nil}
		if c.reason != "" {
			ans["reason"] = c.reason
		}
		r.runner.set(func(fr *fakeRunner) { fr.refuse = ans })
		res := r.call("open_session", map[string]any{"desktop": tHostA})
		if !res.IsError || !strings.HasPrefix(res.text(), c.want) {
			t.Errorf("%d %s %s: %q, want %s", c.status, c.code, c.reason, res.text(), c.want)
		}
	}
	r.stop()
}

// TestRequestFileContract checks the request files of 27 10.2 and the timeout.
func TestRequestFileContract(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	r.call("done", map[string]any{})
	r.runner.mu.Lock()
	raw := slices.Clone(r.runner.raw)
	r.runner.mu.Unlock()
	if len(raw) != 2 {
		t.Fatalf("%d requests", len(raw))
	}
	for _, b := range raw {
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		switch m["op"] {
		case "list":
			if !slices.Equal(keys, []string{"nonce", "op", "v"}) {
				t.Errorf("list request members %v", keys)
			}
		case "open":
			if !slices.Equal(keys, []string{"client_ephemeral_key_hash", "host_id", "nonce", "op", "pin_changed", "v"}) {
				t.Errorf("open request members %v", keys)
			}
		}
	}
	if entries, _ := os.ReadDir(r.runner.dir); len(entries) != 0 {
		t.Errorf("request folder keeps %d files", len(entries))
	}

	// No answer within the timeout: host_offline, the files stay for the runner's sweep.
	r.stop()
	r.runner.set(func(fr *fakeRunner) { fr.silent = true })
	cfg := r.config()
	cfg.Timeout = 50 * time.Millisecond
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	res := srv.callTool(r.ctx, "list_desktops", json.RawMessage(`{}`))
	if !res.isError || !strings.HasPrefix(res.text, "host_offline") {
		t.Errorf("timeout: %+v", res)
	}
	entries, _ := os.ReadDir(r.runner.dir)
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".request.json") {
		t.Errorf("after a timeout the folder holds %v", entries)
	}
	info, _ := os.Stat(filepath.Join(r.runner.dir, entries[0].Name()))
	if info.Mode().Perm() != 0o600 {
		t.Errorf("request file mode %v", info.Mode().Perm())
	}
}

// TestNothingWrittenButPinsAndRequests lists every folder the server could write before and
// after a session: only the pin file is new.
func TestNothingWrittenButPinsAndRequests(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	cwd := t.TempDir()
	t.Chdir(cwd)
	r := newRig(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	snap := func() []string {
		var out []string
		for _, d := range []string{tmp, cwd, home, r.runner.dir, filepath.Dir(filepath.Dir(r.pins))} {
			_ = filepath.WalkDir(d, func(p string, e os.DirEntry, err error) error {
				if err == nil && !e.IsDir() {
					out = append(out, p)
				}
				return nil
			})
		}
		sort.Strings(out)
		return out
	}
	before := snap()
	r.initialize()
	r.open()
	r.call("screenshot", map[string]any{})
	r.call("zoom", map[string]any{"region": []int{0, 0, 10, 10}})
	r.call("type", map[string]any{"text": "secret text of Sam"})
	r.call("done", map[string]any{})
	r.stop()
	after := snap()
	added := slices.DeleteFunc(slices.Clone(after), func(p string) bool { return slices.Contains(before, p) })
	if !slices.Equal(added, []string{r.pins}) {
		t.Errorf("files written: %v, want only the pin file", added)
	}
	log := r.logBuf.String()
	for _, secret := range []string{"secret text", "Sam", "tok.", "\"coordinate\"", "region"} {
		if strings.Contains(log, secret) {
			t.Errorf("the log holds %q", secret)
		}
	}
}

// TestProtocolEras: the handshake of the legacy era and server/discover of 2026-07-28.
func TestProtocolEras(t *testing.T) {
	r := newRig(t)
	resp := r.rpc("initialize", map[string]any{"protocolVersion": "1999-01-01", "capabilities": map[string]any{}})
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
		Capabilities struct {
			Tools *json.RawMessage `json:"tools"`
		} `json:"capabilities"`
	}
	if resp.Error != nil || json.Unmarshal(resp.Result, &init) != nil || init.ProtocolVersion != "2025-11-25" || init.ServerInfo.Name != "bothy_computer" || init.Capabilities.Tools == nil {
		t.Errorf("initialize: %s %+v", resp.Result, resp.Error)
	}
	if p := r.rpc("ping", map[string]any{}); p.Error != nil || string(p.Result) != "{}" {
		t.Errorf("ping: %s", p.Result)
	}
	r.stop()

	r.start()
	meta := map[string]any{"io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": map[string]any{}}
	d := r.rpc("server/discover", map[string]any{"_meta": meta})
	var disc struct {
		ResultType        string   `json:"resultType"`
		SupportedVersions []string `json:"supportedVersions"`
	}
	if d.Error != nil || json.Unmarshal(d.Result, &disc) != nil || disc.ResultType != "complete" || !slices.Contains(disc.SupportedVersions, "2026-07-28") {
		t.Errorf("server/discover: %s %+v", d.Result, d.Error)
	}
	l := r.rpc("tools/list", map[string]any{"_meta": meta})
	if l.Error != nil || !strings.Contains(string(l.Result), `"resultType":"complete"`) {
		t.Errorf("tools/list modern: %s", l.Result)
	}
	bad := r.rpc("tools/list", map[string]any{"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": "1900-01-01", "io.modelcontextprotocol/clientCapabilities": map[string]any{}}})
	if bad.Error == nil || bad.Error.Code != -32022 || !strings.Contains(string(bad.Error.Data), "2026-07-28") {
		t.Errorf("unsupported version: %+v", bad.Error)
	}
	missing := r.rpc("tools/list", map[string]any{})
	if missing.Error == nil || missing.Error.Code != -32602 {
		t.Errorf("no version and no initialize: %+v", missing.Error)
	}
	unknown := r.rpc("tools/call", map[string]any{"_meta": meta, "name": "drag", "arguments": map[string]any{}})
	if unknown.Error == nil || unknown.Error.Code != -32602 {
		t.Errorf("unknown tool: %+v", unknown.Error)
	}
	method := r.rpc("resources/list", map[string]any{"_meta": meta})
	if method.Error == nil || method.Error.Code != -32601 {
		t.Errorf("unknown method: %+v", method.Error)
	}
	r.send(`{not json`)
	if parse := r.read(); parse.Error == nil || parse.Error.Code != -32700 {
		t.Errorf("parse error: %+v", parse.Error)
	}
	r.stop()
}

// TestInvalidArguments: well-formed calls with arguments against 6.4 answer invalid_action and
// send nothing.
func TestInvalidArguments(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	for name, args := range map[string]any{
		"left_click":   map[string]any{"coordinate": []int{1}},
		"scroll":       map[string]any{"coordinate": []int{1, 2}, "direction": "up"},
		"key":          map[string]any{"keys": "a", "text": "b"},
		"type":         map[string]any{"text": ""},
		"zoom":         map[string]any{"region": []int{0, 0, 1}},
		"batch":        map[string]any{"actions": []any{}},
		"wait":         map[string]any{"duration_ms": "long"},
		"mouse_move":   map[string]any{"coordinate": []int{1, 2}, "scale_id": 0},
		"double_click": map[string]any{"coordinate": []int{1, 2}, "modifiers": []string{"hyper"}},
	} {
		if res := r.call(name, args); !res.IsError || !strings.HasPrefix(res.text(), "invalid_action") {
			t.Errorf("%s %v: %q", name, args, res.text())
		}
	}
	if n := len(r.host.received()); n != 0 {
		t.Errorf("host got %d actions", n)
	}
	r.stop()
}

// TestClickBeforeAnyImage: a coordinate without a scale_id and without an image is refused; the
// first screenshot of open_session failed here, and open_session says so with the session open.
func TestClickBeforeAnyImage(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.host.mu.Lock()
	r.host.script = func(a frame.Action) []frame.Frame { return codeOnly(a, "os_refused") }
	r.host.mu.Unlock()
	if res := r.open(); len(res.Content) != 1 || !strings.Contains(res.text(), "os_refused") || !strings.Contains(res.text(), "take a screenshot") {
		t.Errorf("open_session with a failed first screenshot: %+v", res)
	}
	if res := r.call("left_click", map[string]any{"coordinate": []int{1, 2}}); !res.IsError || !strings.HasPrefix(res.text(), "invalid_action") {
		t.Errorf("click before an image: %q", res.text())
	}
	r.stop()
}
