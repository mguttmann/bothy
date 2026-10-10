# M1-71: Contract tests with Schemathesis (G7, G8)

Status: ready | Depends on: M1-49, M1-50, M1-62, M1-63 | Etappe: M1

## Goal

After this ticket every response of every Go integration test is validated against
`docs/api/openapi.yaml` (G8), and `make contract-test` runs Schemathesis against the server built
from the working tree for every operation of the enablement list, with the project checks
(problem format, `Correlation-Id`, no JSON `null`, 404 equality) and the schema fuzzing of SEC-T06.
The server is then held to its specification by construction: a drifting field, status or header
breaks the gate, and every failure prints a reproducible `curl` command.

## Read first

- `docs/spec/06-api.md`: "Grundregeln" (rows Kein `null`, Unbekanntes, Korrelation, Medientypen),
  "Authentifizierung je Prinzipal-Art" ("DPoP", "Step-up"), "Autorisierung in der API", "Fehler"
  (generated problem table), "Idempotenz", "Nebenläufigkeit", "Rate-Limits", "Codegenerierung",
  "Vertragstests und Gate" (G7, G8 and items 1 to 3).
- `docs/spec/19-sicherheit.md`: "Eingabevalidierung im Server" (SEC-IN-1, SEC-IN-2), "Tests"
  (SEC-T06).
- `docs/spec/08-identitaet.md`: "DPoP", "Step-up-Ablauf", "Konfigurationswerte dieses Kapitels"
  (`auth.dpop_required_for`, `auth.step_up_max_age` is in 03, `auth.access_token_ttl`).
- `docs/spec/23-qualitaet.md`: "Grundsätze", "Komponenten-Vertrag", "Konfigurationswerte dieses
  Kapitels"; `docs/quality/gates.md`; `docs/quality/toolchain.md`.
- `docs/PLAN.md`: section "M1 Fundament" ("Teststrategie", known contradiction W7), row M1-71,
  decision P-04 and P-09.
- `docs/api/openapi.yaml` (components `securitySchemes`, `headers/Correlation-Id`, `responses`,
  `schemas/Problem`), `services/server/api/operations.yaml` (enablement list).
- ADRs 0013, 0015, 0053.
- Code: `internal/platform/httpserver` (M1-18: decoder, problem writer, duplicate-key rejection and
  the problem code it uses), `api/rest` and `api/routes` (M1-17), `internal/platform/egress` (M1-22),
  `tools/docs/tests/test_config_values.py`, `tools/docs/tests/test_toolchain_doc.py`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every test is written and seen failing before the code that makes it pass.
7. No en dash and no em dash in any file; white label.
8. Configuration only through named keys from `docs/operations/configuration.md`; new gate values
   only as named values in 23 "Konfigurationswerte dieses Kapitels".
9. G7 (06 "Vertragstests und Gate"): property-based tests against the running server for status
   codes, media types, response schemas and headers of every operation; stateful sequences over
   derived links; project checks: problem format, `Correlation-Id`, no `null`, 404 equality for
   foreign and missing IDs.
10. G8: every response of every integration test matches the specification.
11. SEC-T06: schema fuzzing of all routes: no `5xx` except expected ones, unknown fields not
    accepted (W7: SEC-IN-1 wins for writing schemas), duplicate JSON keys rejected (SEC-IN-2).
12. 06 item 1 of "Vertragstests und Gate": DPoP proofs are built by a Python hook.

## Scope

1. G8 validator: package `internal/platform/httpserver/contractcheck` in files with the build
   constraint `//go:build contract_validate`, registered into the middleware chain of
   `internal/platform/httpserver` only in that build (a tagged file in `httpserver` that installs
   it; the untagged build has no reference to it).
2. G7 orchestrator: `services/server/test/contract/` with `contract_live_test.go`
   (`//go:build contract_live`), `broker.go` (credential broker), `README.md`.
