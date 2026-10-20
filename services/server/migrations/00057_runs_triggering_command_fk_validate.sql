-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key runs.triggering_command_id -> commands(id) that 00056 added NOT VALID
-- (M2-16, docs/operations/upgrade.md rule M5): a new constraint on an existing table is checked in
-- its own step, under migrate.lock_timeout (rule M6), without blocking writes for the scan. Before
-- 00056 no run could carry a triggering_command_id (command runs come with M2-16), so the
-- validation finds nothing.

-- +goose Up
ALTER TABLE runs VALIDATE CONSTRAINT runs_triggering_command_id_fkey;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00056 drops it.
SELECT 1;
