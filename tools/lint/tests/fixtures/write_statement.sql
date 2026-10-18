-- SPDX-License-Identifier: Apache-2.0
-- Writes are not checked: the row was authorized before.

-- name: DeleteMessage :exec
DELETE FROM messages WHERE id = @id;

-- name: TouchThread :one
UPDATE threads SET last_message_seq = last_message_seq + 1 WHERE id = @id
RETURNING (SELECT count(*) FROM messages WHERE thread_id = @id);

-- name: CopyMention :exec
INSERT INTO mentions (message_id, principal_id)
SELECT id, @principal_id FROM messages WHERE id = @id;
