// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// CursorMover is the outgoing port of bothy restore finalize to the cursors of another module:
// agents, apps and sync each implement it with their port RestoreCursors, and internal/wire
// passes them in. instance declares it itself, because the other modules depend on instance and
// not the other way round (02 4.5).
type CursorMover interface {
	// MoveCursorsTo sets every cursor of the module to headSeq in the unit of work of ctx.
	MoveCursorsTo(ctx context.Context, headSeq int64) error
}

// AuditHeadSource is the outgoing port to the head of the audit chain, which bothy restore
// finalize --relocation compares with the manifest (module audit, adapted in internal/wire).
type AuditHeadSource interface {
	// AuditHead returns the seq and the hash (lowercase hex) of the last row of audit_events; 0
	// and "" for an empty chain.
	AuditHead(ctx context.Context) (seq int64, hash string, err error)
}

// RestoreDeps are the collaborators of the restore use cases.
type RestoreDeps struct {
	// Repo reads and writes the instance row, the counters and the session view.
	Repo RestoreRepository
	// UoW is the unit of work of the process; its registry must hold the audit hook, so Audit
	// writes its entry in the transaction of the change.
	UoW txn.UnitOfWork
	// Audit records the entry of the finalize.
	Audit auditing.Recorder
	// Cursors move the cursors of agents, apps and sync, in this order (03 6.7 item 1).
	Cursors []CursorMover
	// AuditHead reads the head of the audit chain for a relocation.
	AuditHead AuditHeadSource
	// ServerRole is the database role of api and worker (bothy_app); a session of it refuses
	// the finalize.
	ServerRole string
}

// restore implements instance.Restore.
type restore struct {
	d RestoreDeps
}

// Compile-time check: restore is the port.
var _ instance.Restore = (*restore)(nil)

// NewRestore returns the start lock and bothy restore finalize over d (M7-04). Errors: a missing
// collaborator or server role.
func NewRestore(d RestoreDeps) (instance.Restore, error) {
	if d.Repo == nil || d.UoW == nil || d.Audit == nil || d.AuditHead == nil || d.ServerRole == "" {
		return nil, errors.New("instance: NewRestore needs the repository, the unit of work, the recorder, the audit head and the server role")
	}
	for _, c := range d.Cursors {
		if c == nil {
			return nil, errors.New("instance: NewRestore got a nil cursor mover")
		}
	}
	return &restore{d: d}, nil
}

// Finalize implements instance.Restore: everything in one unit of work, so an error anywhere,
// also in the last cursor port or in the audit hook, leaves epoch, cursors, identity and audit
// as they were (brief M7-04 design note 2).
func (r *restore) Finalize(ctx context.Context, cmd instance.FinalizeRestore) (instance.FinalizeResult, error) {
	var res instance.FinalizeResult
	err := r.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res, err = r.finalize(ctx, cmd)
		return err
	})
	if err != nil {
		return instance.FinalizeResult{}, err
	}
	return res, nil
}

// finalize is the body of Finalize inside its unit of work.
func (r *restore) finalize(ctx context.Context, cmd instance.FinalizeRestore) (instance.FinalizeResult, error) {
	rec, head, current, err := r.lock(ctx)
	if err != nil {
		return instance.FinalizeResult{}, err
	}
	res := instance.FinalizeResult{PreviousSyncEpoch: rec.SyncEpoch, SyncEpoch: rec.SyncEpoch, HeadSeq: head, Identity: current}
	action := instance.ActionRestoreFinalize
	if cmd.Relocation != nil {
		if err := r.checkRelocation(ctx, *cmd.Relocation, rec.SyncEpoch, head); err != nil {
			return instance.FinalizeResult{}, err
		}
		res.Relocation, action = true, instance.ActionRelocationFinalize
	} else {
		res.SyncEpoch = ids.New()
		for _, c := range r.d.Cursors {
			if err := c.MoveCursorsTo(ctx, head); err != nil {
				return instance.FinalizeResult{}, err
			}
		}
	}
	if err := r.d.Repo.FinalizeInstance(ctx, res.SyncEpoch, current); err != nil {
		return instance.FinalizeResult{}, err
	}
	err = r.d.Audit.Record(ctx, auditing.Entry{
		Action: action, Decision: auditing.DecisionAllow, ResourceKind: "instance", ResourceID: rec.InstanceID,
		Attrs: auditing.Attrs{
			"head_seq":             auditing.Int(head),
			"previous_sync_epoch":  auditing.ID(res.PreviousSyncEpoch),
			"sync_epoch":           auditing.ID(res.SyncEpoch),
			"pg_system_identifier": auditing.Code(current.SystemIdentifier),
			"pg_timeline_id":       auditing.Int(current.TimelineID),
		},
	})
	if err != nil {
		return instance.FinalizeResult{}, err
	}
	return res, nil
}

// lock refuses while another session of the server role is connected (ErrServerConnected), then
// locks the instance row and reads head_seq and the current identity, in the unit of work of ctx.
func (r *restore) lock(ctx context.Context) (instance.Record, int64, instance.DatabaseIdentity, error) {
	n, err := r.d.Repo.OtherSessionsOfRole(ctx, r.d.ServerRole)
	if err != nil {
		return instance.Record{}, 0, instance.DatabaseIdentity{}, err
	}
	if n > 0 {
		return instance.Record{}, 0, instance.DatabaseIdentity{}, instance.ErrServerConnected
	}
	rec, found, err := r.d.Repo.LockInstance(ctx)
	if err != nil {
		return instance.Record{}, 0, instance.DatabaseIdentity{}, err
	}
	if !found {
		return instance.Record{}, 0, instance.DatabaseIdentity{}, errs.New(errs.CodeNotFound, `the instance row is missing; run "bothy migrate up"`)
	}
	head, err := r.d.Repo.CounterValue(ctx, string(instance.CounterEventSeq))
	if err != nil {
		return instance.Record{}, 0, instance.DatabaseIdentity{}, err
	}
	current, err := r.d.Repo.ClusterIdentity(ctx)
	if err != nil {
		return instance.Record{}, 0, instance.DatabaseIdentity{}, err
	}
	return rec, head, current, nil
}

// checkRelocation compares the manifest of the final backup with the restored database: the
// head of the event log, the audit head and the epoch (design note 3). A difference is a
// RelocationMismatchError naming the first differing field.
func (r *restore) checkRelocation(ctx context.Context, m instance.RelocationManifest, epoch ids.ID, head int64) error {
	seq, hash, err := r.d.AuditHead.AuditHead(ctx)
	if err != nil {
		return err
	}
	field, ok := domain.RelocationDifference(
		domain.RelocationFacts{HeadSeq: m.HeadSeq, AuditLastSeq: m.AuditLastSeq, AuditLastHash: m.AuditLastHash, SyncEpoch: m.SyncEpoch.String()},
		domain.RelocationFacts{HeadSeq: head, AuditLastSeq: seq, AuditLastHash: hash, SyncEpoch: epoch.String()},
	)
	if !ok {
		return &instance.RelocationMismatchError{Field: field}
	}
	return nil
}

// toDomainIdentity converts the port type to the domain type.
func toDomainIdentity(id instance.DatabaseIdentity) domain.Identity {
	return domain.Identity{SystemIdentifier: id.SystemIdentifier, TimelineID: id.TimelineID}
}
