# M4-42: Thread layout on the desktop

Status: ready | Depends on: M4-40 | Milestone: M4

## Goal

After this ticket a person on the desktop can give an open thread the whole content width and size every column: when a thread or a forum post is open next to its channel column (the main history of a text channel, D2, or the pinned forum overview, D15), "Hide channel" in the thread header, Cmd+Shift+H (Ctrl+Shift+H) and the item of the Thread menu hide that column, and "Show channel" or the same keys show it again; the choice is kept on this device like the other layout state and stays when another thread opens from the sidebar. Every column border, also the one between the main history and the thread, can be dragged, with minimum widths, and the widths are stored as `layout.column_widths`. Why: owner feedback on the Mac app (2026-10-28, sidebar, channel history `# builds`, open thread "Watch the nightly build"): the channel history could neither be hidden nor resized, so the thread where the person writes stayed small, although 01 D1 already said that column borders can be dragged.

## Read first

- `docs/PLAN.md`: section 10 rows M4-18, M4-20, M4-24, M4-40, M4-42.
- Dependencies: the header names M4-40; through it this ticket builds on M4-18 (columns and `layout.column_widths`), M4-20 (thread view and header), M4-24 (shortcuts) and M4-25 (settings writer) (all implied, so not in the header).
- `docs/spec/01-product.md`: 5.1 D1, D2, D15, 7 (shortcut table), 11 (`layout.column_widths`), 13 (R-PR-17, R-PR-19).
- `docs/decisions.d/M4-18.md` (D-M4-18-5, column names and widths), `docs/decisions.d/M4-40.md` (D-M4-40-2, local view state).
- Code on the base branch: `apps/client/lib/app/layout/` (`column_layout.dart`, `column_widths.dart`, `desktop_shell.dart`), `apps/client/lib/thread/` (`history_area.dart`, `history_column.dart`, `thread_header.dart`), `apps/client/lib/forum/providers.dart` (`overviewPinnedProvider`), `apps/client/lib/keyboard/` (`intents.dart`, `bindings.dart`, `menu_bar.dart`, `shortcut_list.dart`), `packages/local_store/lib/src/tables/local_only.dart` (`shell_view_state`), `packages/local_store/lib/src/daos/forum_view_dao.dart`.

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
9. No new configuration key and no new personal setting key (01 11 is normative and the server rejects unknown keys); a new column name inside `layout.column_widths` is allowed, because 01 11 lets the client interpret the names.
10. No GUI test on a maintainer's own machine; widget tests run headless.

Product requirements implemented here:

11. A control in the thread header ("Hide channel", "Show channel" while hidden), a keyboard shortcut (Cmd+Shift+H, Ctrl+Shift+H) and an item of the Thread menu hide and show the channel column next to the open thread: the main history of a text channel, or the pinned overview of a forum next to an open post. The control shows only where such a column exists. The existing "Column" button keeps its meaning (session column) unless it fits better to fold the new action into it; rule which.
12. The state is kept per device like the other layout state of the shell (`shell_view_state`, never synchronized) and survives a restart.
13. Opening another thread, from the sidebar or elsewhere, while the channel column is hidden keeps it hidden.
14. Every column border between sidebar, pinned overview, channel history, thread and session column can be dragged; the widths are stored as `layout.column_widths`; no column becomes narrower than `DsLayout.columnMin` by a drag.

## Scope

- `packages/local_store`: the column `shell_view_state.channel_column_hidden` with a new schema version and its migration step, the DAO methods to read, watch and set it; the stored schema and the generated schema helpers of `make-migrations`.
- `apps/client/lib/app/layout/`: the column name `main_history` in `column_widths.dart`; the drag handle usable outside `ColumnLayout`; the shell hides the pinned overview next to an open post while the state says so, and binds the shortcut.
- `apps/client/lib/thread/`: the border between main history and thread as a drag handle with the stored width; the main history hidden while the state says so; the header control.
- `apps/client/lib/forum/providers.dart`: the provider of the state.
- `apps/client/lib/keyboard/`: the action, its binding, the menu item and the list text; the shortcut table of `docs/spec/01-product.md` 7 and of `apps/client/README.md` in the same change.
- ARB keys in `packages/l10n` for every new text.

Fragments: `docs/decisions.d/M4-42.md` (`D-M4-42-<n>`), changelog fragment `changelog.d/M4-42.md`; README of `apps/client` and `packages/local_store`; `docs/spec/01-product.md` D1, D2, 7, 11 and the decision R-PR-19.

## Out of scope

- Phone layouts (the phone has no columns, M5-15).
- Server changes and new setting keys.
- A new glyph for the control.

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with the package or component as scope and a DCO sign-off on every commit (`git commit -s`).
- Never push to `main`, never tag; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the fragments and READMEs named in this brief.
- Tests bind no port and make no network call; time comes from the injected clock.
- Never lower a threshold, disable a gate, delete a test or raise `repo.max_file_kb`.
- Where the merged code names a type, file, provider or make target differently from this brief, follow the code and list each difference in the report.

## Design notes

1. **One state for both kinds.** The main history of a text channel and the pinned overview of a forum are both "the channel next to the thread"; one device value hides either, so the keys and the button behave the same everywhere. Hiding the overview does not unpin it.
2. **The thread keeps its place.** The thread stays the same element in the tree when the main history goes or comes, so its scroll position and its input survive.
3. **Widths.** The main history next to a thread gets the column name `main_history`; the thread takes the rest. Without a stored width the two share the room, as before this ticket. A drag is one write at its end, and the dragged width stays until the stored setting comes back (D-M4-18-5).
4. **Rulings for `docs/decisions.d/M4-42.md`**: the meaning of "channel column" for forums; where the state lives; whether the "Column" button takes the action; the width name and its default.

## Tests (write them first)

Widget tests (`flutter test`, in-memory `local_store`, fake clock):

- `test/thread/channel_column_test.dart`: "Hide channel" leaves the thread alone and wider, "Show channel" brings the main history back, and the choice survives a restart on the same database; Cmd+Shift+H hides and shows; a thread opened from the sidebar keeps the channel hidden; the border between main history and thread can be dragged, the width is stored as `main_history` and both sides keep `DsLayout.columnMin`; in a forum the control hides the pinned overview and keeps the pin; without the pin and on the main history alone there is no control.
- `test/app/column_widths_test.dart`: `main_history` is read, written and has no default.
- `packages/local_store`: the DAO test of the new state (the pin stays), the column list of `shell_view_state`, the migration test of the new version with a kept row.
- `test/keyboard/bindings_spec_test.dart` and `readme_table_test.dart` compare the shortcut tables with `bindings.dart`.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test`
- `cd packages/local_store && dart test`
- `make codegen-check`, `make lint`, `make fmt-check`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] The header control, the shortcut and the menu item hide and show the channel column next to an open thread or forum post.
- [ ] The state is kept per device across restarts and when another thread opens.
- [ ] Every column border can be dragged; widths are stored in `layout.column_widths` with minimum widths.
- [ ] Spec, READMEs, rulings and changelog fragment updated; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 D1, D2 or R-PR-19.
- Rulings in `docs/decisions.d/M4-42.md`; changelog fragment as in Scope.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`).
- Land the change as `AGENTS.md` ("How work lands on `main`") describes.

## Report

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits, changed files, test results (the red run before the change and the green run after).
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example a platform test that ran only in CI).
5. Follow-ups that could not be done in this scope, each with its reason, and every difference to the names of this brief.
