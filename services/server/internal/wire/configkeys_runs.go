// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// runsConfigKeys returns the keys of the runs module from docs/spec/03-datenmodell.md
// "Konfigurationswerte dieses Kapitels" (M2-05): the largest encoded run_steps.detail, to which the
// detail of the adapter_reason step is cut (D-M2-05-8).
func runsConfigKeys() []config.Key {
	return []config.Key{
		{Name: "run.step_detail_max_bytes", Kind: config.KindInt, Default: "16384", Min: "64", Max: "1048576", Chapter: chapter03,
			Doc: "Largest encoded run_steps.detail in bytes; a longer detail is cut."},
	}
}
