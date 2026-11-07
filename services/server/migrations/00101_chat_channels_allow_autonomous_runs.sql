-- SPDX-License-Identifier: Apache-2.0
-- The channel switch of autonomous runs (AR-04, docs/spec/03-data-model.md section 4.5 `channels`,
-- docs/spec/09-agents.md 5.8.6): persons other than owners may let agents run on alone in a channel
-- only when allow_autonomous_runs is true; changed with manage (channel.autonomous_runs.set).
--
-- Expand only (upgrade.md rules M1 and M3): a column with a constant default is added without a
-- table rewrite, every existing channel gets false ("as before", ADR 0077), and the previous binary
-- neither reads nor writes it.

-- +goose Up
ALTER TABLE channels ADD COLUMN allow_autonomous_runs boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE channels DROP COLUMN IF EXISTS allow_autonomous_runs;
