// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// Rule is one entry of egress.allowed_hosts: a target the operator allows in addition to the
// targets of a consumer (docs/spec/19-security.md SEC-NET-2).
//
// ParseRule builds a Rule from the configuration text; New accepts only rules in the form
// ParseRule returns.
type Rule struct {
	// Host is a lower-case DNS name without a trailing dot, or an IP literal in the form of
	// netip.Addr.String (IPv4-mapped IPv6 addresses unmapped, no zone). It has no wildcard,
	// scheme, path or user info.
	Host string
	// Port is the only port the rule allows; 0 means the default port of the scheme (443 for
	// https, 80 for http).
	Port uint16
	// AllowPrivate lets the rule reach loopback, RFC 1918, shared (RFC 6598) and unique local
	// (RFC 4193) addresses and allows plain http. It never opens the addresses that are always
	// blocked (link-local and metadata, multicast, unspecified and the other ranges of
	// SEC-NET-3).
	AllowPrivate bool
}

// flagAllowPrivate is the only flag of an entry.
const flagAllowPrivate = "allow_private"

// maxNameLength is the longest DNS name in text form (RFC 1035 section 2.3.4: 255 octets on
// the wire, 253 characters without the trailing dot).
const maxNameLength = 253

// maxLabelLength is the longest label of a DNS name (RFC 1035 section 2.3.4).
const maxLabelLength = 63

// errPortText is the reason of an invalid port.
const errPortText = "the port must be a decimal number from 1 to 65535 without leading zeros"

// errRule returns the error of an invalid entry. The text is fixed: a value can be a secret
// that landed in the wrong key, so no message repeats the input (02 "Configuration loading"
// item 4).
func errRule(reason string) error { return errors.New("egress: invalid rule: " + reason) }

// ParseRule parses one entry of egress.allowed_hosts in the form
// "host[:port][;allow_private]" (docs/operations/configuration.md section 2, row
// "Egress target").
//
// The host is a DNS name or an IP literal. A DNS name has labels of letters, digits, "-" and
// "_" (a label neither starts nor ends with "-"), is normalised to lower case and has no
// trailing dot; a name whose last label is a number ("1.2.3", "0x7f000001") is refused because
// resolvers read it as an IPv4 address. An IPv6 literal stands in brackets when a port follows;
// zones are refused, and an IPv4-mapped IPv6 literal becomes its IPv4 address. The port is a
// decimal number from 1 to 65535 without leading zeros; without a port the rule allows only the
// default port of the scheme. The only flag is allow_private.
//
// The error says what is wrong without quoting s.
//
// Example:
//
//	r, err := egress.ParseRule("[2001:db8::10]:8443;allow_private")
//	// r == egress.Rule{Host: "2001:db8::10", Port: 8443, AllowPrivate: true}
func ParseRule(s string) (Rule, error) {
	hostPort, flags, hasFlags := strings.Cut(s, ";")
	var r Rule
	if hasFlags {
		if flags != flagAllowPrivate {
			return Rule{}, errRule("the only flag is allow_private, given once after one \";\"")
		}
		r.AllowPrivate = true
	}
	host, port, err := splitHostPort(hostPort)
	if err != nil {
		return Rule{}, err
	}
	r.Host, r.Port = host, port
	return r, nil
}

// splitHostPort splits "host", "host:port", "[ipv6]", "[ipv6]:port" and a bare IPv6 literal,
// and returns the canonical host and the port (0 when none is given).
func splitHostPort(s string) (string, uint16, error) {
	if s == "" {
		return "", 0, errRule("the host is empty")
	}
	if rest, ok := strings.CutPrefix(s, "["); ok {
		return splitBracketed(rest)
	}
	switch strings.Count(s, ":") {
	case 0:
		host, err := canonicalHost(s)
		return host, 0, err
	case 1:
		name, text, _ := strings.Cut(s, ":")
		port, ok := parsePort(text)
		if !ok {
			return "", 0, errRule(errPortText)
		}
		host, err := canonicalHost(name)
		return host, port, err
	}
	// More than one colon: an IPv6 literal without port; a port needs brackets.
	addr, err := netip.ParseAddr(s)
	if err != nil || addr.Zone() != "" {
		return "", 0, errRule("an IPv6 literal must be a valid address without a zone, in brackets when a port follows")
	}
	return addr.Unmap().String(), 0, nil
}

