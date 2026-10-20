// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// uuidText matches the canonical lowercase UUID form of a correlation ID.
var uuidText = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const (
	thingID   = "0192f4a0-7b1c-7577-8bf0-c4f4ee4f887e"
	thingPath = "/api/v1/things/{thing_id}"
)

// getThing is the test operation GET /api/v1/things/{thing_id} with action thing.read.
func getThing() httpserver.Operation {
	return httpservertest.Op("getThing", http.MethodGet, thingPath, "thing.read")
}

// okHandler checks the action and answers 200 with a small JSON body.
func okHandler(action string) http.HandlerFunc {
	return httpservertest.Authorized(action, func(w http.ResponseWriter, _ *http.Request) {
		httpservertest.JSON(w, http.StatusOK, map[string]any{"ok": true})
	})
}

func newChain(t *testing.T, ids ...string) *httpserver.Chain {
	t.Helper()
	v, err := httpserver.NewValidator(httpservertest.Contract, ids)
	if err != nil {
		t.Fatal(err)
	}
	c, err := httpserver.NewChain(httpserver.DefaultChainConfig(httpservertest.PublicURL),
		httpserver.Deps{Authenticator: actor.AnonymousOnly{}, Validator: v})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestChainOrder_Golden holds the order of the steps against testdata/chain_order.golden, the
// order of 02 "Kette im api-Knoten": the documented order (Order) and the order a request
// actually enters the stages.
func TestChainOrder_Golden(t *testing.T) {
	t.Parallel()
	golden, err := os.ReadFile("testdata/chain_order.golden")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Fields(string(golden))
	c := newChain(t, "getThing")
	var entered []string
	httpserver.SetTrace(c, func(name string) { entered = append(entered, name) })
	for _, s := range []httpserver.Stage{httpserver.StageRateLimit, httpserver.StageWebhookToken, httpserver.StageMaintenance, httpserver.StageIdempotency} {
		c.Use(s, func(next http.Handler) http.Handler { return next })
	}
	mux := http.NewServeMux()
	op := getThing()
	mux.Handle(op.Pattern, c.Wrap(op, okHandler(op.Action)))
	h := c.Public(mux)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/things/"+thingID, nil)
	req.Header.Set("X-Mode", "fast")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if got := c.Order(); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("Order() = %v\nwant      %v", got, want)
	}
	if strings.Join(entered, " ") != strings.Join(want, " ") {
		t.Errorf("entered %v\nwant    %v", entered, want)
	}
}

func TestUsePanics(t *testing.T) {
	t.Parallel()
	pass := func(next http.Handler) http.Handler { return next }
	cases := map[string]func(c *httpserver.Chain){
		"own stage":   func(c *httpserver.Chain) { c.Use(httpserver.StageCORS, pass) },
		"route stage": func(c *httpserver.Chain) { c.Use(httpserver.StageRoute, pass) },
		"twice": func(c *httpserver.Chain) {
			c.Use(httpserver.StageRateLimit, pass)
			c.Use(httpserver.StageRateLimit, pass)
		},
		"after Wrap": func(c *httpserver.Chain) {
			c.Wrap(getThing(), http.NotFoundHandler())
			c.Use(httpserver.StageIdempotency, pass)
		},
		"out of range": func(c *httpserver.Chain) { c.Use(httpserver.Stage(99), pass) },
		"hook twice": func(c *httpserver.Chain) {
			f := func(_ *http.Request, err error) error { return err }
			c.SetFailedCredentialsHook(f)
			c.SetFailedCredentialsHook(f)
		},
		"nil middleware": func(c *httpserver.Chain) { c.Use(httpserver.StageMaintenance, nil) },
		"precheck twice": func(c *httpserver.Chain) {
			f := func(*http.Request) (func(error), error) { return nil, nil }
			c.SetCredentialsPrecheck(f)
			c.SetCredentialsPrecheck(f)
		},
		"precheck after Wrap": func(c *httpserver.Chain) {
			c.Wrap(getThing(), http.NotFoundHandler())
			c.SetCredentialsPrecheck(func(*http.Request) (func(error), error) { return nil, nil })
		},
		"nil precheck": func(c *httpserver.Chain) { c.SetCredentialsPrecheck(nil) },
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recover() == nil {
					t.Error("no panic")
				}
			}()
			f(newChain(t, "getThing"))
		})
	}
}

