-- SPDX-License-Identifier: Apache-2.0
-- Queries on user_space_states (chat module, M1-52) and user_channel_states (M4-51;
-- docs/spec/03-data-model.md section 4.5, 04 section 14). Only this module reads and writes the
-- tables (03 Grundsatz 12).

-- name: UpsertSidebarState :one
-- Last write wins per space: the row of the person for the space, version + 1, the device of the
-- write; keep_key keeps the stored personal key and keep_order the stored channel order (M4-51);
-- a NULL channel_order is the default order.
INSERT INTO user_space_states (user_id, space_id, collapsed, sort_key, channel_order, updated_by_device_id)
VALUES (@user_id, @space_id, @collapsed, sqlc.narg(sort_key), sqlc.narg(channel_order)::uuid[], sqlc.narg(updated_by_device_id))
ON CONFLICT (user_id, space_id) DO UPDATE
SET collapsed = EXCLUDED.collapsed,
    sort_key = CASE WHEN @keep_key::boolean THEN user_space_states.sort_key ELSE EXCLUDED.sort_key END,
    channel_order = CASE WHEN @keep_order::boolean THEN user_space_states.channel_order ELSE EXCLUDED.channel_order END,
    version = user_space_states.version + 1,
    updated_by_device_id = EXCLUDED.updated_by_device_id,
    updated_at = now()
RETURNING *;

-- name: ListSidebarStates :many
-- The rows of one person for the spaces that are not archived, with the predicate of 07 section
-- 7.4 rule 1: every space for an instance-wide reader, else the visible spaces of Scope;
-- authorize then checks each row.
SELECT st.* FROM user_space_states st
JOIN spaces sp ON sp.id = st.space_id
WHERE st.user_id = @user_id AND sp.archived_at IS NULL
  AND (@all_spaces::boolean OR st.space_id = ANY (@visible::uuid[]))
ORDER BY st.space_id;

-- name: UpsertSidebarChannelState :one
-- Last write wins per channel (M4-51): the personal post order of the person for the channel,
-- version + 1, the device of the write.
INSERT INTO user_channel_states (user_id, channel_id, thread_order, updated_by_device_id)
VALUES (@user_id, @channel_id, @thread_order::uuid[], sqlc.narg(updated_by_device_id))
ON CONFLICT (user_id, channel_id) DO UPDATE
SET thread_order = EXCLUDED.thread_order,
    version = user_channel_states.version + 1,
    updated_by_device_id = EXCLUDED.updated_by_device_id,
    updated_at = now()
RETURNING *;

-- name: ListSidebarChannelStates :many
-- The channel rows of one person (M4-51) for the channels that are not archived and whose space
-- is not archived, with the predicate of 07 section 7.4 rule 1: every channel for an
-- instance-wide reader or a fully visible space, else the visible channels of Scope; authorize
-- then checks each row.
SELECT st.* FROM user_channel_states st
JOIN channels c ON c.id = st.channel_id
LEFT JOIN spaces sp ON sp.id = c.space_id
WHERE st.user_id = @user_id AND c.archived_at IS NULL AND sp.archived_at IS NULL
  AND (@all_channels::boolean OR st.channel_id = ANY (@visible::uuid[]) OR c.space_id = ANY (@fully_visible::uuid[]))
ORDER BY st.channel_id;
