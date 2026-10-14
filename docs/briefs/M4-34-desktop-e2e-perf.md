# M4-34: Desktop E2E and performance targets

Status: ready | Depends on: M4-22, M4-24, M4-33, M1-73 | Milestone: M4

## Goal

After this ticket the desktop app is tested end to end against the real server: a new make target `make e2e-desktop` starts the Compose stack of the installation from the `bothy-server` image with an E2E overlay (the pattern of M1-73's smoke overlay, plus a Fake-Agent container from the Python SDK of M2-29 and M2-31 for runs), prepares a world (first owner, enrollment codes for Alex and Sam, a space with a text channel and a forum, an agent served by the Fake-Agent) and runs the E2E suite of the app against it: with `patrol` on macOS and with `integration_test` on Windows (15 20, R-CL-11). The suite covers login with an enrollment code and device keys, a waiting device and its confirmation, sending online and offline with the outbox, a run with live box and follow-up, an approval card, a file upload and download, search, a notification while running (Windows), the channel menu, keyboard navigation, and revocation with `4012`. A performance level with profile builds measures the targets `client.perf.*` for the desktop (cold start, thread open, local send echo, patch to frame, jank ratio, catch-up rate, desktop memory) and fails when a target is missed (15 19). Why: the integration tests run against a fake; only an E2E against the real server and image proves that the app and the server agree (PLAN 10 exit criterion 3), and the targets are only real when a test can fail on them.

## Read first

- `docs/PLAN.md`: section 3; section 5 (test strategy, end to end); section 10 (Exit criteria items 2 and 3, Test strategy, Risks: "Windows E2E without `patrol` does not cover system dialogs"; rows M4-18 to M4-33, M4-38); section 7 row M1-73.
- `docs/spec/15-client-architecture.md`: 19 "Performance targets" (names, reference devices), 20 "Test strategy" (rows "E2E on devices" and "Performance"), 21 (R-CL-11), 22 (`client.perf.*` defaults).
- `docs/spec/23-quality.md` (levels, runners, no silent skip R-QA-02) and `docs/quality/gates.md` (where `portability` and other non-gate targets are described).
- `docs/spec/16-operations.md` 3.1 (first setup) and `infra/compose/README.md` (installation Compose file).
- `docs/spec/09-agents.md` 16.1 (test doubles: the Fake-Agent).
- `docs/DECISIONS.md`: D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M1-73-compose-smoke.md` (`infra/compose/smoke.sh`, the test overlay, preflight, workspace, cleanup), `docs/briefs/M2-31-agent-conformance-suite.md` (the Fake-Agent and how it is started), `docs/briefs/M4-17-fake-server.md` (the scenario drivers to reuse), `docs/briefs/M4-18-desktop-shell.md` to `docs/briefs/M4-33-channel-menu.md` (the flows), `docs/briefs/M4-31-client-metrics.md` if its measurement points exist (this ticket measures with the profile timeline and does not depend on them).
- `patrol` documentation for macOS desktop and the Flutter documentation of `flutter drive --profile` and timeline summaries.

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
9. Targets only from `AppConfig` (`client.perf.cold_start_p95`, `client.perf.thread_open_p95`, `client.perf.send_local_echo_p95`, `client.perf.patch_to_frame_p95`, `client.perf.jank_ratio_max`, `client.perf.catchup_events_per_s_min`, `client.perf.memory_desktop_max`); no literal target in a test.
10. Rule for where the suites run and for the verdict (Design note 6): E2E and performance runs need a desktop session, a Docker engine for the Compose stack and, for the targets, a reference device (15 19). The session runs the macOS suite and the macOS targets on its build host when it is a Mac with a Docker engine; it never claims a run it did not do. A Windows run needs a Windows host that reaches the stack; when the session has none, the Windows suite is built, compiled in the `apps-windows` job, and listed in report item 5 as not proven, with the exact command; the verdict stays `Verdict: DONE` when everything the session can run is green.

Product requirements implemented here:

11. E2E against the real server from the `bothy-server` image with the installation Compose file and a test overlay, never against the fake (15 20).
12. `patrol` on macOS (system dialogs such as the keychain and notification permission), `integration_test` on Windows (R-CL-11).
13. The flows: enrollment login with device keys and DPoP, waiting device and confirmation, send online and offline, run with live box and follow-up through the Fake-Agent, approval card, file upload and download, search, Windows notification while running, channel level and export, keyboard walkthrough, revocation with `4012` and the wiped store.
14. The targets `client.perf.*` for the desktop are measured with a profile build and fail the performance level when missed (15 19).
15. The stack and its temporary data are removed after every run unless asked to keep them.

## Scope

Create:

- `infra/compose/test/compose.e2e.yaml` (overlay: the api published on `127.0.0.1` with a free port, a Fake-Agent service from the SDK image or a Python container running `bothy_sdk.testing.fake_agent` with a script for the E2E runs, no other published port) and `infra/compose/e2e.sh` (bash, the structure of `smoke.sh`: inputs, sanitize, preflight, workspace with `mktemp -d`, bootstrap of the world, `E2E_SERVER_URL` and the enrollment codes into a file for the app tests, cleanup on exit; Design note 1).
- Root `Makefile`: target `e2e-desktop` (not part of `gates`; calls `e2e.sh` and then the suite for the host system) and `perf-desktop` (profile runs); `docs/quality/gates.md` rows for both; the table "Make targets created by tickets" in `docs/briefs/README.md` gains `e2e-desktop` and `perf-desktop` with M4-34.
- `apps/client/e2e_test/` (the suite, Design note 2; a folder outside `integration_test/`, so the `apps-macos` and `apps-windows` jobs of M4-01, which run `integration_test/` through `platform-tests.json` without a server, never pick it up) with `patrol` tests under `apps/client/patrol_test/` if `patrol` needs its own folder, and `apps/client/test_driver/perf_driver.dart` with `apps/client/perf_test/` (Design note 3); `make e2e-desktop` and `make perf-desktop` are the only runners.
- `tools/ci/tests/test_e2e_script.py` (pytest over `e2e.sh` with fake `docker`: preflight messages, sanitizing, cleanup) and `tools/ci/perf_summary.py` with its test if the timeline evaluation is done in Python (Design note 3).
- `.github/workflows/e2e-desktop.yml` only for what GitHub-hosted runners can do (Design note 6): a scheduled and manual workflow that builds the E2E suite on `macos-15` and `windows-2025` and fails on compile errors; `make runners-check` and the workflow rules stay green.

Change:

- `apps/client/pubspec.yaml` (dev: `patrol` and its CLI pin in `docs/quality/toolchain.md`), `apps/client/README.md` (how to run E2E and performance), `infra/compose/README.md` (the E2E overlay).
- `docs/spec/15-client-architecture.md` 20 (where E2E and performance run, Design note 6).
- `docs/DECISIONS.md` section 10: rows `D-M4-34-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Terminal E2E and the terminal performance targets (M4-38, which builds on this target and overlay).
- Mobile E2E (M5) and console E2E (M6).
- Signing, notarization, MSIX and auto-update (M7).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-34-desktop-e2e-perf`; one writer per worktree.
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
- The stack runs only on `127.0.0.1` with free ports; never on a reserved port of the host rules; the project name is unique per run; `docker compose down -v` in cleanup, and the session removes only the containers, volumes and temporary folders it created.
- No real third-party service: the Fake-Agent replaces agents, the Collector of the stack replaces telemetry targets, crash reports stay off.

## Design notes

1. **Stack and world.** `e2e.sh` follows the structure of `smoke.sh` and copies the few functions it needs (factoring them into a shared file would change `smoke.sh`, which is outside this scope); after the stack is healthy it creates the first owner and enrollment codes the way `smoke.sh` does (setup command in the container), creates the world through the public API with the owner's token (space Support with channel general and forum requests, agent support-agent with the Fake-Agent as its consumer, Sam as a person with `trigger`), and writes `E2E_SERVER_URL`, the codes and the IDs into `$SMOKE_OUT/e2e.json` for the app. The Fake-Agent script answers a mention with a run (status line, two steps, a patch stream long enough for the patch-to-frame measurement), accepts a follow-up, posts an approval card, and finishes.

2. **Suite.** One test per flow of "Product requirements", each starting from a clean app data folder: enrollment login (macOS through `patrol` including the keychain or Touch ID dialog where the runner shows one; Windows without system dialogs), confirm a second device of Alex from the first, send and receive between Alex and Sam (two app processes or Alex in the app and Sam through the API), offline send by stopping the network of the app's host side (block the published port, then reopen) and the outbox delivering exactly once, a run with follow-up and approval through the Fake-Agent, upload and download of a file, search, the channel level, the keyboard walkthrough of M4-24, Windows notification while hidden to the tray, and the owner revoking Alex's device so the app ends in E7 with a wiped store. The scenario drivers of M4-17 are reused where they fit.

3. **Performance.** Profile builds driven by `flutter drive --profile`: cold start until the sidebar is visible from the cache (app restart with a filled database), thread open from the cache until the first messages are visible, send until the local "sending" display, patch received until drawn (timestamps from the run store's emission and the next rasterized frame), jank ratio while scrolling a thread of several thousand messages (frames over budget divided by frames, from the timeline summary), catch-up rate (events applied per second after a reconnect with a backlog the world script creates), memory with twenty threads in focus (resident size from the platform). Each value is computed as p95 over at least twenty runs where 15 19 says p95, compared with the `AppConfig` target, and written to `apps/client/build/perf/summary.json`; a missed target fails `make perf-desktop`.

4. **Reference devices.** 15 19 measures on a Mac with a first-generation Apple chip and a Windows laptop with 8 GB RAM. The report names the device the numbers came from; numbers from another device are recorded but marked as not from the reference device.

5. **Rulings for docs/DECISIONS.md** (`D-M4-34-<n>`): the shared Compose functions (note 1); two processes or the API for the second person (note 2); the measurement points and p95 runs (note 3); the device marking (note 4); where E2E and performance run (note 6).

6. **Where it runs.** GitHub-hosted macOS runners have no Docker engine and Windows runners run only Windows containers, so the Compose stack cannot run next to the app there (ADR 0039 allows only hosted runners). Therefore: the E2E and performance runs are local targets (`make e2e-desktop`, `make perf-desktop`) for a host with a desktop session and a Docker engine, run before a release and at the close of M4 (PLAN 4); the scheduled workflow compiles the suites on both systems so they cannot rot. 15 20 states this in the same change; the ruling records it.

## Tests (write them first)

- `tools/ci/tests/test_e2e_script.py`: preflight messages for a missing Docker engine or image, sanitized environment, cleanup removes what was created.
- The E2E suite itself (Design note 2), run on the build host for macOS when it is a Mac with a Docker engine.
- The performance level (Design note 3) with a unit test of the summary computation (p95 and ratio) on recorded timeline samples.
- `make runners-check` for the new workflow.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on a Mac with a Docker engine: `make image`, `make e2e-desktop`, `make perf-desktop`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make runners-check`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `patrol` on macOS against the Compose stack is green on the build host (or, without a Mac with a Docker engine, the report says so and names the command); `integration_test` on Windows is built and green where a Windows host reached the stack, otherwise listed as not proven with the command.
- [ ] The targets `client.perf.*` for the desktop are measured with profile builds and met on the device named in the report; a missed target fails `make perf-desktop`.
- [ ] `make e2e-desktop` and `make perf-desktop` exist, are documented in gates.md and in the brief index table, and clean up after themselves.
- [ ] 15 20 says where E2E and performance run; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol and a header comment in every script (purpose, inputs, exit codes); comments explain why and cite 15 19 and 20.
- `apps/client/README.md` and `infra/compose/README.md` (English) as in Scope.
- 15 20 updated; rulings of Design note 5 in `docs/DECISIONS.md` section 10.
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
4. The E2E results per flow and platform with where they ran, the performance numbers per target with the device and whether it is a reference device, and the run time of `make e2e-desktop`.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
