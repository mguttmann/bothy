// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// Scratch is the scratch area of the restore test, the volume backup-scratch
// (docs/operations/backup-restore.md 4, brief M7-03): it restores the backup, runs the copy and
// fetches objects, and it never touches the production database. adapters/scratch implements
// it. The methods are called in the order Restore, Verify, Start, FetchObjects, Clear; Clear is
// always called last, also after a failure or a panic.
type Scratch interface {
	// Restore empties the scratch directory and restores the pgBackRest backup set label from
	// repo1 into it with recovery target immediate; pgBackRest checks the checksum of every file
	// it restores. The error is an *ExecError or wraps one.
	Restore(ctx context.Context, label string) error
	// Verify runs pgbackrest verify on repo1. The error is an *ExecError or wraps one.
	Verify(ctx context.Context) error
	// Start starts PostgreSQL on the restored directory, listening on a Unix socket in the
	// scratch directory only (listen_addresses empty, no TCP) and without WAL archiving, waits
	// until it accepts queries outside recovery and returns the copy.
	Start(ctx context.Context) (ScratchDB, error)
	// FetchObjects restores the objects of keys (files.object_key) from the latest object
	// backup into the scratch directory and returns the SHA-256 of each one it found; a key the
	// backup lacks is absent from the map.
	FetchObjects(ctx context.Context, keys []string) (map[string][sha256.Size]byte, error)
	// Clear stops the scratch PostgreSQL when it runs, closes the connections of the copy and
	// empties the scratch directory. It may be called more than once.
	Clear(ctx context.Context) error
}

// ScratchDB is the restored copy, read over its Unix socket as migrate.bootstrap_user. Every
// method only reads.
type ScratchDB interface {
	// Facts reads the facts of backup_manifest.v1 from the copy in one snapshot, with the
	// queries of ManifestSource.Snapshot, so the copy and its manifest are counted alike.
	Facts(ctx context.Context) (ManifestFacts, error)
	// EventLog reads count, smallest and largest seq of every partition of event_log, the rows
	// of sequence_counters and the largest seq of audit_events.
	EventLog(ctx context.Context) (EventLogFacts, error)
	// AuditChain returns the chain of the copy for audit.RowsVerifier: instance_id, the public
	// keys of server_signing_keys and iterators over audit_events and audit_checkpoints in seq
	// order.
	AuditChain(ctx context.Context) (audit.RowsInput, error)
	// ThreadSample returns at most n threads drawn at random from all threads, or all of them
	// when there are fewer, each with the count and bounds of thread_seq of its messages.
	ThreadSample(ctx context.Context, n int) ([]ThreadSeqs, error)
	// FileSample returns at most n files drawn at random from the files with deleted_at IS NULL,
	// upload_state complete and scan_status clean or skipped (the files that have an object), or
	// all of them when there are fewer.
	FileSample(ctx context.Context, n int) ([]SampleFile, error)
}

// EventLogFacts are the facts of check 3 (16 8.3 item 3).
type EventLogFacts struct {
	// Partitions are the partitions of event_log that hold rows, in seq order.
	Partitions []EventPartition
	// Counters are the rows of sequence_counters by name.
	Counters map[string]int64
	// AuditHead is max(audit_events.seq), 0 for an empty log.
	AuditHead int64
}

// EventPartition is one partition of event_log.
type EventPartition struct {
	// Name is the table name of the partition.
	Name string
	// Count, MinSeq and MaxSeq are count(*), min(seq) and max(seq) of its rows.
	Count, MinSeq, MaxSeq int64
}

// ThreadSeqs are the thread_seq facts of one thread (16 8.3 item 5).
type ThreadSeqs struct {
	// ThreadID is threads.id.
	ThreadID string
	// Messages, MinSeq and MaxSeq are count(*), min(thread_seq) and max(thread_seq) of its
	// messages (0 without messages).
	Messages, MinSeq, MaxSeq int64
	// LastMessageSeq is threads.last_message_seq, the counter of the thread.
	LastMessageSeq int64
}

// SampleFile is one file of the sample (16 8.3 item 6).
type SampleFile struct {
	// ID is files.id.
	ID string
	// ObjectKey is files.object_key, the key in the object store.
	ObjectKey string
	// SHA256 is files.sha256.
	SHA256 []byte
}

// RestoreConfig is the configuration of the restore test (16 14 backup.restore_test.* and
// dr.rto_target), bound by cmd/bothy-backupd.
type RestoreConfig struct {
	// Schedule is backup.restore_test.schedule; the scheduler starts the test only when
	// Scheduled is true.
	Schedule  domain.Schedule
	Scheduled bool
	// SampleThreads is backup.restore_test.sample_threads.
	SampleThreads int
	// SampleFiles is backup.restore_test.sample_files.
	SampleFiles int
	// RTOTarget is dr.rto_target, the bound of check 7.
	RTOTarget time.Duration
}
