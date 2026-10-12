// SPDX-License-Identifier: Apache-2.0

// Package jobs is the only place of the server that knows River, the job queue on PostgreSQL
// (docs/spec/02-architecture.md "Background jobs (River)", R-AR-09, ADR 0007 rule 8). Modules
// define job arguments and handlers against the River-free types of this package, so a breaking
// change of River (0.x) touches this package alone.
//
// Contents:
//
//   - Args, Job, Handler, HandlerOptions: the interface for job authors; ValidateArgsType keeps
//     arguments to IDs.
//   - Registry, Register, PeriodicJob: the handlers and periodic jobs of a process, checked at
//     registration (argument type, kind, queue).
//   - Runtime, New, Inserter: the River client of a role. api inserts only, worker and all also
//     run jobs; InsertTx joins the ambient transaction of the unit of work (pg.Tx).
//   - Migrate, MigrationStatus, ValidateMigrations, CheckEmbeddedMigrations: River's tables in
//     their own schema jobs.schema, run by bothy migrate and checked at the start of bothy serve.
//
// The test helper jobstest (subpackage) applies River's migrations to a pgtest database; only
// tests import it.
//
// Example:
//
//	reg := jobs.NewRegistry()
//	if err := jobs.Register(reg, scanHandler, jobs.HandlerOptions{Queue: jobs.QueueFiles}); err != nil {
//		return err
//	}
//	rt, err := jobs.New(pool, cfg, jobs.RoleWorker, reg, nil)
//	if err != nil {
//		return err
//	}
//	if err := rt.Start(ctx); err != nil {
//		return err
//	}
//	defer rt.Stop(stopCtx)
//	err = uow.Do(ctx, func(ctx context.Context) error {
//		return rt.Inserter().InsertTx(ctx, ScanArgs{File: fileID}, nil)
//	})
package jobs
