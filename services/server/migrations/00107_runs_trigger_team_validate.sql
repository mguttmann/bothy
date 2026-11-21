-- SPDX-License-Identifier: Apache-2.0
-- Validates the two checks of runs that 00106 added NOT VALID (FEAT-TEAM-CHANNEL-SERVER,
-- docs/operations/upgrade.md rule M5): each is checked in its own step, under
-- migrate.lock_timeout (rule M6), without blocking writes for the scan. Every stored run has
-- trigger_kind message, command, interaction or wake, so the validation finds nothing.

-- +goose Up
ALTER TABLE runs VALIDATE CONSTRAINT runs_triggering_check;
ALTER TABLE runs VALIDATE CONSTRAINT runs_trigger_kind_check;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00106 drops them.
SELECT 1;
