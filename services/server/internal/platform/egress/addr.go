// SPDX-License-Identifier: Apache-2.0

package egress

import "net/netip"

// alwaysBlocked are the ranges no rule opens, not even with allow_private (19 SEC-NET-3,
// R-SE-10): unspecified, link-local with the metadata address of the clouds, multicast,
// broadcast and reserved, IETF protocol assignments, benchmarking, deprecated site-local,
// discard-only, IPv4-compatible, NAT64, 6to4 and Teredo. The last five carry IPv4 addresses
// inside IPv6 and are known ways around an IPv4 check.
var alwaysBlocked = prefixes(
	"0.0.0.0/8",
	"169.254.0.0/16",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"192.0.0.0/24",
	"198.18.0.0/15",
	"::/128",
	"fe80::/10",
	"ff00::/8",
	"fec0::/10",
	"100::/64",
	"::/96",
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"2002::/16",
	"2001::/32",
)

// privateRanges are reachable only through a rule with allow_private: loopback, RFC 1918,
// shared address space (RFC 6598, also used by overlay networks) and unique local addresses
// (RFC 4193). ::1 is handled before alwaysBlocked, because it lies inside ::/96.
var privateRanges = prefixes(
	"127.0.0.0/8",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"100.64.0.0/10",
	"fc00::/7",
)

// ipv6Loopback is ::1.
var ipv6Loopback = netip.IPv6Loopback()

func prefixes(texts ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(texts))
	for _, t := range texts {
		out = append(out, netip.MustParsePrefix(t))
	}
	return out
}

// canonicalAddr unmaps an IPv4-mapped IPv6 address and drops the zone, the form every check
// works on: netip.Prefix.Contains matches neither a mapped address against an IPv4 prefix nor
// an address with a zone.
func canonicalAddr(a netip.Addr) netip.Addr { return a.Unmap().WithZone("") }

// addrAllowed reports whether a connection to a may be opened. Documentation ranges (RFC 5737,
// RFC 3849) and every other address outside the two lists count as public.
func addrAllowed(a netip.Addr, allowPrivate bool) bool {
	a = canonicalAddr(a)
	if !a.IsValid() {
		return false
	}
	if a == ipv6Loopback {
		return allowPrivate
	}
	for _, p := range alwaysBlocked {
		if p.Contains(a) {
			return false
		}
	}
	for _, p := range privateRanges {
		if p.Contains(a) {
			return allowPrivate
		}
	}
	return true
}

// loopbackAddr reports whether a is a loopback address (127.0.0.0/8 or ::1), also when written
// as an IPv4-mapped IPv6 address.
func loopbackAddr(a netip.Addr) bool { return canonicalAddr(a).IsLoopback() }
