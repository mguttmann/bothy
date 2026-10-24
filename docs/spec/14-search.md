# 14 Search

Status: draft M0. Normative for the full-text search in the server, the file text extraction and the
offline search in the client. Terms: [00-glossary.md](00-glossary.md). Tables and search vectors:
[03-data-model.md](03-data-model.md) section 5; this chapter defines what section 5 there assigns
to it: "language choice, weighting, ranking, highlighting". Permissions:
[07-permissions.md](07-permissions.md) and [05-actions.md](05-actions.md) (`search.query`). Files:
[12-files.md](12-files.md). Decision: ADR 0032 (full-text search in PostgreSQL).

All limits and targets are named configuration values with a default in section 15. External
facts refer to the list of sources (section 16). The SQL examples were run on a
throwaway instance of PostgreSQL 16. The target is PostgreSQL 18 [Q1, verified]; its
documentation describes the functions used in the same way [Q2, Q3, verified]. Since M3-13 the
language tests of section 12, the examples of 3.3 and 4.2 and the catalog of the generated
columns run as build tests on PostgreSQL 18 (`pgtest`, `migrations/search_vectors_test.go`,
`test/integration/search_test.go`); every result matched the PostgreSQL 16 examples.

## 1. Goals and limits

1. **Permissions above all.** Hits, snippets and counts come only from channels that the
   principal may read. The query filters in SQL (P2), not afterwards.
2. **German and English well, everything else usable.** Word stems per language plus an
   unchanged word form (`simple`) for names, identifiers and foreign languages.
3. **One database path.** PostgreSQL full text with a GIN index, no second data store
   (ADR 0032). A port allows an external search engine later.
4. **Bounded costs.** Every search has an upper limit of candidates and a time limit.

Not searchable: run steps and live box contents (tool outputs often contain
secrets and noise), channel prompts (readable only with `manage`, R-AZ-09), deleted messages
(empty content, empty vector), files that are not deliverable ([12-files.md](12-files.md)
section 2), drafts.

## 2. What is searchable

| Result type | Source | Column | Weight | Condition |
|---|---|---|---|---|
| `messages` | message text, word stems | `messages.search_vector` | B | `deleted_at IS NULL` |
| `messages` | message text, unchanged words | `messages.search_vector` | D | as above |
| `threads` | thread title, stems and words | `threads.title_search` | A | thread visible |
| `files` | file name | `files.search_vector` | A | file deliverable and attached to a visible, non-deleted message |
| `files` | transcript | `files.search_vector` | B | as above |
| `files` | file text (section 5), stems | `file_text_extracts.search_vector` | C | as above |
| `files` | file text, unchanged words | `file_text_extracts.search_vector` | D | as above |

Weights are the four classes of PostgreSQL. By default `ts_rank` weights them with
`{0.1, 0.2, 0.4, 1.0}` for D, C, B, A [Q2, verified]. This matters above all for files: a
hit in the name counts more than one in the transcript and more than one in the document text.
Stem forms (B) count more than merely equal word forms (D), because a stem hit in the
right language is the stronger signal; the `simple` form catches the rest.

## 3. Languages and search vectors

### 3.1 Language per object

The column `messages.search_language` (`regconfig`) already exists. It is set on creation
and not changed afterwards, except by a reindexing (section 8):

- Person as author: `search.locale_map[users.locale]` (first language tag, for example `de`).
- Agent, app, import, system: `search.locale_map[instance_settings['locale.default']]`.
- No entry in the map: `simple`.

Threads get the same rule via the creator (column `threads.search_language`, 03).
File text gets the language of the instance.

Details (M3-13, docs/DECISIONS.md D-M3-13-1 to D-M3-13-6): the key of the map is the first
language subtag of the locale (the part before the first `-`, ASCII lower case; `de-AT` uses
`de`); `simple` is allowed as a map value and is never listed in `search.languages`, because
every query adds it. Main threads get the language of the creator of the channel. The language
is computed by `messaging` and `chat` through `identity.Directory` in the unit of work of the
insert, never comes from a request and is never changed by an edit, a change of `users.locale`
or of `locale.default`, or a move. Besides the configuration check, the start checks the
database (`pg.CheckTextSearch`): every configuration of `search.languages` and
`search.locale_map` must exist, and the database must fold the case of non-ASCII letters
([16-operations.md](16-operations.md) 7.2).

