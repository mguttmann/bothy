-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key and the two checks of agents.hosted_by that 00094 added NOT VALID
-- (A1-03, docs/operations/upgrade.md rule M5): each is checked in its own step, under
-- migrate.lock_timeout (rule M6), without blocking writes for the scan.

-- +goose Up
ALTER TABLE agents VALIDATE CONSTRAINT agents_hosted_by_fkey;
ALTER TABLE agents VALIDATE CONSTRAINT agents_hosted_by_check;
ALTER TABLE agents VALIDATE CONSTRAINT agents_hosted_by_not_self_check;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00094 drops them.
SELECT 1;
