-- SPDX-License-Identifier: Apache-2.0
-- The copy messages.channel_id on a thread move (M1-62; docs/spec/03-data-model.md threads "A
-- move changes channel_id"). chat locked the thread row and authorized thread.move in the same
-- transaction; this statement writes only the rows of that thread and is therefore exempt from
-- the visible-channel predicate (07 section 7.4 rule 1: writes are outside the lint).

-- name: MoveThreadMessages :exec
-- A copy only: no version change and no event; clients learn the new channel from thread.moved.
UPDATE messages SET channel_id = @channel_id WHERE thread_id = @thread_id;
