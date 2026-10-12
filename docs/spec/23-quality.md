# 23 Quality, repository and supply chain

Status: draft M0. This chapter defines which checks every change passes, how
they run locally and in CI and how the repository is published. The guides for this are under [docs/quality/](../quality/README.md).
Decisions: ADR [0038](../adr/0038-local-gates-first-one-command-identical-to-ci.md),
[0039](../adr/0039-only-github-hosted-runners-public-from-the-first-push.md),
[0040](../adr/0040-sonarqube-local-and-cloud-with-the-same-profile-and-gate.md),
[0041](../adr/0041-dependency-updates-with-renovate.md),
[0042](../adr/0042-deny-list-scanner-with-the-list-from-the-ops-repo.md),
[0045](../adr/0045-documentation-site-generator.md).

## 1. Principles

1. **One command.** `make check` runs every gate. CI runs the same make targets,
   split across three jobs (`gates`, `denylist`, `sonar`). What is green locally is green
   in CI; the only intended exception is the path selection of the components (section 4).
2. **Local first.** Errors are found before the push. The pre-push hook calls
   `make check`. CI is the confirmation, not the place to debug.
3. **Fail closed.** If a tool, a list or a token is missing, the gate fails.
   A gate may skip with a `SKIP` line only when the checked
   component has no code yet. The Makefile makes this decision from the
   tracked files, not the developer. The only exception is the explicit switches
   for contributions from forks (R-QA-02).
4. **Pinned tools.** Every tool has a fixed version with a checksum
   (`tools/uv.lock`, `tools/node/package-lock.json`, `tools/bin/tools.lock.json`,
   image digests, action SHAs). Updates come as a pull request from Renovate.
5. **Numbers only as names.** Thresholds and deadlines of this chapter are
   configuration values (section 11). A test (`tools/docs/tests/test_config_values.py`)
   keeps the table, the tool configuration and the community files in sync.

## 2. Gates

The complete list with tool, threshold and where it runs is in
[gates.md](../quality/gates.md). Overview:

| Group | make targets | Checks |
|---|---|---|
| Code | `fmt-check`, `codegen-check`, `lint`, `typecheck`, `arch`, `register-check`, `migrations-lint`, `test`, `coverage` | Format, generated code without drift, linters, types, module boundaries of the Go modules, module register equal to the core contract and docs, migration rules, tests with coverage per component |
| Commits | `commitlint` | Conventional Commits and DCO sign-off of the new commits |
| Docs | `docs`, `config-doc-check`, `linkcheck`, `diagrams`, `catalog-check`, `api-lint`, `api-checks`, `api-clients`, `tokens` | Strict site build, configuration reference against schema and chapters, links and anchors, diagrams, action catalogue and its generated copies, API contracts, copies of the API contracts equal to their sources, generated clients, design tokens |
| Operations | `ops-check` | collector, Prometheus, Alertmanager, Loki, Tempo, Sloth, OpenTofu; derived values equal their source |
| CI and release rules | `runners-check`, `labels-check`, `release-pins` | GitHub-hosted runners, triggers, SHA pinning, timeouts, permissions; labels; pinned server version in the product bundle ([18](18-release.md) 2.1) |
| Security | `gitleaks`, `denylist`, `semgrep`, `trivy`, `osv` | Secrets, white label, project rules, misconfiguration, known vulnerabilities |
| Legal | `licenses` | License text, SPDX header lines, licenses of the dependencies |
| Analysis | `sonar` | SonarQube with a shared profile and gate |

At release, these are added: SBOM, provenance, signature, image scan (`trivy-image`) and the
portability test (`portability`). At the end of every milestone and in the release, `make perf`
runs with the performance targets from 02 ([ADR 0063](../adr/0063-performance-gate-for-the-event-log.md),
PLAN P-07). See [toolchain.md](../quality/toolchain.md).

## 3. Component contract

The root Makefile discovers components by their manifests: Go modules under `services/`,
Dart packages under `apps/` and `packages/`, Python projects under `sdk/python/` and
`plugins/`, TypeScript projects under `sdk/typescript/` and `plugins/`. For the gates,
the following applies per kind:

