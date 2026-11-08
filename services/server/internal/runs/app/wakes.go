// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The wake-ups of stage AR (docs/spec/09-agents.md 5.8, ADR 0077): the operations of 5.8.2, the
// fire on both paths (a wake run or a wake input, 5.8.3), the checks before host executions, the
// visible ends of 5.8.4 and the job runs.wake_due. Every write of a wake-up runs in one unit of
// work that locks the run row first (where there is one), then the wake-up, then the counts (03
// 6.3), and appends run.wake_changed in the same unit.

// The values of the wake-ups that this file writes.
const (
	// triggerKindWake is runs.trigger_kind of a wake run (03 runs, R-DM-60).
	triggerKindWake = "wake"
	// listThreadWakes is the cursor list of listThreadWakes.
	listThreadWakes = "runs.wakes"
	// resourceThreadKind is the resource kind of run.wake_changed (its resource is the thread).
	resourceThreadKind = "thread"
	// The audit attributes of the wake-ups.
	attrWakeID     = "wake_id"
	attrWakeKind   = "wake_kind"
	attrWakeReason = "wake_reason"
	attrCheck      = "check"
	// toolClassHost is the tool class a command wake-up needs.
	toolClassHost = "host"
)

// ignoredAtCheck are the refusals of agent.invoke that run.wake.check does not end a wake-up for:
// 5.8.3 lists the checks of the person, the agent, the channel, the tool class and SR2 before a
// host execution; billing and project readiness belong to the fire, which checks them again
// (ruling D-AR-04-7).
var ignoredAtCheck = []string{
	"billing_mode_personal_not_holder", "project_not_ready", "operator_cannot_serve_project", "run_active_other_agent",
}

// wakeBox holds the bound configuration and collaborators of the wake-ups.
type wakeBox struct {
	cfg WakeConfig
	d   WakeDeps
}

// BindWakes binds the wake-ups (stage AR). internal/wire calls it once after the messaging,
// authz and job collaborators exist. Errors: a missing collaborator or a configuration value out
// of its range.
//
// Example:
//
//	err := svc.BindWakes(cfg, app.WakeDeps{Repo: repo, Schedule: jobs, Notes: messages, Owners: az})
func (s *Service) BindWakes(cfg WakeConfig, d WakeDeps) error {
	if d.Repo == nil || d.Schedule == nil || d.Notes == nil || d.Owners == nil {
		return errors.New("runs: BindWakes needs the repository, the scheduler, the notes and the owners")
	}
	if cfg.MaxPending < 1 || cfg.MaxWait <= 0 || cfg.MinDelay < 0 || cfg.UsageWakeMax < 1 || cfg.LabelMaxChars < 1 ||
		cfg.ChainMax < 0 || cfg.IdleMax < 1 {
		return errors.New("runs: run.wake_max_pending, run.wake_max_wait, run.usage_wake_max, run.wake_label_max_chars and " +
			"run.autonomous_idle_max must be positive, run.wake_min_delay and run.autonomous_chain_max not negative")
	}
	if d.Metrics == nil {
		d.Metrics = noWakeMetrics{}
	}
	s.wakes.Store(&wakeBox{cfg: cfg, d: d})
	return nil
}

// wakeBox returns the bound wake-ups. Errors: internal before BindWakes.
func (s *Service) wakeBox() (*wakeBox, error) {
	b := s.wakes.Load()
	if b == nil {
		return nil, errs.New(errs.CodeInternalError, "runs: the wake-ups are not bound")
	}
	return b, nil
}

// limits returns the field limits of the wake-ups.
func (s *Service) wakeLimits(b *wakeBox) domain.WakeLimits {
	return domain.WakeLimits{LabelMaxChars: b.cfg.LabelMaxChars, TextMaxBytes: s.cfg.MaxContentBytes, MinDelay: b.cfg.MinDelay, MaxWait: b.cfg.MaxWait}
}

// autonomyLimits returns the caps of 5.8.6.
func (b *wakeBox) autonomyLimits() domain.AutonomyLimits {
	return domain.AutonomyLimits{ChainMax: b.cfg.ChainMax, IdleMax: b.cfg.IdleMax}
}

// Wakes implements runs.Wakes over the Service (the names Cancel and the reads would collide with
// runs.Runs on one type).
type Wakes struct{ s *Service }

// Compile-time check: Wakes is the incoming port.
var _ runs.Wakes = (*Wakes)(nil)

// Wakes returns the wake-up operations of the service.
func (s *Service) Wakes() *Wakes { return &Wakes{s: s} }

// wakeResult is the outcome of a unit of work of a wake-up operation: the wake-up, whether it was
// created, and an error that follows only after the commit (a refusal whose audit entry, note and
// stored refusal commit).
type wakeResult struct {
	wake      WakeRow
	created   bool
	check     runs.WakeCheck
	committed error
}

// conflict is the 409 of a refusal with its reason (06 R-API-30).
func conflict(reason string) error {
	return errs.New(errs.CodeConflict, "the wake-up was refused: "+reason).WithReason(reason)
}

// Create implements runs.Wakes (createRunWake, run.wake.create; 09 5.8.2): the fields of the
// request before anything is read (400), then in one unit of work the run is locked and
// run.wake.create authorized (agent-self of the run's agent); a usage_limit request of a known
// outage moves the pending wake-up's not_before; otherwise the checks of createRefusal run and a
// pending wake-up is stored with its job runs.wake_due at not_before (at) and at the deadline,
// run.wake_changed and an audit entry. A refusal stores nothing and answers 409, except a deferred
// request and usage_wake_max, which are stored refused with note and notification.
func (w *Wakes) Create(ctx context.Context, in runs.CreateWake) (runs.WakeResult, error) {
	s := w.s
	b, err := s.wakeBox()
	if err != nil {
		return runs.WakeResult{}, err
	}
	req := domain.WakeRequest{
		Kind: domain.WakeKind(in.Kind), Reason: domain.WakeReason(in.Reason), Label: in.Label, Condition: conditionOf(in.Condition),
		NotBefore: in.NotBefore, Deadline: in.Deadline, Outage: in.Outage,
	}
	if err := domain.CheckWakeRequest(req, s.wakeLimits(b)); err != nil {
		return runs.WakeResult{}, err
	}
	a := caller(ctx)
	var res wakeResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res, err = w.createTx(ctx, b, a, in, req)
		return err
	})
	return res.answer(ctx, err)
}

// answer turns the result of a unit of work into the answer of an operation. A refusal whose unit
// committed (its audit entry, a wake-up stored refused with its note) keeps that effect also under
// an Idempotency-Key (02 8.4, idempotency.KeepEffectOnFinal).
func (r wakeResult) answer(ctx context.Context, err error) (runs.WakeResult, error) {
	if err != nil {
		return runs.WakeResult{}, err
	}
	if r.committed != nil {
		idempotency.KeepEffectOnFinal(ctx)
		return runs.WakeResult{}, r.committed
	}
	return runs.WakeResult{Wake: wakeView(r.wake), Created: r.created}, nil
}

// createTx is Create inside its unit of work.
func (w *Wakes) createTx(ctx context.Context, b *wakeBox, a actor.Actor, in runs.CreateWake, req domain.WakeRequest) (wakeResult, error) {
	s := w.s
	run, err := s.lockOwnRun(ctx, a, authorize.ActionRunWakeCreate, in.RunID)
	if err != nil {
		return wakeResult{}, err
	}
	now := s.now()
	if req.Reason == domain.WakeReasonUsageLimit {
		ex, found, err := b.d.Repo.WakeByOutage(ctx, run.ID, req.Outage)
		if err != nil {
			return wakeResult{}, err
		}
		if found {
			return w.repeatOutage(ctx, b, ex, req, now)
		}
	}
	nb, dl, err := domain.WakeWindow(now, req.NotBefore, req.Deadline, req.Reason, s.wakeLimits(b))
	if err != nil {
		return wakeResult{}, err
	}
	nw := NewWake{
		ID: ids.New(), AgentID: run.AgentID, ThreadID: run.ThreadID, SourceRunID: run.ID, TriggeredBy: run.TriggeredBy, Kind: req.Kind,
		Reason: req.Reason, State: domain.WakePending, Condition: domain.EncodeCondition(req.Condition), Label: req.Label,
		Outage: req.Outage, NotBefore: nb, Deadline: dl, CreatedAt: now,
	}
	refusal, err := w.createRefusal(ctx, b, run, req)
	if err != nil {
		return wakeResult{}, err
	}
	if refusal != "" {
		// A deferred request and usage_wake_max are stored refused, so the person sees them (09
		// 5.8.2, 18.19.10); every other refusal stores nothing.
		return w.refuse(ctx, b, a, authorize.ActionRunWakeCreate, nw, refusal, in.Deferred || refusal == domain.WakeCodeUsageWakeMax)
	}
	return w.insertPending(ctx, b, a, authorize.ActionRunWakeCreate, nw, func(ctx context.Context) (WakeRow, bool, error) {
		return b.d.Repo.WakeByOutage(ctx, run.ID, req.Outage)
	})
}

