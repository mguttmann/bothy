// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/authz"
	authzhttp "github.com/OWNER/bothy/services/server/internal/authz/adapters/httpapi"
	authzmetrics "github.com/OWNER/bothy/services/server/internal/authz/adapters/otelmetrics"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/gateway"
	"github.com/OWNER/bothy/services/server/internal/identity"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/terminal"
)

// msgRegistrationCheckFailed is the log message of each violation the catalogue check finds.
const msgRegistrationCheckFailed = "registration check failed"

// moduleRegistrations are the Register(r *authorize.Registry) functions of the module ports, in
// the order of the module register. In M1 identity registers user.updated (M1-31), authz
// access.changed (M1-29), grant.changed, group.changed and access.expiring (M1-30) and agents
// agent.updated (M1-51), chat space.created, space.updated, space.archived and sidebar.updated
// (M1-52), messaging message.created (M1-56), terminal (M2-39), sync read_state.updated,
// draft.updated, draft.deleted and settings.updated (M1-61) and gateway the ops identify, resume,
// heartbeat and reauth (M1-64), instance instance.updated (M1-67) and files file.updated (M3-03);
// each module's ticket
// appends its function here.
var moduleRegistrations = []func(*authorize.Registry){instance.Register, identity.Register, authz.Register, agents.Register, chat.Register,
	files.Register, messaging.Register, terminal.Register, sync.Register, gateway.Register}

// BuildRegistry returns a registry filled by every function of register; nil means the
// production list of the module ports.
func BuildRegistry(register []func(*authorize.Registry)) *authorize.Registry {
	if register == nil {
		register = moduleRegistrations
	}
	r := authorize.NewRegistry()
	for _, f := range register {
		f(r)
	}
	return r
}

// RouteBindings converts operations of the route table to the bindings the catalogue check reads.
func RouteBindings(ops []routes.Operation) []authorize.RouteBinding {
	out := make([]authorize.RouteBinding, len(ops))
	for i, op := range ops {
		out[i] = authorize.RouteBinding{
			OperationID: op.ID, Method: op.Method, Pattern: op.Pattern,
			Action: op.Action, RateLimitClass: op.RateLimitClass,
		}
	}
	return out
}

// NewAuthzCatalog returns the closed action catalogue of authz with bothy.authz.errors on the
// global meter provider of platform/otel (call it after otel.Setup). log nil means slog.Default().
func NewAuthzCatalog(log *slog.Logger) (*authzapp.Catalog, error) {
	counter, err := telemetry.NewErrorCounter(nil)
	if err != nil {
		return nil, err
	}
	return authzapp.NewCatalog(log, authzapp.WithErrorCounter(counter)), nil
}

// checkRegistrations runs the catalogue check of authz over the route bindings and the registry
// and logs one ERROR line per violation (02 15.1 step 7, 07 AZ-P1 and AZ-P2). The lines carry only
// attributes of the signals catalogue (error.type with the violation kind, bothy.action,
// bothy.ratelimit.class, bothy.event.type), because the log handlers drop every other key; the
// full text with operation id, op and module is the returned error, which `bothy serve` writes to
// stderr, one line per violation.
func checkRegistrations(cat *authzapp.Catalog, bindings []authorize.RouteBinding, reg *authorize.Registry, log *slog.Logger) error {
	err := cat.VerifyRegistrations(bindings, reg)
	if err == nil {
		return nil
	}
	var re *authzapp.RegistrationError
	if !errors.As(err, &re) {
		log.Error(msgRegistrationCheckFailed, slog.String(attr.ErrorType, "registration"))
		return err
	}
	for _, v := range re.Violations {
		attrs := []slog.Attr{slog.String(attr.ErrorType, v.Kind)}
		if v.Action != "" {
			attrs = append(attrs, slog.String(attr.Action, v.Action))
		}
		if v.RateLimitClass != "" {
			attrs = append(attrs, slog.String(attr.RatelimitClass, v.RateLimitClass))
		}
		if v.EventType != "" {
			attrs = append(attrs, slog.String(attr.EventType, v.EventType))
		}
		log.LogAttrs(context.Background(), slog.LevelError, msgRegistrationCheckFailed, attrs...)
	}
	return err
}

// startRoutes is start step 7 for routes and registrations: the catalogue check over the route
// bindings and the registry for every role (a worker emits events too), then mount, which builds
// the HTTP chain and calls MountRoutes with the same catalogue as the one definition of "known"
// (api and all; nil for the worker). A violation ends the step before mount runs, so nothing is
// built or mounted, the public listener never opens and `bothy serve` exits with code 1.
func startRoutes(bindings []authorize.RouteBinding, reg *authorize.Registry, cat *authzapp.Catalog, log *slog.Logger,
	mount func() error) error {
	if log == nil {
		log = slog.Default()
	}
	if err := checkRegistrations(cat, bindings, reg, log); err != nil {
		return err
	}
	if mount == nil {
		return nil
	}
	return mount()
}

