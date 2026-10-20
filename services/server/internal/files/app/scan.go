// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The values of file.scan.mode (docs/spec/12-dateien.md 5.3).
const (
	// ScanModeRequired scans every file and keeps it pending while clamd is not reachable.
	ScanModeRequired = "required"
	// ScanModeOff scans nothing; files end skipped.
	ScanModeOff = "off"
)

// scanEngine is files.scan_engine of a file clamd decided (docs/DECISIONS.md D-M3-04-5).
const scanEngine = "clamd"

// The reason classes of a retried pass in the Warn log (brief M3-04 design note 7).
const (
	RetryUnreachable = "unreachable"
	RetryTimeout     = "timeout"
	RetryProtocol    = "protocol"
	RetryStorage     = "storage"
)

// The values of PassResult.Outcome besides the scan statuses.
const (
	// OutcomeFailed: the file ended upload_state failed.
	OutcomeFailed = "failed"
	// OutcomeRetry: the pass failed transiently and the next round is scheduled.
	OutcomeRetry = "retry"
	// OutcomeNone: the file did not await the pass, or another run decided it first.
	OutcomeNone = "none"
)

// logKeyReason is bothy.reason of the attribute registry (platform/otel/attr.Reason): the reason
// class of a retry, a value of a fixed list.
const logKeyReason = "bothy.reason"

// Log messages of the scan pass; constant text, the only attributes are bothy.file.id,
// bothy.outcome and bothy.reason (signals.md "Logs").
const (
	msgScanRetry        = "file scan retried"
	msgScanDeleteFailed = "file scan could not delete objects"
	msgScanStorage      = "file scan storage failure"
)

// ScanConfig holds the configuration of the scan pass, bound in internal/wire (wire.ScanConfig).
// Limits are named keys of docs/operations/configuration.md.
type ScanConfig struct {
	// Mode is file.scan.mode, ScanModeRequired or ScanModeOff.
	Mode string `config:"file.scan.mode"`
	// Timeout is file.scan.timeout, the deadline of one pass.
	Timeout time.Duration `config:"file.scan.timeout"`
	// HealthInterval is file.scan.health_interval: the first retry delay and the deadline of PING.
	HealthInterval time.Duration `config:"file.scan.health_interval"`
	// RetryBackoffCap is file.scan.retry_backoff_cap, the longest retry delay.
	RetryBackoffCap time.Duration `config:"file.scan.retry_backoff_cap"`
	// SniffBytes is file.sniff_bytes, the head the type detection reads.
	SniffBytes int `config:"file.sniff_bytes"`
	// DeniedMIMETypes is file.denied_mime_types, checked against the detected type.
	DeniedMIMETypes []string `config:"file.denied_mime_types"`
}

// check refuses values the pass cannot work with.
func (c ScanConfig) check() error {
	if c.Mode != ScanModeRequired && c.Mode != ScanModeOff {
		return errors.New("files: file.scan.mode must be required or off")
	}
	if c.Timeout <= 0 || c.HealthInterval <= 0 || c.RetryBackoffCap <= 0 || c.SniffBytes < 1 {
		return errors.New("files: file.scan.timeout, file.scan.health_interval, file.scan.retry_backoff_cap and file.sniff_bytes must be positive")
	}
	return nil
}

// ScanDeps are the collaborators of the scan pass.
type ScanDeps struct {
	// Repo reads the confirmed parts (their ETags for an S3 multipart upload).
	Repo Repository
	// Scans reads and writes the state of the pass.
	Scans ScanRepository
	// Store reads the bytes and writes the assembled object.
	Store ObjectStore
	// UoW runs the result and the retry units.
	UoW txn.UnitOfWork
	// Events appends file.updated; Audit records the malware entry.
	Events eventlog.Appender
	Audit  auditing.Recorder
	// Channels gives the space of the scope channel for file.updated.
	Channels ChannelLocator
	// Scanner is clamd; it may be nil with file.scan.mode = off.
	Scanner Scanner
	// Types detects the media type.
	Types TypeDetector
	// Queue inserts the next round.
	Queue ScanQueue
	// Metrics records bothy.file.scan and bothy.file.scan.duration; nil records nothing.
	Metrics ScanMetrics
	// Clock gives scanned_at and the retry time; nil means clock.System().
	Clock clock.Clock
	// Log receives the Warn of a retry and the Error of a storage failure; nil means
	// slog.Default().
	Log *slog.Logger
}

