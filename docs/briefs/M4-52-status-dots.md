# M4-52: Status dots for unseen agent news in the sidebar

Status: ready | Depends on: M4-51 | Milestone: M4

## Goal

After this ticket the sidebar on macOS, Windows, iOS and Android tells the person at a glance in
which chats an agent did something they have not seen yet. A chat row shows a small status mark
when the agent state of the chat changed since the person last had the chat open: green when a
run finished, yellow when a run is working, blue when the agent waits for the person (an open
question, an approval card, an input request), red when a run failed. The mark leaves as soon as
the person opens the chat, whatever its colour, and comes back only with the next change (the run
they started finishes while they are elsewhere: green; the agent asks something: blue; an error:
red). For chats with agent runs the mark replaces the unread bar, so a row never carries two news
markers; chats without runs keep the unread bar. A forum or project row shows the most urgent
unseen state of its chats (red over blue over yellow over green) while the personal setting "Show
status on projects" ("Status auf Projekten anzeigen", default on) is on. Why: owner request and
amendment of 2026-10-31; the idea comes from the tab dots of a terminal app; with many agent chats
the person could not tell where something had happened without opening each one.

## Read first

- `docs/PLAN.md`: section 10 rows M4-40, M4-46, M4-51, M4-52.
- `docs/spec/01-product.md`: 6.3, 6.8 items 6 and 7, 6.17 (new), 8 ("Color"), 11 (R-PR-31).
- `docs/spec/09-agents.md` 5 (the replies come before `run.finish`; a question or approval is a
  card message and then `waiting_input`) and the state table of a run.
- `docs/spec/04-sync-protocol.md` 14 (read state); `docs/design/components.md` 7.
- Code on the base branch: `apps/client/lib/sidebar/` (`sidebar_threads.dart`, `sidebar_model.dart`,
  `channel_entry.dart`, `thread_entry.dart`, `providers.dart`), `apps/client/lib/thread/history_column.dart`,
  `packages/design_system/lib/src/components/ds_sidebar_thread_row.dart`, `ds_channel_row.dart`,
  `packages/local_store/lib/src/tables/local_only.dart`, `apps/client/lib/settings/`.

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

6. Test first: write the failing test, run it and keep the red output for the report, then make it
   green, then clean up.
7. No en dash and no em dash in any file you write.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin.
   Interface texts come from `packages/l10n`, never as literals in widgets.
9. Limits only through named keys, never a literal in code. Sizes and colours come from the design
   tokens; this ticket adds no colour token.
10. No GUI test on a maintainer's own machine; widget and golden tests run headless.

Product requirements implemented here:

11. Four states with a colour and a shape of their own (01 6.17 item 1).
12. Unseen means changed since the person last had the chat open; opening clears every colour; a
    new change brings the mark back (items 2 and 3).
13. Seen follows the person's read state to every device (item 3).
14. The mark replaces the unread bar on rows with agent runs; rows without runs keep it (item 4).
15. Tooltip and screen reader label name the state in words (item 5).
16. The forum or project row aggregates its chats by urgency behind the setting
    `sidebar.show_project_status`, default on (item 6).

## Scope

- `apps/client/lib/sidebar/` (new `sidebar_status.dart` with the pure rules, new
  `chat_seen_marker.dart` that writes the seen mark, the providers `seenStatesProvider`,
  `chatSeenWriterProvider` and `channelStatusesProvider`, the row data of thread and channel
  rows), `apps/client/lib/thread/history_column.dart` (wraps the history in `ChatSeenMarker`),
  `apps/client/lib/settings/` (the switch, `showProjectStatusProvider`).
- `packages/design_system` (`ds_chat_status.dart` with `DsChatStatus`, `DsChatStatusShape`,
  `DsChatStatusMark`, `dsChatStatusLabel`; the member `status` of `DsSidebarThreadRowData` and
  `DsChannelRowData`, the mark in `DsSidebarThreadRow` and `DsChannelRow`).
- `packages/local_store`: the local-only table `thread_seen_states` (schema version 13), the value
  type `ThreadSeenState`, `ForumViewDao.watchSeenStates` and `ForumViewDao.markSeen`, and the
  deletion with its thread.
- `services/server/internal/sync/domain/settings.go`: the key `sidebar.show_project_status` in the
  settings registry.
- `packages/l10n` (de, en).

