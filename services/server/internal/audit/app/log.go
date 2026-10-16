// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// LogDeps are the collaborators of Log.
type LogDeps struct {
	// Store is the storage of the module.
	Store Store
	// UoW is the unit of work of the checkpoint insert.
	UoW txn.UnitOfWork
	// Authorizer decides audit.read and audit.verify (owner with fresh step-up).
	Authorizer authorize.Authorizer
	// Audit records the allow entries of the reads and the error entry of a broken chain.
	Audit auditing.Recorder
	// Keys signs checkpoints with purpose audit_checkpoint and verifies those of the sink.
	Keys instance.Keys
	// Sink is audit.checkpoint_sink; NoSink for the empty value.
	Sink CheckpointSink
	// Cursors encodes the cursors of List.
	Cursors Cursors
	// Metrics receives the instruments of the job.
	Metrics Metrics
	// Reporter receives the reports of the module.
	Reporter Reporter
	// Clock is the source of exported_at and of the last success; nil means clock.System().
	Clock clock.Clock
	// InstanceID is instance.instance_id, the instance_id of every checkpoint.
	InstanceID ids.ID
}

// Log implements audit.Log (listAuditEvents, exportAuditEvents, verifyAuditChain) and runs the
// job audit.chain_verify (RunCheckpoint). It is safe for concurrent use.
type Log struct {
	cfg Config
	d   LogDeps
}

// Compile-time check: Log is the port.
var _ audit.Log = (*Log)(nil)

// NewLog returns the log over d. Errors: a missing collaborator, a zero instance ID, or page
// sizes that are not positive with the default not above the maximum.
func NewLog(cfg Config, d LogDeps) (*Log, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Keys == nil || d.Sink == nil ||
		d.Cursors == nil || d.Metrics == nil || d.Reporter == nil || d.InstanceID.IsZero() {
		return nil, errors.New("audit: the log needs a store, a unit of work, an authorizer, a recorder, the keys, a sink, cursors, metrics, a reporter and the instance ID")
	}
	if cfg.PageSizeDefault < 1 || cfg.PageSizeMax < cfg.PageSizeDefault {
		return nil, errors.New("audit: api.page_size_default must be at least 1 and not above api.page_size_max")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Log{cfg: cfg, d: d}, nil
}

// check authorizes act on the instance for the principal of ctx (anonymous when there is none)
// and returns that principal.
func (l *Log) check(ctx context.Context, act authorize.Action) error {
	a, ok := actor.FromContext(ctx)
	if !ok {
		a = actor.Anonymous()
	}
	d, err := l.d.Authorizer.Authorize(ctx, a, act, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return err
	}
	return d.Err()
}

// record writes an allow entry of act with attrs at once, in a unit of work of its own: a read
// has no transaction, and the entry must exist even when the response fails later.
func (l *Log) record(ctx context.Context, act authorize.Action, attrs auditing.Attrs) error {
	return l.d.Audit.RecordDetached(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	})
}

// event converts a stored row into the port type.
func event(r domain.Row) audit.Event {
	return audit.Event{
		Seq: r.Seq, ID: r.ID, OccurredAt: r.OccurredAt.UTC(), ActorPrincipalID: r.ActorPrincipalID,
		ActorDeviceID: r.ActorDeviceID, ActorSessionID: r.ActorSessionID, Action: r.Action, Decision: r.Decision,
		DenyReason: r.DenyReason, ResourceKind: r.ResourceKind, ResourceID: r.ResourceID, Details: r.Details,
		CorrelationID: r.CorrelationID, PrevHash: r.PrevHash, Hash: r.Hash,
	}
}

// RowOf converts an event of the port back into the row of the domain, for the canonical form
// of an export line.
func RowOf(e audit.Event) domain.Row {
	return domain.Row{
		Seq: e.Seq, ID: e.ID, OccurredAt: e.OccurredAt, ActorPrincipalID: e.ActorPrincipalID,
		ActorDeviceID: e.ActorDeviceID, ActorSessionID: e.ActorSessionID, Action: e.Action, Decision: e.Decision,
		DenyReason: e.DenyReason, ResourceKind: e.ResourceKind, ResourceID: e.ResourceID, Details: e.Details,
		CorrelationID: e.CorrelationID, PrevHash: e.PrevHash, Hash: e.Hash,
	}
}

// now returns the time of the clock.
func (l *Log) now() time.Time { return l.d.Clock.Now() }
