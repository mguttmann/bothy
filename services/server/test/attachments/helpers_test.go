// SPDX-License-Identifier: Apache-2.0

// Package attachments_test holds the integration tests of attaching, reading, moving and deleting
// of brief M3-07 (section "Tests", Integration): the real server of internal/wire in role all
// through test/harness, PostgreSQL 18 from pgtest, the file system driver in a temporary root, the
// matrix world of 07 sections 10 and 11.1 (space Support with Allgemein and Eskalation, Alex with
// trigger and upload, Sam reading Allgemein through the group Leser, Robin without a grant on
// Support, the Owner), uploads through the real HTTP chain (createUpload, uploadPart,
// completeUpload) and the scan pass of M3-04 run by the worker of the server against a scripted
// clamd (clamdtest.Fake) on a Unix socket in a temporary directory. The veto subscribers of
// internal/veto stand in for a failing subscriber of the same unit of work.
package attachments_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	"github.com/pb33f/libopenapi/datamodel/high/base"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/attachments/internal/veto"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The limits of the test servers. maxAttachments is message.max_attachments of the test
// configuration, small so the limit test stays short.
const (
	partBytes      = 16384
	maxAttachments = 3
)

// env is one test server with its world, its scripted clamd and its veto subscribers.
type env struct {
	s     *harness.Server
	w     *harness.World
	clamd *clamdtest.Fake
	veto  *veto.Veto
	db    *pgxpool.Pool
	owner *harness.Client
	alex  *harness.Client
	sam   *harness.Client
	robin *harness.Client
	lotse *harness.Client
	// allg and esk are the channels Support/Allgemein and Support/Eskalation.
	allg, esk string
}

// servers bounds the harness servers of this package that run at the same time: every server
// opens its own pools on the one pgtest container, whose connection slots run out with more than
// a few parallel servers (SQLSTATE 53300).
var servers = make(chan struct{}, maxServers)

// maxServers is the number of servers of this package that run at the same time.
const maxServers = 3

// start runs a server in role all with the fake clamd, the veto subscribers and the matrix world.
func start(t *testing.T) *env {
	t.Helper()
	servers <- struct{}{}
	t.Cleanup(func() { <-servers })
	fake, err := clamdtest.NewFake()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fake.Close() })
	cfg := harness.RaisedRateLimits()
	cfg["file.upload_part_bytes"] = strconv.Itoa(partBytes)
	cfg["file.max_upload_bytes"] = "1048576"
	cfg["file.upload_max_open_sessions"] = "1000"
	cfg["file.scan.clamd_address"] = fake.Address()
	cfg["file.scan.health_interval"] = "1s"
	cfg["file.scan.timeout"] = "1s"
	cfg["file.scan.retry_backoff_cap"] = "2s"
	cfg["message.max_attachments"] = strconv.Itoa(maxAttachments)
	v := &veto.Veto{}
	s := harness.Start(t, harness.Options{Config: cfg, TxnTestSubscribers: v.Register})
	w := harness.BuildMatrixWorld(t, s)
	return &env{s: s, w: w, clamd: fake, veto: v, db: s.AdminDB(), owner: s.Owner, alex: w.People["Alex"], sam: w.People["Sam"],
		robin: w.People["Robin"], lotse: w.Agents["Lotse"], allg: w.Channels["Support/Allgemein"], esk: w.Channels["Support/Eskalation"]}
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// upload is one created upload with its parts sent.
type upload struct {
	FileID string
	parts  []map[string]any
}

