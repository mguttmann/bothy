-- SPDX-License-Identifier: Apache-2.0
-- The threads that can be unread for somebody, for the badge of notify (M3-19 design note 16 a;
-- docs/spec/13-push-benachrichtigungen.md section 8). Only this module reads threads and channels
-- (03 Grundsatz 12).

-- name: UnreadCandidates :many
-- The threads of the given channels (the channels the recipient can see, from authz Scope) with a
-- message above unread_floor_seq, with the kind of their channel and both counters, ascending by
-- thread id. The read positions live in sync and the mentions in messaging; notify combines them
-- in Go (07 7.4 rule 6, no join across modules) and confirms every counted thread with
-- push.deliver.
SELECT t.id, t.channel_id, c.kind AS channel_kind, t.last_message_seq, t.unread_floor_seq
FROM threads t
JOIN channels c ON c.id = t.channel_id
WHERE t.channel_id = ANY(sqlc.arg(channel_ids)::uuid[]) AND t.last_message_seq > t.unread_floor_seq
ORDER BY t.id;
