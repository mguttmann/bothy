# M3-14: Search coordinator and prefix search

Status: ready | Depends on: M1-57, M3-07, M3-13 | Milestone: M3
## Goal

After this ticket `GET /api/v1/search` (`search`) and `GET /api/v1/search/thread-titles` (`searchThreadTitles`) answer people, agents and apps with hits from messages, thread titles and files that the caller may read, and from nothing else. The module `search` owns no tables: it takes the visible channels from `Authorizer.Scope` (cut down to the triggering person for an agent in a run, SR9), lets `messaging`, `chat` and `files` run the full-text query with `channel_id = ANY($visible)` through their own read operations, ranks the newest `search.max_candidates` candidates, highlights the delivered page as plain text segments split at random markers, pages with an opaque cursor bound to the request, and checks every delivered item with `authorize` again, so a revocation between building the channel set and running the query cannot leak. The quick switcher gets a prefix search over thread titles with the same filter. No total, no per-channel count, no `candidates_capped` flag and no cursor depends on a channel the caller cannot read (T-AZ-02, AZ-M06, AZ-P5), and no search term reaches logs, traces, metrics or the audit log (SEC-LOG-4, SEC-T10). This is the server side of "Full-text search with permission filter" and "Quick switcher" in 01 3.5 and of ADR 0032 ("PostgreSQL full text behind a port").

## Read first

- `docs/spec/14-search.md`: the whole chapter; in particular 1 "Goals and limits" (not searchable), 2 "What is searchable", 3.1 "Language per object" (`search.languages`), 4.1 "Input", 4.2 "Query construction" (`search_tsquery`), 4.3 "Permission filter" (items 1 to 5 and the residual risk), 4.4 "Filters", 4.5 "Ranking and sorting", 4.6 "Highlighting", 4.7 "Paging", 4.8 "Example: messages", 4.9 "Prefix search for the quick switcher", 7 "Performance targets and load test" (statement timeout, `search_timeout`), 9 "Limit and outlook", 10 "Module and port", 11 "Endpoints", 12 "Tests", 13 (R-SU-01 to R-SU-09), 15 "Configuration values of this chapter".
- `docs/spec/07-permissions.md`: 2.8 "Direct messages", 3 "Principal context" (`acting_for`, `view_as`), 4.2 "Flow" (filter mode, denials into the audit), 4.3 "Visibility", SR6, "SR9 Acting-for binding (confused deputy)", 7.2 "Permission state (snapshot)", 7.3 "Validity of the cache", 7.4 "Query pattern" (rules 1 to 6), 11.1 row AZ-M06, 11.2 rows AZ-P5, AZ-P11 and AZ-P12.
- `docs/spec/05-actions.md`: 1 "Binding rules" rules 1, 2 and 9; 2.4 (column "Audit"); 2.6 "Scopes for apps"; 2.7 "Effect (`effect`)"; 3 "Rate-limit classes" (`rl.search`); 4.6 row `thread.read`; 4.7 row `message.read`; 4.8 "Files and search" (rows `file.read`, `search.query`).
- `docs/spec/03-data-model.md`: 1 "Principles" items 7, 12 and 13; 2 "Modules" (row `search`); tables `channels`, `threads` (with the paragraph on moving), `thread_tags`, `messages`, `files`, `message_attachments`, `file_text_extracts`; 5 "Search vectors"; 8 row R-DM-43.
- `docs/spec/02-architecture.md`: 4.1 rule 7; 4.2; 4.3 (rows `search`, `messaging`, `chat`, `files`); 4.4 (row `search`); 4.5 (kernel table, `kernel/authorize`, `kernel/txn`); 5 "Layers within a module" (rule 5, metrics); 7 (table ownership gate); 8.1 "Flow"; 14.1 "Error kinds"; 14.2 "Deadlines".
- `docs/spec/06-api.md`: 5 "Errors" (`validation_failed` with `errors[]`, `cursor_invalid`, `unavailable` with `search_timeout`), 6 "Pagination", 15 "Contract tests and gate" (G7, G8), 16 rows `search` and `searchThreadTitles`.
- `docs/spec/12-files.md`: 2 "States of a file" (definition of deliverable), 3.7 "Purpose of a file", 10 "Attaching and agent invocation".
- `docs/spec/01-product.md`: 3.5 "Notification, search, navigation", D6 (quick switcher), D7 (search).
- `docs/spec/19-security.md`: SEC-LOG-4, SEC-T10. `docs/spec/20-privacy.md`: DP-T01.
- `docs/api/openapi.yaml`: operations `search` and `searchThreadTitles`; schemas `SearchHitPage`, `SearchHit`, `SearchHitKind`, `ThreadTitleHitList`, `Message`, `Thread`, `File`, `ThreadState`, `Cursor`, `PageLimit`, `FieldError`, `Problem`; parameters `Cursor`, `Limit`; response `BadRequest`.
- ADRs 0008, 0013, 0015, 0017, 0018, 0032, 0050, 0053, 0065.
- `docs/DECISIONS.md`: D-W-19 (Scope cuts with `ActingFor`, also for the candidate limit, `candidates_capped` and the cursor), D-W-100.
- `docs/security/threat-model.md`: T-AZ-02, T-DS-01. `docs/RISKS.md`: RK-10 (time side channel of the search).
- `docs/observability/signals.md`: 2 "Cardinality", 4.1 `bothy.authz.errors` (`filter_mismatch`), 4.4 `bothy.search.duration`, 5.3; `docs/observability/alerting.md`: `BothyAuthzError`.
- `docs/operations/configuration.md`: every `search.*` key of chapter 14 except `search.extract.*`, `search.p95_target`, `search.prefix_p95_target` and `search.benchmark_messages`; `ratelimit.rl.search.per_minute`, `ratelimit.rl.search.burst`; `message.list_content_max_bytes`; `database.statement_timeout`; `http.unavailable_retry_after`.
- `docs/PLAN.md`: 3 "Way of working per ticket"; 9 "M3 Platform services" (Goal, Scope, Out of scope, Exit criteria 1, 2 and 4, Test strategy "fuzzing of the search input", Red team "T-AZ-02 for search" and "timing side channel of the search", Risks) and rows M3-03, M3-04, M3-07, M3-13, M3-15, M3-16; M2 row M2-14; M1 rows M1-57, M1-58, M1-69, M1-70, M1-71; M4 row M4-23.
- Briefs: M1-13 (unit of work, `SET LOCAL`), M1-18 (cursor codec `EncodeCursor`/`DecodeCursor`, problem writer), M1-28 (`Authorizer.Scope`, `Filter(ctx)`, `WithRequestScope`, `actor.Principal.ActingFor` and `ViewAs`), M1-53, M1-55, M1-56, M1-57 (list representation, `filter_mismatch` handling), M1-62, M1-68 (instrument registry), M1-69 (canary scenario), M1-70 (leak tests, SQL lint), M1-71 (shared test harness); the briefs of M3-03, M3-04, M3-07 and M3-13.
- Code (follow the merged names): `internal/kernel/authorize`, `internal/kernel/actor`, `internal/kernel/txn`, `internal/kernel/errs`, `internal/kernel/ids`, `internal/kernel/clock`, `internal/authz`, `internal/chat`, `internal/messaging`, `internal/files`, `internal/search` (skeleton of M1-05), `internal/platform/pg` and `pgtest`, `internal/platform/httpserver`, `internal/platform/otel/metrics`, `internal/wire` (`configkeys_*.go`, `configchapters.go`, `configchecks.go`), `api/operations.yaml`, `architecture/modules.yaml`, `test/harness`, `test/canary`, `test/leak`, `tools/lint/sql_visibility.yaml`.
- Dependencies: the header names M3-13 (search vectors, `threads.search_language`, `search_tsquery`), M3-07 and M1-57; M1-58, M1-69 and M1-70 come through M3-07 (D-W-106); M3-13 implies M1-56 and its chain (M1-55, M1-53, M1-52, M1-28, M1-18, M1-13 and the rest of its closure). It builds on M3-07 (and through it M3-03 and M3-04: tables `files` and `message_attachments`, the attachment facts of 03 R-DM-43, the live `file.read` entry, the `Scanner` port), M1-57 (list representation of messages), M1-58 (deleting messages), M1-69 (canary scenario) and M1-70 (leak tests and SQL lint; through it M1-19 rate-limit classes, M1-65, M1-71 harness, and through M1-69 the M1-68 instrument registry). D-W-106 added them to the plan column, because this brief cannot be built without them; design note 0 checks that they are merged.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. Hits, excerpts and flags come only from channels the principal may read. Every query carries `channel_id = ANY($visible)` with the list that `Authorizer.Scope` builds per request; threads filter through `threads.channel_id`, files through the channel of the message they are attached to; the items of the delivered page run through `authorize` with `message.read`, `thread.read` or `file.read` again, and a denied item drops silently (14 4.3 items 1 to 3; 07 7.4 rules 1 to 3; 05 rules 1 and 2).
12. No total and no count per channel; the answer says only whether more pages exist (`next_cursor`) and whether the candidate limit was reached (`candidates_capped`) (14 4.3 item 4; 06 6 item 4; R-SU-05).
13. DM channels count only for participants, also for the owner (14 4.3 item 1; 07 2.8, SR6).
14. An agent that searches in a run (`acting_for`) searches on the intersection with the triggering person; candidate limit, `candidates_capped`, more pages and cursor come only from channels both may read; apps need the scope `messages:read` (14 4.3 item 5; 07 SR9 and 7.4 rule 1; D-W-19; 05 2.6).
15. Input `q` in web search syntax never produces a syntax error; more than `search.query_max_chars` gives `400 validation_failed` with `errors[]` on `q` and code `too_long`; no term left gives `400 validation_failed` with code `empty`; filters are parameters, the server never interprets `q` (14 4.1).
16. The query is an OR over the configurations of `search.languages` plus `simple`, built with the platform function `search_tsquery` (14 4.2, R-SU-03).
17. Filters of 14 4.4 (`space_id`, `channel_id`, `author_id`, `agent_id`, `after`, `before`, `has_file`, `thread_state`, `tag_id`) and `thread_id` of `openapi.yaml`; an unknown or invisible channel, space, thread or tag in a filter gives no hits and no error (14 4.4).
18. Ranking: the newest `search.max_candidates` hits after filters and rights are the candidates, `candidates_capped` reports the limit; `relevance` = `ts_rank(vector, query, search.rank_normalization)` divided by `1 + age / search.recency_scale`, ties to the higher id; `newest` by `created_at` then id (14 4.5, R-SU-04).
19. Highlighting only for the delivered page, input cut to `search.headline_input_max_chars`, options from `search.headline_max_fragments`, `search.headline_max_words`, `search.headline_min_words` and `ShortWord` as 14 4.6 fixes it; `StartSel` and `StopSel` are random hex strings of 128 bits per request; the result is split into `segments` `[{text, match}]`; HTML in the content stays text (14 4.6, R-SU-06).
20. Pages: opaque cursor bound to `q`, filters and sort; a cursor that does not fit the request gives `400 cursor_invalid`; page size `search.page_size_default`, at most `search.page_size_max` (14 4.7; 06 6).
21. Every search statement runs under `SET LOCAL statement_timeout` = `search.statement_timeout`; a timeout gives `503 unavailable` with `reason` `search_timeout` (14 7; 06 5).
22. Prefix search for the quick switcher over thread titles: words shorter than `search.prefix_min_chars` drop, the remaining words become prefix terms joined with AND; nothing left gives an empty answer; same rights filter as 14 4.3; newest activity first (14 4.9; `searchThreadTitles`).
23. `search` owns no tables; it reads messages, thread titles and files only through read operations of `messaging`, `chat` and `files` (`SearchMessages`, `SearchThreadTitles`, `SearchFiles`); the coordinator owns the port `SearchIndex`; shared parts (query types, segmentation, cursor positions) live in a stateless library (14 10; 03 1 item 12; R-SU-09; 02 4.1 rule 7 and 4.4).
24. Not searchable: run steps and live box contents, channel prompts, deleted messages, files that are not deliverable or never attached, drafts (14 1 and 2).
25. `search.query`: resource `instance`, right `auth`, rate-limit class `rl.search`, effect `read` (it keeps working in maintenance mode, ADR 0065), no audit entry for an allow; denials go to the audit as for every action (05 4.8, 2.4, 2.7).
26. Search terms never appear in logs, spans, metrics or audit details; the canary scenario carries a search term (SEC-LOG-4, SEC-T10).

