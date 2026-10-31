# M4-49: Readable code blocks

Status: ready | Depends on: M4-48 | Milestone: M4

## Goal

After this ticket a person can read every code block of a message before copying it, on the
desktop (macOS and Windows) and on the phone (iOS and Android). A fenced block wraps its long
lines by default, so its whole content is visible; a small toggle "Wrap lines" in the block
header switches to one line per line with a horizontal scroll that works (a visible scrollbar on
the desktop, shift and the mouse wheel, the trackpad, a drag on the phone), and the choice is kept
per device. A very long block shows collapsed with a fade and "Show all", which expands it in
place, and "Show less" collapses it again. A wrapped indented line keeps its indentation (hanging
indent). Wrapping is display only: the copy of M4-48 stays the exact original text. Why: owner
decision of 2026-10-31, which overrides the rule of 01 6.4 that code blocks scroll horizontally
instead of wrapping. An agent sent a long sentence as a fenced block "to copy"; the macOS app
showed it as one cut-off line that could not be scrolled sideways (no visible scrollbar, the
trackpad did not move it) and could not be expanded.

## Read first

- `docs/PLAN.md`: section 10 rows M4-07, M4-48, M4-49.
- Dependencies: the header names M4-48 (the copy of single blocks, `DsMarkdownCodeBlock` with the
  copy button and the tap to copy); through it this ticket builds on M4-07 (the Markdown
  renderer) and M5-02 (the phone density `touch`).
- `docs/spec/01-product.md`: 6.4, 6.12, 6.14 (new), 13 (R-PR-28), 14
  (`ui.code_block_collapse_lines`).
- `docs/design/components.md` section 1 (message, long replies, code blocks).
- `docs/decisions.d/M4-48.md` and `docs/decisions.d/M4-45.md` (the local view state of the shell).
- Code on the base branch: `packages/design_system/lib/src/components/markdown/ds_markdown.dart`
  (`DsMarkdownCodeBlock`), `packages/design_system/lib/src/components/ds_message.dart`
  (`DsCollapsible`), `packages/local_store/lib/src/tables/local_only.dart` (`shell_view_state`),
  `packages/local_store/lib/src/daos/forum_view_dao.dart`, `apps/client/lib/app/app.dart`.

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
9. One new configuration key, `ui.code_block_collapse_lines` (place App, default 20, 01 14); no
   new personal setting key: the wrap choice is local view state of the device.
10. No GUI test on a maintainer's own machine; widget tests and goldens run headless.

Product requirements implemented here:

11. Wrap by default: a code block wraps its long lines so its whole content is visible.
12. Toggle: "Wrap lines" in the block header switches the block to no wrap with a working
    horizontal scroll: a visible scrollbar on the desktop that can be dragged, shift and the
    mouse wheel, a sideways trackpad gesture, a drag on the phone. The choice is kept per device
    and applies to every block of the device.
13. Long blocks: above `ui.code_block_collapse_lines` lines (rendered height) the block shows
    collapsed with a fade and "Show all", which expands it in place; "Show less" collapses it.
14. Indentation: a wrapped indented line keeps its indentation on every wrapped line (hanging
    indent).
15. Display only: the copy stays the raw content of 01 6.12 item 3; the M4-48 copy fidelity tests
    stay green.

## Scope

- `packages/design_system`: `DsMarkdownCodeBlock` (wrap, toggle, scrollbar, collapse),
  `DsCodeBlockScope` (the device choice, its callback and the limit), `DsCodeLines` (wrapped
  lines with a hanging indent and a selection that keeps line breaks), `DsClipToHeight` (shared
  with `DsCollapsible`), the `toggled` state of `DsTappable`.
- `packages/local_store`: the column `shell_view_state.code_blocks_unwrapped` (schema version 11)
  and its DAO methods.
- `packages/domain`: the generated key `ui.code_block_collapse_lines`.
- `apps/client/lib`: `app/code_block_preference.dart` (`CodeBlockPreferenceScope`, the provider
  of the device choice) and its place at the root in `app/app.dart`.
- ARB keys in `packages/l10n` for every new text (`markdownCodeWrap`, `markdownCodeShowAll`,
  `markdownCodeShowLess`).

Fragments: `docs/decisions.d/M4-49.md` (`D-M4-49-<n>`), changelog fragment
`changelog.d/M4-49.md`; READMEs of `apps/client`, `packages/design_system` and
`packages/local_store`; `docs/spec/01-product.md` 6.4, 6.14, 13 (R-PR-28) and 14;
`docs/operations/configuration.md` (generated); `docs/design/components.md` section 1.

