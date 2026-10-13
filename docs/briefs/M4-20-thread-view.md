# M4-20: Thread view, input and message actions

Status: ready | Depends on: M4-19 | Milestone: M4

## Goal

After this ticket a person can read and write in the desktop app: the history column shows a thread (a forum post or a thread from a message) and the main thread of a text channel (01 5.1 D2) from the local database with the message component of M4-07 (grouping, origin badges, safe Markdown, "Show more" for long replies, tombstones, system lines), loads older pages and fills gaps through the engine, reports the visible thread to the gateway (`focus`) and marks it read; the thread header names the operator (or "No agent answers here"), the title with its "automatic" mark, and offers Done and reopen, pin and the session column; the composer of M4-07 sends through the outbox (Enter or Shift+Enter by `composer.enter_sends`), keeps the draft in sync across devices, suggests mentions from visible people and agents and shows who is typing; the message actions edit (with the `412` conflict shown as two revisions), delete with one confirmation, reply with quote, start a thread from a message, reactions, pin and unpin, mark as unread and copy; and every offline state of 01 6.6 is visible (banner, "sending", "Sending delayed", failed with "Send again", "Edit" and "Discard", "No access anymore"). The session column gets its frame with the pinned messages; M4-21 and M4-22 add the agent session and files. Rendering passes SEC-T13: nothing in a message is executed or loaded. Why: this is the conversation itself; every rule of 01 6 and 04 13 meets the person here.

## Read first

- `docs/PLAN.md`: section 3; section 10 (Red team: T-AI-06; rows M4-20 to M4-24).
- `docs/spec/01-product.md`: 3.1, 3.3 "Messages", 5.1 D1 and D2 (history, thread header, session column, composer, typing line), 6 "Interaction rules" (6.1, 6.4, 6.6, 6.7), 7 (Enter, Shift+Enter, arrow up, Esc, Option+Click: the actions that M4-24 binds to shortcuts), 8, 11 (`composer.enter_sends`), 14 (`ui.message_group_window`, `ui.message_collapse_lines`, `ui.composer_max_lines`).
- `docs/design/components.md`: 1 "Message", 9 "Input with slash suggestions" (the composer states without commands; commands are M4-21).
- `docs/spec/03-data-model.md`: `messages` (content format with the mention rules, `reply_to_message_id`, `kind`, `origin`, tombstones), `message_revisions`, `reactions`, `pins`, `drafts`, `read_states`, `threads` (`title_source`, `state`, operator).
- `docs/spec/04-sync-protocol.md`: 4, 5.1, 8.3 items 3 and 4, 11.3 item 2 (focus), 12 item 1 (typing), 13 "Offline behavior of the client" (13.2 to 13.4), 14 (read state, drafts with `overwrote_newer` and `clear_draft`), 16.1 and 16.2.
- `docs/spec/07-permissions.md` 4.6 (who can be mentioned; the client offers only visible principals).
- `docs/spec/19-security.md`: SEC-OUT-1 to SEC-OUT-3, SEC-T13; `docs/security/threat-model.md` T-AI-05, T-AI-06.
- `docs/spec/15-client-architecture.md`: 4, 5, 7.3 item 4 (gaps), 7.5, 11 (`/t/{thread_id}?m={message_id}`).
- `docs/api/openapi.yaml`: `listThreadMessages`, `getThread`, `createMessage`, `updateMessage` (`If-Match`, `412`, `410`), `deleteMessage`, `listMessageRevisions`, `addReaction`, `removeReaction`, `listReactions`, `pinMessage`, `unpinMessage`, `listPins`, `putReadState`, `markThreadUnread`, `putThreadDraft`, `deleteThreadDraft`, `closeThread`, `reopenThread`, `setThreadTitle`, `createThread` (`parent_message_id`), `getInstance` (`InstanceLimits`).
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115; section 10.
- Briefs: `docs/briefs/M4-19-sidebar-post-list.md`, `docs/briefs/M4-18-desktop-shell.md` (columns, router, providers), `docs/briefs/M4-07-core-components.md` (`DsMessage`, `DsMarkdown`, `DsComposer`, `DsSuggestionList`, `DsBanner`), `docs/briefs/M4-15-sync-outbox.md` (executors, `watchThread`, edit and delete rules), `docs/briefs/M4-14-sync-catch-up.md` (gaps, `targetsRemoved`, `draftOverwritten`, `DraftEditing` port), `docs/briefs/M4-13-sync-connection.md` (`FocusManager`, `TypingThrottle`), `docs/briefs/M4-17-fake-server.md` (handlers, scenarios).

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
9. Limits only through named keys: `ui.message_group_window`, `ui.message_collapse_lines`, `ui.composer_max_lines` from `AppConfig`; content and attachment limits from `InstanceLimits` of `getInstance` (D-W-115), checked before sending; no literal in code.
10. Specification first: if a flow of 01 6 needs an operation or field that `docs/api/openapi.yaml` lacks, stop (Boundaries).

