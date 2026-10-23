// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// The values of cluster.mode (docs/spec/02-architecture.md section 22).
const (
	keyClusterMode     = "cluster.mode"
	clusterModeSingle  = "single"
	clusterModeMulti   = "multi"
	defaultClusterMode = clusterModeSingle
	// The keys of the shared presence state gateway_presence (M7-17, R-DM-51).
	keyPresenceFlushInterval = "cluster.presence_flush_interval"
	keyPresenceTTL           = "cluster.presence_ttl"
)

// clusterConfigKeys returns the keys of the operation of several nodes: the cluster mode (the
// terminal across several api nodes since M2-47, the gateway since M7-17: the channel ephemeral,
// the fencing of agent consumers and the device replacement across nodes) and the shared presence
// state gateway_presence (M7-17), which processes in role api (and in role all with multi) write
// and the worker reads.
func clusterConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: keyClusterMode, Kind: config.KindEnum, Default: defaultClusterMode, Enum: []string{clusterModeSingle, clusterModeMulti},
			Doc:     "single or multi (several api nodes behind a proxy that routes both terminal stream paths of a session to one node).",
			Chapter: chapter02,
		},
		{
			Name: keyPresenceFlushInterval, Kind: config.KindDuration, Default: "2s", Min: "100ms",
			Doc:     "Interval at which an api node writes the presence of its live connections into gateway_presence.",
			Chapter: chapter02,
		},
		{
			Name: keyPresenceTTL, Kind: config.KindDuration, Default: defaultTenSeconds, Min: "1s",
			Doc:     "Age after which a row of gateway_presence that its node stopped refreshing is ignored and deleted; more than twice cluster.presence_flush_interval.",
			Chapter: chapter02,
		},
	}
}

// presenceTTLAboveFlush requires cluster.presence_ttl to be more than twice
// cluster.presence_flush_interval, so one late flush never lets the rows of a live node expire.
func presenceTTLAboveFlush(v *config.Values) error {
	if v.Duration(keyPresenceTTL) <= 2*v.Duration(keyPresenceFlushInterval) {
		return errors.New("cluster.presence_ttl must be more than twice cluster.presence_flush_interval")
	}
	return nil
}
