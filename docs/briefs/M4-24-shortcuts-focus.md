# M4-24: Keyboard shortcuts and focus order

Status: ready | Depends on: M4-21, M4-23 | Milestone: M4

## Goal

After this ticket the desktop app is fully usable by keyboard: every shortcut of the first table of 01 7 is bound on macOS and Windows with the platform modifier (quick switcher, search, search in thread, new post, focus the input, next and previous channel and unread channel, F6 and Shift+F6 between the columns, mark read and unread, done and reopen, stop the run, live box toggle, session column toggle, edit last message, send and new line, Running now, Mentions, Settings, list of shortcuts), the focus order follows the columns (sidebar, list, history, input, session) with a visible focus ring everywhere, the list of shortcuts opens with Cmd+/ or Ctrl+/, and no shortcut can trigger an approval or any other component action (R-PR-04). A keyboard walkthrough runs as an integration test on both platforms. Why: 01 8 requires a keyboard pass as a test, and an approval reachable by a shortcut would let a typing slip approve an agent action (R-PR-04).

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-20, M4-21, M4-23, M4-24, M4-32, M4-36.
- `docs/spec/01-product.md`: 5.1 D1 and D2 (columns), 6.5, 7 "Keyboard shortcuts" (the first table and the rules above it; the terminal table is M4-36), 8 (row "Focus": order per column, F6, focus ring; row "Screen reader"), 13 (R-PR-04, R-PR-10).
- `docs/design/components.md` (focus ring tokens `color.focus.ring`, `color.focus.ring-on-sidebar`; keyboard rules of 6, 7, 8 and 9).
- `docs/spec/15-client-architecture.md` 23.4 (Windows app shortcuts only with Ctrl+Shift in the terminal: why the binding layer must know a terminal focus scope, used by M4-36).
- Flutter documentation of `Shortcuts`, `Actions`, `Intent`, `FocusTraversalGroup`, `SingleActivator` (platform modifiers) and `PlatformMenuBar` (macOS menu).
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-116 (this ticket depends on M4-21 for the run and component actions); section 10.
- Briefs: `docs/briefs/M4-18-desktop-shell.md` (columns, router), `docs/briefs/M4-19-sidebar-post-list.md` (`SidebarNavigation`), `docs/briefs/M4-20-thread-view.md` (message actions, read state, composer), `docs/briefs/M4-21-agent-interface.md` (Stop, live box toggle, components), `docs/briefs/M4-23-search-switcher.md` (views and their actions).

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
9. No new configuration key and no personal setting for custom key bindings (01 11 has none).
10. Specification first: if a shortcut of 01 7 collides with a system assignment on the pinned macOS or Windows versions, record the collision and the chosen behaviour as a ruling and change the table of 01 7 in the same change, never silently.

Product requirements implemented here:

11. Every shortcut of the first table of 01 7 works on macOS with Cmd or Option and on Windows with Ctrl or Alt, as listed; none uses a system assignment (Cmd+H, Cmd+M, Cmd+Q, Alt+F4, the Windows key).
12. No shortcut triggers an approval or another component action; components react only to Enter or Space on the focused button (R-PR-04).
13. F6 and Shift+F6 move between the columns sidebar, list, history and session; the focus order within a column follows the reading direction; the focus ring is always visible (01 8).
14. Enter sends and Shift+Enter makes a new line, reversed by `composer.enter_sends`; arrow up in the empty field edits the last own message (01 7).
15. Cmd+/ or Ctrl+/ shows the list of shortcuts in the language of the app.

## Scope

Create in `apps/client/lib/`:

- `keyboard/intents.dart` (one `Intent` per action of the table), `keyboard/bindings.dart` (the table as data: action, macOS activator, Windows activator; Design note 1), `keyboard/app_shortcuts.dart` (the `Shortcuts` and `Actions` at the shell level, scoped so that text fields and later the terminal keep their keys, Design note 2), `keyboard/column_focus.dart` (F6 order, Design note 3), `keyboard/shortcut_list.dart` (the overlay of Cmd+/), `keyboard/menu_bar.dart` (macOS `PlatformMenuBar` with the same actions where 01 7 lists them).
- Tests under `test/keyboard/`, golden of the shortcut list under `test/goldens/keyboard/`, integration test under `integration_test/` (Tests).
- ARB keys for the action names of the shortcut list in `packages/l10n`.

Change:

