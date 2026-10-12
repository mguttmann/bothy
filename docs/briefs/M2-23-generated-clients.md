# M2-23: Generated TypeScript and Python clients as a gate (G4, G5)

Status: ready | Depends on: M1-17 | Milestone: M2

## Goal

After this ticket `sdk/python` and `sdk/typescript` are real components with a generated core. openapi-python-client 0.29.1 and openapi-typescript 7.13.0 (with openapi-fetch as the runtime client), pinned inside each component, generate the clients from `docs/api/openapi.yaml`; the generated code is checked in; `make -C sdk/python generate-check` and `make -C sdk/typescript generate-check` regenerate it, fail on any drift and compile it; and `make api-clients` runs both in every `make check` (gate G4). A small hand-written tolerance layer per SDK makes both clients accept what a newer server may send: an unknown field is ignored, an unknown enum value is kept as its raw string, an event of an unknown type is skipped while the cursor still advances, and a received JSON `null` counts as absent; one test per SDK proves this on shared fixtures (gate G5, ADR 0053). Why: a new enum value or event type is only an additive change (06 "Versioning, deprecation, compatibility") if every client tolerates it, and openapi-python-client throws on unknown values (06 13 point 3, RK-15). Without this gate a spec change would break the SDKs, the adapters and apps built on them without anyone noticing, and M2-29 and M2-30 would build their hand layers on an unchecked core.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 5 "Test strategy" (row "Contract": G1 to G6 as own gates in `make check`); section 8 "M2 Agent platform, extensions and terminal": "Goal" (apps and webhooks use the same public API, P-18), "Exit criteria" item 4 (G4 and G5 for TypeScript and Python through `make api-clients`, M2-23), "Test strategy" ("contracts G4, G5, G10"), "Risks" ("the Python generator throws on unknown values (hand-written layer, G5)"); block "agents and extensions" rows M2-23, M2-29, M2-30, M2-31, M2-37; section 10 row M4-02 (the Dart client, same gate); section 7 "Test strategy" ("G4 and G5 follow with the SDK clients"); section 19 P-04 (the enable list governs the server only), P-09, P-10, P-11.
- `docs/spec/06-api.md`: 2 "Basic rules" (rows "No `null`", "Unknown values", "Language"); 11 "Real-time gateway" point 3 (`Event` is the union of all durable types with discriminator `type`); 13 "Versioning, deprecation, compatibility" point 3 (Python only with a hand layer; openapi-python-client 0.29.1 throws on unknown values, also with `literal_enums`); 14 "Code generation" (generator table, points 1 to 4); 15 "Contract tests and gate" (rows G4 and G5, point 3); 19 "Decisions of this chapter" (R-API-04, R-API-05, R-API-06, R-API-18).
- `docs/spec/11-extensions.md`: 11 "SDK design" (11.1 "Generated core": the TypeScript runtime client is "choice in the build ticket"; 11.2 "Hand-written layer"; 11.3 "Runtimes and dependencies"); 12 "Versioning and compatibility"; 14 "Decisions of this chapter" (R-EX-10, R-EX-11); 16 "Configuration values of this chapter" (`sdk.*`, used by M2-29 and M2-30, not here).
- `docs/spec/04-sync-protocol.md` (core contract, read only): 6.1 "Endpoint and version" ("Clients ignore what they do not know"); 7.1 "Cursor"; 7.4 "REST variant" (`{events, next_cursor, head_seq, sync_epoch}`).
- `docs/spec/23-quality.md`: 1 "Principles" (points 3 to 5); 2 "Gates"; 3 "Component contract" (rows "Python project", "TypeScript project", "Generated client") with "Documentation in code"; 10 "Rulings" (R-QA-02, R-QA-14); 11 "Configuration values of this chapter".
- `docs/spec/02-architecture.md`: 6 "Code layout", the list "Places that carry the product name" (the only list of places where the working name may appear in code).
- `docs/spec/18-release.md`: 2 "Components" (rows `sdk-python`, `sdk-typescript`; rule 1: `VERSION` and `CHANGELOG.md` per component).
- `docs/spec/19-security.md`: SEC-SC-1. `docs/security/threat-model.md`: T-SC-01.
- `docs/api/openapi.yaml`: operations `listEvents` and `pollAgentEvents` (both answer `EventPage`), `getThread`; schemas `Event` (its `discriminator.mapping`), `EventPage`, `EventEnvelope`, `EventActor`, `ActorKind`, `EventScope`, `EventResource`, `ThreadUpdatedEvent`, `ThreadPayload`, `Thread` (with the inline enum `kind` and `state`), `ThreadState`, `ThreadMovedOutEvent`, `ThreadMovedOutPayload`, `Problem` (`code` is a string, unknown codes are handled by status). `docs/api/asyncapi.yaml` (references the schemas of openapi.yaml; no generator, 06 14 point 3). `docs/api/README.md` ("Change").
- ADRs: 0013 (spec first, generated code checked in), 0014 (versioning; "Confirmation": G5), 0026 (generated core plus hand layer; "Confirmation"), 0038 (one local gate command identical to CI), 0041 (Renovate), 0044 (working name only at listed places), 0052 (one event envelope), 0053 (no JSON `null`; "Confirmation": G5 skips a received `null`).
- `docs/DECISIONS.md`: D-R-CLIENTS, D-API-13, D-QA-22, D-QA-25, D-W-54, D-W-100, section 10 "Rulings of the build milestones" (row format `D-M2-23-<n>`).
- `docs/RISKS.md`: RK-15, RK-21. `docs/OPEN-QUESTIONS.md`: OQ-17 (package names and accounts at PyPI and npm; default: reserve only after the final name; the plan builds the SDKs without publishing).
- `docs/quality/gates.md` (rows `api-clients`, `coverage`, `lint`, `typecheck`, `licenses`; section "Skipping and failing": a missing part of the component contract is an error), `docs/quality/toolchain.md` ("Gate tools"), `docs/quality/licenses.md` ("License gate (`make licenses`)", point 3: `PackageOverrides` only per package with reason), `docs/quality/public-switch.md` (step 18 "Trial run").
- `sdk/README.md`, `sdk/python/README.md`, `sdk/typescript/README.md` (status "planned" today).
- M1 briefs and merged code this ticket builds on (read the merged versions on the base branch):
    - M1-01: `tools/ci/codegen_check.py` (`--dir <path> -- <command>`, snapshot of every file git would see before and after the generator; exit 1 with `codegen-check: generated code in <path> was stale; commit the regenerated files`), the root `Makefile` (variables `CODEGEN_CHECK`, `PY_PROJECTS`, `TS_PROJECTS`, `PY_ON`, `TS_ON`, function `want`, targets `api-clients`, `bootstrap-ci`, `fmt-check`, `lint`, `typecheck`, `test`, `coverage`, `licenses`), `tools/ci/tests/test_make_server_gates.py` (pattern for testing make recipes with fakes).
    - M1-17: the Go server stubs generated from the same spec (the precedent for generator pins inside the component: oapi-codegen as a `tool` directive in `services/server/go.mod`), `services/server/api/rest/rest_test.go` (`TestEventUnion_UnknownTypeDecodes`: the Go side of the same tolerance), the coverage exclusion of `*.gen.go`, `tools/ci/components.py` (`GATE_COMPONENTS`, `GATE_COMPONENT_INPUTS`, `API_PREFIX`), `tools/ci/license_check.py` (`GENERATED` patterns, `*/generated/*` is exempt from the SPDX header), `tools/docs/tests/test_toolchain_doc.py` (`pinned_tools`, `go_tools`), `sonar-project.properties` (`**/generated/**` excluded, coverage report paths `sdk/python/coverage.xml` and `sdk/typescript/coverage/lcov.info`, test inclusions `**/tests/**`, `**/test/**`, `**/*.test.ts`).
    - M1-03: `make api-checks` (G2) and `make api-tables` (G3), which keep `openapi.yaml` consistent before any client is generated from it.
    - M1-74 (merged on the base branch, not in the dependency closure): `.github/workflows/codeql.yml` and `tools/ci/tests/test_codeql_workflow.py`, which require a matrix entry for every tracked component language and name the first TypeScript project as the one that adds `javascript-typescript`; the runbook check of step "Trial run".
