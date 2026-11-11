-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Indexes for the retention jobs (docs/spec/20-privacy.md 4.1; audit finding data-layer-3). The
-- hourly participants of class security and the job retention.content read "the rows whose end
-- lies before the cutoff, oldest first" in batches; without an index on the end time every batch
-- reads its whole table. Deleting a device checks every foreign key that references devices;
-- without an index on the referencing column each check reads the whole child table.
--
-- 1. End time: refresh_tokens (expires_at), devices (revoked_at), terminal_sessions (closed_at)
--    and messages (created_at of the rows that still hold content), each with id as the second
--    column, the order of the candidate queries. auth_sessions ends at LEAST(expires_at,
--    revoked_at); the expression index serves the condition and the order of that query as it is.
-- 2. Foreign keys to devices: updated_by_device_id of read_states, drafts, user_settings,
--    user_space_states and user_channel_states (ON DELETE SET NULL) and device_id of
--    push_registrations (ON DELETE CASCADE).
--
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4); the DROP ... IF EXISTS before
-- each build removes an invalid leftover of an interrupted build. Additive only: no column
-- changes, the previous binary keeps working.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS refresh_tokens_expires_at_idx;
CREATE INDEX CONCURRENTLY refresh_tokens_expires_at_idx ON refresh_tokens (expires_at, id);
DROP INDEX CONCURRENTLY IF EXISTS auth_sessions_end_idx;
CREATE INDEX CONCURRENTLY auth_sessions_end_idx ON auth_sessions ((LEAST(expires_at, revoked_at)), id);
DROP INDEX CONCURRENTLY IF EXISTS devices_revoked_at_idx;
CREATE INDEX CONCURRENTLY devices_revoked_at_idx ON devices (revoked_at, id) WHERE revoked_at IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS terminal_sessions_closed_at_idx;
CREATE INDEX CONCURRENTLY terminal_sessions_closed_at_idx ON terminal_sessions (closed_at, id) WHERE closed_at IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS messages_content_created_idx;
CREATE INDEX CONCURRENTLY messages_content_created_idx ON messages (created_at, id)
    WHERE deleted_at IS NULL AND kind <> 'system';

DROP INDEX CONCURRENTLY IF EXISTS read_states_updated_by_device_id_idx;
CREATE INDEX CONCURRENTLY read_states_updated_by_device_id_idx ON read_states (updated_by_device_id)
    WHERE updated_by_device_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS drafts_updated_by_device_id_idx;
CREATE INDEX CONCURRENTLY drafts_updated_by_device_id_idx ON drafts (updated_by_device_id)
    WHERE updated_by_device_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS user_settings_updated_by_device_id_idx;
CREATE INDEX CONCURRENTLY user_settings_updated_by_device_id_idx ON user_settings (updated_by_device_id)
    WHERE updated_by_device_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS user_space_states_updated_by_device_id_idx;
CREATE INDEX CONCURRENTLY user_space_states_updated_by_device_id_idx ON user_space_states (updated_by_device_id)
    WHERE updated_by_device_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS user_channel_states_updated_by_device_id_idx;
CREATE INDEX CONCURRENTLY user_channel_states_updated_by_device_id_idx ON user_channel_states (updated_by_device_id)
    WHERE updated_by_device_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS push_registrations_device_id_idx;
CREATE INDEX CONCURRENTLY push_registrations_device_id_idx ON push_registrations (device_id);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS push_registrations_device_id_idx;
DROP INDEX CONCURRENTLY IF EXISTS user_channel_states_updated_by_device_id_idx;
DROP INDEX CONCURRENTLY IF EXISTS user_space_states_updated_by_device_id_idx;
DROP INDEX CONCURRENTLY IF EXISTS user_settings_updated_by_device_id_idx;
DROP INDEX CONCURRENTLY IF EXISTS drafts_updated_by_device_id_idx;
DROP INDEX CONCURRENTLY IF EXISTS read_states_updated_by_device_id_idx;
DROP INDEX CONCURRENTLY IF EXISTS messages_content_created_idx;
DROP INDEX CONCURRENTLY IF EXISTS terminal_sessions_closed_at_idx;
DROP INDEX CONCURRENTLY IF EXISTS devices_revoked_at_idx;
DROP INDEX CONCURRENTLY IF EXISTS auth_sessions_end_idx;
DROP INDEX CONCURRENTLY IF EXISTS refresh_tokens_expires_at_idx;
