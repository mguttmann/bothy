# M4-41: New chat from the sidebar on the desktop

Status: ready | Depends on: M4-40 | Milestone: M4

## Goal

After this ticket a person starts a new chat in a forum from the sidebar in one step (01 6.8 item 8, R-PR-18): every forum row has the action "+" (visible on hover, on keyboard focus and on the selected forum), labelled "New chat in <forum>" in a project forum and "New post in <forum>" otherwise, also in the context menu of the forum; Cmd+N (Ctrl+N) does the same in the forum of the focused or selected row. It opens the new chat view in the content column without a dialog: the input has the focus, the title is optional when the forum has an operator agent (the agent names the chat, 09 12) and required otherwise, tags are optional. At the top of the forum's group a draft row "New chat" appears and is selected; its text is the person's draft of a new post in this forum (03 `drafts` with `thread_id` NULL), so it survives a restart and appears on the person's other devices, and "+" on a forum that has such a draft opens it instead of a second one. Sending creates the post with its first message through the outbox; the draft row becomes the post row in place, and the runner's session starts with the first run. The new post dialog of M4-19 is replaced by this view everywhere ("New chat" and "New post" in D15 open it too). No git worktree is created for a new chat (D-SB-12). Why: the owner wants starting another chat in a project to be quick and visible in the sidebar (2026-10-27; [design note](../design/sidebar-sessions.md), D-SB-9).

## Read first

- `docs/design/sidebar-sessions.md` (all), `docs/decisions.d/sidebar-sessions.md`.
- `docs/PLAN.md`: section 3; section 10 rows M4-19, M4-20, M4-40, M4-41; section 11 row M5-15 (who builds on it).
- Dependencies: the header names M4-40; through it this ticket builds on M2-52, M4-24 and everything below them, including M4-15 (outbox), M4-20 (composer and draft sync) and M2-18 (auto-title) (all implied, so not in the header).
- `docs/spec/01-product.md`: 3.1 (row "Button New post or New chat"), 5.1 D1, D15, 6.6 (offline and queue), 6.7 (titles with the "automatic" mark), 6.8 items 8 and 9, 7 (Cmd+N), 13 (R-PR-18).
- `docs/spec/09-agents.md`: 12 (auto-title: empty and placeholder titles stay `auto`; the reference runners set the title after the first run), 18.1 (several sessions per project, no worktree per thread).
- `docs/spec/03-data-model.md`: `drafts` (one draft per person and forum with `thread_id` NULL), `threads` (`title`, `title_source`).
- `docs/spec/04-sync-protocol.md`: 13.2 (outbox), 14 (drafts across devices, `overwrote_newer`).
- `docs/api/openapi.yaml`: `createThread` (`ThreadCreate` with an empty `title`, `tag_ids`, `first_message`), `putChannelDraft`, `deleteChannelDraft`, `listMyDrafts`.
- `docs/decisions.d/M4-19.md`, `docs/decisions.d/M4-20.md`, `docs/decisions.d/M4-40.md`.
- Briefs: `docs/briefs/M4-19-sidebar-post-list.md` (Design note 3, new post), `M4-20-thread-view.md` (composer, drafts), `M4-15-sync-outbox.md`, `M4-40-sidebar-threads.md`; merged code wins on names.
- Code on the base branch: `apps/client/lib/forum/new_post_dialog.dart` and `post_list_column.dart` (the dialog this ticket replaces), `apps/client/lib/composer/` (`thread_composer.dart`, `composer_controller.dart`, `draft_sync.dart`), `apps/client/lib/sidebar/` (the "+" slot of M4-40), `apps/client/lib/keyboard/` (`intents.dart`, `bindings.dart`), `apps/client/lib/sync/` (`thread_ops.dart`).

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
10. No permission logic in the client (15 1): the "+" action shows where the merged post list shows "New post"; a refusal of `createThread` is shown with its reason as every outbox failure (01 6.6).

Product requirements implemented here:

11. "+" on every forum row with the visibility and labels of 01 6.8 item 8 and components.md 7; the same entry in the context menu of the forum; Cmd+N (Ctrl+N) in the forum of the focused or selected row; in a text channel Cmd+N does nothing (threads come from messages).
12. The new chat view in the content column: header "New chat in <forum>" or "New post in <forum>", the operator line of the channel header, the title field (optional with an operator agent, then with the hint that the agent names the chat; required without one), tags (optional, `thread.max_tags`), the composer of M4-20 with the focus; Esc leaves the view.
13. The draft row: at the top of the forum's group, in italics, selected while the view is open; its text is the channel draft (`putChannelDraft` through the outbox, `draft.updated` from other devices as 04 14 says); an empty draft is deleted when the person leaves the view and the row disappears; "+" on a forum with a draft opens that draft.
14. Sending: one `createThread` through the outbox with `title` (empty when none was given), `tag_ids` and `first_message` with its own `client_msg_id`, followed by `deleteChannelDraft` of the forum's draft in the same outbox order (use `clear_draft` instead only if the merged server deletes the channel draft with it, and rule which); while it waits, the row shows "sending" (01 6.6); on the echo the draft row becomes the post row in place and the history of the new post opens; the title shows "New chat" with the "automatic" mark until the agent sets it (01 6.7, 09 12).
15. Offline: the view, the draft row and sending work offline; the post is created after reconnecting in outbox order (04 13.2).
16. The new post dialog of M4-19 is removed; "New chat" and "New post" in D15 open the new chat view.
17. No git worktree, no extra folder and no runner request is involved; the new post is a new session in the project folder (09 18.1).

