# M3-16: Search load test

Status: ready | Depends on: M1-72, M3-14 | Milestone: M3

## Goal

After this ticket `make perf` measures the two performance targets of the search on a recorded host profile, prints value and threshold of each and fails when one is missed: `search.p95_target`, the p95 of the server time of `GET /api/v1/search` (`search`, type `messages`) on a corpus of exactly `search.benchmark_messages` messages, and `search.prefix_p95_target`, the p95 of `GET /api/v1/search/thread-titles` (`searchThreadTitles`) on the same world. The corpus is synthetic and deterministic, as 14 "Performance targets and load test" demands: German and English texts with Zipf-distributed word frequencies, several spaces and channels, people who each see only a part of the channels, and queries with one to three words, phrases and negation. The measurement runs against a server process with the default configuration on a durable database, and it counts only when the path behaved correctly while it was measured: every answer `200`, every hit from a channel the searcher may read, every query built from a visible message finds something. The same run measures what the specification leaves to the load test: the duration of a rewrite of `messages.search_vector` with its GIN index per million messages, which the upgrade guide turns into a maintenance window (14 "Index maintenance", RK-38), the share of the search index in the database size (16 "Capacity and resources"), and the time difference between a term that occurs only in channels the searcher cannot read and a term that occurs nowhere (the residual risk of 14 "Permission filter", RK-10). A make target `bench-search` runs the same test by hand. This is the search part of M3 exit criterion 3, the "Load test" row of 14 "Tests" and the confirmation "Load test" of ADR 0032; the target stops being unmeasured.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket" (item 6: `make perf` in every ticket that touches a performance target, no lowered threshold); section 4 "Closing a milestone" item 3; section 5 "Test strategy" row "Load, endurance, chaos"; section 9 "M3 Platform services" (Goal, Exit criteria 3 and 4, Red team "timing side channel of the search", Risks "a change of the search vectors rewrites the table") with the rows M3-13 and M3-14 (what is measured), M3-15 (file text, not measured here) and M3-16 (this ticket); section 7 rows M1-01, M1-16, M1-71 and M1-72; section 8 row M2-36 (the sibling load test of the agent path); section 19 P-07, P-09, P-10, P-11 and P-26. No ticket of the plan depends on M3-16; its result is the search part of M3 exit criterion 3.
- Dependencies: the header names M1-72 (the perf frame, D-W-108) and M3-14. Implied through it and therefore not in the header: M3-13 (search vectors, `threads.search_language`, `search_tsquery`, `kernel/searchlang`), M3-07 and the M1 tickets below them, among them M1-01 (the `perf` frame target and `PERF_TIMEOUT`), M1-07 (configuration defaults), M1-08 (`tools/configdoc` with the rules D4 and D6 and `tools/configdoc/planned.yaml`), M1-12 (`pgtest`), M1-16 (`perf/event_commit`, the first `perf/internal/report`, `pgtest.Durable()`), M1-31 (`updateMyProfile`, `users.locale`), M1-37 (`createUser` with `local_account`, `redeemEnrollmentCode`), M1-48 (`createGrant`), M1-53, M1-55, M1-56 and M1-57 (channels, threads, messages, history), M1-71 (`test/harness`).
- `docs/spec/14-search.md`: 1 "Goals and limits"; 2 "What is searchable"; 3.1 "Language per object"; 3.2 "Expressions"; 3.3 "Limits of language processing"; 4.1 "Input"; 4.2 "Query construction"; 4.3 "Permission filter" (items 1 to 4 and the paragraph "Residual risk"); 4.5 "Ranking and sorting" (the candidate limit); 4.7 "Paging"; 4.8 "Example: messages"; 4.9 "Prefix search for the quick switcher"; 7 "Performance targets and load test"; 8 "Index maintenance" ("Changing the expression"); 9 "Limit and outlook"; 10 "Module and port"; 11 "Endpoints"; 12 "Tests" (row "Load test"); 13 R-SU-04; 15 "Configuration values of this chapter".
- `docs/spec/02-architecture.md`: 6 "Code layout" (`perf/`, `tools/`, component `testsupport`); 7.1 "Module register as the source" (`exclude: [tools]`, `testsupport`); 17 "Performance targets"; 22 "Configuration values of this chapter" (the paragraph on `perf.*` with `Place` `Load test`, the row `perf.reference_profile`).
- `docs/spec/03-data-model.md` (core contract, read only): 1 "Principles" items 1 (UUIDv7 from the application), 2 (one server clock) and 12 (only the owning module writes a table; design note 6 says why the corpus fixture of a throwaway database is no module); tables `threads` and `messages` (columns, checks, unique keys, indexes, the import rule for `unread_floor_seq`); 5 "Search vectors".
- `docs/spec/05-actions.md` (core contract, read only): 3 "Rate-limit classes" (`rl.search` per principal); 4.8 row `search.query`.
- `docs/spec/07-permissions.md`: 7.4 "Query pattern" rules 1, 3 and 5.
- `docs/spec/16-operations.md`: 13 "Capacity and resources" ("indexes (the load test measures their share)").
- `docs/spec/23-quality.md`: 2 (the paragraph on `make perf`), 3 (coverage leaves out `perf/**`), R-QA-11.
- `docs/api/openapi.yaml`: operations `search`, `searchThreadTitles`, `createSpace`, `createChannel`, `createUser`, `redeemEnrollmentCode`, `updateMyProfile`, `createGrant`, `getThread`, `listThreadMessages`, `createMessage`, `createThread`; schemas `SearchHitPage`, `SearchHit`, `ThreadTitleHitList`, `Message`, `Thread`, `ThreadCreate`, `Problem` (`x-problem-codes`).
- ADRs 0032 "Full-text search in PostgreSQL" (Consequences, Confirmation "Load test in the release workflow"), 0063 "Performance gate for the event log" (points 2 to 5, Confirmation, More Information), 0050 (module register).
- `docs/operations/configuration.md`: 4 subsections of chapters 02 and 14 and the keys of design note 17; `docs/operations/upgrade.md`: 1 item 1 (release notes name the duration of migrations on large tables) and 4 (rules M3, M4, M6).
- `docs/RISKS.md`: RK-10 (time side channel of the search), RK-38 (rewrite of `messages`). `docs/security/threat-model.md`: T-AZ-02 ("response time"), T-DS-01 (its test column names the load tests). `docs/DECISIONS.md`: D-L4, D-W-100, D-W-106 (M2-36 got M1-72 for the same frame); section 10 for the rows of this ticket. No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- Briefs: `docs/briefs/M1-72-performance-gate.md` (the perf frame this ticket extends: `perf/internal/report`, `.cache/perf/results.json`, profile modes, durability check, the seeding rule, `harness.StartProcess`, `Options.DurableDB`, the open-loop scheduler of `load_messages`), `M1-71-contract-tests.md` (`test/harness`: `Start`, `Options`, `NewUser`, `AdminDB`), `M1-16-event-log.md` (origin of the report, `pgtest.Durable()`), `M1-01-server-gates.md` (`perf` target, `PERF_TIMEOUT`, the Makefile conventions for server targets), `M1-08-config-reference.md` (`Place` column, D4, D6, `planned.yaml`), `M3-13-search-vectors.md` (the generated columns, the GIN indexes and their migration, `search_tsquery`, `kernel/searchlang` with `New` and `Map.For`, the rewrite note in `migrations/README.md`), `M3-14-search-coordinator.md` (the coordinator, the read operations and their SQL, `search.statement_timeout`, the cursor). `M2-36-agent-path-perf.md` is the sibling load test (named constants, `-p 1`, the move of the scheduler to `perf/internal/load`); it is not a dependency.
- Code on the base branch (merged names win, D-W-100): the root `Makefile` (target `perf`, variable `PERF_TIMEOUT`, helpers `skip`, `GO_ON`, `GO_PREREQS`); `services/server/perf/` (`event_commit`, `load_messages`, `internal/report`, `README.md`); `services/server/test/harness`; `internal/platform/pg/pgtest`; `internal/platform/config`; `internal/kernel/searchlang`; `internal/kernel/ids`; `migrations/` (the search columns and GIN indexes of M3-13); `internal/search`; `internal/messaging/adapters/postgres/queries/search.sql`; `internal/chat/adapters/postgres/queries/search.sql`; `tools/configdoc/planned.yaml`.

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

