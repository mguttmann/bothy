// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats"
)

// Statistics implements the REST operations getUsageStats, getRunStats, getToolCallStats,
// getErrorStats and getCostStats (M6-05) as part of rest.StrictServerInterface. The use cases
// authorize, filter and cache; the handlers only convert and pass errors to the problem writer of
// the chain unchanged. The request validator of the chain has already refused an unknown query
// member, a malformed timestamp and a value outside the enums with 400.
//
// Example:
//
//	h := httpapi.NewStatistics(st)
//	built, err := routes.Build(wire.Handlers{Statistics: h}, wire.RouteOptions(), cat)
type Statistics struct {
	st stats.Statistics
}

// NewStatistics returns the handlers over the port st.
func NewStatistics(st stats.Statistics) *Statistics { return &Statistics{st: st} }

// GetUsageStats implements rest.StrictServerInterface (GET /stats/usage).
func (h *Statistics) GetUsageStats(ctx context.Context, req rest.GetUsageStatsRequestObject) (rest.GetUsageStatsResponseObject, error) {
	p := req.Params
	s, err := h.st.Usage(ctx, query(p.From, p.To, p.Granularity, p.SpaceId, nil))
	if err != nil {
		return nil, err
	}
	return rest.GetUsageStats200JSONResponse{Body: SeriesREST(s)}, nil
}

// GetRunStats implements rest.StrictServerInterface (GET /stats/runs).
func (h *Statistics) GetRunStats(ctx context.Context, req rest.GetRunStatsRequestObject) (rest.GetRunStatsResponseObject, error) {
	p := req.Params
	s, err := h.st.Runs(ctx, query(p.From, p.To, p.Granularity, p.SpaceId, p.AgentId))
	if err != nil {
		return nil, err
	}
	return rest.GetRunStats200JSONResponse{Body: SeriesREST(s)}, nil
}

// GetToolCallStats implements rest.StrictServerInterface (GET /stats/tool-calls).
func (h *Statistics) GetToolCallStats(ctx context.Context, req rest.GetToolCallStatsRequestObject) (rest.GetToolCallStatsResponseObject, error) {
	p := req.Params
	s, err := h.st.ToolCalls(ctx, query(p.From, p.To, p.Granularity, p.SpaceId, p.AgentId))
	if err != nil {
		return nil, err
	}
	return rest.GetToolCallStats200JSONResponse{Body: SeriesREST(s)}, nil
}

// GetErrorStats implements rest.StrictServerInterface (GET /stats/errors).
func (h *Statistics) GetErrorStats(ctx context.Context, req rest.GetErrorStatsRequestObject) (rest.GetErrorStatsResponseObject, error) {
	p := req.Params
	s, err := h.st.Errors(ctx, query(p.From, p.To, p.Granularity, p.SpaceId, nil))
	if err != nil {
		return nil, err
	}
	return rest.GetErrorStats200JSONResponse{Body: SeriesREST(s)}, nil
}

// GetCostStats implements rest.StrictServerInterface (GET /stats/costs).
func (h *Statistics) GetCostStats(ctx context.Context, req rest.GetCostStatsRequestObject) (rest.GetCostStatsResponseObject, error) {
	p := req.Params
	q := stats.CostQuery{Query: query(p.From, p.To, p.Granularity, p.SpaceId, p.AgentId)}
	if p.GroupBy != nil {
		q.GroupBy = string(*p.GroupBy)
	}
	if p.Currency != nil {
		q.Currency = *p.Currency
	}
	s, err := h.st.Costs(ctx, q)
	if err != nil {
		return nil, err
	}
	out := rest.CostSeries{Granularity: rest.StatsGranularity(s.Granularity), Currency: s.Currency, Buckets: make([]rest.CostBucket, len(s.Buckets))}
	for i, b := range s.Buckets {
		cb := rest.CostBucket{Start: timestamp(b.Start), BillingMode: rest.BillingMode(b.BillingMode), AmountMicros: b.AmountMicros, IsEstimate: b.IsEstimate}
		if b.GroupKey != "" {
			k := b.GroupKey
			cb.GroupKey = &k
		}
		out.Buckets[i] = cb
	}
	return rest.GetCostStats200JSONResponse{Body: out}, nil
}

// SeriesREST converts a series to the schema StatsSeries: buckets never null, metrics never null,
// group_key and below_min_group absent when empty.
func SeriesREST(s stats.Series) rest.StatsSeries {
	out := rest.StatsSeries{Granularity: rest.StatsGranularity(s.Granularity), Buckets: make([]rest.StatsBucket, len(s.Buckets))}
	for i, b := range s.Buckets {
		metrics := make(map[string]float64, len(b.Metrics))
		for k, v := range b.Metrics {
			metrics[k] = v
		}
		sb := rest.StatsBucket{Start: timestamp(b.Start), Metrics: metrics}
		if b.GroupKey != "" {
			k := b.GroupKey
			sb.GroupKey = &k
		}
		if len(b.BelowMinGroup) > 0 {
			below := append([]string(nil), b.BelowMinGroup...)
			sb.BelowMinGroup = &below
		}
		out.Buckets[i] = sb
	}
	return out
}

// query builds the port's query from the parameters; absent ones stay zero.
func query(from, to time.Time, g *rest.StatsGranularity, space, agent *rest.Uuid) stats.Query {
	q := stats.Query{From: from, To: to}
	if g != nil {
		q.Granularity = string(*g)
	}
	if space != nil {
		q.SpaceID = ids.ID(*space)
	}
	if agent != nil {
		q.AgentID = ids.ID(*agent)
	}
	return q
}

// timestamp is t in UTC at the millisecond precision of the schema Timestamp.
func timestamp(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }
