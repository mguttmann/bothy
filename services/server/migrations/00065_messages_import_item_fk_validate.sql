-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key messages.import_item_id -> import_items(id) that 00064 added NOT VALID
-- (M8-03, docs/operations/upgrade.md rule M5): a new constraint on an existing table is checked in
-- its own step, under migrate.lock_timeout (rule M6), without blocking writes for the scan. Before
-- 00064 no message could carry an import_item_id (imported messages come with M8-11), so the
-- validation finds nothing.

-- +goose Up
ALTER TABLE messages VALIDATE CONSTRAINT messages_import_item_id_fkey;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00064 drops it.
SELECT 1;
