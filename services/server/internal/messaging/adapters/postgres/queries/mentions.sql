-- SPDX-License-Identifier: Apache-2.0
-- Queries on mentions (messaging, docs/spec/03-datenmodell.md section 4.6).

-- name: InsertMentions :exec
-- One row per resolved principal of a message, in one statement (brief M1-56 D9).
INSERT INTO mentions (message_id, principal_id, thread_id, created_at)
SELECT @message_id, p, @thread_id, @created_at
FROM unnest(@principal_ids::uuid[]) AS p;

-- name: ListMentions :many
-- The resolved principals of a message, sorted (Message.mention_ids).
SELECT principal_id FROM mentions WHERE message_id = @message_id ORDER BY principal_id;
