// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// SameSite reports whether the URLs a and b are on the same site in the sense of browsers
// (schemeful same-site): the same scheme and the same registrable domain of the public suffix
// list (ICANN and private sections). A host without a registrable domain (an IP address,
// localhost or a public suffix itself) is its own site; there is never a fallback to the last two
// labels, which would treat a.example.co.uk and b.other.co.uk as one site. A URL without a host
// is on no site, so the answer is false. Ports do not count. bothy config check uses it for the
// warning of console.grafana_embed_url (docs/spec/25-visualization.md 9.2 item 4, brief M6-17
// Design note 2).
//
// Example:
//
//	httpserver.SameSite("https://chat.example.com", "https://grafana.example.com/d/ops") // true
func SameSite(a, b string) bool {
	ua, err := url.Parse(a)
	if err != nil || ua.Hostname() == "" {
		return false
	}
	ub, err := url.Parse(b)
	if err != nil || ub.Hostname() == "" {
		return false
	}
	return strings.EqualFold(ua.Scheme, ub.Scheme) && registrableSite(ua.Hostname()) == registrableSite(ub.Hostname())
}

// registrableSite returns the registrable domain of host (eTLD+1 of the public suffix list) in
// lower case, or the host itself when it has none: an IP address, localhost or a public suffix.
func registrableSite(host string) string {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if net.ParseIP(host) != nil {
		return host
	}
	site, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return host
	}
	return site
}
