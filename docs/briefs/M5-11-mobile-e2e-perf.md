# M5-11: Device E2E and performance targets on mobile

Status: ready | Depends on: M5-05, M5-06, M5-07, M5-08, M5-09 | Milestone: M5

## Goal

After this ticket the phone apps are tested end to end on devices against the real server in the Compose stack (exit criterion 1 of M5) and their performance against the mobile targets of 15 19 is measured (exit criterion 3). A new make target `e2e-mobile` starts the Compose smoke stack of M1-73 with an overlay for mobile E2E (fake APNs and fake FCM endpoints that capture what the server sends, a seeded instance with Alex, Sam and Robin and the 07 10 world), then runs the `patrol` suites on an iOS simulator and an Android emulator: login through E1, E2 and E4 with the device confirmed by a second session, the push permission dialog, a message from another person that becomes a push captured by the fake provider and delivered to the device (`xcrun simctl push` on iOS, a broadcast into the service on Android), the Notification Service Extension or the Android service fetching the text from the real server, tapping the notification to open the thread at the message, sharing an image from the Photos app into a thread, and sending while offline with delivery after reconnect. A nightly workflow runs the Android suite on a GitHub-hosted runner with an emulator and Docker; the iOS suite runs on a macOS host with Docker and is named in the report. A profile build measures `client.perf.cold_start_p95`, `client.perf.thread_open_p95`, `client.perf.send_local_echo_p95`, `client.perf.patch_to_frame_p95`, `client.perf.jank_ratio_max`, `client.perf.catchup_events_per_s_min` and `client.perf.memory_mobile_max` with `flutter drive --profile` and fails a target that is missed; the measurement is evidence only on the reference devices of 15 19 (an iPhone two generations old, an Android mid-range device with 6 GB RAM) and is part of `make perf` at the milestone closing (P-07). Why: system dialogs, notifications and share sheets are the critical mobile paths, and only `patrol` on devices against the real server shows that they work together (15 20, R-CL-11).

## Read first

