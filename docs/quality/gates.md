# Gates

`make check` runs all gates in a fixed order and stops at the first failure.
CI runs the same targets: job `gates` calls `make gates`, job `denylist` calls
`make denylist`, job `sonar` calls `make sonar`; `check` is exactly `gates denylist sonar`
(chapter 23, R-QA-01). Thresholds appear as names; their default values appear only in
chapter 23, section 11.

![Flow from commit to release](pipeline.svg)

Source of the diagram: `docs/quality/pipeline.mmd`.

## List

| make target | Tool | Checks | Threshold | Active today |
|---|---|---|---|---|
| `fmt-check` | ruff format, gofmt, dart format, script `format:check` of the TypeScript projects | format without changes | | tools |
| `codegen-check` | `tools/ci/codegen_check.py` with sqlc and `go generate` (go-arch-lint and sqlc from the pins at the front of `PATH`) | generated code (sqlc code, `.go-arch-lint.yml`, API code) matches a fresh generation; only changes by the generator count, not manual changes in the working tree | | from the first Go module |
| `lint` | pre-commit hooks over all files (text rules, Stripe webhook secret format `whsec_` of GitHub secret scanning, whitespace, YAML/TOML/JSON, merge markers, private keys, file size), ruff (with required docstrings `D`, section "Documentation in code" in chapter 23), bandit, check-jsonschema (issue forms, Renovate, CodeRabbit), actionlint, zizmor, go vet and golangci-lint, flutter analyze and custom_lint, script `lint` | style, sources of errors, workflow security | `repo.max_file_kb` | yes |
| `typecheck` | mypy (strict), go build, tsc | types | | tools |
| `arch` | go-arch-lint with `<module>/.go-arch-lint.yml`, generated from `architecture/modules.yaml` | module boundaries and allowed third-party libraries of the Go modules (chapter 02 "go-arch-lint"); no symbolic links below a module, because go-arch-lint does not follow a linked directory and assigns it by the path of the link, while the Go compiler builds the code behind it (go-arch-lint v1.19.0 walks the module with [`filepath.WalkDir`](https://github.com/fe3dback/go-arch-lint/blob/v1.19.0/internal/services/project/scanner/scanner.go#L74), which [does not follow symbolic links](https://pkg.go.dev/path/filepath#WalkDir), retrieved 2026-10-12; M1-05/24) | | from the first Go module |
| `migrations-lint` | `tools/ci/migrations_lint.py` | consecutive five-digit numbers without gaps and without timestamp versions, checksums of merged migrations unchanged (`migrations.sha256`), `STORED` with `GENERATED ALWAYS AS`, `-- +goose NO TRANSACTION` with `CONCURRENTLY` (ADR 0007, chapter 14) | | from the first migration |
| `test` | pytest, go test -race, flutter test, script `test` | tests with coverage data | | tools |
| `coverage` | coverage, go tool cover, LCOV evaluation | minimum coverage; Go without the packages under `test/` and `perf/` (test code, chapter 23 "Component contract") | `quality.coverage.tools_min_percent`, `quality.coverage.go_min_percent`, `quality.coverage.dart_min_percent` | tools |
| `commitlint` | `tools/hooks/commit_msg.py` | Conventional Commits and DCO of the commits that are not yet on `origin/main` | `commit.header_max_length` | from the first remote |
| `docs` | `tools/docs/language_check.py`, `tools/docs/adr_index.py`, `tools/docs/config_reference.py`, `tools/docs/problem_code_refs.py`, `tools/docs/brief_deps.py`, `tools/docs/register_ids.py`, `tools/docs/risk_ranking.py`, MkDocs `--strict` with the hook `tools/docs/mkdocs_repo_links.py` | ADR index and configuration reference up to date, no configuration name twice (ruling L4), every pair of status and problem code in Markdown and API files appears as such in `x-problem-codes` (06 section 5 item 7), index, waves and graph of the briefs match the plan and no brief calls a `make` target that only a ticket outside its dependencies creates, every ID that a table in `docs/**/*.md` defines in the first column (rulings, tests, guard rules, threats, decisions, risks, open questions, tickets) appears as such exactly once in the whole repository (D-W-60), every table row has as many cells as its header (D-W-71), the overview and the sections of the open questions name the same IDs, the ranking of the largest risks names the rating of the register and leaves out no higher-rated risk (D-W-78), and the list behind its last rank names exactly the remaining risks with that rank's rating (D-W-79), the site builds without a warning; links to existing files outside `docs/` point to the repository in the site, a missing target stays an error; no tracked text file contains German prose or a word with an umlaut, except the exceptions in `tools/docs/language-allow.txt` and text between the marker lines `language-check: off` and `language-check: on` (D-W-94) | | yes |
| `linkcheck` | lychee `--offline --include-fragments`; `tools/docs/placeholder_links.py` | local links and anchors of all tracked Markdown files; no link outside code to the placeholder repository `OWNER` (P-05), because it leads nowhere after publishing | | yes |
| `diagrams` | `tools/docs/render_diagrams.py` with mermaid-cli, `tools/docs/erd_check.py` | every Mermaid diagram renders; every `.mmd` has its `.svg`, rendered with the design theme (`design/build/mermaid.config.json`), stamped with the SHA-256 of `.mmd` and theme (otherwise stale, D-W-56) and not wider or taller than `MAX_ASPECT`; `.mmd` files name only token colors; the ERDs show every table and every foreign key from 03 | | yes |
| `catalog-check` | `tools/catalog/action_catalog.py` | `docs/spec/actions.yaml` valid (schema, unique IDs, rate-limit classes, receive actions, ops without a system action), equal to the hand-maintained parts of chapter 05 (scopes for apps, effect, configuration values of the classes); the generated tables in 05 and lists in `docs/api/redocly.yaml` are current ([ADR 0049](../adr/0049-machine-readable-action-catalogue-as-the-single-source.md)) | | yes |
| `api-lint` | Redocly CLI with `docs/api/redocly.yaml` | OpenAPI and AsyncAPI, requirement of `x-authz-action` | | once `docs/api/openapi.yaml` exists |
| `api-checks` | `tools/api-checks/api_checks.py` | copies of the API contracts equal their sources (chapter 06, G2 and G3): `x-authz-action`, `x-rate-limit-class`, ops, events, receive actions, scopes (`AppScope`) and the union `Event` in `openapi.yaml` and `asyncapi.yaml` equal `docs/spec/actions.yaml`, close codes equal 04, `Idempotency-Key` or `x-idempotency-exempt` with a known reason on every POST and PATCH, deprecation dates against `api.deprecation_min_notice`, `x-problem-codes` and `x-status` complete; the generated tables in 06 and `docs/api/webhooks.md` are current ([README](../../tools/api-checks/README.md)) | | yes |
| `ops-check` | `tools/opscheck/opscheck.py` with the pinned validators otelcol-contrib, promtool, amtool, loki, tempo, sloth and tofu ([README](../../tools/opscheck/README.md)) | collector base alone, with each overlay and with every ordered pair of two overlays; Prometheus configuration, rules and rule unit tests (every hand-written alert has one); Alertmanager configuration and routing; Loki; Tempo; OpenTofu modules; generated SLO rules equal to a fresh Sloth generation; SLO table equal to the Sloth files; collector allowlist equal to the signal catalogue; alert thresholds equal to their configuration values; validator versions equal to the image versions of the Compose stack | | yes |
| `api-clients` | target `generate-check` of the client packages | generated clients are up to date and compile | | from the first client package |
| `tokens` | target `check` in `tools/design-tokens/` | tests of the generator; design tokens build, contrast and palette guard checks pass, generated files up to date, embedded files match `vendor/MANIFEST.json` | | yes |
| `runners-check` | `tools/ci/check_runners.py` | Only fixed GitHub-hosted runner labels (ADR 0039), triggers, SHA pinning, timeouts, permissions (no `write-all`, no `actions: write`, also not on a job, D-W-111); environment names only literal (D-W-110); in jobs with environment `release*` in any spelling, outputs of jobs without this environment only in the `if:` of the job or of a step (allow-list D-W-61, D-W-72), artifacts only via `artifact-ids` (D-W-52); matrix values in `runs-on` checked per combination, no runtime matrix behind `runs-on` (D-W-53); release workflows are called only by a job of `release.yml` without a matrix (D-W-73); `push` only on tags, except `denylist-main.yml` on `main` | | yes |
| `release-pins` | `tools/ci/components.py pins` | the server version that the product bundle pins (`BOTHY_VERSION=` in `infra/compose/.env.example`): exactly one line, SemVer without `v` or the placeholder of the template, and a stable product (`VERSION` without a pre-release identifier) pins no server pre-release, which only a product beta (`X.Y.Z-beta.N`) may pin; so a typo fails at the latest in the required check `ci-ok` of the pull request with the release commit, that is before the tag, not only in the build of the release chain; the push of the tag alone starts no check ([18 2.1](../spec/18-release.md#21-server-version-and-registry-in-the-product-bundle), D-W-98) | | yes |
| `labels-check` | `tools/ci/labels.py` | label file valid, templates and GOVERNANCE.md use only defined labels | | yes |
| `gitleaks` | gitleaks with `.gitleaks.toml` | secrets in every commit reachable from `HEAD`; before going public, in all refs; the EXAMPLE marker exempts only this product's own token formats and generic key-like values | | yes |
| `semgrep` | Semgrep with `.semgrep/` | project rules and their rule tests | | yes |
| `trivy` | Trivy `config` on a copy of the tracked and not ignored files (`tools/ci/tracked_tree.py`) | misconfiguration in Dockerfiles, Kubernetes, Helm, Terraform | `HIGH`, `CRITICAL` | once such files exist |
| `osv` | OSV-Scanner, govulncheck | known vulnerabilities of all lock files; reachable Go vulnerabilities | | yes |
| `licenses` | `tools/ci/license_check.py`, OSV-Scanner `--licenses`, go-licenses | license text, SPDX header lines, licenses of shipped dependencies; a list without a license is an error (if it were empty, `--licenses=` and `--allowed_licenses=` would no longer restrict the licenses: OSV-Scanner v2.6.0 then prints only a summary, [getters.go](https://github.com/google/osv-scanner/blob/v2.6.0/cmd/osv-scanner/internal/helper/getters.go#L17-L21), and go-licenses v2.0.1 rejects only forbidden and unknown licenses, [check.go](https://github.com/google/go-licenses/blob/v2.0.1/check.go#L64-L68), both retrieved 2026-10-12; M1-05/23) | `quality/licenses/allowed.txt`, `license.spdx_header_lines` | yes (dependencies from the first component) |
| `denylist` | `tools/denylist/denylist_scan.py` | white label in contents, names, commits, identities, refs | | yes, needs the private list |
| `sonar` | SonarScanner (npm) against the local server or SonarQube Cloud | gate "Bothy way" | `quality.sonar.*` | from the first product code |

In addition, the release workflow runs: `trivy-image` (vulnerabilities in the built image), SBOM
(Syft via `anchore/sbom-action`), build provenance and SBOM attestation (public),
`portability` (whole stack on a fresh machine).

## Skipping and failing

* A gate writes `SKIP <name> <reason>` and counts as passed when the component
  has no code. The Makefile decides this from `git ls-files`: a Go module is a
  tracked `go.mod` under `services/`, a Dart package a `pubspec.yaml` under `apps/`
  or `packages/`, a Python project a `pyproject.toml` under `sdk/python/` or
  `plugins/`, a TypeScript project a `package.json` under `sdk/typescript/` or
  `plugins/`. New files count only after `git add`.
* A missing tool, a missing list, a missing token or a missing
  part of the component contract (chapter 23, section 3) is an error.
* Contributors from forks explicitly switch off what they do not have:
  `DENYLIST_FLAGS=--allow-missing` (public rules only), `SONAR_SKIP=1` and
  `REVIEW_SKIP=1` (no independent review of their own). Everything is visible in the output;
  CI treats fork pull requests the same way (R-QA-02). What the fork could not check,
  the maintainer catches up on before the merge; after the merge, `denylist-main.yml` also
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

`make perf` runs the load tests of every Go module: `go test -tags perf -count=1` over
`<module>/perf/...`, with the time limit `quality.perf.timeout` per module. It runs at the
close of every milestone and in the release, never in `gates` and never in the pre-push hook
(PLAN P-07): the performance targets from chapter 02 "Performance targets" apply only on
`perf.reference_profile`; on arbitrary hardware the measurements would not be comparable
(ADR 0063). That is why the output shows the note about `perf.reference_profile` before every
module.

Load tests carry the build tag `perf`, so that `make test` does not run them. Their packages
are under `<module>/perf/` and do not count toward coverage. Without a Go module,
`make perf` reports `SKIP`; a Go module without `perf/` is an error. Like the server gates,
`make perf` needs go, a Docker-compatible runtime and the pins.

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
| pre-push | before every push | `tools/hooks/review_gate.py` (report `.review/<sha>.md` of the pushed commit ends with `Urteil: JA`, [red-team-gate.md](red-team-gate.md)), then `make check` |

## Individual gates

Every gate also runs alone, for example `make lint` or `make diagrams`. For the
diagrams, `MMDC_PUPPETEER_CONFIG` can point to a Puppeteer configuration. If it names
its own Chromium (`executablePath`), `make bootstrap` downloads no browser; otherwise it downloads
the one that matches Puppeteer. CI uses `tools/node/puppeteer-ci.json` (Chromium without
sandbox, because runners can restrict user namespaces; only the project's
own diagrams are rendered).

Further switches: `COMMIT_RANGE` and `DENYLIST_RANGE` (commit range instead of
`origin/main..HEAD`), `GITLEAKS_LOG_OPTS` (default `HEAD`, all refs with `--all`), `GITLEAKS_FLAGS` (for
example a JSON report), `DENYLIST_FLAGS` (for example `--reveal` locally).

## What the gates do not cover

* Links to the internet: `make linkcheck-external` when needed (R-QA-09).
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