`search.languages` names all configurations from which queries are built. The server
does not start if a value from `search.locale_map` is missing there; otherwise queries would never
find the stems of this language. PostgreSQL ships configurations for many languages, including
`german`, `english` and `simple` (checked with `\dF` on PostgreSQL 16).

### 3.2 Expressions

The columns are stored generated columns with a GIN index as in
[03-data-model.md](03-data-model.md) section 5; the expressions there match this
section (with weight and `simple` part, plus the table `file_text_extracts`):

```sql
-- messages
search_vector tsvector GENERATED ALWAYS AS (
  setweight(to_tsvector(search_language, content_md), 'B') ||
  setweight(to_tsvector('simple'::regconfig, content_md), 'D')
) STORED

-- threads (new column search_language regconfig NOT NULL DEFAULT 'simple')
title_search tsvector GENERATED ALWAYS AS (
  setweight(to_tsvector(search_language, title), 'A') ||
  setweight(to_tsvector('simple'::regconfig, title), 'A')
) STORED

-- files: unchanged from 03 (name A, transcript B, both simple)

-- file_text_extracts (new, section 5)
search_vector tsvector GENERATED ALWAYS AS (
  setweight(to_tsvector(search_language, coalesce(text_content, '')), 'C') ||
  setweight(to_tsvector('simple'::regconfig, coalesce(text_content, '')), 'D')
) STORED
```

PostgreSQL 18 creates generated columns without a keyword as virtual columns [Q3, verified].
Every migration must therefore write `STORED` explicitly; a check step in the migration lint
rejects `GENERATED ALWAYS AS` without `STORED`.

One vector instead of two: stems and unchanged words are in the same `tsvector`, each with its own
weight. One GIN query is enough, no OR over two indexes.

### 3.3 Limits of language processing

<!-- language-check: off -->

- Snowball does not split German compounds [Q4, verified]. "Rechnung" finds "Rechnungen", but
  not "Rechnungsadresse" (observed this way on PostgreSQL 16). Hunspell dictionaries could do this
  in part, but they are not shipped, and PostgreSQL supports Hunspell compounds only
  in basic form [Q4, verified]. A test records this limit so that it stays visible.
- The German stemmer folds umlauts (`verfügbar` becomes `verfugbar`); `simple` keeps them. Both
  forms are in the vector.
- Words that the parser splits (for example `ERR_CONN_RESET` into `err`, `conn`, `reset`) are found
  via the parts, more precisely as a phrase.

<!-- language-check: on -->

## 4. Query

### 4.1 Input

- Text `q` in the syntax of `websearch_to_tsquery`: words (AND), `"phrase"`, `or`, `-word`.
  This function never raises a syntax error [Q2, verified]; every input is safe.
- Length at most `search.query_max_chars`, otherwise `400 validation_failed` with `errors[]` on
  `q` and code `too_long` ([06-api.md](06-api.md), section 5).
- If no term remains after processing (only punctuation), the server responds
  `400 validation_failed` with `errors[]` on `q` and code `empty`.
- Filters come as separate parameters, not as text in `q` (4.4). The app may offer a
  notation such as `from:Alex` and translate it into parameters; the server does not interpret `q`.

### 4.2 Query construction

The query is an OR over all languages from `search.languages` plus `simple`. Empty
subqueries (for example only stop words in one language) are dropped:

```sql
CREATE FUNCTION search_tsquery(langs regconfig[], q text) RETURNS tsquery
LANGUAGE sql STABLE AS $$
  SELECT string_agg('(' || tq::text || ')', ' | ')::tsquery
  FROM (
    SELECT websearch_to_tsquery(l, q) AS tq
    FROM unnest(array_append(langs, 'simple'::regconfig)) AS l
  ) s
  WHERE numnode(tq) > 0
$$;
```

