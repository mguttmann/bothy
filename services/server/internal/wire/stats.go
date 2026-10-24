// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	appspg "github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	billingpg "github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres"
	billingapp "github.com/OWNER/bothy/services/server/internal/billing/app"
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
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	runspg "github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	statshttp "github.com/OWNER/bothy/services/server/internal/stats/adapters/httpapi"
	statsotel "github.com/OWNER/bothy/services/server/internal/stats/adapters/otelmetrics"
	statsapp "github.com/OWNER/bothy/services/server/internal/stats/app"
)

// Stats is the part of the stats module the start builds: the client metrics of the apps
// (M4-31, ADR 0057).
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
