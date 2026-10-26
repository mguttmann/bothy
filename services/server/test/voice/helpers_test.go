// SPDX-License-Identifier: Apache-2.0

package voice_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/filestest"
	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/transcription/transcriptiontest"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	runsjobs "github.com/OWNER/bothy/services/server/internal/runs/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestMain migrates the pgtest templates like bothy migrate up. Without a Docker-compatible
// runtime the tests fail; they never skip (plan P-09).
func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The values of the test configuration. The fake clock only moves when a test moves it, so no
// time of the test run itself reaches a window.
const (
	holdMax       = 2 * time.Minute
	transcriptMax = 5 * time.Minute
	maxDuration   = 60 * time.Second
	maxAttempts   = 3
	partBytes     = 16384
	maxContent    = 4096
)

// seconds formats d as a configuration value in whole seconds.
func seconds(d time.Duration) string { return strconv.FormatInt(int64(d/time.Second), 10) + "s" }

// world is one voice test server with the matrix world of 07 11.1 on the fake clock: the real
// handlers of files.scan, files.thumbnail, files.transcribe, runs.invocation_hold_timeout and
// runs.queued_expiry are kept for the test, River runs no-ops in their place, so every step runs
// only when the test calls it. River also runs no-ops for files.purge_objects and
// files.cleanup_pending, so the row of a deleted file stays for the test to read (PL/54).
type world struct {
	t   *testing.T
	s   *harness.Server
	w   *harness.World
	clk *clock.Fake
	db  *pgxpool.Pool
	svc *transcriptiontest.Server

	mu         sync.Mutex
	scan       filesjobs.Runner
	images     filesjobs.ImageRunner
	transcribe filesjobs.TranscribeRunner
	holds      runsjobs.Releaser

	alex, sam, robin, lotse, texter, owner *harness.Client
	allgemein, eskalation                  string
	main                                   string
}

// worldOptions change a voice test world.
type worldOptions struct {
	// mode is voice.transcription_mode; "" means agent.
	mode string
	// config are extra configuration keys.
	config map[string]string
	// scan keeps the real virus scan with the fake clamd at this address; "" sets
	// file.scan.mode off.
	clamd string
}

// servers bounds the harness servers of this package that run at the same time (the connection
// slots of the one pgtest container, as in test/invocationhold).
var servers = make(chan struct{}, maxServers)

// maxServers is the number of servers of this package that run at the same time.
const maxServers = 3