Product requirements implemented here:

11. The history reads only from the local database; a gap in `thread_seq` shows a loading row until the engine fills it, never messages out of order (04 4, 15 7.3).
12. Messages of the same author within `ui.message_group_window` are grouped; messages over `ui.message_collapse_lines` lines collapse with "Show more"; code blocks scroll; long replies are never split (01 6.4).
13. Messages from agents, apps, imports and the system carry their origin badge; only server messages of kind `system` render as system lines (SEC-OUT-3).
14. Rendering executes nothing and loads nothing: raw HTML stays text, `javascript:` and other unsafe links stay text, image URLs are not loaded, there is no link preview (SEC-OUT-1, SEC-OUT-2, SEC-T13).
15. Every write goes through the outbox (15 1 principle 3); a sent message appears at once with "sending" after the last confirmed `thread_seq`; permanent errors show the reason with "Send again" (new ID), "Edit" and "Discard"; "Sending delayed" after `client.outbox_stuck_after` (01 6.6, 04 13.2).
16. Editing a sent message sends `If-Match`; `412 version_conflict` shows both revisions and the person decides; `410 message_deleted` keeps the text for copying (04 13.3).
17. Deleting asks for confirmation once (01 6.7); a deleted message shows its tombstone.
18. A new message in a done thread reopens it (the server writes the system line); "Done" and reopen from the header (01 6.1).
19. The header names the operator or says "No agent answers here"; an automatic title carries the "automatic" mark and editing makes it manual (01 6.7).
20. Mention suggestions show only people and agents that the local database knows (what the person may see, 01 6.7); a token is shown as a mention only when its principal is in `mention_ids` (03 content format).
21. Read state: the visible thread is marked read through the outbox and only increases; "Mark as unread" uses `markThreadUnread` with `If-Match` (04 14).
22. Drafts are saved through the outbox and synchronized; a change from another device does not overwrite text the person is typing; `overwrote_newer` shows a notice; sending with `clear_draft` (04 14).
23. Typing: `typing.start` at most once per `sync.typing_min_interval`, `typing.stop` on clear or send; the typing line shows people typing in the thread (04 12).
24. On access removal the open view closes with "No access anymore" and waiting messages stay as text for copying (01 6.6, 04 8.3).

## Scope

Create in `apps/client/lib/`:

- `thread/` (`history_column.dart`, `message_list.dart` virtualized with paging up and the gap row, `message_tile.dart` mapping to `DsMessageData`, `thread_header.dart`, `title_editor.dart`, `system_line.dart`, `revisions_dialog.dart` for the `412` case, `delete_confirm.dart`, `reaction_picker.dart`, `message_menu.dart` with the actions, `outbox_rows.dart` for pending and failed entries, `typing_line.dart`, `providers.dart`).
- `composer/` (`composer_controller.dart` with draft sync, mention suggestions and the send path; `mention_suggestions.dart`; `draft_sync.dart` implementing the `DraftEditing` port of M4-14).
- `session_column/` (`session_column.dart` frame with the section "Pinned", `pins_section.dart`).
- `sync/message_ops_app.dart` only for the executors M4-15 does not have yet (`markThreadUnread`, `pinMessage`, `unpinMessage`, `closeThread`, `reopenThread`, `setThreadTitle`), registered in the bootstrap.
- Fake server handlers for the operations above that M4-17 does not have (`listMessageRevisions`, `markThreadUnread`, `pinMessage`, `unpinMessage`, `listPins`, `closeThread`, `reopenThread`, `setThreadTitle`, `createThread` with `parent_message_id`, `getInstance`), through the handler registry of M4-17.
- Tests under `test/thread/`, `test/composer/`, goldens under `test/goldens/thread/`, integration tests under `integration_test/` (Tests).
- ARB keys for every new text in `packages/l10n`.

Change:

- `apps/client/README.md` (thread view, composer, message actions).
- `docs/DECISIONS.md` section 10: rows `D-M4-20-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Live box, trigger state, follow-ups, components on messages, slash commands and the agent part of the session column (M4-21).
- Attachments, images, voice messages and the "Files" section (M4-22).
- Search in the thread, quick switcher, mentions view (M4-23); keyboard shortcuts as bindings (M4-24; this ticket offers the actions they call).
- Moving threads (console and administration; the desktop shows the result through `thread.moved`).
- Link preview (R-PR-09: later and only with opt-in).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-20-thread-view`; one writer per worktree.
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

1. **History.** `MessageList` over `MessagesDao.watchThread` with the newest page first and paging up through the engine's `Fetcher` when the person scrolls near the top; the scroll position stays anchored on new pages; a jump to `?m=<message_id>` loads the range around it and highlights it. Outbox entries of the thread (M4-15 `watchThread`) render after the last confirmed `thread_seq` with their state. Grouping through `isContinuation` of M4-08 with `ui.message_group_window`. The main thread of a text channel shows thread starters as a line "Thread: <title>, <n> replies" that opens the thread in the next column (D2).

2. **Focus and read state.** When the history of a thread is visible, `FocusManager.setVisible` includes it (M4-13), with `idle` when the window has no focus. When the newest message is visible and the window is focused, the controller writes `putReadState` with the newest `thread_seq` through the outbox (at most once per newest message; the read state only increases). `Esc` in the history marks read (the binding is M4-24; the action is here). "Mark as unread" from a message calls `markThreadUnread` with `If-Match` of the read state version.

3. **Composer.** The `DsComposer` of M4-07 in the bottom of the history; placeholder "Message to <channel>" or "Message to <thread title>"; send: validate against `InstanceLimits` (content bytes), create the outbox entry with `clear_draft: true`, clear the field, `typing.stop`. Enter or Shift+Enter by `composer.enter_sends` (without a stored value `true` on desktop). Arrow up in the empty field opens the last own message of the thread for editing. Drafts: every change of the field updates the one pending `putThreadDraft` entry of the thread in place (M4-15 `edit` on a `pending` entry) or enqueues one when none is pending, so at most one draft write is in flight and one waits, with the newest text, and no timer or literal delay is needed; apply `draft.updated` only when the field is not being edited (`DraftEditing` port), show the `overwrote_newer` notice from `draftOverwritten`. Mentions: typing `@` opens `DsSuggestionList` with principals from the local `principals` table filtered by the typed prefix (people and agents only, active); choosing inserts `@<handle>`.

4. **Message actions.** A menu on hover and by keyboard (focus a message, then the menu key or Shift+F10): Reply (quote the first lines as a block quote and set `reply_to_message_id`), Start thread (`createThread` with `parent_message_id`; in a text channel only), React (picker with the reactions already on the message plus the system emoji input; `addReaction` and `removeReaction` toggle), Pin and Unpin, Edit (own messages; inline editor, `updateMessage` with `If-Match`; `412` opens `RevisionsDialog` with the local text and the server's current revision: keep mine (new edit with the new version) or take theirs; `410` shows the text for copying), Delete (own messages or with permission from the server's capabilities, one confirmation; `deleteMessage`), Mark as unread, Copy text, Copy link (`<scheme>://t/<thread>?m=<message>`). Actions that the server's capabilities do not allow are hidden, never derived from grants (15 1 principle 1).

