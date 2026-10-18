-- SPDX-License-Identifier: Apache-2.0
-- ANY over a subquery reads a foreign table and is no visibility parameter (03 item 12).

-- name: MessagesOfGrantedChannels :many
SELECT m.id FROM messages m
WHERE m.channel_id = ANY(SELECT target_channel_id FROM grants WHERE subject_user_id = @user_id);
