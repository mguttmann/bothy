// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/api/routes/routestest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// checkingBuild builds the fixture with stub and records an allowed check of each route's action
// before the generated handler runs, as an Authorizer would.
func checkingBuild(stub routestest.Stub) routeBuilder {
	return func(opts routes.Options, cat routes.Catalog) ([]routes.Route, error) {
		rs, err := routestest.BuildWith(stub, opts, cat)
		for i, r := range rs {
			action, next := r.Operation.Action, r.Handler
			rs[i].Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				authorize.RecordCheck(req.Context(), authorize.Check{Action: action, Kind: authorize.CheckAuthorize, Allowed: true})
				next.ServeHTTP(w, req)
			})
		}
		return rs, err
	}
}

// serveFixture mounts the fixture through the chain on a real mux and returns the public handler.
func serveFixture(t *testing.T, build routeBuilder, chain *httpserver.Chain) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	if err := mountRoutes(RoleAPI, mux, build, routestest.Catalog(), chain, slog.New(&captureHandler{})); err != nil {
		t.Fatal(err)
	}
	return chain.Public(mux)
}

func do(h http.Handler, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func problemCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("not a problem: %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
	var p struct{ Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Code
}

// TestMountRoutes_ChainAroundGeneratedHandlers: every mounted route runs through the chain; the
// generated fixture handlers answer through it, and its problem writers are the error handlers.
func TestMountRoutes_ChainAroundGeneratedHandlers(t *testing.T) {
	h := serveFixture(t, checkingBuild(routestest.Stub{}), testChain(t))
	rec := do(h, http.MethodGet, "/api/v1/things/"+routestest.ThingID, "")
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Correlation-Id") == "" {
		t.Fatalf("getThing: %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
	if code := problemCode(t, do(h, http.MethodPost, "/api/v1/things", `{"name":"a","bogus":1}`)); code != "validation_failed" {
		t.Errorf("unknown member: %s", code)
	}
	if code := problemCode(t, do(h, http.MethodPatch, "/api/v1/things/"+routestest.ThingID, `{"name":"b"}`)); code != "precondition_required" {
		t.Errorf("missing If-Match: %s", code)
	}
	if rec := do(h, http.MethodPatch, "/api/v1/things/"+routestest.ThingID, `{"name":"b"}`, "If-Match", `"7"`); rec.Code != 200 {
		t.Errorf("updateThing: %d %s", rec.Code, rec.Body)
	}
	if code := problemCode(t, do(h, http.MethodGet, "/api/v1/things/"+routestest.ThingID, "", "Authorization", "Bearer x")); code != "unauthenticated" {
		t.Errorf("credentials with AnonymousOnly: %s", code)
	}
	if code := problemCode(t, do(h, http.MethodDelete, "/api/v1/things/"+routestest.ThingID, "")); code != "method_not_allowed" {
		t.Errorf("unmatched method: %s", code)
	}
	failing := serveFixture(t, checkingBuild(routestest.Stub{Err: errs.New(errs.CodeConflict, "").WithReason("one_time_secret")}), testChain(t))
	if code := problemCode(t, do(failing, http.MethodGet, "/api/v1/things/"+routestest.ThingID, "")); code != "conflict" {
		t.Errorf("handler error through ResponseErrorHandlerFunc: %s", code)
	}
}

// TestMountRoutes_HandlerWithoutAuthorizeGives500: the generated stub never authorizes, so the
// proof of the chain replaces its success by 500 internal_error (02 9.3).
func TestMountRoutes_HandlerWithoutAuthorizeGives500(t *testing.T) {
	h := serveFixture(t, fixtureBuild, testChain(t))
	rec := do(h, http.MethodGet, "/api/v1/things/"+routestest.ThingID, "")
	if rec.Code != 500 || problemCode(t, rec) != "internal_error" || strings.Contains(rec.Body.String(), routestest.ThingID+`","name`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestMountRoutes_NoChain(t *testing.T) {
	err := mountRoutes(RoleAPI, &recordingMux{}, fixtureBuild, routestest.Catalog(), nil, slog.New(&captureHandler{}))
	if err == nil || !strings.Contains(err.Error(), "chain") {
		t.Fatalf("error = %v", err)
	}
	if err := mountRoutes(RoleWorker, &recordingMux{}, fixtureBuild, nil, nil, nil); err != nil {
		t.Fatalf("worker needs no chain: %v", err)
	}
}

// TestMountRoutes_PreflightWithCORSOrigins: with api.cors_allowed_origins the start step mounts
// OPTIONS <path> for every enabled path with that path's methods; without, none.
func TestMountRoutes_PreflightWithCORSOrigins(t *testing.T) {
	cfg := httpserver.DefaultChainConfig("https://chat.example.com")
	cfg.CORSAllowedOrigins = []string{"https://app.example.com"}
	var ids []string
	for _, op := range routestest.Operations() {
		ids = append(ids, op.ID)
	}
	v, err := httpserver.NewValidator(routestest.Spec(), ids)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := httpserver.NewChain(cfg, httpserver.Deps{Authenticator: actor.AnonymousOnly{}, Validator: v, Logger: slog.New(&captureHandler{})})
	if err != nil {
		t.Fatal(err)
	}
	mux := &recordingMux{}
	if err := mountRoutes(RoleAPI, mux, fixtureBuild, routestest.Catalog(), chain, slog.New(&captureHandler{})); err != nil {
		t.Fatal(err)
	}
	var preflights []string
	for _, p := range mux.patterns {
		if strings.HasPrefix(p, "OPTIONS ") {
			preflights = append(preflights, p)
		}
	}
	if strings.Join(preflights, "|") != "OPTIONS /api/v1/things|OPTIONS /api/v1/things/{thing_id}" {
		t.Fatalf("preflights %q", preflights)
	}
	h := serveFixture(t, fixtureBuild, chain)
	rec := do(h, http.MethodOptions, "/api/v1/things/"+routestest.ThingID, "", "Origin", "https://app.example.com")
	if rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Methods") != "GET, PATCH" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	plain := &recordingMux{}
	if err := mountRoutes(RoleAPI, plain, fixtureBuild, routestest.Catalog(), testChain(t), nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range plain.patterns {
		if strings.HasPrefix(p, "OPTIONS ") {
			t.Errorf("preflight %s without CORS origins", p)
		}
	}
}

func TestHTTPOperationCarriesEveryField(t *testing.T) {
	for _, op := range routestest.Operations() {
		h := HTTPOperation(op)
		if h.ID != op.ID || h.Method != op.Method || h.Path != op.Path || h.Pattern != op.Pattern || h.Action != op.Action || h.RateLimitClass != op.RateLimitClass ||
			h.Public != op.Public || h.IdempotencyKey != op.IdempotencyKey || int(h.IfMatch) != int(op.IfMatch) ||
			int(h.RequestBody) != int(op.RequestBody) || h.BodyRequired != op.BodyRequired || !slices.Equal(h.ETagStatuses, op.ETagStatuses) {
			t.Errorf("%+v -> %+v", op, h)
		}
	}
	// The enum values of both packages must line up, since the conversion is numeric.
	if int(routes.IfMatchRequired) != int(httpserver.IfMatchRequired) || int(routes.BodyOctetStream) != int(httpserver.BodyOctetStream) ||
		int(routes.BodyMultipart) != int(httpserver.BodyMultipart) || int(routes.IfMatchOptional) != int(httpserver.IfMatchOptional) {
		t.Fatal("enum values of routes and httpserver differ")
	}
}
