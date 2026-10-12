// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// syncConfigKeys returns the keys of the event log (M1-16) and the idempotency keys (M1-20): the
// partition size of 03, the retention, payload limit and key TTL of 04 and the tail, maintenance
// and cleanup keys of 02 "Configuration values of this chapter".
// database.listen_reconnect_backoff_cap and database.lock_timeout, which the tail and the
// maintenance read too, are database keys (configkeys_database.go).
// defaultEventRetention is the default of sync.event_retention (04 "Configuration values of this chapter").
const defaultEventRetention = "30d"

func syncConfigKeys() []config.Key {
	return []config.Key{
		{Name: "sync.event_log_partition_size", Kind: config.KindInt, Default: "1000000", Min: "1", Max: "1000000000000", Chapter: chapter03,
			Doc: "Sequence numbers per partition of event_log."},
		{Name: "sync.event_retention", Kind: config.KindDuration, Default: defaultEventRetention, Min: "1s", Chapter: chapter04,
			Doc: "How long durable events stay available for catch-up before their partition is detached."},
		{Name: "sync.event_payload_max_bytes", Kind: config.KindInt, Default: "65536", Min: "3", Max: "1048576", Chapter: chapter04,
			Doc: "The payload of an event must be smaller than this many bytes; larger objects get a reduced payload."},
		{Name: "sync.log_poll_interval", Kind: config.KindDuration, Default: "1s", Min: minDeadline, Chapter: chapter02,
			Doc: "How often the log tail reads event_log without a notification."},
		{Name: "sync.tail_batch_size", Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: chapter02,
			Doc: "Events per read of the log tail."},
		{Name: "sync.partition_maintenance_interval", Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: chapter02,
			Doc: "Interval of the job sync.partition_maintenance."},
		{Name: "sync.idempotency_key_ttl", Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: chapter04,
			Doc: "How long an Idempotency-Key and its stored response stay valid per principal."},
		{Name: "sync.idempotency_cleanup_interval", Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: chapter02,
			Doc: "Interval of the job sync.idempotency_cleanup."},
		{Name: "sync.idempotency_cleanup_batch_size", Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: chapter02,
			Doc: "Rows per delete batch of the job sync.idempotency_cleanup."},
		{Name: "sync.event_log_partitions_ahead", Kind: config.KindInt, Default: "3", Min: "1", Max: "1000", Chapter: chapter02,
			Doc: "Partitions of event_log that stand ready above the head."},
	}
}
