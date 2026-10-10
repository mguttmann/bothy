# M1-55: Threads

Status: ready | Depends on: M1-53 | Etappe: M1

## Goal

After this ticket threads are first-class objects of the `chat` module: a fixed ID, a gapless
thread sequence handed out under the thread row lock, a title with its source, done and reopen,
forum posts with tags and a forum list with a done section, tag filter and stable pages. The
operator of a thread is captured at creation and never changes. The `chat.Threads` port offers
`Create`, `AppendToThread` and `Info`, which M1-56 uses to send messages and to expose
`createThread`; this ticket enables the six read and state operations.

## Read first

- `docs/spec/03-datenmodell.md`: tables `threads`, `thread_tags`, `forum_tags`, `channels`;
  "Thread-Sequenz" (6.2), "Sperrreihenfolge" (6.3); R-DM-11, R-DM-12, R-DM-21; config table
  (`thread.title_max_chars`).
- `docs/spec/04-sync-protokoll.md`: "Thread-Sequenz" (4), "Umschlag (Envelope)", "Idempotente
  Schreibvorgänge" 5.2, "Zustellgarantie", "Reihenfolge", "Bearbeiten und Löschen" row
  "Thread inzwischen erledigt".
- `docs/spec/05-aktionen.md`: "Threads und Agenten-Sitzungen" rows `thread.read`,
  `thread.create`, `thread.title.update`, `thread.close`, `thread.reopen`, `thread.tags.set`;
  "Ziel einer Ressource"; rule 1.8; section 6 rows `thread.created`, `thread.updated`,
  `thread.closed`, `thread.reopened`.
- `docs/spec/09-agenten.md`: 3.1 "Bediener", 3.7 "Auto-Thread aus einer Kanal-Nachricht", 12
  "Auto-Titel" (placeholder rule, `409 title_manual`), test AG-19, config `thread.placeholder_titles`.
- `docs/spec/01-produkt.md`: 3.1 rows on threads and forums, 6.1 "Erledigt und Wiederöffnen".
- `docs/spec/06-api.md`: "Paginierung", "Nebenläufigkeit" points 2 and 3, endpoint table
  "Threads", "Fehler".
- `docs/spec/07-rechte.md`: 4.2 and 4.3, 7.4 rules 1 to 3; `docs/spec/19-sicherheit.md` SEC-IN-4,
  SEC-T07.
- `docs/api/openapi.yaml`: operationIds `listChannelThreads`, `getThread`, `setThreadTitle`,
  `closeThread`, `reopenThread`, `setThreadTags` (and `createThread`, enabled by M1-56);
  schemas `Thread`, `ThreadPage`, `ThreadCreate`, `ThreadState`, `ThreadTitlePut`, `ThreadTagsPut`,
  `ForumSort`, `BillingMode`, `Cursor`, `PageLimit`, events `ThreadCreatedEvent`,
  `ThreadUpdatedEvent`, `ThreadClosedEvent`, `ThreadReopenedEvent`.
- ADRs 0009, 0013, 0015, 0017, 0053.
- Code of merged tickets: `internal/chat` (M1-52, M1-53: the `threads` table, main threads,
  forum tags, `StructureSource` adapter), `internal/authz`, `internal/agents`
  (`authz.AgentPolicySource`), shared cursor codec if one exists (`grep -rn cursor_invalid
  services/server`).

## Fixed requirements that apply