A NULL result means "no term" (4.1). With `'{german,english}'`, `Rechnungen` becomes
`'rechnung' | 'rechnungen' | 'rechnungen'`; the GIN query uses the index (plan checked).
The function is created by a platform migration. Migration `00035_search_vectors.sql` (M3-13) creates it; it belongs
to `bothy_owner`, runs as the caller, and only `bothy_app` may execute it (`REVOKE ALL ... FROM
PUBLIC`, D-M3-13-9). On PostgreSQL 18 the examples give the same text as on 16: `the` gives
`'the' | 'the'` (the English branch holds only a stop word), only punctuation, empty text, white
space and NULL give NULL, `'{}'` or a NULL array give only the `simple` branch, and `-z` gives
`!'z' | !'z' | !'z'`. A query of only negations matches nearly every row; it is bounded only by
`search.max_candidates` and `search.statement_timeout` (M3-14). The text round trip inside the
function is lossless (fuzz test); the text form of a nested OR has no parentheses, so
`r::text::tsquery` is the same query with another tree.

Known imprecision: negation (`-word`) applies per subquery. If a text contains the word only in
another form, the `simple` subquery can still match it. Negation is therefore
"best effort" and documented as such.

### 4.3 Permission filter

1. `authz` returns the set of visible channels of the principal for `message.read` from the
   permission cache (R-DM-19, [07-permissions.md](07-permissions.md)). It contains DM channels only for
   participants, also for the owner (R-AZ-02).
2. Every query contains `channel_id = ANY($visible)` with the list that `Authorizer.Scope` builds per
   request ([07-permissions.md](07-permissions.md), section 7.4; [03-data-model.md](03-data-model.md)
   section 5). Threads filter via `threads.channel_id`, files via the channel of the
   message they are attached to.
3. The entries of the delivered page additionally go through `authorize(..., message.read |
   thread.read | file.read, ...)`. A rejected entry is silently dropped. This catches permissions
   that changed between loading the channel set and the query. The check runs in filter mode in a
   fresh request scope, so it reads the rights version again, and calls `Authorizer.Scope` once
   more: an entry whose channel is missing from that fresh scope lost its permission meanwhile and
   drops without a metric; an entry whose channel is still visible but that `authorize` refuses is
   a mismatch between SQL filter and `authorize`, drops too, and counts `bothy.authz.errors` with
   `bothy.reason = filter_mismatch` and one log line without IDs. Resources carry only IDs that
   never change (message and thread, thread, file and the thread of its message), never the channel
   copy. Dropping never moves the cursor: the next page continues after the last row the statement
   returned (M3-14).
4. There is no total count and there are no counts per channel. The response only says whether
   there are more pages and whether the candidate limit was reached.
5. Agents search with their grants. If an agent searches within a run (invocation token in the
   request), `Authorizer.Scope` additionally intersects the channel set with the permissions of the
   triggering person (SR9, [07-permissions.md](07-permissions.md)). Steps 1 to 4 then run on
   this intersection; candidate limit, `candidates_capped`, `has_more` and cursor therefore
   arise only from channels that both may read. Reason: otherwise the candidate limit and
   page markers would tell the person, through the agent, something about channels they may not
   read (confused deputy). Apps need the scope `messages:read` ([05-actions.md](05-actions.md)
   2.6).

Residual risk: the duration of a search also depends on hits in other channels, because the index
filters by channel only afterwards. Through many measured queries, an attacker could infer the
frequency of a word. Candidate limit, time limit and rate limit `rl.search`
limit this; the item is in the threat model ([19-security.md](19-security.md)).

### 4.4 Filters

| Parameter | Effect | Types |
|---|---|---|
| `space_id` (multiple) | only channels of these spaces; intersection with the visible ones | all |
| `channel_id` (multiple) | only these channels; intersection with the visible ones | all |
| `author_id` | `messages.author_principal_id` or `files.uploaded_by` respectively | `messages`, `files` |
| `agent_id` | thread with `threads.operator_agent_id` = agent (conversations with this agent, including the messages of the people) | all |
| `after`, `before` | `created_at` in the half-open interval | all |
| `has_file` | at least one attachment | `messages` |
| `thread_state` | `open` or `closed` | `threads`, `messages` |
| `tag_id` (multiple) | forum tags of the thread | `threads`, `messages` |
| `thread_id` | only this thread (for `threads` the thread itself) | all |