3. Python package `tools/contract/` (in the tools environment): `dpop.py`, `broker_client.py`,
   `auth.py`, `checks.py`, `operations.py`, `reporting.py`, `live_contract.py` (the live
   Schemathesis module, never collected by default because its name does not match `test_*.py`),
   `README.md`; unit tests in `tools/contract/tests/`.
4. `tools/pyproject.toml`: add to the `dev` group `schemathesis==4.30.1` (the version probed in 06)
   and `cryptography` pinned to the current release at build time; refresh `tools/uv.lock` with
   `uv lock`; add `contract` to pytest `testpaths` and `pythonpath`, coverage `source`, mypy
   `files`. Rows for both in `docs/quality/toolchain.md` (section "Gate-Werkzeuge", with license).
5. `Makefile`:
   - `CONTRACT_MAX_EXAMPLES ?= 100` and new target `contract-test` (help `## Contract tests G7:
     Schemathesis against the server of this tree (Docker runtime)`) added to `gates`; SKIP line
     when no Go module is selected; otherwise
     `cd services/server && CONTRACT_MAX_EXAMPLES=$(CONTRACT_MAX_EXAMPLES) go test -count=1 -tags
     contract_validate,contract_live -run '^TestG7_' ./test/contract/...` with a generous
     `-timeout` from a Makefile variable.
   - Go `test`: add `-tags contract_validate` so G8 runs in every integration test; Go `lint`:
     `go vet -tags contract_validate,contract_live` and golangci-lint `--build-tags
     contract_validate,contract_live`; `typecheck` keeps the untagged `go build ./...` (production).
   - bandit directory list in `lint`: add `tools/contract`.
6. Named value: row `quality.contract.max_examples` with default `100`, unit "Anzahl", meaning
   "Beispiele je Operation in G7", held by `Makefile` (`CONTRACT_MAX_EXAMPLES`), in 23
   "Konfigurationswerte dieses Kapitels"; matching entry in `SOURCES` of
   `tools/docs/tests/test_config_values.py`.
7. Module register: vendor entry for the validator library allowed only for `platform` (and the
   `testsupport` component, see the shared harness); `make arch` green.
8. Shared test harness `services/server/test/harness` (create or extend, text below); this ticket
   adds `Options.EgressDenyAll`.
9. Fixes for every non-conformance found: server code in the owning module, or the specification
   when the specification is wrong (spec first, outside the core contract only; a needed change in
   00, 03, 04 or 05 ends the ticket with `BLOCKED`).
10. Documentation: `docs/quality/gates.md` (row `contract-test`, note that G8 runs inside `test`),
    `docs/spec/06-api.md` "Vertragstests und Gate" (G7 "Werkzeug" cell: Schemathesis against the
    server built from the working tree with a disposable PostgreSQL in `make contract-test`; the
    Compose stack is covered by the smoke test; item 3 accordingly), `docs/quality/toolchain.md`,
    `services/server/README.md`, `services/server/CHANGELOG.md` and root `CHANGELOG.md` under
    "Unreleased".

### Shared test harness (identical text in the briefs M1-69 to M1-72)

If `services/server/test/harness` exists on the base branch, use it and extend it only additively.
If it does not exist, first search for an existing full-server test helper
(`grep -rn "pgtest" services/server/internal/wire services/server/test`); wrap it if it builds the
real `internal/wire` graph, otherwise create the package with exactly this interface. Parallel
tickets build the same interface, so merges converge.

- Package `harness`, non-test Go files, architecture component `testsupport` (Design notes).
- `func Start(tb testing.TB, opts Options) *Server`: the real `internal/wire` graph with role `all`
  in this process; PostgreSQL 18 from `internal/platform/pg/pgtest`; migrations as
  `bothy migrate up --bootstrap` runs them; secrets created as `bothy secrets init` creates them, in
  `tb.TempDir()`; public and internal listener on `127.0.0.1:0`; the first owner created through the
  same application service that `bothy setup bootstrap-owner` calls (08 "Erstinstallation"), then
  enrolled with its enrollment code and fresh ES256 device keys (08 "Einladungs-Code",
  "Registrierung"); everything stopped in `tb.Cleanup` in reverse order.