// lockOwnRun locks the run id and authorizes act for the agent a on it (agent-self of the run's
// agent). Errors: 404 for an unknown or another agent's run, the denial of authorize.
func (s *Service) lockOwnRun(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID) (RunRow, error) {
	run, found, err := s.d.Repo.LockRun(ctx, id)
	if err != nil {
		return RunRow{}, err
	}
	d, err := s.authorizeAgent(ctx, a, act, run, found, id, nil)
	if err != nil {
		return RunRow{}, err
	}
	if !d.Allowed() {
		return RunRow{}, d.Err()
	}
	return run, nil
}

// refuse audits a refused create or recover and answers 409 with the reason after the commit;
// with store the wake-up is stored in state refused with its note and run.wake_changed.
func (w *Wakes) refuse(ctx context.Context, b *wakeBox, a actor.Actor, act authorize.Action, nw NewWake, refusal string, store bool) (wakeResult, error) {
	if err := w.s.auditWake(ctx, a, act, refusal, nw.SourceRunID, nw.ID, nw.Kind, nw.Reason, ""); err != nil {
		return wakeResult{}, err
	}
	if store {
		if err := w.storeRefused(ctx, b, nw, refusal); err != nil {
			return wakeResult{}, err
		}
	}
	return wakeResult{committed: conflict(refusal)}, nil
}

// insertPending stores the pending wake-up nw with its jobs, run.wake_changed and the audit entry
// of act; when its idempotency key exists (a concurrent request of the same outage or recovery)
// existing reads and answers that wake-up.
func (w *Wakes) insertPending(ctx context.Context, b *wakeBox, a actor.Actor, act authorize.Action, nw NewWake,
	existing func(context.Context) (WakeRow, bool, error)) (wakeResult, error) {
	stored, inserted, err := b.d.Repo.InsertWake(ctx, nw)
	if err != nil {
		return wakeResult{}, err
	}
	if !inserted {
		ex, found, err := existing(ctx)
		if err != nil || !found {
			return wakeResult{}, errors.Join(err, errs.New(errs.CodeConflict, "runs: the wake-up of this key changed concurrently"))
		}
		return wakeResult{wake: ex}, nil
	}
	if err := w.afterInsert(ctx, b, stored); err != nil {
		return wakeResult{}, err
	}
	if err := w.s.auditWake(ctx, a, act, "", nw.SourceRunID, stored.ID, stored.Kind, stored.Reason, ""); err != nil {
		return wakeResult{}, err
	}
	return wakeResult{wake: stored, created: true}, nil
}

// repeatOutage answers a usage_limit request of an outage that has its wake-up (18.19.10): a
// pending one takes the later not_before (and its deadline from there), a refused one answers its
// refusal again, any other end answers itself.
func (w *Wakes) repeatOutage(ctx context.Context, b *wakeBox, ex WakeRow, req domain.WakeRequest, now time.Time) (wakeResult, error) {
	s := w.s
	switch ex.State {
	case domain.WakeRefused:
		return wakeResult{committed: conflict(ex.StateReason)}, nil
	case domain.WakePending:
	case domain.WakeFired, domain.WakeCancelled, domain.WakeExpired, domain.WakeFailed:
		return wakeResult{wake: ex}, nil
	}
	nb, dl, err := domain.WakeWindow(now, req.NotBefore, req.Deadline, req.Reason, s.wakeLimits(b))
	if err != nil {
		return wakeResult{}, err
	}
	if !nb.After(ex.NotBefore) {
		return wakeResult{wake: ex}, nil
	}
	row, changed, err := b.d.Repo.UpdateWake(ctx, WakeWrite{ID: ex.ID, Version: ex.Version, State: domain.WakePending, StateReason: ex.StateReason,
		NotBefore: nb, Deadline: dl})
	if err != nil {
		return wakeResult{}, err
	}
	if !changed {
		return wakeResult{}, errs.New(errs.CodeInternalError, "runs: a locked wake-up changed")
	}
	if err := w.afterInsert(ctx, b, row); err != nil {
		return wakeResult{}, err
	}
	return wakeResult{wake: row}, nil
}

// afterInsert schedules runs.wake_due of a pending wake-up at not_before (at) and at its deadline
// (every kind) and appends run.wake_changed.
func (w *Wakes) afterInsert(ctx context.Context, b *wakeBox, r WakeRow) error {
	if r.Kind == domain.WakeAt {
		if err := b.d.Schedule.ScheduleWakeDue(ctx, r.ID, r.NotBefore); err != nil {
			return err
		}
	}
	if err := b.d.Schedule.ScheduleWakeDue(ctx, r.ID, r.Deadline); err != nil {
		return err
	}
	return w.s.emitWakeChanged(ctx, r, noteRef{})
}

// createRefusal returns the reason of 5.8.1 and 5.8.6 that refuses a new wake-up of run, "" for
// none: the run is active; the autonomy rule (owner or allow_autonomous_runs); pr_checks only for
// an owner; command only for an agent of tool class host; agent.invoke of the person (trigger,
// reachability, channel, I2, SR2); the pending cap of the thread; usage_wake_max per run.
func (w *Wakes) createRefusal(ctx context.Context, b *wakeBox, run RunRow, req domain.WakeRequest) (string, error) {
	s := w.s
	if !run.State.Active() {
		return domain.WakeCodeRunNotActive, nil
	}
	if r, err := s.kindRefusal(ctx, b, run.AgentID, run.ThreadID, run.TriggeredBy, req.Kind, true); err != nil || r != "" {
		return r, err
	}
	denial, err := s.wakeInvokeDenial(ctx, run.AgentID, run.ThreadID, run.TriggeredBy, run.AgentID)
	if err != nil || denial != "" {
		return denial, err
	}
	return w.capRefusal(ctx, b, run.ThreadID, run.ID, req.Reason)
}

// capRefusal applies the pending cap of the thread and usage_wake_max of the source run.
func (w *Wakes) capRefusal(ctx context.Context, b *wakeBox, threadID, runID ids.ID, reason domain.WakeReason) (string, error) {
	n, err := b.d.Repo.CountPendingWakes(ctx, threadID)
	if err != nil {
		return "", err
	}
	if n >= b.cfg.MaxPending {
		return domain.WakeCodeMaxPending, nil
	}
	if reason == domain.WakeReasonUsageLimit {
		u, err := b.d.Repo.CountUsageWakes(ctx, runID)
		if err != nil {
			return "", err
		}
		if u >= b.cfg.UsageWakeMax {
			return domain.WakeCodeUsageWakeMax, nil
		}
	}
	return "", nil
}

// kindRefusal applies the autonomy rule and the rules of the kind (5.8.1): autonomous inputs need
// an owner or the channel switch; pr_checks needs an owner; command needs an agent of tool class
// host where hostNeeded (at create and before every execution, not at the fire).
func (s *Service) kindRefusal(ctx context.Context, b *wakeBox, agentID, threadID, person ids.ID, kind domain.WakeKind, hostNeeded bool) (string, error) {
	owner, refusal, err := s.autonomyRule(ctx, b, threadID, person)
	if err != nil || refusal != "" {
		return refusal, err
	}
	switch kind {
	case domain.WakePRChecks:
		if !owner {
			return domain.WakeCodePRChecksOwnerOnly, nil
		}
	case domain.WakeCommand:
		if hostNeeded {
			return s.hostRefusal(ctx, agentID)
		}
	case domain.WakeAt:
	}
	return "", nil
}

