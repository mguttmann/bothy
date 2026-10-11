# M1-01: Server gates in the make contract

Status: ready | Depends on: none | Milestone: M1

## Goal

After this ticket the root `Makefile` carries the gates for Go server code: architecture rules
(`arch`), drift of generated code (`codegen-check`), migration rules (`migrations-lint`) and the
frame of the performance gate (`perf`, deliberately outside `gates`). While no Go module is
tracked under `services/`, each target prints a `SKIP` line; with the first tracked
`services/server/go.mod` (ticket M1-05) they switch on by themselves and fail closed when `go`, a
Docker-compatible runtime or a tool pin is missing. The migration rules of ADR 0007 and chapter
14 exist as a tested tool before the first migration, so no later ticket can merge a migration
that breaks them.

## Read first

- `docs/PLAN.md`: section "Way of working per ticket"; section "M1 Foundation" (Block A, row
  M1-01; "Test strategy"); section "Decisions of this plan" (P-07, P-09, P-10, P-12).
- `docs/spec/23-quality.md`: "Principles", "Gates", "Component contract", "Rulings" (R-QA-01,
  R-QA-02), "Configuration values of this chapter".
- `docs/spec/02-architecture.md`: "Code layout"; "Automatic check of the rules" with its
  subsections "Module register as the source", "go-arch-lint" and "Other checks in the gate";
  "Performance targets"; R-AR-16, R-AR-19, R-AR-20 in "Decisions of this chapter".
- `docs/spec/14-search.md`: subsection "Expressions" (paragraph on `STORED`) and the row
  "Migration lint" of the test table.
- ADRs: `docs/adr/0007-typed-database-access-sqlc-and-migration-tool.md` (rules 2 to 5
  and "Confirmation"), `docs/adr/0008-hexagonal-modules-and-checked-dependency-rules.md`,
  `docs/adr/0038-local-gates-first-one-command-identical-to-ci.md`,
  `docs/adr/0001-adrs-in-madr-format-decisions-as-ruling-log.md` (small rulings go to
  `docs/DECISIONS.md`).
- `docs/quality/gates.md`, `docs/quality/toolchain.md`, `CONTRIBUTING.md` ("Development setup",
  "The gates"), `tools/README.md`, `CHANGELOG.md`, `docs/DECISIONS.md`.
- `Makefile` (whole file; helpers `skip`, `tool`, `want`, variables `GO_MODS`, `GO_ON`, targets
  `test` and `gates`), `tools/bin/fetch_tools.py`, `tools/bin/tools.lock.json`,
  `tools/pyproject.toml`, `tools/docs/tests/test_toolchain_doc.py`,
  `tools/docs/tests/test_config_values.py`, `renovate.json` (custom manager for
  `tools.lock.json`).

This ticket touches no API operation, no table of chapter 03 and no action id.

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
6. Test first: every behavior gets a test that is red before the change (show the red output in
   the report), then green.
7. No en dash (U+2013) and no em dash (U+2014) in any file or commit message. Check every changed
   file with `uv run --project tools --locked --quiet python tools/hooks/text_rules.py <file>` (exit code 0; plain `grep -P` fails on macOS).
8. White label: no company, person or host names, no real domains (only example.com and
   example.org), no real IP addresses, no absolute paths into a home directory, no internal
   system names, in any file including tests and fixtures.
9. Limits, time windows and thresholds are named configuration values; prose and docs use the
   name (for example `quality.perf.timeout`), never a bare number. Runtime values come only from
   keys listed in `docs/operations/configuration.md`; repository tool thresholds live in
   `docs/spec/23-quality.md` "Configuration values of this chapter".
10. A gate skips only while its component has no code; a missing tool, list or prerequisite is an
    error, never a skip (23 "Principles" item 3, R-QA-02).
11. Every tool is pinned with a checksum (23 "Principles" item 4); release binaries are pinned in
    `tools/bin/tools.lock.json` and fetched by `tools/bin/fetch_tools.py`.
12. Module boundaries are checked by go-arch-lint with a configuration generated from
    `architecture/modules.yaml`; any violation exits non-zero (02 "go-arch-lint", R-AR-19).
