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
- `docs/spec/01-product.md` 5.3 M3 (with the addendum of 2026-10-31) and 13 (R-PR-31).
- `docs/spec/15-client-architecture.md` 11 (phone thread layout, addendum of 2026-10-31).
- `docs/design/components.md` 1, 2.3 and 9; `docs/design/spacing-radii-elevation-motion.md`.
- Code on the base branch: `packages/design_system/lib/src/components/ds_message.dart`,
  `ds_composer.dart`, `ds_live_box.dart`; `apps/client/lib/thread/history_column.dart`,
  `thread_header.dart`, `message_tile.dart`; `apps/client/lib/composer/thread_composer.dart`;
  `apps/client/lib/phone/phone_shell.dart`.

## Fixed requirements that apply

1. No MVP; strictly modular; doc comment on every public symbol; documentation in the repo is
   part of the change; quality gates block the merge (the owner's fixed build rules).
2. Test first: write the failing test, keep its red output for the report, then make it green.
3. No en dash and no em dash in any file.
4. White label: Alex, Sam, Robin and `example.com` in tests and goldens.
5. No GUI test on the owner's devices; widget tests and goldens run headless.
6. Tap targets stay at least `density.target-min` (48 on touch, which covers 44 pt and 48 dp);
   every control keeps its screen reader label.

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

## Report

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits, changed files, the red and the green test runs.
3. What is not proven (the real devices; no GUI test runs on the owner's devices).
