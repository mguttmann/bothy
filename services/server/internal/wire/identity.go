// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"

	identityhttp "github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/locale"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/replay"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// IdentityConfig binds the keys of the identity module (principal.*, the DPoP windows,
// auth.token_pepper_ref, server.public_url and the page sizes) into its configuration.
//
// The error names the key whose binding failed (config.Bind), never a value.
func IdentityConfig(v *config.Values) (app.Config, error) {
	var c app.Config
	if err := config.Bind(v, &c); err != nil {
		return app.Config{}, err
	}
	return c, nil
}

// pepperSize is the length of the token pepper: secrets init writes 32 random bytes as hex
// (secrets/stack GeneratorKey).
const pepperSize = 32

// errTokenPepper reports a token pepper that is not 32 bytes as hex; it never names the value.
var errTokenPepper = errors.New("wire: auth.token_pepper_ref does not hold 32 bytes as hex")

// TokenPepper resolves auth.token_pepper_ref through the secret provider and returns the raw 32
// bytes. Errors: those of the provider (a missing or unreadable secret) and errTokenPepper; no
// error contains the secret.
func TokenPepper(ctx context.Context, sp *secrets.Provider, ref string) ([]byte, error) {
	v, _, err := sp.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	raw, err := hex.DecodeString(strings.TrimSpace(string(v.Bytes())))
	if err != nil || len(raw) != pepperSize {
		return nil, errTokenPepper
	}
	return raw, nil
}

// Identity is the identity module of the process (M1-33): the device use cases, the DPoP verifier
// with its jti store, the people and owner use cases (M1-31) and the REST handlers of their
// operations.
type Identity struct {
	// Devices is the port identity.Devices.
	Devices *app.Devices
	// People implements the ports identity.People and identity.Owners (M1-31).
	People *app.People
	// PeopleHandlers are the REST handlers of the ten people, owner and self operations.
	PeopleHandlers *identityhttp.People
	// DPoP is the port identity.DPoP; M1-34 hands it to the Authenticator.
	DPoP *app.DPoPVerifier
	// Replay is the jti store of DPoP; Start sweeps it.
	Replay *replay.Store
	// Handlers are the REST handlers of listMyDevices, listUserDevices and getDevice.
	Handlers *identityhttp.Devices
	// Sessions implements the port identity.Auth (M1-34); nil until BindSessions.
	Sessions *app.Sessions
	// Authenticator is the kernel/actor.Authenticator of people (M1-34); nil until BindSessions.
	Authenticator *app.Authenticator
	// SessionHandlers are the REST handlers of refreshToken, logout, listMySessions,
	// listUserSessions and createExtensionSession; nil until BindSessions.
	SessionHandlers *identityhttp.Sessions
	// StepUps implements the port identity.StepUp (M1-35); nil until BindSessions.
	StepUps *app.StepUps
	// StepUpHandlers are the REST handlers of createStepUpChallenge and
	// completeStepUpWithDeviceKey; nil until BindSessions.
	StepUpHandlers *identityhttp.StepUps
	// AuthCleanup is the use case of the job identity.auth_cleanup (M1-35); nil until
	// BindSessions.
	AuthCleanup *app.AuthCleanup
	// Providers implements the port identity.Providers (M1-36); nil until BindSessions, because
	// disabling a provider revokes sessions through identity.Auth.
	Providers *app.Providers
	// Identities implements the port identity.Identities (M1-36); nil until BindSessions.
	Identities *app.Identities
	// ProviderHandlers are the REST handlers of the five provider operations; nil until
	// BindSessions.
	ProviderHandlers *identityhttp.Providers
	// CredentialHandlers are the REST handlers of getMyCredentials, getUserCredentials and
	// unlinkIdentity; nil until BindSessions.
	CredentialHandlers *identityhttp.Credentials
	cfg                app.Config
	repo               *identitypg.Repository
	uow                txn.UnitOfWork
	az                 authorize.Authorizer
	writes             IdentityWrites
	pepper             []byte
	clk                clock.Clock
	log                *slog.Logger
}

// IdentityWrites are the collaborators of the writing use cases of identity (M1-31): the
// recorder of audit and the appender of sync.
type IdentityWrites struct {
	// Audit records the allow entries.
	Audit auditing.Recorder
	// Events appends user.updated.
	Events eventlog.Appender
}

