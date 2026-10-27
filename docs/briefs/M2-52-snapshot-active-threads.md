# M2-52: Active threads in the snapshot and the project mark of a forum

Status: ready | Depends on: M2-03 | Milestone: M2

## Goal

After this ticket the snapshot (`ready` and `getSnapshot`) carries `active_threads`: per visible channel the thread objects of every open thread with an active run, of every open thread in the caller's `unread_threads`, plus the open threads whose `last_activity_at` lies within `sync.snapshot_thread_window`, newest activity first, until the channel has `sync.snapshot_threads_per_channel` entries (threads with an active run and unread threads count toward the bound but are never left out). The API document `Channel` carries `project_bound`, true exactly when the forum has a project binding, and creating or deleting a project binding raises `channels.version` and writes `channel.updated` in the same transaction. Why: the owner wants the sessions of a project in the sidebar as in Discord (2026-10-27); the apps can show the open posts of every channel right after connecting only if the snapshot names them, and they can say "project" and "New chat" only if the channel says it is bound ([design note](../design/sidebar-sessions.md), D-SB-10, D-SB-11).

## Read first

- `docs/design/sidebar-sessions.md` (all), `docs/decisions.d/sidebar-sessions.md` (`D-SB-1` to `D-SB-13`).
- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 row M2-52 (this ticket) and M2-03 (what exists); section 10 rows M4-40 and M4-41, section 11 row M5-15 (who builds on it).
- Dependencies: the header names M2-03. Through it this ticket builds on M1-63 (snapshot and catch-up), M1-55 (threads), M1-53 (channels) and everything below them (all implied, so not in the header).
- `docs/spec/04-sync-protocol.md` (core contract, read only): 6.3 item 4 (snapshot with `active_threads`), 8.1 (visibility), 17 (R-SY-28), 18 (`sync.snapshot_thread_window`, `sync.snapshot_threads_per_channel`).
- `docs/spec/03-data-model.md` (core contract, read only): `threads` (index `(channel_id, state, last_activity_at DESC)`), `runs` (active states), `channels.version`, `project_bindings` and the paragraph on `project_bound`, R-DM-54.
- `docs/spec/05-actions.md` (core contract, read only): event table row `channel.updated` (`project_bound`), actions `channel.project.bind`, `thread.read`, `sync.snapshot.read`.
- `docs/spec/01-product.md`: 6.8 (what the apps do with the data), 13 R-PR-17.
- `docs/api/openapi.yaml`: schemas `Snapshot`, `Thread`, `Channel`, `ChannelView`; operations `getSnapshot`, `putProjectBinding`, `deleteProjectBinding`, `getChannel`, `listSpaceChannels`; `docs/api/asyncapi.yaml` frame `ready` and event `channel.updated`.
- `docs/decisions.d/M1-63.md` (snapshot reads) and `docs/decisions.d/M2-03.md` (binding writes), where present.
- Briefs: `docs/briefs/M1-63-sync-catch-up.md` (snapshot assembly), `docs/briefs/M2-03-project-binding.md` (binding use cases); merged code wins on names.
- Code on the base branch: `services/server/internal/sync/app/snapshot.go` (`SnapshotDeps`, ports `SnapshotStructure`, `ThreadCounters`, `RunSummarySource`), `services/server/internal/sync/adapters/httpapi/sync.go`, `services/server/internal/chat/app/project_binding.go`, `services/server/internal/chat/app/threads.go`, `services/server/internal/chat/app/channels.go`, the module wiring files `services/server/internal/wire/module_chat.go` and `module_sync.go` (PL/28).

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) already holds this ticket's changes and is not edited by it.

Product requirements implemented here:

11. `Snapshot.active_threads` is an array of `Thread` (the document of `getThread`, without messages), required, empty when nothing qualifies; it holds only threads with `kind = 'thread'` and `state = 'open'` of channels in `Snapshot.channels` (04 6.3 item 4, 8.1).
12. Per channel: every open thread with a run in a state that `listActiveRuns` counts as active, every open thread that is in `Snapshot.unread_threads` of the same snapshot (unread messages or mentions of the caller, whatever its age), plus the open threads with `last_activity_at` not older than `sync.snapshot_thread_window` before the server time of the snapshot, in descending `last_activity_at`, until the channel has `sync.snapshot_threads_per_channel` entries; a thread with an active run or in `unread_threads` is never left out, even when these alone exceed the bound; no thread appears twice (R-SY-28).
13. `ready` and `getSnapshot` give the same `active_threads` for the same state; the read happens in the same snapshot read as the other members, so a thread event after `head_seq` is never missing from both snapshot and live delivery (04 6.3 item 4).
14. `Channel.project_bound` is a required boolean in every `Channel` the API returns (`getChannel`, `listSpaceChannels`, the snapshot, events that carry a channel): true exactly when a `project_bindings` row exists for the channel; never stored (03, R-DM-54).
15. `putProjectBinding` that creates a binding and `deleteProjectBinding` that removes one raise `channels.version` by one and write one `channel.updated` in the same transaction; a `putProjectBinding` that changes an existing binding, an identical PUT and a readiness report write no `channel.updated` and do not change `channels.version` (03, 05 `channel.updated`).
16. No new action, permission rule, event type, gateway op or table; reading needs `sync.snapshot.read` as before, visibility follows the channel's `thread.read` (05).

## Scope

Server (`services/server/`):

