// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// agentsConfigKeys returns the keys of the agent consumers from docs/spec/04-sync-protokoll.md
// "Konfigurationswerte dieses Kapitels" (M2-07): the ack timeout, the save interval of acked_seq
// and the inflight bound per consumer.
func agentsConfigKeys() []config.Key {
	return []config.Key{
		{Name: "agent.ack_timeout", Kind: config.KindDuration, Default: thirtySeconds, Min: minDeadline, Chapter: chapter04,
			Doc: "Without an ack within this time the server delivers again from acked_seq + 1 with redelivered."},
		{Name: "agent.ack_persist_interval", Kind: config.KindDuration, Default: "1s", Min: minDeadline, Chapter: chapter04,
			Doc: "acked_seq of an agent consumer is saved at the latest after this interval."},
		{Name: "agent.max_inflight_events", Kind: config.KindInt, Default: "100", Min: "1", Max: "2147483647", Chapter: chapter04,
			Doc: "Most unacknowledged events on the way per agent consumer."},
	}
}
