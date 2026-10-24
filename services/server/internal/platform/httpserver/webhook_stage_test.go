// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

const (
	hookPath     = "/api/v1/hooks/{hook_id}/{token}"
	hookLimitKey = "webhook_in.max_body_bytes"
)

// hookOp is postHook with its own body limit, as internal/wire builds the incoming webhook
// operations (M2-27).
func hookOp(limit int64) httpserver.Operation {
	op := httpservertest.Op("postHook", http.MethodPost, hookPath, "hook.receive")
	op.Public, op.BodyLimit, op.BodyLimitKey = true, limit, hookLimitKey
	return op
}

// jsonOfSize returns a JSON object of exactly n bytes.
func jsonOfSize(n int) string {
	const frame = `{"x":""}`
	return `{"x":"` + strings.Repeat("y", n-len(frame)) + `"}`
}

// TestBodyLimitOverride (M2-27 design note 1a): Operation.BodyLimit replaces
// http.max_json_body_bytes for its operation only, and a 413 names BodyLimitKey. A declared length
// above the override reaches the stages before StageBody (the webhook token check answers first,
// 11 8.4: token before size) and gets 413 at StageBody; a body without a declared length is still
// never read beyond the limit.
func TestBodyLimitOverride(t *testing.T) {
	t.Parallel()
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.MaxJSONBodyBytes = 64
	hook := hookOp(256)
	create := httpservertest.Op("createThing", http.MethodPost, "/api/v1/things", "thing.create")
	var tokenStage atomic.Int32
	var got string
	h := httpservertest.New(t, httpservertest.Options{
		Config: &cfg,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{
			httpserver.StageWebhookToken: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					tokenStage.Add(1)
					if r.PathValue("token") == "wrong" {
						http.NotFound(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		Routes: []httpservertest.Route{
			{Op: hook, Handler: httpservertest.Authorized(hook.Action, func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				got = string(b)
				w.WriteHeader(http.StatusNoContent)
			})},
			{Op: create, Handler: httpservertest.Authorized(create.Action, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
			})},
		},
	})
	hookURL := func(token string) string { return "/api/v1/hooks/" + thingID + "/" + token }

	// Above http.max_json_body_bytes, within the override: the hook takes it, createThing does not.
	mid := jsonOfSize(128)
	if resp := h.Do(h.NewRequest(http.MethodPost, hookURL("good"), mid)); resp.StatusCode != http.StatusNoContent || got != mid {
		t.Fatalf("hook within its limit: status %d %s", resp.StatusCode, resp.Body)
	}
	p := wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", mid)), 413, "payload_too_large")
	if p["detail"] != "Body exceeds http.max_json_body_bytes." {
		t.Fatalf("createThing detail %v", p["detail"])
	}

	// Above the override with a declared length: the token stage runs first.
	big := jsonOfSize(512)
	before := tokenStage.Load()
	if resp := h.Do(h.NewRequest(http.MethodPost, hookURL("wrong"), big)); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("wrong token with a large body: status %d %s, want 404 (token before size)", resp.StatusCode, resp.Body)
	}
	got = ""
	p = wantProblem(t, h.Do(h.NewRequest(http.MethodPost, hookURL("good"), big)), 413, "payload_too_large")
	if p["detail"] != "Body exceeds "+hookLimitKey+"." {
		t.Fatalf("hook detail %v", p["detail"])
	}
	if n := tokenStage.Load() - before; n != 2 {
		t.Fatalf("the token stage ran %d times for the two large bodies, want 2", n)
	}
	if got != "" {
		t.Fatal("the handler ran for a body above the override")
	}

	// Without a declared length the override bounds the read.
	req := h.NewRequest(http.MethodPost, hookURL("good"), big)
	req.ContentLength = -1
	req.Body = io.NopCloser(strings.NewReader(big))
	p = wantProblem(t, h.Do(req), 413, "payload_too_large")
	if p["detail"] != "Body exceeds "+hookLimitKey+"." || got != "" {
		t.Fatalf("streamed: detail %v, handler got %d bytes", p["detail"], len(got))
	}
}

// countingAuth counts its calls and authenticates like httpservertest.FakeAuthenticator.
type countingAuth struct{ calls *atomic.Int32 }

func (a countingAuth) Authenticate(ctx context.Context, c actor.Credentials) (actor.Actor, error) {
	a.calls.Add(1)
	return httpservertest.FakeAuthenticator{}.Authenticate(ctx, c)
}

func (countingAuth) Revalidate(context.Context, actor.Actor) error { return nil }

