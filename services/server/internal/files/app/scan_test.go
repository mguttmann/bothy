// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// scanCfg is the configuration of the scan tests.
var scanCfg = app.ScanConfig{Mode: app.ScanModeRequired, Timeout: 5 * time.Second, HealthInterval: 30 * time.Second,
	RetryBackoffCap: 5 * time.Minute, SniffBytes: 16}

// scanEnv is a scan pass over the fakes.
type scanEnv struct {
	pass      *app.ScanPass
	repo      *repo
	store     *countingStore
	scanner   *scanner
	queue     *scanQueue
	purges    *purgeQueue
	audit     *audit
	events    *events
	metrics   *scanMetrics
	clk       *clock.Fake
	logs      *bytes.Buffer
	mu        sync.Mutex
	finalized []ids.ID
}

func newScanEnv(t *testing.T, cfg app.ScanConfig) *scanEnv {
	t.Helper()
	return newScanEnvTypes(t, cfg, detector{})
}

// newScanEnvTypes is newScanEnv with the type detection types.
func newScanEnvTypes(t *testing.T, cfg app.ScanConfig, types app.TypeDetector) *scanEnv {
	t.Helper()
	e := &scanEnv{repo: newRepo(), store: newCountingStore(newStore(t)), scanner: &scanner{verdict: domain.Verdict{Kind: domain.VerdictClean}},
		queue: &scanQueue{}, purges: &purgeQueue{}, audit: &audit{}, events: &events{}, metrics: &scanMetrics{},
		clk: clock.NewFake(time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)), logs: &bytes.Buffer{}}
	u := newUoWWith(t, func(r *txn.Registry) error {
		return txn.Subscribe(r, func(_ context.Context, ev files.FileFinalized) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.finalized = append(e.finalized, ev.FileID)
			return nil
		})
	})
	p, err := app.NewScanPass(cfg, app.ScanDeps{Repo: e.repo, Scans: e.repo, Store: e.store, UoW: u, Events: e.events, Audit: e.audit,
		Channels: channels{space: ids.New()}, Scanner: e.scanner, Types: types, Queue: e.queue, Purge: e.purges, Metrics: e.metrics, Clock: e.clk,
		Log: slog.New(slog.NewJSONHandler(e.logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	e.pass = p
	return e
}

// seed stores a completed upload of content declared with size and digest of declared, in parts of
// four bytes (assembled false) or as the joined object (assembled true).
func (e *scanEnv) seed(t *testing.T, content, declared []byte, assembled bool) app.SessionRecord {
	t.Helper()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(declared)
	f := app.FileRecord{ID: id, ObjectKey: key, SHA256: sum, SizeBytes: int64(len(declared)), MIMEType: "application/octet-stream",
		DeclaredMIMEType: "text/plain", OriginalName: "canary-name.txt", UploadedBy: ids.New(), Purpose: "attachment", ScopeChannelID: ids.New(),
		UploadState: "pending", ScanStatus: "pending", TranscriptStatus: "none", CreatedAt: e.clk.Now()}
	count := (len(declared) + 3) / 4
	s := app.SessionRecord{FileID: id, Mode: "direct", State: domain.SessionCompleted, DeclaredSizeBytes: int64(len(declared)),
		DeclaredSHA256: sum, PartSizeBytes: 4, PartCount: count, File: f}
	e.repo.files[id], e.repo.sessions[id] = f, s
	ctx := context.Background()
	if assembled {
		if err := e.store.store.Put(ctx, key, bytes.NewReader(content), int64(len(content))); err != nil {
			t.Fatal(err)
		}
		return s
	}
	for n := 1; n <= count; n++ {
		lo := (n - 1) * 4
		b := content[min(lo, len(content)):min(lo+4, len(content))]
		pk, _ := domain.UploadPartKey(id, n)
		if err := e.store.store.Put(ctx, pk, bytes.NewReader(b), int64(len(b))); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func (e *scanEnv) file(id ids.ID) app.FileRecord {
	e.repo.mu.Lock()
	defer e.repo.mu.Unlock()
	return e.repo.files[id]
}

func (e *scanEnv) finalCount(id ids.ID) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, f := range e.finalized {
		if f == id {
			n++
		}
	}
	return n
}

func (e *scanEnv) partKeys(s app.SessionRecord) []string {
	var out []string
	for n := 1; n <= s.PartCount; n++ {
		k, _ := domain.UploadPartKey(s.FileID, n)
		out = append(out, k)
	}
	return out
}

var pngContent = append(append([]byte{}, pngMagic...), []byte("pixels of a small image")...)

func TestScanPassReadsEachByteOnce(t *testing.T) {
	t.Parallel()
	t.Run("parts", func(t *testing.T) {
		e := newScanEnv(t, scanCfg)
		content := []byte("0123456789abcdefghij")
		s := e.seed(t, content, content, false)
		res, err := e.pass.Run(context.Background(), s.FileID, 0)
		if err != nil || res.Outcome != "clean" || !res.Transitioned {
			t.Fatalf("Run = %+v, %v", res, err)
		}
		for _, k := range e.partKeys(s) {
			if e.store.gets[k] != 1 {
				t.Errorf("part %s read %d times", k, e.store.gets[k])
			}
			if e.store.has(k) {
				t.Errorf("part %s not deleted", k)
			}
		}
		var total int64
		for _, n := range e.store.read {
			total += n
		}
		if total != int64(len(content)) {
			t.Errorf("read %d bytes, want %d", total, len(content))
		}
		if !bytes.Equal(e.store.puts[s.File.ObjectKey], content) || !bytes.Equal(e.scanner.seen, content) {
			t.Errorf("Put %q, scanner %q", e.store.puts[s.File.ObjectKey], e.scanner.seen)
		}
		if !e.store.has(s.File.ObjectKey) {
			t.Error("the clean object is gone")
		}
		f := e.file(s.FileID)
		if f.UploadState != "complete" || f.ScanStatus != "clean" || f.MIMEType != "application/octet-stream" {
			t.Errorf("file %+v", f)
		}
		if got := e.repo.finished[0]; got.ScanEngine != "clamd" || got.ScannedAt.IsZero() {
			t.Errorf("result %+v", got)
		}
		if len(e.events.list) != 1 || e.finalCount(s.FileID) != 1 || len(e.audit.list()) != 0 {
			t.Errorf("events %d, finalized %d, audits %d", len(e.events.list), e.finalCount(s.FileID), len(e.audit.list()))
		}
		ev := e.events.list[0]
		if ev.ActorKind != "" || !ev.ActorPrincipalID.IsZero() || ev.ReceiveAction != "file.read" || ev.ResourceKind != "file" ||
			ev.AudiencePrincipalID != s.File.UploadedBy || bytes.Contains(ev.Payload, []byte("null")) {
			t.Errorf("event %+v %s", ev, ev.Payload)
		}
		if !strings.Contains(string(ev.Payload), `"scan_status":"clean"`) || !strings.Contains(string(ev.Payload), `"sha256":"`) {
			t.Errorf("payload %s", ev.Payload)
		}
		if got := e.metrics.outcomes; len(got) != 1 || got[0] != "clean" {
			t.Errorf("metrics %v", got)
		}
	})
	t.Run("object", func(t *testing.T) {
		e := newScanEnv(t, scanCfg)
		s := e.seed(t, pngContent, pngContent, true)
		res, err := e.pass.Run(context.Background(), s.FileID, 0)
		if err != nil || res.Outcome != "clean" {
			t.Fatalf("Run = %+v, %v", res, err)
		}
		if e.store.gets[s.File.ObjectKey] != 1 || e.store.read[s.File.ObjectKey] != int64(len(pngContent)) || len(e.store.puts) != 0 {
			t.Errorf("gets %v, read %v, puts %d", e.store.gets, e.store.read, len(e.store.puts))
		}
		if f := e.file(s.FileID); f.MIMEType != "image/png" {
			t.Errorf("mime %q", f.MIMEType)
		}
	})
}

func TestScanPassModeOff(t *testing.T) {
	t.Parallel()
	cfg := scanCfg
	cfg.Mode = app.ScanModeOff
	e := newScanEnv(t, cfg)
	s := e.seed(t, pngContent, pngContent, false)
	res, err := e.pass.Run(context.Background(), s.FileID, 0)
	if err != nil || res.Outcome != "skipped" {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	if scans, pings := e.scanner.counts(); scans != 0 || pings != 0 {
		t.Errorf("scanner called: %d scans, %d pings", scans, pings)
	}
	// A PNG stays pending until files.thumbnail stored its cleaned original (M3-06 design note 1c):
	// the scan queues the job and publishes no FileFinalized.
	f := e.file(s.FileID)
	if f.ScanStatus != "skipped" || f.UploadState != "pending" || f.MIMEType != "image/png" {
		t.Errorf("file %+v", f)
	}
	if r := e.repo.finished[0]; r.ScanEngine != "" || !r.ScannedAt.IsZero() {
		t.Errorf("skipped result %+v sets the engine", r)
	}
	if !e.store.has(s.File.ObjectKey) || e.finalCount(s.FileID) != 0 || len(e.events.list) != 1 {
		t.Error("object or event missing, or the image finalized by the scan")
	}
	if got := e.queue.thumbnails(); len(got) != 1 || got[0] != s.FileID {
		t.Errorf("files.thumbnail jobs %v", got)
	}
	if got := e.metrics.outcomes; len(got) != 1 || got[0] != "skipped" {
		t.Errorf("metrics %v", got)
	}
}

func TestScanPassTransientReschedules(t *testing.T) {
	t.Parallel()
	e := newScanEnv(t, scanCfg)
	e.scanner.pingErr = errors.New("connection refused")
	s := e.seed(t, pngContent, pngContent, false)
	wantDelays := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute}
	for round, d := range wantDelays {
		res, err := e.pass.Run(context.Background(), s.FileID, int64(round))
		if err != nil || res.Outcome != app.OutcomeRetry || res.RetryReason != app.RetryUnreachable {
			t.Fatalf("round %d: %+v, %v", round, res, err)
		}
		jobs := e.queue.list()
		j := jobs[len(jobs)-1]
		if j.round != int64(round)+1 || !j.at.Equal(e.clk.Now().Add(d)) {
			t.Errorf("round %d: job %+v, want round %d at +%v", round, j, round+1, d)
		}
	}
	if total := len(e.store.read); total != 0 {
		t.Errorf("bytes read: %v", e.store.read)
	}
	if f := e.file(s.FileID); f.UploadState != "pending" || f.ScanStatus != "pending" {
		t.Errorf("file %+v", f)
	}
	if len(e.events.list) != 0 || len(e.audit.list()) != 0 || e.finalCount(s.FileID) != 0 || len(e.metrics.outcomes) != 0 {
		t.Error("a retry wrote an event, an audit entry, an in-transaction event or a metric")
	}
	if !strings.Contains(e.logs.String(), `"bothy.reason":"unreachable"`) || strings.Contains(e.logs.String(), "canary-name") {
		t.Errorf("logs %s", e.logs.String())
	}
	// A transient scan error and a protocol error are retried as well.
	e.scanner.pingErr = nil
	e.scanner.err = context.DeadlineExceeded
	if res, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil || res.RetryReason != app.RetryTimeout {
		t.Fatalf("timeout: %+v, %v", res, err)
	}
	e.scanner.err = domain.ErrClamdProtocol
	if res, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil || res.RetryReason != app.RetryProtocol {
		t.Fatalf("protocol: %+v, %v", res, err)
	}
	for _, k := range e.partKeys(s) {
		if !e.store.has(k) {
			t.Errorf("part %s deleted by a retried pass", k)
		}
	}
}

func TestScanPassNotAwaiting(t *testing.T) {
	t.Parallel()
	e := newScanEnv(t, scanCfg)
	final := e.seed(t, pngContent, pngContent, true)
	f := e.repo.files[final.FileID]
	f.UploadState, f.ScanStatus = "complete", "clean"
	e.repo.files[final.FileID] = f
	deleted := e.seed(t, pngContent, pngContent, true)
	e.repo.setDeleted(deleted.FileID)
	open := e.seed(t, pngContent, pngContent, true)
	sOpen := e.repo.sessions[open.FileID]
	sOpen.State = domain.SessionOpen
	e.repo.sessions[open.FileID] = sOpen
	for _, id := range []ids.ID{final.FileID, deleted.FileID, open.FileID, ids.New()} {
		res, err := e.pass.Run(context.Background(), id, 0)
		if err != nil || res.Outcome != app.OutcomeNone {
			t.Errorf("%s: %+v, %v", id, res, err)
		}
	}
	if scans, pings := e.scanner.counts(); scans+pings != 0 || e.store.count("Delete") != 0 || len(e.events.list) != 0 {
		t.Error("a file that does not await the pass was touched")
	}
}

func TestScanPassDrainsAfterEarlyVerdict(t *testing.T) {
	t.Parallel()
	e := newScanEnv(t, scanCfg)
	e.scanner.readLimit = 3
	e.scanner.verdict = domain.Verdict{Kind: domain.VerdictError, Reason: domain.ReasonScanStreamLimitExceeded}
	content := []byte("0123456789abcdef")
	s := e.seed(t, content, content, false)
	res, err := e.pass.Run(context.Background(), s.FileID, 0)
	if err != nil || res.Outcome != "error" {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	if !bytes.Equal(e.store.puts[s.File.ObjectKey], content) {
		t.Errorf("assembly %q, want the whole file", e.store.puts[s.File.ObjectKey])
	}
	if e.store.has(s.File.ObjectKey) || e.repo.failure[s.FileID] != "scan_stream_limit_exceeded" {
		t.Errorf("blocked object kept or reason %q", e.repo.failure[s.FileID])
	}
	// The same early stop with a wrong checksum decides failed, never clamd.
	e2 := newScanEnv(t, scanCfg)
	e2.scanner.readLimit = 3
	e2.scanner.err = errors.New("connection reset")
	s2 := e2.seed(t, []byte("0123456789abcdeX"), content, false)
	res, err = e2.pass.Run(context.Background(), s2.FileID, 0)
	if err != nil || res.Outcome != app.OutcomeFailed || e2.repo.failure[s2.FileID] != "checksum_mismatch" {
		t.Fatalf("checksum with transient: %+v, %v, %q", res, err, e2.repo.failure[s2.FileID])
	}
	if e2.store.has(s2.File.ObjectKey) || len(e2.store.puts) != 0 {
		t.Error("a failed assembly left an object")
	}
}

func TestScanPassInfected(t *testing.T) {
	t.Parallel()
	e := newScanEnv(t, scanCfg)
	e.scanner.verdict = domain.Verdict{Kind: domain.VerdictInfected, Signature: "Local.Test.EICAR.UNOFFICIAL", Reason: domain.ReasonMalwareDetected}
	content := []byte("not really malware")
	s := e.seed(t, content, content, false)
	res, err := e.pass.Run(context.Background(), s.FileID, 0)
	if err != nil || res.Outcome != "infected" {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	if e.store.has(s.File.ObjectKey) {
		t.Error("infected object kept")
	}
	for _, k := range e.partKeys(s) {
		if e.store.has(k) {
			t.Errorf("part %s kept", k)
		}
	}
	entries := e.audit.list()
	if len(entries) != 1 {
		t.Fatalf("audits %+v", entries)
	}
	a := entries[0]
	sum := sha256.Sum256(content)
	if a.Action != "file.upload" || a.Decision != auditing.DecisionDeny || a.Reason != auditing.ReasonMalwareDetected ||
		a.ResourceKind != "file" || a.ResourceID != s.FileID || len(a.Attrs) != 3 ||
		a.Attrs["signature"].Data() != "local.test.eicar.unofficial" || a.Attrs["size_bytes"].Data() != int64(len(content)) ||
		a.Attrs["sha256"].Data() != hexOf(sum) {
		t.Errorf("audit %+v", a)
	}
	if e.finalCount(s.FileID) != 1 || len(e.events.list) != 1 || e.repo.failure[s.FileID] != "malware_detected" {
		t.Error("event, finalized or reason")
	}
	if strings.Contains(e.logs.String(), "EICAR") || strings.Contains(string(e.events.list[0].Payload), "EICAR") {
		t.Error("the signature left the audit log")
	}
	if got := e.metrics.outcomes; len(got) != 1 || got[0] != "infected" {
		t.Errorf("metrics %v", got)
	}
}

func hexOf(b [32]byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 64)
	for i, c := range b {
		out[2*i], out[2*i+1] = digits[c>>4], digits[c&15]
	}
	return string(out)
}

func TestScanPassFailedOutcomes(t *testing.T) {
	t.Parallel()
	t.Run("checksum with FOUND", func(t *testing.T) {
		e := newScanEnv(t, scanCfg)
		e.scanner.verdict = domain.Verdict{Kind: domain.VerdictInfected, Signature: "X", Reason: domain.ReasonMalwareDetected}
		s := e.seed(t, []byte("abcdefgX"), []byte("abcdefgh"), false)
		res, err := e.pass.Run(context.Background(), s.FileID, 0)
		if err != nil || res.Outcome != app.OutcomeFailed || e.repo.failure[s.FileID] != "checksum_mismatch" {
			t.Fatalf("%+v %v %q", res, err, e.repo.failure[s.FileID])
		}
		f := e.file(s.FileID)
		if f.UploadState != "failed" || f.ScanStatus != "pending" || f.MIMEType != "application/octet-stream" {
			t.Errorf("file %+v", f)
		}
		if len(e.audit.list()) != 1 || e.finalCount(s.FileID) != 1 || len(e.events.list) != 1 || len(e.metrics.outcomes) != 0 {
			t.Error("audit, finalized, event or metric")
		}
		if r := e.repo.finished[0]; r.ScanEngine != "" {
			t.Errorf("failed file got an engine: %+v", r)
		}
	})
	t.Run("size", func(t *testing.T) {
		e := newScanEnv(t, scanCfg)
		s := e.seed(t, []byte("abcdefghi"), []byte("abcdefgh"), true)
		res, err := e.pass.Run(context.Background(), s.FileID, 0)
		if err != nil || res.Outcome != app.OutcomeFailed || e.repo.failure[s.FileID] != "size_mismatch" {
			t.Fatalf("%+v %v %q", res, err, e.repo.failure[s.FileID])
		}
		if e.store.read[s.File.ObjectKey] > 9 || e.store.has(s.File.ObjectKey) {
			t.Errorf("read %d bytes, object kept %v", e.store.read[s.File.ObjectKey], e.store.has(s.File.ObjectKey))
		}
	})
	t.Run("denied type", func(t *testing.T) {
		cfg := scanCfg
		cfg.DeniedMIMETypes = []string{"Image/PNG"}
		e := newScanEnv(t, cfg)
		s := e.seed(t, pngContent, pngContent, false)
		res, err := e.pass.Run(context.Background(), s.FileID, 0)
		if err != nil || res.Outcome != app.OutcomeFailed || e.repo.failure[s.FileID] != "file_type_denied" {
			t.Fatalf("%+v %v %q", res, err, e.repo.failure[s.FileID])
		}
		if f := e.file(s.FileID); f.MIMEType != "image/png" || e.store.has(s.File.ObjectKey) {
			t.Errorf("file %+v", f)
		}
	})
}

func TestScanConcurrentRuns(t *testing.T) {
	t.Parallel()
	e := newScanEnv(t, scanCfg)
	e.scanner.verdict = domain.Verdict{Kind: domain.VerdictInfected, Signature: "X", Reason: domain.ReasonMalwareDetected}
	s := e.seed(t, []byte("0123456789"), []byte("0123456789"), false)
	// Both runs are inside their scan before either writes its result.
	var arrived sync.WaitGroup
	arrived.Add(2)
	e.scanner.during = func() {
		arrived.Done()
		arrived.Wait()
	}
	var wg sync.WaitGroup
	results := make([]app.PassResult, 2)
	for i := range results {
		wg.Go(func() {
			r, err := e.pass.Run(context.Background(), s.FileID, 0)
			if err != nil {
				t.Error(err)
			}
			results[i] = r
		})
	}
	wg.Wait()
	transitions := 0
	for _, r := range results {
		if r.Transitioned {
			transitions++
		}
	}
	if transitions != 1 || len(e.events.list) != 1 || len(e.audit.list()) != 1 || e.finalCount(s.FileID) != 1 {
		t.Fatalf("results %+v, events %d, audits %d", results, len(e.events.list), len(e.audit.list()))
	}
	if e.store.has(s.File.ObjectKey) {
		t.Error("the infected object came back through the losing run")
	}
}

func TestScanFileDeletedDuringPass(t *testing.T) {
	t.Parallel()
	e := newScanEnv(t, scanCfg)
	s := e.seed(t, []byte("0123456789"), []byte("0123456789"), false)
	e.scanner.during = func() { e.repo.setDeleted(s.FileID) }
	res, err := e.pass.Run(context.Background(), s.FileID, 0)
	if err != nil || res.Transitioned || res.Outcome != app.OutcomeNone {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	if f := e.file(s.FileID); f.UploadState != "pending" || f.ScanStatus != "pending" {
		t.Errorf("file %+v", f)
	}
	if len(e.events.list) != 0 || e.finalCount(s.FileID) != 0 {
		t.Error("event for a deleted file")
	}
	if e.store.has(s.File.ObjectKey) {
		t.Error("the assembled object of a deleted file stays")
	}
}

func TestScanPassCancelled(t *testing.T) {
	t.Parallel()
	e := newScanEnv(t, scanCfg)
	s := e.seed(t, pngContent, pngContent, false)
	ctx, cancel := context.WithCancel(context.Background())
	e.scanner.during = cancel
	e.scanner.err = context.Canceled
	if _, err := e.pass.Run(ctx, s.FileID, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run = %v, want the cancellation", err)
	}
	if len(e.queue.list()) != 0 {
		t.Error("a cancelled run scheduled a round")
	}
}

func TestScanHealthMonitor(t *testing.T) {
	t.Parallel()
	r := newRepo()
	for range 2 {
		id := ids.New()
		r.files[id] = app.FileRecord{ID: id, UploadState: "pending", ScanStatus: "pending"}
		r.sessions[id] = app.SessionRecord{FileID: id, State: domain.SessionCompleted}
	}
	sc := &scanner{}
	m := &scanMetrics{}
	var logs bytes.Buffer
	clk := clock.NewFake(time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC))
	h, err := app.NewScanHealth(scanCfg, sc, r, m, clk, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if st := h.ScannerState(); st.Status != files.ScannerUnknown {
		t.Fatalf("initial %+v", st)
	}
	ctx := context.Background()
	h.Check(ctx)
	if st := h.ScannerState(); st.Status != files.ScannerOK || !st.CheckedAt.Equal(clk.Now()) {
		t.Fatalf("after PONG %+v", st)
	}
	sc.pingErr = errors.New("refused")
	h.Check(ctx)
	h.Check(ctx)
	if st := h.ScannerState(); st.Status != files.ScannerUnreachable {
		t.Fatalf("after failure %+v", st)
	}
	sc.pingErr = nil
	h.Check(ctx)
	if st := h.ScannerState(); st.Status != files.ScannerOK {
		t.Fatalf("after recovery %+v", st)
	}
	if c := strings.Count(logs.String(), "virus scanner unreachable"); c != 1 {
		t.Errorf("unreachable logged %d times", c)
	}
	if c := strings.Count(logs.String(), "virus scanner reachable again"); c != 1 {
		t.Errorf("recovery logged %d times", c)
	}
	if len(m.queue) != 4 || m.queue[0] != 2 {
		t.Errorf("queue %v", m.queue)
	}
	off := scanCfg
	off.Mode = app.ScanModeOff
	sc2 := &scanner{}
	h2, err := app.NewScanHealth(off, sc2, r, m, clk, nil)
	if err != nil {
		t.Fatal(err)
	}
	h2.Check(ctx)
	if st := h2.ScannerState(); st.Status != files.ScannerOff || sc2.pings != 0 {
		t.Errorf("mode off %+v, %d pings", st, sc2.pings)
	}
}

func TestCompleteUploadInsertsScanJob(t *testing.T) {
	t.Parallel()
	e := newEnv(t, "fs", testCfg)
	s, refs := uploadAll(t, e, e.ctx)
	fileID := s.FileID
	if _, err := e.svc.Complete(e.ctx, files.CompleteUpload{FileID: fileID, Parts: refs}); err != nil {
		t.Fatal(err)
	}
	// A completed session answers again without a second job.
	if _, err := e.svc.Complete(e.ctx, files.CompleteUpload{FileID: fileID, Parts: refs}); err != nil {
		t.Fatal(err)
	}
	jobs := e.scans.list()
	if len(jobs) != 1 || jobs[0].fileID != fileID || jobs[0].round != 0 || !jobs[0].at.IsZero() {
		t.Fatalf("jobs %+v, want one round 0 job", jobs)
	}
}