Note on rule 9: the parameters of the load (counts, shares, durations of design note 3) are named constants of a test, not limits of the product; their values stand in `perf/internal/searchbench/constants.go` and `perf/README.md`, and every product limit they depend on is read by its key name.

Product requirements implemented here:

11. `make perf` measures the p95 of the server time of `GET /api/v1/search` with type `messages` against `search.p95_target` on a corpus of `search.benchmark_messages` messages, and the p95 of `GET /api/v1/search/thread-titles` against `search.prefix_p95_target`, and fails when one is missed (14 7, PLAN row M3-16, M3 exit criterion 3, ADR 0032 Confirmation).
12. The corpus follows 14 7: synthetic German and English texts, word frequency after a Zipf distribution, several spaces and channels, a principal sees a part of the channels; the queries have one to three words, phrases and negation.
13. `search.p95_target`, `search.prefix_p95_target` and `search.benchmark_messages` are values of the load test, not runtime configuration: like `perf.*` in 02 22 they have no environment name, are not in the configuration schema of the server, and `make perf` reads them from the table of 14 15 (D-L4, ADR 0063); no value appears as a literal in code or docs.
14. Measured only on databases with `fsync` and `synchronous_commit` on; every result names the profile mode of `perf.reference_profile`; only `match` and `emulated` can end green, `below` never does (ADR 0063 points 2 and 3).
15. `make perf` runs at the milestone close, in the release and in every ticket that touches a performance target, never in `make check` or the pre-push gate (P-07; PLAN 3 item 6; 14 7 "not on every PR").
16. The measured server runs with the default configuration; only the seeding server raises `ratelimit.*`, and every searcher of the measurement stays within `rl.search` (R-QA-11, 05 3).
17. Rights before everything also under load: every hit and every title hit comes from a channel the searcher may read, and a term that occurs only in channels the searcher cannot read gives the same answer as a term that occurs nowhere (14 4.3, T-AZ-02).
18. Every search statement runs under `search.statement_timeout` (14 7); a `503 unavailable` with `reason` `search_timeout` during the measurement is a failure, not a slow sample.
19. No threshold is lowered and no default of a key named in this brief is changed to pass a target (PLAN 3 item 6, ADR 0063 point 5). A miss on the reference profile is a release blocker (14 7) and leads to the path of 14 9: first optimise query and indexes, a different engine only with a new ADR.
20. The load test measures the duration of a rewrite of a search vector expression on its corpus, which the upgrade guide turns into the maintenance window of a release (14 8, RK-38), and the share of the search index in the size of the database (16 13); no fixed duration or size goes into documentation.

## Scope

All paths under `services/server/` unless they start with `docs/`, `tools/configdoc/` or `tools/ci/` or name the root `Makefile` or a root file. Names of M1-72's perf frame below are the expected ones; where the merged code names them differently, follow the code (Boundaries).

Create:

- `perf/search/` (build tag `perf`, the one tag of M1-16 and M1-72): `search_test.go` with `TestPerf_Search` and its phases `messages`, `prefix`, `probe` and `rewrite` (design notes 7 to 13), and a package comment naming both targets. The package clause follows the convention M1-72 chose for `load_messages`.
- `perf/internal/searchbench/` (untagged, unit tested in `make test` without a container; the database parts of `loader.go` and `rewrite.go` are tested by the small-corpus tests in `perf/internal/searchworld`):
    - `constants.go`: the named test parameters of design note 3.
    - `vocab.go`: function words, generated stems with inflections, the Zipf samplers, the probe terms (design note 5).
    - `corpus.go`: the deterministic generator of threads and messages by index (design note 5).
    - `plan.go`: the `Plan` derived from configuration defaults, the values of 14 15 and the constants (design notes 3, 4 and 7).
    - `queries.go`: search queries, prefix queries and probe queries per searcher (design notes 8, 9 and 11).
    - `loader.go`: the bulk loader of threads and messages and the consistency queries (design note 6).
    - `rewrite.go`: the rewrite and size measurement (design note 12).
    - `samples.go`: samples per phase, window filter, per-class statistics (design notes 7 and 13).
    - `checks.go`: the response checks of design note 10.
    - `doc.go`: package documentation.
- `perf/internal/searchworld/` (untagged): `Seed` builds spaces, channels, people, locales and grants through the public API (design note 4) and returns the handles; `seed_test.go` holds the small-corpus integration tests of "Tests", which need the container runtime like every `pgtest` test and run in `make test`.
- `perf/internal/load/` only if M1-72 kept its open-loop scheduler inside `perf/load_messages` and no other ticket moved it yet: move it here without a change of behaviour so that `load_messages` and `perf/search` use one scheduler; M1-72's tests keep their names and stay green. Never write a second scheduler.

Change:

