// SPDX-License-Identifier: Apache-2.0

package ratelimit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
)

const (
	thingID   = "0192f4a0-7b1c-7577-8bf0-c4f4ee4f8871"
	missingID = "0192f4a0-7b1c-7577-8bf0-c4f4ee4f8872"
)

// getThing is the test operation GET /api/v1/things/{thing_id} with class.
func getThing(class string) httpserver.Operation {
	op := httpservertest.Op("getThing", http.MethodGet, "/api/v1/things/{thing_id}", "thing.read")
	op.RateLimitClass = class
	return op
}

// getDiscovery is the public test operation GET /api/v1/discovery in rl.public_auth.
func getDiscovery() httpserver.Operation {
	op := httpservertest.Op("getDiscovery", http.MethodGet, "/api/v1/discovery", "instance.discovery.read")
	op.Public, op.RateLimitClass = true, "rl.public_auth"
	return op
}

// countingHandler answers 200 for thingID and 404 not_found for every other ID, after recording
// the check of action, and counts its calls.
func countingHandler(action string, calls *atomic.Int64) http.HandlerFunc {
	return httpservertest.Authorized(action, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if id := r.PathValue("thing_id"); id != "" && id != thingID {
			httpserver.WriteProblem(w, r, errs.New(errs.CodeNotFound, ""))
			return
		}
		httpservertest.JSON(w, http.StatusOK, map[string]any{"ok": true})
	})
}

// harness starts the chain with the limiter installed and the given routes.
func harness(t *testing.T, l *ratelimit.Limiter, opts httpservertest.Options) *httpservertest.Harness {
	t.Helper()
	opts.Setup = func(c *httpserver.Chain) { ratelimit.Install(c, l) }
	return httpservertest.New(t, opts)
}

func getThingReq(h *httpservertest.Harness, id string, headers ...string) *http.Request {
	return h.NewRequest(http.MethodGet, "/api/v1/things/"+id, "", append([]string{"X-Mode", "fast"}, headers...)...)
}

func TestHTTP_429ProblemWithRetryAfter(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.read": {PerMinute: 20, Burst: 2}}))
	var calls atomic.Int64
	op := getThing("rl.read")
	h := harness(t, l, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}}})
	for range 2 {
		if resp := h.Do(getThingReq(h, thingID, "Authorization", httpservertest.GoodToken)); resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
		}
	}
	resp := h.Do(getThingReq(h, thingID, "Authorization", httpservertest.GoodToken))
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", resp.StatusCode)
	}
	p := resp.Problem(t)
	if p["code"] != "rate_limited" || p["status"] != float64(429) || p["correlation_id"] == "" || p["correlation_id"] != resp.Header.Get("Correlation-Id") {
		t.Fatalf("problem %v", p)
	}
	want := map[string]string{
		"Retry-After":            "3",
		"RateLimit-Policy":       `"rl.read";q=20;w=60`,
		"RateLimit":              `"rl.read";r=0;t=6`,
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
		"Content-Type":           "application/problem+json",
	}
	for k, v := range want {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if resp.Header.Get("Cross-Origin-Resource-Policy") == "" || resp.Header.Get("Server") != "" {
		t.Errorf("headers of SEC-HDR: %v", resp.Header)
	}
	if calls.Load() != 2 {
		t.Errorf("handler ran %d times, want 2", calls.Load())
	}
}

func TestHTTP_HeadersOnSuccessAndError(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(nil))
	var calls atomic.Int64
	op := getThing("rl.read")
	h := harness(t, l, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}}})
	ok := h.Do(getThingReq(h, thingID, "Authorization", httpservertest.GoodToken))
	notFound := h.Do(getThingReq(h, missingID, "Authorization", httpservertest.GoodToken))
	if ok.StatusCode != http.StatusOK || notFound.StatusCode != http.StatusNotFound {
		t.Fatalf("statuses %d and %d", ok.StatusCode, notFound.StatusCode)
	}
	for name, resp := range map[string]*httpservertest.Response{"200": ok, "404": notFound} {
		if got := resp.Header.Get("RateLimit-Policy"); got != `"rl.read";q=600;w=60` {
			t.Errorf("%s: RateLimit-Policy %q", name, got)
		}
	}
	if got := ok.Header.Get("RateLimit"); got != `"rl.read";r=99;t=1` {
		t.Errorf("200: RateLimit %q", got)
	}
	if got := notFound.Header.Get("RateLimit"); got != `"rl.read";r=98;t=1` {
		t.Errorf("404: RateLimit %q", got)
	}
	// A validation error after the stage carries the headers too.
	bad := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "Authorization", httpservertest.GoodToken))
	if bad.StatusCode != http.StatusBadRequest || bad.Header.Get("RateLimit") == "" {
		t.Errorf("400: status %d, RateLimit %q", bad.StatusCode, bad.Header.Get("RateLimit"))
	}
}

