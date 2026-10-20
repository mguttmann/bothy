-- SPDX-License-Identifier: Apache-2.0
-- backup_runs (docs/spec/03-datenmodell.md 4.12, M7-02): bothy-backupd records its runs as
-- bothy_backup, which may only select, insert and update this table; bothy migrate reads the last
-- successful run.

-- name: BackupRunsExists :one
-- A schema migrated before M7-02 has no backup_runs; the caller checks before it reads.
SELECT (to_regclass('public.backup_runs') IS NOT NULL)::boolean AS exists;

-- name: InsertBackupRun :exec
INSERT INTO backup_runs (id, kind, state, started_at)
VALUES (sqlc.arg(id), sqlc.arg(kind), 'running', sqlc.arg(started_at));

-- name: FinishBackupRun :execrows
-- Only a running row moves: a second finish of the same run changes nothing and the caller fails.
UPDATE backup_runs
SET state = sqlc.arg(state), finished_at = sqlc.arg(finished_at), size_bytes = sqlc.narg(size_bytes),
    manifest = sqlc.arg(manifest), checks = sqlc.arg(checks), error = sqlc.narg(error)
WHERE id = sqlc.arg(id) AND state = 'running';

-- name: LastBackupSuccess :one
-- The index (kind, started_at DESC) serves this per kind.
SELECT max(started_at)::timestamptz AS last_success
FROM backup_runs
WHERE state = 'succeeded' AND kind = ANY (sqlc.arg(kinds)::text[]);

-- name: LatestBackupRun :one
-- The newest succeeded run of the kinds with its manifest: the backup the restore test restores
-- (M7-03). The index (kind, started_at DESC) serves this per kind.
SELECT id, kind, started_at, manifest
FROM backup_runs
WHERE state = 'succeeded' AND kind = ANY (sqlc.arg(kinds)::text[])
ORDER BY started_at DESC, id DESC
LIMIT 1;
