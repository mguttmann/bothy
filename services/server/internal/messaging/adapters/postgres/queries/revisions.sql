-- SPDX-License-Identifier: Apache-2.0
-- Queries on message_revisions (messaging, M1-58, docs/spec/03-data-model.md
-- message_revisions). Every statement joins messages with the visible channels of the caller.

-- name: InsertRevision :exec
-- The previous content of the locked message as the next revision (1 for the first edit, the
-- original); the message row lock serialises the numbers.
INSERT INTO message_revisions (id, message_id, revision, content_md, edited_by, created_at)
SELECT @id, m.id,
       COALESCE((SELECT max(r.revision) FROM message_revisions AS r WHERE r.message_id = m.id), 0) + 1,
       @content_md, @edited_by, @created_at
FROM messages AS m
WHERE m.id = @message_id AND m.channel_id = ANY(@visible::uuid[]);

-- name: ListRevisions :many
-- The earlier versions of a message in ascending revision order.
SELECT r.revision, r.content_md, r.edited_by, r.created_at
FROM message_revisions AS r
JOIN messages AS m ON m.id = r.message_id
WHERE r.message_id = @message_id AND m.channel_id = ANY(@visible::uuid[])
ORDER BY r.revision;

-- name: DeleteRevisions :exec
-- A deletion removes every revision of the message (03 section 6.5).
DELETE FROM message_revisions AS r
USING messages AS m
WHERE r.message_id = m.id AND m.id = @message_id AND m.channel_id = ANY(@visible::uuid[]);
