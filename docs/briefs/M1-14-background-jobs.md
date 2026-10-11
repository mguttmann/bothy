# M1-14: Background jobs with River

Status: ready | Depends on: M1-11, M1-13 | Milestone: M1

## Goal

After this ticket `internal/platform/jobs` is the only place in the server that knows River: jobs
are inserted inside the ambient transaction of a unit of work (visible only after commit), run only
in the role `worker` (or `all`), use the five queues from 02, stop softly and then hard on
shutdown, and the River tables live in their own schema `jobs.schema`, migrated by
`bothy migrate`. Modules get a small, River-free interface for job arguments, handlers and
periodic jobs, so a breaking change of River (0.x) touches one package (R-AR-09).

## Read first

- `docs/spec/02-architecture.md`, sections "Background jobs (River)" (all of "Usage",
  "Queues", "Job catalogue", "Rules for jobs"), "Roles of a process", "Commands of the binary",
  "Module list" (closing paragraph on technical tables), "Allowed dependencies"
  (closing rule "River only in `platform/jobs`"), "Layers within a module", "Code layout",
  "go-arch-lint", "What never happens in a transaction", "Panics",
  "Start and graceful shutdown" ("Start", "Health", "Shutdown"), "Open" (PollOnly),
  "Decisions of this chapter" (R-AR-09, R-AR-16), "Configuration values of this chapter",
  "Sources" Q3 to Q7.
- `docs/spec/03-data-model.md`, "Technical tables".
- `docs/spec/16-operations.md`, "Database" ("Roles": `bothy_owner` migrates, `bothy_app` runs).
- `docs/operations/upgrade.md`, "Rules for migrations" (M11: River migrations in the same run,
  before the domain migrations).
- ADR 0007 (rule 8), ADR 0008.
- `docs/observability/signals.md`, "Spans" (`job <bothy.job.kind>`),
  "Attributes in spans and logs" (`bothy.job.kind`, `bothy.outcome`).
- What earlier tickets built: `internal/platform/lifecycle` and `internal/platform/health` (M1-11),
  `internal/platform/pg` with the unit of work, `pg.Tx(ctx)` and `txn.CurrentPhase` (M1-13),
  `cmd/bothy` `migrate` (M1-12), `architecture/modules.yaml` and `tools/archgen` (M1-05).
- River documentation for the pinned version: https://pkg.go.dev/github.com/riverqueue/river
  (`Config.Schema`, `Config.Queues`, `Config.Workers`, `Config.PeriodicJobs`,
  `Config.SoftStopTimeout`, `Client.InsertTx`, `Client.Stop`, `Client.StopAndCancel`,
  `Client.Stopped`; insert-only clients omit `Queues` and never call `Start`),
  https://pkg.go.dev/github.com/riverqueue/river/rivermigrate (`Config.Schema`, `Migrate`,
  `Validate`, `ExistingVersions`), https://riverqueue.com/docs/graceful-shutdown.
  Checked on 2026-10-10 against v0.49.0.
- `docs/PLAN.md` section "Way of working per ticket" and the M1 risk "River is 0.x";
  `docs/quality/gates.md`; `CONTRIBUTING.md`; `docs/quality/red-team-gate.md`.

This ticket touches no API operation, no table of 03 (River tables are technical tables) and no
action id.

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
   and the config schema in code (Ruling L4). No literal limit, timeout or worker count in code
   outside the schema defaults.
10. River only in `internal/platform/jobs`; tables in schema `jobs.schema`, migrated with
    `rivermigrate` inside `bothy migrate`; no River Pro features (02 "Usage", R-AR-09,
    ADR 0007 rule 8).
11. Role `api` uses an insert-only client (never started, no leader election); role `worker` runs
    jobs, periodic jobs and maintenance; role `all` does both (02 "Roles of a process").
12. Jobs are inserted with `InsertTx` in the same transaction as the cause
    (02 "What never happens in a transaction").
13. Job arguments contain only IDs, never content or secrets (02 "Usage").
14. Queues `default`, `files`, `delivery`, `maintenance`, `privacy` with parallelism
    `jobs.queue.<queue>.max_workers` (02 "Queues").
15. Every job is idempotent, passes its context on and returns `ctx.Err()` when cancelled; `nil`
    counts as success in River (02 "Rules for jobs" items 1, 2); chapters with their own retry
    rule set their own backoff (item 4).
16. Shutdown of the worker part: soft stop with `shutdown.jobs_soft_timeout`, then job contexts
    are cancelled; cancelled jobs end with an error and are retried (02 "Shutdown").
17. A panic in a job is reported to River as an error; the process survives (02 "Panics").
18. River migrations run in the same `migrate` run, before the domain migrations (upgrade.md M11).