13. Generated files (`.go-arch-lint.yml`, sqlc code, API code) equal a fresh generation (02
    "Other checks in the gate", row "Generated files current"; ADR 0007 "Confirmation").
14. The base branch carries only sequential, gap-free migration numbers, and a merged migration
    never changes; a checksum file in the gate holds this (ADR 0007 rules 4 and 5,
    "Confirmation").
15. Every `GENERATED ALWAYS AS (...)` column states `STORED`, because PostgreSQL 18 creates
    virtual generated columns by default (14 "Expressions").
16. Statements that cannot run inside a transaction stand in their own migration file with
    `-- +goose NO TRANSACTION` (ADR 0007 rule 3).
17. `make perf` runs at the end of a milestone and in releases, never in `gates` or the pre-push
    hook (P-07). Integration and load tests need a Docker-compatible runtime and fail instead of
    skipping without one (P-09).

## Scope

### 1. Tool pins (`tools/bin/tools.lock.json`)

Add two entries. Keep the key order `version`, `repo`, `tag`, `license`, `binary`, `assets`: the
Renovate regex in `renovate.json` expects `version` and `repo` right after the name.

| name | version | repo | tag | license | binary | asset template | member |
|---|---|---|---|---|---|---|---|
| `go-arch-lint` | `1.19.0` | `fe3dback/go-arch-lint` | `v{version}` | `MIT` | `go-arch-lint` | `go-arch-lint_{version}_<os>_<arch>.tar.gz` | `go-arch-lint` |
| `sqlc` | `1.31.1` | `sqlc-dev/sqlc` | `v{version}` | `MIT` | `sqlc` | `sqlc_{version}_<os>_<arch>.tar.gz` | `sqlc` |

`<os>_<arch>` is `darwin_amd64`, `darwin_arm64`, `linux_amd64`, `linux_arm64` for the platform
keys `darwin-amd64`, `darwin-arm64`, `linux-amd64`, `linux-arm64`. Use the `.tar.gz` assets, never
the `.zip` ones. sha256 values (GitHub release asset digests, read on 2026-10-10):

| tool | platform | sha256 |
|---|---|---|
| go-arch-lint | darwin-amd64 | `09b641e03b5314bfee70e98fa9caf91986a650d28b834713871c8d25867a6285` |
| go-arch-lint | darwin-arm64 | `63a64239de9acdbd0ea3f0bf7b94705cc8a441b02b37c01147d3027e8f78611e` |
| go-arch-lint | linux-amd64 | `b4454546abf95ab1b32c26af146bc7c73ba3856c534d66890ac0738bada5a03c` |
| go-arch-lint | linux-arm64 | `50174a73916c767eda2b27cf18a4db80cd7f102eeb4c514a705239279fd2754b` |
| sqlc | darwin-amd64 | `c5af76772e3785d21663a62697056b383f07629979b1bd25b93872e73dbd519b` |
| sqlc | darwin-arm64 | `21602158c99eb1f2bae197a66abfb1941d1e9e50b23125bb193349c6b1acc71e` |
| sqlc | linux-amd64 | `497ae4fcdfa64c5b0c311ffe4c2bd991e43991e82e5367792ed78bc2dca27354` |
| sqlc | linux-arm64 | `b7cae247740d0c51a1e657479e5b2d21e6fef428f596682a01bc55bf4ab8a23d` |

Verify: `make tools-lock TOOL=go-arch-lint VERSION=1.19.0` and `make tools-lock TOOL=sqlc
VERSION=1.31.1` leave the file unchanged (`git diff --exit-code tools/bin/tools.lock.json`), and
`uv run --project tools --locked python tools/bin/fetch_tools.py path go-arch-lint` (and `sqlc`)
prints a path. These are the only network calls of this ticket besides package downloads.

### 2. `tools/ci/migrations_lint.py` (new, Python standard library only)

CLI (argparse, exit 0 = clean, 1 = findings or refusal, 2 = usage error):

```
migrations_lint.py check --dir <migrations dir> --sum <sum file>
migrations_lint.py lock  --dir <migrations dir> --sum <sum file>
```

