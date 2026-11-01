-- SPDX-License-Identifier: Apache-2.0
-- Queries on project_bindings (chat module, M2-03; docs/spec/03-data-model.md section 4.5). A
-- change locks the space row FOR SHARE and the channel row FOR UPDATE first (ShareSpace and
-- LockChannel), then the binding row; a readiness report locks the spaces FOR SHARE, the channels
-- FOR SHARE and then the bindings FOR UPDATE, each set in ascending id (03 section 6.3). The
-- project facts of authorize (project_bound, project_ready) come with the channel queries in
-- channels.sql and operators.sql through a LEFT JOIN on this table.

-- name: LockProjectBinding :one
-- The binding of one forum locked for a PUT or DELETE, read after authorize allowed
-- channel.project.bind on the channel.
SELECT channel_id, workspace_ref, repository_url, default_branch, runner_config, ready, created_by, created_at, version,
       provision, ready_reason
FROM project_bindings
WHERE channel_id = @channel_id
FOR UPDATE;

-- name: InsertProjectBinding :one
-- A new binding with ready false, no reason and version 1; created_at comes from the column default.
INSERT INTO project_bindings (channel_id, workspace_ref, repository_url, default_branch, runner_config, provision, created_by)
VALUES (@channel_id, @workspace_ref, sqlc.narg(repository_url), sqlc.narg(default_branch), @runner_config, @provision, @created_by)
RETURNING created_at;

-- name: ReplaceProjectBinding :exec
-- Replaces the whole binding of the locked row (HTTP PUT) with ready false, no reason, no folder
-- check (D-DEL-10: it checked the former folder) and the new version; created_by and created_at
-- stay.
UPDATE project_bindings
SET workspace_ref = @workspace_ref, repository_url = sqlc.narg(repository_url), default_branch = sqlc.narg(default_branch),
    runner_config = @runner_config, provision = @provision, ready = false, ready_reason = NULL, version = @version,
    trash_check_ok = NULL, trash_check_reason = NULL, trash_checked_at = NULL
WHERE channel_id = @channel_id;

-- name: DeleteProjectBinding :exec
DELETE FROM project_bindings WHERE channel_id = @channel_id;

-- name: LockReportedBindings :many
-- The bindings of the channels of a readiness report, locked in ascending channel id, with the
-- stored folder check (D-DEL-10).
SELECT channel_id, ready, ready_reason, trash_check_ok, trash_check_reason
FROM project_bindings
WHERE channel_id = ANY (@channel_ids::uuid[])
ORDER BY channel_id
FOR UPDATE;

-- name: SetProjectReady :exec
-- Applies one readiness report of the serving runner to the locked row; the reason is NULL with
-- ready true (R-DM-55).
UPDATE project_bindings SET ready = @ready, ready_reason = sqlc.narg(ready_reason), version = version + 1 WHERE channel_id = @channel_id;

-- name: ResetProjectReadiness :exec
-- Sets ready false and clears the reason with a new version for the bound forums among channel_ids
-- that are ready or have a reason: the effective operator of these channels changed (design note 7
-- of M2-03, R-DM-55). The folder check of the former runner goes too (D-DEL-10), without a
-- version: it is runtime state of the runner, not part of the binding.
UPDATE project_bindings
SET version = CASE WHEN ready OR ready_reason IS NOT NULL THEN version + 1 ELSE version END,
    ready = false, ready_reason = NULL, trash_check_ok = NULL, trash_check_reason = NULL, trash_checked_at = NULL
WHERE channel_id = ANY (@channel_ids::uuid[]) AND (ready OR ready_reason IS NOT NULL OR trash_check_ok IS NOT NULL);

-- name: BoundProjectFolders :many
-- The entries of names that the workspace_ref of a binding names, compared without case as the
-- unique index does; only these names leave, no channel and no row (listAgentProjectFolders,
-- M4-44).
SELECT n::text AS name
FROM unnest(@names::text[]) AS n
WHERE EXISTS (SELECT 1 FROM project_bindings WHERE lower(workspace_ref) = lower(n));
