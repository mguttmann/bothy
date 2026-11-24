// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

type internalKEK struct{}

func (internalKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(strings.Repeat("6b", 32)), nil
}

// TestAuditStop_BudgetEndsDuringWriteReportsOpenBuckets: when the stop budget ends while the
// aggregator is still inside a write, the buckets still open are reported as dropped instead of
// vanishing without a report.
func TestAuditStop_BudgetEndsDuringWriteReportsOpenBuckets(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	pool := db.OpenPool(t, cfg)
	var logs bytes.Buffer
	unit := pg.UnitConfig{RetryMax: 1, StatementTimeout: cfg.StatementTimeout, LockTimeout: cfg.LockTimeout, IdleInTxTimeout: cfg.IdleInTxTimeout}
	a, err := NewAudit(pool, unit, auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, internalKEK{}, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	// The aggregator is still inside a write: its goroutine has not ended.
	a.cancel, a.done = func() {}, make(chan struct{})
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New()})
	if err := a.recorder.RecordDenial(ctx, auditing.Entry{Action: "grant.create", Decision: auditing.DecisionDeny, Reason: auditing.ReasonI1Violation}); err != nil {
		t.Fatal(err)
	}
	stopCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.stop(stopCtx); err == nil {
		t.Fatal("stop returned nil after its budget ended")
	}
	if !strings.Contains(logs.String(), "aggregated audit denials dropped") {
		t.Fatalf("the open bucket was not reported as dropped; logs: %s", logs.String())
	}
}

// TestCheckMaintLogin_MissingPasswordFailsTheStart: the start check of the audit trim opens one
// connection as bothy_maint, so a missing migrate.maint_password_ref ends the start like the
// other secrets instead of failing every run of the job retention.audit later.
func TestCheckMaintLogin_MissingPasswordFailsTheStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v := loadValues(t, "BOTHY_MIGRATE__MAINT_PASSWORD_REF=file:"+filepath.Join(dir, "db_maint_password"))
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	open, err := maintPoolOpener(v, sp, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = checkMaintLogin(ctx, open)
	if err == nil || !strings.Contains(err.Error(), "migrate.maint_password_ref") {
		t.Fatalf("checkMaintLogin = %v, want an error naming migrate.maint_password_ref", err)
	}
}
