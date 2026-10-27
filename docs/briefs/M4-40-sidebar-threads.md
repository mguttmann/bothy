# M4-40: Threads in the sidebar and the forum overview on the desktop

Status: ready | Depends on: M2-52, M4-24 | Milestone: M4

## Goal

After this ticket the desktop sidebar shows, under each forum and each text channel, its open threads as nested rows with a tree line, as in Discord (01 D1, 6.8): in a forum with a project binding every row is one agent session, so the sessions of a project are visible and open with one click. Which rows show, their bound, order, markers, collapse and keyboard follow 01 6.8 items 1 to 7, 10 and 11. The post list column of M4-19 is no longer always there: a click on a forum name opens the forum overview D15 in the content column (post list, search in the forum, filters, tags, Done section), and "Pin" or Cmd+Shift+O keeps it as its own column next to the history (R-PR-17). The channel icons tell a text channel, a forum and a project forum apart (`channel.text`, `channel.forum`, new `channel.project`) and explain the kind in a tooltip, the channel header and the empty state. The sync engine fills the threads from `Snapshot.active_threads` and keeps them current. Why: the owner's first use of the desktop app (2026-10-27): "I have no sessions"; the post list "takes far too much space"; "what is the difference between # and the chat bubble?" ([design note](../design/sidebar-sessions.md), D-SB-1 to D-SB-8, D-SB-10).

## Read first

- `docs/design/sidebar-sessions.md` (all), `docs/decisions.d/sidebar-sessions.md`.
- `docs/PLAN.md`: section 3; section 10 rows M4-19, M4-20, M4-23, M4-24, M4-33, M4-40, M4-41 (who builds on it); section 11 row M5-15; section 8 row M2-52.
- Dependencies: the header names M2-52 and M4-24; through M4-24 this ticket builds on M4-19 to M4-23 and everything below them (all implied, so not in the header).
- `docs/spec/01-product.md`: 3.1, 4 (the kind paragraph), 5 (symbols), 5.1 D1, D2, D15, 6.1 (Done section), 6.7, 6.8 (all), 7 (shortcuts: channel navigation, Cmd+Shift+O, F6), 8 (row "Focus", tree semantics, text scaling), 11 (`layout.column_widths`), 13 (R-PR-17), 14 (`ui.sidebar_thread_window`, `ui.sidebar_threads_per_channel`, `ui.desktop_min_width_four_columns`, `ui.desktop_min_width_three_columns`).
- `docs/design/components.md`: 6 (thread list, header with "Pin"), 7 (channel row, thread row, "+" action, tree semantics), 13 (review list); `docs/design/icons.md` (mapping); `design/tokens/icon.tokens.json`.
- `docs/spec/04-sync-protocol.md`: 6.3 item 4 (`active_threads`), 14 (read state and unread counting), R-SY-28.
- `docs/spec/03-data-model.md`: `threads`, `read_states`, `project_bindings` and the paragraph on `project_bound`.
- `docs/spec/15-client-architecture.md`: 4 (providers per query), 6.1 (local tables), 11 (paths `/c/`, `/t/`).
- `docs/api/openapi.yaml`: `Snapshot.active_threads`, `Channel.project_bound`, `Thread`, `getThread`, `listChannelThreads`.
- `docs/decisions.d/M4-19.md` (D-M4-19-1 to D-M4-19-6, in particular the local table `forum_view_states` and "Waiting for me"), `docs/decisions.d/M4-24.md`.
- Briefs: `docs/briefs/M4-19-sidebar-post-list.md`, `M4-24-shortcuts-focus.md`, `M4-18-desktop-shell.md` (columns, router), `M2-52-snapshot-active-threads.md`; merged code wins on names.
- Code on the base branch: `apps/client/lib/sidebar/` (`space_tree.dart`, `channel_entry.dart`, `sidebar_model.dart`, `providers.dart`), `apps/client/lib/forum/` (`post_list_column.dart`, `post_list_header.dart`, `post_row.dart`, `done_section.dart`, `providers.dart`), `apps/client/lib/app/layout/` (`column_layout.dart`, `column_widths.dart`, `desktop_shell.dart`), `apps/client/lib/app/router.dart`, `apps/client/lib/keyboard/` (`bindings.dart`, `intents.dart`, `column_focus.dart`), `packages/sync_engine/lib/src/snapshot_applier.dart`, `packages/sync_engine/lib/src/appliers/thread_appliers.dart`, `packages/local_store/lib/src/database.dart`, the fake server under `packages/sync_engine/lib/src/testing/`.

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
9. Limits only through named keys: `ui.sidebar_thread_window`, `ui.sidebar_threads_per_channel`, `ui.desktop_min_width_four_columns` and `ui.desktop_min_width_three_columns` from `AppConfig`, never a literal in code. No new personal setting key (01 11 is normative and the server rejects unknown keys).
10. No permission logic in the client (15 1): the "+" action and "New post" follow what the merged post list of M4-19 already uses to show "New post"; rows show only threads the sync engine holds.

