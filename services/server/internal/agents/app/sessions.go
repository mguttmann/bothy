// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// SessionConfig holds the configuration of the session use cases, bound in internal/wire
// (wire.SessionsConfig).
type SessionConfig struct {
	// SessionRefMaxBytes is agent.session_ref_max_bytes, the longest external session reference
	// in bytes (M2-12 design note 13).
	SessionRefMaxBytes int `config:"agent.session_ref_max_bytes"`
}

// ThreadInfos is the part of chat.Threads the sessions read: the routing data of a thread without
// an authorization (unknown gives not_found).
type ThreadInfos interface {
	Info(ctx context.Context, threadID ids.ID) (chat.ThreadInfo, error)
}

// SessionDeps are the collaborators of the session use cases.
type SessionDeps struct {
	// Store reads and writes agent_sessions.
	Store SessionStore
	// Threads gives the routing data of a thread (chat).
	Threads ThreadInfos
	// Channels gives the effective operator of a channel (chat, 03 R-DM-21).
	Channels chat.TriggerFacts
	// Runs is the declared source of the active run of a thread (runs implements it); nil is the
	// empty source, which answers "no run" (02 section 4.4).
	Runs agents.RunSource
	// UoW runs every operation; the calls of runs join the caller's unit of work.
	UoW txn.UnitOfWork
	// Authorizer decides the four actions agent_session.*.
	Authorizer authorize.Authorizer
	// Audit writes the allow entry of agent_session.bind.
	Audit auditing.Recorder
	// Events appends agent_session.changed.
	Events eventlog.Appender
	// Clock gives the times of the rows; nil means clock.System().
	Clock clock.Clock
	// Log receives the line of a start whose reference could not be attached; nil means
	// slog.Default().
	Log *slog.Logger
}

// SessionService implements agents.Sessions (M2-12, docs/spec/09-agenten.md section 9). It is
// safe for concurrent use.
type SessionService struct {
	cfg SessionConfig
	d   SessionDeps
}

// Compile-time check: the incoming port.
var _ agents.Sessions = (*SessionService)(nil)

