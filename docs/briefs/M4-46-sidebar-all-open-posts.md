# M4-46: Sidebar shows all open forum posts

Status: ready | Depends on: M4-45 | Milestone: M4

## Goal

After this ticket the sidebar lists every open post of a forum, as Discord does: under each forum all posts that are not done, ordered by last activity, newest first, with their unread, mention and run state markers. When a forum has more open posts than `ui.sidebar_forum_posts_visible` (15), the rest waits behind the row "Show more" above "All posts", which lists them in place and then reads "Show less"; the choice is kept on this device. Text channels keep the thread rule of 01 6.8. Why: owner decision of 2026-10-29. The rule of M4-40 showed an open post only when it was unread, had a running run or was active within `ui.sidebar_thread_window` (72h), at most `ui.sidebar_threads_per_channel` (8) per channel; after an import most open posts had no recent activity, and a forum with 24 open posts showed none of them.

## Read first

- `docs/PLAN.md`: section 10 rows M4-40, M4-45, M4-46.
- Dependencies: the header names M4-45 (schema version 8 of `local_store`); through it this ticket builds on M4-40 (thread groups in the sidebar) and M4-42 (all implied, so not in the header).
- `docs/spec/01-product.md`: 6.8 (all), 13 (R-PR-17, R-PR-23), 14 (`ui.sidebar_thread_window`, `ui.sidebar_threads_per_channel`, `ui.sidebar_forum_posts_visible`).
- `docs/spec/04-sync-protocol.md`: 6.3 item 4 (`active_threads` of the snapshot).
- `docs/design/sidebar-sessions.md` sections 3 and 6; `docs/decisions.d/sidebar-sessions.md` (D-SB-2 to D-SB-6).
- Code on the base branch: `apps/client/lib/sidebar/` (`sidebar_threads.dart` `selectSidebarThreads`, `providers.dart` `ChannelThreads`, `channel_entry.dart`, `thread_entry.dart`, `sidebar_model.dart`), `packages/design_system/lib/src/components/ds_sidebar_thread_row.dart`, `packages/local_store/lib/src/tables/local_only.dart` (`channel_view_states`), `packages/local_store/lib/src/daos/forum_view_dao.dart`.

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
9. Limits only through named keys from `AppConfig`, never a literal in code. The one new configuration key is `ui.sidebar_forum_posts_visible` (place App); no new personal setting key (01 11 is normative and the server rejects unknown keys).
10. No GUI test on a maintainer's own machine; widget tests run headless.

Product requirements implemented here:

11. Under each forum every open (not done) post shows; a done post only while it is selected.
12. The rows of a forum are ordered by last activity, newest first; unread, mentions and the run state icon stay on the rows.
13. With more open posts than `ui.sidebar_forum_posts_visible` (default 15) the group shows the newest that many and the selected post, and a row "Show more" ("Mehr anzeigen") above "All posts" lists every open post in place; it then reads "Show less" ("Weniger anzeigen"). The state is kept per device.
14. "All posts" stays and keeps the counters of every post the group does not show.
15. Text channels keep their thread rule (`ui.sidebar_thread_window`, `ui.sidebar_threads_per_channel`, priority, order by creation).
16. No row moves under the pointer: while the pointer or the focus is in the sidebar the rows of a forum keep their order.
17. The two existing keys are renamed or redefined cleanly in the spec and in the configuration reference, with a migration of stored configuration if there is any.

## Scope

- `apps/client/lib/sidebar/`: the forum rule of the selection, the "Show more" state in the provider, the row in the channel entry and the callback through the tree.
- `packages/design_system`: the row `DsSidebarMoreRow` next to `DsSidebarAllPostsRow`.
- `packages/local_store`: the column of `channel_view_states` with a new schema version and its migration step, the DAO methods to watch and set it; the stored schema and the generated schema helpers of `make-migrations`.
- `packages/domain`: the regenerated `AppConfig` key table.
- ARB keys in `packages/l10n` for every new text.

