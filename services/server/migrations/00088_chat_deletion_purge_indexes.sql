-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Deleting spaces, channels and chats (FEAT-delete-spaces-chats): partial indexes of the job
-- messaging.purge_deleted, which reads the deleted rows whose purge is still open in the order of
-- deleted_at, and of the heartbeat acknowledgement, which reads the open folder requests of one
-- runner (09 section 18.15). Built CONCURRENTLY outside a transaction (upgrade.md rule M4); the
-- DROP ... IF EXISTS before each build removes an invalid leftover of an interrupted build.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS spaces_purge_pending_idx;
CREATE INDEX CONCURRENTLY spaces_purge_pending_idx ON spaces (deleted_at) WHERE deleted_at IS NOT NULL AND purged_at IS NULL;
DROP INDEX CONCURRENTLY IF EXISTS channels_purge_pending_idx;
CREATE INDEX CONCURRENTLY channels_purge_pending_idx ON channels (deleted_at) WHERE deleted_at IS NOT NULL AND purged_at IS NULL;
DROP INDEX CONCURRENTLY IF EXISTS threads_purge_pending_idx;
CREATE INDEX CONCURRENTLY threads_purge_pending_idx ON threads (deleted_at) WHERE deleted_at IS NOT NULL AND purged_at IS NULL;
DROP INDEX CONCURRENTLY IF EXISTS project_bindings_trash_requested_idx;
CREATE INDEX CONCURRENTLY project_bindings_trash_requested_idx ON project_bindings (trash_agent_id) WHERE trash_state = 'requested';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS project_bindings_trash_requested_idx;
DROP INDEX CONCURRENTLY IF EXISTS threads_purge_pending_idx;
DROP INDEX CONCURRENTLY IF EXISTS channels_purge_pending_idx;
DROP INDEX CONCURRENTLY IF EXISTS spaces_purge_pending_idx;