| Kind | Expected from the component |
|---|---|
| Go module | `go.mod` and `go.sum`; tests with `go test`; golangci-lint configuration in the module (dependency rules: chapter 02); `.go-arch-lint.yml`, generated from `architecture/modules.yaml` (gate `arch`); migrations under `migrations/` with the checksum file `migrations.sha256` next to them (gate `migrations-lint`, as soon as there are migrations); load tests under `perf/` with the build tag `perf` (`make perf`); coverage does not count `test/**` and `perf/**`, because they are test code themselves (02, section 6), and not generated files `*.gen.go`, because their generator and its golden test hold them |
| Dart package | `pubspec.yaml`, tests under `test/`; `custom_lint` as a dev dependency when own rules apply |
| Python project | `pyproject.toml` with a uv lock; ruff, mypy and pytest in the dev group; coverage threshold in its own pytest configuration |
| TypeScript project | `package.json` with the scripts `format`, `format:check`, `lint`, `typecheck`, `test` (coverage threshold in the test runner) |
| Release | Makefile in the component folder with the target `release-build` (arguments `VERSION`, `OUT`); apps in addition `release-apple` and `release-windows`; the product (`bothy`, root Makefile) also reads `GITHUB_REPOSITORY_OWNER` for the registry in the bundle ([18-release.md](18-release.md), section 2.1) |
| Generated client | Makefile with the target `generate-check` (generate from `docs/api/`, compile; a deviation from the checked-in state reports an error) |
| Design tokens | `tools/design-tokens/Makefile` with the target `check` |
| Portability | executable `infra/compose/smoke.sh`; from M7-14, with `PORTABILITY_TAG`, the test from `tests/portability` (interface in [16-operations.md](16-operations.md), section 11.1) |

### Documentation in code

Build rule 3 of the owner requires a doc comment on every public module,
class, function and interface, and API documentation that is generated from the code. Both are a
gate, not a review item (R-QA-14):

| Language | Gate for doc comments (in `make lint`) | Generated API documentation | From |
|---|---|---|---|
| Python | ruff rules `D` with convention `google`; tests excluded | Python SDK: reference in the docs site from the docstrings (mkdocstrings), built in `make docs` | Tools in `tools/` today; SDK M2-29 |
| Go | golangci-lint with `revive`, rules `exported` and `package-comments` | no separate site: the public interface of the server is `docs/api/openapi.yaml` (ADR 0013, spec first), rendered in the docs site; Go packages are internal | M1-05 |
| Dart | lint `public_member_api_docs` in `analysis_options.yaml` of every package; generated code excluded | `dart doc` per package without a warning, result in the docs site | M4-01 |
| TypeScript | typedoc with the check for undocumented exports as an error | TypeScript SDK: typedoc reference in the docs site | M2-30 |

Every gate has a test: a public symbol without a doc comment makes it fail.
"OpenAPI from the routes" means one source for both here: `openapi.yaml` is normative,
routes and clients are generated from it, and `make codegen-check` keeps them in sync
([briefs/README.md](../briefs/README.md), item 6).

If a part is missing although the component has code, the gate fails. The part "Release" applies
from the release chain of the component on, which brings the target `release-build` and its check
(server: [PLAN.md](../PLAN.md), ticket M7-10; product: M7-29); before that there is no artifact
that it would have to build.

## 4. CI

- Pull requests to `main` trigger `ci.yml`. The job `plan` determines the affected
  components from the changed paths (`tools/ci/components.py`). Gates of all other
  components skip; the repository-wide gates always run. A path outside
  known areas selects all components.
- The only required check is `ci-ok`; it summarizes all jobs.
- No `push` trigger on branches, no `pull_request_target`, `permissions: contents: read`
  as the default, `timeout-minutes` on every job, actions by full commit SHA. The only
  exception to the `push` ban is `denylist-main.yml` on exactly `main` (R-QA-15).
  `tools/ci/check_runners.py` enforces this (gate `runners-check`), actionlint and
  zizmor check in addition.
- Runners: only GitHub-hosted runners with a fixed label (`ubuntu-24.04`, for the
  portability test also `ubuntu-24.04-arm`, for the app builds `macos-15` and
  `windows-2025`); the repository is public from the first push (ADR 0039, R-QA-03).
  Jobs with the environment `release*` use outputs of other jobs only in conditions (R-QA-12).
  Only `release.yml` calls a release workflow, from one job and without a matrix, no workflow
  has `actions: write`, with which it could start another one by `workflow_dispatch` (sources
  in D-W-111), and no
  release workflow has the trigger `repository_dispatch` (rule `release-trigger`); so every run
  of a tag is one of its release (D-W-73, D-W-111).
- `concurrency` cancels superseded runs of a pull request; uv, npm, pinned binaries
  and pre-commit use caches. No path filter at the workflow level: a workflow that such a filter
  did not start would leave the required check `ci-ok` open; the job `plan` selects the
  component gates.

## 5. Push discipline

One pull request per finished feature, commits squashed locally beforehand, `main`
protected with a linear history, required check `ci-ok` and signed commits. A
follow-up commit "fix CI" or "typo" on `main` counts as a process error and gets an
analysis. Errors after the push are only acceptable when an external reviewer (SonarQube
Cloud, CodeRabbit) finds something that was not visible locally; this becomes a local
rule. Details: [push-discipline.md](../quality/push-discipline.md).

## 6. White label

