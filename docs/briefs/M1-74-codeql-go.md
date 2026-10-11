# M1-74: CodeQL for Go

Status: ready | Depends on: M1-05 | Etappe: M1

## Goal

After this ticket the CodeQL workflow analyses the Go server module `services/server` next to the
workflow files and the Python tools, in the public phase only, exactly as ADR 0039 rule 1 allows.
The Go entry builds the shipped module manually, because CodeQL offers no build-free mode for Go
and its autobuilder would first run the repository root `Makefile`. A test against the real
workflow file keeps the CodeQL matrix in step with the languages the repository ships, so the next
supported language cannot be forgotten silently.

## Read first

- `docs/PLAN.md`: section "M1 Fundament", ticket row M1-74 (Block H), "Arbeitsweise je Ticket",
  "Entscheidungen dieses Plans" (P-09, P-12).
- `.github/workflows/codeql.yml` (current state: matrix `[actions, python]`, every entry with
  `build-mode: none`; the job condition is the public guard).
- `docs/adr/0039-nur-github-gehostete-runner-oeffentlich-ab-dem-ersten-push.md`, "Decision
  Outcome", rule 1 "Öffentlich-Jobs".
- `tools/ci/check_runners.py` (`PUBLIC_GUARD`, `has_public_guard`, `load_yaml`, the rule list in
  the module docstring) and `tools/ci/tests/test_check_runners.py` (test style, `REPO` constant).
- `docs/quality/toolchain.md`, section "GitHub Actions" (pins of `actions/checkout`,
  `actions/setup-go`, `github/codeql-action`) and `tools/docs/tests/test_toolchain_doc.py`
  (`test_actions_table_matches_workflows`: one version per action across all workflows).
- `docs/quality/gates.md`: "Liste" (rows `lint`, `runners-check`) and "Was die Gates nicht
  abdecken" (no shellcheck; workflows only run short commands).
- `docs/quality/public-switch.md`, step 18 "Probelauf".
- `docs/spec/23-qualitaet.md`: "Komponenten-Vertrag" (components are discovered from tracked
  manifests) and R-QA-05 (CodeQL is part of the general static analysis).
- `Makefile`: component discovery (`GO_MODS`, `PY_PROJECTS`, `TS_PROJECTS`), targets `lint`
  (actionlint, zizmor), `runners-check`, `typecheck` (`go build ./...` per Go module).
- `.github/workflows/ci.yml`: how `actions/setup-go` is pinned and fed with `go-version-file`.
- `services/server/go.mod` and `services/server/README.md` (from M1-05).
- External facts (checked 2026-10-10):
    - "CodeQL supports build modes `autobuild` or `manual` for Go code"; Go has no `none` mode;
      the Go autobuilder first invokes `make`, `ninja`, `./build` or `./build.sh` and then needs
      `go list ./...` to succeed; test files (`_test.go`) are not analysed by default.
      https://docs.github.com/en/code-security/reference/code-scanning/codeql/build-options-for-compiled-languages
      [belegt]
    - CodeQL CLI terms: analysis is permitted only on an "Open Source Codebase" (released under an
      OSI-approved license); use "in connection with any codebase that is not an Open Source
      Codebase (e.g., code in a private repo in GitHub)" is excluded without a paid GitHub Advanced
      Security license. https://raw.githubusercontent.com/github/codeql-cli-binaries/main/LICENSE.md
      [belegt]
- This ticket touches no specification chapter, API operation, data table or action id.

## Fixed requirements that apply

Owner build rules (verbatim):

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

Rules for every brief:

6. Test first: the new test is red before the workflow changes (record the red output), then green.
7. No en or em dashes in any text you write (`uv run --project tools --locked --quiet python tools/hooks/text_rules.py <file>` exits 0).
   White label: no company, person, host or domain names except `example.com`; no paths of a
   personal machine.
8. Configuration values only through named keys from `docs/operations/configuration.md`; this
   ticket introduces none.

Product requirements of this ticket:

9. CodeQL is a public-phase job: the job condition stays exactly `PUBLIC_GUARD` from
   `tools/ci/check_runners.py` (ADR 0039 rule 1).
10. Every action is pinned to a full commit SHA with the version comment already listed in
    `docs/quality/toolchain.md`; no new action and no new version (ADR 0039 "Confirmation",
    `test_actions_table_matches_workflows`).
