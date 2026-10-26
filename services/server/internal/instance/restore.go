// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The audit actions of bothy restore finalize (docs/decisions.d/M7-04.md D-M7-04-2): system actions of
// the host, written with no actor and no content.
const (
	// ActionRestoreFinalize records a finalized restore: a new sync epoch, every cursor at
	// head_seq and the new database identity.
	ActionRestoreFinalize = "instance.restore.finalize"
	// ActionRelocationFinalize records a finalized relocation: the new database identity with
	// the epoch and the cursors unchanged.
	ActionRelocationFinalize = "instance.restore.finalize_relocation"
)

// DatabaseIdentity is the identity of the database cluster (R-DM-50, docs/spec/03-data-model.md
// 4.1): system_identifier of pg_control_system() as decimal text and timeline_id of
// pg_control_checkpoint(). The zero value means none is stored.
type DatabaseIdentity struct {
	// SystemIdentifier is the decimal text of the unsigned 64-bit system identifier.
	SystemIdentifier string
	// TimelineID is the timeline of the last checkpoint.
	TimelineID int64
}

// IdentityCheck is the comparison of the start lock: the stored and the current identity and
// whether they mismatch (an empty stored identity never does).
type IdentityCheck struct {
	Stored, Current DatabaseIdentity
	Mismatch        bool
}

// RelocationManifest are the facts of the manifest of the final backup that bothy restore
// finalize --relocation compares with the restored database (backup_manifest.v1:
// event_log.head_seq, audit.last_seq, audit.last_hash, sync_epoch).
type RelocationManifest struct {
	HeadSeq int64
	// AuditLastSeq and AuditLastHash are the head of the audit chain; the hash is lowercase hex,
	// "" with seq 0 for an empty chain.
	AuditLastSeq  int64
	AuditLastHash string
	SyncEpoch     ids.ID
}

// FinalizeRestore is the input of bothy restore finalize: Relocation is nil after a restore
// (new epoch, cursors to head_seq) and the manifest of the final backup after a relocation
// without data loss (epoch and cursors kept).
type FinalizeRestore struct {
	Relocation *RelocationManifest
}

// FinalizeResult is the outcome of a finalize.
type FinalizeResult struct {
	// Relocation reports a finalized relocation: SyncEpoch equals PreviousSyncEpoch and no
	// cursor moved.
	Relocation bool
	// PreviousSyncEpoch and SyncEpoch are the epoch before and after.
	PreviousSyncEpoch, SyncEpoch ids.ID
	// HeadSeq is the head of the event log the cursors were set to (or kept at, relocation).
	HeadSeq int64
	// Identity is the stored database identity afterwards.
	Identity DatabaseIdentity
	// Journal is what the application of the deletion journal did (M7-07).
	Journal JournalApplied
	// PermissionVersion is permission_version after the finalize raised it (SEC-BAK-3).
	PermissionVersion int64
}

// JournalApplied is what bothy restore finalize did with the deletion journal of
// docs/spec/20-privacy.md section 4.6 (SEC-BAK-3, M7-07): counts per kind, never IDs.
type JournalApplied struct {
	// Entries is the number of journal objects read from the restored next_number on.
	Entries int
	// Messages, Files and Accounts count the resources deleted again.
	Messages, Files, Accounts int
	// Skipped counts entries whose resource is missing or deleted in the restored database.
	Skipped int
	// Refused counts accounts the deletion refused, because the person is the last active owner
	// of the restored database.
	Refused int
	// Next is the number after the last object found, stored as
	// deletion_journal_state.next_number.
	Next int64
}

// ErrServerConnected is the refusal of bothy restore finalize while another session of the role
// bothy_app is connected to the database (brief M7-04 requirement 16): a running api or worker
// could see half of the change. Stop them first. Kind conflict; compare with errors.Is.
var ErrServerConnected = errs.New(errs.CodeConflict, "a server process (role bothy_app) is connected to the database; stop api and worker first")

// RelocationMismatchError is the refusal of bothy restore finalize --relocation when a fact of
// the restored database differs from the manifest; nothing changed. Field names the first
// differing manifest field.
type RelocationMismatchError struct {
	Field string
}

// Error names the differing field and the way out.
func (e *RelocationMismatchError) Error() string {
	return "the restored database differs from the manifest in " + e.Field +
		`; nothing changed. If data was lost, run "bothy restore finalize" without --relocation`
}

// IsRelocationMismatch reports whether err is a RelocationMismatchError and returns it.
func IsRelocationMismatch(err error) (*RelocationMismatchError, bool) {
	var m *RelocationMismatchError
	ok := errors.As(err, &m)
	return m, ok
}

// Restore is the port of bothy restore finalize (R-DM-50, 03 6.7, docs/spec/16-operations.md 8.4);
// the start lock reads State.CheckIdentity. internal/wire builds it for the host command. It is
// safe for concurrent use.
type Restore interface {
	// Finalize runs bothy restore finalize: it refuses while another session of bothy_app is
	// connected (ErrServerConnected) and applies the deletion journal (SEC-BAK-3, M7-07): every
	// journal entry whose resource the restored database holds is deleted again through the
	// deletion use cases, each in its own units of work, so an error stops the command before the
	// epoch step and a second run continues where it stopped. Then, in one unit of work, it checks
	// the connected sessions again, locks the instance row, reads head_seq (after the deletions,
	// so no consumer replays them) and the current identity; after a restore (cmd.Relocation nil)
	// it sets a new UUIDv7 epoch and every agent, webhook and worker cursor to head_seq; after a
	// relocation it compares the manifest first (RelocationMismatchError) and keeps epoch and
	// cursors. It raises permission_version once, stores the number after the last journal
	// object as deletion_journal_state.next_number, stores the identity and records the audit
	// entry. Any error in the unit rolls the unit back.
	//
	// Example:
	//
	//	res, err := restore.Finalize(ctx, instance.FinalizeRestore{})
	Finalize(ctx context.Context, cmd FinalizeRestore) (FinalizeResult, error)
}
