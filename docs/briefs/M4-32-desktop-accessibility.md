# M4-32: Accessibility on the desktop

Status: ready | Depends on: M4-22, M4-24, M4-28, M4-29, M4-33 | Milestone: M4

## Goal

After this ticket the desktop app has a written accessibility checklist for macOS (VoiceOver) and Windows (Narrator) according to 01 8, every item of it has automated evidence where a machine can give it, and every gap the session finds is fixed with a test that would have failed before. The checklist (`docs/quality/accessibility-checklist.md`) covers the rows of 01 8 for every desktop screen of M4 except the terminal (onboarding, sidebar, post list, thread view, composer, live box, cards, commands, files and voice, search, quick switcher, mentions, Running now, settings with devices and sessions, notification settings and the channel menu, administration A1 to A6): roles and labels, the live box as a container with a summary, trigger state labels, announcements only for state changes and questions, text scaling to 200 percent, focus order and F6, visible focus, targets, reduced motion, color never alone, the language of the app in the semantics tree. The automated evidence is a semantics test per screen in `flutter test` and a check of the platform accessibility tree on both desktop systems (macOS Accessibility API, Windows UI Automation) in the platform CI jobs, so that what VoiceOver and Narrator receive is tested, not assumed. The manual pass with the screen readers is prepared as a step-by-step section of the checklist that runs before every release (15 20 row "Accessibility by hand"). Why: WCAG 2.2 AA is a test target in 01 8, and the plan names the accessibility of Flutter on macOS as a risk (PLAN 10 "Risks").

## Read first

- `docs/PLAN.md`: section 3; section 10 (Exit criteria item 4, Risks: "accessibility of Flutter on macOS"; rows M4-18 to M4-29, M4-32, M4-33, M4-36).
- `docs/spec/01-product.md`: 5.1 and 5.6 (the screens), 6.2 and 6.3 (live box and trigger state labels), 7 (keyboard), 8 "Accessibility" (every row and the known risk), 9 (language).
- `docs/design/components.md` (the accessibility paragraph of every component, 2.4, 13 "Review list") and `docs/design/contrast.md`.
- `docs/spec/15-client-architecture.md` 20 (rows "Widget", "Golden", "Accessibility by hand").
- WCAG 2.2 success criteria named in 01 8 (1.4.3, 1.4.4, 1.4.11, 2.4.7, 2.5.8, 4.1.2, 4.1.3) and the Flutter documentation of `SemanticsTester`, `meetsGuideline`, `SemanticsService.announce`.
- Apple documentation of the Accessibility API (`AXUIElement`) and Microsoft documentation of UI Automation, for the platform tree checks.
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-116 (this ticket depends on every desktop screen ticket except the terminal, so the checklist has a fixed scope); section 10.
- Briefs: `docs/briefs/M4-07-core-components.md` (component semantics and `LiveAnnouncer`), `docs/briefs/M4-18-desktop-shell.md` to `docs/briefs/M4-29-admin-audit.md` and `docs/briefs/M4-33-channel-menu.md` (the screens and the keyboard layer).

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
9. No new configuration key; announcements keep using `a11y.live_announce_min_interval`.
10. A gap is fixed in the component or screen that has it, with a test in that member; this ticket may change any client member for that, never a server file.
11. Rule for the verdict: the session cannot listen to a screen reader; the ticket ends with `Verdict: DONE` when every checklist item has its automated evidence green, every gap found is fixed with a test, and the manual section is written; the manual pass itself is listed in report item 5 as not proven and runs before the release (15 20).

Product requirements implemented here:

12. Every interactive element has a role and a label; images and icons that carry meaning have a label; decorative ones are excluded from semantics (01 8, WCAG 4.1.2).
13. The live box is a container with a summary; only state changes and questions are announced, at most once per `a11y.live_announce_min_interval`; patches never (01 8, WCAG 4.1.3).
14. Trigger states, run states and badges have their own labels; states are never shown by color alone (01 8, components.md).
15. Text scaling to 200 percent loses nothing; the layout wraps or stacks (WCAG 1.4.4).
16. Focus order per column, F6 between columns, focus ring always visible (WCAG 2.4.7).
17. Targets at least `density.target-min` (WCAG 2.5.8); contrast AA for text and controls in light and dark (WCAG 1.4.3, 1.4.11).
18. Reduced motion from the system or the setting switches to `motion.reduced`.
19. The language of the app is set in the semantics tree; agent text without a language tag is not switched (01 8).

