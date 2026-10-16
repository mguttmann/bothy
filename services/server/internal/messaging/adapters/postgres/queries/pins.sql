-- SPDX-License-Identifier: Apache-2.0
-- Queries on pins (messaging, docs/spec/03-data-model.md table pins; M1-59).
--
-- Every statement joins messages with the visible channels of the caller (brief design note 13).
-- pinMessage runs under the thread row lock and the message row lock, so the bound of
-- pin.max_per_thread is exact.

-- name: GetPin :one
SELECT p.message_id, p.thread_id, p.pinned_by, p.pinned_at
FROM pins p
JOIN messages m ON m.id = p.message_id
WHERE p.message_id = @message_id AND m.channel_id = ANY(@visible::uuid[]);

-- name: CountThreadPins :one
-- The pins of one thread for pin.max_per_thread.
SELECT count(*)::int
FROM pins p
JOIN messages m ON m.id = p.message_id
WHERE p.thread_id = @thread_id AND m.channel_id = ANY(@visible::uuid[]);

-- name: InsertPin :execrows
-- The idempotent insert of pinMessage; thread_id is the thread of the message.
INSERT INTO pins (message_id, thread_id, pinned_by, pinned_at)
SELECT m.id, m.thread_id, sqlc.arg(pinned_by)::uuid, sqlc.arg(pinned_at)::timestamptz
FROM messages m
WHERE m.id = @message_id AND m.channel_id = ANY(@visible::uuid[])
ON CONFLICT DO NOTHING;

-- name: DeletePin :many
-- unpinMessage and the deletion of a message remove the pin and return it as it was (at most one
-- row).
DELETE FROM pins AS p
USING messages AS m
WHERE p.message_id = m.id AND m.id = @message_id AND m.channel_id = ANY(@visible::uuid[])
RETURNING p.message_id, p.thread_id, p.pinned_by, p.pinned_at;

-- name: PinnedMessages :many
-- The pinned live messages of one page (Message.pinned).
SELECT p.message_id
FROM pins p
JOIN messages m ON m.id = p.message_id
WHERE p.message_id = ANY(@message_ids::uuid[]) AND m.channel_id = ANY(@visible::uuid[]) AND m.deleted_at IS NULL;

-- name: ListThreadPins :many
-- listPins (brief design note 12): the pins of the live messages of a thread, newest pin first,
-- then by message id descending, with channel and author of each message for message.read.
SELECT p.message_id, p.thread_id, p.pinned_by, p.pinned_at, m.channel_id, m.author_principal_id
FROM pins p
JOIN messages m ON m.id = p.message_id
WHERE p.thread_id = @thread_id AND m.channel_id = ANY(@visible::uuid[]) AND m.deleted_at IS NULL
ORDER BY p.pinned_at DESC, p.message_id DESC;