// newWorld starts the server with the fake transcription service and builds the world.
func newWorld(t *testing.T, o worldOptions) *world {
	t.Helper()
	servers <- struct{}{}
	t.Cleanup(func() { <-servers })
	w := &world{t: t, clk: clock.NewFake(time.Now().UTC().Truncate(time.Millisecond))}
	w.svc = transcriptiontest.New(t)
	mode := o.mode
	if mode == "" {
		mode = "agent"
	}
	cfg := harness.RaisedRateLimits()
	for k, v := range map[string]string{
		"auth.clock_leeway":                "48h",
		"auth.dpop_proof_max_age":          "48h",
		"file.upload_part_bytes":           strconv.Itoa(partBytes),
		"file.max_upload_bytes":            "1048576",
		"file.upload_max_open_sessions":    "1000",
		"file.scan.mode":                   "off",
		"file.invocation_hold_max":         seconds(holdMax),
		"voice.transcript_hold_max":        seconds(transcriptMax),
		"voice.max_duration":               seconds(maxDuration),
		"voice.transcription_mode":         mode,
		"voice.transcription_url":          w.svc.URL(),
		"voice.transcription_max_attempts": strconv.Itoa(maxAttempts),
		"egress.allowed_hosts":             w.svc.Rule(),
		"message.max_content_bytes":        strconv.Itoa(maxContent),
	} {
		cfg[k] = v
	}
	if o.clamd != "" {
		cfg["file.scan.mode"] = "required"
		cfg["file.scan.clamd_address"] = o.clamd
		cfg["file.scan.health_interval"] = "1s"
		cfg["file.scan.retry_backoff_cap"] = "4s"
	}
	for k, v := range o.config {
		cfg[k] = v
	}
	w.s = harness.Start(t, harness.Options{Config: cfg, Clock: w.clk, JobModules: w.takeJobs})
	w.db = w.s.AdminDB()
	w.w = harness.BuildMatrixWorld(t, w.s)
	w.owner, w.alex, w.sam, w.robin = w.s.Owner, w.w.People["Alex"], w.w.People["Sam"], w.w.People["Robin"]
	w.lotse, w.texter = w.w.Agents["Lotse"], w.w.Agents["Texter"]
	w.allgemein, w.eskalation = w.w.Channels["Support/Allgemein"], w.w.Channels["Support/Eskalation"]
	if err := w.db.QueryRow(t.Context(), `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, w.allgemein).Scan(&w.main); err != nil {
		t.Fatal(err)
	}
	w.behavior(w.allgemein)
	// The first poll creates Lotse's consumer at the head of the log (M2-07).
	w.poll()
	return w
}

// takeJobs keeps the real handlers and gives River no-ops in their place.
func (w *world) takeJobs(m *wire.JobModules) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.scan, w.images, w.transcribe, w.holds = m.FilesScan, m.FilesImages, m.FilesTranscribe, m.InvocationHolds
	m.FilesScan, m.FilesImages, m.FilesTranscribe, m.InvocationHolds = noScan{}, noImages{}, noTranscribe{}, noHold{}
	m.FilesPurge, m.FilesCleanup = noPurge{}, noCleanup{}
}

// The River handlers of the test server: they do nothing, so only the test runs the jobs.
type (
	noScan       struct{}
	noImages     struct{}
	noTranscribe struct{}
	noHold       struct{}
	noPurge      struct{}
	noCleanup    struct{}
)

func (noScan) Run(context.Context, ids.ID, int64) (filesapp.PassResult, error) {
	return filesapp.PassResult{Outcome: filesapp.OutcomeNone}, nil
}
func (noImages) ProcessImage(context.Context, ids.ID) error        { return nil }
func (noTranscribe) Run(context.Context, ids.ID, int) error        { return nil }
func (noHold) InvocationHoldTimeout(context.Context, ids.ID) error { return nil }
func (noPurge) Run(context.Context, ids.ID) error                  { return nil }
func (noCleanup) Run(context.Context) error                        { return nil }

// behavior sets the behaviour of channel to the defaults of 09 3.3 with auto_thread on.
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

// --- switches ---

// voiceEnabled switches voice.enabled for everybody through putFeatureFlag as the Owner (M1-47).
func (w *world) voiceEnabled(on bool) {
	w.t.Helper()
	r, err := harness.DoWithStepUp(w.t.Context(), w.owner, harness.Request{Method: http.MethodPut, Path: "/feature-flags/voice.enabled",
		Body: map[string]any{"enabled": on, "reason": "voice messages for the support team"}})
	if err != nil || r.Status != http.StatusOK {
		w.t.Fatalf("putFeatureFlag: %v %d %s", err, r.Status, r.Body)
	}
}

// voiceInput gives Lotse the effective feature voice_input (09 2.2): the Owner sets
// supports_voice and Lotse's runtime reports capabilities.voice_input through its heartbeat.
func (w *world) voiceInput(permitted, reported bool) {
	w.t.Helper()
	var version int64
	if err := w.db.QueryRow(w.t.Context(), `SELECT version FROM agents WHERE principal_id = $1`, w.lotse.PrincipalID).Scan(&version); err != nil {
		w.t.Fatal(err)
	}
	h := http.Header{}
	h.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	harness.MustDo(w.t, w.owner, harness.Request{Method: http.MethodPatch, Path: "/agents/" + w.lotse.PrincipalID, Header: h,
		Body: map[string]any{"operator_config": map[string]any{"supports_voice": permitted}}}, http.StatusOK)
	harness.MustDo(w.t, w.lotse, harness.Request{Method: http.MethodPost, Path: "/agents/me/heartbeat",
		Body: map[string]any{"state": "online", "details": map[string]any{"capabilities": map[string]any{"voice_input": reported}}}}, http.StatusOK)
	testkit.Until(w.t, "the heartbeat report of voice_input persisted", func() bool {
		var got *bool
		if err := w.db.QueryRow(w.t.Context(), `SELECT (reported_details->'capabilities'->>'voice_input')::boolean FROM agents WHERE principal_id = $1`,
			w.lotse.PrincipalID).Scan(&got); err != nil {
			w.t.Fatal(err)
		}
		return got != nil && *got == reported
	})
}

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
// (M3-03) and returns the file id. The file is pending until the test runs the scan.
func (w *world) upload(c *harness.Client, channel, name string, content []byte) string {
	w.t.Helper()
	r := c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + channel + "/uploads",
		Body: map[string]any{"name": name, "size_bytes": len(content), "sha256": sha(content)}})
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

// ready uploads content as c into channel and runs files.scan and files.thumbnail, as River
// would, so the file is final (12 3.4, M3-09 design note 5).
func (w *world) ready(c *harness.Client, channel, name string, content []byte) string {
	w.t.Helper()
	id := w.upload(c, channel, name, content)
	w.mustScan(id)
	w.mustThumbnail(id)
	return id
}

// m4a is a ready voice recording of Alex of d in Support/Allgemein.
func (w *world) m4a(d time.Duration) string {
	w.t.Helper()
	ms := uint32(d.Milliseconds()) //nolint:gosec // G115: test durations are minutes at most.
	return w.ready(w.alex, w.allgemein, "note.m4a", filestest.M4A(1000, ms, filestest.AudioOptions{}))
}

// scanFile runs files.scan once for the file with the next round.
func (w *world) scanFile(fileID string) error {
	w.mu.Lock()
	pass := w.scan
	w.mu.Unlock()
	if pass == nil {
		return errors.New("the scan pass of the server is not built")
	}
	round := w.count(`SELECT coalesce(max((args->>'round')::bigint), 0) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1`, fileID)
	return filesjobs.Handler{Pass: pass}.Work(context.Background(), jobs.Job[filesjobs.Scan]{Attempt: 1, Args: filesjobs.Scan{FileID: ids.MustParse(fileID), Round: round}})
}

// mustScan is scanFile that fails the test on an error.
func (w *world) mustScan(fileID string) {
	w.t.Helper()
	if err := w.scanFile(fileID); err != nil {
		w.t.Fatalf("files.scan %s: %v", fileID, err)
	}
}

// mustThumbnail runs files.thumbnail once for the file.
func (w *world) mustThumbnail(fileID string) {
	w.t.Helper()
	w.mu.Lock()
	img := w.images
	w.mu.Unlock()
	if err := (filesjobs.ThumbnailHandler{Images: img}).Work(context.Background(),
		jobs.Job[filesjobs.ThumbnailArgs]{Attempt: 1, Args: filesjobs.ThumbnailArgs{FileID: ids.MustParse(fileID)}}); err != nil {
		w.t.Fatalf("files.thumbnail %s: %v", fileID, err)
	}
}

// runTranscribe runs files.transcribe once for the file as attempt.
func (w *world) runTranscribe(fileID string, attempt int) error {
	w.mu.Lock()
	tr := w.transcribe
	w.mu.Unlock()
	return (filesjobs.TranscribeHandler{Transcription: tr}).Work(context.Background(),
		jobs.Job[filesjobs.TranscribeArgs]{Attempt: attempt, Args: filesjobs.TranscribeArgs{FileID: ids.MustParse(fileID)}})
}

// fileRow is the part of a files row the tests read; the absent columns are "".
type fileRow struct {
	Upload, Scan, Status, Text, Language, Source string
	DurationMS                                   int64
	Deleted                                      bool
}

// file reads the file row.
func (w *world) file(fileID string) fileRow {
	w.t.Helper()
	var f fileRow
	if err := w.db.QueryRow(w.t.Context(), `SELECT upload_state, scan_status, transcript_status, coalesce(transcript_text, ''),
		coalesce(transcript_language, ''), coalesce(transcript_source, ''), coalesce(duration_ms, 0), deleted_at IS NOT NULL
		FROM files WHERE id = $1`, fileID).Scan(&f.Upload, &f.Scan, &f.Status, &f.Text, &f.Language, &f.Source, &f.DurationMS, &f.Deleted); err != nil {
		w.t.Fatal(err)
	}
	return f
}

// transcribeJobs counts the jobs files.transcribe of the file.
func (w *world) transcribeJobs(fileID string) int64 {
	return w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'files.transcribe' AND args->>'file_id' = $1`, fileID)
}

