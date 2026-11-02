-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- FIX-delete-releases-binding: the unique index on lower(workspace_ref) covers only bindings that
-- are not released, so the folder of a purged forum can be bound to another forum (03
-- project_bindings, R-DM-57). The new index is built CONCURRENTLY outside a transaction
-- (upgrade.md rule M4) next to the old one, then the old one goes and the new one takes its name,
-- which the code of this and of the previous version maps to 409 conflict. Every step can run
-- again after an interruption: the first DROP removes an invalid leftover of an interrupted build.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS project_bindings_workspace_ref_live_key;
CREATE UNIQUE INDEX CONCURRENTLY project_bindings_workspace_ref_live_key ON project_bindings (lower(workspace_ref))
    WHERE released_at IS NULL;
DROP INDEX CONCURRENTLY IF EXISTS project_bindings_workspace_ref_key;
ALTER INDEX project_bindings_workspace_ref_live_key RENAME TO project_bindings_workspace_ref_key;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS project_bindings_workspace_ref_all_key;
CREATE UNIQUE INDEX CONCURRENTLY project_bindings_workspace_ref_all_key ON project_bindings (lower(workspace_ref));
DROP INDEX CONCURRENTLY IF EXISTS project_bindings_workspace_ref_key;
ALTER INDEX project_bindings_workspace_ref_all_key RENAME TO project_bindings_workspace_ref_key;
