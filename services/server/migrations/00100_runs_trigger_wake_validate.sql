-- SPDX-License-Identifier: Apache-2.0
-- Validates the constraints of wake runs that 00099 added NOT VALID (AR-04, docs/operations/upgrade.md
-- rule M5): each is checked in its own step, under migrate.lock_timeout (rule M6), without blocking
-- writes for the scan. Every stored run has trigger_kind message, command or interaction and no
-- triggering_wake_id, so the validation finds nothing.

-- +goose Up
ALTER TABLE runs VALIDATE CONSTRAINT runs_triggering_wake_id_fkey;
ALTER TABLE runs VALIDATE CONSTRAINT runs_triggering_wake_check;
ALTER TABLE runs VALIDATE CONSTRAINT runs_trigger_kind_check;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00099 drops them.
SELECT 1;