// ScanPass is the use case of the job files.scan (12 3.4, 5): one read pass over the stored bytes
// for size, SHA-256, type and clamd, then the result in one unit of work. It is safe for
// concurrent use; two runs for the same file decide it once (the result unit locks the rows).
type ScanPass struct {
	cfg ScanConfig
	d   ScanDeps
}

// PassResult reports what one run did.
type PassResult struct {
	// Outcome is clean, infected, error, skipped, OutcomeFailed, OutcomeRetry or OutcomeNone.
	Outcome string
	// Transitioned reports that this run wrote the new state.
	Transitioned bool
	// RetryReason is the reason class of OutcomeRetry.
	RetryReason string
}

// NewScanPass returns the use case. Errors: a missing collaborator (the scanner may be nil only
// with mode off) or a value ScanConfig refuses.
func NewScanPass(cfg ScanConfig, d ScanDeps) (*ScanPass, error) {
	if d.Repo == nil || d.Scans == nil || d.Store == nil || d.UoW == nil || d.Events == nil || d.Audit == nil ||
		d.Channels == nil || d.Types == nil || d.Queue == nil || (d.Scanner == nil && cfg.Mode == ScanModeRequired) {
		return nil, errors.New("files: NewScanPass needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Metrics == nil {
		d.Metrics = noScanMetrics{}
	}
	return &ScanPass{cfg: cfg, d: d}, nil
}

// awaiting reports a file that waits for the pass (brief M3-04 design note 2).
func awaiting(s SessionRecord) bool {
	return s.State == domain.SessionCompleted && !s.File.Deleted && s.File.UploadState == domain.UploadPending &&
		s.File.ScanStatus == domain.ScanPending
}

// Run runs round of the pass for fileID (design notes 2 to 7). It never calls the scanner or the
// object store inside a unit of work (02 8.5). A transient failure (clamd unreachable, timeout,
// protocol, storage) schedules round+1 and returns nil, so River's attempt limit never ends the
// retries (12 5.3); only a failed database unit returns an error, and a cancelled ctx returns
// ctx.Err().
func (p *ScanPass) Run(ctx context.Context, fileID ids.ID, round int64) (PassResult, error) {
	start := time.Now()
	target, found, err := p.d.Scans.ScanTarget(ctx, fileID)
	if err != nil {
		return PassResult{}, err
	}
	if !found || !awaiting(target) {
		return PassResult{Outcome: OutcomeNone}, nil
	}
	if p.cfg.Mode == ScanModeRequired {
		pctx, cancel := context.WithTimeout(ctx, p.cfg.HealthInterval)
		err := p.d.Scanner.Ping(pctx)
		cancel()
		if err != nil {
			return p.retry(ctx, fileID, round, retryClass(err))
		}
	}
	pctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	res, err := p.pass(pctx, target)
	cancel()
	if ctx.Err() != nil {
		return PassResult{}, ctx.Err()
	}
	if err != nil {
		p.d.Log.ErrorContext(ctx, msgScanStorage, slog.String(logKeyFileID, fileID.String()), slog.String(logKeyErrorType, RetryStorage))
		return p.retry(ctx, fileID, round, RetryStorage)
	}
	out := domain.DecidePass(res.facts)
	if !out.Retry && !out.DeleteObjects && res.assembled && res.putErr != nil {
		// A kept file needs its assembled object; without it the pass is repeated.
		p.d.Log.ErrorContext(ctx, msgScanStorage, slog.String(logKeyFileID, fileID.String()), slog.String(logKeyErrorType, RetryStorage))
		return p.retry(ctx, fileID, round, RetryStorage)
	}
	if out.Retry {
		return p.retry(ctx, fileID, round, retryClass(res.scanErr))
	}
	return p.finish(ctx, target, res, out, start)
}

// passState is what the read pass produced.
type passState struct {
	facts    domain.PassFacts
	mimeType string
	// assembled reports that the pass wrote object_key from the parts (file system layout);
	// written reports that the Put succeeded.
	assembled, written bool
	putErr             error
	scanErr            error
}

// errAssemblyAborted closes the pipe of an assembly whose bytes are not accepted.
var errAssemblyAborted = errors.New("files: assembly aborted")

// pass is the one read pass of 12 3.4 (design note 4). The error is a storage failure before or
// while reading; a scanner error is kept in passState.scanErr.
func (p *ScanPass) pass(ctx context.Context, t SessionRecord) (passState, error) {
	src, assemble, err := p.source(ctx, t)
	if err != nil {
		return passState{}, err
	}
	defer func() { _ = src.Close() }()
	declared := t.DeclaredSizeBytes
	sr := &sourceReader{r: io.LimitReader(src, declared+1)}
	h := sha256.New()
	cnt := &counter{}
	head := &headBuffer{max: p.cfg.SniffBytes}
	sinks := []io.Writer{h, cnt, head}
	var put *assembly
	if assemble {
		put = p.assemble(ctx, t.File.ObjectKey, declared)
		sinks = append(sinks, put)
	}
	tee := io.TeeReader(sr, multiSink(sinks))
	st := passState{assembled: assemble}
	if p.cfg.Mode == ScanModeRequired {
		v, err := p.d.Scanner.Scan(ctx, tee)
		st.facts.Verdict, st.facts.Scanned, st.scanErr = v, err == nil, err
	} else {
		st.facts.ScanOff = true
	}
	// Drain what the scanner left, so size, digest and the assembly never depend on clamd.
	_, _ = io.Copy(io.Discard, tee)
	if sr.err != nil {
		if put != nil {
			put.abort()
		}
		return passState{}, sr.err
	}
	copy(st.facts.ComputedSHA256[:], h.Sum(nil))
	st.facts.DeclaredSize, st.facts.DeclaredSHA256, st.facts.CountedSize = declared, t.DeclaredSHA256, cnt.n
	st.mimeType = domain.MediaType(p.d.Types.Detect(head.b))
	st.facts.TypeDenied = domain.TypeDenied(st.mimeType, p.cfg.DeniedMIMETypes)
	if put != nil {
		if cnt.n != declared || st.facts.ComputedSHA256 != t.DeclaredSHA256 {
			put.abort()
		}
		st.putErr = put.wait()
		st.written = st.putErr == nil
	}
	return st, nil
}

// source opens the bytes of t (design note 4a): object_key when it exists; on S3 the multipart
// upload is completed first (merged M3-03 leaves it open at completion, docs/DECISIONS.md
// D-M3-04-7); otherwise the parts uploads/<file_id>/<n> in order, and assemble is true.
func (p *ScanPass) source(ctx context.Context, t SessionRecord) (io.ReadCloser, bool, error) {
	key := t.File.ObjectKey
	_, err := p.d.Store.Head(ctx, key)
	switch {
	case err == nil:
	case !errors.Is(err, ErrNotFound):
		return nil, false, err
	case t.StorageUploadID != "":
		if err := p.completeMultipart(ctx, t); err != nil {
			return nil, false, err
		}
	default:
		return &partsReader{ctx: ctx, store: p.d.Store, fileID: t.FileID, count: t.PartCount}, true, nil
	}
	r, err := p.d.Store.Get(ctx, key, nil)
	if err != nil {
		return nil, false, err
	}
	return r, false, nil
}

// completeMultipart joins the confirmed parts of an S3 upload into object_key.
func (p *ScanPass) completeMultipart(ctx context.Context, t SessionRecord) error {
	rows, err := p.d.Repo.Parts(ctx, t.FileID)
	if err != nil {
		return err
	}
	parts := make([]Part, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, Part{Number: r.PartNumber, ETag: r.ETag})
	}
	return p.d.Store.CompleteMultipart(ctx, t.File.ObjectKey, t.StorageUploadID, parts)
}

