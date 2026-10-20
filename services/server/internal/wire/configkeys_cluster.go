// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// The values of cluster.mode (docs/spec/02-architektur.md section 22).
const (
	keyClusterMode     = "cluster.mode"
	clusterModeSingle  = "single"
	clusterModeMulti   = "multi"
	defaultClusterMode = clusterModeSingle
)

// clusterConfigKeys returns the key of the cluster mode, whose first consumer is the terminal
// across several api nodes (M2-47): with multi the terminal sends its signals over NOTIFY
// ephemeral and every api process holds a LISTEN ephemeral connection; with single nothing of it
// runs. M7-17 adds the rest of the operation of several nodes on the same key.
func clusterConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: keyClusterMode, Kind: config.KindEnum, Default: defaultClusterMode, Enum: []string{clusterModeSingle, clusterModeMulti},
			Doc:     "single or multi (several api nodes behind a proxy that routes both terminal stream paths of a session to one node).",
			Chapter: chapter02,
		},
	}
}
