# Gates

`make check` runs all gates in a fixed order and stops at the first failure.
CI runs the same targets: the jobs of section "CI jobs" run `make gates` in parallel parts, job
`denylist` calls `make denylist`, job `sonar` calls `make sonar-scan`; `check` is exactly
`gates denylist sonar` (chapter 23, R-QA-01). `make sonar` is `make test` followed by
`make sonar-scan`. In CI the tests run once per run: each test job packs the coverage reports of
its `make test` (every report path of `sonar-project.properties`) into the artifact
`sonar-coverage-<part>`, and job `sonar` waits for the test jobs, unpacks every artifact and only
scans, with the same quality gate and `sonar.qualitygate.wait=true` (on `main` `false`: that
analysis must not block `main`). When a test job fails,
`sonar` is skipped and `ci-ok` fails through that job. ci.yml also runs on every
push to `main`, so SonarQube keeps an analysis of `main` (its overall state and the reference for
new code; on `main` only with the repository variable `SONAR_MAIN_ANALYSIS`, sonarqube-local.md
section 6); a running analysis of `main` is never cancelled by the next merge, the newest
push waits for it; a push event only comes from this repository's own branch, so fork code never runs
with the secrets (fork pull requests keep the rules of push-discipline.md). Thresholds appear as names; their default values appear only in
chapter 23, section 11.

![Flow from commit to release](pipeline.svg)

## CI jobs

`make gates` runs in seven jobs at the same time, each with every toolchain, the Go, npm, uv and
pub caches and the cache of the pinned binaries (the server tests as a matrix of shards, followed
by `test (server coverage)` with Go alone); together they run every target of `make gates`
once for the components of the plan, the tests split by component
(`tools/ci/tests/test_ci_parallel_gates.py`). `ci-ok` requires each of them (`tools/ci/ci_ok.py`).

| Job | Make targets | Runs |
|---|---|---|
| `gates` | `fmt-check codegen-check lint typecheck arch register-check migrations-lint flag-check commitlint fragments-check runners-check labels-check release-pins install-options` | always |
| `gates (docs and contracts)` | `docs config-doc-check linkcheck diagrams catalog-check api-lint api-checks api-breaking ops-check api-clients tokens` | always |
| `gates (security and supply chain)` | `gitleaks semgrep trivy osv licenses`, after `gradle/actions/wrapper-validation` checks every tracked `gradle-wrapper.jar` | always |
| `test (tools)` | `test coverage` with no component (the Python tools) | always |
| `test (server, shard I of N)` | `test COMPONENTS=server AREAS=none GO_TEST_SHARD=I/N`, one job per shard of a matrix | when the plan needs it |
| `test (server coverage)` | `-o test coverage COMPONENTS=server AREAS=none` on the merged profiles of the shards | when the plan needs it |
| `test (server contract)` | `contract-test COMPONENTS=server AREAS=none`, beside the shards | when the plan needs it |
| `test (apps)` | `test coverage COMPONENTS=apps AREAS=none` (Dart and JVM) | when the plan needs it |
| `test (SDKs and plugins)` | `test coverage` for the planned `sdk-python`, `sdk-typescript`, `plugins` with `AREAS=none` | when the plan needs it |

`AREAS=none` leaves the tools suite to `test (tools)`; only the three tools tests that read
component sources (`TOOLS_SOURCE_TESTS`) run in each test job as well.

The Go tests run in shards at the same time (`tools/ci/go_shards.py`): every package of
`go list ./...` in every Go module is assigned to exactly one shard, balanced by the package
durations in `tools/ci/go_test_timings.json` (a package without a duration weighs the median),
and each shard runs `go test` with the flags of the single run for its packages only. A package
of `SPLITS` in `go_shards.py` (`internal/wire` and `test/agents` in four, `test/terminal` in three) runs in parts instead, each in one shard: its
top-level tests go into groups of about equal measured duration (`<package>#<test>` in the
timings), part J < K runs `-run '^(group J)$'` and the last part `-skip` with the other groups,
so every top-level test runs in exactly one part; in the shard split a part weighs its own
measured duration (`<package>@<J>/<K>`). Two parts of one package share a shard only when there are fewer
shards than parts (PL/55): a part goes to the least loaded of the shards with the fewest parts
of its package. The plan
lists `test-server` for all shards together; the result of the matrix fails when any shard
fails. `test (server coverage)` merges the profiles as one `go test ./...` writes them (one mode
line, then the blocks of every test binary; the same coverage, with fewer repeated count-0
copies), after checking that every shard ran, every package or part ran in exactly
one shard and the filters of the parts run each top-level test once, and runs the coverage threshold on the merged profile; `ci-ok` and `build-verify-ok`
require the shards, `test (server coverage)` and `test (server contract)` (`FOLLOWS` in `tools/ci/ci_ok.py`). To refresh the durations, run
`tools/ci/go_shards.py timings` on the logs of the shards. A shard that runs an e2e package of
the terminal suite (`test/terminal/e2e/...`) first builds the server and host images of the
suite once (`PREBUILD`, `test/terminal/cmd/termsuite-images`), so the builds in their TestMain
are cache hits instead of five to ten minutes each inside the tests.

Time budget (PL/39): in the CI shards of `make test` (`flakes.py go --budget`) a Go test binary (a package, a part of a split package, a rerun)
that runs longer than half of its `-timeout` (`quality.test.go_timeout`) fails with its name and
time (`tools/ci/flakes.py`), so a package that creeps towards its timeout shows up as such and
not as a killed binary that names whichever test ran last. The remedy: split it (`SPLITS`), make
it faster, or refresh the durations when its shard was overloaded.

## Jobs a change needs

One plan decides which jobs a change needs: `tools/ci/components.py ci-plan`, the component map
that the gates (`COMPONENTS`) and the pre-push plan (`tools/hooks/push_plan.py`) use. The job
`plan` of `ci.yml` and of `build-verify.yml` runs it and writes `components` and `jobs`; each
conditional job runs exactly when `jobs` lists its id, and no other condition skips it
(`tools/ci/tests/test_ci_ok.py`). The other jobs (`plan`, the three `gates` jobs, `test (tools)`,
the deny list, `sonar`, `ci-ok`) run on every change: the tools tests read workflows, the
Makefile, docs and manifests, and some build component sources for other platforms (the release
builds of server and host service).

| Jobs | Run when the change touches |
|---|---|
| `test (server)` (all shards, `test (server coverage)` and `test (server contract)`), `test (apps)`, `test (SDKs and plugins)` | their component in the component map (`server`; `apps`; `sdk-python`, `sdk-typescript` or `plugins`) |
| `apps (macOS)`, `apps (iOS)`, `apps (Windows)`, `apps (Android API 26)`, `apps (Android API 36)` | `apps/`, `packages/` (with the native code of the members), the root `pubspec.yaml` (Flutter pin) or `pubspec.lock`; the Android jobs only in the public phase. Other inputs of the component `apps` (`design/`, the SLO generator, `analysis_options.yaml`, `tools/arch-check/`) feed the Dart gates and committed generated files under `packages/`, which `test (apps)` checks |
| `postgres-image (amd64)`, `postgres-image (arm64)` | what `bothy-postgres` is built and tested from: `services/server/images/bothy-postgres/`, `internal/`, `cmd/bothy-backupd/`, `migrations/`, `migrations.sha256`, `test/postgresimage/`, `go.mod`, `go.sum`, `VERSION` and `.golangci.yml` under `services/server/` |
| `desktop trial (macOS)`, `desktop trial (Windows)` | the release steps of the desktop app (brief M7-12): `apps/distribution/`, `tools/distribution/` (but `hostdist.py` and its test), `apps/client/Makefile`, `.github/workflows/release.yml`, the branding tool and example, and the runner files they package or sign (`DISTRIBUTION_FILES` in `tools/ci/components.py`); never for a shared input alone and never on `main`, because each builds the app twice on its own runner (docs/decisions.d/M7-12.md) |
| `host trial (macOS)`, `host trial (Windows)` | the host service and its release steps (brief M7-23): `services/host/`, `infra/host/`, `tools/distribution/` and `.github/workflows/release.yml` (`HOST_TRIAL_JOBS` in `tools/ci/components.py`); never for a shared input alone and never on `main`, because each builds, signs and installs the package on its own runner (docs/decisions.d/M7-23.md) |
| `test (adapters end to end)` | the component `sdk-python` or `plugins` (both own `services/server/`, so a server change runs it): `make adapters-e2e`, the end-to-end suites of the Hermes adapter and the Claude Code runner against the server of this tree (pytest marker `e2e`, left out of `make test`; docs/decisions.d/HERMES-E2E-REAL-SERVER.md) |
| `console-e2e` | the build context of the server image (`services/server/Dockerfile.dockerignore`: `api/`, `cmd/`, `internal/`, `migrations/`, `test/smoke/`, `go.mod`, `go.sum`, `VERSION` and the Dockerfile under `services/server/`; `apps/console/`, `packages/`, the root `pubspec.yaml`, `pubspec.lock` and `analysis_options.yaml`) or the Compose stack of the test (`infra/compose/compose.yaml`, `compose.smoke.yaml`, `compose.console-e2e.yaml`, `smoke.env`, `console-e2e.env` and `console-e2e/` under `infra/compose/test/`) |

Every job but the two desktop trials and the two host trials runs on a push to `main`, on a manual run, for an empty
diff and when a shared input changes: every path the component map does not know (among them the `Makefile`, `.github/`, the
root configuration files and the shared parts of `tools/`) and the API contract `docs/api/`.

