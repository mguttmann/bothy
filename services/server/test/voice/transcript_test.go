// SPDX-License-Identifier: Apache-2.0

package voice_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// transcriptView is the schema Transcript.
type transcriptView struct {
	FileID   string  `json:"file_id"`
	Status   string  `json:"status"`
	Text     *string `json:"text"`
	Language string  `json:"language"`
	Source   string  `json:"source"`
}

// getFileETag reads the file as c and returns the status and the header ETag.
func (w *world) getFileETag(c *harness.Client, fileID string) (int, string) {
	w.t.Helper()
	r := c.Do(w.t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + fileID})
	return r.Status, r.Header.Get("ETag")
}

// submit calls submitTranscript as c with ifMatch ("" sends none) and the body.
func (w *world) submit(c *harness.Client, fileID, ifMatch string, body map[string]any) harness.Response {
	w.t.Helper()
	h := http.Header{}
	if ifMatch != "" {
		h.Set("If-Match", ifMatch)
	}
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPut, Path: "/files/" + fileID + "/transcript", Header: h, Body: body})
}

// request calls requestTranscription as c with a fresh Idempotency-Key, or key when set.
func (w *world) request(c *harness.Client, fileID, key string) harness.Response {
	w.t.Helper()
	if key == "" {
		key = ids.New().String()
	}
	h := http.Header{}
	h.Set("Idempotency-Key", key)
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/files/" + fileID + "/transcribe", Header: h})
}

// agentVoice prepares mode agent with Lotse's voice_input and sends a voice message of Alex
// mentioning Lotse; it returns the file and the message.
func (w *world) agentVoice() (string, message) {
	w.t.Helper()
	w.voiceInput(true, true)
	f := w.m4a(4 * time.Second)
	return f, w.voiceTo(f)
}