// NewSessions returns the session use cases. Errors: a missing collaborator or
// agent.session_ref_max_bytes below 1.
//
// Example:
//
//	svc, err := app.NewSessions(app.SessionConfig{SessionRefMaxBytes: max}, app.SessionDeps{Store: store, ...})
func NewSessions(cfg SessionConfig, d SessionDeps) (*SessionService, error) {
	if d.Store == nil || d.Threads == nil || d.Channels == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Events == nil {
		return nil, errors.New("agents: NewSessions needs every collaborator")
	}
	if cfg.SessionRefMaxBytes < 1 {
		return nil, errors.New("agents: agent.session_ref_max_bytes must be at least 1")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &SessionService{cfg: cfg, d: d}, nil
}

// Values of the events, the audit entries and the descriptors of the sessions (05 sections 4.6
// and 6; M2-12 design note 15).
const (
	eventSessionChanged = "agent_session.changed"
	resourceThread      = "thread"
	receiveThreadRead   = "thread.read"
	stateRunning        = "running"
	stateWaitingInput   = "waiting_input"
	threadKindThread    = "thread"
	originHuman         = "human"

	attrSessionID         = "session_id"
	attrEndedSessionID    = "ended_session_id"
	attrTakenOverSession  = "taken_over_session_id"
	attrTakenOverThread   = "taken_over_thread_id"
	attrTakeOver          = "take_over"
	attrCause             = "cause"
	causeBind             = "bind"
	causeImport           = "import"
	logAttachSkipped      = "agents: the session reference of a run start was not attached; the thread or the reference has another current session"
	logKeyRunID           = "bothy.run_id"
	logKeyThreadID        = "bothy.thread_id"
	maxLockAttempts       = 3
	errMsgLockedRowsDrift = "agents: the session rows kept changing while they were locked"
)

// invokeDenials are the reasons of the agent.invoke guard that a session switch answers with 403
// agent_invocation_denied, as M2-09 maps the agent.invoke denials of run.token.renew (AG-29;
// M2-12 design note 5). Every other denial keeps the code of its outcome.
var invokeDenials = map[authorize.Reason]bool{
	"agent_not_reachable": true, "billing_mode_personal_not_holder": true, "tool_class_owner_only": true,
	"project_not_ready": true, "dev_environment_not_ready": true, "run_active_other_agent": true,
}

// now is the clock time in the precision of the columns.
func (s *SessionService) now() time.Time { return s.d.Clock.Now().UTC().Truncate(time.Microsecond) }

// thread loads the routing data of threadID; unknown gives not_found.
func (s *SessionService) thread(ctx context.Context, threadID ids.ID) (chat.ThreadInfo, error) {
	if threadID.IsZero() {
		return chat.ThreadInfo{}, invisible()
	}
	return s.d.Threads.Info(ctx, threadID)
}

// operator returns the operator of the thread: the fixed operator of a thread of kind thread
// (R-DM-12), the effective operator of the channel for a main thread (R-DM-21); zero for none.
func (s *SessionService) operator(ctx context.Context, info chat.ThreadInfo) (ids.ID, error) {
	if info.Kind == threadKindThread {
		return info.OperatorAgentID, nil
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
	if err != nil {
		return ids.ID{}, err
	}
	return ch.EffectiveOperatorID, nil
}

// threadResource describes the thread info for a session action; invoke adds the facts of the
// agent.invoke guard of a session switch (M2-12 design note 5).
func threadResource(info chat.ThreadInfo, invoke *authorize.InvokeFacts) authorize.Resource {
	return authorize.Resource{
		Kind: authorize.ResourceThread, ID: info.ID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: invoke,
	}
}

// switchFacts are the agent.invoke facts of a session switch: the operator addressed explicitly
// (the mention rule counts as met), a human trigger, no active run (point 8 does not apply).
func switchFacts(op ids.ID) *authorize.InvokeFacts {
	return &authorize.InvokeFacts{AgentID: op, TriggerOrigin: originHuman, MentionsAgent: true}
}

// authorize decides act on r; a guard reason of agent.invoke becomes 403 agent_invocation_denied
// with that reason, every other denial its own problem. authz audits every denial (R-AZ-04).
func (s *SessionService) authorize(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	if d.Allowed() {
		return nil
	}
	if d.Outcome == authorize.OutcomeForbidden && invokeDenials[d.Reason] {
		return errs.New(errs.CodeAgentInvocationDenied, "").WithReason(string(d.Reason))
	}
	return d.Err()
}

// List implements agents.Sessions (listAgentSessions, agent_session.read): every row of the
// thread newest first; no audit (05 4.6).
func (s *SessionService) List(ctx context.Context, a actor.Actor, threadID ids.ID) ([]agents.Session, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return nil, err
	}
	var out []agents.Session
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		info, err := s.thread(ctx, threadID)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, a, authorize.ActionAgentSessionRead, threadResource(info, nil)); err != nil {
			return err
		}
		out, err = s.d.Store.ThreadSessions(ctx, info.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []agents.Session{}
	}
	return out, nil
}

// Reset implements agents.Sessions (resetAgentSession, agent_session.reset; 09 9.2, M2-12 design
// note 7): the current row ends with reset, agent_session.changed follows, and the ended row is
// the answer. A run in progress keeps its row. No allow audit (05 4.6).
func (s *SessionService) Reset(ctx context.Context, a actor.Actor, threadID ids.ID) (agents.Session, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return agents.Session{}, err
	}
	var out agents.Session
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		info, err := s.thread(ctx, threadID)
		if err != nil {
			return err
		}
		op, err := s.operator(ctx, info)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, a, authorize.ActionAgentSessionReset, threadResource(info, switchFacts(op))); err != nil {
			return err
		}
		cur, found, err := s.d.Store.LockCurrentSession(ctx, info.ID)
		if err != nil {
			return err
		}
		if !found {
			return errs.New(errs.CodeConflict, "the thread has no current session").WithReason(agents.ReasonNoCurrentSession)
		}
		out, err = s.end(ctx, cur, domain.EndReset)
		if err != nil {
			return err
		}
		return s.emitChanged(ctx, info, out)
	})
	return out, err
}