func TestHTTP_RateLimitBeforeAuthorize(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.read": {PerMinute: 60, Burst: 1}}))
	var calls atomic.Int64
	op := getThing("rl.read")
	h := harness(t, l, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}}})
	h.Do(getThingReq(h, thingID, "Authorization", httpservertest.GoodToken))
	foreign := h.Do(getThingReq(h, thingID, "Authorization", httpservertest.GoodToken))
	missing := h.Do(getThingReq(h, missingID, "Authorization", httpservertest.GoodToken))
	if calls.Load() != 1 {
		t.Fatalf("handler ran %d times, want only the first request", calls.Load())
	}
	pf, pm := foreign.Problem(t), missing.Problem(t)
	for _, k := range []string{"type", "title", "status", "code", "detail"} {
		if pf[k] != pm[k] {
			t.Errorf("%s differs: %v and %v", k, pf[k], pm[k])
		}
	}
	for _, k := range []string{"Retry-After", "RateLimit", "RateLimit-Policy", "Content-Length"} {
		if foreign.Header.Get(k) != missing.Header.Get(k) {
			t.Errorf("%s differs: %q and %q", k, foreign.Header.Get(k), missing.Header.Get(k))
		}
	}
	if foreign.StatusCode != http.StatusTooManyRequests || missing.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("statuses %d and %d", foreign.StatusCode, missing.StatusCode)
	}
}

func TestHTTP_UnknownClassFailsClosed(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(nil))
	var calls atomic.Int64
	for _, class := range []string{"", "rl.nope", "rl.typing", "rl.gateway_connect"} {
		op := getThing(class)
		h := harness(t, l, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}}})
		if resp := h.Do(getThingReq(h, thingID, "Authorization", httpservertest.GoodToken)); resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("class %q: status %d, want 500", class, resp.StatusCode)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("handler ran %d times", calls.Load())
	}
}

// forwarded returns the headers of a request that the trusted test peer forwards for client.
func forwarded(client string) []string { return []string{"X-Forwarded-For", client} }

func TestPublicAuth_IPv6KeyedBySlash64(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.public_auth": {PerMinute: 60, Burst: 2}}))
	var calls atomic.Int64
	op := getDiscovery()
	h := harness(t, l, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}}})
	status := func(client string) int {
		return h.Do(h.NewRequest(http.MethodGet, "/api/v1/discovery", "", forwarded(client)...)).StatusCode
	}
	if status("2001:db8:1:2::1") != 200 || status("2001:db8:1:2:ffff::9") != 200 {
		t.Fatal("first two requests of one /64 refused")
	}
	if got := status("2001:db8:1:2::3"); got != http.StatusTooManyRequests {
		t.Fatalf("third address of the same /64: %d, want 429", got)
	}
	if got := status("2001:db8:1:3::1"); got != http.StatusOK {
		t.Fatalf("another /64: %d, want 200", got)
	}
	if status("192.0.2.7") != 200 || status("::ffff:192.0.2.7") != 200 {
		t.Fatal("IPv4 requests refused")
	}
	if got := status("192.0.2.7"); got != http.StatusTooManyRequests {
		t.Fatalf("IPv4 and its mapped form use separate buckets: %d", got)
	}
}

