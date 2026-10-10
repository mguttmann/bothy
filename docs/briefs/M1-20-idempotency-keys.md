# M1-20: Idempotency keys

Status: ready | Depends on: M1-16, M1-18, M1-26 | Etappe: M1

Note on dependencies: the plan lists M1-16 and M1-18. M1-26 is added because `idempotency_keys.principal_id` references `principals(id)` (03), and that table comes from M1-26. Do not start before all three are merged.

## Goal

After this ticket the `Idempotency-Key` header works inside the same unit of work as the operation: the key row and the effect commit together, only final responses are stored, a repeat gets the stored response with `Idempotent-Replay: true`, a different request under the same key gets `422 idempotency_key_reuse`, a repeat while the first runs gets `409 request_in_progress`, and the job `sync.idempotency_cleanup` removes keys after `sync.idempotency_key_ttl`. With this, the client outbox (04 "Ausgangsfach (Outbox)") can repeat any keyed write blindly, and every later write ticket gets idempotency without code of its own.

## Read first

- `docs/PLAN.md`: section "M1 Fundament" (rows M1-12 to M1-20, M1-26, M1-31, M1-56), "Arbeitsweise je Ticket", "Entscheidungen dieses Plans" (P-09 to P-12).
- `docs/spec/04-sync-protokoll.md`: "Idempotente Schreibvorgänge" (both subsections), "Ausgangsfach (Outbox)".
- `docs/spec/03-datenmodell.md`: "Grundsätze" (2, 9, 11, 12, 13), tables `idempotency_keys` and `principals`, "Sperrreihenfolge", "Aufbewahrungsklassen".
- `docs/spec/02-architektur.md`: "Ports, Ereignisse und Quellen je Modul" (`sync` implements kernel `idempotency.Store`), "Querschnitts-Ports (`internal/kernel`)", "Transaktionen und Unit of Work" ("Ablauf", "Sperrreihenfolge", "Idempotenz im Unit of Work"), "Fristen" (`database.lock_timeout`), "Wiederholungen", "Hintergrundjobs (River)" ("Warteschlangen", "Job-Katalog", "Regeln für Jobs"), "Konfigurationswerte dieses Kapitels".
- `docs/spec/06-api.md`: "Idempotenz" (all points), "Nebenläufigkeit" (points 1 and 5), "Authentifizierung je Prinzipal-Art" ("Step-up": repeat with the same key), "Fehler".
- `docs/adr/0015-fehlerformat-rfc-9457-idempotenz-paginierung.md`, `docs/adr/0009-commit-geordnetes-ereignis-log-mit-globaler-sequenz.md`.
- `docs/api/openapi.yaml`: parameter `IdempotencyKey`, header `Idempotent-Replay`, header `ETag`.
- Code: `internal/kernel/{txn,ids,clock,errs,actor}`, `internal/platform/{pg,pg/pgtest,jobs,httpserver,httpserver/httpservertest}`, `internal/sync` (M1-16), `internal/identity` port `Directory` (M1-26), `api/routes` and `api/routes/gen` (M1-17, M1-18), `migrations/`, `sqlc.yaml`.
- `docs/quality/gates.md`, `CONTRIBUTING.md`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc comment (purpose, parameters, return value, errors, example where useful); comments explain why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a CHANGELOG entry per release, docs site kept current. A change without matching documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: write the failing test, show it red, then make it green. No en or em dashes in any text. White label everywhere. Configuration values only through named keys from `docs/operations/configuration.md`.
7. 04 "Alle anderen Schreibvorgänge: `Idempotency-Key`" and 06 "Idempotenz": a repeat with the same fingerprint gets the stored response with `Idempotent-Replay: true`; another fingerprint gets `422 idempotency_key_reuse`; a repeat while the first request runs gets `409 request_in_progress`.
8. 06 "Idempotenz" point 2: stored are only final responses: `2xx`, `400`, `403`, `404`, `409` except `request_in_progress`, `410`, `412`, `413`, `415`, `422`. Not stored: `401`, `429`, `5xx`. So a repeat after step-up, token refresh or throttling works with the same key.
9. 06 "Idempotenz" point 1 and 03: the key is valid per principal for `sync.idempotency_key_ttl`; primary key `(principal_id, key)`.
10. 02 "Idempotenz im Unit of Work": key row and effect commit in one transaction; a concurrent repeat waits at the primary key until the first transaction ends and then reads the stored response; if the wait ends with `database.lock_timeout`, the answer is `409 request_in_progress`.
11. 03 table `idempotency_keys` exactly as specified (columns, types, defaults, CHECK, PK, FK, index, retention class `transient`).
12. 02 "Job-Katalog": job `sync.idempotency_cleanup`, module `sync`, periodic, queue `maintenance`; 02 "Regeln für Jobs" apply.

