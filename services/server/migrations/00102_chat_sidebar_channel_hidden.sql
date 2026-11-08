-- SPDX-License-Identifier: Apache-2.0
-- Personal archive of a channel in the own sidebar (PROJ-ARCHIVE, docs/spec/03-data-model.md
-- section 4.5 `user_channel_states`, docs/spec/01-product.md 6.22): hidden_at is set while the
-- person has archived the channel for themselves and NULL while it shows in the normal list.
--
-- Expand only (upgrade.md rules M1 and M5): a new nullable column without a default needs no
-- rewrite and no scan, every existing row stays shown, and the previous binary neither reads nor
-- writes it. The grants of user_channel_states (00085) cover the column.

-- +goose Up
ALTER TABLE user_channel_states ADD COLUMN hidden_at timestamptz;

-- +goose Down
ALTER TABLE user_channel_states DROP COLUMN IF EXISTS hidden_at;