## Scope

`apps/client/lib`:

- `forum/new_chat_view.dart` (the view of requirement 12), `forum/new_chat_controller.dart` (draft, title rule, send), removal of `new_post_dialog.dart` and its call sites.
- `sidebar/`: the "+" action in the slot of M4-40, the draft row in the thread group (it counts toward neither the bound nor the counters of 01 6.8), the context menu entry.
- `keyboard/`: Cmd+N (Ctrl+N) bound to the forum of the focused or selected row; the shortcut list text. In the same change, update the Cmd+N row of the shortcut table of `docs/spec/01-product.md` 7 as its "Planned" paragraph says and remove that item (and the paragraph when it is empty); `test/keyboard/bindings_spec_test.dart` compares the table with `bindings.dart` row by row.
- `sync/thread_ops.dart` (names of the merged code win): `createThread` with an empty title, then the removal of the channel draft (requirement 14).
- ARB keys in `packages/l10n` for every new text.
- Fake server handlers (registry of M4-17) for `putChannelDraft`, `deleteChannelDraft` and `createThread` with an empty title, if missing.

Fragments: `docs/decisions.d/M4-41.md` (`D-M4-41-<n>`), changelog fragments of the touched components (`<component>/changelog.d/M4-41.md`, root `changelog.d/M4-41.md` otherwise); README of `apps/client` (new chat).

## Out of scope

- Phone layouts (M5-15).
- Server or runner changes; a worktree per post (D-SB-12).
- New chat in a text channel (threads come from messages, 01 3.1).
- Templates in the first message beyond what the composer of M4-20 already offers.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-41-sidebar-new-chat`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG fragment, README and decisions fragment entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server. Temporary directories only via `Directory.systemTemp.createTemp`, removed at the end. Time comes from the injected clock, never from a fixed sleep.
- Never read or print secrets. Never lower a threshold, disable a gate, delete a test (other than the tests of the removed dialog, replaced by tests of the view), raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written after M4-19 to M4-24 merged, in the manner of D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **The draft is the state.** The view holds no state of its own beyond the focus: text and attachments are the channel draft through the composer and draft sync of M4-20, the title and tags live in the view until sending (the server draft has no title; ruling: a title typed and abandoned is not kept). The draft row exists exactly while a channel draft of the forum exists or the view of that forum is open.
2. **Title rule.** "Has an operator agent" is the effective operator of the channel as the channel header shows it (M4-20); with one, an empty title is sent as empty (the server keeps `title_source` `auto`, 09 12 rule 2); without one, the send button stays disabled until a title is there.
3. **In place.** The draft row and the post row share the top position of the group (newest creation first, 01 6.8 item 4), so the replacement on the echo does not move anything; the selection moves from the draft to the post.
4. **Rulings for `docs/decisions.d/M4-41.md`** (`D-M4-41-<n>`): title and tags not kept in the draft; the operator rule for the title; the removal of the dialog.

## Tests (write them first)

Widget tests (`flutter test`, in-memory `local_store`, fake clock):

- `test/forum/new_chat_view_test.dart`: the view opens from "+", the context menu and Cmd+N with the focus in the input; the title is optional with an operator agent and required without; Esc leaves; an empty draft is deleted on leaving.
- `test/sidebar/draft_row_test.dart`: the draft row appears at the top, is selected, counts toward neither the bound nor the counters; a second "+" opens the same draft; the row survives a restart.
- `test/goldens/forum/new_chat_*` and `test/goldens/sidebar/draft_row_*`: light and dark, densities, text scaling 1 and 2, `de` and `en`.

Integration tests (`integration_test/`, against `FakeServer`):

- `new_chat_test.dart`: "+" in a project forum, type, send: one `createThread` with an empty title, the row stays in place and becomes the post, its history opens; the fake runner sets a title and the row shows it; offline the row shows "sending" and the post is created after reconnecting; a forum without an operator agent requires a title.
- `new_chat_two_devices_test.dart`: a draft typed on one simulated device appears as a draft row on the other.

Run the added tests by name with repetitions as the team rules require (Dart `--plain-name`, 10 repetitions), through the machine's test slot.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] "+", the context menu and Cmd+N open the new chat view without a dialog, with the focus in the input.
- [ ] Sending without a title creates the post with `title_source` `auto`; the row stays in place; a forum without an operator agent requires a title.
- [ ] The draft row is the channel draft: one per forum, across restarts and devices; empty drafts disappear.
- [ ] Offline sending works through the outbox.
- [ ] The dialog of M4-19 is gone; goldens in light and dark, two densities, text scaling 1 and 2, `de` and `en`.
- [ ] Rulings recorded; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01 6.8 item 8 or 09 12.
- `apps/client/README.md` (English) as in Scope.
- Rulings in `docs/decisions.d/M4-41.md`; changelog fragments as in Scope.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the integration tests ran.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