An unknown or invisible channel, space, thread or tag in the filter yields no hits, not an
error (otherwise the error would reveal the existence). A filter outside its types gives `400
validation_failed` with `errors[]` on `#query/<name>` and code `not_applicable`. Several `tag_id`
values match a thread with at least one of them; `has_file=false` keeps messages without
attachments.

### 4.5 Ranking and sorting

`sort = relevance` (default) or `newest`.

1. Candidates: the newest `search.max_candidates` hits after filters and permissions. Ranking
   must read the vector of every hit and is expensive with many hits [Q2, verified]; the
   limit makes the cost independent of the number of hits. If it is reached, the
   response reports `candidates_capped: true`, and the app asks for a narrower search.
2. `relevance`: score = `ts_rank(vector, query, search.rank_normalization)` divided by
   `1 + age / search.recency_scale`. Normalization 1 divides by `1 + log(length)` [Q2,
   verified], so that long messages do not win by length alone. On a tie, the
   higher ID (newer) wins.
3. `newest`: by `created_at` descending, then ID.

### 4.6 Highlighting

- Only for the delivered page, because `ts_headline` reads the original instead of the vector and
  can be slow [Q2, verified].
- Input truncated to `search.headline_input_max_chars`.
- Options: `MaxFragments = search.headline_max_fragments`, `MaxWords =
  search.headline_max_words`, `MinWords = search.headline_min_words`, `ShortWord=3` [Q2, verified].
- Markers: `StartSel` and `StopSel` are random hex strings per request (128 bits).
  The server splits the result at them into segments `[{text, match}]`. A text cannot contain the
  marker in advance; HTML in the content stays text. The default markers `<b>`
  would be unsafe, because `ts_headline` does not guarantee safe HTML output [Q2, verified].
  The parser of `ts_headline` reads an HTML tag of the content as one token and puts a space in
  its place in the excerpt; entities and every other text stay as they are (M3-14).

### 4.7 Paging

Cursor by (score, ID) for `relevance` and (`created_at`, ID) for `newest`, opaquely
encoded, with a hash of `q`, filters and sorting. If the hash does not match the request:
`400 cursor_invalid` (06, section 5). Page size `search.page_size_default`, at most
`search.page_size_max`; the page size is not part of the hash. The cursor also carries the
reference time of the first page (the request time of the server clock): every later page
measures the age of the score from it and keeps only candidates created at or before it, so a
message created between two pages neither shifts the scores nor pushes a candidate out. The
cursor is bound to the principal too, so the cursor strings of a person and of an agent in that
person's run differ while their positions and reference times are equal (M3-14).

### 4.8 Example: messages

Checked on PostgreSQL 16. `$4` is the visible channel set, `$5` the candidate limit.

```sql
WITH p AS (
  SELECT search_tsquery($1::regconfig[], $2) AS tsq
),
cand AS (
  SELECT m.id, m.created_at, m.search_language, m.content_md,
         ts_rank(m.search_vector, p.tsq, $3) AS rank
  FROM messages m, p
  WHERE m.search_vector @@ p.tsq
    AND m.channel_id = ANY ($4::uuid[])
    AND m.deleted_at IS NULL
  ORDER BY m.created_at DESC
  LIMIT $5
),
page AS (
  SELECT c.*, c.rank / (1 + extract(epoch FROM now() - c.created_at) / $6) AS score
  FROM cand c
  ORDER BY score DESC, id DESC
  LIMIT $7
)
SELECT pg.id, pg.score,
       ts_headline(pg.search_language, left(pg.content_md, $8), p.tsq,
                   'StartSel=' || $9 || ',StopSel=' || $10 ||
                   ',MaxFragments=2,MaxWords=30,MinWords=10,ShortWord=3') AS snippet
FROM page pg, p
ORDER BY pg.score DESC, pg.id DESC;
```

`$6` is `search.recency_scale` in seconds; the option values in the example are the defaults from
section 15 and in the code come from the configuration. Filters from 4.4 and the cursor are added
as further conditions in `cand` or `page` respectively.

### 4.9 Prefix search for the quick switcher

The quick switcher (Cmd/Ctrl+K, [01-product.md](01-product.md)) searches spaces and channels in the
local snapshot of the client, and thread titles on the server by prefix:

```sql
SELECT string_agg(quote_literal(lexeme) || ':*', ' & ')::tsquery
FROM unnest(to_tsvector('simple', $1))
WHERE length(lexeme) >= $2;   -- search.prefix_min_chars
```

`rechnungs okt` becomes `'okt':* & 'rechnungs':*`; this finds "Rechnungslauf Oktober" (checked).
Words shorter than `search.prefix_min_chars` are dropped; if nothing remains, the result is NULL
and the response is empty.
Prefix queries are provided for in `to_tsquery` with `:*` [Q2, verified]. Same permission filter as
4.3, sorted by `last_activity_at`.

## 5. File text extraction

### 5.1 Flow

1. If a file is deliverable ([12-files.md](12-files.md) section 2) and its type is
   supported, the worker creates a row `file_text_extracts` (`state = 'pending'`) and
   enqueues the job `files.extract_text` in the queue `files`
   ([02-architecture.md](02-architecture.md), section 12.3).
2. The job calls the port `TextExtractor` (`Extract(ctx, reader, mime) -> text, truncated`).
3. Result: `state = 'done'`, text at most `search.extract.max_text_bytes` (`truncated`
   marks truncation), `extractor` with name and version, `extracted_at`. Error: `failed` with
   a reason without content. Unsupported type: `unsupported`.
4. If the version of an extractor increases, a maintenance job catches up on older rows.

### 5.2 Types and tools

| Type | Adapter | License | Note |
|---|---|---|---|
| `text/plain`, `text/markdown`, `text/csv`, `application/json`, source code | `builtin` | own code | check UTF-8, replace invalid bytes |
| `text/html` | `builtin` with `golang.org/x/net/html` | BSD-3-Clause [Q7, verified] | text nodes only; scripts and styles are dropped |
| DOCX, XLSX, PPTX | `builtin` with `archive/zip` and `encoding/xml` | Go standard library | text from document, shared strings and slides |
| ODT, ODS, ODP | `builtin` | Go standard library | `content.xml` |
| PDF, older Office formats, RTF, EPUB and more | `tika` (optional) | Apache-2.0 [Q8, derived] | Apache Tika Server as a separate container |

`builtin` is pure Go and always on. `tika` is off (`search.extract.tika_url` empty), because it
is a Java runtime with its own memory needs. Without `tika`, PDFs can be found only via name and
message. Reason for Tika instead of a pure Go library for PDF: reading text from PDFs
robustly takes a lot of effort, and a separate process separates errors in the parser from the
server, as with the malware scan (ADR 0033).

Tika call: `PUT /tika` with the raw bytes and `Accept: text/plain` [Q5, verified]; embedded
files are skipped with `X-Tika-Skip-Embedded: true` [Q5, verified]. The server documentation
describes for 2.x: parsing in a child process that restarts after running out of memory or a
timeout; during that time the server responds `503` [Q5, verified]. The job then retries. 2.x
has been at end of life since 2025; current are 3.x (maintained) and 4.1.0 [Q8, verified]; whether
the behavior is the same in the pinned version is checked by the integration test of the adapter.
The container has no outbound network access and is reachable only in the internal network.

### 5.3 Limits and protection

- Only files up to `search.extract.max_input_bytes`.
- Archives (Office, ODF): at most `search.extract.max_uncompressed_bytes` unpacked and
  `search.extract.max_archive_entries` entries; otherwise abort with `failed` (protection against
  zip bombs).
- Time limit per file `search.extract.timeout`.
- XML with `encoding/xml`; the package does not load external entities [derived from the absence
  of such a function; to be verified by a test with an XXE file].
- The text is content of the class `content`; it is deleted with the file (ON DELETE CASCADE).

## 6. Offline search in the client

- The client searches its local cache ([04-sync-protocol.md](04-sync-protocol.md) 13.1)
  with SQLite FTS5. drift ships an SQLite version with FTS5; FTS5 tables must be in
  `.drift` files, queries can use `bm25`, `highlight` and `snippet` [Q6, verified].
- Index: messages, thread titles and transcripts that are in the cache. No file text.
- Permissions: the cache contains only what the client was allowed to see. On `access.changed` with
  `removed`, the client also deletes the local search index of these targets
  ([04-sync-protocol.md](04-sync-protocol.md) 8.3). On session or device revocation, everything.
