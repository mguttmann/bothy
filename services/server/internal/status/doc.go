// SPDX-License-Identifier: Apache-2.0

// Package status is the port of the status module: the status page, components, incidents,
// automatic checks and backup runs. Other modules import only this package
// (docs/spec/02-architecture.md, "Layers within a module").
//
// Since M7-02 it holds BackupRuns, the port of the table backup_runs: bothy-backupd records each
// backup run through it with the role bothy_backup, and bothy migrate up reads the last
// successful run before it applies a migration (docs/spec/16-operations.md 10 item 1). The backup
// rules live in domain, the use cases and the scheduler in app, the implementations of the ports
// in adapters.
//
// Example:
//
//	id, err := runs.Start(ctx, status.BackupLogical)
//	err = runs.Finish(ctx, status.FinishedRun{ID: id, Succeeded: true, SizeBytes: size, Manifest: manifest})
package status
