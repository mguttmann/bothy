-- SPDX-License-Identifier: Apache-2.0
-- Queries on reactions (messaging, docs/spec/03-data-model.md table reactions; M1-59).
--
-- Every statement joins messages with the visible channels of the caller (brief design note 13,
-- TestQueriesCarryVisibleChannelPredicate). The writes run under the row lock of
-- LockVisibleMessage, which serialises every reaction and pin change of one message.

-- name: ReactionStats :one
-- The distinct symbols of a message and the reactions with one symbol: the bound of
-- reaction.max_distinct_per_message and the count of the event.
SELECT count(DISTINCT r.emoji)::int AS distinct_emoji,
       (count(*) FILTER (WHERE r.emoji = sqlc.arg(emoji)::text))::int AS emoji_count
FROM reactions r
JOIN messages m ON m.id = r.message_id
WHERE r.message_id = @message_id AND m.channel_id = ANY(@visible::uuid[]);

-- name: InsertReaction :execrows
-- The idempotent insert of addReaction (PK message_id, principal_id, emoji).
INSERT INTO reactions (message_id, principal_id, emoji, created_at)
SELECT m.id, sqlc.arg(principal_id)::uuid, sqlc.arg(emoji)::text, sqlc.arg(created_at)::timestamptz
FROM messages m
WHERE m.id = @message_id AND m.channel_id = ANY(@visible::uuid[])
ON CONFLICT DO NOTHING;

-- name: DeleteReaction :execrows
-- removeReaction deletes only the caller's row for one symbol.
DELETE FROM reactions AS r
USING messages AS m
WHERE r.message_id = m.id AND m.id = @message_id AND m.channel_id = ANY(@visible::uuid[])
  AND r.principal_id = @principal_id AND r.emoji = @emoji;

-- name: DeleteMessageReactions :exec
-- The deletion of a message removes its reactions in the same transaction (M1-59 design note 9).
DELETE FROM reactions AS r
USING messages AS m
WHERE r.message_id = m.id AND m.id = @message_id AND m.channel_id = ANY(@visible::uuid[]);

-- name: ReactionGroups :many
-- The summary of the live messages of one page: count and first reaction per message and symbol,
-- in one statement (brief design note 7).
SELECT r.message_id, r.emoji, count(*)::int AS reaction_count, min(r.created_at)::timestamptz AS first_at
FROM reactions r
JOIN messages m ON m.id = r.message_id
WHERE r.message_id = ANY(@message_ids::uuid[]) AND m.channel_id = ANY(@visible::uuid[]) AND m.deleted_at IS NULL
GROUP BY r.message_id, r.emoji;

-- name: OwnReactions :many
-- The symbols of one principal on the live messages of one page (my_reactions).
SELECT r.message_id, r.emoji
FROM reactions r
JOIN messages m ON m.id = r.message_id
WHERE r.principal_id = @principal_id AND r.message_id = ANY(@message_ids::uuid[])
  AND m.channel_id = ANY(@visible::uuid[]) AND m.deleted_at IS NULL;

-- name: ListReactions :many
-- One page of listReactions (brief design note 8): keyset (created_at, principal_id, emoji)
-- ascending after the cursor, optionally one symbol; the caller asks for one row more than the
-- page. The symbol compares bytewise (COLLATE "C"), so the order does not depend on the
-- collation of the database.
SELECT r.principal_id, r.emoji, r.created_at
FROM reactions r
JOIN messages m ON m.id = r.message_id
WHERE r.message_id = @message_id AND m.channel_id = ANY(@visible::uuid[]) AND m.deleted_at IS NULL
  AND (sqlc.arg(emoji)::text = '' OR r.emoji = sqlc.arg(emoji)::text)
  AND (NOT sqlc.arg(has_after)::boolean
       OR (r.created_at, r.principal_id, r.emoji COLLATE "C")
          > (sqlc.arg(after_at)::timestamptz, sqlc.arg(after_principal)::uuid, sqlc.arg(after_emoji)::text COLLATE "C"))
ORDER BY r.created_at, r.principal_id, r.emoji COLLATE "C"
LIMIT sqlc.arg(row_limit);
