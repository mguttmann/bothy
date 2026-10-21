// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Values of the ephemeral event run.updated (05 section 6, table of ephemeral events).
const (
	eventRunUpdated         = string(authorize.EventRunUpdated)
	runUpdatedSchemaVersion = 1
)

// A patch on an ended run is 409 conflict with reasonRunNotActive (answer.go), the reason of an
// answer to an ended run as well (ruling D-M2-10-1).

// Log messages of the patch use case; they carry the run id and codes only, never a value
// (SEC-LOG-4).
const (
	msgPublishFailed = "run.updated not published, the next patch or getRun heals the live box"
	logKeyRunID      = "bothy.run.id"
	logKeyErrorType  = "error.type"
)

// patchOutcome is what one committed patch hands to the steps after the commit.
type patchOutcome struct {
	version int64
	event   eventlog.EphemeralEvent
	added   []domain.DocStep
	agentID ids.ID
}

// Patch implements runs.Runs (patchRun and the gateway op run.update, action run.update; M2-10
// design note 1). The order of the checks: the run is loaded without a lock and run.update is
// authorized on it (agent-self; authz audits the denials, an allowed patch writes no audit
// entry); the ops pass the static checks of domain.ParseOps; then, in one unit of work, the row
// is locked, a run that is not active gives 409 conflict run_not_active, a stale base_version 409
// base_version_conflict with the document, the dynamic checks of domain.Apply run against the
// locked state, and the change is written in the lock order of 03 with exactly one version step
// and stall_warned_at cleared. No event_log row, no event_seq, no audit_seq and no NOTIFY are
// written. Only after the unit of work returned without error, run.updated is published and
// bothy.run.steps counted.
//
// Errors: see runs.Runs.Patch.
func (s *Service) Patch(ctx context.Context, in runs.PatchRun) (runs.PatchResult, error) {
	a := caller(ctx)
	row, found, err := s.d.Repo.Run(ctx, in.RunID)
	if err != nil {
		return runs.PatchResult{}, err
	}
	d, err := s.authorizeAgent(ctx, a, authorize.ActionRunUpdate, row, found, in.RunID, nil)
	if err != nil {
		return runs.PatchResult{}, err
	}
	if !d.Allowed() {
		return runs.PatchResult{}, d.Err()
	}
	raw := make([]domain.RawOp, 0, len(in.Ops))
	for _, o := range in.Ops {
		raw = append(raw, domain.RawOp{Op: o.Op, Path: o.Path, Value: o.Value})
	}
	ops, fields := domain.ParseOps(raw)
	if len(fields) > 0 {
		return runs.PatchResult{}, errs.New(errs.CodeValidationFailed, "the patch has invalid operations").WithFields(fields...)
	}
	var out patchOutcome
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.patchLocked(ctx, a, in, ops)
		return err
	})
	if err != nil {
		return runs.PatchResult{}, err
	}
	s.afterPatch(ctx, out)
	return runs.PatchResult{Version: out.version}, nil
}

// patchLocked runs steps c to g of design note 1 inside the unit of work of ctx.
func (s *Service) patchLocked(ctx context.Context, a actor.Actor, in runs.PatchRun, ops []domain.Op) (patchOutcome, error) {
	row, found, err := s.d.Repo.LockRun(ctx, in.RunID)
	if err != nil {
		return patchOutcome{}, err
	}
	if !found {
		return patchOutcome{}, invisible()
	}
	// The state comes before the version: an adapter after cancel_timeout learns that the run
	// ended, not only that it is behind (ruling D-M2-10-1).
	if !row.State.Active() {
		return patchOutcome{}, errs.New(errs.CodeConflict, "the run is not active").WithReason(reasonRunNotActive)
	}
	if row.Version != in.BaseVersion {
		doc, err := s.documentOf(ctx, row)
		if err != nil {
			return patchOutcome{}, err
		}
		return patchOutcome{}, errs.New(errs.CodeBaseVersionConflict, "base_version is not the current version of the run").WithCurrent(doc)
	}
	state, err := s.patchState(ctx, row, ops)
	if err != nil {
		return patchOutcome{}, err
	}
	now := s.now()
	ch, norm, err := domain.Apply(state, ops, s.limits(), domain.Env{Now: now, NewID: ids.New})
	if err != nil {
		return patchOutcome{}, err
	}
	stored, err := s.d.Repo.WritePatch(ctx, PatchWrite{ID: row.ID, Change: ch})
	if err != nil {
		return patchOutcome{}, err
	}
	for _, f := range ch.Followups {
		if err := s.followups().SetState(ctx, row.ID, f.MessageID, f.State); err != nil {
			return patchOutcome{}, err
		}
	}
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return patchOutcome{}, err
	}
	payload := s.updatedPayload(row, in.BaseVersion, stored.Version, norm)
	return patchOutcome{
		version: stored.Version, added: ch.AddedSteps, agentID: row.AgentID,
		event: eventlog.EphemeralEvent{
			Type: eventRunUpdated, SchemaVersion: runUpdatedSchemaVersion,
			Actor:      &eventlog.ActorRef{Kind: eventlog.ActorAgent, PrincipalID: a.ID},
			Scope:      eventlog.Scope{SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: row.ThreadID},
			Resource:   eventlog.ResourceRef{Kind: resourceRun, ID: row.ID, Version: stored.Version},
			OccurredAt: now, Payload: payload,
		},
	}, nil
}