## Scope

All paths under `services/server/` unless noted.

Create:

- `internal/kernel/searchkit/` (new kernel core package; standard library and `internal/kernel/ids` only): the stateless library of 14 10, used by the ports of `search`, `messaging`, `chat` and `files`. Files `doc.go`, `searchkit.go`, `segments.go`, tests. Contract (names may follow merged conventions; the semantics may not change):

```go
type Sort string // SortRelevance = "relevance", SortNewest = "newest"

// Position is the keyset of the last row of a page (14 4.7).
type Position struct {
    Score     float64   // sort relevance only
    CreatedAt time.Time // sort newest only
    ID        ids.ID
}

// Limits carries the configured values; the coordinator fills it from the search.* keys.
type Limits struct {
    MaxCandidates         int
    RankNormalization     int
    RecencyScale          time.Duration
    HeadlineInputMaxChars int
    HeadlineMaxFragments  int
    HeadlineMaxWords      int
    HeadlineMinWords      int
    PageSize              int // already capped by the coordinator
}

type Markers struct{ Start, Stop string } // random hex, 128 bits each (14 4.6)
func NewMarkers(r io.Reader) (Markers, error)

type Segment struct{ Text string; Match bool }
func Split(headline string, m Markers) []Segment

// ThreadFilter holds the filters that need chat's tables (14 4.4: agent_id, thread_state, tag_id).
type ThreadFilter struct {
    State           string   // "", "open" or "closed"
    TagIDs          []ids.ID // any of them
    OperatorAgentID *ids.ID
}

type Query struct {
    TSQuery    string       // tsquery text built by the search module (07 7.4 example)
    ChannelIDs []ids.ID     // visible channels after all channel filters; ascending; never empty
    ThreadIDs  []ids.ID     // nil: no restriction; otherwise only these threads
    Thread     ThreadFilter // applied by chat for type threads; resolved into ThreadIDs otherwise
    AuthorID   *ids.ID
    After      *time.Time   // created_at >= After
    Before     *time.Time   // created_at < Before
    HasFile    *bool        // messages only
    Sort       Sort
    RefTime    time.Time    // reference time of the first page (design note 7)
    From       *Position    // nil on the first page
    Limits     Limits
    Markers    Markers
}

type Hit struct {
    ID, ThreadID, ChannelID ids.ID
    MessageID               ids.ID // files only: the message the file is attached to
    Score                   float64
    CreatedAt               time.Time
    Segments                []Segment
}

type Page struct {
    Hits             []Hit
    Next             *Position // from the last row the statement returned, nil on the last page
    CandidatesCapped bool
}

type PrefixQuery struct { TSQuery string; ChannelIDs []ids.ID; Limit int }
type TitleHit struct { ThreadID, ChannelID ids.ID; Title string; LastActivityAt time.Time }

// ErrTimeout reports that a read operation hit search.statement_timeout.
var ErrTimeout = errors.New("searchkit: statement timeout")
```

- `internal/search` (port, extend the skeleton of M1-05): the inbound port `Search` with `Query(ctx, p actor.Principal, r Request) (Result, error)` and `ThreadTitles(ctx, p actor.Principal, r PrefixRequest) ([]searchkit.TitleHit, error)`; DTOs `Request` (type, `q`, sort, every filter of 14 4.4 plus `thread_id`, limit, decoded cursor position and reference time), `Result` (`Hits []ResultHit`, `Next *searchkit.Position`, `RefTime`, `CandidatesCapped`) and `ResultHit` (kind, ids, optional score, segments, `ThreadTitle`, `ChannelName`, `SpaceName`; empty strings mean absent). The port imports only `kernel`.
- `internal/search/domain`: validation and sanitising of `q` (design note 2), the table of filters per type (14 4.4 column "Types"), limit capping, the canonical cursor binding (design note 7).
- `internal/search/app`: `Coordinator` implementing `search.Search`; outbound ports `SearchIndex` (14 10) and `Metrics` (02 5 rule 5) with a no-op default; `Config` filled from the keys of design note 12.

```go
// SearchIndex is the coordinator's index port (14 10); the PostgreSQL adapter delegates to the
// read operations of messaging, chat and files; an external engine would replace only it.
type SearchIndex interface {
    BuildQuery(ctx context.Context, q string) (tsq string, ok bool, err error)  // ok=false: no term
    BuildPrefix(ctx context.Context, q string) (tsq string, ok bool, err error) // ok=false: empty answer
    Messages(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
    Threads(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
    Files(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
    TitlePrefix(ctx context.Context, q searchkit.PrefixQuery) ([]searchkit.TitleHit, error)
}
```

- `internal/search/adapters/postgres`: implements `SearchIndex`. `BuildQuery` runs `search_tsquery(<search.languages>, q)::text` and `BuildPrefix` the statement of 14 4.9 with `search.prefix_min_chars`, both in the ambient unit through `platform/pg` (no table is read); `Messages`, `Threads`, `Files` and `TitlePrefix` call the read operations of the module ports and resolve `Query.Thread` into `Query.ThreadIDs` through `chat` for messages and files (design note 5).
- `internal/search/adapters/httpapi`: the strict-server methods for `search` and `searchThreadTitles`; parameter mapping, cursor encoding with the M1-18 codec, rendering of hit objects through the module ports (design note 9), problem mapping.
- `internal/search/adapters/otelmetrics`: `bothy.search.duration` (design note 13).

Change:

- `internal/messaging` port: read interface for the coordinator with `SearchMessages(ctx, searchkit.Query) (searchkit.Page, error)` and `HitMessages(ctx, messageIDs, channelIDs []ids.ID) ([]Message, error)` (the list representation of M1-56 and M1-57, content cut at `message.list_content_max_bytes` with `content_truncated`, in input order, missing ids absent); implementation in `app`, SQL in `internal/messaging/adapters/postgres/queries/search.sql`.
- `internal/chat` port: read interface with `SearchThreadTitles(ctx, searchkit.Query) (searchkit.Page, error)`, `PrefixThreadTitles(ctx, searchkit.PrefixQuery) ([]searchkit.TitleHit, error)`, `ThreadsMatching(ctx, channelIDs []ids.ID, f searchkit.ThreadFilter) ([]ids.ID, error)`, `ChannelsInSpaces(ctx, spaceIDs []ids.ID) ([]ids.ID, error)`, `Labels(ctx, visibleSpaceIDs, channelIDs, threadIDs []ids.ID) (Labels, error)` (channel name, space name, thread title per id) and `HitThreads(ctx, threadIDs, channelIDs []ids.ID) ([]Thread, error)` (the `Thread` representation of M1-55); SQL in `internal/chat/adapters/postgres/queries/search.sql`.
- `internal/files` port: read interface with `SearchFiles(ctx, searchkit.Query) (searchkit.Page, error)` and `HitFiles(ctx, fileIDs, channelIDs []ids.ID) ([]File, error)` (the `File` representation of M3-03); SQL in `internal/files/adapters/postgres/queries/search.sql`.
- `internal/kernel/txn`: option `StatementTimeout(d time.Duration) Option` (outermost unit only, like `RightsChange`); `internal/platform/pg`: apply it with `SET LOCAL statement_timeout` instead of `database.statement_timeout`, and `IsStatementTimeout(ctx context.Context, err error) bool` (SQLSTATE `57014` while `ctx.Err() == nil`).
- `internal/wire`: `configkeys_search.go` (keys of design note 12) with the constant `chapter14` in `configchapters.go`; cross-checks in `configchecks.go`; build the coordinator, its index and metrics adapters; embed the search handler in the strict-server struct.
- `internal/platform/otel/metrics` (M1-68 registry): the instrument `bothy.search.duration` with its closed `bothy.outcome` set.
- `api/operations.yaml`: enable `search` and `searchThreadTitles`; rerun the route generator.
- `architecture/modules.yaml`: add `searchkit` to `kernel.core`; regenerate `.go-arch-lint.yml` with the archgen generator of M1-05.
- `test/canary/scenario.go`: both operations with canary search terms (design note 14).
- `test/leak` (`world.go`, `counters_test.go`): search surfaces in AZ-P5 (Tests); `test/harness`: additive helpers only if a test needs one.
- `docs/api/openapi.yaml`: descriptions of `search` and `searchThreadTitles` name the page size keys (`search.page_size_default`, `search.page_size_max`, 14 4.7); nothing else.
- `docs/spec/14-search.md`: 4.4 row `thread_id` and the rule for filters outside their types; 4.3 the page check of design note 8; 4.7 the reference time of design note 7; 10 the composition of thread filters (design note 5) and the read operations that render hit objects.
- `docs/spec/02-architecture.md`: 4.3 rows `messaging`, `chat`, `files` (read operations for `search`); 4.5 row `kernel/searchkit` and the option `StatementTimeout` in the row `kernel/txn`; 5 the list of `kernel-core` packages; 14.2 a row for search statements with `search.statement_timeout`. The archgen test of chapter 02 must stay green.
- `docs/spec/07-permissions.md`: AZ-M06 "Expected" reads "0 hits, no counters" (the search has no total, 14 4.3 item 4).
- `docs/observability/signals.md`: the `bothy.outcome` values of `bothy.search.duration` (design note 13).
- READMEs of `search`, `messaging`, `chat`, `files` and the package doc of `kernel/searchkit`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M3-14-<n>` (design note 16).

This ticket adds no migration and no new top-level Makefile target.

## Out of scope

- Search vectors and their generated columns, `messages.search_language`, `threads.search_language`, `threads.title_search`, the GIN indexes, the platform function `search_tsquery`, the migration lint rule against `GENERATED ALWAYS AS` without `STORED`, the language tests of 14 12 and the start check for `search.locale_map` (M3-13).
- File text extraction, `file_text_extracts` and its weights C and D in `SearchFiles` (M3-15, which extends `SearchFiles`).
- The search load test, `tools/bench-search`, `search.p95_target`, `search.prefix_p95_target`, `search.benchmark_messages` and `make perf` for search (M3-16).
- Upload, scan, attachment, `message_attachments` and the attachment facts that design note 15 needs (M3-03, M3-04, M3-07); deleting and purging files (M3-08).
- Reading the invocation mark from the request header and filling `acting_for` (M2-14); this ticket consumes `actor.Principal.ActingFor` as M1-28 defines it.
- Direct messages and `dm_participants` (V3-01).
- Client search, offline search with SQLite FTS5 and the purge of the local index after `access.changed` (M4-12, M4-23).
- An external search engine, `pg_trgm` or any other extension (14 9, ADR 0032).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-14-search-coordinator`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(search): ...`, `feat(messaging): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same rule applies to the merged code of M3-03, M3-04, M3-07 and M3-13, whose briefs were written together with this one.
- No migration: the search columns are M3-13, the attachment facts M3-07. If you find that a migration is needed, stop with `Verdict: BLOCKED <reason>`.
- No SQL of one module names a table of another module (03 1 item 12, 02 7); compose across modules through ports only.
- No search term, excerpt or content in logs, span attributes, metric attributes, audit details or problem bodies (SEC-LOG-4); IDs only in spans and logs, never in metrics.
- No real third-party service in tests; files reach their scan state through the scanner the M3-04 tests use (throwaway clamd container or its fake `Scanner`).

## Design notes

0. Start check. `make check` green on the fresh worktree, else `Verdict: BLOCKED base branch red`. Merged and present, else `Verdict: BLOCKED dependency <id> missing`: M3-13 (columns `messages.search_vector`, `threads.title_search`, `threads.search_language`, function `search_tsquery`), M3-07 (tables `files` and `message_attachments` with the copies `message_attachments.thread_id` and `message_attachments.channel_id`, the `chat.ThreadMoved` subscriber of `files` that moves `message_attachments.channel_id`, the column `messages.has_attachments` with its check, `file.read` no longer deferred in `internal/authz`), M1-57, M1-58, M1-69 (`test/canary`), M1-70 (`test/leak`, `tools/lint/sql_visibility.py`). The core contract defines the attachment facts of design note 15 (03 `message_attachments` and `messages`, R-DM-43, D-W-104); a copy, column or subscriber of them missing from the merged M3-07 code means `Verdict: BLOCKED dependency M3-07 missing`.