5. **Header.** Title with the "automatic" mark when `title_source` is automatic; editing calls `setThreadTitle`. Operator: the effective operator of the channel from the local row, or "No agent answers here". Buttons: Done or Reopen (`closeThread`, `reopenThread`), Pin (pins the thread view in the session column list), Column (toggle the session column). Read-only channels replace the composer with the note "Read only in this channel".

6. **Offline and removal.** The engine's connection state shows `DsBanner` "Offline" with the connecting hint; failed outbox entries show the reason through `problemText` of M4-06 with the three actions; `targetsRemoved` for the open thread or channel closes the view and shows "No access anymore" with the waiting messages as copyable text.

7. **Rulings for docs/DECISIONS.md** (`D-M4-20-<n>`): read state trigger (note 2); draft writes coalesced in the outbox (note 3); the conflict dialog choices (note 4); action visibility from server capabilities (note 4).

## Tests (write them first)

Widget and golden tests (in-memory `local_store` with fixtures, fake engine parts, fake clock):

- `test/thread/history_test.dart`: grouping within the window; "Show more" above the collapse line count; tombstone; system line; origin badges; gap row until the range is filled; paging up keeps the anchor; jump to a message.
- `test/thread/sec_t13_test.dart` (SEC-T13): a message with raw HTML (`<script>`, `<img src=...>`), a `javascript:` link, an image URL in Markdown and a bare URL renders as text or inert links, no network request is attempted (the test installs an `HttpOverrides` that fails on any request), and there is no preview.
- `test/thread/actions_test.dart`: each action creates the right outbox entry; edit with `412` shows both revisions; `410` keeps the text; delete asks once; actions hidden without the capability.
- `test/composer/composer_test.dart`: Enter and Shift+Enter by setting; arrow up edits the last own message; mention suggestions only from visible principals; content over the limit is refused with the reason; draft saved, foreign draft not applied while typing, `overwrote_newer` notice.
- `test/thread/offline_test.dart`: banner, sending, delayed, failed with the three actions; "No access anymore" on removal with copyable text.
- `test/goldens/thread/*`: history with grouped, collapsed, edited, deleted, pending, failed and system messages; header with and without operator; composer states; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Integration tests (`integration_test/`, against `FakeServer`):

- `interaction_rules_test.dart`: the flows of 01 6: send online; send offline with a queue and reconnect (exactly one message, scenario 16.2 through the interface); a message in a done thread reopens it; edit conflict; delete; reply with quote; reactions; pin; mark as unread; read state across two simulated devices; draft across two devices.
- `removal_test.dart`: the fake removes Alex's access to the channel while the thread is open: the view closes with "No access anymore" and the pending message remains as text (scenario 16.4 through the interface).

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Integration tests of the flows of 01 "Interaction rules" are green against the fake server, including sending offline with a queue.
- [ ] SEC-T13: Markdown with HTML, a `javascript:` link, an image URL and a link preview case execute and load nothing.
- [ ] Editing, deleting, reply with quote, reactions, pinning, long replies and every offline state behave as 01 6 and 04 13 say.
- [ ] Read state, drafts and typing follow 04 12 and 14.
- [ ] Goldens in light and dark, two densities, text scaling 1 and 2, `de` and `en`; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01, 04 or 19.
- `apps/client/README.md` (English) as in Scope.
- Rulings of Design note 7 in `docs/DECISIONS.md` section 10.
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
4. Where the integration tests ran, the result of the SEC-T13 test with the list of inputs, and the number of draft writes the composer test produced for one typed sentence.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