// assembly writes object_key from a pipe in its own goroutine (the driver writes a temporary
// file, fsync and rename, 12 4.2). Writes never fail towards the tee: after a failed Put the
// bytes are dropped and wait reports the error.
type assembly struct {
	pw   *io.PipeWriter
	done chan error
	err  error
}

func (p *ScanPass) assemble(ctx context.Context, key string, size int64) *assembly {
	pr, pw := io.Pipe()
	a := &assembly{pw: pw, done: make(chan error, 1)}
	go func() {
		err := p.d.Store.Put(ctx, key, pr, size)
		// Unblock the writer for good: after the Put no byte is taken any more.
		_ = pr.CloseWithError(errAssemblyAborted)
		a.done <- err
	}()
	return a
}

func (a *assembly) Write(b []byte) (int, error) {
	if a.err == nil {
		if _, err := a.pw.Write(b); err != nil {
			a.err = err
		}
	}
	return len(b), nil
}

// abort makes the Put fail, so no object stays.
func (a *assembly) abort() { _ = a.pw.CloseWithError(errAssemblyAborted) }

// wait ends the stream and returns the result of the Put.
func (a *assembly) wait() error {
	_ = a.pw.Close()
	return <-a.done
}

// finish is the result unit of work (design note 6) and the deletions and metrics after it.
func (p *ScanPass) finish(ctx context.Context, t SessionRecord, st passState, out domain.PassOutcome, start time.Time) (PassResult, error) {
	var current SessionRecord
	var transitioned bool
	err := p.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		current, transitioned, err = p.write(ctx, t.FileID, st, out)
		return err
	})
	if err != nil {
		return PassResult{}, err
	}
	if !transitioned {
		// Another run decided the file, or it was deleted: keep the state, remove only what this
		// run wrote and nobody may serve.
		if st.written && !domain.IsDeliverable(current.File.UploadState, current.File.ScanStatus, current.File.Deleted) &&
			!awaitingImage(current.File) {
			p.deleteKeys(ctx, t.FileID, t.File.ObjectKey)
		}
		return PassResult{Outcome: OutcomeNone}, nil
	}
	p.cleanup(ctx, t, out)
	if out.MetricOutcome != "" {
		p.d.Metrics.ScanCompleted(ctx, out.MetricOutcome, time.Since(start))
	}
	outcome := out.ScanStatus
	if out.UploadState == domain.UploadFailed {
		outcome = OutcomeFailed
	}
	return PassResult{Outcome: outcome, Transitioned: true}, nil
}

