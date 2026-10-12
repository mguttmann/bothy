// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// idempotencyKeysVersion is the migration that creates idempotency_keys (M1-20).
const idempotencyKeysVersion = 7

// TestMigration_MatchesDataModel compares idempotency_keys with the table of
// docs/spec/03-datenmodell.md (module sync): columns, types, nullability, defaults, the CHECK of
// state, primary key, foreign key and the index for the cleanup job, read from pg_catalog.
func TestMigration_MatchesDataModel(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	super := db.SuperConn(t)

	want := []column{
		{"principal_id", "uuid", "NO", ""},
		{"key", "uuid", "NO", ""},
		{"action", "text", "NO", ""},
		{"request_sha256", "bytea", "NO", ""},
		{"state", "text", "NO", "'in_progress'::text"},
		{"response_status", "smallint", "YES", ""},
		{"response_body", "jsonb", "YES", ""},
		{"resource_id", "uuid", "YES", ""},
		{"created_at", "timestamp with time zone", "NO", "now()"},
		{"expires_at", "timestamp with time zone", "NO", ""},
	}
	if got := columns(t, super, "idempotency_keys"); !slices.Equal(got, want) {
		t.Errorf("idempotency_keys columns\n got %v\nwant %v", got, want)
	}
	wantCons := []string{
		"c CHECK ((state = ANY (ARRAY['in_progress'::text, 'completed'::text])))",
		"f FOREIGN KEY (principal_id) REFERENCES principals(id) ON DELETE CASCADE",
		"p PRIMARY KEY (principal_id, key)",
	}
	if got := constraints(t, super, "idempotency_keys"); !slices.Equal(got, wantCons) {
		t.Errorf("idempotency_keys constraints\n got %v\nwant %v", got, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX idempotency_keys_expires_at_idx ON public.idempotency_keys USING btree (expires_at)",
		"CREATE UNIQUE INDEX idempotency_keys_pkey ON public.idempotency_keys USING btree (principal_id, key)",
	}
	if got := indexes(t, super, "idempotency_keys"); !slices.Equal(got, wantIdx) {
		t.Errorf("idempotency_keys indexes\n got %v\nwant %v", got, wantIdx)
	}
}

// TestMigration_IdempotencyKeysDown removes the table and applies it again.
func TestMigration_IdempotencyKeysDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	provider, exists := migrateProvider(t, db)
	if _, err := provider.DownTo(ctx, idempotencyKeysVersion-1); err != nil {
		t.Fatalf("down: %v", err)
	}
	if exists("idempotency_keys") {
		t.Error("idempotency_keys survives the down migration")
	}
	if _, err := provider.Up(ctx); err != nil || !exists("idempotency_keys") {
		t.Fatalf("up after down: %v", err)
	}
}
