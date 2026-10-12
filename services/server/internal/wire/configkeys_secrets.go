// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// secretsConfigKeys returns the keys of the secret provider from docs/spec/16-operations.md
// "Configuration values of this chapter": the base directory that <secrets.dir> stands for in
// secret references and how long a read secret stays in use (M1-09), and the key store of the
// signing keys, the only directory the server writes (M1-32, ADR 0066).
func secretsConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "secrets.dir", Kind: config.KindString, Default: "/run/bothy/secrets",
			Doc:     "Base directory of the default file: references of secrets.",
			Chapter: chapter16,
		},
		{
			// At least 1s (docs/DECISIONS.md M1-09/24): with 0 every call reads the secret, and
			// while a refresh fails every call logs a line and counts a failure.
			Name: "secrets.cache_ttl", Kind: config.KindDuration, Default: "5m", Min: "1s",
			Doc:     "How long a secret that was read stays in use before it is read again.",
			Chapter: chapter16,
		},
		{
			Name: "secrets.signing_key_dir", Kind: config.KindString, Default: "/var/lib/bothy/signing-keys",
			Doc:     "Writable directory of the private signing keys; owned by the service user with mode 0700.",
			Chapter: chapter16,
		},
	}
}
