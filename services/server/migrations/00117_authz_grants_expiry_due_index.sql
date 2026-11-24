-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Index for the job authz.expiry_sweep on grants (docs/spec/07-permissions.md 8.3, audit finding
-- srv-authz-11): ListDueExpiredGrants reads the expired grants not yet reported. Grants are never
-- deleted, so grants_expiry_idx (00005) also holds every grant reported long ago, and each run of
-- the sweep walked all of them. This index holds only the grants still to report; the reminder
-- and the list "Expiring soon" keep reading grants_expiry_idx.
--
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4), one index per file; the
-- DROP ... IF EXISTS before the build removes an invalid leftover of an interrupted build.
-- Additive only: the previous binary keeps working.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS grants_expiry_due_idx;
CREATE INDEX CONCURRENTLY grants_expiry_due_idx ON grants (expires_at)
    WHERE revoked_at IS NULL AND expires_at IS NOT NULL AND expiry_reported_at IS NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS grants_expiry_due_idx;
