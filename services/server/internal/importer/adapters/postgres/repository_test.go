// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/importer/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMain starts the PostgreSQL container of the integration tests and removes it at the end.
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// TestRepository runs every method of the repository against PostgreSQL: sources with their
// keyset list, pending items with the conflict of an existing key, posting, the lists, the
// import control of a channel and the origin of a target; writes outside a unit fail.
func TestRepository(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute}, reg)
	owner := ids.New()
	if _, err := db.SuperConn(t).Exec(ctx, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'robin', 'Robin')`, owner); err != nil {
		t.Fatal(err)
	}
	r := postgres.NewRepository(pool)
	if _, err := r.InsertSource(ctx, importer.Source{ID: ids.New()}); err == nil {
		t.Fatal("a write outside a unit must fail")
	}
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	var sources []importer.Source
	channel := ids.New()
	do := func(fn func(ctx context.Context) error) {
		t.Helper()
		if err := uow.Do(ctx, fn); err != nil {
			t.Fatal(err)
		}
	}
	do(func(ctx context.Context) error {
		for i := range 3 {
			s, err := r.InsertSource(ctx, importer.Source{ID: ids.New(), Kind: "discord", Name: "Old", Config: json.RawMessage(`{"version":1}`),
				CreatedBy: owner, CreatedAt: at.Add(time.Duration(i) * time.Minute)})
			if err != nil {
				return err
			}
			sources = append(sources, s)
		}
		s, found, err := r.LockSource(ctx, sources[0].ID)
		if err != nil || !found || s.State != "draft" {
			t.Errorf("lock %+v %v %v", s, found, err)
		}
		s.State, s.StartedAt = "running", at
		s.Stats, s.Cursor = json.RawMessage(`{"server":{}}`), json.RawMessage(`{"run":{"id":"a"}}`)
		return r.SaveSource(ctx, s)
	})
	got, found, err := r.Source(ctx, sources[0].ID)
	if err != nil || !found || got.State != "running" || got.StartedAt.IsZero() || string(got.Cursor) != `{"run": {"id": "a"}}` {
		t.Errorf("source %+v %s %v %v", got, got.Cursor, found, err)
	}
	if _, found, _ := r.Source(ctx, ids.New()); found {
		t.Error("unknown source found")
	}
	page, err := r.Sources(ctx, nil, 2)
	if err != nil || len(page) != 2 || page[0].ID != sources[2].ID {
		t.Fatalf("first page %+v %v", page, err)
	}
	page, err = r.Sources(ctx, &app.SourceKey{At: page[1].CreatedAt, ID: page[1].ID}, 2)
	if err != nil || len(page) != 1 || page[0].ID != sources[0].ID {
		t.Fatalf("second page %+v %v", page, err)
	}
	src := sources[0].ID
	do(func(ctx context.Context) error {
		row, created, err := r.InsertPendingItem(ctx, app.NewItem{ID: ids.New(), SourceID: src, OriginKey: "channel:1", ItemKind: "channel",
			ContentSHA256: []byte{1, 2}, At: at})
		if err != nil || !created || row.State != "pending" {
			t.Errorf("insert %+v %v %v", row, created, err)
		}
		if _, created, err := r.InsertPendingItem(ctx, app.NewItem{ID: ids.New(), SourceID: src, OriginKey: "channel:1", ItemKind: "channel", At: at}); err != nil || created {
			t.Errorf("an existing key was inserted again: %v %v", created, err)
		}
		locked, found, err := r.LockItem(ctx, src, "channel:1")
		if err != nil || !found || locked.ID != row.ID || string(locked.ContentSHA256) != string([]byte{1, 2}) {
			t.Errorf("lock item %+v %v %v", locked, found, err)
		}
		if _, err := r.FinishItem(ctx, row.ID, "posted", "channel", channel, at); err != nil {
			return err
		}
		skip, _, err := r.InsertPendingItem(ctx, app.NewItem{ID: ids.New(), SourceID: src, OriginKey: "user:2", ItemKind: "user", At: at})
		if err != nil {
			return err
		}
		_, err = r.FinishItem(ctx, skip.ID, "skipped", "", ids.ID{}, at)
		return err
	})
	if it, found, err := r.Item(ctx, src, "channel:1"); err != nil || !found || it.TargetID != channel || it.TargetKind != "channel" {
		t.Errorf("item %+v %v %v", it, found, err)
	}
	if _, found, _ := r.Item(ctx, src, "channel:9"); found {
		t.Error("unknown item found")
	}
	all, err := r.Items(ctx, src, "", ids.ID{}, 10)
	if err != nil || len(all) != 2 {
		t.Errorf("items %+v %v", all, err)
	}
	if rest, err := r.Items(ctx, src, "", all[0].ID, 10); err != nil || len(rest) != 1 {
		t.Errorf("after %+v %v", rest, err)
	}
	if skipped, err := r.Items(ctx, src, "skipped", ids.ID{}, 10); err != nil || len(skipped) != 1 || skipped[0].TargetKind != "" {
		t.Errorf("skipped %+v %v", skipped, err)
	}
	if posted, err := r.ItemsOfKind(ctx, src, "channel", "posted"); err != nil || len(posted) != 1 {
		t.Errorf("of kind %+v %v", posted, err)
	}
	if c, err := r.ControlledChannel(ctx, channel); err != nil || !c {
		t.Errorf("controlled %v %v", c, err)
	}
	if o, found, err := r.OriginOfTarget(ctx, "channel", channel); err != nil || !found || o.OriginKey != "channel:1" {
		t.Errorf("origin %+v %v %v", o, found, err)
	}
	do(func(ctx context.Context) error {
		s, _, err := r.LockSource(ctx, src)
		if err != nil {
			return err
		}
		s.State, s.FinishedAt = "completed", at
		return r.SaveSource(ctx, s)
	})
	if c, err := r.ControlledChannel(ctx, channel); err != nil || c {
		t.Errorf("a completed source controls nothing: %v %v", c, err)
	}
}