// autonomyRule reports whether person is an active owner and refuses an autonomous input of a
// person who is no owner in a channel without allow_autonomous_runs (07 2.14 item 1).
func (s *Service) autonomyRule(ctx context.Context, b *wakeBox, threadID, person ids.ID) (owner bool, refusal string, err error) {
	if owner, err = b.d.Owners.IsActiveOwner(ctx, person); err != nil || owner {
		return owner, "", err
	}
	info, err := s.d.Threads.Info(ctx, threadID)
	if err != nil {
		return false, "", err
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
	if err != nil {
		return false, "", err
	}
	if !ch.AllowAutonomousRuns {
		return false, domain.WakeCodeAutonomousNotAllowed, nil
	}
	return false, "", nil
}

// hostRefusal refuses a command wake-up of an agent whose configured tool class is not host.
func (s *Service) hostRefusal(ctx context.Context, agentID ids.ID) (string, error) {
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{agentID})
	if err != nil {
		return "", err
	}
	if facts[agentID].ToolClass != toolClassHost {
		return domain.WakeCodeToolClassMismatch, nil
	}
	return "", nil
}

// wakeInvokeDenial decides agent.invoke for the wake-up's person on the thread now, as for a human
// message that addresses the agent (07 2.14 items 2 to 4): the check principal of the person
// without a request, the agent counted as mentioned (the wake-up is the agent's own address to the
// person, like the component of its card), the active run's agent for point 8. It returns the
// authorize reason of a denial, "" for an allow; it writes no audit entry (filter mode), the caller
// audits.
func (s *Service) wakeInvokeDenial(ctx context.Context, agentID, threadID, person, activeAgent ids.ID) (string, error) {
	info, err := s.d.Threads.Info(ctx, threadID)
	if err != nil {
		return "", err
	}
	if s.d.Versions == nil {
		return "", errs.New(errs.CodeInternalError, "runs: the wake-ups need the permission versions")
	}
	v, err := s.d.Versions.PermissionVersion(ctx)
	if err != nil {
		return "", err
	}
	r := authorize.Resource{
		Kind: authorize.ResourceThread, ID: threadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: &authorize.InvokeFacts{AgentID: agentID, TriggerOrigin: originHuman, MentionsAgent: true, ActiveRunAgentID: activeAgent},
	}
	d, err := s.d.Authorizer.Authorize(authorize.WithPermissionVersion(authorize.Filter(ctx), v), actor.Recheck(person), authorize.ActionAgentInvoke, r)
	if err != nil {
		return "", err
	}
	if d.Allowed() {
		return "", nil
	}
	if d.Reason == "" {
		return string(d.Outcome), nil
	}
	return string(d.Reason), nil
}

// storeRefused stores a wake-up directly in state refused with its reason (a deferred request,
// usage_wake_max, a refused recovery; 09 5.8.2), with the note and run.wake_changed, so the person
// sees it and is notified (5.8.4).
func (w *Wakes) storeRefused(ctx context.Context, b *wakeBox, nw NewWake, reason string) error {
	nw.State, nw.StateReason = domain.WakeRefused, reason
	if nw.NotBefore.After(nw.Deadline) {
		nw.Deadline = nw.NotBefore
	}
	row, inserted, err := b.d.Repo.InsertWake(ctx, nw)
	if err != nil || !inserted {
		return err
	}
	return w.s.afterEnd(ctx, b, row)
}

// endWake ends the locked wake-up r in state with reason: the compare-and-swap, the note of 5.8.4
// for every end other than fired, run.wake_changed and the metric after the commit.
func (s *Service) endWake(ctx context.Context, b *wakeBox, r WakeRow, state domain.WakeState, reason string, w WakeWrite) (WakeRow, error) {
	w.ID, w.Version, w.State, w.StateReason, w.At = r.ID, r.Version, state, reason, s.now()
	row, changed, err := b.d.Repo.UpdateWake(ctx, w)
	if err != nil {
		return WakeRow{}, err
	}
	if !changed {
		return WakeRow{}, errs.New(errs.CodeInternalError, "runs: a locked wake-up changed")
	}
	return row, s.afterEnd(ctx, b, row)
}

// afterEnd writes the note of an end other than fired, appends run.wake_changed and records the
// end after the commit.
func (s *Service) afterEnd(ctx context.Context, b *wakeBox, row WakeRow) error {
	var note noteRef
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return err
	}
	// A deleted thread takes no new message (D-DEL-3); its wake-ups end without a note, which
	// nobody could read.
	if row.State != domain.WakeFired && !info.Deleted {
		if note.id, note.seq, err = b.d.Notes.AppendSystemNote(ctx, row.ThreadID, row.AgentID, domain.WakeNoteText(row.Label, row.State, row.StateReason)); err != nil {
			return err
		}
	}
	if err := s.emitWakeChanged(ctx, row, note); err != nil {
		return err
	}
	m, state, reason := b.d.Metrics, row.State, row.Reason
	txn.AfterCommit(ctx, func() { m.WakeEnded(context.WithoutCancel(ctx), state, reason) })
	return nil
}

// noteRef is the note of a wake-up end: its message and thread_seq; zero for none.
type noteRef struct {
	id  ids.ID
	seq int64
}

// wakeChangedJSON is RunWakeChangedPayload.
type wakeChangedJSON struct {
	WakeID        ids.ID  `json:"wake_id"`
	ThreadID      ids.ID  `json:"thread_id"`
	AgentID       ids.ID  `json:"agent_id"`
	TriggeredBy   ids.ID  `json:"triggered_by"`
	State         string  `json:"state"`
	StateReason   string  `json:"state_reason,omitempty"`
	NoteMessageID *ids.ID `json:"note_message_id,omitempty"`
	NoteThreadSeq int64   `json:"note_thread_seq,omitempty"`
}

// emitWakeChanged appends run.wake_changed for r (09 5.8.2): resource the thread without version,
// receive action run.wake.read, so the clients that may read the thread and the agent of the
// wake-up (operator rule, 07 2.9) receive it; the payload carries ids and codes only.
func (s *Service) emitWakeChanged(ctx context.Context, r WakeRow, note noteRef) error {
	info, err := s.d.Threads.Info(ctx, r.ThreadID)
	if err != nil {
		return err
	}
	pl := wakeChangedJSON{WakeID: r.ID, ThreadID: r.ThreadID, AgentID: r.AgentID, TriggeredBy: r.TriggeredBy, State: string(r.State),
		StateReason: r.StateReason}
	if !note.id.IsZero() {
		pl.NoteMessageID, pl.NoteThreadSeq = &note.id, note.seq
	}
	p, err := encode(pl)
	if err != nil {
		return err
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventRunWakeChanged), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: r.ThreadID, ResourceKind: resourceThreadKind, ResourceID: r.ThreadID,
		ReceiveAction: string(authorize.ActionRunWakeRead), Payload: p,
	})
}

// auditWake writes the audit entry of a wake-up operation: allow without reason, deny with the
// reason of the refusal, the run and the wake-up as attributes, check naming the failed check of
// run.wake.check. The actor is the caller, or the check principal of the person for the job.
func (s *Service) auditWake(ctx context.Context, a actor.Actor, act authorize.Action, reason string, runID, wakeID ids.ID,
	kind domain.WakeKind, wr domain.WakeReason, check string) error {
	if s.d.Audit == nil {
		return errs.New(errs.CodeInternalError, "runs: the wake-ups have no audit recorder")
	}
	attrs := auditing.Attrs{attrWakeID: auditing.ID(wakeID), attrWakeKind: auditing.Code(string(kind)), attrWakeReason: auditing.Code(string(wr))}
	if check != "" {
		attrs[attrCheck] = auditing.Code(check)
	}
	e := auditing.Entry{Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceRun, ResourceID: runID, Attrs: attrs}
	if reason != "" {
		e.Decision, e.Reason = auditing.DecisionDeny, auditing.Reason(reason)
	}
	return s.d.Audit.Record(actor.WithActor(ctx, a), e)
}

// Recover implements runs.Wakes (recoverRunWake, run.wake.recover; 09 5.8.2, 18.19.9, 18.19.10):
// a wake-up of kind at for an ended run of the calling agent in state failed, one per run and
// reason (a repetition answers the first); a refusal of the autonomy checks is stored refused with
// note and notification and answers 409. A run that has not ended answers 409 run_not_ended and
// stores nothing; a run that ended cancelled or completed is stored refused with run_not_failed;
// usage_limit answers the pending usage_limit wake-up of an outage of the run when there is one.
func (w *Wakes) Recover(ctx context.Context, in runs.RecoverWake) (runs.WakeResult, error) {
	s := w.s
	b, err := s.wakeBox()
	if err != nil {
		return runs.WakeResult{}, err
	}
	reason := domain.WakeReason(in.Reason)
	if reason != domain.WakeReasonRecovery && reason != domain.WakeReasonUsageLimit {
		return runs.WakeResult{}, fieldError("/reason", "invalid", "the reason is recovery or usage_limit")
	}
	if err := domain.CheckLabel(in.Label, b.cfg.LabelMaxChars); err != nil {
		return runs.WakeResult{}, err
	}
	a := caller(ctx)
	var res wakeResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res, err = w.recoverTx(ctx, b, a, in, reason)
		return err
	})
	return res.answer(ctx, err)
}

