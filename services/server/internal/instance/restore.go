// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The audit actions of bothy restore finalize (docs/DECISIONS.md D-M7-04-2): system actions of
// the host, written with no actor and no content.
const (
	// ActionRestoreFinalize records a finalized restore: a new sync epoch, every cursor at
	// head_seq and the new database identity.
	ActionRestoreFinalize = "instance.restore.finalize"
	// ActionRelocationFinalize records a finalized relocation: the new database identity with
	// the epoch and the cursors unchanged.
	ActionRelocationFinalize = "instance.restore.finalize_relocation"
)

// DatabaseIdentity is the identity of the database cluster (R-DM-50, docs/spec/03-datenmodell.md
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

// Restore is the port of bothy restore finalize (R-DM-50, 03 6.7, docs/spec/16-betrieb.md 8.4);
// the start lock reads State.CheckIdentity. internal/wire builds it for the host command. It is
// safe for concurrent use.
type Restore interface {
	// Finalize runs bothy restore finalize in one unit of work: it refuses while another session
	// of bothy_app is connected (ErrServerConnected), locks the instance row, reads head_seq and
	// the current identity; after a restore (cmd.Relocation nil) it sets a new UUIDv7 epoch and
	// every agent, webhook and worker cursor to head_seq; after a relocation it compares the
	// manifest first (RelocationMismatchError) and keeps epoch and cursors. Then it stores the
	// identity and records the audit entry. Any error rolls everything back.
	//
	// Example:
	//
	//	res, err := restore.Finalize(ctx, instance.FinalizeRestore{})
	Finalize(ctx context.Context, cmd FinalizeRestore) (FinalizeResult, error)
}
