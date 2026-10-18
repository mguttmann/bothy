-- SPDX-License-Identifier: Apache-2.0
-- Validates the foreign key grants.target_space_id -> spaces(id) that 00023 added NOT VALID
-- (upgrade.md rule M5: the check runs without a write lock on grants). No grant on a space can
-- exist before spaces did, so the validation finds nothing (M1-52).

-- +goose Up
ALTER TABLE grants VALIDATE CONSTRAINT grants_target_space_id_fkey;

-- +goose Down
-- A validated constraint stays valid; 00023 Down drops it.
SELECT 1;
