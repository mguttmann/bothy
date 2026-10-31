# M4-51: Sidebar ordering

Status: ready | Depends on: M4-46, M4-49 | Milestone: M4

## Goal

After this ticket a person puts the sidebar into an order of their own, on macOS, Windows, iOS and
Android: the spaces, the channels and forums of each space, and the open posts of each forum. On
the desktop a row is dragged with the mouse and a line shows where it lands; on the phone a long
press lifts the row; on the desktop keyboard and with a screen reader "Move up" and "Move down" in
the row menu move it one place. The order is the person's own, follows the person to every device
and never changes what anyone else sees. Three switches in Settings, Appearance ("Arrange spaces
automatically", "Arrange channels and projects automatically", "Arrange chats automatically",
German labels in the `de` locale) arrange a level by its last activity instead, newest first; such a level
cannot be dragged and says so. Defaults: spaces and channels manual, chats by activity (the order
of M4-46). Why: owner request of 2026-10-31; nothing in the sidebar could be moved, neither five
imported spaces, nor the 25 project forums of a dev space, nor the chats of a forum.

## Read first

- `docs/PLAN.md`: section 10 rows M4-19, M4-40, M4-46, M4-49, M4-51.
- Dependencies: the header names M4-46 (all open forum posts, the held order) and M4-49 (`local_store`
  schema version 11); through them this ticket builds on M4-19 (the personal space order), M4-40
  and M4-45.
- `docs/design/sidebar-ordering.md` (the decisions D-M4-51-1 to D-M4-51-9).
- `docs/spec/01-product.md`: 6.7, 6.8 items 3, 4 and 10, 6.16 (new), 11, 13 (R-PR-24, R-PR-30).
- `docs/spec/03-data-model.md` `user_space_states`, `user_channel_states`; `docs/spec/04-sync-protocol.md`
  14; `docs/spec/06-api.md` Preferences.
- Code on the base branch: `apps/client/lib/sidebar/` (`space_tree.dart`, `space_node.dart`,
  `channel_entry.dart`, `thread_entry.dart`, `sidebar_threads.dart`, `sidebar_order.dart`,
  `providers.dart`), `apps/client/lib/menus/`, `apps/client/lib/settings/`,
  `apps/client/lib/sync/sidebar_ops.dart`, `services/server/internal/chat/app/sidebar.go`.

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
9. Limits only through named keys, never a literal in code. The one new configuration key is the
   server key `sidebar.order_max_items`; the three new personal setting keys are in 01 11.
10. No GUI test on a maintainer's own machine; widget tests run headless.

Product requirements implemented here:

11. Three levels, each in the manual order or arranged by activity (01 6.16 item 1); text channel
    threads keep their rule.
12. Desktop: a mouse drag with a drop line; touch: a long press lifts the row, a swipe scrolls, a
    short touch opens; a row never lands in another parent (item 2).
13. "Move up" and "Move down" in the menus of a space, a channel and a forum post in the sidebar,
    disabled at the ends, and the same screen reader actions on the rows (item 2).
14. The order is personal, synchronized per person, never visible to others; rows not in the
    person's list come first in the default order (item 3).
15. Three personal settings with the defaults off, off, on; an automatic level orders by last
    activity, newest first, with ties in the manual order, holds while the person is in the
    sidebar, and a message moves its chat and forum to the top (item 4).
16. An automatic level cannot be dragged; a note names the switch; its menu moves are disabled
    below "Arranged by activity" (item 5).
17. The limit of "Show more" takes the first posts of a manual order; a move keeps the posts behind
    it in place (item 6).

## Scope

- `services/server`: a migration with `user_space_states.channel_order` and the table
  `user_channel_states`; the member `channel_order` of `putMySidebarSpace`, the new operation
  `putMySidebarChannel`, the event `sidebar_channel.updated`, the snapshot member
  `sidebar_channels`, the configuration key `sidebar.order_max_items`, the three keys of the
  settings registry; the personal data export.
