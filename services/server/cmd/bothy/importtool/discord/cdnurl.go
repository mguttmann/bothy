// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// SafeURL returns scheme, host and path of raw, without user info, query and fragment, in the
// path encoding raw used; "" when raw does not parse. Every error and log line of the package
// that names a URL uses it, because the query of an attachment link carries its signature
// (ex, is, hm; 22 6.5 item 4, REUSE L22).
//
// Example:
//
//	discord.SafeURL("https://cdn.example.com/attachments/1/2/a.png?ex=1&is=2&hm=3")
//	// "https://cdn.example.com/attachments/1/2/a.png"
func SafeURL(raw string) string {
	p, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return (&url.URL{Scheme: p.Scheme, Host: p.Host, Path: p.Path, RawPath: p.RawPath}).String()
}

// safeOp is the op of a CallError of a CDN request: the method and the safe URL.
func safeOp(raw string) string {
	s := SafeURL(raw)
	if s == "" {
		s = "(invalid URL)"
	}
	return "GET " + s
}

// cdnHost is one entry of import.discord.cdn_hosts: a lower-case host name without trailing
// dot and its port, 443 when the entry names none.
type cdnHost struct {
	host string
	port uint16
}

// defaultHTTPSPort is the port of an https URL or allow-list entry without one.
const defaultHTTPSPort = 443

// parseCDNHosts parses the entries of import.discord.cdn_hosts: "host" or "host:port", a DNS
// name without scheme, path or wildcard. Errors name the index of the entry.
func parseCDNHosts(entries []string) ([]cdnHost, error) {
	if len(entries) == 0 {
		return nil, errors.New("discord: CDNConfig.Hosts (import.discord.cdn_hosts) is empty")
	}
	hosts := make([]cdnHost, 0, len(entries))
	for i, e := range entries {
		h, ok := parseCDNHost(e)
		if !ok {
			return nil, fmt.Errorf("discord: CDNConfig.Hosts[%d] is not a host name with an optional port", i)
		}
		hosts = append(hosts, h)
	}
	return hosts, nil
}

// parseCDNHost parses one allow-list entry.
func parseCDNHost(entry string) (cdnHost, bool) {
	name, port := entry, uint16(defaultHTTPSPort)
	if strings.Contains(entry, ":") {
		h, p, err := net.SplitHostPort(entry)
		if err != nil {
			return cdnHost{}, false
		}
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil || n == 0 {
			return cdnHost{}, false
		}
		name, port = h, uint16(n)
	}
	name = canonicalHost(name)
	if !validHostName(name) {
		return cdnHost{}, false
	}
	return cdnHost{host: name, port: port}, true
}

// canonicalHost lowers a host name and drops one trailing dot.
func canonicalHost(name string) string { return strings.TrimSuffix(strings.ToLower(name), ".") }

// validHostName reports whether name is a DNS name of letters, digits, "-" and "." with a
// letter or digit at both ends of every label; IP literals and wildcards are not host names.
func validHostName(name string) bool {
	if name == "" || net.ParseIP(name) != nil {
		return false
	}
	for label := range strings.SplitSeq(name, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

// checkCDNURL is the URL check before any connection (22 6.5 item 2, Design note 1 of M8-02):
// https only (ErrCDNScheme), no user info, and a host and port that match an entry of hosts
// exactly (ErrCDNHostNotAllowed). It returns the matching entry.
func checkCDNURL(raw string, hosts []cdnHost) (cdnHost, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" {
		return cdnHost{}, ErrCDNHostNotAllowed
	}
	if u.Scheme != schemeHTTPS {
		return cdnHost{}, ErrCDNScheme
	}
	if u.User != nil || u.Hostname() == "" {
		return cdnHost{}, ErrCDNHostNotAllowed
	}
	port := uint64(defaultHTTPSPort)
	if p := u.Port(); p != "" {
		port, err = strconv.ParseUint(p, 10, 16)
		if err != nil {
			return cdnHost{}, ErrCDNHostNotAllowed
		}
	}
	name := canonicalHost(u.Hostname())
	for _, h := range hosts {
		if h.host == name && uint64(h.port) == port {
			return h, nil
		}
	}
	return cdnHost{}, ErrCDNHostNotAllowed
}