func TestNewChainRejectsBadConfig(t *testing.T) {
	t.Parallel()
	v, err := httpserver.NewValidator(httpservertest.Contract, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := httpserver.DefaultChainConfig("https://chat.example.com/path")
	cfg.CORSAllowedOrigins = []string{"https://app.example.com/"}
	cfg.MaxJSONDepth = 0
	cfg.HSTSMaxAge = 1500 * time.Millisecond
	_, err = httpserver.NewChain(cfg, httpserver.Deps{})
	for _, want := range []string{"server.public_url", "api.cors_allowed_origins", "JSON depth", "http.hsts_max_age", "Authenticator", "Validator"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error %v does not name %s", err, want)
		}
	}
	if _, err := httpserver.NewChain(httpserver.DefaultChainConfig(httpservertest.PublicURL), httpserver.Deps{Authenticator: actor.AnonymousOnly{}, Validator: v}); err != nil {
		t.Errorf("valid config: %v", err)
	}
}

func TestCheckOrigin(t *testing.T) {
	t.Parallel()
	good := []string{"https://chat.example.com", "http://127.0.0.1:8080", "https://[2001:db8::1]:8443"}
	bad := []string{"", "chat.example.com", "ftp://example.com", "https://example.com/", "https://example.com/api",
		"https://example.com?x", "https://example.com#f", "https://user@example.com", "HTTPS://example.com", "https://"}
	for _, s := range good {
		if err := httpserver.CheckOrigin(s); err != nil {
			t.Errorf("CheckOrigin(%q) = %v", s, err)
		}
	}
	for _, s := range bad {
		if err := httpserver.CheckOrigin(s); err == nil {
			t.Errorf("CheckOrigin(%q) = nil, want an error", s)
		}
	}
}

func TestRecover_PanicGives500Problem(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: func(http.ResponseWriter, *http.Request) { panic("boom") }}}})
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast"))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d", resp.StatusCode)
	}
	p := resp.Problem(t)
	if p["code"] != "internal_error" || p["correlation_id"] != resp.Header.Get("Correlation-Id") {
		t.Fatalf("problem %v, header %q", p, resp.Header.Get("Correlation-Id"))
	}
	if strings.Contains(string(resp.Body), "boom") {
		t.Fatalf("panic value leaked: %s", resp.Body)
	}
	rec, ok := h.Logs.Find(httpserver.LogPanicRecovered)
	if !ok || rec.Attrs["bothy.correlation_id"] != p["correlation_id"] || !strings.Contains(rec.Attrs["code.function.name"], "TestRecover_PanicGives500Problem") {
		t.Fatalf("log record %+v", rec)
	}
}

func TestRecover_ErrAbortHandlerRepanics(t *testing.T) {
	t.Parallel()
	c := newChain(t, "getThing")
	mux := http.NewServeMux()
	op := getThing()
	mux.Handle(op.Pattern, c.Wrap(op, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })))
	h := c.Public(mux)
	defer func() {
		if p := recover(); p != http.ErrAbortHandler { //nolint:errorlint // identity, as net/http compares it
			t.Fatalf("recovered %v, want http.ErrAbortHandler", p)
		}
	}()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/things/"+thingID, nil)
	req.Header.Set("X-Mode", "fast")
	h.ServeHTTP(httptest.NewRecorder(), req)
	t.Fatal("no panic")
}

