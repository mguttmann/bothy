// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// Compile-time check: the service is the source of the active runs of the live frame (M6-15).
var _ runs.LiveSource = (*Service)(nil)

// LiveRuns implements runs.LiveSource (docs/spec/25-visualization.md 5.2, M6-15): every active
// run in batches of api.page_size_max in keyset order (newest first), the place of its thread
// through chat.Threads.Locate, at most perChannel per channel, then the step kind and name and the
// approval flag of the kept runs in one read and the steps of their agent trees in another. It
// does not authorize; stats filters per receiver.
// Errors: those of the reads.
func (s *Service) LiveRuns(ctx context.Context, perChannel int) ([]runs.LiveRun, error) {
	var rows []RunRow
	var after *RunKey
	for {
		batch, err := s.d.Repo.ActiveRuns(ctx, ActiveRunsRead{After: after, Limit: s.cfg.PageSizeMax})
		if err != nil {
			return nil, err
		}
		rows = append(rows, batch...)
		if len(batch) < s.cfg.PageSizeMax {
			break
		}
		last := batch[len(batch)-1]
		after = &RunKey{QueuedAt: last.QueuedAt, ID: last.ID}
	}
	if len(rows) == 0 {
		return []runs.LiveRun{}, nil
	}
	infos, err := s.locate(ctx)(rows)
	if err != nil {
		return nil, err
	}
	perCh := map[ids.ID]int{}
	out := make([]runs.LiveRun, 0, len(rows))
	kept := make([]ids.ID, 0, len(rows))
	for _, r := range rows {
		info, ok := infos[r.ThreadID]
		if !ok || perCh[info.ChannelID] >= perChannel {
			continue
		}
		perCh[info.ChannelID]++
		mode := r.BillingModeUsed
		if mode == "" {
			mode = r.BillingModeExpected
		}
		out = append(out, runs.LiveRun{
			ID: r.ID, ThreadID: r.ThreadID, AgentID: r.AgentID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, State: string(r.State),
			QueuedAt: r.QueuedAt, StartedAt: r.StartedAt, BillingMode: mode,
			Cost: runs.Money{AmountMicros: r.CostMicros, Currency: r.Currency, IsEstimate: r.CostIsEstimate},
		})
		kept = append(kept, r.ID)
	}
	if err := s.liveDetails(ctx, out, kept); err != nil {
		return nil, err
	}
	return out, nil
}

// liveDetails sets the current step, the approval flag and the steps of the agent tree of each
// run of out; kept are their IDs.
func (s *Service) liveDetails(ctx context.Context, out []runs.LiveRun, kept []ids.ID) error {
	facts, err := s.d.Repo.LiveFacts(ctx, kept)
	if err != nil {
		return err
	}
	steps, err := s.d.Repo.LiveSteps(ctx, kept)
	if err != nil {
		return err
	}
	for i := range out {
		f := facts[out[i].ID]
		out[i].StepKind, out[i].StepName, out[i].ApprovalPending = f.StepKind, f.StepName, f.ApprovalPending
		out[i].Steps = steps[out[i].ID]
	}
	return nil
}
