// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	stdsync "sync"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/apps/adapters/hmackey"
	appshttp "github.com/OWNER/bothy/services/server/internal/apps/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/apps/adapters/identitysource"
	appsjobs "github.com/OWNER/bothy/services/server/internal/apps/adapters/jobs"
	appspg "github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/apps/adapters/webhookhttp"
	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/authz"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// AppsConfig binds the keys of the module apps (app.manifest_max_bytes, webhook.allowed_hosts,
// the command limits, the principal limits, audit.reason_max_chars and the page sizes). The error
// names the key whose binding failed, never a value.
func AppsConfig(v *config.Values) (appsapp.Config, error) {
	var c appsapp.Config
	if err := config.Bind(v, &c); err != nil {
		return appsapp.Config{}, err
	}
	return c, nil
}

// NewAppInstallations returns identity.AppInstallations over the apps tables of pool
// (apps/adapters/identitysource, M2-24 design note 12): identity reads the installation of an app
// principal through it, so identity never imports apps.
func NewAppInstallations(pool *pg.Pool) identity.AppInstallations {
	return identitysource.New(appspg.NewRepository(pool))
}

// principalSource is the authz.PrincipalSource of identity over pool with the installations of
// apps, so an app without an active installation has an empty snapshot (M2-24 design note 11 b).
func principalSource(pool *pg.Pool) *identityapp.PrincipalSource {
	return identityapp.NewPrincipalSource(identitypg.NewRepository(pool)).WithApps(NewAppInstallations(pool))
}

// webhookKeys are the keys of the outgoing webhooks besides those of appsapp.WebhookConfig.
type webhookKeys struct {
	MasterRef string `config:"webhook.hmac_master_key_ref"`
}

// IncomingConfig binds the keys of the incoming webhooks (M2-27): message.max_content_bytes,
// webhook_in.formats_enabled, identity.last_active_granularity and server.public_url. The error
// names the key whose binding failed, never a value.
func IncomingConfig(v *config.Values) (appsapp.IncomingConfig, error) {
	var c appsapp.IncomingConfig
	if err := config.Bind(v, &c); err != nil {
		return appsapp.IncomingConfig{}, err
	}
	return c, nil
}

// WebhooksConfig binds the keys of the outgoing webhooks (M2-26): the values of the use cases and
// webhook.hmac_master_key_ref. The error names the key whose binding failed, never a value.
func WebhooksConfig(v *config.Values) (appsapp.WebhookConfig, string, error) {
	var c appsapp.WebhookConfig
	if err := config.Bind(v, &c); err != nil {
		return appsapp.WebhookConfig{}, "", err
	}
	var k webhookKeys
	if err := config.Bind(v, &k); err != nil {
		return appsapp.WebhookConfig{}, "", err
	}
	return c, k.MasterRef, nil
}

// NewWebhookSender builds the sender of the outgoing webhooks over the egress configuration: an
// egress client with purpose webhook, the hosts of webhook.allowed_hosts, no redirects and the
// time limit webhook.delivery_timeout (M2-26 requirement 21). Errors: an entry of
// webhook.allowed_hosts that is no host[:port], or what egress.New refuses.
func NewWebhookSender(eg egress.Config, cfg appsapp.WebhookConfig, deps egress.Deps) (*webhookhttp.Sender, error) {
	hosts, err := domain.ParseAllowedHosts(cfg.AllowedHosts)
	if err != nil {
		return nil, err
	}
	return webhookhttp.New(eg, hosts, cfg.DeliveryTimeout, deps, nil)
}

// NewHMACMasterKey returns the master key of the derived v1 secrets behind ref
// (webhook.hmac_master_key_ref) through the secret provider sp.
func NewHMACMasterKey(sp *secrets.Provider, ref string) *hmackey.Master { return hmackey.New(sp, ref) }

