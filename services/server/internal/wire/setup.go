// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/migrations"
)

// setupCompleted returns the setup state of the gate over state (M1-37): the setup is complete when
// the instance row has setup_completed_at. A missing row or a database error is an error, which
// the gate answers with 503 unavailable (fail closed).
func setupCompleted(state instance.State) httpserver.SetupStateFunc {
	return func(ctx context.Context) (bool, error) {
		rec, err := state.Load(ctx)
		if err != nil {
			return false, err
		}
		return rec.SetupCompleted(), nil
	}
}

// RouteActions maps the pattern of every operation in ops to its catalogue action, the table the
// setup gate resolves requests with (httpserver.MuxLookup): the same patterns MountRoutes mounts,
// derived from api/routes, never written by hand.
func RouteActions(ops []routes.Operation) map[string]string {
	out := make(map[string]string, len(ops))
	for _, op := range ops {
		out[op.Pattern] = op.Action
	}
	return out
}

// InstallSetupGate puts the setup gate in front of mux on chain (step 4 of the public listener):
// until setup_completed_at is set only the instance.discovery.read operations of the enabled route
// table pass (08 "Erstinstallation").
func InstallSetupGate(chain *httpserver.Chain, mux *http.ServeMux, state instance.State) {
	chain.UseGate(httpserver.SetupGate(setupCompleted(state), httpserver.MuxLookup(mux, RouteActions(routes.Enabled()))))
}

// Setup is the part of the server that bothy setup needs (M1-37): the pool of the application role
// bothy_app (16 section 7.1; the commands only write rows), the audit module with the rights hooks,
// the event log, the Authorizer, the signing keys and the identity module with its login service.
// It serves nothing and starts no job. Close it when the command is done.
type Setup struct {
	pool  *pg.Pool
	audit *Audit
	id    *Identity
}

