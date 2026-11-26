-- SPDX-License-Identifier: Apache-2.0
-- Validates the end reason check of agent_sessions that 00126 added NOT VALID (CU-04,
-- docs/operations/upgrade.md rule M5), under migrate.lock_timeout (rule M6), without blocking
-- writes for the scan. Every stored row has one of the earlier reasons or none.

-- +goose Up
ALTER TABLE agent_sessions VALIDATE CONSTRAINT agent_sessions_end_reason_check;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00126 drops it.
SELECT 1;
