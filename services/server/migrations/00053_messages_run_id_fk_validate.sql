-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key messages.run_id -> runs(id) that 00051 added NOT VALID (M2-05,
-- docs/operations/upgrade.md rule M5): a new constraint on an existing table is checked in its own
-- step, under migrate.lock_timeout (rule M6), without blocking writes for the scan. Before 00051 no
-- message could carry a run_id (anchors come with M2-05), so the validation finds nothing.

-- +goose Up
ALTER TABLE messages VALIDATE CONSTRAINT messages_run_id_fkey;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00051 drops it.
SELECT 1;
