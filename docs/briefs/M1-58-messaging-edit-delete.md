# M1-58: Editing, deleting and revisions

Status: ready | Depends on: M1-56 | Milestone: M1

## Goal

After this ticket authors edit their own messages with `If-Match` and everyone who may read a
message can read its earlier versions. Deleting leaves a tombstone that keeps its `thread_seq`,
removes content, revisions and mentions, and redacts the payloads of all stored events about the
message in the same transaction, while their envelopes stay intact. Moderators with `manage` delete
other people's messages, audited. This is the content-removal guarantee that data protection
(DP-T01) and the offline outbox (04 "Editing and deleting") rely on.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 7 "M1 Foundation" (row M1-58,
  "Prerequisites from the core contract": K2 and K4, "Red team").
- `docs/briefs/README.md`.
- `docs/spec/00-glossary.md`: tombstone, redaction, message revision, event envelope.
- `docs/spec/02-architecture.md`: "Modules of the monolith" (4.1; 4.3 rows `messaging`, `chat`,
  `sync`, `runs`; 4.4; 4.5 row `kernel/eventlog`), "Layers within a module", "Transactions and
  Unit of Work" (8.1 to 8.5), "Request path".
- `docs/spec/03-data-model.md`: "Principles" (items 7, 10, 12, 13), tables `messages`,
  `message_revisions`, `mentions`, `threads`, `event_log`; "Transaction rules" 6.2, 6.3, 6.5;
  R-DM-18.
- `docs/spec/04-sync-protocol.md`: "Envelope" (`redacted`, `truncated`), "Global, commit-ordered
  sequence", "Idempotent writes" (5.2), "Editing and deleting" (13.3), "Delivery guarantee".
- `docs/spec/05-actions.md`: rules 1.1, 1.2, 1.7, 1.8; 'Columns "Owner", "Step-up", "Audit",
  "Permissions"'; table 4.7 (`message.update`, `message.delete`, `message.revisions.read`);
  section 6 rows `message.updated`, `message.deleted`.
- `docs/spec/06-api.md`: "Errors" (codes `version_conflict`, `precondition_required`,
  `message_deleted`, `payload_too_large`, `validation_failed`), "Idempotency", "Concurrency",
  "Endpoints" > "Messages".
- `docs/spec/07-permissions.md`: 4.1, 4.2, 4.3, 7.4; "Test matrix".
- `docs/spec/09-agents.md`: section 6 "Trigger state (`trigger_state`)" (owned by `runs`;
  editing never triggers).
- `docs/spec/19-security.md`: SEC-LOG-4.
- `docs/spec/20-privacy.md`: "Deleting a message" (4.2), "Audit log and IP addresses" (4.5),
  "Tests" (DP-T01).
- `docs/api/openapi.yaml`: operations `updateMessage`, `deleteMessage`, `listMessageRevisions`;
  schemas `Message`, `MessageUpdate`, `MessageRevision`, `MessageRevisionList`, `Problem`
  (`current`), `MessagePayload`, `MessageDeletedPayload`, `ReactionPayload`, `PinPayload`,
  `EventEnvelope`; parameters `IfMatchRequired`, `IdempotencyKey`.
- ADRs: `docs/adr/0009-commit-ordered-event-log-with-a-global-sequence.md`,
  `0015-error-format-rfc-9457-idempotency-pagination.md`,
  `0017-one-authorization-function-404-instead-of-403-mandatory-registration.md`,
  `0021-hash-chained-audit-log-separate-from-telemetry.md`; ADR 0052 and 0053 once M1-16 has
  written them.
