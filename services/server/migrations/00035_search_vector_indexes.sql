-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- GIN indexes of the search vectors of 00034 (M3-13, docs/spec/03-datenmodell.md section 5,
-- docs/spec/14-suche.md section 8), built CONCURRENTLY outside a transaction so writes go on
-- while they build (upgrade.md rule M4, ADR 0007 rule 3). An interrupted concurrent build leaves
-- an invalid index behind; the DROP ... IF EXISTS before each build removes it, so a second
-- "bothy migrate up" succeeds (docs/DECISIONS.md D-M3-13-7). GIN with default settings (14 8).
-- bothy migrate runs every statement with lock_timeout = migrate.lock_timeout (rule M6).

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS messages_search_vector_idx;
CREATE INDEX CONCURRENTLY messages_search_vector_idx ON messages USING gin (search_vector);
DROP INDEX CONCURRENTLY IF EXISTS threads_title_search_idx;
CREATE INDEX CONCURRENTLY threads_title_search_idx ON threads USING gin (title_search);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS threads_title_search_idx;
DROP INDEX CONCURRENTLY IF EXISTS messages_search_vector_idx;
