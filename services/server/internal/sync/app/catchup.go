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

// Page implements sync.CatchUp (04 section 7.2): PageEvents, then the envelope of every admitted
// event. It reads no content table per event.
func (s *CatchUpService) Page(ctx context.Context, in sync.PageInput) (sync.PageResult, error) {
	page, err := s.PageEvents(ctx, in)
	if err != nil {
		return sync.PageResult{}, err
	}
	out := sync.PageResult{Events: make([]sync.EnvelopeJSON, 0, len(page.Events)), NextCursor: page.NextCursor}
	// trace_context only in deliveries to agents (04 "Umschlag"; docs/DECISIONS.md M1-63/5).
	fields := sync.DeliveryFields{IncludeTraceContext: caller(ctx).Kind == actor.KindAgent}
	for _, ev := range page.Events {
		raw, err := json.Marshal(ev.Envelope(fields))
		if err != nil {
			return sync.PageResult{}, errs.Wrap(errs.CodeInternalError, err, "sync: encoding an envelope")
		}
		out.Events = append(out.Events, raw)
	}
	return out, nil
}

// PageEvents implements sync.CatchUp: the visible sets of Authorizer.Scope, the page query, then
// per row the audience rule or authorize with the receive action in filter mode, and the cursor
// of domain.NextCursor.
func (s *CatchUpService) PageEvents(ctx context.Context, in sync.PageInput) (sync.EventPage, error) {
	a := caller(ctx)
	limit := in.Limit
	if limit < 1 || limit > s.cfg.PageSize {
		limit = s.cfg.PageSize
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionChannelRead)
	if err != nil {
		return sync.EventPage{}, err
	}
	rows, err := s.d.Log.VisibleEvents(ctx, VisibleQuery{
		After: in.After, Head: in.Head, Limit: limit, Principal: a.ID, ChannelIDs: sc.ChannelIDs, SpaceIDs: sc.SpaceIDs,
		FullyVisibleSpaceIDs: sc.FullyVisibleSpaceIDs, InstanceWide: sc.InstanceWide,
	})
	if err != nil {
		return sync.EventPage{}, err
	}
	out := sync.EventPage{Events: make([]sync.Event, 0, len(rows))}
	// One structure scope per page: the checks read the thread and channel of the page's events
	// once each instead of once per event (M1-72, perf.catchup_page_p99).
	fctx := authorize.Filter(authorize.WithStructureScope(ctx))
	var last int64
	for _, ev := range rows {
		last = ev.Seq
		ok, _, err := admit(fctx, s.d.Authorizer, a, ev, in.AgentConsumer)
		if err != nil {
			return sync.EventPage{}, err
		}
		if ok {
			out.Events = append(out.Events, ev)
		}
	}
	out.NextCursor = domain.NextCursor(in.Head, limit, len(rows), last)
	return out, nil
}

// Admit implements sync.CatchUp: admit in filter mode; an unauthenticated decision is its error.
// An own personal event, which needs no receive action, is checked against the credential with
// gateway.connect on the instance, so a revoked credential (for example an API token whose
// revocation changed no visible target and wrote no access.changed) gets the unauthenticated error
// instead of its events; any other answer of that check leaves the event admitted.
func (s *CatchUpService) Admit(ctx context.Context, a actor.Actor, ev sync.Event) (bool, error) {
	fctx := authorize.Filter(ctx)
	ok, d, err := admit(fctx, s.d.Authorizer, a, ev, false)
	if err != nil {
		return false, err
	}
	if ok && !ev.AudiencePrincipalID.IsZero() {
		if d, err = s.d.Authorizer.Authorize(fctx, a, authorize.ActionGatewayConnect, authorize.Resource{Kind: authorize.ResourceInstance}); err != nil {
			return false, err
		}
	}
	if d.Outcome == authorize.OutcomeUnauthenticated {
		return false, d.Err()
	}
	return ok, nil
}

// admit is the delivery rule of one stored event for a (07 section 8.1, 05 section 6): a personal
// event only to its audience, and, when its receive action is not self, only when authorize also
// allows that action (docs/DECISIONS.md D-M2-02-7); any other event only when authorize allows its
// receive action on the event's resource at delivery time, with the decision of authorize (zero
// for a personal event with self). ctx is in filter
// mode, so a refusal writes no audit entry. agentConsumer is PageInput.AgentConsumer.
func admit(ctx context.Context, az authorize.Authorizer, a actor.Actor, ev sync.Event, agentConsumer bool) (bool, authorize.Decision, error) {
	if !ev.AudiencePrincipalID.IsZero() {
		if ev.AudiencePrincipalID != a.ID {
			return false, authorize.Decision{}, nil
		}
		if ev.ReceiveAction == receiveSelf {
			return true, authorize.Decision{}, nil
		}
		// A personal event with a receive action (agent.config_changed, M2-02) also needs that
		// action at delivery time: an agent that stopped operating the channel gets nothing.
	}
	act, ok := authorize.ParseAction(ev.ReceiveAction)
	if !ok {
		// A receive action outside the catalogue of this build cannot be decided: fail closed.
		return false, authorize.Decision{}, nil
	}
	r := authorize.ResourceFromEvent(ev.ResourceKind, ev.ResourceID, ev.SpaceID, ev.ChannelID, ev.ThreadID)
	if agentConsumer {
		// The audience is matched above; for the consumer engine, which runs runs.Delivery before
		// every invocation-bearing event, a receive action on the agent itself decides on it (M2-08).
		r.Event.AudienceID = ev.AudiencePrincipalID
	}
	d, err := az.Authorize(ctx, a, act, r)
	if err != nil {
		return false, authorize.Decision{}, err
	}
	return d.Allowed(), d, nil
}