// open creates an upload of content named name for channel as c and sends its parts, without
// completing it (the session stays open).
func (e *env) open(t *testing.T, c *harness.Client, channel, name string, content []byte) upload {
	t.Helper()
	r := c.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + channel + "/uploads",
		Body: map[string]any{"name": name, "size_bytes": len(content), "sha256": sha(content)}})
	if r.Status != http.StatusCreated {
		t.Fatalf("createUpload: %d %s", r.Status, r.Body)
	}
	var s struct {
		FileID string `json:"file_id"`
	}
	r.JSON(t, &s)
	u := upload{FileID: s.FileID}
	for n, lo := 1, 0; lo < len(content); n, lo = n+1, lo+partBytes {
		part := content[lo:min(lo+partBytes, len(content))]
		h := http.Header{}
		h.Set("Content-Type", "application/octet-stream")
		h.Set("Content-Digest", digest(part))
		pr := c.Do(t.Context(), harness.Request{Method: http.MethodPut, Path: "/uploads/" + s.FileID + "/parts/" + strconv.Itoa(n), Body: part, Header: h})
		if pr.Status != http.StatusOK {
			t.Fatalf("uploadPart %d: %d %s", n, pr.Status, pr.Body)
		}
		var out struct {
			ETag string `json:"etag"`
		}
		pr.JSON(t, &out)
		u.parts = append(u.parts, map[string]any{"part_number": n, "etag": out.ETag})
	}
	return u
}

// complete calls completeUpload as c.
func (e *env) complete(t *testing.T, c *harness.Client, u upload) {
	t.Helper()
	harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/uploads/" + u.FileID + "/complete", Body: map[string]any{"parts": u.parts}},
		http.StatusOK)
}

// file uploads content named name for channel as c through the whole chain and returns its id;
// the scan pass runs afterwards in the worker.
func (e *env) file(t *testing.T, c *harness.Client, channel, name string, content []byte) string {
	t.Helper()
	u := e.open(t, c, channel, name, content)
	e.complete(t, c, u)
	return u.FileID
}

// clean uploads content as c for channel and waits until the scan pass made it complete and clean.
func (e *env) clean(t *testing.T, c *harness.Client, channel, name string, content []byte) string {
	t.Helper()
	id := e.file(t, c, channel, name, content)
	if st := e.waitFinal(t, id); st != "complete/clean" {
		t.Fatalf("file %s ended %s", id, st)
	}
	return id
}

// stallScans makes clamd never answer, so every scan pass ends in its timeout and schedules a
// retry: an uploaded file stays upload_state and scan_status pending. answerScans makes clamd
// answer "stream: OK" again; the next retry of each pending file makes it clean.
func (e *env) stallScans()  { e.clamd.SetScript(clamdtest.Script{Stall: true}) }
func (e *env) answerScans() { e.clamd.SetScript(clamdtest.Script{}) }

// state is upload_state/scan_status of a file.
func (e *env) state(t *testing.T, id string) string {
	t.Helper()
	var up, scan string
	if err := e.db.QueryRow(t.Context(), `SELECT upload_state, scan_status FROM files WHERE id = $1`, id).Scan(&up, &scan); err != nil {
		t.Fatal(err)
	}
	return up + "/" + scan
}

// waitFinal waits until the file left upload_state pending and returns its state; the scan pass
// runs in the worker of the server, so the test waits for the condition within testkit.Patience.
func (e *env) waitFinal(t *testing.T, id string) string {
	t.Helper()
	var st string
	testkit.Until(t, "the scan of file "+id, func() bool {
		st = e.state(t, id)
		return !strings.HasPrefix(st, "pending/")
	})
	return st
}