## Scope

All Go paths are relative to `services/server/`.

1. `internal/platform/jobs` (River, `riverpgxv5`, `rivermigrate` only here; nothing of River in
   its exported API):
    - `type Args interface { Kind() string }`.
    - `type Job[A Args] struct { ID int64; Attempt int; Args A }`.
    - `type Handler[A Args] interface { Work(ctx context.Context, job Job[A]) error }`.
    - `type HandlerOptions struct { Queue string; MaxAttempts int; NextRetry func(attempt int) time.Duration }`
      (`Queue` empty means `default`; `MaxAttempts` 0 and `NextRetry` nil mean the River defaults).
    - `type Registry`; `func NewRegistry() *Registry`;
      `func Register[A Args](r *Registry, h Handler[A], o HandlerOptions) error`;
      `func (r *Registry) AddPeriodic(p PeriodicJob) error` with
      `type PeriodicJob struct { Args Args; Interval time.Duration; RunOnStart bool }`.
    - `type InsertOptions struct { ScheduledAt time.Time }` (zero means now).
    - `type Inserter interface { InsertTx(ctx context.Context, args Args, opts *InsertOptions) error }`:
      uses `pg.Tx(ctx)`; outside a unit of work or in phases `sync`/`audit` it returns an internal
      error; an unregistered kind is an internal error.
    - `type Config struct { Schema string; MaxWorkers map[string]int; SoftStopTimeout time.Duration }`
      filled in `internal/wire` from `jobs.schema`, `jobs.queue.default.max_workers`,
      `jobs.queue.files.max_workers`, `jobs.queue.delivery.max_workers`,
      `jobs.queue.maintenance.max_workers`, `jobs.queue.privacy.max_workers`,
      `shutdown.jobs_soft_timeout`.
    - `type Role` with `RoleAPI`, `RoleWorker`, `RoleAll` (map from `server.role`).
    - `type Runtime`; `func New(pool <pool type>, cfg Config, role Role, reg *Registry, tracer <otel tracer>) (*Runtime, error)`;
      `func (rt *Runtime) Inserter() Inserter`; `func (rt *Runtime) Start(ctx context.Context) error`
      (no-op for `RoleAPI`); `func (rt *Runtime) Stop(ctx context.Context) error` (soft, then
      hard); `func (rt *Runtime) Failed() <-chan error` (closed with an error when the River
      client stops without `Stop`).
    - `func Migrate(ctx context.Context, pool <owner pool>, schema string) error`,
      `func MigrationStatus(ctx, pool, schema) (Status, error)`,
      `func ValidateMigrations(ctx, pool, schema) error`.
    - `func ValidateArgsType(t reflect.Type) error` (exported for the wire test).
    - `README.md`: purpose, role behavior, queues, rules for job authors, River defaults that
      stay in force (fetch polling, job timeout, retry policy) with the River version.
2. `cmd/bothy` `migrate`: `up` creates the schema, sets default privileges and runs River
   migrations before goose; `status` shows the River migration version next to goose; `validate`
   runs the River validation too.
3. `internal/platform/lifecycle`: the worker shutdown step "River soft stop then cancel" in the
   order of 02 "Shutdown" (worker step 3; role `all`: after api steps 1 to 4); start step 8 for
   `worker`; a failure on `rt.Failed()` ends the process with a non-zero exit code.
4. `internal/wire`: one `jobs.Registry` for all roles (no handler registered yet; M1-16 adds the
   first), the `Runtime` per role, config mapping.
5. Architecture gate: in `architecture/modules.yaml` a vendor entry
   `river: [github.com/riverqueue/river, github.com/riverqueue/river/**]` that only the component
   of `internal/platform/jobs` may use; extend `tools/archgen` minimally if it cannot express a
   vendor for one platform package today; regenerate `.go-arch-lint.yml`; add archgen fixtures.
6. `go.mod`: `github.com/riverqueue/river`, `.../riverdriver/riverpgxv5`,
   `.../rivermigrate` at the newest release (v0.49.0 on 2026-10-10), pinned in `go.sum`; license
   MPL-2.0 is on `quality/licenses/allowed.txt`; `make licenses` green.
7. Docs: the span `job <bothy.job.kind>` and the metric `bothy.job.duration` already list the
   `bothy.outcome` values `completed`, `failed`, `cancelled`, `snoozed` in signals.md (use exactly
   these); 02 "Rules for jobs" rules 5 and 6 cover River defaults and timeouts;
   `services/server/README.md` platform table if wording changes; `docs/spec/02-architecture.md`
   "Usage" one sentence on the type check of arguments (allowed field types, see Design notes);
   the README of `platform/jobs` names the River defaults in force with the River version (02
   "Rules for jobs" rule 5); CHANGELOG; DECISIONS.md rows for the rulings below.