Public interface (all with docstrings):

- `@dataclass(frozen=True) class Finding: file: str; line: int | None; rule: str; message: str`
  with `__str__` giving `<file>:<line>: <rule>: <message>` (or `<file>: <rule>: <message>`).
- `def lint(migrations_dir: Path, sum_file: Path) -> list[Finding]`
- `def lock(migrations_dir: Path, sum_file: Path) -> tuple[list[str], list[Finding]]`: returns the
  names it added and the findings that made it refuse.
- `def main(argv: list[str] | None = None) -> int`

Rules are in "Design notes". `lock` first runs every rule except the sum-file rules; if any finding
remains it refuses (exit 1) and writes nothing. Otherwise it adds an entry for every `.sql` file
not yet listed, never changes or removes an existing entry, rewrites the file sorted by name and
prints `locked: <name>` per added file.

### 3. `tools/ci/codegen_check.py` (new)

CLI: `codegen_check.py --dir <path> -- <command> [args...]`. It snapshots every file that git
would see under `<path>` (`git ls-files -z --cached --others --exclude-standard -- .`, run with
`cwd=<path>`; content hashed with sha256), runs the command with `cwd=<path>`, snapshots again and
compares. Public interface: `def snapshot(directory: Path) -> dict[str, str]`,
`def compare(before, after) -> list[str]` (sorted lines `changed: <p>`, `added: <p>`,
`removed: <p>`), `def main(argv) -> int`.

Exit codes: the command's own non-zero exit code if it fails (message `codegen-check: <command>
failed with exit code <n>`); 1 if any file changed, appeared or disappeared (print the lines, then
`codegen-check: generated code in <path> was stale; commit the regenerated files`); 0 otherwise.
Why a snapshot and not `git diff --exit-code`: a developer runs `make check` on a dirty tree, and
an uncommitted hand edit is not drift. Only changes caused by the generator count.

### 4. `tools/ci/go_prereqs.py` (new)

CLI: `go_prereqs.py <gate name>`. Checks, collecting all problems before exiting:

1. `go` is on `PATH` (`shutil.which`); else `<gate>: go is not on PATH; install the version named
   in services/server/go.mod (CONTRIBUTING.md)`.
2. `docker info` succeeds (stdout and stderr discarded, timeout as a module constant); else
   `<gate>: no Docker-compatible runtime is reachable ("docker info" failed); integration and load
   tests need one (docs/PLAN.md P-09)`. A missing `docker` binary gives the same problem.
3. The pins `go-arch-lint` and `sqlc` resolve through `fetch_tools.load_lock`,
   `fetch_tools.platform_key` and `fetch_tools.ensure` (this downloads and verifies on first use).
   A tool absent from the lock gives `<gate>: <tool> is not pinned in tools/bin/tools.lock.json`;
   `LockError` or `FetchError` gives `<gate>: <tool>: <error>`.

Exit 1 with one line per problem, else 0 without output. Make the function testable:
`def check(gate: str, *, which=shutil.which, run=subprocess.run, ensure=fetch_tools.ensure,
lock_path: Path = fetch_tools.DEFAULT_LOCK) -> list[str]`. Import `fetch_tools` from
`tools/bin` (it is on the pytest `pythonpath`; in the script add `tools/bin` to `sys.path` the way
`tools/docs/tests/test_config_values.py` does for its imports, with a comment why).

### 5. `Makefile`

- New variable with a comment naming the value: `PERF_TIMEOUT ?= 60m` (named value
  `quality.perf.timeout`, see section 6).
- `GO_PREREQS := $(UVRUN) python tools/ci/go_prereqs.py`.
- New targets, each with a `## ` help text, added to `.PHONY`:
    - `codegen-check`: without `GO_ON` print `SKIP codegen-check no Go module selected under
      services/`. Else run `$(GO_PREREQS) codegen-check`, then for each module `m` in `GO_MODS`: if
      `$m/sqlc.yaml` exists, `codegen_check.py --dir $m -- <sqlc path> generate -f sqlc.yaml`; then
      always `codegen_check.py --dir $m -- go generate ./...` with `PATH` prefixed by the
      directories of the pinned `go-arch-lint` and `sqlc` binaries, so that `//go:generate`
      directives of later tickets call the pinned versions by name.
    - `arch`: without `GO_ON` print the SKIP line. Else `$(GO_PREREQS) arch`; for each module fail
      with `arch: <m>/.go-arch-lint.yml is missing (generated from <m>/architecture/modules.yaml,
      docs/spec/02-architecture.md "Automatic check of the rules")` if the file is absent, else run
      `<go-arch-lint path> check --project-path <absolute module path> --arch-file
      .go-arch-lint.yml --output-color=false`.
    - `migrations-lint`: without `GO_ON` print the SKIP line. Else `$(GO_PREREQS) migrations-lint`;
      for each module without a `migrations/` directory print `SKIP migrations-lint <m> has no
      migrations/ directory yet` (no migration code means nothing to check, R-QA-02); otherwise run
      `migrations_lint.py check --dir <m>/migrations --sum <m>/migrations.sha256`.
    - `migrations-lock`: runs `migrations_lint.py lock` for each module with a `migrations/`
      directory; not part of `gates`.
    - `perf`: without `GO_ON` print the SKIP line. Else `$(GO_PREREQS) perf`; for each module fail
      with `perf: <m>/perf does not exist; load tests live there (docs/spec/02-architecture.md
      "Performance targets")` if absent; print `  perf: targets hold only on perf.reference_profile`; run
      `go test -tags perf -count=1 -timeout $(PERF_TIMEOUT) ./perf/...` in the module. Not part of
      `gates` (P-07).
