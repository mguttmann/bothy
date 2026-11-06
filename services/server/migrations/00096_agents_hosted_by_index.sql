-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- The hosted list of a runner (A1-03, listHostedAgents, docs/spec/26-builtin-agents.md 3.2): the
-- partial index (hosted_by) WHERE hosted_by IS NOT NULL of docs/spec/03-data-model.md section 4.4.
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4); the DROP ... IF EXISTS before the
-- build removes an invalid leftover of an interrupted build.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS agents_hosted_by_idx;
CREATE INDEX CONCURRENTLY agents_hosted_by_idx ON agents (hosted_by) WHERE hosted_by IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS agents_hosted_by_idx;
