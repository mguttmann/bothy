// SPDX-License-Identifier: Apache-2.0

// Package jobstest prepares pgtest databases for code that runs the job runtime: it applies
// River's migrations as bothy migrate up does, which pgtest's template (goose only) lacks. Only
// tests import it.
//
// Example:
//
//	db := pgtest.New(t)
//	jobstest.Migrate(t, db, "jobs")
package jobstest

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// Migrate runs jobs.Migrate for schema on db as bothy_owner with the session settings of bothy
// migrate; it fails t on an error.
func Migrate(t testing.TB, db *pgtest.DB, schema string) {
	t.Helper()
	cfg := db.Config(pg.RoleOwner)
	pool := db.OpenPool(t, cfg.ForMigrate(cfg.LockTimeout))
	defer pool.Close()
	if _, err := jobs.Migrate(t.Context(), pool, schema); err != nil {
		t.Fatalf("jobstest: migrate River into schema %s: %v", schema, err)
	}
}
