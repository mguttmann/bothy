-- SPDX-License-Identifier: Apache-2.0
-- The participant of class import (docs/spec/20-privacy.md 4.1, docs/spec/22-import-migration.md
-- 13.5): the items and then the row of a source that ended before the cutoff
-- (retention.import_after_completion before now). A source ends by its completion or its
-- cancellation, both final (state completed or failed with finished_at); a source without
-- finished_at (draft, running, paused) never matches, so a running import keeps every item it
-- needs to stay idempotent.

-- name: PurgeImportItems :execrows
-- One batch of the items of ended sources. No order: every item of such a source goes, and an
-- order by id would read all its items for each batch. Deleting an item clears
-- messages.import_item_id (ON DELETE SET NULL), which messages_import_item_id_idx serves.
DELETE FROM import_items
WHERE id IN (
    SELECT i.id FROM import_items i
    JOIN import_sources s ON s.id = i.source_id
    WHERE s.state IN ('completed', 'failed') AND s.finished_at < @cutoff
    LIMIT @row_limit
);

-- name: PurgeImportSources :execrows
-- The row of an ended source goes last, once PurgeImportItems removed its items, so its deletion
-- never cascades over the items of a whole import in one transaction.
DELETE FROM import_sources
WHERE id IN (
    SELECT s.id FROM import_sources s
    WHERE s.state IN ('completed', 'failed') AND s.finished_at < @cutoff
      AND NOT EXISTS (SELECT 1 FROM import_items i WHERE i.source_id = s.id)
    ORDER BY s.finished_at, s.id
    LIMIT @row_limit
);
