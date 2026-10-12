// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/api/routes/routestest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
)

// loadValues loads the server schema with the test environment plus env.
func loadValues(t *testing.T, env ...string) *config.Values {
	t.Helper()
	s, err := ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	v, err := config.Load(s, config.Sources{Env: append([]string{"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com"}, env...)})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return v
}

func TestRateLimitConfig_FromValues(t *testing.T) {
	t.Parallel()
	cfg, err := RateLimitConfig(loadValues(t, "BOTHY_RATELIMIT__RL__WRITE__BURST=7", "BOTHY_RATELIMIT__SWEEP_INTERVAL=5s"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Classes["rl.write"] != (ratelimit.Limits{PerMinute: 120, Burst: 7}) || cfg.Classes["rl.read"] != (ratelimit.Limits{PerMinute: 600, Burst: 100}) {
		t.Errorf("classes %v", cfg.Classes)
	}
	if cfg.MaxTrackedKeys != 100000 || cfg.SweepInterval != 5*time.Second || len(cfg.Classes) != len(ratelimit.Classes()) {
		t.Errorf("config %+v", cfg)
	}
	if _, err := ratelimit.New(cfg, clock.System(), nil); err != nil {
		t.Errorf("the defaults do not build a limiter: %v", err)
	}
}

func fixtureOp(id, class, path string) routes.Operation {
	return routes.Operation{ID: id, Method: http.MethodPost, Path: path, Pattern: "POST " + path, Action: "x.y", RateLimitClass: class}
}

func TestStart_RejectsGatewayClassOnRESTOperation(t *testing.T) {
	t.Parallel()
	cfg, err := RateLimitConfig(loadValues(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := checkRateLimitRoutes(routestest.Operations(), cfg); err != nil {
		t.Fatalf("fixture surface: %v", err)
	}
	cases := map[string]routes.Operation{
		"rl.typing":           fixtureOp("typeIt", "rl.typing", "/api/v1/typing"),
		"rl.gateway_connect":  fixtureOp("connect", "rl.gateway_connect", "/api/v1/connect"),
		"unknown class":       fixtureOp("odd", "rl.nope", "/api/v1/odd"),
		"no class":            fixtureOp("none", "", "/api/v1/none"),
		"webhook without key": fixtureOp("hook", "rl.webhook_in", "/api/v1/hooks/{token}"),
	}
	for name, op := range cases {
		err := checkRateLimitRoutes(append(routestest.Operations(), op), cfg)
		if err == nil || !strings.Contains(err.Error(), op.ID) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := checkRateLimitRoutes([]routes.Operation{fixtureOp("hook", "rl.webhook_in", "/api/v1/hooks/{webhook_id}/{token}")}, cfg); err != nil {
		t.Errorf("webhook with webhook_id: %v", err)
	}
	// A class of the catalogue without limits fails too.
	delete(cfg.Classes, "rl.host")
	if err := checkRateLimitRoutes(nil, cfg); err == nil || !strings.Contains(err.Error(), "rl.host") {
		t.Errorf("missing rl.host: %v", err)
	}
}

// TestInstallRateLimits_ChainLimitsFixtureRoutes: the start step builds the limiter, installs it
// in the chain and the fixture routes carry the headers.
func TestInstallRateLimits_ChainLimitsFixtureRoutes(t *testing.T) {
	t.Parallel()
	chain := testChain(t)
	l, err := InstallRateLimits(chain, loadValues(t, "BOTHY_RATELIMIT__RL__PUBLIC_AUTH__BURST=1"), routestest.Operations(), clock.NewFake(time.Unix(0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if l == nil {
		t.Fatal("no limiter")
	}
	h := serveFixture(t, checkingBuild(routestest.Stub{}), chain)
	var public routes.Operation
	for _, op := range routestest.Operations() {
		if op.RateLimitClass == "rl.public_auth" {
			public = op
		}
	}
	path := strings.ReplaceAll(public.Path, "{thing_id}", routestest.ThingID)
	statuses := []int{}
	for range 2 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), public.Method, path, nil))
		statuses = append(statuses, rec.Code)
		if rec.Header().Get("RateLimit-Policy") != `"rl.public_auth";q=30;w=60` {
			t.Errorf("RateLimit-Policy %q", rec.Header().Get("RateLimit-Policy"))
		}
	}
	if statuses[1] != http.StatusTooManyRequests || statuses[0] == http.StatusTooManyRequests {
		t.Fatalf("statuses %v, want the second refused", statuses)
	}
	if _, err := InstallRateLimits(chain, loadValues(t), []routes.Operation{fixtureOp("typeIt", "rl.typing", "/api/v1/typing")}, nil); err == nil {
		t.Fatal("a gateway class on a REST operation was installed")
	}
}

// TestSweeper_StopsWithItsStopStep: the sweeper runs in its own goroutine from the start and the
// stop step of PhaseHTTP ends it and waits for it.
func TestSweeper_StopsWithItsStopStep(t *testing.T) {
	t.Parallel()
	cfg, err := RateLimitConfig(loadValues(t, "BOTHY_RATELIMIT__SWEEP_INTERVAL=1s"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := ratelimit.New(cfg, clock.System(), nil)
	if err != nil {
		t.Fatal(err)
	}
	stop := startSweeper(l)
	if stop.Phase != lifecycle.PhaseHTTP || stop.Name != "ratelimit_sweeper" {
		t.Fatalf("stop step %+v", stop)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := stop.Run(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// A second stop (a repeated shutdown plan) returns at once.
	if err := stop.Run(ctx); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}
