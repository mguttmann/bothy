-- +goose NO TRANSACTION
-- SPDX-License-Identifier: Apache-2.0
-- Index for the job retention.import (docs/spec/20-privacy.md 4.1, docs/spec/22-import-migration.md
-- 13.5). The job deletes the items of an import source after retention.import_after_completion;
-- every deleted item clears messages.import_item_id of its message (ON DELETE SET NULL, 00064).
-- Without an index on the referencing column each deleted item reads the whole table messages.
-- Only imported messages carry the column, so the index is partial.
--
-- Built CONCURRENTLY outside a transaction (upgrade.md rule M4); the DROP ... IF EXISTS before the
-- build removes an invalid leftover of an interrupted build. Additive only: no column changes, the
-- previous binary keeps working.

-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS messages_import_item_id_idx;
CREATE INDEX CONCURRENTLY messages_import_item_id_idx ON messages (import_item_id)
    WHERE import_item_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS messages_import_item_id_idx;
