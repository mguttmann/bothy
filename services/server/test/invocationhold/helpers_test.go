// SPDX-License-Identifier: Apache-2.0

package invocationhold_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	runsjobs "github.com/OWNER/bothy/services/server/internal/runs/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/invocationhold/internal/finalveto"
)

// TestMain migrates the pgtest templates like bothy migrate up. Without a Docker-compatible
// runtime the tests fail; they never skip (plan P-09).
func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The values of the test configuration. holdMax is file.invocation_hold_max: small, and the fake
// clock only moves when a test moves it, so no time of the test run itself reaches it.
const (
	holdMax   = 2 * time.Minute
	partBytes = 16384
	// eventWait bounds the wait for the log tail of the server, which serves pollAgentEvents a
	// moment after the commit; no test waits for a window of the specification.
	eventWait = 30 * time.Second
)

// config are the keys of a hold test server with the clamd at address; extra entries win. The
// clients sign their DPoP proofs with the real clock while the server runs on the frozen fake
// clock, so auth.clock_leeway and auth.dpop_proof_max_age are wide (as in test/terminal).
func config(address string, extra map[string]string) map[string]string {
	cfg := harness.RaisedRateLimits()
	for k, v := range map[string]string{
		"auth.clock_leeway":             "48h",
		"auth.dpop_proof_max_age":       "48h",
		"file.upload_part_bytes":        strconv.Itoa(partBytes),
		"file.max_upload_bytes":         "1048576",
		"file.upload_max_open_sessions": "1000",
		"file.scan.clamd_address":       address,
		"file.scan.health_interval":     "1s",
		"file.scan.retry_backoff_cap":   "4s",
		"file.invocation_hold_max":      seconds(holdMax),
	} {
		cfg[k] = v
	}
	for k, v := range extra {
		cfg[k] = v
	}
	return cfg
}

// seconds formats d as a configuration value in whole seconds.
func seconds(d time.Duration) string { return strconv.FormatInt(int64(d/time.Second), 10) + "s" }

// switchClamd is the fake clamd of clamdtest behind a fixed socket path in a temporary directory:
// the server's file.scan.clamd_address names a symbolic link that points at the socket of a
// running fake, and stop removes link and fake, so the scanner is unreachable as a stopped clamd.
type switchClamd struct {
	t    *testing.T
	dir  string
	link string
	mu   sync.Mutex
	fake *clamdtest.Fake
}

// newSwitchClamd returns a stopped switchable clamd; the directory and a running fake are removed
// in t.Cleanup.
func newSwitchClamd(t *testing.T) *switchClamd {
	t.Helper()
	dir, err := os.MkdirTemp("", "hold")
	if err != nil {
		t.Fatal(err)
	}
	c := &switchClamd{t: t, dir: dir, link: filepath.Join(dir, "clamd.sock")}
	t.Cleanup(func() {
		c.stop()
		_ = os.RemoveAll(dir)
	})
	return c
}

// address is the unix:// address of the link.
func (c *switchClamd) address() string { return "unix://" + c.link }

// start runs a fake with script behind the link; a running fake gets the script.
func (c *switchClamd) start(script clamdtest.Script) {
	c.t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fake != nil {
		c.fake.SetScript(script)
		return
	}
	f, err := clamdtest.NewFake()
	if err != nil {
		c.t.Fatal(err)
	}
	f.SetScript(script)
	if err := os.Symlink(strings.TrimPrefix(f.Address(), "unix://"), c.link); err != nil {
		_ = f.Close()
		c.t.Fatal(err)
	}
	c.fake = f
}

// stop removes the link and closes the fake.
func (c *switchClamd) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fake == nil {
		return
	}
	_ = os.Remove(c.link)
	_ = c.fake.Close()
	c.fake = nil
}

// eicarReply is the answer of clamd to the EICAR test file.
const eicarReply = "stream: Eicar-Test-Signature FOUND"

// world is one hold test server with the matrix world of 07 11.1 on the fake clock: the real job
// handlers are kept for the test, River runs no-ops in their place, so a file stays pending, a
// held run stays held and a queued run stays queued until the test calls the handler.
type world struct {
	t     *testing.T
	s     *harness.Server
	w     *harness.World
	clk   *clock.Fake
	db    *pgxpool.Pool
	veto  *finalveto.Veto
	clamd *switchClamd

	mu     sync.Mutex
	scan   filesjobs.Runner
	holds  runsjobs.Releaser
	expiry runsjobs.Expirer

	alex, lotse, owner    *harness.Client
	allgemein, eskalation string
	main                  string
}

