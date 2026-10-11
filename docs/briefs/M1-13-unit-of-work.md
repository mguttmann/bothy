# M1-13: Unit of work with phase hooks and in-transaction events

Status: ready | Depends on: M1-06, M1-08, M1-12 | Milestone: M1

## Goal

After this ticket every write use case of the server runs in exactly one PostgreSQL transaction
that `internal/platform/pg` opens, carries through the context (ambient transaction), runs the
pre-commit phase hooks in the fixed order `authz`, `sync`, `audit`, delivers in-transaction
events synchronously and retries the whole unit on deadlock or lock timeout. This is the
mechanism that makes the commit-ordered event log (B8, ADR 0009), the I1 check and the audit
hash chain correct: the lock order of 03 "Lock order" is enforced in one place instead of
in every use case. No module hook is built here; the phase hooks of `authz`, `sync` and `audit`
arrive with M1-29, M1-16 and M1-24 and plug into the registry this ticket builds.

## Read first

- `docs/spec/02-architecture.md`, sections "Transactions and Unit of Work" (all of "Flow",
  "Lock order", "In-Tx events", "Idempotency in the Unit of Work",
  "What never happens in a transaction"), "Cross-cutting ports (`internal/kernel`)",
  "Layers within a module", "Allowed dependencies", "Automatic check of the rules"
  ("Module register as the source", "go-arch-lint", "Other checks in the gate"),
  "Error and timeout strategy" ("Error kinds", "Deadlines", "Retries", "Panics"),
  "Decisions of this chapter" (R-AR-04 to R-AR-07, R-AR-18),
  "Configuration values of this chapter".
- `docs/spec/03-data-model.md`, "Principles" (item 11 "Isolation level"), "Transaction rules"
  ("Global event sequence", "Lock order", "I1 check").
- `docs/spec/04-sync-protocol.md`, "Global, commit-ordered sequence" (why the counter step is
  last and why `READ COMMITTED` is required).
- `docs/spec/06-api.md`, "Errors" (the generated problem code table; only codes listed there
  exist on the wire).
- ADR 0007 (`docs/adr/0007-typed-database-access-sqlc-and-migration-tool.md`),
  ADR 0008 (`docs/adr/0008-hexagonal-modules-and-checked-dependency-rules.md`),
  ADR 0009 (`docs/adr/0009-commit-ordered-event-log-with-a-global-sequence.md`, section
  "Confirmation").
- `docs/observability/signals.md`, sections "Spans" and "Attributes in spans and logs".
- What earlier tickets built: `services/server/internal/kernel/` (M1-05: `ids`, `errs`, `clock`,
  the declared `txn` port), `services/server/internal/platform/pg/` and `pgtest/` (M1-12: pool,
  `SET LOCAL` timeouts, disposable PostgreSQL 18 container), `services/server/internal/wire/`,
  `services/server/architecture/modules.yaml`, `tools/archgen/`, `.semgrep/` (rule plus fixture
  pattern, see `bothy-go-sql-string-building.*`).
- `docs/PLAN.md` section "Way of working per ticket" and section 7 "M1 Foundation";
  `docs/quality/gates.md`; `CONTRIBUTING.md`; `docs/quality/red-team-gate.md`.

This ticket touches no API operation, no table of 03 and no action id.

## Fixed requirements that apply

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
6. Test first: write each test, run it and see it fail (keep the red output for the report), then
   write the code until it passes, then clean up.
7. No en dash or em dash characters in anything you write (code, comments, docs, commit
   messages). Check every changed file with `uv run --project tools --locked --quiet python tools/hooks/text_rules.py <file>`;
   it must exit 0 (plain `grep -P` fails on macOS).
8. White label: no company names, company or mail domains, person names, host names, real IP
   addresses or paths under a user home directory. Examples use `example.com` and the names Alex,
   Sam, Robin.
9. Configuration values only through named keys that appear in `docs/operations/configuration.md`
   and the config schema in code (Ruling L4). No literal limit, timeout or retry count in code
   outside the schema defaults.
10. Every write use case runs in exactly one transaction at `READ COMMITTED`; a call into another
    module joins the ambient transaction; only `platform/pg` opens and ends transactions
    (02 "Flow", R-AR-06).
11. Phases before commit in fixed order: permission begin hook (only with `RightsChange`), work,
    `authz` check hook (only with `RightsChange`), `sync` hook, `audit` hook, commit
    (02 "Flow"). A veto in the work phase or in the `authz` check rolls everything back.