// write is the work of the result unit: lock and check again, the rows, file.updated, the
// malware entry and FileFinalized. It returns the locked state and whether this run wrote.
func (p *ScanPass) write(ctx context.Context, fileID ids.ID, st passState, out domain.PassOutcome) (SessionRecord, bool, error) {
	cur, found, err := p.d.Scans.LockScanTarget(ctx, fileID)
	if err != nil || !found || !awaiting(cur) {
		return cur, false, err
	}
	res := p.result(cur, st, out)
	thumbnail := thumbnailJob(res)
	if thumbnail && domain.OriginalPlanFor(res.MIMEType) != domain.PlanNone {
		// An image becomes deliverable only with its cleaned original (docs/DECISIONS.md D-M3-06-1).
		res.UploadState = domain.UploadPending
	}
	if err := p.d.Scans.FinishScan(ctx, fileID, res); err != nil {
		return cur, false, err
	}
	f := cur.File
	f.UploadState, f.ScanStatus, f.MIMEType, f.SHA256, f.SizeBytes = res.UploadState, res.ScanStatus, res.MIMEType, res.SHA256, res.SizeBytes
	if err := p.appendUpdated(ctx, f); err != nil {
		return cur, false, err
	}
	if out.AuditMalware {
		if err := p.auditMalware(ctx, fileID, st); err != nil {
			return cur, false, err
		}
	}
	if domain.Final(f.UploadState, f.ScanStatus, false) {
		if err := txn.Publish(ctx, files.FileFinalized{FileID: fileID}); err != nil {
			return cur, false, err
		}
	}
	// files.thumbnail for the images of 12 7.1 and the MP4 and QuickTime files of 12 7.3 (M3-06);
	// M3-15 inserts files.extract_text here for a deliverable file, in this unit of work (12 3.4
	// last paragraph).
	if thumbnail {
		if err := p.d.Queue.EnqueueThumbnail(ctx, fileID); err != nil {
			return cur, false, err
		}
	}
	return cur, true, nil
}

