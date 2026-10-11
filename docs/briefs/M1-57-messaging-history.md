# M1-57: Reading history

Status: ready | Depends on: M1-56 | Milestone: M1

## Goal

After this ticket a principal who may read a thread pages through its history by thread sequence
(newest page, `before_seq`, `after_seq`, `around_seq`) and loads any single message in full. Pages
are keyset pages over `thread_seq`: stable against new messages, contiguous, and with tombstones at
the positions of deleted messages, so a client recognises a real gap as missing data. Nothing leaks:
an invisible thread or message answers exactly like an unknown one, and every content query carries
the visible-channel predicate. Clients, the offline cache, and later the agent run context and the
search build on this read path.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 7 "M1 Foundation" (row M1-57,
  "Test strategy", "Red team").
- `docs/briefs/README.md` ("How a brief runs").
- `docs/spec/00-glossary.md`: Tombstone, Thread sequence, Message.
- `docs/spec/02-architecture.md`: "Modules of the monolith" (4.1 Rules, 4.3 Ports, 4.4 Allowed
  dependencies, 4.5 Cross-cutting ports), "Layers within a module", "Code layout", "Request path"
  (9.1 to 9.3), "Error and timeout strategy" (14.1).
- `docs/spec/03-data-model.md`: "Principles" (items 7, 12, 13), tables `messages` and `threads`
  (sections 4.5 and 4.6), "Transaction rules" 6.2.
- `docs/spec/04-sync-protocol.md`: "Envelope" (payload rule 2, `truncated`),
  "Thread sequence", "Delivery guarantee", "Ordering".
- `docs/spec/05-actions.md`: section 1 rules 1 to 3; "Target of a resource"; "Decision";
  table 4.6 (`thread.read`) and table 4.7 (`message.read`); section 9 item 4.
- `docs/spec/06-api.md`: "Basic rules", "Authorization in the API", "Errors", "Pagination"
  (items 2, 3, 5), "Concurrency" (item 1), "Endpoints" > "Messages", "Configuration values of
  this chapter".
- `docs/spec/07-permissions.md`: "The `authorize` function" (4.1, 4.2, 4.3), "Query pattern" (7.4,
  rules 1 and 3), "Test matrix" (AZ-M52) and "Required tests" (AZ-P4, AZ-P12).
- `docs/spec/19-security.md`: "Logs without secrets and without content" (SEC-LOG-3, SEC-LOG-4).
- `docs/api/openapi.yaml`: operations `listThreadMessages`, `getMessage`; schemas `Message`,
  `MessagePage`, `PageLimit`, `Problem`; parameters `ThreadIdPath`, `MessageIdPath`.
- ADRs in `docs/adr/`: `0008-hexagonal-modules-and-checked-dependency-rules.md`,
  `0013-spec-first-openapi-31-and-asyncapi-30-normative.md`,
  `0015-error-format-rfc-9457-idempotency-pagination.md`,
  `0017-one-authorization-function-404-instead-of-403-mandatory-registration.md`; ADR 0053 (no JSON
  `null`) once M1-16 has written it.
