# M4-48: Copy single blocks of a message

Status: ready | Depends on: M5-02 | Milestone: M4

## Goal

After this ticket a person copies exactly one block of a message on every platform. On the desktop (macOS and Windows, the same widget) every fenced code block and every quote block has a small copy button at its top right that copies exactly that block. On the phone (iOS and Android) a tap on a code block or a quote block copies that block, with a short "Copied" and a haptic; a long press on the message opens the message menu, where "Copy text" copies the whole message. A code block copies its raw content byte for byte, a quote its text without the `>` markers, as plain text and, where the platform clipboard holds rich text, also as HTML; a nested block copies only itself. Why: owner request of 2026-10-29; in Discord a tap on a code block on the phone never copied it and "Copy text" always copied the whole message.

## Read first

- `docs/PLAN.md`: section 10 rows M4-07, M4-20, M4-48; section 11 row M5-02.
- Dependencies: the header names M5-02 (the phone form factor and density `touch`); through it this ticket builds on M4-07 (the Markdown renderer with "Copy code") and M4-20 (message actions and the menu) (both implied, so not in the header).
- `docs/spec/01-product.md`: 3.3, 6.4, 6.12, 11 (density), 13 (R-PR-25).
- `docs/design/components.md` section 1 (message, long replies, code blocks).
- `docs/decisions.d/M4-07.md` (D-M4-07-2, the renderer) and `docs/decisions.d/M4-20.md` (D-M4-20-4, the actions).
- Code on the base branch: `packages/design_system/lib/src/components/markdown/ds_markdown.dart` (`DsMarkdownCodeBlock`), `apps/client/lib/thread/` (`message_tile.dart`, `message_menu.dart`, `history_column.dart`), `apps/client/lib/app/app.dart`, the runners in `apps/client/macos`, `ios`, `android` and `windows`, `apps/client/platform-tests.json`.

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
10. No GUI test on a maintainer's own machine; widget tests run headless, the platform test runs in the CI jobs of macOS and Windows.

Product requirements implemented here:

11. Desktop (macOS and Windows): a copy button at the top right of every code block and every quote block, copying exactly that block, reachable by keyboard and screen reader, with "Copied".
12. Phone (iOS and Android): a tap on a code block or a quote block copies that block with "Copied" and a haptic; a long press on the message opens the message menu, where "Copy text" copies the whole message.
13. Code block fidelity: raw content byte for byte (indentation, tabs, trailing spaces, Unicode, long lines), no language tag, no fence markers, no soft wraps or rendering artefacts; the trailing line break rule of 01 6.12 item 3.
14. Quote block: text without the `>` markers, with its line breaks and inline Markdown as plain text; also as rich text where the platform clipboard supports it, so a paste into a rich editor keeps the formatting.
15. Nested blocks copy only the tapped block.
16. Selecting part of a message with the system selection keeps working where a view offers it.

## Scope

- `packages/design_system`: `DsMarkdownCodeBlock`, the new `DsMarkdownQuoteBlock`, the copy model (`dsCodeBlockText`, `dsQuoteCopy`, `DsCopyData`) and the clipboard port (`DsClipboard`, `DsPlainClipboard`, `DsClipboardScope`).
- `apps/client/lib`: the rich clipboard (`app/rich_clipboard.dart`), its scope in `app/app.dart`, the long press menu of `thread/message_tile.dart` set from `thread/history_column.dart`.
- The runners of `apps/client`: the channel `bothy/clipboard` in `macos/Runner/AppDelegate.swift` and `MainFlutterWindow.swift`, `ios/Runner/AppDelegate.swift`, `android/.../RichClipboardChannel.kt` and `MainActivity.kt`, `windows/runner/rich_clipboard.*`, `flutter_window.*` and `CMakeLists.txt`; `apps/client/platform-tests.json` and the integration test.
- ARB keys in `packages/l10n` for every new text.

Fragments: `docs/decisions.d/M4-48.md` (`D-M4-48-<n>`), changelog fragment `changelog.d/M4-48.md`; README of `apps/client` and `packages/design_system`; `docs/spec/01-product.md` 3.3, 6.12 and 13 (R-PR-25); `docs/design/components.md` section 1.

## Out of scope

