// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// auditConfigKeys returns the keys of the audit module (M1-24): the aggregation of identical
// denials from docs/spec/05-actions.md and the IP pseudonym keys from 20-privacy.md
// "Configuration values of this chapter". The cross-check audit_ip_key_retention in configchecks.go
// holds the retention at least one period.
// denyAggregateWindowDefault is the default of audit.deny_aggregate_window (05 table); a named
// constant because the same text is the default of other keys (goconst).
const denyAggregateWindowDefault = "60s"

func auditConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "audit.deny_aggregate_window", Kind: config.KindDuration, Default: denyAggregateWindowDefault, Min: "1ms",
			Doc:     "Window from the first of identical denials within which they become one audit entry with a count.",
			Chapter: chapter05,
		},
		{
			Name: "audit.deny_aggregate_max_keys", Kind: config.KindInt, Default: "10000", Min: "1",
			Doc:     "Most open aggregations of denials per process; a new key beyond it writes the oldest at once.",
			Chapter: chapter05,
		},
		{
			// At least 1s: periods are aligned in Unix time, and a shorter one would create a key
			// row per request.
			Name: "privacy.audit_ip_key_period", Kind: config.KindDuration, Default: "30d", Min: "1s",
			Doc:     "Length of the period of one key of the IP pseudonyms in the audit log.",
			Chapter: chapter20,
		},
		{
			Name: "privacy.audit_ip_key_retention", Kind: config.KindDuration, Default: "90d", Min: "1s",
			Doc:     "How long after its period a key of the IP pseudonyms is kept before it is destroyed.",
			Chapter: chapter20,
		},
		{
			Name: "privacy.audit_ip_kek_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/audit_ip_kek",
			Doc:     "Secret reference of the key that encrypts the keys of the IP pseudonyms in the database; 32 bytes as hex.",
			Chapter: chapter20,
		},
	}
}