// TestCorrelationID_OnEveryResponseKind checks 200, 204, 400, 404 unmatched, 405, 413 and 500 from
// a panic: the header equals correlation_id of the body and the trace ID of the server span.
func TestCorrelationID_OnEveryResponseKind(t *testing.T) {
	t.Parallel()
	get := getThing()
	post := httpservertest.Op("createThing", http.MethodPost, "/api/v1/things", "thing.create")
	hook := httpservertest.Op("postHook", http.MethodPost, "/api/v1/hooks/{hook_id}/{token}", "hook.receive")
	hook.RequestBody, hook.Public = httpserver.BodyJSON, true
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.MaxJSONBodyBytes = 64
	h := httpservertest.New(t, httpservertest.Options{Config: &cfg, Routes: []httpservertest.Route{
		{Op: get, Handler: okHandler(get.Action)},
		{Op: post, Handler: func(http.ResponseWriter, *http.Request) { panic("boom") }},
		{Op: hook, Handler: httpservertest.Authorized(hook.Action, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })},
	}})
	cases := []struct {
		name   string
		req    *http.Request
		status int
	}{
		{"200", h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast"), 200},
		{"204", h.NewRequest(http.MethodPost, "/api/v1/hooks/"+thingID+"/s3cret", `{}`), 204},
		{"400", h.NewRequest(http.MethodGet, "/api/v1/things/not-a-uuid", "", "X-Mode", "fast"), 400},
		{"404", h.NewRequest(http.MethodGet, "/api/v1/nowhere", ""), 404},
		{"405", h.NewRequest(http.MethodDelete, "/api/v1/things/"+thingID, ""), 405},
		{"413", h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"`+strings.Repeat("a", 100)+`","color":"red"}`), 413},
		{"500", h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red"}`), 500},
	}
	for _, tc := range cases {
		resp := h.Do(tc.req)
		id := resp.Header.Get("Correlation-Id")
		if resp.StatusCode != tc.status || !uuidText.MatchString(id) {
			t.Errorf("%s: status %d, Correlation-Id %q", tc.name, resp.StatusCode, id)
			continue
		}
		if resp.StatusCode >= 400 {
			if got := resp.Problem(t)["correlation_id"]; got != id {
				t.Errorf("%s: body correlation_id %v, header %s", tc.name, got, id)
			}
		}
		if !spanWithTraceID(id) {
			t.Errorf("%s: no server span with trace ID %s", tc.name, id)
		}
	}
}

// spanWithTraceID reports whether a recorded span has the trace ID written as the UUID id.
func spanWithTraceID(id string) bool {
	want := strings.ReplaceAll(id, "-", "")
	for _, s := range httpservertest.Spans().Ended() {
		if s.SpanContext().TraceID().String() == want {
			return true
		}
	}
	return false
}

func TestCorrelationID_GeneratedWithoutTracer(t *testing.T) {
	t.Parallel()
	c := newChain(t)
	var got string
	h := c.CorrelateOnly(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = reqmeta.CorrelationID(r.Context())
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if trace.SpanContextFromContext(req.Context()).IsValid() {
		t.Fatal("the request must have no span")
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	first := got
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !uuidText.MatchString(first) || first == got {
		t.Fatalf("correlation IDs %q and %q: want two random UUIDs", first, got)
	}
}

func TestCorrelationID_ClientHeaderIgnored(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}}})
	const forged = "4bf92f35-77b3-4da6-a3ce-929d0e0e4736"
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast",
		"Correlation-Id", forged, "traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"))
	if got := resp.Header.Get("Correlation-Id"); got == forged || !uuidText.MatchString(got) {
		t.Fatalf("Correlation-Id %q: the client chose it", got)
	}
}