Product requirements implemented here:

11. Thread groups under every forum and text channel with the selection of 01 6.8 item 2 ("Waiting for me" as D-M4-19 defines it), the bound and priority of item 3, the order and the deferred removal of item 4, "All posts" of item 5 and the markers and counters of item 6; a title that is still empty reads "New chat" with the "automatic" mark.
12. A group per channel collapses with its own arrow; expanded by default; the state and the pinned overview column are kept per device in the local database, not synchronized (01 6.8 item 7, D-SB-6).
13. A click on a forum name or on "All posts" opens D15 in the content column; "Pin" and Cmd+Shift+O (Ctrl+Shift+O) pin or unpin it as a column with the width from `layout.column_widths` (`post_list`); below `ui.desktop_min_width_three_columns` a pinned overview returns to the content column (01 D1, D15).
14. Keyboard (01 6.8 item 7 and 7, D-SB-8): the sidebar tree reaches thread rows with the arrow keys; arrow right on a channel expands its group or enters its first row, arrow left goes to the parent or collapses; "Next or previous channel" and "next unread" include thread rows; F6 visits the pinned overview column when it is shown. No shortcut triggers an approval (R-PR-04).
15. Thread rows and channel rows cannot be dragged; the personal order of spaces stays (01 6.8 item 10).
16. Icons and explanation (01 6.8 item 11, D-SB-10): `channel.project` for a channel with `project_bound`; tooltip and screen reader text with the kind sentence; the sentence as the subtitle of the channel header and as the empty state of a forum; "New chat" in a project forum, "New post" otherwise.
17. The sync engine stores `Snapshot.active_threads` into `threads` on every snapshot and resync; a `message.created` for a thread it does not hold loads that thread with `getThread`; `channel.updated` updates `project_bound`. No list request per channel on connect.

## Scope

`packages/design_system` and `design/tokens`:

- `design/tokens/icon.tokens.json`: semantic glyph `channel.project` (Phosphor `brackets-angle`, weight `regular`; if the pinned Phosphor version lacks it, choose a glyph of the pinned set that the review list accepts and rule it), regenerated outputs; `docs/design/icons.md` mapping row.
- A thread row view model and widget next to the channel row of M4-07 (`DsThreadRowData` or the name the merged code uses), with goldens.

`packages/sync_engine`: `active_threads` in `snapshot_applier.dart`; the `getThread` load for unknown threads of `message.created`; `project_bound` in the channel applier; fake server: `active_threads` in the snapshot and `project_bound` in channels (registry of M4-17).

`packages/local_store`: the local view state of 01 6.8 item 7 (group collapsed per channel, overview pinned) in the existing local-only table `forum_view_states` or a new local-only table in the next free schema version, with the migration test and the survival test of M4-12 extended; a `project_bound` column in the local channel table if the schema mirror needs one.

`apps/client/lib`:

- `sidebar/`: thread groups in `space_tree.dart` and `channel_entry.dart` (names of the merged code win), a `thread_entry.dart`, the selection function of 01 6.8 items 2 to 6 as a pure function with unit tests (`sidebar_threads.dart`), "All posts", the channel counters of item 6, the "+" action slot that M4-41 fills (a callback; until then it opens D15 with "New post").
- `forum/`: the overview D15 in the content column and as a pinnable column, the "Pin" button, the kind sentence and the empty state; the new post dialog of M4-19 stays until M4-41.
- `app/layout/`: three columns by default, the pinned overview as the optional fourth (01 D1); `app/router.dart`: the forum route opens D15 in the content column.
- `keyboard/`: Cmd+Shift+O (Ctrl+Shift+O), the sidebar navigation over thread rows, the shortcut list and the menu bar.
- ARB keys in `packages/l10n` for every new text (kind sentences, "All posts", "New chat", tooltips, labels).

Fragments: `docs/decisions.d/M4-40.md` (`D-M4-40-<n>`), changelog fragments of the touched components (`<component>/changelog.d/M4-40.md`, root `changelog.d/M4-40.md` otherwise); README of `apps/client` (sidebar, overview).

## Out of scope

