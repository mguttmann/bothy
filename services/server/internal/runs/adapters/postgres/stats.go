// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	runsport "github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// Compile-time check: Repository answers the statistics reads of the app (M6-05).
var _ app.StatsReader = (*Repository)(nil)

// RunStats implements app.StatsReader with the statements RunStateStats and RunDurationStats
// (M6-05): the counts per end state and failure reason, then the duration percentiles.
func (r *Repository) RunStats(ctx context.Context, q runsport.StatsQuery) ([]runsport.StatsValue, error) {
	threads, groups, agent := statsUUIDs(q.ThreadIDs), statsUUIDs(q.GroupIDs), statsAgent(q.AgentID)
	db := r.read(ctx)
	states, err := db.RunStateStats(ctx, sqlcgen.RunStateStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone, ByAgent: q.ByAgent,
		ThreadIds: threads, GroupIds: groups, FromAt: statsTS(q.From), ToAt: statsTS(q.To), AgentID: agent})
	if err != nil {
		return nil, statsError(err, "counting runs")
	}
	type key struct {
		bucket time.Time
		group  string
	}
	totals := map[key]float64{}
	var order []key
	var out []runsport.StatsValue
	for _, row := range states {
		k := key{row.Bucket.Time, statsGroup(row.GroupID)}
		if _, ok := totals[k]; !ok {
			order = append(order, k)
		}
		totals[k] += float64(row.Value)
		out = append(out, runsport.StatsValue{Bucket: k.bucket, Group: k.group, Metric: row.State, Value: float64(row.Value)})
		if row.State == runsport.MetricFailed && row.StateReason != "" {
			out = append(out, runsport.StatsValue{Bucket: k.bucket, Group: k.group, Metric: runsport.MetricFailedReasonPrefix + row.StateReason, Value: float64(row.Value)})
		}
	}
	for _, k := range order {
		out = append(out, runsport.StatsValue{Bucket: k.bucket, Group: k.group, Metric: runsport.MetricRuns, Value: totals[k]})
	}
	durations, err := db.RunDurationStats(ctx, sqlcgen.RunDurationStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone, ByAgent: q.ByAgent,
		ThreadIds: threads, GroupIds: groups, FromAt: statsTS(q.From), ToAt: statsTS(q.To), AgentID: agent})
	if err != nil {
		return nil, statsError(err, "computing run durations")
	}
	for _, row := range durations {
		b, g := row.Bucket.Time, statsGroup(row.GroupID)
		out = append(out, runsport.StatsValue{Bucket: b, Group: g, Metric: runsport.MetricDurationP50, Value: row.P50},
			runsport.StatsValue{Bucket: b, Group: g, Metric: runsport.MetricDurationP95, Value: row.P95})
	}
	return out, nil
}

// ToolStats implements app.StatsReader with the statement ToolStats (M6-05). The median is absent
// for a bucket without a finished call.
func (r *Repository) ToolStats(ctx context.Context, q runsport.StatsQuery) ([]runsport.StatsValue, error) {
	rows, err := r.read(ctx).ToolStats(ctx, sqlcgen.ToolStatsParams{Granularity: q.Granularity, TimeZone: q.TimeZone,
		ThreadIds: statsUUIDs(q.ThreadIDs), FromAt: statsTS(q.From), ToAt: statsTS(q.To), AgentID: statsAgent(q.AgentID)})
	if err != nil {
		return nil, statsError(err, "counting tool calls")
	}
	var out []runsport.StatsValue
	for _, row := range rows {
		b := row.Bucket.Time
		out = append(out, runsport.StatsValue{Bucket: b, Group: row.Name, Metric: runsport.MetricCalls, Value: float64(row.Calls)})
		if row.Failed > 0 {
			out = append(out, runsport.StatsValue{Bucket: b, Group: row.Name, Metric: runsport.MetricFailed, Value: float64(row.Failed)})
		}
		if row.P50 >= 0 {
			out = append(out, runsport.StatsValue{Bucket: b, Group: row.Name, Metric: runsport.MetricDurationP50, Value: row.P50})
		}
	}
	return out, nil
}

// statsAgent is the optional agent filter: NULL for every agent.
func statsAgent(id ids.ID) pgtype.UUID {
	if id.IsZero() {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// statsUUIDs converts ids for an array parameter of the statistics reads.
func statsUUIDs(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(list))
	for i, id := range list {
		out[i] = pgtype.UUID{Bytes: id, Valid: true}
	}
	return out
}

// statsTS converts an instant for a parameter of the statistics reads.
func statsTS(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// statsGroup is the group key of a statistics row as text: the UUID, or "" for NULL.
func statsGroup(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return ids.ID(u.Bytes).String()
}

// statsError wraps a database error of a statistics read as internal_error.
func statsError(err error, what string) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, ": "+what+" failed")
}