func TestUnmatched_404Problem(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}}})
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/hooks/x/s3cret?q=1", ""))
	p := resp.Problem(t)
	if resp.StatusCode != 404 || p["code"] != "not_found" || p["type"] != "/api/v1/problems/not_found" || p["title"] != "Not found" {
		t.Fatalf("%d %v", resp.StatusCode, p)
	}
	if _, ok := p["instance"]; ok || strings.Contains(string(resp.Body), "s3cret") {
		t.Fatalf("unmatched problem must not echo the path: %s", resp.Body)
	}
}

func TestUnmatched_405ProblemWithAllow(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}}})
	resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things/"+thingID, `{}`))
	p := resp.Problem(t)
	if resp.StatusCode != 405 || p["code"] != "method_not_allowed" {
		t.Fatalf("%d %v", resp.StatusCode, p)
	}
	if allow := resp.Header.Get("Allow"); !strings.Contains(allow, "GET") {
		t.Fatalf("Allow = %q", allow)
	}
}

func TestAuth_AnonymousWithoutHeader(t *testing.T) {
	t.Parallel()
	op := getThing()
	var got actor.Actor
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			got, _ = actor.FromContext(r.Context())
			w.WriteHeader(http.StatusNoContent)
		})}}})
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast"))
	if resp.StatusCode != http.StatusNoContent || !got.IsAnonymous() {
		t.Fatalf("status %d, actor %+v", resp.StatusCode, got)
	}
	resp = h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", httpservertest.GoodToken))
	if resp.StatusCode != http.StatusNoContent || got.ID != httpservertest.FakeUserID {
		t.Fatalf("with a good token: status %d, actor %+v", resp.StatusCode, got)
	}
}

func TestAuth_AnonymousOnlyRejectsAuthorization401(t *testing.T) {
	t.Parallel()
	op := getThing()
	reached := false
	h := httpservertest.New(t, httpservertest.Options{Authenticator: actor.AnonymousOnly{}, Routes: []httpservertest.Route{{Op: op,
		Handler: func(http.ResponseWriter, *http.Request) { reached = true }}}})
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", "Bearer anything"))
	if resp.StatusCode != 401 || resp.Problem(t)["code"] != "unauthenticated" || resp.Header.Get("WWW-Authenticate") == "" || reached {
		t.Fatalf("status %d, WWW-Authenticate %q, handler reached %v", resp.StatusCode, resp.Header.Get("WWW-Authenticate"), reached)
	}
}

func TestAuth_HtuFromPublicURL(t *testing.T) {
	t.Parallel()
	op := getThing()
	var creds actor.Credentials
	h := httpservertest.New(t, httpservertest.Options{
		Authenticator: httpservertest.FakeAuthenticator{LastCredentials: &creds},
		Routes:        []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}},
	})
	req := h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID+"?limit=5", "", "X-Mode", "fast", "DPoP", "proof")
	req.Host = "evil.example.net"
	if resp := h.Do(req); resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	want := httpservertest.PublicURL + "/api/v1/things/" + thingID
	if creds.HTU != want || creds.Method != http.MethodGet || creds.DPoP != "proof" || !creds.ClientIP.IsValid() {
		t.Fatalf("credentials %+v, want htu %s", creds, want)
	}
}

func TestAuth_FailedCredentialsHook(t *testing.T) {
	t.Parallel()
	op := getThing()
	var seen error
	c := newChain(t, "getThing")
	c.SetFailedCredentialsHook(func(_ *http.Request, err error) error {
		seen = err
		return errs.New(errs.CodeRateLimited, "").WithRetryAfter(7e9)
	})
	mux := http.NewServeMux()
	mux.Handle(op.Pattern, c.Wrap(op, okHandler(op.Action)))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/things/"+thingID, nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	c.Public(mux).ServeHTTP(rec, req)
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "7" || errs.KindOf(seen) != errs.KindUnauthenticated {
		t.Fatalf("status %d, Retry-After %q, hook saw %v", rec.Code, rec.Header().Get("Retry-After"), seen)
	}
}

