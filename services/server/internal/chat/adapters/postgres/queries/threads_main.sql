-- SPDX-License-Identifier: Apache-2.0
-- The main thread of a text channel (chat module, M1-53; docs/spec/03-datenmodell.md section
-- 4.5, R-DM-11): inserted in the transaction that creates the channel. The thread use cases on
-- the same table come with M1-55 (docs/DECISIONS.md M1-53/6). The main thread gets the language
-- of the creator of the channel like every thread (14 3.1, docs/DECISIONS.md D-M3-13-2), as text
-- cast here; the returned columns leave out search_language and title_search (D-M3-13-10).

-- name: InsertMainThread :one
INSERT INTO threads (id, channel_id, kind, created_by, created_at, last_activity_at, search_language)
VALUES (@id, @channel_id, 'main', @created_by, @created_at, @created_at, sqlc.arg(search_language)::text::regconfig)
RETURNING id, channel_id, kind, parent_message_id, title, title_source, state, closed_at, closed_by, operator_agent_id,
    last_message_seq, unread_floor_seq, last_activity_at, message_count, created_by, created_at, moved_at, version;
