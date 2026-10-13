# M4-38: Terminal E2E and performance targets on the desktop

Status: ready | Depends on: M4-34, M4-36, M4-37 | Milestone: M4

## Goal

After this ticket the terminal of the desktop app is tested end to end and measured: the terminal stack of M4-36 (the Go stack of M2-48 with its TLS front and host container) gains an E2E mode with an unpaired host and an output generator, and the E2E suite gains the terminal flow with `patrol` on macOS and `integration_test` on Windows: an owner pairs a host through the administration of M4-37 (pairing code, the host container runs `bothy-host pair`, fingerprint confirmation, LLM login and server host answers, session user assignment, grant `terminal`), then the person opens a terminal with step-up, runs a command and sees its output, detaches, reattaches with the scrollback, and ends the session. The performance level gains `client.perf.terminal_frame_to_paint_p95` (a decrypted `data` frame until drawn) and `client.perf.terminal_throughput_min` (bytes per second drawn during continuous output without backlog), measured with a profile build against the generator, failing when missed. Why: PLAN 10 exit criterion 5 asks for the terminal E2E and both targets on the desktop; only the real path (server relay, host service, PTY, Noise, renderer) shows whether the view keeps up.

## Read first

- `docs/PLAN.md`: section 3; section 10 (Exit criteria item 5, Test strategy "Terminal": output generator for the performance targets; rows M4-34 to M4-37); section 8 row M2-48.
- `docs/spec/15-client-architecture.md`: 19 (`client.perf.terminal_frame_to_paint_p95`, `client.perf.terminal_throughput_min`), 20, 22 (defaults), 23.2, 23.5 (rows E2E and Performance).
- `docs/spec/24-terminal.md`: 3.2 (pairing), 6.1 and 6.2 (open, detach, reattach), 11.1 (profiles: a `command` profile for the generator), 14.
- `docs/spec/01-product.md` 5.1 D11 and 5.2 A8.
- `docs/DECISIONS.md`: D-W-100, D-W-114; the rulings of M4-34 on where E2E and performance run (`D-M4-34-<n>`); section 10.
- Briefs: `docs/briefs/M4-34-desktop-e2e-perf.md` (`infra/compose/e2e.sh`, the E2E overlay, `make e2e-desktop`, `make perf-desktop`, the run-location rule), `docs/briefs/M4-36-terminal-desktop.md` (`make terminal-stack`, `infra/compose/terminal-client.sh`), `docs/briefs/M4-37-admin-hosts.md` (the pairing screens), `docs/briefs/M2-48-terminal-conformance.md` (host containers, `bothy-host install` and `pair` in the harness), `docs/briefs/M2-45-host-pty-profiles.md` (profiles `shell`, `claude` and `command`).

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
9. Targets only from `AppConfig` (`client.perf.terminal_frame_to_paint_p95`, `client.perf.terminal_throughput_min`); no literal target in a test.
10. The run-location and verdict rule of M4-34 applies unchanged: the macOS suite and the macOS targets run on the build host when it is a Mac with Go and a Docker engine; a Windows run needs a Windows host that reaches the stack, otherwise it is listed as not proven with the command; the session never claims a run it did not do.

Product requirements implemented here:

11. E2E against the Compose stack with a host service container: pair a host, open a terminal, run a command, detach, reattach, end; `patrol` on macOS and `integration_test` on Windows (15 20, 23.5).
12. Pairing goes through the real administration screens and the real `bothy-host pair` in the container; the fingerprint is typed from what the container prints (24 3.2).
13. `client.perf.terminal_frame_to_paint_p95` and `client.perf.terminal_throughput_min` measured with a profile build and an output generator in the host service; a missed target fails the performance level (15 19, 23.5).
14. No terminal content in logs, crash reports or metrics during the runs (R-CL-17): the E2E run ends with the canary check of M4-36 on the real stack.

## Scope

Create or change:

