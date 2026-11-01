// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// actionUserDisable is the action whose access change ends every active run of the person (08
// section 11 point 7).
const actionUserDisable = "user.disable"

// OnAccessChanged is the in-transaction subscriber of authorize.AccessChanged (R-SY-11, M2-09
// design note 8). It runs in phase 3.1 inside the revoking transaction, never calls authorize
// (the snapshot cache does not see the uncommitted rights) and works only on the diff: the active
// runs started by an affected principal are locked in ascending id, the runs whose channel the
// principal lost (every one for user.disable) get run.cancel_requested with the reason of the
// action and the job runs.cancel_timeout; their states stay. Until Bind it does nothing.
//
// Errors: those of the database and the scheduler, which roll the revocation back.
func (a *Acceptor) OnAccessChanged(ctx context.Context, ev authorize.AccessChanged) error {
	s := a.svc.Load()
	if s == nil {
		return nil
	}
	return s.onAccessChanged(ctx, ev)
}

// OnAgentDisabled is the in-transaction subscriber of agents.AgentDisabled (09 2.1, M2-09 design
// note 9): every active run of the agent ends at once with cancelled and agent_disabled and gets
// run.cancel_requested; no timeout job, because the agent's tokens are revoked in the same
// transaction. Until Bind it does nothing. Errors: those of the database.
func (a *Acceptor) OnAgentDisabled(ctx context.Context, ev agents.AgentDisabled) error {
	s := a.svc.Load()
	if s == nil {
		return nil
	}
	return s.onAgentDisabled(ctx, ev)
}

// onAccessChanged is OnAccessChanged with the bound use cases. The rights change of a deleted
// space or channel cancels nothing here: chat.ContentDeleted of the same transaction cancels
// every active run of the item once (OnContentDeleted, D-DEL-7).
func (s *Service) onAccessChanged(ctx context.Context, ev authorize.AccessChanged) error {
	if len(ev.Changes) == 0 || ev.Action == authorize.ActionSpaceDelete || ev.Action == authorize.ActionChannelDelete {
		return nil
	}
	principals := make([]ids.ID, 0, len(ev.Changes))
	changes := make([]domain.AccessChange, 0, len(ev.Changes))
	for _, c := range ev.Changes {
		principals = append(principals, c.PrincipalID)
		changes = append(changes, domain.AccessChange{PrincipalID: c.PrincipalID, Removed: targets(c.Removed), Changed: targets(c.Changed)})
	}
	rows, err := s.d.Repo.LockActiveRunsByStarter(ctx, principals)
	if err != nil || len(rows) == 0 {
		return err
	}
	kept, err := s.lostRuns(ctx, string(ev.Action), changes, rows)
	if err != nil {
		return err
	}
	reason := domain.CancelReasonFor(string(ev.Action))
	for _, row := range kept {
		if err := s.requestCancel(ctx, row, reason, ids.ID{}, false); err != nil {
			return err
		}
	}
	return nil
}

// lostRuns returns the rows whose starter lost trigger on the run's channel now: all of them for
// user.disable, otherwise the runs LostTriggerRuns keeps after the channel and space of each
// thread are resolved.
func (s *Service) lostRuns(ctx context.Context, action string, changes []domain.AccessChange, rows []RunRow) ([]RunRow, error) {
	if action == actionUserDisable {
		return rows, nil
	}
	active := make([]domain.LiveRun, 0, len(rows))
	byID := make(map[ids.ID]RunRow, len(rows))
	for _, r := range rows {
		info, err := s.d.Threads.Info(ctx, r.ThreadID)
		if err != nil {
			return nil, err
		}
		active = append(active, domain.LiveRun{
			ID: r.ID, AgentID: r.AgentID, TriggeredBy: r.TriggeredBy, ThreadID: r.ThreadID, ChannelID: info.ChannelID,
			SpaceID: info.SpaceID, State: r.State, QueuedAt: r.QueuedAt, StartedAt: r.StartedAt,
		})
		byID[r.ID] = r
	}
	lost := domain.LostTriggerRuns(changes, active)
	out := make([]RunRow, 0, len(lost))
	for _, id := range lost {
		out = append(out, byID[id])
	}
	return out, nil
}