- `type Options struct { Config map[string]string; LogWriter io.Writer; Telemetry *TelemetrySinks;
  EgressDenyAll bool; Clock clock.Clock }`. `Config` takes dotted keys from
  `docs/operations/configuration.md` and is applied through `platform/config` with flag precedence,
  so an unknown key still fails.
- `type Server struct { PublicURL, APIBase, GatewayURL, InternalURL, SecretsDir string; Owner *Client }`
  with `APIBase = PublicURL + "/api/v1"` and `func (s *Server) AdminDB() *pgxpool.Pool` (superuser
  of the disposable container; assertions only).
- `func (s *Server) NewUser(tb, UserSpec{Handle, DisplayName, Email string}) *Client` and
  `func (s *Server) NewAgent(tb, AgentSpec{Handle, DisplayName, BillingMode, CredentialHolder string}) *Client`,
  only through the public API (`createUser`, `createEnrollmentCode`, `completeEnrollment`;
  `createAgent`, `createToken`), owner step-up via `createStepUpChallenge` and
  `completeStepUpWithDeviceKey`.
- `type Client struct { PrincipalID, DeviceID string; ... }` with `Do(ctx, Request) Response`
  (DPoP proof per RFC 9449 with the latest `DPoP-Nonce`, one retry on `use_dpop_nonce`, refresh on
  `token_expired`, `Idempotency-Key` UUIDv7 on POST and PATCH except the list in 06 "Idempotenz",
  optional `Observer func(Request, Response)`), `StepUp(ctx)`, `Gateway(ctx) *GatewayConn`
  (`hello`, `identify`, `ready`, `Next`, `Send`, `Close`, automatic `heartbeat` and `reauth`).
- `type Response struct { Status int; Header http.Header; Body []byte; Elapsed time.Duration }`.
- `func BuildMatrixWorld(tb, s *Server) *World`: the M1 subset of 07 "Beispiele" and
  "Testmatrix": spaces Support, Marketing, Engineering, Research; channels Support/Allgemein,
  Support/Eskalation, Marketing/Kampagnen, Engineering/Projekt-A (forum, no project binding),
  Engineering/Werkzeuge, Research/Notizen; people Alex, Sam, Robin; agents Lotse (organization,
  operator of Support), Texter (organization, operator of Marketing), Runner (organization, operator
  of Engineering/Werkzeuge), Assistent (personal, credential holder the owner, operator of Research);
  group Leser with member Sam (membership with expiry); grants G-1 (Alex, `trigger`, `upload` and
  `approve_actions`, space Support), G-2 (Sam, `trigger`, `upload`, space Marketing), G-3 (Robin,
  `trigger`, `upload`, channel Engineering/Projekt-A, with expiry), G-4 (group Leser, `read`,
  Support/Allgemein), G-5 (Lotse, `write`, `upload`, space Support); one thread with messages per
  text channel and one forum post in Projekt-A. Left out until their etappe: app Tracker and G-6
  (M2), the DM Alex-Sam (V3), project binding and tool class `host` (M2). Expiry times are relative
  to the harness clock, never calendar dates.
- `func StartProcess(tb, ProcessOptions) *Process` (subprocess server for M1-72): not needed in
  this ticket, do not create it here if absent.
- Unit tests for every part you add (at least: DPoP proof claims and the nonce retry).

## Out of scope

- G4 and G5 (generated clients, M2 and M4), G6 (break check against a release, M7), G9 (gateway
  frames, M1-67), G10 (webhooks, M2).