Under `tools/` the self-contained tool folders are components of the map
(`TOOL_COMPONENTS` in `tools/ci/components.py`): a change runs the conditional jobs of the
components that read the tool, and the repository-wide gates and `test (tools)` as on every
change. Most tools (`catalog`, `configdoc`, `denylist`, `doccheck`, `docs`, `opscheck`,
`scancode`, `sonarqube`, `testing`, `api-checks`, `tools/README.md`) are read only by
repository-wide gates and the tools tests, so they need no conditional job. Some files select a
component: `tools/api-checks/breaking.py` (`make api-breaking` runs for `server`),
`tools/sonarqube/compose.yaml` (a test of the server reads its image pin),
`tools/contract/` (`make contract-test` runs G7 in the job `test (server contract)`), `tools/lint/`
(`make sql-visibility` checks the queries of the server module) and
`tools/design-tokens/` (`apps` like `design/`, and `server`, whose designkeys generator and tests
read `design/build/tokens.resolved.json`, which also selects `server`). Shared stay the folders every gate depends on
(`SHARED_TOOL_FOLDERS`: `tools/bin/`, `tools/ci/` with the component map, `tools/hooks/`,
`tools/node/` with its lock file), the root files of the tools project (`pyproject.toml`,
`uv.lock`, `conftest.py`) and any tool folder the map does not name yet. A release (`release.yml` calls `ci.yml`)
runs the jobs of the components of its tag. Documentation (`docs/` outside `docs/api/`, `infra/`,
the root Markdown files, `LICENSE`, `NOTICE`) needs no conditional job, except the files the map
assigns to a component (for example `docs/spec/actions.yaml` to `server`, and `infra/compose/` and `infra/otel/` to `server`, whose terminal suite starts that stack, M2-48). The plan reads the diff
without rename detection, so a moved file counts for its old and its new path. A build branch
(`build-verify.yml`) measures from the merge base with `build-base/<ticket>` when the build team
pushed that ref before it (the integration head the ticket branch was built on), else from the
merge base with `main`; without either merge base it runs every job (`tools/ci/build_base.py`;
push-discipline.md, "Build branches").

A skip never hides a failure: `ci-ok` and `build-verify-ok` are the last job of their run, need
every other job and run with `always()`, and `tools/ci/ci_ok.py needs` checks the results of their
`needs` against the plan of the run. A skipped conditional job passes only when the plan did not
list it; a job that ran must pass, listed or not; a missing, empty or unknown plan fails
(docs/DECISIONS.md PL/14, PL/15). `sonar` and the deny-list pair keep their own rules: `sonar` is skipped without
the secrets and after a failed or cancelled test job, which fails `ci-ok` through that job.

A new commit of a pull request cancels the run of the older commit, and so does an edit of title or
body, which starts a new run (the `concurrency` group of `ci.yml` per pull request); a newer push
to a build branch cancels its older `build-verify` run.

Source of the diagram: `docs/quality/pipeline.mmd`.

## List

