// SPDX-License-Identifier: Apache-2.0

package agentstest_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/agentstest"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestInsertImportedSession writes import rows with the helper and reads them back through the
// session store: is_current false, started true, end_reason import, ended_at equal to created_at,
// the baseline, the given currency or the column default; an unknown thread fails the test.
func TestInsertImportedSession(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	ctx := context.Background()
	alex, lotse, space, channel, thread := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, []any{alex.String()}},
		{`INSERT INTO users (principal_id) VALUES ($1)`, []any{alex.String()}},
		{`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'lotse', 'Lotse')`, []any{lotse.String()}},
		{`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, []any{space.String(), alex.String()}},
		{`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
			VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, []any{channel.String(), space.String(), alex.String()}},
	} {
		if _, err := owner.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := uow.Do(ctx, func(ctx context.Context) error {
		return postgres.NewRepository(pool).InsertAgent(ctx, domain.NewAgent(lotse, "hermes", "", domain.OperatorConfig{}))
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`,
		thread.String(), channel.String(), alex.String()); err != nil {
		t.Fatal(err)
	}

	before := time.Now().Add(-time.Minute)
	first := agentstest.InsertImportedSession(t, pool, agentstest.ImportedSession{
		ThreadID: thread, AgentID: lotse, ExternalSessionRef: "sess-old", CostBaselineMicros: 2500, Currency: "EUR",
	})
	second := agentstest.InsertImportedSession(t, owner, agentstest.ImportedSession{
		ThreadID: thread, AgentID: lotse, ExternalSessionRef: "sess-old",
	})
	rows, err := postgres.NewSessionStore(pool).ThreadSessions(ctx, thread)
	if err != nil || len(rows) != 2 {
		t.Fatalf("ThreadSessions = %+v, %v", rows, err)
	}
	byID := map[ids.ID]agents.Session{rows[0].ID: rows[0], rows[1].ID: rows[1]}
	for id, want := range map[ids.ID]struct {
		baseline int64
		currency string
	}{first: {2500, "EUR"}, second: {0, "USD"}} {
		got, ok := byID[id]
		if !ok || got.IsCurrent || !got.Started || got.EndReason != agents.EndImport || !got.EndedAt.Equal(got.CreatedAt) ||
			got.CreatedAt.Before(before) || got.CostBaselineMicros != want.baseline || got.Currency != want.currency ||
			got.ThreadID != thread || got.AgentID != lotse || got.ExternalSessionRef != "sess-old" || got.Model != "" || got.Version != 1 {
			t.Errorf("import row %s = %+v, %v", id, got, ok)
		}
	}

	rec := &recorder{TB: t}
	agentstest.InsertImportedSession(rec, pool, agentstest.ImportedSession{ThreadID: ids.New(), AgentID: lotse, ExternalSessionRef: "x"})
	if !rec.failed {
		t.Fatal("InsertImportedSession for an unknown thread did not fail")
	}
}
