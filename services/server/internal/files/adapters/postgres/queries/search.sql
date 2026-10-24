-- SPDX-License-Identifier: Apache-2.0
-- The read operations of files for the search coordinator (M3-14, files.SearchReads;
-- docs/spec/14-search.md 4.3 to 4.8 and section 10). files never reads messages (03 principle
-- 12): the place of a file is the attachment row, whose thread_id and channel_id copy the thread
-- and channel of the message (R-DM-43). Every read carries the visibility predicate
-- a.channel_id = ANY(@channel_ids) on the attachment side, never files.scope_channel_id (it holds
-- only before attaching, 05 2.2). Deliverable is upload_state complete, scan_status clean or
-- skipped, not deleted and purpose attachment (12 section 2). message_attachments has one row per
-- file (UNIQUE (file_id)), and the deletion of a message deletes its rows and marks its files
-- deleted (DeleteForMessage), so every file is at most one candidate and none of a deleted
-- message is one.

-- name: SearchFiles :many
-- SearchFiles (14 4.5 to 4.8, brief M3-14 design note 6, brief M3-15 design note 11), one
-- statement for both orders. A file matches by its own vector (name A, transcript B; by_file) or
-- by the vector of its extracted text (stems C, words D; by_text, M3-15); each branch has its own
-- GIN-indexed predicate, the same visibility, deliverable, filter and reference-time conditions,
-- and at most max_candidates plus one rows. found is their union (a file found both ways counts
-- once), newest first, again at most max_candidates plus one; capped reports the extra row, which
-- kept drops before ranking. score is ts_rank over the file vector joined with the text vector
-- (the extract row in any state: failed and unsupported rows have an empty vector, a pending
-- refresh keeps the old text), divided by the age from ref_time, never now(), over
-- recency_seconds (0 for newest). page is the keyset strictly after the from position in the order
-- of the sort (score, id or created_at, id, both descending), page_size rows plus one that only
-- tells a further page exists; n numbers them in that order. The outer join on the one row of flag
-- keeps capped on an empty page; its page columns are then NULL. ts_headline reads only the
-- page_size rows the caller returns (n <= page_size): the name, the transcript and the extracted
-- text joined by line feeds, cut at headline_max_chars, in the language of the text (simple
-- without one), so a stem match in the document text is highlighted.
WITH by_file AS (
    SELECT f.id, f.created_at
    FROM files f
    JOIN message_attachments a ON a.file_id = f.id
    WHERE f.search_vector @@ sqlc.arg(tsq)::tsquery
      AND a.channel_id = ANY(sqlc.arg(channel_ids)::uuid[])
      AND f.upload_state = 'complete'
      AND f.scan_status IN ('clean', 'skipped')
      AND f.deleted_at IS NULL
      AND f.purpose = 'attachment'
      AND (NOT sqlc.arg(filter_threads)::boolean OR a.thread_id = ANY(sqlc.arg(thread_ids)::uuid[]))
      AND (sqlc.narg(author_id)::uuid IS NULL OR f.uploaded_by = sqlc.narg(author_id)::uuid)
      AND (sqlc.narg(after)::timestamptz IS NULL OR f.created_at >= sqlc.narg(after)::timestamptz)
      AND (sqlc.narg(before)::timestamptz IS NULL OR f.created_at < sqlc.narg(before)::timestamptz)
      AND f.created_at <= sqlc.arg(ref_time)::timestamptz
    ORDER BY f.created_at DESC, f.id DESC
    LIMIT sqlc.arg(max_candidates)::integer + 1
),
by_text AS (
    SELECT f.id, f.created_at
    FROM file_text_extracts e
    JOIN files f ON f.id = e.file_id
    JOIN message_attachments a ON a.file_id = f.id
    WHERE e.search_vector @@ sqlc.arg(tsq)::tsquery
      AND a.channel_id = ANY(sqlc.arg(channel_ids)::uuid[])
      AND f.upload_state = 'complete'
      AND f.scan_status IN ('clean', 'skipped')
      AND f.deleted_at IS NULL
      AND f.purpose = 'attachment'
      AND (NOT sqlc.arg(filter_threads)::boolean OR a.thread_id = ANY(sqlc.arg(thread_ids)::uuid[]))
      AND (sqlc.narg(author_id)::uuid IS NULL OR f.uploaded_by = sqlc.narg(author_id)::uuid)
      AND (sqlc.narg(after)::timestamptz IS NULL OR f.created_at >= sqlc.narg(after)::timestamptz)
      AND (sqlc.narg(before)::timestamptz IS NULL OR f.created_at < sqlc.narg(before)::timestamptz)
      AND f.created_at <= sqlc.arg(ref_time)::timestamptz
    ORDER BY f.created_at DESC, f.id DESC
    LIMIT sqlc.arg(max_candidates)::integer + 1
),
found AS (
    SELECT u.id, u.created_at
    FROM (SELECT b.id, b.created_at FROM by_file b UNION SELECT t.id, t.created_at FROM by_text t) u
    ORDER BY u.created_at DESC, u.id DESC
    LIMIT sqlc.arg(max_candidates)::integer + 1
),
cand AS (
    SELECT f.id, f.created_at, f.original_name, f.transcript_text,
           f.search_vector || coalesce(e.search_vector, ''::tsvector) AS search_vector,
           e.text_content, e.search_language, a.message_id, a.thread_id, a.channel_id
    FROM found x
    JOIN files f ON f.id = x.id
    JOIN message_attachments a ON a.file_id = f.id
    LEFT JOIN file_text_extracts e ON e.file_id = f.id
    WHERE a.channel_id = ANY(sqlc.arg(channel_ids)::uuid[])
),
flag AS (
    SELECT count(*) > sqlc.arg(max_candidates)::integer AS capped FROM cand
),
kept AS (
    SELECT * FROM cand ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(max_candidates)::integer
),
scored AS (
    SELECT k.*,
           CASE WHEN sqlc.arg(newest)::boolean THEN 0::float8
                ELSE ts_rank(k.search_vector, sqlc.arg(tsq)::tsquery, sqlc.arg(rank_normalization)::integer)::float8
                     / (1 + extract(epoch FROM (sqlc.arg(ref_time)::timestamptz - k.created_at))::float8
                            / sqlc.arg(recency_seconds)::float8)
           END AS score
    FROM kept k
),
page AS (
    SELECT s.*,
           row_number() OVER (ORDER BY CASE WHEN sqlc.arg(newest)::boolean THEN NULL ELSE s.score END DESC,
                                       CASE WHEN sqlc.arg(newest)::boolean THEN s.created_at END DESC, s.id DESC) AS n
    FROM scored s
    WHERE NOT sqlc.arg(has_from)::boolean
       OR (sqlc.arg(newest)::boolean AND (s.created_at, s.id) < (sqlc.arg(from_created_at)::timestamptz, sqlc.arg(from_id)::uuid))
       OR (NOT sqlc.arg(newest)::boolean AND (s.score, s.id) < (sqlc.arg(from_score)::float8, sqlc.arg(from_id)::uuid))
    ORDER BY CASE WHEN sqlc.arg(newest)::boolean THEN NULL ELSE s.score END DESC,
             CASE WHEN sqlc.arg(newest)::boolean THEN s.created_at END DESC, s.id DESC
    LIMIT sqlc.arg(page_size)::integer + 1
)
SELECT flag.capped::boolean AS capped,
       p.id AS file_id, p.message_id, p.thread_id, p.channel_id, p.created_at,
       coalesce(p.score, 0)::float8 AS score,
       coalesce(CASE WHEN p.n <= sqlc.arg(page_size)::integer THEN
                    ts_headline(coalesce(p.search_language, 'simple'::regconfig),
                                left(concat_ws(E'\n', p.original_name, p.transcript_text, p.text_content),
                                     sqlc.arg(headline_max_chars)::integer),
                                sqlc.arg(tsq)::tsquery, sqlc.arg(headline_options)::text)
                END, '')::text AS headline
