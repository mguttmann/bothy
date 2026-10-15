// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestSettingsRepository_WritesAndLocks: the first write inserts version 1 (a second insert of the
// same key reports inserted false), a later write raises the version, jsonb comes back in its own
// canonical form, NULL updated_by stays zero, and LockSettings needs a unit of work and returns
// only the stored rows of the asked keys (M1-46).
func TestSettingsRepository_WritesAndLocks(t *testing.T) {
	e := newEnv(t, 4)
	ctx := context.Background()
	repo := postgres.NewRepository(e.pool)
	owner := ids.New()
	if _, err := e.db.SuperConn(t).Exec(ctx, `INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'alex', 'Alex')`,
		owner.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LockSettings(ctx, []string{"locale.default"}); err == nil {
		t.Fatal("LockSettings outside a unit of work did not fail")
	}
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	uow := e.uow(txn.NewRegistry())
	err := uow.Do(ctx, func(ctx context.Context) error {
		rows, err := repo.LockSettings(ctx, []string{"locale.default", "branding.product_name"})
		if err != nil || len(rows) != 0 {
			t.Fatalf("empty table: %+v %v", rows, err)
		}
		row, inserted, err := repo.InsertSetting(ctx, app.SettingRow{Key: "locale.default", Value: json.RawMessage(`"en"`), UpdatedBy: owner, UpdatedAt: at})
		if err != nil || !inserted || row.Version != 1 || row.UpdatedBy != owner || !row.UpdatedAt.Equal(at) {
			t.Fatalf("insert: %+v %v %v", row, inserted, err)
		}
		if _, inserted, err = repo.InsertSetting(ctx, app.SettingRow{Key: "locale.default", Value: json.RawMessage(`"de"`), UpdatedAt: at}); err != nil || inserted {
			t.Fatalf("second insert: %v %v", inserted, err)
		}
		row, err = repo.UpdateSetting(ctx, app.SettingRow{Key: "locale.default", Value: json.RawMessage(`"A&B"`), UpdatedAt: at.Add(time.Second)})
		if err != nil || row.Version != 2 || !row.UpdatedBy.IsZero() || string(row.Value) != `"A&B"` {
			t.Fatalf("update: %+v %s %v", row, row.Value, err)
		}
		rows, err = repo.LockSettings(ctx, []string{"locale.default", "branding.product_name"})
		if err != nil || len(rows) != 1 || rows[0].Version != 2 {
			t.Fatalf("locked: %+v %v", rows, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListSettings(ctx)
	if err != nil || len(all) != 1 || all[0].Key != "locale.default" {
		t.Fatalf("list: %+v %v", all, err)
	}
}
