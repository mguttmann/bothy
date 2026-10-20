// SPDX-License-Identifier: Apache-2.0

package domain

// Identity is the identity of a database cluster as instance stores it (R-DM-50):
// system_identifier of pg_control_system() as decimal text and timeline_id of
// pg_control_checkpoint(). A point-in-time restore keeps the system identifier and raises the
// timeline; a logical restore into a fresh cluster changes the system identifier.
type Identity struct {
	// SystemIdentifier is the decimal text of the unsigned 64-bit system identifier; "" when
	// nothing is stored.
	SystemIdentifier string
	// TimelineID is the timeline of the last checkpoint.
	TimelineID int64
}

// Empty reports whether no identity is stored (an instance migrated before M7-04, until the next
// bothy migrate up fills it).
func (i Identity) Empty() bool { return i.SystemIdentifier == "" }

// IdentityMismatch reports whether the current identity of the database differs from the stored
// one, the start lock of R-DM-50: true keeps the server not ready until bothy restore finalize
// has run. An empty stored identity never blocks, because bothy migrate up fills it at the next
// run (brief M7-04 design note 3).
func IdentityMismatch(stored, current Identity) bool {
	if stored.Empty() {
		return false
	}
	return stored != current
}

// The fields RelocationDifference names, in the order it compares them: the names of the
// manifest backup_manifest.v1 (docs/operations/backup-restore.md section 3).
const (
	FieldHeadSeq       = "event_log.head_seq"
	FieldAuditLastSeq  = "audit.last_seq"
	FieldAuditLastHash = "audit.last_hash"
	FieldSyncEpoch     = "sync_epoch"
)

// RelocationFacts are the facts of a relocation check: the head of the event log, the head of the
// audit chain (seq and hash as lowercase hex, 0 and "" for an empty chain) and the sync epoch as
// UUID text.
type RelocationFacts struct {
	HeadSeq       int64
	AuditLastSeq  int64
	AuditLastHash string
	SyncEpoch     string
}

// RelocationDifference compares the facts of the manifest of the final backup with those of the
// restored database (bothy restore finalize --relocation, design note 3). It returns "" and true
// when all are equal, else the name of the first differing field and false; the command then
// changes nothing.
func RelocationDifference(manifest, restored RelocationFacts) (string, bool) {
	switch {
	case manifest.HeadSeq != restored.HeadSeq:
		return FieldHeadSeq, false
	case manifest.AuditLastSeq != restored.AuditLastSeq:
		return FieldAuditLastSeq, false
	case manifest.AuditLastHash != restored.AuditLastHash:
		return FieldAuditLastHash, false
	case manifest.SyncEpoch != restored.SyncEpoch:
		return FieldSyncEpoch, false
	}
	return "", true
}