func TestIDT25_SpoofedForwardedForCountsPeer(t *testing.T) {
	t.Parallel()
	for _, trusted := range []bool{false, true} {
		l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.public_auth": {PerMinute: 60, Burst: 3}}))
		var calls atomic.Int64
		op := getDiscovery()
		cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
		if !trusted {
			cfg.TrustedProxies = nil
		}
		h := harness(t, l, httpservertest.Options{Config: &cfg, Routes: []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}}})
		var limited int
		for i := range 6 {
			client := netip.AddrFrom4([4]byte{198, 51, 100, byte(i + 1)}).String()
			if h.Do(h.NewRequest(http.MethodGet, "/api/v1/discovery", "", forwarded(client)...)).StatusCode == http.StatusTooManyRequests {
				limited++
			}
		}
		if trusted && limited != 0 {
			t.Errorf("trusted peer: %d of 6 forwarded clients limited, want 0", limited)
		}
		if !trusted && limited != 3 {
			t.Errorf("untrusted peer: %d of 6 spoofed requests limited, want 3 after the burst", limited)
		}
	}
}

// countingAuth is FakeAuthenticator with a call counter and the 401 codes token_expired and
// use_dpop_nonce for the tokens "Bearer expired" and "Bearer nonce".
type countingAuth struct {
	calls *atomic.Int64
}

func (a countingAuth) Authenticate(ctx context.Context, c actor.Credentials) (actor.Actor, error) {
	if c.Present() {
		a.calls.Add(1)
	}
	switch c.Authorization {
	case "Bearer expired":
		return actor.Actor{}, errs.New(errs.CodeTokenExpired, "")
	case "Bearer nonce":
		return actor.Actor{}, errs.New(errs.CodeUseDPoPNonce, "").WithHeader("DPoP-Nonce", "n1")
	case "Bearer broken":
		return actor.Actor{}, errs.New(errs.CodeUnavailable, "")
	}
	return httpservertest.FakeAuthenticator{}.Authenticate(ctx, c)
}

func (countingAuth) Revalidate(context.Context, actor.Actor) error { return nil }

func TestFailedCredentials_CountAgainstPublicAuth(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.public_auth": {PerMinute: 60, Burst: 3}}))
	var calls, verified atomic.Int64
	op := getThing("rl.read")
	h := harness(t, l, httpservertest.Options{
		Authenticator: countingAuth{calls: &verified},
		Routes:        []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}},
	})
	do := func(token, client string) *httpservertest.Response {
		return h.Do(getThingReq(h, thingID, append([]string{"Authorization", token}, forwarded(client)...)...))
	}
	// Normal flows and server errors do not count.
	for _, token := range []string{"Bearer expired", "Bearer nonce", "Bearer broken", "Bearer expired", "Bearer nonce"} {
		if resp := do(token, "192.0.2.10"); resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("%s counted against rl.public_auth", token)
		}
	}
	for i := range 3 {
		if resp := do("Bearer bad", "192.0.2.10"); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("failure %d: status %d, want 401", i+1, resp.StatusCode)
		}
	}
	before := verified.Load()
	for _, token := range []string{"Bearer bad", httpservertest.GoodToken} {
		resp := do(token, "192.0.2.10")
		if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "1" {
			t.Fatalf("%s after the bucket ran empty: status %d, Retry-After %q", token, resp.StatusCode, resp.Header.Get("Retry-After"))
		}
		if p := resp.Problem(t); p["code"] != "rate_limited" {
			t.Fatalf("problem %v", p)
		}
		if got := resp.Header.Get("RateLimit"); !strings.HasPrefix(got, `"rl.public_auth";r=0;`) {
			t.Fatalf("RateLimit %q", got)
		}
	}
	if verified.Load() != before {
		t.Fatalf("the authenticator ran %d times while rl.public_auth was empty", verified.Load()-before)
	}
	// Anonymous requests and another client IP are not blocked by the failures.
	if resp := do("Bearer bad", "192.0.2.11"); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("another client IP: %d", resp.StatusCode)
	}
	if resp := h.Do(getThingReq(h, thingID, forwarded("192.0.2.10")...)); resp.StatusCode != http.StatusOK {
		t.Fatalf("anonymous request from the blocked IP: %d", resp.StatusCode)
	}
	if calls.Load() != 1 {
		t.Fatalf("handler ran %d times", calls.Load())
	}
}