- The new chat view, the draft row and the "+" behaviour of 01 6.8 item 8 (M4-41).
- Phone layouts (M5-15).
- Server changes (M2-52); any runner change or worktree per post (D-SB-12).
- A synchronized collapse or pin state and any new personal setting key (D-SB-6).
- Terminal sessions nested under hosts (design note section 4).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-40-sidebar-threads`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `feat(design-system): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG fragment, README and decisions fragment entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server. Temporary directories only via `Directory.systemTemp.createTemp`, removed at the end. Time comes from the injected clock, never from a fixed sleep.
- Never read or print secrets. Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The app shows `branding.product_name` from the instance, never a literal product name.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written after M4-19 to M4-24 merged, in the manner of D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **One selection function.** `selectSidebarThreads(channel, threads, readStates, runs, selection, now, config)` returns the rows of a group and the counters of the channel row and of "All posts" (01 6.8 items 2, 3, 5, 6). It is pure, lives in `apps/client/lib/sidebar/` (or `packages/domain` if the merged code keeps such rules there), and the phone layout of M5-15 reuses it. Property test: the result never exceeds the bound, always holds the selected thread, and never drops a waiting thread for one with lower priority.
2. **No jumping.** The group widget keeps the last shown list while the pointer is over the sidebar or the focus is inside it; it applies removals when both leave (`MouseRegion` and the focus node of the sidebar); additions and state changes apply at once. Order is by `created_at` descending only.
3. **Counters without double counting.** The channel row of an expanded group sums unread and mentions of its main thread and of the threads not shown; a collapsed group sums all; the existing `UnreadCounter` of M4-08 gives the per thread values. The run dot stays an aggregate.
4. **Overview in two places.** D15 is one widget that the router places in the content column or, when pinned, in the column of M4-19; the pinned state is read from the local view state at start, so a restart restores it. `layout.column_widths` keeps the key `post_list` for the pinned column.
5. **Snapshot data.** On `ready` the applier writes `active_threads` into `threads` in the same transaction as the rest of the snapshot; the existing appliers of `thread.*` keep them current; for a `message.created` of an unknown thread the engine queues one `getThread` (deduplicated per thread) and the row appears when it arrives.
6. **Rulings for `docs/decisions.d/M4-40.md`** (`D-M4-40-<n>`): where the selection function lives; the local table for the view state with its schema version; the glyph of `channel.project`; how the deferred removal is detected.

## Tests (write them first)

Unit and widget tests (`flutter test`, in-memory `local_store` filled by fixtures, fake clock):

- `test/sidebar/sidebar_threads_test.dart`: selection by each condition of 01 6.8 item 2; bound and priority of item 3; order by creation; "All posts" counters; channel counters for expanded and collapsed groups; property tests of Design note 1.
- `test/sidebar/thread_group_test.dart`: a thread that stops qualifying stays while the pointer is in the sidebar and leaves when it leaves; a new thread appears at the top at once; collapse state survives a restart on this device.
- `test/forum/overview_test.dart`: a click on the forum name opens D15 in the content column; "Pin" and Cmd+Shift+O move it to its own column and back; the state survives a restart; below `ui.desktop_min_width_three_columns` a pinned overview returns to the content column.
- `test/sidebar/kind_test.dart`: `channel.project` for a forum with `project_bound`; tooltip and semantics label with the kind sentence; "New chat" and "New post" labels; the empty state of a forum.
- `packages/sync_engine/test/snapshot_active_threads_test.dart`: `active_threads` stored on `ready` and on resync; a `message.created` of an unknown thread loads it once; `channel.updated` with `project_bound` updates the channel.
- `test/goldens/sidebar/*`: the sidebar with thread groups in every row state, a collapsed and an expanded group, "All posts", a project forum, light and dark, densities `comfortable` and `compact`, text scaling 1 and 2, `de` and `en`; `test/goldens/forum/*`: D15 in the content column and pinned.

Integration tests (`integration_test/`, against `FakeServer`):

- `sidebar_threads_test.dart`: after connect every forum shows its active posts without a `listChannelThreads` request (the fake counts requests); a post that Sam creates appears at the top of its group; a post closed by the fake leaves after the pointer leaves the sidebar; a run starting in a post shows its run icon.
- `sidebar_keyboard_test.dart`: a keyboard pass through spaces, channels and thread rows with the arrow keys, "next unread" stops at an unread post, F6 visits the pinned overview column.

Run the added tests by name with repetitions as the team rules require (Dart `--plain-name`, 10 repetitions), through the machine's test slot.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make tokens`, `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Open posts and threads appear under their channel with the selection, bound, order, markers and counters of 01 6.8; nothing moves under the pointer.
- [ ] D15 opens in the content column and can be pinned as a column; the pinned and collapsed states survive a restart on this device and are not synchronized.
- [ ] The keyboard reaches thread rows; Cmd+Shift+O works; no shortcut triggers an approval.
- [ ] Text channel, forum and project forum are told apart by icon, tooltip, header subtitle and empty state.
- [ ] After connect the sidebar is filled from the snapshot without a list request per channel.
- [ ] Goldens in light and dark, two densities, text scaling 1 and 2, `de` and `en`.
- [ ] Rulings recorded; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01 6.8, D1, D15 or components.md 7.
- `apps/client/README.md` (English) as in Scope; `docs/design/icons.md` row for `channel.project`.
- Rulings in `docs/decisions.d/M4-40.md`; changelog fragments as in Scope.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the integration tests ran, the local table and schema version of the view state, and the glyph chosen for `channel.project`.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