// fileUpdated returns the payloads of the stored file.updated events of the file in log order.
func (w *world) fileUpdated(fileID string) []string {
	w.t.Helper()
	rows, err := w.db.Query(w.t.Context(), `SELECT payload::text FROM event_log WHERE type = 'file.updated' AND resource_id = $1 ORDER BY seq`, fileID)
	if err != nil {
		w.t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		w.t.Fatal(err)
	}
	return out
}

// --- messages ---

// send posts a message as c into thread with the kind and attachments and returns the response.
func (w *world) send(c *harness.Client, thread, kind, text string, fileIDs ...string) harness.Response {
	w.t.Helper()
	body := map[string]any{"client_msg_id": ids.New().String(), "content_md": text}
	if kind != "" {
		body["kind"] = kind
	}
	if fileIDs != nil {
		body["attachment_file_ids"] = fileIDs
	}
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages", Body: body})
}

// message is the part of a Message body the tests read.
type message struct {
	ID           string `json:"id"`
	ThreadID     string `json:"thread_id"`
	Kind         string `json:"kind"`
	TriggerState string `json:"trigger_state"`
	Attachments  []struct {
		File struct {
			ID               string `json:"id"`
			DurationMS       *int64 `json:"duration_ms"`
			TranscriptStatus string `json:"transcript_status"`
		} `json:"file"`
	} `json:"attachments"`
}