// recoverTx is Recover inside its unit of work.
func (w *Wakes) recoverTx(ctx context.Context, b *wakeBox, a actor.Actor, in runs.RecoverWake, reason domain.WakeReason) (wakeResult, error) {
	s := w.s
	run, err := s.lockOwnRun(ctx, a, authorize.ActionRunWakeRecover, in.RunID)
	if err != nil {
		return wakeResult{}, err
	}
	if !run.State.Ended() {
		return wakeResult{}, conflict(domain.WakeCodeRunNotEnded)
	}
	if reason == domain.WakeReasonUsageLimit {
		// One outage gets one wake-up: while the outage's wake-up of run.wake.create is pending,
		// a recovery after a restart changes nothing and answers it, whatever end the source run
		// reached (5.8.2, 18.19.10, issue 567 item 4).
		if p, found, err := b.d.Repo.PendingUsageWakeOfRun(ctx, run.ID); err != nil || found {
			return wakeResult{wake: p}, err
		}
	}
	existing := func(ctx context.Context) (WakeRow, bool, error) {
		return b.d.Repo.WakeBySourceReason(ctx, run.ID, reason)
	}
	if ex, found, err := existing(ctx); err != nil || found {
		return wakeResult{wake: ex}, err
	}
	now := s.now()
	nb, dl, err := domain.WakeWindow(now, in.NotBefore, time.Time{}, reason, s.wakeLimits(b))
	if err != nil {
		return wakeResult{}, err
	}
	nw := NewWake{
		ID: ids.New(), AgentID: run.AgentID, ThreadID: run.ThreadID, SourceRunID: run.ID, TriggeredBy: run.TriggeredBy, Kind: domain.WakeAt,
		Reason: reason, State: domain.WakePending, Condition: domain.EncodeCondition(domain.WakeCondition{}), Label: in.Label,
		NotBefore: nb, Deadline: dl, CreatedAt: now,
	}
	if run.State != domain.StateFailed {
		// A run that ended cancelled or completed gets no recovery run (its end was asked for or
		// reached); its lost work still shows: the wake-up is stored refused with the note and
		// the notification (18.19.9, issue 567 item 7).
		return w.refuse(ctx, b, a, authorize.ActionRunWakeRecover, nw, domain.WakeCodeRunNotFailed, true)
	}
	refusal, err := w.recoverRefusal(ctx, b, run, reason)
	if err != nil {
		return wakeResult{}, err
	}
	if refusal != "" {
		// Without permission the thread gets the visible note instead (18.19.9).
		return w.refuse(ctx, b, a, authorize.ActionRunWakeRecover, nw, refusal, true)
	}
	return w.insertPending(ctx, b, a, authorize.ActionRunWakeRecover, nw, existing)
}

// recoverRefusal applies the autonomy rule, agent.invoke of the person (with the thread's active
// run for point 8), the pending cap and usage_wake_max to a recovery or usage-limit wake-up.
func (w *Wakes) recoverRefusal(ctx context.Context, b *wakeBox, run RunRow, reason domain.WakeReason) (string, error) {
	s := w.s
	if r, err := s.kindRefusal(ctx, b, run.AgentID, run.ThreadID, run.TriggeredBy, domain.WakeAt, false); err != nil || r != "" {
		return r, err
	}
	var activeAgent ids.ID
	active, found, err := s.d.Repo.ActiveRunOfThread(ctx, run.ThreadID)
	if err != nil {
		return "", err
	}
	if found {
		activeAgent = active.AgentID
	}
	denial, err := s.wakeInvokeDenial(ctx, run.AgentID, run.ThreadID, run.TriggeredBy, activeAgent)
	if err != nil || denial != "" {
		return denial, err
	}
	return w.capRefusal(ctx, b, run.ThreadID, run.ID, reason)
}

// ListPending implements runs.Wakes (listPendingRunWakes, run.wake.list_pending): agent-self, then
// every pending wake-up of the calling agent, ascending by id.
func (w *Wakes) ListPending(ctx context.Context) ([]runs.Wake, error) {
	s := w.s
	b, err := s.wakeBox()
	if err != nil {
		return nil, err
	}
	a := caller(ctx)
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionRunWakeListPending, authorize.Resource{Kind: authorize.ResourceAgent, ID: a.ID, OwnerID: a.ID})
	if err != nil {
		return nil, err
	}
	if !d.Allowed() {
		return nil, d.Err()
	}
	rows, err := b.d.Repo.PendingWakesOfAgent(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	out := make([]runs.Wake, 0, len(rows))
	for _, r := range rows {
		out = append(out, wakeView(r))
	}
	return out, nil
}

// lockWake locks the wake-up id and authorizes act for the agent a on it (agent-self of the
// wake-up's agent; resource the source run with the wake-up's agent as owner). An unknown
// wake-up answers the same not_found as one of another agent.
func (s *Service) lockWake(ctx context.Context, b *wakeBox, a actor.Actor, act authorize.Action, id ids.ID) (WakeRow, error) {
	r, found, err := b.d.Repo.LockWake(ctx, id)
	if err != nil {
		return WakeRow{}, err
	}
	res := authorize.Resource{Kind: authorize.ResourceRun, ID: id, ThreadID: id}
	if found {
		res = authorize.Resource{Kind: authorize.ResourceRun, ID: r.SourceRunID, ThreadID: r.ThreadID, OwnerID: r.AgentID}
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, act, res)
	if err != nil {
		return WakeRow{}, err
	}
	if d.Outcome == authorize.OutcomeNotFound || (d.Allowed() && !found) {
		return WakeRow{}, invisible()
	}
	if !d.Allowed() {
		return WakeRow{}, d.Err()
	}
	return r, nil
}

// Check implements runs.Wakes (checkRunWake, run.wake.check; 09 5.8.3, R-AG-50): in one unit of
// work the wake-up is locked and authorized; an ended one is 409 wake_not_pending, one before its
// not_before 409 wake_not_due (it stays pending); then the checks of checkRefusal; a failed check
// ends the wake-up refused with the audit entry naming the check, the note and the notification,
// and 409 follows after the commit; an allow answers the configured tool class and billing mode.
func (w *Wakes) Check(ctx context.Context, wakeID ids.ID) (runs.WakeCheck, error) {
	s := w.s
	b, err := s.wakeBox()
	if err != nil {
		return runs.WakeCheck{}, err
	}
	a := caller(ctx)
	var res wakeResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res, err = w.checkTx(ctx, b, a, wakeID)
		return err
	})
	return res.checkAnswer(err)
}

// checkAnswer turns the result of a unit of work into the answer of a check.
func (r wakeResult) checkAnswer(err error) (runs.WakeCheck, error) {
	if err != nil {
		return runs.WakeCheck{}, err
	}
	if r.committed != nil {
		return runs.WakeCheck{}, r.committed
	}
	return r.check, nil
}

// checkTx is Check inside its unit of work.
func (w *Wakes) checkTx(ctx context.Context, b *wakeBox, a actor.Actor, wakeID ids.ID) (wakeResult, error) {
	s := w.s
	r, err := s.lockWake(ctx, b, a, authorize.ActionRunWakeCheck, wakeID)
	if err != nil {
		return wakeResult{}, err
	}
	switch {
	case r.State != domain.WakePending:
		return wakeResult{}, conflict(domain.WakeCodeNotPending)
	case s.now().Before(r.NotBefore):
		return wakeResult{}, conflict(domain.WakeCodeNotDue)
	}
	reason, err := s.checkRefusal(ctx, b, r)
	if err != nil {
		return wakeResult{}, err
	}
	if reason == "" {
		check, err := s.configured(ctx, r.AgentID)
		return wakeResult{check: check}, err
	}
	if err := s.auditWake(ctx, a, authorize.ActionRunWakeCheck, reason, r.SourceRunID, r.ID, r.Kind, r.Reason, reason); err != nil {
		return wakeResult{}, err
	}
	if _, err := s.endWake(ctx, b, r, domain.WakeRefused, reason, WakeWrite{}); err != nil {
		return wakeResult{}, err
	}
	return wakeResult{committed: conflict(reason)}, nil
}

