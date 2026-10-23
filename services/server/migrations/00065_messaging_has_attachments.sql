-- SPDX-License-Identifier: Apache-2.0
-- messages.has_attachments (M3-07, docs/spec/03-data-model.md section 4.6, R-DM-43): true exactly
-- when the message was created with at least one attachment and is not deleted. messaging sets it
-- in the INSERT of a message and resets it in the UPDATE that deletes the message. The constant
-- default adds the column without a table rewrite (docs/operations/upgrade.md rule M3); the CHECK
-- starts NOT VALID (rule M5) and 00066 validates it in its own step.

-- +goose Up
ALTER TABLE messages ADD COLUMN has_attachments boolean NOT NULL DEFAULT false;
ALTER TABLE messages ADD CONSTRAINT messages_has_attachments_deleted_check
    CHECK (deleted_at IS NULL OR NOT has_attachments) NOT VALID;

-- +goose Down
ALTER TABLE messages DROP CONSTRAINT messages_has_attachments_deleted_check;
ALTER TABLE messages DROP COLUMN has_attachments;
