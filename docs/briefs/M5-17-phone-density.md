# M5-17: Phone thread density

Status: ready | Depends on: M5-15, M5-16 | Milestone: M5

## Goal

After this ticket a person reading a thread on iOS or Android sees message text from the edge of
the screen, types into an input whose text field is the whole input, and keeps most of the height
for the history. Why: the owner's screenshots of 2026-10-31 (iPhone, a thread in a development
forum while Claude Code ran) showed three problems. (1) Message text started about a quarter of
the screen width from the edge: an avatar column of `size.avatar-md` plus `space.12` stood beside
every message, also beside continuation messages. (2) The input's editable text area was a small
box in the middle of the input: the templates button ("+") and the microphone sat outside the
input, attach and Send inside it, and the text field had padding of its own, so the placeholder
"Follow-up to Claude Code" and the caret sat in the middle and only that inner part was the text
field. (3) The back link, a two-line header (title and the operator line "Operator: ...") and the
two-line live box ("Running 0:56" over "compacting") took a large part of the height.

## Read first

- `docs/PLAN.md`: section 11 row M5-17.
- `docs/spec/01-product.md` 5.3 M3 (with the addendum of 2026-10-31) and 13 (R-PR-32).
- `docs/spec/15-client-architecture.md` 11 (phone thread layout, addendum of 2026-10-31).
- `docs/design/components.md` 1, 2.3 and 9; `docs/design/spacing-radii-elevation-motion.md`.
- Code on the base branch: `packages/design_system/lib/src/components/ds_message.dart`,
  `ds_composer.dart`, `ds_live_box.dart`; `apps/client/lib/thread/history_column.dart`,
  `thread_header.dart`, `message_tile.dart`; `apps/client/lib/composer/thread_composer.dart`;
  `apps/client/lib/phone/phone_shell.dart`.

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

6. Test first: write the failing test, keep its red output for the report, then make it green.
7. No en dash and no em dash in any file.
8. White label: Alex, Sam, Robin and `example.com` in tests and goldens.
9. No GUI test on the owner's devices; widget tests and goldens run headless.
10. Tap targets stay at least `density.target-min` (48 on touch, which covers 44 pt and 48 dp); every control keeps its screen reader label.

## Scope

- Density tokens (`design/tokens/density.*.tokens.json`, generated with
  `uv run tools/design-tokens/tokens.py build`): `density.message-gutter` (44 on desktop, 0 on
  touch), `density.composer-pad-x`, `density.composer-inset-x`, `density.composer-inset-bottom`.
- `DsMessage`: the avatar in the gutter where it fits, else a small avatar in the header line and
  the content at the full width; `DsAvatar.size`.
- `DsComposer`: `trailing` inside the field before Send; the text field fills the field between
  the buttons with `density.composer-pad-x` inside it, starts at the left, a single line centered
  on the buttons.
- `DsLiveBox.mobile`: one line of state, run time and status line, Stop at the end.
- App: the thread page frame hands the back action to the header (`PhoneBackScope`,
  `PhoneHeaderBack`, `PhoneHeaderBackFrame`) and keeps its bar until a header claims it; the
  compact `ThreadHeader` in one line with the operator as the first line of the overflow menu;
  the header collapses while scrolling into older messages (`HeaderCollapse`,
  `CollapsingHeader`); the phone composer with the microphone as `trailing` and the templates in
  the attachment menu; the rows beside the content (files, thread starter lines, run lines, files
  of a waiting message) follow `density.message-gutter`.

Fragments: `docs/decisions.d/M5-17.md` (`D-M5-17-<n>`), `changelog.d/M5-17.md`.

## Out of scope

- The desktop layout: its density values keep the avatar column and the composer as before.
- A swap of microphone and Send by the text state (one button that changes): it changes the
  voice flows of M5-07 and was not asked for.
- Collapsing the live box while scrolling: it is one line and carries Stop.

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with the package or component as scope and a DCO sign-off on every commit (`git commit -s`).
- Never push to `main`, never tag; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the fragments and READMEs named in this brief.
- Tests bind no port and make no network call.
- Never lower a threshold, disable a gate or delete a test.
- This brief was written together with the change, in the manner of D-W-100 and D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Gutter as a token.** The touch density is used exactly on iPhone and Android (15 2), so the
   phone layout of a message is the value 0 of `density.message-gutter`, not a flag.
2. **The text field is the target.** The padding moves from around the `TextField` into its
   `contentPadding`; the decorator keeps a single line at the top of a field taller than the line,
   so the vertical padding centers it: `max(space.8, (target-min - line height) / 2)`.
3. **Back in the header.** A thread page has no back bar while its header shows the back button;
   a header claims the action after its first frame, so a thread that never builds a header (it is
   loading, or not found) keeps the bar.
4. **Collapse.** `UserScrollNotification` of the reversed history: `reverse` (older messages)
   hides, `forward` and the newest message in view show; never under
   `MediaQuery.accessibleNavigation`. The animation uses `motion.duration-move`.
5. **One line.** State, run time and status line are one `Text.rich` cut at its end, so the state
   always shows first, also at a text scale of 200 percent; the sheet shows the rest.

## Tests (write them first)

- `packages/design_system/test/components/phone_density_test.dart`: content start and avatar per
  density; the text field between the buttons with `trailing` inside the field; the placeholder at
  its left and centered; a tap on its left padding and its right end focuses it; growth to
  `maxLines` and then scrolling; button targets; the one-line live box and no overflow at text
  scale 2.
- `apps/client/test/phone/thread_density_test.dart`: the history on the phone at the full width,
  the one-line header with back button and the operator in the menu, collapse and return while
  scrolling, no collapse with a screen reader, the one-line live box opening the sheet, the
  composer with the microphone inside the field, templates in the attachment menu, focus from a
  tap on the field's edge, growth and scrolling, and the page frame's back bar with and without a
  header.
- Goldens: `phone_m3_dense` and `android_m3_dense` (light and dark, text scale 1 and 2, de and en)
  through `phone_screens_golden_test.dart` and `android_goldens_test.dart`; the existing phone,
  thread and component goldens of the density `touch` regenerated.

Commands that must pass, from the repository root: `cd packages/design_system && flutter analyze
--fatal-infos && flutter test`; `cd apps/client && flutter analyze --fatal-infos && flutter test`;
`make tokens`, `make lint`, `make fmt-check`, `make docs`; `make check` in the pull request CI.

## Acceptance criteria

- [ ] On the phone message content starts at the row padding; continuation messages align with it.
- [ ] The input's text field fills the input between attach and the microphone, starts at the left,
      grows and then scrolls; a tap anywhere on it focuses it.
- [ ] The thread header is one line and collapses while older messages are read; the live box is
      one line with Stop and opens the sheet on a tap.
- [ ] Targets and labels kept; spec 01 and 15, components, tokens, PLAN, rulings and changelog
      updated.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 5.3 M3 or M5-17.
- Rulings in `docs/decisions.d/M5-17.md`; changelog fragment as in Scope.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`).
- Land the change as `AGENTS.md` ("How work lands on `main`") describes.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits, changed files, the red and the green test runs.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (the real devices; no GUI test runs on the owner's devices).
5. Follow-ups with reasons and differences to the names of this brief.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
