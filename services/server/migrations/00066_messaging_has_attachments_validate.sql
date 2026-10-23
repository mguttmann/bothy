-- SPDX-License-Identifier: Apache-2.0
-- Validates messages_has_attachments_deleted_check that 00065 added NOT VALID (M3-07,
-- docs/operations/upgrade.md rule M5): the scan runs in its own step under migrate.lock_timeout
-- (rule M6) without blocking writes. Before 00065 no message could carry the flag, so the
-- validation finds nothing.

-- +goose Up
ALTER TABLE messages VALIDATE CONSTRAINT messages_has_attachments_deleted_check;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00065 drops it.
SELECT 1;
