// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"net"
	"net/netip"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// configCheck is a cross-check of the configuration schema.
type configCheck struct {
	name string
	keys []string
	fn   func(*config.Values) error
}

// configChecks are the cross-checks of docs/spec/02-architecture.md "Configuration loading" item 4.
// Their errors become problem messages verbatim, so they name keys and never values.
var configChecks = []configCheck{
	{name: "listeners_distinct", keys: []string{"server.listen_addr", "server.internal_listen_addr"}, fn: listenersDistinct},
	{name: "json_limit_above_message", keys: []string{"http.max_json_body_bytes", "message.max_content_bytes"}, fn: jsonLimitAboveMessage},
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
// for it would be a guess without measurement (02 "Configuration loading" item 4).
func jsonLimitAboveMessage(v *config.Values) error {
	if v.Int("http.max_json_body_bytes") <= v.Int("message.max_content_bytes") {
		return errors.New("http.max_json_body_bytes must be greater than message.max_content_bytes")
	}
	return nil
}
