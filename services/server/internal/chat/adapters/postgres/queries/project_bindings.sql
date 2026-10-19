-- SPDX-License-Identifier: Apache-2.0
-- Queries on project_bindings (chat module, M2-03; docs/spec/03-datenmodell.md section 4.5). A
-- change locks the space row FOR SHARE and the channel row FOR UPDATE first (ShareSpace and
-- LockChannel), then the binding row; a readiness report locks the spaces FOR SHARE, the channels
-- FOR SHARE and then the bindings FOR UPDATE, each set in ascending id (03 section 6.3). The
-- project facts of authorize (project_bound, project_ready) come with the channel queries in
-- channels.sql and operators.sql through a LEFT JOIN on this table.

-- name: LockProjectBinding :one
-- The binding of one forum locked for a PUT or DELETE, read after authorize allowed
-- channel.project.bind on the channel.
SELECT channel_id, workspace_ref, repository_url, default_branch, runner_config, ready, created_by, created_at, version
FROM project_bindings
WHERE channel_id = @channel_id
FOR UPDATE;

-- name: InsertProjectBinding :one
-- A new binding with ready false and version 1; created_at comes from the column default.
INSERT INTO project_bindings (channel_id, workspace_ref, repository_url, default_branch, runner_config, created_by)
VALUES (@channel_id, @workspace_ref, sqlc.narg(repository_url), sqlc.narg(default_branch), @runner_config, @created_by)
RETURNING created_at;

-- name: ReplaceProjectBinding :exec
-- Replaces the whole binding of the locked row (HTTP PUT) with ready false and the new version;
-- created_by and created_at stay.
UPDATE project_bindings
SET workspace_ref = @workspace_ref, repository_url = sqlc.narg(repository_url), default_branch = sqlc.narg(default_branch),
    runner_config = @runner_config, ready = false, version = @version
WHERE channel_id = @channel_id;

-- name: DeleteProjectBinding :exec
DELETE FROM project_bindings WHERE channel_id = @channel_id;

-- name: LockReportedBindings :many
-- The bindings of the channels of a readiness report, locked in ascending channel id.
SELECT channel_id, ready
FROM project_bindings
WHERE channel_id = ANY (@channel_ids::uuid[])
ORDER BY channel_id
FOR UPDATE;

-- name: SetProjectReady :exec
-- Applies one readiness report of the serving runner to the locked row.
UPDATE project_bindings SET ready = @ready, version = version + 1 WHERE channel_id = @channel_id;

-- name: ResetProjectReadiness :exec
-- Sets ready false with a new version for the bound forums among channel_ids that are ready: the
-- effective operator of these channels changed (design note 7 of M2-03).
UPDATE project_bindings SET ready = false, version = version + 1 WHERE channel_id = ANY (@channel_ids::uuid[]) AND ready;
