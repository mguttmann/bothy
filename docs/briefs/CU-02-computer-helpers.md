# CU-02: Computer helpers for macOS and Windows

Status: ready | Depends on: none (specification merged) | Milestone: CU

## Goal

After this ticket the "Bothy Computer Helper" exists for macOS (Swift) and Windows (C++, Win32) as the only process with screen capture and input rights, in the owner's desktop session: capture, input, app identity, lock and secure-desktop state, keeping the display awake, the Esc tap, the indicator with a stop button, the enablement switch with its generation, and the helper side of the local channel of `docs/spec/27-computer-use.md` 9.4. It goes into a host package with an optional helper component, is signed on the owner's own machines and never published (D-CU-01-29), and a probe job tells whether hosted CI runners can prove capture and input; native jobs in `host-platform.yml` then test it.

## Read first

- `docs/spec/27-computer-use.md` sections 1, 4, 6.4, 6.5, 8.2 to 8.4, 9 (all), 15 (CU-T13 to CU-T16, CU-T23), 16 and 18.
- `docs/spec/24-terminal.md` 2.2 (packages and install of the host service), 6.4 (the confirmation agent and its console-user rule, the model for the peer side); `docs/spec/18-release.md` 6.3 (the host package chain the helper packages reuse); `docs/decisions.d/CU-01.md` D-CU-01-29 (signing on the owner's machines).
- `services/host/testdata/computer/channel-vectors.json` (the channel vectors every codec must read).
- Code on the base branch: `services/host/Makefile` (targets `release-apple`, `release-windows`, `platform-test`), `tools/distribution/hostdist.py` (its package and signing steps, which the helper packages reuse), `.github/workflows/host-platform.yml`, `infra/host/` (package definitions), `services/host/internal/confirm` (dialog agent in the desktop session).
- Design note `docs/design/computer-use.md` (context and evidence; where it differs from the chapters, the chapters win); ADR 0078; `docs/decisions.d/CU-01.md`; `docs/security/threat-model.md` 5.18 and 6.10.

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
8. White label: examples, templates and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no internal script names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it.
11. Additive only: every new key defaults to off or to the behaviour before, and an installation that sets none of the new keys (`runner.computer_use`, `host_service.computer_use`) and enables no helper behaves exactly as before.
12. Full operation (owner ruling OR-1, D-CU-01-1): build no app tier, no key or app block list, no confirmation per session or action and no lock on the Bothy windows; the only protections are those of `docs/spec/27-computer-use.md` section 1 item 3.

Note on rule 1: "stages" are the milestones and the stages A1, AR and CU of the plan (PLAN P-27, P-29, P-30).

Product requirements implemented here:

13. Both helpers implement the interfaces `Capturer`, `Injector`, `AppInspector`, `LockMonitor`, `Waker`, `EscTap`, `Indicator`, `EnablementStore` and `HostChannel` (names in the language's style); every platform call sits behind one of them, so the logic runs against fakes in unit tests.
14. The channel codec reads every vector of `channel-vectors.json`, and its own output decodes to the same objects; frames longer than `host_service.computer_frame_max_bytes` are refused.
15. Enablement: off after installation; the switch in the menu bar or tray turns it on only by a click of the person (no API, no command line, no file the helper reads as an order); every switch-on raises the generation by one and persists switch and generation in the helper's own state file in the user's account (mode `0600`); `disable` and a local switch-off end a live session.
16. `show` starts the indicator (border window excluded from the capture, stop button, menu bar or tray text "Agent <name> controls this computer, Esc to stop"), the Esc tap and keeping the display awake; `hide`, a broken channel and the helper's start release every key and button and undo all three.
17. Esc tap: a real Esc ends the session with `local_escape` and is swallowed; an Esc the helper injects for `key` passes to the app and stops nothing; a stop button click ends it with `indicator_stop`.
18. Capture and scaling: one screen (the one with the frontmost window, else the main screen), native resolution scaled into `max_edge`, JPEG or PNG; the `done` carries `screen` with origin, size and scale; `region` for `zoom`.
19. Input: clicks with modifiers, double and triple click, move, scroll with direction and amount, Unicode text independent of the keyboard layout, any key or combination with `repeat`; no key or app is refused by the helper.
20. Lock and secure desktop: `state` with `locked` on lock, screen saver, user switch and display sleep, `secure_desktop` on Windows; every `do` and `capture` while locked answers `screen_locked` and injects nothing; an operating system refusal answers `os_refused`.
21. App identity: bundle ID and team ID (macOS), executable name and signer subject (Windows) of the frontmost app in every `done`.
22. Permissions (macOS): `CGPreflightScreenCaptureAccess` and `AXIsProcessTrusted` reported in `hello` and `state`; a menu item opens the matching pane of the system settings; the helper never grants itself a permission.
23. Packaging (normal host mode only; the owner account mode does not support computer use, 27 9.4): macOS app bundle with `Info.plist` usage texts, hardened runtime, no sandbox, LaunchAgent in the GUI domain, inside a host `.pkg` with the helper as an optional component; Windows executable with `uiAccess=true` in its manifest under `Program Files`, start entry at login, inside a host MSI with the helper as an optional component; selecting the component sets `host_service.computer_use = true`. These packages are built on the owner's machine from the signed helper (D-CU-01-29); the packages of `release.yml` stay without a helper.
24. Signing (D-CU-01-29, replacing the release jobs and their secrets): a `services/host/Makefile` target signs the macOS helper on the owner's Mac with the owner's Developer ID Application identity and the hardened runtime, as a one-shot job in the owner's GUI session (the login keychain is open there; nothing unlocks it), without notarization; a `hostdist.py` step signs the Windows helper on the owner's Windows computer with a self-signed code signing certificate trusted only there and checks it with `signtool verify /pa`; the macOS signing step compares the designated requirement with the one of the previous signed build and fails on a change; a helper without a signature that verifies marks itself as a development build (`signature: development`). No signing secret goes to GitHub, and `release.yml` neither builds nor signs a helper.
25. CI: first a probe job in `host-platform.yml` on `macos-15` and `windows-2025` that opens a test window, captures it and injects a click and a key, and records whether it worked; then native jobs that run the helper against a fixture window with a fake host channel (click, type, image check, lock state where the runner allows). A platform whose probe fails keeps build, signature and doctor-free tests only, and the result goes into 27 section 16 and the report.

## Scope

Create:

- `services/host/desktop/macos/` (Swift package and app bundle assembly, `README.md`), `services/host/desktop/windows/` (CMake project, manifest, `README.md`), `services/host/desktop/README.md`.
- Their unit tests (XCTest; a C++ test runner pinned in the CMake project) against fakes and the channel vectors.
- `docs/decisions.d/CU-02.md`.

Change:

- `services/host/Makefile` (targets that build and sign the helpers and build the packages with the helper component), `tools/distribution/hostdist.py` and its tests, `infra/host/` package definitions, `.github/workflows/host-platform.yml` (probe and native jobs) and its workflow tests in `tools/ci/tests`, `services/host/README.md`, `docs/spec/27-computer-use.md` section 16 (probe results) and section 9 where the code needs a precise name.

## Out of scope

- The host service side of the channel, the doctor check and the Linux test backend (CU-03).
- Server, MCP server, runner and clients (CU-04 to CU-07).
- Dragging, holding keys or buttons, several screens (CU-08); a background mode (CU-11); a Windows system service (CU-13).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/CU-02-computer-helpers`, created from the branch of the stage that holds CU-01; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers.
- Do not push and do not open a pull request: the stage lands as one pull request with CU-01 to CU-07 and one CI round (PLAN P-30, `AGENTS.md`); the coordinator merges your branch into the stage branch. Never push to `main`, never tag, never create remote repositories.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules); no GUI or input test on a maintainer's machine and no run with a real model; native GUI tests run only in CI jobs.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers. Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, attach tokens, DPoP keys, signing material and screenshots never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money; a check of the pinned CLI runs only without a login against a local stand-in for the API on `127.0.0.1`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/27-computer-use.md`, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that `docs/spec/27-computer-use.md` has section 9.4 and the vectors file exists, else `Verdict: BLOCKED specification missing`.
2. **Probe first.** Push nothing; write the probe job so that a later CI run of the stage records its result. Until that run, keep the native jobs behind the probe result (a job output), so a failed probe makes them skip with a visible notice instead of failing.
3. **Indicator excluded from capture.** macOS: `SCContentFilter` excluding the indicator window; Windows: `SetWindowDisplayAffinity` with `WDA_EXCLUDEFROMCAPTURE` (Windows 10 2004 or later).
4. **Injected or real Esc.** macOS: compare the event source state of the tap's event with the helper's own source; Windows: `LLKHF_INJECTED`. Test both with the fake engine; the real behaviour is a probe of section 16.
5. **Keeping awake.** macOS `IOPMAssertionCreateWithName` with `kIOPMAssertionTypePreventUserIdleDisplaySleep`; Windows `SetThreadExecutionState(ES_CONTINUOUS | ES_DISPLAY_REQUIRED)`; both released on `hide` and every end.
6. **Rulings for `docs/decisions.d/CU-02.md`:** the build system of each helper, the test runner of the C++ project, the state file paths, the capture API on Windows after the probe, every name that differs from chapter 27.

## Tests (write them first)

- Swift (XCTest) and C++ unit tests against fakes: the channel vectors; enablement and generation; `show` and `hide`; Esc real and injected (CU-T14); every key and combination of a shortcut table passed unchanged (CU-T15); release on `hide`, on a broken channel and at start (part of CU-T16); `screen_locked` while locked (part of CU-T13); scaling and the `screen` member.
- Distribution tests in `tools/distribution` for the optional helper component and the configuration it sets.
- The probe job and the native jobs in `host-platform.yml` (run by the stage's CI round, not locally).

```sh
(cd services/host/desktop/macos && swift test)
cmake -S services/host/desktop/windows -B <temporary build folder> && cmake --build <temporary build folder> && ctest --test-dir <temporary build folder>
uv run --project tools --locked pytest tools/distribution
make check
```

## Acceptance criteria

- [ ] Both helpers read every channel vector and pass their unit tests on their platform; the macOS test runs on macOS, the Windows test in the CI job (cross-checking the C++ code locally only where a Windows toolchain exists).
- [ ] The locally built packages carry the helper as an optional component; selecting it sets `host_service.computer_use`; the local signing steps of D-CU-01-29 cover it; `release.yml` publishes no helper.
- [ ] The probe job and the native jobs exist; their result after the stage's CI round is in 27 section 16.
- [ ] No app, key or setting is refused by the helper (CU-T15).
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- An entry `changelog.d/CU-02.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/CU-02.md`, one row each with date, ruling and reason; every name that differs from chapter 27 is a ruling and a change of chapter 27 in the same commit.
- An ADR only if you take an architecture decision beyond ADR 0078 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green for the components you changed.
- An independent review of your diff by a fresh reviewer (one follow-up round at most); record its verdict. The red team of the stage runs once over the whole stage diff before the pull request; it checks at least, for this ticket: the helper refuses nothing the operating system accepts; a real Esc and the stop button always end the session; an injected Esc never does; no key stays pressed after any end; the enablement cannot be switched on except by a click at the machine; the indicator is never in a screenshot; no image is written to disk.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. The review verdict and the findings fixed.
4. What is not proven (at least: the native behaviour on a real desktop beyond what the CI probe shows, and the rows of 27 section 16 that the probe could not settle), each probe of `docs/spec/27-computer-use.md` section 16 you ran with its result, and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
