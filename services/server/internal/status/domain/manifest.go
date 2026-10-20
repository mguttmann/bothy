// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// ManifestSchema is the schema name of the manifest (03 4.12, backup_runs.manifest).
const ManifestSchema = "backup_manifest.v1"

// Manifest is backup_manifest.v1: the fields of docs/spec/03-datenmodell.md 4.12 and the
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
	// RowCounts is count(*) per table of the domain schema, keyed by table name.
	RowCounts map[string]int64 `json:"row_counts"`
	// Files are the objects the table files names.
	Files FileTotals `json:"files"`
	// PgBackRest refers to the pgBackRest backup; set exactly for pg_full and pg_diff.
	PgBackRest *PgBackRestRef `json:"pgbackrest,omitempty"`
	// Restic refers to the restic snapshot; required for logical, set for objects with the
	// driver fs, absent otherwise.
	Restic *ResticRef `json:"restic,omitempty"`
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
