# M3-13: Search vectors and search_tsquery

Status: ready | Depends on: M1-56 | Milestone: M3

## Goal

After this ticket every message and every thread carries a full-text vector that PostgreSQL keeps current without trigger or job: `messages.search_vector` and `threads.title_search` are `STORED` generated columns with a GIN index, each holding the word stems of the object's language and the unchanged words of `simple` with the weights of 14 "What is searchable" (message stems `B`, message words `D`, thread title both `A`). `messages.search_language` and `threads.search_language` are set once, when the row is created, from the author's or creator's locale through `search.locale_map` (14 "Language per object"), and the platform function `search_tsquery(langs regconfig[], q text)` builds the OR query over every language of `search.languages` plus `simple` exactly as 14 "Query construction" writes it. The language tests of 14 "Tests" pass on PostgreSQL 18, the server refuses to start with a language configuration that could never find a stem, and `make migrations-lint` stays green. Why: ADR 0032 keeps search inside PostgreSQL on one GIN predicate per object, a deleted message must lose its vector at once (DP-T01), and PostgreSQL 18 would create a virtual column without the keyword `STORED`; the search coordinator (M3-14), file text search (M3-15) and the load test (M3-16) all build on these columns and this function.

## Read first

- `docs/spec/14-search.md`: the header (examples ran on PostgreSQL 16; "A run against 18 is still pending as a build test": this ticket is that run), 1 "Goals and limits" (deleted messages: empty content, empty vector), 2 "What is searchable" (weights), 3.1 "Language per object", 3.2 "Expressions", 3.3 "Limits of language processing", 4.1 "Input" (a NULL query means code `empty`), 4.2 "Query construction", 4.8 "Example: messages" (how M3-14 will call the function), 8 "Index maintenance", 10 "Module and port", 12 "Tests", 13 "Decisions of this chapter" (R-SU-01 to R-SU-03), 14 "Alignment with the core contract" rows 1 and 2, 15 "Configuration values of this chapter" (`search.languages`, `search.locale_map`).
- `docs/spec/03-data-model.md`: 1 "Principles" item 12 (only the owning module writes a table; others read through its port), tables `threads` (`search_language`, `title_search`, GIN `(title_search)`), `messages` (`search_language`, `search_vector`, the check `deleted_at IS NULL OR content_md = ''`, GIN `(search_vector)`), `users` (`locale`, BCP 47), `instance_settings` (`locale.default`), 5 "Search vectors" (the expressions; `to_tsvector(regconfig, text)` is `IMMUTABLE`; "every migration writes `STORED` explicitly").
- `docs/spec/02-architecture.md`: 4.4 "Allowed dependencies" (rows `identity`, `chat`, `messaging`), 4.5 "Cross-cutting ports (`internal/kernel`)", 6 "Code layout" (`migrations/`, `sqlc.yaml`), 7.1 "Module register as the source", 13 "Configuration loading" (items 4, 5 and 7), 15.1 "Start" (steps 1 and 4), 17 "Performance targets" (`perf.message_create_p99`, `perf.startup_ready_max`).
- `docs/spec/16-operations.md`: 7.1 "Roles" (`bothy_owner` runs DDL, `bothy_app` does DML), 7.2 "Settings".
- `docs/spec/20-privacy.md` 10 "Tests" row DP-T01 ("search hits ... empty"); `docs/spec/19-security.md` SEC-LOG-4 (search terms never in a log).
- `docs/operations/configuration.md`: 2 "Naming rule and formats", 3 "Check", 4 subsection "14 Search", 6 "Instance settings" (`locale.default`). `docs/operations/upgrade.md` 4 "Rules for migrations (Expand/Contract)", rules M3, M4, M6, M7 and M12.
- ADRs: 0032 (Decision Outcome and Confirmation: language tests, compound test, migration lint), 0006, 0007, 0011, 0050.
- `docs/DECISIONS.md`: D-K-G13 and D-K-G14 (vectors and `threads.search_language` taken into the core contract), D-W-100, the format of section 10.
- `docs/RISKS.md` RK-38 (a change of the vector expressions rewrites `messages`); RK-10 and `docs/security/threat-model.md` T-AZ-02 and T-DS-01 concern the query side of M3-14 and are context only.
- `docs/OPEN-QUESTIONS.md`: no open question concerns this ticket.
- `docs/api/openapi.yaml`: nothing changes. `MessageCreate` and `ThreadCreate` have no language member and keep none. Tests use the operations `createMessage`, `createThread`, `closeThread`, `createChannel`, `updateMyProfile` and `putInstanceSetting`. `search` and `searchThreadTitles` belong to M3-14.
- `docs/PLAN.md`: 3 "Way of working per ticket"; 9 "M3 Platform services" (Goal, Scope, Exit criteria 1 and 2, Test strategy "fuzzing of the search input", Red team, Risks "a change of the search vectors rewrites the table"); rows M3-13 to M3-16; rows M1-53, M1-55, M1-56 and M1-58.
- Briefs: `M1-56-messaging-send.md` (Scope item 1: "`messages.search_language`, `messages.search_vector` and the GIN index: M3-13 adds them"; D1 flow of `createMessage`; D6 system line; D9 fixed statement count), `M1-55-chat-threads.md` (`chat.Threads.Create`, "Search columns `threads.search_language`, `title_search`: M3-13"), `M1-53-chat-channels.md` (`queries/threads_main.sql`, the same deferral), `M1-12-postgres-migrations.md` (goose, `pgtest`, D4 grants, D6 numbering), `M1-01-server-gates.md` (migration lint), `M1-07-config-schema-loader.md`, `M1-08-config-reference.md` (`make config-doc-check`, `tools/configdoc/planned.yaml`), `M1-11-lifecycle-health.md` (`wire.Serve` start order and exit codes), `M1-26-identity-principals.md` (`identity.Directory`, `UserView.Locale`), `M1-31-identity-people.md` (`users.locale` in canonical BCP 47 form), `M1-46-instance-settings.md` (`instance.Settings.LocaleDefault`), `M1-58-messaging-edit-delete.md` (row state of a deleted message; not a dependency).
- Dependencies implied by M1-56 and therefore not in the header: M1-55, M1-53, M1-46, M1-31, M1-26, M1-12, M1-11, M1-08, M1-07, M1-06 and M1-01 are all in its dependency closure.
- Code (paths under `services/server/` unless they start with `tools/`): `migrations/` with `migrations.sha256`, `tools/ci/migrations_lint.py` and `tools/ci/tests/test_migrations_lint.py` (rules `generated-not-stored` and `no-transaction`; the test `test_generated_without_stored_fails` already holds the 14 row "Migration lint"), `internal/platform/config` (`Kind`, `Values`, `Bind`, `Schema.AddCheck`), `internal/wire` (`config.go` with `configKeyGroups`, `configchapters.go`, `configchecks.go` with `configChecks`, `configkeys_*.go`, `Serve`), `internal/platform/pg` (`CheckSchema`) and `internal/platform/pg/pgtest`, `internal/identity`, `internal/instance`, `internal/messaging`, `internal/chat`, `architecture/modules.yaml`, `sqlc.yaml`.

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

