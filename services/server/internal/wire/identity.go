// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"log/slog"
	"path"
	"strings"

	identityhttp "github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/instanceflags"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/locale"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/replay"
	identitywebauthn "github.com/OWNER/bothy/services/server/internal/identity/adapters/webauthn"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	identitydomain "github.com/OWNER/bothy/services/server/internal/identity/domain"
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
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
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
// with its jti store, the people and owner use cases (M1-31), the API token use cases (M1-44) and
// the REST handlers of their operations.
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
	// Tokens implements the ports identity.Tokens and identity.TokenAdmin (M1-44).
	Tokens *app.Tokens
	// TokenHandlers are the REST handlers of listTokens, createToken and revokeToken.
	TokenHandlers *identityhttp.Tokens
	// Enrollment issues and matches the enrollment codes (M1-37); createUser with local_account
	// uses it too.
	Enrollment *app.EnrollmentCodes
	// Logins is the login completion shared by every login path, the enrollment code operations
	// and the use cases of bothy setup (M1-37); nil until BindSessions.
	Logins *app.Service
	// EnrollmentHandlers are the REST handlers of redeemEnrollmentCode and createEnrollmentCode;
	// nil until BindSessions.
	EnrollmentHandlers *identityhttp.Enrollment
	// ApprovalHandlers are the REST handlers of approveDevice, beginPairing, approvePairing and
	// completePairing (M1-38); nil until BindSessions.
	ApprovalHandlers *identityhttp.Approval
	// Passkeys implements the port identity.Passkeys (M1-40); nil until BindSessions.
	Passkeys *app.Passkeys
	// PasskeyHandlers are the REST handlers of the seven passkey operations and
	// completeStepUpWithPasskey; nil until BindSessions.
	PasskeyHandlers *identityhttp.Passkeys
	// OIDC implements the port identity.OIDCLogin (M1-41); nil until BindOIDC.
	OIDC *app.OIDC
	// OIDCHandlers are the REST handlers of beginOidcLogin, handleOidcCallback, completeOidcLogin,
	// beginIdentityLink and completeIdentityLink; nil until BindOIDC.
	OIDCHandlers *identityhttp.OIDC
	// TrustedHeader is the driver trusted_header (M1-43): the loopback flag of the public listener,
	// the readiness check and the per-request binding.
	TrustedHeader *app.TrustedHeaderDriver
	// TrustedHeaderHandlers is the REST handler of loginWithTrustedHeader; nil until BindSessions.
	TrustedHeaderHandlers *identityhttp.TrustedHeader
	// Principals implements the port identity.PrincipalWriter for the agents module (M1-51).
	Principals *app.PrincipalWriter
	cfg        app.Config
	state      instance.State
	repo       *identitypg.Repository
	uow        txn.UnitOfWork
	az         authorize.Authorizer
	writes     IdentityWrites
	pepper     []byte
	clk        clock.Clock
	log        *slog.Logger
}

// IdentityWrites are the collaborators of the writing use cases of identity (M1-31): the
// recorder of audit, the appender of sync, the settings and the feature flags of instance.
type IdentityWrites struct {
	// Audit records the allow entries.
	Audit auditing.Recorder
	// Events appends user.updated.
	Events eventlog.Appender
	// Flags are the feature flags of instance (M1-47); createToken asks tokens.personal_enabled
	// through them (identity/adapters/instanceflags).
	Flags instance.Flags
	// Settings reads locale.default, the language createUser stores when the request names none
	// (M1-46).
	Settings instance.Settings
}

