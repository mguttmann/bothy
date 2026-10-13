// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// checkpointInsert inserts the checkpoint of seq $1 signed by key k1.
const checkpointInsert = `INSERT INTO audit_checkpoints (seq, hash, signature, kid) VALUES ($1::bigint, '\x01'::bytea, '\x02'::bytea, 'k1')`

// signingKeyInsert inserts the key k1 that checkpointInsert names.
const signingKeyInsert = `INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref) VALUES ('k1', 'audit_checkpoint', 'EdDSA', '{}', 'file:/k')`

// TestMigration_AuditCheckpointsMatchSpec compares audit_checkpoints with
// docs/spec/03-data-model.md 4.12: columns, types, nullability, defaults, keys and the partial
// index of the export (M1-50).
func TestMigration_AuditCheckpointsMatchSpec(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	super := db.SuperConn(t)
	want := []column{
		{"seq", "bigint", "NO", ""},
		{"hash", "bytea", "NO", ""},
		{"previous_checkpoint_hash", "bytea", "YES", ""},
		{"signature", "bytea", "NO", ""},
		{"kid", "text", "NO", ""},
		{"created_at", "timestamp with time zone", "NO", "now()"},
		{"exported_at", "timestamp with time zone", "YES", ""},
	}
	if got := columns(t, super, "audit_checkpoints"); !slices.Equal(got, want) {
		t.Errorf("audit_checkpoints columns\n got %v\nwant %v", got, want)
	}
	wantCons := []string{
		"f FOREIGN KEY (kid) REFERENCES server_signing_keys(kid) ON DELETE RESTRICT",
		"p PRIMARY KEY (seq)",
	}
	if got := constraints(t, super, "audit_checkpoints"); !slices.Equal(got, wantCons) {
		t.Errorf("audit_checkpoints constraints\n got %v\nwant %v", got, wantCons)
	}
	rows, err := super.Query(context.Background(), `SELECT indexdef FROM pg_indexes
		WHERE tablename = 'audit_checkpoints' AND indexdef NOT LIKE 'CREATE UNIQUE%' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	indexes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	wantIndexes := []string{
		"CREATE INDEX audit_checkpoints_unexported_idx ON public.audit_checkpoints USING btree (exported_at) WHERE (exported_at IS NULL)",
	}
	if !slices.Equal(indexes, wantIndexes) {
		t.Errorf("audit_checkpoints indexes\n got %v\nwant %v", indexes, wantIndexes)
	}
}

// TestSECT16_CheckpointsAppendOnly: the rights of audit_checkpoints are those of audit_events
// (03): bothy_app may insert and read, and set exported_at once; every other UPDATE, DELETE and
// TRUNCATE is refused, by the missing rights for bothy_app and by the trigger for every role,
// the table owner included.
func TestSECT16_CheckpointsAppendOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	owner := db.Pool(t, pg.RoleOwner)
	if _, err := owner.Exec(ctx, signingKeyInsert); err != nil {
		t.Fatal(err)
	}
	app := db.Pool(t, pg.RoleApp)
	if _, err := app.Exec(ctx, checkpointInsert, 1); err != nil {
		t.Fatalf("bothy_app cannot insert a checkpoint: %v", err)
	}
	if _, err := app.Exec(ctx, "UPDATE audit_checkpoints SET exported_at = now() WHERE seq = 1"); err != nil {
		t.Errorf("bothy_app cannot set exported_at: %v", err)
	}
	if _, err := app.Exec(ctx, "UPDATE audit_checkpoints SET exported_at = now() WHERE seq = 1"); err == nil ||
		!strings.Contains(err.Error(), "audit_checkpoints is append-only") {
		t.Errorf("a second export mark passed: %v", err)
	}
	for _, q := range []string{
		"UPDATE audit_checkpoints SET hash = '\\x03'",
		"UPDATE audit_checkpoints SET signature = '\\x03'",
		"DELETE FROM audit_checkpoints",
		"TRUNCATE audit_checkpoints",
	} {
		if _, err := app.Exec(ctx, q); sqlstate(err) != "42501" {
			t.Errorf("%s as bothy_app: %v, want 42501", q, err)
		}
		if _, err := owner.Exec(ctx, q); err == nil || !strings.Contains(err.Error(), "audit_checkpoints is append-only") {
			t.Errorf("%s as bothy_owner: %v, want the trigger to refuse", q, err)
		}
	}
	if _, err := owner.Exec(ctx, checkpointInsert, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "UPDATE audit_checkpoints SET exported_at = now(), kid = 'k1', hash = '\\x09' WHERE seq = 2"); err == nil {
		t.Error("an export mark that also changes the hash passed")
	}
}
