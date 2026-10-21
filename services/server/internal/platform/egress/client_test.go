// SPDX-License-Identifier: Apache-2.0

package egress_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func hostIs(name string) egress.HostPolicy {
	return func(_ context.Context, _, host string, _ uint16) bool { return host == name }
}

func TestHostNotAllowedRefusedBeforeDial(t *testing.T) {
	t.Parallel()
	var asked []string
	h := newHarness(t, setup{
		rules: []egress.Rule{{Host: "api.example.com", Port: 8443}},
		policy: func(_ context.Context, scheme, host string, port uint16) bool {
			asked = append(asked, scheme+" "+host+" "+strconv.Itoa(int(port)))
			return false
		},
		answers: map[string][]netip.Addr{"api.example.com": addrs("192.0.2.10")},
	})
	for _, target := range []string{
		"https://api.example.com/",        // rule allows only port 8443
		"https://API.example.com.:9443/x", // other port, upper case and trailing dot
		"https://other.example.com:8443/", // other host on the rule's port
		"https://192.0.2.10:8443/",        // the address of the name is no rule
		"https://[2001:db8::10]/",         // no rule
	} {
		err := h.err(t, target)
		if err == nil {
			t.Fatalf("%s: no error", target)
		}
		if !errors.Is(err, egress.ErrBlocked) {
			t.Fatalf("%s: %v is not ErrBlocked", target, err)
		}
	}
	if n := h.res.calls.Load(); n != 0 {
		t.Errorf("resolver called %d times, want 0", n)
	}
	if n := h.dials.Load(); n != 0 {
		t.Errorf("%d dials, want 0", n)
	}
	want := []string{"https api.example.com 443", "https api.example.com 9443", "https other.example.com 8443", "https 192.0.2.10 8443", "https 2001:db8::10 443"}
	if strings.Join(asked, ", ") != strings.Join(want, ", ") {
		t.Errorf("policy asked %v, want %v", asked, want)
	}
}

func TestHostNotAllowedReason(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{})
	err := h.err(t, "https://api.example.com/")
	wantBlocked(t, err, egress.ReasonHostNotAllowed)
	var be *egress.BlockedError
	if errors.As(err, &be) && be.Host != "api.example.com" {
		t.Errorf("Host %q, want api.example.com", be.Host)
	}
}

// The https targets of the SECT09 tests are TLS servers whose certificate the client trusts, so
// that a client that skipped the address check would reach the handler and the count would show
// it.

func TestSECT09_LiteralPrivateAddressRefused(t *testing.T) {
	t.Parallel()
	srv := newTLSServer(t, ok)
	h := newHarness(t, setup{rules: []egress.Rule{{Host: "127.0.0.1", Port: srv.port(t)}}, roots: true, tlsServer: srv})
	err := h.err(t, "https://127.0.0.1:"+strconv.Itoa(int(srv.port(t)))+"/")
	wantBlocked(t, err, egress.ReasonAddressBlocked)
	if n := srv.count.Load(); n != 0 {
		t.Errorf("server saw %d requests, want 0", n)
	}
	if n := h.dials.Load(); n != 0 {
		t.Errorf("%d dials, want 0", n)
	}
}

func TestSECT09_DNSNameWithPrivateAnswerRefused(t *testing.T) {
	t.Parallel()
	srv := newTLSServer(t, ok)
	h := newHarness(t, setup{
		policy:    hostIs("api.example.com"),
		answers:   map[string][]netip.Addr{"api.example.com": {loopback4}},
		roots:     true,
		tlsServer: srv,
	})
	err := h.err(t, "https://api.example.com:"+strconv.Itoa(int(srv.port(t)))+"/")
	wantBlocked(t, err, egress.ReasonAddressBlocked)
	if n := srv.count.Load(); n != 0 {
		t.Errorf("server saw %d requests, want 0", n)
	}
	if n := h.res.calls.Load(); n != 1 {
		t.Errorf("resolver called %d times, want 1", n)
	}
	if n := h.dials.Load(); n != 0 {
		t.Errorf("%d dials, want 0", n)
	}
}

func TestSECT09_MetadataAddressRefusedEvenWithAllowPrivate(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{
		rules:   []egress.Rule{{Host: "metadata.example.com", AllowPrivate: true}},
		answers: map[string][]netip.Addr{"metadata.example.com": addrs("169.254.169.254")},
	})
	err := h.err(t, "http://metadata.example.com/latest/meta-data/")
	wantBlocked(t, err, egress.ReasonAddressBlocked)
	if n := h.dials.Load(); n != 0 {
		t.Errorf("%d dials, want 0", n)
	}
}

