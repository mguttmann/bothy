-- SPDX-License-Identifier: Apache-2.0
-- Adds the system code agent_session.resume_failed to messages.system_code (M2-12 design note 1,
-- R-DM-42): 03 messages lists exactly ('thread.reopened', 'agent_session.resume_failed'), and
-- 00032 was built from a 03 that listed only thread.reopened. The fail-open line of 09 9.5 item 4
-- carries the new code. A new enumeration value is additive (docs/operations/upgrade.md rule M8):
-- the CHECK is dropped and added again with both codes NOT VALID (rule M5), and 00060 validates it.

-- +goose Up
ALTER TABLE messages DROP CONSTRAINT messages_system_code_check;
ALTER TABLE messages ADD CONSTRAINT messages_system_code_check
    CHECK (system_code IN ('thread.reopened', 'agent_session.resume_failed')) NOT VALID;

-- +goose Down
-- The CHECK of 00032 comes back NOT VALID, so the step does not fail on lines the newer server
-- wrote; the older code tolerates the unknown value (rule M8).
ALTER TABLE messages DROP CONSTRAINT messages_system_code_check;
ALTER TABLE messages ADD CONSTRAINT messages_system_code_check
    CHECK (system_code IN ('thread.reopened')) NOT VALID;
