// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// StallCheck is the use case of the periodic job runs.stall_check (M2-09 design note 10): it reads
// the started active runs and the persisted heartbeat of their agents through the agents port,
// and every run OrphanVerdict finds interrupted (missing from active_runs of a heartbeat more than
// agent.presence_timeout after its start) or stale (no heartbeat for longer than
// run.orphan_timeout) fails with that reason, one unit of work per run with the row locked and
// the verdict decided again, without an actor. Nothing is retried: no new run, no new invocation
// (R-AG-17). Then it marks every run still in running whose last_progress_at is older than
// run.stall_after and that has no stall notice yet (09 5.5 row "No progress", M2-51 design note
// 5): one unit of work per run sets stall_warned_at with one version step, and run.updated with
// add /stall_warned_at goes out after the commit. A run in waiting_input waits for a person and
// is never marked. Errors: those of the database and the agents port; the job is then retried.
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
		if err := s.endOrphan(ctx, r.ID, w); err != nil {
			return err
		}
	}
	for _, r := range rows {
		if !s.stalled(r, s.now()) {
			continue
		}
		if err := s.markStalled(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// stalled reports a run of 09 5.5 row "No progress" at now: running, a last_progress_at older
// than run.stall_after and no stall notice. A run without last_progress_at (started before the
// column existed, ruling D-M2-51-2) is never stalled.
func (s *Service) stalled(r RunRow, now time.Time) bool {
	return r.State == domain.StateRunning && r.StallWarnedAt.IsZero() && !r.LastProgressAt.IsZero() &&
		r.LastProgressAt.Before(now.Add(-s.cfg.StallAfter))
}

// markStalled sets the stall notice of the run read as r in its own unit of work: the row is
// locked and must still be at r.Version and stalled (compare-and-swap), so a patch, a start or an
// end stored in between wins and the run is skipped. After the commit run.updated carries
// add /stall_warned_at from r.Version to the new version (04 11.3 item 7).
func (s *Service) markStalled(ctx context.Context, r RunRow) error {
	var event *eventlog.EphemeralEvent
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		event = nil
		row, found, err := s.d.Repo.LockRun(ctx, r.ID)
		if err != nil || !found {
			return err
		}
		now := s.now()
		if row.Version != r.Version || !s.stalled(row, now) {
			return nil
		}
		stored, marked, err := s.d.Repo.MarkRunStalled(ctx, StallMark{ID: row.ID, Version: row.Version, At: now,
			StalledBefore: now.Add(-s.cfg.StallAfter)})
		if err != nil || !marked {
			return err
		}
		info, err := s.d.Threads.Info(ctx, row.ThreadID)
		if err != nil {
			return err
		}
		value, err := encode(stamp(stored.StallWarnedAt))
		if err != nil {
			return err
		}
		payload := s.updatedPayload(row, row.Version, stored.Version,
			[]domain.NormOp{{Op: domain.OpAdd, Path: pathStallWarnedAt, Value: value}})
		event = &eventlog.EphemeralEvent{
			Type: eventRunUpdated, SchemaVersion: runUpdatedSchemaVersion,
			Scope:      eventlog.Scope{SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: row.ThreadID},
			Resource:   eventlog.ResourceRef{Kind: resourceRun, ID: row.ID, Version: stored.Version},
			OccurredAt: now, Payload: payload,
		}
		return nil
	})
	if err != nil || event == nil {
		return err
	}
	s.publishUpdated(ctx, *event)
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

// endOrphan fails one run found orphaned in its own unit of work: the row is locked, the
// persisted heartbeat of its agent read again and the verdict decided again on both, so a start,
// an end or a heartbeat of the adapter stored in between wins.
func (s *Service) endOrphan(ctx context.Context, id ids.ID, w domain.OrphanWindows) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, found, err := s.d.Repo.LockRun(ctx, id)
		if err != nil || !found {
			return err
		}
		hbs, err := s.heartbeatsOf(ctx, []RunRow{row})
		if err != nil {
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
