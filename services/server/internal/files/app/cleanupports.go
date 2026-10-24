// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PurgeQueue inserts the job files.purge_objects (02 12.3 "Job catalogue", M3-08); adapters/jobs
// implements it over jobs.Inserter.InsertTx. It must be called inside a unit of work, so the job
// exists exactly when the deletion or failure that needs it commits (02 8.5).
type PurgeQueue interface {
	// Enqueue inserts files.purge_objects for fileID to run at once; the arguments carry only the
	// id (02 12.1).
	Enqueue(ctx context.Context, fileID ids.ID) error
}

// TempSweeper removes the temporary files a crashed process left in the store (12 4.2; M3-08):
// the file system driver implements it over <file.fs.root>/.tmp, the S3 driver has none. It never
// lists the key space (12 4.1).
type TempSweeper interface {
	// SweepTemp removes the entries of the temporary directory whose modification time is before
	// olderThan and returns how many it removed.
	SweepTemp(ctx context.Context, olderThan time.Time) (int, error)
}

// Candidate is one row a cleanup step reads: the file and the time of its page key.
type Candidate struct {
	FileID ids.ID
	At     time.Time
}

// PurgeRecord is what files.purge_objects reads of a file without a transaction (brief M3-08
// design note 4 c): the facts of the plan, the stored original key and the upload session.
type PurgeRecord struct {
	Facts      domain.PurgeFacts
	ObjectKey  string
	UploadedBy ids.ID
	// HasSession reports a row of file_upload_sessions; PartCount and StorageUploadID are its
	// columns ("" for NULL).
	HasSession      bool
	PartCount       int
	StorageUploadID string
}

// CleanupRecord is what the cleanup and deleteFile read of one file after its lock, in a new
// statement (design notes 6 and 9 e).
type CleanupRecord struct {
	UploadedBy              ids.ID
	Purpose                 string
	UploadState, ScanStatus string
	Deleted                 bool
	Attached                bool
	Ages                    domain.CleanupFacts
}

// CleanupStore is the outgoing port of the purge, the cleanup and deleteFile to the tables of the
// module (M3-08); adapters/postgres implements it. The jobs have no principal, so their reads are
// keyed by a file id or by state and time (the exceptions of AZ-P12). The candidate reads return
// at most limit rows after the page key after, ordered by (time, id).
type CleanupStore interface {
	// ExpiredOpenSessions returns open sessions with expires_at at or before now of files that
	// are not deleted, ordered by expires_at.
	ExpiredOpenSessions(ctx context.Context, now time.Time, after Candidate, limit int) ([]Candidate, error)
	// UnattachedCandidates returns deliverable attachment files that are not deleted, not attached
	// and created at or before cutoff, ordered by created_at.
	UnattachedCandidates(ctx context.Context, cutoff time.Time, after Candidate, limit int) ([]Candidate, error)
	// FailedRowCandidates returns failed or blocked attachment files that are not deleted, not
	// attached and created at or before cutoff, ordered by created_at.
	FailedRowCandidates(ctx context.Context, cutoff time.Time, after Candidate, limit int) ([]Candidate, error)
	// DeletedRows returns the files marked deleted, ordered by deleted_at.
	DeletedRows(ctx context.Context, after Candidate, limit int) ([]Candidate, error)
	// PurgeBacklog counts the files marked deleted and returns the oldest mark (zero without one).
	PurgeBacklog(ctx context.Context) (int64, time.Time, error)
	// PurgeRecord reads the purge facts of fileID, deleted or not; found is false without a row.
	PurgeRecord(ctx context.Context, fileID ids.ID) (PurgeRecord, bool, error)
	// LockFile locks the files row of fileID FOR UPDATE, deleted or not; found is false without a
	// row. It needs a unit of work.
	LockFile(ctx context.Context, fileID ids.ID) (bool, error)
	// LockFileForDelete locks the files row of fileID FOR UPDATE after authorize of file.delete,
	// deleted or not; found is false without a row. It needs a unit of work.
	LockFileForDelete(ctx context.Context, fileID ids.ID) (bool, error)
	// CleanupRecord reads the facts of fileID in a new statement after LockFile or
	// LockFileForDelete.
	CleanupRecord(ctx context.Context, fileID ids.ID) (CleanupRecord, bool, error)
	// FileForDelete reads the row of fileID by id, deleted or not, with what fileResource needs.
	FileForDelete(ctx context.Context, fileID ids.ID) (FileRecord, bool, error)
	// ExpireSession sets the open session of fileID expired, its file failed and deletes its part
	// rows. It needs a unit of work.
	ExpireSession(ctx context.Context, fileID ids.ID) error
	// DeleteFileRow deletes the row of a deleted file and reports whether it did. It needs a
	// unit of work.
	DeleteFileRow(ctx context.Context, fileID ids.ID) (bool, error)
}

// PurgeMetrics records bothy.file.purge.backlog (signals.md 4.4); adapters/otelmetrics implements
// it. No metric carries a file id (signals.md 5.3).
type PurgeMetrics interface {
	// PurgeBacklog records the number of files marked deleted at the end of a cleanup run.
	PurgeBacklog(ctx context.Context, n int64)
}

// noPurgeMetrics records nothing.
type noPurgeMetrics struct{}

func (noPurgeMetrics) PurgeBacklog(context.Context, int64) {
	// Without a meter provider nothing is recorded.
}
