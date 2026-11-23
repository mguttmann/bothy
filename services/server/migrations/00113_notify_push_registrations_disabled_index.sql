-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Index for the job retention.security on push_registrations (docs/spec/20-privacy.md 4.1, audit
-- finding srv-notify-14): ExpiredPushRegistrations reads the disabled registrations whose
-- disabled_at lies before the cutoff, oldest first. Without an index each batch read the whole
-- table, which keeps every disabled row until retention.security_after_end. Only disabled rows
-- carry the column, so the index is partial.
--
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4), one index per file; the
-- DROP ... IF EXISTS before the build removes an invalid leftover of an interrupted build.
-- Additive only: the previous binary keeps working.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS push_registrations_disabled_at_idx;
CREATE INDEX CONCURRENTLY push_registrations_disabled_at_idx ON push_registrations (disabled_at, id)
    WHERE disabled_at IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS push_registrations_disabled_at_idx;