func TestAllowPrivateRuleReachesLoopbackService(t *testing.T) {
	t.Parallel()
	srv := newServer(t, ok)
	port := srv.port(t)
	h := newHarness(t, setup{
		rules: []egress.Rule{
			{Host: "127.0.0.1", Port: port, AllowPrivate: true},
			{Host: "svc.example.com", Port: port, AllowPrivate: true},
		},
		answers: map[string][]netip.Addr{"svc.example.com": {loopback4}},
	})
	for _, target := range []string{
		"http://127.0.0.1:" + strconv.Itoa(int(port)) + "/",
		"http://svc.example.com:" + strconv.Itoa(int(port)) + "/",
	} {
		r, err := h.fetch(t, target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if r.status != http.StatusOK || r.text != "ok" {
			t.Fatalf("%s: %d %q", target, r.status, r.text)
		}
	}
	if n := srv.count.Load(); n != 2 {
		t.Errorf("server saw %d requests, want 2", n)
	}
}

// TestDenyAllRefusesEveryTarget: with Deps.DenyAll even a target that a rule with allow_private
// and the consumer policy allow is refused before any resolution or dial (M1-71, the contract
// server of G7 runs with it).
func TestDenyAllRefusesEveryTarget(t *testing.T) {
	t.Parallel()
	srv := newServer(t, ok)
	port := srv.port(t)
	h := newHarness(t, setup{
		rules:   []egress.Rule{{Host: "127.0.0.1", Port: port, AllowPrivate: true}, {Host: "svc.example.com", Port: port, AllowPrivate: true}},
		policy:  func(context.Context, string, string, uint16) bool { return true },
		answers: map[string][]netip.Addr{"svc.example.com": {loopback4}},
		denyAll: true,
	})
	for _, target := range []string{
		"http://127.0.0.1:" + strconv.Itoa(int(port)) + "/",
		"http://svc.example.com:" + strconv.Itoa(int(port)) + "/",
		"https://192.0.2.1/",
	} {
		_, err := h.fetch(t, target)
		var blocked *egress.BlockedError
		if !errors.As(err, &blocked) || blocked.Reason != egress.ReasonHostNotAllowed {
			t.Fatalf("%s: %v, want host_not_allowed", target, err)
		}
	}
	if n := h.res.calls.Load(); n != 0 {
		t.Errorf("%d resolutions", n)
	}
	if n := srv.count.Load(); n != 0 {
		t.Errorf("server saw %d requests", n)
	}
}

// TestRulePortZeroMatchesOnlyDefaultPort: a rule without port allows only the default port of
// the scheme, so a request to another port of the same host is allowed only by the consumer
// policy and gets no access to private addresses.
func TestRulePortZeroMatchesOnlyDefaultPort(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{
		rules:     []egress.Rule{{Host: "svc.example.com", AllowPrivate: true}},
		policy:    hostIs("svc.example.com"),
		answers:   map[string][]netip.Addr{"svc.example.com": {loopback4}},
		intercept: true,
	})
	if err := h.err(t, "https://svc.example.com/"); !errors.Is(err, errIntercepted) {
		t.Fatalf("default port: want a dial, got %v", err)
	}
	err := h.err(t, "https://svc.example.com:8443/")
	wantBlocked(t, err, egress.ReasonAddressBlocked)
	err = h.err(t, "http://svc.example.com:8080/")
	wantBlocked(t, err, egress.ReasonSchemeNotAllowed)
	if got := h.addresses(); len(got) != 1 || got[0] != "127.0.0.1:443" {
		t.Errorf("dialed %v, want [127.0.0.1:443]", got)
	}
}

func TestHTTPSchemeOnlyWithAllowPrivate(t *testing.T) {
	t.Parallel()
	srv := newServer(t, ok)
	port := srv.port(t)
	h := newHarness(t, setup{
		rules:   []egress.Rule{{Host: "127.0.0.1", Port: port}, {Host: "api.example.com"}},
		policy:  hostIs("policy.example.com"),
		answers: map[string][]netip.Addr{"api.example.com": addrs("192.0.2.10"), "policy.example.com": addrs("192.0.2.11")},
	})
	for _, target := range []string{
		"http://127.0.0.1:" + strconv.Itoa(int(port)) + "/",
		"http://api.example.com/",
		"http://policy.example.com/",
		"ftp://api.example.com/",
		"ws://api.example.com/",
	} {
		err := h.err(t, target)
		wantBlocked(t, err, egress.ReasonSchemeNotAllowed)
	}
	if n := srv.count.Load(); n != 0 {
		t.Errorf("server saw %d requests, want 0", n)
	}
	if n := h.res.calls.Load(); n != 0 {
		t.Errorf("resolver called %d times, want 0", n)
	}
}

func TestUserInfoRefused(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{
		rules:   []egress.Rule{{Host: "api.example.com"}},
		answers: map[string][]netip.Addr{"api.example.com": addrs("192.0.2.10")},
	})
	for _, target := range []string{"https://someone:hunter2@api.example.com/", "https://someone@api.example.com/"} {
		err := h.err(t, target)
		wantBlocked(t, err, egress.ReasonURLNotAllowed)
		if strings.Contains(err.Error(), "hunter2") || strings.Contains(err.Error(), "someone") {
			t.Errorf("error %q contains user info", err)
		}
	}
	if n := h.res.calls.Load(); n != 0 {
		t.Errorf("resolver called %d times, want 0", n)
	}
	if out := h.log.String(); strings.Contains(out, "hunter2") || strings.Contains(out, "someone") {
		t.Errorf("log contains user info: %s", out)
	}
}