// TestVoice_ModeAgent_SubmitTranscript (09 13, 09 4.7): the voice message is invoked at once;
// Lotse reads the file with its ETag and stores the transcript with that If-Match: 200 done,
// source agent, a new ETag, exactly one file.updated without text, Sam receives it, the search
// vector matches a word, and a correction with the new ETag replaces the text.
func TestVoice_ModeAgent_SubmitTranscript(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	f, m := w.agentVoice()
	run := w.runOf(m.ID)
	if !run.HeldUntil.IsZero() || w.invocations(run.ID) != 1 {
		t.Fatalf("mode agent: held until %v, %d invocations", run.HeldUntil, w.invocations(run.ID))
	}
	if got := delivered(w.poll(), run.ID); len(got) != 1 {
		t.Fatalf("%d invocations delivered", len(got))
	}
	status, etag := w.getFileETag(w.lotse, f)
	if status != http.StatusOK || etag == "" {
		t.Fatalf("getFile as Lotse: %d %q", status, etag)
	}
	events := len(w.fileUpdated(f))
	r := w.submit(w.lotse, f, etag, map[string]any{"text": "Please call Robin back tomorrow morning", "language": "en"})
	if r.Status != http.StatusOK {
		t.Fatalf("submitTranscript: %d %s", r.Status, r.Body)
	}
	noNull(t, "submitTranscript", r.Body)
	var v transcriptView
	r.JSON(t, &v)
	next := r.Header.Get("ETag")
	if v.Status != "done" || v.Source != "agent" || v.Language != "en" || v.Text == nil || *v.Text != "Please call Robin back tomorrow morning" ||
		next == "" || next == etag {
		t.Fatalf("submitTranscript answered %s with ETag %q", r.Body, next)
	}
	payloads := w.fileUpdated(f)[events:]
	if len(payloads) != 1 || strings.Contains(payloads[0], "Robin") || !strings.Contains(payloads[0], `"transcript_status": "done"`) &&
		!strings.Contains(payloads[0], `"transcript_status":"done"`) || !strings.Contains(payloads[0], "transcript_source") ||
		!strings.Contains(payloads[0], "transcript_language") {
		t.Fatalf("file.updated payloads %v", payloads)
	}
	if n := w.count(`SELECT count(*) FROM files WHERE id = $1 AND search_vector @@ plainto_tsquery('simple', 'tomorrow')`, f); n != 1 {
		t.Fatal("the transcript is not in files.search_vector")
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'file.updated' AND resource_id = $1 AND thread_id IS NOT NULL`, f); n == 0 {
		t.Fatal("file.updated is not in the scope of the message")
	}
	w.samSeesFileUpdated(f)
	c := w.submit(w.lotse, f, next, map[string]any{"text": "Please call Robin back on Monday"})
	if c.Status != http.StatusOK || w.file(f).Text != "Please call Robin back on Monday" || w.file(f).Language != "" {
		t.Fatalf("correction: %d %s, row %+v", c.Status, c.Body, w.file(f))
	}
}

// samSeesFileUpdated waits until Sam (read on Support/Allgemein) receives a file.updated of the
// file through listEvents.
func (w *world) samSeesFileUpdated(fileID string) {
	w.t.Helper()
	testkit := func() bool {
		r := w.sam.Do(w.t.Context(), harness.Request{Method: http.MethodGet, Path: "/events?after=0&limit=500"})
		return r.Status == http.StatusOK && strings.Contains(string(r.Body), fileID) && strings.Contains(string(r.Body), "file.updated")
	}
	if !testkit() {
		w.t.Fatal("Sam does not receive file.updated through listEvents")
	}
}

// TestSubmitTranscript_ForeignAgent404 (plan; AZ-P4): Texter gets the same 404 for Alex's voice
// file as for a random id; Alex gets 403; Lotse without If-Match 428, with a stale one 412 with
// current and ETag; the body rules; mode service 409; an image of the same thread 409.
func TestSubmitTranscript_ForeignAgent404(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	f, _ := w.agentVoice()
	_, etag := w.getFileETag(w.lotse, f)
	body := map[string]any{"text": "hello"}
	foreign := w.submit(w.texter, f, etag, body)
	unknown := w.submit(w.texter, ids.New().String(), etag, body)
	if foreign.Status != http.StatusNotFound || unknown.Status != http.StatusNotFound || !sameProblem(foreign.Body, unknown.Body) ||
		!sameHeaders(foreign.Header, unknown.Header) {
		t.Fatalf("foreign %d %s %v, unknown %d %s %v", foreign.Status, foreign.Body, foreign.Header, unknown.Status, unknown.Body, unknown.Header)
	}
	if r := w.submit(w.alex, f, etag, body); r.Status != http.StatusForbidden {
		t.Errorf("Alex: %d %s", r.Status, r.Body)
	}
	if r := w.submit(w.lotse, f, "", body); r.Status != http.StatusPreconditionRequired || problemOf(t, r).Code != "precondition_required" {
		t.Errorf("without If-Match: %d %s", r.Status, r.Body)
	}
	stale := w.submit(w.lotse, f, `"AAAAAAAAAAAAAAAAAAAAAA"`, body)
	if stale.Status != http.StatusPreconditionFailed || problemOf(t, stale).Code != "version_conflict" || stale.Header.Get("ETag") != etag ||
		!strings.Contains(string(stale.Body), `"current"`) {
		t.Errorf("stale If-Match: %d %s %v", stale.Status, stale.Body, stale.Header)
	}
	wantField(t, "too long", w.submit(w.lotse, f, etag, map[string]any{"text": strings.Repeat("a", maxContent+1)}), "/text", "too_long")
	wantField(t, "bad language", w.submit(w.lotse, f, etag, map[string]any{"text": "hi", "language": "english"}), "/language", "pattern_mismatch")
	if got := w.file(f); got.Status != "none" {
		t.Fatalf("a refused submit wrote %+v", got)
	}
	// An image attachment of the same thread is no voice file.
	png := w.ready(w.alex, w.allgemein, "screen.png", tinyPNG())
	m := w.send(w.alex, w.main, "", "@lotse screenshot", png)
	if m.Status != http.StatusCreated {
		t.Fatalf("image message: %d %s", m.Status, m.Body)
	}
	_, pe := w.getFileETag(w.lotse, png)
	if r := w.submit(w.lotse, png, pe, body); r.Status != http.StatusConflict || problemOf(t, r).Reason != "transcription_unavailable" {
		t.Errorf("an image: %d %s", r.Status, r.Body)
	}
}

// TestSubmitTranscript_ModeService409: in mode service submitTranscript is 409
// transcription_unavailable.
func TestSubmitTranscript_ModeService409(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{mode: "service"})
	w.voiceEnabled(true)
	f := w.m4a(3 * time.Second)
	m := w.voiceTo(f)
	_ = m
	_, etag := w.getFileETag(w.lotse, f)
	if r := w.submit(w.lotse, f, etag, map[string]any{"text": "x"}); r.Status != http.StatusConflict || problemOf(t, r).Reason != "transcription_unavailable" {
		t.Errorf("mode service: %d %s", r.Status, r.Body)
	}
}

// sameProblem compares two problem bodies without correlation_id and instance, which name the
// request and the path of the id.
func sameProblem(a, b []byte) bool {
	var pa, pb map[string]any
	if json.Unmarshal(a, &pa) != nil || json.Unmarshal(b, &pb) != nil {
		return false
	}
	for _, p := range []map[string]any{pa, pb} {
		delete(p, "correlation_id")
		delete(p, "instance")
	}
	return reflect.DeepEqual(pa, pb)
}

// sameHeaders compares two header sets without Correlation-Id and Date, and without the
// remaining count of RateLimit, which every request lowers.
func sameHeaders(a, b http.Header) bool {
	clean := func(h http.Header) http.Header {
		c := h.Clone()
		c.Del("Correlation-Id")
		c.Del("Date")
		c.Del("Ratelimit")
		return c
	}
	ca, cb := clean(a), clean(b)
	if len(ca) != len(cb) {
		return false
	}
	for k, v := range ca {
		if strings.Join(v, ",") != strings.Join(cb[k], ",") {
			return false
		}
	}
	return true
}

// tinyPNG is a 1 by 1 PNG.
func tinyPNG() []byte {
	return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 13, 'I', 'H', 'D', 'R', 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4,
		0x89, 0, 0, 0, 13, 'I', 'D', 'A', 'T', 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0xf0, 0x1f, 0, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99, 0x3d, 0x1d,
		0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82}
}

// TestGetTranscript_ETag (M3-05, design note 8 d): getTranscript and getFile send the same ETag;
// after submitTranscript both send the new one, which is accepted as If-Match.
func TestGetTranscript_ETag(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	f, _ := w.agentVoice()
	tr := w.lotse.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + f + "/transcript"})
	_, fe := w.getFileETag(w.lotse, f)
	if tr.Status != http.StatusOK || tr.Header.Get("ETag") == "" || tr.Header.Get("ETag") != fe {
		t.Fatalf("getTranscript %d ETag %q, getFile %q", tr.Status, tr.Header.Get("ETag"), fe)
	}
	r := w.submit(w.lotse, f, fe, map[string]any{"text": "first"})
	if r.Status != http.StatusOK {
		t.Fatalf("submit: %d %s", r.Status, r.Body)
	}
	tr2 := w.lotse.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + f + "/transcript"})
	_, fe2 := w.getFileETag(w.lotse, f)
	if tr2.Header.Get("ETag") != fe2 || fe2 != r.Header.Get("ETag") || fe2 == fe {
		t.Fatalf("after submit: transcript %q, file %q, submit %q", tr2.Header.Get("ETag"), fe2, r.Header.Get("ETag"))
	}
	if r := w.submit(w.lotse, f, fe2, map[string]any{"text": "second"}); r.Status != http.StatusOK {
		t.Fatalf("the new ETag as If-Match: %d %s", r.Status, r.Body)
	}
}

// TestRequestTranscription: in mode service a failed transcript goes back to pending with a new
// job; pending and done change nothing; the key replays; voice.enabled off 403; mode agent 409;
// Robin 404 like an unknown id; Sam (read only) 403; a pending file 409 file_not_ready.
func TestRequestTranscription(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{mode: "service"})
	w.voiceEnabled(true)
	f := w.m4a(3 * time.Second)
	w.voiceTo(f)
	if got := w.file(f); got.Status != "pending" || w.transcribeJobs(f) != 1 {
		t.Fatalf("after the send: %+v, %d jobs", got, w.transcribeJobs(f))
	}
	if r := w.request(w.alex, f, ""); r.Status != http.StatusAccepted || w.transcribeJobs(f) != 1 {
		t.Fatalf("pending: %d %s, %d jobs", r.Status, r.Body, w.transcribeJobs(f))
	}
	w.svc.Respond(http.StatusBadRequest, `{"error":"bad audio"}`)
	if err := w.runTranscribe(f, 1); err != nil {
		t.Fatal(err)
	}
	if got := w.file(f); got.Status != "failed" {
		t.Fatalf("after a 400: %+v", got)
	}
	key := ids.New().String()
	r := w.request(w.alex, f, key)
	if r.Status != http.StatusAccepted || w.file(f).Status != "pending" || w.transcribeJobs(f) != 2 {
		t.Fatalf("failed: %d %s, %+v, %d jobs", r.Status, r.Body, w.file(f), w.transcribeJobs(f))
	}
	noNull(t, "requestTranscription", r.Body)
	if again := w.request(w.alex, f, key); again.Status != http.StatusAccepted || !sameProblem(again.Body, r.Body) || w.transcribeJobs(f) != 2 {
		t.Fatalf("replay: %d %s, %d jobs", again.Status, again.Body, w.transcribeJobs(f))
	}
	unknown := w.request(w.robin, ids.New().String(), "")
	robin := w.request(w.robin, f, "")
	if robin.Status != http.StatusNotFound || unknown.Status != http.StatusNotFound || !sameProblem(robin.Body, unknown.Body) {
		t.Errorf("Robin %d %s, unknown %d %s", robin.Status, robin.Body, unknown.Status, unknown.Body)
	}
	if r := w.request(w.sam, f, ""); r.Status != http.StatusForbidden {
		t.Errorf("Sam: %d %s", r.Status, r.Body)
	}
	pending := w.upload(w.alex, w.allgemein, "later.m4a", []byte("not scanned yet"))
	if r := w.request(w.alex, pending, ""); r.Status != http.StatusConflict || problemOf(t, r).Code != "file_not_ready" {
		t.Errorf("a pending file: %d %s", r.Status, r.Body)
	}
	w.svc.Respond(http.StatusOK, `{"text":"done text","language":"en"}`)
	if err := w.runTranscribe(f, 1); err != nil {
		t.Fatal(err)
	}
	if r := w.request(w.alex, f, ""); r.Status != http.StatusAccepted || w.file(f).Status != "done" || w.transcribeJobs(f) != 2 {
		t.Fatalf("done: %d %s, %+v", r.Status, r.Body, w.file(f))
	}
	w.voiceEnabled(false)
	if r := w.request(w.alex, f, ""); r.Status != http.StatusForbidden || problemOf(t, r).Code != "feature_disabled" {
		t.Errorf("voice.enabled off: %d %s", r.Status, r.Body)
	}
}

// TestRequestTranscription_ModeAgent409: mode agent answers 409 transcription_unavailable.
func TestRequestTranscription_ModeAgent409(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	w.voiceEnabled(true)
	f := w.m4a(3 * time.Second)
	if r := w.request(w.alex, f, ""); r.Status != http.StatusConflict || problemOf(t, r).Reason != "transcription_unavailable" {
		t.Errorf("mode agent: %d %s", r.Status, r.Body)
	}
}