11. Permissions stay minimal: top-level `contents: read`; job-level `contents: read` and
    `security-events: write` only; `persist-credentials: false` on checkout (zizmor `artipacked`).
12. CodeQL covers every tracked component language that CodeQL supports (plan ticket row M1-74;
    R-QA-05). Dart is not supported by CodeQL and is covered by `flutter analyze` and SonarQube
    Cloud (23 "Gates").

## Scope

1. `.github/workflows/codeql.yml` (change):
    - Header comment: replace the sentence "Languages: add `go` and `javascript-typescript` ..."
      with: the matrix lists every tracked language CodeQL supports;
      `tools/ci/tests/test_codeql_workflow.py` enforces it; `javascript-typescript` is added by the
      ticket that adds the first TypeScript project; Go uses `build-mode: manual` because CodeQL
      has no `none` mode for Go and the autobuilder would run the root `Makefile` first; Dart is not
      supported by CodeQL.
    - The matrix becomes an `include` list, exactly:
        ```yaml
        strategy:
          fail-fast: false
          matrix:
            include:
              - language: actions
                build-mode: none
              - language: go
                build-mode: manual
              - language: python
                build-mode: none
        ```

    - Steps, in this order:
        1. `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1` with
           `persist-credentials: false` (unchanged).
        2. Go toolchain, only for Go:
            ```yaml
            - if: matrix.language == 'go'
              uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
              with:
                go-version-file: services/server/go.mod
                cache: false
            ```
            `cache: false` because the Go build cache is not checksum-verified (unlike modules, which
            `go.sum` verifies) and an analysis job must not trust state written by another run; the job
            runs on free public runners, so a cache saves nothing worth that risk.

        3. `github/codeql-action/init@24c54180a607b1449ed407dd24f251e4e9147c8d # v4.38.3` with
           `languages: ${{ matrix.language }}`, `build-mode: ${{ matrix.build-mode }}`,
           `queries: security-extended`.
        4. Build, only for Go:
            ```yaml
            - name: Build the server module for CodeQL
              if: matrix.language == 'go'
              working-directory: services/server
              env:
                CGO_ENABLED: "0"
              run: go build ./...
            ```
            `CGO_ENABLED=0` because the server ships as a static binary (baseline B2, ADR 0005), so
            CodeQL sees the build configuration that is shipped.

        5. `github/codeql-action/analyze@24c54180a607b1449ed407dd24f251e4e9147c8d # v4.38.3` with
           `category: /language:${{ matrix.language }}` (unchanged).
    - Unchanged: name, triggers (`pull_request` to `main`, `schedule`, `workflow_dispatch`),
      top-level `permissions`, `concurrency`, job `if`, `runs-on: ubuntu-24.04`,
      `timeout-minutes`, job `permissions`.
