// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// StallCheck is the use case of the periodic job runs.stall_check (M2-09 design note 10): it reads
// the started active runs and the persisted heartbeat of their agents through the agents port,
// and every run OrphanVerdict finds interrupted (missing from active_runs of a heartbeat more than
// agent.presence_timeout after its start) or stale (no heartbeat for longer than
// run.orphan_timeout) fails with that reason, one unit of work per run with the row locked and
// the verdict decided again, without an actor. Nothing is retried: no new run, no new invocation
// (R-AG-17). Errors: those of the database and the agents port; the job is then retried.
func (s *Service) StallCheck(ctx context.Context) error {
	if s.d.Heartbeats == nil {
		return errs.New(errs.CodeInternalError, "runs: the stall check has no heartbeat reader")
	}
	ctx = actor.WithActor(ctx, actor.Anonymous())
	rows, err := s.d.Repo.OrphanCandidates(ctx)
	if err != nil || len(rows) == 0 {
		return err
	}
	hbs, err := s.heartbeatsOf(ctx, rows)
	if err != nil {
		return err
	}
	w := domain.OrphanWindows{PresenceTimeout: s.cfg.PresenceTimeout, OrphanTimeout: s.cfg.OrphanTimeout}
	for _, r := range rows {
		if domain.OrphanVerdict(activeRun(r), hbs[r.AgentID], s.d.Clock.Now(), w) == domain.OrphanNone {
			continue
		}
		if err := s.endOrphan(ctx, r.ID, hbs, w); err != nil {
			return err
		}
	}
	return nil
}

// heartbeatsOf reads the persisted heartbeat of every agent of rows.
func (s *Service) heartbeatsOf(ctx context.Context, rows []RunRow) (map[ids.ID]domain.HeartbeatState, error) {
	seen := map[ids.ID]bool{}
	var agentIDs []ids.ID
	for _, r := range rows {
		if !seen[r.AgentID] {
			seen[r.AgentID] = true
			agentIDs = append(agentIDs, r.AgentID)
		}
	}
	states, err := s.d.Heartbeats.HeartbeatStates(ctx, agentIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[ids.ID]domain.HeartbeatState, len(states))
	for _, h := range states {
		out[h.AgentID] = domain.HeartbeatState{At: h.LastHeartbeatAt, ActiveRuns: h.ActiveRunIDs, ActiveRunsReported: h.ActiveRunsReported}
	}
	return out, nil
}

// endOrphan fails one run found orphaned in its own unit of work: the row is locked and the
// verdict decided again on it, so a start, an end or a heartbeat of the adapter in between wins.
func (s *Service) endOrphan(ctx context.Context, id ids.ID, hbs map[ids.ID]domain.HeartbeatState, w domain.OrphanWindows) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, found, err := s.d.Repo.LockRun(ctx, id)
		if err != nil || !found {
			return err
		}
		var reason string
		switch domain.OrphanVerdict(activeRun(row), hbs[row.AgentID], s.d.Clock.Now(), w) {
		case domain.OrphanInterrupted:
			reason = domain.RunReasonInterrupted
		case domain.OrphanStale:
			reason = domain.RunReasonStale
		case domain.OrphanNone:
			return nil
		}
		_, err = s.endRun(ctx, row, domain.StateFailed, reason, domain.SetterServer, TransitionStore{})
		return err
	})
}

// activeRun is the domain view of a run row for the verdicts.
func activeRun(r RunRow) domain.LiveRun {
	return domain.LiveRun{ID: r.ID, AgentID: r.AgentID, TriggeredBy: r.TriggeredBy, ThreadID: r.ThreadID, State: r.State,
		QueuedAt: r.QueuedAt, StartedAt: r.StartedAt}
}
