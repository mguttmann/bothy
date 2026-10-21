// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The agent sessions of a thread on the side of runs (09 section 9, M2-12): agents owns the rows
// and every session rule; runs owns runs.agent_session_id and therefore the question whether a run
// used a row. Each call joins the unit of work of its caller after the run row (03 6.3).

// operatorOf returns the operator of the thread of info (03 R-DM-12, R-DM-21): its own operator
// for a thread of kind thread, the channel's effective operator for a main thread; zero for none.
func operatorOf(info chat.ThreadInfo, ch chat.TriggerChannel) ids.ID {
	if info.Kind == string(domain.ThreadThread) {
		return info.OperatorAgentID
	}
	return ch.EffectiveOperatorID
}

// invocationSession decides the session of the new run sum (M2-12 design note 9): a guest run has
// none (09 3.6); for the operator ForInvocation applies the rules of 09 9.1 and 9.5 in this unit of
// work, and the mode follows from the row and whether a run of the thread already used it. Without
// Sessions every operator run is new without a row.
func (s *Service) invocationSession(ctx context.Context, sum runs.RunSummary, operator ids.ID) (domain.SessionDecision, error) {
	if operator.IsZero() || sum.AgentID != operator {
		return domain.DecideSession(false, nil), nil
	}
	if s.d.Sessions == nil {
		return domain.DecideSession(true, nil), nil
	}
	got, err := s.d.Sessions.ForInvocation(ctx, agents.InvocationSessionQuery{ThreadID: sum.ThreadID, AgentID: sum.AgentID, TriggeredBy: sum.TriggeredBy})
	if err != nil || got.Row == nil {
		return domain.DecideSession(true, nil), err
	}
	used, err := s.d.Repo.SessionUsed(ctx, sum.ThreadID, got.Row.ID)
	if err != nil {
		return domain.SessionDecision{}, err
	}
	raw, err := agents.SessionJSON(*got.Row)
	if err != nil {
		return domain.SessionDecision{}, errs.Wrap(errs.CodeInternalError, err, "runs: the agent_session of agent.invocation")
	}
	// The run records the row it is invoked with, so its start keeps it even when a reset or a
	// bind ends it while the run is queued (09 9.4, design note 8 h); "used" counts only started
	// runs (SessionUsed), so recording it here changes no mode.
	if err := s.d.Repo.SetRunSession(ctx, sum.ID, got.Row.ID); err != nil {
		return domain.SessionDecision{}, err
	}
	return domain.DecideSession(true, &domain.SessionRow{JSON: raw, Started: got.Row.Started, Used: used}), nil
}

// attachOnStart attaches the session row of a run that moves from queued to running (M2-12 design
// note 10) in the unit of work of the start, after the token checks and the compare-and-swap:
// AttachOnStart decides (a guest run and a start without reference and without a current row get
// none) and runs.agent_session_id stores the row before the document is read. A validation error of
// the reference is returned unchanged (400 with pointer /external_session_ref). Without Sessions a
// reference is ignored.
func (s *Service) attachOnStart(ctx context.Context, row RunRow, cmd runs.StartRun) error {
	if s.d.Sessions == nil {
		return nil
	}
	ref := ""
	if cmd.ExternalSessionRef != nil {
		ref = *cmd.ExternalSessionRef
	}
	id, err := s.d.Sessions.AttachOnStart(ctx, agents.AttachOnStartCmd{
		ThreadID: row.ThreadID, RunID: row.ID, AgentID: row.AgentID, SessionID: row.AgentSessionID, ExternalSessionRef: ref, Model: cmd.Model,
	})
	if err != nil || id.IsZero() || id == row.AgentSessionID {
		return err
	}
	return s.d.Repo.SetRunSession(ctx, row.ID, id)
}

// OnSessionAttached is the in-transaction subscriber of agents.SessionAttached (02 section 4.3;
// M2-12 design notes 11 and 12): a report created the session row of a run or replaced it after
// bind_failed. It stores runs.agent_session_id = ev.SessionID for ev.RunID and, when
// ev.BindFailed, writes exactly one system line agent_session.resume_failed with the run's agent
// as author, which never reopens a closed thread (R-DM-42) and never triggers (SR4); the run
// continues. Until Bind it does nothing. Errors: those of the database and of messaging, which roll
// the report back.
func (a *Acceptor) OnSessionAttached(ctx context.Context, ev agents.SessionAttached) error {
	s := a.svc.Load()
	if s == nil {
		return nil
	}
	return s.onSessionAttached(ctx, ev)
}

// onSessionAttached is OnSessionAttached with the bound use cases.
func (s *Service) onSessionAttached(ctx context.Context, ev agents.SessionAttached) error {
	if err := needUnit(ctx, "OnSessionAttached"); err != nil {
		return err
	}
	if err := s.d.Repo.SetRunSession(ctx, ev.RunID, ev.SessionID); err != nil {
		return err
	}
	if !ev.BindFailed {
		return nil
	}
	_, err := s.d.Messages.AppendSystemLine(ctx, messaging.SystemLine{
		ThreadID: ev.ThreadID, AuthorID: ev.AgentID, Code: messaging.SystemAgentSessionResumeFailed,
	})
	return err
}