- `perf/internal/report` (M1-16, M1-72): the threshold loader reads every row with `Place` `Load test` of the tables "Configuration values of this chapter" of 02 and 14 (design note 2); extend its tests; add the exit-logic test of "Tests". No other new mechanism.
- `perf/README.md`: a section `search` (what is measured, the measuring point, world and corpus, load shape, phases, parameters with their values, failure conditions, how to read `extra`, how to run `make bench-search`).
- Root `Makefile`: the new target `bench-search` (design note 1) with a `## ` help text and an entry in `.PHONY`; add `-p 1` to the `go test` call of `perf` if it does not serialize the packages yet (design note 14).
- `tools/ci/tests/test_make_server_gates.py` (M1-01): `bench-search` in the skip test and absent from `gates` and `check` (Tests).
- `docs/spec/14-search.md`: 7, 8, 12 and 15 as design note 16 says (new text in English; existing text stays as it is).
- `docs/spec/02-architecture.md`: 6 the line of `perf/` (thresholds also from 14 15); 17 one sentence after the table (design note 16).
- `docs/spec/23-quality.md` 2: the sentence on `make perf` names the targets of 02 and 14.
- `docs/operations/configuration.md`: regenerate with `make config-reference` (the three rows lose their environment name and get `Place` `Load test`).
- `tools/configdoc/planned.yaml`: remove or narrow every rule that matched only the three keys of requirement 13, so rule D6 finds no stale rule; keep rules that still match a key with `Place` `Server`.
- `docs/operations/upgrade.md`: one new paragraph in English in section 4 on a release that changes a search vector expression (design note 12).
- `docs/adr/0063-performance-gate-for-the-event-log.md`: "Confirmation" gains a bullet for M3-16 (both search targets, durable database, profile modes, failure on a miss); "More Information" names the targets of 14 7; the decision stays unchanged. `docs/adr/0032-full-text-search-in-postgresql.md`: the Confirmation bullet "Load test" names `perf/search` and `search.prefix_p95_target`. Then `make adr-index`.
- `docs/quality/gates.md`: `bench-search` in the section on `make perf` (by hand, not in `make check`).
- `docs/RISKS.md`: the column "Mitigation" of RK-38 names the measurement per million messages and the upgrade guide, that of RK-10 names the probe phase of `perf/search` as the measurement of the residual risk, and the column "Reference" of RK-10 adds M3-16 (RK-38 names it already); no rating changes.
- `docs/DECISIONS.md` section 10: rows `D-M3-16-<n>` (design note 18).
- `services/server/CHANGELOG.md` and the root `CHANGELOG.md` (the make target) under "Unreleased".

If the base branch already carries a sentence of design note 16 (a planning change made after this brief), keep it and change nothing there.

This ticket creates no migration, no operation, no event, no metric and no runtime configuration key. It creates one new top-level make target, `bench-search`.

## Out of scope

- The search itself: vectors, `search_tsquery`, the language assignment and the migrations (M3-13); coordinator, read operations, ranking, highlighting and cursor (M3-14); file text extraction (M3-15). Only a fix of design note 15 may touch them.
- Types `threads` and `files` and the filters of 14 4.4: 14 7 names a target only for type `messages` and for the prefix search.
- The thresholds, `search.benchmark_messages` and `perf.reference_profile` themselves; the other targets of `make perf` (M1-16, M1-72, M2-36, M3-24).
- Running `make perf` inside the release workflow (release chain, M7-10) and the milestone-close run on the reference host (owner operation after the merge, PLAN 4 item 3).
- The capacity table of 16 13 (M7-18); this ticket records the index share only in `results.json`.
- The job "Reassigning the language" of 14 8 and any change of a vector expression.
- The offline search of the clients (M4-12, M4-23).
- Ratings in `docs/RISKS.md` and in the threat model.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-16-search-load-test`; one writer per worktree.
- Conventional Commits with scope (for example `test(perf): ...`, `docs(spec): ...`, `build(make): ...`, `perf(<module>): ...` for a fix of design note 15) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief and the fixes of design note 15, each with a test and listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1`; in-process listeners use port 0, and the server subprocess of `harness.StartProcess` listens on loopback ports picked free just before its start (M1-72). PostgreSQL runs only as a throwaway container through `pgtest`, with `pgtest.Durable()` for every measured database (P-09, P-10). Without a Docker-compatible runtime the tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d` or `t.TempDir()`, removed at the end.
- Never read or print secrets. API tokens and DPoP proofs never appear in logs, test output, `results.json` or failure messages. No network calls except package and image downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test. A missed target on a host in mode `below` is reported as not proven, never "fixed" by changing numbers.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M1-72, M3-13 and M3-14: follow their names and list each difference in the report.
- This ticket extends the perf frame of M1-72, a dependency since D-W-108 (D-W-106 added the same edge for the load test of the agent path). If the base branch lacks `perf/internal/report` with the profile modes and the results file, or `harness.StartProcess` with `Options.DurableDB`, stop with `Verdict: BLOCKED perf frame missing`; never rebuild that frame here, it is the scope of its own ticket.
- Raised rate limits only on the seeding server; the measured server runs with the default configuration of every key.
- Direct SQL writes only in `perf/internal/searchbench` (the bulk loader and the rewrite measurement of design notes 6 and 12), only through the admin pool of the throwaway container, and never while a request runs against a server on that database (`TestPerf_Search` loads between the seeding and the measured server and rewrites after the measured server stopped); no production package gains a bulk path, a test hook or a second write path.
- No real third-party service and no data from outside the repository: vocabulary and texts are generated (design note 5).

## Design notes

0. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red` and the failing gate. Confirm that the base has M3-14 (`search` and `searchThreadTitles` enabled in `api/operations.yaml`, the read operations of `messaging` and `chat`) and M3-13 (`messages.search_vector`, `threads.title_search`, `search_tsquery`, `kernel/searchlang`), else `Verdict: BLOCKED dependency M3-14 missing`; the perf frame is checked as Boundaries say. Run `make perf` once only at the end (Definition of done); if a target of another ticket fails in that run, run that package on the base commit and report both values, so that a host limit is not mistaken for a regression of this ticket.

1. **Where the test lives** (Ruling for docs/DECISIONS.md). The plan row names `tools/bench-search` and a Makefile target. 02 6 puts load tests under `services/server/perf/` with the build tag `perf` and the shared report, while `tools/` holds generators and gates that go-arch-lint skips (02 7.1); Go's `internal` rule keeps `perf/internal/report` out of reach of a package outside `perf/`, and a second results file or profile detection would contradict M1-72 (one results file, one table). So the test is `perf/search`, the library `perf/internal/searchbench`, the seeding `perf/internal/searchworld`, and the plan's tool is the make target `bench-search`, the entry point for runs by hand (14 7 "by hand"). The plan row names these paths since D-W-108.

    Recipe of `bench-search`, built like `perf` (M1-01, M1-72): without `GO_ON` print the SKIP line with the `skip` macro; else `$(GO_PREREQS) perf`, then in `services/server` `go test -tags perf -count=1 -timeout $(PERF_TIMEOUT) -run '^TestPerf_Search$$' ./perf/search/` without `-race`, then print the result table the way `perf` prints it; exit non-zero when the test fails. Not in `gates`, `check` or the pre-push hook (P-07). Uses: by-hand runs, tuning after a missed target, and the window of a release that changes a vector expression (design note 12).

