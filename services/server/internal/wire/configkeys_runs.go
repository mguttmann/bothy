// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// runsConfigKeys returns the keys of the runs module from docs/spec/03-data-model.md
// "Konfigurationswerte dieses Kapitels" (M2-05): the largest encoded run_steps.detail, to which the
// detail of the adapter_reason step is cut (D-M2-05-8); and from 09 section 21 (M2-08) the
// largest age of a queued run.
func runsConfigKeys() []config.Key {
	return []config.Key{
		{Name: "run.step_detail_max_bytes", Kind: config.KindInt, Default: "16384", Min: "64", Max: "1048576", Chapter: chapter03,
			Doc: "Largest encoded run_steps.detail in bytes; a longer detail is cut."},
		// M2-08: a run nobody starts within this age fails with agent_unavailable (09 5.5, D-AG-16).
		{Name: "run.queued_max_age", Kind: config.KindDuration, Default: "48h", Min: "1s", Chapter: chapter09,
			Doc: "How old a run that was not started may become; then it fails with agent_unavailable."},
	}
}
