// SPDX-License-Identifier: Apache-2.0

package app

import "time"

// Config is the configuration of the audit module, bound in internal/wire from the keys of 05
// "Konfigurationswerte", 19 "Konfigurationswerte", 20 "Konfigurationswerte" and the page sizes of
// 06 (docs/operations/configuration.md).
type Config struct {
	// DenyAggregateWindow is audit.deny_aggregate_window: identical denials within this time from
	// the first one become one row.
	DenyAggregateWindow time.Duration `config:"audit.deny_aggregate_window"`
	// DenyAggregateMaxKeys is audit.deny_aggregate_max_keys: at most this many open aggregations
	// per process; a new key beyond it writes the oldest at once.
	DenyAggregateMaxKeys int `config:"audit.deny_aggregate_max_keys"`
	// IPKeyPeriod is privacy.audit_ip_key_period: one IP pseudonym key per period.
	IPKeyPeriod time.Duration `config:"privacy.audit_ip_key_period"`
	// IPKeyRetention is privacy.audit_ip_key_retention: a key is destroyed this long after its
	// period (the destruction job comes with M7-05; the configuration check holds it at least
	// IPKeyPeriod).
	IPKeyRetention time.Duration `config:"privacy.audit_ip_key_retention"`
	// IPKEKRef is privacy.audit_ip_kek_ref: the secret reference of the key that encrypts the IP
	// keys in audit_ip_keys.
	IPKEKRef string `config:"privacy.audit_ip_kek_ref"`
	// CheckpointInterval is audit.checkpoint_interval: the period of the job audit.chain_verify,
	// which signs the next checkpoint (19 SEC-AUD-2).
	CheckpointInterval time.Duration `config:"audit.checkpoint_interval"`
	// CheckpointSink is audit.checkpoint_sink: empty (no sink, a permanent warning),
	// file://<absolute path> or s3://<bucket>[/<prefix>] (19 section 16, ParseSink).
	CheckpointSink string `config:"audit.checkpoint_sink"`
	// PageSizeDefault is api.page_size_default: the page of listAuditEvents without limit.
	PageSizeDefault int `config:"api.page_size_default"`
	// PageSizeMax is api.page_size_max: the cap of limit and the batch of the export and of the
	// chain walk.
	PageSizeMax int `config:"api.page_size_max"`
}
