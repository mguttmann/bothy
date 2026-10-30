# M4-47: Long pastes become a file and long texts stay one message

Status: ready | Depends on: M3-12, M4-46 | Milestone: M4

## Goal

After this ticket a person never hits a length limit. In the composer of a thread and of a new post (desktop and phone), a paste of more than `ui.composer_paste_file_chars` (4000) characters or more than `ui.composer_paste_file_lines` (100) lines becomes the attachment `paste.txt`, or `paste.<extension>` when it is clearly code, shown as a file chip with "Insert as text"; a draft larger than `message.max_content_bytes` goes out as one message whose rest is one text attachment `message.txt`, with a note before sending. Agents (both SDKs, the Hermes adapter, the Claude Code runner) send a long answer as one message and its rest above the server limit as one attachment `answer.txt`; nothing splits a message into several. Why: owner decision of 2026-10-29, coming from a chat tool where 2000 characters forced splitting a text or a paid plan.

## Read first

- `docs/PLAN.md`: section 10 rows M4-22, M4-41, M3-12, M4-47.
- Dependencies: the header names M3-12 (answer attachments of the runner) and M4-46; through M4-46 this ticket builds on M4-22 (attachments in the composer), M4-41 (the new chat view), M2-30 and M2-33 (all implied, so not in the header).
- `docs/spec/01-product.md`: 3.3, 6.4, 6.13 (new), 13 (R-PR-26), 14.
- `docs/spec/09-agents.md`: 4.7, 17.3, 18.10; `docs/spec/11-extensions.md` 11.2 (`outbox`); `docs/spec/12-files.md` 3.
- `docs/operations/configuration.md`: `message.max_content_bytes`, `http.max_json_body_bytes`, `hermes_adapter.max_message_bytes`.
- Code on the base branch: `apps/client/lib/composer/` (`composer_controller.dart`, `thread_composer.dart`), `apps/client/lib/files/attach_controller.dart`, `apps/client/lib/files/attachment_tile.dart` (`UploadChip`), `apps/client/lib/forum/new_chat_controller.dart`, `new_chat_view.dart`; `sdk/python/src/bothy_sdk/outbox.py`, `uploads.py`, `testing/fakeserver.py`; `sdk/typescript/src/outbox/`; `plugins/hermes-adapter/src/bothy_hermes_adapter/handler.py`, `platform.py`, `entry.py`; `plugins/claude-code-runner/src/claude_code_runner/runturn.py`, `reply_upload.py`.

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
9. Limits only through named keys, never a literal in client code. The new configuration keys are `ui.composer_paste_file_chars` and `ui.composer_paste_file_lines` (place App); the server limit stays `message.max_content_bytes`.
10. No GUI test on a maintainer's own machine; widget tests run headless.

Product requirements implemented here:

11. A paste over either threshold does not go into the field: it becomes a file chip `paste.txt` above the field; one fenced block with a known language becomes `paste.<extension>` with the code of the block; a script with `#!`, JSON, XML, HTML or a diff gets that extension.
12. The chip has "Insert as text", which removes the file and its upload entry and puts the pasted text into the field.
13. Typed text, templates and quotes never become a file; a paste the attachment rules refuse stays text with the refusal shown.
14. While the draft is larger than `message.max_content_bytes`, a note says that the rest goes out as `message.txt`; sending makes one message with the head and the rest as one attachment, never a limit error and never several messages. Only when the attachment is refused (count full) the limit shows as before.
15. The cut is at a line break, never inside a character, never inside a fenced code block (the block moves whole into the rest); head and rest together are exactly the text.
16. The new chat view of a forum behaves the same; its post waits for its uploads.
17. The Python and TypeScript SDKs offer one call that sends a long answer as one message with its rest as one attachment `answer.txt`, and the head with a note when that upload fails; the Python FakeServer refuses a `content_md` above the limit as the server does.
18. The Hermes adapter sends a long answer through that call; Hermes never splits an answer (`MAX_MESSAGE_LENGTH` and `max_message_length` far above any limit, `splits_long_messages = True`).
19. The Claude Code runner sends an answer above the limit as one message with the rest as the last attachment `answer.txt` of the answer's set.
20. The renderer keeps one fenced code block as one block however long it is (01 6.4).

## Scope

- `apps/client/lib/composer/`: `long_text.dart` (the pure rules), the paste detection, "Insert as text", the overflow cut and note in `composer_controller.dart` and `thread_composer.dart`, `attachment_chips.dart` shared with the new chat view.
- `apps/client/lib/files/`: `AttachController.addText`, the pasted text on `PendingAttachment`, an attachment controller without a thread, `newPostAttachmentsProvider`, "Insert as text" on `UploadChip`.
- `apps/client/lib/forum/`: attachments, chips and notes of the new chat view.
- `packages/domain` (regenerated `AppConfig` keys), `packages/l10n` (`composerPasteAsText`, `composerOverflowNote`), `packages/design_system` (renderer test only).
- `sdk/python` (`bothy_sdk.long_answers`, FakeServer limit), `sdk/typescript` (`sendLongMessage`, `attachmentFileIds`), `plugins/hermes-adapter`, `plugins/claude-code-runner`.

