// SPDX-License-Identifier: Apache-2.0

package stats

import (
	"context"
	"time"
)

// SLITotal is the sum of one sli.* metric of stats_minutes over a window for one dimension
// (docs/spec/25-visualization.md 3.2): Count events, Sum of their durations in milliseconds, Good
// of them within the bound of the SLO.
type SLITotal struct {
	// Dimension is stats_minutes.dimension, for example the provider of sli.push_acceptance; ""
	// for a metric without one.
	Dimension string
	Count     int64
	Sum       int64
	Good      int64
}

// SLIWindows is the read port of the own measurement for the automatic checks of the status page
// (M7-09, docs/spec/16-operations.md 12): the checks read the measurement of every process from
// stats_minutes, never from Prometheus (R-OP-15). Only the stats module runs SQL on the table (03
// principle 12). Implementations are safe for concurrent use.
type SLIWindows interface {
	// SLITotals returns the sums of metric over the minute buckets with bucket_start in [from,
	// to), one entry per dimension in dimension order, over every channel and agent; none when the
	// window has no row. Errors: the database error.
	//
	// Example:
	//
	//	totals, err := w.SLITotals(ctx, "sli.api_availability", from, to)
	SLITotals(ctx context.Context, metric string, from, to time.Time) ([]SLITotal, error)
}