- `docs/api` and every generated copy (`packages/api_client`, `sdk/python`, `sdk/typescript`, the
  contract copy of the fake server, the domain event catalogue).
- `packages/domain` (`UserSpaceState.channelOrder`, `UserChannelState`), `packages/local_store`
  (schema version 12), `packages/sync_engine` (appliers, snapshot, fake world).
- `apps/client/lib`: `sidebar/` (`sidebar_arrange.dart`, `sidebar_drag.dart`, the tree widgets, the
  providers), `menus/`, `settings/`, `sync/sidebar_ops.dart`; `packages/l10n`.

Fragments: `docs/decisions.d/M4-51.md` (`D-M4-51-<n>`); changelog fragments `changelog.d/M4-51.md`
and next to the CHANGELOG.md of every touched component; READMEs of the touched modules;
`docs/spec/01-product.md`, 03, 04, 05 (generated), 06 (generated), 20; `docs/design/components.md`
7 and `docs/design/sidebar-sessions.md` D-SB-7.

## Out of scope

- An order an owner sets for everyone (the global `sort_key` stays the administration's tool).
- Dragging a row into another parent; the order of text channel threads and direct messages.

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with the package or component as scope and a DCO sign-off on every commit (`git commit -s`).
- Never push to `main`, never tag; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the fragments and READMEs named in this brief.
- Tests bind no port and make no network call; time comes from the injected clock.
- Never lower a threshold, disable a gate, delete a test or raise `repo.max_file_kb`.
- This brief was written together with the change, in the manner of D-W-100 and D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Pure rules.** `sidebar_arrange.dart` holds the rules: `applyPersonalOrder` (rows not in the
   list first), `orderByActivity` (stable, newest first), `holdOrder`, `moveAmong` (a move among
   the rows on screen of a longer list) and the activity of a channel and a space.
   `selectSidebarThreads` takes the manual order of a forum (`manualOrder`), in which no held
   order applies, because the person's own drag must show at once.
2. **Drag widgets.** Channels and posts use `SidebarDragRow` (a `Draggable`, a
   `LongPressDraggable` on touch, and a `DragTarget` with the drop line); the spaces keep the
   reorderable list of M4-19, with the delayed drag start on touch. A level arranged by activity
   wraps its rows in `SidebarArrangedRow`, which shows the note on a drag attempt.
3. **One write per move.** A move writes the whole list of the level's rows the client holds in the
   new order (`SidebarWriter.orderChannels`, `SidebarWriter.orderThreads`), the local row at once
   and the outbox entry after it.
4. **Settings.** The three switches are personal settings (`SettingKeys.autoArrange*`), read by
   `sidebarArrangementProvider`; choosing a default deletes the key.

## Tests (write them first)

- `apps/client/test/sidebar/sidebar_arrange_test.dart`: the rules, with property tests.
- `apps/client/test/sidebar/sidebar_threads_manual_test.dart`: the manual order of a forum group.
- `apps/client/test/sidebar/sidebar_drag_test.dart`: drag at each level with the drop line, touch,
  other parents, automatic levels and the note, the menu and screen reader moves.
- `apps/client/test/sidebar/sidebar_ordering_test.dart`: over the local database: a channel and a
  post drag write the outbox and survive a restart, automatic levels order by activity, a new
  message moves its chat and forum to the top, the held order.
- `apps/client/test/settings/sidebar_arrange_settings_test.dart`: the switches.
- Server, DAO, migration and sync tests of the data layer.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test`
- `cd packages/local_store && dart test`, `cd packages/sync_engine && dart test`
- `make codegen-check`, `make lint`, `make fmt-check`, `make config-doc-check`, `make api-checks`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] Every level can be dragged and moved by the menu on the desktop and the phone, within its
      parent only.
- [ ] The order is kept across restarts and devices of the person and changes nothing for others.
- [ ] The three switches arrange their level by activity and disable its dragging with a note.
- [ ] Spec, API, READMEs, rulings and changelog fragments updated; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 6.16 or M4-51.
- Rulings in `docs/decisions.d/M4-51.md`; changelog fragments as in Scope.
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