- Code of earlier tickets: `services/server/internal/messaging` (M1-56: tables, the `Message`
  representation, `createMessage`), the `chat` port `Threads` (M1-55), `internal/kernel/authorize`
  (`Authorizer`, `Scope`; M1-28), `internal/platform/httpserver` (problem format, ETag helper;
  M1-18), `internal/platform/pg/pgtest` (M1-12), `services/server/api/operations.yaml` (M1-17),
  and the integration test world of 07 "Examples" used by M1-54 and M1-56.
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/quality/red-team-gate.md`,
  `services/server/internal/messaging/README.md`.
- Tables: `messages` (read). Actions: `thread.read`, `message.read`.

## Fixed requirements that apply

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
6. Test first: write the failing test, record it red, then make it green (PLAN section 3 item 3).
7. No en dash or em dash characters in anything you write; use a hyphen or rephrase.
8. White label: no company, person, host or domain names; examples use Alex, Sam, Robin, the spaces
   Support, Marketing, Engineering, Research and `example.com`.
9. Limits and windows only through named configuration keys from
   `docs/operations/configuration.md`; prose and comments name the key, never the number.
10. History of one thread over `thread_seq`: at most one of `before_seq`, `after_seq`,
    `around_seq`; without any, the newest page; items ascending; tombstones included; the window
    rules of `around_seq` and the flags as in 06 "Pagination" item 5 (openapi
    `listThreadMessages`; 04 "Thread sequence").
11. Keyset pages over `thread_seq`: a new message never changes an existing page (06 "Pagination"
    items 2 and 5; ADR 0015).
12. A deleted message keeps its row and `thread_seq` and is returned as a tombstone without content
    (03 "Principles" item 7; 03 `messages`).
13. No read permission gives `404 not_found`, identical for foreign and unknown IDs in status, body
    and headers (05 rule 2; 07 4.2; 02 "404 instead of 403"; AZ-M52; AZ-P4).
14. Every SQL statement on messaging tables carries `channel_id = ANY($visible)` (directly or via a
    join to `messages`), and every returned element is checked again with `authorize` (07 7.4 rules
    1 and 3; preparation for AZ-P12).
15. Lists cut `content_md` above `message.list_content_max_bytes` at a UTF-8 character boundary and
    set `content_truncated: true`; `getMessage` always returns the full content and is the reload
    path for content cut in lists and events (06 "Pagination" item 5, 04 "Envelope" rule 2,
    `Message.content_truncated`). Reason: one page stays below `api.page_size_max` times that
    value.
16. `limit` defaults to `api.page_size_default`; larger values are capped at `api.page_size_max`,
    not rejected (06 "Pagination" item 3).
17. `getMessage` returns `ETag: "<version>"` (06 "Concurrency" item 1); no JSON `null` in any
    response (06 "Basic rules").
18. No message content, query strings or display names in logs or telemetry (SEC-LOG-3,
    SEC-LOG-4).

## Scope

All paths below `services/server/` unless stated. Follow the file and type naming M1-56
established inside `internal/messaging`; the names below are the required public names.

1. Port `internal/messaging` (interface `Messages`), new methods with doc comments:
    - `History(ctx context.Context, q HistoryQuery) (HistoryPage, error)`
    - `Get(ctx context.Context, messageID ids.ID) (MessageView, error)`
    - `HistoryQuery{ThreadID ids.ID; Mode HistoryMode; Anchor int64; Limit int}` with
      `HistoryMode` values `HistoryNewest`, `HistoryBefore`, `HistoryAfter`, `HistoryAround`;
      `Limit = 0` means `api.page_size_default`.
    - `HistoryPage{Items []MessageView; HasMoreBefore bool; HasMoreAfter bool}`.
    - `MessageView` is the message DTO M1-56 introduced; reuse it, do not add a second one.
      `ids.ID` stands for the UUID type of `internal/kernel/ids`. The acting principal comes from
      the context (`kernel/actor`), as in M1-56. Later callers: `runs` (run context, M2), `search`
      (M3).
2. `internal/messaging/domain`: pure functions for request validation and window selection
   (`ValidateHistoryRequest`, `PickAround`, `PageFlags`), see Design notes 3 and 4.
3. `internal/messaging/app`: use cases `ListThreadMessages` and `GetMessage`.
4. `internal/messaging/adapters/postgres/queries/history.sql` (sqlc, regenerated `sqlcgen/`):
   one statement per mode (`ListThreadNewest`, `ListThreadBefore`, `ListThreadAfter`,
   `ListThreadAround`) and `GetVisibleMessage` (reuse M1-56's lookup if it already carries the
   visible predicate).
5. `internal/messaging/adapters/httpapi`: strict-server methods `ListThreadMessages` and
   `GetMessage`.
6. `internal/messaging/adapters/postgres/predicate_test.go`:
   `TestQueriesCarryVisibleChannelPredicate` (Design note 7).
7. `api/operations.yaml`: move `listThreadMessages` and `getMessage` from `later` into the built
   list (format from M1-17); regenerate with the generator `make codegen-check` verifies.
8. Docs: `internal/messaging/README.md` (keep its English text and section structure; status,
   operations, queries), `services/server/CHANGELOG.md` (entry under `Unreleased`).

## Out of scope

- Sending messages, idempotency by `client_msg_id`, mention validation, event truncation (M1-56).
- Editing, deleting, revisions, redaction (M1-58). This ticket only renders tombstones correctly.
- Reaction summaries, `my_reactions`, `pinned` (M1-59). Render whatever the shared representation
  function produces on your base.
- Mention list, templates (M1-60); read states and unread counters (M1-61); thread move (M1-62);
  REST catch-up and snapshot (M1-63).
- Attachments (M3) and components (M2): keep the empty arrays the representation already emits.
- The repository-wide SQL predicate lint with exception list (M1-70); the full leak suite (M1-70);
  load tests (M1-72); the order binding of agents in a run, SR9 (M2).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-57-messaging-history`, created from the
  base branch the launcher names, after checking that the tree is clean and M1-56 is merged.