- `gates:` gains the three gates in this order:
  `fmt-check codegen-check lint typecheck arch migrations-lint test coverage ...` (rest unchanged).
  Generated code is checked before lint and type checks so that a stale generator output is
  reported as drift, not as a confusing compile error.
- The Go branch of `test` runs `$(GO_PREREQS) test` before the loop, so a missing Docker runtime
  fails with a clear message before any integration test starts (P-09).
- The Go coverage total counts production code only: the `coverage` target removes the lines of
  packages under `<module>/test/` and `<module>/perf/` from `coverage.out` before `go tool cover`
  (02 "Code layout", 23 "Component contract"; they are test code).
- Print SKIP lines with the existing `skip` macro. Where the reason contains a shell variable
  (the per-module skip of `migrations-lint`), use `printf '  SKIP  %-14s %s\n' migrations-lint
  "$$m has no migrations/ directory yet"`, because the macro single-quotes its argument.

### 6. Documentation and named value

- `docs/spec/23-quality.md`: "Gates" table, group "Code": add `codegen-check`, `arch`,
  `migrations-lint`. "Component contract", row "Go module": add `.go-arch-lint.yml` (generated
  from `architecture/modules.yaml`), `migrations/` with `migrations.sha256`, load tests under
  `perf/` with build tag `perf`. "Configuration values of this chapter": new row
  `| \`quality.perf.timeout\` | \`60m\` | duration | Time limit of \`make perf\` per Go module | \`Makefile\` (\`PERF_TIMEOUT\`) |`.
- `tools/docs/tests/test_config_values.py`: add the source
  `"quality.perf.timeout": lambda: _makefile_var("PERF_TIMEOUT")`.
- `docs/quality/gates.md` (English): rows for `codegen-check`, `arch`, `migrations-lint` in "List"
  (column "Active today": "from the first Go module" or "from the first migration"); in
  "Skipping and failing" the server prerequisites (`go`, Docker runtime, pins) and that they apply
  to every server gate and to Go tests; a new section "Performance (`make perf`)" with P-07,
  `quality.perf.timeout`, `perf.reference_profile`, build tag `perf` and the package location
  `<module>/perf/`; a short section "Migrations" with the workflow from "Design notes".
- `docs/quality/toolchain.md`, table "Gate tools": rows
  `| \`go-arch-lint\` | \`1.19.0\` | module boundaries of the Go modules from the module register | MIT | checked in 02 "go-arch-lint" on a skeleton of all modules; checks packages and third-party libraries |`
  and `| \`sqlc\` | \`1.31.1\` | typed Go code from SQL, drift check | MIT | ADR 0007 |`.
  `tools/docs/tests/test_toolchain_doc.py` must stay green.