2. `tools/ci/tests/test_codeql_workflow.py` (new, SPDX header
   `# SPDX-License-Identifier: Apache-2.0`, module docstring). Contents:
    - `REPO = Path(__file__).resolve().parents[3]`; YAML loaded with `check_runners.load_yaml`
      (keeps `on` a string).
    - `LANGUAGE_SOURCES`: tracked-manifest patterns to CodeQL languages, mirroring the `Makefile`
      discovery plus the repository's own code:

        | tracked path (fnmatch) | CodeQL language |
        |---|---|
        | `.github/workflows/*.yml`, `.github/workflows/*.yaml` | `actions` |
        | `services/*/go.mod` | `go` |
        | `tools/pyproject.toml`, `sdk/python/pyproject.toml`, `plugins/*/pyproject.toml` | `python` |
        | `sdk/typescript/package.json`, `plugins/*/package.json` | `javascript-typescript` |

        `tools/node/package.json` is excluded on purpose: it only pins Node tools and has no source.

    - `ALLOWED_BUILD_MODES = {"actions": {"none"}, "python": {"none"},
      "javascript-typescript": {"none"}, "go": {"manual"}}` with a comment citing the GitHub page
      above and the reason `autobuild` is not allowed for Go.
    - `tracked_files() -> list[str]`: `git ls-files` in `REPO` (tracked files only, like the
      `Makefile`; subprocess with a fixed argument list, no shell, `check=True`).
    - `required_languages(tracked: Iterable[str]) -> set[str]`.
    - `codeql_findings(doc: dict[str, Any], tracked: Iterable[str]) -> list[str]`: one readable
      finding per violation:
        - the `analyze` job has no `strategy.matrix.include` list, or an entry lacks `language` or
          `build-mode`;
        - a required language is missing from the matrix;
        - a build mode is not in `ALLOWED_BUILD_MODES` for its language (an unknown language too);
        - the job `if` is not the public guard (`check_runners.has_public_guard`);
        - for every tracked `services/<m>/go.mod`: no step `actions/setup-go@...` with
          `if: matrix.language == 'go'` and `with.go-version-file: services/<m>/go.mod`; no `run`
          step `go build ./...` with `working-directory: services/<m>` and
          `if: matrix.language == 'go'`; or the build step is not after the `init` step and before
          the `analyze` step;
        - the `init` step does not pass `build-mode: ${{ matrix.build-mode }}`.
    - Tests (names fixed):
        - `test_required_languages_mapping` (parametrized: each row of the table, plus
          `tools/node/package.json` and `apps/client/pubspec.yaml` mapping to nothing).
        - `test_matrix_without_include_is_rejected` (plain `language: [go]` list).
        - `test_go_build_mode_none_is_rejected`, `test_go_autobuild_is_rejected`.
        - `test_missing_required_language_is_rejected` (synthetic tracked list with
          `sdk/typescript/package.json`).
        - `test_go_setup_step_must_use_module_go_mod`, `test_go_build_step_must_run_in_module`,
          `test_go_build_step_must_sit_between_init_and_analyze`.
        - `test_init_must_pass_matrix_build_mode`.
        - `test_job_must_keep_public_guard` (guard removed or weakened).
        - `test_compliant_synthetic_workflow_passes` (one fixture that satisfies every rule, so the
          negative tests cannot pass for a wrong reason).
        - `test_repository_codeql_workflow_has_no_findings`: `codeql_findings(<parsed
          .github/workflows/codeql.yml>, tracked_files()) == []`; it first asserts that at least one
          `services/*/go.mod` is tracked, so it cannot pass vacuously.
    - Each negative test asserts on the text of the expected finding, not only on a non-empty list.
3. `docs/quality/public-switch.md`, step 18 "Probelauf": name the checks to look at after the
   first public pull request: jobs `analyze (actions)`, `analyze (go)`, `analyze (python)` green,
   and code scanning shows results for the category `/language:go`. German, like the file.
4. `CHANGELOG.md` (root, section `[Unreleased]`, subsection `### Changed`): one line, for example
   "CodeQL analyses the Go server module (manual build of `services/server`) in the public phase;
   a test keeps the CodeQL languages in step with the tracked components."

No other file changes. No ADR: ADR 0039 holds the decision, and the reasons live in the workflow
comment.

## Out of scope

- `javascript-typescript` in the matrix: added by the ticket that adds the first TypeScript project
  (`sdk/typescript`, Etappe M2); the new test will force it.