func TestTLSVerificationAlwaysOn(t *testing.T) {
	t.Parallel()
	srv := newTLSServer(t, ok)
	port := srv.port(t)
	rules := []egress.Rule{
		{Host: "127.0.0.1", Port: port, AllowPrivate: true},
		{Host: "deep.wrong.example.com", Port: port, AllowPrivate: true},
	}
	answers := map[string][]netip.Addr{"deep.wrong.example.com": {loopback4}}
	target := "https://127.0.0.1:" + strconv.Itoa(int(port)) + "/"

	t.Run("system roots", func(t *testing.T) {
		h := newHarness(t, setup{rules: rules, answers: answers})
		err := h.err(t, target)
		var verr *tls.CertificateVerificationError
		if !errors.As(err, &verr) {
			t.Fatalf("want a certificate verification error, got %v", err)
		}
	})
	t.Run("server certificate as root", func(t *testing.T) {
		h := newHarness(t, setup{rules: rules, answers: answers, roots: true, tlsServer: srv})
		r, err := h.fetch(t, target)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if r.text != "ok" {
			t.Fatalf("body %q", r.text)
		}
		if r.tls == nil || r.tls.Version < tls.VersionTLS12 {
			t.Fatalf("TLS state %+v", r.tls)
		}
	})
	// The test certificate of net/http covers example.com and *.example.com; a name two labels
	// below example.com is outside it.
	t.Run("host name outside the certificate", func(t *testing.T) {
		h := newHarness(t, setup{rules: rules, answers: answers, roots: true, tlsServer: srv})
		err := h.err(t, "https://deep.wrong.example.com:"+strconv.Itoa(int(port))+"/")
		var herr x509.HostnameError
		if !errors.As(err, &herr) {
			t.Fatalf("want a host name error, got %v", err)
		}
	})
	if n := srv.count.Load(); n != 1 {
		t.Errorf("server saw %d requests, want 1", n)
	}
}

// TestProxyEnvironmentIgnored sets the proxy variables to a counting server; the request goes
// straight to its target. The target is a DNS name: net/http never proxies loopback literals,
// so a loopback URL would pass even with environment proxies.
func TestProxyEnvironmentIgnored(t *testing.T) {
	proxy := newServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		t.Setenv(name, proxy.URL)
	}
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	srv := newServer(t, ok)
	port := srv.port(t)
	h := newHarness(t, setup{
		rules:   []egress.Rule{{Host: "svc.example.com", Port: port, AllowPrivate: true}},
		answers: map[string][]netip.Addr{"svc.example.com": {loopback4}},
	})
	r, err := h.fetch(t, "http://svc.example.com:"+strconv.Itoa(int(port))+"/")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if r.text != "ok" {
		t.Fatalf("body %q, status %d", r.text, r.status)
	}
	if n := proxy.count.Load(); n != 0 {
		t.Errorf("proxy saw %d requests, want 0", n)
	}
}

// logRecords decodes the JSON log lines.
func logRecords(t *testing.T, out string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		records = append(records, m)
	}
	return records
}

func TestBlockedAttemptLog(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{
		policy:  hostIs("private.example.com"),
		answers: map[string][]netip.Addr{"private.example.com": {loopback4}},
	})
	err := h.err(t, "https://blocked.example.com/secret-path/x?token=tok3n&q=1")
	wantBlocked(t, err, egress.ReasonHostNotAllowed)
	err = h.err(t, "https://private.example.com/other-path?key=k3y")
	wantBlocked(t, err, egress.ReasonAddressBlocked)

	out := h.log.String()
	for _, secret := range []string{"secret-path", "token", "tok3n", "other-path", "key", "k3y", "127.0.0.1"} {
		if strings.Contains(out, secret) {
			t.Errorf("log contains %q: %s", secret, out)
		}
	}
	records := logRecords(t, out)
	want := []map[string]any{
		{"level": "WARN", "msg": "egress request blocked", "bothy.egress.purpose": "test", "bothy.reason": "host_not_allowed", "server.address": "blocked.example.com"},
		{"level": "WARN", "msg": "egress request blocked", "bothy.egress.purpose": "test", "bothy.reason": "address_blocked", "server.address": "private.example.com"},
	}
	if len(records) != len(want) {
		t.Fatalf("%d log records, want %d: %s", len(records), len(want), out)
	}
	for i, w := range want {
		delete(records[i], "time")
		if len(records[i]) != len(w) {
			t.Errorf("record %d has fields %v, want %v", i, records[i], w)
		}
		for k, v := range w {
			if records[i][k] != v {
				t.Errorf("record %d: %s = %v, want %v", i, k, records[i][k], v)
			}
		}
	}
}

func TestBlockedAttemptCounted(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{
		rules:   []egress.Rule{{Host: "api.example.com"}},
		policy:  hostIs("private.example.com"),
		answers: map[string][]netip.Addr{"private.example.com": {loopback4}},
	})
	for _, target := range []string{
		"https://blocked.example.com/",
		"https://private.example.com/",
		"http://api.example.com/",
		"https://user@api.example.com/",
	} {
		err := h.err(t, target)
		if !errors.Is(err, egress.ErrBlocked) {
			t.Fatalf("%s: %v", target, err)
		}
	}
	got := h.metrics.got()
	want := []egress.Reason{egress.ReasonHostNotAllowed, egress.ReasonAddressBlocked, egress.ReasonSchemeNotAllowed, egress.ReasonURLNotAllowed}
	if len(got) != len(want) {
		t.Fatalf("counted %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("counted %v, want %v", got, want)
		}
	}
}

