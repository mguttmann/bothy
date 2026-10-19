// SPDX-License-Identifier: Apache-2.0

// Package app holds the backup use cases of the status module (brief M7-02,
// docs/spec/16-betrieb.md 8.1 and 8.2): one run per kind with its backup_runs row and its
// manifest, the scheduler that starts the kinds on their backup.* schedules with at most one run
// per slot, the owner's off-site expiry, and the samples of the WAL archive and the volumes for
// the metrics. The outgoing ports Executor, ManifestSource and Metrics are implemented by the
// adapters of the module; the inbound port status.BackupRuns by adapters/postgres.
//
// Example:
//
//	svc, err := app.New(cfg, app.Deps{Runs: runs, Exec: exec, Source: src, Metrics: m, Clock: clock.System(), Log: log})
//	err = svc.Run(ctx, domain.KindPGFull)
package app