// configured returns the configured tool class and billing mode of the agent (compare_actual).
func (s *Service) configured(ctx context.Context, agentID ids.ID) (runs.WakeCheck, error) {
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{agentID})
	if err != nil {
		return runs.WakeCheck{}, err
	}
	f := facts[agentID]
	return runs.WakeCheck{ToolClass: f.ToolClass, BillingMode: f.BillingMode}, nil
}

// checkRefusal returns the reason of the first failed check before a host execution (09 5.8.3):
// the autonomy rule and the rules of the kind (command: tool class host; pr_checks: an owner), then
// agent.invoke of the person without the active run (trigger, reachability, archived or read-only
// channel, SR2), with the reasons of ignoredAtCheck left to the fire.
func (s *Service) checkRefusal(ctx context.Context, b *wakeBox, r WakeRow) (string, error) {
	if reason, err := s.kindRefusal(ctx, b, r.AgentID, r.ThreadID, r.TriggeredBy, r.Kind, true); err != nil || reason != "" {
		return reason, err
	}
	denial, err := s.wakeInvokeDenial(ctx, r.AgentID, r.ThreadID, r.TriggeredBy, ids.ID{})
	if err != nil || denial == "" || slices.Contains(ignoredAtCheck, denial) {
		return "", err
	}
	return denial, nil
}

// Fire implements runs.Wakes (fireRunWake, run.wake.fire; 09 5.8.2, 5.8.3): fired stores the result
// and fires the wake-up with fireLocked; failed ends it failed with the reason; input_skipped puts a
// wake-up that fired into the named run back to pending, once per delivery, so it fires after that
// run as a wake run. Every allowed call is audited.
func (w *Wakes) Fire(ctx context.Context, in runs.FireWake) (runs.Wake, error) {
	s := w.s
	b, err := s.wakeBox()
	if err != nil {
		return runs.Wake{}, err
	}
	outcome := domain.FireOutcome(in.Outcome)
	switch {
	case !outcome.Valid():
		return runs.Wake{}, fieldError("/outcome", "invalid", "unknown outcome")
	case outcome == domain.FireInputSkipped && in.RunID.IsZero():
		return runs.Wake{}, fieldError("/run_id", "required", "a skipped wake input names its run")
	case outcome == domain.FireFired:
		if err := domain.CheckResultText(in.ResultText, s.cfg.MaxContentBytes); err != nil {
			return runs.Wake{}, err
		}
	case outcome == domain.FireFailed, outcome == domain.FireInputSkipped:
	}
	a := caller(ctx)
	var res wakeResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res, err = w.fireTx(ctx, b, a, in, outcome)
		return err
	})
	if err != nil {
		return runs.Wake{}, err
	}
	if res.committed != nil {
		return runs.Wake{}, res.committed
	}
	return wakeView(res.wake), nil
}

// fireTx is Fire inside its unit of work. The thread's active run is locked before the wake-up
// (lock order of 03 6.3: runs before run_wakes), so a fire never waits on a delivery that holds
// the run and waits on the wake-up.
func (w *Wakes) fireTx(ctx context.Context, b *wakeBox, a actor.Actor, in runs.FireWake, outcome domain.FireOutcome) (wakeResult, error) {
	s := w.s
	active, err := s.lockThreadRunOfWake(ctx, b, in.WakeID)
	if err != nil {
		return wakeResult{}, err
	}
	r, err := s.lockWake(ctx, b, a, authorize.ActionRunWakeFire, in.WakeID)
	if err != nil {
		return wakeResult{}, err
	}
	if outcome == domain.FireInputSkipped {
		return w.returnSkipped(ctx, b, a, r, in.RunID)
	}
	switch {
	case r.State != domain.WakePending:
		return wakeResult{}, conflict(domain.WakeCodeNotPending)
	case r.Kind == domain.WakeAt:
		return wakeResult{}, conflict(domain.WakeCodeFiredByServer)
	case outcome == domain.FireFailed:
		reason := in.Reason
		if reason == "" {
			reason = domain.WakeCodeCheckFailed
		}
		if err := s.auditWake(ctx, a, authorize.ActionRunWakeFire, "", r.SourceRunID, r.ID, r.Kind, r.Reason, reason); err != nil {
			return wakeResult{}, err
		}
		row, err := s.endWake(ctx, b, r, domain.WakeFailed, reason, WakeWrite{})
		return wakeResult{wake: row}, err
	case s.now().Before(r.NotBefore):
		return wakeResult{}, conflict(domain.WakeCodeNotDue)
	}
	// The condition holds: the result and the mark stay with the pending wake-up, so a fire that
	// must wait for another person's run still fires later with the result (job runs.wake_due). A
	// wake-up returned by a skipped input keeps wake_input_skipped, so a later fire of the runner's
	// check waits for the end of that run instead of firing into it again (09 5.8.3).
	reason := domain.WakeCodeConditionMet
	if r.StateReason == domain.WakeCodeInputSkipped {
		reason = domain.WakeCodeInputSkipped
	}
	row, changed, err := b.d.Repo.UpdateWake(ctx, WakeWrite{ID: r.ID, Version: r.Version, State: domain.WakePending,
		StateReason: reason, ResultText: in.ResultText})
	if err != nil {
		return wakeResult{}, err
	}
	if !changed {
		return wakeResult{}, errs.New(errs.CodeInternalError, "runs: a locked wake-up changed")
	}
	if err := s.auditWake(ctx, a, authorize.ActionRunWakeFire, "", r.SourceRunID, r.ID, r.Kind, r.Reason, ""); err != nil {
		return wakeResult{}, err
	}
	row, err = s.fireLocked(ctx, b, row, active)
	return wakeResult{wake: row}, err
}

// threadRun is the active run of a wake-up's thread, locked before the wake-up; found is false
// for a thread without one.
type threadRun struct {
	row   RunRow
	found bool
}

// lockThreadRunOfWake reads the wake-up id without a lock and locks the active run of its
// thread; an unknown wake-up gives no run (the locked read that follows answers it).
func (s *Service) lockThreadRunOfWake(ctx context.Context, b *wakeBox, id ids.ID) (threadRun, error) {
	r, found, err := b.d.Repo.Wake(ctx, id)
	if err != nil || !found {
		return threadRun{}, err
	}
	row, found, err := s.d.Repo.LockActiveRunOfThread(ctx, r.ThreadID)
	return threadRun{row: row, found: found}, err
}

// returnSkipped puts a wake-up that fired as a wake input into runID back to pending with
// wake_input_skipped (09 5.8.3) and schedules runs.wake_due, which fires it as a wake run once the
// thread has no active run; any other state answers unchanged, so a repeated report of one delivery
// changes it once.
func (w *Wakes) returnSkipped(ctx context.Context, b *wakeBox, a actor.Actor, r WakeRow, runID ids.ID) (wakeResult, error) {
	s := w.s
	if r.State != domain.WakeFired || r.FiredIntoRunID != runID {
		return wakeResult{wake: r}, nil
	}
	row, err := s.reopenWake(ctx, b, r, domain.WakeCodeInputSkipped)
	if err != nil {
		return wakeResult{}, err
	}
	return wakeResult{wake: row}, s.auditWake(ctx, a, authorize.ActionRunWakeFire, "", r.SourceRunID, r.ID, r.Kind, r.Reason, domain.WakeCodeInputSkipped)
}

// reopenWake puts the locked wake-up r back to pending with reason, clears the fired columns,
// appends run.wake_changed and schedules runs.wake_due now.
func (s *Service) reopenWake(ctx context.Context, b *wakeBox, r WakeRow, reason string) (WakeRow, error) {
	row, changed, err := b.d.Repo.UpdateWake(ctx, WakeWrite{ID: r.ID, Version: r.Version, State: domain.WakePending, StateReason: reason, ClearFired: true})
	if err != nil {
		return WakeRow{}, err
	}
	if !changed {
		return WakeRow{}, errs.New(errs.CodeInternalError, "runs: a locked wake-up changed")
	}
	if err := s.emitWakeChanged(ctx, row, noteRef{}); err != nil {
		return WakeRow{}, err
	}
	return row, b.d.Schedule.ScheduleWakeDue(ctx, row.ID, s.now())
}

