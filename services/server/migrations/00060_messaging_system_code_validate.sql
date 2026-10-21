-- SPDX-License-Identifier: Apache-2.0
-- Validates messages_system_code_check that 00059 added again NOT VALID with the code
-- agent_session.resume_failed (M2-12 design note 1, docs/operations/upgrade.md rule M5): the check
-- runs in its own step, under migrate.lock_timeout (rule M6), without blocking writes for the scan.
-- Every stored code passed the CHECK of 00032, which the new one widens.

-- +goose Up
ALTER TABLE messages VALIDATE CONSTRAINT messages_system_code_check;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00059 replaces it.
SELECT 1;
