// SPDX-License-Identifier: Apache-2.0

// Package pg is the only way the server reaches PostgreSQL 18 (ADR 0006, ADR 0007): one pgx
// pool per process with the deadlines of docs/spec/02-architecture.md "Deadlines", the schema
// check at start, the four database roles and their bootstrap, and the migrator behind
// `bothy migrate` (migrations never run at `bothy serve`, R-AR-16).
//
// Contents:
//
//   - Config, Open and Pool: the pool. Every new connection resolves its password through the
//     secret provider, so a rotated password works after secrets.cache_ttl without a restart.
//     Acquire, Begin and Ping wait at most database.acquire_timeout; an overloaded pool becomes
//     the kernel/errs error of kind unavailable with the internal cause db_pool_exhausted.
//   - Timeouts and SetLocalTimeouts: per-transaction deadlines for the unit of work (M1-13).
//   - CheckSchema and RequiredSchemaVersion: the start refuses a schema older than the
//     binary needs; a newer one is allowed (expand and contract, docs/operations/upgrade.md).
//   - Role, Bootstrap, BootstrapRoles and BootstrapDatabase: least-privilege roles, created
//     idempotently; passwords reach the server only as SCRAM-SHA-256 verifiers.
//   - Migrator and Validate: embedded goose migrations with before and after steps.
//
// The test helper pgtest (subpackage) gives each test a fresh database in a disposable
// PostgreSQL container; only tests import it.
//
// Example:
//
//	pool, err := pg.Open(ctx, cfg, secretsProvider, logger)
//	if err != nil {
//		return err
//	}
//	defer pool.Close()
//	if err := pool.WaitReady(ctx, dependencyWait); err != nil {
//		return err
//	}
//	need, err := pg.RequiredSchemaVersion(migrations.FS)
//	if err != nil {
//		return err
//	}
//	return pg.CheckSchema(ctx, pool, need)
package pg