Fragments: `docs/decisions.d/M4-47.md` (`D-M4-47-<n>`), changelog fragments `changelog.d/M4-47.md` at the root and next to the CHANGELOG of `sdk/python`, `sdk/typescript`, `plugins/hermes-adapter` and `plugins/claude-code-runner`; READMEs of `apps/client`, both SDKs and both plugins; `docs/sdk/python.md`; `docs/spec/01-product.md` 3.3, 6.4, 6.13, 13 and 14; `docs/spec/09-agents.md` 17.3, 18.10 and 21; `docs/spec/11-extensions.md` 11.2; `docs/operations/configuration.md` (generated); `docs/design/components.md` 9.

## Out of scope

- Editing a message above the limit: `MessageUpdate` carries no attachments, so an edit keeps the limit (D-M4-47-4).
- A server change: `message.max_content_bytes` and `http.max_json_body_bytes` stay as they are; the server already refuses a longer content.
- `send_document` of the Hermes adapter (files Hermes produces itself), a follow-up of 09 17.3.

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with the package or component as scope and a DCO sign-off on every commit (`git commit -s`).
- Never push to `main`, never tag; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the fragments and READMEs named in this brief.
- Tests bind no port and make no network call except the in-process fake servers; time comes from the injected clock.
- Never lower a threshold, disable a gate, delete a test or raise `repo.max_file_kb`.
- This brief was written together with the change, in the manner of D-W-100 and D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Detection by the change, not by the key.** A paste is the text one change of the field inserted (common prefix and suffix of the old and new text). Every paste path counts (Cmd+V or Ctrl+V, the context menu, a phone keyboard, a text drop); the controller marks the text it sets itself (templates, quotes, a draft from another device, "Insert as text"), so those never become a file.
2. **The pasted text stays with the file.** The attachment keeps the pasted text in memory, so "Insert as text" works after the upload deleted its copy; the attachments live in a provider kept while the app runs, as M4-22 does for a thread.
3. **One cut rule in three languages.** Dart, Python and TypeScript cut the same way: the longest prefix within the bytes, back to the last line break, and back before the opening line of a fenced block the cut lies in (unless the block starts the text).
4. **Agents read the limit only when it matters.** The Hermes adapter uses `hermes_adapter.max_message_bytes`; the runner reads `getInstance` only for an answer text above `LIMIT_READ_FLOOR_BYTES`; the SDK call takes the limit or reads it once.

## Tests (write them first)

- `apps/client/test/composer/long_text_test.dart`: the paste rule, the inserted text of a change (property), the file name and content of a paste, the cut (line break, characters, surrogate pairs, fences, property), the byte check.
- `apps/client/test/composer/paste_as_file_test.dart`: a long paste becomes `paste.txt` with the text around it kept; 101 lines become a file and 100 not; a fenced python block becomes `paste.py`; "Insert as text"; the paste goes out as the message's attachment; a draft over the limit shows the note and goes out as one message with `message.txt`, head and file together the text; the refusal only when no file may be added; no note within the limit; the new chat view with a long paste and with a post over the limit.
- `apps/client/test/composer/composer_test.dart`: content over the limit goes out as one message.
- `packages/design_system/test/components/markdown_test.dart`: one long fenced block with blank lines stays one block with all its lines.
- `sdk/python/tests/test_long_answers.py`, `sdk/typescript/test/long-answers.test.ts`: the cut, one message within and above the limit with the rest attached, the head with a note when the upload fails, the FakeServer limit.
- `plugins/hermes-adapter/tests/unit/test_handler_runs.py`: a long answer is one message with `answer.txt`; Hermes never splits.
- `plugins/claude-code-runner/tests/scenarios/test_long_answers.py`: a long answer is one message with `answer.txt`; within the limit no file; a short answer reads no limits; the rest comes after the `ATTACH:` files.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test`
- `cd packages/design_system && flutter test`
- `cd sdk/python && uv run --locked pytest`; `cd sdk/typescript && npm test`
- `cd plugins/hermes-adapter && uv run --locked pytest`; `cd plugins/claude-code-runner && uv run --locked pytest`
- `make codegen-check`, `make lint`, `make fmt-check`, `make config-doc-check`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] A long paste becomes a file chip with "Insert as text" in the thread composer and the new chat view.
- [ ] A text over the limit goes out as one message with its rest attached, with the note before sending.
- [ ] Both SDKs, the Hermes adapter and the runner send a long answer as one message with one attachment; nothing splits a message.
- [ ] Spec, configuration reference, READMEs, rulings and changelog fragments updated; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 6.13 or M4-47.
- Rulings in `docs/decisions.d/M4-47.md`; changelog fragments as in Scope.
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