- `services/server/test/terminal/clientstack/` (M4-36): a mode for the E2E run in which the host container is installed but not paired (the test pairs it through the administration), the generator is configured as a `command` profile of the host service, and a step runs `bothy-host pair --server <origin> --code <code>` inside the host container with the code the test writes to a file and returns the printed fingerprint; nothing new is published. The stack has the TLS front and the test CA of M2-48, which the host service needs (it dials only `https` origins, M2-43), so the terminal E2E runs on this stack and not on the plain HTTP overlay of M4-34.
- The generator: a small program inside the host image (Go next to the stack helper, or a shell script if the image has no Go toolchain at run time) that writes lines of known length with a sequence number at a fixed rate, or at full speed for a fixed byte count.
- `packages/terminal_ui` (M4-35): a test-only timing hook in the controller and the renderer (the time a `data` frame was decrypted, the time the frame that shows it was rasterized, and the renderer's backlog), compiled only when the define `TERMINAL_PERF_HOOKS` is set, so release and normal profile builds contain none; a test proves the hook is absent without the define.
- `apps/client/e2e_test/terminal_e2e_test.dart` (and the `patrol` variant if `patrol` needs its own folder), outside `integration_test/` like the E2E suite of M4-34: Design note 1.
- `apps/client/perf_test/terminal_perf_test.dart` and the driver additions of M4-34: Design note 2.
- Root `Makefile`: `e2e-desktop` and `perf-desktop` (M4-34) gain the terminal parts, which start the stack of M4-36's helper in the E2E mode; `docs/quality/gates.md` rows updated.
- `apps/client/README.md` (the terminal parts).
- `docs/DECISIONS.md` section 10: rows `D-M4-38-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Mobile and console terminal E2E (M5, M6 with SEC-T05).
- Two-node relay tests (M2-48 TM-T15) and chaos tests (TM-T16) beyond what the E2E flow does.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-38-terminal-e2e-perf`; one writer per worktree.
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
- The host container runs only inside the project of the run, with no published port; the generator writes only synthetic output; cleanup removes the containers, volumes and temporary folders of the run.

## Design notes

1. **E2E flow.** Alex (owner) logs in to the stack of the E2E mode (its world as M4-36's helper creates it, without the host pairing), opens Administration, Hosts, creates a host with Alex as assignee, gets the pairing code; the test hands the code to the pairing step of the stack helper, which runs `bothy-host pair` and returns the printed fingerprint; Alex types its first characters, answers organization and "no" for the server host, assigns his session user from the reported names (owners have `terminal` implicitly, 24 8.1; the session user assignment is still required, SR12). Then Alex opens the group "Hosts", the host, a new `shell` session (step-up), types `echo` with a canary-free marker and sees it, detaches (the session stays `detached`), reattaches and sees the scrollback replayed, runs `exit` and sees the exit code, ends the tab. At the end the canary check of M4-36 runs against the stack's logs and the app's paths.

2. **Terminal targets.** Frame to paint: the generator writes one line per 100 milliseconds with a sequence number; the app's profile build records, per `data` frame, the time it was decrypted (instrumented in the controller of M4-35 through a test-only hook that the release build compiles out) and the time of the next rasterized frame that shows that sequence number; p95 over at least 1000 frames, compared with `client.perf.terminal_frame_to_paint_p95`. Throughput: the generator writes at full speed for a fixed byte count; the measurement is the drawn bytes per second while the renderer's backlog stays below one screen (no growing queue), compared with `client.perf.terminal_throughput_min`. Both values go to `apps/client/build/perf/summary.json` with the device, marked as reference device or not (M4-34 Design note 4).

3. **Rulings for docs/DECISIONS.md** (`D-M4-38-<n>`): the pairing step through the container (Scope), the generator design and the measurement points (note 2), the test-only hook and its removal from release builds (note 2).

## Tests (write them first)

- The E2E terminal test (Design note 1) through `make e2e-desktop` on the build host for macOS when it is a Mac with a Docker engine.
- The terminal performance test (Design note 2) through `make perf-desktop`, plus a unit test of the throughput and p95 computation on recorded samples.
- A Go test of the E2E mode of the stack helper: the host starts unpaired, the pairing step runs only inside the project, and no port is published beyond the TLS front.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on a Mac with Go and a Docker engine: `make image`, `make e2e-desktop`, `make perf-desktop`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `patrol` on macOS against the Compose stack with a host service container: pair a host, open a terminal, run a command, detach, reattach, end, green on the build host (or the report says why not and names the command); `integration_test` on Windows built and green where a Windows host reached the stack, otherwise listed as not proven.
- [ ] `client.perf.terminal_frame_to_paint_p95` and `client.perf.terminal_throughput_min` are measured and met on the device named in the report; a missed target fails `make perf-desktop`.
- [ ] The canary check finds no terminal content in the stack's logs or the app's paths; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol and a header comment in every script; comments explain why and cite 15 19 or 23.5.
- READMEs (English) as in Scope.
- Rulings of Design note 3 in `docs/DECISIONS.md` section 10.
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
4. The E2E terminal results with where they ran, both target values with the device and whether it is a reference device, the generator settings used, and the proof that release builds contain no timing hook.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