## Out of scope

- Line numbers in code blocks (no code block has them today).
- Syntax highlighting, a new glyph for the toggle, a wrap choice per block that survives a
  restart.
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

1. **One choice per device.** The design system stores nothing: `DsCodeBlockScope` hands down
   whether lines wrap, a callback for a new choice and the limit. The app provides it at its root
   from the local view state (`shell_view_state`, never synchronized, like the columns of M4-42
   and M4-45), so every block follows one choice and it survives a restart.
2. **Own scrollbar.** The default scroll behavior draws no scrollbar on a horizontal scroll view;
   the block brings a `Scrollbar` on its own controller, always visible on the desktop. Shift and
   the wheel and the trackpad are handled by the scroll view itself.
3. **Hanging indent without a paragraph feature.** Flutter's paragraph has no hanging indent, so a
   wrapped indented line is its indentation beside a box with the rest; a selection container
   joins the lines with a line break, so a system selection keeps the text.
4. **Height, not line count.** The collapse measures the rendered height against the limit times
   the line height of `type.code`, so one long wrapped line collapses too.
5. **Rulings for `docs/decisions.d/M4-49.md`**: wrap by default and the device choice; the
   scrollbar and the input devices; the collapse limit as a configuration key; the hanging indent
   and its bound; the selection; the platform coverage.

## Tests (write them first)

- `packages/design_system/test/components/markdown_code_block_test.dart`: a long single line
  wraps on the desktop and the phone; the copy of a wrapped block and of a block without wrap is
  exact; the hanging indent and its bound; blank lines; a system selection keeps line breaks and
  indentation; the toggle in the header beside the copy button with its toggled state, a visible
  scrollbar after it, the device choice for every block, the block's own choice without a
  callback, the keyboard order; on the phone the toggle copies nothing and a tap still copies;
  shift and the wheel, the trackpad, the scrollbar thumb and a drag scroll sideways; a block
  above the limit shows collapsed with a fade and expands and collapses in place on the desktop
  and the phone, the limit comes from the scope, one long wrapped line collapses, a collapsed
  block copies all its lines, the screen reader state of the control, the German labels.
- `packages/design_system/test/goldens/components/code_block_golden_test.dart`: a long
  single-line block wrapped and without wrap, light and dark, comfortable and touch.
- `packages/design_system/test/components/markdown_test.dart`: the former horizontal scroll test
  now holds the wrap by default and the scroll without wrap.
- `packages/local_store`: the DAO test of the choice, the migration 10 to 11, the schema test.
- `apps/client/test/thread/readable_code_blocks_test.dart`: in the history on macOS and Windows a
  long line wraps, the toggle stores the device choice, the scrollbar shows, shift and the wheel
  and the trackpad scroll, the copy is exact and the choice survives a restart; on iOS and
  Android the toggle copies nothing, a drag scrolls and a tap copies; on all four a block above
  `ui.code_block_collapse_lines` collapses and expands; without a database the toggle switches
  only its block.
- `apps/client/test/app/app_test.dart`: the app provides the code block scope.

Commands that must pass, from the repository root:

- `cd packages/design_system && flutter analyze --fatal-infos && flutter test`
- `cd packages/local_store && flutter analyze --fatal-infos && flutter test`
- `cd apps/client && flutter analyze --fatal-infos && flutter test`
- `make codegen-check`, `make lint`, `make fmt-check`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] A long code block line wraps by default on all four platforms and the whole content is
      visible.
- [ ] "Wrap lines" switches to a horizontal scroll that works with the scrollbar, shift and the
      wheel, the trackpad and a drag, kept per device.
- [ ] A block above `ui.code_block_collapse_lines` shows collapsed with a fade, "Show all" and
      "Show less".
- [ ] The copy is unchanged; the M4-48 tests are green.
- [ ] Spec, READMEs, rulings and changelog fragment updated; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 6.14 or M4-49.
- Rulings in `docs/decisions.d/M4-49.md`; changelog fragment as in Scope.
- An independent code review over the diff, then the red team gate
  (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`).
- Land the change as `AGENTS.md` ("How work lands on `main`") describes.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits, changed files, test results (the red run before the change and the green run after).
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example the feel of the trackpad on a real Mac).
5. Follow-ups with reasons and differences to the names of this brief.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
