# M4-50: Free text selection in the history

Status: ready | Depends on: M4-49 | Milestone: M4

## Goal

After this ticket a person can select and copy any part of a message in the history. On the
desktop (macOS and Windows) the content of a message is selectable with the mouse: a click and a
drag across its paragraphs, lists, quotes, tables and code blocks, a double click for a word, a
triple click for a paragraph or a line of code, shift and a click to extend. Cmd+C (Ctrl+C) and
"Copy" at the top of the message menu copy the selection as plain text in the structure of the
message; a part of a code block copies exactly, without fences and without the line breaks of the
display wrap of M4-49. Selecting triggers no message action; links, the copy buttons of M4-48 and
the toggle and expand of M4-49 keep working. On the phone (iOS and Android) the long press keeps
the message menu, which gains "Select text": it opens the message in a sheet with the system
selection. Why: owner report of 2026-10-31 (macOS app); no text of the history could be selected,
so parts of messages, logs and code could not be copied; only "Copy code" (a whole block) and
"Copy text" (a whole message) worked.

## Read first

- `docs/PLAN.md`: section 10 rows M4-20, M4-48, M4-49, M4-50.
- Dependencies: the header names M4-49 (the readable code block with `DsCodeLines`, which already
  joins its lines under a selection); through it this ticket builds on M4-48 (the copy buttons
  and the long press menu on the phone) and M4-20 (the history and its message menu).
- `docs/spec/01-product.md`: 6.12, 6.14, 6.15 (new), 13 (R-PR-29).
- `docs/design/components.md` section 1 (message, long replies, code blocks).
- `docs/decisions.d/M4-48.md`, `docs/decisions.d/M4-49.md`.
- Code on the base branch: `packages/design_system/lib/src/components/markdown/ds_markdown.dart`,
  `packages/design_system/lib/src/components/markdown/code_lines.dart`,
  `packages/design_system/lib/src/components/ds_message.dart`,
  `apps/client/lib/thread/message_tile.dart`, `apps/client/lib/thread/message_menu.dart`,
  `apps/client/lib/thread/history_column.dart`, `apps/client/lib/thread/message_list.dart`.

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

6. Test first: write the failing test, run it and keep the red output for the report, then make
   it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them,
   and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no
   company, host or personal names, no paths under a home directory. Interface texts come from
   `packages/l10n`, never as literals in widgets.
9. No new configuration key and no new personal setting.
10. No GUI test on a maintainer's own machine; widget tests run headless.

Product requirements implemented here:

11. Desktop selection: drag, double click (word), triple click (paragraph or code line), shift and
    click, across the paragraphs, lists, quotes, tables and code blocks of one message. Across
    consecutive messages only if the virtualized history allows it without losing text; otherwise
    within one message, and the spec says so.
12. Copy: Cmd+C (Ctrl+C) and the context menu copy the selection as exact plain text: code parts
    without fence markers, the display wrap of M4-49 inserts no line break.
13. Selection triggers no message action; links stay clickable; the copy buttons of M4-48 and the
    wrap toggle and expand of M4-49 keep working.
14. Phone: the long press keeps the message menu; "Select text" in it opens a selectable view of
    the message (system selection handles, copy).
15. Find why selection is blocked today (gesture detectors, list item wrappers,
    `SelectionContainer.disabled`, custom painting) and fix the cause.

## Scope

- `packages/design_system`: `DsSelectionJoin` (the separators of a selection: blocks, list items
  and their markers, table cells and rows, code lines), its use in `DsMarkdown` and `DsCodeLines`
  under a selection, the language label of a code block out of the selection,
  `DsMessage.onSelectionChanged` (the selection area around the content on the desktop).
- `apps/client/lib`: `thread/message_tile.dart` (selection on the desktop, "Copy" in the menu,
  the menu on the release of a right click, "Select text" on the phone),
  `thread/message_menu.dart` (`MessageAction.copySelection`, `MessageAction.selectText`),
  `thread/select_text_view.dart` (the sheet), `thread/history_column.dart` (the handler leaves
  both to the tile).
- ARB keys in `packages/l10n` for every new text (`messageActionCopySelection`,
  `messageActionSelectText`, `messageSelectTextTitle`, `messageSelectTextClose`).

Fragments: `docs/decisions.d/M4-50.md` (`D-M4-50-<n>`), changelog fragment
`changelog.d/M4-50.md`; READMEs of `apps/client` and `packages/design_system`;
`docs/spec/01-product.md` 6.12 item 6, 6.15 and 13 (R-PR-29); `docs/design/components.md`
section 1.

