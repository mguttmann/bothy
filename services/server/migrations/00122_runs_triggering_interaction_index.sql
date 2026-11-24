-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Index for the run an interaction opened (docs/spec/03-data-model.md runs, audit finding
-- srv-runs-core-4): RunOfInteraction reads runs by triggering_interaction_id in the answer of
-- createInteraction and respondToInteraction, and the foreign key to interactions (ON DELETE SET
-- NULL) checks the same column for every interaction a message deletion, a purge or the retention
-- removes. Runs are never deleted, so without this index both read the whole table. Only runs
-- opened by an interaction carry the column, so the index is partial.
--
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4), one index per file; the
-- DROP ... IF EXISTS before the build removes an invalid leftover of an interrupted build.
-- Additive only: the previous binary keeps working.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS runs_triggering_interaction_idx;
CREATE INDEX CONCURRENTLY runs_triggering_interaction_idx ON runs (triggering_interaction_id)
    WHERE triggering_interaction_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS runs_triggering_interaction_idx;