// awaitingImage reports a file the winning run left pending for files.thumbnail (M3-06): its
// original must stay for the job.
func awaitingImage(f FileRecord) bool {
	return !f.Deleted && f.UploadState == domain.UploadPending && (f.ScanStatus == domain.ScanClean || f.ScanStatus == domain.ScanSkipped)
}

// thumbnailJob reports a scan result that inserts files.thumbnail (brief M3-06 design note 1a):
// a deliverable file whose type has an original plan or container values.
func thumbnailJob(res ScanResult) bool {
	if !domain.IsDeliverable(res.UploadState, res.ScanStatus, false) {
		return false
	}
	return domain.OriginalPlanFor(res.MIMEType) != domain.PlanNone || domain.HasContainerMetadata(res.MIMEType)
}

// result builds the row change of out.
func (p *ScanPass) result(cur SessionRecord, st passState, out domain.PassOutcome) ScanResult {
	res := ScanResult{UploadState: out.UploadState, ScanStatus: out.ScanStatus, MIMEType: cur.File.MIMEType,
		SHA256: cur.File.SHA256, SizeBytes: cur.File.SizeBytes, FailureReason: out.FailureReason}
	if out.SetMIMEType {
		res.MIMEType = st.mimeType
	}
	if out.UploadState == domain.UploadComplete {
		res.SHA256, res.SizeBytes = st.facts.ComputedSHA256, st.facts.CountedSize
	}
	if out.ScanEngine {
		res.ScannedAt, res.ScanEngine = p.d.Clock.Now().UTC().Truncate(time.Microsecond), scanEngine
	}
	return res
}

// appendUpdated writes file.updated of f without actor (a job without principal, 04 2.2) to the
// uploader, in the scope of the channel of an attachment (design note 6c).
func (p *ScanPass) appendUpdated(ctx context.Context, f FileRecord) error {
	var space ids.ID
	if !f.ScopeChannelID.IsZero() {
		s, err := p.d.Channels.SpaceOf(ctx, f.ScopeChannelID)
		if err != nil {
			return err
		}
		space = s
	}
	ev, err := FileUpdated(actor.Actor{}, space, FileView(f), true)
	if err != nil {
		return err
	}
	return p.d.Events.Append(ctx, ev)
}

// auditMalware records the malware entry (design note 6d): signature, digest and size, never
// content; the actor is NULL because the job has none.
func (p *ScanPass) auditMalware(ctx context.Context, fileID ids.ID, st passState) error {
	return p.d.Audit.Record(ctx, auditing.Entry{
		Action: "file.upload", Decision: auditing.DecisionDeny, Reason: auditing.ReasonMalwareDetected,
		ResourceKind: "file", ResourceID: fileID,
		Attrs: auditing.Attrs{
			"signature":  auditing.Code(domain.AuditSignature(st.facts.Verdict.Signature)),
			"sha256":     auditing.Code(hex.EncodeToString(st.facts.ComputedSHA256[:])),
			"size_bytes": auditing.Int(st.facts.CountedSize),
		},
	})
}

// cleanup deletes after the commit (design note 6f): object_key of a failed or blocked file, and
// the parts of the file system layout for every outcome. A failed delete is logged and left to
// the sweep of M3-08.
func (p *ScanPass) cleanup(ctx context.Context, t SessionRecord, out domain.PassOutcome) {
	var keys []string
	if out.DeleteObjects {
		keys = append(keys, t.File.ObjectKey)
	}
	if t.StorageUploadID == "" {
		for n := 1; n <= t.PartCount; n++ {
			if k, err := domain.UploadPartKey(t.FileID, n); err == nil {
				keys = append(keys, k)
			}
		}
	}
	p.deleteKeys(ctx, t.FileID, keys...)
}

// deleteKeys removes keys; a missing object is no error (12 4.1).
func (p *ScanPass) deleteKeys(ctx context.Context, fileID ids.ID, keys ...string) {
	ctx = context.WithoutCancel(ctx)
	for _, k := range keys {
		if err := p.d.Store.Delete(ctx, k); err != nil {
			p.d.Log.WarnContext(ctx, msgScanDeleteFailed, slog.String(logKeyFileID, fileID.String()), slog.String(logKeyErrorType, RetryStorage))
		}
	}
}