func TestReasonValues(t *testing.T) {
	t.Parallel()
	// The values are the closed set of the attribute bothy.reason of bothy.egress.blocked in
	// docs/observability/signals.md; dashboards and alerts use them.
	for got, want := range map[egress.Reason]string{
		egress.ReasonURLNotAllowed:      "url_not_allowed",
		egress.ReasonSchemeNotAllowed:   "scheme_not_allowed",
		egress.ReasonHostNotAllowed:     "host_not_allowed",
		egress.ReasonAddressBlocked:     "address_blocked",
		egress.ReasonRedirectNotAllowed: "redirect_not_allowed",
	} {
		if string(got) != want {
			t.Errorf("reason %q, want %q", got, want)
		}
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	good := egress.Config{Timeout: time.Second, MaxResponseBytes: 1}
	goodOpts := egress.Options{Purpose: "oidc"}
	cases := map[string]struct {
		cfg  egress.Config
		opts egress.Options
	}{
		"zero timeout":           {egress.Config{MaxResponseBytes: 1}, goodOpts},
		"negative timeout":       {egress.Config{Timeout: -time.Second, MaxResponseBytes: 1}, goodOpts},
		"zero size":              {egress.Config{Timeout: time.Second}, goodOpts},
		"negative option time":   {good, egress.Options{Purpose: "oidc", Timeout: -1}},
		"negative option size":   {good, egress.Options{Purpose: "oidc", MaxResponseBytes: -1}},
		"empty purpose":          {good, egress.Options{}},
		"purpose with space":     {good, egress.Options{Purpose: "oidc discovery"}},
		"purpose upper case":     {good, egress.Options{Purpose: "OIDC"}},
		"unknown redirect mode":  {good, egress.Options{Purpose: "oidc", Redirects: egress.RedirectMode(7)}},
		"rule upper case":        {egress.Config{Timeout: time.Second, MaxResponseBytes: 1, AllowedHosts: []egress.Rule{{Host: "API.example.com"}}}, goodOpts},
		"rule wildcard":          {egress.Config{Timeout: time.Second, MaxResponseBytes: 1, AllowedHosts: []egress.Rule{{Host: "*.example.com"}}}, goodOpts},
		"rule mapped address":    {egress.Config{Timeout: time.Second, MaxResponseBytes: 1, AllowedHosts: []egress.Rule{{Host: "::ffff:192.0.2.1"}}}, goodOpts},
		"rule empty host":        {egress.Config{Timeout: time.Second, MaxResponseBytes: 1, AllowedHosts: []egress.Rule{{}}}, goodOpts},
		"rule duplicate":         {egress.Config{Timeout: time.Second, MaxResponseBytes: 1, AllowedHosts: []egress.Rule{{Host: "a.example.com", Port: 443}, {Host: "a.example.com", Port: 443, AllowPrivate: true}}}, goodOpts},
		"rule with trailing dot": {egress.Config{Timeout: time.Second, MaxResponseBytes: 1, AllowedHosts: []egress.Rule{{Host: "a.example.com."}}}, goodOpts},
		"rule with zone":         {egress.Config{Timeout: time.Second, MaxResponseBytes: 1, AllowedHosts: []egress.Rule{{Host: "fe80::1%en0"}}}, goodOpts},
	}
	for name, tc := range cases {
		if c, err := egress.New(tc.cfg, tc.opts, egress.Deps{}); err == nil || c != nil {
			t.Errorf("%s: New = %v, %v; want nil and an error", name, c, err)
		}
	}
	if _, err := egress.New(good, goodOpts, egress.Deps{}); err != nil {
		t.Errorf("valid input: %v", err)
	}
}

func TestSECT09_RedirectToPrivateRefused(t *testing.T) {
	t.Parallel()
	for _, scheme := range []string{"https", "http"} {
		for _, mode := range []egress.RedirectMode{egress.RedirectNever, egress.RedirectChecked} {
			t.Run(scheme+"/"+strconv.Itoa(int(mode)), func(t *testing.T) {
				t.Parallel()
				target := newServer(t, ok)
				if scheme == "https" {
					target = newTLSServer(t, ok)
				}
				location := scheme + "://internal.example.com:" + strconv.Itoa(int(target.port(t))) + "/admin"
				origin := newServer(t, func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, location, http.StatusFound)
				})
				h := newHarness(t, setup{
					rules:     []egress.Rule{{Host: "127.0.0.1", Port: origin.port(t), AllowPrivate: true}},
					policy:    hostIs("internal.example.com"),
					answers:   map[string][]netip.Addr{"internal.example.com": {loopback4}},
					redirects: mode,
					roots:     scheme == "https",
					tlsServer: target,
				})
				r, err := h.fetch(t, origin.URL+"/start")
				switch {
				case mode == egress.RedirectNever:
					if err != nil {
						t.Fatalf("Do: %v", err)
					}
					if r.status != http.StatusFound || r.header.Get("Location") != location {
						t.Fatalf("got %d %q, want 302 to %s", r.status, r.header.Get("Location"), location)
					}
				case scheme == "https":
					wantBlocked(t, err, egress.ReasonAddressBlocked)
					wantCounted(t, h, egress.ReasonAddressBlocked)
				default:
					wantBlocked(t, err, egress.ReasonRedirectNotAllowed)
					wantCounted(t, h, egress.ReasonRedirectNotAllowed)
					if n := h.res.calls.Load(); n != 0 {
						t.Errorf("resolver called %d times, want 0", n)
					}
				}
				if n := target.count.Load(); n != 0 {
					t.Errorf("target saw %d requests, want 0", n)
				}
				if n := origin.count.Load(); n != 1 {
					t.Errorf("origin saw %d requests, want 1", n)
				}
			})
		}
	}
}