- Conventional Commits with scope `messaging` (or `api` for `operations.yaml`), every commit with
  DCO sign-off (`git commit -s`); squash into meaningful commits before the local merge
  (`docs/quality/push-discipline.md`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope list, except the CHANGELOG entry and `docs/DECISIONS.md`
  entries named here.
- Follow the host rules supplied by the launcher (ports, machine rules, SonarQube, deny list).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL 18 runs as a throwaway container through
  `pgtest` on a random port (P-09, P-10). Without a Docker runtime the test fails; it never skips.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove them.
- Never read or print secrets; never open `.env` files.
- No network calls except package downloads for pinned tools and pinned Go modules.
- Do not lower a threshold, disable a gate, skip or delete a test.
- If the core contract (chapters 00, 03, 04, 05) lacks something this brief needs, stop with
  `Verdict: BLOCKED <reason>` (P-11) instead of building a deviation.

## Design notes

1. Operations.

    | operationId | Route | Action | Resource given to `authorize` | Class |
    |---|---|---|---|---|
    | `listThreadMessages` | `GET /api/v1/threads/{thread_id}/messages` | `thread.read` | thread | `rl.read` |
    | `getMessage` | `GET /api/v1/messages/{message_id}` | `message.read` | message (thread, channel, author) | `rl.read` |

2. Order of checks for `listThreadMessages` (each step only if the previous passed):
    1. Query validation without any lookup (it cannot leak): more than one of `before_seq`,
       `after_seq`, `around_seq` gives `400 validation_failed` with one `errors[]` entry per given
       parameter, pointer `#query/<name>`, code `mutually_exclusive`. `after_seq` must be `>= 0`,
       `before_seq` and `around_seq` `>= 1`, all below 2^53; otherwise `400 validation_failed`,
       code `out_of_range`.
    2. Load the thread through the `chat` port and call `authorize(thread.read, thread)`. Missing
       and invisible threads take the same code path and return the same error value (02 9.2).
    3. Get the visible channel IDs for `message.read` from `Authorizer.Scope`.
    4. Run the one statement for the mode (note 4); then call `authorize(message.read, message)`
       per item (cached snapshot) in filter mode `authorize.Filter(ctx)` (M1-28; no denial audit, 07 4.2). A denied item is dropped, logged at error level with only the
       action and the resource kind (no IDs, no content), and counted in `bothy.authz.errors` with
       `bothy.reason = filter_mismatch` (07 7.4 rule 3, signals.md, alert `BothyAuthzError`).
    5. Build each item with the single representation function of M1-56.
3. Window. `L = min(limit or api.page_size_default, api.page_size_max)`.
    - Newest: the `L` highest `thread_seq`.
    - `before_seq = X`: the `L` highest with `thread_seq < X`.
    - `after_seq = X`: the `L` lowest with `thread_seq > X`.
    - `around_seq = X`: fetch `B` = up to `L` rows with `thread_seq < X` (descending) and `A` = up to
      `L` rows with `thread_seq >= X` (ascending). `b0 = min(|B|, (L - 1) / 2)` (integer division),
      `a = min(|A|, L - b0)`, `b = min(|B|, L - a)`. Items are the first `b` of `B` reversed,
      followed by the first `a` of `A`. A short side gives its share to the other side, so a page
      holds `min(L, messages in thread)` items.
    - Items are always ascending by `thread_seq` and contiguous: the server never returns a gap
      inside a page, because `thread_seq` has no gaps (03 6.2) and tombstones keep their number.
4. Flags and snapshot. `has_more_before` is true iff a message with `thread_seq` below the first
   item exists; `has_more_after` iff one above the last item exists. With no items:
   `before_seq = X` gives `has_more_before = false` and `has_more_after` = exists `thread_seq >= X`;
   `after_seq = X` gives `has_more_after = false` and `has_more_before` = exists `thread_seq <= X`;
   newest and around give both `false` (empty thread). Items and both flags come from one SQL
   statement per request (CTEs with `LIMIT L + 1` per direction and `EXISTS` for the opposite
   side), so all values stem from one snapshot under `READ COMMITTED`. The statements use the
   unique index `(thread_id, thread_seq)` and contain `m.thread_id = @thread_id AND m.channel_id =
   ANY(@visible_channel_ids::uuid[])`.
5. List representation: as `getMessage`, but `content_md` cut at `message.list_content_max_bytes`
   (UTF-8 boundary) with `content_truncated: true` when cut, absent otherwise.
6. Tombstone representation (one function, also used by M1-58): when `deleted_at` is set, the
   `Message` has `content_md = ""`, `attachments = []`, `mention_ids = []`, `reactions = []`,
   `my_reactions = []` (REST), `components = []`, `pinned = false`, `deleted_at` present,
   `content_truncated` absent; all other fields as stored. Tests create tombstones through a test
   helper that sets `content_md = ''`, `deleted_at`, `deleted_by` and `version = version + 1`
   directly, because the delete operation arrives with M1-58.
7. `getMessage`: load with the visible predicate, then `authorize(message.read, message)`; missing
   and invisible return the same error value. `200` with the REST representation (full
   `content_md`, never `content_truncated`) and `ETag: "<version>"` from the M1-18 helper.
   Tombstones are `200` too.
8. `TestQueriesCarryVisibleChannelPredicate` parses every `queries/*.sql` file of
   `internal/messaging/adapters/postgres` and fails when a `SELECT`, `UPDATE` or `DELETE` statement
   that names `messages`, `mentions`, `reactions`, `pins`, `message_revisions` or
   `message_templates` contains neither `channel_id = ANY(` nor an entry in an allowlist map in the
   test (`query name -> reason`). Seed the allowlist with M1-56's statements that legitimately lack
   it (for example the replay lookup by author and `client_msg_id`), each with its reason. M1-70
   later replaces this test with the repository-wide lint.
8. No JSON `null`: absent optional fields are omitted (encoder of M1-18). No new configuration key.
9. 404 equality: compare status, body without `correlation_id` and `instance`, and headers without
   `Correlation-Id`.

## Tests (write them first)

Unit (`internal/messaging/domain`, table-driven):

- `TestValidateHistoryRequest`: two or three anchors, negative and zero values, values at 2^53,
  valid single anchors.
- `TestPickAround`: middle of a long thread, near the start (fill from above), near the end (fill
  from below), anchor beyond the last message, `L = 1`, `L = 2`, empty thread.
- `TestPageFlags`: every mode with full, partial and empty results.

Integration against PostgreSQL 18 (`pgtest`, real `authz` sources, world of 07 "Examples"; HTTP
through the real chain on `127.0.0.1:0` with real authentication as in M1-48 and M1-56):

- `TestListThreadMessages_NewestPage`: 30 messages, `limit=10` gives 21 to 30,
  `has_more_before = true`, `has_more_after = false`.
- `TestListThreadMessages_BeforeAfterAround`: each mode against note 3, including both flags.
- `TestListThreadMessages_PagesStableAgainstNewMessages`: `before_seq=21&limit=10` returns 11 to 20;
  after five new messages the same request returns the identical body.
- `TestListThreadMessages_PagingCoversThreadExactlyOnce`: walking backwards from the newest page
  with `before_seq` yields every `thread_seq` from 1 to N exactly once, each page contiguous.
- `TestListThreadMessages_TombstonesKeepThreadSeq`: a tombstone appears at its position with the
  representation of note 6; the page stays contiguous.
- `TestListThreadMessages_ListContentCut`: with `message.list_content_max_bytes` set small in the
  test configuration, a long message appears cut at a UTF-8 boundary with `content_truncated:
  true`; `getMessage` returns it in full.
- `TestListThreadMessages_LimitDefaultAndCap`: with the test configuration setting small values for
  `api.page_size_default` and `api.page_size_max`, no `limit` returns the default size and a larger
  `limit` is capped.
- `TestListThreadMessages_ParamErrors`: `400 validation_failed` with the pointers and codes of note 2.
- `TestListThreadMessages_InvisibleThreadIs404`: Alex on a Marketing thread and a random UUIDv7
  give equal responses (note 9).
- `TestListThreadMessages_OperatorRuleReads`: the operator agent of a channel reads its thread
  without a grant (05 2.3 item 4).
- `TestGetMessage_FullContentWhenEventTruncated`: with `sync.event_payload_max_bytes` set small in
  the test configuration, a long message's `message.created` row is truncated, while `getMessage`
  returns the full `content_md` without `content_truncated`.
- `TestGetMessage_ETagMatchesVersion` and `TestGetMessage_Tombstone`.
- `TestAZM52_GetMessageForeignEqualsUnknown`: Alex requests a Marketing message ID and a random
  UUIDv7; responses equal per note 9.
- `TestQueriesCarryVisibleChannelPredicate` (note 7).
- A captured-logger assertion in the integration tests: no `content_md`, query string or display
  name appears in any log record.

Commands that must pass:

- `cd services/server && go test -race ./internal/messaging/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make docs`
- `make check` (includes `make sonar` against the local SonarQube; if it is not reachable, stop
  with `Verdict: BLOCKED`, never set `SONAR_SKIP`).

## Acceptance criteria

- [ ] `listThreadMessages` and `getMessage` are registered from `api/operations.yaml`, each runs
      `authorize` with its catalogue action (the proof check of M1-18 stays green).
- [ ] Every window and flag case of notes 3 and 4 is covered by a passing test.
- [ ] Pages are stable against new messages and contiguous; tombstones keep `thread_seq`.
- [ ] Foreign and unknown thread and message IDs give equal `404` responses (AZ-M52).
- [ ] Every messaging SQL statement carries the visible predicate or an allowlisted reason, and
      every listed item passes `authorize(message.read)`.
- [ ] `getMessage` returns the full content and `ETag: "<version>"`.
- [ ] No JSON `null` and no content in logs.
- [ ] README, doc comments on every new public symbol and CHANGELOG entry are present.
- [ ] `make check` is green.

## Definition of done

Code, tests and docs are complete: module README updated, a doc comment on every public symbol,
an ADR if an architecture decision was taken (none expected; record smaller rulings beyond 06 such as the
`around_seq` window in `docs/DECISIONS.md`), a CHANGELOG entry under `Unreleased`. `make check` is
green. An independent review and the red team (`docs/quality/red-team-gate.md`) have run over the
diff and their verdicts are recorded in the report. Merge locally into the base branch only if both
are green, then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red run of each
new test before the code), what is not proven, and follow-ups that could not be done in scope with
the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