// NewIdentity builds the identity module over pool with the unit of work uow, the Authorizer az,
// the recorder and appender of w, the token pepper (raw bytes), the clock (nil means
// clock.System()) and log. It opens no connection and starts nothing. Errors: a window, limit or
// page size outside its range, an empty pepper, a missing recorder, appender or feature flags.
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
		Store: repo, People: repo, UoW: uow, Authorizer: az, Cursors: identityhttp.Cursors{}, Pepper: pepper,
	})
	if err != nil {
		return nil, err
	}
	enrollment, err := app.NewEnrollmentCodes(cfg, pepper, app.EnrollmentDeps{Codes: repo, People: repo, UoW: uow, Clock: clk})
	if err != nil {
		return nil, err
	}
	instanceRepo := instancepg.NewRepository(pool)
	people, err := app.NewPeople(cfg, app.PeopleDeps{
		Store: repo, UoW: uow, Authorizer: az, Audit: w.Audit, Events: w.Events, Cursors: identityhttp.Cursors{},
		Versions: instanceapp.NewVersions(instanceRepo), Locales: locale.Parser{}, Providers: repo, Identities: repo,
		Enrollment: enrollment, Settings: w.Settings, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	personal, err := instanceflags.New(w.Flags)
	if err != nil {
		return nil, err
	}
	driver, err := app.NewTrustedHeaderDriver(cfg, repo, repo, orDiscard(log))
	if err != nil {
		return nil, err
	}
	principals, err := app.NewPrincipalWriter(cfg, repo)
	if err != nil {
		return nil, err
	}
	tokens, err := app.NewTokens(cfg, pepper, app.TokensDeps{
		Store: repo, Principals: repo, UoW: uow, Authorizer: az, Audit: w.Audit, Cursors: identityhttp.Cursors{},
		Switch: personal, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	return &Identity{
		Devices: devices, People: people, PeopleHandlers: identityhttp.NewPeople(people, people), DPoP: verifier, Replay: store,
		Handlers: identityhttp.NewDevices(devices), Tokens: tokens, TokenHandlers: identityhttp.NewTokens(tokens),
		Enrollment: enrollment, TrustedHeader: driver, Principals: principals, cfg: cfg, state: instanceapp.NewState(instanceRepo, uow),
		repo: repo, uow: uow, az: az, writes: w, pepper: append([]byte(nil), pepper...), clk: clk, log: orDiscard(log),
	}, nil
}

// BindSessions builds the session part of identity (M1-34) over the signing keys of instance and
// the instance id (the issuer of access tokens): the use cases of identity.Auth, the Authenticator
// of the HTTP chain and the session handlers, the step-up use cases with their handlers and
// the use case of identity.auth_cleanup (M1-35), the providers and identities (M1-36), the
// login service with the enrollment code handlers and the use cases of bothy setup (M1-37), and
// the approval and pairing handlers (M1-38). The start calls it after the signing keys exist.
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
		Audit: i.writes.Audit, Events: i.writes.Events, Cursors: identityhttp.Cursors{}, Clock: i.clk, Binding: i.TrustedHeader,
	})
	if err != nil {
		return err
	}
	auth, err := app.NewAuthenticator(i.cfg, app.AuthenticatorDeps{
		AccessTokens: tokens, DPoP: i.DPoP, Sessions: i.repo, APITokens: i.repo, Pepper: i.pepper, Clock: i.clk, Log: i.log,
		Binding: i.TrustedHeader, Revoker: sessions, UoW: i.uow, Audit: i.writes.Audit,
	})
	if err != nil {
		return err
	}
	rp, verifier, err := relyingParty(i.cfg)
	if err != nil {
		return err
	}
	stepUps, err := app.NewStepUps(i.cfg, i.pepper, app.StepUpDeps{
		Challenges: i.repo, Sessions: i.repo, Devices: i.repo, Auth: sessions, UoW: i.uow, Authorizer: i.az, Audit: i.writes.Audit,
		Clock: i.clk, Passkeys: i.repo, People: i.repo, Verifier: verifier, RelyingParty: rp,
	})
	if err != nil {
		return err
	}
	providers, err := app.NewProviders(i.cfg, app.ProvidersDeps{
		Store: i.repo, Rules: app.DefaultProviderRules(SecretRefSyntax{}), UoW: i.uow, Authorizer: i.az, Audit: i.writes.Audit,
		Auth: sessions, Clock: i.clk, TrustedHeader: i.TrustedHeader,
	})
	if err != nil {
		return err
	}
	identities, err := app.NewIdentities(app.IdentitiesDeps{
		Providers: i.repo, Identities: i.repo, People: i.repo, UoW: i.uow, Authorizer: i.az, Audit: i.writes.Audit,
		Auth: sessions, Clock: i.clk, Sources: []app.LoginMethodSource{app.NewPasskeySource(i.repo, i.repo)},
	})
	if err != nil {
		return err
	}
	i.Sessions, i.Authenticator, i.SessionHandlers = sessions, auth, identityhttp.NewSessions(sessions, i.DPoP)
	i.StepUps, i.StepUpHandlers = stepUps, identityhttp.NewStepUps(stepUps)
	i.Providers, i.ProviderHandlers = providers, identityhttp.NewProviders(providers)
	i.Identities, i.CredentialHandlers = identities, identityhttp.NewCredentials(identities)
	logins, err := app.NewService(i.cfg, app.ServiceDeps{
		People: i.repo, Devices: i.repo, Providers: i.repo, Identities: i.repo, Enrollment: i.Enrollment, Auth: sessions,
		DPoP: i.DPoP, Instance: i.state, Keys: keys, Locales: locale.Parser{}, UoW: i.uow, Authorizer: i.az, Audit: i.writes.Audit,
		Clock: i.clk, Sessions: i.repo, Pairings: i.repo, Pepper: i.pepper, TrustedHeader: i.TrustedHeader, Events: i.writes.Events,
	})
	if err != nil {
		return err
	}
	// The cleanup job revokes expired pending devices and expires pairings through the login
	// service (M1-38).
	cleanup, err := app.NewAuthCleanup(i.cfg, i.uow, i.repo, logins, i.clk)
	if err != nil {
		return err
	}
	i.Logins, i.EnrollmentHandlers, i.AuthCleanup = logins, identityhttp.NewEnrollment(logins), cleanup
	i.ApprovalHandlers = identityhttp.NewApproval(logins, i.DPoP)
	i.TrustedHeaderHandlers = identityhttp.NewTrustedHeader(logins)
	passkeys, err := app.NewPasskeys(i.cfg, i.pepper, app.PasskeysDeps{
		Passkeys: i.repo, Challenges: i.repo, Verifier: verifier, RelyingParty: rp, Logins: logins, UoW: i.uow, Authorizer: i.az,
		Audit: i.writes.Audit, Clock: i.clk,
	})
	if err != nil {
		return err
	}
	i.Passkeys, i.PasskeyHandlers = passkeys, identityhttp.NewPasskeys(passkeys, stepUps)
	return nil
}