1. Operations.

    | operationId | Route | Action | Resource given to `authorize` | Class |
    |---|---|---|---|---|
    | `search` | `GET /api/v1/search` | `search.query` | instance | `rl.search` |
    | `searchThreadTitles` | `GET /api/v1/search/thread-titles` | `search.query` | instance | `rl.search` |

    The plan row names only the operation `search`; `searchThreadTitles` is the prefix search of the same row (14 11) and is enabled here.

2. Order of checks for `search` (each step only if the previous passed):
    1. Parameter rules without any lookup: `q` longer than `search.query_max_chars` (counted in runes of the raw value, an invalid byte counts as one) gives `400 validation_failed` with pointer `#query/q` and code `too_long`. A filter outside its types (14 4.4 column "Types"; `thread_id` applies to all types) gives `400 validation_failed` with pointer `#query/<name>` and code `not_applicable`. `limit` above `search.page_size_max` is capped, absent means `search.page_size_default` (06 6 item 3). Before building the query, invalid UTF-8 sequences and control characters in `q` become a space: they cannot be part of a term, and PostgreSQL rejects a NUL byte.
    2. Decode the cursor with the M1-18 codec against the canonical binding of design note 7; any mismatch gives `400 cursor_invalid`.
    3. `authorize(p, search.query, instance)`: unauthenticated, pending and missing app scope (`messages:read`) end here with the M1-28 mapping.
    4. Open one read unit (design note 11). `SearchIndex.BuildQuery`; no term gives `400 validation_failed` with pointer `#query/q` and code `empty`.
    5. `Authorizer.Scope(ctx, p, read)` with `read` = `message.read` for type `messages`, `thread.read` for `threads`, `file.read` for `files`. Scope already cuts with `ViewAs` and `ActingFor` (07 7.4 rule 1, SR9, D-W-19); the coordinator adds no rights logic (05 rule 1).
    6. Channel set: `Scope.ChannelIDs`, intersected with `channel_id` when given and with `chat.ChannelsInSpaces(space_id ∩ Scope.SpaceIDs)` when `space_id` is given. An empty set runs no query and answers an empty page with `candidates_capped` false; a filter on an unknown or invisible channel or space therefore answers exactly as an empty filter result (14 4.4).
    7. `SearchIndex.Messages`, `Threads` or `Files` with the `searchkit.Query` of the request.
    8. Page check (design note 8), labels (`chat.Labels`), close the unit, record the metric, return.

3. Rights. Never join `grants` or compute visibility outside `authz`; the only inputs are `Scope` and `authorize` (03 1 item 12, B7). Owners see DM channels only as participants (Scope does this). For an agent in a run, `Scope` returns only channels the triggering person may read, so the candidate set, `candidates_capped`, `next_cursor` and the reference time are the same as for that person (D-W-19). Apps get an empty `file.read` scope without `files:read`, so type `files` returns nothing for them; that is the expected result of 05 2.6, not an error.

4. Query building. The tsquery is built once per request by the search module, inside the unit: `SELECT search_tsquery($1::regconfig[], $2)::text` with `search.languages` (14 4.2), and for the prefix search the statement of 14 4.9 with `search.prefix_min_chars`. NULL means no term. Module queries take the text as `@tsq::tsquery` (07 7.4 example: "search expression, built by the search module"). If the merged M3-13 code did not register `search.languages`, register it here together with the start check of 14 3.1 and its test, and name that in the report.

5. Composition across modules (07 7.4 rule 6, 14 10). Filters on thread facts (`thread_state`, `tag_id`, `agent_id`) for types `messages` and `files` are resolved by the PostgreSQL index adapter through `chat.ThreadsMatching(channelIDs, filter)` in the same unit into `Query.ThreadIDs`, intersected with `thread_id` when given; an empty result runs no further query. For type `threads` chat applies `Query.Thread` itself. Several `tag_id` values match a thread that has at least one of them. `agent_id` matches `threads.operator_agent_id` (main threads never match). `author_id` means `messages.author_principal_id` or `files.uploaded_by`; `after` and `before` filter `created_at` of the hit's own row (messages, threads, files) in the half-open interval.

6. Read operations (one statement per call, the shape of 14 4.8):
    - Candidates: rows matching `@tsq`, `channel_id = ANY(@channel_ids::uuid[])` (the AZ-P12 lint needs exactly this form), the filters, `created_at <= @ref_time`, ordered `created_at DESC, id DESC`, `LIMIT` `search.max_candidates` plus one; `candidates_capped` is true when the extra row exists; the extra row is never ranked or returned. The limit applies to both sorts (R-SU-04).
    - `messages`: `deleted_at IS NULL`; vector `messages.search_vector`; headline over `left(content_md, search.headline_input_max_chars)` with `messages.search_language`.
    - `threads`: vector `threads.title_search`; headline over the title with `threads.search_language`; main threads never match (empty title).
    - `files`: deliverable files only (`upload_state = 'complete'`, `scan_status IN ('clean','skipped')`, `deleted_at IS NULL`, `purpose = 'attachment'`, 12 2) that are attached to a message (design note 15); vector `files.search_vector`; headline over the name followed by the transcript, cut to `search.headline_input_max_chars`, with `simple`.
    - Score for `relevance`: `ts_rank(vector, tsq, search.rank_normalization) / (1 + age / search.recency_scale)` with age measured from `@ref_time`, not `now()`; order `score DESC, id DESC`; `newest`: `created_at DESC, id DESC`. Keyset after `Query.From` (strictly after the position). Fetch the page size plus one to know whether a next page exists; `Page.Next` is the position of the last row of the page.
    - Headline only for the returned rows (14 4.6), options built from the configured values and the markers; split with `searchkit.Split`.
    - A statement timeout maps to `searchkit.ErrTimeout` through `pg.IsStatementTimeout`.
    - Every other new read carries the visibility predicate too, so the SQL lint of M1-70 needs no new exception: `chat.ThreadsMatching` and `PrefixThreadTitles` with `t.channel_id = ANY(...)`, `chat.ChannelsInSpaces` with `c.space_id = ANY(...)` (spaces already cut to `Scope.SpaceIDs`), `chat.Labels` with `c.space_id = ANY(@visible_space_ids)` for channels and `t.channel_id = ANY(...)` for threads, and every `Hit*` operation with `channel_id = ANY(...)` over the channels of the kept hits.

