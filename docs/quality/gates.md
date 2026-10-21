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
| `gates` | `fmt-check codegen-check lint typecheck arch register-check migrations-lint flag-check commitlint runners-check labels-check release-pins` | always |
| `gates (docs and contracts)` | `docs config-doc-check linkcheck diagrams catalog-check api-lint api-checks api-breaking ops-check api-clients tokens` | always |
| `gates (security and supply chain)` | `gitleaks semgrep trivy osv licenses` | always |
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
and each shard runs `go test` with the flags of the single run for its packages only. The plan
lists `test-server` for all shards together; the result of the matrix fails when any shard
fails. `test (server coverage)` merges the profiles as one `go test ./...` writes them (one mode
line, then the blocks of every test binary; the same coverage, with fewer repeated count-0
copies), after checking that every shard ran and every package ran in exactly
one shard, and runs the coverage threshold on the merged profile; `ci-ok` and `build-verify-ok`
require the shards, `test (server coverage)` and `test (server contract)` (`FOLLOWS` in `tools/ci/ci_ok.py`). To refresh the durations, run
`tools/ci/go_shards.py timings` on the logs of the shards.

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
| `apps (macOS)`, `apps (Windows)`, `apps (Android API 26)`, `apps (Android API 36)` | `apps/`, `packages/` (with the native code of the members), the root `pubspec.yaml` (Flutter pin) or `pubspec.lock`; the Android jobs only in the public phase. Other inputs of the component `apps` (`design/`, the SLO generator, `analysis_options.yaml`, `tools/arch-check/`) feed the Dart gates and committed generated files under `packages/`, which `test (apps)` checks |
| `postgres-image (amd64)`, `postgres-image (arm64)` | what `bothy-postgres` is built and tested from: `services/server/images/bothy-postgres/`, `internal/`, `cmd/bothy-backupd/`, `migrations/`, `migrations.sha256`, `test/postgresimage/`, `go.mod`, `go.sum`, `VERSION` and `.golangci.yml` under `services/server/` |

Every job runs on a push to `main`, on a manual run, for an empty diff and when a shared input
changes: every path the component map does not know (among them the `Makefile`, `.github/`, the
root configuration files and the shared parts of `tools/`) and the API contract `docs/api/`.

