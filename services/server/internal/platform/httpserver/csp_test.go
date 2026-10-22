// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// wantDirectives are the directives of docs/spec/19-security.md 5 without the nonce and the
// Trusted Types names, which the test checks on their own.
var wantDirectives = []string{
	"default-src 'none'",
	"script-src 'self' 'wasm-unsafe-eval'",
	"img-src 'self' blob: data:",
	"font-src 'self'",
	"connect-src 'self'",
	"worker-src 'self' blob:",
	"manifest-src 'self'",
	"base-uri 'none'",
	"form-action 'self'",
	"frame-ancestors 'none'",
	"object-src 'none'",
	"require-trusted-types-for 'script'",
	"report-to csp",
}

// styleNonce extracts the nonce of the style-src directive of a CSP value.
var styleNonce = regexp.MustCompile(`(?:^|; )style-src 'self' 'nonce-([A-Za-z0-9+/=]+)'(?:;|$)`)

// bodyNonce extracts the nonce attribute of the bootstrap script tag of index.html.
var bodyNonce = regexp.MustCompile(`<script src="/console/flutter_bootstrap.js" async nonce="([^"]*)">`)

// directives splits a CSP value into its directives.
func directives(csp string) map[string]string {
	out := map[string]string{}
	for d := range strings.SplitSeq(csp, ";") {
		d = strings.TrimSpace(d)
		name, _, _ := strings.Cut(d, " ")
		out[name] = d
	}
	return out
}

// TestCSPHeader holds the policy of 19 5 on index.html and the assets: every directive, a fresh
// 128-bit nonce per response, the same nonce in the header and in the body of index.html, the
// Trusted Types names of the one list, no frame-src and no 'unsafe-inline' or 'unsafe-eval'.
func TestCSPHeader(t *testing.T) {
	t.Parallel()
	h := consoleServer(t, consoleBuild(t))
	seen := map[string]bool{}
	for _, p := range []string{"/console/", "/console/", "/console/stats", "/console/index.html", "/console/main.dart.js", "/console/canvaskit/canvaskit.wasm", "/console/missing.js"} {
		res, body := do(t, h, http.MethodGet, p)
		csp := res.Header.Get("Content-Security-Policy")
		d := directives(csp)
		for _, want := range wantDirectives {
			name, _, _ := strings.Cut(want, " ")
			if d[name] != want {
				t.Errorf("GET %s directive %s = %q, want %q", p, name, d[name], want)
			}
		}
		if want := "trusted-types " + strings.Join(httpserver.TrustedTypesPolicies, " "); d["trusted-types"] != want {
			t.Errorf("GET %s trusted-types = %q, want %q", p, d["trusted-types"], want)
		}
		if len(d) != len(wantDirectives)+2 {
			t.Errorf("GET %s has %d directives, want %d: %s", p, len(d), len(wantDirectives)+2, csp)
		}
		for _, bad := range []string{"frame-src", "'unsafe-inline'", "'unsafe-eval'", "report-uri"} {
			if strings.Contains(csp, bad) {
				t.Errorf("GET %s CSP contains %s: %s", p, bad, csp)
			}
		}
		m := styleNonce.FindStringSubmatch(csp)
		if m == nil {
			t.Fatalf("GET %s style-src without a nonce: %s", p, csp)
		}
		raw, err := base64.StdEncoding.DecodeString(m[1])
		if err != nil || len(raw) != 16 {
			t.Errorf("GET %s nonce %q is not 16 bytes of base64", p, m[1])
		}
		if seen[m[1]] {
			t.Errorf("GET %s reuses the nonce %q", p, m[1])
		}
		seen[m[1]] = true
		if res.Header.Get("Content-Type") == "text/html; charset=utf-8" {
			b := bodyNonce.FindStringSubmatch(body)
			if b == nil || b[1] != m[1] {
				t.Errorf("GET %s body nonce %v, header nonce %q", p, b, m[1])
			}
			if strings.Contains(body, httpserver.NoncePlaceholder) {
				t.Errorf("GET %s body keeps the placeholder", p)
			}
		}
	}
	if res, _ := do(t, h, http.MethodGet, "/api/v1/unknown"); res.Header.Get("Content-Security-Policy") != "" {
		t.Error("an /api/ response carries the console CSP")
	}
}

// TestConsoleIsolationHeaders: every console response (index.html, an asset, the SPA fallback,
// the redirect of /console and a 404 below /console/) is cross-origin isolated and not
// embeddable: COOP same-origin, COEP require-corp, CORP same-origin, X-Frame-Options DENY, the
// Permissions-Policy of SEC-HDR-3 and the reporting endpoint of the group csp.
func TestConsoleIsolationHeaders(t *testing.T) {
	t.Parallel()
	h := consoleServer(t, consoleBuild(t))
	want := map[string]string{}
	for k, v := range hdr1 {
		want[k] = v
	}
	for k, v := range hdr3 {
		want[k] = v
	}
	for _, p := range []string{"/console/", "/console/main.dart.js", "/console/stats", "/console", "/console/escape.txt"} {
		res, _ := do(t, h, http.MethodGet, p)
		for k, v := range want {
			if got := res.Header.Get(k); got != v {
				t.Errorf("GET %s %s = %q, want %q", p, k, got, v)
			}
		}
		if res.Header.Get("Content-Security-Policy") == "" {
			t.Errorf("GET %s has no CSP", p)
		}
	}
}

// TestOpenConsoleAssetsRefusesAnIndexWithoutNoncePlaceholder: a build whose index.html has no
// nonce placeholder would run without the nonce the loader needs for its styles; the start ends.
func TestOpenConsoleAssetsRefusesAnIndexWithoutNoncePlaceholder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeIndex(t, dir, "<!DOCTYPE html><html><body><script src=\"/console/flutter_bootstrap.js\" async></script></body></html>")
	if _, err := httpserver.OpenConsoleAssets(dir); err == nil || !strings.Contains(err.Error(), httpserver.NoncePlaceholder) {
		t.Fatalf("err = %v, want one naming the placeholder", err)
	}
}
