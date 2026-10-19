-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key principals.avatar_file_id -> files(id) that 00047 added NOT VALID
-- (M3-03, docs/operations/upgrade.md rule M5): a new constraint on an existing table is checked
-- in its own step, under migrate.lock_timeout (rule M6), without blocking writes for the scan.

-- +goose Up
ALTER TABLE principals VALIDATE CONSTRAINT principals_avatar_file_id_fkey;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00047 drops it.
SELECT 1;
