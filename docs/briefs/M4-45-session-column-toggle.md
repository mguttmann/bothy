# M4-45: Session column toggle on the desktop

Status: ready | Depends on: M4-42 | Milestone: M4

## Goal

After this ticket a person on the desktop can hide the right session column (agent, backend, costs, runs, pinned messages, files) and bring it back the same way the channel column of M4-42 works: "Hide session" in the header of a thread or main history, Cmd+Shift+I (Ctrl+Shift+I) and the item of the Thread menu hide it, so the history takes its width; "Show session", the same keys, "Pin" and a slim rail at the right edge of the window show it again. The choice is kept on this device like the other layout state and stays when another thread opens. Why: owner feedback on the Mac app (2026-10-29): with a forum post open, the session column read as not collapsible, because the header button only said "Column" and the choice was lost on every restart, although 01 D1 calls the column collapsible.

## Read first

- `docs/PLAN.md`: section 10 rows M4-20, M4-24, M4-42, M4-45.
- Dependencies: the header names M4-42; through it this ticket builds on M4-18 (columns), M4-20 (thread header and the session toggle kept while the app runs) and M4-24 (shortcuts) (all implied, so not in the header).
- `docs/spec/01-product.md`: 5.1 D1, D2, 7 (shortcut table), 13 (R-PR-19, R-PR-22).
- `docs/decisions.d/M4-42.md` (D-M4-42-2, D-M4-42-3: local view state and the "Column" button).
- Code on the base branch: `apps/client/lib/app/layout/` (`column_layout.dart`, `desktop_shell.dart`), `apps/client/lib/thread/` (`providers.dart` `SessionColumnShown`, `history_column.dart`, `thread_header.dart`), `apps/client/lib/keyboard/`, `packages/local_store/lib/src/tables/local_only.dart` (`shell_view_state`), `packages/local_store/lib/src/daos/forum_view_dao.dart`.

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
9. No new configuration key and no new personal setting key (01 11 is normative and the server rejects unknown keys).
10. No GUI test on a maintainer's own machine; widget tests run headless.

Product requirements implemented here:

11. A control in the thread header hides and shows the session column; its label says which column and what a tap does ("Hide session", "Show session" while hidden). Rule whether it replaces "Column" or sits next to it.
12. The existing shortcut Cmd+Shift+I (Ctrl+Shift+I) and the Thread menu item keep toggling the same state.
13. The state is kept per device like the channel column of M4-42 (`shell_view_state`, never synchronized) and survives a restart; opening another thread keeps it.
14. While the column is hidden the thread takes its width, and a small affordance at the right edge brings it back; where the window is too narrow for the column (`ui.desktop_min_width_four_columns`) there is nothing to bring back and no affordance.
15. The M4-42 controls and the column borders are named in the German interface (labels in `packages/l10n/lib/l10n/app_de.arb`).

## Scope

- `packages/local_store`: the state columns of `shell_view_state` with a new schema version and its migration step, the DAO methods to read, watch and set them; the stored schema and the generated schema helpers of `make-migrations`.
- `apps/client/lib/thread/`: the provider of the state replaces the in-memory toggle; the header control.
- `apps/client/lib/app/layout/`: the shell reads the state and binds the shortcut to it; `ColumnLayout` shows the rail.
- ARB keys in `packages/l10n` for every new text.

Fragments: `docs/decisions.d/M4-45.md` (`D-M4-45-<n>`), changelog fragment `changelog.d/M4-45.md`; README of `apps/client` and `packages/local_store`; `docs/spec/01-product.md` D1, D2 and the decision R-PR-22.

## Out of scope

- Phone layouts (the phone has no session column; its session opens as a sheet, 01 5.3).
- Server changes and new setting keys.
- A new glyph for the control or the rail.

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with the package or component as scope and a DCO sign-off on every commit (`git commit -s`).
- Never push to `main`, never tag; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the fragments and READMEs named in this brief.
- Tests bind no port and make no network call; time comes from the injected clock.
- Never lower a threshold, disable a gate, delete a test or raise `repo.max_file_kb`.
- This brief was written after M4-42 merged, in the manner of D-W-100 and D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Same as the channel column.** The session state lives next to `channel_column_hidden` in `shell_view_state`, is read through a stream provider and written by a toggle function, so both columns behave the same: kept on this device, kept when another thread opens, written at once.
2. **Two layouts, two defaults.** D1 shows the column beside a forum post or a main history alone; D2 collapses it beside a thread of a text channel. Each layout keeps its own stored choice, so hiding it in a forum does not show it beside a text channel's thread.
3. **Rail.** A slim column at the right edge, one button "Show session", only while the column is hidden and would have room; its width is taken from the content column before the other columns are fitted.
4. **Rulings for `docs/decisions.d/M4-45.md`**: whether the control replaces "Column"; where the state lives and its defaults; the rail and when it shows; the German labels.

## Tests (write them first)

Widget tests (`flutter test`, in-memory `local_store`, fake clock):

- `test/thread/session_column_toggle_test.dart`: "Hide session" gives the history the width and the rail brings the column back, the choice survives a restart on the same database; Cmd+Shift+I hides and shows; "Pin" shows; beside a thread of a text channel the column is collapsed until shown and that choice is kept apart; a post opened from the sidebar keeps the column hidden; a window too narrow for the column has no rail; the German interface names "Sitzung ausblenden", "Sitzung einblenden", "Kanal ausblenden", "Kanal einblenden" and the drag handles "Spaltenbreite ändern".
- `packages/local_store`: the DAO test of the new state (defaults, layouts apart, pin and channel column stay), the column list of `shell_view_state`, the migration test of the new version with a kept row.
- The thread goldens show the new label.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test`
- `cd packages/local_store && dart test`
- `make codegen-check`, `make lint`, `make fmt-check`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] The header control, the shortcut, the menu item and the rail hide and show the session column.
- [ ] The state is kept per device and per layout across restarts and when another thread opens.
- [ ] The German interface names the controls of M4-42 and M4-45 and the column borders.
- [ ] Spec, READMEs, rulings and changelog fragment updated; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 D1, D2 or R-PR-22.
- Rulings in `docs/decisions.d/M4-45.md`; changelog fragment as in Scope.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`).
- Land the change as `AGENTS.md` ("How work lands on `main`") describes.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits, changed files, test results (the red run before the change and the green run after).
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example a platform test that ran only in CI).
5. Follow-ups with reasons and differences to the names of this brief.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
