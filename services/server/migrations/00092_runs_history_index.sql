-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- The run history (F-LAUFHISTORIE, listRunHistory, docs/spec/25-visualization.md 6.7): the
-- finished runs of the threads the caller may read, newest end first. The partial index serves the
-- keyset (finished_at DESC, id DESC); the thread filter is checked on the rows of the index. Built
-- CONCURRENTLY outside a transaction (upgrade.md rule M4); the DROP ... IF EXISTS before the build
-- removes an invalid leftover of an interrupted build. Additive only: no column changes.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS runs_finished_idx;
CREATE INDEX CONCURRENTLY runs_finished_idx ON runs (finished_at DESC, id DESC)
    WHERE state IN ('completed', 'failed', 'cancelled');

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS runs_finished_idx;