FROM flag
LEFT JOIN page p ON true
ORDER BY CASE WHEN sqlc.arg(newest)::boolean THEN NULL ELSE p.score END DESC,
         CASE WHEN sqlc.arg(newest)::boolean THEN p.created_at END DESC, p.id DESC;

-- name: HitFiles :many
-- HitFiles (files.SearchReads): the deliverable files of file_ids attached in channel_ids, in the
-- order of file_ids (ord); the rest is absent.
SELECT f.id, f.object_key, f.sha256, f.size_bytes, f.mime_type, f.declared_mime_type, f.original_name, f.uploaded_by,
       f.purpose, f.scope_channel_id, f.upload_state, f.scan_status, f.width, f.height, f.duration_ms,
       f.transcript_status, f.transcript_language, f.transcript_source, f.created_at
FROM unnest(sqlc.arg(file_ids)::uuid[]) WITH ORDINALITY AS r(id, ord)
JOIN files f ON f.id = r.id
JOIN message_attachments a ON a.file_id = f.id
WHERE a.channel_id = ANY(sqlc.arg(channel_ids)::uuid[])
  AND f.upload_state = 'complete'
  AND f.scan_status IN ('clean', 'skipped')
  AND f.deleted_at IS NULL
  AND f.purpose = 'attachment'
ORDER BY r.ord;

-- name: HitFileThumbnails :many
-- HitFiles: the thumbnails of the listed files that hang in channel_ids; the caller orders them by
-- variant.
SELECT t.file_id, t.variant, t.object_key, t.mime_type, t.width, t.height, t.size_bytes
FROM file_thumbnails t
JOIN message_attachments a ON a.file_id = t.file_id
WHERE t.file_id = ANY(sqlc.arg(file_ids)::uuid[])
  AND a.channel_id = ANY(sqlc.arg(channel_ids)::uuid[]);