11. `messages.search_language regconfig NOT NULL DEFAULT 'simple'` and `messages.search_vector tsvector NOT NULL GENERATED ALWAYS AS (setweight(to_tsvector(search_language, content_md), 'B') || setweight(to_tsvector('simple'::regconfig, content_md), 'D')) STORED` with a GIN index (03 `messages`, 03 5 "Search vectors", 14 3.2, R-SU-01).
12. `threads.search_language regconfig NOT NULL DEFAULT 'simple'` and `threads.title_search tsvector NOT NULL GENERATED ALWAYS AS (setweight(to_tsvector(search_language, title), 'A') || setweight(to_tsvector('simple'::regconfig, title), 'A')) STORED` with a GIN index (03 `threads`, 03 5, 14 3.2, D-K-G14).
13. Every generated column states `STORED`; PostgreSQL 18 would create a virtual column otherwise, and the migration lint rejects `GENERATED ALWAYS AS` without `STORED` (14 3.2, 03 5, ADR 0032 Confirmation, PLAN row M3-13).
14. The language is set when the row is created and never changed afterwards except by a reindex: a person as author gets `search.locale_map[users.locale]` (first language subtag), an agent, app, import or system author gets `search.locale_map[instance_settings['locale.default']]`, a locale without entry gets `simple`; threads follow the same rule through their creator; there is no automatic language detection (14 3.1, R-SU-02).
15. `search.languages` names every configuration a query is built with; the server does not start when a value of `search.locale_map` is missing there (14 3.1; 14 12 row "Start with a language in `search.locale_map` that is missing from `search.languages`").
16. `search_tsquery(langs regconfig[], q text) RETURNS tsquery LANGUAGE sql STABLE` exactly as 14 4.2: an OR over all given languages plus `simple`, empty sub-queries dropped, NULL means "no term"; created by a platform migration (14 4.1, 4.2, R-SU-03).
17. Vectors stay current through the generated columns alone; a deleted message (empty `content_md`) has an empty vector and is found by no query (03 5, 14 1 and 8, ADR 0032 Consequences, DP-T01 "search hits").
18. The documented limits stay visible in tests: no splitting of German compounds, umlaut folding by the German stemmer with the unchanged form kept by `simple`, identifiers found through their parts and as a phrase (14 3.3), negation per sub-query only "best effort" (14 4.2).

## Scope

All paths under `services/server/` unless they start with `docs/` or `tools/`. This ticket creates no Makefile target, no operation, no event, no job, no metric, no action and no audit entry.

Create:

- `migrations/<next number>_search_vectors.sql` (one transaction): the four columns of requirements 11 and 12, the function `search_tsquery` and its privileges (Design notes 2 and 5).
- `migrations/<next number + 1>_search_vector_indexes.sql` (`-- +goose NO TRANSACTION`): the two GIN indexes with `CREATE INDEX CONCURRENTLY` (Design note 3).
- `migrations/search_vectors_test.go` (package `migrations`, `pgtest`): catalog, function, privilege, index-use and fuzz tests of the section "Tests".
- `internal/kernel/searchlang` (`doc.go`, `searchlang.go`, tests): the validated language configuration of Design note 6; standard library only.
- `internal/platform/pg/textsearch.go` with tests: `CheckTextSearch` (Design note 9).
- `internal/wire/configkeys_search.go`: `searchConfigKeys()` with `search.languages` and `search.locale_map`.
- Integration tests of the language rule and the 14 examples (Design note 13 names the package).

Change:

- `internal/platform/config`: a new kind `KindStringMap` (name `map(string)`), its parser, the file form, `Values.StringMap(key) map[string]string` and `Bind` support for `map[string]string` fields (Design note 8); README section "Interface" and the kind documentation.
- `internal/wire`: `searchConfigKeys` in `configKeyGroups` (`config.go`); `chapter14 = "14-search.md"` in `configchapters.go`; the checks `search_languages` and `search_locale_map` in `configChecks` (`configchecks.go`); the two keys in the expected keys of `TestConfigSchemaBuilds`; in `Serve`, the `searchlang.Map` built from the loaded values, passed to `identity`, and the start step `pg.CheckTextSearch` directly after `pg.CheckSchema`.
- `internal/identity` port `Directory`: `SearchLanguage(ctx context.Context, id ids.ID) (string, error)` and `InstanceSearchLanguage(ctx context.Context) (string, error)` (Design note 7); implementation in `internal/identity/app` (reads `users.locale` through its repository and `locale.default` through `instance.Settings.LocaleDefault`); the fakes in `internal/identity/adapters/identitytest` (or the merged test helper package).
- `internal/messaging`: the insert in `adapters/postgres/queries/messages.sql` takes `search_language`; the use cases `CreateMessage` and `CreateThread` (and the system line of M1-56 D6) compute it (Design note 7); a static test over the module's query files.
- `internal/chat`: the inserts in `adapters/postgres/queries/threads.sql` (`Threads.Create`, M1-55) and `adapters/postgres/queries/threads_main.sql` (main thread at channel creation, M1-53) take `search_language`; the app layer computes it from the creator; a static test over the module's query files.
- `architecture/modules.yaml`: `searchlang` in `kernel.core`; regenerate `.go-arch-lint.yml` (`go generate ./tools/archgen`).
- `sqlc.yaml`: type overrides for `regconfig` and `tsvector` only if the regenerated code needs them; regenerate the sqlc code (`make codegen-check` must show no drift).
- `tools/configdoc/planned.yaml` (M1-08): remove a rule that names `search.languages` or `search.locale_map` exactly and would become stale; keep a prefix rule that still matches other planned `search.*` keys.
- `docs/spec/14-search.md`: header (the language tests now run on PostgreSQL 18); 3.1 (first language subtag, `simple` allowed as map value, the start check of Design note 9); 4.2 (privilege of the function; the results observed on PostgreSQL 18 if they differ from the PostgreSQL 16 example).
- `docs/spec/02-architecture.md`: 4.5 a row `kernel/searchlang`; 13 item 4 the new cross-check in "Cross conditions today"; 15.1 step 4 extended by the text search check.
- `docs/spec/16-operations.md` 7.2: a row for the character classification of the database (Design note 9).
- `docs/operations/configuration.md`: section 2 a row for `map(string)`; section 4 regenerated with `make config-reference`.
- READMEs: `migrations/README.md` (the platform function, the migration pair, the rewrite note), `internal/messaging/README.md`, `internal/chat/README.md`, `internal/identity/README.md` (new port methods), `internal/platform/config/README.md`, `internal/platform/pg/README.md` (start check); package documentation of `internal/kernel/searchlang`.
- `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` section 10 rows `D-M3-13-1` to `D-M3-13-10` (Design note 14).

## Out of scope