- `internal/chat` (names of the merged code win): a read `ActiveThreads(ctx, channelIDs, window, perChannel, now)` on the threads port that returns the rows of requirement 12 in one query (for example a `LATERAL` join per channel over the index `(channel_id, state, last_activity_at DESC)` united with the threads of active runs); `project_bound` in the channel view (a join or `EXISTS` on `project_bindings` in the channel reads); the version step and the `channel.updated` in the create and delete paths of the binding use cases.
- `internal/sync/app/snapshot.go`: a port for the active threads in `SnapshotDeps`, the configuration keys `sync.snapshot_thread_window` and `sync.snapshot_threads_per_channel` in the server configuration schema with the defaults of 04 18, and the member `active_threads` in the snapshot document; wire it only in the module files of `internal/wire` (PL/28).
- The active runs per thread come from the existing runs port of the snapshot (`RunSummarySource`) or a narrow port of the runs module; no cross access into tables of `runs`.
- `docs/api/openapi.yaml`: `Snapshot.active_threads` (required array of `Thread`, description as 04 6.3 item 4), `Channel.project_bound` (required boolean, description as 03), examples updated; `docs/api/asyncapi.yaml` where it repeats these schemas; regenerate every generated client and server part (each component's `generate` target; `make codegen-check` and `make api-clients` prove it).
- Configuration reference: `make config-reference` after the schema change.
- READMEs of `internal/sync` and `internal/chat` (what the snapshot holds, when `channel.updated` follows a binding change).

Fragments: `docs/decisions.d/M2-52.md` with rows `D-M2-52-<n>`; `changelog.d/M2-52.md` at the root.

## Out of scope

- The sidebar, the forum overview and the new chat view (M4-40, M4-41, M5-15); the Dart client of the new members is regenerated here, its use in the apps is not.
- Any change to the Claude Code runner, its working folder or its readiness (D-SB-12, 09 18.1).
- A synchronized collapse or pin state (D-SB-6) and any new personal setting key.
- Changes to the core contract (already made by the design change).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-52-snapshot-active-threads`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(sync): ...`, `feat(chat): ...`, `docs(api): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the README, fragment and generated files named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; database tests use one database per test from `pgtest`; time comes from the injected clock, never from a fixed sleep.
- Never read or print secrets. Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M2 closed, in the manner of D-W-100. Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **One query, bounded.** The active threads are read once per snapshot for all visible channels, not per channel in a loop: a `LATERAL` subquery per channel with `ORDER BY last_activity_at DESC LIMIT sync.snapshot_threads_per_channel` over the existing index, united with the open threads that have an active run and the open threads of the caller's `unread_threads` (the same read state computation, not a second one), deduplicated, sorted by channel and descending activity. Check the plan with `EXPLAIN` in a test on a seeded database (index scan on `threads`, no sequential scan of all threads); name the result in the report.
2. **Active run.** "Active" means the states that `listActiveRuns` returns (09 5.1); take the list from the runs module through its port so both reads agree; a thread whose run ended between the two reads is still correct with its activity rule.
3. **Same read.** The active threads belong to the same snapshot read as the channels, counters and runs (the transaction or read view the merged snapshot use case already has), so 04 6.3 item 4 ("every earlier event is in the snapshot or in the catch-up") holds for thread events.
4. **`project_bound` without a column.** Derive it in the channel reads; the binding use cases already lock the channel or the binding row; the version step uses the same compare-and-swap as other channel changes, and the `channel.updated` carries the new version (05). An identical PUT stays a no-op (M2-03).
5. **Rulings for `docs/decisions.d/M2-52.md`** (`D-M2-52-<n>`): the query shape and its plan; where the active run states come from; the version step on binding create and delete only.

## Tests (write them first)

Server (`services/server/internal/chat`, `internal/sync`, with `pgtest` and an injected clock):

- `TestActiveThreads_BoundAndOrder`: a forum with more open threads within the window than `sync.snapshot_threads_per_channel` yields exactly the bound, newest activity first.
- `TestActiveThreads_ActiveRunAlwaysIncluded`: an open thread with an active run and activity older than the window is included, also when the channel already has the bound of newer threads.
- `TestActiveThreads_UnreadAlwaysIncluded`: an open thread with an unread mention of the caller and one with only unread messages, both older than the window and with the oldest activity of a channel that already has the bound of newer threads, are both included; for another person who has read them they are absent.
- `TestActiveThreads_Excludes`: done threads, main threads, threads older than the window without an active run, and threads of channels the caller may not read are absent.
- `TestActiveThreads_QueryPlan`: the query uses the index on a seeded database (Design note 1).
- `TestSnapshot_ActiveThreadsSameInReadyAndRest`: `ready` over the gateway harness and `getSnapshot` return the same `active_threads`.
- `TestSnapshot_ThreadEventAfterHeadSeq`: a thread created right after the snapshot read arrives as `thread.created` after `head_seq`, never lost and never only in neither.
- `TestProjectBinding_CreateDeleteWritesChannelUpdated`: create raises `channels.version` and writes one `channel.updated` with `project_bound` true; delete the same with false; a changed binding, an identical PUT and a readiness report write none.
- `TestChannel_ProjectBoundInEveryRead`: `getChannel`, `listSpaceChannels` and the snapshot carry `project_bound` for a bound and an unbound forum and for a text channel (false).
- Contract tests of `Snapshot.active_threads` and `Channel.project_bound` against `openapi.yaml`.

Run the added tests by name with repetitions as the team rules require (Go `-race -count=20`), through the machine's test slot.

Commands that must pass, from the repository root:

- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `make coverage`
- `make api-lint`, `make api-checks`, `make codegen-check`, `make api-clients`
- `make docs`, `make fragments-check`
- `make check`

## Acceptance criteria

- [ ] The snapshot carries `active_threads` with the selection, bound and order of 04 6.3 item 4; threads with an active run and threads in `unread_threads` are never left out; `ready` and `getSnapshot` agree.
- [ ] No thread event between snapshot and live delivery is lost.
- [ ] `Channel.project_bound` is in every channel read; creating and deleting a binding raise the version and write one `channel.updated`; other binding changes write none.
- [ ] `Snapshot.active_threads` and `Channel.project_bound` are in `openapi.yaml` and every generated client; the configuration keys are in the schema and the configuration reference.
- [ ] Fragments written; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 04 6.3, 03 `project_bindings` or R-SY-28.
- READMEs of `internal/sync` and `internal/chat` updated.
- Rulings in `docs/decisions.d/M2-52.md`; changelog fragment as in Scope.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The query plan of Design note 1 and the size of the snapshot on the seeded database before and after.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
