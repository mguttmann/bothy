// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// ClientIP returns the client IP of r (06 "Basic rules" row Client IP, 08 "Login
// protection", ID-T25). It starts from the peer of the connection. A peer outside trusted is the
// client, and forwarding headers are ignored. A trusted peer's headers count: Forwarded (RFC
// 7239, the for= values) when present, else X-Forwarded-For, never both. The list is walked from
// right to left, skipping addresses inside trusted; the first address outside is the client, and
// when all are trusted the leftmost is. A malformed or obfuscated entry stops the walk, and the
// last trusted hop is the client (fail closed against spoofing). IPv4-mapped IPv6 addresses are
// unmapped and zones dropped. The result never goes into general logs (SEC-LOG-5).
func ClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	peer, ok := parseHop(r.RemoteAddr)
	if !ok || !isTrusted(peer, trusted) {
		return peer
	}
	var hops []string
	if values := r.Header.Values(headerForwarded); len(values) > 0 {
		hops = forwardedFor(values)
	} else {
		for _, v := range r.Header.Values(headerXForwardedFor) {
			hops = append(hops, strings.Split(v, ",")...)
		}
	}
	client := peer
	for _, hop := range slices.Backward(hops) {
		addr, ok := parseHop(hop)
		if !ok {
			return client
		}
		client = addr
		if !isTrusted(addr, trusted) {
			return addr
		}
	}
	return client
}

// isTrusted reports whether addr lies in one of the prefixes.
func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	return slices.ContainsFunc(trusted, func(p netip.Prefix) bool { return p.Contains(addr) })
}

// forwardedFor returns the for= value of every element of the Forwarded header lines, in order.
// An element without for= yields "" so the walk stops there.
func forwardedFor(lines []string) []string {
	var out []string
	for _, line := range lines {
		for _, element := range splitOutsideQuotes(line, ',') {
			value := ""
			for _, pair := range splitOutsideQuotes(element, ';') {
				name, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
				if ok && strings.EqualFold(strings.TrimSpace(name), "for") {
					value = strings.Trim(strings.TrimSpace(v), `"`)
				}
			}
			out = append(out, value)
		}
	}
	return out
}

// splitOutsideQuotes splits s at sep where sep is not inside a quoted string.
func splitOutsideQuotes(s string, sep byte) []string {
	var parts []string
	quoted, start := false, 0
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '"':
			quoted = !quoted
		case s[i] == sep && !quoted:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// parseHop parses one address of a peer or forwarding header: an IP, an IP with port, or a
// bracketed IPv6 with or without port. Obfuscated identifiers ("unknown", "_hidden") fail.
func parseHop(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Addr{}, false
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return normalize(ap.Addr()), true
	}
	s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false
	}
	return normalize(a), true
}

// normalize unmaps IPv4-mapped IPv6 addresses and drops zones.
func normalize(a netip.Addr) netip.Addr {
	return a.Unmap().WithZone("")
}
