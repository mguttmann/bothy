// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"

	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// MaintUnits implements auditapp.MaintUnits (M7-05): every unit of work runs on a pool of the
// role bothy_maint that Do opens for it and closes afterwards, so the maintenance role is
// connected only while the trim runs (16 7.1). The registry of its units holds only the audit
// hook (auditapp.Chain.Hook), which numbers, chains and inserts the anchor entry in the same
// transaction as the trim; the chain reads and writes through the ambient transaction.
type MaintUnits struct {
	open    func(ctx context.Context) (*pg.Pool, error)
	unitCfg pg.UnitConfig
	reg     *txn.Registry
}

// Compile-time check.
var _ auditapp.MaintUnits = (*MaintUnits)(nil)

// NewMaintUnits returns the units over open, which returns a new pool of bothy_maint per call, the
// deadlines unitCfg and the audit hook chain. Errors: a nil open or chain, or a rejected hook.
func NewMaintUnits(open func(ctx context.Context) (*pg.Pool, error), unitCfg pg.UnitConfig, chain *auditapp.Chain) (*MaintUnits, error) {
	if open == nil || chain == nil {
		return nil, errors.New("wire: the maintenance units need a pool opener and the audit chain")
	}
	reg := txn.NewRegistry()
	if err := reg.SetHook(txn.PhaseAudit, chain.Hook); err != nil {
		return nil, err
	}
	reg.Seal()
	return &MaintUnits{open: open, unitCfg: unitCfg, reg: reg}, nil
}

// Do runs fn in one unit of work as bothy_maint and closes the pool afterwards.
func (m *MaintUnits) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	pool, err := m.open(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	return pg.NewUnitOfWork(pool, m.unitCfg, m.reg).Do(txn.Detach(ctx), fn)
}

// maintPoolOpener returns the opener of the pools of bothy_maint: the database.* keys of v with
// the user bothy_maint, the password of migrate.maint_password_ref (16 7.1) and one connection.
func maintPoolOpener(v *config.Values, sp *secrets.Provider, log *slog.Logger) (func(ctx context.Context) (*pg.Pool, error), error) {
	cfg, err := DatabaseConfig(v, "bothy-retention-audit")
	if err != nil {
		return nil, err
	}
	mc, err := MigrateSettings(v)
	if err != nil {
		return nil, err
	}
	cfg.User, cfg.PasswordRef, cfg.MaxConns = string(pg.RoleMaint), mc.MaintPasswordRef, 1
	return func(ctx context.Context) (*pg.Pool, error) { return pg.Open(ctx, cfg, sp, log) }, nil
}

// registerAuditRetention registers the participants of audit in the job retention.audit (M7-05):
// the trim over units with retention.audit, then the destruction of the IP keys with
// privacy.audit_ip_key_retention.
func registerAuditRetention(reg *retention.Registry, cfg RetentionConfig, units auditapp.MaintUnits, pool *pg.Pool,
	rec *auditapp.Recorder, c clock.Clock) error {
	repo := auditpg.NewRepository(pool)
	trim, err := auditapp.NewTrim(units, repo, rec)
	if err != nil {
		return err
	}
	if err := reg.Register(trim, cfg.Audit); err != nil {
		return err
	}
	return reg.Register(auditapp.NewIPKeys(repo, c), cfg.IPKeyRetention)
}