// keyRequest returns a request with the client IP, the actor (when a is not nil) and the path
// value webhook_id as the chain would have set them.
func keyRequest(ip string, a *actor.Actor, webhook string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ctx := reqmeta.WithClientIP(r.Context(), netip.MustParseAddr(ip))
	if a != nil {
		ctx = actor.WithActor(ctx, *a)
	}
	r = r.WithContext(ctx)
	if webhook != "" {
		r.SetPathValue("webhook_id", webhook)
	}
	return r
}

func TestKeys_PerClass(t *testing.T) {
	t.Parallel()
	user := actor.Actor{ID: ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f887e"), Kind: actor.KindUser}
	agent := actor.Actor{ID: ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f887f"), Kind: actor.KindAgent}
	host := actor.Actor{ID: ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f8880"), Kind: actor.KindHost}
	for _, row := range chapterClasses(t) {
		kind, ok := ratelimit.KindOf(row.class)
		if !ok {
			t.Errorf("%s: unknown class", row.class)
			continue
		}
		var want, got string
		var err error
		switch kind {
		case ratelimit.KeyClientIP:
			want = "ip:2001:db8:1:2::/64"
			got, err = ratelimit.RequestKey(keyRequest("2001:db8:1:2::5", &user, ""), row.class)
		case ratelimit.KeyPrincipal:
			want = "p:" + user.ID.String()
			got, err = ratelimit.RequestKey(keyRequest("192.0.2.1", &user, ""), row.class)
		case ratelimit.KeyAgent:
			want = "a:" + agent.ID.String()
			got, err = ratelimit.RequestKey(keyRequest("192.0.2.1", &agent, ""), row.class)
		case ratelimit.KeyHost:
			want = "h:" + host.ID.String()
			got, err = ratelimit.RequestKey(keyRequest("192.0.2.1", &host, ""), row.class)
		case ratelimit.KeyIncomingWebhook:
			want = "w:0192f4a0-7b1c-7000-8a00-000000000901"
			got, err = ratelimit.RequestKey(keyRequest("192.0.2.1", &actor.Actor{Kind: actor.KindAnonymous}, "0192f4a0-7b1c-7000-8a00-000000000901"), row.class)
		case ratelimit.KeyConnection, ratelimit.KeyDeviceOrToken:
			// Gateway classes: a REST request has no key for them.
			if _, err := ratelimit.RequestKey(keyRequest("192.0.2.1", &user, ""), row.class); err == nil {
				t.Errorf("%s: a REST request got a key for a gateway class", row.class)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("%s (%s): key %q, %v; want %q", row.class, row.key, got, err, want)
		}
	}
}

func TestKeys_AnonymousUsesClientIP(t *testing.T) {
	t.Parallel()
	anon := actor.Anonymous()
	for _, class := range []ratelimit.Class{"rl.read", "rl.write", "rl.agent_runtime", "rl.host", "rl.terminal"} {
		for _, a := range []*actor.Actor{&anon, nil} {
			got, err := ratelimit.RequestKey(keyRequest("2001:db8:7:8::1", a, ""), class)
			if err != nil || got != "ip:2001:db8:7:8::/64" {
				t.Errorf("%s anonymous (%v): %q, %v", class, a != nil, got, err)
			}
		}
	}
	// The fallback never shares a bucket with a principal: the prefixes differ.
	user := actor.Actor{ID: ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f887e"), Kind: actor.KindUser}
	k, _ := ratelimit.RequestKey(keyRequest("192.0.2.1", &user, ""), "rl.read")
	if strings.HasPrefix(k, "ip:") {
		t.Errorf("authenticated key %q", k)
	}
}

func TestKeys_WebhookPathValue(t *testing.T) {
	t.Parallel()
	const hook1, hook2 = "0192f4a0-7b1c-7000-8a00-000000000901", "0192f4a0-7b1c-7000-8a00-000000000902"
	a, _ := ratelimit.RequestKey(keyRequest("192.0.2.1", nil, hook1), "rl.webhook_in")
	b, _ := ratelimit.RequestKey(keyRequest("192.0.2.2", nil, hook1), "rl.webhook_in")
	c, _ := ratelimit.RequestKey(keyRequest("192.0.2.1", nil, hook2), "rl.webhook_in")
	if a != "w:"+hook1 || a != b || a == c {
		t.Errorf("keys %q, %q, %q", a, b, c)
	}
	if _, err := ratelimit.RequestKey(keyRequest("192.0.2.1", nil, ""), "rl.webhook_in"); err == nil {
		t.Error("no webhook_id: no error")
	}
}

// TestKeys_WebhookIDMustBeAnID: webhook_id is an ID (06), and StageRateLimit runs before the path
// is validated. Any other value is 404 not_found without a bucket, so a client cannot fill the
// table with keys of any length (red team M1-19, finding 2).
func TestKeys_WebhookIDMustBeAnID(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"hook-1", strings.Repeat("a", 60000), "0192F4A0-7B1C-7000-8A00-000000000901x"} {
		_, err := ratelimit.RequestKey(keyRequest("192.0.2.1", nil, v), "rl.webhook_in")
		if e, ok := errs.As(err); !ok || e.PublicCode() != errs.CodeNotFound {
			t.Errorf("webhook_id of %d bytes: error %v, want not_found", len(v), err)
		}
	}
	upper, err := ratelimit.RequestKey(keyRequest("192.0.2.1", nil, "0192F4A0-7B1C-7000-8A00-000000000901"), "rl.webhook_in")
	if err == nil && upper != "w:0192f4a0-7b1c-7000-8a00-000000000901" {
		t.Errorf("an ID in another spelling must share the bucket of its canonical form: %q", upper)
	}
}

// TestHTTP_LongWebhookIDCreatesNoBucket: a request with an invalid webhook_id gets 404 and leaves
// the table empty.
func TestHTTP_LongWebhookIDCreatesNoBucket(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(nil))
	var calls atomic.Int64
	op := httpserver.Operation{
		ID: "postHook", Method: http.MethodPost, Path: "/api/v1/hooks/{webhook_id}/{token}",
		Pattern: "POST /api/v1/hooks/{webhook_id}/{token}", Action: "webhook_in.post",
		RateLimitClass: "rl.webhook_in", Public: true,
	}
	h := harness(t, l, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}}})
	resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/hooks/"+strings.Repeat("a", 4000)+"/tok", ""))
	if resp.StatusCode != http.StatusNotFound || calls.Load() != 0 || l.Len() != 0 {
		t.Fatalf("status %d, handler ran %d times, %d buckets", resp.StatusCode, calls.Load(), l.Len())
	}
}