## Scope

Create:

- `docs/quality/accessibility-checklist.md` (English): one table per desktop screen with the rows of 01 8, the platform (macOS, Windows), the automated evidence (test file and name) and a column for the manual pass; a section "Manual pass before a release" with the steps for VoiceOver and Narrator; a section for later screens (terminal rows are added by M4-36; mobile by M5).
- `apps/client/test/a11y/` semantics tests per screen (Design note 1) and `apps/client/integration_test/a11y_platform_tree_test.dart` with the platform helpers `apps/client/macos/RunnerTests/AXTreeProbe.swift` and `apps/client/windows/runner/uia_probe.cpp` or a PowerShell UI Automation probe script under `apps/client/tool/` (Design note 2), run through `platform-tests.json`.
- Fixes with tests in `packages/design_system`, `packages/admin_ui` and `apps/client` for every gap found.

Change:

- `mkdocs.yml` (navigation entry for the checklist), `docs/quality/README.md` (link).
- `docs/design/components.md` only where a gap needs a design rule change.
- `apps/client/platform-tests.json` (the platform tree test).
- `docs/DECISIONS.md` section 10: rows `D-M4-32-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The terminal view's accessibility (M4-35 semantics, M4-36 adds its checklist rows).
- Mobile screen readers (TalkBack, VoiceOver on iOS: M5) and the console (NVDA, M6).
- The manual listening pass itself (before a release; Rules).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-32-desktop-accessibility`; one writer per worktree.
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

1. **Semantics tests.** Per screen a test that pumps it with fixtures and asserts with `SemanticsTester` or `find.bySemanticsLabel` the label, role flags and actions of every interactive element, the container and summary of the live box, the trigger state labels, the absence of semantics for decorative icons, and the locale of the semantics tree; plus `meetsGuideline` for `textContrastGuideline`, `labeledTapTargetGuideline` and the desktop target size; plus golden-free layout tests at text scale 2 that assert no overflow (`FlutterError` for overflow fails the test) and that every label is still reachable.

2. **Platform tree.** The integration test starts the app against the fake server, opens each screen, and calls a probe that reads the native accessibility tree of the app window: on macOS a small Swift helper using `AXUIElementCreateApplication` with the app's PID (the CI runner grants accessibility to the test runner, or the test reports itself as skipped with the reason "accessibility permission not granted" and the report names it); on Windows UI Automation through C++ or a PowerShell script with `System.Windows.Automation`. The probe asserts that the elements of the semantics test exist there with the same names and control types (button, text, list, tree item), that the live box container has a name, and that a state change raises a live region or announcement event where the platform exposes it.

3. **Checklist file.** Generated rows are not needed; the file is written by hand and a test (`tools/docs/tests/test_accessibility_checklist.py` or a Dart test) checks that every automated evidence it names exists as a test name in the repository, so the file cannot point at a deleted test. Add that test file to the Scope where it lives.

4. **Rulings for docs/DECISIONS.md** (`D-M4-32-<n>`): automated evidence per item (notes 1 and 2); the skip rule of the macOS probe; the checklist link test (note 3); the verdict rule (Rules).

## Tests (write them first)

- `apps/client/test/a11y/*_semantics_test.dart`: one per screen of the checklist (Design note 1), each written to fail on a missing label or role first.
- `apps/client/integration_test/a11y_platform_tree_test.dart` on macOS and Windows (Design note 2).
- A test for each gap found, in the member that had the gap, red before the fix.
- The checklist link test (Design note 3).

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- `cd packages/design_system && flutter test --coverage`, `cd packages/admin_ui && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] The checklist for VoiceOver on macOS and Narrator on Windows is complete for every desktop screen of the Goal, with automated evidence per item and the manual section for the release.
- [ ] Every gap found is fixed with a test that failed before the fix.
- [ ] Semantics tests per screen and the platform tree test on both systems are green (or the macOS probe's skip is named in the report).
- [ ] The checklist cannot point at a missing test; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01 8 or WCAG.
- `docs/quality/accessibility-checklist.md` (English) as in Scope.
- Rulings of Design note 4 in `docs/DECISIONS.md` section 10.
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
4. The gaps found (screen, item, fix, test), where the platform tree test ran and whether the macOS probe had the accessibility permission, and the list of checklist items that only the manual pass can prove.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