- Running Schemathesis against the Compose stack (M1-73 smoke, M7 installation).
- Adding OpenAPI `links` to openapi.yaml (a spec change for the API owner; see Design notes).
- Rate-limit semantics (M1-19), authorize semantics (M1-28), 404 timing (M1-70).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-71-contract-tests`.
- Conventional Commits with scope (`test(contract): ...`, `build(make): ...`, `fix(<module>): ...`)
  and DCO sign-off; description language as in the existing `git log`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside this scope, except the docs and changelog entries named here, the
  shared harness, and minimal fixes for non-conformance this ticket finds, each with a regression
  test and listed in the report.
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
  Temporary directories only via `mktemp` or `t.TempDir()`, removed at the end; the Hypothesis
  database is off or lives in such a directory, never in the repository.
- Never read or print secrets; test credentials are per run. No network calls except package
  downloads for pinned tools: the contract server runs with `EgressDenyAll` so no generated
  issuer or host value causes DNS or outbound traffic; bind only `127.0.0.1` and port 0.
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

G8 validator:

1. Library: `github.com/pb33f/libopenapi-validator` (MIT, supports OpenAPI 3.1), pinned in
   `go.mod`; check at the pinned version that it validates `oneOf` with discriminator and
   `format: uuid`. If it cannot validate this spec, stop with `BLOCKED` and the evidence instead of
   silently validating less.
2. Spec source: the build-tagged code finds `docs/api/openapi.yaml` by walking up from the working
   directory (tests run inside the repository); missing file is a hard error.
3. It validates the response (status, headers, media type, body) of every request whose method and
   route match an operation in openapi.yaml. Unmatched requests (internal listener, test-only
   routes) are skipped; AZ-P1 already proves that every registered route is an operation.
4. On a violation it writes a report to standard error (operationId, method, route pattern, status,
   JSON pointer, schema keyword, the offending value truncated) and ends the process with exit code
   3. Ruling (record in the README and in `docs/DECISIONS.md` if it exists): a hard exit is the
   only mechanism that fails every integration test regardless of how it asserts, without touching
   each test; it exists only in the test build.
5. The untagged production build contains neither the package nor the library
   (`TestG8_ProductionBuildExcludesValidator`).

G7 run:

1. `TestG7_SchemathesisAllEnabledOperations` (tag `contract_live`) starts `harness.Start` with
   `EgressDenyAll: true` and every `ratelimit.<class>.per_minute` and `.burst` raised through
   `Options.Config` (G7 checks conformance, not throttling; M1-19 owns throttling; written in the
   README), builds `BuildMatrixWorld`, starts the credential broker and runs
   `uv run --project tools --locked pytest -p no:cacheprovider tools/contract/live_contract.py`
   from the repository root with the environment `BOTHY_CONTRACT_BASE_URL` (the `APIBase`),
   `BOTHY_CONTRACT_BROKER_URL`, `BOTHY_CONTRACT_OPERATIONS` (path of operations.yaml),
   `BOTHY_CONTRACT_SPEC` (path of openapi.yaml), `CONTRACT_MAX_EXAMPLES`, `CONTRACT_SEED` (printed;
   from the environment or random). The Go test fails when pytest exits non-zero and streams its
   output.
2. Credential broker: an HTTP server of the Go test on `127.0.0.1:0`, never part of the product:
   `POST /principals` with `{"kind":"owner"|"user"}` returns `{principal_id, device_id, session_id,
   access_token, refresh_token, dpop_private_jwk}` for a fresh session (owner: a new device of the
   bootstrap owner; user: a new person through `harness.NewUser`); `POST /step-up` with
   `{session_id}` performs a fresh step-up for that session. The Python side asks for new
   credentials when a call returns `401` with `session_revoked` or `unauthenticated`, so a
   generated call that revokes its own session does not end the run.
3. Python auth provider (`auth.py`): Schemathesis custom auth (`get` obtains broker credentials,
   refreshed before `auth.step_up_max_age` and `auth.access_token_ttl` run out; `set` adds
   `Authorization: DPoP <token>` and a fresh `DPoP` proof per case). Anonymous operations
   (`security: []`) still get a `DPoP` header without `ath` where 06 "DPoP" item 3 requires it.
   Nonce: keep the latest `DPoP-Nonce`; when none is known or it may be older than
   `auth.dpop_nonce_ttl`, fetch one with a probe call to `getMe`. Use the API names of the pinned
   Schemathesis version (check its documentation in the installed package); keep the semantics.
4. `dpop.py`: ES256 proof per RFC 9449: header `typ=dpop+jwt`, `alg=ES256`, public `jwk`; claims
   `jti` (random), `htm`, `htu` without query and fragment, `iat`, `ath` (base64url SHA-256 of the
   token) when a token is sent, `nonce` when known; signature as raw `r||s` (64 bytes), built with
   `cryptography`.
5. Two passes over the enabled operations: as owner (main coverage) and as a plain user (Alex from
   the world, through the broker kind `user`). Schemathesis loads openapi.yaml with base URL
   `BOTHY_CONTRACT_BASE_URL` and is restricted to the enabled operation IDs (`operations.py` reads
   operations.yaml; an ID missing from openapi.yaml fails). Settings: `max_examples` from
   `CONTRACT_MAX_EXAMPLES`, positive and negative generation, `derandomize` with `CONTRACT_SEED`,
   no Hypothesis database.
6. Checks: all built-in checks of the pinned version (status code, content type, response headers,
   response schema conformance, server errors, negative data rejection, ignored auth) plus
   `checks.py`:
   - `problem_format`: every response with status 400 or higher has `Content-Type:
     application/problem+json`, the required fields of `Problem`, `type == "/api/v1/problems/" +
     code`, `status` equal to the HTTP status, and a `code` from the generated table in 06 "Fehler".
   - `correlation_id`: header `Correlation-Id` on every response, a UUIDv7, equal to
     `correlation_id` in a problem body.
   - `no_json_null`: no JSON `null` at any depth of any JSON body (ADR 0053).
   - `not_found_equality`: for a `404` on an operation with UUID path parameters, the same request
     with fresh random UUIDv7 values gives the same normalised answer (drop `Correlation-Id`,
     `Date`, `DPoP-Nonce`, `RateLimit` values; drop `correlation_id`, replace the IDs in
     `instance`).
7. Stateful phase: run Schemathesis' state machine over the same restricted schema. openapi.yaml
   has no `links`; Schemathesis infers them. If the pinned version infers no transition for the
   enabled operations, say so in the README and the report as not proven; do not add `links` here.
8. Coverage: a session fixture in `live_contract.py` records the operation of every executed case
   and fails at teardown when an enabled operation was not called in a pass, listing them.
9. SEC-T06 extra tests in `live_contract.py`:
   - `test_sec_t06_unknown_field_rejected`: for every enabled operation with a JSON request body, a
     valid body plus the property `x_unknown_field` gives `400 validation_failed` (W7).
   - `test_sec_t06_duplicate_json_keys_rejected`: a raw body with one key twice gives `400` with the
     problem code that M1-18 assigns to SEC-IN-2 (read it from its decoder tests; do not invent a
     code).
   - Any `5xx` fails. No `5xx` is expected in this run; if an operation documents `503` for a
     condition the run does not induce, a `503` is still a failure.
10. Reproduction: on every failure the message contains `curl` for the request with `Authorization`
    and `DPoP` replaced by `"$BOTHY_CONTRACT_AUTH"` and `"$(uv run --project tools python -m
    contract.dpop --key <file> --htm <M> --htu <URL>)"`, and the seed. `reporting.py` builds it;
    `dpop.py` has that command line entry point.