// end ends the locked current row cur with reason.
func (s *SessionService) end(ctx context.Context, cur agents.Session, reason string) (agents.Session, error) {
	ended, err := domain.EndSession(toDomain(cur), reason, s.now())
	if err != nil {
		return agents.Session{}, errs.Wrap(errs.CodeInternalError, err, "agents: ending a session")
	}
	row, changed, err := s.d.Store.EndSession(ctx, SessionEnd{ID: cur.ID, Version: cur.Version, Reason: reason, At: ended.EndedAt})
	if err != nil {
		return agents.Session{}, err
	}
	if !changed {
		return agents.Session{}, errs.New(errs.CodeInternalError, "agents: a locked session row changed")
	}
	return row, nil
}

// Bind implements agents.Sessions (bindAgentSession, agent_session.bind; 09 9.4, M2-12 design
// note 8).
func (s *SessionService) Bind(ctx context.Context, a actor.Actor, threadID ids.ID, cmd agents.BindCmd) (agents.Session, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return agents.Session{}, err
	}
	var out agents.Session
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.bind(ctx, a, threadID, cmd)
		return err
	})
	return out, err
}

// bindTarget is the thread in which the reference of a bind is current, if any.
type bindTarget struct {
	row  agents.Session
	info chat.ThreadInfo
	set  bool
}

// bind is Bind inside its unit of work, in the order of design note 8.
func (s *SessionService) bind(ctx context.Context, a actor.Actor, threadID ids.ID, cmd agents.BindCmd) (agents.Session, error) {
	info, err := s.thread(ctx, threadID)
	if err != nil {
		return agents.Session{}, err
	}
	op, err := s.operator(ctx, info)
	if err != nil {
		return agents.Session{}, err
	}
	var target bindTarget
	if !op.IsZero() {
		if target, err = s.boundElsewhere(ctx, op, cmd.ExternalSessionRef, info.ID); err != nil {
			return agents.Session{}, err
		}
	}
	res := threadResource(info, switchFacts(op))
	if target.set {
		res.TargetThreadID = target.row.ThreadID
	}
	if err := s.authorize(ctx, a, authorize.ActionAgentSessionBind, res); err != nil {
		return agents.Session{}, err
	}
	if err := domain.CheckExternalSessionRef(cmd.ExternalSessionRef, s.cfg.SessionRefMaxBytes); err != nil {
		return agents.Session{}, err
	}
	if target.set && !cmd.TakeOver {
		return agents.Session{}, boundElsewhere()
	}
	cur, other, err := s.lockForBind(ctx, info.ID, target)
	if err != nil {
		return agents.Session{}, err
	}
	if cur.ID != cmd.ExpectedCurrentSessionID {
		return agents.Session{}, changedSession()
	}
	return s.rebind(ctx, info, op, cmd, cur, other)
}

// boundElsewhere finds the row in which (op, ref) is current in a thread other than thread.
func (s *SessionService) boundElsewhere(ctx context.Context, op ids.ID, ref string, thread ids.ID) (bindTarget, error) {
	row, found, err := s.d.Store.CurrentByRef(ctx, op, ref)
	if err != nil || !found || row.ThreadID == thread {
		return bindTarget{}, err
	}
	info, err := s.thread(ctx, row.ThreadID)
	if err != nil {
		return bindTarget{}, err
	}
	return bindTarget{row: row, info: info, set: true}, nil
}

// lockForBind locks the current row of thread and, when taking over, the row of the target, in
// ascending id (03 6.3), and returns them as they are under the locks: the current row of thread
// (zero ID for none) and the target row, nil when it is no longer current (the bind continues
// without a take-over). A current row that changes between the read and the lock is read again.
func (s *SessionService) lockForBind(ctx context.Context, thread ids.ID, target bindTarget) (agents.Session, *agents.Session, error) {
	for range maxLockAttempts {
		cur, found, err := s.d.Store.CurrentSession(ctx, thread)
		if err != nil {
			return agents.Session{}, nil, err
		}
		var want []ids.ID
		if found {
			want = append(want, cur.ID)
		}
		if target.set {
			want = append(want, target.row.ID)
		}
		locked, err := s.d.Store.LockSessions(ctx, want)
		if err != nil {
			return agents.Session{}, nil, err
		}
		again, foundAgain, err := s.d.Store.CurrentSession(ctx, thread)
		if err != nil {
			return agents.Session{}, nil, err
		}
		if foundAgain != found || (found && again.ID != cur.ID) {
			continue
		}
		var curRow agents.Session
		var other *agents.Session
		for i := range locked {
			switch {
			case found && locked[i].ID == cur.ID:
				curRow = locked[i]
			case target.set && locked[i].ID == target.row.ID && locked[i].IsCurrent:
				o := locked[i]
				other = &o
			}
		}
		return curRow, other, nil
	}
	return agents.Session{}, nil, errs.New(errs.CodeUnavailable, errMsgLockedRowsDrift)
}

