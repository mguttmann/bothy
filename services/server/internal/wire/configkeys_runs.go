// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// runsConfigKeys returns the keys of the runs module from docs/spec/03-data-model.md
// "Konfigurationswerte dieses Kapitels" (M2-05): the largest encoded run_steps.detail, to which the
// detail of the adapter_reason step is cut (D-M2-05-8); and from 09 section 21 the largest age of
// a queued run (M2-08) and the window of a run without heartbeat (M2-09); the limits of the
// components of cards from 03 and 09 section 21 (M2-15).
func runsConfigKeys() []config.Key {
	return []config.Key{
		{Name: "run.step_detail_max_bytes", Kind: config.KindInt, Default: "16384", Min: "64", Max: "1048576", Chapter: chapter03,
			Doc: "Largest encoded run_steps.detail in bytes; a longer detail is cut."},
		// M2-08: a run nobody starts within this age fails with agent_unavailable (09 5.5, D-AG-16).
		{Name: "run.queued_max_age", Kind: config.KindDuration, Default: "48h", Min: "1s", Chapter: chapter09,
			Doc: "How old a run that was not started may become; then it fails with agent_unavailable."},
		// M2-09: a started run whose agent sends no heartbeat this long fails with stale (09 5.5).
		{Name: keyOrphanTimeout, Kind: config.KindDuration, Default: "15m", Min: minDeadline, Chapter: chapter09,
			Doc: "Without a heartbeat for this long while a run is active, the run fails with stale."},
		// M2-15: the components of cards (03 message_components, 09 11.1).
		{Name: "component.default_ttl", Kind: config.KindDuration, Default: defaultDay, Min: "1s", Chapter: chapter03,
			Doc: "Expiry of a component without its own expires_at."},
		{Name: "component.max_per_message", Kind: config.KindInt, Default: "25", Min: "1", Max: "1000", Chapter: chapter09,
			Doc: "Most components of one card message."},
		{Name: "component.custom_id_max_chars", Kind: config.KindInt, Default: "100", Min: "1", Chapter: chapter09,
			Doc: "Longest custom_id of a component in Unicode characters."},
		{Name: "component.form_max_fields", Kind: config.KindInt, Default: "10", Min: "1", Chapter: chapter09,
			Doc: "Most fields of one form component."},
	}
}

// keyOrphanTimeout is run.orphan_timeout, which the cross-check
// heartbeat_persist_below_orphan_timeout compares with agent.heartbeat_persist_interval.
const keyOrphanTimeout = "run.orphan_timeout"
