// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// apnsValues are the keys push.ttl and push.apns.* as config.Bind fills them (13 17).
type apnsValues struct {
	TeamID               string        `config:"push.apns.team_id"`
	KeyID                string        `config:"push.apns.key_id"`
	PrivateKeyRef        string        `config:"push.apns.private_key_ref"`
	Topics               []string      `config:"push.apns.topics"`
	JWTRefreshInterval   time.Duration `config:"push.apns.jwt_refresh_interval"`
	Retry5xxDelay        time.Duration `config:"push.apns.retry_5xx_delay"`
	TTL                  time.Duration `config:"push.ttl"`
	TimeSensitiveEnabled bool          `config:"push.apns.time_sensitive_enabled"`
}

// APNsConfig binds the configuration of the APNs adapter (brief M3-20 design note 2) from the
// loaded values. The error names a key, never a value.
func APNsConfig(v *config.Values) (apns.Config, error) {
	var raw apnsValues
	if err := config.Bind(v, &raw); err != nil {
		return apns.Config{}, err
	}
	return apns.Config{TeamID: raw.TeamID, KeyID: raw.KeyID, PrivateKeyRef: raw.PrivateKeyRef, Topics: raw.Topics,
		JWTRefreshInterval: raw.JWTRefreshInterval, Retry5xxDelay: raw.Retry5xxDelay, TTL: raw.TTL,
		TimeSensitiveEnabled: raw.TimeSensitiveEnabled}, nil
}

// pushAPNsComplete is the cross-check push_apns_complete (02 13 item 4): team ID, key ID and
// topics together or not at all, the key reference when they are set, and no empty or repeated
// topic (apns.Config.Validate).
func pushAPNsComplete(v *config.Values) error {
	return apns.Config{TeamID: v.String(keyAPNsTeamID), KeyID: v.String(keyAPNsKeyID), Topics: v.Strings(keyAPNsTopics),
		PrivateKeyRef: v.String(keyAPNsPrivateKeyRef)}.Validate()
}

// The keys of the cross-check push_apns_complete.
const (
	keyAPNsTeamID        = "push.apns.team_id"
	keyAPNsKeyID         = "push.apns.key_id"
	keyAPNsTopics        = "push.apns.topics"
	keyAPNsPrivateKeyRef = "push.apns.private_key_ref"
)

// apnsCheckKeys are the keys of the cross-check push_apns_complete.
var apnsCheckKeys = []string{keyAPNsTeamID, keyAPNsKeyID, keyAPNsTopics, keyAPNsPrivateKeyRef}

// APNsSecrets resolves push.apns.private_key_ref; the secret provider of the process implements
// it.
type APNsSecrets = apns.SecretResolver

// APNsDeps are the collaborators of the APNs adapter from the process.
type APNsDeps struct {
	// Secrets is the secret provider of the process.
	Secrets APNsSecrets
	// Egress is egress.* (EgressConfig); EgressDeps the dependencies of the outgoing clients of
	// the process.
	Egress     egress.Config
	EgressDeps egress.Deps
	// Endpoints maps an environment to a base URL (apns.Deps.Endpoints); only in-process tests
	// against Fake-APNs and the E2E server build of M5-11 (Go build tag e2e) set it
	// (Deps.APNsEndpoints), nil means the two Apple hosts.
	Endpoints map[string]string
	// Clock is the time source; nil means clock.System().
	Clock clock.Clock
	// Logger receives the records of the adapter.
	Logger *slog.Logger
}

// APNsProviders returns the push providers of APNs for role: the adapter when APNs is enabled
// and the role runs notify.push_send (worker and all), otherwise none. Without APNs it reads no
// secret. The adapter targets the two Apple hosts unless d.Endpoints (tests and the E2E server
// build only) names others.
//
// Errors: a configuration the adapter refuses, a key that cannot be resolved or parsed (naming
// push.apns.private_key_ref, never a value); they stop the start (fail fast).
func APNsProviders(ctx context.Context, v *config.Values, role Role, d APNsDeps) ([]notifyapp.PushProvider, error) {
	if role != RoleWorker && role != RoleAll {
		return nil, nil
	}
	cfg, err := APNsConfig(v)
	if err != nil || !cfg.Enabled() {
		return nil, err
	}
	clk := d.Clock
	if clk == nil {
		clk = clock.System()
	}
	a, err := apns.New(ctx, cfg, apns.Deps{Egress: d.Egress, EgressDeps: d.EgressDeps, Secrets: d.Secrets, Clock: clk, Logger: d.Logger, Endpoints: d.Endpoints})
	if err != nil {
		return nil, err
	}
	return []notifyapp.PushProvider{a}, nil
}
