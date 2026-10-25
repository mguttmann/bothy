-- SPDX-License-Identifier: Apache-2.0
-- The job retention.ops (docs/spec/20-privacy.md 4.1, M7-09): incidents resolved before the cutoff
-- (their updates and component links go with them, ON DELETE CASCADE) and backup runs started
-- before it, except the latest succeeded run of each kind, which bothy migrate up and the restore
-- test read (D-M7-09-6). Each delete checks the rule again, so a row that changed in between stays.

-- name: ExpiredIncidents :many
SELECT id FROM status_incidents
WHERE state = 'resolved' AND resolved_at < sqlc.arg(cutoff)
ORDER BY resolved_at, id
LIMIT sqlc.arg(row_limit);

-- name: DeleteIncident :execrows
DELETE FROM status_incidents WHERE id = sqlc.arg(id) AND state = 'resolved' AND resolved_at < sqlc.arg(cutoff);

-- name: ExpiredBackupRuns :many
SELECT b.id FROM backup_runs b
WHERE b.started_at < sqlc.arg(cutoff)
  AND (b.state <> 'succeeded' OR EXISTS (
      SELECT 1 FROM backup_runs n
      WHERE n.kind = b.kind AND n.state = 'succeeded' AND n.started_at > b.started_at))
ORDER BY b.started_at, b.id
LIMIT sqlc.arg(row_limit);

-- name: DeleteBackupRun :execrows
DELETE FROM backup_runs b
WHERE b.id = sqlc.arg(id) AND b.started_at < sqlc.arg(cutoff)
  AND (b.state <> 'succeeded' OR EXISTS (
      SELECT 1 FROM backup_runs n
      WHERE n.kind = b.kind AND n.state = 'succeeded' AND n.started_at > b.started_at));