// fireLocked fires the locked pending wake-up r (09 5.8.3): past its deadline it expires; with an
// active run in the thread it is a wake input when that run runs for the same person and agent and
// the wake-up was not returned by a skipped input, and stays pending otherwise (the end of that run
// schedules runs.wake_due); without one it starts a wake run. Both paths run the checks of a new
// run and the caps first.
func (s *Service) fireLocked(ctx context.Context, b *wakeBox, r WakeRow, active threadRun) (WakeRow, error) {
	if !s.now().Before(r.Deadline) {
		return s.endWake(ctx, b, r, domain.WakeExpired, domain.WakeCodeDeadline, WakeWrite{})
	}
	if !active.found {
		return s.fireRun(ctx, b, r)
	}
	run := active.row
	if run.State != domain.StateRunning || run.TriggeredBy != r.TriggeredBy || run.AgentID != r.AgentID ||
		r.StateReason == domain.WakeCodeInputSkipped {
		return r, nil
	}
	return s.fireInput(ctx, b, r, run)
}

// fireRefusal applies the checks of a fire (09 5.8.3, 5.8.6): the autonomy rule and pr_checks for
// an owner, agent.invoke of the person with the active run's agent, then the caps of the person in
// the thread. It returns the reason and the counts read under their row lock.
func (s *Service) fireRefusal(ctx context.Context, b *wakeBox, r WakeRow, activeAgent ids.ID) (string, domain.Autonomy, error) {
	if reason, err := s.kindRefusal(ctx, b, r.AgentID, r.ThreadID, r.TriggeredBy, r.Kind, false); err != nil || reason != "" {
		return reason, domain.Autonomy{}, err
	}
	denial, err := s.wakeInvokeDenial(ctx, r.AgentID, r.ThreadID, r.TriggeredBy, activeAgent)
	if err != nil || denial != "" {
		return denial, domain.Autonomy{}, err
	}
	a, _, err := b.d.Repo.LockAutonomy(ctx, r.ThreadID, r.TriggeredBy)
	if err != nil {
		return "", domain.Autonomy{}, err
	}
	if reason := domain.AutonomyRefusal(a, b.autonomyLimits()); reason != "" {
		if reason == domain.WakeCodeChainMax {
			m := b.d.Metrics
			txn.AfterCommit(ctx, func() { m.AutonomyCapReached(context.WithoutCancel(ctx), reason) })
		}
		return reason, a, nil
	}
	return "", a, nil
}

// refuseFire ends the wake-up refused with the reason of a failed fire check, audited with the
// check principal of the person (07 2.14 item 3), with note and notification.
func (s *Service) refuseFire(ctx context.Context, b *wakeBox, r WakeRow, reason string) (WakeRow, error) {
	if err := s.auditWake(ctx, actor.Recheck(r.TriggeredBy), authorize.ActionRunWakeFire, reason, r.SourceRunID, r.ID, r.Kind, r.Reason, reason); err != nil {
		return WakeRow{}, err
	}
	return s.endWake(ctx, b, r, domain.WakeRefused, reason, WakeWrite{})
}

// fireRun starts the wake run of r (09 5.8.3): the checks, then Open with trigger_kind wake and the
// wake-up's person, agent.invocation with the wake-up's facts, one more autonomous input, and the
// wake-up fired with fired_run_id. A thread that got an active run meanwhile keeps the wake-up
// pending.
func (s *Service) fireRun(ctx context.Context, b *wakeBox, r WakeRow) (WakeRow, error) {
	reason, a, err := s.fireRefusal(ctx, b, r, ids.ID{})
	if err != nil {
		return WakeRow{}, err
	}
	if reason != "" {
		return s.refuseFire(ctx, b, r, reason)
	}
	cfg, err := s.configured(ctx, r.AgentID)
	if err != nil {
		return WakeRow{}, err
	}
	sum, err := s.Open(ctx, OpenCmd{
		ThreadID: r.ThreadID, AgentID: r.AgentID, TriggeredBy: r.TriggeredBy, TriggeringWakeID: r.ID, TriggerKind: triggerKindWake,
		BillingModeExpected: cfg.BillingMode,
	})
	var exists *ActiveRunExistsError
	if errors.As(err, &exists) {
		return r, nil
	}
	if err != nil {
		return WakeRow{}, err
	}
	if err := s.emitWakeInvocation(ctx, sum, r, cfg.BillingMode); err != nil {
		return WakeRow{}, err
	}
	if err := b.d.Repo.SaveAutonomy(ctx, r.ThreadID, r.TriggeredBy, domain.AfterInput(a)); err != nil {
		return WakeRow{}, err
	}
	return s.endWake(ctx, b, r, domain.WakeFired, "", WakeWrite{FiredRunID: sum.ID, ResultText: r.ResultText})
}

// fireInput delivers r as a wake input into the running run of its person (09 5.8.3, 04 15.9):
// the checks with the run's agent, then agent.invoke's durable personal event agent.wake_input to
// the agent (the delivery adds the token of trigger kind wake), one more autonomous input, and the
// wake-up fired with fired_into_run_id.
func (s *Service) fireInput(ctx context.Context, b *wakeBox, r WakeRow, active RunRow) (WakeRow, error) {
	reason, a, err := s.fireRefusal(ctx, b, r, active.AgentID)
	if err != nil {
		return WakeRow{}, err
	}
	if reason != "" {
		return s.refuseFire(ctx, b, r, reason)
	}
	info, err := s.d.Threads.Info(ctx, active.ThreadID)
	if err != nil {
		return WakeRow{}, err
	}
	p, err := domain.WakeInputPayload(active.ID, active.ThreadID, wakeFacts(r))
	if err != nil {
		return WakeRow{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.wake_input")
	}
	kind, actorID := eventActor(ctx)
	if err := s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventAgentWakeInput), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: active.ThreadID, ResourceKind: resourceRun, ResourceID: active.ID,
		AudiencePrincipalID: active.AgentID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	}); err != nil {
		return WakeRow{}, err
	}
	if err := b.d.Repo.SaveAutonomy(ctx, r.ThreadID, r.TriggeredBy, domain.AfterInput(a)); err != nil {
		return WakeRow{}, err
	}
	return s.endWake(ctx, b, r, domain.WakeFired, "", WakeWrite{FiredIntoRunID: active.ID, ResultText: r.ResultText})
}

// wakeFacts returns the members of the wake-up in the payloads of a wake run and a wake input.
func wakeFacts(r WakeRow) domain.WakeFacts {
	return domain.WakeFacts{ID: r.ID, Kind: r.Kind, Reason: r.Reason, Label: r.Label, ResultText: r.ResultText}
}

// Cancel implements runs.Wakes (cancelRunWake, run.wake.cancel; 09 5.8.2): the agent of the
// wake-up, its person or a person with manage on the channel; a pending wake-up ends cancelled with
// the note and no notification, one that already ended answers unchanged. Audited.
func (w *Wakes) Cancel(ctx context.Context, in runs.CancelWake) (runs.Wake, error) {
	s := w.s
	b, err := s.wakeBox()
	if err != nil {
		return runs.Wake{}, err
	}
	if in.Reason != "" && !domain.IsReasonCode(in.Reason) {
		return runs.Wake{}, fieldError("/reason", "invalid", "the reason is not a code")
	}
	a := caller(ctx)
	var out WakeRow
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = w.cancelTx(ctx, b, a, in)
		return err
	})
	if err != nil {
		return runs.Wake{}, err
	}
	return wakeView(out), nil
}

// cancelTx is Cancel inside its unit of work.
func (w *Wakes) cancelTx(ctx context.Context, b *wakeBox, a actor.Actor, in runs.CancelWake) (WakeRow, error) {
	s := w.s
	r, found, err := b.d.Repo.LockWake(ctx, in.WakeID)
	if err != nil {
		return WakeRow{}, err
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionRunWakeCancel, cancelResource(a, r, found, in.WakeID))
	if err != nil {
		return WakeRow{}, err
	}
	if d.Outcome == authorize.OutcomeNotFound || (d.Allowed() && !found) {
		return WakeRow{}, invisible()
	}
	if !d.Allowed() {
		return WakeRow{}, d.Err()
	}
	if r.State != domain.WakePending {
		return r, nil
	}
	out, err := s.endWake(ctx, b, r, domain.WakeCancelled, cancelReason(a, in.Reason), WakeWrite{})
	if err != nil {
		return WakeRow{}, err
	}
	return out, s.auditWake(ctx, a, authorize.ActionRunWakeCancel, "", r.SourceRunID, r.ID, r.Kind, r.Reason, "")
}

