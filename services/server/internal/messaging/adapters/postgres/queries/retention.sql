-- SPDX-License-Identifier: Apache-2.0

-- name: ExpiredMessages :many
-- The candidates of the job retention.content (M7-05, docs/spec/20-privacy.md 4.1): messages not
-- deleted yet and created before the cutoff, oldest first, with their channel. System lines carry
-- no content. A system job without a principal: no visibility predicate
-- (tools/lint/sql_visibility.yaml); each candidate is deleted through the deletion path of 4.2.
SELECT id, channel_id FROM messages
WHERE deleted_at IS NULL AND kind <> 'system' AND created_at < @cutoff
ORDER BY created_at, id
LIMIT @row_limit;