## Scope

Paths are relative to `services/server/` unless they start with `docs/`.

1. `internal/kernel/idempotency` (port; stdlib and kernel only): types, `Store` interface, `ErrInProgress`.
2. `internal/sync`: the store implementation behind the kernel port (`app` for TTL and cleanup logic, `adapters/postgres` with `queries/idempotency_keys.sql` and the sqlc output), the job handler `sync.idempotency_cleanup`, `README.md` (table, job, port).
3. Migration `migrations/<next free number>_sync_idempotency_keys.sql` (goose, sequential number per M1-01 lint) and the `sqlc.yaml` entry if needed.
4. `internal/platform/httpserver`: the middleware for `StageIdempotency` (file `idempotency.go`), built on `kernel/idempotency` and `kernel/txn`.
5. Canonical JSON (RFC 8785): reuse an implementation already in the tree (M1-24 may have added one). Otherwise add `internal/platform/jsoncanon` as a thin wrapper over `github.com/gowebpki/jcs` (Apache-2.0) with the RFC 8785 test vectors.
6. `api/routes/gen` (M1-17): checks for keyed operations (Design notes "Generator checks") with negative fixtures.
7. `internal/wire`: register the stage for roles `api` and `all`; bind the `sync` store to the kernel port; register the periodic job for roles `worker` and `all` through `platform/jobs`.
8. Configuration schema: `sync.idempotency_key_ttl` (04) if not present; new keys `sync.idempotency_cleanup_interval` (default `1h`) and `sync.idempotency_cleanup_batch_size` (default `1000`) in 02 "Konfigurationswerte dieses Kapitels".
9. Docs: 02 "Idempotenz im Unit of Work" (storage of final errors in a second transaction, replay headers), 02 "Job-Katalog" (interval name), 06 "Idempotenz" (replay headers, `ETag` derivation, when the header is ignored), `docs/DECISIONS.md`, `docs/operations/configuration.md` regenerated, `services/server/CHANGELOG.md`.

## Out of scope

- `client_msg_id` and `client_interaction_id` (04 "Nachrichten: `client_msg_id`"): M1-56 and M2.
- Repeat of `startRun` with the same invocation token: M2.
- `If-Match` semantics of each resource: the handler tickets.
- Rate limiting (M1-19), the chain itself (M1-18).
- Any change to the core contract chapters 03 and 04.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-20-idempotency-keys`.
- Conventional Commits with the component as scope (for example `feat(sync): ...`, `feat(httpserver): ...`) and DCO sign-off (`git commit -s`). Squash into meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp`; remove them when done.
- Never read or print secrets. No network calls except package downloads for pinned tools, Go modules and the pinned PostgreSQL 18 test image.
- Tests bind only `127.0.0.1` and port 0 (P-10); PostgreSQL runs as a throwaway container from `pgtest` (M1-12); without a Docker-compatible runtime the tests fail, they never skip (P-09).
- Do not lower a threshold, disable a gate or delete a test.
- A gap in the core contract (chapters 00, 03, 04, 05) that this ticket needs ends the ticket with `Verdict: BLOCKED` (P-11).