// cancelResource is the resource of run.wake.cancel: the thread of the wake-up with its agent as
// owner for an agent caller and its person for a person, so AgentSelf and Self each name their
// subject; an unknown wake-up a path that cannot resolve.
func cancelResource(a actor.Actor, r WakeRow, found bool, id ids.ID) authorize.Resource {
	if !found {
		return authorize.Resource{Kind: authorize.ResourceThread, ID: id}
	}
	owner := r.TriggeredBy
	if a.Kind == actor.KindAgent {
		owner = r.AgentID
	}
	return authorize.Resource{Kind: authorize.ResourceThread, ID: r.ThreadID, OwnerID: owner}
}

// cancelReason is the stored reason of a cancel: the agent's code (cancelled_by_agent without one),
// cancelled_by_person for a person.
func cancelReason(a actor.Actor, reason string) string {
	if a.Kind != actor.KindAgent {
		return domain.WakeCodeCancelledByPerson
	}
	if reason == "" {
		return domain.WakeCodeCancelledByAgent
	}
	return reason
}

// wakeKey is the cursor key of listThreadWakes.
type wakeKey struct {
	CreatedAt time.Time `json:"c"`
	ID        ids.ID    `json:"i"`
}

// ListByThread implements runs.Wakes (listThreadWakes, run.wake.read): the thread must be visible
// with read (Scope of run.wake.read), then a keyset page, newest first, with a cursor bound to the
// thread and the principal.
func (w *Wakes) ListByThread(ctx context.Context, q runs.ThreadWakesQuery) (runs.WakePage, error) {
	s := w.s
	b, err := s.wakeBox()
	if err != nil {
		return runs.WakePage{}, err
	}
	a := caller(ctx)
	threadID, err := s.visibleThread(ctx, a, authorize.ActionRunWakeRead, q.ThreadID)
	if err != nil {
		return runs.WakePage{}, err
	}
	binding := []byte(a.ID.String() + "|" + threadID.String())
	after, err := s.wakeCursor(binding, q.Cursor)
	if err != nil {
		return runs.WakePage{}, err
	}
	limit := s.pageSize(q.Limit)
	rows, err := b.d.Repo.ThreadWakes(ctx, threadID, after, limit+1)
	if err != nil {
		return runs.WakePage{}, err
	}
	out := runs.WakePage{Items: make([]runs.Wake, 0, min(len(rows), limit))}
	for _, r := range rows[:min(len(rows), limit)] {
		out.Items = append(out.Items, wakeView(r))
	}
	if len(rows) > limit {
		last := rows[limit-1]
		if out.NextCursor, err = s.d.Cursors.Encode(listThreadWakes, binding, wakeKey{CreatedAt: last.CreatedAt, ID: last.ID}); err != nil {
			return runs.WakePage{}, errs.Wrap(errs.CodeInternalError, err, "runs: wake-up list cursor")
		}
	}
	return out, nil
}

// visibleThread returns the thread id when its channel is in the Scope of act for a and it is not
// deleted; an unknown, invisible or deleted thread is the same not_found.
func (s *Service) visibleThread(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID) (ids.ID, error) {
	sc, err := s.d.Authorizer.Scope(ctx, a, act)
	if err != nil {
		return ids.ID{}, err
	}
	info, err := s.d.Threads.Info(ctx, id)
	if errs.KindOf(err) == errs.KindNotFound {
		return ids.ID{}, invisible()
	}
	if err != nil {
		return ids.ID{}, err
	}
	if _, ok := slices.BinarySearchFunc(sc.ChannelIDs, info.ChannelID, compareIDs); !ok || info.Deleted {
		return ids.ID{}, invisible()
	}
	return info.ID, nil
}

// wakeCursor decodes the cursor of listThreadWakes; nil for the first page.
func (s *Service) wakeCursor(binding []byte, cursor string) (*WakeKey, error) {
	if cursor == "" {
		return nil, nil
	}
	var k wakeKey
	if err := s.d.Cursors.Decode(listThreadWakes, binding, cursor, &k); err != nil {
		return nil, err
	}
	return &WakeKey{CreatedAt: k.CreatedAt, ID: k.ID}, nil
}

// LaunchCheck implements runs.Wakes (checkRunLaunch, run.launch.check; 09 18.19.6, R-AG-48):
// agent-self of the run's agent, then the run is running and agent.invoke of its person allows the
// agent in the thread now (trigger, reachability, channel, I2, SR2, project); a refusal is audited
// as a denial and answers 409 with its reason; the run is not changed (the runner finishes it).
func (w *Wakes) LaunchCheck(ctx context.Context, runID ids.ID) (runs.WakeCheck, error) {
	s := w.s
	a := caller(ctx)
	var res wakeResult
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res, err = w.launchTx(ctx, a, runID)
		return err
	})
	return res.checkAnswer(err)
}

// launchTx is LaunchCheck inside its unit of work.
func (w *Wakes) launchTx(ctx context.Context, a actor.Actor, runID ids.ID) (wakeResult, error) {
	s := w.s
	run, found, err := s.d.Repo.Run(ctx, runID)
	if err != nil {
		return wakeResult{}, err
	}
	d, err := s.authorizeAgent(ctx, a, authorize.ActionRunLaunchCheck, run, found, runID, nil)
	if err != nil {
		return wakeResult{}, err
	}
	if !d.Allowed() {
		return wakeResult{}, d.Err()
	}
	reason := domain.WakeCodeRunNotRunning
	if run.State == domain.StateRunning {
		if reason, err = s.wakeInvokeDenial(ctx, run.AgentID, run.ThreadID, run.TriggeredBy, run.AgentID); err != nil {
			return wakeResult{}, err
		}
	}
	if reason == "" {
		check, err := s.configured(ctx, run.AgentID)
		return wakeResult{check: check}, err
	}
	if s.d.Audit == nil {
		return wakeResult{}, errs.New(errs.CodeInternalError, "runs: the launch check has no audit recorder")
	}
	return wakeResult{committed: conflict(reason)}, s.d.Audit.Record(actor.WithActor(ctx, a), auditing.Entry{
		Action: string(authorize.ActionRunLaunchCheck), Decision: auditing.DecisionDeny, Reason: auditing.Reason(reason),
		ResourceKind: resourceRun, ResourceID: run.ID,
	})
}

// WakeDue is the use case of the job runs.wake_due (09 5.8.4) in one unit of work: the wake-up is
// locked; an ended one is nothing; past its deadline it expires; an at wake-up before its
// not_before schedules the job again; a pr_checks or command wake-up fires only after its runner
// reported the condition (condition_met) or a skipped wake input returned it; then fireLocked.
// Errors: those of the database and of the collaborators; an unknown wake-up is nothing.
func (s *Service) WakeDue(ctx context.Context, wakeID ids.ID) error {
	b, err := s.wakeBox()
	if err != nil {
		return err
	}
	// The job is the server's own step without a principal: ctx carries no actor, so the run
	// anchor and the notes are written as the agent (messaging, chat).
	return s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.wakeDueTx(ctx, b, wakeID) })
}

// wakeDueTx is the unit of work of WakeDue: the thread's run before the wake-up (03 6.3).
func (s *Service) wakeDueTx(ctx context.Context, b *wakeBox, wakeID ids.ID) error {
	active, err := s.lockThreadRunOfWake(ctx, b, wakeID)
	if err != nil {
		return err
	}
	r, found, err := b.d.Repo.LockWake(ctx, wakeID)
	if err != nil || !found || r.State != domain.WakePending {
		return err
	}
	now := s.now()
	if !now.Before(r.Deadline) {
		_, err := s.endWake(ctx, b, r, domain.WakeExpired, domain.WakeCodeDeadline, WakeWrite{})
		return err
	}
	if now.Before(r.NotBefore) {
		if r.Kind == domain.WakeAt {
			return b.d.Schedule.ScheduleWakeDue(ctx, r.ID, r.NotBefore)
		}
		return nil
	}
	if r.Kind != domain.WakeAt && r.StateReason != domain.WakeCodeConditionMet && r.StateReason != domain.WakeCodeInputSkipped &&
		r.StateReason != domain.WakeCodeRunNotRunning {
		return nil
	}
	_, err = s.fireLocked(ctx, b, r, active)
	return err
}

