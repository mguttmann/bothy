-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key grants.target_channel_id -> channels(id) that 00026 added NOT VALID
-- (upgrade.md rule M5: the check runs without a write lock on grants). authz refuses a grant on
-- an unknown channel, and no channel existed before 00026, so the validation finds nothing (M1-53).

-- +goose Up
ALTER TABLE grants VALIDATE CONSTRAINT grants_target_channel_id_fkey;

-- +goose Down
-- A validated constraint stays valid; 00026 Down drops it.
SELECT 1;
