-- SPDX-License-Identifier: Apache-2.0
-- Queries on user_space_states (chat module, M1-52; docs/spec/03-datenmodell.md section 4.5,
-- 04 section 14). Only this module reads and writes the table (03 Grundsatz 12).

-- name: UpsertSidebarState :one
-- Last write wins per space: the row of the person for the space, version + 1, the device of the
-- write; keep_key keeps the stored personal key.
INSERT INTO user_space_states (user_id, space_id, collapsed, sort_key, updated_by_device_id)
VALUES (@user_id, @space_id, @collapsed, sqlc.narg(sort_key), sqlc.narg(updated_by_device_id))
ON CONFLICT (user_id, space_id) DO UPDATE
SET collapsed = EXCLUDED.collapsed,
    sort_key = CASE WHEN @keep_key::boolean THEN user_space_states.sort_key ELSE EXCLUDED.sort_key END,
    version = user_space_states.version + 1,
    updated_by_device_id = EXCLUDED.updated_by_device_id,
    updated_at = now()
RETURNING *;

-- name: ListSidebarStates :many
-- The rows of one person for the spaces that are not archived; visibility runs through
-- authorize afterwards.
SELECT st.* FROM user_space_states st
JOIN spaces sp ON sp.id = st.space_id
WHERE st.user_id = @user_id AND sp.archived_at IS NULL
ORDER BY st.space_id;