// NewSetup assembles the setup over the loaded values v and the process environment env, with log
// for its reports (stderr of the command): the secret provider, the database within
// startup.dependency_wait, the schema check against the embedded migrations, the audit module, the
// event log with its partitions, the Authorizer with the bound rights hooks, the signing keys and
// the identity module. Errors: a configuration that does not bind, a missing secret, an
// unreachable or too old database; none contains a secret.
func NewSetup(ctx context.Context, v *config.Values, env []string, log *slog.Logger) (*Setup, error) {
	log = orDiscard(log)
	sp, err := NewSecretsProvider(v, env, log)
	if err != nil {
		return nil, err
	}
	pool, err := setupPool(ctx, v, sp, log)
	if err != nil {
		return nil, err
	}
	s := &Setup{pool: pool}
	if err := s.build(ctx, v, sp, log); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

// setupPool opens the pool of the application role and checks the schema.
func setupPool(ctx context.Context, v *config.Values, sp *secrets.Provider, log *slog.Logger) (*pg.Pool, error) {
	db, err := DatabaseConfig(v, "bothy-setup")
	if err != nil {
		return nil, err
	}
	var wait struct {
		DependencyWait time.Duration `config:"startup.dependency_wait"`
	}
	if err := config.Bind(v, &wait); err != nil {
		return nil, err
	}
	pool, err := pg.Open(ctx, db, sp, log)
	if err != nil {
		return nil, err
	}
	required, err := pg.RequiredSchemaVersion(migrations.FS)
	if err == nil {
		err = pool.WaitReady(ctx, wait.DependencyWait)
	}
	if err == nil {
		err = pg.CheckSchema(ctx, pool, required)
	}
	if err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// build assembles the modules over s.pool in the order of the start of bothy serve.
func (s *Setup) build(ctx context.Context, v *config.Values, sp *secrets.Provider, log *slog.Logger) error {
	acfg, err := AuditConfig(v)
	if err != nil {
		return err
	}
	unitCfg, err := UnitConfig(v)
	if err != nil {
		return err
	}
	kek := SecretsKEK{Provider: sp}
	if err := checkAuditKEK(ctx, kek, acfg.IPKEKRef); err != nil {
		return err
	}
	if s.audit, err = NewAudit(s.pool, unitCfg, acfg, kek, log); err != nil {
		return err
	}
	syncCfg, err := SyncConfigFrom(v)
	if err != nil {
		return err
	}
	sy, err := NewSync(s.pool, s.audit.UoW(), syncCfg, nil, log)
	if err != nil {
		return err
	}
	// The bootstrap appends access.changed; the partitions it needs exist when serve ran, and this
	// keeps the command independent of that.
	if _, err := sy.Maintenance().Run(ctx); err != nil {
		return err
	}
	az, err := s.authorizer(v, sy, log)
	if err != nil {
		return err
	}
	return s.identity(ctx, v, sp, az, sy, log)
}

// authorizer builds the Authorizer and binds the rights hooks to the appender and the recorder.
func (s *Setup) authorizer(v *config.Values, sy *Sync, log *slog.Logger) (authorize.Authorizer, error) {
	cfg, err := AuthzConfig(v)
	if err != nil {
		return nil, err
	}
	cat, err := NewAuthzCatalog(log)
	if err != nil {
		return nil, err
	}
	az, err := NewAuthorizer(s.pool, cfg, cat, s.audit.Recorder(), nil, log)
	if err != nil {
		return nil, err
	}
	if err := s.audit.RightsHook().Bind(authzapp.RightsBindings{Events: sy.Appender(), Audit: s.audit.Recorder()}); err != nil {
		return nil, err
	}
	return az, nil
}

// identity builds the signing keys and the identity module with its login service.
func (s *Setup) identity(ctx context.Context, v *config.Values, sp *secrets.Provider, az authorize.Authorizer, sy *Sync, log *slog.Logger) error {
	kcfg, err := SigningKeysConfig(v)
	if err != nil {
		return err
	}
	keys, err := NewSigningKeys(kcfg, s.pool, sp, s.audit.UoW(), az, s.audit.Recorder(), nil, log)
	if err != nil {
		return err
	}
	icfg, err := IdentityConfig(v)
	if err != nil {
		return err
	}
	pepper, err := TokenPepper(ctx, sp, icfg.TokenPepperRef)
	if err != nil {
		return err
	}
	rec, err := instanceapp.NewState(instancepg.NewRepository(s.pool), s.audit.UoW()).Load(ctx)
	if err != nil {
		return err
	}
	// The people use cases read locale.default through the instance settings (M1-46, INT-W13/5).
	scfg, err := InstanceConfig(v)
	if err != nil {
		return err
	}
	st, err := NewInstanceSettings(scfg, LocalPasswordEnabled(v), s.pool, s.audit.UoW(), az,
		SettingsWrites{Audit: s.audit.Recorder(), Events: sy.Appender()}, nil)
	if err != nil {
		return err
	}
	w := IdentityWrites{Audit: s.audit.Recorder(), Events: sy.Appender(), Settings: st.Admin}
	if s.id, err = NewIdentity(icfg, s.pool, s.audit.UoW(), az, w, pepper, nil, log); err != nil {
		return err
	}
	return s.id.BindSessions(keys.Keys, rec.InstanceID)
}

// BootstrapOwner runs bothy setup bootstrap-owner (identity app.Service.BootstrapOwner).
func (s *Setup) BootstrapOwner(ctx context.Context, cmd BootstrapOwnerCmd) (BootstrapResult, error) {
	return s.id.Logins.BootstrapOwner(ctx, cmd)
}

// IssueRecoveryCode runs bothy setup issue-enrollment-code (app.Service.IssueRecoveryCode).
func (s *Setup) IssueRecoveryCode(ctx context.Context, handle string) (IssuedEnrollmentCode, error) {
	return s.id.Logins.IssueRecoveryCode(ctx, handle)
}

// Close writes the open aggregations of denials within ctx and closes the pool.
func (s *Setup) Close(ctx context.Context) {
	if s.audit != nil {
		_ = s.audit.Recorder().Aggregator().Close(ctx)
	}
	s.pool.Close()
}

// The input and results of the setup use cases for cmd/bothy, which may not import module
// packages (go-arch-lint): aliases of the identity types.
type (
	// BootstrapOwnerCmd is the input of bothy setup bootstrap-owner.
	BootstrapOwnerCmd = identityapp.BootstrapOwnerCmd
	// BootstrapResult is its outcome.
	BootstrapResult = identityapp.BootstrapResult
	// IssuedEnrollmentCode is a code with its display form, shown once.
	IssuedEnrollmentCode = identityapp.IssuedEnrollmentCode
)

// The answers of the setup use cases that cmd/bothy maps to exit codes (errors.Is).
var (
	// ErrSetupCompleted: bootstrap-owner found the setup complete and changed nothing.
	ErrSetupCompleted = identityapp.ErrSetupCompleted
	// ErrSetupPending: issue-enrollment-code before the setup.
	ErrSetupPending = identityapp.ErrSetupPending
	// ErrNotActiveOwner: issue-enrollment-code for a handle that is no active owner.
	ErrNotActiveOwner = identityapp.ErrNotActiveOwner
)