12. In-transaction events are delivered synchronously, in registration order, inside the same
    transaction; a subscriber may write, append durable events and veto the commit with an error;
    every subscriber gets its own OpenTelemetry span (02 "In-Tx events").
13. Registration of hooks and subscriptions happens only in `internal/wire`, as one list; a golden
    file records it; a subscription may only point from a higher module to a lower one
    (02 "Other checks in the gate", R-AR-04).
14. On SQLSTATE `40P01` (deadlock) or `55P03` (lock not available) the whole unit is retried at
    most `database.tx_retry_max` times (02 "Lock order", "Retries", R-AR-18).
15. Write transactions never run at `REPEATABLE READ` or `SERIALIZABLE`; a test proves it
    (ADR 0009, "Confirmation").
16. Every transaction applies `database.statement_timeout`, `database.lock_timeout` and
    `database.idle_in_tx_timeout` with `SET LOCAL` (02 "Deadlines").

## Scope

All Go paths are relative to `services/server/`. Where M1-05 or M1-12 already created a file or a
name below, extend it and keep the existing name if it is compatible; the shapes below are the
contract later tickets (M1-14, M1-15, M1-16, M1-24, M1-29) build on.

1. `internal/kernel/txn` (port, standard library and `internal/kernel/ids`, `internal/kernel/errs`
   only):
    - `type Phase uint8` with `PhaseNone`, `PhaseRightsBegin`, `PhaseWork`, `PhaseRightsCheck`,
      `PhaseSync`, `PhaseAudit`; `String()` returns `none`, `rights_begin`, `work`,
      `rights_check`, `sync`, `audit`.
    - `type UnitOfWork interface { Do(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error }`.
    - `type Option`; `func RightsChange(affected ...ids.ID) Option` (02 names the option
      `RightsChange`); `func AffectedPrincipals(ctx context.Context) ([]ids.ID, bool)`.
    - `func CurrentPhase(ctx context.Context) Phase` (returns `PhaseNone` outside a unit).
    - `func InUnit(ctx context.Context) bool`.
    - `type Hook func(ctx context.Context) error`.
    - `type Registry`; `func NewRegistry() *Registry`;
      `func (r *Registry) SetHook(p Phase, h Hook) error` (allowed phases: `PhaseRightsBegin`,
      `PhaseRightsCheck`, `PhaseSync`, `PhaseAudit`; each at most once; after `Seal` an error);
      `func Subscribe[E any](r *Registry, handler func(ctx context.Context, ev E) error) error`;
      `func (r *Registry) SetTracer(t Tracer)`; `func (r *Registry) Seal()`;
      `func (r *Registry) Sealed() bool`; `func (r *Registry) Registrations() []Registration`.
    - `type Registration struct { Kind string /* "hook" or "subscribe" */; Phase Phase; Event string /* "<module>.<TypeName>", kernel events "kernel.<TypeName>" */; Publisher, Subscriber string /* module names */; Handler string /* runtime function name */ }`.
      Publisher and subscriber modules are derived, not declared: the publisher from the Go package
      path of `E` (segment after `/internal/`; `kernel` for `internal/kernel/...`), the subscriber
      and hook owner from `runtime.FuncForPC` of the handler (same rule). A handler whose module
      cannot be derived (for example a closure defined in `internal/wire`) is rejected with an
      error, so registrations always name the real owner. Hooks and handlers are therefore method
      values or functions of the owning module.
    - `func Publish[E any](ctx context.Context, ev E) error`: delivers `ev` to the subscribers of
      type `E` in registration order. Errors: outside a unit; phase not `PhaseWork` or
      `PhaseRightsCheck`; first subscriber error is returned unchanged (veto) and later subscribers
      do not run.
    - Per-unit state: `type Key[T any]`; `func NewKey[T any](name string) *Key[T]`;
      `func Load[T any](ctx context.Context, k *Key[T]) (T, bool)`;
      `func Store[T any](ctx context.Context, k *Key[T], v T) error` (error outside a unit). The
      state is fresh for every attempt and dropped on rollback; this is where M1-16 buffers durable
      events and M1-24 buffers audit records.
    - `type Tracer func(ctx context.Context, spanName string) (context.Context, func(err error))`;
      the registry calls it once per subscriber delivery with span name
      `txn.subscribe <Subscriber>` (see Design notes).
    - `type Unit interface` plus `WithUnit(ctx, Unit) context.Context` and
      `UnitFrom(ctx) (Unit, bool)`: the minimal contract `platform/pg` implements so the kernel
      helpers above can read phase, registry, options and state. Document that only `platform/pg`
      implements it.
