-- SPDX-License-Identifier: Apache-2.0
-- A read on a content table without the visibility predicate.

-- name: ListMessagesOfThread :many
SELECT id, content_md FROM messages
WHERE thread_id = @thread_id
ORDER BY id;