- Display: offline, the app shows local hits with the note "messages stored only on this
  device". Online, the server results apply; local hits only supplement them
  while the server response is pending.
- Limits: older threads that were never loaded are missing; word stems differ from the server
  (tokenizer choice in [15-client-architecture.md](15-client-architecture.md)). The local index is in
  the encrypted database of the client.

## 7. Performance targets and load test

| Target | Value |
|---|---|
| p95 of the server duration of `GET /api/v1/search` (type `messages`, without network) | `search.p95_target` |
| at corpus size | `search.benchmark_messages` |
| prefix search p95 | `search.prefix_p95_target` |

Load test: synthetic corpus with German and English texts, word frequency following a
Zipf distribution, several spaces and channels, a principal sees a part of the channels.
Queries with one to three words, phrases and negation. Measurement on the reference profile from
[16-operations.md](16-operations.md) (capacity). The test runs in the release workflow and by hand,
not on every PR (run time). If it misses the target, that is a release blocker.

Measuring point: from the scheduled send time to the fully read response over loopback, an upper
bound of the server time; the server runs with its default configuration; open load below
`ratelimit.rl.search.per_minute` per person; requests with the defaults of `openapi.yaml`. The test
`perf/search` runs in `make perf`; `make bench-search` runs it by hand. The same run measures the
rewrite of `messages.search_vector` per million messages (section 8), the share of the search
indexes in the database size and the time difference between a term that occurs only in channels
the searcher cannot read and a term that occurs nowhere (residual risk of 4.3).

Every search query runs with `SET LOCAL statement_timeout` = `search.statement_timeout`. On a
timeout: `503 unavailable` with `reason` `search_timeout` and the hint to narrow the
search.

## 8. Index maintenance

- Generated columns keep vectors current without a trigger or job
  ([03-data-model.md](03-data-model.md) section 5).
- GIN indexes with default settings (fast insert via a pending list that autovacuum
  merges in). A dashboard watches index size and bloat
  ([17-observability.md](17-observability.md)); if needed, `REINDEX CONCURRENTLY`.
- **Changing the expression** (new weights, new languages in the expression) rewrites the table. On
  PostgreSQL 16, `ADD COLUMN ... GENERATED ... STORED` changes the file node of the table
  (checked); the same applies to 18 [derived]. This needs a maintenance window. The load test measures the duration per million messages
  (section 7); the upgrade guide (`docs/operations/upgrade.md`) says how the release notes turn it
  into the maintenance window.
- **Reassigning the language** (for example after a change of `search.locale_map`): a job resets
  `search_language` in small batches. Updating the base column recomputes the
  vector, row by row, without a table lock.

## 9. Limit and outlook

An external search engine comes only when one of these conditions occurs:

1. The load test misses `search.p95_target` on a real data volume after query and indexes
   have been optimized.
2. Fault tolerance (typos) or compound splitting in German is required (3.3).
3. Cross-type ranking (messages, threads, files in one list) is required.

Intermediate step before an external engine: `pg_trgm` for titles and file names (partial-word and
similarity search), an extension that PostgreSQL ships.

The port `SearchIndex` stays the same (section 10). An external adapter reads the
event log like a consumer, stores the `channel_id` per document and gets the visible channel set
from `authz` as a filter with every request. There is never permission logic in the search index
itself (P2, B7). Deletions come via `message.deleted` and redaction
([03-data-model.md](03-data-model.md) 6.5).

## 10. Module and port

The search reads tables of three modules (`messaging`, `chat`, `files`). According to
[03-data-model.md](03-data-model.md) 1.12, a module reads other modules' tables only via their port.
Therefore:

- `messaging`, `chat` and `files` each offer a read operation: `SearchMessages`,
  `SearchThreadTitles`, `SearchFiles`. They receive query, filters, visible channels and
  limits and return hits with score and segments.
- A coordinator `search` without its own tables owns the port `SearchIndex`, fetches the
  visible channels from `authz` and calls the matching operation. A later external adapter
  replaces only this coordinator adapter.
