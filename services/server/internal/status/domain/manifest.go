// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"maps"
	"slices"
	"time"
)

// ManifestSchema is the schema name of the manifest bothy-backupd writes (03 4.12,
// backup_runs.manifest): backup_manifest.v2 names every table in tables and counts exactly only
// ExactCountTables in row_counts (AUD srv-stats-status-19).
const ManifestSchema = "backup_manifest.v2"

// ManifestSchemaV1 is the schema name of the manifests written before backup_manifest.v2: its
// row_counts holds an exact count of every table and it has no tables. ParseManifest still reads
// it, so the backups of earlier runs stay restorable and checkable.
const ManifestSchemaV1 = "backup_manifest.v1"

// ExactCountTables are the tables whose exact row count the manifest holds: the restore test
// compares only these at equal heads, because every row of them carries the number of its log
// (docs/decisions.d/M7-03.md D-M7-03-3). Counting the other tables exactly held a snapshot over
// every table of a large instance after each run for numbers nobody read.
var ExactCountTables = []string{"audit_events", "event_log"}

// Manifest is backup_manifest.v2 (and the older backup_manifest.v1): the fields of docs/spec/03-data-model.md 4.12 and the
// references of docs/operations/backup-restore.md 3. It describes a backup, never its content:
// every string has a closed form (ids, versions, labels, hashes, table names), so a field with
// message text fails Validate.
type Manifest struct {
	// Schema is ManifestSchema.
	Schema string `json:"schema"`
	// BackupID is the id of the backup_runs row, a UUID.
	BackupID string `json:"backup_id"`
	// Kind is the backup kind.
	Kind Kind `json:"kind"`
	// StartedAt and FinishedAt bound the run, in UTC.
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	// ServerVersion is the version of bothy-backupd (the server version).
	ServerVersion string `json:"server_version"`
	// SchemaVersion is instance.schema_version.
	SchemaVersion int64 `json:"schema_version"`
	// InstanceID and SyncEpoch are instance.instance_id and instance.sync_epoch, UUIDs.
	InstanceID string `json:"instance_id"`
	SyncEpoch  string `json:"sync_epoch"`
	// EventLog holds the bounds of event_log.
	EventLog EventLogBounds `json:"event_log"`
	// Audit is the head of the audit chain.
	Audit AuditHead `json:"audit"`
	// RowCounts is count(*) per table, keyed by table name: in backup_manifest.v2 exactly the
	// tables of ExactCountTables, in backup_manifest.v1 every table of the domain schema.
	RowCounts map[string]int64 `json:"row_counts"`
	// Tables are the names of every table of the domain schema, sorted (backup_manifest.v2),
	// always written, also when empty; absent or null in backup_manifest.v1, whose row_counts
	// names them.
	Tables []string `json:"tables"`
	// Files are the objects the table files names.
	Files FileTotals `json:"files"`
	// PgBackRest refers to the pgBackRest backup; set exactly for pg_full and pg_diff.
	PgBackRest *PgBackRestRef `json:"pgbackrest,omitempty"`
	// Restic refers to the restic snapshot; required for logical, set for objects with the
	// driver fs, absent otherwise.
	Restic *ResticRef `json:"restic,omitempty"`
	// DeletionJournal is the position of the deletion journal at the run (M7-07): bothy-backupd
	// sets it for every kind; a manifest of a run before M7-07 has none.
	DeletionJournal *DeletionJournalRef `json:"deletion_journal,omitempty"`
}

// DeletionJournalRef names the position of the deletion journal of docs/spec/20-privacy.md
// section 4.6: NextNumber is deletion_journal_state.next_number in the snapshot of the manifest,
// the number after the newest stored journal object. A restore of this backup applies the journal
// from there, so the retention of the journal keeps every object from it on while the backup is
// kept (03 4.12).
type DeletionJournalRef struct {
	NextNumber int64 `json:"next_number"`
}

// EventLogBounds are head_seq and min_retained_seq of event_log (0 when the log is empty).
type EventLogBounds struct {
	HeadSeq        int64 `json:"head_seq"`
	MinRetainedSeq int64 `json:"min_retained_seq"`
}

// AuditHead is the last row of audit_events: its seq and its hash as lowercase hex; seq 0 and
// an empty hash when the log is empty.
type AuditHead struct {
	LastSeq  int64  `json:"last_seq"`
	LastHash string `json:"last_hash"`
}

// FileTotals are the count and the sum of size_bytes of the files rows with deleted_at IS NULL.
type FileTotals struct {
	Count      int64 `json:"count"`
	TotalBytes int64 `json:"total_bytes"`
}

// PgBackRestRef names the pgBackRest backup set, for example 20261019-020000F.
type PgBackRestRef struct {
	Label string `json:"label"`
}

// ResticRef names the restic snapshot by its full id (64 lowercase hex characters).
type ResticRef struct {
	SnapshotID string `json:"snapshot_id"`
}

// TableNames returns the names of every table of the domain schema the manifest describes,
// sorted: Tables for backup_manifest.v2, the keys of RowCounts for backup_manifest.v1.
func (m Manifest) TableNames() []string {
	if m.Schema == ManifestSchemaV1 {
		return slices.Sorted(maps.Keys(m.RowCounts))
	}
	return m.Tables
}
