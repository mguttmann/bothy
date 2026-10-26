// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// m603Policy is the policy M6-03 built for the nonce N, written out byte by byte: the console
// without console.grafana_embed_url must keep sending exactly these bytes (brief M6-17
// Design note 1, VZ-T12).
const m603Policy = "default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'nonce-N'; " +
	"img-src 'self' blob: data:; font-src 'self'; connect-src 'self'; worker-src 'self' blob:; " +
	"manifest-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; object-src 'none'; " +
	"require-trusted-types-for 'script'; trusted-types flutter-js; report-to csp"

// embedURL is a dashboard URL of the white-label Grafana example.
const embedURL = "https://grafana.example.com/d/ops/operations?orgId=1&kiosk"

// consoleServerWithEmbed serves the console of dir like consoleServer, with embed as the value of
// console.grafana_embed_url.
func consoleServerWithEmbed(t *testing.T, dir, embed string) http.Handler {
	t.Helper()
	v, err := httpserver.NewValidator(httpservertest.Contract, nil)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := httpserver.NewChain(httpserver.DefaultChainConfig(httpservertest.PublicURL),
		httpserver.Deps{Authenticator: actor.AnonymousOnly{}, Validator: v})
	if err != nil {
		t.Fatal(err)
	}
	assets, err := httpserver.OpenConsoleAssets(dir, embed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = assets.Close() })
	mux := http.NewServeMux()
	assets.Mount(mux)
	return chain.Public(mux)
}

// TestCSP_FrameSrcOnlyWithKey (VZ-T12, SEC-T19, brief M6-17 Design note 1): without
// console.grafana_embed_url the policy is byte-identical to M6-03 and has no frame-src; with it
// every console response (index.html, an asset, the SPA fallback, the redirect of /console and a
// 404 below /console/) carries exactly one frame-src with exactly the origin of the URL and
// nothing else, while frame-ancestors 'none' and X-Frame-Options: DENY stay.
func TestCSP_FrameSrcOnlyWithKey(t *testing.T) {
	t.Parallel()
	if got := httpserver.ContentSecurityPolicy("N", ""); got != m603Policy {
		t.Errorf("policy without the key\n got %s\nwant %s", got, m603Policy)
	}
	if got, want := httpserver.ContentSecurityPolicy("N", "https://grafana.example.com"), m603Policy+"; frame-src https://grafana.example.com"; got != want {
		t.Errorf("policy with the origin\n got %s\nwant %s", got, want)
	}
	dir := consoleBuild(t)
	cases := []struct {
		name, embed, want string
	}{
		{"without the key", "", ""},
		{"with the key", embedURL, "frame-src https://grafana.example.com"},
		{"default port and case dropped", "https://Grafana.Example.com:443/d/ops", "frame-src https://grafana.example.com"},
		{"own port kept", "https://grafana.example.com:3443/d/ops", "frame-src https://grafana.example.com:3443"},
	}
	for _, c := range cases {
		h := consoleServerWithEmbed(t, dir, c.embed)
		for _, p := range []string{"/console/", "/console/main.dart.js", "/console/stats", "/console", "/console/escape.txt"} {
			res, _ := do(t, h, http.MethodGet, p)
			csp := res.Header.Get("Content-Security-Policy")
			var frames []string
			for d := range strings.SplitSeq(csp, ";") {
				if d = strings.TrimSpace(d); strings.HasPrefix(d, "frame-src") {
					frames = append(frames, d)
				}
			}
			switch {
			case c.want == "" && len(frames) != 0:
				t.Errorf("%s: GET %s has %v", c.name, p, frames)
			case c.want != "" && (len(frames) != 1 || frames[0] != c.want):
				t.Errorf("%s: GET %s frame-src %v, want exactly %q", c.name, p, frames, c.want)
			}
			if d := directives(csp); d["frame-ancestors"] != "frame-ancestors 'none'" {
				t.Errorf("%s: GET %s frame-ancestors %q", c.name, p, d["frame-ancestors"])
			}
			if got := res.Header.Get("X-Frame-Options"); got != "DENY" {
				t.Errorf("%s: GET %s X-Frame-Options %q", c.name, p, got)
			}
		}
	}
}

// TestFrameOrigin: the origin of console.grafana_embed_url is scheme, host and port of an https
// URL, the host in lower case and the default port dropped; anything else is refused, and the
// error never repeats the value.
func TestFrameOrigin(t *testing.T) {
	t.Parallel()
	good := map[string]string{
		embedURL:                              "https://grafana.example.com",
		"https://grafana.example.com":         "https://grafana.example.com",
		"https://GRAFANA.example.com:443/x":   "https://grafana.example.com",
		"https://grafana.example.com:8443/#x": "https://grafana.example.com:8443",
		"https://[2001:db8::1]:9443/d/ops":    "https://[2001:db8::1]:9443",
	}
	for in, want := range good {
		got, err := httpserver.FrameOrigin(in)
		if err != nil || got != want {
			t.Errorf("FrameOrigin(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"http://grafana.example.com/d/ops", "https://", "grafana.example.com/d/ops",
		"https://alex:pw@grafana.example.com/d/ops", "javascript:alert(1)", "https:opaque", "ftp://grafana.example.com/",
		// Only a DNS name or an IP literal: a wildcard or a CSP delimiter in the host would turn
		// "exactly this origin" into a pattern or another directive (brief M6-17 Design note 1).
		"https://*/d/ops", "https://*.example.com/d/ops", "https://grafana.example.com;sandbox/d/ops",
		"https://grafana.example.com,b.example.net/d/ops", "https://grafana_ops.example.com/d/ops",
		"https://-grafana.example.com/d/ops", "https://grafana..example.com/d/ops"} {
		got, err := httpserver.FrameOrigin(bad)
		if err == nil {
			t.Errorf("FrameOrigin(%q) = %q, want an error", bad, got)
			continue
		}
		if strings.Contains(err.Error(), bad) {
			t.Errorf("FrameOrigin(%q) error repeats the value: %v", bad, err)
		}
	}
	if _, err := httpserver.OpenConsoleAssets(consoleBuild(t), "http://grafana.example.com/d/ops"); err == nil {
		t.Error("OpenConsoleAssets accepted an http embed URL")
	}
}