- `CONTRIBUTING.md` (English): "Development setup" says a Docker-compatible runtime is needed for
  the local SonarQube and for server work (integration tests against PostgreSQL in throwaway
  containers, `make perf`); add a short paragraph "Database migrations" with the workflow.
- `tools/README.md` (English): rows for `ci/migrations_lint.py` (`migrations-lint`,
  `migrations-lock`), `ci/codegen_check.py` (`codegen-check`), `ci/go_prereqs.py` (server gates);
  add go-arch-lint and sqlc to the list in the `bin/` row.
- `CHANGELOG.md`, `[Unreleased]`, `### Added`: one bullet for the server gates.
- `docs/DECISIONS.md`: the rulings listed in "Design notes", in the format of the existing entries
  (date, ruling, reason).

## Out of scope

- Go module, `.go-arch-lint.yml`, generator `tools/archgen`, `.golangci.yml`, Sonar Go settings:
  M1-05.
- Migrations themselves, goose embedding, `bothy migrate`, the PostgreSQL test helper: M1-12.
- Load tests under `perf/`, their thresholds and ADR 0063: M1-16 and M1-72.
- Register check and table-ownership check: M1-06. oapi-codegen and the operation allow-list:
  M1-17. Action catalogue: M1-02. Ops configuration checks: M1-04.
