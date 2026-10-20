// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"log/slog"
	"net/http"
	stdsync "sync"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	appshttp "github.com/OWNER/bothy/services/server/internal/apps/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
)

// InstallWebhookToken installs the token check of the incoming webhooks (apps, M2-27 design note
// 2) at httpserver.StageWebhookToken of chain: after rl.webhook_in and before any body stage, with
// the failed-credential guard of rl.public_auth of limiter. Call it once per chain, before the
// routes are mounted. Errors: a missing apps module or limiter.
func InstallWebhookToken(chain *httpserver.Chain, ap *Apps, limiter *ratelimit.Limiter) error {
	if ap == nil || ap.Incoming == nil || limiter == nil {
		return errors.New("wire: the webhook token check needs the apps module and the rate limiter")
	}
	chain.Use(httpserver.StageWebhookToken, appshttp.TokenStage(ap.Incoming, limiter.FailedCredentialGuard))
	return nil
}

// webhookPostRoutes returns the routes of the two post operations of the incoming webhooks among
// rs (action webhook_in.post).
func webhookPostRoutes(rs []routes.Route) []routes.Route {
	var out []routes.Route
	for _, r := range rs {
		if r.Operation.Action == actionWebhookInPost {
			out = append(out, r)
		}
	}
	return out
}

// webhookPostOperations returns the enabled operations with action webhook_in.post.
func webhookPostOperations() []routes.Operation {
	var out []routes.Operation
	for _, op := range routes.Enabled() {
		if op.Action == actionWebhookInPost {
			out = append(out, op)
		}
	}
	return out
}

// webhookPostOperationIDs returns the operation ids of webhookPostOperations.
func webhookPostOperationIDs() []string {
	var ids []string
	for _, op := range webhookPostOperations() {
		ids = append(ids, op.ID)
	}
	return ids
}

// internalHooksValidator compiles the request validator of the two webhook post operations once
// per process. The internal chain serves nothing else, so it does not need the validator of every
// enabled operation, whose compile takes seconds under load and would delay the readiness of a
// worker role, which mounts no public API.
var internalHooksValidator = stdsync.OnceValues(func() (*httpserver.Validator, error) {
	return httpserver.NewValidator(rest.Spec, webhookPostOperationIDs())
})

// mountInternalHooks mounts the two post operations of the incoming webhooks on the internal
// listener, in every role (16 section 4.2, 02 section 3.2, M2-27 design note 12): senders inside
// the stack (Alertmanager) reach the same routes there. They run through a chain built for this
// listener with the stages of the public one: the trusted proxies are empty, so the client IP is
// the peer address and forwarded headers are never read (R-SE-09); its own rate limiter counts
// rl.webhook_in and rl.public_auth for this listener. Nothing else of the API is mounted there.
func (s *server) mountInternalHooks(cat routes.Catalog, log *slog.Logger) error {
	s.mu.Lock()
	ap, mux := s.apps, s.internalMux
	s.mu.Unlock()
	if ap == nil || mux == nil {
		return errors.New("wire: the internal webhook routes need the apps module and the internal listener")
	}
	cfg := s.chainCfg
	cfg.TrustedProxies, cfg.CORSAllowedOrigins = nil, nil
	v, err := internalHooksValidator()
	if err != nil {
		return err
	}
	chain, err := httpserver.NewChain(cfg, httpserver.Deps{Logger: log, Authenticator: actor.AnonymousOnly{}, Validator: v})
	if err != nil {
		return err
	}
	limiter, err := InstallRateLimits(chain, s.values, webhookPostOperations(), s.deps.Clock)
	if err != nil {
		return err
	}
	s.runner.OnStop(startSweeper(limiter))
	if err := InstallWebhookToken(chain, ap, limiter); err != nil {
		return err
	}
	rs, err := routes.Build(s.handlers(), RouteOptions(), cat)
	if err != nil {
		return err
	}
	hooks := http.NewServeMux()
	wrapped := wrapRoutes(chain, webhookPostRoutes(rs), WebhookBodyLimit(s.values.Int(keyWebhookInMaxBodyBytes)))
	if err := mount(hooks, wrapped); err != nil {
		return err
	}
	public := chain.Public(hooks)
	for i := range wrapped {
		wrapped[i].Handler = public
	}
	return mount(mux, wrapped)
}