// WebhookDeps are the collaborators of the outgoing webhooks (M2-26).
type WebhookDeps struct {
	// Config are the values of the use cases (WebhooksConfig).
	Config appsapp.WebhookConfig
	// Keys are the signing keys; nil leaves them to Apps.BindKeys, because the signing keys start
	// after the apps step (02 "Start" step 7).
	Keys appsapp.Signers
	// State is the instance row; Log the log reader of sync.
	State appsapp.InstanceState
	Log   sync.LogReader
	// Master is the master key of the v1 secrets; Sender sends the attempts.
	Master appsapp.HMACMasterKey
	Sender appsapp.WebhookSender
	// Rand is the jitter source; nil means math/rand/v2 (tests inject one).
	Rand func(n int64) int64
	// Logger receives the delivery and state lines; nil discards them.
	Logger *slog.Logger
}

// Apps is the module apps of the process (M2-24, M2-26): the use cases and the REST handlers of
// the app operations and of the outgoing webhooks, the consumer webhooks and the delivery job.
type Apps struct {
	// Service implements apps.Apps.
	Service *appsapp.Service
	// Handlers are the REST handlers of the apps.
	Handlers *appshttp.Apps
	// Webhooks implements apps.Webhooks, the consumer and the attempt of apps.webhook_deliver.
	Webhooks *appsapp.WebhookService
	// WebhookHandlers are the REST handlers of the subscriptions and deliveries.
	WebhookHandlers *appshttp.Webhooks
	// Scheduler inserts apps.webhook_deliver; BindJobs gives it the job runtime.
	Scheduler *appsjobs.Scheduler
	// Incoming implements apps.IncomingWebhooks (M2-27); IncomingHandlers are its REST handlers.
	Incoming         *appsapp.IncomingService
	IncomingHandlers *appshttp.Incoming
	keys             *lateSigners
}

// lateSigners forwards to the signing keys once they are bound (Apps.BindKeys); before, every
// call is kind unavailable, so nothing signs with a missing key.
type lateSigners struct {
	mu   stdsync.RWMutex
	keys appsapp.Signers
}

// bound returns the bound keys or an unavailable error.
func (l *lateSigners) bound() (appsapp.Signers, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.keys == nil {
		return nil, errs.New(errs.CodeUnavailable, "The signing keys are not started yet.")
	}
	return l.keys, nil
}

// Signers implements appsapp.Signers.
func (l *lateSigners) Signers(ctx context.Context, p instance.Purpose) ([]instance.Signer, error) {
	k, err := l.bound()
	if err != nil {
		return nil, err
	}
	return k.Signers(ctx, p)
}

// Signer implements appsapp.Signers.
func (l *lateSigners) Signer(ctx context.Context, p instance.Purpose) (instance.Signer, error) {
	k, err := l.bound()
	if err != nil {
		return nil, err
	}
	return k.Signer(ctx, p)
}

// BindKeys binds the signing keys (start step 7, after the apps step).
func (ap *Apps) BindKeys(k appsapp.Signers) {
	ap.keys.mu.Lock()
	defer ap.keys.mu.Unlock()
	ap.keys.keys = k
}

// BindJobs binds the inserter of the job runtime of the role to the delivery scheduler.
func (ap *Apps) BindJobs(ins jobs.Inserter) { ap.Scheduler.Bind(ins) }

// RegisterJobs registers apps.webhook_deliver on the queue delivery in reg (every role).
func (ap *Apps) RegisterJobs(reg *jobs.Registry) error { return appsjobs.Register(reg, ap.Webhooks) }

// Consumer returns the handler of the worker consumer webhooks.
func (ap *Apps) Consumer() sync.ConsumerHandler { return ap.Webhooks.Consumer() }

// AppsDeps are the collaborators of the module apps from the other start steps.
type AppsDeps struct {
	// UoW is the unit of work of the audit step; Rights its rights hook (AddAffected).
	UoW    txn.UnitOfWork
	Rights authz.RightsChanges
	// Authorizer decides every call; Audit writes the allow entries.
	Authorizer authorize.Authorizer
	Audit      auditing.Recorder
	// Identity provides the directory, the principal writes and the API tokens.
	Identity *Identity
	// Grants is the Admin service of authz (NewAuthzAdmin); uninstall revokes the grants of the
	// app through it (RevokeSubjectGrants).
	Grants *authzapp.AdminService
	// Clock is the time of the writes; nil means clock.System().
	Clock clock.Clock
	// Webhooks are the collaborators of the outgoing webhooks (M2-26).
	Webhooks WebhookDeps
	// Incoming is the configuration of the incoming webhooks (IncomingConfig, M2-27).
	Incoming appsapp.IncomingConfig
	// Chat gives the incoming webhooks the channel facts, the threads and the thread of a parent
	// message; Messages writes their messages (M2-27).
	Chat     *Chat
	Messages *Messaging
}