- Shared parts (query construction, segmentation, cursor) are in an internal library
  without state (`internal/kernel/searchkit`).
- Filters on thread facts (`thread_state`, `tag_id`, `agent_id`) for the types `messages` and
  `files` are composed through `chat` in the same unit of work: `chat.ThreadsMatching` returns the
  matching threads of the visible channels, intersected with `thread_id` when given, and the read
  operation of `messaging` or `files` receives that thread list; an empty list runs no further
  query. For `threads`, `chat` applies the filters itself (07 7.4 rule 6).
- The objects of the delivered hits come from the owning modules too (`messaging.HitMessages`
  with the list representation, `chat.HitThreads` with `Thread`, `files.HitFiles` with `File`),
  called with the IDs and channels of the kept hits; the names come from `chat.Labels`.
- `file_text_extracts` belongs to `files`, like the transcript.

The module list in [02-architecture.md](02-architecture.md) must list `search` as a coordinator;
this is reported to team A in the return report.

## 11. Endpoints

Binding is `docs/api/openapi.yaml` (`search`, `searchThreadTitles`); this table reflects the
state of the spec.

| Method and path | Action | RL class | Purpose |
|---|---|---|---|
| `GET /api/v1/search` | `search.query` | `rl.search` | parameters `q`, `type` (`messages`, `threads`, `files`; default `messages`), filters from 4.4, `sort`, `cursor`, `limit` |
| `GET /api/v1/search/thread-titles` | `search.query` | `rl.search` | prefix search for the quick switcher (4.9) |

Response per hit: object summary (message, thread or file), thread title, channel
and space name, `segments`, for `relevance` the score. In addition `next_cursor` and
`candidates_capped`.

## 12. Tests

| Test | Holds |
|---|---|
| "Rechnungen" finds "Rechnung" (German), "invoice run" finds "invoices are running" (English), `err_conn_reset` finds the identifier | section 3 |
| "Rechnung" does not find "Rechnungsadresse" | documented limit 3.3 |
| Random inputs (fuzzing) | never an error other than `400 validation_failed` on `q` (`empty` or `too_long`) |
| Two principals, same query, different grants | hits only from their own visible channels; no counts |
| Owner searches | no hits from other people's DMs |
| Agent with `read` on channel B searches in a run of Alex, who may not read B; channel B contains so many hits that `search.max_candidates` would be reached without the intersection | the same response as for Alex's own search: no hits from B, same `candidates_capped`, same `has_more` and same cursor (SR9) |
| Revocation, then search | no hits from the revoked channel; the page check catches a race (test with revocation between channel set and query) |
| Filter on an invisible channel | empty result, same response as for an unknown channel |
| Deleted message, file `pending` or `infected`, file never attached | no hit |
| Content with `<b>` and text that looks like a marker | segments correct, no HTML |
| Cursor with a changed query | `400 cursor_invalid` |
| Migration lint | `GENERATED ALWAYS AS` without `STORED` fails |
| Start with a language in `search.locale_map` that is missing from `search.languages` | start error |
| Office file with a zip bomb, XML with an external entity | `failed`, nothing loaded |
| Load test | `search.p95_target` at `search.benchmark_messages` and `search.prefix_p95_target` (`perf/search`) |
| Client: `access.changed` with `removed` | the local FTS index no longer contains any row of these targets |

## 13. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-SU-01 | One `tsvector` per object with language stems and `simple` words, weighted differently. | One index, one predicate; names, identifiers and foreign languages stay findable. |
| R-SU-02 | Language per message from the language of the author, stored; no automatic language detection. | Detection works poorly on short chat messages; the `simple` form catches wrong assignments. |
| R-SU-03 | Query as an OR over all instance languages plus `simple`. | One query matches texts of every configured language. |
| R-SU-04 | Candidate limit before ranking, newest first. | Cost independent of the number of hits; in chat, what is recent counts. |
| R-SU-05 | One result type per query, no total count. | No cross-type ranking needed; counts are expensive and a leak risk. |
| R-SU-06 | Highlighting with random markers and segments instead of HTML. | No HTML injection, unambiguous splitting. |
| R-SU-07 | File text: pure Go for text and Office formats, Tika optional as a separate service. | No forced JVM; parser risk outside the server. |
| R-SU-08 | Run steps are not searchable. | Tool outputs contain secrets and noise. |
| R-SU-09 | Search as a coordinator with read ports of the owning modules. | Keeps the module rule from 03; an external adapter replaces only the coordinator. |

