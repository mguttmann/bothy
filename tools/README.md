# tools

Tools of the repository: gates, check scripts and generators. None of them is
shipped. Always run them through `make` (root `Makefile`), so that the same pinned
versions run as in CI. Setup once per clone: `make bootstrap`.

| Path | Purpose | make target | Tests |
|---|---|---|---|
| `pyproject.toml`, `uv.lock`, `.python-version` | pinned Python toolchain (ruff, mypy, bandit, pytest, pre-commit, MkDocs, zizmor, check-jsonschema) | `bootstrap` | |
| `node/` | pinned Node tools (mermaid-cli, Redocly CLI, SonarScanner); install scripts of npm packages are disabled | `bootstrap` | |
| `bin/fetch_tools.py`, `bin/tools.lock.json` | download release binaries (actionlint, amtool, gitleaks, go-arch-lint, golangci-lint, loki, lychee, osv-scanner, otelcol-contrib, promtool, sloth, sqlc, tempo, tofu, trivy) with a sha256 pin; the asset as a binary, tar.gz or zip | `bootstrap`, `tools-lock` | `bin/tests/` |
| `ci/check_runners.py` | workflow rules: GitHub-hosted runners, triggers, SHA pinning, timeouts, permissions (no `actions: write`), environments without case and only literal | `runners-check` | `ci/tests/` |
| `ci/tests/test_codeql_workflow.py` | CodeQL guard: the matrix of `.github/workflows/codeql.yml` covers every language that CodeQL supports and that the workflows, `tools/pyproject.toml` or the component manifests show (the same manifests as the component discovery of the Makefile); Go with a manual build per module; workflow, job and steps only with options from closed lists; step "Trial run" in `docs/quality/public-switch.md` names every check | `test` | |
| `ci/ci_ok.py` | the last jobs `ci-ok` of `ci.yml` and `build-verify-ok` of `build-verify.yml`: check the results of their `needs` against the plan of the run | CI jobs `ci-ok` and `build-verify-ok` | `ci/tests/test_ci_ok.py` |
| `ci/build_base.py` | base of the change plan of a build branch: the merge base with `build-base/<ticket>` when that ref was pushed, else with `origin/main` | CI job `plan` of `build-verify.yml` | `ci/tests/test_build_base.py` |
| `ci/components.py` | component map: path filters in CI, resolution of release tags; `pins` checks the server version that the product bundle pins | `release-pins` (otherwise CI) | `ci/tests/` |
| `ci/license_check.py` | license text and SPDX headers | `licenses` | `ci/tests/` |
| `ci/labels.py` | check labels and create them on GitHub | `labels-check`, `labels-sync` | `ci/tests/` |
| `ci/tracked_tree.py` | copy tracked and non-ignored files into a folder, so that scanners that walk folders see only the repository | `trivy`, test `ci/tests/test_gitleaks_rules.py` | `ci/tests/` |
| `ci/migrations_lint.py` | migration rules of the Go modules (ADR 0007, chapter 14): consecutive numbers, checksum file `migrations.sha256`, `STORED`, `NO TRANSACTION`; `lock` only adds new checksums | `migrations-lint`, `migrations-lock` | `ci/tests/` |
| `ci/codegen_check.py` | run the generator and report drift of generated code through snapshots before and after the run | `codegen-check` | `ci/tests/` |
| `ci/go_prereqs.py` | prerequisites of the server gates: go, Docker-compatible runtime, pins of go-arch-lint and sqlc | `codegen-check`, `arch`, `migrations-lint`, `perf`, `test` | `ci/tests/` (recipes: `ci/tests/test_make_server_gates.py`) |
| `conftest.py` | test isolation: removes variables such as `GIT_DIR`, which Git hooks set, from the environment of every test, so that Git fixtures never hit the real repository; runs the whole test run (collection, fixtures of every scope, every test) without a Git identity, like a fresh CI runner (no global or system configuration, `user.useConfigOnly`) | `test` | `ci/tests/test_git_env_isolation.py` |
| `testing/git_fixtures.py` | throw-away Git repositories for tests with their own identity, no signing and no hooks; the fixtures that commit or tag use it, and one without an identity fails in every test run, as on the runner | `test` | `ci/tests/test_git_env_isolation.py` |
| `opscheck/` | check the operations configuration (collector, Prometheus, Alertmanager, Loki, Tempo, Sloth, OpenTofu) with the pinned validators; derived values equal their source ([README](opscheck/README.md)) | `ops-check` | `opscheck/tests/` |
| `signalcheck/` | Dashboards, Prometheus rules and Sloth files only use metrics, labels and bucket bounds of the signal catalogue; dashboard queries parse with the pinned promtool ([README](signalcheck/README.md)) | `signal-check` | `signalcheck/tests/` |
| `hooks/text_rules.py` | no en or em dashes (U+2013, U+2014) in text files | pre-commit, `lint` | `hooks/tests/` |
| `hooks/secret_formats.py` | no value matching the Stripe webhook secret format `whsec_` (`FORMATS["stripe_webhook_signing_secret"]`) that GitHub secret scanning reports; other GitHub patterns are not covered (docs/quality/gates.md); examples stay short of the format, tests assemble their samples at runtime | pre-commit, `lint`, `test` | `hooks/tests/` |
| `hooks/commit_msg.py` | Conventional Commits and DCO sign-off | commit-msg, `commitlint` | `hooks/tests/` |
| `configdoc/configdoc.py`, `configdoc/planned.yaml`, `configdoc/retired.yaml` | generate and check section 4 of the configuration reference from the server schema (`services/server/architecture/config-schema.json`) and the chapter tables (rules D1 to D10: duplicate names, schema against chapters, milestones of planned keys, names retired by W1, Compose deadline, environment names, reference freshness); the tests hold the detection of code against the CommonMark parser `markdown-it-py` | `config-reference`, `config-doc-check` | `configdoc/tests/` |
| `configdoc/app_config.py` | Key table of `AppConfig` in `packages/domain` from section 4 of the configuration reference: every value of place `App` and the server keys of D-W-115 and D-W-116, kind derived from the default (docs/DECISIONS.md D-M4-08-5) | `app-config`, `codegen-check` | `configdoc/tests/test_app_config.py` |
| `denylist/` | white-label scanner with a private list and public rules, also for the pull request title and body (`PR_TITLE`, `PR_BODY`, `PR_BODY_FILE`; [README](denylist/README.md)) | `denylist`, `denylist-history` | `denylist/tests/` |
| `docs/language_check.py` | no German text in tracked files: German prose (share of German function words) and words with an umlaut fail, except the paths and names in `docs/language-allow.txt` and text between `language-check: off` and `language-check: on` (D-W-94) | `docs` | `docs/tests/` |
| `docs/adr_index.py` | generate and check the ADR index | `adr-index`, `docs` | `docs/tests/` |
| `docs/problem_code_refs.py` | every pair of status and problem code in Markdown and API files appears exactly so in `x-problem-codes` (06 section 5 item 7) | `docs` | `docs/tests/` |
| `docs/brief_deps.py` | keep the brief index, waves, graph, the plan's column "Depends on" and the brief headers equal; no brief calls a `make` target of a ticket outside its dependencies, names such a ticket in a start check, or calls a ticket of its own closure missing or optional; `--write` regenerates waves and graph | `docs` | `docs/tests/` |
| `docs/register_ids.py` | every ID in the first column of a table in `docs/**/*.md` (for example `R-AG-22`, `T-SC-05`, `ID-T01`, `D-W-01`, `RK-82`) is defined exactly once in the whole repository (D-W-60); every table row has as many cells as its header (D-W-71); the overview and the sections of the open questions name the same IDs | `docs` | `docs/tests/` |
| `docs/risk_ranking.py` | the table "The five biggest risks" in `docs/RISKS.md` follows the first criterion: the same likelihood and impact as in the register, ranks without gaps, the rating never rises further down, no risk outside the table rated higher than its last rank (D-W-78), the list "Behind rank" names exactly the risks outside the table with the rating of the last rank (D-W-79) | `docs` | `docs/tests/` |
| `docs/render_diagrams.py` | every Mermaid diagram renders with the design theme, aspect ratio and token colors; `--write` re-renders the `.svg` | `diagrams` | `docs/tests/` |
| `docs/erd_check.py` | ERDs in `docs/architecture/` against 03: every table exactly once in full, every foreign key drawn | `diagrams` | `docs/tests/` |
| `docs/mkdocs_repo_links.py` | MkDocs hook: links to existing files outside `docs/` point to the repository in the site | `docs` | `docs/tests/` |
| `doccheck/register_check.py` | compare the module register `services/server/architecture/modules.yaml` with chapter 03, `docs/spec/actions.yaml` (loader from `catalog/`), the module tables of chapter 02, the component diagram and the module READMEs, one line per difference (ADR 0050) | `register-check` | `doccheck/tests/` (renders with Markdown and the pinned mermaid-cli, `make bootstrap`) |
| `catalog/` | load and check the action catalogue `docs/spec/actions.yaml`, generate the tables of chapter 05 and the lists in `docs/api/redocly.yaml` from it ([README](catalog/README.md)); `event_catalog_dart.py` writes the event catalogue of `packages/domain`, `contract_documents_dart.py` the copy of `docs/api/` in the fake server of `packages/sync_engine` | `catalog`, `catalog-check`, `event-catalog`, `contract-documents` | `catalog/tests/` |
| `api-checks/` | check the copies of the API contracts against their sources (chapter 06, G2), generate the generated tables of 06 and `docs/api/webhooks.md` and check them against a fresh generation (G3) ([README](api-checks/README.md)) | `api-checks` (G2 and G3), `api-tables` | `api-checks/tests/` |
| `contract/` | G7 against a live server: Schemathesis run, DPoP proofs, credential broker client, project checks, `curl` reproduction ([README](contract/README.md)); runs with Schemathesis and `cryptography`, the exception to the rule below | `contract-test` (from the Go test of `services/server/test/contract`) | `contract/tests/` |
| `docs/tests/test_config_values.py`, `docs/tests/test_toolchain_doc.py` | number and version guards for the docs | `test` | |
| `sonarqube/` | local SonarQube and syncing of profile and gate ([README](sonarqube/README.md)) | `sonar-up`, `sonar-config`, `sonar-config-check`, `sonar-main-branch`, `sonar-visibility`, `sonar-new-code` | `sonarqube/tests/` |
| `design-tokens/` | translate design tokens into CSS, Dart, contrast table and preview (chapter 15, design system) | `tokens` (`design-tokens/Makefile`, target `check`) | `design-tokens/tests/` (unittest, via `make tokens`) |

Rules for new tools: Python with the standard library or PyYAML (a MkDocs hook
additionally with MkDocs and Markdown from the same pinned environment), one test file per
tool, `SPDX-License-Identifier: Apache-2.0` in every file, a make target and a row
in this table. A check script of our own also gets its row under the gate in
docs/quality/gates.md; a new pinned third-party tool (binary, Python or
Node package) gets its row in docs/quality/toolchain.md, because the test of that file holds the
versions against the pins.