// targets converts the entries of access.changed into the domain copy.
func targets(in []authorize.AccessTarget) []domain.AccessTarget {
	out := make([]domain.AccessTarget, 0, len(in))
	for _, t := range in {
		out = append(out, domain.AccessTarget{Kind: string(t.Kind), ID: t.ID, Level: t.Level})
	}
	return out
}

// onAgentDisabled is OnAgentDisabled with the bound use cases.
func (s *Service) onAgentDisabled(ctx context.Context, ev agents.AgentDisabled) error {
	rows, err := s.d.Repo.LockActiveRunsByAgent(ctx, ev.AgentID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.requestCancel(ctx, row, domain.RunReasonAgentDisabled, ids.ID{}, true); err != nil {
			return err
		}
	}
	return nil
}

// OnChannelReadOnlySet is the in-transaction subscriber of chat.ChannelReadOnlySet (M8-03, M2-09
// design note 7 g; 05 channel.read_only.set, 22 13.4): when the channel becomes read-only, every
// active run in its threads is locked in ascending id and gets run.cancel_requested with reason
// channel_read_only and the job runs.cancel_timeout through requestCancel; lifting read-only does
// nothing. Until Bind it does nothing. Errors: those of the database and the scheduler, which roll
// the change of the flag back.
func (a *Acceptor) OnChannelReadOnlySet(ctx context.Context, ev chat.ChannelReadOnlySet) error {
	s := a.svc.Load()
	if s == nil || !ev.ReadOnly {
		return nil
	}
	return s.onChannelReadOnly(ctx, ev.ChannelID)
}

// OnContentDeleted is the in-transaction subscriber of chat.ContentDeleted (05 space.delete,
// channel.delete, thread.delete; D-DEL-7): every active run in the threads of the deleted channels
// and in the deleted thread is locked in ascending id and gets run.cancel_requested with reason
// channel_read_only and the job runs.cancel_timeout through requestCancel, so the runner stops the
// process. Until Bind it does nothing. Errors: those of the database and the scheduler, which
// roll the deletion back.
func (a *Acceptor) OnContentDeleted(ctx context.Context, ev chat.ContentDeleted) error {
	s := a.svc.Load()
	if s == nil {
		return nil
	}
	return s.onContentDeleted(ctx, ev)
}

// onContentDeleted requests the cancellation of the active runs of the deleted content.
func (s *Service) onContentDeleted(ctx context.Context, ev chat.ContentDeleted) error {
	threads := slices.Clone(ev.ThreadIDs)
	if len(ev.ChannelIDs) > 0 {
		if s.d.ChannelThreads == nil {
			return errs.New(errs.CodeInternalError, "runs: the subscriber of chat.ContentDeleted needs the threads of chat")
		}
		more, err := s.d.ChannelThreads.ThreadIDsOfChannels(ctx, ev.ChannelIDs)
		if err != nil {
			return err
		}
		threads = append(threads, more...)
	}
	if len(threads) == 0 {
		return nil
	}
	rows, err := s.d.Repo.LockActiveRunsByThreads(ctx, threads)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.requestCancel(ctx, row, domain.RunReasonChannelReadOnly, ids.ID{}, false); err != nil {
			return err
		}
	}
	return nil
}

// onChannelReadOnly requests the cancellation of the active runs of the channel.
func (s *Service) onChannelReadOnly(ctx context.Context, channelID ids.ID) error {
	if s.d.ChannelThreads == nil {
		return errs.New(errs.CodeInternalError, "runs: the subscriber of chat.ChannelReadOnlySet needs the threads of chat")
	}
	threads, err := s.d.ChannelThreads.ThreadIDsOfChannels(ctx, []ids.ID{channelID})
	if err != nil || len(threads) == 0 {
		return err
	}
	rows, err := s.d.Repo.LockActiveRunsByThreads(ctx, threads)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.requestCancel(ctx, row, domain.RunReasonChannelReadOnly, ids.ID{}, false); err != nil {
			return err
		}
	}
	return nil
}
