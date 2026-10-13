# M4-17: Fake server for integration tests

Status: ready | Depends on: M4-14, M4-15, M4-16 | Milestone: M4

## Goal

After this ticket the client side has one Dart fake of the server that every integration test uses (15 20 row "Integration"): `package:sync_engine/testing.dart` starts a fake that serves the gateway (`/api/v1/gateway`, the frames and close codes of 04 6) and a REST subset over loopback, keeps a small consistent world in memory (instance and discovery, people with devices and sessions, spaces, channels, threads, messages with `thread_seq`, an event log with a global `seq` and a sync epoch, read states, drafts and runs), verifies the DPoP proofs it receives, and injects faults on demand (disconnect with any close code, duplicate delivery, delay, `resync_required`, `4012`, `409`, `410`, `412`, `429`, `5xx`, a lost response). The same ticket composes the engine: `SyncEngine` wires the connection of M4-13, the dispatcher of M4-14, the outbox of M4-15 and the run store of M4-16 (echo confirmation, start of the outbox after `catchup.complete`, `run.updated` to the run store, run end to the run store), so the scenarios of 04 section 16 run green against the sync engine with its local database in headless tests; the app tickets from M4-18 on run the same scenarios against the app. Why: failure cases that occur only by chance against a real server must be reproducible (15 20 "Rationale"), and a fake shared by the headless tests and the app tests prevents two fakes that drift apart (D-W-114).

## Read first

- `docs/PLAN.md`: section 3; section 5 (test strategy); section 10 (Exit criteria item 2, Test strategy: "Dart fake server with fault injection"; rows M4-17 to M4-24, M4-34).
- `docs/spec/15-client-architecture.md`: 7 "Sync engine" (all of it), 20 "Test strategy" (rows "Integration" and the rationale).
- `docs/spec/04-sync-protocol.md`: 2 (envelope), 3 (global `seq`), 4 (`thread_seq`), 5 (idempotency: the replay and `422` rules the fake must imitate), 6 (gateway), 7 (catch-up, `resync_required`, epoch), 8.2 and 8.3 (`access.changed`), 9, 11.3 (coalesced `run.updated`, whole document on focus), 13, 14, 16 "Sequence diagrams" (16.1 to 16.5).
- `docs/spec/08-identity.md` 7.3 (what a DPoP proof must contain; the fake checks it) and 14.1 to 14.3 (flows the fake offers in a reduced form).
- `docs/api/openapi.yaml`: the operations of Design note 3 with their request and response schemas and problem codes; `docs/api/asyncapi.yaml` channel `gateway`.
- `docs/DECISIONS.md`: D-W-100, D-W-114 (the fake lives in `packages/sync_engine` as `testing.dart` and the app shares it); section 10.
- Briefs: `docs/briefs/M4-13-sync-connection.md`, `docs/briefs/M4-14-sync-catch-up.md`, `docs/briefs/M4-15-sync-outbox.md`, `docs/briefs/M4-16-sync-run-documents.md` (the parts this ticket composes and their hooks), `docs/briefs/M4-09-auth-dpop.md` (proof structure, nonce), `docs/briefs/M4-02-dart-api-client.md` (`createApiClient` with an adapter); for the server behaviour the fake imitates `docs/briefs/M1-56-messaging-send.md`, `docs/briefs/M1-63-sync-catch-up.md`, `docs/briefs/M1-64-gateway-connection.md`, `docs/briefs/M1-65-gateway-delivery.md`.

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. The fake implements a subset of the contract, never a different contract: every request and response it sends validates against the schemas of `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml` (Design note 5). Where the fake simplifies (no permissions beyond visibility per channel, no rate limits unless injected), the README says so.
10. Values the fake reports to clients (heartbeat interval, resume window) are parameters with the defaults of 04 18, so tests can shorten them.

Product requirements implemented here:

11. The scenarios 16.1 to 16.5 of 04 run against the real sync engine with its local database: send online with echo, send offline and reconnect with a lost response (exactly one message), reconnect with catch-up and with `resync_required`, access removal during an open connection, agent run with follow-up and live box patches.
12. Fault injection: disconnect with a chosen close code, duplicate delivery, delay, `resync_required` with each reason, `4012`, and the HTTP answers `409`, `410`, `412`, `429` with `Retry-After`, `5xx` and a lost response after the server committed (15 20 row "Integration").
13. The fake checks DPoP proofs (signature with the `jwk`, `htm`, `htu`, `ath`, `nonce`, `jti` reuse) and rejects bad ones as the server would (`401` with `use_dpop_nonce` or `invalid_dpop_proof`, 06 5; gateway `4004`).
14. One composition root `SyncEngine` that the apps use (no app wires the parts itself).

## Scope

Create in `packages/sync_engine/`:

- `lib/testing.dart` (a separate library, never exported by `lib/sync_engine.dart`) exporting `lib/src/testing/`: `fake_server.dart` (`FakeServer.start({...})`, `stop`, `baseUri`), `world.dart` (the in-memory model and the event log), `gateway_fake.dart`, `rest_fake.dart` with one handler file per operation group under `rest/`, `dpop_verifier.dart`, `faults.dart` (`FaultPlan`, Design note 4), `scripted_agent.dart` (Design note 6), `schema_validator.dart`, `seed.dart` (`WorldSeed` builders with the example people Alex, Sam and Robin), `scenarios.dart` (the five scenarios as reusable drivers that take a client under test, Design note 7).
- `lib/src/engine.dart`: `SyncEngine` (Design note 2), exported from `lib/sync_engine.dart`.
- Tests: `test/fake_server/` (the fake's own tests) and `test/scenarios/` (the five scenarios headless).

Change:

- `packages/sync_engine/pubspec.yaml` (dev and runtime dependencies the fake needs; the fake uses `dart:io` `HttpServer` and `WebSocketTransformer`, `api_client` models, `crypto`, `pointycastle` for the ES256 check of proofs and `yaml` to read the API documents, each only if `make licenses` and `make osv` stay green; a JSON Schema validator only if one with an allowed license exists, otherwise Design note 5), `README.md` (how to start the fake, the world, the fault plan, what it simplifies).
- `tools/arch-check/arch_check.py`, `tools/arch-check/rules.yaml` and `tools/arch-check/tests/test_arch_check.py`: the rule that `package:sync_engine/testing.dart` is imported only from `test/` and `integration_test/` folders, with a failing and a passing case (Design note 8).
- `docs/spec/15-client-architecture.md` 20 (row "Integration": the fake lives in `sync_engine/testing.dart`; one sentence on `SyncEngine` in 7.1).
- `docs/DECISIONS.md` section 10: rows `D-M4-17-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Running the scenarios against the app (M4-18 and the feature tickets add their app-level tests on this fake).
- Upload, download, search, commands, components, administration and terminal endpoints: the tickets that need them (M4-21, M4-22, M4-23, M4-25 to M4-29, M4-36) add handlers to the fake in their own change, following the pattern of this ticket.
- The real server in the Compose stack (M4-34 E2E).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-17-fake-server`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Process and ports.** `FakeServer.start({FakeServerOptions options})` binds `HttpServer` on `127.0.0.1` port 0 (plain HTTP; the client is configured with `http://127.0.0.1:<port>` in tests, which only test builds allow, Design note 8), serves REST under `/api/v1/` and the gateway upgrade at `/api/v1/gateway`. One fake per test; `stop()` closes all sockets. No global state; parallel tests start their own fakes.

2. **Composition `SyncEngine`.** `SyncEngine.create({required Uri baseUrl, required AuthSession auth, required LocalStore store, required GatewayTransport transport, required NetworkStatus network, required AppLifecycle lifecycle, required FrameTicker ticker, required AppConfig config, Clock? clock, RandomSource? random, FileCacheEraser? eraser})` builds the API client (M4-02) with the `auth` interceptors (M4-09), the `GatewayConnection` (M4-13) with the `CatchUpListener` and `Dispatcher` (M4-14), the `Outbox` with the message and personal executors (M4-15) and the `RunStore` with a fetcher on `getRun` (M4-16), and connects the hooks: `Dispatcher.onEcho` to `Outbox.confirmByEcho`; `CatchUpListener.onLive` to `Outbox.start` and every disconnect to `Outbox.pause`; `onRunUpdated` to `RunStore.onRunUpdated`; a `run.state_changed` with an end state to `RunStore.onRunEnded`; focus changes of `RunStore.focus` to the threads of the `FocusManager`. It exposes `state`, `outbox`, `runs`, `ephemeral`, `notices`, `focus`, `typing`, `api`, `start()` and `dispose()`. Ruling for docs/DECISIONS.md: one composition root in the package so the desktop and mobile apps cannot wire the hooks differently.

3. **REST subset.** Handlers for: `getDiscovery`; login `loginWithPassword` (with the TOTP step), `beginPairing`, `completePairing`, `getDevice`, `approveDevice` and `approvePairing` (to let a test confirm a pending device or a pairing from a second simulated device), `refreshToken`, `logout`; `getMe`, `getInstance`; `getSnapshot`, `listEvents`; `listSpaces`, `getSpace`, `listSpaceChannels`, `getChannel`, `listChannelThreads`, `getThread`, `listThreadMessages`, `getMessage`, `createMessage` (with the `client_msg_id` rules of 04 5.1: replay with `200` and `Idempotent-Replay: true`, `422 idempotency_key_reuse` on a different hash), `updateMessage` (with `If-Match`: `412 version_conflict` with the current revision, `410 message_deleted`), `deleteMessage` (idempotent `204`), `addReaction`, `removeReaction`, `putReadState`, `putThreadDraft`, `deleteThreadDraft`, `listMyReadStates`, `listMyDrafts`, `listMySettings`, `putMySetting`, `deleteMySetting`; `getRun`, `cancelRun`. World controls for tests (`FakeWorld`): add people, devices, spaces, channels and threads; expire or reject a pending device; expire a pairing; remove or add a person's access to a channel (writes `access.changed` as 04 8.2 does); set the catch-up page size and the `cursor` frame interval so 16.3 runs with several pages. Every write appends the events of 05 section 6 to the event log in one step with a new `seq` and pushes them to the connected gateways that may see them. Writes with `Idempotency-Key` follow 04 5.2. A handler registry (`fake.handle(String operationId, FakeHandler h)`) lets later tickets add operations without editing this file.

4. **Faults.** `FaultPlan` is a queue of rules matched in order: `onRequest(operationId, {int times})` answering a status with a problem body (and `Retry-After`), `dropResponseAfterCommit(operationId)` (the write is stored, the connection is closed before the response: scenario 16.2), `delay(Duration)`, `closeGateway(int code, {String? reason, Duration? after})`, `duplicateNextDispatch()`, `resyncRequired(String reason)`, `revoke()` (close every connection of the device with `4012` and mark the session revoked). Faults are deterministic (no randomness in the fake).

5. **Contract checks.** The fake validates every response body and every frame it sends against the schemas, and every request body it receives, with a small validator for the JSON Schema subset the documents use (`type`, `required`, `properties`, `additionalProperties`, `enum`, `items`, `$ref`, `format` for `uuid` and `date-time`, `oneOf` with a discriminator), or with a package if one with an allowed license exists; a violation fails the test with the pointer. The validator reads `docs/api/openapi.yaml` (walk up to the repository root) once per test run. Ruling for docs/DECISIONS.md: the fake can never drift from the contract silently.

6. **Scripted agent.** `ScriptedAgent` turns a mention of an agent in a created message into the run of 04 16.5: `trigger_state` changes (`received`, `queued`, `working`, `done`, `followup_delivered`) as `message.updated`, `run.state_changed` (`queued`, `running`, `completed`), `run.updated` patches only to connections that focus the thread (whole document on focus, coalesced patches after), a follow-up turned into `followups/<id>/state` `delivered` then `absorbed`, and an agent reply message at the end. Steps and timing come from a script the test passes.

7. **Scenarios.** `scenarios.dart` defines the five flows of 04 16 as functions that take a `ScenarioClient` interface (send a message, go offline and online, read the local state of a thread, read the outbox, focus a thread, read the run document) and a running `FakeServer`, and assert the outcome: 16.1 one message with the echo confirming the entry; 16.2 one message after a lost response and a retry with the same `client_msg_id`; 16.3 catch-up from a cursor with pages and `cursor` frames, and the `resync_required` branch; 16.4 removal of a channel while connected deletes its local content and closes the view (notice), later events of that channel never arrive; 16.5 a run with a follow-up and live box patches. `test/scenarios/` runs them with a `ScenarioClient` over the `SyncEngine`; M4-18 implements the interface over the app.

8. **Test-only transport.** The client accepts a plain `http` base URL only through an explicit test flag of `SyncEngine` and the app's test entry point; production builds accept only `https` addresses (a ruling of this ticket: DPoP and the device binding assume a protected transport, 15 8). Arch rule: `package:sync_engine/testing.dart` may be imported only from `test/` and `integration_test/` folders; `arch_check.py` gets this rule (as a general rule for libraries named `testing.dart`, which also covers `package:design_system/testing.dart` of M4-04).

9. **Rulings for docs/DECISIONS.md** (`D-M4-17-<n>`): one composition root (note 2); the handler registry for later tickets (note 3); deterministic faults (note 4); contract validation in the fake (note 5); the scenario interface shared with the app (note 7); test-only plain HTTP and the import rule (note 8).

## Tests (write them first)

`packages/sync_engine/test/`:

- `fake_server/contract_test.dart`: every handler's response and every frame validate; an invalid body sent to the fake fails with the pointer.
- `fake_server/dpop_test.dart`: a proof with a wrong signature, a reused `jti`, a wrong `htu` or a stale nonce is rejected as the server would.
- `fake_server/idempotency_test.dart`: replay of a `client_msg_id` returns the original with `Idempotent-Replay: true`; a different body gives `422 idempotency_key_reuse`.
- `fake_server/faults_test.dart`: each fault of Design note 4 has the effect described.
- `scenarios/scenario_16_1_test.dart` to `scenario_16_5_test.dart`: the five scenarios against `SyncEngine` with an in-memory `local_store`, each green, plus `scenario_16_3_resync_test.dart` for the `resync_required` branch.
- `scenarios/engine_wiring_test.dart`: the outbox does not send before `catchup.complete` and pauses on disconnect; the echo confirms; run patches reach the run store; a run end discards the document.

Commands that must pass, from the repository root:

- `cd packages/sync_engine && dart test --coverage=coverage` (or the command M4-01 uses for pure Dart members)
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`
- `make check`

## Acceptance criteria

- [ ] The scenarios of 04 section 16 (16.1 to 16.5, and the `resync_required` branch of 16.3) run green against the sync engine with its local database.
- [ ] The fake injects every fault of Design note 4 and checks DPoP proofs; its requests, responses and frames validate against the API documents.
- [ ] `SyncEngine` is the single composition root; its wiring test is green.
- [ ] `testing.dart` is a separate library, importable only from test folders; the scenario drivers are reusable by the app (M4-18).
- [ ] 15 7.1 and 20 updated; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 04.
- `packages/sync_engine/README.md` (English) as in Scope.
- 15 updated; rulings of Design note 9 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The list of fake operations and faults, the run time of the scenario suite, and every place where the fake had to simplify the contract.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