// splitBracketed splits "ipv6]" or "ipv6]:port", the text after the opening bracket.
func splitBracketed(rest string) (string, uint16, error) {
	literal, after, closed := strings.Cut(rest, "]")
	if !closed {
		return "", 0, errRule("an IPv6 literal in brackets lacks the closing bracket")
	}
	addr, err := netip.ParseAddr(literal)
	if err != nil || !addr.Is6() || addr.Zone() != "" {
		return "", 0, errRule("brackets must hold an IPv6 address without a zone")
	}
	if after == "" {
		return addr.Unmap().String(), 0, nil
	}
	text, ok := strings.CutPrefix(after, ":")
	if !ok {
		return "", 0, errRule("only \":port\" may follow an IPv6 literal in brackets")
	}
	port, ok := parsePort(text)
	if !ok {
		return "", 0, errRule(errPortText)
	}
	return addr.Unmap().String(), port, nil
}

// parsePort accepts a decimal port from 1 to 65535 without sign or leading zeros.
func parsePort(text string) (uint16, bool) {
	if text == "" || text[0] < '1' || text[0] > '9' || len(text) > 5 {
		return 0, false
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(text, 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(n), true
}

// canonicalHost returns the canonical form of a host without brackets and port: an IP literal
// in netip form (unmapped, no zone), or a DNS name in lower case. It refuses everything else
// with an error that does not quote the host.
func canonicalHost(host string) (string, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.Zone() != "" {
			return "", errRule("an IP address must not carry a zone")
		}
		return addr.Unmap().String(), nil
	}
	name := strings.ToLower(host)
	if !validName(name) {
		return "", errRule("the host must be a DNS name of letters, digits, \"-\" and \"_\" without trailing dot, or an IP address")
	}
	return name, nil
}

// validName reports whether name is a lower-case DNS name in the grammar of ParseRule.
func validName(name string) bool {
	if name == "" || len(name) > maxNameLength {
		return false
	}
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if !validLabel(label) {
			return false
		}
	}
	return !numericLabel(labels[len(labels)-1])
}

// validLabel reports whether label has 1 to 63 characters of a-z, 0-9, "-" and "_" and neither
// starts nor ends with "-".
func validLabel(label string) bool {
	if label == "" || len(label) > maxLabelLength || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, c := range []byte(label) {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// numericLabel reports whether label is a number in the sense of the WHATWG URL standard
// ("ends in a number"): decimal digits, or "0x" followed by hexadecimal digits. Resolvers that
// follow inet_aton read such a name as an IPv4 address ("127.1", "0x7f000001").
func numericLabel(label string) bool {
	if hex, ok := strings.CutPrefix(label, "0x"); ok {
		label = hex
		for _, c := range []byte(label) {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
		return true
	}
	for _, c := range []byte(label) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validateRule reports whether r is in the form ParseRule returns; New uses it for rules that a
// caller built without ParseRule.
func validateRule(r Rule) error {
	host, err := canonicalHost(r.Host)
	if err != nil {
		return err
	}
	if host != r.Host {
		return errRule("the host is not in canonical form (lower case, IP literal as netip prints it)")
	}
	return nil
}

// String returns the entry in its canonical form, which ParseRule reads back to the same Rule:
// "host", "host:port", "[ipv6]:port", each optionally followed by ";allow_private".
func (r Rule) String() string {
	s := r.Host
	if r.Port != 0 {
		s = net.JoinHostPort(r.Host, strconv.Itoa(int(r.Port)))
	}
	if r.AllowPrivate {
		s += ";" + flagAllowPrivate
	}
	return s
}
