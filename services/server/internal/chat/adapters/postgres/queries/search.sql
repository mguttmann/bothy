-- SPDX-License-Identifier: Apache-2.0
-- The read operations of chat for the search coordinator (M3-14; docs/spec/14-search.md sections
-- 4.3 to 4.9 and 10). Each carries the visible set the coordinator passes (07 7.4 rule 1): threads
-- by t.channel_id, channels by c.space_id; a deleted thread never matches (D-DEL-3). None authorizes; the coordinator checks every delivered
-- item with authorize again (14 4.3). The tsquery text comes from the search module and is cast
-- here (07 7.4 example). No read selects title_search or search_language as a column
-- (docs/DECISIONS.md D-M3-13-10).

-- name: SearchThreadTitles :many
-- The full-text search over thread titles in one statement (14 4.5 to 4.8):
--   cand: the newest matches after every filter, one more than the candidate limit, so capped
--         tells whether more matched (14 4.5 item 1); main threads have an empty title and an
--         empty vector, so they never match.
--   kept: cand without that extra row, which is never ranked or returned.
--   scored: the score of the sort (relevance: ts_rank divided by 1 + age / recency_scale with the
--         age measured from ref_time, never now(), so later pages rank the same rows the same way;
--         newest: 0) and the keyset strictly after the position of the previous page (14 4.7).
--   page: one row more than the page size, so the caller knows whether a next page exists.
-- The outer LEFT JOIN from capped returns one row without a thread for an empty page, so the flag
-- is reported then too. The headline runs only for the rows of the page (14 4.6).
WITH cand AS (
    SELECT t.id, t.channel_id, t.created_at, t.title, t.search_language,
        ts_rank(t.title_search, sqlc.arg(tsq)::text::tsquery, sqlc.arg(rank_normalization)::integer) AS rank
    FROM threads t
    WHERE t.title_search @@ sqlc.arg(tsq)::text::tsquery
      AND t.channel_id = ANY(@channel_ids::uuid[]) AND t.deleted_at IS NULL
      AND t.created_at <= sqlc.arg(ref_time)::timestamptz
      AND (NOT sqlc.arg(restrict_threads)::boolean OR t.id = ANY(@thread_ids::uuid[]))
      AND (sqlc.narg(state)::text IS NULL OR t.state = sqlc.narg(state)::text)
      AND (cardinality(@tag_ids::uuid[]) = 0
           OR EXISTS (SELECT 1 FROM thread_tags tt WHERE tt.thread_id = t.id AND tt.tag_id = ANY(@tag_ids::uuid[])))
      AND (sqlc.narg(operator_agent_id)::uuid IS NULL OR t.operator_agent_id = sqlc.narg(operator_agent_id)::uuid)
      AND (sqlc.narg(after)::timestamptz IS NULL OR t.created_at >= sqlc.narg(after)::timestamptz)
      AND (sqlc.narg(before)::timestamptz IS NULL OR t.created_at < sqlc.narg(before)::timestamptz)
    ORDER BY t.created_at DESC, t.id DESC
    LIMIT sqlc.arg(max_candidates)::integer + 1
),
capped AS (
    SELECT count(*) > sqlc.arg(max_candidates)::integer AS capped FROM cand
),
kept AS (
    SELECT * FROM cand ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(max_candidates)::integer
),
scored AS (
    SELECT k.id, k.channel_id, k.created_at, k.title, k.search_language,
        CASE WHEN sqlc.arg(newest)::boolean THEN 0::float8
             ELSE k.rank::float8 / (1 + extract(epoch FROM (sqlc.arg(ref_time)::timestamptz - k.created_at))::float8
                                        / sqlc.arg(recency_seconds)::float8)
        END AS score
    FROM kept k
),
page AS (
    SELECT s.*, row_number() OVER (ORDER BY s.score DESC, CASE WHEN sqlc.arg(newest)::boolean THEN s.created_at END DESC, s.id DESC) AS pos
    FROM scored s
    WHERE NOT sqlc.arg(has_from)::boolean
       OR (sqlc.arg(newest)::boolean AND (s.created_at, s.id) < (sqlc.arg(from_created_at)::timestamptz, sqlc.arg(from_id)::uuid))
       OR (NOT sqlc.arg(newest)::boolean AND (s.score, s.id) < (sqlc.arg(from_score)::float8, sqlc.arg(from_id)::uuid))
    ORDER BY s.score DESC, CASE WHEN sqlc.arg(newest)::boolean THEN s.created_at END DESC, s.id DESC
    LIMIT sqlc.arg(page_size)::integer + 1
)
SELECT c.capped::boolean AS capped, p.id::uuid AS id, p.channel_id::uuid AS channel_id, p.created_at::timestamptz AS created_at,
    coalesce(p.score, 0)::float8 AS score,
    coalesce(CASE WHEN p.pos <= sqlc.arg(page_size)::integer
        THEN ts_headline(p.search_language, left(p.title, sqlc.arg(headline_max_chars)::integer), sqlc.arg(tsq)::text::tsquery,
                         sqlc.arg(headline_options)::text)
    END, '')::text AS headline
