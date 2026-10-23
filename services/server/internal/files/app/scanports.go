// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Scanner is the outgoing port to the virus scanner (docs/spec/12-files.md 5, ADR 0033);
// adapters/clamd implements it over the clamd protocol. Implementations are safe for concurrent
// use and bound the number of parallel scans themselves (file.scan.max_concurrent).
type Scanner interface {
	// Scan streams r to the scanner and returns its verdict. An error is transient (connection,
	// timeout through ctx, or a reply that wraps domain.ErrClamdProtocol): the file stays pending.
	// Scan may return before r is exhausted (a verdict such as the stream limit, or an error).
	Scan(ctx context.Context, r io.Reader) (domain.Verdict, error)
	// Ping checks that the scanner answers (PING, PONG); any other result is an error.
	Ping(ctx context.Context) error
}

// TypeDetector detects the media type of a file from its first bytes (12 6.1); adapters/mimetype
// implements it. The result may carry parameters; the pass strips them (domain.MediaType).
type TypeDetector interface {
	// Detect returns the media type of head, application/octet-stream when nothing matches.
	Detect(head []byte) string
}

// bulkJobsKey is the context key of WithBulkJobs.
type bulkJobsKey struct{}

// WithBulkJobs marks ctx so ScanQueue inserts files.scan and files.thumbnail of the work it does
// with the lowest job priority (docs/spec/12-files.md 11, M8-11 design note 6 c): the completion of
// an import file marks it, and the job handlers mark the context of a job that was inserted so,
// which carries the mark into its retry rounds and its thumbnails. Live uploads keep the default
// priority and are fetched first from the shared queue files.
func WithBulkJobs(ctx context.Context) context.Context { return context.WithValue(ctx, bulkJobsKey{}, true) }

// BulkJobs reports the mark of WithBulkJobs.
func BulkJobs(ctx context.Context) bool {
	b, _ := ctx.Value(bulkJobsKey{}).(bool)
	return b
}

// ScanQueue inserts the jobs files.scan and files.thumbnail (02 12.3 "Job catalogue");
// adapters/jobs implements it over the job runtime. It must be called inside a unit of work, so the
// job exists exactly when the change that needs it commits. A context marked with WithBulkJobs
// inserts with the lowest job priority.
type ScanQueue interface {
	// EnqueueScan inserts files.scan for fileID with the retry round; a zero at means now.
	EnqueueScan(ctx context.Context, fileID ids.ID, round int64, at time.Time) error
	// EnqueueThumbnail inserts files.thumbnail for fileID (M3-06), in the final unit of work of
	// the scan.
	EnqueueThumbnail(ctx context.Context, fileID ids.ID) error
}

// ScanResult is what the result unit of work of the scan pass writes (brief M3-04 design note 6b).
type ScanResult struct {
	// UploadState and ScanStatus are the new values of files.
	UploadState, ScanStatus string
	// MIMEType is the new files.mime_type.
	MIMEType string
	// SHA256 and SizeBytes are the computed digest and the counted size (equal to the declaration
	// for a complete file); they replace the declared values in files.
	SHA256    [32]byte
	SizeBytes int64
	// ScannedAt and ScanEngine are NULL when zero and "" (skipped and failed files).
	ScannedAt  time.Time
	ScanEngine string
	// FailureReason is file_upload_sessions.failure_reason; "" leaves it NULL.
	FailureReason string
}

// ScanRepository is the outgoing port of the scan pass to the tables of the module (03 4.7);
// adapters/postgres implements it. The scan job has no principal, so its reads are keyed by the
// file id only.
type ScanRepository interface {
	// ScanTarget reads the session of fileID with its file, also when the file is deleted
	// (FileRecord.Deleted). found is false when no session exists. Outside a unit of work it reads
	// from the pool.
	ScanTarget(ctx context.Context, fileID ids.ID) (SessionRecord, bool, error)
	// LockScanTarget reads as ScanTarget and locks the session row and then the files row FOR
	// UPDATE, the order of the completion and the abort of M3-03. It needs a unit of work.
	LockScanTarget(ctx context.Context, fileID ids.ID) (SessionRecord, bool, error)
	// FinishScan writes r to the file and its session. It needs a unit of work.
	FinishScan(ctx context.Context, fileID ids.ID, r ScanResult) error
	// CountAwaitingScan counts the files that await the pass (brief M3-04 design note 2) for
	// bothy.file.scan.queue; it uses the partial index on scan_status = 'pending'.
	CountAwaitingScan(ctx context.Context) (int64, error)
}

// ScanMetrics records the signals of the scan pass (docs/observability/signals.md 4.4);
// adapters/otelmetrics implements it.
type ScanMetrics interface {
	// ScanCompleted counts one final scan outcome (clean, infected, error, skipped) in
	// bothy.file.scan and records the duration of the whole pass in bothy.file.scan.duration.
	ScanCompleted(ctx context.Context, outcome string, d time.Duration)
	// ScanQueue records bothy.file.scan.queue, the number of files awaiting the pass.
	ScanQueue(ctx context.Context, n int64)
}

// noScanMetrics records nothing (unit tests, 02 5 "Layers within a module" rule 5).
type noScanMetrics struct{}

func (noScanMetrics) ScanCompleted(context.Context, string, time.Duration) {}
func (noScanMetrics) ScanQueue(context.Context, int64)                     {}