The product repository contains nothing instance-specific. The deny-list scanner
(`tools/denylist/`) checks tracked content, file names, commit messages,
commit identities, branch and tag names; `--history` checks every object ever
stored. The private list lives outside the repository; without it, the scan fails.
Public, generic rules (`tools/denylist/generic.rules`) always run, also for
fork contributions without access to the list. The output names only the location and the rule line,
never the match. After every merge, `denylist-main.yml` checks the commits pushed to `main`
with the private list (R-QA-15). The allowed commit identity is valid only as author,
committer and own sign-off (D-W-37). ADR 0042.

## 7. SonarQube

A profile "Bothy way" per language inherits from the built-in profile and additionally activates
only the rules from `quality/sonar/quality-profile-<language>.xml`. The gate "Bothy way"
(`quality/sonar/quality-gate.json`) uses only metrics that exist in both modes of
SonarQube. `tools/sonarqube/quality_sync.py` loads both into the local Community
Build and into SonarQube Cloud and checks that they are equal. Only SonarQube Cloud
analyzes Dart; locally, `flutter analyze` with `custom_lint` covers Dart. ADR 0040,
[sonarqube-local.md](../quality/sonarqube-local.md).

## 8. Supply chain

- Actions by full commit SHA, images by digest, release binaries by sha256.
- Renovate proposes updates only after `deps.min_release_age` and groups them.
  Security updates come immediately.
- Trivy stays on the version that the vendor advisory names as safe after the incident of
  2026-03-19 (GHSA-69fq-xp46-6x23,
  https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23,
  retrieved 2026-10-10) [verified]. A change follows the check in
  [toolchain.md](../quality/toolchain.md).
- Releases: SBOM (SPDX), checksums; in the public state, in addition, signed
  build provenance via GitHub Artifact Attestations. Images carry Buildx provenance and
  SBOM in the registry entry. A server image is first pushed by digest only, checked with Trivy
  and only then tagged with its version.

## 9. Red-team gate

Before every push and every local merge, an independent, read-only reviewer checks the
diff; their report `.review/<sha>.md` ends with `Urteil: JA` (verdict: yes), otherwise the
pre-push hook blocks (R-QA-13). Every milestone also ends with a red-team review of the
whole milestone with the verdict `Urteil: JA` or `Urteil: NEIN` (no). On NEIN, the findings are
fixed and the delta is checked again; the threat model (chapter 19) is updated. Procedure: [red-team-gate.md](../quality/red-team-gate.md).

## 10. Rulings

| No. | Rule | Reason |
|---|---|---|
| R-QA-01 | `make check` = `gates` + `denylist` + `sonar`; CI jobs call exactly these targets. | Equality between local and CI comes from construction, not from comparing two lists. |
| R-QA-02 | A gate skips only when the component has no code; missing tools are errors. The only exception is the explicit switches for contributions from forks (`DENYLIST_FLAGS=--allow-missing`, `SONAR_SKIP=1`, `REVIEW_SKIP=1`); CI checks fork pull requests the same way, maintainers check fully before the merge, and `denylist-main.yml` checks after the merge. | A silent skip would be a green gate that checked nothing; outside contributors have neither the private list nor a SonarQube server. |
| R-QA-03 | Only GitHub-hosted runners with a fixed label; no self-hosted runners and no switch in `runs-on` (D-W-92). | The repository is public from the first push; there the standard runners are free, and a self-hosted runner would run code from outside pull requests. |
| R-QA-04 | Scheduled workflows run only with the public condition; their event carries no repository object, so they also use `vars.BOTHY_VISIBILITY` there. | Code scanning and Scorecard require a public repository, and Pages would turn even a private repository into a public website. |
| R-QA-05 | Semgrep runs only with the project's own rules, without registry rule sets. | Registry rules change without a version reference and have their own license; SonarQube, CodeQL, golangci-lint and bandit provide general static analysis. |
| R-QA-06 | SonarQube analyzes product code; ruff, mypy (strict), bandit and pytest check the repository tools. | Today there is only tool code; otherwise a running SonarQube server would be required for every planning change. |
| R-QA-07 | DCO is required for the commits beyond an `origin/main`, from the first push on; without a remote, the commit-msg hook checks every new commit. | The planning commits were made before the gates; new contributions need the proof of origin. |
| R-QA-08 | Without a remote, the deny-list scan checks all commits of `HEAD`. | The first push is the most dangerous one; identities of old commits must be correct before it. |
| R-QA-09 | Links to the internet are not a gate (`linkcheck-external` manual). | External servers would otherwise make a gate non-reproducible. `linkcheck` checks local links and anchors strictly. |
| R-QA-10 | No `.sonarcloud.properties`; SonarQube Cloud analyzes via CI, automatic analysis stays off. | Coverage and pull request decoration need the CI analysis; both ways at the same time interfere with each other. |
| R-QA-11 | Test servers that build large worlds via the public API (contract, leak and canary tests, preparation of the load tests) raise `ratelimit.*` through their configuration; measured load tests and tests of the rate limits run with the default values. | Building via the API checks the real paths but fails on limits meant for people; measurements and limit tests must see the shipped values. |
| R-QA-12 | Jobs with the environment `release*` (in any spelling, as GitHub compares environment names, source in D-W-110; a name is literal, rule `environment-not-literal`, D-W-110) use outputs of other jobs only in the `if:` of the job or of a step (D-W-72), resolve the tag again and compare, and take artifacts only by ID (rules `release-untrusted-output`, `release-artifact-by-id`, D-W-52). | A job with signing secrets builds, signs and publishes only what it determined itself; no other job dictates its path, version or artifact (T-SC-05). |
| R-QA-13 | Before every push, an independent review of the diff with the report `.review/<sha>.md` and the last line `Urteil: JA`; the pre-push hook checks it. The overall review per milestone stays. | Requirement 23 requires an AI review before every push; without proof it would be a note, not a gate. |
| R-QA-14 | Doc comments are a lint gate per language (ruff `D`, revive `exported`, `public_member_api_docs`, typedoc); API documentation is generated for the SDKs and Dart packages, for the server `openapi.yaml` is the source. | Build rule 3; a rule without a gate is not followed (in the tool code, docstrings were missing for most public symbols). |
| R-QA-15 | `denylist-main.yml` checks every push to `main` with the private list; the only exception to the `push` ban. | Fork contributions see only the generic rules, and a squash can change the message after the checks; requirement 22 requires a CI job that fails. |
| R-QA-16 | Superseded by D-W-92: no rule anymore. | The rule applied only to runs in the private state, which does not exist. |

