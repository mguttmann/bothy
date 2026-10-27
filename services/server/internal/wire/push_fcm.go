// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// fcmValues are the keys of the FCM adapter as config.Bind fills them (13 17).
type fcmValues struct {
	ProjectID         string        `config:"push.fcm.project_id"`
	ServiceAccountRef string        `config:"push.fcm.service_account_ref"`
	TTL               time.Duration `config:"push.ttl"`
	PauseFor          time.Duration `config:"push.retry_cap"`
}

// FCMConfig builds the configuration of the FCM adapter from the loaded values: push.fcm.*,
// push.ttl and push.retry_cap as the length of the pause (brief M3-21 Design note 2).
//
// The error reports a value that Load would have refused; with values from Load it does not
// occur.
func FCMConfig(v *config.Values) (fcm.Config, error) {
	var raw fcmValues
	if err := config.Bind(v, &raw); err != nil {
		return fcm.Config{}, err
	}
	return fcm.Config(raw), nil
}

// FCMEndpoints are the test-only base URLs of the FCM adapter (fcm.Endpoints): the send base and
// the token URL of a Fake-FCM. In-process tests and the E2E server build of M5-11 (the Go build
// tag e2e of cmd/bothy) set them through Deps.FCMEndpoints; the zero value means Google's
// endpoints, as in production.
type FCMEndpoints = fcm.Endpoints

// FCMDeps are the collaborators of the FCM adapter from the process.
type FCMDeps struct {
	// Egress is egress.* (EgressConfig); EgressDeps the resolver, logger and metrics of the
	// process's outgoing clients.
	Egress     egress.Config
	EgressDeps egress.Deps
	// Secrets resolves push.fcm.service_account_ref.
	Secrets fcm.SecretResolver
	// Clock is the time source; nil means clock.System().
	Clock clock.Clock
	// Logger receives the records of the adapter.
	Logger *slog.Logger
	// Endpoints replace Google's endpoints (Deps.FCMEndpoints); zero in production.
	Endpoints FCMEndpoints
}

// FCMProviders returns the FCM adapter as the push providers of role: none when FCM is not
// enabled (no secret is read; jobs for fcm registrations end with no_provider) or in the role
// api, which runs no notify.push_send; the adapter in worker and all. d.Endpoints is zero in
// production, so the adapter talks to Google only.
//
// Errors: a service account that cannot be resolved or parsed and an egress configuration that
// egress.New refuses stop the start; the text names push.fcm.service_account_ref, never a value.
func FCMProviders(ctx context.Context, role Role, cfg fcm.Config, d FCMDeps) ([]notifyapp.PushProvider, error) {
	if !cfg.Enabled() || (role != RoleWorker && role != RoleAll) {
		return nil, nil
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	a, err := fcm.New(ctx, cfg, fcm.Deps{Egress: d.Egress, EgressDeps: d.EgressDeps, Secrets: d.Secrets, Clock: d.Clock, Logger: d.Logger,
		Endpoints: d.Endpoints})
	if err != nil {
		return nil, err
	}
	return []notifyapp.PushProvider{a}, nil
}