// NewApps builds the module apps over pool: the repository, the use cases with the ports of
// identity (directory, principal writes, tokens), the grant revocation of authz (the Admin
// service of the authz step, d.Grants), the scope table of authz and the spec version of the
// contract (routes.SpecVersion), the outgoing webhooks over d.Webhooks with their delivery
// scheduler (bound later with BindJobs), and the REST handlers. It opens no connection. Errors: a
// missing collaborator or a value the use cases refuse.
func NewApps(cfg appsapp.Config, pool *pg.Pool, d AppsDeps) (*Apps, error) {
	if d.UoW == nil || d.Rights == nil || d.Authorizer == nil || d.Audit == nil || d.Identity == nil || d.Grants == nil || d.Chat == nil ||
		d.Messages == nil {
		return nil, errors.New("wire: NewApps needs the unit of work, the rights hook, the Authorizer, the recorder, identity, the authz Admin, chat and messaging")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	repo := appspg.NewRepository(pool)
	keys := &lateSigners{keys: d.Webhooks.Keys}
	sched := &appsjobs.Scheduler{}
	hooks, err := appsapp.NewWebhooks(d.Webhooks.Config, appsapp.WebhookDeps{
		Store: repo, Repo: repo, UoW: d.UoW, Authorizer: d.Authorizer, Audit: d.Audit, Scheduler: sched, Sender: d.Webhooks.Sender,
		Master: d.Webhooks.Master, Keys: keys, State: d.Webhooks.State, Log: d.Webhooks.Log, Cursors: appshttp.Cursors{},
		Clock: d.Clock, Rand: d.Webhooks.Rand, Logger: d.Webhooks.Logger,
	})
	if err != nil {
		return nil, err
	}
	in, err := appsapp.NewIncoming(d.Incoming, appsapp.IncomingDeps{
		Store: repo, Repo: repo, UoW: d.UoW, Authorizer: d.Authorizer, Audit: d.Audit, Tokens: d.Identity.Tokens,
		Channels: d.Chat.Service, Threads: d.Chat.Service, ByParent: d.Chat.Service, Messages: d.Messages.Service, Clock: d.Clock,
		Logger: d.Webhooks.Logger,
	})
	if err != nil {
		return nil, err
	}
	svc, err := appsapp.New(cfg, appsapp.Deps{
		Repo: repo, UoW: d.UoW, Authorizer: d.Authorizer, Audit: d.Audit,
		Principals: appsapp.IdentityPrincipals{Directory: d.Identity.Directory, Writes: d.Identity.PrincipalWrites},
		Tokens:     d.Identity.Tokens, Grants: d.Grants, Rights: d.Rights, Scopes: authzapp.ScopeTable{}, Cursors: appshttp.Cursors{},
		Webhooks: hooks, Incoming: in, SpecVersion: routes.SpecVersion, Clock: d.Clock,
	})
	if err != nil {
		return nil, err
	}
	return &Apps{Service: svc, Handlers: appshttp.NewApps(svc), Webhooks: hooks, WebhookHandlers: appshttp.NewWebhooks(hooks),
		Scheduler: sched, Incoming: in, IncomingHandlers: appshttp.NewIncoming(in), keys: keys}, nil
}

// fillHandlers sets h.Apps to the handlers of ap when it is nil, so an injected handler stays; a
// nil ap (the apps step did not run) leaves h unchanged (INT-W19/7).
func (ap *Apps) fillHandlers(h *Handlers) {
	if ap != nil {
		fill(&h.Apps, ap.Handlers)
		fill(&h.Webhooks, ap.WebhookHandlers)
		fill(&h.Incoming, ap.IncomingHandlers)
	}
}