// rebind ends the current row of the thread and, when taking over, the row of the other thread
// with rebind, inserts the new current row with started true and the cost baseline of design
// note 14, and writes the events and the allow audit entry.
func (s *SessionService) rebind(ctx context.Context, info chat.ThreadInfo, op ids.ID, cmd agents.BindCmd, cur agents.Session,
	other *agents.Session) (agents.Session, error) {
	var latest *domain.Session
	if prev, found, err := s.d.Store.LatestByRef(ctx, op, cmd.ExternalSessionRef); err != nil {
		return agents.Session{}, err
	} else if found {
		d := toDomain(prev)
		latest = &d
	}
	baseline, currency := domain.CostBaseline(latest)
	type endedRow struct {
		row  agents.Session
		info chat.ThreadInfo
	}
	var ended []endedRow
	if !cur.ID.IsZero() {
		row, err := s.end(ctx, cur, domain.EndRebind)
		if err != nil {
			return agents.Session{}, err
		}
		ended = append(ended, endedRow{row: row, info: info})
	}
	var otherInfo chat.ThreadInfo
	if other != nil {
		row, err := s.end(ctx, *other, domain.EndRebind)
		if err != nil {
			return agents.Session{}, err
		}
		if otherInfo, err = s.thread(ctx, other.ThreadID); err != nil {
			return agents.Session{}, err
		}
		ended = append(ended, endedRow{row: row, info: otherInfo})
	}
	row, inserted, err := s.d.Store.InsertSession(ctx, NewSession{
		ID: ids.New(), ThreadID: info.ID, AgentID: op, ExternalSessionRef: cmd.ExternalSessionRef, Started: true, IsCurrent: true,
		CostBaselineMicros: baseline, Currency: currency, CreatedAt: s.now(),
	})
	if err != nil {
		return agents.Session{}, err
	}
	if !inserted {
		return agents.Session{}, s.insertConflict(ctx, info.ID)
	}
	slices.SortFunc(ended, func(x, y endedRow) int { return compareIDs(x.row.ID, y.row.ID) })
	for _, e := range ended {
		if err := s.emitChanged(ctx, e.info, e.row); err != nil {
			return agents.Session{}, err
		}
	}
	if err := s.emitChanged(ctx, info, row); err != nil {
		return agents.Session{}, err
	}
	attrs := auditing.Attrs{attrSessionID: auditing.ID(row.ID), attrTakeOver: auditing.Bool(cmd.TakeOver), attrCause: auditing.Code(causeBind)}
	if !cur.ID.IsZero() {
		attrs[attrEndedSessionID] = auditing.ID(cur.ID)
	}
	if other != nil {
		attrs[attrTakenOverSession], attrs[attrTakenOverThread] = auditing.ID(other.ID), auditing.ID(other.ThreadID)
	}
	return row, s.recordBind(ctx, info.ID, attrs)
}

// insertConflict tells why an insert of a current row did nothing: the thread got another
// current row (session_changed) or the reference became current in another thread
// (session_bound_elsewhere).
func (s *SessionService) insertConflict(ctx context.Context, thread ids.ID) error {
	_, found, err := s.d.Store.CurrentSession(ctx, thread)
	if err != nil {
		return err
	}
	if found {
		return changedSession()
	}
	return boundElsewhere()
}

// recordBind writes the allow entry of agent_session.bind on the thread (SEC-AUD-5: IDs and
// codes only, never the reference).
func (s *SessionService) recordBind(ctx context.Context, thread ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAgentSessionBind), Decision: auditing.DecisionAllow, ResourceKind: resourceThread,
		ResourceID: thread, Attrs: attrs,
	})
}

