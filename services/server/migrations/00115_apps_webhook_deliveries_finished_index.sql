-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Index for the job retention.transient on webhook_deliveries (docs/spec/20-privacy.md 4.1, audit
-- finding srv-apps-billing-8): ExpiredWebhookDeliveries reads the finished deliveries whose end
-- (coalesce(delivered_at, created_at), docs/DECISIONS.md M7-05) lies before the cutoff, oldest
-- first. Without an index each batch read and sorted the whole table. Pending rows never expire,
-- so the index is partial.
--
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4), one index per file; the
-- DROP ... IF EXISTS before the build removes an invalid leftover of an interrupted build.
-- Additive only: the previous binary keeps working.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS webhook_deliveries_finished_end_idx;
CREATE INDEX CONCURRENTLY webhook_deliveries_finished_end_idx
    ON webhook_deliveries ((coalesce(delivered_at, created_at)), id) WHERE status <> 'pending';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS webhook_deliveries_finished_end_idx;