## Out of scope

- Any concrete job of the catalogue 02 "Job catalogue" (`sync.partition_maintenance` comes with
  M1-16, the others with their modules).
- Worker consumers of the event log with cursor (ADR 0051, P-13: M2-01).
- Job metrics `bothy.job.duration` and `bothy.job.queue` (M1-68).
- The unit of work itself (M1-13) and the health endpoints themselves (M1-11).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-14-background-jobs`.
- Conventional Commits with scope (for example `feat(jobs): ...`) and DCO sign-off
  (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end. Test containers bind `127.0.0.1` and a random port (P-09, P-10).
- Never read or print secrets. No network calls except package downloads for pinned tools and Go
  modules.

## Design notes

- Clients: `RoleAPI` builds an insert-only River client (no `Queues`, with `Workers` so River
  checks inserted kinds) and never calls `Start`. `RoleWorker` and `RoleAll` build one full client
  with `Queues` (`MaxWorkers` from config), `Workers`, `PeriodicJobs`, `Schema`,
  `SoftStopTimeout = shutdown.jobs_soft_timeout`, and call `Start`.
- Shutdown: `Stop(ctx)` with a deadline of `shutdown.jobs_soft_timeout`; if it does not finish,
  `StopAndCancel(ctx)` so job contexts are cancelled; follow the River graceful shutdown guide.
- Handler wrapper (in `platform/jobs`, around every registered handler):
    1. starts span `job <kind>` (CONSUMER) with `bothy.job.kind` and sets `bothy.outcome`;
    2. recovers a panic into an error (logged with kind and job ID, never arguments);
    3. if the handler returns `nil` but `ctx.Err() != nil`, returns `ctx.Err()` (ruling: a swallowed
       cancel would mark an unfinished job as done; jobs are idempotent, so a rerun is safe);
    4. applies `NextRetry` when set.
- Argument type check (`ValidateArgsType`, run by `Register`, so a bad type fails at startup):
  the args type is a struct; every exported field is `kernel/ids.ID`, `[]kernel/ids.ID` or
  `int64` (sequence numbers and versions); no strings, byte slices, maps, interfaces, pointers or
  nested structs; an empty struct is allowed. Ruling: this is the enforceable form of "only IDs".
- Kind check: `Kind()` matches `^[a-z][a-z_]*\.[a-z][a-z_]*$`, the part before the dot equals
  the module of the args type's package (segment after `/internal/`), and each kind is registered
  once.
- Queue check: `HandlerOptions.Queue` must be one of the five queue names.
- Schema creation without string building (the Semgrep rule `bothy-go-sql-string-building`
  forbids `fmt.Sprintf` in statements): in one transaction as `bothy_owner`,
  `SELECT set_config('bothy.jobs_schema', $1, true)` and then a `DO` block that runs
  `EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', current_setting('bothy.jobs_schema'))` and the
  `ALTER DEFAULT PRIVILEGES FOR ROLE bothy_owner IN SCHEMA %I GRANT ...` statements:
  `SELECT, INSERT, UPDATE, DELETE` on tables, `USAGE, SELECT` on sequences, `EXECUTE` on
  functions to `bothy_app`, plus `GRANT USAGE ON SCHEMA %I TO bothy_app`. Mirror the role names
  and connection handling M1-12 uses in `migrate`.
- Migration order in `migrate up`: schema and privileges, then `rivermigrate` `Migrate(up)` with
  `Config.Schema = jobs.schema`, then goose. Idempotent: a second run changes nothing.
- `jobs.schema` must be a valid unquoted identifier (`^[a-z_][a-z0-9_]{0,62}$`); register this
  cross check in the config schema.
- Configuration (02 "Configuration values of this chapter"): `jobs.schema` (`jobs`),
  `jobs.queue.default.max_workers` (`8`), `jobs.queue.files.max_workers` (`2`),
  `jobs.queue.delivery.max_workers` (`16`), `jobs.queue.maintenance.max_workers` (`2`),
  `jobs.queue.privacy.max_workers` (`1`), `shutdown.jobs_soft_timeout` (`20s`). Register in the
  schema whatever M1-07/M1-11 did not register yet.
- Health: there is no separate check name; an unexpected stop of the River client ends the
  process (rule of 02 "Panics" for components without which the process would be useless), and
  `/readyz` is false during shutdown as M1-11 built it.

## Tests (write them first)