// changedSession is the 409 conflict with reason session_changed.
func changedSession() error {
	return errs.New(errs.CodeConflict, "the current session of the thread changed").WithReason(agents.ReasonSessionChanged)
}

// boundElsewhere is the 409 conflict with reason session_bound_elsewhere.
func boundElsewhere() error {
	return errs.New(errs.CodeConflict, "the session is current in another thread").WithReason(agents.ReasonSessionBoundElsewhere)
}

// Report implements agents.Sessions (reportAgentSession, agent_session.report; M2-12 design notes
// 11 and 12).
func (s *SessionService) Report(ctx context.Context, a actor.Actor, threadID ids.ID, cmd agents.ReportCmd) (agents.Session, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return agents.Session{}, err
	}
	var out agents.Session
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.report(ctx, a, threadID, cmd)
		return err
	})
	return out, err
}

// report is Report inside its unit of work.
func (s *SessionService) report(ctx context.Context, a actor.Actor, threadID ids.ID, cmd agents.ReportCmd) (agents.Session, error) {
	info, err := s.thread(ctx, threadID)
	if err != nil {
		return agents.Session{}, err
	}
	run, err := s.reportingRun(ctx, a, info)
	if err != nil {
		return agents.Session{}, err
	}
	res := authorize.Resource{Kind: authorize.ResourceAgentSession, ID: run.SessionID, ThreadID: info.ID, OwnerID: run.AgentID}
	if err := s.authorize(ctx, a, authorize.ActionAgentSessionReport, res); err != nil {
		return agents.Session{}, err
	}
	if err := checkReport(cmd, s.cfg.SessionRefMaxBytes); err != nil {
		return agents.Session{}, err
	}
	r := domain.SessionReport{ExternalSessionRef: cmd.ExternalSessionRef, Model: cmd.Model, Started: cmd.Started, BindFailed: cmd.BindFailed}
	if !run.SessionID.IsZero() {
		locked, err := s.d.Store.LockSessions(ctx, []ids.ID{run.SessionID})
		if err != nil {
			return agents.Session{}, err
		}
		if len(locked) == 1 {
			return s.reportOn(ctx, info, run, locked[0], r, cmd.IfMatch)
		}
	}
	return s.firstReport(ctx, info, run, r, cmd.IfMatch)
}

// reportingRun reads the active run of the thread with its row lock and checks that it is a
// started run of the caller that operates the thread (a guest run has no session); anything else
// is the not_found of an unknown thread (design note 11 a).
func (s *SessionService) reportingRun(ctx context.Context, a actor.Actor, info chat.ThreadInfo) (agents.ActiveRun, error) {
	if s.d.Runs == nil {
		return agents.ActiveRun{}, invisible()
	}
	run, found, err := s.d.Runs.ActiveRun(ctx, info.ID)
	if err != nil {
		return agents.ActiveRun{}, err
	}
	if !found || run.AgentID != a.ID || (run.State != stateRunning && run.State != stateWaitingInput) {
		return agents.ActiveRun{}, invisible()
	}
	op, err := s.operator(ctx, info)
	if err != nil {
		return agents.ActiveRun{}, err
	}
	if op != run.AgentID {
		return agents.ActiveRun{}, invisible()
	}
	return run, nil
}

// checkReport validates the body of a report (design note 13).
func checkReport(cmd agents.ReportCmd, maxBytes int) error {
	if err := domain.CheckExternalSessionRef(cmd.ExternalSessionRef, maxBytes); err != nil {
		return err
	}
	if err := domain.CheckModel(cmd.Model); err != nil {
		return err
	}
	return domain.CheckBindFailed(cmd.BindFailed)
}