// NewIdentity builds the identity module over pool with the unit of work uow, the Authorizer az,
// the recorder and appender of w, the token pepper (raw bytes), the clock (nil means
// clock.System()) and log. It opens no connection and starts nothing. Errors: a window, limit or
// page size outside its range, an empty pepper, a missing recorder or appender.
func NewIdentity(cfg app.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, w IdentityWrites, pepper []byte,
	clk clock.Clock, log *slog.Logger) (*Identity, error) {
	if clk == nil {
		clk = clock.System()
	}
	repo := identitypg.NewRepository(pool)
	store := replay.New(clk)
	verifier, err := app.NewDPoPVerifier(cfg, pepper, store, clk, orDiscard(log))
	if err != nil {
		return nil, err
	}
	devices, err := app.NewDevices(cfg, app.DevicesDeps{
		Store: repo, People: repo, UoW: uow, Authorizer: az, Cursors: identityhttp.Cursors{},
	})
	if err != nil {
		return nil, err
	}
	people, err := app.NewPeople(cfg, app.PeopleDeps{
		Store: repo, UoW: uow, Authorizer: az, Audit: w.Audit, Events: w.Events, Cursors: identityhttp.Cursors{},
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Locales: locale.Parser{}, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	return &Identity{
		Devices: devices, People: people, PeopleHandlers: identityhttp.NewPeople(people, people), DPoP: verifier, Replay: store,
		Handlers: identityhttp.NewDevices(devices), cfg: cfg, repo: repo, uow: uow, az: az, writes: w,
		pepper: append([]byte(nil), pepper...), clk: clk, log: orDiscard(log),
	}, nil
}

// BindSessions builds the session part of identity (M1-34) over the signing keys of instance and
// the instance id (the issuer of access tokens): the use cases of identity.Auth, the Authenticator
// of the HTTP chain and the session handlers, and the step-up use cases with their handlers and
// the use case of identity.auth_cleanup (M1-35). The start calls it after the signing keys exist.
// Errors: a duration, URL or collaborator the use cases refuse.
func (i *Identity) BindSessions(keys instance.Keys, instanceID ids.ID) error {
	tokens, err := app.NewAccessTokens(app.AccessTokenConfig{
		InstanceID: instanceID, PublicURL: i.cfg.PublicURL, TTL: i.cfg.AccessTokenTTL, Leeway: i.cfg.ClockLeeway,
	}, keys, i.clk)
	if err != nil {
		return err
	}
	sessions, err := app.NewSessions(i.cfg, i.pepper, app.SessionsDeps{
		Store: i.repo, Devices: i.repo, People: i.repo, Tokens: tokens, DPoP: i.DPoP, UoW: i.uow, Authorizer: i.az,
		Audit: i.writes.Audit, Events: i.writes.Events, Cursors: identityhttp.Cursors{}, Clock: i.clk,
	})
	if err != nil {
		return err
	}
	auth, err := app.NewAuthenticator(i.cfg, tokens, i.DPoP, i.repo, i.clk, i.log)
	if err != nil {
		return err
	}
	stepUps, err := app.NewStepUps(i.cfg, i.pepper, app.StepUpDeps{
		Challenges: i.repo, Sessions: i.repo, Devices: i.repo, Auth: sessions, UoW: i.uow, Authorizer: i.az, Audit: i.writes.Audit,
		Clock: i.clk,
	})
	if err != nil {
		return err
	}
	cleanup, err := app.NewAuthCleanup(i.cfg, i.uow, i.repo, i.clk)
	if err != nil {
		return err
	}
	providers, err := app.NewProviders(i.cfg, app.ProvidersDeps{
		Store: i.repo, Rules: app.DefaultProviderRules(SecretRefSyntax{}), UoW: i.uow, Authorizer: i.az, Audit: i.writes.Audit,
		Auth: sessions, Clock: i.clk,
	})
	if err != nil {
		return err
	}
	identities, err := app.NewIdentities(app.IdentitiesDeps{
		Providers: i.repo, Identities: i.repo, People: i.repo, UoW: i.uow, Authorizer: i.az, Audit: i.writes.Audit,
		Auth: sessions, Clock: i.clk,
	})
	if err != nil {
		return err
	}
	i.Sessions, i.Authenticator, i.SessionHandlers = sessions, auth, identityhttp.NewSessions(sessions, i.DPoP)
	i.StepUps, i.StepUpHandlers, i.AuthCleanup = stepUps, identityhttp.NewStepUps(stepUps), cleanup
	i.Providers, i.ProviderHandlers = providers, identityhttp.NewProviders(providers)
	i.Identities, i.CredentialHandlers = identities, identityhttp.NewCredentials(identities)
	return nil
}

// secretsDirStandIn replaces the placeholder <secrets.dir> while a reference is only checked: the
// syntax of a reference does not depend on the real directory, and the check never resolves it.
const secretsDirStandIn = "/secrets-dir"

// SecretRefSyntax implements app.SecretRefs with the parser of platform/secrets (M1-36): a
// reference is valid when secrets.ParseRef accepts it, with the schemes file and env this server
// version resolves; azkv and awssm have no driver yet and are refused, because a provider with
// them could never sign in. Nothing is resolved, and the error never repeats the reference.
type SecretRefSyntax struct{}

// Valid implements app.SecretRefs.
func (SecretRefSyntax) Valid(ref string) error {
	_, err := secrets.ParseRef(ref, secretsDirStandIn)
	return err
}

// Start runs the sweeper of the jti store every auth.dpop_proof_max_age (the length of the replay
// window, docs/DECISIONS.md M1-33) and registers its stop step in the HTTP phase.
func (i *Identity) Start(r *lifecycle.Runner) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		i.Replay.Run(ctx, i.cfg.DPoPProofMaxAge)
	}()
	r.OnStop(lifecycle.StopStep{Name: "dpop_replay_sweeper", Phase: lifecycle.PhaseHTTP, Run: func(stopCtx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stopCtx.Done():
			return stopCtx.Err()
		}
	}})
}