Integration tests use `pgtest` with the roles created by `migrate --bootstrap`; the runtime
connects as `bothy_app`, `migrate` as `bothy_owner`. Test handlers live in test packages under
`internal/platform/jobs` with a test module path segment so the kind check passes (or use a
fixture module directory under `testdata`).

- `TestInsertTx_VisibleOnlyAfterCommit`: insert inside `Do`, block before commit; a second
  connection counts zero rows in `<schema>.river_job`; after commit the worker runs the job.
- `TestInsertTx_RollbackNeverRuns`.
- `TestInsertTx_OutsideUnitFails` and `TestInsertTx_UnregisteredKindFails`.
- `TestInsertTx_ScheduledAt`: the job does not run before `ScheduledAt` (fake times, short
  offsets).
- `TestRoleAPI_DoesNotWorkJobs`: runtime with `RoleAPI`; inserted job stays `available` for
  several River fetch intervals; no leader row from this client.
- `TestRoleWorker_RunsJobAsAppRole`: end to end as `bothy_app` (proves the grants).
- `TestRoleAll_InsertsAndWorks`.
- `TestShutdown_SoftTimeoutCancelsAndRetries`: a job blocks on `ctx.Done()`; `Stop` with a short
  `SoftStopTimeout` from the test config cancels it after the soft timeout; the job row is
  `retryable` with attempt 1 and an error; a new worker runtime runs it again (attempt 2,
  `NextRetry` returns 0 in the test) to completion.
- `TestHandler_NilAfterCancelBecomesError`.
- `TestHandler_PanicIsJobError`: the job is recorded with an error, the process keeps running.
- `TestValidateArgsType`: table with allowed (`ids.ID`, `[]ids.ID`, `int64`, empty struct) and
  rejected (`string`, `[]byte`, `map[string]string`, `any`, pointer, nested struct) fields.
- `TestRegister_KindMustMatchModule`, `TestRegister_UnknownQueue`, `TestRegister_DuplicateKind`.
- `TestWireJobArgsOnlyIDs` (in `internal/wire`): every kind in the real registry passes
  `ValidateArgsType` (empty today, real once M1-16 lands).
- `TestMigrate_RiverInOwnSchema`: with `jobs.schema = jobs_it`, `migrate up` creates
  `jobs_it.river_job` and friends, nothing with prefix `river_` in the domain schema; River
  migrations ran before goose (versions recorded); a second `up` changes nothing; `status` lists
  the River version; `validate` passes.
- `TestFailed_UnexpectedStopEndsProcess`: closing the River client outside `Stop` makes
  `Failed()` deliver an error and the lifecycle exits non-zero (test the lifecycle wiring with a
  fake runtime).
- Archgen fixtures (`go test ./tools/archgen/...`): a package under `internal/platform/pg` and one
  under `internal/<module>/adapters/` importing River are reported; `internal/platform/jobs`
  importing River is clean.

Commands that must pass:

```sh
cd services/server && go test -race -count=1 ./internal/platform/jobs/... ./internal/platform/lifecycle/... ./internal/wire/... ./cmd/...
go test ./tools/archgen/...
make arch
make codegen-check
make licenses
make config-doc-check
make check
```

## Acceptance criteria

- [ ] A job inserted with `InsertTx` is visible only after commit; a rollback leaves nothing.
- [ ] Role `api` never works a job and never joins the leader election.
- [ ] A running job is cancelled after `shutdown.jobs_soft_timeout` and runs again afterwards.
- [ ] go-arch-lint rejects River outside `internal/platform/jobs` (fixture).
- [ ] Job arguments are checked to contain only IDs; a string field fails registration.
- [ ] River tables live only in `jobs.schema`; `migrate up` runs River before goose and is
      idempotent.
- [ ] Queues and parallelism come from `jobs.queue.*.max_workers`.
- [ ] README, signals.md, 02, CHANGELOG, DECISIONS.md updated; `make check` green.

## Definition of done

Code, tests and docs are complete: `internal/platform/jobs/README.md` current (roles, queues,
rules for job authors, River defaults in force), doc comment on every public symbol (purpose,
parameters, return value, errors, example where useful), comments explain why. No new ADR (R-AR-09
and ADR 0007 cover the decisions); rulings go into `docs/DECISIONS.md`. CHANGELOG entry under
"Unreleased" in `services/server/CHANGELOG.md`. `make check` green. Independent review
(`make review` plus a code review over the diff) and the red team gate
(`docs/quality/red-team-gate.md`) over the whole diff with verdict `Urteil: JA` recorded in the
report; findings fixed with a test each and the delta re-checked. Merge locally into the base
branch only if review and red team are green, then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red output of
the first test run), what is not proven, and follow-ups that could not be done in scope with the
reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