// count runs a count query.
func (e *env) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(t.Context(), q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

// text runs a query of one text value.
func (e *env) text(t *testing.T, q string, args ...any) string {
	t.Helper()
	var s string
	if err := e.db.QueryRow(t.Context(), q, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return s
}

// exec runs a statement of the test setup.
func (e *env) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(t.Context(), q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// mainThread returns the main thread of the text channel ch (threads.kind main), read from the
// database because the world keeps only the threads it created.
func (e *env) mainThread(t *testing.T, ch string) string {
	t.Helper()
	return e.text(t, `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, ch)
}

// newThread creates a thread in the text channel ch on a new message of c in its main thread and
// returns the thread id.
func (e *env) newThread(t *testing.T, c *harness.Client, ch string) string {
	t.Helper()
	parent := msgOf(t, e.send(t, c, e.mainThread(t, ch), "Start", nil, nil), http.StatusCreated)
	var th struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/channels/" + ch + "/threads",
		Body: map[string]any{"parent_message_id": parent.ID}}, http.StatusCreated).JSON(t, &th)
	return th.Thread.ID
}

// body is a MessageCreate with a fresh client_msg_id, content and the files; extra members win.
func body(content string, fileIDs []string, extra map[string]any) map[string]any {
	b := map[string]any{"client_msg_id": ids.New().String(), "content_md": content}
	if fileIDs != nil {
		b["attachment_file_ids"] = fileIDs
	}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

// send posts a message with content and the files into thread as c (createMessage).
func (e *env) send(t *testing.T, c *harness.Client, thread, content string, fileIDs []string, extra map[string]any) harness.Response {
	t.Helper()
	return e.post(t, c, thread, body(content, fileIDs, extra))
}

// post sends createMessage with b into thread as c.
func (e *env) post(t *testing.T, c *harness.Client, thread string, b map[string]any) harness.Response {
	t.Helper()
	r, err := harness.DoWithStepUp(t.Context(), c, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages", Body: b})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// message is the part of a Message representation the tests read; Raw keeps every member.
type message struct {
	ID          string       `json:"id"`
	ThreadID    string       `json:"thread_id"`
	ChannelID   string       `json:"channel_id"`
	ThreadSeq   int64        `json:"thread_seq"`
	Attachments []attachment `json:"attachments"`
	Raw         map[string]any
}

// attachment is one member of Message.attachments with the raw File object.
type attachment struct {
	Position int            `json:"position"`
	File     map[string]any `json:"file"`
}

// msgOf decodes a Message answer of status want.
func msgOf(t *testing.T, r harness.Response, want int) message {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status %d %s, want %d", r.Status, r.Body, want)
	}
	return decodeMessage(t, r.Body)
}

// decodeMessage decodes one Message object.
func decodeMessage(t *testing.T, raw []byte) message {
	t.Helper()
	var m message
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	if err := json.Unmarshal(raw, &m.Raw); err != nil {
		t.Fatal(err)
	}
	if m.Raw["attachments"] == nil {
		t.Fatalf("message without attachments member: %s", raw)
	}
	return m
}

// fileIDs are the file ids of the attachments in list order, after checking that position counts
// from 1 in that order.
func fileIDs(t *testing.T, m message) []string {
	t.Helper()
	out := make([]string, 0, len(m.Attachments))
	for i, a := range m.Attachments {
		if a.Position != i+1 {
			t.Fatalf("attachment %d has position %d", i, a.Position)
		}
		id, _ := a.File["id"].(string)
		out = append(out, id)
	}
	return out
}

// attachedForm checks the attached form of each File: no scope_channel_id (D-M3-07) and no JSON
// null anywhere (ADR 0053).
func attachedForm(t *testing.T, m message) {
	t.Helper()
	for _, a := range m.Attachments {
		if _, ok := a.File["scope_channel_id"]; ok {
			t.Errorf("attached file %v carries scope_channel_id", a.File["id"])
		}
	}
	raw, err := json.Marshal(m.Raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "null") {
		t.Errorf("JSON null in %s", raw)
	}
}

// problem is a decoded problem body.
type problem struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Detail string `json:"detail"`
	Errors []struct {
		Pointer string `json:"pointer"`
		Code    string `json:"code"`
	} `json:"errors"`
}

// wantProblem checks status, code and reason of r and returns the problem.
func wantProblem(t *testing.T, r harness.Response, status int, code, reason string) problem {
	t.Helper()
	var p problem
	if r.Status != status || json.Unmarshal(r.Body, &p) != nil || p.Code != code || p.Reason != reason {
		t.Fatalf("got %d %s, want %d %s/%s", r.Status, r.Body, status, code, reason)
	}
	return p
}

// wantField checks a 400 validation_failed with a field error of code at pointer.
func wantField(t *testing.T, r harness.Response, pointer, code string) {
	t.Helper()
	p := wantProblem(t, r, http.StatusBadRequest, "validation_failed", "")
	for _, e := range p.Errors {
		if e.Pointer == pointer && e.Code == code {
			return
		}
	}
	t.Fatalf("errors %+v, want %s at %q", p.Errors, code, pointer)
}

// wantNotAttachable checks the one answer of a refused file (12 10 point 3): 409 conflict with
// reason file_not_attachable and detail pointer.
func wantNotAttachable(t *testing.T, r harness.Response, pointer string) {
	t.Helper()
	if p := wantProblem(t, r, http.StatusConflict, "conflict", "file_not_attachable"); p.Detail != pointer {
		t.Fatalf("detail %q, want %q", p.Detail, pointer)
	}
}

// shape is the answer without its per-request parts: status, the problem members other than
// correlation_id, instance and detail, and the headers other than Correlation-Id, Date,
// Content-Length and the rate limit headers. Two refusals with the same shape tell nothing apart.
func shape(t *testing.T, r harness.Response) string {
	t.Helper()
	var p map[string]any
	r.JSON(t, &p)
	for _, k := range []string{"correlation_id", "instance", "detail"} {
		delete(p, k)
	}
	h := r.Header.Clone()
	for _, k := range []string{"Correlation-Id", "Date", "Content-Length", "Ratelimit", "Ratelimit-Policy"} {
		h.Del(k)
	}
	pb, _ := json.Marshal(p)
	hb, _ := json.Marshal(h)
	return strconv.Itoa(r.Status) + " " + string(pb) + " " + string(hb)
}

// maxSeq is the head of the event log.
func (e *env) maxSeq(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := e.db.QueryRow(t.Context(), `SELECT coalesce(max(seq), 0) FROM event_log`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// event is one envelope of listEvents with its raw JSON.
type event struct {
	Type     string `json:"type"`
	Audience string `json:"audience"`
	Scope    struct {
		SpaceID   string `json:"space_id"`
		ChannelID string `json:"channel_id"`
		ThreadID  string `json:"thread_id"`
	} `json:"scope"`
	Resource struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"resource"`
	Payload json.RawMessage `json:"payload"`
	Raw     json.RawMessage
}

// events returns the events c receives through listEvents after seq.
func events(t *testing.T, c *harness.Client, after int64) []event {
	t.Helper()
	r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/events?limit=500&after=" + strconv.FormatInt(after, 10)})
	if r.Status != http.StatusOK {
		t.Fatalf("listEvents %d %s", r.Status, r.Body)
	}
	var page struct {
		Events []json.RawMessage `json:"events"`
	}
	r.JSON(t, &page)
	out := make([]event, 0, len(page.Events))
	for _, raw := range page.Events {
		var ev event
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		ev.Raw = raw
		out = append(out, ev)
	}
	return out
}

// eventsOf returns the events of type typ about resource id that c receives after seq.
func eventsOf(t *testing.T, c *harness.Client, after int64, typ, id string) []event {
	t.Helper()
	var out []event
	for _, ev := range events(t, c, after) {
		if ev.Type == typ && ev.Resource.ID == id {
			out = append(out, ev)
		}
	}
	return out
}

// schemas are the component schemas of docs/api/openapi.yaml, built once per test binary.
var schemas = sync.OnceValues(func() (map[string]*base.Schema, error) {
	doc, err := libopenapi.NewDocument(rest.Spec)
	if err != nil {
		return nil, err
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, err
	}
	out := map[string]*base.Schema{}
	for name, proxy := range model.Model.Components.Schemas.FromOldest() {
		out[name] = proxy.Schema()
	}
	return out, nil
})

// schemaMu serializes the validations over the shared schemas.
var schemaMu sync.Mutex

// validateSchema checks payload against the component schema name of docs/api/openapi.yaml and
// that it holds no JSON null (ADR 0053).
func validateSchema(t *testing.T, name string, payload []byte) {
	t.Helper()
	all, err := schemas()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := all[name]
	if !ok {
		t.Fatalf("no schema %s", name)
	}
	if strings.Contains(string(payload), "null") {
		t.Fatalf("JSON null in %s", payload)
	}
	// libopenapi renders the shared schema objects while it validates and writes into them, so
	// parallel tests validate one at a time (the race detector reports it otherwise).
	schemaMu.Lock()
	valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(s, string(payload))
	schemaMu.Unlock()
	if !valid {
		for _, p := range problems {
			t.Errorf("schema %s: %s %s", name, p.Message, p.Reason)
		}
		t.Fatalf("%s does not match %s", payload, name)
	}
}