// FreshStepUpCheck binds terminal.step_up_max_age into the fresh step-up check of authz (M2-40),
// which identity uses for StepUp.ConsumeFresh, so the consumption and authorize judge with one
// rule. The error names the key whose binding failed, never a value.
func FreshStepUpCheck(v *config.Values) (authzapp.FreshCheck, error) {
	var c authzapp.FreshCheck
	if err := config.Bind(v, &c); err != nil {
		return authzapp.FreshCheck{}, err
	}
	return c, nil
}

// AuthzConfig binds authz.snapshot_cache_max_entries, auth.step_up_max_age,
// authz.view_as_max_duration and the terminal windows terminal.step_up_max_age and
// terminal.quick_reattach_window (M2-40) into the configuration of the Authorizer. The error
// names the key whose binding failed, never a value.
func AuthzConfig(v *config.Values) (authzapp.AuthorizerConfig, error) {
	var c authzapp.AuthorizerConfig
	if err := config.Bind(v, &c); err != nil {
		return authzapp.AuthorizerConfig{}, err
	}
	return c, nil
}

// NewAuthorizer builds the Authorizer of authz, the kernel/authorize.Authorizer of the process
// (M1-28), over pool: the PrincipalSource of identity (M1-26), the grant store of authz (M1-27),
// the rights version of instance (instance.Versions), the recorder of audit (M1-24), the clock and
// log, the CredentialSource of identity (devices since M1-33, sessions since M1-34, API tokens
// since M1-44), the agent policy source of agents (M1-51) and the structure source with the
// spaces of chat (M1-52) and no channels until M1-53 (NewStructureSource), and the counter
// bothy.authz.decisions of authz/adapters/otelmetrics on the global meter provider (M1-68).
// It opens no connection.
//
// Errors: those of authzapp.NewAuthorizer (a cache bound below 1) and of the meter provider.
func NewAuthorizer(pool *pg.Pool, cfg authzapp.AuthorizerConfig, cat *authzapp.Catalog, rec auditing.Recorder,
	clk clock.Clock, log *slog.Logger) (*authzapp.Authorizer, error) {
	if clk == nil {
		clk = clock.System()
	}
	decisions, err := authzmetrics.New(nil)
	if err != nil {
		return nil, err
	}
	return authzapp.NewAuthorizer(cfg, authzapp.AuthorizerDeps{
		Catalog:     cat,
		Store:       authzpg.NewRepository(pool),
		Principals:  principalSource(pool),
		Credentials: identityapp.NewCredentialSource(identitypg.NewRepository(pool), identitypg.NewRepository(pool), identitypg.NewRepository(pool)),
		Structure:   NewStructureSource(pool),
		Policies:    NewAgentPolicySource(pool),
		Hosts:       NewHostSource(pool),
		Versions:    instanceapp.NewVersions(instancepg.NewRepository(pool)),
		Recorder:    rec,
		Clock:       clk,
		Log:         orDiscard(log),
		Metrics:     decisions,
	})
}

// Compile-time check: the process's authorizer is the kernel port.
var _ authorize.Authorizer = (*authzapp.Authorizer)(nil)

// RightsSources are the source ports of the rights hook that tests may replace; a nil field
// means the production source: the PrincipalSource of identity, the agent policy source of agents
// (M1-51) and the structure source of chat (spaces since M1-52, NewStructureSource).
type RightsSources struct {
	Principals authz.PrincipalSource
	Structure  authz.StructureSource
	Policies   authz.AgentPolicySource
	// Hosts nil means the host source of terminal (M2-39).
	Hosts authz.HostSource
}

// NewRightsHook returns the rights hooks of authz over pool (M1-29): the counters of instance
// (only the phase hooks receive them, TestCountersOnlyUsedByPhaseHooks), the grant store of authz
// and the sources src, the clock clk (nil for the system clock) and log. It opens no connection.
// The hook fails every rights change until RightsHook.Bind.
//
// Errors: those of authzapp.NewRightsHook.
func NewRightsHook(pool *pg.Pool, src RightsSources, clk clock.Clock, log *slog.Logger) (*authzapp.RightsHook, error) {
	if src.Principals == nil {
		src.Principals = principalSource(pool)
	}
	if src.Structure == nil {
		src.Structure = NewStructureSource(pool)
	}
	if src.Policies == nil {
		src.Policies = NewAgentPolicySource(pool)
	}
	if src.Hosts == nil {
		src.Hosts = NewHostSource(pool)
	}
	if clk == nil {
		clk = clock.System()
	}
	return authzapp.NewRightsHook(authzapp.RightsHookDeps{
		Counters: InstanceCounters(pool), Store: authzpg.NewRepository(pool), Expiry: authzpg.NewRepository(pool), Principals: src.Principals,
		Structure: src.Structure, Policies: src.Policies, Hosts: src.Hosts, Clock: clk, Log: orDiscard(log),
	})
}