- The search coordinator `internal/search`, the read ports `SearchMessages`, `SearchThreadTitles` and `SearchFiles`, the operations `search` and `searchThreadTitles`, the rights filter, filters, ranking (`search.rank_normalization`, `search.recency_scale`), highlighting, cursor, prefix search, input checks (`search.query_max_chars`, codes `empty` and `too_long`), `search.statement_timeout`, `rl.search`, the canary search term and fuzzing at the HTTP level: M3-14. Hand-off notes for M3-14 are in Design note 12.
- `files.search_vector` (file name and transcript, `simple` only, 03 5): created with the `files` table by M3-03. `file_text_extracts` with its vector and the language of the instance for file text: M3-15 (it can use `identity.Directory.InstanceSearchLanguage` from this ticket).
- The search load test, `search.p95_target` at `search.benchmark_messages`, and the measured rewrite duration for the upgrade guide (RK-38): M3-16.
- The offline search of the client (14 6): M4-23.
- Deleting and editing messages through the API: M1-58. This ticket proves the vector side at row level (Design note 13).
- The language of imported messages (origin `import`): M8-03; the static tests of this ticket make its insert name `search_language`.
- The job of 14 8 "Reassigning the language" (new language for existing rows, for example after a change of `search.locale_map`): no ticket in the plan names it; reported as a plan gap. Rows that exist before this ticket keep `simple` (Design note 4).
- Any change to the expressions of 03 5 (core contract).

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M3-13-search-vectors`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(migrations): ...`, `feat(identity): ...`, `feat(config): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- Never connect to a database you did not start. The extra database with a `C` character classification that one test needs is created inside the `pgtest` container (`CREATE DATABASE ... TEMPLATE template0`) through `SuperConn` and dropped by the same test.
- Do not edit the merged migrations of M1-53, M1-55 and M1-56; the columns come only through the new migrations. Do not touch the migration lint (`tools/ci/migrations_lint.py`): it already holds the `STORED` rule, and this ticket only has to pass it.
- No backfill job and no data change of existing rows in a migration (upgrade.md rule M7).
- Do not add a pgx notice handler that logs notice texts: `websearch_to_tsquery` repeats the raw input in some notices, and search terms never reach a log (SEC-LOG-4).

## Design notes

1. Start check. `make check` is green on the fresh worktree, else `Verdict: BLOCKED base branch red`. M1-56 is merged (table `messages`, `createMessage` enabled), else `Verdict: BLOCKED dependency M1-56 missing`. Run `git tag -l 'server/v*'`: if it lists any tag, a server release with the tables `messages` and `threads` exists and the table rewrite of note 4 needs the owner's decision and a maintenance window; stop with `Verdict: BLOCKED server release exists; rewrite of messages needs a maintenance window (RK-38)`.

2. Migration `<next number>_search_vectors.sql`, numbered by M1-12 D6, one transaction, each deliberate choice commented in the file:

    ```sql
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
    ```

    - The column `search_language` comes first in its own statement because the generated column reads it; a constant default adds it without a rewrite (upgrade.md rule M3), the generated column then rewrites the table once (note 4).
    - The expressions are copied character for character from 03 5 and the function body from 14 4.2. If PostgreSQL 18 rejects an expression of 03, stop with `Verdict: BLOCKED <reason>`: 03 is the core contract. If it rejects the function body, correct 14 4.2 in the same change (14 is no core contract) and name the change in the report.
    - Privileges: the migrations run as `bothy_owner` (16 7.1), so the function belongs to it. A new function is executable by `PUBLIC` by default; following M1-12 D4 (least privilege, no default privileges) the migration revokes that and grants `EXECUTE` to `bothy_app` only. If the merged migrations already follow another written convention for functions (`grep -n FUNCTION migrations/*.sql`), follow it and report the difference. Tables need no new grant: column grants come with the table grants of M1-53 and M1-56.
    - The `Down` section drops the function and the four columns (generated columns first); goose files carry it although production never runs it (upgrade.md rule M12).

3. Migration `<next number + 1>_search_vector_indexes.sql`, first line `-- +goose NO TRANSACTION` (the lint's `no-transaction` rule, ADR 0007 rule 3, upgrade.md rule M4):

    ```sql
    -- +goose NO TRANSACTION
    -- +goose Up
    DROP INDEX CONCURRENTLY IF EXISTS messages_search_vector_idx;
    CREATE INDEX CONCURRENTLY messages_search_vector_idx ON messages USING gin (search_vector);
    DROP INDEX CONCURRENTLY IF EXISTS threads_title_search_idx;
    CREATE INDEX CONCURRENTLY threads_title_search_idx ON threads USING gin (title_search);
    -- +goose Down
    DROP INDEX CONCURRENTLY IF EXISTS threads_title_search_idx;
    DROP INDEX CONCURRENTLY IF EXISTS messages_search_vector_idx;
    ```

    The `DROP ... IF EXISTS` before each build removes an invalid index that an interrupted concurrent build leaves behind, so a second `bothy migrate up` succeeds. Use the index naming convention of the merged migrations if it differs (`grep -n 'CREATE INDEX' migrations/*.sql`). GIN with default settings (14 8). `migrate up` runs both files with `lock_timeout = migrate.lock_timeout` (M1-12 D5, upgrade.md rule M6).

4. Rewrite and existing rows. Adding a `STORED` generated column rewrites the table under an exclusive lock; upgrade.md rule M3 wants new columns without a rewrite, while 03 5 and 14 8 prescribe generated columns and accept the rewrite with a maintenance window (ADR 0032 Consequences, RK-38). The deviation is acceptable only because no server release exists (note 1); record it as a ruling and say in `migrations/README.md` that a later change of an expression is a release with a maintenance window whose duration M3-16 measures. Existing rows get `search_language = 'simple'` from the column default; there is no backfill in the migration (upgrade.md rule M7) and no reassignment job in this ticket (Out of scope). Before the first release no instance holds real data, so nothing is lost.

5. `search_tsquery` behaviour. The planning ran the function and the expressions on a throwaway PostgreSQL 16 with a UTF-8 character classification; the tests of this ticket repeat each line on PostgreSQL 18 (`pgtest`). If PostgreSQL 18 answers differently, keep the function as written, correct the example in 14 4.2 or 3.3 to the observed value and name the difference in the report; never change the function to match an example.
    - `search_tsquery('{german,english}', 'Rechnungen')` gives `'rechnung' | 'rechnungen' | 'rechnungen'` (14 4.2).
    - `search_tsquery('{german,english}', 'the')` gives `'the' | 'the'`: the English branch holds only a stop word and is dropped.
    - Input with only punctuation (`!!! ...`), the empty string, white space only and a NULL `q` give NULL (14 4.1: no term).
    - `search_tsquery('{}', 'Rechnungen')` gives `'rechnungen'` (only `simple`); a NULL array behaves the same.
    - `search_tsquery('{german,english}', '-z')` gives `!'z' | !'z' | !'z'` (see note 12).
    - The function turns each sub-query into text and parses the joined text again (`tq::text` to `::tsquery`). The text form of a `tsquery` quotes every lexeme and escapes quotes inside it, so the round trip is expected to be lossless; the fuzz test holds this for arbitrary input. A counterexample (an error or a changed query) breaks 14 4.1 ("every input is safe"): then correct the function and 14 4.2 together in this change, keep the input as a seed and report it; never skip such an input.
    - `websearch_to_tsquery` raises a NOTICE for stop-word-only and empty input; for white-space-only input the notice repeats the input. A NOTICE is no error; see the boundary on notice handlers.

6. Kernel package `internal/kernel/searchlang` (ruling): the validated pair `search.languages` and `search.locale_map`, used by `identity` (write side), `wire` (checks and start check) and later by M3-14 (query languages) and M3-15. Standard library only; it joins `kernel.core` in `architecture/modules.yaml` and the table of 02 4.5.

    ```go
    // Simple is the configuration that keeps words unchanged; every vector and every query has it.
    const Simple = "simple"

    // Map is the validated language configuration of 14 "Language per object". The zero value maps
    // every locale to Simple and has no languages.
    type Map struct{ /* unexported */ }

    // New validates search.languages and search.locale_map by the rules of design note 8.
    func New(languages []string, localeMap map[string]string) (Map, error)
    // For returns the configuration for a BCP 47 locale: the entry of the map for its first
    // language subtag (the part before the first "-", ASCII lower case), or Simple.
    func (m Map) For(locale string) string
    // Languages returns search.languages in configuration order, without Simple.
    func (m Map) Languages() []string
    // Configs returns every configuration name the server uses: Languages, the values of the
    // locale map and Simple, sorted and without duplicates.
    func (m Map) Configs() []string
    ```

7. Who writes the language (14 3.1, R-SU-02; ruling for the details):
    - `identity.Directory.SearchLanguage(ctx, id)`: for a principal of kind `user`, `Map.For(users.locale)`; for `agent` and `app`, `Map.For` of `instance.Settings.LocaleDefault`; unknown id gives the `kernel/errs` kind `not_found`. `InstanceSearchLanguage(ctx)` gives `Map.For` of `locale.default` without a principal. Both read inside the caller's unit of work (M1-26 convention: reads join the ambient transaction). `identity` may use `instance` (02 4.4); `messaging` and `chat` may use `identity`.
    - `messaging`: inside the unit of work of `CreateMessage` and `CreateThread` (M1-56 D1 step 5 and D8), before the insert, origin `human`, `agent` and `app` take `SearchLanguage(author)`; origin `system` (the line `thread.reopened` of M1-56 D6, whose author is a person) and `import` take `InstanceSearchLanguage`. When the actor kind already says the author is no person, call `InstanceSearchLanguage` directly and skip the principal read.
    - `chat`: `Threads.Create` takes `SearchLanguage(created_by)`; the main thread inserted at channel creation (M1-53) takes the same for the creator of the channel (14 3.1: "Threads get the same rule via the creator"; the main thread's title is empty, the rule still holds for every row).
    - The language never comes from a request, is not part of any API schema or event payload, and no later write changes it: an edit recomputes the vector with the stored language, a change of `users.locale` or of `locale.default` affects only new rows, moving a thread (M1-62) keeps it.
    - The value goes to SQL as text and is cast there (`sqlc.arg(search_language)::text::regconfig`), so pgx needs no codec for `regconfig`. No read query selects `search_vector` or `title_search` (replace `SELECT *`, `RETURNING *` or `sqlc.embed` on these tables by column lists where the regenerated code would carry the vectors).
    - Enforcement for later writers (M2 agent messages, M8-03 import, V3-01 DMs): a static test in `internal/messaging/adapters/postgres` and one in `internal/chat/adapters/postgres` fail when an `INSERT INTO messages` or `INSERT INTO threads` in the module's query files does not name `search_language`.

8. Configuration (14 15, 02 13; rulings for what 14 leaves open):
    - Keys in `internal/wire/configkeys_search.go`, chapter `chapter14`: `search.languages`, `config.KindStringList`, default exactly `["german","english"]`; `search.locale_map`, the new `config.KindStringMap`, default exactly `{"de":"german","en":"english"}` (the chapter cells; `make config-doc-check` compares them, M1-08 rule D5).
    - `KindStringMap` (`map(string)`): text form a JSON object whose values are strings (default, environment, a scalar in the file); in the file also a YAML mapping of plain scalars under the key (like lists, which take a YAML sequence there); duplicate keys, empty keys, non-string values and anything but an object are problems; canonical text is the JSON object with keys sorted. Parser messages never quote the input (M1-07 rule).
    - Check `search_languages` (keys `search.languages`): entries are unique, each an unqualified PostgreSQL identifier in lower case (letters, digits, `_`, not starting with a digit, at most the PostgreSQL identifier length), and none is `simple` (always added, 14 4.2).
    - Check `search_locale_map` (keys `search.languages`, `search.locale_map`): every key is a first language subtag in lower-case ASCII letters; every value is a valid configuration name and is listed in `search.languages` or equals `simple`; otherwise the problem names `search.locale_map` and the position of the entry in key order. This is the start error of 14 3.1: `bothy config check` reports it with exit `1`, `bothy serve` stops at step 1 of 02 15.1 with the configuration error code of `cmd/bothy` before any listener opens.
    - Both checks call `searchlang.New`; problem messages name keys and positions, never values (02 13 item 4).

9. Start check against the database (ruling; extends 02 15.1 step 4, documented in 16 7.2): in `wire.Serve` for every role, directly after `pg.CheckSchema`, `pg.CheckTextSearch(ctx, pool, m.Configs())`:
    - every name must be a text search configuration visible on the search path (`pg_catalog.pg_ts_config` with `pg_catalog.pg_ts_config_is_visible(oid)`); a missing one would fail every message insert and every search with a SQL error;
    - the database must fold the case of non-ASCII letters in text search: `to_tsvector('simple', 'A') = to_tsvector('simple', 'a')` must be true, where the two letters are U+00C4 (capital A with diaeresis) and U+00E4 (small a with diaeresis). Planning observed on PostgreSQL 16 that a database with the character classification `C` keeps `'Arger'` and `'Uber'` (first letters U+00C4 and U+00DC, capital A and U with diaeresis) unchanged, so "arger" (first letter U+00E4) never finds "Arger" (first letter U+00C4), while `en_US.UTF-8` gives `'arger'` and `'uber'` (first letters U+00E4 and U+00FC, small a and u with diaeresis). The official Debian-based PostgreSQL image, which `pgtest` pins and the Compose image builds on, initialises with `en_US.utf8`; `TestTextSearchCaseFolding` confirms it for `pgtest`. If PostgreSQL 18 folds the case even under `C`, keep the probe (it then passes everywhere), replace the `C` case of `TestCheckTextSearch` by a probe result injected through a test seam, and report the observation.
    - On failure the start aborts with exit `1` (M1-11 start failure) and no public listener; the message names the key and the entry position for a missing configuration, or the character classification of the database, never a value. Add a row to 16 7.2 ("Character classification of the database: UTF-8 capable, for example `en_US.utf8`; reason: upper and lower case in text search") and the check to 02 15.1 step 4.

10. Transactions and concurrency: the language is computed in the same unit of work as the insert, under `READ COMMITTED` (03 1 item 11); a concurrent change of the author's locale may or may not be seen, and both results satisfy 14 3.1 because the value is taken once at creation. No lock is added; the thread row lock and lock order of M1-55 and M1-56 stay unchanged. The generated columns are computed by PostgreSQL inside the insert or update, so a deleted message (M1-58 sets `content_md = ''`) has an empty vector in the same transaction (03 5).

11. Performance: the write path of `createMessage` gains one read (the author's `users` row by primary key, or the instance setting); M1-56 D9 (a fixed number of statements) still holds. `perf.message_create_p99` (02 17) is held by `perf/load_messages` of M1-72; if that test exists on the base branch, run `make perf` on the reference profile when it is available (PLAN 3 item 6), otherwise list it under "What is not proven". The start check adds two short queries (`perf.startup_ready_max`).

12. Hand-off to M3-14 (write it into `migrations/README.md` under the function): a NULL result means `400 validation_failed` with code `empty` on `q` (14 4.1); a query of only negations (`-z`) gives `!'z' | !'z' | !'z'`, which matches nearly every row and is bounded only by `search.max_candidates` and `search.statement_timeout` (reported as a spec gap); PostgreSQL `text` cannot hold a NUL byte and rejects invalid UTF-8, so `q` must be checked before it reaches SQL; call the function with the languages of `searchlang.Map.Languages()` cast as `$n::text[]::regconfig[]`.

13. Test placement: catalog, function and fuzz tests live in package `migrations` (as M1-12 placed `TestPrivilegesAfterAllMigrations`) and need no domain rows. Tests with messages and threads use the real modules through `internal/wire`, HTTP through the real chain and people from the identity test helpers, in `test/integration` (02 6) or next to the M1-56 integration tests if the merged layout keeps them there. Deleted and edited rows are produced with SQL as the owner role on a message created through `createMessage`, because the delete and edit path of M1-58 is not in this ticket's dependency closure.

14. Rulings for `docs/DECISIONS.md` section 10 (one row each with reason and place):
    - D-M3-13-1: the language of 14 3.1 is assigned in this ticket by `messaging` and `chat` through `identity.Directory.SearchLanguage` and `InstanceSearchLanguage`; the map key is the first language subtag in ASCII lower case; origin `system` and `import` use the instance language.
    - D-M3-13-2: main threads get the creator's language like every thread.
    - D-M3-13-3: the kernel package `kernel/searchlang` holds the validated language configuration.
    - D-M3-13-4: configuration kind `map(string)` for `search.locale_map` (JSON object; YAML mapping in the file).
    - D-M3-13-5: validation of `search.languages` and `search.locale_map` (note 8); `simple` may be a map value and is never listed in `search.languages`.
    - D-M3-13-6: the start check of note 9 (configurations exist, case folding works).
    - D-M3-13-7: columns and function in one transactional migration, GIN indexes concurrently in a second migration that first drops an invalid leftover.
    - D-M3-13-8: the table rewrite against upgrade.md rule M3 is accepted only while no `server/v*` tag exists; existing rows keep `simple`, no backfill.
    - D-M3-13-9: `EXECUTE` on `search_tsquery` only for `bothy_app`.
    - D-M3-13-10: the language goes to SQL as text with a cast; read queries never select the vectors.

## Tests (write them first)

Unit:

- `internal/kernel/searchlang`: `TestNew` with a table: the chapter defaults are valid; a map value missing from `search.languages` fails and the error names `search.locale_map` and the entry position but not the value; `simple` as map value is valid; `simple` in `search.languages` fails; a duplicate language fails; names with upper case, a dash, a leading digit, empty or longer than the PostgreSQL identifier length fail; map keys `de-AT`, `DE` and the empty key fail. `TestFor`: `de` and `de-AT` and `DE-at` give `german`, `en-US` gives `english`, `fr`, the empty string and `-` give `simple`. `TestConfigs`: sorted, unique, contains `simple` and every map value. `TestLanguages`: configuration order, without `simple`.
- `internal/platform/config`: `TestKindStringMap` (JSON object accepted and canonicalised with sorted keys; array, number value, nested object, duplicate key and empty key rejected; no message contains the input); `TestLoadStringMapFromFile` (YAML mapping of plain scalars accepted; nested mapping, sequence and alias rejected; a scalar with JSON text accepted); `TestLoadStringMapFromEnv`; `TestBindStringMap`; `TestValuesStringMapReturnsCopy`.
- `internal/wire`: `TestConfigSchemaBuilds` lists the two keys; `TestConfigKeysMatchChapterTables` and `TestDefaultsPassCrossChecks` pass with chapter 14; `TestSearchLanguagesCheck` and `TestSearchLocaleMapCheck` hold note 8 with one case per rule.
- `internal/identity/app`: `TestSearchLanguage` with fakes: a person with locale `de` gives `german`; a person with locale `en` and a map without `en` gives `simple`; an agent and an app give the mapping of `locale.default` (`en` gives `english`); an unknown id gives `not_found`; `InstanceSearchLanguage` follows a changed `locale.default`.
- Static: `TestMessageInsertsSetSearchLanguage` (`internal/messaging/adapters/postgres`) and `TestThreadInsertsSetSearchLanguage` (`internal/chat/adapters/postgres`) parse the module's `queries/*.sql`; every `INSERT INTO messages` or `INSERT INTO threads` names `search_language`; a fixture query without it makes the check fail.

Integration (PostgreSQL 18 via `pgtest`):

- `TestSearchColumnsOnPostgres18` (`migrations`): `current_setting('server_version_num')` belongs to major version 18; `pg_attribute.attgenerated = 's'` (stored, not virtual) and `attnotnull` for `messages.search_vector` and `threads.title_search`; `search_language` of both tables has type `regconfig`, is `NOT NULL` and defaults to `simple`; the indexes `messages_search_vector_idx` and `threads_title_search_idx` exist, use access method `gin` and are valid (`pg_index.indisvalid`). This holds the 14 row "Migration lint" against the real artifact.
- `TestSearchTsqueryExamples` (`migrations`, as `bothy_app`): every line of note 5 with its exact text result or NULL.
- `TestSearchTsqueryUsesIndex` (`migrations`): with `SET LOCAL enable_seqscan = off`, `EXPLAIN` of `SELECT id FROM messages WHERE search_vector @@ search_tsquery('{german,english}', 'Rechnungen')` and of the same on `threads.title_search` shows a bitmap index scan on the GIN index (14 4.2 "the GIN query uses the index").
- `TestSearchTsqueryPrivileges` (`migrations`): `has_function_privilege` is true for `bothy_app` and false for `public` and `bothy_maint`; the owner is `bothy_owner`.
- `FuzzSearchTsquery` (`migrations`, Go native fuzzing; `go test` runs the seed corpus, Test strategy "fuzzing of the search input"): seeds with quotes, backslashes, `&`, `|`, `!`, `<->`, `:*`, parentheses, `or`, `-`, leading `-`, Unicode white space, bidi marks, combining characters and long words; inputs that are invalid UTF-8 or contain NUL are skipped (PostgreSQL `text` cannot hold them; M3-14 rejects them before SQL). Property: the call never returns an error; the result is NULL or a `tsquery` with `numnode > 0` whose text form parses back to an equal value.
- `TestTextSearchCaseFolding` (`migrations`): in the `pgtest` database `to_tsvector('simple', 'Arger')` equals `'arger':1`, where the input starts with U+00C4 (capital A with diaeresis) and the lexeme with U+00E4 (small a with diaeresis).
- `TestCheckTextSearch` (`internal/platform/pg`): the defaults pass; a list with an unknown configuration name fails with a message that names its position but not the name; a database created in the container with `TEMPLATE template0 LC_COLLATE 'C' LC_CTYPE 'C'` fails the case-folding probe; the test drops that database.
- `TestServeAbortsOnUnknownTextSearchConfig` (`internal/wire` or `cmd/bothy`): `bothy serve` against `pgtest` with `BOTHY_SEARCH__LANGUAGES` holding a configuration the database does not have exits `1` after the schema check; no public listener was opened (no `public listener started` log line); the output names `search.languages`.
- `TestConfigCheckRejectsLocaleMapLanguageMissing` (`cmd/bothy`; 14 12 row "Start with a language in `search.locale_map` that is missing from `search.languages`: start error"): with `BOTHY_SEARCH__LOCALE_MAP={"de":"german","fr":"french"}` and the default languages, `bothy config check` exits `1` and `bothy serve` exits with the configuration error code before any listener; both outputs name `search.locale_map` and do not contain the configured value `french`.

Integration with the real modules (note 13; world: space Support with the text channel Allgemein and the forum channel Fragen; Alex with `users.locale` `de`, Sam with `en`, Robin with `de`; an agent from the M1 test helpers with an API token and the right to write in Allgemein; Alex is owner):

- `TestLanguageExamplesFromChapter14` (the language rows of 14 12 and the limits of 14 3.3 and 4.2): Alex posts "Rechnung", "Rechnungsadresse", "Fehler ERR_CONN_RESET beim Abruf", "verfugbar" with U+00FC (small u with diaeresis) as the fifth letter, "Rechnung offen" and "Arger mit dem Abruf" with U+00C4 (capital A with diaeresis) as the first letter; Sam posts "invoices are running". As `bothy_app`, `SELECT id FROM messages WHERE search_vector @@ search_tsquery($1::text[]::regconfig[], $2)` with the default `search.languages` asserts: "Rechnungen" finds "Rechnung" (German); "invoice run" finds "invoices are running" (English); `err_conn_reset` and the phrase `"err_conn_reset"` find the identifier message; "Rechnung" does not find "Rechnungsadresse" (compound limit, 14 3.3); "verfugbar" (fifth letter U+00FC) finds "verfugbar" (fifth letter U+00FC); "arger" (first letter U+00E4, small a with diaeresis) finds "Arger mit dem Abruf" (first letter U+00C4); "offen -Rechnungen" finds "Rechnung offen" (negation per sub-query, the documented limit of 14 4.2).
- `TestSearchVectorWeights`: `unnest(search_vector)` of Alex's "Rechnung offen" gives German stems with weight `B` (`off`, `rechnung`) and unchanged words with weight `D` (`offen`, `rechnung`); "verfugbar" (fifth letter U+00FC) holds `verfugbar` with `B` and the unchanged form with U+00FC with `D` (14 3.3: both forms); a forum post by Alex in Fragen titled "Arger mit Rechnungen" (first letter U+00C4) has only weight `A` in `title_search`, with `arg`, `mit`, `rechnung`, `rechnungen` and `arger` with U+00E4 as the first letter (the German stop word `mit` comes only from `simple`).
- `TestSearchLanguageAssignedAtCreation` (14 3.1): Alex's message gets `german`, Sam's `english`; the agent's message gets the mapping of `locale.default` (`german` with the default, `english` after Alex sets `locale.default` to `en` through `putInstanceSetting` with step-up, as the M1-46 tests do); after `closeThread` a message of Sam reopens the thread and the system line gets the instance language while Sam's message gets `english`; with `search.locale_map` set to `{"de":"german"}` Sam's next message gets `simple`; `createThread` by Sam in Fragen with title "Invoices are running" gives `threads.search_language = 'english'` and `title_search @@ search_tsquery('{german,english}', 'invoice')` is true; `createChannel` by Alex gives the new main thread `german`; after Alex changes `locale` to `en` through `updateMyProfile`, her earlier message keeps `german`; a body with a member `search_language` is rejected as an unknown member (M1-18) and changes nothing.
- `TestDeletedMessageHasEmptyVector` (DP-T01 part "search hits"): a message created through `createMessage` is brought into the row state of M1-58 with SQL as the owner role (`content_md = ''`, `deleted_at` and `deleted_by` set); afterwards `search_vector = ''::tsvector` and no query of the earlier content finds it. A second message updated with SQL to a new `content_md` is found by a word of the new text and no longer by a word that only the old text had (14 8: no trigger, no job).
- The existing tests of M1-53, M1-55 and M1-56 (channel creation, threads, `createMessage`, `createThread`, replay) stay green unchanged.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./migrations/... ./internal/kernel/searchlang/... ./internal/platform/config/... ./internal/platform/pg/... ./internal/identity/... ./internal/messaging/... ./internal/chat/... ./internal/wire/... ./cmd/bothy/... ./test/...`
- `cd tools && uv run --locked --quiet pytest ci/tests/test_migrations_lint.py`
- `make migrations-lint`, `make codegen-check`, `make arch`, `make register-check`, `make config-doc-check`
- `make check`

## Acceptance criteria

- [ ] The new migrations add `search_language` and the `STORED` generated vectors to `messages` and `threads` with the expressions of 03 5 and a valid GIN index each; `make migrations-lint` is green, and the catalog shows stored, not virtual, columns on PostgreSQL 18.
- [ ] The language tests of 14 "Tests" are green on PostgreSQL 18: "Rechnungen" finds "Rechnung", "invoice run" finds "invoices are running", `err_conn_reset` finds the identifier, "Rechnung" does not find "Rechnungsadresse", and a language in `search.locale_map` that is missing from `search.languages` stops the start.
- [ ] `search_tsquery` exists exactly as 14 4.2, gives NULL without a term, uses the GIN index, never fails on fuzzed valid input, and only `bothy_app` may execute it.
- [ ] Every new message and thread carries the language of 14 3.1, set once at creation and never from a request; the static tests hold this for every insert of `messages` and `threads`.
- [ ] A deleted message has an empty vector and is found by no query; an edited message is found by its new text only.
- [ ] The server does not start with an unknown text search configuration or a database without case folding of non-ASCII letters.
- [ ] Configuration reference, chapters 02, 14 and 16 and the READMEs describe the new keys, the kind `map(string)`, the start check and the function.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `migrations`, `messaging`, `chat`, `identity`, `platform/config` and `platform/pg` updated (the platform function with the hand-off of note 12, the migration pair and the rewrite note, the language rule and its static tests, the new port methods, the kind `map(string)`, the start check); package documentation of `kernel/searchlang`.
- Rulings of note 14 recorded in `docs/DECISIONS.md` section 10 (one row each with reason).
- `docs/spec/02-architecture.md`, `14-search.md`, `16-operations.md` and `docs/operations/configuration.md` updated as listed in the scope; the configuration reference regenerated with `make config-reference`.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. Ask the red team to look at the round trip of the query text inside `search_tsquery`, the privileges of the function, the table rewrite and lock of the migrations, whether any request can set or change `search_language`, and whether a start or configuration message prints a configured value.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example `perf.message_create_p99` when `perf/load_messages` does not exist yet or the reference profile is not available; behaviour on managed PostgreSQL services).
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found (including every PostgreSQL 18 result that differed from the PostgreSQL 16 examples of 14).

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
