-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Index for the files marked deleted (docs/spec/03-data-model.md files, audit findings
-- srv-files-adapters-7 and srv-files-core-7): the catch-up step of files.cleanup_pending
-- (DeletedRows) pages through them by (deleted_at, id) and the backlog gauge (PurgeBacklog) counts
-- them on every run of file.gc_interval. Without an index both read the whole table, although only
-- the few rows still waiting for their purge match. The rows leave the index with their purge, so
-- it stays small.
--
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4), one index per file; the
-- DROP ... IF EXISTS before the build removes an invalid leftover of an interrupted build.
-- Additive only: the previous binary keeps working.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS files_deleted_at_idx;
CREATE INDEX CONCURRENTLY files_deleted_at_idx ON files (deleted_at, id) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS files_deleted_at_idx;
