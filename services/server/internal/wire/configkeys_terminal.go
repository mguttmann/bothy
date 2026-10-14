// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// terminalConfigKeys returns the keys of the terminal module whose first consumer is M2-39: the
// text limits of hosts and session users (docs/spec/03-datenmodell.md section 9), the validity
// of a pairing code (24 section 17), and the validity of a session token, which sets the window
// a retiring signing key of the purpose terminal stays usable (19 section 10).
func terminalConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "host.name_max_chars", Kind: config.KindInt, Default: "100", Min: "1",
			Doc:     "Longest host name in Unicode characters.",
			Chapter: chapter03,
		},
		{
			Name: "host.os_user_max_chars", Kind: config.KindInt, Default: "64", Min: "1",
			Doc:     "Longest name of a session user in characters.",
			Chapter: chapter03,
		},
		{
			Name: "terminal.pairing_code_ttl", Kind: config.KindDuration, Default: defaultQuarterHour, Min: "1s",
			Doc:     "Validity of the pairing code of a host.",
			Chapter: chapter24,
		},
		{
			Name: "terminal.session_token_ttl", Kind: config.KindDuration, Default: thirtySeconds, Min: "1s",
			Doc:     "Validity of a session token of the terminal.",
			Chapter: chapter24,
		},
	}
}
