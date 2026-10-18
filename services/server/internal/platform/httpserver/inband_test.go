// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// countingAuthenticator counts its calls and refuses every credential.
type countingAuthenticator struct {
	actor.AnonymousOnly
	calls int
}

// Authenticate counts the call and answers like AnonymousOnly.
func (a *countingAuthenticator) Authenticate(ctx context.Context, c actor.Credentials) (actor.Actor, error) {
	a.calls++
	return a.AnonymousOnly.Authenticate(ctx, c)
}

// TestWrap_AuthorizedInBand (M1-64): an operation that authorizes in-band (the gateway upgrade)
// runs only the outer steps: no per-route stage, so no authentication of the upgrade request, no
// body limit or deadline and no proof of an authorize check for its response; the operation is
// in the context. The same handler on a normal operation is refused for the missing proof.
func TestWrap_AuthorizedInBand(t *testing.T) {
	t.Parallel()
	auth := &countingAuthenticator{}
	v, err := httpserver.NewValidator(httpservertest.Contract, []string{"getThing"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := httpserver.NewChain(httpserver.DefaultChainConfig(httpservertest.PublicURL), httpserver.Deps{Authenticator: auth, Validator: v})
	if err != nil {
		t.Fatal(err)
	}
	inBand := httpserver.Operation{ID: "gateway", Method: http.MethodGet, Path: "/api/v1/gateway", Pattern: "GET /api/v1/gateway",
		Action: "gateway.connect", RateLimitClass: "rl.gateway_connect", AuthorizedInBand: true}
	var seen httpserver.Operation
	unchecked := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = httpserver.OperationFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	mux := http.NewServeMux()
	mux.Handle(inBand.Pattern, c.Wrap(inBand, unchecked))
	normal := getThing()
	mux.Handle(normal.Pattern, c.Wrap(normal, unchecked))
	h := c.Public(mux)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/gateway", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || seen.ID != "gateway" || auth.calls != 0 {
		t.Fatalf("in-band: status %d, operation %q, authenticator calls %d", rec.Code, seen.ID, auth.calls)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/things/"+thingID, nil)
	req.Header.Set("X-Mode", "fast")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("normal operation without proof: status %d, want 500", rec.Code)
	}
}
