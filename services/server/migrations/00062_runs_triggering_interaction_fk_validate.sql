-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key runs.triggering_interaction_id -> interactions(id) that 00061 added
-- NOT VALID (M2-15, docs/operations/upgrade.md rule M5): a new constraint on an existing table is
-- checked in its own step, under migrate.lock_timeout (rule M6), without blocking writes for the
-- scan. Before 00061 no run could name an interaction, so the validation finds nothing.

-- +goose Up
ALTER TABLE runs VALIDATE CONSTRAINT runs_triggering_interaction_id_fkey;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00061 drops it.
SELECT 1;
