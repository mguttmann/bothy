// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// agentsConfigKeys returns the keys of the agent consumers from docs/spec/04-sync-protocol.md
// "Configuration values of this chapter" (M2-07): the ack timeout, the save interval of acked_seq
// and the inflight bound per consumer; the keys of the presence of agents (M2-20); and the grace of
// a cancel request (M2-09).
func agentsConfigKeys() []config.Key {
	return []config.Key{
		{Name: "agent.ack_timeout", Kind: config.KindDuration, Default: thirtySeconds, Min: minDeadline, Chapter: chapter04,
			Doc: "Without an ack within this time the server delivers again from acked_seq + 1 with redelivered."},
		{Name: "agent.ack_persist_interval", Kind: config.KindDuration, Default: "1s", Min: minDeadline, Chapter: chapter04,
			Doc: "acked_seq of an agent consumer is saved at the latest after this interval."},
		{Name: "agent.max_inflight_events", Kind: config.KindInt, Default: "100", Min: "1", Max: "2147483647", Chapter: chapter04,
			Doc: "Most unacknowledged events on the way per agent consumer."},
		// The presence of agents (M2-20): 04, 03 and 10 "Configuration values of this chapter".
		{Name: keyPresenceTimeout, Kind: config.KindDuration, Default: "90s", Min: minDeadline, Chapter: chapter04,
			Doc: "Without a heartbeat for this long an agent is offline."},
		{Name: "agent.heartbeat_persist_interval", Kind: config.KindDuration, Default: denyAggregateWindowDefault, Min: minDeadline, Chapter: chapter03,
			Doc: "Heartbeat fields are stored at most this often."},
		{Name: "billing.mismatch_alert_after", Kind: config.KindDuration, Default: "5m", Min: minDeadline, Chapter: chapter10,
			Doc: "How long the reported mode may differ from the configured one before the warning and the alarm."},
		// M2-09: a cancel request the adapter does not confirm in time ends the run (09 8).
		{Name: "agent.cancel_grace", Kind: config.KindDuration, Default: thirtySeconds, Min: minDeadline, Chapter: chapter04,
			Doc: "Time for the adapter to report a cancellation; then the server cancels the run with cancel_timeout."},
	}
}

// keyPresenceTimeout is agent.presence_timeout, which the cross-check
// presence_timeout_above_heartbeat_interval compares with sync.heartbeat_interval.
const keyPresenceTimeout = "agent.presence_timeout"