- The views of M4-19, M4-20, M4-21 and M4-23 only to expose their actions as `Action`s and to put their columns into `FocusTraversalGroup`s; no behaviour change.
- `apps/client/README.md` (shortcut table generated from `bindings.dart` or kept equal to it by a test).
- `docs/spec/01-product.md` 7 only for a recorded collision (Specification first).
- `docs/DECISIONS.md` section 10: rows `D-M4-24-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Shortcuts in the terminal (M4-36). "Open hosts and terminal" (Cmd+Shift+T, Ctrl+Shift+T) is bound here as a disabled action until M4-36 enables it with its route.
- Custom key bindings (not planned).
- Screen reader checks by hand (M4-32).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-24-shortcuts-focus`; one writer per worktree.
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

1. **Table as data.** `bindings.dart` holds every row of the first table of 01 7 with its action and the activators per platform (`SingleActivator(LogicalKeyboardKey.keyK, meta: true)` on macOS, `control: true` on Windows; Option and Alt for the channel navigation; F6 without modifier). A test compares the table with 01 7 row by row (read the Markdown table between its heading and the terminal table, walking up to the repository root), so a change of the spec without code fails. Option+Click and Alt+Click for "Mark as unread" are pointer gestures on a message, bound in the message tile through the same action.

2. **Scopes.** App shortcuts sit above the columns; a focused text field keeps Enter, Shift+Enter, arrow up and Esc according to the composer rules (the composer handles them; app shortcuts with modifiers still work). An `Action` for an intent is enabled only where its target exists (Stop only with an active run the server allows to stop, Done only in a thread); a disabled action does nothing and does not consume the key. Components register no intent at all; a test enumerates every binding and asserts that none maps to a component action (R-PR-04). The layer exposes a `KeyboardScope.passThrough` region that M4-36 uses for the terminal, where only platform-modifier shortcuts reach the app (R-PR-10).

3. **Focus order.** Each column is a `FocusTraversalGroup` with `ReadingOrderTraversalPolicy`; F6 and Shift+F6 move to the first focusable item of the next or previous visible column (collapsed columns are skipped) in the order of 01 7: sidebar, list, history, session (the input belongs to the history column; within it the focus order of 01 8 goes on to the input); Esc in the history marks the thread read, and "Esc then Tab" or Cmd+L (Ctrl+L) focuses the input, exactly as 01 7 lists. The focus ring uses `color.focus.ring` (`color.focus.ring-on-sidebar` in the sidebar) and is visible for keyboard focus at all times.

4. **Rulings for docs/DECISIONS.md** (`D-M4-24-<n>`): the table as data with the spec test (note 1); scopes and the pass-through region (note 2); the column order and Esc behaviour (note 3); any collision handled (Specification first).

## Tests (write them first)

Widget tests:

- `test/keyboard/bindings_spec_test.dart`: the table equals 01 7 for both platforms; no activator uses a system assignment of the rules above the table.
- `test/keyboard/no_approval_shortcut_test.dart` (R-PR-04): every binding is dispatched with a focused approval card in view and no interaction is created; Enter and Space on the focused button create exactly one.
- `test/keyboard/scopes_test.dart`: a text field keeps Enter, Shift+Enter and arrow up; a disabled action does not consume its key; the pass-through region lets plain Ctrl and Alt keys through.
- `test/keyboard/column_focus_test.dart`: F6 and Shift+F6 cycle through the visible columns and skip collapsed ones; the focus ring is painted.
- `test/goldens/keyboard/shortcut_list_*`: the list on macOS and Windows, `de` and `en`, light and dark.

Integration test (`integration_test/keyboard_walkthrough_test.dart`, against `FakeServer`, on macOS and Windows): from the sidebar, move to a channel with Alt or Option plus arrows, open a thread, F6 to the history, mark read with Esc, focus the input, write and send with Enter, edit with arrow up, open the quick switcher and jump, open search, Running now, Mentions and Settings, stop a scripted run, toggle the live box and the session column, open the shortcut list; every step asserted by the visible state.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] The integration test of a keyboard walkthrough is green on macOS and Windows.
- [ ] No shortcut triggers an approval (R-PR-04), proven by the enumeration test.
- [ ] Every shortcut of the first table of 01 7 is bound as listed, held equal to the spec by a test; F6 moves between the columns; the focus ring is always visible.
- [ ] The pass-through region for the terminal exists for M4-36; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01 7 or 8.
- `apps/client/README.md` (English) as in Scope.
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
4. Where the walkthrough ran, any collision with a system shortcut and how it was resolved, and the list of actions that are disabled outside their context.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