// relyingParty resolves the WebAuthn relying party of cfg and builds its verifier (M1-40); the
// verifier stays nil (no passkeys) when server.public_url names an IP address and
// auth.webauthn_rp_id is empty. Errors: a relying party the cross-check or go-webauthn refuses.
func relyingParty(cfg app.Config) (identitydomain.RelyingParty, app.PasskeyVerifier, error) {
	rp, err := identitydomain.ResolveRelyingParty(cfg.PublicURL, cfg.WebAuthnRPID, cfg.WebAuthnOrigins)
	if err != nil || !rp.Enabled {
		return rp, nil, err
	}
	v, err := identitywebauthn.New(rp, cfg.LoginFlowTTL)
	if err != nil {
		return identitydomain.RelyingParty{}, nil, err
	}
	return rp, v, nil
}

// OIDCDeps are the collaborators of the OIDC login (M1-41) beside the identity module itself.
type OIDCDeps struct {
	// Egress is egress.* (EgressConfig); every provider request goes through it.
	Egress egress.Config
	// Secrets resolves client_secret_ref and client_key_ref (ProviderSecrets in production).
	Secrets oidc.Secrets
	// RootCAs are the trusted TLS roots of the provider requests; nil means the system roots.
	// Only tests set them.
	RootCAs *x509.CertPool
}

// BindOIDC builds the OIDC login (M1-41) over the login service of BindSessions: the provider
// client over platform/egress with auth.oidc_jwks_cache_ttl and auth.oidc_client_assertion_ttl,
// the use cases with auth.login_flow_ttl and auth.server_code_ttl, and their handlers. The start
// calls it after BindSessions. Errors: BindSessions has not run, a duration or collaborator the
// client or the use cases refuse.
func (i *Identity) BindOIDC(d OIDCDeps) error {
	if i.Logins == nil {
		return errors.New("wire: BindOIDC needs BindSessions first")
	}
	client, err := oidc.New(oidc.Config{CacheTTL: i.cfg.OIDCJWKSCacheTTL, AssertionTTL: i.cfg.OIDCClientAssertionTTL, Egress: d.Egress},
		oidc.Deps{Providers: i.repo, Secrets: d.Secrets, RootCAs: d.RootCAs, Logger: i.log, Clock: i.clk})
	if err != nil {
		return err
	}
	o, err := app.NewOIDC(i.cfg, i.pepper, app.OIDCDeps{Logins: i.Logins, Flows: i.repo, Client: client, Log: i.log})
	if err != nil {
		return err
	}
	i.OIDC, i.OIDCHandlers = o, identityhttp.NewOIDC(o)
	return nil
}

