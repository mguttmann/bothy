-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- The pending deliveries of one subscription (docs/spec/03-data-model.md section 4.11, audit
-- finding srv-apps-billing-8): HeadDelivery (the pending row with the lowest event_seq),
-- CountPending and ExpirePending filter on subscription_id and status = 'pending'. The unique key
-- (subscription_id, event_seq) also holds the finished rows of up to retention.transient, so
-- without this index a head read walked the history of the subscription.
--
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4), one index per file; the
-- DROP ... IF EXISTS before the build removes an invalid leftover of an interrupted build.
-- Additive only: the previous binary keeps working.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS webhook_deliveries_pending_seq_idx;
CREATE INDEX CONCURRENTLY webhook_deliveries_pending_seq_idx ON webhook_deliveries (subscription_id, event_seq)
    WHERE status = 'pending';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS webhook_deliveries_pending_seq_idx;
