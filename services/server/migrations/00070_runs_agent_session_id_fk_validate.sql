-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key runs.agent_session_id -> agent_sessions(id) that 00069 added NOT VALID
-- (M2-12, docs/operations/upgrade.md rule M5): a new constraint on an existing table is checked in
-- its own step, under migrate.lock_timeout (rule M6), without blocking writes for the scan. Before
-- 00069 no run could carry a session (agent_sessions did not exist), so the validation finds nothing.

-- +goose Up
ALTER TABLE runs VALIDATE CONSTRAINT runs_agent_session_id_fkey;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00069 drops it.
SELECT 1;