// updatedPayload builds the payload of run.updated from the normalized ops; when its encoded form
// reaches sync.event_payload_max_bytes it goes without ops and with Truncated, and the gateway
// sends the whole document instead (04 section 2.2 rule 2).
func (s *Service) updatedPayload(row RunRow, base, version int64, norm []domain.NormOp) runs.RunUpdated {
	payload := runs.RunUpdated{RunID: row.ID, ThreadID: row.ThreadID, BaseVersion: base, Version: version,
		Ops: make([]runs.PatchOp, 0, len(norm))}
	for _, o := range norm {
		payload.Ops = append(payload.Ops, runs.PatchOp{Op: string(o.Op), Path: o.Path, Value: o.Value})
	}
	if b, err := encode(payload); err != nil || len(b) >= s.cfg.EventPayloadMaxBytes {
		payload.Ops, payload.Truncated = nil, true
	}
	return payload
}

// patchState reads the locked state a patch applies to: the run columns, the steps only when an
// op names a step (without their details, apart from the summary step's), and the states of the
// follow-ups the ops name.
func (s *Service) patchState(ctx context.Context, row RunRow, ops []domain.Op) (domain.DocumentState, error) {
	st := domain.DocumentState{
		StatusLine: row.StatusLine, LastTextExcerpt: row.LastTextExcerpt, Model: row.Model,
		Metrics: jsonObject(row.Metrics), Plan: jsonObject(row.Plan),
		Cost:      domain.Money{AmountMicros: row.CostMicros, Currency: row.Currency, IsEstimate: row.CostIsEstimate},
		Followups: map[ids.ID]string{},
	}
	var steps bool
	var followups []ids.ID
	for _, o := range ops {
		if o.Target == domain.TargetStep || o.Target == domain.TargetStepMember {
			steps = true
		}
		if o.Target == domain.TargetFollowupState {
			followups = append(followups, o.ID)
		}
	}
	if steps {
		rows, err := s.d.Repo.PatchSteps(ctx, row.ID)
		if err != nil {
			return domain.DocumentState{}, err
		}
		st.Steps = make([]domain.DocStep, 0, len(rows))
		for _, r := range rows {
			st.Steps = append(st.Steps, domain.DocStep{
				ID: r.ID, Ordinal: r.Ordinal, ParentStepID: r.ParentStepID, Kind: r.Kind, Name: r.Name, Title: r.Title,
				Status: r.Status, Detail: r.Detail, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
			})
		}
	}
	if len(followups) > 0 {
		states, err := s.followups().States(ctx, row.ID, followups)
		if err != nil {
			return domain.DocumentState{}, err
		}
		st.Followups = states
	}
	return st, nil
}

// afterPatch publishes run.updated and counts the steps the adapter added; both happen only for
// a committed patch. A publish error is logged with the run id and never reaches the adapter: the
// patch is stored, and the next patch or a getRun heals the box (ADR 0025, Consequences).
func (s *Service) afterPatch(ctx context.Context, out patchOutcome) {
	ctx = context.WithoutCancel(ctx)
	if box := s.publisher.Load(); box != nil {
		if err := box.p.PublishEphemeral(ctx, out.event); err != nil {
			s.d.Log.LogAttrs(ctx, slog.LevelWarn, msgPublishFailed, slog.String(logKeyRunID, out.event.Resource.ID.String()),
				slog.String(logKeyErrorType, errs.KindOf(err).String()))
		}
	}
	if len(out.added) == 0 || s.d.Steps == nil {
		return
	}
	handle, err := s.handleOf(ctx, out.agentID)
	if err != nil {
		return
	}
	for _, st := range out.added {
		s.d.Steps.StepAdded(ctx, handle, st.Kind)
	}
}

// limits returns the limits of the patch rules from the configuration.
func (s *Service) limits() domain.Limits {
	return domain.Limits{ExcerptMaxChars: s.cfg.ExcerptMaxChars, StepDetailMaxBytes: s.cfg.StepDetailMaxBytes,
		MaxStepsPersisted: s.cfg.MaxStepsPersisted}
}

// followups returns the follow-up port of a patch: Deps.Followups when set, else the Followups
// service, else NoFollowups.
func (s *Service) followups() FollowupStates {
	switch {
	case s.d.Followups != nil:
		return s.d.Followups
	case s.fu != nil:
		return s.fu
	}
	return NoFollowups{}
}

// jsonObject returns raw, or the empty object for an empty column value.
func jsonObject(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}

// StepMetrics counts bothy.run.steps (signals.md 4.3): one per step an adapter added, after the
// commit, with the agent handle and the step kind; summary steps are not counted.
type StepMetrics interface {
	StepAdded(ctx context.Context, agentHandle, kind string)
}