- Code of earlier tickets: `internal/messaging` (M1-56), `internal/chat` port `Threads` (M1-55),
  `internal/kernel/eventlog` and `internal/sync` (Appender, envelope; M1-16),
  `internal/kernel/txn` (M1-13), `internal/kernel/auditing` (M1-24), `internal/kernel/idempotency`
  and its middleware (M1-20), the ETag and If-Match helpers (M1-18), `pgtest` (M1-12),
  `api/operations.yaml` (M1-17).
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/quality/red-team-gate.md`, the READMEs of
  `internal/messaging`, `internal/chat`, `internal/sync`.
- Tables: `messages`, `message_revisions` (new), `mentions`, `threads` (via `chat`), `event_log`
  (via `kernel/eventlog`). Actions: `message.update`, `message.delete`, `message.revisions.read`.

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
6. Test first: write the failing test, record it red, then make it green.
7. No en dash or em dash characters in anything you write; use a hyphen or rephrase.
8. White label: examples use Alex, Sam, Robin, Support, Marketing, Engineering, Research and
   `example.com` only.
9. Limits only through named configuration keys from `docs/operations/configuration.md`.
10. Only the author edits, with `If-Match`; no edit deadline (05 `message.update`; 01 3.3).
    Missing `If-Match` gives `428 precondition_required`, a stale one `412 version_conflict` with
    the current representation in `current` (06 "Concurrency" item 2; 04 13.3; K4).
11. Editing a deleted message gives `410 message_deleted` (04 13.3; openapi `updateMessage`).
12. Each edit stores the previous content as a revision; revision 1 is the original
    (03 `message_revisions`); run anchors (`kind = 'run'`) store no revisions.
13. Deleting is idempotent: an already deleted message gives `204` (04 13.3; 06 "Idempotency").
    Own messages need `write`; other people's need `manage` and are audited
    (05 `message.delete`, column Audit "yes for other people's").
14. Deleting clears `content_md`, removes revisions and mentions, keeps the row and `thread_seq`,
    redacts event payloads with resource `message` and this ID, sets `redacted_at`, keeps every
    envelope column, and appends `message.deleted`, all in one transaction (03 6.5; R-DM-18;
    20 4.2; DP-T01 part messages).
15. `message_count` of the thread counts messages without deleted ones (03 `threads`).
16. Every edit and deletion raises `messages.version` (03 `messages`) and appends a durable event
    with `resource.version` after the change (04 "Envelope").
17. Editing never triggers an agent and never changes `trigger_state` (09 section 6; only new human
    messages trigger).
18. No content, emoji or display names in logs, telemetry or audit details (SEC-LOG-4; 20 4.5).

## Scope

All paths below `services/server/` unless stated. Reuse equivalents that earlier tickets already
built instead of adding a second version.

1. Migration `migrations/<next number>_message_revisions.sql`: table `message_revisions` exactly as
   03 (columns, `UNIQUE (message_id, revision)`, FKs `message_id` ON DELETE CASCADE, `edited_by` ON
   DELETE RESTRICT). Use the next free goose number at your local merge; if a parallel ticket took
   it, renumber yours before merging (a merged migration never changes).
2. Port `internal/messaging`:
    - `Messages.Update(ctx context.Context, c UpdateCommand) (MessageView, error)` with
      `UpdateCommand{MessageID ids.ID; IfMatch int64; Content *string; MentionIDs *[]ids.ID}`
      (pointer means "field given"; it never reaches JSON).
    - `Messages.Delete(ctx context.Context, messageID ids.ID) error`.
    - `Messages.Revisions(ctx context.Context, messageID ids.ID) ([]RevisionView, error)`.
    - In-transaction event `MessageDeleted{MessageID, ThreadID, ChannelID ids.ID; ThreadSeq int64;
      AuthorID, DeletedBy ids.ID; MentionedPrincipalIDs []ids.ID; Version int64}` (02 4.3; `runs`
      subscribes in M2). `ids.ID` is the UUID type of `kernel/ids`.
3. `internal/messaging/domain`: edit rules (no-op detection, revision needed by kind), the
   tombstone representation (shared with M1-57; one function), and `RedactPayload` (note 6).
4. `internal/messaging/app`: use cases `UpdateMessage`, `DeleteMessage`, `ListMessageRevisions`.
5. `internal/messaging/adapters/postgres/queries/*.sql` and regenerated `sqlcgen/`.
6. `internal/messaging/adapters/httpapi`: `UpdateMessage`, `DeleteMessage`,
   `ListMessageRevisions`.
7. `internal/kernel/eventlog`: new port, implemented by `sync`:
    ```go
    // Redactor replaces the payload of stored durable events of one resource.
    type Redactor interface {
        // Redact calls fn for every stored event with resource (kind, id) in ascending seq,
        // writes the returned payload, sets redacted_at if unset, and leaves every other
        // column unchanged. Runs in the ambient transaction; returns the number of rows.
        Redact(ctx context.Context, kind string, id ids.ID, fn RedactFunc) (int, error)
    }
    type RedactFunc func(eventType string, schemaVersion int, payload json.RawMessage) (json.RawMessage, error)
    ```
    Reason: `messaging` may not import `sync` (02 4.4, `sync` is the higher module); the port is
    therefore a kernel port, like `Appender` (02 4.5, PLAN row M1-58).

8. `internal/sync`: postgres implementation of `Redactor` (rows locked with `FOR UPDATE`, index
   `(resource_kind, resource_id)`), registration in `internal/wire`; check that the envelope
   builder maps `redacted_at IS NOT NULL` to `"redacted": true` and add that mapping with a test if
   it is missing.
9. `internal/chat` port `Threads` plus postgres adapter and tests: `LockForMessageChange(ctx,
   threadID ids.ID) error` (`SELECT ... FOR UPDATE` on the thread row) and
   `DecrementMessageCount(ctx, threadID ids.ID) error` (`message_count - 1`, `version + 1`, error if
   no row or the count would go below zero).
10. `api/operations.yaml`: enable `updateMessage`, `deleteMessage`, `listMessageRevisions`;
    regenerate.
11. The specification is in place: `docs/spec/02-architecture.md` 4.3 (`chat.Threads` with the
    message count, `sync` implements `eventlog.Redactor`) and 4.5 (row `kernel/eventlog` with
    `Redactor`); 04 2.2 rules 4 and 5 (redacted shapes); 03 `messages` (deletion removes reactions
    and the pin). Change them only where the implementation proves them wrong;
    `architecture/modules.yaml` only if it lists kernel ports or their implementers.
12. Docs: READMEs of `messaging`, `chat`, `sync` (keep their English text and structure; add the
    lock order of note 4), `services/server/CHANGELOG.md`, `docs/DECISIONS.md` entries for the
    no-op rule and error precedence of note 2 and for notes 4, 6 and 8.

## Out of scope

- Reading history and the tombstone in history (M1-57); reactions and pins and their removal on
  delete (M1-59, note 8); mention list and the computed counters (M1-60; no counter is stored,
  so deleting needs no counter update, 03 R-DM-32); read states (M1-61, M1-63); thread move
  (M1-62).
- Deleting attachments (M3, through the `files` port) and resetting `messages.has_attachments`
  (M3-07 adds the column, 03 R-DM-43); components (M2, `runs` subscribes
  `MessageDeleted`); cancelling a run of a deleted trigger message (M2).
- Deletion journal entry of 20 4.6 (M7-07); GDPR jobs (M7).
- Gateway delivery of the events (M1-65); search index (M3).

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-58-messaging-edit-delete`, created from
  the base branch the launcher names, after checking that the tree is clean and M1-56 is merged.
- Conventional Commits with scopes `messaging`, `chat`, `sync`, `api`, every commit with DCO
  sign-off (`git commit -s`); squash into meaningful commits before the local merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope list, except the CHANGELOG entry and `docs/DECISIONS.md`
  entries named here.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL 18 through `pgtest` on a random port; without
  a Docker runtime the test fails, it never skips.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove them.
- Never read or print secrets; never open `.env` files.
- No network calls except package downloads for pinned tools and pinned Go modules.
- Do not lower a threshold, disable a gate, skip or delete a test.
- If the core contract (00, 03, 04, 05) lacks something this brief needs, stop with
  `Verdict: BLOCKED <reason>` (P-11).

## Design notes

1. Operations.

    | operationId | Route | Action | Resource | Class | Headers |
    |---|---|---|---|---|---|
    | `updateMessage` | `PATCH /api/v1/messages/{message_id}` | `message.update` | message (thread, channel, author) | `rl.message` | `If-Match` required, `Idempotency-Key` optional |
    | `deleteMessage` | `DELETE /api/v1/messages/{message_id}` | `message.delete` | message | `rl.message` | none |
    | `listMessageRevisions` | `GET /api/v1/messages/{message_id}/revisions` | `message.revisions.read` | message | `rl.read` | none |

    Build the message resource descriptor the same way M1-56 does (thread, channel, author), so
    `authorize` can evaluate "write and author" and "(write and author) or manage".

2. `updateMessage`, precedence: `428` (no lookup) > `404` > `403` (including
   `channel_read_only`) > `410` > `412` > `413` > `400`.

    1. Missing `If-Match`: `428 precondition_required` before any lookup.
    2. Load with the visible predicate, `authorize(message.update)`; missing and invisible share one
       code path.
    3. In the unit of work: lock the message row (`SELECT ... FOR UPDATE` with the visible
       predicate); `deleted_at` set gives `410 message_deleted`; version different from `If-Match`
       gives `412 version_conflict` with `current` = REST representation.
    4. Resulting state: `content_md` = given or current; mentions = resolved again from the
       resulting content with M1-56's scanner and resolution (03 `messages` "Content format", 07
       4.6); a given `mention_ids` must equal that set (same function, same errors,
       `400 validation_failed`). Content above `message.max_content_bytes` gives
       `413 payload_too_large` (detail names the key).
    5. No change (same content, same mention set, also an empty body): `200` with the current
       representation and its ETag; no write, no revision, no event.
    6. Otherwise: unless `kind = 'run'`, insert `message_revisions(revision = max + 1 or 1,
       content_md = previous content, edited_by = caller)`; update `content_md`, `edited_at = now()`,
       `version = version + 1`; replace `mentions` (delete removed, insert added with the thread ID
       copy); append `message.updated` (resource version = new version, payload `{message}` built
       with M1-56's event representation and truncation). No `MessagePosted`, no change of
       `trigger_state`, `trigger_state_at` or `trigger_state_reason`.
    7. `200` with the REST representation and `ETag: "<version>"`. `Idempotency-Key` is handled by
       the M1-20 middleware: a repeat returns the stored response and creates no second revision.
3. `deleteMessage`:
    1. Load with the visible predicate, `authorize(message.delete)`; missing and invisible share one
       code path; `403` when the caller may read but is neither author with `write` nor holder of
       `manage`.
    2. Already deleted: `204`, no write, no event, no audit.
    3. Unit of work: `chat.Threads.LockForMessageChange(thread_id)`, then lock the message row; if
       it was deleted meanwhile, `204` without writes.
    4. Remember the mentioned principal IDs; `UPDATE messages SET content_md = '', deleted_at =
       now(), deleted_by = caller, version = version + 1` (the check `deleted_at IS NULL OR
       content_md = ''` holds; `search_vector` becomes empty by itself).
    5. Delete the message's rows in `message_revisions` and `mentions` (and in `reactions` and
       `pins`, note 8).
    6. `chat.Threads.DecrementMessageCount(thread_id)`.
    7. `eventlog.Redactor.Redact(ctx, "message", id, fn)` with `fn` from note 6.
    8. Publish `MessageDeleted`.
    9. Append `message.deleted`: actor = caller, scope = space, channel, thread, resource =
       `{kind: message, id, version: new}`, payload `MessageDeletedPayload{message_id, thread_id,
       thread_seq, deleted_at, deleted_by}`. It is appended after the redaction, so it is never
       redacted.
    10. Caller is not the author: write an `allow` audit entry in the same transaction through
        `auditing.Recorder` (action `message.delete`, resource message ID, details only `thread_id`,
        `channel_id`, `author_principal_id`). Denials are recorded by the existing deny path
        (R-AZ-04); do not add a second one.
    11. `204`.
4. Lock order (document in the messaging README): thread row (through `chat`) before message row;
   several message rows in ascending `id`; the counters last (03 6.3, enforced by the unit of work).
   Thread move (M1-62) also locks the thread first, so delete and move cannot deadlock; edit locks
   only the message row.
5. `listMessageRevisions`: load with the visible predicate, `authorize(message.revisions.read)`;
   items ascending by `revision` (`MessageRevision{revision, content_md, edited_by, created_at}`);
   a deleted message returns `{"items": []}`; unpaginated (`MessageRevisionList`).
6. Redacted payloads (`RedactPayload`, a pure function of the domain, used as `RedactFunc`):
    - `message.created`, `message.updated`: `{"message": T}` where `T` is the event form of the
      tombstone after this deletion: all stored fields of the message, but `content_md = ""`,
      `attachments = []`, `mention_ids = []`, `reactions = []`, `components = []`,
      `pinned = false`, `deleted_at` set, `version` = the new version, no `my_reactions`, no
      `content_truncated`.
    - `reaction.added`, `reaction.removed`: keep `message_id`, `thread_id`, `principal_id`; set
      `emoji = ""` and `count = 0`.
    - `pin.added`, `pin.removed`: unchanged payload (it carries no content); only `redacted_at` is
      set.
    - Any other type: return an internal error, so the transaction fails; a future event type on
      resource `message` must define its redaction before it can be deleted.
    Every redacted payload stays valid against its schema in `openapi.yaml` (test). 04 2.2 rule 5
    states these shapes.
7. Events and audit carry no content beyond the schema; logs carry no content, emoji or names.
8. Reactions and pins of a deleted message are removed in the same transaction (03 `messages`, 20
   4.2: a tombstone shows no reactions or pin, and their event payloads are redacted by note 6).
   Their tables come from M1-59, which depends on this ticket and adds the two `DELETE` statements
   to this delete path with the test `TestDeleteMessage_RemovesReactionsAndPin`. Leave a marked
   step for them in the use case.
9. Concurrency: the message row lock serialises edits, deletions and (from M1-59) reactions and
   pins of one message; revision numbers come from `max(revision) + 1` under that lock.

## Tests (write them first)

Unit:

- `TestRedactPayload`: each event type of note 6; unknown type fails; output validates against the
  openapi schema (use the schema validator the contract tests use, or decode into the generated
  types with unknown fields rejected).
- `TestEditRules`: no-op detection (same content and mentions, empty body), `kind = 'run'` stores no
  revision.

Integration against PostgreSQL 18 (`pgtest`, world of 07 "Examples", HTTP through the real chain):

- `TestUpdateMessage_MissingIfMatch428`, `TestUpdateMessage_StaleIfMatch412WithCurrent`
  (`current.version` equals the stored version).
- `TestUpdateMessage_Deleted410`.
- `TestUpdateMessage_NotAuthor403`: the Owner edits Alex's message (may read, not author).
- `TestUpdateMessage_InvisibleIs404`: Alex on a Marketing message equals a random UUIDv7.
- `TestUpdateMessage_ReadOnlyChannel403` (`channel_read_only`).
- `TestUpdateMessage_StoresRevisions`: two edits give revisions 1 (original) and 2 with the
  previous contents; `edited_at` set; version raised by two.
- `TestUpdateMessage_NoChangeIsNoop`: no revision, no event, same version.
- `TestUpdateMessage_ReplacesMentions` (new content without `mention_ids` resolves its tokens;
  removed tokens remove rows) and `TestUpdateMessage_MentionMismatch400`.
- `TestUpdateMessage_TooLarge413`.
- `TestUpdateMessage_AppendsMessageUpdated`: one `message.updated` with the new `resource.version`.
- `TestUpdateMessage_NeverTriggers`: a test subscriber sees no `MessagePosted`; `trigger_state`
  columns unchanged.
- `TestUpdateMessage_IdempotencyKeyReplay`: same key twice gives the stored response and one
  revision.
- `TestUpdateMessage_ConcurrentSameIfMatch`: two parallel edits with the same `If-Match` give one
  `200` and one `412`.
- `TestDeleteMessage_OwnIs204AndIdempotent`: second call `204`, exactly one `message.deleted`.
- `TestDeleteMessage_ForeignNeedsManageAndIsAudited`: the Owner deletes Alex's message, `204` and
  one `allow` audit entry without content; a principal with `write` but without `manage` gets `403`
  and the deny path records it.
- `TestDeleteMessage_TombstoneKeepsThreadSeq`: row and `thread_seq` stay, `message_count` drops by
  one, thread `version` rises by one.
- `TestDeleteMessage_ConcurrentDeletesDecrementOnce`.
- `TestDeleteMessage_EditDeleteRace`: either the edit wins and the delete follows, or the edit gets
  `410`; never a revision after the tombstone.
- `TestDeleteMessage_PublishesMessageDeleted`: a test subscriber receives the event with the
  remembered mention IDs.
- `TestDPT01_DeleteMessageRemovesContent`: a message with a unique canary string in content, a
  mention and two edits is deleted; afterwards `content_md = ''`, `search_vector` is empty, no rows
  in `message_revisions` and `mentions`, `SELECT count(*) FROM event_log WHERE payload::text LIKE
  '%<canary>%'` is 0, every event of the message except `message.deleted` has `redacted_at`, all
  other `event_log` columns equal their values before the deletion, and the delivered envelope
  carries `"redacted": true`.
- `TestDeleteMessage_RemovesReactionsAndPin` (note 8, only once M1-59 is on the base).
- `TestListMessageRevisions_OrderAndVisibility`: ascending revisions; invisible message `404`;
  deleted message empty list.
- `internal/sync`: `TestRedactor_OnlyMatchingRows`, `TestRedactor_EnvelopeUnchanged`,
  `TestRedactor_Idempotent`, `TestRedactor_ErrorRollsBack`, `TestEnvelope_RedactedFlag`.
- `internal/chat`: `TestThreads_DecrementMessageCount` (never below zero) and
  `TestThreads_LockForMessageChange`.
- Keep `TestQueriesCarryVisibleChannelPredicate` (M1-57) green if it is on your base; in any case
  every new `SELECT`, `UPDATE` or `DELETE` on messaging tables carries `channel_id = ANY(...)` or
  joins `messages` with it.

Commands that must pass:

- `cd services/server && go test -race ./internal/messaging/... ./internal/sync/... ./internal/chat/...`
- `make codegen-check`, `make migrations-lint`, `make arch`, `make api-lint`, `make docs`
- `make check` (with the local SonarQube; if unreachable, `Verdict: BLOCKED`, never `SONAR_SKIP`).

## Acceptance criteria

- [ ] Wrong `If-Match` gives `412` with `current`, missing gives `428`, a deleted message `410
      message_deleted`.
- [ ] Each edit stores the previous content as the next revision; `listMessageRevisions` returns
      them in order.
- [ ] Deleting is idempotent (`204`); other people's messages only with `manage`, audited without
      content.
- [ ] After deletion content, revisions, mentions and event payloads are empty; envelopes and
      `thread_seq` stay; `message.deleted` is appended once (R-DM-18, DP-T01 part messages).
- [ ] `message_count` excludes deleted messages; the lock order of note 4 is implemented and
      documented.
- [ ] `kernel/eventlog.Redactor` exists, is implemented by `sync`, and 02 4.3 and 4.5 name it.
- [ ] Edits never trigger and never change `trigger_state`.
- [ ] DECISIONS.md entries, README, doc comments and CHANGELOG are present.
- [ ] `make check` is green.

## Definition of done

Code, tests and docs are complete: module READMEs updated, a doc comment on every public symbol,
an ADR if an architecture decision was taken (the kernel port follows 02 and needs none; rulings go
to `docs/DECISIONS.md`), a CHANGELOG entry under `Unreleased`. `make check` is green. An independent
review and the red team (`docs/quality/red-team-gate.md`) have run over the diff and their verdicts
are recorded in the report. Merge locally into the base branch only if both are green, then
`git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red run of each
new test), what is not proven, and follow-ups that could not be done in scope with the reason
(say whether note 8 was applied). Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or
`exit=1 <reason>`.
