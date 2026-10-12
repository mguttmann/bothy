// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// Names of the server keys that the schema, the cross-checks and TransportConfig all read.
const (
	keyPublicURL  = "server.public_url"
	keyTLSCertRef = "server.tls_cert_ref"
	keyTLSKeyRef  = "server.tls_key_ref"
)

// serverConfigKeys returns the keys of the process itself from docs/spec/16-operations.md
// "Konfigurationswerte dieses Kapitels": the role, which the flag --role sets, the two
// listeners, which the cross-check listeners_distinct compares, and the transport of the public
// listener (M1-21): the trusted proxies and the optional certificate and key references, which
// the cross-checks tls_pair and tls_https read.
func serverConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "server.role", Kind: config.KindEnum, Default: "all", Enum: []string{"api", "worker", "all"},
			Doc:     "Role of the process: api serves REST and the gateway, worker runs jobs and consumers, all does both.",
			Chapter: chapter16,
		},
		{
			Name: "server.listen_addr", Kind: config.KindListenAddr, Default: "127.0.0.1:8080",
			Doc:     "Address of the public listener that serves REST and the gateway.",
			Chapter: chapter16,
		},
		{
			Name: "server.internal_listen_addr", Kind: config.KindListenAddr, Default: "127.0.0.1:8081",
			Doc:     "Address of the internal listener for health, readiness and incoming webhooks from services in the stack.",
			Chapter: chapter16,
		},
		{
			Name: keyPublicURL, Kind: config.KindURL, Required: true,
			Doc:     "Origin under which apps reach the server; the base of the DPoP htu, token audience and WebAuthn.",
			Chapter: chapter16,
		},
		{
			Name: "server.trusted_proxies", Kind: config.KindCIDRList, Default: "127.0.0.1/32, ::1/128",
			Doc:     "Peers whose forwarding and identity headers count and that may send plaintext.",
			Chapter: chapter16,
		},
		{
			// Empty means no reference: the server terminates no TLS (16 "Secret provider (ADR 0011)",
			// docs/DECISIONS.md M1-21/1).
			Name: keyTLSCertRef, Kind: config.KindSecretRef, Default: "",
			Doc:     "Reference to the PEM certificate chain, leaf first; empty means the server terminates no TLS.",
			Chapter: chapter16,
		},
		{
			Name: keyTLSKeyRef, Kind: config.KindSecretRef, Default: "",
			Doc:     "Reference to the PEM private key of the certificate; only together with server.tls_cert_ref.",
			Chapter: chapter16,
		},
	}
}
