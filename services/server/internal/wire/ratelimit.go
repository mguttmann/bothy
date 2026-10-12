// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
)

// RateLimitConfig reads the configuration of the rate limiter from v: per_minute and burst of
// every class of ratelimit.Classes(), ratelimit.max_tracked_keys and ratelimit.sweep_interval. The
// schema bounds each value, so the error only reports values loaded with another schema.
func RateLimitConfig(v *config.Values) (cfg ratelimit.Config, err error) {
	defer func() {
		// Values panics on a key the schema does not have; a caller with a foreign schema gets an
		// error instead.
		if p := recover(); p != nil {
			err = fmt.Errorf("wire: rate-limit configuration: %v", p)
		}
	}()
	cfg = ratelimit.Config{
		Classes:        make(map[ratelimit.Class]ratelimit.Limits, len(ratelimit.Classes())),
		MaxTrackedKeys: int(v.Int(keyMaxTrackedKeys)),
		SweepInterval:  v.Duration(keySweepInterval),
	}
	for _, c := range ratelimit.Classes() {
		cfg.Classes[c] = ratelimit.Limits{PerMinute: int(v.Int(perMinuteKey(c))), Burst: int(v.Int(burstKey(c)))}
	}
	return cfg, nil
}

// checkRateLimitRoutes is the start check of M1-19: every class of the catalogue has limits in
// cfg, and every enabled REST operation has a class with limits that a REST request can key:
// rl.typing and rl.gateway_connect belong to the gateway, and rl.webhook_in needs the path
// parameter webhook_id. The error names every operation that fails.
func checkRateLimitRoutes(ops []routes.Operation, cfg ratelimit.Config) error {
	var problems []error
	for _, c := range ratelimit.Classes() {
		if _, ok := cfg.Classes[c]; !ok {
			problems = append(problems, fmt.Errorf("rate-limit class %s of the catalogue has no limits", c))
		}
	}
	for _, op := range ops {
		class := ratelimit.Class(op.RateLimitClass)
		_, configured := cfg.Classes[class]
		kind, known := ratelimit.KindOf(class)
		switch {
		case !known || !configured:
			problems = append(problems, fmt.Errorf("operation %s has the rate-limit class %q, which has no limits", op.ID, class))
		case ratelimit.GatewayClass(class):
			problems = append(problems, fmt.Errorf("operation %s has the rate-limit class %s, which only the gateway counts", op.ID, class))
		case kind == ratelimit.KeyIncomingWebhook && !strings.Contains(op.Path, "{"+ratelimit.PathWebhookID+"}"):
			problems = append(problems, fmt.Errorf("operation %s has the rate-limit class %s but no path parameter %s", op.ID, class, ratelimit.PathWebhookID))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("rate-limit check failed: %w", errors.Join(problems...))
	}
	return nil
}

// InstallRateLimits is the rate-limit part of start step 7 (roles api and all): it reads the
// configuration from v, runs the start check over the enabled operations ops, builds the limiter
// on clk (nil means clock.System()) with bothy.ratelimit.rejected on the global meter provider of
// platform/otel (call it after otel.Setup) and installs it in chain (ratelimit.Install). The
// caller starts the sweeper (Limiter.Run) and stops it on shutdown. Call it before the routes are
// mounted. The error is the failed check or an
// invalid value; the start ends with exit code 1.
func InstallRateLimits(chain *httpserver.Chain, v *config.Values, ops []routes.Operation, clk clock.Clock) (*ratelimit.Limiter, error) {
	cfg, err := RateLimitConfig(v)
	if err != nil {
		return nil, err
	}
	if err := checkRateLimitRoutes(ops, cfg); err != nil {
		return nil, err
	}
	l, err := ratelimit.New(cfg, clk, nil)
	if err != nil {
		return nil, err
	}
	ratelimit.Install(chain, l)
	return l, nil
}

// startSweeper starts l.Run in its own goroutine, independent of the start context (lifecycle:
// long-running work must not depend on it), and returns the stop step that ends it and waits for
// it. The step runs in PhaseHTTP, after the public listener closed, because only requests use
// the buckets; running it twice is harmless.
func startSweeper(l *ratelimit.Limiter) lifecycle.StopStep {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		l.Run(ctx)
	}()
	return lifecycle.StopStep{Name: "ratelimit_sweeper", Phase: lifecycle.PhaseHTTP, Run: func(stopCtx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stopCtx.Done():
			return stopCtx.Err()
		}
	}}
}