// worldOptions change a hold test world.
type worldOptions struct {
	// address is file.scan.clamd_address; empty means a switchable fake (world.clamd).
	address string
	// config are extra configuration keys.
	config map[string]string
}

// servers bounds the harness servers of this package that run at the same time: every server
// opens its own pools on the one pgtest container, whose connection slots run out with more than
// a few parallel servers (SQLSTATE 53300).
var servers = make(chan struct{}, maxServers)

// maxServers is the number of servers of this package that run at the same time.
const maxServers = 3

// newWorld starts the server and builds the world; the fake clamd starts stopped.
func newWorld(t *testing.T, o worldOptions) *world {
	t.Helper()
	servers <- struct{}{}
	t.Cleanup(func() { <-servers })
	w := &world{t: t, clk: clock.NewFake(time.Now().UTC().Truncate(time.Millisecond)), veto: &finalveto.Veto{}}
	address := o.address
	if address == "" {
		w.clamd = newSwitchClamd(t)
		address = w.clamd.address()
	}
	w.s = harness.Start(t, harness.Options{
		Config: config(address, o.config), Clock: w.clk, EgressDenyAll: true,
		JobModules: w.takeJobs,
		TxnTestSubscribers: func(reg *txn.Registry) error {
			return w.veto.Register(reg)
		},
	})
	w.db = w.s.AdminDB()
	w.w = harness.BuildMatrixWorld(t, w.s)
	w.owner, w.alex, w.lotse = w.s.Owner, w.w.People["Alex"], w.w.Agents["Lotse"]
	w.allgemein, w.eskalation = w.w.Channels["Support/Allgemein"], w.w.Channels["Support/Eskalation"]
	if err := w.db.QueryRow(t.Context(), `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, w.allgemein).Scan(&w.main); err != nil {
		t.Fatal(err)
	}
	w.behavior(w.allgemein)
	// The first poll creates Lotse's consumer at the head of the log (M2-07): every later event
	// reaches it.
	w.poll()
	return w
}

// takeJobs keeps the real handlers of files.scan, runs.invocation_hold_timeout and
// runs.queued_expiry and gives River no-ops in their place.
func (w *world) takeJobs(m *wire.JobModules) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.scan, w.holds, w.expiry = m.FilesScan, m.InvocationHolds, m.QueuedExpiry
	m.FilesScan, m.InvocationHolds, m.QueuedExpiry = noScan{}, noHold{}, noExpiry{}
}

// noScan, noHold and noExpiry are the River handlers of the test server: they do nothing, so only
// the test runs the scan pass, the hold timeout and the queued expiry.
type (
	noScan   struct{}
	noHold   struct{}
	noExpiry struct{}
)

func (noScan) Run(context.Context, ids.ID, int64) (filesapp.PassResult, error) {
	return filesapp.PassResult{Outcome: filesapp.OutcomeNone}, nil
}

func (noHold) InvocationHoldTimeout(context.Context, ids.ID) error { return nil }

func (noExpiry) ExpireQueued(context.Context, ids.ID) error { return nil }

// behavior sets the behaviour of channel to the defaults of 09 3.3 with auto_thread on
// (putChannelBehavior as the owner with If-Match).
func (w *world) behavior(channel string) {
	w.t.Helper()
	var cur struct {
		SettingsVersion int64 `json:"settings_version"`
	}
	harness.MustDo(w.t, w.owner, harness.Request{Method: http.MethodGet, Path: "/channels/" + channel + "/behavior"}, http.StatusOK).JSON(w.t, &cur)
	h := http.Header{}
	h.Set("If-Match", `"`+strconv.FormatInt(cur.SettingsVersion, 10)+`"`)
	harness.MustDo(w.t, w.owner, harness.Request{Method: http.MethodPut, Path: "/channels/" + channel + "/behavior", Header: h, Body: map[string]any{
		"require_mention": true, "thread_require_mention": false, "free_response": false, "auto_thread": true,
		"allow_mentioned_agents": false, "history_backfill_limit": 20, "prompt": "", "skills": []string{},
	}}, http.StatusOK)
}

// ctx is the context of a direct handler call: no actor, like a job of River.
func (w *world) ctx() context.Context { return context.Background() }

// --- files ---

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// upload uploads content as c for channel through createUpload, uploadPart and completeUpload
// (M3-03) and returns the file id; declared is the content the upload announces (its size and
// SHA-256), so a different declared content of the same length fails the scan with
// checksum_mismatch. The file is pending until the test runs the scan pass.
func (w *world) upload(c *harness.Client, channel, name string, content, declared []byte) string {
	w.t.Helper()
	r := c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + channel + "/uploads",
		Body: map[string]any{"name": name, "size_bytes": len(declared), "sha256": sha(declared), "declared_mime_type": "text/plain"}})
	if r.Status != http.StatusCreated {
		w.t.Fatalf("createUpload: %d %s", r.Status, r.Body)
	}
	var s struct {
		FileID string `json:"file_id"`
	}
	r.JSON(w.t, &s)
	var parts []map[string]any
	for n, lo := 1, 0; lo < len(content); n, lo = n+1, lo+partBytes {
		part := content[lo:min(lo+partBytes, len(content))]
		h := http.Header{}
		h.Set("Content-Type", "application/octet-stream")
		h.Set("Content-Digest", digest(part))
		pr := c.Do(w.t.Context(), harness.Request{Method: http.MethodPut, Path: "/uploads/" + s.FileID + "/parts/" + strconv.Itoa(n), Body: part, Header: h})
		if pr.Status != http.StatusOK {
			w.t.Fatalf("uploadPart %d: %d %s", n, pr.Status, pr.Body)
		}
		var out struct {
			ETag string `json:"etag"`
		}
		pr.JSON(w.t, &out)
		parts = append(parts, map[string]any{"part_number": n, "etag": out.ETag})
	}
	cr := c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/uploads/" + s.FileID + "/complete", Body: map[string]any{"parts": parts}})
	if cr.Status != http.StatusOK {
		w.t.Fatalf("completeUpload: %d %s", cr.Status, cr.Body)
	}
	return s.FileID
}

// file uploads a small text file of Alex for Support/Allgemein.
func (w *world) file(name string) string {
	w.t.Helper()
	content := []byte("attachment " + name + " for the support agent")
	return w.upload(w.alex, w.allgemein, name, content, content)
}

// scanFile runs the handler of files.scan once for the file with the next round, as River would
// run the due job (M3-04): the pass of the real server, here on the test's goroutine.
func (w *world) scanFile(fileID string) error {
	w.mu.Lock()
	pass := w.scan
	w.mu.Unlock()
	if pass == nil {
		return errors.New("the scan pass of the server is not built")
	}
	round := w.count(`SELECT coalesce(max((args->>'round')::bigint), 0) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1`, fileID)
	return filesjobs.Handler{Pass: pass}.Work(w.ctx(), jobs.Job[filesjobs.Scan]{Attempt: 1, Args: filesjobs.Scan{FileID: ids.MustParse(fileID), Round: round}})
}

// mustScan is scanFile that fails the test on an error.
func (w *world) mustScan(fileID string) {
	w.t.Helper()
	if err := w.scanFile(fileID); err != nil {
		w.t.Fatalf("files.scan %s: %v", fileID, err)
	}
}

// fileState reads upload_state and scan_status of a file.
func (w *world) fileState(fileID string) (upload, scan string) {
	w.t.Helper()
	if err := w.db.QueryRow(w.t.Context(), `SELECT upload_state, scan_status FROM files WHERE id = $1`, fileID).Scan(&upload, &scan); err != nil {
		w.t.Fatal(err)
	}
	return upload, scan
}

// --- messages ---

// send posts a message as c into thread with the attachments and returns the status and body.
func (w *world) send(c *harness.Client, thread, text string, fileIDs ...string) harness.Response {
	w.t.Helper()
	body := map[string]any{"client_msg_id": ids.New().String(), "content_md": text}
	if len(fileIDs) > 0 {
		body["attachment_file_ids"] = fileIDs
	}
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages", Body: body})
}

// message is the part of a Message body the tests read.
type message struct {
	ID           string `json:"id"`
	ThreadID     string `json:"thread_id"`
	TriggerState string `json:"trigger_state"`
}

// mention sends "@lotse ..." as Alex into the main thread of Support/Allgemein with the
// attachments and requires 201; auto_thread opens Lotse's run in a new thread.
func (w *world) mention(fileIDs ...string) message {
	w.t.Helper()
	r := w.send(w.alex, w.main, "@lotse please look at the attached export", fileIDs...)
	if r.Status != http.StatusCreated {
		w.t.Fatalf("createMessage: %d %s", r.Status, r.Body)
	}
	var m message
	r.JSON(w.t, &m)
	return m
}

// triggerState reads trigger_state of a message.
func (w *world) triggerState(messageID string) string {
	w.t.Helper()
	var s string
	if err := w.db.QueryRow(w.t.Context(), `SELECT coalesce(trigger_state, '') FROM messages WHERE id = $1`, messageID).Scan(&s); err != nil {
		w.t.Fatal(err)
	}
	return s
}

// --- runs ---

// runRow is the part of a runs row the tests read; HeldUntil is zero for NULL.
type runRow struct {
	ID, Thread, State, Reason string
	Version                   int64
	HeldUntil                 time.Time
}

// runOf reads the run whose triggering message is messageID and fails without exactly one.
func (w *world) runOf(messageID string) runRow {
	w.t.Helper()
	var r runRow
	var held *time.Time
	err := w.db.QueryRow(w.t.Context(), `SELECT id::text, thread_id::text, state, coalesce(state_reason, ''), version, invocation_held_until
		FROM runs WHERE triggering_message_id = $1`, messageID).Scan(&r.ID, &r.Thread, &r.State, &r.Reason, &r.Version, &held)
	if err != nil {
		w.t.Fatalf("the run of message %s: %v", messageID, err)
	}
	if held != nil {
		r.HeldUntil = held.UTC()
	}
	return r
}

// run reads the run id.
func (w *world) run(id string) runRow {
	w.t.Helper()
	var r runRow
	var held *time.Time
	if err := w.db.QueryRow(w.t.Context(), `SELECT id::text, thread_id::text, state, coalesce(state_reason, ''), version, invocation_held_until
		FROM runs WHERE id = $1`, id).Scan(&r.ID, &r.Thread, &r.State, &r.Reason, &r.Version, &held); err != nil {
		w.t.Fatal(err)
	}
	if held != nil {
		r.HeldUntil = held.UTC()
	}
	return r
}

// holdJobs returns the scheduled times of the runs.invocation_hold_timeout jobs of the run.
func (w *world) holdJobs(runID string) []time.Time {
	w.t.Helper()
	rows, err := w.db.Query(w.t.Context(), `SELECT scheduled_at FROM jobs.river_job WHERE kind = 'runs.invocation_hold_timeout'
		AND args->>'run_id' = $1 ORDER BY id`, runID)
	if err != nil {
		w.t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (time.Time, error) {
		var at time.Time
		err := r.Scan(&at)
		return at.UTC(), err
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return out
}

// holdTimeout runs the handler of runs.invocation_hold_timeout for the run once (02 12.3).
func (w *world) holdTimeout(runID string) {
	w.t.Helper()
	w.mu.Lock()
	r := w.holds
	w.mu.Unlock()
	if err := (runsjobs.HoldHandler{Releaser: r}).Work(w.ctx(), jobs.Job[runsjobs.InvocationHoldTimeout]{Attempt: 1,
		Args: runsjobs.InvocationHoldTimeout{RunID: ids.MustParse(runID)}}); err != nil {
		w.t.Fatalf("runs.invocation_hold_timeout: %v", err)
	}
}

// queuedExpiry runs the use case of runs.queued_expiry for the run once (M2-08).
func (w *world) queuedExpiry(runID string) {
	w.t.Helper()
	w.mu.Lock()
	e := w.expiry
	w.mu.Unlock()
	if err := e.ExpireQueued(w.ctx(), ids.MustParse(runID)); err != nil {
		w.t.Fatalf("runs.queued_expiry: %v", err)
	}
}

// --- events ---

// invocation is one stored agent.invocation of a run.
type invocation struct {
	Seq                               int64
	ActorKind, ActorID, CorrelationID string
	SpaceID, ChannelID, ThreadID      string
	Audience, ReceiveAction, Resource string
	Payload                           string
}

// invocations reads the stored agent.invocation events of the run in log order.
func (w *world) invocations(runID string) []invocation {
	w.t.Helper()
	rows, err := w.db.Query(w.t.Context(), `SELECT seq, coalesce(actor_kind, ''), coalesce(actor_principal_id::text, ''),
		coalesce(correlation_id::text, ''), coalesce(space_id::text, ''), coalesce(channel_id::text, ''), coalesce(thread_id::text, ''),
		coalesce(audience_principal_id::text, ''), receive_action, resource_kind, payload::text
		FROM event_log WHERE type = 'agent.invocation' AND resource_id = $1 ORDER BY seq`, runID)
	if err != nil {
		w.t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (invocation, error) {
		var i invocation
		err := r.Scan(&i.Seq, &i.ActorKind, &i.ActorID, &i.CorrelationID, &i.SpaceID, &i.ChannelID, &i.ThreadID, &i.Audience,
			&i.ReceiveAction, &i.Resource, &i.Payload)
		return i, err
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return out
}

// count runs a count query.
func (w *world) count(q string, args ...any) int64 {
	w.t.Helper()
	var n int64
	if err := w.db.QueryRow(w.t.Context(), q, args...).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

// maxSeq is the newest seq of the event log.
func (w *world) maxSeq() int64 { return w.count(`SELECT coalesce(max(seq), 0) FROM event_log`) }

// feedEvent is the part of an event of pollAgentEvents the tests read.
type feedEvent struct {
	Seq      int64  `json:"seq"`
	Type     string `json:"type"`
	Resource struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"resource"`
	Payload json.RawMessage `json:"payload"`
	// InvocationToken is the delivery field of the envelope (04 2.2, R-SY-07).
	InvocationToken string `json:"invocation_token"`
}

// poll reads Lotse's consumer default through pollAgentEvents (M2-07) until it reached the newest
// event of the log, acknowledging every page, and returns the events. The log tail serves the
// poll a moment after the commit, so it waits up to eventWait for the head to arrive.
func (w *world) poll() []feedEvent {
	w.t.Helper()
	target := w.maxSeq()
	deadline := time.Now().Add(eventWait)
	var out []feedEvent
	for {
		r := w.lotse.Do(w.t.Context(), harness.Request{Method: http.MethodGet, Path: "/agents/me/consumers/default/events"})
		if r.Status != http.StatusOK {
			w.t.Fatalf("pollAgentEvents: %d %s", r.Status, r.Body)
		}
		var page struct {
			Events     []feedEvent `json:"events"`
			NextCursor int64       `json:"next_cursor"`
			HeadSeq    int64       `json:"head_seq"`
		}
		r.JSON(w.t, &page)
		out = append(out, page.Events...)
		if len(page.Events) > 0 {
			harness.MustDo(w.t, w.lotse, harness.Request{Method: http.MethodPost, Path: "/agents/me/consumers/default/ack",
				Body: map[string]any{"seq": page.NextCursor}}, http.StatusOK)
		}
		if page.HeadSeq >= target && page.NextCursor >= page.HeadSeq {
			return out
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("pollAgentEvents did not reach seq %d within %v (head %d, cursor %d)", target, eventWait, page.HeadSeq, page.NextCursor)
		}
		if len(page.Events) == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// delivered returns the agent.invocation events of the run among evs.
func delivered(evs []feedEvent, runID string) []feedEvent {
	var out []feedEvent
	for _, e := range evs {
		if e.Type == "agent.invocation" && e.Resource.ID == runID {
			out = append(out, e)
		}
	}
	return out
}

// token returns invocation_token of a delivered event, "" without one; the payload never carries
// it.
func token(e feedEvent) string {
	if strings.Contains(string(e.Payload), "invocation_token") {
		return ""
	}
	return e.InvocationToken
}

// --- contract ---

// validInvocationPayload fails unless payload is valid against AgentInvocationPayload of
// openapi.yaml and carries no JSON null and no token (the token is a delivery field, R-SY-07).
func validInvocationPayload(t *testing.T, payload string) {
	t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	if err != nil {
		t.Fatal(err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatal(err)
	}
	proxy, ok := model.Model.Components.Schemas.Get("AgentInvocationPayload")
	if !ok {
		t.Fatal("no schema AgentInvocationPayload")
	}
	if strings.Contains(payload, "null") || strings.Contains(payload, "invocation_token") {
		t.Fatalf("payload %s", payload)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), payload); !valid {
		t.Fatalf("%s does not match AgentInvocationPayload: %v", payload, problems)
	}
}

// threadScope reads space, channel and thread of a thread now.
func (w *world) threadScope(threadID string) (space, channel string) {
	w.t.Helper()
	if err := w.db.QueryRow(w.t.Context(), `SELECT c.space_id::text, t.channel_id::text FROM threads t JOIN channels c ON c.id = t.channel_id
		WHERE t.id = $1`, threadID).Scan(&space, &channel); err != nil {
		w.t.Fatal(err)
	}
	return space, channel
}

// getFile reads the file as c and returns status and scan_status.
func (w *world) getFile(c *harness.Client, fileID string) (int, string) {
	w.t.Helper()
	r := c.Do(w.t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + fileID})
	var f struct {
		ScanStatus string `json:"scan_status"`
	}
	if r.Status == http.StatusOK {
		r.JSON(w.t, &f)
	}
	return r.Status, f.ScanStatus
}

// problemCode returns the member code of a problem body, or "".
func problemCode(body []byte) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &p)
	return p.Code
}
