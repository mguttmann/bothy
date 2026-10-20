-- SPDX-License-Identifier: Apache-2.0
-- The copies read_states.channel_id and drafts.channel_id on a thread move (M1-62;
-- docs/spec/03-data-model.md threads "A move changes channel_id"). Unlike the personal statements
-- of read_states.sql and drafts.sql they name no person: they change the rows of every person for
-- the one thread that chat locked FOR UPDATE and authorized with thread.move in the same
-- transaction. Copies only: no version change and no event; clients learn the new channel from
-- thread.moved. A concurrent putReadState or putThreadDraft holds the thread FOR SHARE
-- (Threads.InfoForShare), so it either committed before this statement or writes the new channel.

-- name: MoveThreadReadStates :exec
UPDATE read_states SET channel_id = @channel_id WHERE thread_id = @thread_id;

-- name: MoveThreadDrafts :exec
UPDATE drafts SET channel_id = @channel_id WHERE thread_id = @thread_id;
