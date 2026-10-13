# M5-10: Accessibility on mobile

Status: ready | Depends on: M5-06, M5-07, M5-08, M5-09 | Milestone: M5

## Goal

After this ticket the phone apps meet the accessibility rules of 01 8 (WCAG 2.2 level AA) on iOS with VoiceOver and on Android with TalkBack, held by tests instead of intentions: a checklist `docs/quality/accessibility-mobile.md` lists every rule of 01 8 for every phone screen of M5 (M1 to M7, E1 to E7, the share sheet of M5-06, the recording of M5-07, the lock of M5-08, the emergency actions of M5-09) with how it is checked; every item that a machine can check is a test (Flutter semantics and guideline tests, `XCUIApplication.performAccessibilityAudit()` on the iOS simulator, the Accessibility Test Framework checks on Android), and the session works through the checklist on the simulator and emulator with VoiceOver and TalkBack where the tools allow it; every gap found is fixed in this ticket and pinned by a test; the items that need a person with a screen reader on a physical device stay on the checklist with the state "manual, before every app release" (PLAN 5 row "Accessibility") and are named in the report. Terminal screens M8 and M9 are checked by M5-13, the live views M10 by M6-16. Why: exit criterion 3 of M5 requires the checklist with VoiceOver and TalkBack, and 01 8 makes accessibility a test, not a hope.

## Read first

- `docs/spec/01-product.md`: 8 "Accessibility" (the table: contrast, screen reader, live content, font size, focus, targets, motion, color, language), 5.3 and 5.4 (screens), 5.6 (onboarding rules: codes read character by character, remaining time announcements), 9 (languages: the semantics tree carries the app language).
- `docs/spec/15-client-architecture.md`: 20 (rows "Widget", "Accessibility by hand").
- `docs/PLAN.md`: section 5 (row "Accessibility": checklist by hand before every app release), section 11 (Exit criteria item 3), rows M4-32 (the desktop checklist this one mirrors), M5-10, M5-13.
- `docs/design/components.md` (semantics per component), `docs/design/contrast.md`.
- Apple: `XCUIApplication.performAccessibilityAudit(for:_:)` (Xcode 15 and later). Android: the Accessibility Test Framework (`AccessibilityChecks.enable()` in Espresso, or `AccessibilityValidator`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M5-10-<n>` rows).
- Briefs (merged names win, D-W-100): the brief of M4-32 (desktop checklist format; use it where its checklist exists on the base branch), `M5-02-iphone-layout.md` to `M5-09-mobile-emergency.md` (the screens and their semantics), `M4-07-core-components.md` (component semantics).
- Code on the base branch: `apps/client`, `packages/admin_ui`, `packages/design_system`.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names. Interface texts come from `packages/l10n`.
9. No configuration key is added; announcements follow `a11y.live_announce_min_interval`.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Every rule of 01 8 holds on every phone screen named in the Goal, in de and en, light and dark, text scaling 1 and 2 (up to 200 percent).
12. Every machine-checkable item is an automated test in the gate or in the platform tests; every gap found is fixed here with a test that was red before.
13. The checklist is complete: every screen times every rule has a row with the check and its state; manual items are explicit.

## Scope

Create:

- `docs/quality/accessibility-mobile.md`: the checklist (Design note 1), linked from `docs/quality/README.md` and the site navigation in `mkdocs.yml`.
- `apps/client/test/a11y/`: semantics and guideline tests per phone screen (Design note 2).
- `apps/client/ios/RunnerUITests/AccessibilityAuditTests.swift`: the iOS audit per screen through a test entry route (Design note 3).
- `apps/client/android/app/src/androidTest/.../AccessibilityChecksTest.kt`: the Android checks per screen (Design note 3).
- Fixes in `apps/client`, `packages/admin_ui` or `packages/design_system` for every gap, each with its test.

Change:

- `apps/client/platform-tests.json`: the iOS audit in `macos`, the Android checks in `android`.
- `packages/l10n` ARB files for any label added.
- Root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-10-1` to `D-M5-10-3`.

## Out of scope

- Terminal screens M8 and M9 (M5-13) and live views M10 (M6-16); the desktop (M4-32); the console (M6).
- Manual checks on physical devices with a person using a screen reader: they stay on the checklist for the release (M7).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-10-mobile-a11y`; one writer per worktree.
- Conventional Commits with the component as scope (for example `fix(client): ...`, `test(client): ...`, `docs(quality): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and use the fake server of M4-17. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID; simulators by UDID, emulators by serial.
- Never read or print secrets.
- The platform audits need macOS with Xcode and an iOS simulator, and an Android emulator; where missing, `make apps-platform` prints its SKIP line; the report names where they ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate; never suppress an audit finding without a row in the checklist that names the reason and the follow-up.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Checklist format** (01 8; PLAN 5; Ruling for docs/DECISIONS.md). A table per screen with one row per rule of 01 8 (contrast, screen reader role and label, live content, font size, focus order, targets, motion, color, language) and the columns "Check" (test name and file, or "manual on device: <steps>"), "State" (`automated`, `manual before release`, `not applicable` with reason) and "Platform" (iOS, Android, both). If the M4-32 checklist exists, use the same columns. The checklist names the assistive technology settings used (VoiceOver, TalkBack, larger text, reduce motion, bold text, high contrast).

2. **Flutter tests** (01 8; 15 20). Per screen: `meetsGuideline` for `textContrastGuideline`, `androidTapTargetGuideline`, `iOSTapTargetGuideline`, `labeledTapTargetGuideline`; a semantics snapshot (the list of labels and actions in reading order) compared with a checked-in expectation, so a lost label fails; text scaling 2 without overflow; reduced motion without running animations; the locale of the semantics tree; live regions announce at most once per `a11y.live_announce_min_interval` (fake clock); codes read character by character; the gesture of M5-07 has its button mode.

3. **Platform audits** (Ruling for docs/DECISIONS.md). iOS: a UI test target that launches the app with a test route per screen against the fake server and calls `performAccessibilityAudit()` with all audit types; known Flutter engine limitations that the audit reports (if any) are listed with a reason, never silently ignored. Android: an instrumented test with the Accessibility Test Framework on each screen (touch target size, contrast, labels, duplicate descriptions). Both run in `platform-tests.json`.

4. **Rulings for `docs/DECISIONS.md`** (`D-M5-10-<n>`): the checklist format and states (note 1); the semantics snapshot as a regression test (note 2); the platform audits and how a reported engine limitation is handled (note 3).

## Tests (write them first)

- Red first: run the new Flutter tests and the platform audits against the current app, keep the failures in the report, then fix each gap.
- `apps/client/test/a11y/<screen>_a11y_test.dart` per screen of the Goal (Design note 2).
- `AccessibilityAuditTests.swift` (iOS simulator) and `AccessibilityChecksTest.kt` (Android emulator) per screen (Design note 3).
- A test that every phone route of the router has a row in the checklist (parse the checklist table and the route list), so a new screen without a row fails.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS with Xcode and with an attached Android emulator: `make apps-platform`; otherwise the SKIP lines
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] The checklist with VoiceOver and TalkBack is complete for every phone screen named in the Goal; manual items are explicit.
- [ ] Every machine-checkable item is a test; every gap found is fixed with a test that was red before.
- [ ] The iOS audit and the Android checks run in the platform tests and pass.
- [ ] Checklist, site navigation, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public symbol; comments explain why.
- `docs/quality/accessibility-mobile.md` written in English and linked.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
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
4. The gaps found and fixed, the manual items left for the release, where the audits ran.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