FROM capped c
LEFT JOIN page p ON true
ORDER BY p.pos;

-- name: PrefixThreadTitles :many
-- The prefix search of the quick switcher (14 4.9) over the GIN index of title_search: threads of
-- the visible channels, newest activity first.
SELECT t.id, t.channel_id, t.title, t.last_activity_at
FROM threads t
WHERE t.channel_id = ANY(@channel_ids::uuid[]) AND t.title_search @@ sqlc.arg(tsq)::text::tsquery AND t.deleted_at IS NULL
ORDER BY t.last_activity_at DESC, t.id DESC
LIMIT sqlc.arg(max_rows)::integer;

-- name: ThreadsMatching :many
-- The threads of the visible channels that match the thread facts of a filter (07 7.4 rule 6,
-- M3-14 design note 5): state, any of the tags, the operator agent (a main thread has none).
SELECT t.id
FROM threads t
WHERE t.channel_id = ANY(@channel_ids::uuid[]) AND t.deleted_at IS NULL
  AND (sqlc.narg(state)::text IS NULL OR t.state = sqlc.narg(state)::text)
  AND (cardinality(@tag_ids::uuid[]) = 0
       OR EXISTS (SELECT 1 FROM thread_tags tt WHERE tt.thread_id = t.id AND tt.tag_id = ANY(@tag_ids::uuid[])))
  AND (sqlc.narg(operator_agent_id)::uuid IS NULL OR t.operator_agent_id = sqlc.narg(operator_agent_id)::uuid)
ORDER BY t.id;

-- name: ChannelsInSpaces :many
-- The channels of the spaces of a space_id filter; the coordinator passes only visible spaces.
SELECT c.id
FROM channels c
WHERE c.space_id = ANY(@space_ids::uuid[])
ORDER BY c.id;

-- name: ChannelLabels :many
-- Channel and space names of the hits whose space is visible (14 11); a dm channel has no space
-- and no label.
SELECT c.id, c.name, s.name AS space_name
FROM channels c
JOIN spaces s ON s.id = c.space_id
WHERE c.space_id = ANY(@visible_space_ids::uuid[]) AND c.id = ANY(@channel_ids::uuid[]);

-- name: ThreadTitleLabels :many
-- Thread titles of the hits in the visible channels; a main thread has no title and no label.
SELECT t.id, t.title
FROM threads t
WHERE t.channel_id = ANY(@channel_ids::uuid[]) AND t.id = ANY(@thread_ids::uuid[]) AND t.title <> '' AND t.deleted_at IS NULL;

-- name: ThreadsIn :many
-- The threads of the hits for HitThreads, the columns of GetThread, in the visible channels.
SELECT t.id, t.channel_id, t.kind, t.parent_message_id, t.title, t.title_source, t.state, t.closed_at, t.closed_by,
    t.operator_agent_id, t.last_message_seq, t.unread_floor_seq, t.last_activity_at, t.message_count, t.created_by, t.created_at,
    t.moved_at, t.version, t.deleted_at, t.deleted_by, t.purged_at
FROM threads t
WHERE t.channel_id = ANY(@channel_ids::uuid[]) AND t.id = ANY(@thread_ids::uuid[]) AND t.deleted_at IS NULL;
