-- SPDX-License-Identifier: Apache-2.0
-- Personal pins in the own sidebar (FEAT-PIN-CHANNELS, docs/spec/03-data-model.md section 4.5
-- `user_channel_states`, docs/spec/01-product.md 6.24): pinned_at is set while the person has
-- pinned the channel to the top of its space and NULL otherwise; pinned_threads lists the posts of
-- the channel the person pinned to the top of the forum, in the order they were pinned.
--
-- Expand only (upgrade.md rules M1 and M3): a nullable column without a default and an array
-- column with a constant default need no rewrite and no scan, every existing row stays unpinned,
-- and the previous binary neither reads nor writes them (its upsert names only its own columns,
-- so a rollback keeps the pins). The grants of user_channel_states (00085) cover the columns.

-- +goose Up
ALTER TABLE user_channel_states ADD COLUMN pinned_at timestamptz;
ALTER TABLE user_channel_states ADD COLUMN pinned_threads uuid[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE user_channel_states DROP COLUMN IF EXISTS pinned_threads;
ALTER TABLE user_channel_states DROP COLUMN IF EXISTS pinned_at;
