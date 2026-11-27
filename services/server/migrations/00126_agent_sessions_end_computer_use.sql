-- SPDX-License-Identifier: Apache-2.0
-- agent_sessions.end_reason accepts 'computer_use' (CU-04, docs/spec/03-data-model.md
-- agent_sessions, docs/spec/27-computer-use.md 10.5): the run that ended had opened a computer
-- session, and the server ends the thread's current agent session in the transaction that ends
-- the run, so no screenshot of it is resumed.
--
-- Expand only (upgrade.md rules M5 and M8): the wider check replaces the old one NOT VALID, so it
-- holds for every new and changed row at once, and 00127 validates it without blocking writes.
-- Every stored row keeps its end reason.
--
-- Down: the old check comes back only while no session ended with computer_use; the step refuses
-- otherwise and names the count.

-- +goose Up
ALTER TABLE agent_sessions DROP CONSTRAINT agent_sessions_end_reason_check;
ALTER TABLE agent_sessions ADD CONSTRAINT agent_sessions_end_reason_check
    CHECK (end_reason IN ('reset', 'rebind', 'import', 'agent_changed', 'computer_use')) NOT VALID;

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    found bigint;
BEGIN
    SELECT count(*) INTO found FROM agent_sessions WHERE end_reason = 'computer_use';
    IF found > 0 THEN
        RAISE EXCEPTION 'agent sessions with end_reason computer_use exist: %', found;
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE agent_sessions DROP CONSTRAINT agent_sessions_end_reason_check;
ALTER TABLE agent_sessions ADD CONSTRAINT agent_sessions_end_reason_check
    CHECK (end_reason IN ('reset', 'rebind', 'import', 'agent_changed'));
