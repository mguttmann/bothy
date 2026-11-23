-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Drops agents_last_heartbeat_at_idx (00022, audit finding srv-agents-10): no query filters or
-- sorts on last_heartbeat_at; presence is evaluated in the domain from the row read by its key,
-- and PersistHeartbeat writes the column by principal_id. The index only kept every persisted
-- heartbeat of every agent from being a HOT update.
--
-- Dropped CONCURRENTLY outside a transaction (upgrade.md rule M4), one index per file. Contract
-- step of upgrade.md rule M1: the previous binary does not read the index either.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS agents_last_heartbeat_at_idx;

-- +goose Down
CREATE INDEX CONCURRENTLY IF NOT EXISTS agents_last_heartbeat_at_idx ON agents (last_heartbeat_at);