7. Cursor. The position (`Score` or `CreatedAt`, and `ID`) and the reference time of the first page (the request time from `kernel/clock`) travel in the cursor; later pages use the same reference time for the score and as the upper bound of the candidates, so a new message between two pages neither shifts the scores nor pushes a candidate out. `created_at` and the reference time must come from the same server clock (03 1 item 2); if the merged modules write `created_at` with the database default instead of `kernel/clock`, take the reference time from the database inside the read unit and name the difference in the report. Only `search` has a cursor; `searchThreadTitles` has none. Encode with the M1-18 codec (list name `search`) and a canonical binding from `internal/search/domain`: type, raw `q`, sort and every filter with defaults applied, id lists sorted and without duplicates, times in UTC with millisecond precision; `limit` is not bound. A cursor of another request, of another list, of another principal or a tampered one gives `400 cursor_invalid` (06 6 item 2, 14 4.7). The M1-18 codec binds the caller's principal, so the cursor strings of Alex and of an agent in Alex's run differ while their decoded positions and reference times are equal; "same cursor" in 14 12 and in the plan row means that.

8. Page check (14 4.3 item 3; 07 7.4 rule 3). After the read operation, in a new request scope `fctx := authorize.Filter(authorize.WithRequestScope(ctx))`, so the rights version is read again (M1-28 "Permission version"): call `Scope(fctx, p, read)` once, then for each hit:
    - its channel is not in the fresh scope: the rights changed between the first scope and the query; drop the hit silently (no metric);
    - otherwise `Authorize(fctx, p, read, resource)`; a denial is a mismatch between SQL filter and `authorize`: drop it, log at error level with only the action and the resource kind (no ids, no content), count `bothy.authz.errors` with `bothy.reason = filter_mismatch` the way M1-57 does.
    Resources carry only ids that never change (message: `ID`, `ThreadID`; thread: `ID`; file: `ID` and what M3-03 defines), never the channel copy, so a concurrent thread move cannot contradict the path that `authz` resolves (M1-28 "Resolution"). Dropping never moves `Page.Next`, so the next page continues after the last row the statement returned. Filter mode writes no denial into the audit (07 4.2).

9. Rendering in `adapters/httpapi`. `SearchHitPage.items` in page order; `kind` `message`, `thread` or `file`; `score` only for `relevance`; `segments` always; `thread_title` when the thread is not a main thread; `channel_name` and `space_name` from `chat.Labels` (absent for DM channels, which have no space); the object from the owning module: `message` from `messaging.HitMessages`, `thread` from `chat.HitThreads`, `file` from `files.HitFiles`, each called with the ids and channels of the kept hits; a hit whose object vanished in between is dropped. `next_cursor` absent on the last page; `candidates_capped` always present. No JSON `null` (03 1 item 13, ADR 0053). The mapping from the module DTO to the API type must render exactly what the owning module renders (test below).

10. `searchThreadTitles`. `q` longer than `search.query_max_chars` gives `400 validation_failed` (`#query/q`, `too_long`); `authorize(search.query, instance)`; in one read unit `BuildPrefix` (no term: `200` with empty `items`), `Scope(thread.read)`, `chat.PrefixThreadTitles` with `t.channel_id = ANY(@channel_ids::uuid[])`, `t.title_search @@ @tsq::tsquery`, order `last_activity_at DESC, id DESC`, limit `search.page_size_default` up to `search.page_size_max`; page check as in note 8; items `{thread_id, channel_id, title, last_activity_at}`. Example from 14 4.9: "rechnungs okt" finds "Rechnungslauf Oktober".

11. Unit of work and timeout. Each search request runs its database work in one `txn.UnitOfWork.Do(ctx, fn, txn.StatementTimeout(search.statement_timeout))` (READ COMMITTED as every unit, so the version read of note 8 sees a revocation committed meanwhile). Nothing is written, so no counter is locked (M1-16). `searchkit.ErrTimeout` becomes `503 unavailable` with `reason` `search_timeout` and `Retry-After` as the M1-18 problem writer sets it for kind `unavailable` (02 14.1). No retry of a timed out search.

12. Configuration (register in `internal/wire/configkeys_search.go` with chapter 14 and the kinds the M1-07 schema offers): `search.query_max_chars`, `search.max_candidates`, `search.rank_normalization`, `search.recency_scale`, `search.page_size_default`, `search.page_size_max`, `search.headline_input_max_chars`, `search.headline_max_fragments`, `search.headline_max_words`, `search.headline_min_words`, `search.prefix_min_chars`, `search.statement_timeout`. Start checks in `configchecks.go`: `search.page_size_default` not above `search.page_size_max`, and the headline values that `ts_headline` would reject (for example `search.headline_min_words` not below `search.headline_max_words`); an integration test proves each rejected combination with a real `ts_headline` call. `docs/operations/configuration.md` already lists the keys; the M1-08 check keeps schema and reference equal.

13. Telemetry. `bothy.search.duration` (signals.md 4.4) for `search` only, measured in the coordinator from the start of the use case to its end, with `bothy.outcome` from the closed set `ok`, `client_error` (any `4xx`), `timeout` (`search_timeout`), `server_error`; recorded through `internal/search/app/metrics.go` and `adapters/otelmetrics` (02 5 rule 5). The prefix search is measured by `http.server.request.duration` with its route. No span attribute or log field carries `q`, excerpts or names (SEC-LOG-4).

14. Canary (SEC-T10). Extend `test/canary/scenario.go`: `search` with each type and with `searchThreadTitles`, each with a canary value of class `content` in `q`; messages, thread titles and file names of the world carry the same canary so the search finds them; `TestSEC_T10_ScenarioCoversEveryEnabledOperation` lists both operations; the scan finds the canary in no log, span, metric, audit row or problem body.

15. Attachment facts (03 `message_attachments` and `messages`, R-DM-43, D-W-104). `files` may not read `messages` and `messaging` may not read `message_attachments` (03 1 item 12), so the core contract gives each module the fact in its own table: `message_attachments.thread_id` and `message_attachments.channel_id` copy the thread and channel of the message, written by `files` at attach, and `files` moves `channel_id` in the move transaction on `chat.ThreadMoved` like `messages`, `read_states` and `drafts`; `messages.has_attachments` is true exactly when the message was created with at least one attachment and is not deleted (set by `messaging` in the creating transaction, `false` after deletion, check `deleted_at IS NULL OR NOT has_attachments`). Build `SearchFiles` over `files` joined with `message_attachments` on `file_id`, with `channel_id = ANY(@channel_ids::uuid[])` on the attachment side (the AZ-P12 lint form) and `Query.ThreadIDs` against `message_attachments.thread_id`, never with `files.scope_channel_id` (it holds only before attaching, 05 2.2); build `has_file` in `SearchMessages` on `messages.has_attachments`; use the names of the merged M3-07 code. `has_file=true` keeps messages with at least one attachment, `has_file=false` messages without. Never fall back to filtering after the candidate limit (14 4.5 item 1: limit after filter and rights).