- Also read: `tools/docs/tests/test_config_values.py`, `tools/ci/tests/test_components.py`, `.pre-commit-config.yaml` (hygiene hooks run over all tracked files, `check-added-large-files` with `repo.max_file_kb`, `text-rules`), `.gitignore`, `renovate.json`, `.github/workflows/ci.yml` (jobs `gates` and `sonar` run `bootstrap-ci` and then `make gates` or `make sonar`), `quality/licenses/allowed.txt`.
- Implied dependencies: the header names only M1-17; M1-03, M1-05, M1-02 and M1-01 are in its closure and therefore merged before this brief starts.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

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

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. G4: both clients are generated in the gate from `docs/api/openapi.yaml` with the pinned generators of 06 14 (Python: openapi-python-client 0.29.1; TypeScript: openapi-typescript 7.13.0 with openapi-fetch), compiled, and the gate fails on any difference to the checked-in code (06 15 row G4, 06 14 point 1, R-API-18, ADR 0013). The generated core is never changed by hand (11 11.1, R-EX-10).
12. Component contract (23 3): each client directory has a `Makefile` with the target `generate-check` ("generate from `docs/api/`, compile; a deviation from the checked-in state reports an error"); the Python project has `pyproject.toml` with a uv lock and ruff, mypy and pytest in the dev group and its coverage threshold in its own pytest configuration; the TypeScript project has `package.json` with the scripts `format`, `format:check`, `lint`, `typecheck`, `test` and the threshold in its test runner. A missing part fails instead of skipping (23 1 point 3, R-QA-02, D-QA-22, gates.md "Skipping and failing").
13. G5: one test per SDK with fixtures: an unknown field, an unknown enum value and an unknown event type decode without an exception (06 15 row G5; 06 2 row "Unknown values"; R-API-04; 04 6.1; ADR 0014 "Confirmation"), and a received JSON `null` is treated as an absent field (06 2 row "No `null`"; ADR 0053 point 5 and "Confirmation").
14. Python tolerates unknown values only through the hand layer; the generator runs with its standard settings (06 13 point 3, 06 14 row "Python SDK and Hermes adapter", ADR 0014 "Consequences", ADR 0026).
15. A client skips an event of unknown type and still advances its cursor (`Event` description in openapi.yaml; 04 7.1 and 7.4).
16. Dependencies are locked with checksums and pass the license and vulnerability gates (SEC-SC-1, T-SC-01, licenses.md "License gate (`make licenses`)" point 3).
17. Each new component has `VERSION` and `CHANGELOG.md` (18 2 rule 1).
18. Doc comments on every public symbol of the hand-written SDK code; for Python ruff `D` with convention `google`, tests excluded (23 "Documentation in code", R-QA-14). The Python API reference in MkDocs (rule 3) and the typedoc gate come with M2-29 and M2-30.

## Scope

Paths are relative to the repository root. This ticket creates no new top-level make target: `api-clients` exists; the new targets `generate` and `generate-check` live in the component Makefiles.

Create:

- `sdk/python/pyproject.toml`, `sdk/python/uv.lock`: project `bothy-sdk`, `version = "0.0.0"`, `license = "Apache-2.0"`, src layout, the runtime dependencies the generated code imports (Design note 11), dev group with the exact pin `openapi-python-client==0.29.1` and ruff, mypy, pytest, pytest-cov, PyYAML (for the spec tests) and type stubs; `[tool.ruff]`, `[tool.mypy]`, `[tool.pytest.ini_options]`, `[tool.coverage.*]` per Design notes 8 and 10.
- `sdk/python/openapi-python-client.yaml`: generator configuration (Design note 3).
- `sdk/python/Makefile`: targets `generate` and `generate-check` (Design note 3), header comment like `tools/design-tokens/Makefile`.
- `sdk/python/src/bothy_sdk/__init__.py`, `sdk/python/src/bothy_sdk/py.typed`, `sdk/python/src/bothy_sdk/tolerance.py` (Design note 5).
- `sdk/python/src/bothy_sdk/generated/`: generated core, written only by `make -C sdk/python generate`.
- `sdk/python/tests/conftest.py`, `sdk/python/tests/test_tolerance.py`, `sdk/python/tests/test_generated_core.py`.
- `sdk/python/VERSION` (`0.0.0`), `sdk/python/CHANGELOG.md` (Keep a Changelog, like `services/server/CHANGELOG.md`).
- `sdk/typescript/package.json`, `sdk/typescript/package-lock.json`, `sdk/typescript/tsconfig.json`, `sdk/typescript/eslint.config.mjs`, `sdk/typescript/.prettierignore`, `sdk/typescript/vitest.config.ts` (Design notes 6, 10 and 12).
- `sdk/typescript/Makefile`: targets `generate` and `generate-check`.
- `sdk/typescript/src/index.ts`, `sdk/typescript/src/tolerance.ts` (Design note 6).
- `sdk/typescript/src/generated/openapi.ts`: generated types, written only by `make -C sdk/typescript generate`.
- `sdk/typescript/test/tolerance.test.ts`, `sdk/typescript/test/generated.test.ts`.
- `sdk/typescript/VERSION` (`0.0.0`), `sdk/typescript/CHANGELOG.md`.
- `sdk/testdata/tolerance/unknown_field.json`, `unknown_enum_value.json`, `unknown_event_type.json`, `null_member.json` and `README.md` (English, like the other READMEs under `sdk/`: what each fixture holds and which test reads it) (Design note 7).
- `tools/ci/tests/test_make_api_clients.py` (Design note 9 and the tests below).
- `sdk/python/osv-scanner.toml` or `sdk/typescript/osv-scanner.toml` only if OSV-Scanner reports a license as `UNKNOWN` or non-standard (Design note 13).

Change:

- `Makefile` (root): `api-clients` fails closed and respects `COMPONENTS`; `bootstrap-ci` installs the SDK dependencies; the Python lines of `typecheck` and `lint` (Design note 9).
- `tools/ci/components.py` and `tools/ci/tests/test_components.py`: `sdk/testdata/` selects both SDK components (Design note 7).
- `tools/docs/tests/test_config_values.py`: sources of the two new coverage keys (Design note 10).
- `tools/docs/tests/test_toolchain_doc.py`: generator pins read from `sdk/python/uv.lock` and `sdk/typescript/package-lock.json`, and 06 14 names the pinned versions (Design note 2).
- `.github/workflows/codeql.yml`: matrix entry `language: javascript-typescript`, `build-mode: none`; header comment updated (the first TypeScript project is now tracked). `docs/quality/public-switch.md` step 18 "Trial run": name `analyze (javascript-typescript)`.
- `.gitignore`: `sdk/**/coverage/`.
- `renovate.json`: a package rule for the generator pins (Design note 13).
- `docs/spec/06-api.md` 14 (table cell of the TypeScript row with the openapi-fetch pin; where the generated code lives; the Python tolerance hook) and 15 (row G5, Design note 14); `docs/spec/11-extensions.md` 11.1 (runtime client openapi-fetch; gateway types from the schemas of openapi.yaml that asyncapi.yaml references, as 06 14 point 3 says); `docs/spec/23-quality.md` 3 (rows "Python project" and "TypeScript project": coverage keys, generated code outside coverage and style lint) and 11 (two keys); `docs/spec/02-architecture.md` 6 (list "Places that carry the product name": the SDK package names).
- `docs/quality/gates.md` (rows `api-clients`, `coverage`, `lint`, `typecheck`), `docs/quality/toolchain.md` ("Gate tools": rows `openapi-python-client` and `openapi-typescript`; the intro sentence names the SDK locks as pin sources), `docs/api/README.md` ("Change" point 5: regenerate both SDK cores with `make -C sdk/python generate` and `make -C sdk/typescript generate`).
- `sdk/README.md` (table "State", section "Package names"), `sdk/python/README.md`, `sdk/typescript/README.md` (status, layout, how to regenerate, what the tolerance layer guarantees and what it does not), all in English like today.
- `docs/DECISIONS.md` section 10: rows `D-M2-23-1` and following (Design note 15).
- `CHANGELOG.md` (root) under "Unreleased": the tool chain changes (`api-clients` fails closed, SDK dependencies in `bootstrap-ci`, CI path filter for `sdk/testdata/`, CodeQL for TypeScript).
- If `tools/ci/tests/test_product_name_places.py` (M1-46) exists on the base branch: allow the SDK package names as the new kind from 02 6 there.

## Out of scope

