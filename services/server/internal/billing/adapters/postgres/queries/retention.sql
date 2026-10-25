-- SPDX-License-Identifier: Apache-2.0
-- The participant of class cost (M7-05, docs/spec/20-privacy.md 2 and 4.1): cost entries after
-- occurred_at and closed price rows after valid_to; an open price row (valid_to NULL) has no end
-- time. Cost entries go newest first, so a correction goes before the entry it supersedes
-- (supersedes_id ON DELETE RESTRICT); an entry a kept correction still names stays with it.

-- name: ExpiredCostEntries :many
SELECT id FROM cost_entries WHERE occurred_at < @cutoff ORDER BY occurred_at DESC, id LIMIT @row_limit;

-- name: DeleteCostEntry :execrows
DELETE FROM cost_entries WHERE id = @id AND occurred_at < @cutoff;

-- name: ExpiredPrices :many
SELECT id FROM price_table WHERE valid_to < @cutoff ORDER BY valid_to, id LIMIT @row_limit;

-- name: DeletePrice :execrows
DELETE FROM price_table WHERE id = @id AND valid_to < @cutoff;