Under `tools/` the self-contained tool folders are components of the map
(`TOOL_COMPONENTS` in `tools/ci/components.py`): a change runs the conditional jobs of the
components that read the tool, and the repository-wide gates and `test (tools)` as on every
change. Most tools (`catalog`, `configdoc`, `denylist`, `doccheck`, `docs`, `opscheck`,
`scancode`, `sonarqube`, `testing`, `api-checks`, `tools/README.md`) are read only by
repository-wide gates and the tools tests, so they need no conditional job. Some files select a
component: `tools/api-checks/breaking.py` (`make api-breaking` runs for `server`),
`tools/sonarqube/compose.yaml` (a test of the server reads its image pin),
`tools/contract/` (`make contract-test` runs G7 in the job `test (server contract)`) and
`tools/design-tokens/` (`apps` like `design/`, and `server`, whose designkeys generator and tests
read `design/build/tokens.resolved.json`, which also selects `server`). Shared stay the folders every gate depends on
(`SHARED_TOOL_FOLDERS`: `tools/bin/`, `tools/ci/` with the component map, `tools/hooks/`,
`tools/node/` with its lock file), the root files of the tools project (`pyproject.toml`,
`uv.lock`, `conftest.py`) and any tool folder the map does not name yet. A release (`release.yml` calls `ci.yml`)
runs the jobs of the components of its tag. Documentation (`docs/` outside `docs/api/`, `infra/`,
the root Markdown files, `LICENSE`, `NOTICE`) needs no conditional job, except the files the map
assigns to a component (for example `docs/spec/actions.yaml` to `server`). The plan reads the diff
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
| `codegen-check` | `tools/ci/codegen_check.py` with sqlc and `go generate` (go-arch-lint and sqlc from the pins at the front of `PATH`; oapi-codegen as a `tool` from `services/server/go.mod`) | generated code (sqlc code, `.go-arch-lint.yml`, the exported configuration schema `architecture/config-schema.json`, API code) matches a fresh generation; only changes by the generator count, not manual changes in the working tree | | from the first Go module |
| `lint` | pre-commit hooks over all files (text rules, Stripe webhook secret format `whsec_` of GitHub secret scanning, whitespace, YAML/TOML/JSON, merge markers, private keys, file size), ruff (with required docstrings `D`, section "Documentation in code" in chapter 23), bandit, Dockerfile pin check (`tools/ci/dockerfile_pins.py`: every image of `FROM`, `COPY --from`, `RUN --mount=from` and every `# syntax=` line pinned by `@sha256` digest, except `scratch` and stages; SEC-SC-4), check-jsonschema (issue forms, Renovate, CodeRabbit), actionlint, zizmor, go vet and golangci-lint, `flutter analyze --fatal-infos` once at the root of the Dart workspace (every member, `public_member_api_docs` as an error, generated code excluded from doc and style lint) and custom_lint per member that declares it, script `lint` (ESLint); ruff, bandit, Prettier and ESLint skip the generated code of the SDKs, bandit also the project's `.venv` | style, sources of errors, workflow security | `repo.max_file_kb` | yes |
| `typecheck` | mypy (strict), go build, tsc | types; mypy runs inside each Python project with its configuration, generated SDK code included | | tools |
| `arch` | go-arch-lint with `<module>/.go-arch-lint.yml`, generated from `architecture/modules.yaml`; `tools/arch-check/arch_check.py` with `tools/arch-check/rules.yaml` for the Dart workspace | module boundaries and allowed third-party libraries of the Go modules (chapter 02 "go-arch-lint"); no symbolic links below a module, because go-arch-lint does not follow a linked directory and assigns it by the path of the link, while the Go compiler builds the code behind it (go-arch-lint v1.19.0 walks the module with [`filepath.WalkDir`](https://github.com/fe3dback/go-arch-lint/blob/v1.19.0/internal/services/project/scanner/scanner.go#L74), which [does not follow symbolic links](https://pkg.go.dev/path/filepath#WalkDir), retrieved 2026-10-12; M1-05/24). Dart: the dependency rules 1 to 9 of 15 3.1 for every member (manifests and every import and export, the reach of the diagram as an allow list, native code only in `device_key` and `apps/client`, every manifest a listed member; [README](../../tools/arch-check/README.md)) | | from the first Go module; Dart part with the first Dart package (`SKIP arch-dart` otherwise) |
| `register-check` | `tools/doccheck/register_check.py` | module register `services/server/architecture/modules.yaml` equal to the core contract (tables and their modules from chapter 03, actions, event types and ops from `docs/spec/actions.yaml`) and equal to the places that repeat it: tables "Module list", "Allowed dependencies", "Cross-cutting ports", "Actions per module" and "Event types and WebSocket ops per module" in chapter 02, component diagram `c4-3-server-components.mmd`, module READMEs; one line per difference with file and item (ADR 0050, rules C1 to C12 in the docstring) | | yes |
| `migrations-lint` | `tools/ci/migrations_lint.py` | consecutive five-digit numbers without gaps and without timestamp versions, checksums of merged migrations unchanged (`migrations.sha256`), `STORED` with `GENERATED ALWAYS AS`, `-- +goose NO TRANSACTION` with `CONCURRENTLY` (ADR 0007, chapter 14) | | from the first migration |
| `flag-check` | `services/server/tools/flagcheck` | feature flag registration in `internal/instance` valid (key form, unique, kind, description, module from `architecture/modules.yaml`, `remove_by` exactly for `release`); no registered `release` flag whose `remove_by` the version in `services/server/VERSION` has reached; table in [configuration.md](../operations/configuration.md) 7 equal to the registration in key, kind and default ([ADR 0064](../adr/0064-feature-flag-registration-with-a-cleanup-gate.md)) | | once `services/server/go.mod` is tracked |
| `test` | pytest, go test -race -tags contract_validate (G8: every response of every Go integration test is validated against `docs/api/openapi.yaml`, a violation ends the test binary with exit code 3; [contractcheck](../../services/server/internal/platform/httpserver/contractcheck/README.md); G9: every frame of the gateway integration tests is validated by the harness `wstest` against `docs/api/asyncapi.yaml` and `openapi.yaml`, a violation fails the test, M1-67), flutter test (one `flutter pub get --enforce-lockfile` at the workspace root, then `flutter test --coverage` per member), the JVM tests of every Gradle module with a tracked `src/test/` under `packages/*/android/` (`./gradlew -p <module> testDebugUnitTest` in the member's `android/`; needs a JDK 17 and the Android SDK through `ANDROID_HOME`, without them the gate fails), script `test` | tests with coverage data | | tools |
| `coverage` | coverage, go tool cover, LCOV evaluation | minimum coverage; Go without the packages under `test/` and `perf/` (test code, chapter 23 "Component contract") and without generated files `*.gen.go`; Dart per member without generated files (`tools/ci/lcov_filter.py`: `*.g.dart`, `*.freezed.dart`, `*.drift.dart`, `lib/src/generated/`, `build/`); Python and TypeScript projects without generated code, threshold in the project's own test configuration (pytest-cov `fail_under`, Vitest `coverage.thresholds`) | `quality.coverage.tools_min_percent`, `quality.coverage.go_min_percent`, `quality.coverage.dart_min_percent`, `quality.coverage.python_min_percent`, `quality.coverage.typescript_min_percent` | tools |
| `contract-test` | Go test `TestG7_SchemathesisAllEnabledOperations` (tags `contract_validate`, `contract_live`) with Schemathesis from `tools/contract/` | G7 (06 "Contract tests and gate"): the server of this tree with a throwaway PostgreSQL and the matrix world; every operation of the enable list as owner and as a plain user, positive and negative cases, the stateful phase over inferred links, SEC-T06; built-in checks plus problem format, `Correlation-Id`, no JSON `null`, 404 equality; every failure prints a `curl` reproduction and the seed ([README](../../services/server/test/contract/README.md)) | `quality.contract.max_examples`, `quality.contract.timeout` | from the first Go module (Docker runtime) |
| `commitlint` | `tools/hooks/commit_msg.py` | Conventional Commits and DCO of the commits that are not yet on `origin/main`; in CI also the pull request title, and a dependency bot's own commits in its own pull request need no sign-off (`PR_AUTHOR`, D-DEPS-3) | `commit.header_max_length` | from the first remote |
| `docs` | `tools/docs/language_check.py`, `tools/docs/adr_index.py`, `tools/docs/problem_code_refs.py`, `tools/docs/brief_deps.py`, `tools/docs/register_ids.py`, `tools/docs/risk_ranking.py`, `tools/docs/dart_api_docs.py` (`dart doc` per package into the ignored folder `docs/api/dart/`), MkDocs `--strict` with the hooks `tools/docs/mkdocs_repo_links.py` and `tools/docs/mkdocs_api_reference.py`, then `tools/docs/external_assets_check.py` | ADR index up to date, every pair of status and problem code in Markdown and API files appears as such in `x-problem-codes` (06 section 5 item 7), index, waves and graph of the briefs match the plan and no brief calls a `make` target that only a ticket outside its dependencies creates, every ID that a table in `docs/**/*.md` defines in the first column (rulings, tests, guard rules, threats, decisions, risks, open questions, tickets) appears as such exactly once in the whole repository (D-W-60), every table row has as many cells as its header (D-W-71), the overview and the sections of the open questions name the same IDs, the ranking of the largest risks names the rating of the register and leaves out no higher-rated risk (D-W-78), and the list behind its last rank names exactly the remaining risks with that rank's rating (D-W-79), the site builds without a warning; links to existing files outside `docs/` point to the repository in the site, a missing target stays an error; no tracked text file contains German prose or a word with an umlaut, except the exceptions in `tools/docs/language-allow.txt` and text between the marker lines `language-check: off` and `language-check: on` (D-W-94); the REST, schema and gateway reference pages are rendered from `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` in every build (a committed copy, an unresolved `$ref` or a duplicate anchor fails); the built HTML and CSS reference no script, stylesheet or font of another origin (anchors to external pages stay allowed; requests a bundled script makes at run time are not covered); `dart doc` of every Dart package without a warning or an error, its pages in the site ([Dart API reference](../reference/dart-api.md)); built for every component selection, because the site links them | | yes |
| `config-doc-check` | `tools/configdoc/configdoc.py --check` | configuration reference (ruling L4): no name twice, every key of the schema with a row, chapter, place `Server` and the same default in its chapter table, every planned server key with a milestone in `tools/configdoc/planned.yaml` and no stale rule, no name retired by W1 (`tools/configdoc/retired.yaml`), Compose waits longer than `shutdown.total_timeout`, no two names with the same environment variable, section 4 current (rules D1 to D10 in [configuration.md](../operations/configuration.md) 10). Reads the committed schema file and needs no Go | | yes |
| `linkcheck` | lychee `--offline --include-fragments`; `tools/docs/placeholder_links.py` | local links and anchors of all tracked Markdown files; no link outside code to the placeholder repository `OWNER` (P-05), because it leads nowhere after publishing; builds the Dart API pages first when `docs/api/dart/` is missing, because `docs/reference/dart-api.md` links them | | yes |
| `diagrams` | `tools/docs/render_diagrams.py` with mermaid-cli, `tools/docs/erd_check.py` | every Mermaid diagram renders; every `.mmd` has its `.svg`, rendered with the design theme (`design/build/mermaid.config.json`), stamped with the SHA-256 of `.mmd` and theme (otherwise stale, D-W-56) and not wider or taller than `MAX_ASPECT`; `.mmd` files name only token colors; the ERDs show every table and every foreign key from 03 | | yes |
| `catalog-check` | `tools/catalog/action_catalog.py` | `docs/spec/actions.yaml` valid (schema, unique IDs, rate-limit classes, receive actions, ops without a system action), equal to the hand-maintained parts of chapter 05 (scopes for apps, effect, configuration values of the classes); the generated tables in 05 and lists in `docs/api/redocly.yaml` are current ([ADR 0049](../adr/0049-machine-readable-action-catalogue-as-the-single-source.md)) | | yes |
| `api-lint` | Redocly CLI with `docs/api/redocly.yaml` | OpenAPI and AsyncAPI, requirement of `x-authz-action` | | once `docs/api/openapi.yaml` exists |
| `api-checks` | `tools/api-checks/api_checks.py` | copies of the API contracts equal their sources (chapter 06, G2 and G3): `x-authz-action`, `x-rate-limit-class`, ops, events, receive actions, scopes (`AppScope`) and the union `Event` in `openapi.yaml` and `asyncapi.yaml` equal `docs/spec/actions.yaml`, close codes equal 04, `Idempotency-Key` or `x-idempotency-exempt` with a known reason on every POST and PATCH, deprecation dates against `api.deprecation_min_notice`, `x-problem-codes` and `x-status` complete; the generated tables in 06 and `docs/api/webhooks.md` are current; rule `viz-fixtures`: every fixture of `packages/bothy_viz/fixtures/` is valid against its schema in `openapi.yaml` and every listed schema has a file (25 7.2, VZ-T11) ([README](../../tools/api-checks/README.md)) | | yes |
| `api-breaking` | `tools/api-checks/breaking.py` | G6 (06 13 and 15): no breaking change of `docs/api/openapi.yaml`, its frame and event schemas included, against the same file of the newest stable tag `server/v*` merged into `HEAD` (a pre-release tag only while no stable tag exists); every row of the right column of the 06 13 table is a finding with its location and row; a major step of `info.version` accepts a break; without a tag the gate passes with a notice, offline ([README](../../tools/api-checks/README.md)) | | when the component `server` is selected (a change under `docs/api/` selects it) |
| `ops-check` | `tools/opscheck/opscheck.py` with the pinned validators otelcol-contrib, promtool, amtool, loki, tempo, sloth and tofu ([README](../../tools/opscheck/README.md)) | collector base alone, with each overlay and with every ordered pair of two overlays; Prometheus configuration, rules and rule unit tests (every hand-written alert has one); Alertmanager configuration and routing; Loki; Tempo; OpenTofu modules; generated SLO rules equal to a fresh Sloth generation; SLO table equal to the Sloth files; collector allowlist equal to the signal catalogue; alert thresholds equal to their configuration values; validator versions equal to the image versions of the Compose stack; checks `sli-bounds` (the generated SLI bounds of server and apps equal a fresh run of `tools/observability/sli_bounds.py`) and `sli-labels` (one valid label `bothy_sli_metric` per SLO, one SLO per `sli.*` metric; VZ-T03) | | yes |
| `signal-check` | `tools/signalcheck/signalcheck.py` with the pinned promtool (rule 6; [README](../../tools/signalcheck/README.md)) | Dashboards, rules and SLOs against the signal catalogue (M1-68): the catalogue rows parse, the Prometheus column follows the translation rule, every histogram has exactly one row of bounds and no metric names a person or ID attribute; every metric of a dashboard, rule or Sloth query is a catalogue metric, a recording rule or a built-in series; every label in the braces of a catalogue metric is one of its attributes, a resource attribute, `job`, `instance` or `le`; every `le` threshold is a bucket bound; every dashboard query parses (`promtool check rules`) | | yes |
| `api-clients` | target `generate-check` of each client directory (`sdk/python`, `sdk/typescript`, `packages/api_client`): `tools/ci/codegen_check.py` with the pinned generator, then compileall and mypy or `tsc --noEmit`; for Dart `packages/api_client/tool/generate.sh check`: the pinned `dart-dio` image, `build_runner` and `dart format` in a temporary copy, a byte comparison of `lib/src/generated/` (missing, extra and changed files by path) and `dart analyze` (errors in generated code, any finding in hand-written code) | G4 (06 15): generated clients equal a fresh generation from `docs/api/openapi.yaml` and compile; a directory with a tracked manifest but without `generate-check` fails, a directory without a manifest or an unselected component gives `SKIP` | | from the first client package (today `sdk/python`, `sdk/typescript`, `packages/api_client`) |
| `tokens` | target `check` in `tools/design-tokens/` | tests of the generator; design tokens build, contrast and palette guard checks pass, generated files up to date, embedded files match `vendor/MANIFEST.json` | | yes |
| `runners-check` | `tools/ci/check_runners.py` | Only fixed GitHub-hosted runner labels (ADR 0039), triggers, SHA pinning, timeouts, permissions (no `write-all`, no `actions: write`, also not on a job, D-W-111); environment names only literal (D-W-110); in jobs with environment `release*` in any spelling, outputs of jobs without this environment only in the `if:` of the job or of a step (allow-list D-W-61, D-W-72), artifacts only via `artifact-ids` (D-W-52); matrix values in `runs-on` checked per combination, no runtime matrix behind `runs-on` (D-W-53); release workflows are called only by a job of `release.yml` without a matrix (D-W-73); `push` only on tags, except `denylist-main.yml` on `main` | | yes |
| `release-pins` | `tools/ci/components.py pins` | the server version that the product bundle pins (`BOTHY_VERSION=` in `infra/compose/.env.example`): exactly one line, SemVer without `v` or the placeholder of the template, and a stable product (`VERSION` without a pre-release identifier) pins no server pre-release, which only a product beta (`X.Y.Z-beta.N`) may pin; so a typo fails at the latest in the required check `ci-ok` of the pull request with the release commit, that is before the tag, not only in the build of the release chain; the push of the tag alone starts no check ([18 2.1](../spec/18-release.md#21-server-version-and-registry-in-the-product-bundle), D-W-98) | | yes |
| `labels-check` | `tools/ci/labels.py` | label file valid, templates and GOVERNANCE.md use only defined labels | | yes |
| `gitleaks` | gitleaks with `.gitleaks.toml` | secrets in every commit reachable from `HEAD`; before going public, in all refs; the EXAMPLE marker exempts only this product's own token formats and generic key-like values | | yes |
| `apps-platform` (not in `gates`) | `tools/ci/apps_platform.py` | the platform tests that a member lists in `platform-tests.json` (argument lists per system, no shell) for the host system, in the member folder; SKIP for the other system and without such a member; the system `android` runs on any host when `adb devices` lists exactly one device or emulator (`SKIP ... android: no device` without one, a failure with two or more; `--system android` runs only it); needs a desktop session or a device, so CI runs it in the jobs `apps (macOS)` (`macos-15`), `apps (Windows)` (`windows-2025`), `apps (Android API 26)` and `apps (Android API 36)` (`ubuntu-24.04` with hardware acceleration, x86_64 Google APIs emulator images that `tools/ci/android_sdk.py` installs with retries before the emulator action, PL/18; only in the public phase), which `ci-ok` accepts as `skipped` only when the plan does not need them (section "Jobs a change needs") | | SKIP until the first member has platform tests (M4-10) |
| `semgrep` | Semgrep with `.semgrep/` | project rules and their rule tests | | yes |
| `trivy` | Trivy `config` on a copy of the tracked and not ignored files (`tools/ci/tracked_tree.py`) | misconfiguration in Dockerfiles, Kubernetes, Helm, Terraform | `HIGH`, `CRITICAL` | once such files exist |
| `osv` | OSV-Scanner, govulncheck | known vulnerabilities of all lock files; reachable Go vulnerabilities | | yes |
| `licenses` | `tools/ci/license_check.py`, OSV-Scanner `--licenses`, go-licenses | license text, SPDX header lines, licenses of shipped dependencies; a list without a license is an error (if it were empty, `--licenses=` and `--allowed_licenses=` would no longer restrict the licenses: OSV-Scanner v2.6.0 then prints only a summary, [getters.go](https://github.com/google/osv-scanner/blob/v2.6.0/cmd/osv-scanner/internal/helper/getters.go#L17-L21), and go-licenses v2.0.1 rejects only forbidden and unknown licenses, [check.go](https://github.com/google/go-licenses/blob/v2.0.1/check.go#L64-L68), both retrieved 2026-10-12; M1-05/23) | `quality/licenses/allowed.txt`, `license.spdx_header_lines` | yes (dependencies from the first component) |
| `denylist` | `tools/denylist/denylist_scan.py` | white label in contents, names, commits, identities, refs, and the pull request title and body (`PR_TITLE`, `PR_BODY`, `PR_BODY_FILE`) | | yes, needs the private list |
| `sonar` | SonarScanner (npm) against the local server or SonarQube Cloud | gate "Bothy way" | `quality.sonar.*` | from the first product code |

In addition, the release workflow runs: `trivy-image` (vulnerabilities in the built image), SBOM
(Syft via `anchore/sbom-action`), build provenance and SBOM attestation (public),
`portability` (whole stack on a fresh machine).

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
* `migrations-lint` skips a Go module with a `SKIP` line as long as it has neither
  `migrations/` nor `migrations.sha256` (no migration, nothing to check). If only
  the checksum file is left, it fails.

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

Dart members (M6-12): a member with `tool/perf_macos.sh` runs its performance test in profile
mode on macOS, today the live topology of `packages/bothy_viz/harness` against
`viz.perf.frame_build_p95`. On another host `make perf` prints `SKIP perf-dart needs macOS`;
the CI job `apps-macos` runs the same script through `platform-tests.json` (`make
apps-platform`), there against the CI budget `viz.perf.frame_build_p95_ci`, because the
reference budget holds on reference hardware only (`make perf` sets
`BOTHY_PERF_BUDGET=reference`; docs/DECISIONS.md D-M6-12-10). The variable `PERF_HOST` (default `uname -s`) names the host system.

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
* An OpenTofu module with providers or remote module sources must pass `tofu init` offline.
  `ops-check` gives `tofu` an empty local mirror as its only provider source and every validator
  a closed proxy, so that the gate downloads nothing from the network; such a module fails until
  the ticket with the first provider defines a pinned way without network.
* That no validator reaches the network is proven by a test only on macOS
  (`test_no_validator_reaches_the_network` in `tools/opscheck/tests/`, with `sandbox-exec`). On
  Linux, so in CI, there is no equivalent lock in the test; there the closed proxy and
  `GCE_METADATA_HOST` work without a proof of their own.
* `otelcol-contrib validate` accepts unknown keys under some exporters (seen on 2026-10-11 with
  0.162.0: `otlp_http`); a typo in an optional key of such an exporter shows only in operation.