- `docs/PLAN.md`: section 3 (rule 6: `make perf` for performance targets), 4 (closing: item 3 performance on the reference profile), 5 (rows "End to end", "Load, endurance, chaos"), 11 (Exit criteria 1 to 3, Test strategy, Red team: T-DV-01, T-PU-01), 19 (P-07).
- `docs/spec/15-client-architecture.md`: 10 (push paths), 12 (sharing), 19 "Performance targets" (the reference devices and the targets), 20 (rows "E2E on devices", "Performance"), 22 (defaults of `client.perf.*`).
- `docs/spec/13-push-notifications.md`: 6.1, 6.2 (payloads that the fake providers capture), 14 (the end-to-end rows).
- `docs/adr/0063-*.md` (reference profile and modes `match`, `emulated`, `below`); `docs/spec/02-architecture.md` 17 (performance targets and `make perf`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M5-11-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-73-compose-smoke.md` (the Compose smoke stack and its overlay mechanism), `M3-20-apns-adapter.md` and `M3-21-fcm-adapter.md` (the fake providers and the provider endpoints the server can be pointed to), `M5-02-iphone-layout.md` to `M5-09-mobile-emergency.md` (the flows under test), the brief of M4-34 (desktop E2E and performance harness; reuse its timeline evaluation where it exists), `M1-72-performance-gate.md` (`make perf` structure).
- Code on the base branch: `infra/compose/`, `apps/client/integration_test`, `apps/client/platform-tests.json`, root `Makefile` (`perf`), `.github/workflows/`.

This ticket touches no API operation, no table, no action, no event and no configuration key of the release server; the E2E server build of Design note 1 wires test-only provider endpoints in code.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. The E2E seed uses only example values (`EXAMPLE` markers as the Compose stack requires).
9. Targets only through named configuration keys (`client.perf.*`); code comments and docs name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. `patrol` E2E on iOS and Android against the Compose stack: login, push permission, tapping a notification, sharing, offline sending (PLAN row M5-11).
12. The push path is real from the server to the device except the provider: the server sends to the fake APNs and FCM, the test hands the captured payload to the device, and the extension or service fetches the text from the real server.
13. The targets `client.perf.*` for mobile are measured with profile builds; a missed target fails the measurement; evidence counts only on the reference devices of 15 19.
14. E2E and performance runs are not part of `make check`; they run nightly (Android in CI) and before a release, and at the milestone closing (`make perf`, P-07).

## Scope

Create:

- `infra/compose/test/compose.e2e-mobile.yaml` (an overlay of the M1-73 stack under `infra/compose/test`, which the installation bundle excludes, 18 and M1-73): the E2E server build of Design note 1 with the APNs and FCM endpoints pointed to fake provider containers, the preview mode `full`, a short `notify.coalesce_window`, a seed job that creates the 07 10 world with example values (Design note 1).
- A fake push provider program usable as a container, from the fakes of M3-20 and M3-21 (`services/server/cmd/fakepush` or a test helper `main` if those tickets left only in-process fakes), exposing an HTTP endpoint for the E2E test to read captured payloads; never reachable outside the Compose network and the loopback.
- `apps/client/integration_test/e2e/`: the `patrol` suites `login_e2e_test.dart`, `push_e2e_test.dart`, `share_e2e_test.dart`, `offline_e2e_test.dart`, sharing one setup that reads the server address and the seed accounts from `--dart-define` (Design note 2).
- `tools/e2e/run_mobile.sh` (or a Python runner in `tools/e2e/` with tests): start the stack on loopback ports, wait for health, boot or pick the simulator and the emulator, run `patrol test` per platform, collect logs without content, stop everything it started by PID and container ID (Design note 3).
- Root Makefile target `e2e-mobile` that calls the runner (not part of `gates`, not part of `check`), with a help text; the README table "Make targets created by tickets" gets the row `e2e-mobile` with this ticket (brief index, `docs/briefs/README.md`).
- `.github/workflows/e2e-mobile.yml`: nightly (`schedule`) and `workflow_dispatch`, job on `ubuntu-24.04` with hardware acceleration, Docker and the Android emulator runner action pinned by SHA, running the Android suite; `permissions: contents: read`; no secrets; `make runners-check` and `make lint` stay green (Design note 4).
- `apps/client/integration_test/perf/mobile_perf_test.dart` and the evaluation: profile-mode runs with `flutter drive --profile` that write a timeline summary per target, and a comparison with the `client.perf.*` values (Design note 5); `make perf` calls it on a host with an attached iOS or Android reference device and prints a SKIP line otherwise.
- Tests of the runner and the evaluation (Tests).

Change:

- `apps/client/README.md` (how to run the mobile E2E and the measurement), `docs/quality/gates.md` (the target and the nightly workflow), `docs/spec/15-client-architecture.md` 20 (row "E2E on devices": the Compose overlay and the push handover), `docs/briefs/README.md` (row of the make target table), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-11-1` to `D-M5-11-6`.

## Out of scope

- Terminal E2E on mobile (M5-13 adds its suite to `e2e-mobile`).
- Real APNs and FCM delivery, TestFlight and Play tracks (M7).
- The desktop E2E (M4-34).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-11-mobile-e2e-perf`; one writer per worktree.
- Conventional Commits with the component as scope (for example `test(client): ...`, `ci(e2e): ...`, `build(compose): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the Compose stack publishes its ports only on `127.0.0.1` with random free ports chosen by the runner, never a reserved port of the host rules.
- Stop only processes and containers you started, by PID or container ID; simulators by UDID, emulators by serial. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; the Compose stack uses generated example secrets only, removed with the stack.
- The runs need Docker, macOS with Xcode for iOS, and the Android SDK with an emulator; the ticket must run `make e2e-mobile` at least once with both platforms on the build host or stop with `Verdict: BLOCKED <tool> missing`; the report names the host, the simulator and emulator images and the result.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never weaken a `client.perf.*` target.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Compose overlay** (M1-73; Ruling for docs/DECISIONS.md). The overlay adds the fake provider service and a seed job. M3-20 and M3-21 inject provider endpoints only in tests (`Deps.Endpoints`, `RootCAs`) and the FCM adapter refuses hosts other than Google's as an SSRF guard; no configuration key may redirect push in the release server. So the overlay runs an E2E server build: a separate `main` package (for example `services/server/cmd/bothy-e2e`, or the server built with a build tag `e2e`) that wires `Endpoints`, the fake's CA in `RootCAs` and an egress rule for the fake's host, is built only by the overlay, and is never part of the release image (a test asserts that the release build has no reference to it). Push credentials are generated example values that the fakes accept. The seed creates Alex as the instance owner (the bootstrap owner of M1-37) and Sam and Robin as members, all with local accounts and TOTP secrets known to the test, spaces Support and Engineering with channels and an agent-free thread in Support, and grants written for the suites rather than copied from 07 10: Alex and Sam `write` on Support, Robin `read` on Engineering; Alex follows the thread; Alex also has one active device registered by the seed (with its keys generated by the test) so that E4 of a new device can be confirmed through that device's session.

2. **Suites** (PLAN row; 15 20). Login: E1 with the stack's address, E2 password and TOTP (TOTP computed in the test from the seed), E4 with confirmation through a second session of the same person by API, then the main view. Push: the system permission dialog is accepted through `patrol` native automation; Sam writes in a thread Alex follows (API call from the test); the fake provider captures the payload; the test delivers it (iOS `xcrun simctl push <udid> <bundle> payload.json`; Android `adb shell am broadcast` to a debug-only receiver that calls the service with the payload, compiled only into the debug build and documented); the notification shows the real text; tapping opens the thread at the message. Share: an image from Photos into a thread through the share sheet (the extension on iOS, the activity on Android); the message appears for Sam. Offline: the runner cuts the device's network to the stack (simulator: stop the stack's published port; emulator: `adb emu network` or airplane mode), Alex sends, the M7 banner shows the waiting message, the network returns, Sam receives it exactly once.

3. **Runner** (Ruling for docs/DECISIONS.md). One runner script or program: chooses free loopback ports, writes a temporary Compose project name, starts the stack, waits for `/readyz`, runs the suites with the server address passed by `--dart-define`, stores `patrol` logs (no message content, no tokens), and always tears down what it started, also after a failure. It prints SKIP lines for a platform whose tools are missing and fails when no platform could run.

4. **CI** (ADR 0039; Ruling for docs/DECISIONS.md). The nightly workflow runs the Android suite only, because GitHub-hosted macOS runners have no Docker for the Compose stack; the iOS suite runs on a macOS host with Docker before every release (PLAN 5 row "End to end": "per milestone, before releases"). The workflow is not a required check of pull requests.

5. **Performance** (15 19; ADR 0063; P-07; Ruling for docs/DECISIONS.md). Profile builds run only on physical devices (Flutter runs only debug builds on the iOS simulator and the Android emulator), so the measurement needs an attached physical device; without one, only the evaluation tests on recorded timeline summaries run, the measurement prints its SKIP line, and the report lists the targets under "not proven". Measurement with `flutter drive --profile` and `traceAction` summaries: cold start until the sidebar from the cache (app process start to the first frame with the sidebar populated, measured with the engine's startup timing and a frame callback), opening a thread from the cache, the local echo on send, live box patch to frame (patches from the fake server of M4-17 inside the app, so the measurement isolates the client), jank ratio while scrolling a long thread, applied events per second in a catch-up of a large page set, and memory with five open threads (resident set from the platform tools). Each value is compared with its `client.perf.*` key; the output names the device model and OS and whether it is a reference device of 15 19 (a fixed list of model identifiers in the evaluation, with a reason per entry). Only a reference device counts as evidence; on other devices the run reports the values and ends without a pass. `make perf` calls the measurement when such a device is attached.

6. **Reaching the stack from a device** (M1-73 D1; SEC-TLS-1; Ruling for docs/DECISIONS.md). A published port of `api` delivers traffic from the bridge gateway, which SEC-TLS-1 refuses as plaintext, and widening `server.trusted_proxies` would weaken the rule under test (M1-73 D1). The overlay therefore adds a TLS sidecar with `network_mode: service:api` (a reverse proxy image pinned by digest) that terminates TLS for `localhost` and `api` (both names in the certificate) with a test CA generated per run and forwards to `127.0.0.1` in the shared namespace; `api` publishes the sidecar's port on `127.0.0.1`; `server.public_url` is `https://localhost:<port>`. The iOS simulator shares the host's loopback and trusts the CA through `xcrun simctl keychain <udid> add-root-cert`; the Android emulator reaches the same `localhost:<port>` through `adb reverse tcp:<port> tcp:<port>` and trusts the CA through a network security configuration compiled only into the debug build. M6-03 uses the same pattern for browsers.

7. **Rulings for `docs/DECISIONS.md`** (`D-M5-11-<n>`): the Compose overlay and fake providers (note 1); the push handover in the suites, the debug-only receiver on Android (note 2); the runner and teardown rule (note 3); Android-only nightly CI and iOS before releases (note 4); the measurement and the reference device list (note 5); the target `e2e-mobile` outside `check` (requirement 14); the TLS sidecar and device trust (note 6).

## Tests (write them first)

- Runner and evaluation (pytest in `tools/e2e/tests` or Dart tests): port selection avoids taken ports; teardown runs after a failing suite (fake commands); SKIP lines per missing tool; the evaluation passes and fails on recorded timeline summaries against the `client.perf.*` values; a non-reference device never yields a pass.
- The four `patrol` suites of Design note 2 (red first: run them before the overlay and the receiver exist and keep the failures).
- `mobile_perf_test.dart` with the measurement of Design note 5.
- The workflow test (the pattern of `tools/ci/tests`): `e2e-mobile.yml` has a schedule, is pinned, has read-only permissions and no secrets.

Commands that must pass, from the repository root:

- `make e2e-mobile` on the build host with an iOS simulator and an Android emulator (both suites green)
- `make perf` on a host with an attached physical reference device; without one, the evaluation tests and the SKIP line, and the targets listed as not proven
- `make runners-check`, `make lint`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `patrol` is green on iOS and Android against the Compose stack: login, push permission, tapping a notification, sharing, offline sending.
- [ ] The push in the E2E travels from the real server through the fake provider to the device, and the text is fetched from the real server.
- [ ] The measurement of the `client.perf.*` targets for mobile exists, its evaluation is tested, and a missed target on a physical reference device fails `make perf`; the report states the values per device, or that no physical reference device was attached (then the targets are checked at the closing of M5, PLAN 4 item 3, D-W-122).
- [ ] `e2e-mobile` exists outside `check`, is listed in the make target table of the brief index, and runs nightly for Android.
- [ ] README, gates.md, 15 20, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public symbol; comments explain why.
- `apps/client/README.md` and `docs/quality/gates.md` updated in English.
- Rulings of Design note 7 recorded in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make e2e-mobile`.
3. Review and red team verdicts and the findings fixed.
4. The performance values per target and device, and whether each device is a reference device.
5. What is not proven (for example real provider delivery; targets on reference devices if none was attached).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
