-- SPDX-License-Identifier: Apache-2.0
-- Forums for existing folders (M4-44): agents gets reported_project_folders as
-- docs/spec/03-data-model.md section 4.4 states it (R-DM-56): the folder names a runner reports in
-- the heartbeat member project_folders (09 section 18.13); NULL until the first report.
--
-- Expand only (upgrade.md rules M1 and M5): a new nullable column without a default needs no
-- rewrite and no scan. The previous binary neither reads nor writes it.

-- +goose Up
ALTER TABLE agents ADD COLUMN reported_project_folders text[];

-- +goose Down
ALTER TABLE agents DROP COLUMN reported_project_folders;