// TestAuth_CredentialsPrecheck: the precheck runs only for requests with credentials, before the
// Authenticator; its error is the answer and the Authenticator never runs. When it passes, its
// settle function gets the result of the Authenticator (nil on success), so a reservation taken
// by the precheck can be returned.
func TestAuth_CredentialsPrecheck(t *testing.T) {
	t.Parallel()
	op := getThing()
	var creds actor.Credentials
	var prechecked int
	var settled []string
	h := httpservertest.New(t, httpservertest.Options{
		Authenticator: httpservertest.FakeAuthenticator{LastCredentials: &creds},
		Routes:        []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}},
		Setup: func(c *httpserver.Chain) {
			c.SetCredentialsPrecheck(func(r *http.Request) (func(error), error) {
				prechecked++
				if r.Header.Get("Authorization") == "Bearer blocked" {
					return func(error) { t.Error("settle called after a refused precheck") },
						errs.New(errs.CodeRateLimited, "").WithRetryAfter(3e9)
				}
				return func(err error) {
					code := "ok"
					if e, ok := errs.As(err); ok {
						code = string(e.PublicCode())
					}
					settled = append(settled, code)
				}, nil
			})
		},
	})
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", "Bearer blocked"))
	if resp.StatusCode != 429 || resp.Header.Get("Retry-After") != "3" || creds.Authorization != "" {
		t.Fatalf("blocked: status %d, Retry-After %q, authenticator saw %q", resp.StatusCode, resp.Header.Get("Retry-After"), creds.Authorization)
	}
	if resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast")); resp.StatusCode != 200 || prechecked != 1 {
		t.Fatalf("anonymous: status %d, precheck ran %d times", resp.StatusCode, prechecked)
	}
	if resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", httpservertest.GoodToken)); resp.StatusCode != 200 || prechecked != 2 {
		t.Fatalf("good token: status %d, precheck ran %d times", resp.StatusCode, prechecked)
	}
	if resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", "Bearer wrong")); resp.StatusCode != 401 || prechecked != 3 {
		t.Fatalf("bad token: status %d, precheck ran %d times", resp.StatusCode, prechecked)
	}
	if strings.Join(settled, ",") != "ok,unauthenticated" {
		t.Fatalf("settle saw %v, want [ok unauthenticated]", settled)
	}
}

// TestAuth_InactiveCredentialSettlesAsFailure (M1-44, red team round 2): an API token the
// Authenticator found revoked or expired passes the stage to authorize, but its settle function
// sees 401 unauthenticated, so the reservation of rl.public_auth stays spent like for any other
// dead credential.
func TestAuth_InactiveCredentialSettlesAsFailure(t *testing.T) {
	t.Parallel()
	op := getThing()
	var settled []string
	h := httpservertest.New(t, httpservertest.Options{
		Authenticator: inactiveAuth{id: ids.New()},
		Routes:        []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}},
		Setup: func(c *httpserver.Chain) {
			c.SetCredentialsPrecheck(func(*http.Request) (func(error), error) {
				return func(err error) {
					code := "ok"
					if e, ok := errs.As(err); ok {
						code = string(e.PublicCode())
					}
					settled = append(settled, code)
				}, nil
			})
		},
	})
	h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", "Bearer dead"))
	if strings.Join(settled, ",") != "unauthenticated" {
		t.Fatalf("settle saw %v, want [unauthenticated]", settled)
	}
}

func TestOperationFrom(t *testing.T) {
	t.Parallel()
	if _, ok := httpserver.OperationFrom(context.Background()); ok {
		t.Fatal("no operation outside the chain")
	}
	op := getThing()
	var got httpserver.Operation
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			got, _ = httpserver.OperationFrom(r.Context())
			w.WriteHeader(204)
		})}}})
	h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast"))
	if got.ID != op.ID {
		t.Fatalf("OperationFrom = %+v", got)
	}
}