## Design notes

### When the stage acts

Only when all of these hold: the operation declares the key (`routes.Operation.IdempotencyKey`), the request carries `Idempotency-Key`, and the actor is not anonymous (the table references `principals`; no public operation declares the key). Otherwise the stage passes the request through and the header has no effect. A key that is not a UUIDv7 (`kernel/ids`) gives `400 validation_failed` with pointer `#header/Idempotency-Key` and code `invalid_format`; nothing is stored.

### Fingerprint (`request_sha256`)

SHA-256 over the upper-case method, a line feed, the escaped request path (`r.URL.EscapedPath()`, including `/api/v1`, without query), a line feed, and the canonical body. Canonical body: RFC 8785 serialization of the body when it parses as JSON without duplicate members; the raw bytes otherwise; empty when there is no body. The stage reads the body under the limit that `StageRoute` set and restores `r.Body` for the following stages. A body over the limit gives `413` and is not stored, because a truncated body has no fingerprint.

### Algorithm

1. Run the rest of the chain inside a unit of work through `kernel/txn` (M1-13), so the use case of the handler joins the same transaction (nested calls join, M1-13).
2. `Store.Begin` in that transaction: delete the row for (principal, key) if `expires_at <= now`; then `INSERT ... ON CONFLICT (principal_id, key) DO NOTHING RETURNING ...` with `state = 'in_progress'`. If no row came back, read the existing row:
   - `completed` with the same fingerprint: replay (below).
   - `completed` with another fingerprint: `422 idempotency_key_reuse`.
   - `in_progress`: `409 request_in_progress`.
   A conflicting, not yet committed insert of a concurrent request makes the insert wait. If that wait ends with `database.lock_timeout` (SQLSTATE `55P03`), the store converts it to `idempotency.ErrInProgress` before returning, so the unit of work does not treat it as a retryable lock timeout, and the stage answers `409 request_in_progress`. In all these cases the transaction is rolled back and nothing is stored.
3. Otherwise call the next stages with a buffering response writer. By the status of the handler's response:
   - `2xx`: `Store.Complete` in the same transaction (`state = 'completed'`, `response_status`, `response_body`, `resource_id` = the top-level `id` of the body when it is a UUID), then commit. If the commit fails, the client gets `500 internal_error` and nothing is stored, because the effect did not happen.
   - `400`, `403`, `404`, `409` except `request_in_progress`, `410`, `412`, `413`, `415`, `422`: roll the unit of work back (no effect), then `Store.SaveFinal` in its own short transaction (`INSERT ... ON CONFLICT DO NOTHING`). Error responses have no effect, so a concurrent duplicate that slips in between rollback and save cannot cause a second effect.
   - every other status (`401`, `428`, `429`, `3xx`, `5xx`): roll back and store nothing.
   Then send the buffered response.
4. Deadlock and lock-timeout retries of the unit of work (M1-13, `database.tx_retry_max`) re-run steps 2 and 3; reset the response buffer and restore the body for every attempt.

### Replay

Status and body from the row; `Content-Type: application/json` for a `2xx` with body, `application/problem+json` for a stored problem; `Idempotent-Replay: true`; `ETag` with the value of `version` from the body in double quotes when the operation declares `ETag` for that status (`routes.Operation.ETagStatuses`; 06 "Nebenläufigkeit" point 1). `Correlation-Id` and the `RateLimit` headers are those of the current request; a stored problem body keeps its original `correlation_id`. The replay does not run the handler, the validator or the proof stage; authentication and rate limiting have run before. Record in `docs/DECISIONS.md` that a principal whose rights changed within `sync.idempotency_key_ttl` still gets its own earlier response (04 "Alle anderen Schreibvorgänge" says the repeat returns the stored response).

### Port `kernel/idempotency`

