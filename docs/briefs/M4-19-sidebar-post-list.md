# M4-19: Sidebar, spaces, channels and post list

Status: ready | Depends on: M4-18 | Milestone: M4

## Goal

After this ticket the first two columns of the desktop app show real data from the local database: the sidebar (01 5.1 D1, components.md 7) with the product mark and `branding.product_name`, the "Jump to" field and the "Running now" entry as placeholders that M4-23 wires, the spaces as collapsible tree nodes in the person's own order (collapse and drag-and-drop order synchronized across devices through `user_space_states`, 04 14), the channels of each space with their kind icon and the states of components.md 7 (read, unread, mentions, active run, muted, selected), unread and mention counters kept by the sync engine, and the own avatar with the settings entry at the bottom; and for forums the post list (components.md 6) with "New post", the filter chips Open, Waiting for me, Tags and Sort, rows with state icon, title, time, badges and tags, and the collapsed "Done (n)" section at the bottom (01 6.1). Selecting a channel opens the main thread (text channel) or the post list (forum) through the router of M4-18. Why: the sidebar is the entry to every conversation; its counters and states are what tells a person where agents work and where they are needed (01 1).

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-18, M4-19, M4-20, M4-23, M4-33, M4-36.
- `docs/spec/01-product.md`: 3.1 (structure and conversations), 4, 5.1 D1 and D2 (sidebar and post list columns), 6.1 "Done and reopening", 6.7 (operator in the channel header; costs), 7 (shortcuts named for the sidebar; M4-24 implements them), 8 (tree semantics, focus, text scaling), 11, 13 (R-PR-01).
- `docs/design/components.md`: 6 "Thread list with status (forum)", 7 "Space and channel tree", 13 (review list).
- `docs/spec/03-data-model.md`: `spaces`, `channels` (`kind`, `forum_default_sort`, `read_only`, `default_notification_level`, operator), `threads` (`state`, `last_activity_at`, `title_source`), `thread_tags`, `forum_tags`, `user_space_states`, `read_states`.
- `docs/spec/04-sync-protocol.md`: 9 item 3 (`sidebar.updated` and `read_state.updated` carry the version of the personal sub-object), 14 (sidebar: last write wins per space; unread counting between snapshots).
- `docs/spec/15-client-architecture.md`: 4 (`StreamProvider` per query), 6.1 (tables), 11 (paths `/s/`, `/c/`, `/t/`).
- `docs/api/openapi.yaml`: `getMySidebar`, `putMySidebarSpace` (`SidebarState`, `SidebarStatePut`), `listSpaces`, `listSpaceChannels`, `listChannelThreads` (filters and sort parameters), `listForumTags`, `createThread`.
- `docs/DECISIONS.md`: D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M4-18-desktop-shell.md` (columns, providers, router), `docs/briefs/M4-07-core-components.md` (`DsSpaceRowData`, `DsChannelRowData`, `DsThreadRowData`, rows), `docs/briefs/M4-08-domain.md` (`ForumOrder`, `UnreadCounter`), `docs/briefs/M4-12-local-store.md` (`StructureDao`, `ThreadsDao`, `PersonalDao`), `docs/briefs/M4-15-sync-outbox.md` (executor registry), `docs/briefs/M4-17-fake-server.md` (handler registry, `FakeServer`); for the server side `docs/briefs/M1-52-chat-spaces.md` (the fractional index `fracindex` with its normative vectors) and `docs/briefs/M1-56-messaging-send.md` (forum posts need a first message).

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
9. No new configuration key and no new personal setting key (01 11 is normative and the server rejects unknown keys).
10. Specification first: 01 6.1 says the collapse state of the Done section is "personal per forum" but 01 11 has no key for it; this ticket keeps it per device in the local database (Design note 4) and says so in 01 6.1 in the same change.

Product requirements implemented here:

11. Spaces collapsible and sortable per person; collapse and order are synchronized across devices through `putMySidebarSpace` and `sidebar.updated` (components.md 7, 04 14); the global order changes only in the administration.
12. Channel rows show read, unread (bold with a bar), mentions (counter), active run (dot), muted and selected; the label is "<Name>, <n> unread, <m> mentions, run active"; the tree has the roles and states of components.md 7.
13. Unread and mention counters come from the snapshot, `listMyReadStates` and `read_state.updated`, and between them from the client's own counting of `message.*` events (04 14); the app never derives them from grants.
14. The forum post list sorts by `last_activity_at` or creation (`forum_default_sort`), offers the filters Open, Waiting for me, Tags and Sort, and shows done posts in a collapsed "Done (n)" section in `color.text.muted` (components.md 6, 01 6.1).
15. Rows of the post list: state icon (active run, otherwise mention), title bold when unread, time, "Waiting for you", operator agent, mention counter, tags, failure reason; label "<Title>, <State>, <Time>, <n> unread" (components.md 6).
16. The sidebar shows `branding.product_name` and the logo of the instance, never a literal name.

## Scope

Create in `apps/client/lib/`:

- `sidebar/` (`sidebar_column.dart`, `space_tree.dart`, `space_node.dart`, `channel_entry.dart`, `sidebar_header.dart`, `sidebar_footer.dart`, `sidebar_order.dart` for drag and drop and the `sort_key` computation, Design note 2), `sidebar/providers.dart` (Riverpod providers over the DAOs: spaces with states, channels per space with counters and run state).
- `forum/` (`post_list_column.dart`, `post_list_header.dart` with the filter chips, `post_row.dart` mapping to `DsThreadRowData`, `done_section.dart`, `new_post_dialog.dart` for the title, first message and tags of a new post, `providers.dart`).
- `packages/domain/lib/src/rules/fractional_index.dart` with its tests (Design note 2).
- `sync/sidebar_ops.dart` (outbox executor for `putMySidebarSpace`) and `sync/thread_ops.dart` (executor for `createThread`), registered in the bootstrap of M4-18.
- Fake server handlers for `getMySidebar`, `putMySidebarSpace` (with the `fracindex` validation), `listForumTags`, `createThread` (with the forum rule) and `getBrandingLogo` added through the handler registry of M4-17 (in `packages/sync_engine/lib/src/testing/rest/`) if M4-17 does not have them.
- Tests under `test/sidebar/`, `test/forum/`, goldens under `test/goldens/sidebar/` and `test/goldens/forum/`, integration tests under `integration_test/` (Tests).
- ARB keys for every text of the sidebar and the post list in `packages/l10n`.

Change:

- `packages/local_store`: a local-only table `forum_view_states` (`channel_id`, `done_collapsed`) in the next free schema version with its migration test and the survival test of M4-12 extended, only if no existing local table fits (Design note 4).
- `docs/spec/01-product.md` 6.1 (the Done collapse state is kept per device), `apps/client/README.md` (sidebar and post list).
- `docs/DECISIONS.md` section 10: rows `D-M4-19-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The thread view, the main thread of a text channel and the message input (M4-20); the channel header with operator and topic (M4-20).
- "Running now", the quick switcher, search and mentions (M4-23); keyboard shortcuts for channel navigation (M4-24, which uses the focus and selection API of this ticket).
- The channel menu with notification level and export (M4-33); the group "Hosts" (M4-36); administration of spaces and channels (M6 console and M4-26 for people only).
- Direct messages (later, 01 4).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-19-sidebar-post-list`; one writer per worktree.
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

1. **Data path.** Providers read from `local_store` only (R-CL-03): `spacesWithStateProvider` joins `spaces` and `user_space_states` and sorts by the personal `sort_key` (absent: the global order of the space); `channelsOfSpaceProvider(spaceId)` with counters from `read_states` and the `UnreadCounter` values the engine stores, run state from `runs` with an active state, muted from `notification_preferences` (the channel's own row with level `mute` and `muted_until` absent or in the future; inherited levels are shown by M4-33); `postListProvider(channelId, filter, sort)` over `threads` and `thread_tags` with `ForumOrder` of M4-08. Mapping to the view models of M4-07 happens in small pure functions with unit tests.

2. **Personal order.** Drag and drop of a space computes a new `sort_key` with `KeyBetween(previous, next)` of the fractional index that the server validates (`fracindex` of M1-52: digits `0-9A-Za-z` in ASCII order, an integer part with a head letter, no trailing `0` in the fraction, the smallest integer invalid); implement it in `packages/domain` as `FractionalIndex.between` and test it with the normative vectors of M1-52 (`("", "") -> "a0"`, `("a0", "") -> "a1"`, `("", "a0") -> "Zz"`, `("a0", "a1") -> "a0V"`) and its property tests, so a key the client sends never fails `putMySidebarSpace` with `400`. The write goes to the outbox as `putMySidebarSpace` with `sort_key`; collapse as `putMySidebarSpace` with `collapsed`. The local row updates at once; the own echo is ignored by `updated_by_device_id`, and a later change from another device wins by version (04 14). Ruling for docs/DECISIONS.md: the shared key rule in `domain`.

3. **Post list.** Header: forum name, "New post" (primary) opening a dialog for the title, the first message and tags (`createThread` with `title`, `first_message` carrying its own `client_msg_id`, and `tag_ids`, through the outbox; a forum post without a first message is refused by the server, `ThreadCreate`), chips Open (default), Waiting for me (ruling for docs/DECISIONS.md: threads with a run that the person triggered in `waiting_input`, or an unread mention of the person), Tags (multi-select from `forum_tags`), Sort (last activity, creation; default from `forum_default_sort`). The list is virtualized; long forums load further pages through `listChannelThreads` when scrolled near the end (the cursor of the API), stored in `threads`. Done section: "Done (n)" with the count of closed threads matching the filters, collapsed by default.

4. **Done collapse per device.** The collapse state of the Done section is kept per forum on this device in a local-only table (`forum_view_states`) or in an existing local-only table that fits; it is not synchronized because 01 11 has no key for it. Ruling for docs/DECISIONS.md and a sentence in 01 6.1.

5. **Selection and router.** Clicking or pressing Enter on a channel navigates to `/c/<id>`; a text channel opens its main thread in the history column, a forum fills the post list column; a post navigates to `/t/<id>`. The selected row follows the route. A space row toggles with click, Enter, arrow left and right (components.md 7). The sidebar exposes `SidebarNavigation` (next or previous channel, next or previous unread channel) as a controller for M4-24.

6. **Rulings for docs/DECISIONS.md** (`D-M4-19-<n>`): data path and mapping (note 1); the order key in `domain` (note 2); "Waiting for me" (note 3); the Done state per device (note 4); paging of the post list (note 3).

## Tests (write them first)

Widget and golden tests (`flutter test`, in-memory `local_store` filled by fixtures, fake clock):

- `test/sidebar/space_tree_test.dart`: personal order from `sort_key` with fallback to the global order; collapse writes one outbox entry and updates the row at once; tree semantics (roles, expanded and collapsed states); the channel label with unread, mentions and run.
- `packages/domain/test/fractional_index_test.dart`: the normative vectors of M1-52, keys between neighbours, at the start and the end, validity, byte order, and repeated inserts at the end keep short keys.
- `test/sidebar/counters_test.dart`: a `message.created` from another person raises unread; one mentioning me raises mentions; `read_state.updated` resets them.
- `test/forum/post_list_test.dart`: sort by activity and creation; filters Open, Waiting for me and Tags; the Done section counts and collapses, and its state survives a restart on this device; row labels.
- `test/goldens/sidebar/*` and `test/goldens/forum/*`: the sidebar with three spaces in every channel state, and the post list with every row state and the Done section expanded, light and dark, densities `comfortable` and `compact`, text scaling 1 and 2, `de` and `en`.

Integration tests (`integration_test/`, against `FakeServer`):

- `sidebar_sync_test.dart`: two simulated devices of Alex: reordering and collapsing on one device appears on the other through `sidebar.updated` (personal order across devices); unread and mention counters follow messages from Sam.
- `forum_test.dart`: create a post, see it at the top, close it from the fake, see it move to Done; reopen through a new message.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Widget and integration tests with the fake server are green; the personal order and collapse state synchronize across two simulated devices.
- [ ] Spaces, channels and the post list show the states and labels of components.md 6 and 7; counters follow 04 14.
- [ ] The post list sorts, filters and shows the Done section as 01 6.1 says; its collapse state is kept per device and 01 6.1 says so.
- [ ] Goldens in light and dark, two densities, text scaling 1 and 2, `de` and `en`.
- [ ] Rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01, 04 or components.md.
- `apps/client/README.md` (English) as in Scope.
- 01 6.1 updated; rulings of Design note 6 in `docs/DECISIONS.md` section 10.
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
4. Where the integration tests ran, the order key algorithm with its length bound, and whether a local table was added for the Done state (with its schema version).
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