func TestRedirectCheckedFollowsAllowed(t *testing.T) {
	t.Parallel()
	final := newServer(t, ok)
	origin := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/done", http.StatusMovedPermanently)
	})
	rules := []egress.Rule{
		{Host: "127.0.0.1", Port: origin.port(t), AllowPrivate: true},
		{Host: "127.0.0.1", Port: final.port(t), AllowPrivate: true},
	}
	h := newHarness(t, setup{rules: rules, redirects: egress.RedirectChecked})
	r, err := h.fetch(t, origin.URL)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if r.text != "ok" || r.url.Path != "/done" {
		t.Fatalf("got %q from %s", r.text, r.url)
	}
}

func TestRedirectLimit(t *testing.T) {
	t.Parallel()
	var loop *server
	loop = newServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, loop.URL+"/again", http.StatusFound)
	})
	h := newHarness(t, setup{
		rules:     []egress.Rule{{Host: "127.0.0.1", Port: loop.port(t), AllowPrivate: true}},
		redirects: egress.RedirectChecked,
	})
	err := h.err(t, loop.URL)
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("want the redirect limit of net/http, got %v", err)
	}
	if n := loop.count.Load(); n != 10 {
		t.Errorf("server saw %d requests, want 10", n)
	}
}

func TestResponseLimit(t *testing.T) {
	t.Parallel()
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		payload := bytes.Repeat([]byte("a"), n)
		if r.URL.Query().Get("gzip") == "1" {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				http.Error(w, "no gzip", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Encoding", "gzip")
			zw := gzip.NewWriter(w)
			_, _ = zw.Write(payload)
			_ = zw.Close()
			return
		}
		if r.URL.Query().Get("chunked") == "1" {
			w.(http.Flusher).Flush()
		}
		_, _ = w.Write(payload)
	})
	port := srv.port(t)
	rules := []egress.Rule{{Host: "127.0.0.1", Port: port, AllowPrivate: true}}
	read := func(t *testing.T, h *harness, query string) (int, error) {
		t.Helper()
		resp, err := h.get(t, srv.URL+"/?"+query)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		return len(b), err
	}
	cases := []struct {
		name     string
		maxBytes int64
		override int64
		query    string
		wantLen  int
		tooLarge bool
	}{
		{"exactly the limit", 1024, 0, "n=1024", 1024, false},
		{"one byte more", 1024, 0, "n=1025", 1024, true},
		{"chunked one byte more", 1024, 0, "n=1025&chunked=1", 1024, true},
		{"gzip exactly the limit", 1024, 0, "n=1024&gzip=1", 1024, false},
		{"gzip decoded one byte more", 1024, 0, "n=1025&gzip=1", 1024, true},
		{"consumer override", 512, 1024, "n=1024", 1024, false},
		{"consumer override exceeded", 4096, 16, "n=17", 16, true},
		{"empty body", 1024, 1, "n=0", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, setup{rules: rules, maxBytes: tc.maxBytes, opts: egress.Options{MaxResponseBytes: tc.override}})
			n, err := read(t, h, tc.query)
			if tc.tooLarge != errors.Is(err, egress.ErrResponseTooLarge) {
				t.Fatalf("read %d bytes, error %v; want too large = %v", n, err, tc.tooLarge)
			}
			if !tc.tooLarge && err != nil {
				t.Fatalf("read: %v", err)
			}
			if n != tc.wantLen {
				t.Fatalf("read %d bytes, want %d", n, tc.wantLen)
			}
		})
	}
}

// slowServer answers only when the client gives up (or the server closes at the end of the
// test), so a client without its time limit waits past testkit.Patience.
func slowServer(t *testing.T) *server {
	return newServer(t, func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
}

func TestTimeout(t *testing.T) {
	t.Parallel()
	srv := slowServer(t)
	rules := []egress.Rule{{Host: "127.0.0.1", Port: srv.port(t), AllowPrivate: true}}
	for name, s := range map[string]setup{
		"egress.timeout": {rules: rules, timeout: 200 * time.Millisecond},
		// egress.timeout far beyond testkit.Patience: only the override ends the request in time.
		"consumer override": {rules: rules, timeout: 2 * testkit.Patience, opts: egress.Options{Timeout: 200 * time.Millisecond}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, s)
			// In a goroutine, so a request that ignored its time limit fails the test after
			// testkit.Patience; the end of the test cancels it.
			done := make(chan error, 1)
			go func() { done <- h.err(t, srv.URL) }()
			err := testkit.Receive(t, "the end of the request by its time limit", done)
			var ne net.Error
			if !errors.As(err, &ne) || !ne.Timeout() {
				t.Fatalf("want a timeout, got %v", err)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("error %v does not wrap context.DeadlineExceeded", err)
			}
			if !os.IsTimeout(err) {
				t.Errorf("os.IsTimeout(%v) = false", err)
			}
		})
	}
}