// retry schedules round+1 in a short unit of work when the file still awaits the pass (design
// note 7): no state change, no event, no audit.
func (p *ScanPass) retry(ctx context.Context, fileID ids.ID, round int64, class string) (PassResult, error) {
	if ctx.Err() != nil {
		return PassResult{}, ctx.Err()
	}
	next := round
	if round < math.MaxInt64 {
		next = round + 1
	}
	at := p.d.Clock.Now().Add(domain.NextScanDelay(round, p.cfg.HealthInterval, p.cfg.RetryBackoffCap))
	var scheduled bool
	err := p.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := p.d.Scans.LockScanTarget(ctx, fileID)
		if err != nil || !found || !awaiting(cur) {
			return err
		}
		scheduled = true
		return p.d.Queue.EnqueueScan(ctx, fileID, next, at)
	})
	if err != nil {
		return PassResult{}, fmt.Errorf("files: scheduling the next scan round: %w", err)
	}
	if !scheduled {
		return PassResult{Outcome: OutcomeNone}, nil
	}
	p.d.Log.WarnContext(ctx, msgScanRetry, slog.String(logKeyFileID, fileID.String()), slog.String(logKeyReason, class))
	return PassResult{Outcome: OutcomeRetry, RetryReason: class}, nil
}

// retryClass maps a scanner error to its reason class.
func retryClass(err error) string {
	switch {
	case errors.Is(err, domain.ErrClamdProtocol):
		return RetryProtocol
	case errors.Is(err, context.DeadlineExceeded):
		return RetryTimeout
	default:
		return RetryUnreachable
	}
}

// sourceReader records the first read error of the source, so a storage failure is told apart
// from a scanner failure.
type sourceReader struct {
	r   io.Reader
	err error
}

func (s *sourceReader) Read(b []byte) (int, error) {
	n, err := s.r.Read(b)
	if err != nil && !errors.Is(err, io.EOF) && s.err == nil {
		s.err = err
	}
	return n, err
}

// counter counts the bytes of the stream.
type counter struct{ n int64 }

func (c *counter) Write(b []byte) (int, error) {
	c.n += int64(len(b))
	return len(b), nil
}

// headBuffer keeps the first max bytes for the type detection.
type headBuffer struct {
	max int
	b   []byte
}

func (h *headBuffer) Write(b []byte) (int, error) {
	if rest := h.max - len(h.b); rest > 0 {
		h.b = append(h.b, b[:min(rest, len(b))]...)
	}
	return len(b), nil
}

// multiSink writes to every writer; none of the sinks of the pass fails.
type multiSink []io.Writer

func (m multiSink) Write(b []byte) (int, error) {
	for _, w := range m {
		_, _ = w.Write(b)
	}
	return len(b), nil
}

// partsReader reads uploads/<file_id>/<n> for n from 1 to count in order, opening each part only
// when the previous one is exhausted.
type partsReader struct {
	ctx    context.Context //nolint:containedctx // the reader is the body of one pass and lives only inside it
	store  ObjectStore
	fileID ids.ID
	count  int
	n      int
	cur    io.ReadCloser
}

func (r *partsReader) Read(b []byte) (int, error) {
	for {
		if r.cur == nil {
			if r.n >= r.count {
				return 0, io.EOF
			}
			if err := r.next(); err != nil {
				return 0, err
			}
		}
		n, err := r.cur.Read(b)
		if errors.Is(err, io.EOF) {
			_ = r.cur.Close()
			r.cur = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

// next opens the following part.
func (r *partsReader) next() error {
	r.n++
	key, err := domain.UploadPartKey(r.fileID, r.n)
	if err != nil {
		return err
	}
	cur, err := r.store.Get(r.ctx, key, nil)
	if err != nil {
		return err
	}
	r.cur = cur
	return nil
}

// Close closes the open part.
func (r *partsReader) Close() error {
	if r.cur != nil {
		return r.cur.Close()
	}
	return nil
}