2. `internal/platform/pg` (implementation):
    - `type UnitConfig struct { RetryMax int; StatementTimeout, LockTimeout, IdleInTxTimeout time.Duration }`,
      filled in `internal/wire` from `database.tx_retry_max`, `database.statement_timeout`,
      `database.lock_timeout`, `database.idle_in_tx_timeout`.
    - `func NewUnitOfWork(pool <M1-12 pool type>, cfg UnitConfig, reg *txn.Registry) *UnitOfWork`
      implementing `txn.UnitOfWork`. `Do` before `reg.Seal()` returns an internal error.
    - `func Tx(ctx context.Context) (pgx.Tx, error)`: the ambient transaction, internal error
      outside a unit (used by repositories for writes and by M1-14 for River `InsertTx`).
    - `func Querier(ctx context.Context, pool <pool type>) <sqlc DBTX>`: the ambient transaction if
      there is one, else the pool (reads outside a unit).
3. `internal/wire`:
    - one function `registerTxn(reg *txn.Registry, c *<container>) error` called by every role, the
      only place with `SetHook` and `Subscribe` calls; then `reg.Seal()`. Today it registers
      nothing except the tracer (no module hook exists yet).
    - `func TxnRegistrations(reg *txn.Registry) string`: the golden rendering (format in Design
      notes).
    - `func CheckTxnDirections(regs []txn.Registration, modules <register>) error` using
      `architecture/modules.yaml` (reuse the loader from `tools/archgen` or the YAML library already
      in `go.mod`; do not add a second YAML library).
    - `testdata/txn_registrations.golden`.
    - module constructors used by `registerTxn` must not open network connections, so the golden
      test can build the real registration without a database.
4. `.semgrep/bothy-go-tx-only-in-platform-pg.yaml` with fixture
   `.semgrep/bothy-go-tx-only-in-platform-pg.go`: flags `$X.Begin(...)`, `$X.BeginTx(...)`,
   `pgx.BeginFunc(...)`, `pgx.BeginTxFunc(...)` in Go files outside
   `services/server/internal/platform/pg/` (test files excluded). Message cites 02 "Flow".
   If a later ticket needs a raw transaction elsewhere (for example River internals), that is
   library code and not scanned.
5. Documentation: 02 "Flow" (registry, outermost `RightsChange`, phases for `Publish` and
   events, hooks and subscribers as functions of the owning module), 02 "Retries"
   (exhausted retries end with `503 unavailable`), 02 "Lock order" (the idempotency wait is
   not retried) and the span `txn.subscribe <Subscriber>` in signals.md already state this; change
   them only if the build proves a statement wrong. Write:
    - `services/server/README.md` (kernel and platform tables if wording changes),
      `internal/kernel/txn` doc.go, `internal/platform/pg/README.md` (create if missing): phases,
      retry rule, timeouts, examples.
    - `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` rows only for
      decisions the spec does not state.

## Out of scope

- The phase hooks of `authz` (M1-29), `sync` (M1-16) and `audit` (M1-24); the counters and
  `instance.Counters` (M1-15).
- Idempotency keys inside the unit (M1-20; that ticket decides how its own `55P03` wait maps to
  `409 request_in_progress`, see 02 "Idempotency in the Unit of Work").