## Out of scope

- A selection across several messages (see 01 6.15 item 1 and D-M4-50-2).
- The agent text inside the components of a card and the rows of the outbox.
- The web console, whose renderer of status updates is not the design system's.

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with the package or component as scope and a DCO sign-off on every commit
  (`git commit -s`).
- Never push to `main`, never tag; the only remote steps are the branch push and the pull request
  of `AGENTS.md`.
- Do not modify files outside the scope above, except the fragments and READMEs named in this
  brief.
- Tests bind no port and make no network call; time comes from the injected clock.
- Never lower a threshold, disable a gate, delete a test or raise `repo.max_file_kb`.
- This brief was written together with the change, in the manner of D-W-100 and D-W-114. Where the
  merged code names a package, type, function, file, provider or make target differently from this
  brief, follow the code and list each difference in the report; where it behaves differently from
  the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **The cause.** A `Text` is selectable only below a selection registrar; the history had none,
   so nothing could be selected. Each message on the desktop gets its own `SelectionArea` around
   its rendered content only.
2. **One message.** The history is a reversed, virtualized list; a selection across messages
   would lose the messages that leave the cache. The selection stays within one message.
3. **Separators.** Flutter joins the selected text of neighboring paragraphs with nothing. The
   renderer wraps each of its containers in a selection container that joins its parts with the
   separator of the structure; the code block joins its lines with a line break, so the display
   wrap never adds one.
4. **One menu.** The selection shows no menu of its own; the message menu opens on the release
   of a right click (after the selection selected the word on macOS) and offers "Copy" while
   text is selected. The menu takes the focus and ends the selection, so the tile copies the text
   selected when the menu opened.
5. **Phone.** A selection in the history would take the long press from the message menu; a
   sheet with the selection keeps both.
6. **Rulings for `docs/decisions.d/M4-50.md`**: the cause and the area, one message, the
   separators, the menu, the phone sheet, the parts out of scope and the platform coverage.

## Tests (write them first)

- `packages/design_system/test/components/markdown_selection_test.dart` (macOS and Windows):
  select all copies the structure of a message with paragraphs, lists, a quote, a code block with
  an empty line and a long line, a table, and no chrome; a drag from a paragraph into the middle
  of a code line; a partial selection inside a code block without fences; the display wrap of a
  long line inserts no line break; without wrap a partial selection is exact; a nested list;
  double and triple click; shift and click; a click on a link opens it and a drag over it
  selects; the copy button, the toggle and "Show all" under a selection; `DsMessage` with the
  callback makes only its content selectable, hears the selected text and shows no menu of its
  own, without the callback nothing is selectable, "Show more" stays out of the selection.
- `apps/client/test/thread/text_selection_test.dart`: on macOS and Windows a drag selects across
  the blocks of a message and the copy shortcut copies it exactly, a part of the code block; the
  right click menu has no "Copy" without a selection and "Copy" first with one, which copies the
  selection, while the drag opened no menu and wrote no outbox entry; a link opens and the copy
  button copies beside a selection; on iOS and Android the history has no selection, the long
  press menu keeps "Copy text" and has "Select text", which opens the sheet with the system
  selection, whose Select all and Copy copy the message's plain text, and the sheet closes; a
  deleted message offers no "Select text".

Commands that must pass, from the repository root:

- `cd packages/design_system && flutter analyze --fatal-infos && flutter test`
- `cd apps/client && flutter analyze --fatal-infos && flutter test`
- `cd packages/l10n && flutter test`
- `make codegen-check`, `make lint`, `make fmt-check`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] On macOS and Windows a drag, a double and a triple click and shift with a click select text
      of a message across its blocks, and the copy is the plain text of 01 6.15 item 3.
- [ ] Cmd+C (Ctrl+C) and "Copy" in the message menu copy the selection; selecting triggers no
      action; links, copy buttons, "Wrap lines" and "Show all" work.
- [ ] On iOS and Android the long press menu has "Select text", which opens a selectable view.
- [ ] Spec, READMEs, rulings and changelog fragment updated; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 6.15 or M4-50.
- Rulings in `docs/decisions.d/M4-50.md`; changelog fragment as in Scope.
- An independent code review over the diff, then the red team gate
  (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`).
- Land the change as `AGENTS.md` ("How work lands on `main`") describes.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits, changed files, test results (the red run before the change and the green run after).
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example the feel of a selection with a real trackpad on a Mac).
5. Follow-ups with reasons and differences to the names of this brief.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
