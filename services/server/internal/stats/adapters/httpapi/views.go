// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats"
)

// Views implements the REST operations getStatsTimeline, getStatsSlo and getStatsBackends (M6-14)
// as part of rest.StrictServerInterface. The use cases authorize, validate and filter; the handlers
// only convert and pass errors to the problem writer of the chain unchanged. Every list is an
// array, never null, and an absent value is an absent member.
//
// Example:
//
//	h := httpapi.NewViews(v)
//	built, err := routes.Build(wire.Handlers{Views: h}, wire.RouteOptions(), cat)
type Views struct {
	v stats.Views
}

// NewViews returns the handlers over the port v.
func NewViews(v stats.Views) *Views { return &Views{v: v} }

// GetStatsTimeline implements rest.StrictServerInterface (GET /stats/timeline).
func (h *Views) GetStatsTimeline(ctx context.Context, req rest.GetStatsTimelineRequestObject) (rest.GetStatsTimelineResponseObject, error) {
	p := req.Params
	q := stats.TimelineQuery{From: p.From, To: p.To}
	if p.SpaceId != nil {
		q.SpaceID = ids.ID(*p.SpaceId)
	}
	if p.Metric != nil {
		for _, m := range *p.Metric {
			q.Metrics = append(q.Metrics, string(m))
		}
	}
	tl, err := h.v.Timeline(ctx, q)
	if err != nil {
		return nil, err
	}
	return rest.GetStatsTimeline200JSONResponse{Body: TimelineREST(tl)}, nil
}

// GetStatsSlo implements rest.StrictServerInterface (GET /stats/slo).
func (h *Views) GetStatsSlo(ctx context.Context, _ rest.GetStatsSloRequestObject) (rest.GetStatsSloResponseObject, error) {
	r, err := h.v.SLO(ctx)
	if err != nil {
		return nil, err
	}
	return rest.GetStatsSlo200JSONResponse{Body: SLOREST(r)}, nil
}

// GetStatsBackends implements rest.StrictServerInterface (GET /stats/backends).
func (h *Views) GetStatsBackends(ctx context.Context, req rest.GetStatsBackendsRequestObject) (rest.GetStatsBackendsResponseObject, error) {
	p := req.Params
	q := stats.BackendQuery{From: p.From, To: p.To}
	if p.SpaceId != nil {
		q.SpaceID = ids.ID(*p.SpaceId)
	}
	items, err := h.v.Backends(ctx, q)
	if err != nil {
		return nil, err
	}
	return rest.GetStatsBackends200JSONResponse{Body: BackendsREST(items)}, nil
}

// TimelineREST converts a timeline to the schema StatsTimeline: channel_id, agent_id, currency,
// started_at and finished_at absent when empty.
func TimelineREST(tl stats.Timeline) rest.StatsTimeline {
	out := rest.StatsTimeline{From: timestamp(tl.From), To: timestamp(tl.To), Buckets: make([]rest.StatsTimelineBucket, len(tl.Buckets)),
		Runs: make([]rest.StatsTimelineRun, len(tl.Runs)), RunsTruncated: tl.RunsTruncated}
	for i, b := range tl.Buckets {
		out.Buckets[i] = rest.StatsTimelineBucket{Start: timestamp(b.Start), Metric: rest.StatsMetric(b.Metric), ChannelId: optionalID(b.ChannelID),
			AgentId: optionalID(b.AgentID), Dimension: b.Dimension, Currency: optionalString(b.Currency), Count: b.Count, Sum: b.Sum, Good: b.Good}
	}
	for i, r := range tl.Runs {
		out.Runs[i] = rest.StatsTimelineRun{RunId: rest.Uuid(r.RunID), ChannelId: rest.Uuid(r.ChannelID), AgentId: rest.Uuid(r.AgentID),
			QueuedAt: timestamp(r.QueuedAt), StartedAt: optionalTime(r.StartedAt), FinishedAt: optionalTime(r.FinishedAt), State: rest.RunState(r.State)}
	}
	return out
}

// SLOREST converts a traffic light report to the schema StatsSloReport with measured_by own;
// threshold_ms, ratio and budget_remaining absent when the entry has none.
func SLOREST(r stats.SLOReport) rest.StatsSloReport {
	out := rest.StatsSloReport{At: timestamp(r.At), MeasuredBy: rest.Own, Items: make([]rest.StatsSloEntry, len(r.Items))}
	for i, e := range r.Items {
		item := rest.StatsSloEntry{Slo: e.SLO, Metric: rest.StatsMetric(e.Metric), Objective: e.Objective, PeriodDays: e.PeriodDays,
			Total: e.Total, Good: e.Good, State: rest.StatsSloState(e.State)}
		if e.Threshold > 0 {
			ms := int(e.Threshold / time.Millisecond)
			item.ThresholdMs = &ms
		}
		if e.Ratio != nil {
			ratio := *e.Ratio
			item.Ratio = &ratio
		}
		if e.BudgetRemaining != nil {
			budget := *e.BudgetRemaining
			item.BudgetRemaining = &budget
		}
		out.Items[i] = item
	}
	return out
}

// BackendsREST converts the backend view to the schema StatsBackendReport: costs present (also
// empty) exactly when they are shown, reported_billing_mode and rate_limit absent when not
// reported. The schema has no member for a runtime, adapter kind, version or evidence (D-M6-14-4).
func BackendsREST(items []stats.BackendAgent) rest.StatsBackendReport {
	out := rest.StatsBackendReport{Items: make([]rest.StatsBackendAgent, len(items))}
	for i, a := range items {
		item := rest.StatsBackendAgent{AgentId: rest.Uuid(a.AgentID), BillingMode: rest.BillingMode(a.BillingMode),
			BillingModesSupported: make([]rest.BillingMode, len(a.BillingModesSupported)), Switches: a.Switches,
			InvocationsDenied: rest.StatsBackendDenials{I2Denied: a.I2Denied, BillingModeMismatch: a.BillingModeMismatch}}
		for j, m := range a.BillingModesSupported {
			item.BillingModesSupported[j] = rest.BillingMode(m)
		}
		if a.ReportedBillingMode != "" {
			m := rest.BillingMode(a.ReportedBillingMode)
			item.ReportedBillingMode = &m
		}
		if a.CostsShown {
			costs := make([]rest.StatsBackendCost, len(a.Costs))
			for j, c := range a.Costs {
				costs[j] = rest.StatsBackendCost{BillingMode: rest.BillingMode(c.BillingMode), Currency: c.Currency, AmountMicros: c.AmountMicros,
					IsEstimate: c.IsEstimate}
			}
			item.Costs = &costs
		}
		if a.RateLimit != nil {
			item.RateLimit = &rest.AgentRateLimitState{State: rest.AgentRateLimitStateState(a.RateLimit.State), ResetsAt: optionalTime(a.RateLimit.ResetsAt)}
		}
		out.Items[i] = item
	}
	return out
}

// optionalID is id as an optional Uuid member: nil for the zero ID.
func optionalID(id ids.ID) *rest.Uuid {
	if id.IsZero() {
		return nil
	}
	u := rest.Uuid(id)
	return &u
}

// optionalString is s as an optional member: nil for "".
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optionalTime is t as an optional Timestamp member: nil for the zero time.
func optionalTime(t time.Time) *rest.Timestamp {
	if t.IsZero() {
		return nil
	}
	ts := timestamp(t)
	return &ts
}