// TestAuth_AnonymousCredentials (M2-27 design note 1b): on an operation with AnonymousCredentials
// the Authorization header is not an identity credential, so neither the precheck of rl.public_auth
// nor the Authenticator sees it and the actor stays anonymous; other operations still authenticate.
func TestAuth_AnonymousCredentials(t *testing.T) {
	t.Parallel()
	hook := hookOp(0)
	hook.AnonymousCredentials = true
	get := getThing()
	var calls, prechecks atomic.Int32
	var seen actor.Actor
	h := httpservertest.New(t, httpservertest.Options{
		Authenticator: countingAuth{calls: &calls},
		Setup: func(c *httpserver.Chain) {
			c.SetCredentialsPrecheck(func(*http.Request) (func(error), error) {
				prechecks.Add(1)
				return nil, nil
			})
		},
		Routes: []httpservertest.Route{
			{Op: hook, Handler: httpservertest.Authorized(hook.Action, func(w http.ResponseWriter, r *http.Request) {
				seen, _ = actor.FromContext(r.Context())
				w.WriteHeader(http.StatusNoContent)
			})},
			{Op: get, Handler: okHandler(get.Action)},
		},
	})
	garbage := "Bearer " + strings.Repeat("g", 24)
	resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/hooks/"+thingID+"/tok", `{}`, "Authorization", garbage))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("hook: status %d %s", resp.StatusCode, resp.Body)
	}
	if calls.Load() != 0 || prechecks.Load() != 0 || !seen.IsAnonymous() {
		t.Fatalf("hook: authenticator ran %d times, precheck %d times, actor kind %q; want 0, 0, anonymous", calls.Load(), prechecks.Load(), seen.Kind)
	}
	if resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", garbage)); resp.StatusCode != http.StatusUnauthorized || calls.Load() != 1 || prechecks.Load() != 1 {
		t.Fatalf("other operation: status %d, authenticator %d, precheck %d", resp.StatusCode, calls.Load(), prechecks.Load())
	}
}

// TestStageAuthenticate_OwnCredentialSkipsAuthenticator (M3-22 design note 12 b): on an operation
// with OwnCredential neither the precheck nor the Authenticator runs, the actor is anonymous and
// actor.CredentialsFrom returns method, htu, Authorization, DPoP and the client IP; any other
// operation calls the Authenticator once.
func TestStageAuthenticate_OwnCredentialSkipsAuthenticator(t *testing.T) {
	t.Parallel()
	preview := httpservertest.Op("getProof", http.MethodGet, "/api/v1/proofs", "thing.read")
	preview.OwnCredential = true
	get := getThing()
	var calls, prechecks atomic.Int32
	var seen actor.Actor
	var creds actor.Credentials
	var found bool
	h := httpservertest.New(t, httpservertest.Options{
		Authenticator: countingAuth{calls: &calls},
		Setup: func(c *httpserver.Chain) {
			c.SetCredentialsPrecheck(func(*http.Request) (func(error), error) {
				prechecks.Add(1)
				return nil, nil
			})
		},
		Routes: []httpservertest.Route{
			{Op: preview, Handler: httpservertest.Authorized(preview.Action, func(w http.ResponseWriter, r *http.Request) {
				seen, _ = actor.FromContext(r.Context())
				creds, found = actor.CredentialsFrom(r.Context())
				w.WriteHeader(http.StatusNoContent)
			})},
			{Op: get, Handler: okHandler(get.Action)},
		},
	})
	auth := "DPoP " + strings.Repeat("p", 24)
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/proofs", "", "Authorization", auth, "DPoP", "proof", "X-Mode", "fast"))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preview: status %d %s", resp.StatusCode, resp.Body)
	}
	if calls.Load() != 0 || prechecks.Load() != 0 || !seen.IsAnonymous() {
		t.Fatalf("preview: authenticator %d, precheck %d, actor %q; want 0, 0, anonymous", calls.Load(), prechecks.Load(), seen.Kind)
	}
	if !found || creds.Method != http.MethodGet || creds.Authorization != auth || creds.DPoP != "proof" ||
		creds.HTU != httpservertest.PublicURL+"/api/v1/proofs" || !creds.ClientIP.IsValid() {
		t.Fatalf("credentials %v: method %q, htu %q, dpop %q, ip %v", found, creds.Method, creds.HTU, creds.DPoP, creds.ClientIP)
	}
	if resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", auth)); resp.StatusCode != http.StatusUnauthorized || calls.Load() != 1 {
		t.Fatalf("other operation: status %d, authenticator %d", resp.StatusCode, calls.Load())
	}
}
