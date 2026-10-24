-- SPDX-License-Identifier: Apache-2.0
-- The windows of a run context (messaging, M2-13; docs/spec/09-agents.md section 4.6). runs
-- authorized run.context.read on the run of the thread first, and messaging checks every returned
-- row with message.read in filter mode, so the statement carries no visible-channel predicate (07
-- section 7.4 rule 1, exception "loading a resource that has already been checked"). Tombstones
-- and the excluded author are filtered here, so the limit counts only selectable messages. One
-- statement: the page rows and the flag of older selectable rows share a snapshot; the one-row
-- relation f keeps the flag in a row whose message columns are NULL when the page is empty.

-- name: ListContextWindow :many
-- The newest @row_limit selectable messages strictly between @after_seq and @before_seq over the
-- unique index (thread_id, thread_seq), ascending; has_older is whether a selectable message
-- below the oldest page row (or below @before_seq for an empty page) and above @after_seq exists.
WITH page AS (
    SELECT m.* FROM messages m
    WHERE m.thread_id = @thread_id AND m.thread_seq > @after_seq AND m.thread_seq < @before_seq
      AND m.deleted_at IS NULL
      AND (sqlc.narg(exclude_author)::uuid IS NULL OR m.author_principal_id <> sqlc.narg(exclude_author)::uuid)
    ORDER BY m.thread_seq DESC
    LIMIT @row_limit
), f AS (
    SELECT EXISTS (
        SELECT 1 FROM messages o
        WHERE o.thread_id = @thread_id AND o.thread_seq > @after_seq
          AND o.thread_seq < COALESCE((SELECT min(p.thread_seq) FROM page p), @before_seq)
          AND o.deleted_at IS NULL
          AND (sqlc.narg(exclude_author)::uuid IS NULL OR o.author_principal_id <> sqlc.narg(exclude_author)::uuid)
    ) AS has_older
)
SELECT p.id, p.thread_id, p.channel_id, p.thread_seq, p.author_principal_id, p.client_msg_id, p.origin, p.kind,
       p.system_code, p.content_md, p.content_format, p.reply_to_message_id, p.run_id, p.trigger_state, p.trigger_state_reason,
       p.trigger_state_at, p.silent, p.version, p.edited_at,
       p.deleted_at, p.deleted_by, p.created_at, f.has_older
FROM f LEFT JOIN page p ON true
ORDER BY p.thread_seq;
