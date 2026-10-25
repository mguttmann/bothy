// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	appspg "github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	billingpg "github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres"
	billingapp "github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/chat"
	chatpg "github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres"
	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	filespg "github.com/OWNER/bothy/services/server/internal/files/adapters/postgres"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	gatewaydomain "github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	messagingpg "github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	notifypg "github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/runs"
	runspg "github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	statshttp "github.com/OWNER/bothy/services/server/internal/stats/adapters/httpapi"
	statsotel "github.com/OWNER/bothy/services/server/internal/stats/adapters/otelmetrics"
	statspg "github.com/OWNER/bothy/services/server/internal/stats/adapters/postgres"
	statsapp "github.com/OWNER/bothy/services/server/internal/stats/app"
)

// Stats is the part of the stats module the start builds: the client metrics of the apps
// (M4-31, ADR 0057), the statistics queries (M6-05) and the live stream (M6-15).
type Stats struct {
	// ClientMetrics implements stats.ClientMetrics.
	ClientMetrics *statsapp.ClientMetrics
	// Handlers is the REST handler of submitClientMetrics.
	Handlers *statshttp.ClientMetrics
	// Statistics are the statistics queries (M6-05), built in the roles api and all; nil in the
	// other roles.
	Statistics *statsapp.Statistics
	// StatisticsHandlers are the REST handlers of the five statistics operations; nil with
	// Statistics.
	StatisticsHandlers *statshttp.Statistics
	// Live is the live stream stats.live of the node and LiveHandler the REST handler of
	// getStatsLive (M6-15); nil in the role worker.
	Live        *statsapp.Live
	LiveHandler *statshttp.Live
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
	if st != nil && h.Statistics == nil && st.StatisticsHandlers != nil {
		h.Statistics = st.StatisticsHandlers
	}
	if st != nil && h.Live == nil && st.LiveHandler != nil {
		h.Live = st.LiveHandler
	}
}

// StatsConfig binds the keys of the statistics queries (stats.cache_ttl, stats.cache_max_entries,
// stats.max_range, stats.time_zone, api.stats_max_buckets, privacy.stats_min_group_size,
// privacy.per_person_stats_enabled; M6-05). The error names the key whose binding failed, never a
// value.
func StatsConfig(v *config.Values) (statsapp.Config, error) {
	var c statsapp.Config
	if err := config.Bind(v, &c); err != nil {
		return statsapp.Config{}, err
	}
	return c, nil
}

// NewStatistics builds the statistics queries (M6-05) over pool: the statistics sources of chat,
// messaging, runs, billing (with the display rule of billingCfg), files, apps and notify, each over
// its own module's PostgreSQL adapter, the Authorizer az and the clock of the cache (nil means
// clock.System()). It opens no connection. Errors: a missing collaborator, an invalid
// configuration (stats.time_zone that is no IANA name).
func NewStatistics(cfg statsapp.Config, billingCfg billingapp.Config, pool *pg.Pool, az authorize.Authorizer, clk clock.Clock) (*statsapp.Statistics, error) {
	if pool == nil || az == nil {
		return nil, errors.New("wire: the statistics need the pool and the Authorizer")
	}
	ch, err := chatapp.NewStats(chatpg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	msg, err := messagingapp.NewStats(messagingpg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	rs, err := runsapp.NewStats(runspg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	bl, err := billingapp.NewStats(billingCfg, billingpg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	fs, err := filesapp.NewStats(filespg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	ap, err := appsapp.NewStats(appspg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	nt, err := notifyapp.NewStats(notifypg.NewRepository(pool))
	if err != nil {
		return nil, err
	}
	return statsapp.NewStatistics(cfg, statsapp.Deps{Authorizer: az, Chat: ch, Messaging: msg, Runs: rs, Billing: bl, Files: fs, Apps: ap,
		Notify: nt, Clock: clk})
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

// liveStream returns the live stream st built, nil without one (a nil st, the role worker).
func (st *Stats) liveStream() *statsapp.Live {
	if st == nil {
		return nil
	}
	return st.Live
}

// StatsLiveConfig binds the keys of the live stream (M6-15, docs/spec/25-visualization.md 15),
// the payload bound of 04 and the cluster mode of 02, which decides the member connections.
type StatsLiveConfig struct {
	Enabled        bool          `config:"stats.live_enabled"`
	Interval       time.Duration `config:"stats.live_interval"`
	MaxRuns        int           `config:"stats.live_max_runs"`
	MaxSubscribers int           `config:"stats.live_max_subscribers"`
	SLIWindow      time.Duration `config:"stats.live_sli_window"`
	PayloadMax     int           `config:"sync.event_payload_max_bytes"`
	ClusterMode    string        `config:"cluster.mode"`
}

// StatsLiveConfigFrom binds StatsLiveConfig from v; the error names the key, never a value.
func StatsLiveConfigFrom(v *config.Values) (StatsLiveConfig, error) {
	var c StatsLiveConfig
	if err := config.Bind(v, &c); err != nil {
		return StatsLiveConfig{}, err
	}
	return c, nil
}

// StatsLiveDeps are the sources of the live stream from the other modules of the start.
type StatsLiveDeps struct {
	Authorizer authorize.Authorizer
	Runs       runs.LiveSource
	Places     chat.ChannelPlaces
	Facts      agents.Facts
	Presence   agents.Presence
	Pool       *pg.Pool
	Clock      clock.Clock
	Log        *slog.Logger
}

// NewStatsLive builds the live stream of an api process (M6-15) over d with cfg and the gauge
// bothy.stats.live.subscribers on the global meter provider of otel.Setup; connections appear in
// the frame only with cluster.mode single. It starts nothing. Errors: a missing source, an invalid
// value or the instrument.
func NewStatsLive(cfg StatsLiveConfig, d StatsLiveDeps) (*statsapp.Live, error) {
	if d.Pool == nil {
		return nil, errors.New("wire: the live stream needs the pool")
	}
	m, err := statsotel.NewLive(nil)
	if err != nil {
		return nil, err
	}
	return statsapp.NewLive(statsapp.LiveConfig{
		Enabled: cfg.Enabled, Interval: cfg.Interval, MaxRuns: cfg.MaxRuns, MaxSubscribers: cfg.MaxSubscribers, SLIWindow: cfg.SLIWindow,
		PayloadMax: cfg.PayloadMax, Connections: cfg.ClusterMode == clusterModeSingle,
	}, statsapp.LiveDeps{Authorizer: d.Authorizer, Runs: d.Runs, Places: d.Places, Facts: d.Facts, Presence: d.Presence,
		Store: statspg.NewRepository(d.Pool), Clock: d.Clock, Metrics: m, Log: d.Log})
}

// startLiveLoop runs the builder loop of live (Live.Run) for the life of the process and returns
// its stop step in the HTTP phase.
func startLiveLoop(live *statsapp.Live) lifecycle.StopStep {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		live.Run(ctx)
	}()
	return lifecycle.StopStep{Name: "stats_live", Phase: lifecycle.PhaseHTTP, Run: func(stopCtx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stopCtx.Done():
			return stopCtx.Err()
		}
	}}
}