2. **Values of the load test** (Ruling for docs/DECISIONS.md: `search.p95_target`, `search.prefix_p95_target` and `search.benchmark_messages` get `Place` `Load test`). 14 15 has no column `Place`, so the generator of the reference gives the three the place `Server` and environment names, although no server process reads them and 02 22 says that targets of load tests have neither. Add the column `Place` to 14 15 (`Server` for every other row, `Load test` for the three) with the sentence of design note 16, run `make config-reference`, and clean `tools/configdoc/planned.yaml` (Scope). If the merged code registered one of the three in the configuration schema, remove it there (rule D4 of M1-08 would fail) and name it in the report.

    Loader in `perf/internal/report`: read every row with `Place` `Load test` from the tables "Configuration values of this chapter" of 02 and 14, each section found by its title, never by number; a name in both tables fails, a missing row fails, and so does a target or count whose value does not parse (`perf.reference_profile` keeps M1-72's own parser). Classification by the column "Meaning": a meaning that contains the word `target` is a target, a lower bound when it contains `(minimum)`, else an upper bound; every other `Load test` row is a parameter (`perf.reference_profile`, `perf.sustained_messages_per_s`, `search.benchmark_messages`). If M1-72's classifier knew only the exact meanings `target`, `target (minimum)` and `target (maximum)`, this rule keeps their results; `TestThresholdsFromChapterTable` against the real 02 proves it. `search.benchmark_messages` is a count.

3. **Parameters** (Ruling for docs/DECISIONS.md: 14 7 fixes languages, Zipf, spaces, channels and a partial view, nothing else). Named constants in `perf/internal/searchbench/constants.go`, documented with their values in `perf/README.md` and copied into `results.json`:

    - World: `Seed = 1`; `Spaces = 8` with `TextChannelsPerSpace = 4` and `ForumChannelsPerSpace = 4`; `ForumPostsPerChannel = 600`; `Authors = 32`; `Searchers = 24`, each with `read` on two of the spaces, so a searcher sees a quarter of the channels.
    - Corpus: `ContentWords = 50000` stems per language; `ZipfS = 1.07` (natural text is close to one, and `rand.NewZipf` needs a value above one); `FunctionWordShare = 0.4` (running text); `MessageWordsMin = 3`, `MessageWordsMax = 40`; `TitleWordsMin = 2`, `TitleWordsMax = 6`; `CorpusSpan = 365 * 24h` (a year, so the recency of `search.recency_scale` matters); `DeletedShare = 0.01`; `ProbeTermShare = 0.2`; `LoadBatch = 100000`; `ConsistencySamples = 50`.
    - Load: `BudgetShare = 0.5` (every searcher plans at most half of `rl.search`); `Warmup = 30s`, `Window = 3m`, `Drain = 30s`, `MinSamples = 1000`; `ProbeWarmup = 20`, `ProbeQueries = 200`.
    - Query mix: `ShareOneWord = 0.40`, `ShareTwoWords = 0.25`, `ShareThreeWords = 0.15`, `SharePhrase = 0.10`, `ShareNegation = 0.10`; bands `BandFrequentMax = 100`, `BandMediumMax = 5000` with `ShareFrequent = 0.2`, `ShareMedium = 0.5`, `ShareRare = 0.3`; `SharePrefixOneWord = 0.7`.

    The constants form `DefaultParams()`; `Plan` takes a `Params` value and the corpus size, so `TestPerf_Search` uses `DefaultParams()` with `search.benchmark_messages`, and the small-corpus tests use a small world with the same code.

4. **World through the API** (the seeding rule of M1-72: what involves rights is built through the public API by a seeding server with raised `ratelimit.*`, R-QA-11). `searchworld.Seed` creates, with the owner and step-up as the harness does:

    - `Spaces` spaces (`createSpace`), each with `TextChannelsPerSpace` text channels and `ForumChannelsPerSpace` forum channels (`createChannel`); a text channel gets its main thread with the channel (M1-53).
    - `Authors` people (`createUser` with `local_account`, `redeemEnrollmentCode` with one device, as `harness.NewUser`); the first half sets `locale` `de`, the second `en` through `updateMyProfile`; each gets a grant of level `write` on every space (`createGrant`).
    - `Searchers` people, `de` and `en` alternating; searcher `k` (from 0) gets a grant of level `read` on the spaces `k mod Spaces` and `(k + Spaces / 2) mod Spaces`, so every space is visible to the same number of searchers. The probe space of design note 11 is the last space.
    - Generated handles and display names (`author-01`, `searcher-01`, `space-01`, `text-01-1`, `forum-01-1`) and addresses at `example.com`.

    `Seed` takes the owner client and a function that creates a person client, so it runs against `harness.Start` (the small-corpus tests) and against the seeding process of M1-72 alike. It returns the ids and, per searcher, the set of visible channels computed from the grants it made: the test's truth, as `BuildMatrixWorld` keeps it.

5. **Corpus** (pure functions of `Seed` and an index, so every run on every machine builds the same corpus and the query generator can rebuild any message without storing it):

    - Vocabulary per language: a list of about forty function words written in this ticket (common German and English words such as `der`, `und`, `nicht`, `the`, `and`, `not`; facts of the language, no third-party list), and `ContentWords` stems generated from syllables, lower case, four to ten letters, a share of the German stems with U+00E4, U+00F6, U+00FC or U+00DF (a, o, u with diaeresis, sharp s; umlaut folding of 14 3.3). Each occurrence of a stem takes an inflection from a fixed list per language (German: none, `e`, `en`, `er`, `es`, `ung`, `ungen`, `lich`; English: none, `s`, `ed`, `ing`, `er`, `ly`), so the stemmed configurations do real work. The generator rejects a stem whose forms collide with a function word or with a form of another stem.
    - Words of a text: with probability `FunctionWordShare` a function word (Zipf over the function words), else a content stem drawn by `rand.NewZipf` with `ZipfS` over the `ContentWords` ranks, then inflected. The first word starts with a capital letter, the text ends with `.` or `?`. Messages have `MessageWordsMin` to `MessageWordsMax` words, titles `TitleWordsMin` to `TitleWordsMax` content words; titles stay within `thread.title_max_chars`, messages within `message.max_content_bytes`.
    - Threads: `ForumPostsPerChannel` posts per forum channel, each with a random author as creator and a title in the creator's language; the main thread of every text channel.
    - Messages: indexes `0` to `N - 1` with `N` = `search.benchmark_messages`. Message `i` gets `created_at = start + (i + 1) * CorpusSpan / N`, with `start` = load time minus `CorpusSpan` and the load time read once from the database clock (03 1 item 2), so time grows with the index. The first messages, one per post, open the posts; every later message picks a channel uniformly among all channels, then the main thread (text channel) or a post (forum channel) uniformly. The author is uniform among the authors, the language that of the author through `searchlang.Map.For` (the function `identity` uses since M3-13, built with `searchlang.New` from the defaults of `search.languages` and `search.locale_map`); with probability `DeletedShare` a message is deleted, never the first message of a post.
    - Probe: a message in a channel of the probe space carries the probe term at a random position with probability `ProbeTermShare`; the absent term occurs nowhere (design note 11). Both are generated stems outside the vocabulary, checked against every form.
    - Randomness: `math/rand/v2` with PCG seeded from (`Seed`, `i`) for message `i` and from (`Seed`, an offset plus the post number) for titles; no global generator.

6. **Bulk loader** (Ruling for docs/DECISIONS.md). Building `search.benchmark_messages` messages through `createMessage` would use most of the time limit of the `perf` target and measure the write path that `perf.message_create_p99` of M1-72 already holds; the targets of this ticket concern the read path. So rights, people and channels come through the API (design note 4), and the content rows (the `threads` of the posts, `messages`, the counters of all threads) come from a bulk loader into the throwaway database, a fixture like the DM fixture of M3-14's tests. 03 1 item 12 binds the modules of the product; the loader is test support of 02 6 and never writes a running instance. P-26 concerns the demo instance, not a throwaway perf database. That the rows equal what the product writes is checked against the product in every `make test` (small-corpus tests) and in every perf run (below).

    - Rows of `threads` (posts): `id` (UUIDv7 from `ids.NewGenerator` with a clock standing at the row's `created_at` and a seeded reader, one generator per id column because the generator never goes back in time, so ids follow time, 03 1 item 1), `channel_id`, `kind = 'thread'`, `title`, `title_source = 'manual'`, `state = 'open'`, `created_by`, `created_at` (that of its first message), `search_language` of the creator, and the final `last_message_seq`, `message_count` (without deleted messages), `unread_floor_seq = last_message_seq` (the import rule of 03 `threads`) and `last_activity_at` (newest message).
    - Rows of `messages`: `id` (UUIDv7 as above), `thread_id`, `channel_id`, `thread_seq` (gapless per thread in index order, from 1 in a post and after the current `last_message_seq` in a main thread), `author_principal_id`, `client_msg_id` (UUIDv7), `client_payload_sha256` (SHA-256 of the content), `origin = 'human'`, `kind = 'text'`, `content_md`, `created_at`, `search_language`; a deleted message has an empty `content_md`, `deleted_at` equal to `created_at`, `deleted_by` the author and `version = 2`. Every other column keeps its database default (`has_attachments` false); the generated columns compute the vectors.
    - Order: `COPY` of the post threads, then of the messages in batches of `LoadBatch` rows (one transaction each), then one `UPDATE` of the counters of the main threads, then `VACUUM (ANALYZE)` of `messages` and `threads`, so the GIN pending list is merged and the planner has statistics, as on an instance where autovacuum has run. Record each duration.
    - Checks after the load (each failure fails the run, naming counts and ids, never content): the count of `messages` equals `search.benchmark_messages`; one aggregate query shows per thread `last_message_seq = max(thread_seq)`, `message_count` equal to the non-deleted count and `last_activity_at = max(created_at)`; checks, foreign keys and unique keys are enforced by the database during `COPY`.
    - Checks through the product: before phase `messages`, for `ConsistencySamples` random threads `getThread` returns the loaded `title`, `last_message_seq`, `message_count` and `last_activity_at`, and `listThreadMessages` returns the loaded messages in `thread_seq` order with the loaded `content_md`. After phase `probe`, one `createMessage` per language into a loaded thread gets `thread_seq` equal to the previous `last_message_seq` plus one and the `search_language` (read through the admin pool) that the loader wrote for that author, and one `createThread` (forum post) by an author gets the `threads.search_language` the loader wrote for that author.
    - Columns follow the merged migrations. If a column the loader must fill is named differently, follow the migration and name it in the report; if the product fills a column with a value the loader cannot derive, stop with `Verdict: BLOCKED loader cannot build <column>`.

7. **Measured server, load and measuring point** (Ruling for docs/DECISIONS.md: 14 7 names the target "server duration ... without network" but no load and no measuring point).

    - Sequence of `TestPerf_Search`: start the seeding process (`harness.StartProcess` on a durable database, every `ratelimit.*` raised); `Seed`; stop it; load (design note 6); start the measured process on the same database with the default configuration and the profile limits of M1-72's mode; the product checks of design note 6; phases `messages`, `prefix` and `probe`; the writing product checks; stop the measured process; phase `rewrite` (design note 12); write the results.
    - Load: every searcher sends open loop, evenly spaced at `BudgetShare` of `ratelimit.rl.search.per_minute`, the searchers phase-shifted evenly over one interval; the plan never approaches `ratelimit.rl.search.burst`. A request goes out at its scheduled time even while earlier ones are open (the scheduler of `perf/internal/load` or the merged equivalent). Both endpoints share `rl.search`, so the phases run one after the other. No background writes: 14 7 defines the target on a corpus, and the write path has its own targets.
    - Measuring point: from the scheduled send time to the moment the whole response body has been read, on loopback, in the test process. The sample bounds the server time from above and avoids coordinated omission (M1-72's rule for `load_messages`); the client's DPoP proof lies inside the interval and only makes the bound looser.
    - Each phase: `Warmup` without samples, `Window` with samples, then up to `Drain` for open requests; a sample counts when its scheduled time lies in the window. The value is the nearest-rank p95 of the report.
    - The `Plan` checks before any work that the searchers' planned rate times `Window` reaches `MinSamples` with the default of `ratelimit.rl.search.per_minute`, else it fails naming that key and the constants.

8. **Query mix of phase `messages`** (Ruling for docs/DECISIONS.md). Every request is `GET /api/v1/search?q=...` with the defaults of `openapi.yaml` (type `messages`, sort `relevance`, no filter, no cursor, no `limit`), the search a person types. Per query the language is German or English with equal chance, and the class follows the shares of design note 3: one word, two words, three words, a phrase, or a word with a negated second word (`a -b`). Every word of the word classes and of the negation class is a content stem from a frequency band chosen by share (frequent: content ranks 1 to `BandFrequentMax`; medium: up to `BandMediumMax`; rare: the rest), uniform within the band, with a random inflection. Reason for the bands: frequent words hit the candidate limit `search.max_candidates`, which R-SU-04 says makes the cost independent of the number of hits, and their share lies well above the five percent a p95 may ignore, so the target cannot pass on rare words alone. A phrase is two adjacent words, at least one of them a content word, of a random message that is not deleted and lies in a channel the searcher may read, written `"a b"`; it must find a hit (design note 10). No query consists of function words only, none contains a probe term, and every query stays within `search.query_max_chars`.

9. **Prefix queries of phase `prefix`**: `GET /api/v1/search/thread-titles?q=...` without `limit`. A query takes a random post in a channel the searcher may read and one word of its title (share `SharePrefixOneWord`) or two, each cut to a random length between `search.prefix_min_chars` and its full length, joined by a space; it must find a title (design note 10). Main threads have an empty title and never match (M3-14).

10. **Failure conditions** (Ruling for docs/DECISIONS.md: a measurement counts only when the path behaved correctly while it was taken). `TestPerf_Search` fails, and `make perf` exits non-zero, when a p95 misses its threshold or on any of:

    - a status other than `200`, among them `429 rate_limited` and `503 unavailable` with `reason` `search_timeout`;
    - a body that does not decode strictly (unknown members rejected) into `SearchHitPage` or `ThreadTitleHitList`, or a hit of `kind` other than `message` in phase `messages`;
    - a hit whose `message.channel_id`, or a title hit whose `channel_id`, the searcher may not read by the truth of design note 4;
    - a phrase or prefix query without a hit;
    - a probe answer with an item, with `candidates_capped` true or with `next_cursor`;
    - a check of design note 6 or 12, a corpus whose count differs from `search.benchmark_messages`, fewer than `MinSamples` samples in a phase.

    Failure messages name counts, handles, ids, the phase and the query class and the query (synthetic words), never message content.

11. **Phase `probe`** (RK-10, T-AZ-02; Ruling for docs/DECISIONS.md: measured without a threshold, because 14 4.3 accepts the residual risk and names no bound). The searchers without a grant on the probe space send, at the rate of design note 7, queries for the probe term (frequent, but only in channels they cannot read) and for the absent term (nowhere), alternating: `ProbeWarmup` of each unmeasured, then until `ProbeQueries` of each are measured. Both answers must be equal: no item, `candidates_capped` false, no `next_cursor` (design note 10). `extra` records p50 and p95 of both terms and their differences, the measurement of the residual risk that RK-10 rates and the red team of M3 checks.

12. **Phase `rewrite` and sizes** (14 8, RK-38, 16 13; Ruling for docs/DECISIONS.md: measured per million messages, no fixed number in documentation, because the duration depends on host and corpus). After the measured process stopped, through the admin pool:

    - Sizes first: `pg_table_size` of `messages`, `pg_relation_size` of every index on `messages.search_vector` and on `threads.title_search`, the size of the database, the share of these indexes in it, each also per million messages.
    - Read the generation expression of `messages.search_vector` (`pg_get_expr` over `pg_attrdef` of the column), its nullability and the definition of every index on it (`pg_indexes.indexdef`); never copy the expression from a migration into code.
    - Time, in one transaction, `ALTER TABLE messages DROP COLUMN search_vector` followed by `ALTER TABLE messages ADD COLUMN search_vector tsvector ... GENERATED ALWAYS AS (<expression>) STORED` with the saved nullability: the drop changes only the catalog and removes the index, the add rewrites the table once, as a changed expression in a release does. Then time each saved index definition, built without `CONCURRENTLY` because a maintenance window has no writers. Record `maintenance_work_mem` with the times.
    - Check afterwards: expression and index definitions in the catalog equal the saved ones, and one fixed phrase of the corpus, queried as SQL with `search_tsquery` and `@@`, returns the same message ids before and after.
    - `extra` records the row count and `rewrite_seconds`, `rewrite_seconds_per_million`, `index_build_seconds` and `index_build_seconds_per_million`.

    `docs/operations/upgrade.md` (a new paragraph in English, section 4 after the table): a release that changes an expression of a search vector rewrites `messages` (14 8); that is the known exception to rule M3 and needs a maintenance window; the release notes name its expected duration under "Migrations" (section 1 item 1), computed from the values per million messages of the search load test (`make bench-search`, the values `rewrite_seconds_per_million` and `index_build_seconds_per_million` under `extra.search` in `.cache/perf/results.json`) times the number of messages of the instance in millions. No number in the guide.

13. **Results** through `perf/internal/report` (M1-72): rows `search.p95_target` and `search.prefix_p95_target` with `value` the p95 in the unit of the 14 row, `threshold` and `comparison` from 14 15, `pass`, `samples`, `test` (`perf/search`); profile and durability fields unchanged from M1-72. `extra` under one key `search`: the corpus counts (messages, deleted messages, posts, main threads, channels, spaces) next to `search.benchmark_messages`; searchers, visible share, planned and achieved rates; per phase p50, p99 and maximum; per query class and per frequency band count and p95; the share of answers with `candidates_capped`; the durations of seeding, `COPY` and `VACUUM (ANALYZE)`; the probe values (design note 11); sizes and rewrite values (design note 12); the PostgreSQL settings `shared_buffers`, `work_mem` and `maintenance_work_mem` as read; the defaults of `search.max_candidates`, `search.recency_scale` and `search.statement_timeout` in effect; all constants. PostgreSQL runs with the defaults of its image and the durability of `pgtest.Durable()`, without tuning (Ruling for docs/DECISIONS.md: 16 prescribes no tuning for `bothy-postgres`, so the shipped database runs with the image defaults too). The table line of each target shows value, threshold, pass and the corpus size.

14. **Serial packages and duration.** Two load tests on one host at the same time measure each other: if the `perf` recipe does not pass `-p 1` to `go test` yet, add it (Ruling for docs/DECISIONS.md, as in M2-36). Report the duration of each step of `TestPerf_Search` and of the whole `make perf`. If the time limit of the `perf` target (`PERF_TIMEOUT`, M1-01) no longer suffices for the package, raise it together with its named value in chapter 23 in the same change (the guard test of M1-01 keeps both equal) and record a row; never shrink the corpus or `Window`.

15. **A missed target** in mode `match` or `emulated` with a durable database: find the cause with `EXPLAIN (ANALYZE, BUFFERS)` of the read operations of M3-14 for the slowest classes and bands in `extra`, and fix it where it is owned (the SQL in `internal/messaging/adapters/postgres/queries/search.sql` or `internal/chat/adapters/postgres/queries/search.sql`, or the coordinator in `internal/search/app`), each fix with a test that holds the behaviour, the M3-14 tests green, each fix in the report. Never lower a threshold, never change a default of a key named here, never weaken the rights filter, the page check or the candidate limit. A new index or a changed vector expression changes 03 (core contract): stop with `Verdict: BLOCKED target missed` with the table, the plans and a proposed change (14 9 point 1, ADR 0063 point 5). In mode `below` report the values as not proven.

16. **Spec texts.** Every text this ticket adds to `docs/spec/`, `docs/operations/`, `docs/adr/`, `docs/RISKS.md`, `docs/quality/`, `docs/DECISIONS.md` and the READMEs is English (owner rule: everything added to the repository is English); existing text around it stays as it is, and the section titles keep their names. The values `Server` and `Load test` of the column `Place` are identifiers that the reference generator and M1-08's tooling read (02 22 and `tools/docs/config_reference.py` use them already), so they stay as they are.

    - 14 7, below the table, a new paragraph: "Measuring point: from the scheduled send time to the fully read response over loopback, an upper bound of the server time; the server runs with its default configuration; open load below `ratelimit.rl.search.per_minute` per person; requests with the defaults of `openapi.yaml`. The test `perf/search` runs in `make perf`; `make bench-search` runs it by hand. The same run measures the rewrite of `messages.search_vector` per million messages (section 8), the share of the search indexes in the database size and the time difference between a term that occurs only in channels the searcher cannot read and a term that occurs nowhere (residual risk of 4.3)."
    - 14 8 "Changing the expression": replace the clause "the upgrade guide ... gives the measured duration from the load test" with the English sentence "The load test measures the duration per million messages (section 7); the upgrade guide (`docs/operations/upgrade.md`) says how the release notes turn it into the maintenance window."
    - 14 12, row "Load test", column of the expectation: "`search.p95_target` at `search.benchmark_messages` and `search.prefix_p95_target` (`perf/search`)".
    - 14 15: the column `Place` (`Server` for every other row, `Load test` for the three keys), and above the table the sentence: "Column `Place`: `Server` = configuration reference of the server; `Load test` = threshold or parameter of the load test like `perf.*` in 02 section 22, without an environment name and not in the configuration schema; `make perf` reads them from this table."
    - 02 6, the line of `perf/`: append "(thresholds from section 22 and from 14 section 15)".
    - 02 17, after the table, a new sentence: "The performance targets of the search (`search.p95_target`, `search.prefix_p95_target`) stand in 14 section 7; `perf/search` holds them in the same `make perf`."
    - 23 2, the sentence on `make perf`: append "(with the performance targets of 02 and 14)".

17. **Configuration keys used**, read by name, no new key, no changed default: `perf.reference_profile`, `search.p95_target`, `search.prefix_p95_target`, `search.benchmark_messages` (through the report from 02 and 14), and through the defaults of `internal/platform/config`: `ratelimit.rl.search.per_minute`, `ratelimit.rl.search.burst`, `search.languages`, `search.locale_map`, `search.query_max_chars`, `search.prefix_min_chars`, `search.max_candidates`, `search.statement_timeout`, `search.recency_scale`, `thread.title_max_chars`, `message.max_content_bytes`. The seeding server raises the `ratelimit.*` keys through the harness `Config` map, never through code.

18. **Rulings for `docs/DECISIONS.md`** (section 10, one row `D-M3-16-<n>` each, with date, decision, reason and place): the location and the make target (design note 1); `Place` `Load test` for the three keys and the loader rule (design note 2); the parameters (design note 3); the bulk loader with its checks (design note 6); `VACUUM (ANALYZE)` before measuring (design note 6); the load and the measuring point (design note 7); the query mix and the bands (design note 8); the failure conditions (design note 10); the probe phase without threshold (design note 11); the rewrite measurement per million and no number in documentation (design note 12); PostgreSQL without tuning (design note 13); `-p 1` and a raised time limit, each only if made (design note 14).

## Tests (write them first)

Unit tests (untagged, in `make test`, no container needed):

- `perf/internal/report`:
    - `TestThresholdsFromChapterTable` (or its merged equivalent) also reads the real 14: `search.p95_target` and `search.prefix_p95_target` with unit `ms` and comparison `<=`, `search.benchmark_messages` as a parameter of kind count; the results for 02 are unchanged.
    - `TestLoadTestRowsAcrossChapters`: with fixture copies, a name in the tables of both 02 and 14 fails, a row with `Place` `Server` is never loaded, a missing `search.p95_target` fails, a meaning `target (minimum)` gives `>=`, `p95 target in the load test` gives `<=`, `corpus size in the load test` gives a parameter.
    - `TestReport_SearchTargetsFailAboveThreshold`: a fixture result for `search.p95_target` at the threshold plus one millisecond makes the exit logic fail and marks that row failed, a result exactly at the threshold passes; the same for `search.prefix_p95_target`; a passing value with `fsync` off in the profile still ends not green.
- `perf/internal/searchbench`:
    - `TestVocabulary_Deterministic`: the same `Seed` gives the same function words, stems, inflections and probe terms; another seed gives other stems.
    - `TestVocabulary_NoCollisions`: no form of a stem equals a function word or a form of another stem; the probe term and the absent term equal no form; every stem has four to ten letters; some German stems contain U+00E4, U+00F6, U+00FC or U+00DF (a, o, u with diaeresis, sharp s).
    - `TestZipf_Shape`: over a large sample the mean count per rank falls from rank 1 to ranks 2 to 10, 11 to 100 and 101 to 1000, and the share of rank 1 lies within a stated tolerance of the value `ZipfS` gives.
    - `TestCorpus_PureByIndex`: message `i` built twice is equal in every field; `created_at` grows strictly with `i`; the first messages open the posts in order; the share of deleted messages lies within a stated tolerance of `DeletedShare` and no first message is deleted; the probe term occurs only in channels of the probe space and the absent term nowhere; every text stays within `message.max_content_bytes`, every title within `thread.title_max_chars`; the language of every message is `searchlang.Map.For` of its author's locale.
    - `TestPlan_FromDefaults`: channel, post and searcher counts follow the constants; `N` equals `search.benchmark_messages` of the real 14; every searcher sees two spaces; every space is visible to the same number of searchers; exactly the searchers without the probe space are probe searchers; the planned rate per searcher is `BudgetShare` of `ratelimit.rl.search.per_minute`; the planned samples per `Window` reach `MinSamples`.
    - `TestPlan_FollowsKeys`: halving `ratelimit.rl.search.per_minute` halves the planned rate; a value too low for `MinSamples` gives an error naming `ratelimit.rl.search.per_minute`.
    - `TestQueries_Mix`: over ten thousand queries the class and band shares lie within a stated tolerance; every phrase is two adjacent words, at least one a content word, of a message that is not deleted and lies in a channel its searcher may read; negation has the form `a -b`; no query consists of function words only or contains a probe term; every query stays within `search.query_max_chars`; the same seed gives the same sequence.
    - `TestPrefixQueries`: every word has at least `search.prefix_min_chars` characters and is a prefix of a word of the title of a post in a channel the searcher may read; the one-word share lies within tolerance.
    - `TestLoaderRows` (the row builder, without database): `thread_seq` is gapless per thread, from 1 in a post and after the given `last_message_seq` in a main thread; the counters of every thread equal the aggregates of its messages and `unread_floor_seq` equals `last_message_seq`; a deleted row has an empty `content_md`, `deleted_at` equal to `created_at`, `deleted_by` the author and `version` 2; every id is a UUIDv7 whose time equals `created_at` to the millisecond, ascending with `created_at`; `client_payload_sha256` is the SHA-256 of the content.
    - `TestSamples_Window`: samples scheduled in `Warmup` or after `Window` are left out; the p95 is the nearest rank of the report; fewer than `MinSamples` samples fail; statistics per class and band add up to the totals.
    - `TestChecks`: each condition of design note 10 fails with a message that names phase, class and query and contains no message content; a correct answer passes.

Makefile tests (`tools/ci/tests/test_make_server_gates.py` of M1-01, pytest, in `make test`):

- `test_server_gates_skip_without_selected_go_module` (or its merged equivalent) also runs `bench-search` and expects its `SKIP` line and exit 0 without a selected Go module.
- `test_gates_contains_server_gates_but_not_perf` (or its merged equivalent) also asserts that `bench-search` is a prerequisite neither of `gates` nor of `check`.

Integration tests (untagged, in `make test`; PostgreSQL 18 through `pgtest`, the real server through `harness.Start` with raised `ratelimit.*`; a small plan of four spaces with one text and one forum channel each, four searchers, a few posts and two thousand messages, so every searcher sees half of the spaces; `perf/internal/searchworld/seed_test.go`):

- `TestSmallCorpus_SeedLoadAndProductAgree`: `Seed`, then the loader; the aggregate check passes; `getThread` and `listThreadMessages` agree with the loader for every thread; `createMessage` into a loaded post and into a main thread continues `thread_seq` and stores the `search_language` the loader wrote for that author; `createThread` by a `de` and by an `en` author stores the `threads.search_language` the loader wrote for them.
- `TestSmallCorpus_SearchFindsLoadedRows`: a German and an English phrase of loaded messages are found by `search` for a searcher who sees their channel and give zero items for a searcher who does not; `searchThreadTitles` with a prefix of a loaded title returns the post; for a searcher without the probe space the probe term and the absent term give equal answers.
- `TestSmallCorpus_RewriteKeepsDefinitions`: the rewrite of design note 12 on the small corpus leaves expression and index definitions in the catalog unchanged, the fixed phrase returns the same ids before and after, and all sizes are above zero.

Tagged test (build tag `perf`, run by `make perf` and `make bench-search`): `TestPerf_Search` as in design notes 7 to 13. It has no red run before its package exists; the red runs to keep for the report are those of the tests above.

Commands that must pass, from the repository root:

```sh
cd services/server && go test -race -count=1 ./perf/internal/...
cd services/server && go vet -tags perf ./perf/...
make config-reference
make config-doc-check
make adr-index
make docs
make bench-search
make perf
make check
```

## Acceptance criteria

- [ ] `make perf` runs `TestPerf_Search` and writes `search.p95_target` and `search.prefix_p95_target` with value, threshold, comparison, pass and samples, together with the corpus size and the profile mode, into `.cache/perf/results.json` and the table: the p95 against `search.p95_target` at `search.benchmark_messages`.
- [ ] `make perf` exits non-zero when one of the two is missed (`TestReport_SearchTargetsFailAboveThreshold`).
- [ ] The corpus holds exactly `search.benchmark_messages` messages in German and English with Zipf-distributed words over several spaces and channels; every searcher sees a part of the channels; the queries have one to three words, phrases and negation (14 7).
- [ ] Thresholds and corpus size come only from 14 15, where they now carry `Place` `Load test`; the configuration reference shows them without environment name, and no value is a literal in code or docs.
- [ ] The measured server runs with the default configuration on a durable database; no request is rate limited or times out; every hit comes from a channel the searcher may read; every phrase and prefix query finds something.
- [ ] The probe term and the absent term give equal answers, and their time difference is in `results.json`.
- [ ] Rewrite and index build per million messages and the share of the search indexes are in `results.json`; 14 8 and the upgrade guide say how the release notes use them, without a number.
- [ ] The small-corpus tests prove in `make test` that the loader writes what the product writes.
- [ ] `make bench-search` runs only the search load test and is not part of `make check`.
- [ ] On the build host both targets pass in mode `match` or `emulated`; in mode `below` the report lists them as not proven.
- [ ] 14, 02, 23, the configuration reference, ADR 0063, ADR 0032, the upgrade guide, `docs/RISKS.md`, the plan row and `docs/DECISIONS.md` carry this ticket.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `services/server/perf/README.md` updated (the search test, measuring point, world and corpus, load, phases, parameters with their values, failure conditions, extras, `make bench-search`).
- The spec texts of design note 16, the regenerated `docs/operations/configuration.md`, `tools/configdoc/planned.yaml`, the upgrade guide, ADR 0063 and ADR 0032 and `docs/RISKS.md` carry this ticket; Rulings of design note 18 recorded in `docs/DECISIONS.md` section 10 (one row each with reason).
- `services/server/CHANGELOG.md` and root `CHANGELOG.md` entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green and `make perf` run on the build host with its full table in the report (PLAN 3 item 6).
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: the measuring point cannot under-report (scheduled send time, whole body read); a leak, a wrong count, a timeout or a rate limit cannot end green; the corpus loader cannot diverge from the product unnoticed; the probe phase measures the time side channel of the search (PLAN 9 "Red team"); no threshold or key default was changed.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`, the full `make perf` table with profile mode, host data (logical CPUs, memory, `GOMAXPROCS`) and durability fields, the `extra` values of the search test (corpus, rates, per-class and per-band p95, `candidates_capped` share, probe values, sizes, rewrite and index build per million) and the duration of each step and of `make perf`.
3. Review and red team verdicts and the findings fixed; every fix of design note 15 with its test.
4. What is not proven (at least: the values on `perf.reference_profile` when the mode was neither `match` nor `emulated`), and each difference between this brief and the merged names of M1-72, M3-13 or M3-14.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
