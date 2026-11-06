// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// The migrations of agents.hosted_by (A1-03, R-DM-58): 00094 adds the column, its foreign key
// and the two checks NOT VALID behind a precheck, 00095 validates them and 00096 builds the
// partial index.

// beforeHostedBy is the last version before agents.hosted_by.
const beforeHostedBy = 93

// hostedProvider returns a migration provider on a fresh database migrated down to
// beforeHostedBy, with its SQL handle.
func hostedProvider(t *testing.T) (*goose.Provider, *sql.DB) {
	t.Helper()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(context.Background(), beforeHostedBy); err != nil {
		t.Fatalf("down to %d: %v", beforeHostedBy, err)
	}
	return provider, sqlDB
}

// execSQL runs stmt on db and fails the test on an error.
func execSQL(t *testing.T, db *sql.DB, stmt string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), stmt, args...); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

// insertAgent inserts an agent principal handle with its record of adapterKind.
func insertAgent(t *testing.T, db *sql.DB, id, handle, adapterKind string) {
	t.Helper()
	execSQL(t, db, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', $2, $2)`, id, handle)
	execSQL(t, db, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, $2)`, id, adapterKind)
}

// TestAgentsHostedByMigration_Precheck: a row with adapter_kind claude_agent before the column
// exists stops 00094 with a message that names the agent ids, and the migration changes nothing
// (design note 8 of the brief A1-03).
func TestAgentsHostedByMigration_Precheck(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider, db := hostedProvider(t)
	const early = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b01"
	insertAgent(t, db, early, "early", "claude_agent")
	_, err := provider.UpTo(ctx, beforeHostedBy+1)
	if err == nil || !strings.Contains(err.Error(), early) || !strings.Contains(err.Error(), "claude_agent") {
		t.Fatalf("up with a claude_agent row: %v, want the refusal naming %s", err, early)
	}
	var has bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_name = 'agents' AND column_name = 'hosted_by')`).Scan(&has); err != nil || has {
		t.Fatalf("hosted_by after the refused migration: %v %v", has, err)
	}
	var kind string
	if err := db.QueryRowContext(ctx, `SELECT adapter_kind FROM agents WHERE principal_id = $1`, early).Scan(&kind); err != nil || kind != "claude_agent" {
		t.Fatalf("the row after the refused migration: %q %v", kind, err)
	}
}

// TestAgentsHostedByMigration_KeepsExisting: a runner and an agent of another runtime with their
// API tokens, created before the migration, keep every value; their hosted_by is NULL; the new
// checks then hold for new rows (acceptance criterion 3 of A1-03).
func TestAgentsHostedByMigration_KeepsExisting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider, db := hostedProvider(t)
	const runner = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b11"
	const assistant = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b12"
	insertAgent(t, db, runner, "runner", "code_runner")
	insertAgent(t, db, assistant, "assistant", "hermes")
	for i, id := range []string{runner, assistant} {
		execSQL(t, db, `INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, scopes, dpop_jkt, expires_at, created_by)
			VALUES (gen_random_uuid(), $1, 'Adapter', 'bothy_agt_EXAMP', $2, '{}', 'jkt', now() + interval '1 hour', $1)`,
			id, []byte{byte(i), 1, 2, 3})
	}
	const snapshot = `SELECT string_agg((row_to_json(a)::jsonb - 'hosted_by')::text, '|' ORDER BY principal_id) FROM agents a`
	const tokens = `SELECT string_agg(row_to_json(t)::text, '|' ORDER BY principal_id) FROM api_tokens t`
	var agentsBefore, tokensBefore string
	if err := db.QueryRowContext(ctx, snapshot).Scan(&agentsBefore); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, tokens).Scan(&tokensBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	var agentsAfter, tokensAfter string
	if err := db.QueryRowContext(ctx, snapshot).Scan(&agentsAfter); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, tokens).Scan(&tokensAfter); err != nil {
		t.Fatal(err)
	}
	if agentsBefore != agentsAfter || tokensBefore != tokensAfter {
		t.Fatalf("rows changed by the migration:\nagents %s\n    -> %s\ntokens %s\n    -> %s", agentsBefore, agentsAfter, tokensBefore, tokensAfter)
	}
	var hosted int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM agents WHERE hosted_by IS NOT NULL`).Scan(&hosted); err != nil || hosted != 0 {
		t.Fatalf("hosted rows after the migration: %d %v", hosted, err)
	}
	// The checks of 03 for new and changed rows: claude_agent needs a runner, no other kind
	// takes one, and no agent hosts itself.
	const built = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b13"
	execSQL(t, db, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'writer', 'Writer')`, built)
	wantHostedState(t, db, "23514", "claude_agent without a runner", `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'claude_agent')`, built)
	wantHostedState(t, db, "23514", "a runner for another kind", `INSERT INTO agents (principal_id, adapter_kind, hosted_by) VALUES ($1, 'hermes', $2)`, built, runner)
	wantHostedState(t, db, "23503", "an unknown runner", `INSERT INTO agents (principal_id, adapter_kind, hosted_by) VALUES ($1, 'claude_agent', $2)`,
		built, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b99")
	execSQL(t, db, `INSERT INTO agents (principal_id, adapter_kind, hosted_by) VALUES ($1, 'claude_agent', $2)`, built, runner)
	wantHostedState(t, db, "23514", "hosted by itself", `UPDATE agents SET hosted_by = principal_id WHERE principal_id = $1`, built)
	wantHostedState(t, db, "23001", "a runner that hosts an agent is deleted", `DELETE FROM agents WHERE principal_id = $1`, runner)
}

// wantHostedState runs stmt and requires the SQLSTATE code.
func wantHostedState(t *testing.T, db *sql.DB, code, what, stmt string, args ...any) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), stmt, args...)
	if err == nil || !strings.Contains(err.Error(), code) {
		t.Fatalf("%s: %v, want SQLSTATE %s", what, err, code)
	}
}
