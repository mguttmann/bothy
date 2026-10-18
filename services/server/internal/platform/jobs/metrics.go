// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// queueStates are the River job states bothy.job.queue counts, the jobs that wait to run
// (signals.md 4.4), as an SQL list of River's enum literals.
const queueStates = "('available', 'scheduled', 'retryable')"

// jobMetrics are bothy.job.duration and bothy.job.queue of the runtime, declared in the metric
// registry; bothy.job.kind takes its values from the kinds registered at startup.
type jobMetrics struct {
	duration *metrics.Histogram
	queue    *metrics.ObservableGauge
	kinds    []string
}

// newJobMetrics creates both instruments on mp (nil means the global provider) for kinds.
func newJobMetrics(mp metric.MeterProvider, kinds []string) (*jobMetrics, error) {
	v := metrics.Values{metrics.SourceJobKinds: kinds}
	d, err := metrics.NewHistogram(mp, metrics.JobDuration, v)
	if err != nil {
		return nil, fmt.Errorf("jobs: %w", err)
	}
	q, err := metrics.NewObservableGauge(mp, metrics.JobQueue, v)
	if err != nil {
		return nil, fmt.Errorf("jobs: %w", err)
	}
	return &jobMetrics{duration: d, queue: q, kinds: kinds}, nil
}

// observed records one attempt of kind that ran for d with outcome.
func (m *jobMetrics) observed(ctx context.Context, kind, outcome string, d time.Duration) {
	m.duration.RecordDuration(context.WithoutCancel(ctx), d, kind, outcome)
}

// registerQueue registers the callback of bothy.job.queue for a runtime that works: one grouped
// count over River's job table per collection, bounded by timeout (database.statement_timeout),
// observing every registered kind (0 when it has no waiting job). A failed count drops the points
// of that collection and never blocks it (17 rule 4).
func (m *jobMetrics) registerQueue(pool *pg.Pool, schema string, timeout time.Duration) (metric.Registration, error) {
	q := "SELECT kind, count(*) FROM " + pgx.Identifier{schema, "river_job"}.Sanitize() +
		" WHERE state IN " + queueStates + " GROUP BY kind"
	return m.queue.Register(func(ctx context.Context, o metric.Observer) error {
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		counts := make(map[string]int64, len(m.kinds))
		rows, err := pool.PGX().Query(ctx, q)
		if err != nil {
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var kind string
			var n int64
			if err := rows.Scan(&kind, &n); err != nil {
				return nil
			}
			counts[kind] = n
		}
		if rows.Err() != nil {
			return nil
		}
		for _, k := range m.kinds {
			m.queue.Observe(o, float64(counts[k]), k)
		}
		return nil
	})
}

// sortedKinds returns the registered kinds in order.
func sortedKinds(kinds map[string]*entry) []string {
	return slices.Sorted(maps.Keys(kinds))
}