// The confinement of provider secrets (docs/DECISIONS.md M1-36/14): a provider configuration may
// name only files below <secrets.dir>/idp/ and variables starting with BOTHY_IDP_, never a value
// of a server *_ref key, so an owner cannot send the token pepper or another server secret to an
// issuer as client secret.
const (
	providerSecretDir    = "idp"
	providerSecretPrefix = "BOTHY_IDP_" //nolint:gosec // G101: a variable name prefix, no secret
)

// errProviderSecret is the refusal of a reference outside the confinement; it never repeats the
// reference.
var errProviderSecret = errors.New("wire: a provider secret must be a file below <secrets.dir>/idp/ or a variable BOTHY_IDP_*, and no server secret")

// ProviderSecrets implements oidc.Secrets over the secret provider with the confinement above.
type ProviderSecrets struct {
	sp     *secrets.Provider
	dir    string
	server map[string]bool
}

// NewProviderSecrets returns the source over sp for the configuration v: secrets.dir and the
// values of every secret_ref key of the schema, which a provider may never name (an empty value
// names nothing). Errors: the bindings of secrets.* and the schema.
func NewProviderSecrets(sp *secrets.Provider, v *config.Values) (*ProviderSecrets, error) {
	var c secrets.Config
	if err := config.Bind(v, &c); err != nil {
		return nil, err
	}
	schema, err := ConfigSchema()
	if err != nil {
		return nil, err
	}
	server := map[string]bool{}
	for _, k := range schema.Keys() {
		if k.Kind != config.KindSecretRef {
			continue
		}
		if ref, err := secrets.ParseRef(v.String(k.Name), c.Dir); err == nil {
			server[ref.String()] = true
		}
	}
	return &ProviderSecrets{sp: sp, dir: c.Dir, server: server}, nil
}

// Resolve implements oidc.Secrets: the reference must parse, lie inside the confinement and be
// no server secret; then the provider reads it (with its cache).
func (s *ProviderSecrets) Resolve(ctx context.Context, raw string) ([]byte, error) {
	ref, err := secrets.ParseRef(raw, s.dir)
	if err != nil {
		return nil, err
	}
	if !confined(ref, s.dir) || s.server[ref.String()] {
		return nil, errProviderSecret
	}
	v, _, err := s.sp.Get(ctx, ref)
	if err != nil {
		return nil, err
	}
	return v.Bytes(), nil
}

// confined reports a file below dir/idp/ or a variable with the provider prefix.
func confined(ref secrets.Ref, dir string) bool {
	if ref.Path == "" {
		return false
	}
	switch ref.Scheme {
	case "file":
		return strings.HasPrefix(ref.Path, path.Join(dir, providerSecretDir)+"/")
	case "env":
		return strings.HasPrefix(ref.Path, providerSecretPrefix) && len(ref.Path) > len(providerSecretPrefix)
	}
	return false
}

// SetPublicListenerLoopback records whether the bound public listener is a loopback address
// (M1-43, W4); the start calls it after the listen, tests inject the flag.
func (i *Identity) SetPublicListenerLoopback(loopback bool) {
	i.TrustedHeader.SetPublicListenerLoopback(loopback)
}

// checkTrustedHeaderLoopback is the readiness check of the trusted_header driver (08 section 2.4,
// 16 section 4.3, W4).
const checkTrustedHeaderLoopback = app.ReasonTrustedHeaderRequiresLoopback

// RegisterTrustedHeaderReadiness adds the readiness check trusted_header_requires_loopback of id to
// reg: /readyz fails with that name while an enabled trusted_header provider exists and the public
// listener is not loopback (W4: the driver stays off instead of stopping the start).
func RegisterTrustedHeaderReadiness(reg *health.Registry, id *Identity) {
	reg.AddReadiness(checkTrustedHeaderLoopback, id.TrustedHeader.Ready)
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