Architecture register and coverage (shared with M1-70 and M1-72): `test/**` and `perf/**` under
`services/server` are test support. If M1-05's generator (`tools/archgen`) has no component for
them, add `testsupport` with the go-arch-lint v3 keys `anyProjectDeps: true` and `anyVendorDeps:
true` (verify against the pinned version) and a generator test that no production component may
depend on it; exclude `services/server/test/**` and `services/server/perf/**` from the Go coverage
total in the `coverage` target.

## Tests (write them first)

Go:

1. `TestG8_ValidatorAcceptsConformingResponse` (unit with a `getMe` route returning a valid body).
2. `TestG8_ValidatorRejectsWrongResponseField`: runs a child test binary (`os/exec` of the test
   binary with `-test.run` and an environment switch) whose `getMe` handler omits a required field;
   expects exit code 3 and a report naming `getMe` and the JSON pointer. This is the acceptance
   "ein absichtlich falsches Antwortfeld lässt G8 scheitern".
3. `TestG8_SkipsUnmatchedRoutes`.
4. `TestG8_ProductionBuildExcludesValidator`: `go list -deps ./cmd/bothy` without tags lists
   neither `contractcheck` nor the validator module.
5. `TestG7_SchemathesisAllEnabledOperations` (tag `contract_live`).
6. Broker unit tests (owner and user credentials, step-up endpoint, loopback only).