// TestTimeoutBoundsResolver: net/http detaches the dial from the request context, so the egress
// dialer bounds name resolution and connect with the timeout itself.
func TestTimeoutBoundsResolver(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{policy: hostIs("slow.example.com"), timeout: 200 * time.Millisecond})
	h.res.block = true
	if err := h.err(t, "https://slow.example.com/"); err == nil {
		t.Fatal("no error")
	}
	select {
	case <-h.res.ended:
	case <-time.After(testkit.Patience):
		t.Fatal("the lookup still runs after the timeout")
	}
}

func TestDoErrorWithoutPathOrQuery(t *testing.T) {
	t.Parallel()
	srv := newTLSServer(t, ok)
	port := srv.port(t)
	h := newHarness(t, setup{rules: []egress.Rule{{Host: "127.0.0.1", Port: port, AllowPrivate: true}}})
	for _, target := range []string{
		"https://blocked.example.com/private/path?token=s3cret",
		"https://127.0.0.1:" + strconv.Itoa(int(port)) + "/private/path?token=s3cret",
	} {
		err := h.err(t, target)
		if err == nil {
			t.Fatalf("%s: no error", target)
		}
		for _, part := range []string{"private", "path", "token", "s3cret"} {
			if strings.Contains(err.Error(), part) {
				t.Errorf("error %q contains %q", err, part)
			}
		}
	}
}

func TestHTTPClientEnforcesChecks(t *testing.T) {
	t.Parallel()
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(bytes.Repeat([]byte("a"), 2048)) })
	h := newHarness(t, setup{
		rules:    []egress.Rule{{Host: "127.0.0.1", Port: srv.port(t), AllowPrivate: true}},
		maxBytes: 1024,
	})
	hc := h.client.HTTPClient()
	get := func(target string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		return hc.Do(req)
	}
	resp, err := get("https://blocked.example.com/")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("blocked host reached")
	}
	wantBlocked(t, err, egress.ReasonHostNotAllowed)

	resp, err = get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.ReadAll(resp.Body); !errors.Is(err, egress.ErrResponseTooLarge) {
		t.Fatalf("want ErrResponseTooLarge, got %v", err)
	}
	// A caller that changes its copy changes nothing for the next caller.
	hc.Timeout = 0
	if h.client.HTTPClient().Timeout != 5*time.Second {
		t.Fatal("HTTPClient returned a shared client")
	}
}

func TestResponseTooLargeStays(t *testing.T) {
	t.Parallel()
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(bytes.Repeat([]byte("a"), 64)) })
	h := newHarness(t, setup{rules: []egress.Rule{{Host: "127.0.0.1", Port: srv.port(t), AllowPrivate: true}}, opts: egress.Options{MaxResponseBytes: 8}})
	resp, err := h.get(t, srv.URL)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf := make([]byte, 64)
	total := 0
	for i := 0; i < 64; i++ {
		n, err := resp.Body.Read(buf)
		total += n
		if errors.Is(err, egress.ErrResponseTooLarge) {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
	}
	if total != 8 {
		t.Fatalf("read %d bytes before the error, want 8", total)
	}
	for range 2 {
		if n, err := resp.Body.Read(buf); n != 0 || !errors.Is(err, egress.ErrResponseTooLarge) {
			t.Fatalf("Read after the limit = %d, %v", n, err)
		}
	}
}

func TestURLFormsRefused(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{
		rules:  []egress.Rule{{Host: "api.example.com"}, {Host: "fe80::1", AllowPrivate: true}},
		policy: func(context.Context, string, string, uint16) bool { return true },
	})
	for _, target := range []string{
		"https://api.example.com:0/",
		"https://[fe80::1%25en0]/",
		"https:///path",
		"https://caf\u00e9.example.com/",
		"https://1.2.3/",
		"https://0x7f000001/",
	} {
		err := h.err(t, target)
		wantBlocked(t, err, egress.ReasonURLNotAllowed)
	}
	if n := h.res.calls.Load(); n != 0 {
		t.Errorf("resolver called %d times, want 0", n)
	}
	var be *egress.BlockedError
	err := h.err(t, "https:///nohost")
	if !errors.As(err, &be) || be.Host != "" || be.Error() != "egress: blocked: url_not_allowed" {
		t.Errorf("URL without host: %v", err)
	}
}

// TestResolverFailureIsNotBlocked: a name that does not resolve is a failure of the target, not
// a refusal; it is neither logged nor counted as blocked.
func TestResolverFailureIsNotBlocked(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{policy: hostIs("missing.example.com")})
	err := h.err(t, "https://missing.example.com/")
	var dnsErr *net.DNSError
	if !errors.As(err, &dnsErr) || errors.Is(err, egress.ErrBlocked) {
		t.Fatalf("want a DNS error, got %v", err)
	}
	if out := h.log.String(); out != "" {
		t.Errorf("log: %s", out)
	}
	if got := h.metrics.got(); len(got) != 0 {
		t.Errorf("counted %v", got)
	}
}