- `.github/workflows/`: no change. `ci.yml` already runs `make gates`, sets up Go when
  `services/server/go.mod` exists, and both runner kinds have a container runtime.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-01-server-gates`.
- Conventional Commits with DCO sign-off (`git commit -s`), scope `gates` or `quality`, for
  example `build(gates): server gates in the Make contract`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `docs/DECISIONS.md` and `CHANGELOG.md` as
  named.
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
  Temporary directories only via `mktemp -d` (or pytest `tmp_path`), removed at the end.
- Never read or print secrets. The launcher provides the private deny list
  (`BOTHY_DENYLIST_FILE`); do not set `DENYLIST_FLAGS=--allow-missing` or `SONAR_SKIP=1`.
- No network calls except package downloads for pinned tools and the two `make tools-lock` runs
  above.
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

### Migration rules (`migrations_lint.py`)

Only regular files ending in `.sql` directly in `--dir` count; other files are ignored (a later
`embed.go` may live there). A subdirectory is a finding `layout`.

| Rule id | Fails when |
|---|---|
| `name` | the file name does not match `^(?P<version>\d+)_(?P<slug>[a-z0-9]+(?:_[a-z0-9]+)*)\.sql$` |
| `timestamp-version` | the version has 14 digits (goose timestamp format); message: number migrations sequentially before the final `make check` (ADR 0007 rule 4) |
| `version-format` | any other version that is not exactly 5 digits (goose sequential format `00001`) |
| `duplicate-version` | two files share a version |
| `version-gap` | the sorted versions are not exactly `1..N` |
| `goose-up` | the file does not contain exactly one line `-- +goose Up` (trailing whitespace ignored) |
| `generated-not-stored` | a `GENERATED ALWAYS AS (` expression is not followed, after its matching `)`, by the keyword `STORED` (case-insensitive); `VIRTUAL` or nothing fails |
| `no-transaction` | a statement contains the keyword `CONCURRENTLY` (except `REFRESH MATERIALIZED VIEW CONCURRENTLY`) and the file lacks a line `-- +goose NO TRANSACTION` |
| `sum-format` | a line of the sum file is not `<64 lowercase hex>  <file name>`, names repeat, or lines are not sorted by name |
| `unlocked-migration` | a `.sql` file has no line in the sum file (hint: `make migrations-lock`) |
| `changed-migration` | the sha256 of a listed file differs from its line (a merged migration never changes; ADR 0007 rule 5) |
| `missing-migration` | a listed file no longer exists |

SQL scanning for `generated-not-stored` and `no-transaction` works on a copy of the text with
line comments (`--` to end of line), nested block comments (`/* */`), single-quoted strings
(with `''`), dollar-quoted strings (`$$...$$`, `$tag$...$tag$`) and double-quoted identifiers
blanked out, preserving line numbers. `GENERATED ALWAYS AS IDENTITY` and `GENERATED BY DEFAULT AS
IDENTITY` never match, because no `(` follows `AS`. goose annotations are read from the raw lines.
Statements are split on `;` after blanking.

Sum file: `<module>/migrations.sha256` next to `migrations/`, format of `shasum -a 256` (two
spaces), sorted by file name, final newline. It lies outside `migrations/` so that neither the
`*.sql` embed pattern nor sqlc reads it.

Workflow for later tickets (goes into `CONTRIBUTING.md` and `docs/quality/gates.md`): create the
next number `NNNNN_<slug>.sql`; run `make migrations-lock`; never edit a migration that is merged
into the base branch, write a new one. Two branches with the same number: the branch merged
second renumbers its file, removes its own sum line and locks again before its final
`make check`. A branch may draft with a timestamp version, but the gate rejects it in every run,
because the local merge happens only after a green `make check`.

### Server prerequisites and SKIP

`GO_ON` (existing) decides SKIP: no tracked `services/*go.mod` or a `COMPONENTS` value without
`server` gives SKIP. With `GO_ON` set, every server gate and the Go branch of `test` first run
`go_prereqs.py`; there is no SKIP for a missing `go`, Docker runtime or pin (R-QA-02). Docker is
required for every server gate, not only for tests, so that a machine that cannot run the
integration tests never reports a partly green server check.

### Rulings for `docs/DECISIONS.md`

1. `migrations-lint` rejects timestamp versions in every run, also on feature branches; branches
   renumber before their final `make check`. This is ADR 0007 rule 4 (no DECISIONS entry needed).
2. Immutability of merged migrations is held by `<module>/migrations.sha256`, written only by
   `make migrations-lock`, which never changes an existing line (ADR 0007 rule 5 and
   "Confirmation"; a diff against a remote base does not exist before the first push).
3. Server gates require `go`, a Docker-compatible runtime and the pins as soon as a Go module is
   tracked. Reason: R-QA-02 and P-09.
4. `codegen-check` detects drift by snapshots before and after the generator instead of
   `git diff`. Reason: hand edits on a dirty tree are not generator drift.
5. `make perf` has the time limit `quality.perf.timeout`. Reason: the default `go test` limit
   would end soak tests early; the value is a named threshold like all others in chapter 23.

## Tests (write them first)

Python tests under `tools/ci/tests/` (pytest, `tmp_path`, no network):

`test_migrations_lint.py` (named in the plan):

- `test_clean_sequence_passes`: `00001_init.sql`, `00002_users.sql` with locked sums, `-- +goose
  Up`, one `GENERATED ALWAYS AS (...) STORED` over several lines with nested parentheses in lower
  case.
- `test_timestamp_version_fails`, `test_five_digit_version_required`, `test_bad_name_fails`,
  `test_duplicate_version_fails`, `test_version_gap_fails`.
- `test_changed_locked_migration_fails`, `test_removed_locked_migration_fails`,
  `test_unlocked_migration_fails`, `test_malformed_or_unsorted_sum_fails`.
- `test_generated_without_stored_fails` (reports the line), `test_generated_virtual_fails`,
  `test_generated_identity_passes` (both identity forms), `test_generated_in_comment_or_string_ignored`
  (line comment, block comment, single-quoted and dollar-quoted strings).
- `test_missing_goose_up_fails`, `test_concurrently_requires_no_transaction` (create index, drop
  index, detach partition), `test_refresh_concurrently_allowed_in_transaction`.
- `test_lock_adds_only_new_entries` (an existing line stays even if its file changed; `check` then
  still fails), `test_lock_refuses_invalid_file`, `test_cli_exit_codes` (0, 1, 2).

`test_codegen_check.py` (temporary git repository from `init_repo` and `commit` in
`tools/testing/git_fixtures.py`, which carry their own identity):

- `test_no_change_passes`, `test_modified_file_fails_and_is_listed`, `test_new_file_fails`,
  `test_deleted_file_fails`, `test_preexisting_dirty_file_is_not_drift`,
  `test_ignored_file_is_not_counted` (`.gitignore`), `test_command_failure_propagates_exit_code`.

`test_go_prereqs.py` (inject `which`, `run`, `ensure`, `lock_path`):

- `test_all_present_passes`, `test_missing_go_fails`, `test_docker_unreachable_fails`,
  `test_docker_binary_missing_fails`, `test_tool_not_pinned_fails`,
  `test_fetch_error_is_reported`, `test_all_problems_reported_together`, `test_main_exit_codes`.

`test_make_server_gates.py`:

- `test_server_gates_skip_without_selected_go_module`: run
  `make -s -C <repo root> codegen-check arch migrations-lint perf COMPONENTS=apps` with an
  environment that drops `MAKEFLAGS` and `MAKELEVEL` (the test itself runs under `make test`);
  expect exit 0 and one `SKIP` line per target.
- `test_gates_contains_server_gates_but_not_perf`: parse the `gates:` prerequisite lines of the
  `Makefile`; `codegen-check`, `arch`, `migrations-lint` are present in the order given above;
  `perf` and `migrations-lock` are absent.

Existing guards that must stay green: `tools/docs/tests/test_toolchain_doc.py` (new rows),
`tools/docs/tests/test_config_values.py` (new named value).

Commands that must pass:

```
uv run --project tools --locked pytest tools/ci/tests/test_migrations_lint.py
uv run --project tools --locked pytest tools/ci/tests tools/docs/tests
make codegen-check arch migrations-lint perf    # today: one SKIP line each, exit 0
make check
```

## Acceptance criteria

- [ ] `pytest tools/ci/tests/test_migrations_lint.py`: timestamp version, duplicate number, changed
      existing migration and `GENERATED ALWAYS AS` without `STORED` fail; a clean sequence passes.
- [ ] `pytest tools/docs/tests/test_toolchain_doc.py` is green with the new pins.
- [ ] Without a selected Go module, `codegen-check`, `arch`, `migrations-lint` and `perf` print
      `SKIP` and exit 0.
- [ ] With a Go module, each of them and the Go branch of `test` fail with a message naming the
      missing piece when `go`, the Docker runtime or a pin is missing (proved by
      `test_go_prereqs.py` and by the recipes calling `go_prereqs.py` first).
- [ ] `gates` contains `codegen-check`, `arch`, `migrations-lint`; `perf` is not in `gates`.
- [ ] `quality.perf.timeout` exists in chapter 23 and equals `PERF_TIMEOUT` (guard test green).
- [ ] `tools/bin/tools.lock.json` pins go-arch-lint 1.19.0 and sqlc 1.31.1; `make tools-lock`
      for both leaves the file unchanged.
- [ ] Docs updated: `docs/quality/gates.md`, `docs/quality/toolchain.md`, `CONTRIBUTING.md`,
      `tools/README.md`, chapter 23, `CHANGELOG.md`, `docs/DECISIONS.md`.
- [ ] `make check` green; no file of the diff contains U+2013 or U+2014.

## Definition of done

Code, tests and docs as above: docstring on every public function and class of the three new
tools; `tools/README.md` rows; no ADR needed (the decisions are in ADR 0007 and 0038; the
implementation rulings go to `docs/DECISIONS.md`); CHANGELOG entry under `[Unreleased]`;
`make check` green. Then `make review` and an independent code review over the diff, then the red
team gate (`docs/quality/red-team-gate.md`) over the diff with verdict recorded in the report. Fix
every finding with a test and re-check the delta. Merge locally into the base branch only if the
review is clean, the red team says `JA` and `make check` is green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits (sha and subject), the files with line counts, the test results with counts (including
the red run before each change), what is not proven (for example: the non-SKIP branches of the
new targets run for real only once M1-05 adds `go.mod`), and follow-ups that could not be done in
scope, each with its reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<merged sha>` or
`exit=1 <reason>`.
