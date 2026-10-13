# M5-13: Terminal E2E on mobile

Status: ready | Depends on: M5-10, M5-11, M5-12 | Milestone: M5

## Goal

After this ticket the terminal on the phones is tested end to end on devices against the Compose stack with a host service container (exit criterion 4 of M5): `make e2e-mobile` of M5-11 gains a terminal suite that pairs a host service container through the server's API (as an owner would on the desktop), assigns Alex a session user, and then on the iOS simulator and the Android emulator opens a terminal with the step-up device key and the simulated biometric match, runs a long command and ends it with Ctrl+C through the extra keys bar, cuts the device's network and reattaches after it returns with the scrollback replayed, moves the app to the background and checks that the app switcher image shows no terminal content, and checks the reattach within and after `terminal.quick_reattach_window`. With a physical reference device attached, the suite also measures `client.perf.terminal_frame_to_paint_p95` and `client.perf.terminal_throughput_min` on the phone with the output generator of the host service test image and the evaluation of M5-11 (15 19 lists them without a platform limit); without one, the evaluation runs on recorded summaries and the targets are listed as not proven. The accessibility checklist of M5-10 gains the rows for M8 and M9, checked with VoiceOver and TalkBack where the tools allow it and by machine-checkable tests for the rest (the line at the cursor readable, running output not announced, labels of the bar, font size). Why: the red team of M5 checks the terminal on a stolen unlocked phone, in the app switcher and under network loss (PLAN 11), and only an end-to-end run with a real host service shows the parts working together.

## Read first

- `docs/PLAN.md`: section 11 (Exit criteria 4 and 5, Test strategy "Terminal", Red team "Terminal"), rows M5-11, M5-12, M5-13; section 10 row M4-38 (desktop terminal E2E: pairing in the test, output generator).
- `docs/spec/24-terminal.md`: 3.2 (pairing), 6.1, 6.2 (detach, reattach, scrollback, quick reattach), 10.3, 12 (row Mobile), 14 (TM-T12, TM-T16 for network loss).
- `docs/spec/15-client-architecture.md`: 23.2, 23.5 (row "E2E": devices).
- `docs/spec/01-product.md`: 5.3 M8, M9; 8 (row "Terminal").
- `docs/quality/accessibility-mobile.md` (checklist of M5-10).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M5-13-<n>` rows) and the rows of M5-10, M5-11, M5-12.
- Briefs (merged names win, D-W-100): `M5-11-mobile-e2e-perf.md` (runner, overlay, suites, CI), `M5-12-mobile-terminal.md` (screens, bar, lifecycle, capture handling), `M5-10-mobile-a11y.md` (checklist format), `M2-48-terminal-conformance.md` (host service container and test overlay), `M2-41-host-pairing.md` (pairing through the API).
- Code on the base branch: `infra/compose/` (E2E overlay of M5-11, host service overlay of M2-48), `apps/client/integration_test/e2e/`, `tools/e2e/`.

This ticket touches no API operation, no table, no action, no event and no configuration key.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names; the host in the test is named "Build host" as in 01.
9. Windows only through named configuration keys (`terminal.quick_reattach_window`, `terminal.detach_grace` set short in the E2E configuration); code comments name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. `patrol` on iOS and Android against the Compose stack with a host service container: opening with biometrics, Ctrl+C through the bar ends a running command, network loss and reattaching with scrollback, app switcher image without terminal content (PLAN row).
12. Reattach within `terminal.quick_reattach_window` without step-up and after it with step-up, on the real server (TM-T12 from the device).
13. The checklist with VoiceOver and TalkBack for M8 and M9 is complete; machine-checkable items are tests.

## Scope

Create:

- `apps/client/integration_test/e2e/terminal_e2e_test.dart`: the suite of Design note 2.
- `apps/client/test/a11y/terminal_a11y_test.dart` and the platform audit entries for M8 and M9 (Design note 3).

Change:

- `infra/compose/test/compose.e2e-mobile.yaml` (M5-11): include the host service container of the M2-48 test overlay, a session user for Alex, and the short terminal windows; the seed pairs the host through the API (pairing code, fingerprint confirmation, LLM login `none`, session user assignment) or reuses the pairing helper of M2-48.
- `tools/e2e/` runner: the terminal suite in the mobile run; the host container's lifecycle under the same teardown rule.
- `.github/workflows/e2e-mobile.yml`: the Android job runs the terminal suite too.
- `docs/quality/accessibility-mobile.md`: rows for M8 and M9.
- `apps/client/README.md`, `docs/spec/15-client-architecture.md` 23.5 (row "E2E" for devices), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-13-1` to `D-M5-13-3`.

