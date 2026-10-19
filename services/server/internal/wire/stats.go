// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	gatewaydomain "github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
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
