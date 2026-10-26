// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	appspg "github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	statusdomain "github.com/OWNER/bothy/services/server/internal/status/domain"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
)

// CheckRestoreNotFinalized is the readiness check that fails for good while the database
// identity differs from the one instance stores (R-DM-50, docs/spec/16-operations.md 8.4): the
// database was restored and bothy restore finalize has not run.
const CheckRestoreNotFinalized = "restore_not_finalized"

// msgRestoreNotFinalized is the log line of the start lock; it names the runbook step.
const msgRestoreNotFinalized = `database identity differs from the stored one: the database was restored or failed over; ` +
	`run "bothy restore finalize" before api and worker (docs/operations/backup-restore.md 5.2 step 6)`

// restoreGuardOff is the value of database.restore_guard that turns the start lock off.
const restoreGuardOff = "off"

// restoreGuard binds database.restore_guard.
type restoreGuard struct {
	Guard string `config:"database.restore_guard"`
}

// checkRestoreGuard is the start lock of R-DM-50 (brief M7-04 design note 3) after the instance
// row loaded: unless database.restore_guard is off, it compares the stored database identity
// with the current one, and a mismatch logs once, registers the readiness check
// restore_not_finalized, which fails for good, for every role, and holds the steps that serve or
// process (whileFinalized: log tail, presence tracker, jobs, worker consumers, public listener;
// the terminal reconcile asks finalized). An empty stored identity never blocks. The error is a failed read; it ends the start.
func (s *server) checkRestoreGuard(ctx context.Context, st instance.State) error {
	if s.role == roleRestore {
		// bothy restore finalize builds the modules on the restored database to end the lock.
		return nil
	}
	var g restoreGuard
	if err := config.Bind(s.values, &g); err != nil {
		return err
	}
	if g.Guard == restoreGuardOff {
		return nil
	}
	check, err := st.CheckIdentity(ctx)
	if err != nil {
		return fmt.Errorf("read the database identity (database.restore_guard): %w", err)
	}
	if !check.Mismatch {
		return nil
	}
	s.logger().ErrorContext(ctx, msgRestoreNotFinalized)
	blocked := errors.New("the database identity differs from the stored one; run bothy restore finalize")
	s.reg.AddReadiness(CheckRestoreNotFinalized, func(context.Context) error { return blocked })
	s.mu.Lock()
	s.blocked, s.restorePending = true, true
	s.mu.Unlock()
	return nil
}

// RestoreCursors returns the cursor ports of bothy restore finalize over pool, in the order of
// 03 6.7 item 1: the agent consumers (agents), the webhook subscriptions (apps) and the worker
// consumers (sync).
func RestoreCursors(pool *pg.Pool) []instanceapp.CursorMover {
	return []instanceapp.CursorMover{
		agentsapp.NewRestoreCursors(agentspg.NewRepository(pool)),
		appsapp.NewRestoreCursors(appspg.NewRepository(pool)),
		syncapp.NewRestoreCursors(syncpg.NewRepository(pool, nil)),
	}
}

// auditHead adapts the repository of audit to instanceapp.AuditHeadSource: the bounds of the
// chain and the hash of its last row as lowercase hex, as the manifest writes it.
type auditHead struct {
	repo *auditpg.Repository
}

// AuditHead implements instanceapp.AuditHeadSource.
func (h auditHead) AuditHead(ctx context.Context) (int64, string, error) {
	_, head, ok, err := h.repo.Bounds(ctx)
	if err != nil || !ok {
		return 0, "", err
	}
	hash, found, err := h.repo.HashAt(ctx, head)
	if err != nil {
		return 0, "", err
	}
	if !found {
		return 0, "", fmt.Errorf("audit: the head row %d has no hash", head)
	}
	return head, hex.EncodeToString(hash[:]), nil
}

// NewRestoreService returns bothy restore finalize (instance.Restore) over pool, which must
// connect as the schema owner (database.user = bothy_owner, like bothy migrate), the audit module
// a over the same pool, the cursor ports, normally RestoreCursors(pool), and the deletion journal,
// normally the RestoreJournal of the privacy module (Privacy.RestoreJournal, M7-07).
//
// Errors: a missing collaborator.
func NewRestoreService(pool *pg.Pool, a *Audit, cursors []instanceapp.CursorMover, journal instanceapp.DeletionJournal) (instance.Restore, error) {
	if pool == nil || a == nil || journal == nil {
		return nil, errors.New("wire: NewRestoreService needs the pool, the audit module and the deletion journal")
	}
	return instanceapp.NewRestore(instanceapp.RestoreDeps{
		Repo: instancepg.NewRepository(pool), UoW: a.UoW(), Audit: a.Recorder(), Cursors: cursors,
		AuditHead: auditHead{repo: auditpg.NewRepository(pool)}, ServerRole: string(pg.RoleApp), Journal: journal,
	})
}

