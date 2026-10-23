# Module `search`

Status: built (M3-14). The coordinator of full-text search and of the prefix search of the quick switcher.
The normative sources are [docs/spec/14-search.md](../../../../docs/spec/14-search.md) and [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapters apply.

## Purpose

Full-text search over messages, thread titles and files with a permission filter and ranking (`GET /api/v1/search`), and the prefix search over thread titles (`GET /api/v1/search/thread-titles`). The module owns no tables: it takes the visible channels from `Authorizer.Scope` and reads through the read operations of `messaging`, `chat` and `files` (14 section 10, R-SU-09).

## Code

- `internal/search` (port): the inbound port `Search` with `Query` and `ThreadTitles` and its DTOs `Request`, `Result`, `ResultHit` and `PrefixRequest`.
- `domain/`: the cleaning and length rule of `q` (14 4.1), the table of filters per result type (14 4.4), the page size and the canonical binding of a cursor to its request.
- `app/`: the `Coordinator` with the outgoing ports `SearchIndex`, `Structure` (part of `chat.SearchReads`), `Cursors`, `Metrics` and `ErrorCounter`.
- `adapters/postgres`: `SearchIndex` over PostgreSQL. It builds the tsquery text with `search_tsquery` over `search.languages` plus `simple` and the prefix query of 14 4.9 (no table is read), and delegates every search to `messaging.SearchMessages`, `chat.SearchThreadTitles`, `files.SearchFiles` and `chat.PrefixThreadTitles`. An external engine would replace only this adapter (ADR 0032).
- `adapters/httpapi`: the strict-server methods `Search` and `SearchThreadTitles` (type `Finder`), the cursor codec of M1-18 and the rendering of the hit objects through the read operations and REST renderers of the owning modules, which `internal/wire` passes in.
- `adapters/otelmetrics`: `bothy.search.duration` with `bothy.outcome` (`ok`, `client_error`, `timeout`, `server_error`), for `search` only.
- The stateless shared library is `internal/kernel/searchkit` (query, page and hit types, keyset position, limits, random highlight markers, the split of a headline into segments).

## Flow of one search

1. Parameter rules without a lookup: `q` longer than `search.query_max_chars` (runes of the raw value) is `400 validation_failed` with `too_long`; a filter outside its types is `not_applicable`; the page size is capped at `search.page_size_max`. Invalid UTF-8 and control characters in `q` become spaces.
2. The cursor is decoded against the canonical binding (type, raw `q`, sort, every filter, the principal); any mismatch is `400 cursor_invalid`.
3. `authorize(search.query, instance)`.
4. One read unit of work with `txn.StatementTimeout(search.statement_timeout)`: the tsquery (no term is `400 validation_failed` with `empty`), `Authorizer.Scope` of `message.read`, `thread.read` or `file.read` (cut by `ViewAs` and `ActingFor`), the channel set cut by `channel_id` and by the channels of the visible spaces of `space_id`, the read operation, the page check and the names of `chat.Labels`. An empty channel set runs no query.
5. A statement timeout answers `503 unavailable` with `reason` `search_timeout`; there is no retry.

Filters on thread facts (`thread_state`, `tag_id`, `agent_id`) for messages and files are composed through `chat.ThreadsMatching` into a thread id list in the same unit (07 7.4 rule 6).

## Page check

After the read operation the coordinator reads the rights again in a fresh request scope in filter mode: one `Scope`, then for each hit its channel in that scope and `authorize`. A hit whose channel left the scope lost its permission meanwhile and drops silently; a hit `authorize` refuses although its channel is visible is a mismatch between the SQL filter and `authorize`: it drops, one log line without IDs is written and `bothy.authz.errors` counts `filter_mismatch`. Resources carry only IDs that never change. Dropping never moves the cursor.

## Residual risk

The duration of a search still depends on hits in channels the caller cannot read, because the index filters by channel only afterwards (14 4.3, RK-10). `search.max_candidates`, `search.statement_timeout` and the rate-limit class `rl.search` bound it.

## Port (root package `internal/search`)

- Inbound: `Search`.
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: none.

## Dependencies

May use the ports of these modules: `chat`, `messaging`, `files`.

Reason: thread titles, messages, file names and transcripts.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

None. The module reads through the ports of other modules (03-data-model.md, principle 12).

## Actions (05-actions.md)

`search.query` (resource instance, right `auth`, rate-limit class `rl.search`, effect `read`, no audit entry for an allow; apps need `messages:read`).

## Events and WebSocket ops

- Durable: none.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Telemetry

`bothy.search.duration` for `search`; the prefix search is measured by `http.server.request.duration`. No log field, span attribute or metric attribute carries `q`, an excerpt or a name (SEC-LOG-4).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [07-permissions.md](../../../../docs/spec/07-permissions.md), [14-search.md](../../../../docs/spec/14-search.md)
