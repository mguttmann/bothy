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
// client, and forwarding headers are ignored. A trusted peer's one forwarding header counts:
// X-Forwarded-For or Forwarded (RFC 7239, the for= values). A request with both names no client
// and the peer is the client: a proxy that sets one of them passes the other through as the
// client sent it, so the client would choose its address. The list is walked from
// right to left, skipping addresses inside trusted; the first address outside is the client, and
// when all are trusted the leftmost is. A malformed or obfuscated entry stops the walk, and the
// last trusted hop is the client (fail closed against spoofing). IPv4-mapped IPv6 addresses are
// unmapped and zones dropped. The result never goes into general logs (SEC-LOG-5).
func ClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	peer, ok := parseHop(r.RemoteAddr)
	if !ok || !isTrusted(peer, trusted) {
		return peer
	}
	forwarded, xff := r.Header.Values(headerForwarded), r.Header.Values(headerXForwardedFor)
	if len(forwarded) > 0 && len(xff) > 0 {
		return peer
	}
	// At most one of the two headers is present here.
	hops := forwardedFor(forwarded)
	for _, v := range xff {
		hops = append(hops, strings.Split(v, ",")...)
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
// An element without for= yields "" so the walk stops there. A line that ends inside a quoted
// string yields one "" as well: its elements cannot be told apart, and a proxy may have appended
// its own element to an open quote of the client.
func forwardedFor(lines []string) []string {
	var out []string
	for _, line := range lines {
		elements, closed := splitOutsideQuotes(line, ',')
		if !closed {
			out = append(out, "")
			continue
		}
		for _, element := range elements {
			value := ""
			// Every element of a closed line is closed.
			pairs, _ := splitOutsideQuotes(element, ';')
			for _, pair := range pairs {
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

// splitOutsideQuotes splits s at sep where sep is not inside a quoted string. Inside a quoted
// string a backslash escapes the next byte (RFC 7230 quoted-pair), so an escaped quote does not
// end the string. closed is false when s ends inside a quoted string.
func splitOutsideQuotes(s string, sep byte) (parts []string, closed bool) {
	quoted, start := false, 0
	for i := 0; i < len(s); i++ {
		switch {
		case quoted && s[i] == '\\':
			i++
		case s[i] == '"':
			quoted = !quoted
		case s[i] == sep && !quoted:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:]), !quoted
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
