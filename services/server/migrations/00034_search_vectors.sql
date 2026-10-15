-- SPDX-License-Identifier: Apache-2.0
-- Search vectors (M3-13): the language and the full-text vector of messages and threads as
-- docs/spec/03-datenmodell.md section 5 and docs/spec/14-suche.md 3.2 state them, and the platform
-- function search_tsquery of 14 4.2 that builds the query over every language of
-- search.languages plus simple (R-SU-01 to R-SU-03, ADR 0032). The GIN indexes follow in 00035,
-- concurrently and outside a transaction.
--
-- Deliberate choices:
--   - search_language comes first, in its own statement, because the generated column reads it;
--     a constant default adds it without a rewrite (upgrade.md rule M3). The writers (messaging
--     and chat) set it once at creation from the author's or creator's locale through
--     search.locale_map (14 3.1); rows that exist before this migration keep 'simple', and there
--     is no backfill (upgrade.md rule M7, docs/DECISIONS.md D-M3-13-8).
--   - The vectors are STORED generated columns, so PostgreSQL keeps them current without trigger
--     or job and a deleted message (content_md = '') has an empty vector at once (03 section 5,
--     DP-T01). STORED is written out because PostgreSQL 18 creates a virtual column without it
--     (make migrations-lint). Adding them rewrites messages and threads once under an exclusive
--     lock; upgrade.md rule M3 wants new columns without a rewrite, and the deviation is accepted
--     only while no server release exists (D-M3-13-8, RK-38). The expressions are those of 03
--     section 5 character for character; to_tsvector(regconfig, text) is IMMUTABLE.
--   - search_tsquery is the body of 14 4.2: an OR of websearch_to_tsquery over every given
--     configuration plus simple, empty sub-queries dropped, NULL for no term (14 4.1). It runs as
--     the caller (no SECURITY DEFINER) and is STABLE like websearch_to_tsquery. Like the trigger
--     functions of 00002 and 00004 it is not executable by PUBLIC; only bothy_app may call it
--     (least privilege of M1-12 D4, D-M3-13-9). The owner is bothy_owner, which runs the
--     migrations (16 7.1).
--
-- Grants: none on tables. The new columns come with the table grants of 00026 (threads) and
-- 00032 (messages).

-- +goose Up
ALTER TABLE threads ADD COLUMN search_language regconfig NOT NULL DEFAULT 'simple';
ALTER TABLE threads ADD COLUMN title_search tsvector NOT NULL GENERATED ALWAYS AS (
  setweight(to_tsvector(search_language, title), 'A') ||
  setweight(to_tsvector('simple'::regconfig, title), 'A')
) STORED;
ALTER TABLE messages ADD COLUMN search_language regconfig NOT NULL DEFAULT 'simple';
ALTER TABLE messages ADD COLUMN search_vector tsvector NOT NULL GENERATED ALWAYS AS (
  setweight(to_tsvector(search_language, content_md), 'B') ||
  setweight(to_tsvector('simple'::regconfig, content_md), 'D')
) STORED;

-- +goose StatementBegin
CREATE FUNCTION search_tsquery(langs regconfig[], q text) RETURNS tsquery
LANGUAGE sql STABLE AS $$
  SELECT string_agg('(' || tq::text || ')', ' | ')::tsquery
  FROM (
    SELECT websearch_to_tsquery(l, q) AS tq
    FROM unnest(array_append(langs, 'simple'::regconfig)) AS l
  ) s
  WHERE numnode(tq) > 0
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION search_tsquery(regconfig[], text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION search_tsquery(regconfig[], text) TO bothy_app;

-- +goose Down
-- Production never runs Down (upgrade.md rule M12); the generated columns go before the column
-- they read.
DROP FUNCTION search_tsquery(regconfig[], text);
ALTER TABLE messages DROP COLUMN search_vector;
ALTER TABLE messages DROP COLUMN search_language;
ALTER TABLE threads DROP COLUMN title_search;
ALTER TABLE threads DROP COLUMN search_language;
