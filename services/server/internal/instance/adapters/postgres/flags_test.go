// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestFlagRepository_WritesAndLocks: the first write inserts version 1 (a second insert reports
// inserted false), a later write raises the version, the rollout is stored in the normalized
// shape of feature_flag_rollout.v1 ({} when empty, empty lists omitted) and read back, and
// LockFlag needs a unit of work (M1-47).
func TestFlagRepository_WritesAndLocks(t *testing.T) {
	e := newEnv(t, 4)
	ctx := context.Background()
	repo := postgres.NewRepository(e.pool)
	owner := ids.New()
	conn := e.db.SuperConn(t)
	if _, err := conn.Exec(ctx, `INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'alex', 'Alex')`,
		owner.String()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.LockFlag(ctx, "dm.enabled"); err == nil {
		t.Fatal("LockFlag outside a unit of work did not fail")
	}
	if _, found, err := repo.GetFlag(ctx, "dm.enabled"); err != nil || found {
		t.Fatalf("empty table: %v %v", found, err)
	}
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	sam, research := ids.New(), ids.New()
	err := e.uow(txn.NewRegistry()).Do(ctx, func(ctx context.Context) error {
		if _, found, err := repo.LockFlag(ctx, "dm.enabled"); err != nil || found {
			t.Fatalf("lock without row: %v %v", found, err)
		}
		row, inserted, err := repo.InsertFlag(ctx, app.FlagRow{Key: "dm.enabled", Enabled: true, UpdatedBy: owner, UpdatedAt: at})
		if err != nil || !inserted || row.Version != 1 || !row.Enabled || !row.Rollout.Empty() || row.UpdatedBy != owner || !row.UpdatedAt.Equal(at) {
			t.Fatalf("insert: %+v %v %v", row, inserted, err)
		}
		if _, inserted, err = repo.InsertFlag(ctx, app.FlagRow{Key: "dm.enabled", UpdatedAt: at}); err != nil || inserted {
			t.Fatalf("second insert: %v %v", inserted, err)
		}
		row, err = repo.UpdateFlag(ctx, app.FlagRow{Key: "dm.enabled", Rollout: domain.Rollout{UserIDs: []ids.ID{sam}}, UpdatedBy: owner, UpdatedAt: at})
		if err != nil || row.Version != 2 || row.Enabled || !slices.Equal(row.Rollout.UserIDs, []ids.ID{sam}) {
			t.Fatalf("update: %+v %v", row, err)
		}
		if row, found, err := repo.LockFlag(ctx, "dm.enabled"); err != nil || !found || row.Version != 2 {
			t.Fatalf("locked: %+v %v %v", row, found, err)
		}
		_, _, err = repo.InsertFlag(ctx, app.FlagRow{Key: "voice.enabled", Rollout: domain.Rollout{GroupIDs: []ids.ID{research}}, UpdatedAt: at})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var dm, voice string
	if err := conn.QueryRow(ctx, `SELECT (SELECT rollout::text FROM feature_flags WHERE key = 'dm.enabled'),
		(SELECT rollout::text FROM feature_flags WHERE key = 'voice.enabled')`).Scan(&dm, &voice); err != nil {
		t.Fatal(err)
	}
	if dm != `{"user_ids": ["`+sam.String()+`"]}` || voice != `{"group_ids": ["`+research.String()+`"]}` {
		t.Errorf("stored rollouts %s %s", dm, voice)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO feature_flags (key) VALUES ('aa.old')`); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListFlags(ctx)
	if err != nil || len(all) != 3 || all[0].Key != "aa.old" || !all[0].Rollout.Empty() || all[0].UpdatedBy != (ids.ID{}) ||
		!slices.Equal(all[2].Rollout.GroupIDs, []ids.ID{research}) {
		t.Fatalf("list: %+v %v", all, err)
	}
	if _, err := conn.Exec(ctx, `UPDATE feature_flags SET rollout = '{"user_ids": 5}' WHERE key = 'aa.old'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ListFlags(ctx); err == nil {
		t.Error("a rollout of a foreign shape must fail")
	}
}
