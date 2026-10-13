// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// --- CORS (SEC-T04, REST part) ---

func discoveryOp() httpserver.Operation {
	op := httpservertest.Op("getDiscovery", http.MethodGet, "/api/v1/discovery", "instance.discovery.read")
	op.Public = true
	return op
}

func corsHarness(t *testing.T, origins ...string) *httpservertest.Harness {
	t.Helper()
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.CORSAllowedOrigins = origins
	get, disc := getThing(), discoveryOp()
	return httpservertest.New(t, httpservertest.Options{Config: &cfg, Routes: []httpservertest.Route{
		{Op: get, Handler: okHandler(get.Action)},
		{Op: disc, Handler: okHandler(disc.Action)},
	}})
}

func corsHeaders(h http.Header) []string {
	var out []string
	for k := range h {
		if strings.HasPrefix(k, "Access-Control-") {
			out = append(out, k)
		}
	}
	return out
}

func TestSECT04_NoCORSHeadersByDefault(t *testing.T) {
	t.Parallel()
	h := corsHarness(t)
	for _, path := range []string{"/api/v1/things/" + thingID, "/api/v1/nowhere"} {
		resp := h.Do(h.NewRequest(http.MethodGet, path, "", "X-Mode", "fast", "Origin", "https://app.example.com"))
		if got := corsHeaders(resp.Header); len(got) > 0 {
			t.Errorf("%s: CORS headers %v", path, got)
		}
	}
	resp := h.Do(h.NewRequest(http.MethodOptions, "/api/v1/things/"+thingID, "", "Origin", "https://app.example.com",
		"Access-Control-Request-Method", "GET"))
	if got := corsHeaders(resp.Header); len(got) > 0 || resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("preflight without configured origins: %d %v", resp.StatusCode, got)
	}
}

func TestSECT04_DiscoveryWildcardWithoutCredentials(t *testing.T) {
	t.Parallel()
	h := corsHarness(t)
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/discovery", "", "Origin", "https://any.example.org"))
	if resp.StatusCode != 200 || resp.Header.Get("Access-Control-Allow-Origin") != "*" || resp.Header.Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("%d %v", resp.StatusCode, resp.Header)
	}
}

func TestCORS_AllowedOriginEchoAndPreflight(t *testing.T) {
	t.Parallel()
	const origin = "https://app.example.com"
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.CORSAllowedOrigins = []string{origin}
	v, err := httpserver.NewValidator(httpservertest.Contract, []string{"getThing"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := httpserver.NewChain(cfg, httpserver.Deps{Authenticator: httpservertest.FakeAuthenticator{}, Validator: v})
	if err != nil {
		t.Fatal(err)
	}
	if !c.CORSEnabled() {
		t.Fatal("CORSEnabled = false")
	}
	op := getThing()
	mux := http.NewServeMux()
	mux.Handle(op.Pattern, c.Wrap(op, okHandler(op.Action)))
	mux.Handle("OPTIONS "+op.Path, c.Preflight(op.Path, []string{"PATCH", "GET"}))
	srv := c.Public(mux)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/things/"+thingID, nil)
	req.Header.Set("X-Mode", "fast")
	req.Header.Set("Origin", origin)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	hd := rec.Header()
	if rec.Code != 200 || hd.Get("Access-Control-Allow-Origin") != origin || hd.Get("Vary") != "Origin" ||
		!strings.Contains(hd.Get("Access-Control-Expose-Headers"), "Correlation-Id") || hd.Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("simple request: %d %v", rec.Code, hd)
	}
	req = httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/v1/things/"+thingID, nil)
	req.Header.Set("Origin", origin)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	hd = rec.Header()
	if rec.Code != 204 || hd.Get("Access-Control-Allow-Origin") != origin || hd.Get("Access-Control-Allow-Methods") != "GET, PATCH" ||
		hd.Get("Access-Control-Allow-Headers") != "Authorization, DPoP, Content-Type, Idempotency-Key, If-Match" || hd.Get("Vary") != "Origin" {
		t.Fatalf("preflight: %d %v", rec.Code, hd)
	}
}

func TestCORS_OtherOriginNoHeaders(t *testing.T) {
	t.Parallel()
	h := corsHarness(t, "https://app.example.com")
	for _, origin := range []string{"https://evil.example.net", "https://app.example.com.evil.example.net", "https://APP.example.com", "null"} {
		resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Origin", origin))
		if got := corsHeaders(resp.Header); len(got) > 0 {
			t.Errorf("%s: %v", origin, got)
		}
	}
}

// --- client IP (ID-T25, header part) ---

var trusted = []netip.Prefix{netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("2001:db8:ffff::/48")}

func clientIP(peer string, headers ...string) string {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.RemoteAddr = peer
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Add(headers[i], headers[i+1])
	}
	return httpserver.ClientIP(r, trusted).String()
}