- Dart (not supported by CodeQL), custom CodeQL queries, a CodeQL config file, path filters.
- Enabling code scanning, rulesets or `BOTHY_VISIBILITY` on GitHub (public-switch runbook, V1).
- Any change to `tools/ci/check_runners.py`, `ci.yml`, `scorecard.yml` or the `Makefile`.
- Running CodeQL at all before the repository is public (see Boundaries).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-74-codeql-go`.
- Conventional Commits with DCO sign-off (`git commit -s`), scope `ci`, for example
  `ci(codeql): analyse Go with a manual build of the server module`.
- Never push, tag, open pull requests, create remote repositories or trigger workflows on GitHub.
- Do not run the CodeQL CLI, a CodeQL bundle or `codeql-action` locally: the CodeQL terms exclude
  analysis of a codebase that is not released open source, and this repository is private until
  V1. The workflow is proven by lint, policy check and tests now, and by its first run after the
  public switch (public-switch.md step 18).
- Do not modify files outside the scope list.
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
  Temporary directories only via `mktemp -d`, removed at the end. Never read or print secrets.
  No network calls except package downloads for pinned tools (`make bootstrap`, Go modules).

## Design notes

- Why `manual` and not `autobuild` for Go: the autobuilder first tries `make` at the repository
  root; the root `Makefile` has `help` as its default goal, so `make` succeeds without building Go,
  and the following `go list ./...` runs at a root without `go.mod`. The outcome would depend on
  fallback heuristics. A manual `go build ./...` in `services/server` is deterministic and is the
  same command `make typecheck` runs.
- Test files stay unanalysed: CodeQL skips `_test.go` by default and test code is not shipped. Do
  not set `CODEQL_EXTRACTOR_GO_OPTION_EXTRACT_TESTS`.
- `matrix.build-mode` is a valid expression (property names may contain `-`); actionlint accepts
  it. Keep every `run` step a single short command (gates.md: shellcheck is not pinned).
- `make lint` runs zizmor offline unless `GH_TOKEN` is set; report which mode ran. Facts zizmor
  relies on here: no untrusted context inside `run`, no cache (`cache: false`),
  `persist-credentials: false`, `security-events: write` only at job level.
- The job condition must stay equal to `PUBLIC_GUARD` (whitespace aside); `make runners-check` and
  `test_job_must_keep_public_guard` both check it.
- `test_actions_table_matches_workflows` requires one version per action across all workflows:
  copy the SHA and the `# v7.0.0` comment of `actions/setup-go` exactly as in `ci.yml`.
- Type hints everywhere in the new test module (mypy strict runs over `tools/`); `ruff` and
  `bandit` stay clean.

## Tests (write them first)

1. Write `tools/ci/tests/test_codeql_workflow.py` completely. Run
   `cd tools && uv run --locked pytest ci/tests/test_codeql_workflow.py -q`: every synthetic test
   passes, and `test_repository_codeql_workflow_has_no_findings` fails with findings that include
   "go missing from the matrix" and "init does not pass the matrix build mode" (record this red
   output for the report).
2. Change `codeql.yml`. The same command is green.
3. Then all of:
    - `cd tools && uv run --locked pytest ci/tests docs/tests/test_toolchain_doc.py -q`
    - `make runners-check`
    - `make lint`
    - `cd services/server && CGO_ENABLED=0 go build ./...` (the exact build the workflow runs)
    - `make check`

## Acceptance criteria

- [ ] `codeql.yml` has the `include` matrix with `actions`/`none`, `go`/`manual`,
      `python`/`none`; `init` passes `build-mode: ${{ matrix.build-mode }}`.
- [ ] Go entry: pinned `setup-go` with `go-version-file: services/server/go.mod` and
      `cache: false`, then `init`, then `go build ./...` in `services/server` with
      `CGO_ENABLED=0`, then `analyze`; both Go steps carry `if: matrix.language == 'go'`.
- [ ] Job `if`, triggers, permissions, concurrency, runner and timeout are unchanged.
- [ ] `test_codeql_workflow.py` was red before the workflow change and is green after it; every
      negative test fails for the reason in its name (asserted on the finding text).
- [ ] `make runners-check`, actionlint and zizmor (via `make lint`) are green.
- [ ] `CGO_ENABLED=0 go build ./...` succeeds in `services/server`.
- [ ] `docs/quality/public-switch.md` step 18 names the CodeQL checks; root `CHANGELOG.md` has the
      entry; the workflow header comment states the language rule and the reason for `manual`.
- [ ] No CodeQL binary was downloaded or run; no workflow was triggered.
- [ ] `make check` is green.

## Definition of done

Code, tests and docs as above: module docstring and doc comments on every helper in the new test
module, workflow comment explaining why, `CHANGELOG.md` entry under Unreleased, public-switch
runbook updated; no ADR needed (ADR 0039 holds the decision). `make check` green. Independent
review (`make review` plus a code review over the diff) and the red team gate
(`docs/quality/red-team-gate.md`) over the diff with the verdict line recorded in the report;
merge locally into the base branch only if the review is clean, the verdict is `JA` and
`make check` is green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits (SHA and subject), files with line counts, test results with counts (the red run of step 1
and the green runs), the zizmor mode (online or offline), what is not proven (the workflow has not
run on GitHub; CodeQL results appear only after the public switch), and follow-ups that could not
be done in scope with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or
`exit=1 <reason>`.