// voiceTo sends the voice file as Alex into the main thread of Support/Allgemein mentioning Lotse
// and requires 201; auto_thread opens Lotse's run in a new thread.
func (w *world) voiceTo(fileID string) message {
	w.t.Helper()
	r := w.send(w.alex, w.main, "voice", "@lotse", fileID)
	if r.Status != http.StatusCreated {
		w.t.Fatalf("createMessage voice: %d %s", r.Status, r.Body)
	}
	var m message
	r.JSON(w.t, &m)
	return m
}

// triggerState reads trigger_state and trigger_reason of a message.
func (w *world) triggerState(messageID string) (state, reason string) {
	w.t.Helper()
	if err := w.db.QueryRow(w.t.Context(), `SELECT coalesce(trigger_state, ''), coalesce(trigger_state_reason, '') FROM messages WHERE id = $1`,
		messageID).Scan(&state, &reason); err != nil {
		w.t.Fatal(err)
	}
	return state, reason
}

// --- runs ---

// runRow is the part of a runs row the tests read; HeldUntil is zero for NULL.
type runRow struct {
	ID, Thread, State string
	HeldUntil         time.Time
}

// runOf reads the run whose triggering message is messageID and fails without exactly one.
func (w *world) runOf(messageID string) runRow {
	w.t.Helper()
	var r runRow
	var held *time.Time
	if err := w.db.QueryRow(w.t.Context(), `SELECT id::text, thread_id::text, state, invocation_held_until FROM runs
		WHERE triggering_message_id = $1`, messageID).Scan(&r.ID, &r.Thread, &r.State, &held); err != nil {
		w.t.Fatalf("the run of message %s: %v", messageID, err)
	}
	if held != nil {
		r.HeldUntil = held.UTC()
	}
	return r
}

// holdTimeout runs the handler of runs.invocation_hold_timeout for the run once.
func (w *world) holdTimeout(runID string) {
	w.t.Helper()
	w.mu.Lock()
	r := w.holds
	w.mu.Unlock()
	if err := (runsjobs.HoldHandler{Releaser: r}).Work(context.Background(), jobs.Job[runsjobs.InvocationHoldTimeout]{Attempt: 1,
		Args: runsjobs.InvocationHoldTimeout{RunID: ids.MustParse(runID)}}); err != nil {
		w.t.Fatalf("runs.invocation_hold_timeout: %v", err)
	}
}

// invocations counts the stored agent.invocation events of the run.
func (w *world) invocations(runID string) int64 {
	return w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation' AND resource_id = $1`, runID)
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
	Payload         json.RawMessage `json:"payload"`
	InvocationToken string          `json:"invocation_token"`
}

// poll reads Lotse's consumer default until it reached the newest event of the log,
// acknowledging every page, and returns the events.
func (w *world) poll() []feedEvent {
	w.t.Helper()
	target := w.maxSeq()
	var out []feedEvent
	testkit.Until(w.t, "pollAgentEvents reaching the head of the log", func() bool {
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
		return page.HeadSeq >= target && page.NextCursor >= page.HeadSeq
	})
	return out
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

// problem is the part of a problem body the tests read.
type problem struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Errors []struct {
		Pointer string `json:"pointer"`
		Code    string `json:"code"`
	} `json:"errors"`
}

// problemOf decodes the problem body of r.
func problemOf(t *testing.T, r harness.Response) problem {
	t.Helper()
	var p problem
	_ = json.Unmarshal(r.Body, &p)
	return p
}

// wantField fails unless r is 400 validation_failed with exactly the field error code at pointer.
func wantField(t *testing.T, what string, r harness.Response, pointer, code string) {
	t.Helper()
	p := problemOf(t, r)
	if r.Status != http.StatusBadRequest || p.Code != "validation_failed" || len(p.Errors) != 1 || p.Errors[0].Pointer != pointer || p.Errors[0].Code != code {
		t.Errorf("%s: %d %s, want 400 %s at %s", what, r.Status, r.Body, code, pointer)
	}
}

// noNull fails when a body carries JSON null (ADR 0053).
func noNull(t *testing.T, what string, body []byte) {
	t.Helper()
	if strings.Contains(string(body), "null") {
		t.Errorf("%s carries null: %s", what, body)
	}
}
