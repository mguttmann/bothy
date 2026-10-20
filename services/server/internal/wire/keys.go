// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	instancehttp "github.com/OWNER/bothy/services/server/internal/instance/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/keystore"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// CheckSigningKeys is the readiness check that fails while a managed purpose has no active
// signing key (02 "Health", ADR 0066 rule 4).
const CheckSigningKeys = "signing_keys"

// keyStoreKeys are the keys of the key store and of the rotation job.
type keyStoreKeys struct {
	Dir           string        `config:"secrets.signing_key_dir"`
	CheckInterval time.Duration `config:"signing_key.rotation_check_interval"`
}

// SigningKeyConfig is the bound configuration of the signing keys: the values of the use cases,
// the key store directory and the interval of the job instance.key_rotation.
type SigningKeyConfig struct {
	// Keys are the values of app.Keys.
	Keys app.KeysConfig
	// Dir is secrets.signing_key_dir.
	Dir string
	// CheckInterval is signing_key.rotation_check_interval.
	CheckInterval time.Duration
}

// SigningKeysConfig binds the signing key values of v. The error names the key whose binding
// failed, never a value.
func SigningKeysConfig(v *config.Values) (SigningKeyConfig, error) {
	var c SigningKeyConfig
	if err := config.Bind(v, &c.Keys); err != nil {
		return SigningKeyConfig{}, err
	}
	var k keyStoreKeys
	if err := config.Bind(v, &k); err != nil {
		return SigningKeyConfig{}, err
	}
	c.Dir, c.CheckInterval = k.Dir, k.CheckInterval
	return c, nil
}

// SigningKeys is the signing key part of the instance module in a process (M1-32): the use cases
// over the key store and the REST handlers of getJwks and rotateSigningKey.
type SigningKeys struct {
	// Keys is the port instance.Keys and the use case of the job instance.key_rotation.
	Keys *app.Keys
	// Handlers are the REST handlers.
	Handlers *instancehttp.Keys
	repo     *postgres.Repository
}

// NewSigningKeys builds the signing keys over pool, the secret provider sp (whose
// secrets.signing_key_dir is cfg.Dir), the unit of work uow with the audit hook, the Authorizer
// and the recorder. It opens no connection and starts nothing. Errors: a key store directory
// that is not absolute and clean, a missing collaborator.
func NewSigningKeys(cfg SigningKeyConfig, pool *pg.Pool, sp *secrets.Provider, uow txn.UnitOfWork, az authorize.Authorizer,
	rec auditing.Recorder, clk clock.Clock, log *slog.Logger) (*SigningKeys, error) {
	ks, err := keystore.New(sp, cfg.Dir)
	if err != nil {
		return nil, err
	}
	repo := postgres.NewRepository(pool)
	keys, err := app.NewKeys(cfg.Keys, app.KeysDeps{
		Repo: repo, Store: ks, UoW: uow, Authorizer: az, Audit: rec, Clock: clk, Log: orDiscard(log),
	})
	if err != nil {
		return nil, err
	}
	return &SigningKeys{Keys: keys, Handlers: instancehttp.NewKeys(keys, cfg.Keys.JWKSMaxAge), repo: repo}, nil
}

// Ready is the readiness check signing_keys: it fails, naming the purposes, while a managed
// purpose has no active key or its active key cannot sign (unreadable private file).
func (k *SigningKeys) Ready(ctx context.Context) error {
	missing, err := k.Keys.Unready(ctx)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		names := make([]string, len(missing))
		for i, p := range missing {
			names[i] = string(p)
		}
		return errors.New("no usable active signing key for " + strings.Join(names, ", "))
	}
	return nil
}

// startSigningKeys is the first extension point of start step 7 (M1-32, 02 section 15.1): it
// builds the signing keys over the unit of work of the audit step and the Authorizer of the authz
// step, creates the missing active keys (EnsureActive, every role) and registers the readiness
// check signing_keys, and binds the keys to the outgoing webhooks of the apps step and to the
// invocation tokens of the runs module of the chat step. A key store that
// cannot be written ends the start here; the secrets step has already checked owner, mode and
// writability of the directory.
func (s *server) startSigningKeys(ctx context.Context) error {
	cfg, err := SigningKeysConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, ap := s.pool, s.audit, s.authorizer, s.apps
	s.mu.Unlock()
	if a == nil || az == nil {
		return errors.New("wire: the signing keys need the unit of work of audit and the Authorizer")
	}
	keys, err := NewSigningKeys(cfg, pool, s.sp, a.UoW(), az, a.Recorder(), s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	s.reg.AddReadiness(CheckSigningKeys, keys.Ready)
	if err := keys.Keys.EnsureActive(ctx); err != nil {
		return err
	}
	if ap != nil {
		// The outgoing webhooks sign with the webhook_out keys (M2-26).
		ap.BindKeys(keys.Keys)
	}
	s.mu.Lock()
	rn := s.runs
	s.mu.Unlock()
	if rn != nil {
		// The invocation tokens sign with the invocation keys (M2-06).
		icfg, err := InvocationTokenConfigFrom(s.values)
		if err != nil {
			return err
		}
		if err := rn.BindInvocationTokens(icfg, keys.Keys, app.NewState(postgres.NewRepository(pool), a.UoW()), s.deps.Clock); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.signingKeys = keys
	s.keyCheck = cfg.CheckInterval
	s.mu.Unlock()
	return nil
}