// reportOn applies a report to R, the locked row of runs.agent_session_id: If-Match must name
// its version (a missing header is a version conflict too, design note 11), then the outcome of
// domain.ApplyReport.
func (s *SessionService) reportOn(ctx context.Context, info chat.ThreadInfo, run agents.ActiveRun, row agents.Session,
	r domain.SessionReport, im agents.Precondition) (agents.Session, error) {
	if !im.Present || !im.Valid || im.Version != row.Version {
		return agents.Session{}, versionConflict(row)
	}
	out, err := domain.ApplyReport(toDomain(row), r)
	if err != nil {
		return agents.Session{}, err
	}
	switch out.Kind {
	case domain.ReportUnchanged:
		return row, nil
	case domain.ReportBindFailed:
		return s.bindFailed(ctx, info, run, row, r)
	case domain.ReportChanged:
	}
	if out.RefChanged && row.IsCurrent {
		if other, found, err := s.d.Store.CurrentByRef(ctx, row.AgentID, out.ExternalSessionRef); err != nil {
			return agents.Session{}, err
		} else if found && other.ID != row.ID {
			return agents.Session{}, boundElsewhere()
		}
	}
	updated, changed, err := s.d.Store.UpdateSession(ctx, SessionChange{
		ID: row.ID, Version: row.Version, ExternalSessionRef: out.ExternalSessionRef, Model: out.Model, Started: out.Started,
	})
	if errors.Is(err, ErrSessionRefTaken) {
		return agents.Session{}, boundElsewhere()
	}
	if err != nil {
		return agents.Session{}, err
	}
	if !changed {
		return agents.Session{}, errs.New(errs.CodeInternalError, "agents: a locked session row changed")
	}
	return updated, s.emitChanged(ctx, info, updated)
}

// versionConflict is 412 version_conflict with the current row as current.
func versionConflict(row agents.Session) error {
	raw, err := agents.SessionJSON(row)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "agents: the current session")
	}
	return errs.VersionConflict(json.RawMessage(raw))
}

// bindFailed is the fail open of 09 9.5 item 4 (design note 12): R ends with rebind when it is
// still current, otherwise the new row N is created already ended with R's end_reason, so a reset
// or bind during the run stays in force. N starts above R's version, so a concurrent duplicate
// with R's If-Match fails. runs attaches N and writes the one system line (SessionAttached).
func (s *SessionService) bindFailed(ctx context.Context, info chat.ThreadInfo, run agents.ActiveRun, r agents.Session,
	rep domain.SessionReport) (agents.Session, error) {
	now := s.now()
	n := NewSession{
		ID: ids.New(), ThreadID: info.ID, AgentID: run.AgentID, ExternalSessionRef: rep.ExternalSessionRef, Model: rep.Model,
		Started: rep.Started, IsCurrent: true, CreatedAt: now, Version: r.Version + 1,
	}
	var ended *agents.Session
	if r.IsCurrent {
		row, err := s.end(ctx, r, domain.EndRebind)
		if err != nil {
			return agents.Session{}, err
		}
		ended = &row
	} else {
		n.IsCurrent, n.EndedAt, n.EndReason = false, now, r.EndReason
	}
	row, inserted, err := s.d.Store.InsertSession(ctx, n)
	if err != nil {
		return agents.Session{}, err
	}
	if !inserted {
		return agents.Session{}, s.insertConflict(ctx, info.ID)
	}
	if ended != nil {
		if err := s.emitChanged(ctx, info, *ended); err != nil {
			return agents.Session{}, err
		}
	}
	if err := s.emitChanged(ctx, info, row); err != nil {
		return agents.Session{}, err
	}
	return row, txn.Publish(ctx, agents.SessionAttached{
		ThreadID: info.ID, RunID: run.RunID, AgentID: run.AgentID, SessionID: row.ID, ReplacedSessionID: r.ID, BindFailed: true,
	})
}

// firstReport creates the session row of a run that has none (09 9.1, 04 15.3 step 4): If-Match
// must be absent, bind_failed is refused, then a current row with the reported values.
func (s *SessionService) firstReport(ctx context.Context, info chat.ThreadInfo, run agents.ActiveRun, r domain.SessionReport,
	im agents.Precondition) (agents.Session, error) {
	if im.Present {
		return agents.Session{}, errs.VersionConflict(nil)
	}
	if r.BindFailed != "" {
		return agents.Session{}, domain.NotBound()
	}
	row, inserted, err := s.d.Store.InsertSession(ctx, NewSession{
		ID: ids.New(), ThreadID: info.ID, AgentID: run.AgentID, ExternalSessionRef: r.ExternalSessionRef, Model: r.Model,
		Started: r.Started, IsCurrent: true, CreatedAt: s.now(),
	})
	if err != nil {
		return agents.Session{}, err
	}
	if !inserted {
		return agents.Session{}, s.insertConflict(ctx, info.ID)
	}
	if err := s.emitChanged(ctx, info, row); err != nil {
		return agents.Session{}, err
	}
	return row, txn.Publish(ctx, agents.SessionAttached{ThreadID: info.ID, RunID: run.RunID, AgentID: run.AgentID, SessionID: row.ID})
}