Fragments: `docs/decisions.d/M4-52.md` (`D-M4-52-<n>`); changelog fragments `changelog.d/M4-52.md`
and next to the CHANGELOG.md of every touched component; READMEs of the touched modules;
`docs/spec/01-product.md` 6.8, 6.17 (new), 11, 13; `docs/spec/03-data-model.md` (the local-only
table), `docs/design/components.md` 7.

## Out of scope

- A mark on space rows, in the forum overview D15 or in "Running now".
- A server-side "seen" record: the ticket uses the run, interaction and read state the client
  already syncs.
- Push notifications for the same changes (13 decides those).

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with a DCO sign-off on every commit (`git commit -s`).
- Never push to `main`, never tag.
- Tests bind no port and make no network call; time comes from the injected clock.
- Never lower a threshold, disable a gate, delete a test or raise `repo.max_file_kb`.
- This brief was written together with the change, in the manner of D-W-100 and D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Status of a chat.** The newest run of a thread (by `queued_at`) gives its status:
   `queued` and `running` are working, `waiting_input` is waiting (09 5: a question or an approval
   is a card message, then the run waits), `completed` is done, `failed` is failed; `cancelled`
   and a state of a newer server give none. The pure function `chatStatusOf` maps it.
2. **Unseen.** `unseenChatStatus` says whether the person has seen the current status. It is seen
   when any of these holds: (a) the chat is open on this device (its history shows on the desktop
   or the phone); (b) this device's seen mark of the chat names the same run and status; (c) the
   person's read state of the chat was written at or after the change (`read_states.updated_at`
   against `started_at` or `queued_at` for working, `finished_at` for done and failed), on any
   device; (d) for working, waiting and done the chat has nothing unread and no unread mention,
   because these changes come with a message the person has then read (the live box anchor, the
   question card, the agent's answer before `run.finish`). A failure can come without a message,
   so red needs (a) to (c).
3. **Seen mark.** While the history of a thread shows, the client writes the thread's current run
   and status into `thread_seen_states` (local-only, never synchronized, removed with the thread).
   The read state (c, d) carries the seen state to the person's other devices.
4. **Urgency.** failed > waiting > working > done. A forum or channel row shows the most urgent
   unseen status of every stored chat of the channel, its main history included, while
   `sidebar.show_project_status` is on; off, a text channel row still shows the status of its main
   history, which is its own chat.
5. **Rendering.** `DsChatStatusMark` paints `size.status-dot` at the trailing edge of the row, where
   the counters stand, before the mention counter: done a filled circle in
   `category.moss.on-sidebar`, working a ring in `category.gorse.on-sidebar`, waiting a filled
   diamond in `category.loch.on-sidebar`, failed a filled triangle in `category.rowan.on-sidebar`.
   The shapes differ, so the state never rests on colour (01 8 "Color"). A tooltip names the state;
   the row label ends with the same words. While the mark shows the unread bar does not.

## Tests (write them first)

- `apps/client/test/sidebar/sidebar_status_test.dart`: the pure rules: each state, each seen
  condition, failure without a message, the urgency order, the aggregate.
- `packages/design_system/test/components/chat_status_mark_test.dart` and the golden sheet
  `sidebar_status`: each state's shape, colour, tooltip and label; the mark replaces the unread
  bar.
- `apps/client/test/sidebar/status_dots_test.dart`: over the local database: each state on a
  thread row; the mark leaves when the chat opens and stays away after it closes; a new change
  brings it back; the forum aggregate and the setting; a chat without runs keeps the bar; a read
  state written on another device clears it; the history column writes the seen mark.
- `apps/client/test/settings/project_status_setting_test.dart`: the switch.
- `packages/local_store/test/seen_states_test.dart`, the migration 12 to 13 and the schema test.
- `services/server`: the registry test compares the key with 01 11.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test test/sidebar test/settings`
- `cd packages/design_system && flutter test`, `cd packages/local_store && dart test`
- `make codegen-check`, `make lint`, `make fmt-check`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] Each of the four states shows on a chat row with its colour, shape, tooltip and label.
- [ ] Opening the chat clears the mark; it stays away until the next change; a new change brings it
      back.
- [ ] A read on another device clears the mark where the change came with a message, and every
      mark where the read came after the change.
- [ ] Rows with runs never show the unread bar next to a mark; rows without runs keep the bar.
- [ ] Forum and project rows show the most urgent unseen state, and nothing with the setting off.
- [ ] Documentation and fragments of the scope are written; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 6.17 or M4-52.
- Rulings in `docs/decisions.d/M4-52.md`; changelog fragments as in Scope.
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
