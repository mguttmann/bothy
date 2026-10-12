// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// configCheck is a cross-check of the configuration schema.
type configCheck struct {
	name string
	keys []string
	fn   func(*config.Values) error
}

// configChecks are the cross-checks of docs/spec/02-architektur.md "Konfigurationsladen" item 4,
// and the check of a key whose entries a platform package parses (egress_allowed_hosts, in
// egress.go). Their errors become problem messages verbatim, so they name keys and never values.
var configChecks = []configCheck{
	{name: "listeners_distinct", keys: []string{"server.listen_addr", "server.internal_listen_addr"}, fn: listenersDistinct},
	{name: "json_limit_above_message", keys: []string{"http.max_json_body_bytes", "message.max_content_bytes"}, fn: jsonLimitAboveMessage},
	{name: "egress_allowed_hosts", keys: []string{"egress.allowed_hosts"}, fn: egressAllowedHosts},
	{name: "public_url_origin", keys: []string{"server.public_url"}, fn: publicURLOrigin},
	{name: "cors_origins", keys: []string{"api.cors_allowed_origins"}, fn: corsOrigins},
}

// publicURLOrigin requires server.public_url to be an origin: scheme, host, optional port and
// nothing else (16 "Konfigurationswerte"), because the DPoP htu is this origin plus the path.
func publicURLOrigin(v *config.Values) error {
	if err := httpserver.CheckOrigin(v.String("server.public_url")); err != nil {
		return errors.New("server.public_url must be an origin (scheme://host[:port], no path, query or fragment): " + err.Error())
	}
	return nil
}

// corsOrigins requires every entry of api.cors_allowed_origins to be an origin; the chain
// compares the Origin header with the entries exactly.
func corsOrigins(v *config.Values) error {
	for i, o := range v.Strings("api.cors_allowed_origins") {
		if err := httpserver.CheckOrigin(o); err != nil {
			return fmt.Errorf("api.cors_allowed_origins entry %d must be an origin (scheme://host[:port]): %w", i+1, err)
		}
	}
	return nil
}

// listenersDistinct rejects a public and an internal listener that would bind the same socket:
// equal ports other than 0 on equal hosts, or when either host is empty, 0.0.0.0 or ::. Port 0
// may repeat because tests bind port 0 (PLAN P-10). IPv4-mapped IPv6 addresses count as their
// IPv4 address, and an unspecified address with an IPv6 zone counts as unspecified.
func listenersDistinct(v *config.Values) error {
	if listenersConflict(v.String("server.listen_addr"), v.String("server.internal_listen_addr")) {
		return errors.New("the public and the internal listener must not share a port on the same or an unspecified address; only port 0 may repeat")
	}
	return nil
}

// listenersConflict compares two canonical listen addresses (config.KindListenAddr).
func listenersConflict(a, b string) bool {
	hostA, portA, errA := net.SplitHostPort(a)
	hostB, portB, errB := net.SplitHostPort(b)
	if errA != nil || errB != nil || portA != portB || portA == "0" {
		return false
	}
	if hostA == "" || hostB == "" {
		return true
	}
	addrA, errA := netip.ParseAddr(hostA)
	addrB, errB := netip.ParseAddr(hostB)
	if errA != nil || errB != nil {
		return false
	}
	addrA, addrB = addrA.Unmap(), addrB.Unmap()
	return unspecified(addrA) || unspecified(addrB) || addrA == addrB
}

// unspecified reports 0.0.0.0 and ::, also with an IPv6 zone: the kernel binds the wildcard for
// "::%eth0" as for "::".
func unspecified(a netip.Addr) bool { return a.WithZone("").IsUnspecified() }

// jsonLimitAboveMessage requires the JSON body limit to be strictly greater than the message
// size: the envelope of a message is small against the gap of the defaults, and a separate value
// for it would be a guess without measurement (02 "Konfigurationsladen" item 4).
func jsonLimitAboveMessage(v *config.Values) error {
	if v.Int("http.max_json_body_bytes") <= v.Int("message.max_content_bytes") {
		return errors.New("http.max_json_body_bytes must be greater than message.max_content_bytes")
	}
	return nil
}