// ForInvocation implements agents.Sessions (M2-12 design note 9 a to e) in the caller's unit of
// work. It never authorizes: runs decided agent.invoke.
func (s *SessionService) ForInvocation(ctx context.Context, q agents.InvocationSessionQuery) (agents.InvocationSession, error) {
	if !txn.InUnit(ctx) {
		return agents.InvocationSession{}, errs.New(errs.CodeInternalError, "agents: ForInvocation outside a unit of work")
	}
	if _, ok := actor.FromContext(ctx); !ok && !q.TriggeredBy.IsZero() {
		ctx = actor.WithActor(ctx, actor.Actor{ID: q.TriggeredBy, Kind: actor.KindUser})
	}
	info, err := s.thread(ctx, q.ThreadID)
	if err != nil {
		return agents.InvocationSession{}, err
	}
	cur, found, err := s.d.Store.LockCurrentSession(ctx, info.ID)
	if err != nil {
		return agents.InvocationSession{}, err
	}
	if found && cur.AgentID != q.AgentID {
		ended, err := s.end(ctx, cur, domain.EndAgentChanged)
		if err != nil {
			return agents.InvocationSession{}, err
		}
		if err := s.emitChanged(ctx, info, ended); err != nil {
			return agents.InvocationSession{}, err
		}
		found = false
	}
	if found {
		return agents.InvocationSession{Row: &cur}, nil
	}
	return s.importBinding(ctx, info, q)
}

// importBinding binds the newest import row of the operator exactly once (09 9.5, 22 section 9
// steps 2 and 3): only while the thread has no row other than import rows; the partial unique
// indexes decide between concurrent first triggers.
func (s *SessionService) importBinding(ctx context.Context, info chat.ThreadInfo, q agents.InvocationSessionQuery) (agents.InvocationSession, error) {
	rows, err := s.d.Store.ThreadSessions(ctx, info.ID)
	if err != nil {
		return agents.InvocationSession{}, err
	}
	history := make([]domain.Session, 0, len(rows))
	for _, r := range rows {
		history = append(history, toDomain(r))
	}
	cand, ok := domain.ImportBindingCandidate(history, q.AgentID)
	if !ok {
		return agents.InvocationSession{}, nil
	}
	row, inserted, err := s.d.Store.InsertSession(ctx, NewSession{
		ID: ids.New(), ThreadID: info.ID, AgentID: q.AgentID, ExternalSessionRef: cand.ExternalSessionRef, Model: cand.Model,
		Started: true, IsCurrent: true, CostBaselineMicros: cand.CostBaselineMicros, Currency: cand.Currency, CreatedAt: s.now(),
	})
	if err != nil {
		return agents.InvocationSession{}, err
	}
	if !inserted {
		// Either a concurrent first trigger bound it (take that row) or the reference is current
		// in another thread (fail open: no session, 22 section 9 steps 3 and 5).
		cur, found, err := s.d.Store.CurrentSession(ctx, info.ID)
		if err != nil || !found || cur.AgentID != q.AgentID {
			return agents.InvocationSession{}, err
		}
		return agents.InvocationSession{Row: &cur}, nil
	}
	if err := s.recordBind(ctx, info.ID, auditing.Attrs{
		attrSessionID: auditing.ID(row.ID), attrTakeOver: auditing.Bool(false), attrCause: auditing.Code(causeImport),
	}); err != nil {
		return agents.InvocationSession{}, err
	}
	if err := s.emitChanged(ctx, info, row); err != nil {
		return agents.InvocationSession{}, err
	}
	return agents.InvocationSession{Row: &row}, nil
}