## 14. Alignment with the core contract

The additions requested during drafting are decided (integration of 2026-10-10):

| No. | Addition | Decision, location |
|---|---|---|
| 1 | new expressions for `messages.search_vector` and `threads.title_search` | adopted, 03 section 5 |
| 2 | column `threads.search_language` | adopted, 03 `threads` |
| 3 | table `file_text_extracts` | adopted, 03 section 4.7 |
| 4 | scope `messages:read` for `search.query` | adopted, 05 section 2.6 |
| 5 | module `search` as a coordinator without tables | adopted, 03 section 2 |

## 15. Configuration values of this chapter

Column `Place`: `Server` = configuration reference of the server; `Load test` = threshold or
parameter of the load test like `perf.*` in 02 section 22, without an environment name and not in
the configuration schema; `make perf` reads them from this table.

| Name | Place | Default | Meaning |
|---|---|---|---|
| `search.languages` | Server | `["german","english"]` | configurations for query construction (plus always `simple`) |
| `search.locale_map` | Server | `{"de":"german","en":"english"}` | language tag to text search configuration |
| `search.query_max_chars` | Server | `256` | length of the input |
| `search.max_candidates` | Server | `2000` | candidates before ranking |
| `search.rank_normalization` | Server | `1` | normalization for `ts_rank` |
| `search.recency_scale` | Server | `30d` | age at which the score falls to half |
| `search.page_size_default` | Server | `20` | hits per page |
| `search.page_size_max` | Server | `50` | maximum hits per page |
| `search.headline_input_max_chars` | Server | `20000` | length of the text for `ts_headline` |
| `search.headline_max_fragments` | Server | `2` | snippets per hit |
| `search.headline_max_words` | Server | `30` | words per snippet, at most |
| `search.headline_min_words` | Server | `10` | words per snippet, at least |
| `search.prefix_min_chars` | Server | `2` | shortest word length in the prefix search |
| `search.statement_timeout` | Server | `3s` | time limit of a search query |
| `search.p95_target` | Load test | `500ms` | p95 target in the load test |
| `search.prefix_p95_target` | Load test | `100ms` | p95 target of the prefix search |
| `search.benchmark_messages` | Load test | `2000000` | corpus size in the load test |
| `search.extract.max_input_bytes` | Server | `52428800` | largest file for the extraction |
| `search.extract.max_text_bytes` | Server | `1048576` | largest stored text |
| `search.extract.max_uncompressed_bytes` | Server | `209715200` | largest unpacked amount for archives |
| `search.extract.max_archive_entries` | Server | `10000` | entries per archive |
| `search.extract.timeout` | Server | `60s` | time limit per extraction |
| `search.extract.tika_url` | Server | empty | Tika server; empty = off |

## 16. Sources

All retrieved on 2026-10-10.

- Q1 PostgreSQL 18, versions and support: https://www.postgresql.org/support/versioning/ [verified]
- Q2 PostgreSQL 18, Controlling Text Search (`websearch_to_tsquery`, `ts_rank`, `ts_headline`, prefix): https://www.postgresql.org/docs/18/textsearch-controls.html [verified]
- Q3 PostgreSQL 18, Generated Columns: https://www.postgresql.org/docs/18/ddl-generated-columns.html [verified]
- Q4 PostgreSQL, Dictionaries (Snowball, Ispell/Hunspell): https://www.postgresql.org/docs/current/textsearch-dictionaries.html [verified]
- Q5 Apache Tika Server: https://cwiki.apache.org/confluence/display/TIKA/TikaServer [verified]
- Q6 drift, Supported sqlite extensions: https://drift.simonbinder.eu/sql_api/extensions/ [verified]
- Q7 `golang.org/x/net/html`: https://pkg.go.dev/golang.org/x/net/html [verified]
- Q8 Apache Tika, home page and release notes: https://tika.apache.org/ [verified for versions; license only through the link to the ASF license page, therefore derived]