- A system text selection over the message history (no view of the history offers one today; D-M4-48-6).
- Copy buttons on other blocks (lists, tables, paragraphs) and a new glyph for the button.
- The web console, whose renderer of status updates keeps its plain text copy.

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with the package or component as scope and a DCO sign-off on every commit (`git commit -s`).
- Never push to `main`, never tag; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the fragments and READMEs named in this brief.
- Tests bind no port and make no network call; time comes from the injected clock.
- Never lower a threshold, disable a gate, delete a test or raise `repo.max_file_kb`.
- This brief was written together with the change, in the manner of D-W-100 and D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **One copy model.** The content of a copy comes from the parse tree of `DsMarkdown.parse`, never from the rendered widgets, so soft wraps and the horizontal scroll cannot leak into it; the code text the block shows and the text it copies are the same string.
2. **Desktop or phone by density.** The renderer decides by the ambient density (`touch` on the phone), like the sidebar rows, so a test can pump either layout on any host.
3. **Clipboard port.** The design system writes through `DsClipboard`; the app provides the rich one in a `DsClipboardScope` at the root. Without a scope the plain text goes to Flutter's clipboard.
4. **Rich text per runner.** Flutter's clipboard holds plain text only; each runner answers the channel `bothy/clipboard`. Dart builds the Windows document "HTML Format" with its byte offsets, so the offsets are unit tested; a missing or failing channel falls back to plain text.
5. **Nested targets.** On the phone the deepest tap target wins (a link, a nested block), a long press and drags go to the message and the scroll view.
6. **Rulings for `docs/decisions.d/M4-48.md`**: button or tap and the density rule; the trailing line break rule; the plain and HTML form of a quote; the channel per runner; the long press menu on the phone; the selection; the confirmation and the platform coverage.

## Tests (write them first)

- `packages/design_system/test/components/markdown_copy_test.dart`: the code copy keeps tabs, indentation, trailing spaces and Unicode (umlaut, emoji, Hebrew, combining marks, zero width space), a 5000 character line stays one line, no language tag and no fence, the trailing line break rule, CR LF, an indented fence, a block inside a quote, an unclosed fence; the quote copy as plain text (line breaks, inline Markdown, lists, tasks, code, tables, nested quote) and as escaped HTML without images; under macOS and Windows: the button at the top right of each block, each button copies exactly its block (nested ones too) with "Copied" and no haptic, a click on the text copies nothing, the keyboard reaches the button, a drag selection copies across the blocks without the button labels; under iOS and Android: a tap copies with "Copied" and a haptic, nested blocks copy only themselves, a link opens and copies nothing, a long press reaches the message and copies nothing, a drag scrolls and copies nothing, the screen reader action copies; without a scope the plain text reaches the system clipboard.
- `apps/client/test/app/rich_clipboard_test.dart`: the channel gets text and HTML, on Windows also the CF_HTML document; a copy without HTML goes to the system clipboard; a missing or failing channel falls back to plain text; the CF_HTML byte offsets with multi-byte characters.
- `apps/client/test/thread/copy_blocks_test.dart`: in the history on macOS and Windows the buttons copy their blocks and the menu's "Copy text" the whole message; on iOS and Android a tap copies a block with a haptic and a long press opens the menu whose "Copy text" copies the whole message; on the desktop a long press opens no menu.
- `apps/client/test/app/app_test.dart`: the app provides the rich clipboard.
- `apps/client/integration_test/copy_blocks_test.dart` (CI jobs `apps-macos` and `apps-windows`): the real clipboard holds the code byte for byte and the quote as text and HTML.

Commands that must pass, from the repository root:

- `cd packages/design_system && flutter analyze --fatal-infos && flutter test`
- `cd apps/client && flutter analyze --fatal-infos && flutter test`
- `make codegen-check`, `make lint`, `make fmt-check`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] Desktop: every code and quote block has a copy button at its top right that copies exactly that block.
- [ ] Phone: a tap copies a block with "Copied" and a haptic; a long press opens the message menu with "Copy text" for the whole message.
- [ ] The code copy is byte exact by the rule of 01 6.12 item 3; the quote copy has no markers and carries HTML where the clipboard holds it.
- [ ] Spec, READMEs, rulings and changelog fragment updated; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 6.12 or M4-48.
- Rulings in `docs/decisions.d/M4-48.md`; changelog fragment as in Scope.
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