// AttachOnStart implements agents.Sessions (M2-12 design note 10) in the caller's unit of work.
// It never changes an existing row, so the version the adapter got with the payload stays valid
// for its first If-Match.
func (s *SessionService) AttachOnStart(ctx context.Context, cmd agents.AttachOnStartCmd) (ids.ID, error) {
	if !txn.InUnit(ctx) {
		return ids.ID{}, errs.New(errs.CodeInternalError, "agents: AttachOnStart outside a unit of work")
	}
	info, err := s.thread(ctx, cmd.ThreadID)
	if err != nil {
		return ids.ID{}, err
	}
	op, err := s.operator(ctx, info)
	if err != nil {
		return ids.ID{}, err
	}
	if op.IsZero() || op != cmd.AgentID {
		// A guest run has no session; its reference is ignored (09 3.6, R-AG-04).
		return ids.ID{}, nil
	}
	if cmd.ExternalSessionRef != "" {
		if err := domain.CheckExternalSessionRef(cmd.ExternalSessionRef, s.cfg.SessionRefMaxBytes); err != nil {
			return ids.ID{}, err
		}
		if err := domain.CheckModel(cmd.Model); err != nil {
			return ids.ID{}, err
		}
	}
	cur, found, err := s.d.Store.LockCurrentSession(ctx, info.ID)
	if err != nil {
		return ids.ID{}, err
	}
	if found && cur.AgentID == cmd.AgentID {
		return cur.ID, nil
	}
	if cmd.ExternalSessionRef == "" {
		return ids.ID{}, nil
	}
	// A reset or bind ended the row after the run was queued: the run keeps the row it was
	// invoked with.
	if prev, found, err := s.d.Store.LatestInThreadByRef(ctx, info.ID, cmd.AgentID, cmd.ExternalSessionRef); err != nil {
		return ids.ID{}, err
	} else if found {
		return prev.ID, nil
	}
	row, inserted, err := s.d.Store.InsertSession(ctx, NewSession{
		ID: ids.New(), ThreadID: info.ID, AgentID: cmd.AgentID, ExternalSessionRef: cmd.ExternalSessionRef, Model: cmd.Model,
		IsCurrent: true, CreatedAt: s.now(),
	})
	if err != nil {
		return ids.ID{}, err
	}
	if !inserted {
		s.d.Log.LogAttrs(ctx, slog.LevelWarn, logAttachSkipped,
			slog.String(logKeyRunID, cmd.RunID.String()), slog.String(logKeyThreadID, info.ID.String()))
		return ids.ID{}, nil
	}
	return row.ID, s.emitChanged(ctx, info, row)
}

// emitChanged appends agent_session.changed for row (05 section 6, 04 9 point 3; design note 15):
// scope of the row's thread, resource the thread without version, receive action thread.read,
// the actor of the request, payload AgentSessionChangedPayload.
func (s *SessionService) emitChanged(ctx context.Context, info chat.ThreadInfo, row agents.Session) error {
	session, err := agents.SessionJSON(row)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "agents: agent_session.changed payload")
	}
	payload, err := json.Marshal(struct {
		ThreadID     string          `json:"thread_id"`
		AgentSession json.RawMessage `json:"agent_session"`
	}{ThreadID: row.ThreadID.String(), AgentSession: session})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "agents: agent_session.changed payload")
	}
	kind, id := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: eventSessionChanged, SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: id,
		SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: info.ID,
		ResourceKind: resourceThread, ResourceID: info.ID, ReceiveAction: receiveThreadRead, Payload: payload,
	})
}

// compareIDs orders two IDs ascending by their bytes (UUIDv7 order).
func compareIDs(x, y ids.ID) int {
	for i := range x {
		if x[i] != y[i] {
			if x[i] < y[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// toDomain converts a port row into the row of the rules.
func toDomain(s agents.Session) domain.Session {
	return domain.Session{
		ID: s.ID, ThreadID: s.ThreadID, AgentID: s.AgentID, ExternalSessionRef: s.ExternalSessionRef, IsCurrent: s.IsCurrent,
		Started: s.Started, Model: s.Model, CumulativeCostMicros: s.CumulativeCostMicros, CostBaselineMicros: s.CostBaselineMicros,
		Currency: s.Currency, CostIsEstimate: s.CostIsEstimate, CreatedAt: s.CreatedAt, EndedAt: s.EndedAt, EndReason: s.EndReason,
		Version: s.Version,
	}
}
