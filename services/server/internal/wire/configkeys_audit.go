// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// denyAggregateWindowDefault is the default of audit.deny_aggregate_window (05 table); a named
// constant because the same text is the default of other keys (goconst).
const denyAggregateWindowDefault = "60s"

// keyCheckpointSink is audit.checkpoint_sink; the cross-check audit_checkpoint_sink reads it.
const keyCheckpointSink = "audit.checkpoint_sink"

// auditConfigKeys returns the keys of the audit module (M1-24): the aggregation of identical
// denials from docs/spec/05-actions.md and the IP pseudonym keys from 20-privacy.md
// "Configuration values of this chapter", and the checkpoints from 19-security.md "16." (M1-50).
// The cross-check audit_ip_key_retention in configchecks.go holds the retention at least one
// period, audit_checkpoint_sink the value of the sink.
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
			Name: "audit.checkpoint_interval", Kind: config.KindDuration, Default: "1h", Min: "1s",
			Doc:     "Interval of the job audit.chain_verify, which recomputes the audit chain and signs the next checkpoint.",
			Chapter: chapter19,
		},
		{
			// A string, not a URL key: the schema allows an empty default only for string and list
			// keys; the cross-check audit_checkpoint_sink parses it.
			Name: keyCheckpointSink, Kind: config.KindString, Default: "",
			Doc:     "Where the signed audit checkpoints are stored outside the database: empty (a permanent warning) or file://<absolute path> on storage outside the database volume.",
			Chapter: chapter19,
		},
		{
			Name: "privacy.audit_ip_kek_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/audit_ip_kek",
			Doc:     "Secret reference of the key that encrypts the keys of the IP pseudonyms in the database; 32 bytes as hex.",
			Chapter: chapter20,
		},
	}
}