// closeRecorder records whether a request body was closed.
type closeRecorder struct {
	io.Reader
	closed bool
}

func (c *closeRecorder) Close() error { c.closed = true; return nil }

func TestRefusedRequestBodyIsClosed(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{})
	b := &closeRecorder{Reader: strings.NewReader("payload")}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://blocked.example.com/", b)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := h.client.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	wantBlocked(t, err, egress.ReasonHostNotAllowed)
	if !b.closed {
		t.Error("request body not closed")
	}
}

// TestSocketCheckRefusalIsLogged: a refusal of the second line (net.Dialer.Control) ends like a
// refusal of the first, logged and counted once as address_blocked.
func TestSocketCheckRefusalIsLogged(t *testing.T) {
	t.Parallel()
	h := newHarness(t, setup{policy: hostIs("api.example.com"), answers: map[string][]netip.Addr{"api.example.com": addrs("192.0.2.10")}})
	egress.OnDial(h.client, func(context.Context, string, string) error {
		return &egress.BlockedError{Reason: egress.ReasonAddressBlocked}
	})
	err := h.err(t, "https://api.example.com/")
	wantBlocked(t, err, egress.ReasonAddressBlocked)
	if got := h.metrics.got(); len(got) != 1 || got[0] != egress.ReasonAddressBlocked {
		t.Errorf("counted %v", got)
	}
	if n := len(logRecords(t, h.log.String())); n != 1 {
		t.Errorf("%d log records, want 1", n)
	}
}

// wantCounted asserts that exactly one refused attempt with reason was counted and logged, with
// the host of the refused target.
func wantCounted(t *testing.T, h *harness, reason egress.Reason) {
	t.Helper()
	if got := h.metrics.got(); len(got) != 1 || got[0] != reason {
		t.Errorf("counted %v, want [%s]", got, reason)
	}
	records := logRecords(t, h.log.String())
	if len(records) != 1 || records[0]["bothy.reason"] != string(reason) || records[0]["server.address"] != "internal.example.com" {
		t.Errorf("log records %v, want one with reason %s and host internal.example.com", records, reason)
	}
}

// switchingServer answers every request with 101 Switching Protocols and then stays silent
// until the client closes the connection.
func switchingServer(t *testing.T) uint16 {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
					return
				}
				_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: x\r\n\r\n"))
				_, _ = io.Copy(io.Discard, conn)
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).AddrPort().Port()
}

// TestProtocolSwitchRefused: after a 101 answer net/http hands over the raw connection and no
// deadline applies any more, so the client refuses protocol switches instead of returning them.
func TestProtocolSwitchRefused(t *testing.T) {
	t.Parallel()
	port := switchingServer(t)
	target := "http://127.0.0.1:" + strconv.Itoa(int(port)) + "/"
	// A returned or waited-on switch would only end with egress.timeout, far beyond testkit.Patience.
	h := newHarness(t, setup{rules: []egress.Rule{{Host: "127.0.0.1", Port: port, AllowPrivate: true}}, timeout: 2 * testkit.Patience})
	loop := egress.NewLoopbackResolver(5*time.Second, newResolver(nil))
	for name, get := range map[string]func() error{
		"Do": func() error { return h.err(t, target) },
		"HTTPClient": func() error {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := h.client.HTTPClient().Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
			return err
		},
		"NewLoopback": func() error {
			_, err := loopbackGet(t, loop, target)
			return err
		},
	} {
		start := time.Now()
		err := get()
		if err == nil || !strings.Contains(err.Error(), "switched protocols") {
			t.Errorf("%s: want the protocol switch refused, got %v", name, err)
		}
		if d := time.Since(start); d > testkit.Patience {
			t.Errorf("%s: took %v", name, d)
		}
	}
}

// TestRedirectWithInvalidLocation: net/http turns a redirect with a Location it cannot parse into
// an error that quotes the header; the client ends it with a fixed text in both modes.
func TestRedirectWithInvalidLocation(t *testing.T) {
	t.Parallel()
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/private%zz?token=s3cret")
		w.WriteHeader(http.StatusFound)
	})
	rules := []egress.Rule{{Host: "127.0.0.1", Port: srv.port(t), AllowPrivate: true}}
	for _, mode := range []egress.RedirectMode{egress.RedirectNever, egress.RedirectChecked} {
		h := newHarness(t, setup{rules: rules, redirects: mode})
		err := h.err(t, srv.URL+"/start")
		if err == nil || !strings.Contains(err.Error(), "invalid Location") {
			t.Errorf("mode %d: want the invalid Location refused, got %v", mode, err)
			continue
		}
		for _, part := range []string{"private", "token", "s3cret", "%zz"} {
			if strings.Contains(err.Error(), part) {
				t.Errorf("mode %d: error %q contains %q", mode, err, part)
			}
		}
	}
}

