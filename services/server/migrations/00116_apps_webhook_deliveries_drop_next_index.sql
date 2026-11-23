-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Drops webhook_deliveries_pending_next_idx (00038, audit finding srv-apps-billing-8): no query
-- filters or sorts on next_attempt_at; the delivery job of a subscription reads its head through
-- webhook_deliveries_pending_seq_idx (00114). The index only cost a write on every attempt.
--
-- Dropped CONCURRENTLY outside a transaction (upgrade.md rule M4), one index per file. Contract
-- step of upgrade.md rule M1: the previous binary does not read the index either.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS webhook_deliveries_pending_next_idx;

-- +goose Down
CREATE INDEX CONCURRENTLY IF NOT EXISTS webhook_deliveries_pending_next_idx ON webhook_deliveries (next_attempt_at)
    WHERE status = 'pending';
