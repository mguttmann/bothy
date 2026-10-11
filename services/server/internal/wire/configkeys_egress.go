// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// chapter19 is the chapter that owns the keys egress.*.
const chapter19 = "19-sicherheit.md"

// egressConfigKeys returns the keys of the egress client from docs/spec/19-sicherheit.md
// "Konfigurationswerte dieses Kapitels". The cross-check egress_allowed_hosts parses every
// entry of egress.allowed_hosts with egress.ParseRule, and EgressConfig builds egress.Config.
func egressConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "egress.allowed_hosts", Kind: config.KindStringList, Default: "",
			Doc:     "Additional allowed targets of outgoing connections, each host[:port][;allow_private]; allow_private also opens private addresses and plain http.",
			Chapter: chapter19,
		},
		{
			Name: "egress.timeout", Kind: config.KindDuration, Default: "15s", Min: "1ms",
			Doc:     "Time limit of one outgoing request, from name resolution to the end of the body.",
			Chapter: chapter19,
		},
		{
			Name: "egress.max_response_bytes", Kind: config.KindInt, Default: "1048576", Min: "1",
			Doc:     "Largest response body and header of one outgoing request in bytes; a larger body is an error.",
			Chapter: chapter19,
		},
	}
}