func TestCloseIdleConnectionsReachesTheTransport(t *testing.T) {
	t.Parallel()
	// Only connections that carried a request count: a parallel test may dial a port this server
	// reused (its stale connection never sends a request), which StateNew would count too.
	var conns atomic.Int64
	var seen sync.Map
	srv := httptest.NewUnstartedServer(http.HandlerFunc(ok))
	srv.Config.ConnState = func(c net.Conn, state http.ConnState) {
		if state == http.StateActive {
			if _, loaded := seen.LoadOrStore(c, true); !loaded {
				conns.Add(1)
			}
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)
	port, err := strconv.ParseUint(srv.URL[strings.LastIndex(srv.URL, ":")+1:], 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, setup{rules: []egress.Rule{{Host: "127.0.0.1", Port: uint16(port), AllowPrivate: true}}})
	for range 2 {
		if _, err := h.fetch(t, srv.URL); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		h.client.HTTPClient().CloseIdleConnections()
	}
	if n := conns.Load(); n != 2 {
		t.Fatalf("%d connections, want 2: the idle connection was not closed", n)
	}
}

// TestDialSplitsTimeAcrossAddresses: an address that never answers does not use up the whole
// time limit; the next address of the answer still gets its share.
func TestDialSplitsTimeAcrossAddresses(t *testing.T) {
	t.Parallel()
	srv := newServer(t, ok)
	port := srv.port(t)
	h := newHarness(t, setup{
		rules:   []egress.Rule{{Host: "svc.example.com", Port: port, AllowPrivate: true}},
		answers: map[string][]netip.Addr{"svc.example.com": addrs("::1", "127.0.0.1")},
		timeout: 2 * time.Second,
	})
	egress.OnDial(h.client, func(ctx context.Context, _, address string) error {
		if strings.HasPrefix(address, "[::1]") {
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	})
	r, err := h.fetch(t, "http://svc.example.com:"+strconv.Itoa(int(port))+"/")
	if err != nil || r.text != "ok" {
		t.Fatalf("got %q, %v; want the second address to answer", r.text, err)
	}
}

// TestTransportDeadlineWithoutClientTimeout: a library that puts the transport of HTTPClient
// into its own client without a timeout still gets the time limit for each request.
func TestTransportDeadlineWithoutClientTimeout(t *testing.T) {
	t.Parallel()
	srv := slowServer(t)
	h := newHarness(t, setup{rules: []egress.Rule{{Host: "127.0.0.1", Port: srv.port(t), AllowPrivate: true}}, timeout: 200 * time.Millisecond})
	own := &http.Client{Transport: h.client.HTTPClient().Transport}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	// In a goroutine, so a request without the time limit fails the test after testkit.Patience;
	// the end of the test cancels it.
	done := make(chan error, 1)
	go func() {
		resp, err := own.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		done <- err
	}()
	err = testkit.Receive(t, "the end of the request by the transport deadline", done)
	if err == nil {
		t.Fatal("no error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %v is not a deadline", err)
	}
}

// TestTimeoutCoversAllHops: each hop stays below the time limit, the whole chain does not; the
// limit holds for the request, not for each hop.
func TestTimeoutCoversAllHops(t *testing.T) {
	t.Parallel()
	var chain *server
	chain = newServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		if n < 5 {
			http.Redirect(w, r, chain.URL+"/?n="+strconv.Itoa(n+1), http.StatusFound)
			return
		}
		ok(w, r)
	})
	h := newHarness(t, setup{
		rules:     []egress.Rule{{Host: "127.0.0.1", Port: chain.port(t), AllowPrivate: true}},
		redirects: egress.RedirectChecked,
		timeout:   400 * time.Millisecond,
	})
	err := h.err(t, chain.URL+"/?n=0")
	if !os.IsTimeout(err) {
		t.Fatalf("want a timeout over all hops, got %v", err)
	}
}

func TestResponseHeaderLimit(t *testing.T) {
	t.Parallel()
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Large", strings.Repeat("a", 2048))
		ok(w, nil)
	})
	rules := []egress.Rule{{Host: "127.0.0.1", Port: srv.port(t), AllowPrivate: true}}
	// egress.max_response_bytes bounds the header; a consumer override changes only the body.
	for name, s := range map[string]setup{
		"egress.max_response_bytes": {rules: rules, maxBytes: 1024},
		"body override":             {rules: rules, maxBytes: 1024, opts: egress.Options{MaxResponseBytes: 1 << 20}},
	} {
		h := newHarness(t, s)
		err := h.err(t, srv.URL)
		if err == nil || !strings.Contains(err.Error(), "headers exceeded") {
			t.Errorf("%s: want the header limit, got %v", name, err)
		}
	}
	h := newHarness(t, setup{rules: rules, maxBytes: 4096})
	if _, err := h.fetch(t, srv.URL); err != nil {
		t.Errorf("header below the limit: %v", err)
	}
}

func TestTLSBelowVersion12Refused(t *testing.T) {
	t.Parallel()
	srv := &server{}
	srv.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.count.Add(1)
		ok(w, r)
	}))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11} // #nosec G402 -- the server offers only TLS 1.0 and 1.1 to prove that the client refuses them
	srv.StartTLS()
	t.Cleanup(srv.Close)
	h := newHarness(t, setup{rules: []egress.Rule{{Host: "127.0.0.1", Port: srv.port(t), AllowPrivate: true}}, roots: true, tlsServer: srv})
	if err := h.err(t, srv.URL); err == nil {
		t.Fatal("TLS 1.1 accepted")
	}
	if n := srv.count.Load(); n != 0 {
		t.Errorf("server saw %d requests", n)
	}
}