Owner build rules (verbatim):

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments
   explain why, not what. API documentation is generated from code (OpenAPI from the routes,
   dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching documentation
   is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.

Note on rule 3: `docs/api/openapi.yaml` is the source; the server interface is generated from it
(ADR 0013) and `make codegen-check` keeps both identical.

Rules for every brief:

6. Test first: red, then green, then cleanup; keep the red output for the report.
7. No dashes U+2013 or U+2014 in any file you write.
8. White label: only the example names of the specification.
9. Limits and windows only through named configuration keys.

Product requirements implemented here:

10. Fixed thread ID that never changes (03 `threads`).
11. `thread_seq` is gapless per thread and handed out by `UPDATE threads ... RETURNING
    last_message_seq` under the row lock (03 6.2, 04 4).
12. Title with source `auto` or `manual`; a manual title is never overwritten automatically;
    placeholder titles at creation keep `auto`; the operator agent may only replace an `auto`
    title (09 12).
13. Done (`closed`) and reopen, explicit and implicit through a new message (glossary
    "Erledigt", 01 6.1).
14. Forum posts with tags; restricted tags need `manage`; a forum list with done section, tag
    filter, sort `last_activity` or `created`, cursor pages without totals (01 3.1, 06 6).
15. The operator of a thread (`kind = 'thread'`) is captured at creation and stays (R-DM-12); the
    main thread has none (R-DM-21).
16. Titles with control or bidi characters are rejected (SEC-IN-4, SEC-T07).

## Scope

Paths relative to `services/server/` unless they start with `docs/`:

1. `migrations/<next number>_chat_thread_tags.sql`: table `thread_tags` exactly as 03.
2. `internal/chat/domain`: `Thread`, `ThreadKind`, `ThreadState` transitions, `TitleSource`,
   placeholder detection, title validation, tag set diff.
3. `internal/chat` (port): the `Threads` port with these methods and DTOs:

```go
// Create inserts a thread of kind "thread" in a text or forum channel, captures the effective
// operator of the channel, sets tags and appends thread.created. It authorizes thread.create
// (and thread.tags.set for tags) itself. It must run inside the caller's unit of work.
Create(ctx, CreateThreadCmd) (Thread, error)
// CreateThreadCmd{ChannelID uuid; Parent *ParentMessage; Title *string; TagIDs []uuid}
// ParentMessage{MessageID, ThreadID uuid}: verified by the caller (messaging) to exist and be
// undeleted; Create checks that ThreadID is the main thread of ChannelID.

// AppendToThread hands out the next thread_seq under the row lock and, when the thread is
// closed and ReopenIfClosed is set, reopens it and appends thread.reopened. The caller must
// already have authorized message.create on the thread in the same unit of work.
AppendToThread(ctx, AppendCmd) (Appended, error)
// AppendCmd{ThreadID uuid; ReopenIfClosed bool}
// Appended{Seq int64; Reopened bool; Thread ThreadInfo}

// Info returns routing and state data of a thread without authorizing; callers authorize
// before they reveal anything derived from it.
Info(ctx, threadID uuid.UUID) (ThreadInfo, error)
// ThreadInfo{ID, ChannelID, SpaceID, ChannelKind, Kind, State, OperatorAgentID?, CreatedBy,
//            Version, ChannelArchived bool}

List(ctx, ListThreadsQuery) (ThreadPage, error)
Get(ctx, threadID uuid.UUID) (Thread, error)
SetTitle(ctx, SetTitleCmd) (Thread, error)       // expected version from If-Match
Close(ctx, threadID uuid.UUID) (Thread, error)
Reopen(ctx, threadID uuid.UUID) (Thread, error)
SetTags(ctx, SetTagsCmd) (Thread, error)          // optional expected version
```

4. `internal/chat/app`: the use cases behind the port.
5. `internal/chat/adapters/postgres`: `queries/threads.sql`, `queries/thread_tags.sql`,
   repository; `DeleteForumTag` of M1-53 extended to delete the `thread_tags` rows of the tag in
   the same transaction.
6. `internal/chat/adapters/httpapi`: handlers `listChannelThreads`, `getThread`, `setThreadTitle`,
   `closeThread`, `reopenThread`, `setThreadTags`.
7. The chat `StructureSource` adapter: thread part (D8).
8. `internal/authz`, only where D8 finds a missing requirement check, with tests.
9. `api/operations.yaml`: enable the six operations above (not `createThread`, see "Out of
   scope"); regenerate.
10. `internal/platform/config` schema: `thread.title_max_chars` (03) and
    `thread.placeholder_titles` (09) if missing; `api.page_size_default`, `api.page_size_max` (06)
    if missing.
11. `docs/api/openapi.yaml` already has the `409` responses of `setThreadTitle` and `closeThread`
    and `title_manual` in `x-problem-codes` (planning, spec first); the generated table in 06
    "Fehler", the `Conflict` description and `getProblemType` include it.
12. Documentation: `internal/chat/README.md`, `services/server/CHANGELOG.md`,
    `docs/DECISIONS.md` (rulings D1 tags at creation, D6 unknown tag filter, D7).

## Out of scope

- The `createThread` operation and its handler: M1-56. The request carries a first message, and
  only `messaging` may compose `chat` with messages (02 4.4: `messaging` uses `chat`, not the
  reverse). This ticket builds `Threads.Create` and tests it through the port.
- Messages, the system line on implicit reopen, `MessagePosted`: M1-56.
- `moveThread`: M1-62. Auto thread on agent trigger (09 3.7) and agent sessions: M2.
- Search columns `threads.search_language`, `title_search`: M3-13.
- Read states and unread counters: M1-61. Typing and focus: M1-66.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-55-chat-threads`, created from the base
  branch the launcher names. One writer per worktree.
- Conventional Commits with a module scope (`feat(chat): ...`), every commit with DCO sign-off
  (`git commit -s`). Squash to meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope list, except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the module README named above.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL 18 through `pgtest` on a random port; a
  missing container runtime fails the test (P-09).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end by their literal path.
- Never read or print secrets or `.env` files. No network calls except package downloads for
  pinned tools.
- Do not lower a threshold, disable a gate, skip or delete a test.

## Design notes

D0. Start check. `make check` green on the fresh worktree, else `Verdict: BLOCKED base branch red`.
M1-53 merged (the `threads` table exists), else `Verdict: BLOCKED dependency M1-53 missing`.

D1. `Threads.Create` (called by `messaging` only, 02 4.6: the action `thread.create` belongs to
`messaging`, which authorizes it on the channel before the call; `chat` writes the row without a
rights check of its own):
- Unknown channel `404` (the caller has already turned an invisible channel into `404` through
  `authorize`).
- Forum channel: `Parent` must be nil (`400`, `/parent_message_id`, `not_allowed_in_forum`).
  Text channel: `Parent` is required (`400`, `/parent_message_id`, `required_in_text_channel`)
  and `Parent.ThreadID` must be the main thread of this channel (`400`, `/parent_message_id`,
  `not_in_main_thread`); `TagIDs` must be empty (`400`, `/tag_ids`, `tags_only_in_forum`).
- A second thread for the same parent violates the unique index on `parent_message_id`; map it
  to `409 conflict`.
- Title: rune count at most `thread.title_max_chars` (`400`, `/title`, `too_long`), no control or
  bidi characters (shared validator `kernel/textcheck` of M1-26, SEC-T07). Source: a human with a title that is neither empty nor a
  placeholder gives `manual`; empty, absent or placeholder gives `auto`; agents and apps always
  give `auto`. The title text is stored as given.
- Placeholder rule (09 12 point 2): trim, collapse inner white space to one space, compare
  case-insensitively (Unicode simple folding, `strings.EqualFold`) with each entry of
  `thread.placeholder_titles`; for a non-empty entry also accept the entry followed by one space
  and one or more ASCII digits ("Neuer Chat 3"). The empty entry matches only the empty title.
- `kind = 'thread'`, `state = 'open'`, `operator_agent_id` = effective operator of the channel
  now (space default when inheriting), NULL when none; `parent_message_id` from `Parent`;
  `created_by` = actor.
- Tags (forum only): same rules as `SetTags` (D5); when any tag is restricted, authorize
  `thread.tags.set` with the restricted flag for the new thread (descriptor with its known
  channel path, D8). Ruling: 05 names the restricted rule for `thread.tags.set`; it applies to
  tags given at creation as well, otherwise creation would bypass it.
- Appends `thread.created` with payload `{thread}`.

D2. `AppendToThread`: inside the ambient unit of work only (error otherwise). `SELECT ... FOR
UPDATE` the thread row, then `UPDATE threads SET last_message_seq = last_message_seq + 1,
last_activity_at = now(), message_count = message_count + 1, version = version + 1` and, when
the thread was closed and `ReopenIfClosed` is set, `state = 'open', closed_at = NULL,
closed_by = NULL`, `RETURNING last_message_seq`. On reopen authorize `thread.reopen` for the actor
and append `thread.reopened` (payload `{thread}` after the change). A rolled back transaction
returns the number with it; there are no gaps (03 6.2). The thread lock is taken before the
`event_seq` lock (04 4); the unit of work already guarantees that order.

D3. Close and reopen (explicit):
- `closeThread`: action `thread.close` (`write`). Main thread: `409 invalid_state_transition`.
  Already closed: `200` with the thread, no event, no version change. Else `state = 'closed'`,
  `closed_at = now()`, `closed_by` = actor, version plus one, `thread.closed`.
- `reopenThread`: action `thread.reopen` (`write`). Open thread: `200`, no event. Else open it,
  clear `closed_at` and `closed_by`, version plus one, `thread.reopened`.
- Both accept `Idempotency-Key` (M1-20 middleware).

D4. Title (`setThreadTitle`, action `thread.title.update`, `If-Match` required):
- `428` without `If-Match`, `412 version_conflict` with `current` on mismatch.
- Main thread: `400` (`/title`, `main_thread_has_no_title`).
- Validation as D1. A human sets `title_source = 'manual'`. The operator agent of the thread
  (fixed operator for `kind = 'thread'`) may set the title only while the source is `auto`; the
  source stays `auto`; with `manual` the answer is `409 title_manual` (AG-19). Who may call is
  decided by `authorize` (write and creator, or `manage`, or operator agent); never by chat code.
- Unchanged title and source: `200`, no event. Else version plus one and `thread.updated`.

D5. Tags (`setThreadTags`, action `thread.tags.set`, `If-Match` optional, 06 8.3):
- Only `kind = 'thread'` in a forum channel (`400`, `/tag_ids`, `not_a_forum_post`).
- Duplicates in `tag_ids` give `400` (`/tag_ids`, `duplicate`); every ID must be a non-deleted tag
  of the thread's channel (`400`, `/tag_ids`, `unknown_tag`).
- Compute the difference to the current set; if any added or removed tag is `restricted`, the
  request needs `manage`: pass the restricted flag in the resource descriptor so that `authz`
  requires `manage` (`403 forbidden`, reason `capability_missing`). No rights logic in chat.
- Same set: `200`, no event. Else replace rows, version plus one, `thread.updated`.
- Deleting a forum tag (M1-53 `DeleteForumTag`) now deletes its `thread_tags` rows in the same
  transaction, without per-thread events; clients reload tags on `channel.updated` (ruling of
  M1-53) and drop unknown tag IDs.

D6. List (`listChannelThreads`, action `thread.read` against the channel as container):
- Invisible channel `404`. Text channels list threads of `kind = 'thread'` only (the main thread is
  reached through `Channel.main_thread_id`); forum channels list posts.
- Filters: `state` (`open` or `closed`), `tag_id` (a tag of this channel; an unknown tag gives an
  empty page, not an error, to avoid a probe), `sort` (`last_activity` or `created`; default the
  channel's `forum_default_sort`, else `last_activity`).
- Keyset pagination: `last_activity`: `(last_activity_at DESC, id DESC)`; `created`:
  `(created_at DESC, id DESC)`. `limit` default `api.page_size_default`, capped at
  `api.page_size_max` (06 6 point 3). `next_cursor` absent on the last page. The cursor is opaque,
  versioned and bound to channel, filters and sort; a cursor from another query gives
  `400 cursor_invalid`. Reuse the existing cursor codec; if none exists, implement it in
  `internal/chat/adapters/httpapi` (decoding is an adapter concern) and document it in the README.
- No totals. Rows are filtered by `authorize` per item as 07 7.4 rule 3 requires.

D7. Representation (`Thread` schema): all stored fields; `tag_ids` = non-deleted tags, ordered by
tag `sort_key`; for `kind = 'thread'` with an operator: `operator_agent_id` and `billing_mode`
from `authz.AgentPolicySource` (chat may use `authz`, 02 4.4; the implementation from `agents` is
injected in `internal/wire`); for main threads both fields are absent and clients use
`Channel.operator.effective_agent_id` (ruling). `unread_floor_seq` only when not 0. No JSON
`null`. `ETag: "<version>"` on `getThread` and `setThreadTitle`.

D8. `StructureSource`, thread part: resolve a thread ID to `{channel_id, space_id, kind,
created_by, operator_agent_id, channel archived or read_only}` in the ambient transaction when one
exists. `authz` needs `created_by` and `operator_agent_id` for the requirement of
`thread.title.update`, the operator rule (thread fixed operator) and the restricted flag of
`thread.tags.set`. If `authz` lacks any of these checks, add them to its requirement step with
table tests (07 4.2 step 6).

D9. Events: scope `{space_id, channel_id, thread_id}`, resource `{thread, id, version}`, payload
`{thread}`. No event when nothing changed.

## Tests (write them first)

Unit (`internal/chat/domain`):
- `TestPlaceholderTitle`: table with every default entry of `thread.placeholder_titles`, case and
  white space variants, entry plus number, the empty entry with a number (not a placeholder), a
  real title.
- `TestTitleLength_CountsRunes` and `TestSEC_T07_ThreadTitleRejectsControlAndBidi`.
- `TestThreadStateTransitions`: close open, close closed (no-op), reopen closed, reopen open
  (no-op), close main (rejected).
- `TestTagDiff_RestrictedFlag`.

Integration (PostgreSQL 18, real modules; responses checked by the response validator):
- `TestThreadsCreate_ForumPostTitleSource`: human title gives `manual`; empty, "New chat" and
  "Neuer Chat 3" give `auto`; app-created thread gives `auto`; `thread.created` event.
- `TestThreadsCreate_FromMainThreadMessage`: parent in the main thread accepted; parent thread of
  another channel `400`; second thread for the same parent `409`; text channel with tags `400`;
  forum with parent `400`.
- `TestR_DM_12_OperatorFixedAfterChannelChange`: thread created while Lotse is the effective
  operator; then change the channel operator (through the chat repository in the test, because
  M1-54 may not be merged); `operator_agent_id` of the thread stays Lotse and `getThread` returns
  it with Lotse's `billing_mode`.
- `TestAppendToThread_GaplessUnderConcurrency`: many goroutines append to one thread, a random
  subset rolls back; committed `thread_seq` values are exactly 1..n.
- `TestAppendToThread_ReopensClosedThread`: closed thread with `ReopenIfClosed` gives
  `Reopened = true`, state `open`, one `thread.reopened` event; without the flag the state stays.
- `TestAppendToThread_RequiresUnitOfWork`.
- `TestCloseReopen_EventsAndNoOps` and `TestCloseMainThread_409`.
- `TestSetThreadTitle_IfMatchAndSource`: `428`, `412` with `current`, human gives `manual`.
- `TestAG_19_OperatorAgentTitleOnManual409`: operator agent on `auto` succeeds and keeps `auto`;
  on `manual` gets `409 title_manual`; a non-operator agent with `write` gets `403`.
- `TestSetThreadTags_Rules`: restricted tag without `manage` `403`, with `manage` `200`; tag of
  another forum `400`; duplicates `400`; text channel thread `400`; same set no event.
- `TestDeleteForumTag_RemovesThreadTags`.
- `TestListChannelThreads_FiltersSortAndPages`: done section through `state=closed`, tag filter,
  both sorts, pages stable while new threads are created between page requests, cursor reused with
  another filter gives `400 cursor_invalid`, `limit` above `api.page_size_max` is capped, main
  thread not listed.
- `TestGetThread_ForeignAndUnknownIdentical404` (AZ-P4 style, ignoring `correlation_id` and
  `instance`).
- `TestThreadWrites_ArchivedChannel403`: close, title and tags in an archived channel give `403
  channel_read_only` with reason `channel_archived`.

Commands that must pass:

```
cd services/server && go test -race ./internal/chat/... ./internal/authz/...
make codegen-check
make arch
make api-lint
make api-checks
make register-check
make config-doc-check
make migrations-lint
make check
```

## Acceptance criteria

- [ ] `thread_tags` exists as in 03; the six operations are enabled; generated code has no drift.
- [ ] `Threads.Create`, `AppendToThread` and `Info` exist with doc comments and are tested through
      the port; `createThread` is not enabled (M1-56 does it).
- [ ] `thread_seq` is gapless under concurrency and rollbacks.
- [ ] Title source rules hold, including placeholders and `409 title_manual` (AG-19).
- [ ] The thread operator never changes after creation (R-DM-12); main threads carry none.
- [ ] Close, reopen, title and tag changes emit exactly one event each and none for no-ops.
- [ ] Forum list filters, sorts and pages as D6, without totals, with `404` for invisible channels.
- [ ] `title_manual` is in the generated problem table and the openapi responses of
      `setThreadTitle` and `closeThread` list `409`.
- [ ] `make check` is green.

## Definition of done

- Code, tests and documentation in the same change: `internal/chat/README.md` current (threads
  port, tables, actions, events), doc comments on every public symbol, comments explain why.
- No architecture decision beyond the rulings; otherwise an ADR (MADR 4, next free number).
- `services/server/CHANGELOG.md` entry under "Unreleased"; rulings in `docs/DECISIONS.md`.
- `make check` green on the final commit.
- Local review (`make review` plus a code review over the diff); red team by a fresh read-only
  reviewer with `docs/quality/red-team-gate.md` over `git diff <base>...HEAD`; record the verdict
  line; on `NEIN` fix, test, re-check the delta.
- Merge locally into the base branch only with clean review, `Urteil: JA` and green
  `make check`; then remove the worktree.

## Report

Write `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits and files with line counts.
3. Test results with counts and the red output of the first failing tests.
4. Review and red team verdicts.
5. What is not proven (for example the HTTP path of `createThread`, which M1-56 enables).
6. Only real owner decisions, each with a recommendation.

Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