- River and `InsertTx` (M1-14).
- Mapping errors to HTTP problems (M1-18); this ticket only returns `kernel/errs` kinds.
- Metrics (M1-68).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-13-unit-of-work`.
- Conventional Commits with module scope (for example `feat(pg): ...`, `feat(txn): ...`) and DCO
  sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end. Test containers bind `127.0.0.1` and a random port (P-09, P-10).
- Never read or print secrets. No network calls except package downloads for pinned tools and Go
  modules.

## Design notes

- Isolation: begin every transaction with `pgx.TxOptions{IsoLevel: pgx.ReadCommitted}`
  explicitly; never rely on the database default.
- Timeouts: right after `BEGIN`, apply the three `SET LOCAL` values with the helper M1-12 built;
  if M1-12 has none, add it in `platform/pg`.
- Order inside one attempt: begin; if `RightsChange`: phase `rights_begin` hook; phase `work`:
  run `fn`; if `RightsChange`: phase `rights_check` hook; phase `sync` hook; phase `audit` hook;
  commit. A missing hook for a phase is skipped. The phase is visible through
  `txn.CurrentPhase`.
- Nesting: `Do` inside a unit joins it, runs `fn` in the current phase and never commits, rolls
  back or retries by itself; any error propagates to the outer `Do`. A nested `Do` with
  `RightsChange` is an internal error unless the outer unit declared `RightsChange` with a superset
  of the principals (ruling: the permission begin hook must lock `permission_version` before any
  domain row, 03 "Lock order" item 1, so the option cannot appear late).
- Retry: only the outermost unit retries, only when `errors.As` finds a `*pgconn.PgError` with
  code `40P01` or `55P03`, only while `ctx.Err() == nil`, at most `database.tx_retry_max` times
  (attempts = 1 + retries). Each attempt starts with fresh per-unit state and re-runs `fn`, so
  in-transaction events are re-published. After the last failed attempt return an error of kind
  `unavailable` with code `unavailable` that wraps the last database error (02 "Error kinds",
  "Retries"; the SQLSTATE stays in the log).
  Any other error is returned unchanged and not retried.
- Rollback on every non-nil error and on panic; after rollback a panic is re-raised so the HTTP
  chain (M1-18) turns it into `500`.
- Context: a cancelled context aborts the transaction (pgx does that); no retry after cancel.
- `Publish` from a subscriber is allowed (nested delivery runs in the same phase); recursion depth
  is bounded by the call stack only; document it.
- Registry derivation of module names: package path segment after `/internal/`; for
  `internal/kernel/...` the module is `kernel`. Event names in registrations are
  `<module>.<TypeName>`.
- Golden format, one line per registration, sorted by kind then phase order then registration
  order:
    ```text
    # txn registrations; regenerate with: go test ./internal/wire -run TestTxnRegistrationsGolden -update
    hook <phase> <module> <handler>
    subscribe <publisher>.<Event> -> <subscriber> <handler>
    ```
    With no module registered yet, the file holds only the header line.

- Direction rule: a subscription is valid if the publisher is `kernel` or the publisher is in
  `depends_on` of the subscriber in `architecture/modules.yaml`. A hook owner must be the module
  named for that phase (`authz` for both permission phases, `sync`, `audit`).
- Span per subscriber: name `txn.subscribe <subscriber module>`, `bothy.outcome` `ok` or `veto`;
  the tracer comes from `platform/otel` and is set in `internal/wire`. Without a tracer the
  registry runs subscribers without spans.
- Configuration (02 "Configuration values of this chapter"): `database.tx_retry_max` (`2`),
  `database.statement_timeout` (`5s`), `database.lock_timeout` (`2s`),
  `database.idle_in_tx_timeout` (`10s`). If M1-07 has not registered any of them in the schema,
  register them with these defaults so `make config-doc-check` stays green.

## Tests (write them first)

Integration tests use `pgtest` (PostgreSQL 18 container on a random port; without a Docker
runtime the test fails, P-09) and a scratch table the test creates.

`internal/platform/pg`:

- `TestUnitOfWork_PhaseOrderWithRightsChange`: fake hooks append their phase to a slice; result
  `rights_begin, work, rights_check, sync, audit`.
- `TestUnitOfWork_PhaseOrderWithoutRightsChange`: `work, sync, audit`; permission hooks not called.
- `TestUnitOfWork_SubscriberVetoRollsBack`: `fn` inserts a row, stores a value under a
  `txn.Key`, publishes an event; the subscriber returns an error; `Do` returns that error
  (`errors.Is`), the row is absent, the sync and audit hooks never ran, and a new unit sees no
  stored value.
- `TestUnitOfWork_RightsCheckVetoRollsBack`: the permission check hook returns an
  `invariant_violation` error; nothing is committed; sync and audit hooks did not run.
- `TestUnitOfWork_RetriesDeadlockUpToMax` and `TestUnitOfWork_RetriesLockTimeoutUpToMax`: `fn`
  returns a `*pgconn.PgError` with `40P01` (resp. `55P03`) for the first k attempts; with
  `RetryMax = 2`: k = 2 succeeds after 3 attempts; k = 3 fails after 3 attempts with kind
  `unavailable` wrapping the `PgError` (`errors.As`).
- `TestUnitOfWork_DoesNotRetryOtherErrors`: `23505` and `57014` are returned after one attempt.
- `TestUnitOfWork_RealDeadlockIsRetried`: two units lock rows A then B and B then A behind a
  barrier; both finally commit; one needed a retry.
- `TestUnitOfWork_FreshStatePerAttempt`: a value stored in attempt 1 is not visible in attempt 2.
- `TestUnitOfWork_NestedJoins`: inner `Do` sees the same `txid_current()`; an inner error rolls
  back the outer work; the inner unit never retries alone.
- `TestUnitOfWork_NestedRightsChangeRejected`: inner `RightsChange` without a covering outer one
  is an internal error.
- `TestUnitOfWork_ReadCommittedEnforced` (ADR 0009 Confirmation): with
  `ALTER DATABASE ... SET default_transaction_isolation = 'serializable'` on the test database,
  `SHOW transaction_isolation` inside `Do` returns `read committed`.
- `TestUnitOfWork_TimeoutsApplied`: `SHOW statement_timeout`, `SHOW lock_timeout`,
  `SHOW idle_in_transaction_session_timeout` inside `Do` equal the configured values.
- `TestUnitOfWork_PanicRollsBackAndRepanics`.
- `TestUnitOfWork_NoRetryAfterCancel`.
- `TestUnitOfWork_RefusesBeforeSeal`.

`internal/kernel/txn`:

- `TestPublish_RegistrationOrder`, `TestPublish_OutsideUnit`, `TestPublish_WrongPhase`
  (`sync`, `audit`), `TestPublish_SpanPerSubscriber` (fake tracer records
  `txn.subscribe <module>` in order and the outcome), `TestRegistry_SetHookTwice`,
  `TestRegistry_SealRejectsChanges`, `TestRegistry_RejectsUnderivableHandler`,
  `TestKey_OutsideUnit`.

`internal/wire`:

- `TestTxnRegistrationsGolden` (with `-update` flag) renders the real registration of all roles
  and compares with `testdata/txn_registrations.golden`; it also runs `CheckTxnDirections`.
- `TestCheckTxnDirections_ReverseSubscriptionFails`: fixture registrations where `instance`
  subscribes to a `chat` event; the check fails and the message names both modules.
- `TestCheckTxnDirections_WrongHookOwnerFails`.

Semgrep: the fixture contains `// ruleid:` cases for each pattern and `// ok:` cases.