func TestClientIP_UntrustedPeerIgnoresForwarding(t *testing.T) {
	t.Parallel()
	if got := clientIP("192.0.2.1:4711", "X-Forwarded-For", "198.51.100.7", "Forwarded", "for=198.51.100.8"); got != "192.0.2.1" {
		t.Fatalf("got %s", got)
	}
}

func TestClientIP_TrustedChainRightToLeft(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		xff  []string
		want string
	}{
		"one hop":           {[]string{"198.51.100.7"}, "198.51.100.7"},
		"skips trusted":     {[]string{"203.0.113.9, 198.51.100.7, 198.18.2.3"}, "198.51.100.7"},
		"spoofed left part": {[]string{"192.0.2.66, 198.51.100.7"}, "198.51.100.7"},
		"several lines":     {[]string{"203.0.113.9", "198.51.100.7, 198.18.0.2"}, "198.51.100.7"},
		"all trusted":       {[]string{"198.18.9.9, 198.18.1.1"}, "198.18.9.9"},
		"none":              {nil, "198.18.0.1"},
	}
	for name, tc := range cases {
		var headers []string
		for _, v := range tc.xff {
			headers = append(headers, "X-Forwarded-For", v)
		}
		if got := clientIP("198.18.0.1:5000", headers...); got != tc.want {
			t.Errorf("%s: got %s, want %s", name, got, tc.want)
		}
	}
}

func TestClientIP_ForwardedPreferredOverXFF(t *testing.T) {
	t.Parallel()
	got := clientIP("198.18.0.1:5000", "X-Forwarded-For", "203.0.113.5",
		"Forwarded", `for=198.51.100.7;proto=https, for="198.18.3.4:80";by=198.18.0.1`)
	if got != "198.51.100.7" {
		t.Fatalf("got %s", got)
	}
	if got := clientIP("198.18.0.1:5000", "Forwarded", `for="[2001:db8::7]:4711"`); got != "2001:db8::7" {
		t.Fatalf("quoted IPv6: got %s", got)
	}
}

func TestClientIP_MalformedStopsAtLastTrusted(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"xff garbage":       {"X-Forwarded-For", "198.51.100.7, garbage, 198.18.4.4"},
		"forwarded unknown": {"Forwarded", "for=198.51.100.7, for=unknown, for=198.18.4.4"},
		"forwarded hidden":  {"Forwarded", "for=198.51.100.7, for=_hidden, for=198.18.4.4"},
		"no for":            {"Forwarded", "for=198.51.100.7, proto=https, for=198.18.4.4"},
	}
	for name, headers := range cases {
		if got := clientIP("198.18.0.1:5000", headers...); got != "198.18.4.4" {
			t.Errorf("%s: got %s, want 198.18.4.4", name, got)
		}
	}
	if got := clientIP("198.18.0.1:5000", "X-Forwarded-For", "garbage"); got != "198.18.0.1" {
		t.Errorf("malformed first hop: got %s, want the peer", got)
	}
}

