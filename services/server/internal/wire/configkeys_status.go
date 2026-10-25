// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// The keys of the status page (M7-09) that the cross-check reads by name.
const (
	keyStatusErrorRatioDegraded = "status.error_ratio_degraded"
	keyStatusErrorRatioOutage   = "status.error_ratio_outage"
)

// statusConfigKeys returns the keys of the status page (M7-09): the automatic checks of
// docs/spec/16-operations.md 14, status.public_enabled of docs/spec/05-actions.md 10,
// status.history_window of docs/spec/06-api.md 20 and retention.ops of docs/spec/20-privacy.md 13.
func statusConfigKeys() []config.Key {
	return []config.Key{
		{Name: "status.check_interval", Kind: config.KindDuration, Default: "30s", Min: "1s", Chapter: chapter16,
			Doc: "Interval of the automatic checks of the status page (job status.probe)."},
		{Name: "status.degrade_after", Kind: config.KindDuration, Default: "2m", Min: "1s", Chapter: chapter16,
			Doc: "How long the checks of a component fail until it is degraded or in major outage."},
		{Name: "status.incident_after", Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: chapter16,
			Doc: "How long a major outage lasts until the automatic incident."},
		{Name: "status.resolve_after", Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: chapter16,
			Doc: "How long a component stays operational until its automatic incident is resolved."},
		{Name: keyStatusErrorRatioDegraded, Kind: config.KindFloat, Default: "0.05", Min: "0", Max: "1", Chapter: chapter16,
			Doc: "Error ratio above which a check fails."},
		{Name: keyStatusErrorRatioOutage, Kind: config.KindFloat, Default: "0.5", Min: "0", Max: "1", Chapter: chapter16,
			Doc: "Error ratio above which a failing component is in major outage."},
		{Name: "status.fanout_lag_degraded", Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: chapter16,
			Doc: "Mean fan-out lag above which the check sync.fanout fails."},
		{Name: "status.public_enabled", Kind: config.KindBool, Default: defaultFalse, Chapter: chapter05,
			Doc: "Serve the status page without login (getPublicStatus); off answers 404."},
		{Name: "status.history_window", Kind: config.KindDuration, Default: "7d", Min: "1m", Chapter: chapter06,
			Doc: "How long the status page shows resolved incidents."},
		{Name: "retention.ops", Kind: config.KindDuration, Default: "395d", Min: "1d", Chapter: chapter20,
			Doc: "How long resolved incidents and backup runs are kept; the latest successful backup run per kind always stays."},
	}
}

// statusErrorRatios is the cross-check status_error_ratios: status.error_ratio_degraded is at
// most status.error_ratio_outage, else a check could be in outage without failing.
func statusErrorRatios(v *config.Values) error {
	if v.Float(keyStatusErrorRatioDegraded) > v.Float(keyStatusErrorRatioOutage) {
		return errors.New("status.error_ratio_degraded must not be above status.error_ratio_outage")
	}
	return nil
}