| make target | Tool | Checks | Threshold | Active today |
|---|---|---|---|---|
| `fmt-check` | ruff format, gofmt, dart format, script `format:check` of the TypeScript projects | format without changes; dart format reads the tracked hand-written `*.dart` files of the workspace members (generated files excluded, docs/DECISIONS.md D-M4-01-4) | | tools |
| `codegen-check` | `tools/ci/codegen_check.py` with sqlc and `go generate` (go-arch-lint and sqlc from the pins at the front of `PATH`; oapi-codegen as a `tool` from `services/server/go.mod`) | generated code (sqlc code, `.go-arch-lint.yml`, the exported configuration schema `architecture/config-schema.json`, API code) matches a fresh generation; only changes by the generator count, not manual changes in the working tree; the hand-kept lists of the durable event types (`KNOWN_EVENT_TYPES` of the TypeScript SDK, the appliers and `skippedDurable` of `packages/sync_engine` with its README) equal the catalogue `docs/spec/actions.yaml` (`tools/catalog/event_copies.py`, for every component selection) | | from the first Go module |
| `lint` | pre-commit hooks over all files (text rules, Stripe webhook secret format `whsec_` of GitHub secret scanning, whitespace, YAML/TOML/JSON, merge markers, private keys, file size), ruff (with required docstrings `D`, section "Documentation in code" in chapter 23), bandit, Dockerfile pin check (`tools/ci/dockerfile_pins.py`: every image of `FROM`, `COPY --from`, `RUN --mount=from` and every `# syntax=` line pinned by `@sha256` digest, except `scratch` and stages; SEC-SC-4), check-jsonschema (issue forms, Renovate, CodeRabbit), actionlint, zizmor, go vet and golangci-lint, `flutter analyze --fatal-infos` once at the root of the Dart workspace (every member, `public_member_api_docs` as an error, generated code excluded from doc and style lint) and custom_lint per member that declares it, script `lint` (ESLint; in `sdk/typescript` then `npm run docs:check`, typedoc with `notDocumented` as an error, so an undocumented export of the hand layer fails, D-M2-30-12); ruff, bandit, Prettier and ESLint skip the generated code of the SDKs, bandit also the project's `.venv`, ShellCheck 0.11.0 (pinned in `tools/bin/tools.lock.json`) over `infra/compose/*.sh` and the scripts of `infra/compose/test/install/` (brief M7-29) and `infra/tofu/**/*.sh` (the host setup of generic-vm, brief M7-15) | style, sources of errors, workflow security | `repo.max_file_kb` | yes |
| `typecheck` | mypy (strict), go build, tsc | types; mypy runs inside each Python project with its configuration, generated SDK code included | | tools |
| `arch` | go-arch-lint with `<module>/.go-arch-lint.yml`, generated from `architecture/modules.yaml`; `tools/arch-check/arch_check.py` with `tools/arch-check/rules.yaml` for the Dart workspace; the counters scans of `services/server/test/countersscan` without the race detector (PL/42) | module boundaries and allowed third-party libraries of the Go modules (chapter 02 "go-arch-lint"); no symbolic links below a module, because go-arch-lint does not follow a linked directory and assigns it by the path of the link, while the Go compiler builds the code behind it (go-arch-lint v1.19.0 walks the module with [`filepath.WalkDir`](https://github.com/fe3dback/go-arch-lint/blob/v1.19.0/internal/services/project/scanner/scanner.go#L74), which [does not follow symbolic links](https://pkg.go.dev/path/filepath#WalkDir), retrieved 2026-10-12; M1-05/24). Dart: the dependency rules 1 to 9 of 15 3.1 for every member (manifests and every import and export, the reach of the diagram as an allow list, native code only in `device_key` and `apps/client`, every manifest a listed member; [README](../../tools/arch-check/README.md)); VZ-T16 `tools/arch-check/no_webview.py`: no WebView package, no `HtmlElementView` or iframe and no native WebView in `apps/client` (docs/spec/25-visualization.md 12, SEC-VZ-1) | | from the first Go module; Dart part with the first Dart package (`SKIP arch-dart` otherwise) |
| `register-check` | `tools/doccheck/register_check.py` | module register `services/server/architecture/modules.yaml` equal to the core contract (tables and their modules from chapter 03, actions, event types and ops from `docs/spec/actions.yaml`) and equal to the places that repeat it: tables "Module list", "Allowed dependencies", "Cross-cutting ports", "Actions per module" and "Event types and WebSocket ops per module" in chapter 02, component diagram `c4-3-server-components.mmd`, module READMEs; one line per difference with file and item (ADR 0050, rules C1 to C12 in the docstring) | | yes |
| `migrations-lint` | `tools/ci/migrations_lint.py` | consecutive five-digit numbers without gaps and without timestamp versions, checksums of merged migrations unchanged (`migrations.sha256`), `STORED` with `GENERATED ALWAYS AS`, `-- +goose NO TRANSACTION` with `CONCURRENTLY` (ADR 0007, chapter 14) | | from the first migration |
| `sql-visibility` | `tools/lint/sql_visibility.py` with `tools/lint/sql_visibility.yaml` | AZ-P12 (07 "Query pattern" rule 1): every reading sqlc query on a content table (`content_tables` of the configuration, each a table heading of chapter 03) carries `channel_id = ANY(<param>)` or `space_id = ANY(<param>)` with a sqlc parameter (on `spaces` and `channels` their key `id`), or stands under `exceptions` with a category (`authorize_resolution`, `after_authorize`, `system_job`, `log_tail`) and a reason; a stale, duplicate or uncategorised exception fails too ([README](../../tools/lint/README.md)) | | from the first Go module |
| `flag-check` | `services/server/tools/flagcheck` | feature flag registration in `internal/instance` valid (key form, unique, kind, description, module from `architecture/modules.yaml`, `remove_by` exactly for `release`); no registered `release` flag whose `remove_by` the version in `services/server/VERSION` has reached; table in [configuration.md](../operations/configuration.md) 7 equal to the registration in key, kind and default ([ADR 0064](../adr/0064-feature-flag-registration-with-a-cleanup-gate.md)) | | once `services/server/go.mod` is tracked |
| `test` | pytest, go test -race -tags contract_validate (G8: every response of every Go integration test is validated against `docs/api/openapi.yaml`, a violation ends the test binary with exit code 3; [contractcheck](../../services/server/internal/platform/httpserver/contractcheck/README.md); G9: every frame of the gateway integration tests is validated by the harness `wstest` against `docs/api/asyncapi.yaml` and `openapi.yaml`, a violation fails the test, M1-67), flutter test (one `flutter pub get --enforce-lockfile` at the workspace root, then `flutter test --coverage` per member), the JVM tests of every Gradle module with a tracked `src/test/` under `packages/*/android/` (`./gradlew -p <module> testDebugUnitTest` in the member's `android/`; needs a JDK 17 and the Android SDK through `ANDROID_HOME`, without them the gate fails), the host compiler tests of the header-only code of the Windows runners (`make runner-test`: every `apps/*/windows/runner/test/run_tests.sh` builds its `*_test.cpp` files with `CXX`, default `c++`, and runs them; FIX-windows-dictation), script `test`; `make install-test`: the tests of the setup wizard `infra/compose/install.sh` (POSIX shell, stubs for `docker`, `curl`, `cosign` and `sudo`) in `debian:trixie-slim` pinned by digest, or on the host with `INSTALL_TEST_NATIVE=1` ([README](../../infra/compose/test/install/README.md)); the terminal conformance suite in the Go part of `test` (`services/server/test/terminal`, M2-48: Compose stacks with the server image built from the worktree and host containers with systemd) needs Docker with `docker buildx`, privileged containers, cgroup version 2 and `kernel.yama.ptrace_scope` of at least `1`; a machine without one of them fails the suite with the fix in the message ([README](../../services/server/test/terminal/README.md)); the agent conformance suite in the Go part of `test` (`services/server/test/agents`, M2-31) starts the fake agent of `sdk/python` through `uv` (`uv` on `PATH`, or the suite fails) ([README](../../services/server/test/agents/README.md)); the check run of the guide `docs/guides/connect-agent.md` in the Go part of `test` (`services/server/test/guide`, M2-37) runs the example adapter of `sdk/python/examples/guide` through `uv` against a fresh server (`uv` on `PATH`, or the check fails) ([README](../../services/server/test/guide/README.md)) | tests with coverage data | | tools |
| `coverage` | coverage, go tool cover, LCOV evaluation | minimum coverage; Go without the packages under `test/` and `perf/` (test code, chapter 23 "Component contract") and without generated files `*.gen.go`; Dart per member without generated files (`tools/ci/lcov_filter.py`: `*.g.dart`, `*.freezed.dart`, `*.drift.dart`, `lib/src/generated/`, `build/`); Python and TypeScript projects without generated code, threshold in the project's own test configuration (pytest-cov `fail_under`, Vitest `coverage.thresholds`) | `quality.coverage.tools_min_percent`, `quality.coverage.go_min_percent`, `quality.coverage.dart_min_percent`, `quality.coverage.python_min_percent`, `quality.coverage.typescript_min_percent` | tools |
| `contract-test` | Go test `TestG7_SchemathesisAllEnabledOperations` (tags `contract_validate`, `contract_live`) with Schemathesis from `tools/contract/` | G7 (06 "Contract tests and gate"): the server of this tree with a throwaway PostgreSQL and the matrix world; every operation of the enable list as owner and as a plain user, positive and negative cases, the stateful phase over inferred links, SEC-T06; built-in checks plus problem format, `Correlation-Id`, no JSON `null`, 404 equality; every failure prints a `curl` reproduction and the seed ([README](../../services/server/test/contract/README.md)) | `quality.contract.max_examples`, `quality.contract.seed`, `quality.contract.timeout` | from the first Go module (Docker runtime) |
| `commitlint` | `tools/hooks/commit_msg.py` | Conventional Commits and DCO of the commits that are not yet on `origin/main`; in CI also the pull request title, and a dependency bot's own commits in its own pull request need no sign-off (`PR_AUTHOR`, D-DEPS-3) | `commit.header_max_length` | from the first remote |
| `fragments-check` | `tools/ci/fragments.py check`, `tools/ci/changelog_summary.py` | changelog and ruling fragments (section "Fragments"): name `<ticket>.md`, place next to a release component's `CHANGELOG.md` or in `docs/decisions.d/`, form; no ticket or integration section in `docs/DECISIONS.md`; in a pull request no line added under "Unreleased" of a `CHANGELOG.md`, against the merge base (the range of `commitlint`); every release section `## [<version>] - <date>` of every `CHANGELOG.md` starts with a `Summary: ` line for users before its first entry, the placeholder of `make release-prep` does not count, `Unreleased` is exempt (M7-28) | | yes |
| `docs` | `tools/docs/language_check.py`, `tools/docs/adr_index.py`, `tools/docs/problem_code_refs.py`, `tools/docs/brief_deps.py`, `tools/docs/register_ids.py`, `tools/docs/risk_ranking.py`, `tools/docs/dart_api_docs.py` (`dart doc` per package into the ignored folder `docs/api/dart/`), MkDocs `--strict` with the hooks `tools/docs/mkdocs_repo_links.py` and `tools/docs/mkdocs_api_reference.py`, then `tools/docs/external_assets_check.py`; `tools/showcase/showcase_cli.py check` (M7-28); `tools/docs/terminal_catalogue.py --check` (M2-48) | ADR index up to date, every pair of status and problem code in Markdown and API files appears as such in `x-problem-codes` (06 section 5 item 7), index, waves and graph of the briefs match the plan and no brief calls a `make` target that only a ticket outside its dependencies creates, every ID that a table in `docs/**/*.md` defines in the first column (rulings, tests, guard rules, threats, decisions, risks, open questions, tickets) appears as such exactly once in the whole repository (D-W-60), every table row has as many cells as its header (D-W-71), the overview and the sections of the open questions name the same IDs, the ranking of the largest risks names the rating of the register and leaves out no higher-rated risk (D-W-78), and the list behind its last rank names exactly the remaining risks with that rank's rating (D-W-79), the site builds without a warning; links to existing files outside `docs/` point to the repository in the site, a missing target stays an error; no tracked text file contains German prose or a word with an umlaut, except the exceptions in `tools/docs/language-allow.txt` and text between the marker lines `language-check: off` and `language-check: on` (D-W-94); the REST, schema and gateway reference pages are rendered from `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` in every build (a committed copy, an unresolved `$ref` or a duplicate anchor fails); every file under `docs/assets/showcase` is listed in its `manifest.json` (script, commit, demo seed) and every entry names an existing file and capture script, names are `<platform>/<[a-z0-9-]+>.png` or `.mp4`, the content is PNG or MP4 and carries no metadata (no text, EXIF, time or ICC chunk of a PNG, no `udta`, `meta`, `uuid` box or creation time of an MP4); the built HTML and CSS reference no script, stylesheet or font of another origin (anchors to external pages stay allowed; requests a bundled script makes at run time are not covered); the terminal catalogue `services/server/test/terminal/catalogue.yaml` equals the test table of 24 section 14, every ID of the PLAN row M2-48 and of M2 exit criterion 8 has a run in `make check` (TM-T16 also one in `make perf`, the `doctor` matrix platform runs on macOS, Windows 10 and Windows 11), every T-TM row of threat model 6.7 names a test that exists and the catalogue lists the threat, and every place the catalogue names exists; `dart doc` of every Dart package without a warning or an error, its pages in the site ([Dart API reference](../reference/dart-api.md)); the typedoc reference of the TypeScript SDK (`npm --prefix sdk/typescript run docs` before MkDocs when `sdk/typescript/package.json` is tracked), mounted under `api/typescript-reference/` by the hook `tools/docs/mkdocs_sdk_reference.py`, which fails when the package is tracked and the reference is missing; built for every component selection, because the site links them | | yes |
| `config-doc-check` | `tools/configdoc/configdoc.py --check` | configuration reference (ruling L4): no name twice, every key of the schema with a row, chapter, place `Server` and the same default in its chapter table, every planned server key with a milestone in `tools/configdoc/planned.yaml` and no stale rule, no name retired by W1 (`tools/configdoc/retired.yaml`), Compose waits longer than `shutdown.total_timeout`, no two names with the same environment variable, section 4 current (rules D1 to D10 in [configuration.md](../operations/configuration.md) 10). Reads the committed schema file and needs no Go | | yes |
| `linkcheck` | lychee `--offline --include-fragments`; `tools/docs/placeholder_links.py` | local links and anchors of all tracked Markdown files; no link outside code to the placeholder repository `OWNER` (P-05), because it leads nowhere after publishing; builds the Dart API pages first when `docs/api/dart/` is missing, because `docs/reference/dart-api.md` links them; the mount of the typedoc reference (`docs/api/typescript-reference/`) is excluded, because it exists only in the site build | | yes |
| `diagrams` | `tools/docs/render_diagrams.py` with mermaid-cli, `tools/docs/erd_check.py` | every Mermaid diagram renders; every `.mmd` has its `.svg` and its `.dark.svg`, rendered with the light and the dark design theme (`design/build/mermaid.config.json`, `design/build/mermaid.dark.config.json`), each stamped with the SHA-256 of `.mmd` and its theme (otherwise stale, D-W-56) and not wider or taller than `MAX_ASPECT`; texts and lines of both renderings meet the contrast gate of the design tokens (`tools/docs/diagram_contrast.py`: theme text colors on their backgrounds and `classDef`/`style` colors on their fill at `text`, line and border colors on the canvas at `ui`, each SVG carries its theme's background and text color); `.mmd` files name only token colors; the ERDs show every table and every foreign key from 03 | | yes |
| `catalog-check` | `tools/catalog/action_catalog.py` | `docs/spec/actions.yaml` valid (schema, unique IDs, rate-limit classes, receive actions, ops without a system action), equal to the hand-maintained parts of chapter 05 (scopes for apps, effect, configuration values of the classes); the generated tables in 05 and lists in `docs/api/redocly.yaml` are current ([ADR 0049](../adr/0049-machine-readable-action-catalogue-as-the-single-source.md)) | | yes |
| `api-lint` | Redocly CLI with `docs/api/redocly.yaml` | OpenAPI and AsyncAPI, requirement of `x-authz-action` | | once `docs/api/openapi.yaml` exists |
| `api-checks` | `tools/api-checks/api_checks.py` | copies of the API contracts equal their sources (chapter 06, G2 and G3): `x-authz-action`, `x-rate-limit-class`, ops, events, receive actions, scopes (`AppScope`) and the union `Event` in `openapi.yaml` and `asyncapi.yaml` equal `docs/spec/actions.yaml`, close codes equal 04, `Idempotency-Key` or `x-idempotency-exempt` with a known reason on every POST and PATCH, deprecation dates against `api.deprecation_min_notice`, `x-problem-codes` and `x-status` complete; the generated tables in 06 and `docs/api/webhooks.md` are current; rule `viz-fixtures`: every fixture of `packages/bothy_viz/fixtures/` is valid against its schema in `openapi.yaml` and every listed schema has a file (25 7.2, VZ-T11) ([README](../../tools/api-checks/README.md)) | | yes |
| `api-breaking` | `tools/api-checks/breaking.py` | G6 (06 13 and 15): no breaking change of `docs/api/openapi.yaml`, its frame and event schemas included, against the same file of the newest stable tag `server/v*` merged into `HEAD` (a pre-release tag only while no stable tag exists); every row of the right column of the 06 13 table is a finding with its location and row; a major step of `info.version` accepts a break; without a tag the gate passes with a notice, offline ([README](../../tools/api-checks/README.md)) | | when the component `server` is selected (a change under `docs/api/` selects it) |
| `ops-check` | `tools/opscheck/opscheck.py` with the pinned validators otelcol-contrib, promtool, amtool, loki, tempo, sloth and tofu ([README](../../tools/opscheck/README.md)) | collector base alone, with each overlay and with every ordered pair of two overlays; Prometheus configuration, rules and rule unit tests (every hand-written alert has one); Alertmanager configuration and routing; Loki; Tempo; OpenTofu modules; generated SLO rules equal to a fresh Sloth generation; SLO table equal to the Sloth files; collector allowlist equal to the signal catalogue; alert thresholds equal to their configuration values; validator versions equal to the image versions of the Compose stack; checks `sli-bounds` (the generated SLI bounds of server and apps equal a fresh run of `tools/observability/sli_bounds.py`) and `sli-labels` (one valid label `bothy_sli_metric` per SLO, one SLO per `sli.*` metric; VZ-T03); check `grafana-unsigned-plugins` (Grafana allows exactly the unsigned plugin `bothy-live-panel`; SEC-VZ-5) | | yes |
| `signal-check` | `tools/signalcheck/signalcheck.py` with the pinned promtool (rule 6; [README](../../tools/signalcheck/README.md)) | Dashboards, rules and SLOs against the signal catalogue (M1-68): the catalogue rows parse, the Prometheus column follows the translation rule, every histogram has exactly one row of bounds and no metric names a person or ID attribute; every metric of a dashboard, rule or Sloth query is a catalogue metric, a recording rule or a built-in series; every label in the braces of a catalogue metric is one of its attributes, a resource attribute, `job`, `instance` or `le`; every `le` threshold is a bucket bound; every dashboard query parses (`promtool check rules`) | | yes |
| `api-clients` | target `generate-check` of each client directory (`sdk/python`, `sdk/typescript`, `packages/api_client`): `tools/ci/codegen_check.py` with the pinned generator, then compileall and mypy or `tsc --noEmit`; for Dart `packages/api_client/tool/generate.sh check`: the pinned `dart-dio` image, `build_runner` and `dart format` in a temporary copy, a byte comparison of `lib/src/generated/` (missing, extra and changed files by path) and `dart analyze` (errors in generated code, any finding in hand-written code) | G4 (06 15): generated clients equal a fresh generation from `docs/api/openapi.yaml` and compile; a directory with a tracked manifest but without `generate-check` fails, a directory without a manifest or an unselected component gives `SKIP` | | from the first client package (today `sdk/python`, `sdk/typescript`, `packages/api_client`) |
| `tokens` | target `check` in `tools/design-tokens/` | tests of the generator; design tokens build, contrast and palette guard checks pass, generated files up to date, embedded files match `vendor/MANIFEST.json` | | yes |
| `runners-check` | `tools/ci/check_runners.py` | Only fixed GitHub-hosted runner labels (ADR 0039), triggers, SHA pinning, timeouts, permissions (no `write-all`, no `actions: write`, also not on a job, D-W-111); environment names only literal (D-W-110); in jobs with environment `release*` in any spelling, outputs of jobs without this environment only in the `if:` of the job or of a step (allow-list D-W-61, D-W-72), artifacts only via `artifact-ids` (D-W-52); matrix values in `runs-on` checked per combination, no runtime matrix behind `runs-on` (D-W-53); release workflows are called only by a job of `release.yml` without a matrix (D-W-73); `push` only on tags, except `denylist-main.yml` on `main` | | yes |
| `release-pins` | `tools/ci/components.py pins` | the server version that the product bundle pins (`BOTHY_VERSION=` in `infra/compose/.env.example`): exactly one line, SemVer without `v` or the placeholder of the template, and a stable product (`VERSION` without a pre-release identifier) pins no server pre-release, which only a product beta (`X.Y.Z-beta.N`) may pin; so a typo fails at the latest in the required check `ci-ok` of the pull request with the release commit, that is before the tag, not only in the build of the release chain; the push of the tag alone starts no check ([18 2.1](../spec/18-release.md#21-server-version-and-registry-in-the-product-bundle), D-W-98) | | yes |
| `install-options` | `tools/ci/install_options_check.py check` | the options of the portability test in `tests/portability/install-options` equal the options after `sh -s --` of the second block of [16 3.1](../spec/16-operations.md#31-installation-in-one-command-setup-wizard) in order, except the value of `--storage` and a `--bundle`; the version in every documented command `https://github.com/<owner>/bothy/releases/download/v<version>/install.sh` of `README.md` and `docs/operations/install-compose.md` equals `VERSION` (brief M7-29 requirement 19); while `VERSION` is `0.0.0` both `README.md` and `docs/index.md` say "No release yet" in their status block, and from the first release on neither does (DOCS-README-HONEST) | | yes |
| `tofu-check` | `tofu` 1.13.1 (pinned in `tools/bin/tools.lock.json`) | `tofu fmt -check` over `infra/tofu`, then `tofu init -backend=false -lockfile=readonly` with the local provider mirror of `tools/bin/tofu_providers.py` as the only source (offline, each archive checked against the lock files) and `tofu validate` of `infra/tofu/test/validate` (a root that calls all four modules), of the test root `infra/tofu/test/generic-vm` (brief M7-15) and of `infra/tofu/test/cloud`, then `tofu test` there: the cloud modules against mocked providers (brief M7-16); the rendered user data of `generic-vm` is checked by `tools/ci/tests/test_tofu_generic_vm.py` and the interface of the cloud modules by `tools/ci/tests/test_tofu_cloud_modules.py` in `test` | | yes |
| `labels-check` | `tools/ci/labels.py` | label file valid, templates and GOVERNANCE.md use only defined labels | | yes |
| `gitleaks` | gitleaks with `.gitleaks.toml` | secrets in every commit reachable from `HEAD`; before going public, in all refs; the EXAMPLE marker exempts only this product's own token formats and generic key-like values | | yes |
| `apps-platform` (not in `gates`) | `tools/ci/apps_platform.py` | the platform tests that a member lists in `platform-tests.json` (argument lists per system, no shell) for the host system, in the member folder; SKIP for the other system and without such a member; the system `android` runs on any host when `adb devices` lists exactly one device or emulator (`SKIP ... android: no device` without one, a failure with two or more; `--system android` runs only it); the system `ios` (the iOS simulator) runs on macOS with `--system ios` or without `--system` (PL/44); a command that stalls while the app starts or whose test file failed to load runs once more (PL/41, PL/44); before each attempt of an Android command it restarts the adb server and waits up to `ANDROID_READY_SECONDS` of `tools/ci/apps_platform.py` for the one device in state `device` with `sys.boot_completed` 1, else that attempt fails with the reason (PL/51), and a failed Android attempt prints the end of the device log (`adb logcat`, cleared before each attempt); needs a desktop session or a device, so CI runs it in the jobs `apps (macOS)` (`macos-15`, `--system macos`), `apps (iOS)` (`macos-15`, `--system ios`), `apps (Windows)` (`windows-2025`), `apps (Android API 26)` and `apps (Android API 36)` (`ubuntu-24.04` with hardware acceleration, x86_64 Google APIs emulator images that `tools/ci/android_sdk.py` installs with retries before the emulator action, PL/18; only in the public phase), which `ci-ok` accepts as `skipped` only when the plan does not need them (section "Jobs a change needs") | | SKIP until the first member has platform tests (M4-10) |
| `terminal-stack` (not in `gates`) | `tools/ci/terminal_stack.py`, the Go helper `services/server/test/terminal/clientstack` and `apps/client/terminal_stack_test/terminal_stack_test.dart` | the app's terminal adapters and `TerminalController` against the terminal stack of M2-48 with one host container: open with the step-up, a command and its output, detach, reattach with the scrollback, `4103` from a second device and `4100` with `access_revoked` after a revocation; the helper removes the stack and fails on a leftover; needs Go, a Docker engine with buildx and the pinned Flutter SDK, so it runs on a developer machine, not in CI | | fails with the preflight message without Docker |
| `demo-e2e` (not in `gates`) | `go test -race -run '^TestDemoViews$'` in `services/server/cmd/bothy/demo` (brief M7-27) | `bothy setup demo` with an exchange directory on an empty harness instance, the demo agent of the Python SDK (`bothy_sdk.testing.demo_agent`) as the agents and the Fake-Host of the terminal tests as the demo host: after the command every view of 25 has data through its query (`getStatsLive`, `getStatsTimeline`, `getStatsSlo`, `getStatsBackends`), a watcher run is `running` with a live box, an approval card waits, the warm-up runs booked their costs and a terminal session opens on the host; the same test also runs in the server shards of CI, the target only names it | | fails without Docker or uv |
| `demo` (not in `gates`) | `infra/compose/demo.sh` with `infra/compose/demo.yaml` (brief M7-27) | not a check: starts the demo instance from the image of `make image`, runs `bothy setup demo` in `api`, pairs the host service container and keeps the demo running until Ctrl-C ([../operations/demo.md](../operations/demo.md)); with `DEMO_CHECK_RESTART=1` it restarts `api` once the demo is ready and fails unless discovery answers again at the address and in the demo agent and the demo host reaches the server on its loopback; `tools/ci/tests/test_compose_demo.py` (tools tests) holds the overlay to one namespace holder that is not `api` and every long-running service to a restart policy | | exit 2 without Docker, buildx, the image or uv |
| `semgrep` | Semgrep with `.semgrep/` | project rules and their rule tests | | yes |
| `trivy` | Trivy `config` on a copy of the tracked and not ignored files (`tools/ci/tracked_tree.py`) | misconfiguration in Dockerfiles, Kubernetes, Helm, Terraform | `HIGH`, `CRITICAL` | once such files exist |
| `osv` | OSV-Scanner, govulncheck; in a pull request through `tools/ci/advisories.py` | known vulnerabilities of all lock files; reachable Go vulnerabilities; a pull request fails only for what it adds or changes (section "Advisories") | | yes |
| `licenses` | `tools/ci/license_check.py`, OSV-Scanner `--licenses` through `tools/ci/dep_licenses.py`, go-licenses | license text, SPDX header lines, licenses of shipped dependencies (license violations only, no advisories: section "Advisories", PL/32); a list without a license is an error (if it were empty, `--licenses=` and `--allowed_licenses=` would no longer restrict the licenses: OSV-Scanner v2.6.0 then prints only a summary, [getters.go](https://github.com/google/osv-scanner/blob/v2.6.0/cmd/osv-scanner/internal/helper/getters.go#L17-L21), and go-licenses v2.0.1 rejects only forbidden and unknown licenses, [check.go](https://github.com/google/go-licenses/blob/v2.0.1/check.go#L64-L68), both retrieved 2026-10-12; M1-05/23) | `quality/licenses/allowed.txt`, `license.spdx_header_lines` | yes (dependencies from the first component) |
| `denylist` | `tools/denylist/denylist_scan.py` | white label in contents, names, commits, identities, refs, and the pull request title and body (`PR_TITLE`, `PR_BODY`, `PR_BODY_FILE`) | | yes, needs the private list |
| `sonar` | SonarScanner (npm) against the local server or SonarQube Cloud | gate "Bothy way" | `quality.sonar.*` | from the first product code |

In addition, the release workflow runs: `trivy-image` (vulnerabilities in the built image), SBOM
(Syft via `anchore/sbom-action`), build provenance and SBOM attestation (public),
`portability` (whole stack on a fresh machine, `make portability` with `PORTABILITY_TAG`, M7-14).

Not gates, but meant for the release (M7) and the Compose smoke test (M1-73): `make image`
builds `bothy-server` from `services/server/Dockerfile` for `IMAGE_PLATFORMS` (default
`linux/amd64,linux/arm64`) into the OCI archive `IMAGE_OCI` and loads the host platform as
`IMAGE_NAME:IMAGE_TAG` (default `bothy-server:<VERSION>-local`, never `latest`); `make image-test`
checks that image with the structure tests in `services/server/test/image` (build tag
`imagetest`: user, health check equal to Compose, no shell, static binary, a run with a read-only
root file system and without capabilities, both platforms in the archive). Both need Docker with
buildx and a builder that builds several platforms (CONTRIBUTING.md); without Docker they fail
with a hint.

`make postgres-image` builds `bothy-postgres` (brief M7-01) from
`services/server/images/bothy-postgres/Dockerfile` the same way into `POSTGRES_IMAGE_OCI` and loads
the host platform as `POSTGRES_IMAGE_NAME:POSTGRES_IMAGE_TAG` (default
`bothy-postgres:pg18-<VERSION>-local`); no build step runs on the target platform, so both
platforms build without emulation. `make postgres-image-test` runs the two-container test of
`services/server/test/postgresimage` (build tag `imagetest`: archive-push over TLS, full backup,
point-in-time restore, `pgbackrest verify`, a foreign client certificate rejected, no egress of the
database container, labels, both platforms). Neither is part of `check`. The CI jobs
`postgres-image (amd64)` and `postgres-image (arm64)` run both targets natively on `ubuntu-24.04`
and `ubuntu-24.04-arm` when the plan needs them (section "Jobs a change needs"); `ci-ok` accepts a
skip only then.

## Skipping and failing

* A gate writes `SKIP <name> <reason>` and counts as passed when the component
  has no code. The Makefile decides this from `git ls-files`: a Go module is a
  tracked `go.mod` under `services/`, a Dart package a `pubspec.yaml` under `apps/`
  or `packages/`, a Python project a `pyproject.toml` under `sdk/python/` or
  `plugins/`, a TypeScript project a `package.json` under `sdk/typescript/` or
  `plugins/`. New files count only after `git add`.
* A missing tool, a missing list, a missing token or a missing
  part of the component contract (chapter 23, section 3) is an error. A Dart package without
  `test/` fails in `make test`, one without `coverage/lcov.info` after the tests in
  `make coverage`.
* CI installs Go as soon as a `services/*/go.mod` is tracked (version from the `go.mod` of the
  server, otherwise of the host service; all modules have the same `go` and `toolchain` lines), and
  Flutter as soon as a Dart package is tracked (version from `environment.flutter` of the root
  `pubspec.yaml` of the Dart workspace; root and every member pin the same exact version).
  `tools/ci/tests/test_ci_toolchains.py` holds both. A change under
  `services/host/testdata/noise/` also selects the component `apps`, because the Dart tests read
  this test data.
* Contributors from forks explicitly switch off what they do not have:
  `DENYLIST_FLAGS=--allow-missing` (public rules only), `SONAR_SKIP=1` and
  `REVIEW_SKIP=1` (no independent review of their own). Everything is visible in the output;
  CI treats fork pull requests the same way (R-QA-02), and also runs that a dependency bot
  (`dependabot[bot]`, `renovate[bot]`) starts on its own pull request: GitHub gives
  Dependabot runs no secrets, and this project treats Renovate the same way
  (`tools/ci/pr_mode.py`, push-discipline.md, section
  [Dependency bot pull requests](push-discipline.md#dependency-bot-pull-requests)); such a
  run shows the notice "Reduced checks". What the fork or the bot run could not check, the
  maintainer catches up on before the merge; after the merge, `denylist-main.yml` also
  checks every push to `main` with the private list.
* `COMPONENTS=<list>` limits the component gates; CI sets it from the changed
  paths. Locally the default is `all`.
* Server prerequisites: as soon as a Go module is selected, all server gates
  (`codegen-check`, `arch`, `migrations-lint`, `perf`) and the Go part of `test` first check
  `go` on the `PATH`, a reachable Docker-compatible runtime (`docker info`) and the
  pins of go-arch-lint and sqlc (`tools/ci/go_prereqs.py`, downloads missing pins and checks
  their sha256). If something is missing, the gate fails with one line per missing part; there
  is no `SKIP` for this (R-QA-02, PLAN P-09). Docker is required for every server gate,
  not only for the tests, so that a host that cannot run the integration tests
  never reports a half-green server state.
  The terminal conformance suite (M2-48) needs more of that runtime: `docker buildx`, privileged
  containers, cgroup version 2 and `kernel.yama.ptrace_scope` of at least `1` on the Docker host;
  its preflight fails with a message that names the fix and never skips. The Docker runtime of the
  GitHub-hosted CI runners (ADR 0039, D-W-92) must therefore start privileged containers on cgroup
  version 2 with `kernel.yama.ptrace_scope` of at least `1`.
* `migrations-lint` skips a Go module with a `SKIP` line as long as it has neither
  `migrations/` nor `migrations.sha256` (no migration, nothing to check). If only
  the checksum file is left, it fails.

## Flakes

A flake is a test that failed in CI and passed when it ran again. Reruns keep a flake from
blocking a pull request or `main`; every one of them is recorded, so the class fixes have a list.
Only CI reruns: GitHub sets `CI=true`, and without it (`make test`, `make check`, the pre-push
gate on a workstation) every test runs once and a failure fails.

What reruns (`tools/ci/flakes.py`, tests `tools/ci/tests/test_flakes.py`):

* Go: every `go test` of `test` (each shard of `test (server)` and the host module), `contract-test`,
  `image-test` and `postgres-image-test` runs through the pinned gotestsum
  (`tools/bin/tools.lock.json`) with `--rerun-fails=2 --rerun-fails-max-failures=10
  --rerun-fails-abort-on-data-race --rerun-fails-run-root-test`. A failed test runs again
  alone as its top-level test (`-run '^Name$'` in its package), at most twice; a package that
  failed without a failed test (TestMain, a check after the tests, an exit outside a test) runs
  again as a whole with the filters of the first run. The flags stay those of the single run
  (`-race`, the tags, `-covermode=atomic -coverpkg=./...`, the timeout), and so does the shard
  split. gotestsum runs `go test -json` through `flakes.py exec` (`--raw-command`), which tests
  only the package of a rerun; the packages are the ones the recipe or `go_shards.py` names.
* Coverage: each attempt writes its own profile, and `coverage.out` is their merge in the form
  `go test ./...` and `go_shards.py merge` write (one mode line, then the blocks of every
  attempt). A statement counts as covered when any attempt ran it, so a run with a rerun has
  the coverage of a run without one; a rerun never replaces the profile of the first run.
* Dart and Flutter: `flutter test` of the pinned Flutter has no `--retry` option, and the global test configuration
  (`DART_TEST_CONFIG`) rejects `retry`. Each member's `dart_test.yaml` therefore defines the
  preset `ci` with `retry: 1`, and `make test` with `CI=true` switches it on through a global
  configuration `add_presets: [ci]`: a failed test runs once more. A member without the preset
  fails in CI ("Undefined preset") and in `test_flakes.py`. The JSON report of
  `flutter test --file-reporter` names the tests that reported an error and then passed.
* Not in the packages a pull request changes (PL/26): `ci.yml` sets `FLAKE_BASE` to the base
  commit of a pull request, and a failed test of a package whose directory (or a `testdata`
  folder below it) has a file the pull request adds, changes or removes against the merge base
  fails the job even when its rerun passed, because the pull request may have made it flaky
  (`FAIL in a package the pull request changes`). It is not listed as a flake. Packages outside
  the change rerun as above; a push to `main` (no `FLAKE_BASE`) reruns every package. When git
  cannot compare with the base (the base commit is not checked out), `flakes.py go` fails
  before the tests, so every job that runs `go test` checks out with `fetch-depth: 0`
  (`test_ci_sets_the_base_of_a_pull_request_and_fetches_it_for_every_go_test`).
* Not rerun: `perf` (not in CI; a budget miss must not pass on a second try), the platform
  suite of the host service (`make -C services/host platform-test`, it changes the machine), the
  JVM tests and the Python and TypeScript tests.

What still fails:

* a test that fails in every attempt, the first run and each rerun;
* more failed tests in one `go test` than `--rerun-fails-max-failures` allows (then nothing
  reruns: that is no flake);
* a data race (`WARNING: DATA RACE`): never rerun, it fails at once;
* a panic, a test timeout, a build error and any other error of `go test` (gotestsum
  stops the reruns);
* a rerun whose package fails in another place (TestMain after the rerun, a new failure).

What a rerun does not catch: a rerun runs only the failed top-level test, alone in its package
(the owner's rule: rerun the failed tests only, not the whole package). A test that fails only
after other tests of its package ran (an order dependency, state that another test leaves behind)
therefore fails in every full run and passes in every rerun: CI passes, and the test is a flake
in every run, so it heads the flake list (`test_an_order_dependent_failure_is_a_flake_of_every_run`).
A check in TestMain that only the other tests trip passes the same way when a test of the same
package flakes in that run. In a package the pull request changes both fail (PL/26); local runs
without `CI=true` keep failing on both. Fix such a test like a flake: the list is the queue.

Where the list is:

* each run prints `FLAKE (<tool>): <package> <test>` lines and writes a table to the job summary;
* each test job uploads its flakes as the artifact `flakes-<job>` (`FLAKE_DIR`, default
  `.cache/flakes/`, JSON with the schema of `flakes.py`; none without a flake);
* the scheduled workflow `flakes.yml` reads the flake artifacts of the CI runs that finished in
  the window its `--hours` names: it adds one comment per run with flakes to the open issue labelled `flake`
  (title "Flaky tests (reruns that passed in CI)", created with the label when there is none) and
  keeps the issue body as the list of every flaky test, the most frequent first. It uses only
  the `GITHUB_TOKEN` (`actions: read`, `issues: write`). Close the issue when its list is fixed;
  the runs it reported stay reported, and the next run with a flake opens a new issue.

## Advisories

A new public advisory makes a dependency vulnerable that `main` and every open pull request
already have. So that it does not stop every landing at once, the advisory gate `osv` of a pull
request fails only for vulnerable dependencies the pull request itself adds or changes (PL/25):

* `ci.yml` sets `OSV_BASE` to the base commit of the pull request in the job
  `gates (security and supply chain)`, and `make osv` then runs `tools/ci/advisories.py`
  (tests `tools/ci/tests/test_advisories.py`) against the merge base of `HEAD` and `OSV_BASE`.
* OSV-Scanner scans the lock files of `HEAD` as in the full scan. When it fails, each failing
  package counts as new unless the same lock file of the merge base has the same package at the
  same version; the base lock files (with the `osv-scanner.toml` files of the base) are scanned
  in the same run, against the same database. An added package, a changed version and a lock
  file new to the pull request fail.
* govulncheck runs per Go module in JSON mode. A called finding counts as new unless the merge
  base, checked out in a temporary worktree, has the same finding (OSV entry, module, version),
  so a new call into a vulnerable dependency fails too. The base runs only when `HEAD` has a
  called finding.
* Findings the merge base has too are printed as warnings. A scanner error, a failing scan
  without a readable finding and a failed scan of the base fail the gate.
* A push to `main` (`ci.yml`) and the daily workflow `advisories.yml` run the full scan without
  `OSV_BASE`; local runs scan in full too. When either fails on a new advisory, the keeper opens
  the update pull request at once.
* The fix is an upgrade of the direct dependency or an npm `overrides` entry to a fixed release
  in the affected lock file. Only when no release that the callers can use has the fix and the
  vulnerable code is not reachable, the `osv-scanner.toml` next to the lock file accepts the
  finding with an `[[IgnoredVulns]]` entry: one advisory `id`, a `reason` that names the
  packages, the reachability argument and a ruling, and an `ignoreUntil` date that forces a new
  check (D-QA-19). `tools/ci/tests/test_osv_ignores.py` refuses an entry without one advisory
  id, a reason or a TOML date as expiry, because OSV-Scanner ignores an entry without
  `ignoreUntil` forever.

`make licenses` judges no advisory (PL/32): OSV-Scanner has no license-only mode and also
exits 1 for a vulnerability when it checks licenses, so `tools/ci/dep_licenses.py` (tests
`tools/ci/tests/test_dep_licenses.py`) reads its JSON report and fails only for license
violations. Otherwise every new public advisory would turn the license step red on every pull
request after `osv` had let the merge base's finding pass.

Trivy runs as `trivy config` with `--skip-check-update` (misconfiguration, no advisory
database) and is not affected; `trivy-image` of the release scans the built images in full.

## Fragments

Every ticket writes its changelog entries and its rulings into files of its own, so that pull
requests that land in parallel do not conflict in one shared file (GitHub's merge ignores git
merge drivers such as `merge=union`; docs/DECISIONS.md PL/24):

* changelog: `<component>/changelog.d/<ticket>.md` next to the `CHANGELOG.md` of the release
  component (`changelog.d/<ticket>.md` at the root for the repository as a whole, the
  directories of `RELEASE_COMPONENTS` in `tools/ci/components.py`). The file holds only Keep a
  Changelog headings `### Added`, `### Changed`, `### Deprecated`, `### Removed`, `### Fixed`
  and `### Security`, each followed by list items (`- `; a continuation line is indented), for
  example the line `### Added`, a blank line and `- Runs and their state machine.`.
  `make release-prep COMPONENT=<name>` puts the entries of the component's fragments at the end
  of their subsection under "Unreleased" (a missing subsection in Keep a Changelog order),
  fragments in file name order with numbers compared as numbers, and deletes the fragments;
  `make changelog-preview COMPONENT=<name>` prints that `CHANGELOG.md` without writing.
* rulings: `docs/decisions.d/<ticket>.md`, first line `# <ticket> <title>`, then the table of
  the ticket's rulings as in docs/DECISIONS.md section 10. The site lists the files on the
  generated page "Ruling log: tickets" (`tools/docs/mkdocs_ruling_fragments.py`); a committed
  `docs/decisions.d/index.md` fails the build.

`make fragments-check` (CI job `gates`) refuses a fragment with another name, place or form, a
section of a ticket or an integration wave in `docs/DECISIONS.md` (`## <ticket> ...`,
`## Integration ...`), and, for the commits of a pull request, a line added under "Unreleased"
of a `CHANGELOG.md`. Edits and deletions of existing lines pass, and so does a release commit,
which empties "Unreleased" into a version section. Not covered: two tickets that change the same
existing line still conflict; a ruling added as a row to a table of `docs/DECISIONS.md` passes
the gate (section 10 asks for the ticket file); and a conflict resolution that drops another
ticket's lines loses them unnoticed, so a branch that still has its rulings in
`docs/DECISIONS.md` moves its section into `docs/decisions.d/<ticket>.md` when it rebases.

The append-only registries that stay shared (every `CHANGELOG.md`, `docs/DECISIONS.md`, every
`migrations.sha256` and `tools/docs/language-allow.txt`) carry `merge=union` in `.gitattributes`
(docs/DECISIONS.md PL/27): when two branches append lines at the same place, a local rebase,
cherry-pick or merge keeps the lines of both sides without a conflict. GitHub ignores the
driver, so a pull request still shows the conflict until its branch is rebuilt locally. A
union does not resolve anything else: when both sides change the same line it keeps both
versions, and two rows that both sides added with the same PL number both stay, so read
the result; two migrations with the same number still fail
`migrations-lint` (`duplicate-version`), and an unsorted sum file is rewritten by
`make migrations-lock`. Code, JSON, YAML and the other Markdown documents keep the normal merge
(`tools/ci/tests/test_merge_union.py` holds the list).

## Performance (`make perf`)

`make perf` runs the load tests of every Go module with performance targets (list `PERF_MODULES`
in the root `Makefile`, today only `services/server`; the host service has no performance targets, 24):
`go test -tags perf -count=1` over `<module>/perf/...`, with the time limit `quality.perf.timeout`
per module. It runs at the
close of every milestone and in the release, never in `gates` and never in the pre-push hook
(PLAN P-07): the performance targets from chapter 02 "Performance targets" apply only on
`perf.reference_profile`; on arbitrary hardware the measurements would not be comparable
(ADR 0063). That is why the output shows the note about `perf.reference_profile` before every
module.

Load tests carry the build tag `perf`, so that `make test` does not run them. Their packages
are under `<module>/perf/` and do not count toward coverage. Without a Go module,
`make perf` reports `SKIP`, as it does without a selected module from `PERF_MODULES`; a module
from `PERF_MODULES` without `perf/` is an error. Like the server gates,
`make perf` needs go, a Docker-compatible runtime and the pins.

The capacity gate (M7-18): `TestPerf_CapacitySteadyThroughput` in `perf/capacity` runs the
scenario of the capacity load test as one steady step at `perf.capacity_steady_messages_per_s`
and fails `make perf` when its goodput (messages answered within `perf.message_create_p99` per
second) falls below `perf.capacity_min_messages_per_s`; both are rows of chapter 02 section 22,
set from the measurement of M7-18 with the margin of `docs/decisions.d/M7-18.md`.

`make load-capacity` (M7-18, [tests/load/README.md](../../tests/load/README.md)) is the capacity
load test itself, not a gate: with the interface of `make portability` it installs the full stack
from the images of a release, ramps people, gateway connections and the message rate in steps
until a performance target of 02 section 17 is missed, and writes the report behind the capacity
table of [16 13](../spec/16-operations.md#13-capacity-and-resources) into `LOAD_REPORT_DIR`. It
runs on the build host against the trial release or on a GitHub-hosted runner, never in `gates`,
`check` or the pre-push hook.

`make bench-search` runs only the search load test `perf/search` (`TestPerf_Search`, M3-16) with
the same tag, time limit and results file and prints the table of its two targets
(`search.p95_target`, `search.prefix_p95_target`); it is the run by hand after a change of the
search, for tuning after a missed target and for the maintenance window of a release that changes
a search vector expression (`docs/operations/upgrade.md` section 4). Like `make perf` it is not in
`gates`, `check` or the pre-push hook.

Terminal (M2-48): TM-T16 runs in `make check` (`services/server/test/terminal/e2e/restart`) and
again under load in `make perf` as `TestChaosTMT16` in `services/server/test/chaos/terminal_test.go`
(build tag `perf`, no `-race`): the scenario file `tm-t16.yaml` on its own Compose stack of the
terminal suite with several parallel sessions that print continuously, through a server restart,
the cut of the device connection and the loss of the host network, then `bothy-host restart`. Its
database is the PostgreSQL of that stack with the image defaults (`fsync` and
`synchronous_commit` on, ADR 0063). It needs the Docker runtime of the terminal suite (section
"Skipping and failing").

Dart members (M6-12): a member with `tool/perf_macos.sh` runs its performance test in profile
mode on macOS, today the live topology of `packages/bothy_viz/harness` against
`viz.perf.frame_build_p95`. On another host `make perf` prints `SKIP perf-dart needs macOS`;
the CI job `apps-macos` runs the same script through `platform-tests.json` (`make
apps-platform`), there against the CI budget `viz.perf.frame_build_p95_ci`, because the
reference budget holds on reference hardware only (`make perf` sets
`BOTHY_PERF_BUDGET=reference`; docs/DECISIONS.md D-M6-12-10). The variable `PERF_HOST` (default `uname -s`) names the host system.

Since M1-72 `make perf` runs the load tests and the chaos tests (`<module>/test/chaos`, same tag)
one package after another (`go test -p 1`), then `perf/internal/report/perfreport`: it prints the
profile mode (`match`, `emulated` or `below`, ADR 0063) and one line per target with value,
threshold, comparison and pass, writes nothing but `.cache/perf/results.json`, and exits non-zero
when a target is missed or missing, the mode is `below`, or a measured database ran without
`fsync` or `synchronous_commit`. A failed chaos test fails `make perf` as well.
`services/server/perf/README.md` describes every test. `make lint` vets and lints the tagged code
(`go vet` and golangci-lint with the tag `perf` next to `contract_validate,contract_live`, and the
tag `sdkserver` of the real server of the Python SDK tests, M2-29), so it cannot rot between
milestones.

## Not in `make check`

These targets check the whole system and run only at their occasion, never in `make check`, `gates`
or the pre-push hook:

| Target | What it checks | When |
|---|---|---|
| `perf` | load tests of the performance targets of 02, the chaos tests of the event path and TM-T16 of the terminal (`TestChaosTMT16`), on `perf.reference_profile` (section above, ADR 0063) | end of a milestone, release, tickets that touch a performance target (PLAN P-07) |
| `portability` | with `PORTABILITY_TAG` the portability test of [16 11.1](../spec/16-operations.md#111-portability-test-in-ci) ([tests/portability/README.md](../../tests/portability/README.md), M7-14): a product release installed with `install.sh` and the options of `tests/portability/install-options` for `fs` and `s3`, every version image verified by its signature before `install.sh`, the running images compared, the smoke scenario of M1-73 and the portability scenario (a run of the test adapter, an upload clean and with EICAR, search, status page), backup and restore test, telemetry through the overlay `local` without a canary, the tailnet case without an account, the digests reported; without `PORTABILITY_TAG` the Compose smoke test (`infra/compose/smoke.sh`) | release workflow (`portability.yml` after `publish` of a product tag, or by hand) |
| `portability-trial` | the portability test end to end against the local trial release of the product (source `release-download`), plus a forced failure that must print state and logs before the cleanup and an unsigned image that must fail before `install.sh`; refuses to run under GitHub Actions (D-W-127) | build host, tickets that change `install.sh`, the bundle, `compose.yaml` or the test |
| `tofu-vm-test` | the OpenTofu module `generic-vm` on a local VM: the product trial, a pinned Ubuntu cloud image in QEMU (KVM, HVF or TCG) with the rendered user data, the smoke probe against the stack, a second delivery over SSH that changes nothing, a wrong checksum and a wrong signature that stop the setup ([infra/tofu/test/generic-vm/README.md](../../infra/tofu/test/generic-vm/README.md)); needs QEMU on the build host and refuses to run under GitHub Actions (D-W-127) | build host, tickets that change `infra/tofu/modules/generic-vm` or the bundle |
| `console-e2e` | SEC-T05: the web console of the server image under its CSP in Chromium, Firefox and WebKit against the Compose smoke stack (`tools/console-e2e`, M6-03) | CI job `console-e2e` of `ci.yml` and `build-verify.yml` when the plan lists it ("Jobs a change needs"), feeding `ci-ok` and `build-verify-ok` |
| `alert-e2e` | 17 10: the alert path end to end against the Compose smoke stack with the profile `observability` (`tools/observability/alert_e2e.py`, M7-20): Alertmanager refuses to start without its URL files, the owner sets up the channel "Operations", app and incoming webhook with step-up, `alert-setup.sh` writes the URLs, a test rule fires and resolves; the channel shows firing and resolved messages with origin `app`, the external stub gets only the `page` alert, the heartbeat stub the Watchdog | before a release and on a change of `infra/observability/alertmanager`, `infra/compose/alert-setup.sh` or the incoming webhooks; locally, needs `make image`; most of its run time is the `group_interval` of Alertmanager before the resolved message |
| `showcase` | `make showcase PLATFORM=<platform>[,...]` (`tools/showcase/capture.py`, M7-28): the demo instance of `make demo` started, the apps captured inside the app through Flutter's image API (`apps/client/e2e_test/showcase`, both themes, on Android also the video of one flow with the emulator's screen recorder), the console and the Grafana panel plugin in headless Chromium, every file stripped of metadata and filed with its manifest entry, the demo removed; refuses a working tree with changes outside `docs/assets/showcase` | by hand on a build host with Docker next to the platform (the Windows app on a host that runs it, D-W-128); after a change of a shown view |
| `e2e-desktop` | 15 20 "E2E on devices": the desktop app against the real server of the Compose stack of `infra/compose/e2e.sh` (installation file, smoke overlay and `test/compose.e2e.yaml` with a TLS front on `127.0.0.1`, the world job and the Fake-Agent), one test per flow of `apps/client/e2e_test` (M4-34): on the host's desktop system through `flutter drive`, headless with `E2E_DEVICE=flutter-tester`; then its terminal part `e2e-desktop-terminal`; the stacks and their temporary data are removed after the run, and the target fails when either part failed | before a release and at the close of M4; locally, needs `make image`, Go, a Docker engine and for the device run a desktop session |
| `e2e-desktop-terminal` | 15 20 and 23.5 row "E2E" (M4-38): `tools/ci/terminal_stack.py --mode e2e` starts the client stack of the terminal (the stack of M2-48 with its TLS front and one host container, installed but not paired; the helper checks that the server knows no host and that only the TLS front publishes a port) and runs `apps/client/e2e_test/terminal_e2e_test.dart`: Alex pairs the host through the administration (the code goes to the helper's loopback endpoint, which runs `bothy-host pair` in the host container of the project), opens a shell with the step-up, runs a command, detaches, reattaches with the scrollback, exits with a code and ends the tab; the canary he typed is in none of the app's paths and none of the stack's logs, database dump, collector export or host journal | with `e2e-desktop`; on its own after a change of the terminal or the client stack |
| `e2e-mobile` | 15 20 "E2E on devices" for the phone apps (M5-11): `tools/e2e/mobile_e2e.py` starts the Compose stack of `infra/compose/e2e.sh` with the overlay `test/compose.e2e-mobile.yaml` (the E2E server build of the Go build tag `e2e`, whose push goes to the fake APNs and FCM providers of the service `fakepush`; their capture endpoint is the only extra published port, on `127.0.0.1`), an iOS simulator it creates and an Android emulator from an AVD (or the ones named by `MOBILE_E2E_UDID` and `MOBILE_E2E_SERIAL`), and runs the `patrol` suites of `apps/client/e2e_test/phone` per platform: login (address, password and TOTP, the device confirmed by a second session), the push permission dialog, a push from Sam that the runner's relay hands to the device (the Android service fetches its text from the real server; the iOS simulator shows the payload's text, its `simctl push` runs no Notification Service Extension), tapping it to the thread, sharing an image from the system share sheet, sending offline with delivery after the reconnect; then the terminal suite of M5-13 against the terminal stack of `services/server/test/terminal/clientstack` (mode `mobile`: a real host service in a container, paired through the API, needs Go): opening from M9 with the step-up, Ctrl+C from the extra keys bar, the reattach with the scrollback after a network loss, no terminal content in the app switcher picture, the reattach within `terminal.quick_reattach_window` without and after it with the step-up; a platform whose tools are missing prints a SKIP line, the target fails when no platform ran; stacks (the host container among them), devices it started and temporary data are removed after the run, also after a failing suite | nightly for Android (`.github/workflows/e2e-mobile.yml`, not a pull request check), both platforms on a macOS host with Docker, Xcode and the Android SDK before a release and at the close of M5; needs `make image` |
| `perf-desktop` | 15 19: the desktop targets `client.perf.*` from `AppConfig` measured in a profile build against the same stack (`apps/client/perf_test`, M4-34), then the terminal targets of `perf-desktop-terminal` added to the same summary; writes `apps/client/build/perf/summary.json` with the device (`PERF_DEVICE_LABEL`, `PERF_REFERENCE=1` on a reference device) and fails when a target is missed | before a release and at the close of M4; on a reference device of 15 19 |
| `perf-desktop-terminal` | 15 19 and 23.5 row "Performance" (M4-38): `client.perf.terminal_frame_to_paint_p95` (a decrypted `data` frame until the rasterization of the first frame that shows its line, p95 over at least 1000 lines of the generator's rate profile) and `client.perf.terminal_throughput_min` (bytes per second of the generator's burst profile, per acknowledged chunk, without a detach) in a profile build with `--dart-define=TERMINAL_PERF_HOOKS=true` against the client stack of the terminal with a paired host (`tools/ci/terminal_stack.py --mode perf`, `apps/client/perf_test/terminal_perf_test.dart`); fails when a target is missed | with `perf-desktop`; on a reference device of 15 19 |

## Migrations

Flow for the migrations of a Go module (ADR 0007, gate `migrations-lint`):

1. Take the next free number of the base branch: `migrations/NNNNN_<slug>.sql`
   (five digits, slug of lowercase letters, digits and single underscores) with exactly
   one line `-- +goose Up`.
2. Call `make migrations-lock`. It enters the checksum of every new file in
   `<module>/migrations.sha256` and never changes or removes an existing line.
3. A migration that is merged in the base branch is never changed or deleted;
   a correction is a new migration.
4. If two branches chose the same number, the branch merged later renumbers its
   file, removes its own line from `migrations.sha256` and calls
   `make migrations-lock` again, before its last `make check`.
5. A draft may carry a timestamp version. The gate rejects it in every run,
   also on feature branches, because the local merge only follows after a green `make check`.

Every column `GENERATED ALWAYS AS (...)` writes `STORED`, because PostgreSQL 18 otherwise creates
a virtual column (chapter 14 "Expressions"). A statement with `CONCURRENTLY` (except
`REFRESH MATERIALIZED VIEW CONCURRENTLY`) stands in its own file with the line
`-- +goose NO TRANSACTION` (ADR 0007, rule 3). Comments, strings and quoted identifiers
do not count for either rule. The checksum file has the format of
`shasum -a 256` (two spaces, sorted by file name, line ending at the end) and is located
next to `migrations/`, so that neither the embedding pattern `*.sql` nor sqlc reads it.

## Hooks

`make bootstrap` installs three hooks from `.pre-commit-config.yaml`:

| Hook | Runs | Content |
|---|---|---|
| pre-commit | on every commit, on the changed files | text rules (no en or em dashes), Stripe webhook secret format `whsec_` (`tools/hooks/secret_formats.py`), file hygiene, ruff for `tools/`, gitleaks on the staged changes |
| commit-msg | on every commit | Conventional Commits and DCO sign-off |
| pre-push | before every push | `tools/hooks/review_gate.py` (report `.review/<sha>.md` of the pushed commit ends with `Urteil: JA`, [red-team-gate.md](red-team-gate.md)), then `make check` for the parts the push touches (`tools/hooks/push_plan.py`, [push-discipline.md](push-discipline.md#affected-only-pre-push)); a push to `build/**` runs only the fast gates and needs no report, because `build-verify.yml` runs every gate ([push-discipline.md](push-discipline.md#build-branches)) |

## Individual gates

Every gate also runs alone, for example `make lint` or `make diagrams`. For the
diagrams, `MMDC_PUPPETEER_CONFIG` can point to a Puppeteer configuration, which the diagram
tests of `make test` use too; a relative path counts from the repository root, also when a test
renders from `tools/`. If it names its own Chromium (`executablePath`), `make bootstrap`
downloads no browser; otherwise it downloads the one that matches Puppeteer. CI sets
`tools/node/puppeteer-ci.json` (Chromium without sandbox, because Ubuntu from 23.10 on keeps the
Chromium of Puppeteer from using user namespaces through AppArmor, [Puppeteer troubleshooting,
"Issues with AppArmor on Ubuntu"](https://pptr.dev/troubleshooting), retrieved 2026-10-12; only
the project's own diagrams are rendered) in every step that reaches
`make diagrams` or `make test`, that is for `make gates` (job `sonar` runs `make sonar-scan`,
which starts no tests); a test in
`tools/doccheck` holds this.

Further switches: `COMMIT_RANGE` and `DENYLIST_RANGE` (commit range instead of
`origin/main..HEAD`), `GITLEAKS_LOG_OPTS` (default `HEAD`, all refs with `--all`), `GITLEAKS_FLAGS` (for
example a JSON report), `DENYLIST_FLAGS` (for example `--reveal` locally).

## What the gates do not cover

* The diagram contrast check measures the colours of the Mermaid theme variables and of the
  `classDef` and `style` lines, not rendered pixels: the fill of a node against the canvas, text
  that Mermaid draws across a line and colours a diagram type takes from variables outside the
  checked list are not measured.
* The showcase check reads names, formats and metadata, not pixels: whether an image shows a
  host, a person or a window title is held by the capture method (inside the app, the page of a
  headless browser, the emulator's screen in demo mode) and by the review of the images; `make
  denylist` reads file names and embedded text chunks, no rendered text (no OCR).
* Links to the internet: `make linkcheck-external` when needed (R-QA-09).
* `register-check` compares only names: the machine columns of the tables in 02, nodes and
  edges of the diagram, lists of the READMEs. Prose ("Responsibility", "Reason per edge", the
  table "Ports, events and sources per module", purpose, port texts and other paragraphs of the
  READMEs) stays with review. It reports a name outside a code span only when a source knows
  it; an unknown word (such as a typo) stays with review. Its static reading approximates
  Python-Markdown (it does not read Setext headings); a test in `make test` holds that it equals
  the rendered site on the real files (DECISIONS M1-06/11). GitHub renders the READMEs as
  GitHub Flavored Markdown ([GFM spec](https://github.github.com/gfm/), retrieved 2026-10-12); what
  only GitHub shows differently from Python-Markdown is not seen by the test. It reads the
  diagram through a closed line grammar; a test with the pinned mermaid-cli holds that it equals
  Mermaid on the real diagram (needs `make bootstrap`).
  Table ownership in SQL files and the alignment with the tables of the migrations (ADR 0050)
  are missing as long as there are no migrations and queries; they come with the first schema
  (PLAN M1-12).
* Whether the content of an `.svg` matches what mermaid-cli would render today from its `.mmd`:
  the output depends on the browser version, so a byte comparison would differ on every
  host. Instead, `make diagrams` checks the stamp that `render_diagrams.py
  --write` writes from `.mmd` and theme (D-W-56); a hand-edited `.svg` with an old
  stamp goes unnoticed.
* actionlint checks shell code in workflows without shellcheck, because shellcheck is not available
  pinned; workflows therefore call only short commands and make targets.
* The equality of local SonarQube and the cloud is checked by `make sonar-config-check`, not by
  every run (sonarqube-local.md).
* `migrations-lint` does not check whether a migration runs on PostgreSQL; that is shown by the
  integration test with all migrations on an empty PostgreSQL 18 (ADR 0007,
  "Confirmation"). `goose validate` checks the files without a database; it comes with
  `bothy migrate validate` (PLAN M1-12) and is not yet part of `migrations-lint`.
* `migrations-lint` compares merged migrations only with `migrations.sha256` in the
  working tree, not with the state of the base branch. Anyone who changes a migration and its line in
  the checksum file by hand at the same time, removes the newest migration together with its line
  or deletes `migrations/` together with `migrations.sha256` passes the gate; the review of the diff
  must catch that.
* GitHub secret scanning has its own patterns
  ([supported patterns](https://docs.github.com/en/code-security/secret-scanning/introduction/supported-secret-scanning-patterns),
  retrieved 2026-10-11): it knows provider formats that gitleaks does not, and its
  `stripe_webhook_signing_secret` type also reports `whsec_` values shorter than the
  gitleaks rule `standard-webhooks-secret` accepts. An example in a third-party format
  can therefore pass the gates and still raise an alert on push; only the `whsec_` format has
  its own check with a wider margin (`tools/hooks/secret_formats.py`). Examples therefore
  stay short of every third-party format (CONTRIBUTING.md).
* `ops-check` checks the files with the same tools and versions as the running components, but
  does not start them. What fails only in operation (a target is unreachable, a secret is wrong, a
  component rejects a value only at start) shows in the portability test. `ops-check` does not
  check Grafana dashboards and their provisioning: `signal-check` (PLAN M1-68) checks the metric
  names and the PromQL of the dashboards, the portability test the provisioning.
* An OpenTofu module must pass `tofu init` offline. `ops-check` and `tofu-check` give `tofu` the
  local mirror of `tools/bin/tofu_providers.py` as the only provider source; before any validator
  runs it holds only the archives the lock files under `infra/tofu/test/` pin, each checked
  against its `zh:` hash (D-M7-16-2), and `ops-check` gives every validator a closed proxy. A
  provider the lock files do not pin and a remote module source therefore fail. Filling the
  mirror needs the network once per version and platform, like the tools of
  `tools/bin/tools.lock.json`.
* That no validator reaches the network is proven by a test only on macOS
  (`test_no_validator_reaches_the_network` in `tools/opscheck/tests/`, with `sandbox-exec`). On
  Linux, so in CI, there is no equivalent lock in the test; there the closed proxy and
  `GCE_METADATA_HOST` work without a proof of their own.
* `otelcol-contrib validate` accepts unknown keys under some exporters (seen on 2026-10-11 with
  0.162.0: `otlp_http`); a typo in an optional key of such an exporter shows only in operation.