// nonceAuthenticator sets a response header through the context and accepts every request.
type nonceAuthenticator struct{ creds *actor.Credentials }

func (n nonceAuthenticator) Authenticate(ctx context.Context, c actor.Credentials) (actor.Actor, error) {
	*n.creds = c
	reqmeta.SetResponseHeader(ctx, "DPoP-Nonce", "fresh-nonce")
	return actor.Anonymous(), nil
}

func (nonceAuthenticator) Revalidate(context.Context, actor.Actor) error { return nil }

// TestAuth_ProofsAndResponseHeaders (M1-34): the Authenticator sees every DPoP value, the handler
// finds the DPoP facts in its context, and a header the Authenticator sets through the context
// reaches the response.
func TestAuth_ProofsAndResponseHeaders(t *testing.T) {
	t.Parallel()
	op := getThing()
	var creds actor.Credentials
	var proof reqmeta.Proof
	h := httpservertest.New(t, httpservertest.Options{
		Authenticator: nonceAuthenticator{creds: &creds},
		Routes: []httpservertest.Route{{Op: op, Handler: func(w http.ResponseWriter, r *http.Request) {
			proof, _ = reqmeta.ProofOf(r.Context())
			okHandler(op.Action)(w, r)
		}}},
	})
	req := h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "DPoP", "p1")
	req.Header.Add("DPoP", "p2")
	resp := h.Do(req)
	want := httpservertest.PublicURL + "/api/v1/things/" + thingID
	if len(creds.DPoPProofs) != 2 || creds.DPoPProofs[1] != "p2" || creds.DPoP != "p1" {
		t.Fatalf("credentials proofs %q, dpop %q", creds.DPoPProofs, creds.DPoP)
	}
	if proof.HTU != want || proof.Method != http.MethodGet || len(proof.Values) != 2 {
		t.Fatalf("proof in the handler %+v", proof)
	}
	if resp.Header.Get("DPoP-Nonce") != "fresh-nonce" {
		t.Fatalf("DPoP-Nonce %q", resp.Header.Get("DPoP-Nonce"))
	}
}

// TestAuth_TransportForIdentityHeaders (M1-43): the Authenticator and the handler find the TCP
// peer (never a forwarded address), the request header and the route's action in the context.
func TestAuth_TransportForIdentityHeaders(t *testing.T) {
	t.Parallel()
	op := getThing()
	var inAuth, inHandler reqmeta.Transport
	h := httpservertest.New(t, httpservertest.Options{
		Authenticator: transportAuthenticator{seen: &inAuth},
		Routes: []httpservertest.Route{{Op: op, Handler: func(w http.ResponseWriter, r *http.Request) {
			inHandler, _ = reqmeta.TransportOf(r.Context())
			okHandler(op.Action)(w, r)
		}}},
	})
	req := h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "X-Forwarded-For", "192.0.2.9",
		"Tailscale-User-Login", "alex@example.com")
	if resp := h.Do(req); resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	for name, tr := range map[string]reqmeta.Transport{"authenticator": inAuth, "handler": inHandler} {
		if tr.Peer != netip.MustParseAddr("127.0.0.1") || tr.Action != op.Action {
			t.Errorf("%s: transport %+v, want peer 127.0.0.1 and action %s", name, tr, op.Action)
		}
		if v := tr.Values("Tailscale-User-Login"); len(v) != 1 || v[0] != "alex@example.com" {
			t.Errorf("%s: header values %q", name, v)
		}
	}
}

// transportAuthenticator records the transport of the request and accepts every request.
type transportAuthenticator struct{ seen *reqmeta.Transport }

func (a transportAuthenticator) Authenticate(ctx context.Context, _ actor.Credentials) (actor.Actor, error) {
	*a.seen, _ = reqmeta.TransportOf(ctx)
	return actor.Anonymous(), nil
}

func (transportAuthenticator) Revalidate(context.Context, actor.Actor) error { return nil }