// ParseRelocationManifest reads the facts bothy restore finalize --relocation compares from the
// manifest of the final backup (backup_manifest.v1, docs/operations/backup-restore.md 3). The
// manifest is checked as bothy-backupd writes it; an invalid one is an error that names the
// field, never a value.
func ParseRelocationManifest(b []byte) (instance.RelocationManifest, error) {
	m, err := statusdomain.ParseManifest(b)
	if err != nil {
		return instance.RelocationManifest{}, err
	}
	epoch, err := ids.Parse(m.SyncEpoch)
	if err != nil {
		return instance.RelocationManifest{}, fmt.Errorf("the manifest field sync_epoch is not a UUID: %w", err)
	}
	return instance.RelocationManifest{
		HeadSeq: m.EventLog.HeadSeq, AuditLastSeq: m.Audit.LastSeq, AuditLastHash: m.Audit.LastHash, SyncEpoch: epoch,
	}, nil
}

// The outcome and the refusals of bothy restore finalize for cmd/bothy, which may not import
// module packages (go-arch-lint).
type (
	// RestoreResult is the outcome of a finalize.
	RestoreResult = instance.FinalizeResult
	// RelocationMismatchError names the manifest field that differs.
	RelocationMismatchError = instance.RelocationMismatchError
)

// ErrServerConnected is the refusal while a session of bothy_app is connected.
var ErrServerConnected = instance.ErrServerConnected

// RestoreFinalize is the part of the server that bothy restore finalize needs: the modules of a
// server of roleRestore over the pool of database.user (the schema owner, as for bothy migrate),
// so the deletion journal deletes again through the normal use cases (M7-07), and the restore use
// case. It serves nothing, runs no job and needs no running api or worker. Close it when the
// command is done.
type RestoreFinalize struct {
	s       *server
	restore instance.Restore
}

// NewRestoreFinalize assembles bothy restore finalize over the loaded values v and the process
// environment env, with log for its reports: the start parts of the server from the secrets to
// the privacy module (restoreModules: the secret provider, the database within
// startup.dependency_wait, the schema check against the embedded migrations, every module, the
// object store of files) and the use case. It needs the configuration and the secrets of the
// server, like a worker (docs/operations/backup-restore.md 5.2 step 6). Errors: a configuration
// that does not bind, a missing secret, an unreachable or too old database, the error of a module
// start; none contains a secret.
func NewRestoreFinalize(ctx context.Context, v *config.Values, env []string, log *slog.Logger) (*RestoreFinalize, error) {
	s, err := restoreModules(ctx, v, env, orDiscard(log))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	pool, a, p := s.pool, s.audit, s.privacy
	s.mu.Unlock()
	if p == nil || p.Journal == nil {
		s.close(ctx)
		return nil, errNoPrivacy
	}
	rs, err := NewRestoreService(pool, a, RestoreCursors(pool), p.RestoreJournal())
	if err != nil {
		s.close(ctx)
		return nil, err
	}
	return &RestoreFinalize{s: s, restore: rs}, nil
}

// Finalize runs bothy restore finalize: after a restore with manifest nil, after a relocation
// with the manifest of the final backup (ParseRelocationManifest). Errors: ErrServerConnected,
// a *RelocationMismatchError, the error of an invalid manifest, the database error.
func (r *RestoreFinalize) Finalize(ctx context.Context, manifest []byte) (RestoreResult, error) {
	cmd := instance.FinalizeRestore{}
	if manifest != nil {
		m, err := ParseRelocationManifest(manifest)
		if err != nil {
			return RestoreResult{}, err
		}
		cmd.Relocation = &m
	}
	return r.restore.Finalize(ctx, cmd)
}

// Close runs the stop steps of the modules within shutdown.total_timeout: the open aggregations
// of denials, the sweepers, the pool and the object store.
func (r *RestoreFinalize) Close(ctx context.Context) {
	r.s.close(ctx)
}