## 11. Configuration values of this chapter

| Name | Default | Unit | Meaning | Held by |
|---|---|---|---|---|
| `quality.coverage.tools_min_percent` | `90` | percent | Minimum coverage of the repository tools | `tools/pyproject.toml` (`fail_under`) |
| `quality.coverage.go_min_percent` | `80` | percent | Minimum coverage per Go module | `Makefile` (`COVERAGE_GO_MIN`) |
| `quality.coverage.dart_min_percent` | `80` | percent | Minimum coverage per Dart package | `Makefile` (`COVERAGE_DART_MIN`) |
| `quality.perf.timeout` | `60m` | duration | Time limit of `make perf` per Go module | `Makefile` (`PERF_TIMEOUT`) |
| `quality.sonar.new_issues_max` | `0` | count | New findings in new code | `quality/sonar/quality-gate.json` |
| `quality.sonar.new_hotspots_reviewed_min_percent` | `100` | percent | Reviewed security hotspots in new code | `quality/sonar/quality-gate.json` |
| `quality.sonar.new_coverage_min_percent` | `80` | percent | Coverage of new code | `quality/sonar/quality-gate.json` |
| `quality.sonar.new_duplication_max_percent` | `3` | percent | Duplicated lines in new code | `quality/sonar/quality-gate.json` |
| `quality.sonar.local_port` | `9000` | port | Port of the local SonarQube on `127.0.0.1` | `tools/sonarqube/compose.yaml`, `Makefile` (`SONAR_HOST_URL`) |
| `quality.sonar.startup_timeout_s` | `300` | seconds | Wait time of `make sonar-up` for the status UP | `Makefile` (`SONAR_STARTUP_TIMEOUT_S`) |
| `commit.header_max_length` | `100` | characters | Longest commit header line | `tools/hooks/commit_msg.py` |
| `repo.max_file_kb` | `1024` | KiB | Largest file in the repository | `.pre-commit-config.yaml` |
| `license.spdx_header_lines` | `5` | lines | Range in which the SPDX header line must be | `tools/ci/license_check.py` |
| `deps.min_release_age` | `7 days` | duration | Wait time before Renovate proposes a new version | `renovate.json` |
| `security.report_ack_target` | `3` | business days | Acknowledgment of a vulnerability report | `SECURITY.md` |
| `security.assessment_target` | `10` | business days | First assessment with severity | `SECURITY.md` |
| `security.fix_target_critical` | `30` | days | Published fix for critical severity | `SECURITY.md` |
| `security.fix_target_high` | `60` | days | Published fix for high severity | `SECURITY.md` |
| `security.disclosure_deadline` | `90` | days | Latest publication | `SECURITY.md` |
| `triage.first_response_target` | `14` | days | First response to a new issue | `GOVERNANCE.md` |
| `triage.needs_info_close_after` | `30` | days | Closing without a response for `status: needs-info` | `GOVERNANCE.md` |
