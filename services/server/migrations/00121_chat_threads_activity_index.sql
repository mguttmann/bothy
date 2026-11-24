-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- The forum list sorted by activity without a state filter (docs/spec/03-data-model.md threads,
-- audit finding data-layer-11): ListThreadsByActivity reads the live threads of one channel in the
-- keyset order (last_activity_at DESC, id DESC). threads_channel_state_activity_idx leads with
-- state, so a list of all states sorted every thread of the forum.
--
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4), one index per file; the
-- DROP ... IF EXISTS before the build removes an invalid leftover of an interrupted build.
-- Additive only: the previous binary keeps working.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS threads_channel_activity_idx;
CREATE INDEX CONCURRENTLY threads_channel_activity_idx ON threads (channel_id, last_activity_at DESC, id DESC)
    WHERE kind = 'thread' AND deleted_at IS NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS threads_channel_activity_idx;
