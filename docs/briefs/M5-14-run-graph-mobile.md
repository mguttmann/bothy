# M5-14: Run graph and control room on iPhone and Android

Status: ready | Depends on: M4-39, M5-03 | Milestone: M5

## Goal

After this ticket the run graph and the control room of M4-39 work on the phone: screen M11 ("Running now" as cards in one column from the tab "Running", the switch "Cards" and "List", filters as a sheet, Stop where allowed) and screen M12 (the run graph as indented rows with a narrow time bar below each label in portrait and the full time axis beside the labels in landscape, opened from the live box sheet of M3 and from a card). Both use the derivation and the views of `packages/bothy_viz` from M4-39 in density `touch`; the visible cards are live only while the app is in the foreground. Android uses the same screens with its own controls (01 5.4). Why: the owner wants to see what runs are doing right now also on the go (2026-10-24; D-RV-11).

## Read first

- `docs/design/run-visualization.md`, `docs/decisions.d/run-visualization.md`.
- `docs/PLAN.md`: section 3; section 11 rows M5-02, M5-03, M5-10, M5-14; section 10 row M4-39.
- Dependencies: the header names M4-39 and M5-03; everything below them (M5-02, M4-21, M4-23, M2-51, M6-11) is implied.
- `docs/spec/01-product.md`: 5.3 M3, M11, M12; 5.4 (Android); 8; 13 (R-PR-15, R-PR-16).
- `docs/spec/09-agents.md` 5.6; `docs/spec/04-sync-protocol.md` 11.3 items 2 and 7; `docs/spec/15-client-architecture.md` (foreground rule of the gateway connection on mobile).
- `docs/spec/25-visualization.md` 7.1 (row `lib/src/run_graph/`), 7.3, 7.4.
- Briefs: `docs/briefs/M4-39-run-graph-desktop.md`, `M5-02-iphone-layout.md`, `M5-03-android-layout.md`, `M5-10-mobile-a11y.md`; merged code wins on names.
- Code on the base branch: `packages/bothy_viz/lib/src/run_graph/`, `apps/client/lib/running/`, the phone layouts of M5-02 and M5-03.

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
9. Limits only through named keys from `AppConfig` (`ui.livebox_visible_steps`, `a11y.live_announce_min_interval`, `sync.max_focused_threads`), never a literal limit in code.
10. No permission logic in the client: Stop only where the run's capabilities allow `run.cancel` (15 1).

Product requirements implemented here:

11. M11 as 01 describes it: cards in one column, switch "Cards" and "List", filters as a sheet, a tap opens M12, Stop at full width where allowed; touch targets at least `density.target-min`.
12. M12 as 01 describes it: the same rows as D14 in density `touch`, a narrow time bar below each label in portrait and the time axis beside the labels in landscape; tap collapses or expands; "As list".
13. Live only in the foreground: the focus of visible cards ends when the app moves to the background; on return the views reload the documents through the client rule of 04 11.3.
14. Screen readers (VoiceOver, TalkBack) read the list forms with the tree semantics of M4-39.

## Scope

- `packages/bothy_viz/lib/src/run_graph/`: the touch layout of the trace view (bars below the labels in narrow widths) as a parameter of the existing view, no second derivation.
- `apps/client/lib`: the phone routes for M11 and M12, the "Graph" button in the live box sheet of M3, the tab "Running" on the control room, Android back behaviour per 01 5.4.
- ARB keys for new texts; goldens for phone sizes.

Fragments: `docs/decisions.d/M5-14.md` (`D-M5-14-<n>`), changelog fragments of the touched components; README of `apps/client`.

## Out of scope

- Server changes (M2-51); the desktop views and the derivation (M4-39); the web console; TV mode.
- Push notifications about stalls (01 6.2: a stall shows a notice, no push).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-14-run-graph-mobile`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(bothy-viz): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package.
- Do not modify files outside the scope above, except the README and fragment files named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); device and simulator runs on the Mac-bound host only through the launcher's rules.
- Tests bind only `127.0.0.1` with port 0 and make no network call; the fake server replaces the server. Time comes from an injected clock, never from a fixed wait.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH`; without it the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M5 closed, in the manner of D-W-100 and D-W-120. Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **One view, two layouts.** The trace view of M4-39 gets a layout parameter (`wide`, `narrow`) chosen from the available width; portrait phones use `narrow` (bar below the label), landscape and tablets `wide`. The derivation is not touched.
2. **Foreground.** The control room registers its focus only while the app lifecycle is `resumed`; on `paused` it releases it. The cards then show the summary until the next foreground.
3. **Rulings for `docs/decisions.d/M5-14.md`** (`D-M5-14-<n>`): the width threshold between `narrow` and `wide` (from the design tokens, never a literal), the filter sheet.

## Tests (write them first)

- `packages/bothy_viz/test/run_graph/narrow_layout_test.dart`: the same rows and values in `narrow` and `wide`; the bar below the label in `narrow`.
- Goldens of M11 and M12 in density `touch`, portrait and landscape, light and dark, text scaling 1 and 2, `de` and `en`, context `reduced`.
- `apps/client/test/running/mobile_control_room_test.dart`: tab "Running" opens M11; a tap opens M12; Stop only with the capability; the focus is released on `paused` and requested again on `resumed`.
- `apps/client/integration_test/mobile_run_graph_test.dart` on the iPhone and Android layouts against the fake server: a scripted run with a subagent grows live in M12.
- Accessibility: a checklist run with VoiceOver and TalkBack over both list forms (report), as M5-10 does.

Run the added tests by name with ten repetitions (`--plain-name`) through the machine's test slot.

Commands that must pass, from the repository root:

- `cd packages/bothy_viz && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- `make apps-platform`; otherwise its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make docs`, `make fragments-check`
- `make check`

## Acceptance criteria

- [ ] M11 and M12 work on iPhone and Android layouts as 01 describes, with the derivation of M4-39 unchanged.
- [ ] Live only in the foreground; the focus is released in the background.
- [ ] Goldens recorded; VoiceOver and TalkBack checklist done.
- [ ] Rulings and fragments recorded; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01.
- `apps/client/README.md` updated.
- Rulings in `docs/decisions.d/M5-14.md`; changelog fragments as in Scope.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