Commands that must pass:

```sh
cd services/server && go test -race -count=1 ./internal/kernel/txn/... ./internal/platform/pg/... ./internal/wire/...
make semgrep
make arch
make codegen-check
make config-doc-check
make check
```

## Acceptance criteria

- [ ] Hook order is logged as specified with and without `RightsChange` (tests above).
- [ ] A subscriber veto rolls back domain rows and per-unit state (buffered events).
- [ ] Simulated `40P01` and `55P03` are retried at most `database.tx_retry_max` times; the
      exhausted case returns kind `unavailable`; other errors are not retried.
- [ ] A real deadlock between two units is resolved by the retry.
- [ ] A nested call joins the transaction; nested `RightsChange` without cover is rejected.
- [ ] Transactions run at `read committed` even with a serializable database default.
- [ ] The golden test fails for a subscription in the wrong direction (fixture).
- [ ] The Semgrep rule flags `Begin`/`BeginTx`/`BeginFunc`/`BeginTxFunc` outside `platform/pg`.
- [ ] READMEs and CHANGELOG updated; 02 and signals.md still match the build; `make check` green.

## Definition of done

Code, tests and docs are complete: module README current (status, ports, phases), doc comment on
every public symbol (purpose, parameters, return value, errors, example where useful), comments
explain why. No new ADR is needed (R-AR-06 and ADR 0009 cover the decision); the rulings go into
`docs/DECISIONS.md`. CHANGELOG entry under "Unreleased" in `services/server/CHANGELOG.md`.
Migration-free. `make check` green. Independent review (`make review` plus a code review over the
diff) and the red team gate (`docs/quality/red-team-gate.md`) over the whole diff with verdict
`Urteil: JA` recorded in the report; findings fixed with a test each and the delta re-checked.
Merge locally into the base branch only if review and red team are green, then
`git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red output of
the first test run), what is not proven, and follow-ups that could not be done in scope with the
reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