Use the ID type of `kernel/ids`.

```go
// ErrInProgress reports that the first request with this key is still running.
var ErrInProgress = errors.New("idempotency: first request still in progress")

type Request struct {
	PrincipalID ID        // actor ID
	Key         ID        // Idempotency-Key, UUIDv7
	Action      string    // x-authz-action of the route
	SHA256      [32]byte  // fingerprint
	Now         time.Time // kernel/clock
	TTL         time.Duration
}
type Response struct {
	Status     int
	Body       []byte // JSON, or empty for 204
	ResourceID ID     // zero value when none
}
type Entry struct {
	Response
	SHA256    [32]byte
	Completed bool
}
type Store interface {
	// Begin needs the ambient transaction of the caller's unit of work.
	Begin(ctx context.Context, req Request) (existing Entry, found bool, err error)
	// Complete needs the same ambient transaction.
	Complete(ctx context.Context, req Request, resp Response) error
	// SaveFinal opens its own transaction; it runs after a rollback.
	SaveFinal(ctx context.Context, req Request, resp Response) error
}
```

### Table and migration

Exactly the 03 table `idempotency_keys`: `principal_id uuid not null`, `key uuid not null`, `action text not null`, `request_sha256 bytea not null`, `state text not null default 'in_progress'` with `CHECK (state IN ('in_progress','completed'))`, `response_status smallint`, `response_body jsonb`, `resource_id uuid`, `created_at timestamptz not null default now()`, `expires_at timestamptz not null`; primary key `(principal_id, key)`; foreign key `principal_id` to `principals(id) ON DELETE CASCADE`; index `(expires_at)`. The application sets `created_at` and `expires_at = created_at + sync.idempotency_key_ttl` from `kernel/clock` (03 Grundsatz 2). Grants for the application role follow the convention of M1-12 (`bothy_app` needs SELECT, INSERT, UPDATE, DELETE). SQL only through sqlc queries of the `sync` module (03 Grundsatz 12, SEC-IN-5).

### Cleanup job

`sync.idempotency_cleanup`: periodic every `sync.idempotency_cleanup_interval`, queue `maintenance`, executed only in roles `worker` and `all`, no arguments (02 "Regeln für Jobs": arguments carry IDs only). It deletes rows with `expires_at <= now` in batches of `sync.idempotency_cleanup_batch_size`, ordered by `expires_at`, and repeats until a batch is smaller than the batch size or the context ends; on cancellation it returns `ctx.Err()`. A repeated run is harmless.

### Generator checks (extend `api/routes/gen`)

For every operation that declares `IdempotencyKey`: no query parameters (the fingerprint covers the path only); request body absent or `application/json` only; every `2xx` response has `application/json` content or no content; when a `2xx` response declares `ETag`, its schema has a required integer property `version`. Each violation fails the generator naming the operation.

### Decisions to record in `docs/DECISIONS.md`

Fingerprint definition; storage of final errors in a second transaction; header ignored for anonymous actors and for operations without the parameter; `413` before the fingerprint not stored; `ETag` on replay derived from `version`; replay does not re-run `authorize`; the two new configuration values.

## Tests (write them first)