func TestClientIP_IPv6AndMapped(t *testing.T) {
	t.Parallel()
	if got := clientIP("[::ffff:192.0.2.1]:4711"); got != "192.0.2.1" {
		t.Errorf("mapped peer: %s", got)
	}
	if got := clientIP("[2001:db8:ffff::1]:443", "X-Forwarded-For", "::ffff:198.51.100.7"); got != "198.51.100.7" {
		t.Errorf("mapped hop behind trusted IPv6 peer: %s", got)
	}
	if got := clientIP("[2001:db8:ffff::1]:443", "X-Forwarded-For", "[2001:db8::9]:1234"); got != "2001:db8::9" {
		t.Errorf("bracketed IPv6 hop: %s", got)
	}
	if got := clientIP("[fe80::1%eth0]:443"); got != "fe80::1" {
		t.Errorf("zone: %s", got)
	}
}

func TestClientIP_InContext(t *testing.T) {
	t.Parallel()
	op := getThing()
	var got string
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			ip, _ := reqmetaClientIP(r)
			got = ip
			w.WriteHeader(204)
		})}}})
	// The harness peer is 127.0.0.1, trusted by default, so the forwarding header counts.
	h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "X-Forwarded-For", "198.51.100.7"))
	if got != "198.51.100.7" {
		t.Fatalf("client IP %q", got)
	}
	for _, r := range h.Logs.Records() {
		for _, v := range r.Attrs {
			if strings.Contains(v, "198.51.100.7") {
				t.Fatalf("client IP in the log: %+v", r)
			}
		}
	}
}

// --- deadlines ---

func TestDeadline_ExceededGives503(t *testing.T) {
	t.Parallel()
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.RequestTimeout = 20 * time.Millisecond
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Config: &cfg, Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
			httpserver.WriteProblem(w, r, r.Context().Err())
		})}}})
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast"))
	if resp.StatusCode != 503 || resp.Problem(t)["code"] != "unavailable" || resp.Header.Get("Retry-After") != "5" {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	rec, ok := h.Logs.Find(httpserver.LogRequestFailed)
	if !ok || rec.Attrs["bothy.reason"] != "deadline_exceeded" {
		t.Fatalf("log %+v", rec)
	}
}

func TestTransferDeadline_UsesTransferTimeout(t *testing.T) {
	t.Parallel()
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.RequestTimeout, cfg.TransferTimeout = time.Second, time.Hour
	blob := httpservertest.Op("uploadBlob", http.MethodPost, "/api/v1/blobs", "blob.upload")
	blob.RequestBody = httpserver.BodyOctetStream
	// A download (a success that streams a body that is not JSON) has no request body but the
	// deadline of transfers too (02 section 14.2, M1-50).
	export := httpservertest.Op("exportThings", http.MethodGet, "/api/v1/things-export", "thing.read")
	export.Download = true
	get := getThing()
	left := map[string]time.Duration{}
	measure := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			d, _ := r.Context().Deadline()
			left[name] = time.Until(d)
			w.WriteHeader(204)
		}
	}
	h := httpservertest.New(t, httpservertest.Options{Config: &cfg, Routes: []httpservertest.Route{
		{Op: blob, Handler: httpservertest.Authorized(blob.Action, measure("blob"))},
		{Op: get, Handler: httpservertest.Authorized(get.Action, measure("get"))},
		{Op: export, Handler: httpservertest.Authorized(export.Action, measure("export"))},
	}})
	h.Do(h.NewRequest(http.MethodPost, "/api/v1/blobs", "raw bytes", "Content-Type", "application/octet-stream"))
	h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast"))
	h.Do(h.NewRequest(http.MethodGet, "/api/v1/things-export", ""))
	if left["blob"] < 59*time.Minute || left["get"] > time.Second || left["export"] < 59*time.Minute {
		t.Fatalf("deadlines %v", left)
	}
}
