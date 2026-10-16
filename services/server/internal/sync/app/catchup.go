// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// CatchUpConfig holds the limits of catch-up (04 section 7), bound in internal/wire.
type CatchUpConfig struct {
	// PageSize is sync.catchup_page_size: the default and the largest limit of a page.
	PageSize int `config:"sync.catchup_page_size"`
	// MaxEvents is sync.catchup_max_events: a larger backlog answers resync_required with
	// too_far_behind.
	MaxEvents int64 `config:"sync.catchup_max_events"`
}

// VisibleQuery is the page query of 04 section 7.2 point 1: the events with After < seq <= Head
// in seq order, at most Limit, whose channel, space or audience matches the visible sets of
// Authorizer.Scope, plus every event without channel, space and audience.
type VisibleQuery struct {
	After, Head int64
	Limit       int
	// Principal is the caller: its personal events match.
	Principal ids.ID
	// ChannelIDs, SpaceIDs and FullyVisibleSpaceIDs are the visible sets; InstanceWide matches
	// every event of a space.
	ChannelIDs, SpaceIDs, FullyVisibleSpaceIDs []ids.ID
	InstanceWide                               bool
}

// CatchUpLog is the outgoing port over event_log for catch-up (adapters/postgres).
type CatchUpLog interface {
	// MinRetainedSeq returns min(seq), 0 for an empty log.
	MinRetainedSeq(ctx context.Context) (int64, error)
	// VisibleEvents runs the page query q. The predicate is the performance; authorize decides.
	VisibleEvents(ctx context.Context, q VisibleQuery) ([]sync.Event, error)
}

// HeadSource reads the committed counter event_seq (instance.Versions).
type HeadSource interface {
	EventSeq(ctx context.Context) (int64, error)
}

// EpochSource reads the instance row with sync_epoch (instance.State).
type EpochSource interface {
	Load(ctx context.Context) (instance.Record, error)
}

// CatchUpDeps are the collaborators of catch-up.
type CatchUpDeps struct {
	Log        CatchUpLog
	Head       HeadSource
	Epoch      EpochSource
	Authorizer authorize.Authorizer
}

// CatchUpService implements sync.CatchUp (M1-63). It is safe for concurrent use.
type CatchUpService struct {
	cfg CatchUpConfig
	d   CatchUpDeps
}

// Compile-time check: the incoming port.
var _ sync.CatchUp = (*CatchUpService)(nil)

// NewCatchUp returns the use case. Errors: a missing collaborator, a page size or maximum below 1.
func NewCatchUp(cfg CatchUpConfig, d CatchUpDeps) (*CatchUpService, error) {
	if d.Log == nil || d.Head == nil || d.Epoch == nil || d.Authorizer == nil {
		return nil, errors.New("sync: NewCatchUp needs every collaborator")
	}
	if cfg.PageSize < 1 || cfg.MaxEvents < 1 {
		return nil, errors.New("sync: sync.catchup_page_size and sync.catchup_max_events must be at least 1")
	}
	return &CatchUpService{cfg: cfg, d: d}, nil
}

// logHead returns the head (the counter, never max(seq): after retention the log can be empty
// while the counter is not, docs/DECISIONS.md M1-63/1) and the sync epoch, read in this order.
func logHead(ctx context.Context, head HeadSource, epoch EpochSource) (int64, ids.ID, error) {
	h, err := head.EventSeq(ctx)
	if err != nil {
		return 0, ids.ID{}, err
	}
	rec, err := epoch.Load(ctx)
	if err != nil {
		return 0, ids.ID{}, err
	}
	return h, rec.SyncEpoch, nil
}

// Check implements sync.CatchUp: events.catch_up on the instance, then head, minimum and epoch
// and the decision of domain.ResyncReason with sync.catchup_max_events.
func (s *CatchUpService) Check(ctx context.Context, in sync.CheckInput) (sync.CheckResult, error) {
	a := caller(ctx)
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionEventsCatchUp, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return sync.CheckResult{}, err
	}
	if err := d.Err(); err != nil {
		return sync.CheckResult{}, err
	}
	head, epoch, err := logHead(ctx, s.d.Head, s.d.Epoch)
	if err != nil {
		return sync.CheckResult{}, err
	}
	minSeq, err := s.d.Log.MinRetainedSeq(ctx)
	if err != nil {
		return sync.CheckResult{}, err
	}
	if minSeq == 0 {
		minSeq = head + 1
	}
	r := domain.ResyncReason(in.After, in.Epoch, head, minSeq, epoch, s.cfg.MaxEvents)
	return sync.CheckResult{HeadSeq: head, MinRetainedSeq: minSeq, SyncEpoch: epoch, Reason: sync.ResyncReason(r)}, nil
}

// Page implements sync.CatchUp (04 section 7.2): the visible sets of Authorizer.Scope, the page
// query, then per row the audience rule or authorize with the receive action in filter mode, and
// the cursor of domain.NextCursor. It reads no content table per event.
func (s *CatchUpService) Page(ctx context.Context, in sync.PageInput) (sync.PageResult, error) {
	a := caller(ctx)
	limit := in.Limit
	if limit < 1 || limit > s.cfg.PageSize {
		limit = s.cfg.PageSize
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionChannelRead)
	if err != nil {
		return sync.PageResult{}, err
	}
	rows, err := s.d.Log.VisibleEvents(ctx, VisibleQuery{
		After: in.After, Head: in.Head, Limit: limit, Principal: a.ID, ChannelIDs: sc.ChannelIDs, SpaceIDs: sc.SpaceIDs,
		FullyVisibleSpaceIDs: sc.FullyVisibleSpaceIDs, InstanceWide: sc.InstanceWide,
	})
	if err != nil {
		return sync.PageResult{}, err
	}
	out := sync.PageResult{Events: make([]sync.EnvelopeJSON, 0, len(rows))}
	fctx := authorize.Filter(ctx)
	// trace_context only in deliveries to agents (04 "Umschlag"; docs/DECISIONS.md M1-63/5).
	fields := sync.DeliveryFields{IncludeTraceContext: a.Kind == actor.KindAgent}
	var last int64
	for _, ev := range rows {
		last = ev.Seq
		ok, err := admit(fctx, s.d.Authorizer, a, ev)
		if err != nil {
			return sync.PageResult{}, err
		}
		if !ok {
			continue
		}
		raw, err := json.Marshal(ev.Envelope(fields))
		if err != nil {
			return sync.PageResult{}, errs.Wrap(errs.CodeInternalError, err, "sync: encoding an envelope")
		}
		out.Events = append(out.Events, raw)
	}
	out.NextCursor = domain.NextCursor(in.Head, limit, len(rows), last)
	return out, nil
}

// admit is the delivery rule of one stored event for a (07 section 8.1): a personal event only to
// its audience, any other only when authorize allows its receive action on the event's resource at
// delivery time. ctx is in filter mode, so a refusal writes no audit entry.
func admit(ctx context.Context, az authorize.Authorizer, a actor.Actor, ev sync.Event) (bool, error) {
	if !ev.AudiencePrincipalID.IsZero() {
		return ev.AudiencePrincipalID == a.ID, nil
	}
	act, ok := authorize.ParseAction(ev.ReceiveAction)
	if !ok {
		// A receive action outside the catalogue of this build cannot be decided: fail closed.
		return false, nil
	}
	r := authorize.ResourceFromEvent(ev.ResourceKind, ev.ResourceID, ev.SpaceID, ev.ChannelID, ev.ThreadID)
	d, err := az.Authorize(ctx, a, act, r)
	if err != nil {
		return false, err
	}
	return d.Allowed(), nil
}
