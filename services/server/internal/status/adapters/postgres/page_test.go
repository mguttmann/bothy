// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
)

// TestDeleteAgentComponents_WaitsForPageWriters (D-M7-09-5, delta red team): an owner's
// transaction that links a new incident to the only component of an agent holds the lock on all
// components; DeleteAgentComponents (agents.AgentDisabled) takes the same lock before it decides
// which incidents to hide, so it sees the committed link and hides the incident that the cascade
// leaves without components. Without the lock it decided on a snapshot without the link and the
// incident became visible to everybody.
func TestDeleteAgentComponents_WaitsForPageWriters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app, owner := db.Pool(t, pg.RoleApp), db.Pool(t, pg.RoleOwner)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(app, pg.UnitConfig{StatementTimeout: time.Minute, LockTimeout: time.Minute, IdleInTxTimeout: time.Minute}, reg)

	person, agent, comp, incident := ids.New(), ids.New(), ids.New(), ids.New()
	for _, f := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'robin', 'Robin')`, []any{person.String()}},
		{`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'support', 'Support')`, []any{agent.String()}},
		{`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, []any{agent.String()}},
		{`INSERT INTO status_components (id, name, sort_key, health_source, agent_id) VALUES ($1, 'Support', '0', 'agent.presence', $2)`,
			[]any{comp.String(), agent.String()}},
	} {
		if _, err := owner.Exec(ctx, f.sql, f.args...); err != nil {
			t.Fatalf("%s: %v", f.sql, err)
		}
	}

	// The owner's writer: lock every component, open the incident on the agent component, hold.
	tx, err := app.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`SELECT id FROM status_components ORDER BY sort_key, id FOR UPDATE`, nil},
		{`INSERT INTO status_incidents (id, title, impact, started_at, created_by) VALUES ($1, 'Support answers late', 'minor', now(), $2)`,
			[]any{incident.String(), person.String()}},
		{`INSERT INTO status_incident_components (incident_id, component_id) VALUES ($1, $2)`, []any{incident.String(), comp.String()}},
	} {
		if _, err := tx.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}

	page := postgres.NewPage(app)
	pid, done := make(chan int32, 1), make(chan error, 1)
	go func() {
		done <- uow.Do(ctx, func(ctx context.Context) error {
			var p int32
			if err := pg.Querier(ctx, app).QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&p); err != nil {
				return err
			}
			pid <- p
			_, err := page.DeleteAgentComponents(ctx, agent)
			return err
		})
	}()
	// The delete waits for a lock of the writer; only then does the writer commit.
	var deleter int32
	select {
	case deleter = <-pid:
	case err := <-done:
		t.Fatalf("the unit of the delete ended early: %v", err)
	}
	deadline := time.Now().Add(testkit.Patience)
	for {
		var blocked bool
		if err := owner.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1)) > 0`, deleter).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("DeleteAgentComponents never waited for the writer")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(testkit.Patience):
		t.Fatal("DeleteAgentComponents did not finish")
	}
	var hidden bool
	if err := owner.QueryRow(ctx, `SELECT hidden FROM status_incidents WHERE id = $1`, incident.String()).Scan(&hidden); err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Fatal("the incident of the deleted agent component is not hidden")
	}
}