- The hand layer modules `auth`, `gateway` (frame router, ephemeral events and gateway ops), `invocation`, `runs`, `components`, `commands`, `outbox`, `retry`, `heartbeat`, `webhooks`, `testing` with the FakeServer, the client defaults from `sdk.http_timeout`, `sdk.retry_max_attempts`, `sdk.retry_backoff_base` and `sdk.retry_backoff_cap`, the deprecation warning of 11 12, the MkDocs API reference (mkdocstrings) and the gate test "a public function without docstring fails `make lint`": M2-29 (Python).
- The same for TypeScript plus the webhook signature verifier, the typedoc gate, the typedoc reference and the build output (`dist/`): M2-30.
- The Dart client `packages/api_client` with its G5 test: M4-02. The Go server stubs and `make codegen-check`: M1-17.
- The breaking-change check against the last release (G6): M7-11. Publishing to PyPI or npm and the final package names: OQ-17, V1-06; the release chain of the SDK components: the release milestone (M7).
- Tolerance of unknown gateway ops and close codes: M2-29 and M2-30 (their gateway modules).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-23-generated-clients`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(sdk-python): ...`, `feat(sdk-typescript): ...`, `build(gates): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a package to PyPI or npm and never reserve a name there (OQ-17); both packages stay private (Design note 11).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests open no listener (if one ever becomes necessary, only `127.0.0.1` and port 0, P-10) and call no server: REST calls in the SDK tests go through an in-process fake transport (`httpx.MockTransport`, a fake `fetch`) with base URL `https://example.com/api/v1`; no real third-party service and no real server of this product is contacted. No PostgreSQL is needed.
- Tests never modify the working tree: the drift tests run on temporary copies (Design note 9).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (or the test framework's temporary directory), removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools, Python packages and npm packages; npm installs run with `--ignore-scripts` like `tools/node` (T-SC-01).
- Generators and dev tools are pinned to exact versions in the component manifests and locks; no `latest`, no unpinned `npx` or `uvx` call in a Makefile or script.
- Never edit generated files by hand; regenerate. Do not exclude generated files from the pre-commit hygiene hooks and do not raise `repo.max_file_kb`; if the pinned generator's output cannot pass them after the deterministic normalization of Design note 3, stop with `Verdict: BLOCKED <reason>`.
- Do not extend `quality/licenses/allowed.txt`; choose dependencies whose whole tree passes it. If a pinned generator or required tool cannot, stop with `Verdict: BLOCKED <package and license>`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. Layout and names. Ruling for docs/DECISIONS.md: Python distribution `bothy-sdk`, import package `bothy_sdk` in `sdk/python/src/bothy_sdk/`, generated core in the subpackage `bothy_sdk.generated` (`sdk/python/src/bothy_sdk/generated/`); npm package `bothy-sdk` with `"private": true`, `"type": "module"`, generated types in `sdk/typescript/src/generated/openapi.ts`. Reasons: the directory name `generated` is already excluded by Sonar (`**/generated/**`) and exempt from the SPDX header check (`*/generated/*` in `tools/ci/license_check.py`), so no gate configuration changes for it; the names follow the Dart packages `bothy_viz` and `bothy_noise` and become a listed kind in 02 6 "Places that carry the product name" (ADR 0044), so V1-06 renames them with the other name-bound places; they are working names until OQ-17 is decided. Add the classifier `Private :: Do Not Upload` to `pyproject.toml` so an accidental upload is refused.

2. Generator pins. Ruling for docs/DECISIONS.md: each generator is pinned inside its component, `openapi-python-client==0.29.1` in the dev group of `sdk/python/pyproject.toml` (locked in `sdk/python/uv.lock`) and `openapi-typescript` `7.13.0` exactly in `devDependencies` of `sdk/typescript/package.json` (locked in `package-lock.json`), like oapi-codegen in `services/server/go.mod` (M1-17). Reason: the generator version belongs to the code it produced, Renovate updates it next to that code, and the license and OSV gates see it. Fallback, only if the license gate rejects a dependency that only the Python generator pulls in: pin it in `tools/pyproject.toml` instead (development tools are outside the allowlist, `quality/licenses/allowed.txt` header) and record the reason in the report. `docs/quality/toolchain.md` gets one row per generator (version, purpose "core of the Python SDK or types of the TypeScript SDK, respectively, from `docs/api/openapi.yaml`, runs in `make api-clients`", license, reason "06 14, ADR 0026"); extend `pinned_tools()` and `test_every_pinned_gate_tool_is_documented` in `tools/docs/tests/test_toolchain_doc.py` to read these two pins from the SDK locks, and add a test that the TypeScript and Python rows of the table in 06 14 name exactly the pinned versions of openapi-typescript, openapi-fetch and openapi-python-client.

3. Targets `generate` and `generate-check` (per SDK Makefile; `SHELL := bash`, `.SHELLFLAGS := -euo pipefail -c` like the root Makefile).
    - Variables with defaults that a test can override: `SPEC ?= ../../docs/api/openapi.yaml` and `TOOLS ?= ../../tools`.
    - `generate` regenerates in place from `$(SPEC)`: Python with `uv run --project . --locked openapi-python-client generate` with `--meta none`, `--overwrite`, `--config openapi-python-client.yaml` and the output path of the generated subpackage (check the flags against `--help` of the pinned version); TypeScript with the local `openapi-typescript` binary from `node_modules/.bin` and output `src/generated/openapi.ts`. Standard generator settings (06 14 column "Settings"): in particular `literal_enums` stays off for Python, because the tolerance of Design note 5 needs enum classes, and openapi-typescript runs without flags that change the output form.
    - Determinism: two runs on the same spec give byte-identical files, wherever they run. Set the post hooks of openapi-python-client explicitly in its configuration (an explicit ruff configuration path, or none) so that the output does not depend on the working directory. If the raw output of a pinned generator does not pass the pre-commit hygiene hooks (trailing whitespace, final newline, line endings, `text-rules`), `generate` normalizes it as its last step with a deterministic rewrite; `make lint` must find nothing to fix in generated files.
    - `generate-check` (G4): first the drift check with the M1-01 tool, `uv run --project $(TOOLS) --locked --quiet python $(TOOLS)/ci/codegen_check.py --dir . -- $(MAKE) generate`, so it reports `changed:`, `added:` and `removed:` lines and fails with exit 1 when the checked-in code differs from a fresh generation; then compile: Python `python -m compileall -q` over the package and `mypy` with the project configuration (generated code included); TypeScript `npm run typecheck` (`tsc --noEmit`, `strict: true`, generated file included). Reason for reusing `codegen_check.py`: one drift rule in the repository, and an uncommitted hand edit outside the generator's output is not drift (M1-01 Design note 4).
    - Size: a generated file larger than `repo.max_file_kb` fails the hook; see Boundaries.

4. The SDKs are generated from the full spec, all operations under `paths`, with no filter by `services/server/api/operations.yaml`: P-04 governs what the server registers, while 11 11.1 asks for "models and calls of all REST operations". A test (`test_g4_every_operation_is_generated`) fails if the Python generator skipped an operation (it can skip with only a warning); for TypeScript the `paths` interface holds every path by construction.

5. Python tolerance, `bothy_sdk.tolerance`. Ruling for docs/DECISIONS.md: tolerance through a hook that the hand layer installs on import, no custom generator templates (06 14 "default", 06 13 point 3 "only with the hand-written layer", R-EX-10 "the core is never changed by hand"). Public API, each with a docstring (google convention):
    - `install() -> None`: idempotent; `bothy_sdk/__init__.py` calls it, so importing any part of the SDK makes the generated core tolerant.
    - `KNOWN_EVENT_TYPES: frozenset[str]`: the discriminator values of the `Event` union in the generated core. Derive it from the generated code if possible, otherwise list it; either way a test holds it equal to the keys of `components.schemas.Event.discriminator.mapping` in `docs/api/openapi.yaml`.
    - `UnknownEvent`: frozen dataclass with `type: str`, `seq: int | None` and `raw: Mapping[str, Any]` (a new hand-layer type; the core has no type for it, ADR 0026 "Confirmation").
    - `decode_event(data: Mapping[str, Any]) -> <generated event model> | UnknownEvent`: classifies by `type` against `KNOWN_EVENT_TYPES` before any variant is tried; an unknown or missing `type` gives `UnknownEvent`, a known one the generated model of exactly that type.
    - `is_unknown(value: Enum) -> bool`: true for a value that a generated enum did not know.

    Behaviour that `install()` establishes, with the recommended mechanism (verify against the pinned generator's output):

    - Unknown enum value: every enum class of `bothy_sdk.generated.models` gets a `_missing_` hook that returns a pseudo-member carrying the raw value (`.value` equals the received string, `to_dict()` sends it back unchanged, it equals no known member). Exception, and the reason for the tolerance rule's only gap: the single-value `type` enum of every event schema composed from `EventEnvelope` (durable and ephemeral) stays strict. A tolerant `type` would let the union parser accept an unknown event as the first variant whose payload happens to fit, for example `{thread_id, channel_id}` as `thread.moved_out`; the fixture of Design note 7 contains exactly this case.
    - JSON `null`: the `from_dict` of every generated model drops members whose value is `None` before it delegates to the generated code (06 2 "No `null`", ADR 0053 point 5). Nested models are covered because the generated code calls their `from_dict`.
    - Unknown event type: `EventPage.from_dict` drops every item of `events` whose `type` is not in `KNOWN_EVENT_TYPES` (also an item without `type` or that is not an object) before it delegates, so `listEvents` and `pollAgentEvents` through the generated functions never raise, and the caller advances with `next_cursor` (04 7.4), as the `Event` description in openapi.yaml says ("Clients skip unknown types and still advance their cursor").
    - Unknown field: nothing to install; the generated models keep unknown members in `additional_properties` and ignore them otherwise (no response schema in openapi.yaml sets `additionalProperties: false`). The test proves it.

    The hook must not change behaviour for known values: a known enum value decodes to the generated member itself (identity), and a fully known page decodes as without the hook.

6. TypeScript runtime client and tolerance. Ruling for docs/DECISIONS.md: the runtime client is openapi-fetch (11 11.1 leaves the choice to this ticket; 06 14 probed openapi-typescript "with openapi-fetch"), a runtime `dependency` locked in `package-lock.json`; record its version in the TypeScript row of 06 14. TypeScript has no runtime validation (06 13 point 3), so unknown fields and enum values pass by construction; the hand layer `src/tolerance.ts` adds, each export with a TSDoc comment:
    - `KNOWN_EVENT_TYPES`: a `readonly` tuple of the discriminator values, checked both ways at compile time against `components["schemas"]["Event"]["type"]` (`satisfies` for "every listed value exists" and a `never` check of the `Exclude` for "every spec value is listed"), so `tsc` in `generate-check` fails when the spec gains or loses an event type and the list is not updated.
    - `type UnknownEvent = { readonly kind: "unknown"; readonly type: string; readonly seq?: number; readonly raw: unknown }` and `decodeEvent(raw: unknown)`, returning `{ kind: "known"; event: Event }` or an `UnknownEvent`, classified by `type` only.
    - `knownEvents(page: EventPage): Event[]`: the known events in order; unknown ones are skipped.
    - `toleranceMiddleware`: an openapi-fetch middleware that, for `application/json` and `application/problem+json` responses, removes object members whose value is `null` (JSON reviver) and returns an equivalent `Response`.
    - `src/index.ts` exports the generated types (`paths`, `components`, `operations`, `webhooks` as openapi-typescript names them), the tolerance API and `createApiClient(options)`, which creates the openapi-fetch client for `paths` and always registers `toleranceMiddleware`. Authentication, DPoP, timeouts and retries are M2-30.

    The difference to Python is deliberate and goes into the README: in Python the generated page already skips unknown events (otherwise it would raise); in TypeScript the page is passed through and `knownEvents` or `decodeEvent` skip them.

7. Fixtures. Ruling for docs/DECISIONS.md: one shared set in `sdk/testdata/tolerance/`, read by the tests of both SDKs, so both prove the same cases (06 15 G5 "one test with a fixture per SDK"); `tools/ci/components.py` adds the prefix `sdk/testdata/` to both entries `sdk-python` and `sdk-typescript` of `GATE_COMPONENTS`, so a fixture change runs both SDK gates in CI (`sonar-project.properties` already excludes `**/testdata/**`). Each fixture is a complete `EventPage` body of `listEvents` that is valid against openapi.yaml except for the one deliberate unknown it is named after, with RFC 3339 times in UTC with milliseconds, lower-case UUIDs, no JSON `null` outside `null_member.json`, no names beyond the people of rule 8:
    - `unknown_field.json`: one `thread.updated` event (payload `ThreadPayload` with a complete `Thread`) with an extra member on the page, on the envelope, on the payload and on `thread` (one with an object value).
    - `unknown_enum_value.json`: one `thread.updated` event with values outside the spec for `thread.state` (named schema `ThreadState`), `thread.kind` (inline enum) and `actor.kind` (`ActorKind` inside `EventActor`).
    - `unknown_event_type.json`: three events with ascending `seq`: `thread.updated`, then type `future.example` whose payload is `{thread_id, channel_id}` (it fits `ThreadMovedOutPayload`), then `thread.updated`; `next_cursor` and `head_seq` equal the last `seq`.
    - `null_member.json`: one `thread.updated` event with `null` for the optional `correlation_id` of the envelope and the optional `thread.closed_at`.

    The chosen unknown names and values must really be unknown: the tests check them against the spec (Python by reading `docs/api/openapi.yaml`, TypeScript with `// @ts-expect-error` assignments to the generated types), so the tests fail instead of silently passing when the spec later gains such a value.

8. Gate scope of generated code. Ruling for docs/DECISIONS.md: generated code is excluded from style lint and formatting checks (ruff format and ruff check through `extend-exclude`, Prettier through `.prettierignore`, ESLint through its ignore list), from bandit (its union parsers use `try`/`except`/`pass` by design), from coverage (pytest-cov `omit`, Vitest coverage `exclude`) and from Sonar (already `**/generated/**`); it stays in the drift check, the compile and type checks (mypy, tsc), the import test, the pre-commit hygiene hooks, gitleaks, the denylist scan, the license and OSV gates of its dependencies and CodeQL. Reason: like `*.gen.go` in M1-17 and generated Dart code in 23 "Documentation in code", its correctness is held by its generator and G4, while hand rules (docstrings, style, coverage) would only measure the generator's templates. Hand-written code, including tests, follows every rule; ruff `D` with convention `google` is active from this ticket for the hand-written SDK code (tests excluded, 23 "Documentation in code"), M2-29 adds its gate test.

9. Root Makefile.
    - `api-clients` (contract of 23 3 and gates.md): for each client directory (`sdk/python` with component `sdk-python`, `sdk/typescript` with `sdk-typescript`, `packages/api_client` with `apps`; keep the list in one variable so a test can replace it): no tracked manifest (`pyproject.toml`, `package.json`, `pubspec.yaml`) gives a `SKIP` line; a component not selected by `COMPONENTS` gives a `SKIP` line like the other component gates (23 4); a tracked manifest without a `Makefile` that defines `generate-check` fails with a message naming the directory and the contract (today it skips silently, which contradicts gates.md "Skipping and failing" and D-QA-22); otherwise `$(MAKE) -C <dir> generate-check`, and its failure fails the gate. Update the help text to "G4".
    - `bootstrap-ci`: when `sdk/typescript/package-lock.json` is tracked, `npm ci --prefix sdk/typescript --ignore-scripts --no-audit --no-fund --loglevel=error`; when `sdk/python/uv.lock` is tracked, `uv sync --project sdk/python --locked --quiet`. CI runs `bootstrap-ci` in the jobs `gates` and `sonar`, so no workflow change is needed.
    - The first Python project exercises the Python lines of the root Makefile for the first time. Run each of `fmt-check`, `lint`, `typecheck`, `test` and `coverage` against it and fix what does not work; two defects are known: `typecheck` runs `uv run --project <p> --locked mypy` from the repository root, where mypy finds no configuration and no target (run it inside the project, as `test` does), and `lint` runs `bandit -r <p> -x <p>/tests`, which descends into `<p>/.venv` (exclude the project's `.venv` and `generated` directories, Design note 8). Same check for the TypeScript lines.

10. Coverage thresholds. 23 3 asks for the threshold in each project's own configuration, but 23 11 names no key for it, and thresholds exist only as named values (23 1 point 5). Ruling for docs/DECISIONS.md: add `quality.coverage.python_min_percent` ("Minimum coverage per Python project under `sdk/` and `plugins/`", held by `fail_under` in `[tool.coverage.report]` of each project's `pyproject.toml`) and `quality.coverage.typescript_min_percent` ("Minimum coverage per TypeScript project", held by `coverage.thresholds` in `vitest.config.ts`) to 23 11, each with the same standard value as `quality.coverage.go_min_percent`; extend `SOURCES` in `tools/docs/tests/test_config_values.py` so each tracked project's value equals the key; name the keys in gates.md row `coverage` and in a comment next to each value. pytest-cov enforces `fail_under` during `pytest --cov`; verify that a run below the threshold fails before you rely on it. Coverage counts hand-written code only (Design note 8). Chapter 23 keys are not part of `docs/operations/configuration.md` (`tools/docs/config_reference.py` excludes chapter 23 on purpose).

11. Versions, runtimes and dependencies. `VERSION` is `0.0.0` in both SDKs, like `services/server/VERSION`; a unit test per SDK holds the manifest version equal to `VERSION`. Python `requires-python` starts at the oldest CPython line that python.org lists as maintained on the build day, and `engines.node` at the oldest Node.js line in active LTS on the build day (11 11.3); record both lines and the date in the READMEs. Runtime dependencies are exactly those the generated code and the hand layer import (for the Python core typically httpx, attrs and python-dateutil; check the generated imports), declared as compatible ranges from the tested version as a library must, with the exact tested versions in the locks; every dev tool is pinned exactly. Ruling for docs/DECISIONS.md: use the same versions of ruff, mypy, pytest, pytest-cov and PyYAML as `tools/pyproject.toml`, so one rule set formats and checks all Python of the repository.

12. TypeScript tool chain. Ruling for docs/DECISIONS.md: TypeScript compiler with `strict: true` (06 14 probed `tsc --strict`), Prettier for `format` and `format:check`, ESLint with typescript-eslint (type-checked recommended rules) for `lint`, Vitest with the v8 coverage provider for `test`, writing `coverage/lcov.info` (the path `sonar-project.properties` reads); all exact pins, all with licenses on the allowlist. Test files are named `*.test.ts` under `test/` (Sonar test inclusions). Hand-written `.ts` and `.mjs` files carry the SPDX header (`tools/ci/license_check.py`). If Sonar counts the configuration files `vitest.config.ts` and `eslint.config.mjs` as uncovered product code, exclude exactly these two from coverage in `sonar-project.properties` with a comment giving the reason, and nothing else.

13. CI and supply chain.
    - CodeQL: with `sdk/typescript/package.json` tracked, `tools/ci/tests/test_codeql_workflow.py` (M1-74) requires the matrix entry `javascript-typescript` with `build-mode: none`; add it, update the header comment, and name `analyze (javascript-typescript)` in step 18 "Trial run" of `docs/quality/public-switch.md`. Python is already in the matrix.
    - Renovate: a package rule in `renovate.json` that groups `openapi-python-client` and `openapi-typescript` as "SDK code generators" with a `prBodyNotes` entry: regenerate with `make -C sdk/python generate` and `make -C sdk/typescript generate` and update the versions in 06 14 before merging; the pull request fails `make api-clients` until then, on purpose (the pattern of the release binaries rule).
    - Licenses: OSV-Scanner checks all lock files of both components (licenses.md point 3). A license reported as `UNKNOWN` or non-standard is checked in the package's LICENSE file and recorded as a `PackageOverrides` entry with reason in `osv-scanner.toml` of that component, never as a blanket rule.

14. Specification and documentation text (spec first, rule 10). 06 15 row G5 becomes: unknown field ignored, unknown enum value kept as its raw value, event of unknown type skipped while the cursor advances, received `null` treated as absent (ADR 0053 "Confirmation" already demands the last part; the row omits it today). 06 14: TypeScript row names openapi-fetch with its pinned version; a sentence that the Python hand layer installs the tolerance on import and the generator keeps its standard settings; where the generated code of each SDK lives. 11 11.1: the TypeScript row says "runtime client openapi-fetch" instead of "choice in the build ticket"; the sentence on gateway types says they come from the schemas of openapi.yaml that asyncapi.yaml references (06 14 point 3; 11.1 says "from `docs/api/asyncapi.yaml`" today). 02 6: a new kind "SDKs: Python distribution `bothy-sdk`, import package `bothy_sdk`, npm package `bothy-sdk` (not published until OQ-17)". 23 3: Python and TypeScript rows name their coverage key and that generated code counts toward neither coverage nor style lint. All text is English, like every file of the repository (chapters, READMEs, quality pages and CHANGELOGs; D-W-94, `AGENTS.md`).

15. Rulings for docs/DECISIONS.md, one row each with date, decision, reason and place: layout and names (note 1); generator pins in the component with the fallback (note 2); full spec without enable list (note 4); Python tolerance by an import hook without custom templates, with the strict `type` enums (note 5); openapi-fetch and the TypeScript tolerance API (note 6); shared fixtures and the CI path filter (note 7); gate scope of generated code and ruff `D` from this ticket (note 8); `api-clients` fails closed and respects `COMPONENTS` (note 9); the two coverage keys and their value (note 10); runtime lines, dependency ranges and shared tool versions (note 11); the TypeScript tool chain (note 12).

## Tests (write them first)

Python, `sdk/python/tests/` (pytest, run by the root `make test`; the generated functions are called with a generated client on `https://example.com/api/v1` whose `httpx` transport is an `httpx.MockTransport` answering with a fixture):

- `test_tolerance.py`:
    - `test_g5_unknown_field_is_ignored`: `unknown_field.json` through the generated `listEvents` function gives an `EventPage` with one typed `thread.updated` event; every known field equals the fixture; the unknown members appear only in `additional_properties`; no exception.
    - `test_g5_unknown_enum_value_keeps_raw_value`: `unknown_enum_value.json` decodes without exception; `thread.state`, `thread.kind` and `actor.kind` carry the raw strings as `.value`, equal no known member and `is_unknown` is true for them; `to_dict()` returns the raw strings unchanged; a known value in the same run decodes to the generated member itself.
    - `test_g5_unknown_event_type_is_skipped`: `unknown_event_type.json` through the generated `listEvents` function gives exactly the two `thread.updated` events in order; `next_cursor` is not lower than the `seq` of the unknown event; `decode_event` on the raw middle event returns `UnknownEvent` with `type == "future.example"` and its `seq`, and never a `thread.moved_out` model.
    - `test_g5_null_member_is_treated_as_absent`: `null_member.json` decodes without exception, and `correlation_id` and `thread.closed_at` are unset, exactly as if the members were missing.
    - `test_g5_poll_agent_events_is_tolerant`: the generated `pollAgentEvents` function with `unknown_event_type.json` behaves like `listEvents`.
    - `test_known_event_types_match_spec`: `KNOWN_EVENT_TYPES` equals the keys of `components.schemas.Event.discriminator.mapping` in `docs/api/openapi.yaml`.
    - `test_event_type_enums_stay_strict`: constructing the `type` enum of any event schema with an unknown value raises; every other generated enum accepts it.
    - `test_fixture_unknowns_are_unknown_to_the_spec`: the unknown event type, the three unknown enum values and the unknown member names of the fixtures do not occur in the corresponding places of `docs/api/openapi.yaml`.
    - `test_install_is_idempotent` and `test_known_page_is_unchanged_by_the_hook`.
- `test_generated_core.py`:
    - `test_g4_every_generated_module_imports`: walks `bothy_sdk.generated` and imports every module.
    - `test_g4_every_operation_is_generated`: every `operationId` under `paths` of `docs/api/openapi.yaml` has its generated function.
    - `test_version_matches_version_file`.

TypeScript, `sdk/typescript/test/` (Vitest, run by the root `make test`; `createApiClient` gets `baseUrl: "https://example.com/api/v1"` and a fake `fetch` answering `GET /events` with a fixture):

- `tolerance.test.ts`:
    - `G5: unknown field is ignored`: `unknown_field.json` through `createApiClient` returns the page without error; `decodeEvent` on its event gives `kind: "known"` with type `thread.updated`, and the known fields equal the fixture.
    - `G5: unknown enum value keeps its raw value`: the three values arrive unchanged as strings; no exception anywhere on the path.
    - `G5: unknown event type is skipped`: `knownEvents` returns the two `thread.updated` events in order; `decodeEvent` on the middle event returns `kind: "unknown"`, `type: "future.example"` and its `seq`, never `thread.moved_out`; `next_cursor` is not lower than that `seq`.
    - `G5: null member is treated as absent`: after the client, `"correlation_id" in event` and `"closed_at" in thread` are false.
    - `known event types are unique` (the compile-time checks of Design note 6 cover equality with the spec).
    - `fixture unknowns are unknown to the spec`: `// @ts-expect-error` assignments of the fixture's unknown event type to `Event["type"]`, of each unknown enum value to its generated type and of each unknown member name to `keyof` its schema, so `tsc` fails when the spec gains one of them.
- `generated.test.ts`: `G4: generated file carries the generator header` and `version matches VERSION file`.

Repository tools, `tools/ci/tests/`:

- `test_make_api_clients.py` (make recipes run against throwaway client directories, the pattern of `test_make_server_gates.py`):
    - `test_client_without_manifest_is_skipped`: `SKIP api-clients` naming the directory, exit 0.
    - `test_client_with_manifest_but_without_generate_check_fails`: non-zero exit, message names the directory and the missing target.
    - `test_failing_generate_check_fails_the_gate` and `test_passing_generate_check_passes`.
    - `test_unselected_component_is_skipped`: with `COMPONENTS=server` both SDK directories print `SKIP`, no `generate-check` runs.
    - `test_g4_spec_change_without_regeneration_fails[python]` and `[typescript]`: copy the SDK directory (without `.venv`, `node_modules` and coverage output; TypeScript gets a link to the real `node_modules`) into a temporary git repository with the repository `.gitignore`, write a copy of `docs/api/openapi.yaml` with one new optional property on schema `Thread`, run the real `make -C <copy> generate-check SPEC=<copy of the spec> TOOLS=<repository tools>`; assert a non-zero exit, the `codegen-check: generated code in` message and a `changed:` line naming the generated file of `Thread` (Python: the module of the `Thread` model; TypeScript: `src/generated/openapi.ts`). The real working tree stays untouched (assert it with `git status --porcelain` before and after).
- `test_components.py`: new case `sdk/testdata/tolerance/unknown_field.json` selects `sdk-python` and `sdk-typescript`.
- `test_codeql_workflow.py` (M1-74, unchanged): passes with the new matrix entry and the runbook line; run it red first by tracking `sdk/typescript/package.json` before the workflow change.
- `tools/docs/tests/test_config_values.py`: the two new keys equal `fail_under` in `sdk/python/pyproject.toml` and the thresholds in `sdk/typescript/vitest.config.ts`.
- `tools/docs/tests/test_toolchain_doc.py`: the generator rows match the SDK locks; the 06 14 versions match the pins.

Test IDs of the plan row:

- G4 (06 15): both generators produce, both outputs compile, and there is no difference to the checked-in code; proven by `make api-clients` (both `generate-check` targets), `test_g4_every_generated_module_imports`, `test_g4_every_operation_is_generated` and `test_g4_spec_change_without_regeneration_fails` for both SDKs.
- G5 (06 15, ADR 0053): per SDK a test with fixture shows that an unknown field, an unknown enum value and an unknown event type decode without an exception and that a received `null` counts as absent; proven by the `test_g5_*` tests in Python and the `G5:` tests in TypeScript.

Commands that must pass, from the repository root:

- `make -C sdk/python generate-check` and `make -C sdk/typescript generate-check`
- `make api-clients`
- `cd sdk/python && uv run --locked pytest`
- `npm --prefix sdk/typescript test`
- `cd tools && uv run --locked --quiet pytest ci/tests/test_make_api_clients.py ci/tests/test_components.py ci/tests/test_codeql_workflow.py docs/tests/test_config_values.py docs/tests/test_toolchain_doc.py`
- `make fmt-check`, `make lint`, `make typecheck`, `make coverage`, `make licenses`, `make osv`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `make api-clients` is green and runs `generate-check` of `sdk/python` and `sdk/typescript`; each regenerates from `docs/api/openapi.yaml`, compiles (mypy and import for Python, `tsc --noEmit` with `strict` for TypeScript) and finds no drift (G4).
- [ ] A spec change without regeneration fails `generate-check` of each SDK with a drift message naming the generated file, proven by the end-to-end tests on temporary copies.
- [ ] Per SDK a test decodes an unknown field, an unknown enum value and an unknown event type from the shared fixtures without an exception, keeps the raw enum value, skips the unknown event while the cursor advances, never classifies it as a known type, and treats a received `null` as absent (G5).
- [ ] Generators are pinned exactly inside the components, documented in `docs/quality/toolchain.md` and 06 14, and held by tests; the generated code is checked in and never edited by hand.
- [ ] `api-clients` fails for a client directory with a manifest but without `generate-check` and respects `COMPONENTS`; the Python and TypeScript lines of the root Makefile work against the new projects.
- [ ] Both SDKs meet the component contract of 23 3, with coverage thresholds under the new keys `quality.coverage.python_min_percent` and `quality.coverage.typescript_min_percent`; the license and OSV gates pass without a change to the allowlist.
- [ ] CodeQL covers `javascript-typescript`; the CI path filter selects both SDKs for `sdk/testdata/`.
- [ ] 06, 11, 23 and 02 are updated as in Design note 14; `VERSION` and `CHANGELOG.md` exist in both SDKs; nothing is published.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `sdk`, `sdk/python`, `sdk/typescript` and `sdk/testdata/tolerance` written or updated (layout, regeneration, generator pins, tolerance guarantees and the Python and TypeScript difference of note 6, what M2-29 and M2-30 add); `docs/api/README.md`, `docs/quality/gates.md`, `docs/quality/toolchain.md` and `docs/quality/public-switch.md` updated.
- Rulings of note 15 recorded in `docs/DECISIONS.md` (one row each with reason).
- `sdk/python/CHANGELOG.md`, `sdk/typescript/CHANGELOG.md` and the root `CHANGELOG.md` entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts (generated directories as one line each with their file count), test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found; include the Python and Node.js lines chosen under note 11 and any `PackageOverrides` entry with its reason.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