## Out of scope

- The console terminal (M6-18).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-13-mobile-terminal-e2e`; one writer per worktree.
- Conventional Commits with the component as scope (for example `test(client): ...`, `build(compose): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the stack publishes its ports only on `127.0.0.1` with free ports chosen by the runner.
- Stop only processes and containers you started, by PID or container ID; simulators by UDID, emulators by serial. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; never log terminal content or keys.
- The run needs Docker, macOS with Xcode for iOS and the Android SDK with an emulator; the ticket must run `make e2e-mobile` with the terminal suite at least once on both platforms on the build host or stop with `Verdict: BLOCKED <tool> missing`; the report names where it ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Host in the stack** (24 3.2; M2-48; Ruling for docs/DECISIONS.md). The host service container joins the Compose network of the E2E overlay and connects over TLS to the sidecar of M5-11 Design note 6 under the name `api` (the certificate names `localhost` and `api`; the host trusts the test CA), never in plaintext (24 2.2, SEC-TLS-1). The pairing declares the host as the server host (the host itself does not detect it, because `api` is not its own address; do not assert `reported_is_server_host`), and only owners may open terminals on it (SR10); Alex is the instance owner in the M5-11 seed, so the owner path is what the suite tests; the seed runs the pairing through the API with an owner session (code, `bothy-host pair` inside the container with the code, fingerprint confirmation with the first characters) and assigns Alex the session user the container reports. Create the host with `assigned_user_id` set to Alex and grant `terminal` on the host to Alex (24 8); as owner on the server host Alex passes SR10, and a test that a non-owner (Sam, given a `terminal` grant on the host and a fresh step-up with the device key, so that visibility and step-up pass first) is refused there with `terminal_denied` `server_host_owner_only` covers the other side; the network cut of the suite affects only the device's path, never the host container's.

2. **Suite** (PLAN row). On each platform: log in as Alex (M5-11 setup); open M9, see "Build host" online; open a `shell` session with the step-up device key and the simulated biometric match; type `echo e2e-terminal` and see it; start `sleep` with a long duration, tap Ctrl then `c` on the bar, see the prompt return; cut the network (as M5-11 does), wait less than `terminal.detach_grace`, restore it, see "detached" and then the reattach with the earlier output replayed; send the app to the background and back within `terminal.quick_reattach_window` and see no step-up prompt, then again after the window and see the prompt; during the background state assert the privacy cover (iOS) or `FLAG_SECURE` (Android) and take the system snapshot where `patrol` can, asserting that no terminal text appears in it.

3. **Accessibility** (01 8 row "Terminal"; components.md 12; M5-10 format). Tests: the line at the cursor is exposed to semantics; a command "read screen" exposes the visible screen; running output does not trigger announcements; the bar keys have labels including the latched state; the terminal font follows the system text size and the separate terminal size setting. The platform audits of M5-10 run on M8 and M9. Manual items (a person with VoiceOver and TalkBack on a device) stay "manual before release" in the checklist.

4. **Rulings for `docs/DECISIONS.md`** (`D-M5-13-<n>`): the host in the E2E stack and its pairing (note 1); the snapshot assertion per platform (note 2); the accessibility checks of M8 and M9 (note 3).

## Tests (write them first)

- `terminal_e2e_test.dart` (Design note 2), red first before the overlay includes the host.
- `terminal_a11y_test.dart` and the audit entries (Design note 3).
- Runner tests: the host container is torn down after a failing suite (fake commands).
- The terminal measurement on a physical device (profile build) with the evaluation of M5-11; its evaluation tests on recorded summaries.

Commands that must pass, from the repository root:

- `make e2e-mobile` on the build host with an iOS simulator and an Android emulator (all suites green, including the terminal suite)
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- `make runners-check`, `make lint`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `patrol` on iOS and Android: opening with biometrics, Ctrl+C through the bar ends a running command, network loss and reattaching with scrollback, app switcher image without terminal content.
- [ ] Reattach within the window without step-up and after it with step-up against the real server.
- [ ] `client.perf.terminal_frame_to_paint_p95` and `client.perf.terminal_throughput_min` are measured on a physical reference phone and a miss fails `make perf`, or the report lists them as not proven for the M5 closing (D-W-122).
- [ ] The checklist with VoiceOver and TalkBack for M8 and M9 is complete.
- [ ] Overlay, runner, workflow, README, 15 23.5, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public symbol; comments explain why.
- `apps/client/README.md` and the checklist updated in English.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
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
4. Where the E2E ran (host, simulator and emulator images).
5. What is not proven (for example a physical phone, a hardware keyboard, screen recording on iOS).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
