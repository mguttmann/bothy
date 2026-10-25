// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	stdsync "sync"
	"time"

	gatewaydomain "github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	statshttp "github.com/OWNER/bothy/services/server/internal/stats/adapters/httpapi"
	statsotel "github.com/OWNER/bothy/services/server/internal/stats/adapters/otelmetrics"
	statspg "github.com/OWNER/bothy/services/server/internal/stats/adapters/postgres"
	statsapp "github.com/OWNER/bothy/services/server/internal/stats/app"
)

// Stats is the part of the stats module the start builds: the client metrics of the apps
// (M4-31, ADR 0057).
type Stats struct {
	// ClientMetrics implements stats.ClientMetrics.
	ClientMetrics *statsapp.ClientMetrics
	// Handlers is the REST handler of submitClientMetrics.
	Handlers *statshttp.ClientMetrics
}

// NewStats builds the client metrics over the Authorizer az and the settings reader of sync
// (sync.Settings, of which the use case calls only ValuesFor; stats may not import sync, 02 4.4).
// The instruments go to the global meter provider of otel.Setup and the process set of the
// pre-aggregated histograms; bothy.close.code takes the close codes of the gateway (04 6.4).
// Errors: a missing collaborator, or an error of the metric registry.
func NewStats(az authorize.Authorizer, settings statsapp.SettingsSource) (*Stats, error) {
	if az == nil || settings == nil {
		return nil, errors.New("wire: stats needs the Authorizer and the settings of sync")
	}
	var codes []int
	for _, c := range gatewaydomain.CloseCodes() {
		codes = append(codes, int(c))
	}
	rec, err := statsotel.New(nil, metrics.DefaultAggregated(), codes)
	if err != nil {
		return nil, err
	}
	cm, err := statsapp.NewClientMetrics(statsapp.ClientMetricsDeps{Authorizer: az, Settings: settings, Recorder: rec, CloseCodes: codes})
	if err != nil {
		return nil, err
	}
	return &Stats{ClientMetrics: cm, Handlers: statshttp.New(cm)}, nil
}

// fillHandlers sets the nil stats handler of h to the one st built; a nil st leaves h unchanged.
func (st *Stats) fillHandlers(h *Handlers) {
	if st != nil && h.ClientMetrics == nil {
		h.ClientMetrics = st.Handlers
	}
}

// MeasureConfig holds the keys of the own measurement (M6-13): the flush of
// docs/spec/25-visualization.md 15, the retention class stats of 20 and the lock timeout of a
// detach.
type MeasureConfig struct {
	// FlushInterval is stats.flush_interval.
	FlushInterval time.Duration `config:"stats.flush_interval"`
	// QueueMax is stats.flush_queue_max.
	QueueMax int `config:"stats.flush_queue_max"`
	// Retention is retention.stats_minutes.
	Retention time.Duration `config:"retention.stats_minutes"`
	// LockTimeout is database.lock_timeout, the bound of a detach of stats.retention.
	LockTimeout time.Duration `config:"database.lock_timeout"`
}

// MeasureConfigFrom binds MeasureConfig from v; the error names the key, never a value.
func MeasureConfigFrom(v *config.Values) (MeasureConfig, error) {
	var c MeasureConfig
	if err := config.Bind(v, &c); err != nil {
		return MeasureConfig{}, err
	}
	return c, nil
}

// Measure is the own measurement of one process (M6-13, 25 3): the recorder the measuring points
// of every module call, its flush to stats_minutes and the use case of the job stats.retention.
type Measure struct {
	// Recorder implements measure.Recorder.
	Recorder *statsapp.Measurement
	// Flush writes the counters every stats.flush_interval and once at the shutdown.
	Flush *statsapp.Flush
	// Retention is the use case of stats.retention.
	Retention *statsapp.Retention

	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce stdsync.Once
}

// NewMeasure builds the own measurement over pool with cfg, the clock (nil means clock.System())
// and log, with the instruments bothy.stats.flush.dropped and bothy.stats.flush.duration on the
// global meter provider of otel.Setup. It starts nothing. Errors: a configuration the use cases
// refuse, or the instruments.
func NewMeasure(cfg MeasureConfig, pool *pg.Pool, clk clock.Clock, log *slog.Logger) (*Measure, error) {
	if clk == nil {
		clk = clock.System()
	}
	m, err := statsotel.NewFlush(nil)
	if err != nil {
		return nil, err
	}
	rec, err := statsapp.NewMeasurement(clk, cfg.QueueMax, m)
	if err != nil {
		return nil, err
	}
	repo := statspg.NewRepository(pool)
	flush, err := statsapp.NewFlush(rec, repo, m, clk, cfg.FlushInterval, log)
	if err != nil {
		return nil, err
	}
	ret, err := statsapp.NewRetention(clk, repo, cfg.Retention, cfg.LockTimeout)
	if err != nil {
		return nil, err
	}
	return &Measure{Recorder: rec, Flush: flush, Retention: ret}, nil
}

// Start runs the periodic flush until the shutdown and registers the stop step "stats_flush" in
// lifecycle.PhaseDatabase: it ends the loop and flushes once within the rest of the shutdown
// budget (25 3.4 item 4). Call Start from a component's Start, after the stop step of the pool was
// registered: steps of one phase run in reverse registration order, so the flush runs before the
// pool closes. Every role calls it.
func (m *Measure) Start(r *lifecycle.Runner) {
	// The stop step calls cancel (Measure.stop); gosec cannot see that through the struct field.
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is called by the stop step stats_flush
	m.cancel, m.done = cancel, make(chan struct{})
	go func() {
		defer close(m.done)
		m.Flush.Run(ctx)
	}()
	r.OnStop(lifecycle.StopStep{Name: "stats_flush", Phase: lifecycle.PhaseDatabase, Run: m.stop})
}

// stop ends the periodic flush and writes the counters once within the budget of ctx; what does
// not make it is dropped and counted as db_error (statsapp.Flush.Shutdown).
func (m *Measure) stop(ctx context.Context) error {
	m.stopOnce.Do(m.cancel)
	select {
	case <-m.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return m.Flush.Shutdown(ctx)
}