func TestDeviceOrTokenKey(t *testing.T) {
	t.Parallel()
	dev := ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f8881")
	tok := ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f8882")
	if k, err := ratelimit.DeviceOrTokenKey(actor.Actor{Kind: actor.KindUser, DeviceID: dev, TokenID: tok}); err != nil || k != "d:"+dev.String() {
		t.Errorf("device: %q, %v", k, err)
	}
	if k, err := ratelimit.DeviceOrTokenKey(actor.Actor{Kind: actor.KindAgent, TokenID: tok}); err != nil || k != "t:"+tok.String() {
		t.Errorf("token: %q, %v", k, err)
	}
	if _, err := ratelimit.DeviceOrTokenKey(actor.Anonymous()); err == nil {
		t.Error("anonymous actor: no error")
	}
}

// blockingAuth counts verifications and holds every one until release is closed, so concurrent
// requests overlap inside the Authenticator. Every credential fails with 401 except GoodToken.
type blockingAuth struct {
	calls   *atomic.Int64
	release chan struct{}
}

func (a blockingAuth) Authenticate(ctx context.Context, c actor.Credentials) (actor.Actor, error) {
	if c.Present() {
		a.calls.Add(1)
		<-a.release
	}
	return httpservertest.FakeAuthenticator{}.Authenticate(ctx, c)
}

func (blockingAuth) Revalidate(context.Context, actor.Actor) error { return nil }

