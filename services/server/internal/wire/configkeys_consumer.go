// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// Keys of the worker consumers that the cross-check consumer_alert_below_retention reads.
const (
	keyConsumerPendingAlertAfter = "consumer.pending_alert_after"
	keySyncEventRetention        = "sync.event_retention"
)

// consumerConfigKeys returns the keys of the worker consumers of the event log (M2-01, 02
// "Konfigurationswerte dieses Kapitels"). database.listen_reconnect_backoff_cap and
// database.statement_timeout, which the consumers read too, are database keys
// (configkeys_database.go); sync.event_retention is a sync key (configkeys_sync.go).
// consumer.pending_alert_after is the threshold of the alarm BothyEventConsumerLag: the rule file
// holds the same number, kept equal by the alert-thresholds check (docs/DECISIONS.md D-M2-01-6).
func consumerConfigKeys() []config.Key {
	return []config.Key{
		{Name: "consumer.batch_size", Kind: config.KindInt, Default: "500", Min: "1", Max: "2147483647", Chapter: chapter02,
			Doc: "Events per round of a worker consumer."},
		{Name: "consumer.poll_interval", Kind: config.KindDuration, Default: "1s", Min: minDeadline, Chapter: chapter02,
			Doc: "How often a worker consumer runs a round without a notification."},
		{Name: keyConsumerPendingAlertAfter, Kind: config.KindDuration, Default: defaultQuarterHour, Min: "1s", Chapter: chapter02,
			Doc: "Age of the oldest unprocessed event of a worker consumer that raises the alarm; strictly below sync.event_retention."},
	}
}