Fragments: `docs/decisions.d/M4-46.md` (`D-M4-46-<n>`), changelog fragment `changelog.d/M4-46.md`; README of `apps/client`, `packages/design_system` and `packages/local_store`; `docs/spec/01-product.md` 6.8, 13 (R-PR-23) and 14; `docs/operations/configuration.md` (generated); `docs/design/sidebar-sessions.md` and `docs/design/components.md` 7.

## Out of scope

- The snapshot rule of 04 6.3 item 4: the sidebar lists the open posts the client holds (the snapshot's `active_threads`, the posts that events brought and the pages of the forum overview). Sending every open post of every forum in each snapshot is a change of the sync contract and of the snapshot size and needs a decision of its own (D-M4-46-5).
- The forum overview D15 and its post list.
- Phone specific layout work beyond what the shared sidebar tree gives M1.

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with the package or component as scope and a DCO sign-off on every commit (`git commit -s`).
- Never push to `main`, never tag; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the fragments and READMEs named in this brief.
- Tests bind no port and make no network call; time comes from the injected clock.
- Never lower a threshold, disable a gate, delete a test or raise `repo.max_file_kb`.
- This brief was written together with the change, in the manner of D-W-100 and D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **One pure rule.** `selectSidebarThreads` keeps both rules: the forum rule (every open post, by activity, limit, "Show more" state) and the text channel rule of M4-40. Ties in activity are broken by creation time and ID, so the result never depends on the input order.
2. **Held order.** The `ChannelThreads` provider remembers the rows it showed last in their order; while the sidebar holds, those rows keep it and new rows come first. A change of the "Show more" state skips the hold once, because the person asked for it.
3. **Local view state.** The "Show more" state is a column of `channel_view_states` next to the collapse state; setting one never changes the other.
4. **Keys.** App keys are compiled defaults of `AppConfig` with no stored copy, so redefining `ui.sidebar_thread_window` and `ui.sidebar_threads_per_channel` as text channel keys needs no migration.
5. **Rulings for `docs/decisions.d/M4-46.md`**: the forum rule and its order; the limit key, the row and where its state lives; the held order; the counters; the snapshot rule.

## Tests (write them first)

- `apps/client/test/sidebar/sidebar_threads_test.dart`: every open post of a forum shows however old, read and idle, a done one only while selected; order by activity with the creation tie break; markers stay; above the limit the newest show and the rest is counted in `more`; "Show more" lists all and offers "Show less"; at most the limit has no row; the selected post beyond the limit; a forum with 24 idle posts shows 15 and 9 behind "Show more"; a kept post; the held order; text channels have no such row; the text channel rule unchanged; property tests over random forums.
- `apps/client/test/sidebar/thread_group_test.dart`: in the sidebar over the local database a forum with 24 open idle posts shows 15 by activity, "Show more" between the last post and "All posts", lists all in place, the choice survives a restart, "Show less" goes back; "Show less" applies while the pointer is in the sidebar; a post that gets activity does not move under the pointer and moves after it leaves; an old read thread of a text channel stays hidden.
- `packages/design_system/test/components/sidebar_rows_test.dart`: the row reads "Show more" or "Show less" with the count, is a button, toggles on tap, is no drag source, reads German.
- `packages/local_store`: the DAO test of the new state (default, per channel, collapse state kept both ways), the column list of `channel_view_states`, the migration test of the new version with kept rows.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test`
- `cd packages/design_system && flutter test`
- `cd packages/local_store && dart test`
- `make codegen-check`, `make lint`, `make fmt-check`, `make config-doc-check`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] A forum lists every open post by last activity with its markers; done posts leave the group.
- [ ] "Show more" and "Show less" work in place and the choice is kept per device across restarts.
- [ ] Text channels behave as before.
- [ ] Spec, configuration reference, READMEs, rulings and changelog fragment updated; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 01 6.8 or M4-46.
- Rulings in `docs/decisions.d/M4-46.md`; changelog fragment as in Scope.
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