// onThreadFree schedules runs.wake_due now for every pending wake-up of the thread that could fire
// (an ended run frees the thread, 09 5.8.3: a wake-up that waited for another person's run, or a
// skipped wake input, fires after it). Without bound wake-ups nothing happens.
func (s *Service) onThreadFree(ctx context.Context, threadID ids.ID) error {
	b := s.wakes.Load()
	if b == nil {
		return nil
	}
	now := s.now()
	ready, err := b.d.Repo.ReadyWakesOfThread(ctx, threadID, now)
	if err != nil {
		return err
	}
	for _, id := range ready {
		if err := b.d.Schedule.ScheduleWakeDue(ctx, id, now); err != nil {
			return err
		}
	}
	return nil
}

// resetAutonomy sets the counts of the person in the thread to 0 (03 run_autonomy): a run of the
// person that is not a wake run, or a follow-up of the person, in the transaction that creates it.
func (s *Service) resetAutonomy(ctx context.Context, threadID, personID ids.ID) error {
	b := s.wakes.Load()
	if b == nil {
		return nil
	}
	return b.d.Repo.ResetAutonomy(ctx, threadID, personID)
}

// idleReport applies the idle flag of a stored /metrics/autonomy_last (09 5.8.6): each distinct
// input counts once in the idle count of the run's thread and person; reaching
// run.autonomous_idle_max ends the chain with autonomous_idle_max and a note. It reports whether
// the chain is ended after the report (the answer's autonomy_cap_reached).
func (s *Service) idleReport(ctx context.Context, row RunRow, before, after []byte) (bool, error) {
	b := s.wakes.Load()
	if b == nil {
		return false, nil
	}
	last, ok := domain.AutonomyLastOf(after)
	if !ok {
		return false, nil
	}
	if prev, had := domain.AutonomyLastOf(before); had && prev.Input == last.Input {
		a, _, err := b.d.Repo.LockAutonomy(ctx, row.ThreadID, row.TriggeredBy)
		return a.EndedReason != "", err
	}
	a, _, err := b.d.Repo.LockAutonomy(ctx, row.ThreadID, row.TriggeredBy)
	if err != nil {
		return false, err
	}
	next, reached := domain.AfterIdleReport(a, last.Idle, b.autonomyLimits())
	if err := b.d.Repo.SaveAutonomy(ctx, row.ThreadID, row.TriggeredBy, next); err != nil {
		return false, err
	}
	if reached {
		if _, _, err := b.d.Notes.AppendSystemNote(ctx, row.ThreadID, row.AgentID, domain.AutonomyNoteText(domain.WakeCodeIdleMax)); err != nil {
			return false, err
		}
		m := b.d.Metrics
		txn.AfterCommit(ctx, func() { m.AutonomyCapReached(context.WithoutCancel(ctx), domain.WakeCodeIdleMax) })
	}
	return next.EndedReason != "", nil
}

// conditionOf converts the condition of the port.
func conditionOf(c runs.WakeCondition) domain.WakeCondition {
	return domain.WakeCondition{Repository: c.Repository, Number: c.Number, HeadCommit: c.HeadCommit, CommandLine: c.CommandLine}
}

// wakeView converts a row (schema RunWake).
func wakeView(r WakeRow) runs.Wake {
	c := domain.DecodeCondition(r.Condition)
	return runs.Wake{
		ID: r.ID, AgentID: r.AgentID, ThreadID: r.ThreadID, SourceRunID: r.SourceRunID, TriggeredBy: r.TriggeredBy, Kind: string(r.Kind),
		Reason: string(r.Reason), State: string(r.State), StateReason: r.StateReason, Label: r.Label,
		Condition: runs.WakeCondition{Repository: c.Repository, Number: c.Number, HeadCommit: c.HeadCommit, CommandLine: c.CommandLine},
		Outage:    r.Outage, NotBefore: r.NotBefore, Deadline: r.Deadline, CreatedAt: r.CreatedAt, EndedAt: r.EndedAt,
		ResultText: r.ResultText, FiredRunID: r.FiredRunID, FiredIntoRunID: r.FiredIntoRunID, Version: r.Version,
	}
}

// emitWakeInvocation appends agent.invocation of the wake run sum (09 5.8.5, 04 15.9): audience the
// agent, receive action agent.events.consume, resource the run without version, the scope of the
// thread and the payload AgentInvocationPayload with trigger_kind wake, wake_id and the wake-up's
// facts instead of a message, the wake-up's person as triggered_by and the session of the run (the
// runner resumes the thread's session, 09 section 9). The token is a delivery field that Prepare
// adds per delivery.
func (s *Service) emitWakeInvocation(ctx context.Context, sum runs.RunSummary, r WakeRow, mode string) error {
	info, err := s.d.Threads.Info(ctx, sum.ThreadID)
	if err != nil {
		return err
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
	if err != nil {
		return err
	}
	refs, err := s.d.Messages.PrincipalRefs(ctx, []ids.ID{r.TriggeredBy})
	if err != nil {
		return err
	}
	ref, ok := refs[r.TriggeredBy]
	if !ok {
		return errs.New(errs.CodeInternalError, "runs: the person of a wake-up is unknown")
	}
	session, err := s.invocationSession(ctx, sum, operatorOf(info, ch))
	if err != nil {
		return err
	}
	facts := wakeFacts(r)
	p, err := domain.InvocationPayload(domain.InvocationPayloadInput{
		RunID: sum.ID, ThreadID: sum.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, TriggerKind: triggerKindWake, Wake: &facts,
		TriggeredBy: domain.PrincipalRefPayload{
			ID: ref.ID.String(), Kind: ref.Kind, Handle: ref.Handle, DisplayName: ref.DisplayName, AvatarFileID: optionalID(ref.AvatarFileID),
		},
		BillingMode: mode, SettingsVersion: ch.SettingsVersion,
	}, session)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.invocation of a wake run")
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventAgentInvocation), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: sum.ThreadID, ResourceKind: resourceRun, ResourceID: sum.ID,
		AudiencePrincipalID: sum.AgentID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	})
}

// resolveInvocationWake is agent.invocation with trigger_kind wake (09 5.8.3): the trigger is the
// wake-up's person (runs.triggered_by), the triggering object the wake-up, the agent counted as
// addressed; a wake run whose wake-up was deleted with its thread cannot carry a valid token and is
// gone.
func resolveInvocationWake(_ *Service, _ context.Context, run RunRow, _ runs.DeliveryInput) (trigger, error) {
	if run.TriggeringWakeID.IsZero() {
		return trigger{gone: true}, nil
	}
	return trigger{person: run.TriggeredBy, object: run.TriggeringWakeID, origin: originHuman, mentions: true, wake: run.TriggeringWakeID}, nil
}

// resolveWakeInput is agent.wake_input (09 5.8.3): the trigger is the run's person, which the fire
// required to be the wake-up's person, the triggering object the wake-up of the payload.
func resolveWakeInput(_ *Service, _ context.Context, run RunRow, in runs.DeliveryInput) (trigger, error) {
	id, err := domain.WakeInputWakeID(in.Payload)
	if err != nil {
		return trigger{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.wake_input")
	}
	return trigger{person: run.TriggeredBy, object: id, origin: originHuman, mentions: true, wake: id}, nil
}

// returnWakeInput puts the wake-up of an agent.wake_input whose run no longer runs at its delivery
// back to pending (09 5.8.3): it fired into that run, so it fires after it as a wake run. Nothing
// happens without bound wake-ups or when the wake-up moved on.
func (s *Service) returnWakeInput(ctx context.Context, run RunRow, payload []byte) error {
	b := s.wakes.Load()
	if b == nil {
		return nil
	}
	id, err := domain.WakeInputWakeID(payload)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.wake_input")
	}
	r, found, err := b.d.Repo.LockWake(ctx, id)
	if err != nil || !found || r.State != domain.WakeFired || r.FiredIntoRunID != run.ID {
		return err
	}
	_, err = s.reopenWake(ctx, b, r, domain.WakeCodeRunNotRunning)
	return err
}

// refuseWakeInput ends the wake-up of a wake input whose delivery agent.invoke refused (domain.
// DenyRefuseWake): refused with the mapped reason, the note and the notification; the run is
// unchanged. The denial's audit entry follows in deny.
func (s *Service) refuseWakeInput(ctx context.Context, wakeID ids.ID, reason string) error {
	b := s.wakes.Load()
	if b == nil {
		return nil
	}
	r, found, err := b.d.Repo.LockWake(ctx, wakeID)
	if err != nil || !found || r.State != domain.WakeFired {
		return err
	}
	_, err = s.endWake(ctx, b, r, domain.WakeRefused, reason, WakeWrite{})
	return err
}