16. Rulings for `docs/DECISIONS.md` (one row each, `D-M3-14-<n>`, with reason):
    1. Shared library `internal/kernel/searchkit` in `kernel.core` (14 10 "internal library without state").
    2. The search module builds the tsquery once per request and passes its text to the read operations.
    3. Thread facts as filters are resolved through `chat.ThreadsMatching` into a thread id list in the same unit (07 7.4 rule 6).
    4. Page check with a fresh request scope and a fresh `Scope`: channel missing from it drops silently, otherwise a denial counts `filter_mismatch`; the next cursor follows the last returned row.
    5. Cursor carries the reference time of the first page; scores and the candidate bound use it.
    6. "Same cursor" for SR9 means the same decoded position and reference time; the strings differ because the M1-18 codec binds the principal.
    7. The candidate limit applies to both sorts.
    8. A filter outside its types gives `400 validation_failed` with code `not_applicable`; several `tag_id` match any of them; `has_file=false` means without attachments.
    9. Invalid UTF-8 and control characters in `q` become spaces; length counted in runes of the raw value.
    10. `searchThreadTitles` uses `search.page_size_default` and `search.page_size_max` and `search.query_max_chars`.
    11. Hit objects: list representation of messages, `Thread`, `File`; `thread_title` absent for main threads; names absent for DM channels.
    12. `txn.StatementTimeout` option for `search.statement_timeout`.
    13. `bothy.outcome` values of `bothy.search.duration`; measured for `search` only.
    14. Start checks of the search keys.

## Tests (write them first)

Unit:

- `internal/kernel/searchkit`: `TestSplit` (table: no marker gives one segment with `match` false; adjacent and repeated matches; content with `<b>`, `&lt;` and a 32-character hex string that is not the marker stays text unchanged; concatenated texts equal the headline without markers); `TestNewMarkers` (two hex strings of 128 bits, different from each other and between calls, error from the reader propagates); `FuzzSplit` (seed corpus in `go test`: never panics, texts concatenate to the input without markers).
- `internal/search/domain`: `TestValidateQ` (one rune over `search.query_max_chars` gives `too_long`, multi-byte runes count once, invalid bytes and control characters become spaces); `TestFilterApplicability` (every parameter of 14 4.4 times every type, `not_applicable` exactly where 14 4.4 says so, `thread_id` everywhere); `TestBinding` (permuted and duplicated id lists give the same binding; any change of type, `q`, sort or a filter changes it; `limit` does not); `TestLimitCap`.
- `internal/search/app` with fake `SearchIndex`, fake `Authorizer` and fake chat port: `TestCoordinator_EmptyChannelSetRunsNoQuery`; `TestCoordinator_SpaceAndChannelFilterIntersect`; `TestCoordinator_PageCheckRaceDropsSilently` (fresh scope without the hit's channel: item gone, no metric, no log); `TestCoordinator_PageCheckMismatchCounts` (fresh scope with the channel, `Authorize` denies: item gone, `filter_mismatch` plus one, log without ids); `TestCoordinator_NextFollowsLastRow`; `TestCoordinator_TimeoutIs503SearchTimeout`; `TestCoordinator_MetricOutcomes`; `TestCoordinator_EmptyQueryIs400Empty`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain and the M1-71 harness with `BuildMatrixWorld`; where a test compares two answers, it freezes the harness clock after its test data exists; G8 validates every response against `openapi.yaml`):

- `TestAZ_M06_SearchShowsNothingFromMarketing` (AZ-M06): three messages with "campaign" in Marketing/Kampagnen by Sam; Alex searches "campaign": `200`, zero items, no `next_cursor`, `candidates_capped` false, no member that counts anything; Sam gets the three.
- `TestSearch_TwoPrincipalsOnlyOwnChannels`: the same term in Support/Allgemein and Marketing/Kampagnen; Alex gets only Support hits, Sam only Marketing hits; neither response names the other channel, its name or its space anywhere; no counts.
- `TestSearch_OwnerNoForeignDMHits`: DMs arrive with V3-01, so a test fixture inserts one channel of kind `dm` with its main thread and a message containing the term through the test database pool (comment why); the owner, who is no participant, gets no hit from it with any type and filter, also with `channel_id` set to it, and the response equals the one for a random channel id.
- `TestSearch_RevokeThenSearch`: after the owner revokes G-1, Alex gets no hit from Support.
- `TestSearch_RevokeBetweenScopeAndQuery`: a test decorator around the real `Authorizer` revokes G-1 (committed in its own unit) after the first `Scope` returned; the read operation still returns Support rows; the response contains none of them; `bothy.authz.errors` with `filter_mismatch` is unchanged.
- `TestSearch_SR9_AgentInRunEqualsAlex` (SR9, AZ-P11 for search): the owner grants Lotse `read` on Engineering; Engineering/Projekt-A holds more matching messages than `search.max_candidates` (set low through the configuration), Support/Allgemein a few. Through the `search.Search` port: Alex; Lotse with `ActingFor{UserID: Alex, RunID, ThreadID: a Support thread, JTI}`; Lotse without `ActingFor`. Alex and Lotse in Alex's run get identical items and scores, `candidates_capped` false, `next_cursor` present or absent alike and, decoded, the same position and reference time; Lotse without the run gets `candidates_capped` true (proves the setup would leak without the cut).
- `TestSearch_ViewAsAlexEqualsAlex`: the owner with `ViewAs` Alex gets Alex's answer.
- `TestSearch_CursorWithOtherRequestIs400`: a `next_cursor` of a first page reused with another `q`, another filter, another sort, another type, by another principal, or with one byte changed: each `400 cursor_invalid`; with another `limit` it works.
- `TestSearch_PagesAreStable`: three pages with `relevance` and with `newest`: no item twice, none missing, order as 14 4.5; a new matching message created between page one and two does not appear on page two and moves nothing.
- `TestSearch_FilterInvisibleChannelEqualsUnknown` (14 12): `channel_id` and `space_id` of an invisible and of a random id give the same status, headers and body after normalising `Correlation-Id` and `Date`.
- `TestSearch_Filters`: each filter of 14 4.4 and `thread_id` per type with a world where exactly the expected hits match; a filter outside its type gives `400 validation_failed` with `not_applicable`; `tag_id` with two tags matches either.
- `TestSearch_NotSearchable` (14 12, DP-T01 search part): a deleted message (M1-58), a file `pending`, `infected`, `error`, `failed`, a file never attached and a file of a deleted message give no hit in any type; a draft containing the term gives none either.
- `TestSearch_SegmentsNoHTML` (14 12): content with `<b>`, `</b>` and a hex string looking like a marker gives segments whose texts appear in the content in order, no added markup, matches only on the term.
- `TestSearch_RankingAndCap`: equal texts of different age rank newer first; equal score ranks the higher id first; with more matches than `search.max_candidates` only the newest are reachable and `candidates_capped` is true.
- `TestSearch_TypesThreadsAndFiles`: type `threads` finds a forum post by its title; type `files` finds an attached clean file by name and by transcript; hits carry `thread_title`, `channel_name`, `space_name`.
- `TestSearch_HitObjectsEqualModuleRepresentation`: `message` of a hit equals the list representation of that message from `listThreadMessages`, `thread` equals `getThread`, `file` equals `getFile`.
- `TestSearchFiles_FollowsThreadMove`: after `moveThread`, the attached file of a message in the moved thread is found by readers of the target and not by readers of the source only.
- `TestSearchThreadTitles`: "rechnungs okt" finds "Rechnungslauf Oktober"; words below `search.prefix_min_chars` drop; only short words give `200` with no items; order by `last_activity_at`; invisible channels never appear; too long `q` gives `too_long`.
- `TestSearch_LimitCapped`: `limit` above `search.page_size_max` returns at most that many.
- `TestUnitOfWork_StatementTimeoutOption` (`internal/platform/pg`): inside a unit with the option `SHOW statement_timeout` shows the given value; `pg_sleep` beyond it makes `IsStatementTimeout` true; a cancelled context makes it false.
- `TestSearchReads_MapStatementTimeout` (one per module read operation): a query layer that returns a `*pgconn.PgError` with SQLSTATE `57014` while the context is alive gives `searchkit.ErrTimeout`; a cancelled context gives the context error.
- `TestSearchHTTP_TimeoutIs503` (`internal/search/adapters/httpapi` with the real coordinator and a `SearchIndex` that returns `searchkit.ErrTimeout`): `503 unavailable`, `reason` `search_timeout`, with `Retry-After`, and `bothy.search.duration` records `timeout`.
- `TestSearchConfig_StartChecks`: each inconsistent combination of note 12 stops the start with the key names.
- `TestSearch_LintClean` (AZ-P12): the SQL visibility lint of M1-70 passes on the tree with the new queries and no new exception.

Fuzz:

- `FuzzSearchQuery` (`internal/search/app`, real database, seed corpus in `go test`: punctuation only, quotes, `-`, `or`, `:*`, `&|!()`, backslashes, NUL, invalid UTF-8, very long input): the result is a page or `400 validation_failed` with pointer `#query/q` and code `empty` or `too_long`, never anything else (14 12).
- `TestSearch_HTTPNeverServerError`: the same seeds URL-encoded through HTTP for `search` and `searchThreadTitles`: status `200` or `400`, never `5xx`.

Property (extend M1-70):

- `TestAZ_P5_ListsAndCountersMatchAuthorize` and `TestAZ_P5_NoiseInvariance` (AZ-P5 for search): the world adds terms to messages, thread titles and attached files in visible and invisible channels; surfaces `search` (each type) and `searchThreadTitles`; property 1: every hit's thread and channel is visible by the truth; property 2: noise with the same terms only in channels P cannot read changes no item, no `candidates_capped` and not the presence of `next_cursor` (`search.max_candidates` set low so that the noise would flip the flag without the rights filter).

Canary:

- `TestSEC_T10_*` of M1-69 with the extended scenario (design note 14).

Contract: the Schemathesis run of M1-71 (G7, part of `make check`) covers both operations once they are enabled.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/search/... ./internal/kernel/... ./internal/messaging/... ./internal/chat/... ./internal/files/... ./internal/platform/pg/... ./internal/wire/... ./test/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make register-check`, `make config-doc-check`
- `make check`

## Acceptance criteria

- [ ] `search` and `searchThreadTitles` are enabled and registered; AZ-P1 passes.
- [ ] AZ-M06: Alex gets no hit for "campaign" from Marketing and no count of any kind.
- [ ] Two principals with the same query get hits only from their own channels.
- [ ] The owner gets no hit from DMs he does not take part in.
- [ ] A revocation between building the channel set and the query is filtered by the page check.
- [ ] An agent in Alex's run gets the same answer as Alex for `search.query`, with the same `candidates_capped` and the same cursor position (14 4.3, SR9).
- [ ] A cursor reused with another request gives `400 cursor_invalid`.
- [ ] Fuzzing never yields `500`; `q` errors are only `validation_failed` with `empty` or `too_long`.
- [ ] The canary scenario contains a search term, and the canary appears in no output.
- [ ] Filters, ranking, highlighting as segments without HTML, pages, the candidate limit and the statement timeout behave as 14 4.4 to 4.7 and 7; deleted, not deliverable and never attached content gives no hit.
- [ ] The prefix search for the quick switcher behaves as 14 4.9 with the same rights filter.
- [ ] `search` owns no tables; all reads go through the read operations of `messaging`, `chat` and `files`; the SQL lint passes without a new exception.
- [ ] AZ-P5 covers the search surfaces and passes.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `search`, `messaging`, `chat` and `files` updated (ports, read operations for the coordinator, the composition of note 5, the page check of note 8, the residual time side channel RK-10); package doc of `kernel/searchkit`.
- Rulings of note 16 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`), which also checks T-AZ-02 for the search and the time side channel of the search (PLAN 9 "Red team"); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
