// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// AuditConfig binds the audit.* and privacy.audit_ip_* keys of v into the configuration of the
// audit module. The error names the key whose binding failed, never a value.
func AuditConfig(v *config.Values) (auditapp.Config, error) {
	var c auditapp.Config
	if err := config.Bind(v, &c); err != nil {
		return auditapp.Config{}, err
	}
	return c, nil
}

// newAuditChain returns the audit phase hook over pool: the counters of instance (only the phase
// hooks receive them, TestCountersOnlyUsedByPhaseHooks), the repository of audit and the log
// reporter. It opens no connection, so the golden test builds it without a database (pool nil).
func newAuditChain(pool *pg.Pool, log *slog.Logger) *auditapp.Chain {
	return auditapp.NewChain(InstanceCounters(pool), auditpg.NewRepository(pool), logs.New(orDiscard(log)))
}

// orDiscard returns log, or a logger that drops everything when log is nil.
func orDiscard(log *slog.Logger) *slog.Logger {
	if log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return log
}

// Audit is the audit module of a process: the unit of work whose registry holds the audit hook,
// the recorder (auditing.Recorder) that later tickets hand to the module constructors, and the
// aggregator that Start runs with the lifecycle.
type Audit struct {
	uow      *pg.UnitOfWork
	recorder *auditapp.Recorder
	modules  *Modules

	stopOnce sync.Once
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewAudit assembles the audit module over pool: the txn registry with the hooks of every module
// (NewTxnRegistry), the unit of work with unitCfg and the recorder with cfg, the key-encryption
// key source kek (SecretsKEK in production) and log for its reports. It opens no connection.
//
// Errors: a registry that cannot be built, and the errors of auditapp.NewRecorder.
func NewAudit(pool *pg.Pool, unitCfg pg.UnitConfig, cfg auditapp.Config, kek auditapp.KEKSource, log *slog.Logger) (*Audit, error) {
	log = orDiscard(log)
	mods := NewModules(pool, log)
	reg, err := NewTxnRegistry(mods)
	if err != nil {
		return nil, err
	}
	uow := pg.NewUnitOfWork(pool, unitCfg, reg)
	repo := auditpg.NewRepository(pool)
	rec, err := auditapp.NewRecorder(cfg, auditapp.Deps{UoW: uow, Store: repo, KEK: kek, Reporter: logs.New(log)})
	if err != nil {
		return nil, err
	}
	return &Audit{uow: uow, recorder: rec, modules: mods}, nil
}

// UoW returns the unit of work of the process; its registry runs the audit hook in phase 3.3.
func (a *Audit) UoW() txn.UnitOfWork { return a.uow }

// RightsHook returns the rights hooks of the registry of UoW; start step authz binds them to the
// appender of sync and the recorder (M1-29).
func (a *Audit) RightsHook() *authzapp.RightsHook { return a.modules.Authz }

// Recorder returns the recorder; it implements auditing.Recorder.
func (a *Audit) Recorder() *auditapp.Recorder { return a.recorder }

// Compile-time check: the recorder handed to the modules is the kernel port.
var _ auditing.Recorder = (*auditapp.Recorder)(nil)

// Start runs the aggregator of denials until the shutdown and registers the stop step
// "audit_denials" in lifecycle.PhaseDatabase: it ends the aggregator and writes every open
// aggregation (02 "15.3", 05 "2.4"). Call Start from a component's Start, after the stop step of
// the pool was registered: steps of one phase run in reverse registration order, so the flush runs
// before the pool closes. Both roles call it.
func (a *Audit) Start(r *lifecycle.Runner) {
	// The stop step calls cancel (Audit.stop); gosec cannot see that through the struct field.
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is called by the stop step audit_denials
	a.cancel, a.done = cancel, make(chan struct{})
	go func() {
		defer close(a.done)
		_ = a.recorder.Aggregator().Run(ctx)
	}()
	r.OnStop(lifecycle.StopStep{Name: "audit_denials", Phase: lifecycle.PhaseDatabase, Run: a.stop})
}

// stop ends the aggregator and flushes it within the budget of ctx; what cannot be written is
// reported as dropped (Aggregator.Close). When the budget ends while the aggregator is still inside
// a write, the buckets still open are closed as well, so they are reported instead of vanishing;
// the buckets of the write in flight stay with that write.
func (a *Audit) stop(ctx context.Context) error {
	a.stopOnce.Do(a.cancel)
	select {
	case <-a.done:
	case <-ctx.Done():
		return errors.Join(ctx.Err(), a.recorder.Aggregator().Close(ctx))
	}
	return a.recorder.Aggregator().Close(ctx)
}

// SecretsKEK is the KEK source of the audit module over the secret provider: it resolves the
// reference (privacy.audit_ip_kek_ref or the key_ref of a stored key) and returns the raw bytes.
type SecretsKEK struct {
	// Provider resolves the references.
	Provider *secrets.Provider
}

// Compile-time check: SecretsKEK is the outgoing port of the audit app.
var _ auditapp.KEKSource = SecretsKEK{}

// KEK returns the secret behind ref; the error never contains the secret.
func (k SecretsKEK) KEK(ctx context.Context, ref string) ([]byte, error) {
	v, _, err := k.Provider.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	return v.Bytes(), nil
}

// errAuditKEK reports a key-encryption key that is not 32 bytes of hex.
var errAuditKEK = errors.New("wire: privacy.audit_ip_kek_ref does not hold 32 bytes as hex")

// checkAuditKEK resolves privacy.audit_ip_kek_ref once, so a missing or malformed key fails the
// start instead of the first audited request with a client IP.
func checkAuditKEK(ctx context.Context, kek auditapp.KEKSource, ref string) error {
	raw, err := kek.KEK(ctx, ref)
	if err != nil {
		return err
	}
	if _, err := domain.DecodeKEK(raw); err != nil {
		return errAuditKEK
	}
	return nil
}
