-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key threads.parent_message_id -> messages(id) that 00032 added NOT VALID
-- (upgrade.md rule M5: the check runs without a write lock on threads). Before 00032 no thread
-- could name a parent message (createThread came with M1-56), so the validation finds nothing.

-- +goose Up
ALTER TABLE threads VALIDATE CONSTRAINT threads_parent_message_id_fkey;

-- +goose Down
-- A validated constraint stays valid; 00032 Down drops it.
SELECT 1;
