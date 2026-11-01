// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// Timeline implements stats.Views (getStatsTimeline, docs/spec/25-visualization.md 6.5, M6-14
// design note 1, D-M6-14-1), in this order:
//
//  1. stats.read on the instance or the space (403, 404 for an unreadable space);
//  2. the range: full minutes, from before to, at most stats.timeline_max_range, not before now
//     minus retention.stats_minutes (400 validation_failed);
//  3. the channel set of M6-05 and cost.read.aggregate on the same target in filter mode;
//  4. the rows of the requested metrics (all without metric) in the channel set, the rows without
//     a channel only for an owner on the instance, cost rows only with cost.read.aggregate;
//  5. the runs of the threads of the channel set whose interval intersects the range, ordered by
//     queued_at, at most stats.timeline_max_runs with runs_truncated.
//
// The result is not cached: the range is minute-precise, so equal requests are rare.
func (v *Views) Timeline(ctx context.Context, q stats.TimelineQuery) (stats.Timeline, error) {
	st := v.d.Statistics
	a, err := st.authorize(ctx, authorize.ActionStatsRead, q.SpaceID)
	if err != nil {
		return stats.Timeline{}, err
	}
	if err := domain.ValidateTimeline(q.From, q.To, v.d.Clock.Now(), domain.TimelineLimits{MaxRange: v.cfg.TimelineMaxRange,
		Retention: v.cfg.Retention}); err != nil {
		return stats.Timeline{}, err
	}
	metrics, err := timelineMetrics(q.Metrics)
	if err != nil {
		return stats.Timeline{}, err
	}
	t, err := st.visible(ctx, a, q.SpaceID)
	if err != nil {
		return stats.Timeline{}, err
	}
	costs, err := v.costsAllowed(ctx, a, q.SpaceID)
	if err != nil {
		return stats.Timeline{}, err
	}
	if !costs {
		metrics = slices.DeleteFunc(metrics, func(m string) bool { return m == string(measure.Cost) })
	}
	out := stats.Timeline{From: q.From, To: q.To, Buckets: []stats.TimelineBucket{}, Runs: []stats.TimelineRun{}}
	if len(metrics) > 0 {
		rows, err := v.d.Store.TimelineRows(ctx, TimelineRowsQuery{From: q.From, To: q.To, Metrics: metrics, ChannelIDs: t.channels,
			WithInstance: t.instance})
		if err != nil {
			return stats.Timeline{}, err
		}
		out.Buckets = keepRows(rows, t, metrics)
	}
	if out.Runs, out.RunsTruncated, err = v.timelineRuns(ctx, a, t, q); err != nil {
		return stats.Timeline{}, err
	}
	return out, nil
}

// timelineRuns returns the runs of the threads of the channel set of t that intersect the range of
// q, at most stats.timeline_max_runs, and whether more intersect it; a run whose thread a may read
// carries its chat (25 4 rule 6).
func (v *Views) timelineRuns(ctx context.Context, a actor.Actor, t target, q stats.TimelineQuery) ([]stats.TimelineRun, bool, error) {
	threads, channelOf, err := v.threadsOf(ctx, t.channels, q.To)
	if err != nil {
		return nil, false, err
	}
	// One run more than the limit tells whether more runs intersect the range (runs_truncated).
	rs, err := v.d.Runs.TimelineRuns(ctx, runs.TimelineQuery{From: q.From, To: q.To, ThreadIDs: threads, Limit: v.cfg.TimelineMaxRuns + 1})
	if err != nil {
		return nil, false, err
	}
	out := make([]stats.TimelineRun, 0, min(len(rs), v.cfg.TimelineMaxRuns))
	threadOf := make([]ids.ID, 0, cap(out))
	truncated := false
	for _, r := range rs {
		ch, ok := channelOf[r.ThreadID]
		if !ok {
			continue
		}
		if len(out) == v.cfg.TimelineMaxRuns {
			truncated = true
			break
		}
		out = append(out, stats.TimelineRun{RunID: r.ID, ChannelID: ch, AgentID: r.AgentID, QueuedAt: r.QueuedAt,
			StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, State: r.State})
		threadOf = append(threadOf, r.ThreadID)
	}
	if err := v.nameTimelineRuns(ctx, a, out, threadOf); err != nil {
		return nil, false, err
	}
	return out, truncated, nil
}

// nameTimelineRuns gives every run of out (whose thread is threadOf at the same index) the chat of
// its thread when its channel is in the Scope of thread.read of a, in one read for all of them; the
// others stay without a name (25 4 rule 6).
func (v *Views) nameTimelineRuns(ctx context.Context, a actor.Actor, out []stats.TimelineRun, threadOf []ids.ID) error {
	if len(out) == 0 {
		return nil
	}
	named, err := namedChannels(ctx, v.d.Statistics.d.Authorizer, a)
	if err != nil {
		return err
	}
	var channels, threads []ids.ID
	for i, r := range out {
		if _, ok := named[r.ChannelID]; !ok {
			continue
		}
		if !slices.Contains(channels, r.ChannelID) {
			channels = append(channels, r.ChannelID)
		}
		threads = append(threads, threadOf[i])
	}
	if len(threads) == 0 {
		return nil
	}
	chats, err := v.d.Statistics.d.Chat.StatsRunChats(ctx, channels, threads)
	if err != nil {
		return err
	}
	byThread := make(map[ids.ID]chat.StatsRunChat, len(chats))
	for _, c := range chats {
		byThread[c.ThreadID] = c
	}
	for i := range out {
		c, ok := byThread[threadOf[i]]
		if _, readable := named[out[i].ChannelID]; !ok || !readable || c.ChannelID != out[i].ChannelID {
			continue
		}
		out[i].Chat = &stats.RunChat{ThreadID: c.ThreadID, ChannelName: c.ChannelName, ThreadTitle: c.ThreadTitle}
	}
	return nil
}

// timelineMetrics returns the requested metrics without duplicates, or every metric of 25 3.2
// when none is requested. Errors: validation_failed for a name outside the closed list (the
// request validator of the chain refuses it first; this keeps the use case closed on its own).
func timelineMetrics(requested []string) ([]string, error) {
	all := measure.Metrics()
	if len(requested) == 0 {
		out := make([]string, len(all))
		for i, m := range all {
			out[i] = string(m)
		}
		return out, nil
	}
	var out []string
	for _, m := range requested {
		if !slices.Contains(all, measure.Metric(m)) {
			return nil, errs.New(errs.CodeValidationFailed, "Unknown metric.").
				WithFields(errs.FieldError{Pointer: "/metric", Code: "invalid_enum", Detail: "Unknown metric."})
		}
		if !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out, nil
}

// keepRows keeps the rows of the channel set, the rows without a channel only for an owner on the
// instance and only metrics of the request (defence in depth: the query already filtered).
func keepRows(rows []stats.TimelineBucket, t target, metrics []string) []stats.TimelineBucket {
	in := make(map[ids.ID]bool, len(t.channels))
	for _, c := range t.channels {
		in[c] = true
	}
	out := make([]stats.TimelineBucket, 0, len(rows))
	for _, r := range rows {
		if !slices.Contains(metrics, r.Metric) || (r.ChannelID.IsZero() && !t.instance) || (!r.ChannelID.IsZero() && !in[r.ChannelID]) {
			continue
		}
		out = append(out, r)
	}
	return out
}
