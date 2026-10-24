-- SPDX-License-Identifier: Apache-2.0
-- The read operations of messaging for the search coordinator (M3-14; docs/spec/14-search.md
-- sections 4.3 to 4.8 and 10). Neither authorizes: the coordinator passes the visible channels of
-- Authorizer.Scope as @channel_ids (07 section 7.4 rule 1) and checks every delivered item with
-- authorize again. The tsquery is built once per request by the search module and arrives as text.

-- name: SearchMessages :many
-- One search over messages.search_vector in one statement (14 4.8, brief M3-14 design note 6):
--   reach    the matching live rows of the visible channels with the filters, created at or before
--            the reference time, newest first, at most @candidate_rows (search.max_candidates
--            plus one, so the extra row shows that the limit was reached);
--   flag     candidates_capped, computed from reach, so an empty page still reports it;
--   cand     the newest @max_candidates rows of reach: the extra row is never ranked or returned;
--   scored   the score of relevance, ts_rank divided by 1 + age / search.recency_scale with the
--            age measured from the reference time, never now(), so every page ranks alike;
--            0 for newest;
--   page     the rows strictly after the keyset position of the previous page, in the order of
--            the sort, at most @page_rows (the page size plus one, so the extra row shows that
--            a next page exists);
--   result   the headline only for the rows of the page, over the content cut at
--            search.headline_input_max_chars in the language of the message.
-- The one-row flag relation is left joined with the page, so an empty page returns one row whose
-- message columns are NULL.
WITH p AS (
    SELECT (@tsq::text)::tsquery AS tsq, @ref_time::timestamptz AS ref_time
), reach AS (
    SELECT m.id, m.thread_id, m.channel_id, m.created_at, m.search_language, m.search_vector,
           left(m.content_md, @headline_input_max_chars::int) AS excerpt
    FROM messages m, p
    WHERE m.search_vector @@ p.tsq
      AND m.channel_id = ANY(@channel_ids::uuid[])
      AND m.deleted_at IS NULL
      AND m.created_at <= p.ref_time
      AND (sqlc.narg(thread_ids)::uuid[] IS NULL OR m.thread_id = ANY(sqlc.narg(thread_ids)::uuid[]))
      AND (sqlc.narg(author_id)::uuid IS NULL OR m.author_principal_id = sqlc.narg(author_id)::uuid)
      AND (sqlc.narg(after)::timestamptz IS NULL OR m.created_at >= sqlc.narg(after)::timestamptz)
      AND (sqlc.narg(before)::timestamptz IS NULL OR m.created_at < sqlc.narg(before)::timestamptz)
      AND (sqlc.narg(has_file)::boolean IS NULL OR m.has_attachments = sqlc.narg(has_file)::boolean)
    ORDER BY m.created_at DESC, m.id DESC
    LIMIT @candidate_rows::int
), flag AS (
    SELECT count(*) > @max_candidates::int AS capped FROM reach
), cand AS (
    SELECT r.id, r.thread_id, r.channel_id, r.created_at, r.search_language, r.search_vector, r.excerpt
    FROM reach r
    ORDER BY r.created_at DESC, r.id DESC
    LIMIT @max_candidates::int
), scored AS (
    SELECT c.id, c.thread_id, c.channel_id, c.created_at, c.search_language, c.excerpt,
           CASE WHEN @newest::boolean THEN 0::float8
                ELSE ts_rank(c.search_vector, p.tsq, @rank_normalization::int)::float8
                     / (1 + date_part('epoch', p.ref_time - c.created_at) / @recency_seconds::float8)
           END AS score,
           @newest::boolean AS newest
    FROM cand c, p
), keyed AS (
    SELECT s.id, s.thread_id, s.channel_id, s.created_at, s.search_language, s.excerpt, s.score,
           CASE WHEN s.newest THEN s.created_at END AS sort_time,
           CASE WHEN NOT s.newest THEN s.score END AS sort_score
    FROM scored s
    WHERE NOT @has_from::boolean
       OR (s.newest AND (s.created_at, s.id) < (@from_created_at::timestamptz, @from_id::uuid))
       OR (NOT s.newest AND (s.score, s.id) < (@from_score::float8, @from_id::uuid))
), ranked AS (
    SELECT k.id, k.thread_id, k.channel_id, k.created_at, k.search_language, k.excerpt, k.score,
           row_number() OVER (ORDER BY k.sort_time DESC, k.sort_score DESC, k.id DESC) AS rn
    FROM keyed k
    ORDER BY k.sort_time DESC, k.sort_score DESC, k.id DESC
    LIMIT @page_rows::int
), result AS (
    SELECT pr.id, pr.thread_id, pr.channel_id, pr.created_at, pr.score, pr.rn,
           CASE WHEN pr.rn <= @page_size::int
                THEN ts_headline(pr.search_language, pr.excerpt, p.tsq, @headline_options::text) END AS headline
    FROM ranked pr, p
)
SELECT f.capped, o.id, o.thread_id, o.channel_id, o.created_at,
       coalesce(o.score, 0)::float8 AS score, coalesce(o.headline, '')::text AS headline
FROM flag f LEFT JOIN result o ON true
ORDER BY o.rn;

-- name: HitMessages :many
-- The live messages @message_ids of the channels @channel_ids (the channels of the kept hits) for
-- the hit objects of the search; the use case orders them like the input and renders them like
-- listThreadMessages. The columns are listed like GetVisibleMessage, so the read carries neither
-- search_language nor search_vector (D-M3-13-10).
SELECT id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
    system_code, content_md, content_format, reply_to_message_id, run_id, trigger_state, trigger_state_reason, trigger_state_at,
    silent, import_item_id, version, edited_at, deleted_at, deleted_by, created_at, has_attachments
FROM messages m
WHERE m.id = ANY(@message_ids::uuid[]) AND m.channel_id = ANY(@channel_ids::uuid[]) AND m.deleted_at IS NULL;
