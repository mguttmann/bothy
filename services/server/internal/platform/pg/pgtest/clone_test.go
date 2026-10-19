// SPDX-License-Identifier: Apache-2.0

package pgtest

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

// grantState is the state pg.BootstrapDatabase shapes on a database: its owner and ACL, its
// role settings (none), and owner and ACL of the schema public.
func grantState(t *testing.T, db *DB) []string {
	t.Helper()
	ctx := context.Background()
	conn := db.SuperConn(t)
	var dbOwner, dbACL, settings, nspOwner, nspACL string
	if err := conn.QueryRow(ctx, `SELECT d.datdba::regrole::text,
		coalesce(d.datacl::text, ''),
		coalesce((SELECT string_agg(array_to_string(s.setconfig, ','), ';') FROM pg_db_role_setting s WHERE s.setdatabase = d.oid), ''),
		n.nspowner::regrole::text, coalesce(n.nspacl::text, '')
		FROM pg_database d, pg_namespace n WHERE d.datname = current_database() AND n.nspname = 'public'`).
		Scan(&dbOwner, &dbACL, &settings, &nspOwner, &nspACL); err != nil {
		t.Fatal(err)
	}
	return []string{dbOwner, dbACL, settings, nspOwner, nspACL}
}

// TestCloneHasTheGrantsOfBootstrapDatabase: a clone of a template carries the same database
// and schema grants as a database bootstrapped with pg.BootstrapDatabase directly.
func TestCloneHasTheGrantsOfBootstrapDatabase(t *testing.T) {
	t.Parallel()
	want := grantState(t, New(t, WithoutMigrations()))
	got := grantState(t, New(t))
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("clone grant state %d = %q, want %q (pg.BootstrapDatabase)", i, got[i], want[i])
		}
	}
}

// TestCloneOpensNoSessionInTheNewDatabase: New clones without connecting to the clone (the
// first connection to a fresh database is the expensive part of New), so the test's own
// connection is the first one.
func TestCloneOpensNoSessionInTheNewDatabase(t *testing.T) {
	t.Parallel()
	db := New(t)
	keeper := New(t, WithoutMigrations())
	var sessions int64
	if err := keeper.SuperConn(t).QueryRow(context.Background(),
		"SELECT coalesce(sessions, 0) FROM pg_stat_database WHERE datname = $1", db.Name()).Scan(&sessions); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Errorf("New opened %d sessions in the new database, want 0", sessions)
	}
}

// TestParallelClonesAreIsolated: many tests clone the same template at once; every clone is
// a separate, migrated database.
func TestParallelClonesAreIsolated(t *testing.T) {
	t.Parallel()
	const n = 12
	var mu sync.Mutex
	seen := map[string]bool{}
	t.Run("clones", func(t *testing.T) {
		for i := range n {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				t.Parallel()
				db := New(t)
				mu.Lock()
				dup := seen[db.Name()]
				seen[db.Name()] = true
				mu.Unlock()
				if dup {
					t.Fatalf("two clones share the name %s", db.Name())
				}
				var version int64
				if err := db.SuperConn(t).QueryRow(context.Background(), "SELECT max(version_id) FROM goose_db_version").Scan(&version); err != nil {
					t.Fatal(err)
				}
				if version < 1 {
					t.Errorf("clone %s has schema version %d, want the migrated template", db.Name(), version)
				}
			})
		}
	})
	if len(seen) != n {
		t.Errorf("%d distinct clones, want %d", len(seen), n)
	}
}