Python (`tools/contract/tests/`, run in `make test`):

1. `test_dpop.py`: header fields, claims, `htu` without query, unique `jti`, `ath` value, nonce
   present when given, signature verifies with `cryptography`, signature length 64.
2. `test_checks.py`: each project check detects its negative fixture (wrong media type, missing
   `code`, `type` not matching `code`, `status` mismatch, missing or non-v7 `Correlation-Id`, nested
   `null`, unequal 404 pair) and accepts the positive fixture.
3. `test_operations.py`: enabled IDs from a fixture operations.yaml; unknown ID fails.
4. `test_reporting.py`: curl output masks `Authorization` and `DPoP`, keeps method, URL, other
   headers and body.
5. `test_auth.py`: refresh on `session_revoked`, nonce probe when none is known (broker and server
   faked with a local stub).

Live (only through `make contract-test`): the parametrised Schemathesis tests per pass, the stateful
test, `test_sec_t06_unknown_field_rejected`, `test_sec_t06_duplicate_json_keys_rejected`, the
coverage teardown.

Commands that must pass:

```sh
cd services/server && go test -race -count=1 -tags contract_validate ./...
make contract-test
make test
make check
```

## Acceptance criteria

- [ ] Every operation of the enablement list is called in both passes; the coverage teardown lists
      none missing.
- [ ] Built-in checks and the four project checks pass; SEC-T06 tests pass; no `5xx`.
- [ ] A deliberately wrong response field makes G8 fail with exit code 3 and a precise report.
- [ ] Every Go integration test runs with the validator (`make test` uses `-tags
      contract_validate`); the production build excludes it.
- [ ] Every failure prints a reproducible `curl` command and the seed.
- [ ] `quality.contract.max_examples` is in 23, the Makefile and `test_config_values.py`;
      `make docs` and the toolchain test pass with the new pins.
- [ ] Every non-conformance found is fixed with a regression test.
- [ ] `make check` is green.

## Definition of done

- Code and tests as above; README in `services/server/test/contract/`, `tools/contract/`, the
  `contractcheck` package and the harness (if created); doc comments and docstrings on every public
  symbol.
- No new ADR (ADR 0013 holds the decision); the hard-exit ruling and the place of G7 go into
  `docs/DECISIONS.md` if it exists, otherwise into the README and the report.
- CHANGELOG entries under "Unreleased" in `services/server/CHANGELOG.md` and root `CHANGELOG.md`.
- `make check` green (includes `contract-test`).
- Independent review and red team over the diff (`docs/quality/red-team-gate.md`) with verdict
  recorded in the report; merge locally into the base branch only if both are green, then
  `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits,
files with line counts, test results with counts (Go, Python unit, live: cases per pass, stateful
steps, seed), the pinned versions added, non-conformance found and fixed (server or spec), what is
not proven (at least: stateful coverage if inference found no links; the Compose stack), follow-ups
that could not be done in scope with reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or
`exit=1 <reason>`.
