// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// ApplyRunWaits stores the wait reasons the heartbeat of agentID reported in waiting_runs (09 4.8
// and 5.2.1, F-WARTESCHLANGE-GRUND) in one unit of work: the queued runs of the agent are locked
// in ascending id, domain.WaitChanges decides, and each changed run gets its reason with one
// version step and run.state_changed without a trigger state. A listed run of another agent, or
// one that is no longer queued, changes nothing. The heartbeat is the agent's own report, so no
// authorize runs here; the presence tracker calls it after agent.heartbeat was allowed.
//
// Errors: those of the database, the chat port and the appender.
func (s *Service) ApplyRunWaits(ctx context.Context, agentID ids.ID, waits []agents.RunWait) error {
	reported := make(map[ids.ID]string, len(waits))
	for _, w := range waits {
		reported[w.RunID] = w.Reason
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		rows, err := s.d.Repo.LockQueuedRunsOfAgent(ctx, agentID)
		if err != nil {
			return err
		}
		queued := make([]domain.QueuedReason, 0, len(rows))
		for _, r := range rows {
			queued = append(queued, domain.QueuedReason{RunID: r.ID, Reason: r.StateReason})
		}
		for _, ch := range domain.WaitChanges(queued, reported) {
			if err := s.setReason(ctx, ch); err != nil {
				return err
			}
		}
		return nil
	})
}

// setReason stores one wait reason and appends its run.state_changed; a run that is no longer
// queued is skipped.
func (s *Service) setReason(ctx context.Context, ch domain.QueuedReason) error {
	row, found, err := s.d.Repo.SetQueuedReason(ctx, ch.RunID, ch.Reason)
	if err != nil || !found {
		return err
	}
	return s.emitReasonChanged(ctx, row)
}

// emitReasonChanged appends run.state_changed for a queued run whose wait reason changed (09
// 5.2.1): the summary with the stored reason, no trigger state.
func (s *Service) emitReasonChanged(ctx context.Context, row RunRow) error {
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return err
	}
	counts, err := s.d.Repo.StepCounts(ctx, []ids.ID{row.ID})
	if err != nil {
		return err
	}
	return s.emitStateChanged(ctx, info.SpaceID, summary(row, info.ChannelID, counts[row.ID]), row.TriggeringMessageID, domain.TriggerStateNone)
}
