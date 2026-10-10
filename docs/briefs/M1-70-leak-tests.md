# M1-70: Leak tests and 404 equality

Status: ready | Depends on: M1-62, M1-65 | Etappe: M1

## Goal

After this ticket no reading path of the server core reveals a foreign channel, thread, message,
space or agent: a route-driven test proves that a foreign ID and an unknown ID give the same status,
body, headers and median response time (AZ-P4, AZ-M52); a property test over random worlds proves
that lists, counters, the snapshot and catch-up count nothing the principal cannot read (AZ-P5); a
lint in the gate rejects every sqlc read query on content tables without the visibility predicate
unless it is on a reviewed exception list (AZ-P12). These are the tests behind threat T-AZ-02.

## Read first

- `docs/spec/07-rechte.md`: "Grundsätze", "Die Funktion `authorize`" ("Entscheidungen und
  Gründe", "Ablauf", "Sichtbarkeit"), "Filterung in SQL: Rechte-Cache mit Versionszähler"
  ("Abfrage-Muster", rules 1 to 5), "Zustellfilter je Empfänger und sofortige Wirkung",
  "Beispiele", "Tests" (AZ-M52, AZ-P4, AZ-P5, AZ-P12), "Konfigurationswerte dieses Kapitels"
  (`authz.not_found_timing_tolerance`).
- `docs/spec/05-aktionen.md`: "Begriffe der Tabellen" ("Spalte Recht", "Ziel einer Ressource",
  "Entscheidung"), "Aktionen" (rows `space.read`, `channel.read`, `thread.read`, `message.read`,
  `agent.read`, `user.read`, `device.list`, `group.read`, `grant.read`, `mention.list`,
  `settings.read`, `draft.read`, `sync.snapshot.read`, `events.catch_up`).
- `docs/spec/04-sync-protokoll.md`: "Lebenszyklus" (`ready` snapshot), "Nachholen (Catch-up)"
  ("Cursor", "Ablauf", "REST-Variante").
- `docs/spec/06-api.md`: "Autorisierung in der API", "Fehler", "Paginierung" (no totals).
- `docs/spec/03-datenmodell.md`: "Grundsätze" item 12, tables `spaces`, `channels`, `threads`,
  `thread_tags`, `forum_tags`, `messages`, `message_revisions`, `mentions`, `reactions`, `pins`,
  `message_templates`, `read_states`, `drafts`, `event_log`, `agents`.
- `docs/spec/02-architektur.md`: "Code-Layout", "Automatische Prüfung der Regeln".
- ADR 0017 (Confirmation), ADR 0018.
- `docs/security/threat-model.md`: T-AZ-02.
- Operations (by operationId): every enabled GET operation in `services/server/api/operations.yaml`;
  in M1 expected among them: `getSpace`, `listSpaces`, `listSpaceChannels`, `getChannel`,
  `listForumTags`, `listTemplates`, `listChannelThreads`, `getThread`, `listThreadMessages`,
  `listPins`, `getMessage`, `listMessageRevisions`, `listReactions`, `getAgent`, `listAgents`,
  `getUser`, `getUserCredentials`, `listUserDevices`, `listUserSessions`, `viewAsUser`, `getDevice`,
  `listTokens`, `getGroup`, `listGroupMembers`, `getGrant`, `explainSpaceAccess`,
  `explainChannelAccess`, `getIdentityProvider`, `getMySidebar`, `listMyMentions`,
  `listMyReadStates`, `listMyDrafts`, `getSnapshot`, `listEvents`.
- `docs/quality/gates.md`, `Makefile`, `tools/pyproject.toml`, `tools/ci/` (style of repository
  tools).

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every test is written and seen failing before the code that makes it pass.
7. No en dash and no em dash in any file; white label.
8. Configuration only through named keys from `docs/operations/configuration.md`.
9. R-AZ-01 and 06 "Autorisierung in der API" item 3: no read right gives `404 not_found`, whether
   the resource exists or not.
10. AZ-P4: answers for a foreign ID and an unknown ID do not differ in status, body or headers, and
    their medians differ by at most `authz.not_found_timing_tolerance`.
11. AZ-P5: in random worlds, unread counters, mention counters, lists and the snapshot never count a
    channel without read right (search, facets and "Läuft gerade" join this test with M2 and M3).
12. AZ-P12 and 07 "Abfrage-Muster" rule 1: every query over content tables carries
    `channel_id = ANY($visible)` or `space_id = ANY($visible_spaces)` unless it is on the reviewed
    exception list.
13. AZ-M52: Alex reading a message in Marketing/Kampagnen by ID gets the same answer as for a random
    ID.

## Scope

1. `services/server/test/leak/` (Go, package `leak`):
   - `routes.go`: reads the enabled operation IDs from `services/server/api/operations.yaml` (with
     the loader M1-17 provides, or its documented format) and the operations from
     `docs/api/openapi.yaml`; selects every enabled GET operation with at least one path parameter
     of format `uuid`.
   - `fixtures.go`: `type ParamFixture func(w *harness.World, requester *harness.Client) (foreignID
     string, ok bool, reason string)` registered by path parameter name (`space_id`, `channel_id`,
     `thread_id`, `message_id`, `agent_id`, `user_id`, `device_id`, `group_id`, `grant_id`,
     `provider_id`, `principal_id`). `ok=false` requires a reason citing 05 (for example `user_id`
     for human requesters: the directory is readable for every human, 05 `user.read`).
   - `normalize.go`: response normalisation (Design notes).
   - `world.go`: random world generator for AZ-P5.
   - `notfound_test.go`, `counters_test.go`, `README.md`.
2. Shared test harness `services/server/test/harness` (create or extend, text below).
3. SQL lint: `tools/lint/sql_visibility.py` (Python 3.12, PyYAML only), configuration
   `tools/lint/sql_visibility.yaml`, tests `tools/lint/tests/test_sql_visibility.py` with fixtures
   in `tools/lint/tests/fixtures/`, `tools/lint/README.md`.
4. `Makefile`: target `sql-visibility` (help text `## SQL visibility predicate on content tables
   (AZ-P12)`) added to `gates`; prints the SKIP line like the other Go gates when no Go module is
   selected; add `tools/lint` to the bandit directory list in `lint`.
5. `tools/pyproject.toml`: add `lint` to pytest `testpaths` and `pythonpath`, coverage `source`
   and mypy `files`.
6. Fixes in owning modules for every leak or lint violation found (see Boundaries).
7. Documentation: `docs/quality/gates.md` (row `sql-visibility`), `docs/spec/07-rechte.md`
   "Abfrage-Muster" rule 1 (one sentence naming the lint and `tools/lint/sql_visibility.yaml`),
   `services/server/README.md` ("Regeln, die das Gate hält" and test packages),
   `services/server/CHANGELOG.md` and root `CHANGELOG.md` (tool) under "Unreleased".

### Shared test harness (identical text in the briefs M1-69 to M1-72)

If `services/server/test/harness` exists on the base branch, use it and extend it only additively.
If it does not exist, first search for an existing full-server test helper
(`grep -rn "pgtest" services/server/internal/wire services/server/test`); wrap it if it builds the
real `internal/wire` graph, otherwise create the package with exactly this interface. Parallel
tickets build the same interface, so merges converge.

- Package `harness`, non-test Go files, architecture component `testsupport` (Design notes).
- `func Start(tb testing.TB, opts Options) *Server`: the real `internal/wire` graph with role `all`
  in this process; PostgreSQL 18 from `internal/platform/pg/pgtest`; migrations as
  `bothy migrate up --bootstrap` runs them; secrets created as `bothy secrets init` creates them, in
  `tb.TempDir()`; public and internal listener on `127.0.0.1:0`; the first owner created through the
  same application service that `bothy setup bootstrap-owner` calls (08 "Erstinstallation"), then
  enrolled with its enrollment code and fresh ES256 device keys (08 "Einladungs-Code",
  "Registrierung"); everything stopped in `tb.Cleanup` in reverse order.
- `type Options struct { Config map[string]string; LogWriter io.Writer; Telemetry *TelemetrySinks;
  EgressDenyAll bool; Clock clock.Clock }`. `Config` takes dotted keys from
  `docs/operations/configuration.md` and is applied through `platform/config` with flag precedence,
  so an unknown key still fails.
- `type Server struct { PublicURL, APIBase, GatewayURL, InternalURL, SecretsDir string; Owner *Client }`
  with `APIBase = PublicURL + "/api/v1"` and `func (s *Server) AdminDB() *pgxpool.Pool` (superuser
  of the disposable container; assertions only).
- `func (s *Server) NewUser(tb, UserSpec{Handle, DisplayName, Email string}) *Client` and
  `func (s *Server) NewAgent(tb, AgentSpec{Handle, DisplayName, BillingMode, CredentialHolder string}) *Client`,
  only through the public API (`createUser`, `createEnrollmentCode`, `completeEnrollment`;
  `createAgent`, `createToken`), owner step-up via `createStepUpChallenge` and
  `completeStepUpWithDeviceKey`.
- `type Client struct { PrincipalID, DeviceID string; ... }` with `Do(ctx, Request) Response`
  (DPoP proof per RFC 9449 with the latest `DPoP-Nonce`, one retry on `use_dpop_nonce`, refresh on
  `token_expired`, `Idempotency-Key` UUIDv7 on POST and PATCH except the list in 06 "Idempotenz",
  optional `Observer func(Request, Response)`), `StepUp(ctx)`, `Gateway(ctx) *GatewayConn`
  (`hello`, `identify`, `ready`, `Next`, `Send`, `Close`, automatic `heartbeat` and `reauth`).
- `type Response struct { Status int; Header http.Header; Body []byte; Elapsed time.Duration }`.
- `func BuildMatrixWorld(tb, s *Server) *World`: the M1 subset of 07 "Beispiele" and
  "Testmatrix": spaces Support, Marketing, Engineering, Research; channels Support/Allgemein,
  Support/Eskalation, Marketing/Kampagnen, Engineering/Projekt-A (forum, no project binding),
  Engineering/Werkzeuge, Research/Notizen; people Alex, Sam, Robin; agents Lotse (organization,
  operator of Support), Texter (organization, operator of Marketing), Runner (organization, operator
  of Engineering/Werkzeuge), Assistent (personal, credential holder the owner, operator of Research);
  group Leser with member Sam (membership with expiry); grants G-1 (Alex, `trigger`, `upload` and
  `approve_actions`, space Support), G-2 (Sam, `trigger`, `upload`, space Marketing), G-3 (Robin,
  `trigger`, `upload`, channel Engineering/Projekt-A, with expiry), G-4 (group Leser, `read`,
  Support/Allgemein), G-5 (Lotse, `write`, `upload`, space Support); one thread with messages per
  text channel and one forum post in Projekt-A. Left out until their etappe: app Tracker and G-6
  (M2), the DM Alex-Sam (V3), project binding and tool class `host` (M2). Expiry times are relative
  to the harness clock, never calendar dates.
- `func StartProcess(tb, ProcessOptions) *Process` (subprocess server for M1-72): not needed in
  this ticket, do not create it here if absent.
- Unit tests for every part you add (at least: DPoP proof claims and the nonce retry).

## Out of scope

- Search, facets, suggestions and "Läuft gerade" in AZ-P5 (M2, M3 extend the property test).
- Gateway `focus` on foreign threads (M1-66 tests it).
- Rate-limit behaviour (M1-19) and authorize matrix rows other than AZ-M52 (M1-28, M1-54).
- Changes to `authorize` semantics; if a leak needs a change in the core contract (00, 03, 04, 05),
  stop with `BLOCKED`.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-70-leak-tests`.
- Conventional Commits with module scope (`test(leak): ...`, `build(lint): ...`,
  `fix(<module>): ...`) and DCO sign-off; description language as in the existing `git log`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside this scope, except the docs and changelog entries named here, the
  shared harness and minimal fixes in the owning module for leaks or lint violations this ticket
  finds, each with a regression test in that module and listed in the report. Never add an
  exception to make a violation pass when the query can carry the predicate.
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
  Temporary directories only via `mktemp` or `t.TempDir()`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools; bind
  only `127.0.0.1` and port 0; PostgreSQL as a disposable container on a random port.
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

AZ-P4 (route-driven, `notfound_test.go`):

1. Server: `harness.Start` with every `ratelimit.<class>.per_minute` and `.burst` raised through
   `Options.Config` (rate limits are not under test here; recorded in the README), then
   `BuildMatrixWorld`.
2. Requesters: Alex (human, DPoP) and Lotse (agent token). For each selected route, requester and
   path parameter set: build request A with the foreign IDs from the fixtures and request B with
   fresh random UUIDv7 values in the same parameters. A foreign ID is an existing resource for which
   the requester has no read path under 07 "Sichtbarkeit".
3. Assert after normalisation: same status; same header names and values; same body; same
   `Content-Length`. When the requester lacks read on a content resource (space, channel, thread,
   message, agent), the status must be `404` with code `not_found`.
4. Normalisation: drop `Correlation-Id`, `Date`, `DPoP-Nonce`; for `RateLimit` keep the policy name
   and drop `r` and `t`; in problem bodies drop `correlation_id` and replace the requested IDs in
   `instance` with a placeholder; compare JSON structurally and the raw byte length after the same
   replacement.
5. Timing: for each route and requester, a warm-up, then interleaved A and B requests (ABAB) with a
   fixed sample count as a named constant in `notfound_test.go`; the medians of `Elapsed` must
   differ by at most `authz.not_found_timing_tolerance`. Read the tolerance the way the existing
   ID-T21 test of M1-39 does (`grep -rn not_found_timing_tolerance services/server`); never a
   literal.
6. `TestAZ_P4_RouteCoverage` fails when a selected route has a path parameter without fixture, or a
   fixture returns `ok=false` without reason. It prints the table of routes, requesters and outcome.

AZ-P5 (property test, `counters_test.go`, `world.go`):

1. Generator: seeded by the environment variable `LEAK_SEED` or the current time; the seed is
   printed so a failure can be replayed. World sizes and the number of worlds are named constants
   in `world.go`. A world has spaces, text and forum channels, the observed principal P (a human
   who is not owner, and in a second run an agent with the operator rule of 07 "Agenten als
   Prinzipale"), grants of P on instance, space and channel with random levels and expiry (some
   already expired by the harness clock), group memberships of P with expiry and group grants,
   threads and messages by others with mentions of P in visible and invisible channels, read
   states and drafts of P, including drafts and read states in channels whose access P loses
   during the world build.
2. Truth: per-item reads by P (`getSpace`, `getChannel`, `getThread`, `getAgent`): `200` means
   visible, `404` means not visible. The SQL filter is performance, `authorize` is truth (07
   "Abfrage-Muster" rule 3).
3. Surfaces checked: `getSnapshot` and the gateway `ready` snapshot, `listSpaces`,
   `listSpaceChannels` per visible space, `listChannelThreads` per visible channel, `listAgents`,
   `getMySidebar`, `listMyMentions`, `listMyReadStates`, `listMyDrafts`, and `listEvents` from cursor
   0 to the head.
4. Property 1 (reference): every space, channel, thread and agent ID referenced by a surface is
   visible by the truth; every event from `listEvents` has a visible scope or `audience` self;
   unread counts per visible thread equal the model (messages after `last_read_seq`, 03
   `read_states`) and mention counts equal the mentions of P in visible channels only.
5. Property 2 (noise invariance): add noise only in channels P cannot read (threads, messages,
   mentions of P, reactions, pins), then query again: all surfaces equal after dropping `head_seq`,
   `next_cursor` and `cursor` frames (the checked cursor advances by design, 04 "Cursor").
6. No totals anywhere (06 "Paginierung").

AZ-M52: `TestAZ_M52_ForeignMessageEqualsUnknown` uses `BuildMatrixWorld`: Alex `GET
/api/v1/messages/{message_id}` with a message of Marketing/Kampagnen versus a random UUIDv7, same
assertions as AZ-P4 items 3 and 4.

SQL lint (`tools/lint/sql_visibility.py`, AZ-P12):

1. Input: every file matching `services/server/internal/*/adapters/postgres/queries/*.sql`. A query
   is a block starting with a sqlc header `-- name: <Name> :<cmd>`; its identifier is
   `<module>.<Name>` with the module from the path.
2. Only reads are checked: the first keyword after comments is `SELECT` or `WITH` and the
   statement contains no `INSERT`, `UPDATE` or `DELETE` at top level.
3. Content tables are listed in `sql_visibility.yaml` under `content_tables` (final state, also
   tables of later etappen so new queries are covered from their first commit): `spaces`,
   `channels`, `channel_skill_bindings`, `project_bindings`, `dm_participants`, `forum_tags`,
   `threads`, `thread_tags`, `agent_sessions`, `messages`, `message_revisions`, `mentions`,
   `reactions`, `pins`, `message_templates`, `files`, `file_thumbnails`, `message_attachments`,
   `file_text_extracts`, `runs`, `run_steps`, `run_followups`, `message_components`,
   `interactions`, `read_states`, `drafts`, `event_log`.
4. A read that references a content table (in `FROM` or `JOIN`, with or without alias) passes when
   it contains a predicate matching `(alias.)?(channel_id|space_id) = ANY(<param>)` where `<param>`
   is a sqlc parameter (`$n`, `@name`, `sqlc.arg(name)`, `sqlc.narg(name)`) with an optional cast.
   A subquery inside `ANY(...)` does not count (03 "Grundsätze" item 12).
5. Otherwise the query must be listed under `exceptions` with `query: <module>.<Name>`,
   `category` from the closed set `authorize_resolution` (resolves a resource path for
   `authorize`), `after_authorize` (loads a single resource after `authorize` allowed it),
   `system_job` (job without principal), `log_tail` (the log tail reads every event; delivery
   checks `authorize` per receiver), and a non-empty `reason`.
6. Fails also on: an exception whose query no longer exists (stale), an unknown category, a
   duplicate entry, a content table missing from 03 (cross-check table names against the table
   headings of `docs/spec/03-datenmodell.md`).
7. Output: one line per finding `file:line: <module>.<Name>: <rule>`; exit code 1 on findings, 0
   otherwise, 2 on usage errors. Usage: `python tools/lint/sql_visibility.py [--repo DIR]`.

Architecture register: `test/**` and `perf/**` under `services/server` are test support. If
M1-05's generator (`tools/archgen`) has no component for them, add a component `testsupport`
covering both, with the go-arch-lint v3 keys `anyProjectDeps: true` and `anyVendorDeps: true`
(verify against the pinned version), and a generator test proving that no production component may
depend on `testsupport`. Exclude `services/server/test/**` and `services/server/perf/**` from the
Go coverage total in the `coverage` target (they are test code; Sonar already classifies
`**/test/**` as tests).

## Tests (write them first)

Go (run in `make test`; names carry the ID with `_`; subtests carry the literal ID):

1. `TestAZ_P4_RouteCoverage`.
2. `TestAZ_P4_NotFoundEquality` with subtests `AZ-P4/<operationId>/<requester>`.
3. `TestAZ_P4_TimingMedian` with the same subtests.
4. `TestAZ_M52_ForeignMessageEqualsUnknown`.
5. `TestAZ_P5_ListsAndCountersMatchAuthorize` (property 1, several seeds per run).
6. `TestAZ_P5_NoiseInvariance` (property 2).
7. Unit tests of `normalize.go` and `world.go` (seed reproducibility).

Python (`tools/lint/tests/test_sql_visibility.py`, run in `make test`):

1. `test_az_p12_read_without_predicate_fails`, `test_az_p12_predicate_with_param_passes` (each
   parameter form), `test_az_p12_any_with_subquery_fails`, `test_az_p12_join_with_alias_predicate_passes`,
   `test_az_p12_write_statement_ignored`, `test_az_p12_exception_passes`,
   `test_az_p12_stale_exception_fails`, `test_az_p12_unknown_category_fails`,
   `test_az_p12_duplicate_exception_fails`, `test_az_p12_unknown_content_table_fails`,
   `test_az_p12_repository_clean` (runs on the real tree).

Commands that must pass:

```sh
cd services/server && go test -race -count=1 ./test/leak/... ./test/harness/...
make sql-visibility
make test
make check
```

## Acceptance criteria

- [ ] AZ-P4 runs for every enabled GET route with a UUID path parameter and both requesters; every
      route without a foreign instance has a reason from 05 in the printed table.
- [ ] Foreign and unknown IDs give identical normalised status, headers, body and length; medians
      within `authz.not_found_timing_tolerance`.
- [ ] AZ-M52 passes.
- [ ] AZ-P5 property 1 and property 2 pass for the printed seeds; a failing seed is replayable with
      `LEAK_SEED`.
- [ ] `make sql-visibility` passes on the tree; each negative fixture fails; every exception has a
      category and reason.
- [ ] Every leak or violation found is fixed in the owning module with a regression test.
- [ ] `make check` is green.

## Definition of done

- Code and tests as above; README in `services/server/test/leak/`, `tools/lint/` and the harness
  (if created); doc comment on every exported symbol and docstrings on every public Python function.
- No new ADR (the decision is ADR 0017); if you add the `testsupport` component, describe it in
  `services/server/README.md` and in 02 "Code-Layout" (one line).
- CHANGELOG entries under "Unreleased" in `services/server/CHANGELOG.md` and root `CHANGELOG.md`.
- `make check` green.
- Independent review and red team over the diff (`docs/quality/red-team-gate.md`) with verdict
  recorded in the report; merge locally into the base branch only if both are green, then
  `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits,
files with line counts, test results with counts (including red runs and the seeds used), the
AZ-P4 coverage table, the exception list with categories, leaks and violations found and fixed,
what is not proven (at least: timing on hosts other than the build host; search, suggestions and
"Läuft gerade" until M2 and M3), follow-ups that could not be done in scope with reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