// mustRightsHook is NewRightsHook for NewModules: every dependency is set, so an error is a
// programming error.
func mustRightsHook(pool *pg.Pool, src RightsSources, clk clock.Clock, log *slog.Logger) *authzapp.RightsHook {
	h, err := NewRightsHook(pool, src, clk, log)
	if err != nil {
		panic(err)
	}
	return h
}

// AuthzExpiryConfig binds authz.expiry_sweep_interval, authz.expiry_reminder_lead and
// authz.expiry_sweep_batch_max into the configuration of the expiry jobs (M1-30). The error names
// the key whose binding failed, never a value.
func AuthzExpiryConfig(v *config.Values) (authzapp.ExpiryConfig, error) {
	var c authzapp.ExpiryConfig
	if err := config.Bind(v, &c); err != nil {
		return authzapp.ExpiryConfig{}, err
	}
	return c, nil
}

// NewExpiryService builds the use case of the jobs authz.expiry_sweep and authz.expiry_reminder
// (M1-30) over pool: the unit of work and the rights hook of the audit step a (the hook must be
// bound, start step 6c; its begin step folds the due expiries of a sweep batch in), the appender of
// the sync step sy, the grant store of authz, the PrincipalSource of identity and the system clock.
// It opens no connection.
//
// Errors: those of authzapp.NewExpiryService.
func NewExpiryService(pool *pg.Pool, a *Audit, sy *Sync, cfg authzapp.ExpiryConfig, log *slog.Logger) (*authzapp.ExpiryService, error) {
	if a == nil || sy == nil {
		return nil, errors.New("wire: NewExpiryService needs the audit and the sync step")
	}
	return authzapp.NewExpiryService(cfg, authzapp.ExpiryDeps{
		UoW: a.UoW(), Expiry: authzpg.NewRepository(pool), Rights: a.RightsHook(),
		Principals: principalSource(pool),
		Events:     sy.Appender(), Clock: clock.System(), Log: orDiscard(log),
	})
}

// AuthzAdminConfig binds group.name_max_chars, group.description_max_chars, api.page_size_default
// and api.page_size_max into the configuration of the Admin use cases (M1-48). The error names
// the key whose binding failed, never a value.
func AuthzAdminConfig(v *config.Values) (authzapp.AdminConfig, error) {
	var c authzapp.AdminConfig
	if err := config.Bind(v, &c); err != nil {
		return authzapp.AdminConfig{}, err
	}
	return c, nil
}

// AuthzAdmin is the group and grant management of authz (M1-48): the use cases of the port
// authz.Admin and the REST handlers of its twelve operations.
type AuthzAdmin struct {
	// Admin implements authz.Admin.
	Admin *authzapp.AdminService
	// Handlers are the REST handlers of the twelve operations of 06 "Groups" and "Access".
	Handlers *authzhttp.Access
}

// NewAuthzAdmin builds the Admin use cases over pool (M1-48): the unit of work and the rights hook
// of the audit step a (the hook must be bound), the Authorizer az (also the manage scope of the
// lists), the PrincipalSource of identity, the structure source of chat (NewStructureSource),
// the recorder of a, the appender of sy, the cursor codec of platform/httpserver and the clock clk
// (nil for the system clock). It opens no connection.
//
// Errors: a missing audit or sync step, and those of authzapp.NewAdminService.
func NewAuthzAdmin(pool *pg.Pool, cfg authzapp.AdminConfig, a *Audit, sy *Sync, az *authzapp.Authorizer, clk clock.Clock) (*AuthzAdmin, error) {
	if a == nil || sy == nil || az == nil {
		return nil, errors.New("wire: NewAuthzAdmin needs the audit and the sync step and the Authorizer")
	}
	if clk == nil {
		clk = clock.System()
	}
	admin, err := authzapp.NewAdminService(authzapp.AdminDeps{
		UoW: a.UoW(), Authorizer: az, Store: authzpg.NewRepository(pool),
		Principals: identityapp.NewPrincipalSource(identitypg.NewRepository(pool)), Structure: NewStructureSource(pool), Hosts: NewHostSource(pool),
		Rights: a.RightsHook(), Audit: a.Recorder(), Clock: clk, Events: sy.Appender(), Scopes: az, Cursors: authzhttp.Cursors{},
		Config: cfg,
	})
	if err != nil {
		return nil, err
	}
	return &AuthzAdmin{Admin: admin, Handlers: authzhttp.New(admin)}, nil
}