// TestFailedCredentials_ConcurrentVerificationsBounded: concurrent requests with credentials from
// one client IP cannot all pass the precheck on the same token; at most burst of them are verified
// at once (red team M1-19, finding 1).
func TestFailedCredentials_ConcurrentVerificationsBounded(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.public_auth": {PerMinute: 60, Burst: 3}}))
	var calls, verified atomic.Int64
	release := make(chan struct{})
	op := getThing("rl.read")
	h := harness(t, l, httpservertest.Options{
		Authenticator: blockingAuth{calls: &verified, release: release},
		Routes:        []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}},
	})
	const n = 30
	var limited atomic.Int64
	statuses := make(chan int, n)
	for range n {
		go func() {
			resp := h.Do(getThingReq(h, thingID, append([]string{"Authorization", "Bearer bad"}, forwarded("192.0.2.20")...)...))
			if resp.StatusCode == http.StatusTooManyRequests {
				limited.Add(1)
			}
			statuses <- resp.StatusCode
		}()
	}
	deadline := time.Now().Add(10 * time.Second)
	for verified.Load()+limited.Load() < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	counts := map[int]int{}
	for range n {
		counts[<-statuses]++
	}
	if verified.Load() > 3 || counts[http.StatusUnauthorized] != 3 || counts[http.StatusTooManyRequests] != n-3 {
		t.Fatalf("%d verifications (want at most the burst 3), statuses %v", verified.Load(), counts)
	}
}

// TestFailedCredentials_ValidCredentialsCostNothing: the token the precheck reserves goes back
// when the credentials are valid, so a client with valid credentials never drains rl.public_auth.
func TestFailedCredentials_ValidCredentialsCostNothing(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.public_auth": {PerMinute: 60, Burst: 3}}))
	var calls, verified atomic.Int64
	op := getThing("rl.read")
	h := harness(t, l, httpservertest.Options{
		Authenticator: countingAuth{calls: &verified},
		Routes:        []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}},
	})
	for i := range 10 {
		if resp := h.Do(getThingReq(h, thingID, append([]string{"Authorization", httpservertest.GoodToken}, forwarded("192.0.2.30")...)...)); resp.StatusCode != http.StatusOK {
			t.Fatalf("valid request %d: status %d", i+1, resp.StatusCode)
		}
	}
	d, err := l.Peek("rl.public_auth", ratelimit.KeyForIP(netip.MustParseAddr("192.0.2.30")))
	if err != nil || d.Remaining != 3 {
		t.Fatalf("rl.public_auth after 10 valid requests: %+v, %v; want 3 tokens left", d, err)
	}
}

// panicAuth panics for every credential except GoodToken, like an Authenticator with a bug.
type panicAuth struct{}

func (panicAuth) Authenticate(ctx context.Context, c actor.Credentials) (actor.Actor, error) {
	if c.Present() && c.Authorization != httpservertest.GoodToken {
		panic("authenticator bug")
	}
	return httpservertest.FakeAuthenticator{}.Authenticate(ctx, c)
}

func (panicAuth) Revalidate(context.Context, actor.Actor) error { return nil }

// TestFailedCredentials_PanicReturnsReservation: a panicking Authenticator is a server failure,
// not a failed credential, so the reserved token goes back and the client IP is not locked out.
func TestFailedCredentials_PanicReturnsReservation(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.public_auth": {PerMinute: 60, Burst: 3}}))
	var calls atomic.Int64
	op := getThing("rl.read")
	h := harness(t, l, httpservertest.Options{
		Authenticator: panicAuth{},
		Routes:        []httpservertest.Route{{Op: op, Handler: countingHandler(op.Action, &calls)}},
	})
	for i := range 5 {
		if resp := h.Do(getThingReq(h, thingID, append([]string{"Authorization", "Bearer boom"}, forwarded("192.0.2.40")...)...)); resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("panic %d: status %d, want 500", i+1, resp.StatusCode)
		}
	}
	if resp := h.Do(getThingReq(h, thingID, append([]string{"Authorization", httpservertest.GoodToken}, forwarded("192.0.2.40")...)...)); resp.StatusCode != http.StatusOK {
		t.Fatalf("valid token after 5 panics: status %d, want 200", resp.StatusCode)
	}
}