Integration tests against PostgreSQL 18 (`pgtest`), through `httpservertest` with fake keyed operations. Test handlers write an effect row into a test table created by the test in its own throwaway database, inside the ambient transaction; principals come from the `identity` port of M1-26 or its test fixture.
- `TestReplay_ReturnsStoredResponseWithHeader`: the handler runs once; status and body equal; `Idempotent-Replay: true`; `ETag` derived from `version`.
- `TestReuse_DifferentBody422`, `TestReuse_DifferentPath422`.
- `TestCanonical_EquivalentJSONReplays`: member order, whitespace and string escapes differ, the request still replays.
- `TestInProgress_409AfterLockTimeout`: the first handler blocks on a channel; with a short `database.lock_timeout` in the test configuration the second request gets `409 request_in_progress` and nothing is stored for it; after the first completes, a third request replays.
- `TestConcurrent_ExactlyOneEffect`: many parallel identical requests produce exactly one effect row; all other answers are replays or `409 request_in_progress`.
- `TestNotStored_401StepUpThenRetrySucceeds`: the handler answers `401 step_up_required` first and succeeds on the second call with the same key; one effect, the `2xx` is stored.
- `TestNotStored_5xx`, `TestNotStored_429` (a fake rate-limit stage answers 429: no row), `TestNotStored_428`.
- `TestStoredFinal_404ReplayedWithoutEffect`, `TestStoredFinal_409RollsBackEffect` (the handler writes an effect row and then returns a conflict: the effect row is gone, the 409 is stored and replayed with its original `correlation_id`).
- `TestPerPrincipal_SameKeyIndependent`.
- `TestExpired_TreatedAsAbsent`.
- `TestInvalidKey_400`, `TestIgnored_OperationWithoutKeyParameter`, `TestIgnored_AnonymousActor`.
- `TestRetry_ReinvokesWithFreshBuffer`: through a fault hook of M1-13 if it exists, else a unit test of the stage with a fake unit of work that reports a retryable error once.
- `TestCleanupJob_DeletesExpiredInBatches`, `TestCleanupJob_Idempotent`, `TestCleanupJob_StopsOnCancel`.
- `TestMigration_MatchesDataModel`: columns, types, nullability, defaults, CHECK, PK, FK and index read from `pg_catalog` equal the 03 table.

Unit and generator tests:
- `TestFingerprint_Definition`, `TestETag_FromVersion`, `TestJCS_RFC8785Vectors` (only if a new canonicalization package is added).
- `TestGenerate_RejectsKeyedOperationWithQuery`, `TestGenerate_RejectsKeyedOperationWithNonJSONBody`, `TestGenerate_RejectsKeyedETagOperationWithoutVersion`, `TestRealSpec_KeyedOperationsReplayable`.

Commands that must pass: `cd services/server && go test -race ./internal/sync/... ./internal/kernel/idempotency/... ./internal/platform/httpserver/... ./api/...`, `make migrations-lint`, `make codegen-check`, `make arch`, `make docs`, `make check`.

## Acceptance criteria

- [ ] A repeat returns the stored response with `Idempotent-Replay: true`, without a second effect.
- [ ] Another body or path under the same key gives `422 idempotency_key_reuse`.
- [ ] A repeat while the first request runs gives `409 request_in_progress` after `database.lock_timeout`.
- [ ] `401`, `429` and `5xx` are not stored; the repeat after step-up takes effect once.
- [ ] Parallel requests with the same key have exactly one effect.
- [ ] The key is valid per principal; expired keys behave as absent and the cleanup job removes them in batches.
- [ ] The migration equals the 03 table; `make migrations-lint` is green.
- [ ] Generator checks hold for every keyed operation of `openapi.yaml`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as above; `make check` green.
- Documentation: `internal/sync/README.md` (table, port, job, configuration) and the doc comments of `kernel/idempotency` and the stage (purpose, parameters, return value, errors, example where useful), comments explain why; the 02 and 06 edits; `docs/DECISIONS.md` entries; regenerated configuration reference; a `services/server/CHANGELOG.md` entry under Unreleased. An ADR only if you take an architecture decision beyond 02 "Idempotenz im Unit of Work" and ADR 0015.
- Independent review over the diff (`make review` plus a code review) and the red team gate (`docs/quality/red-team-gate.md`) over the diff, with the verdict `JA` or `NEIN` recorded in the report. On `NEIN`: fix, one test per finding, recheck the delta.
- Merge locally into the base branch only if review and red team are green and `make check` passes; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits, files with line counts, test results with counts (including the red run of each new test), what is not proven, and follow-ups that could not be done in scope with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
