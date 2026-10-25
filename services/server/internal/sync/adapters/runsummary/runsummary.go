// SPDX-License-Identifier: Apache-2.0

// Package runsummary is the binding of the outgoing port app.RunSummarySource of the snapshot to
// the runs module (M2-05, 04 section 6.3 step 4): the running runs the caller may read come from
// runs.Runs.ActiveForActor, which applies the visibility pipeline of listActiveRuns (Scope of
// run.read, the threads through chat, run.read per run in filter mode) without paging. It replaced
// the empty M1 binding noruns.
//
// Example:
//
//	snap, err := app.NewSnapshot(app.SnapshotDeps{Runs: runsummary.New(runsService), ...})
package runsummary

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// Source implements app.RunSummarySource over the read port of runs.
type Source struct {
	runs runs.Runs
}

// Compile-time check: Source is the outgoing port.
var _ app.RunSummarySource = (*Source)(nil)

// New returns the source over r.
func New(r runs.Runs) *Source { return &Source{runs: r} }

// ActiveRuns implements app.RunSummarySource: the active runs a may read, as the schema
// RunSummary of the snapshot; never nil. channelIDs are the channels the snapshot lists; runs
// derives its own visible set from run.read, and a run outside channelIDs is left out, so the
// snapshot never names a run of a channel it does not list.
func (s *Source) ActiveRuns(ctx context.Context, a actor.Actor, channelIDs []ids.ID) ([]sync.RunSummary, error) {
	list, err := s.runs.ActiveForActor(actor.WithActor(ctx, a))
	if err != nil {
		return nil, err
	}
	listed := make(map[ids.ID]bool, len(channelIDs))
	for _, c := range channelIDs {
		listed[c] = true
	}
	out := make([]sync.RunSummary, 0, len(list))
	for _, r := range list {
		if !listed[r.ChannelID] {
			continue
		}
		out = append(out, sync.RunSummary{
			ID: r.ID, ThreadID: r.ThreadID, ChannelID: r.ChannelID, AgentID: r.AgentID, TriggeredBy: r.TriggeredBy,
			State: r.State, StateReason: r.StateReason, Version: r.Version, BillingModeExpected: r.BillingModeExpected,
			BillingModeUsed: r.BillingModeUsed, StatusLine: r.StatusLine, Summary: r.Summary,
			Cost:      sync.Money{AmountMicros: r.Cost.AmountMicros, Currency: r.Cost.Currency, IsEstimate: r.Cost.IsEstimate},
			StepCount: r.StepCount, DurationMS: r.DurationMS, QueuedAt: r.QueuedAt, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
			StallWarnedAt: r.StallWarnedAt,
		})
	}
	return out, nil
}
