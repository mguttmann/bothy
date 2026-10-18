-- SPDX-License-Identifier: Apache-2.0
-- Queries on thread_tags (chat module, M1-55; docs/spec/03-data-model.md section 4.5). Only this
-- module reads and writes the table (03 principle 12).

-- name: ListThreadTags :many
-- The live tags of the threads, in the order of the tags of their forum.
SELECT tt.thread_id, tt.tag_id FROM thread_tags tt
JOIN forum_tags f ON f.id = tt.tag_id
WHERE tt.thread_id = ANY (@thread_ids::uuid[]) AND f.deleted_at IS NULL
ORDER BY tt.thread_id, f.sort_key, f.id;

-- name: DeleteThreadTags :exec
DELETE FROM thread_tags WHERE thread_id = @thread_id;

-- name: InsertThreadTag :exec
INSERT INTO thread_tags (thread_id, tag_id, added_by, added_at) VALUES (@thread_id, @tag_id, @added_by, @added_at);

-- name: DeleteTagAssignments :exec
-- deleteForumTag drops the rows of its tag in the same transaction (brief M1-55 D5).
DELETE FROM thread_tags WHERE tag_id = @tag_id;
