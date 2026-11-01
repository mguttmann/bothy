// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// billingConfigKeys returns the keys of the billing module from
// docs/spec/10-backend-policy.md section 17 (M2-22): the switch that shows the amounts of runs
// in mode personal as information (10 section 12.2), on by default since FIX-cost-display.
func billingConfigKeys() []config.Key {
	return []config.Key{
		{Name: "billing.show_personal_estimates", Kind: config.KindBool, Default: defaultTrue, Chapter: chapter10,
			Doc: "Show amounts in mode personal too, as information."},
	}
}
