# M1-56: Sending messages

Status: ready | Depends on: M1-55 | Etappe: M1

## Goal

After this ticket people, agents and apps send messages into threads: idempotent by
`client_msg_id`, ordered by a gapless thread sequence, with server-set origin, reply with quote
and resolved mentions, published in the same transaction as the in-transaction event
`MessagePosted` and the durable event `message.created`. A message in a done thread reopens it and
leaves a system line. The same ticket enables `createThread`, so forum posts and threads from a
channel message can be created together with their first message.

## Read first

- `docs/spec/03-datenmodell.md`: tables `messages` (with the paragraph "Inhaltsformat" for
  `content_format = 1` and mention tokens), `mentions`, `threads`; "Grundsätze" points 1, 7, 9, 13;
  "Transaktionsregeln" 6.1 to 6.3; R-DM-11, R-DM-12, R-DM-18; config table
  (`message.max_content_bytes`, `message.max_mentions`).
- `docs/spec/04-sync-protokoll.md`: "Umschlag (Envelope)" with the payload rules,
  "Globale, commit-geordnete Sequenz" 3.2, "Thread-Sequenz", "Nachrichten: `client_msg_id`"
  (5.1), "Alle anderen Schreibvorgänge" (5.2), "Ausgangsfach" 13.2, "Bearbeiten und Löschen"
  row "Thread inzwischen erledigt", "Geräteübergreifende Synchronisation" row "Entwürfe"
  (`clear_draft`), "Nachricht senden, online" (16.1), "Senden offline, dann Wiederverbinden"
  (16.2), config `sync.event_payload_max_bytes`.
- `docs/spec/05-aktionen.md`: rules 1.2, 1.8; "Ziel einer Ressource"; rows `message.create`,
  `thread.create`, `thread.reopen`, `user.read`, `agent.read`; section 6 rows `message.created`,
  `thread.created`, `thread.reopened`; "Scopes für Apps".
- `docs/spec/09-agenten.md`: 3.2 "Herkunft", 3.4 "Entscheidung je Nachricht" (only to know what
  M2 needs from `MessagePosted`), 3.7, 12.
- `docs/spec/01-produkt.md`: 3.3 "Nachrichten", 6.1 "Erledigt und Wiederöffnen", 6.4 "Lange
  Antworten".
- `docs/spec/06-api.md`: "Grundregeln", "Autorisierung in der API" points 6 and 7, "Fehler",
  "Idempotenz", endpoint tables "Threads" and "Messages".
- `docs/spec/02-architektur.md`: 4.3 (`messaging` publishes `MessagePosted`; `chat.Threads` with
  `AppendToThread`), 4.4 (`messaging` may use `identity`, `chat`, `files`), 4.6 "Begründungen für
  Grenzfälle", 8.1 to 8.4, 9.2.
- `docs/spec/07-rechte.md` 4.3, 4.6 "Erwähnungen auflösen", 7.4; `docs/spec/13-push-benachrichtigungen.md` 2 (what mentions are
  for, built in M3); `docs/spec/19-sicherheit.md` SEC-IN-2, SEC-IN-4, SEC-OUT-3.
- `docs/api/openapi.yaml`: operationIds `createMessage`, `createThread`; schemas `MessageCreate`,
  `Message`, `MessageOrigin`, `MessageKind`, `PrincipalRef`, `ThreadCreate`, `ThreadCreated`,
  `MessageCreatedEvent`, `MessagePayload`, `EventEnvelope`; headers `Idempotent-Replay`.
- `docs/api/asyncapi.yaml`: `event.message.created`, `event.thread.created`,
  `event.thread.reopened`.
- ADRs 0009, 0013, 0015, 0017, 0052, 0053.
- Code of merged tickets: `internal/chat` (`Threads.Create`, `AppendToThread`, `Info` from M1-55),
  `internal/identity` (`Directory`), `internal/sync` (Appender, Tail, `pg_notify`),
  `internal/kernel/{txn,eventlog,authorize,actor,ids}`, the idempotency middleware (M1-20).

## Fixed requirements that apply

Owner build rules (verbatim):

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

Note on rule 3: `docs/api/openapi.yaml` is the source; the server interface is generated from it
(ADR 0013) and `make codegen-check` keeps both identical.

Rules for every brief:

6. Test first: red, then green, then cleanup; keep the red output for the report.
7. No dashes U+2013 or U+2014 in any file you write.
8. White label: only the example names of the specification.
9. Limits and windows only through named configuration keys.

Product requirements implemented here:

10. `(author_principal_id, client_msg_id)` is unique forever; a repeat with the same canonical
    request returns the original with `200` and `Idempotent-Replay: true` and no new events; a
    different request gives `422 idempotency_key_reuse`; two concurrent identical requests create
    one message (04 5.1).
11. `thread_seq` gapless per thread, taken under the thread row lock before the `event_seq` lock
    (03 6.2, 04 4).
12. `origin` is set by the server from the principal kind; clients cannot set it (09 3.2).
13. Content up to `message.max_content_bytes`, stored unchanged; larger events are cut to
    `sync.event_payload_max_bytes` with `truncated` (04 2.2 rule 2; 01 6.4).
14. Reply with quote only to a message of the same thread (03 `messages.reply_to_message_id`).
15. Mentions follow the content format of 03 `messages` and are resolved by the rule of 07 4.6;
    no collective mentions; at most `message.max_mentions` distinct handles.
16. A new message reopens a done thread and a system line says so (glossary "Erledigt", 01 6.1).
17. `MessagePosted` is published in the transaction for every new message (02 4.3, 8.3).
18. Forum posts and threads from a channel message through `createThread` (01 3.1).

## Scope

Paths relative to `services/server/` unless they start with `docs/`:

1. `migrations/<next number>_messaging_messages.sql`: tables `messages` and `mentions` as 03, with
   these deliberate deviations, each commented in the file:
    - `messages.search_language`, `messages.search_vector` and the GIN index: M3-13 adds them.
    - FK `messages.run_id -> runs(id)`: M2-05 adds it (table `runs` does not exist yet); the column
      and the CHECK `(kind = 'run') = (run_id IS NOT NULL)` exist now.
    - FK `messages.import_item_id -> import_items(id)`: added by the import ticket of M8 (table
      `import_items` does not exist yet); the column exists now.
    - All other columns, CHECKs, unique and partial indexes as 03.
    - In the same migration: `ALTER TABLE threads ADD CONSTRAINT threads_parent_message_id_fkey
      FOREIGN KEY (parent_message_id) REFERENCES messages(id) ON DELETE RESTRICT DEFERRABLE
      INITIALLY DEFERRED` (left out by M1-53 because `messages` did not exist).
2. `internal/messaging` (port): `Messages` port (`Create`), DTOs, and the in-transaction event:

```go
// MessagePosted is published inside the writing transaction for every new message, including
// system lines. Subscribers (runs in M2, sync in M1-61, notify in M3) may write and may veto.
// It never carries content; subscribers that need it load the message through the port.
type MessagePosted struct {
    MessageID, ThreadID, ChannelID, SpaceID uuid.UUID
    ThreadKind      string      // "main" or "thread"
    ThreadSeq       int64
    AuthorID        uuid.UUID
    AuthorKind      string      // "user", "agent", "app"
    Origin          string      // "human", "agent", "app", "system" ("import" in M8)
    Kind            string      // "text" or "system" in M1
    ClientMsgID     uuid.UUID
    AuthorDeviceID  *uuid.UUID  // device of the author's session; absent for agents, apps, system
    ReplyToID       *uuid.UUID
    MentionedIDs    []uuid.UUID // resolved, sorted, without duplicates
    Silent          bool
    ClearDraft      bool
    CreatedThread   bool        // first message of a thread created in the same transaction
    ReopenedThread  bool        // this message reopened the thread
    CreatedAt       time.Time
}
```

3. `internal/messaging/domain`: `Message`, origin from principal kind, content rules, the mention
   scanner (D5), the canonical request hash (D3), event truncation (D7), the system line (D6).
4. `internal/messaging/app`: use cases `CreateMessage` and `CreateThread` (D8).
5. `internal/messaging/adapters/postgres`: `queries/messages.sql`, `queries/mentions.sql`,
   repository; mention lookups in one batched query per message.
6. `internal/messaging/adapters/httpapi`: handlers `createMessage` and `createThread`.
7. `internal/identity` port, only if missing: a batched lookup of active principals by
   lower-cased handles (kind `user` or `agent`) and `PrincipalRef` data by IDs; with tests in
   `identity`.
8. `internal/kernel/eventlog`, only if missing: a way to mark an appended event as `truncated`.
9. `api/operations.yaml`: enable `createMessage` and `createThread`; regenerate.
10. `internal/platform/config` schema: `message.max_content_bytes`, `message.max_mentions` (03),
    `sync.event_payload_max_bytes` (04) if missing.
11. `internal/wire`: register `MessagePosted` (no subscriber in this ticket) and the handlers;
    update the golden file.
12. `docs/api/openapi.yaml` already has the `200` replay response of `createThread` and
    `Message.system_code` (D6); change them only where the implementation proves them wrong.
13. `docs/spec/02-architektur.md` 4.6 already assigns `thread.create` to `messaging` (bullet in
    "Begründungen für Grenzfälle"); `architecture/modules.yaml` lists `thread.create` under
    `messaging` (M1-06 holds it).
14. Documentation: `internal/messaging/README.md`, `services/server/CHANGELOG.md`,
    `docs/DECISIONS.md` (only decisions beyond the specification; D2, D5, D6 and D8 link it).

## Out of scope

- Agent triggering: `agent.invoke`, `trigger_state`, follow-ups, auto thread on trigger: M2. In
  M1 no message gets a `trigger_state`.
- Attachments (`attachment_file_ids`, `+upload`): M3-07. Voice messages (`kind = voice`): M3.
  Components and cards (`components`, `kind = card`) and `run_id`: M2. Until then the server
  rejects these members (D2).
- Reading history, `getMessage`: M1-57. Edit, delete, revisions, redaction: M1-58. Reactions and
  pins: M1-59. Mention list and templates: M1-60. Drafts and read state, including the effect of
  `clear_draft` and mention counters: M1-61 (subscribes to `MessagePosted`).
- Push and notifications for mentions: M3. Search vectors: M3-13. Gateway delivery: M1-65.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-56-messaging-send`, created from the base
  branch the launcher names. One writer per worktree.
- Conventional Commits with a module scope (`feat(messaging): ...`), every commit with DCO
  sign-off (`git commit -s`). Squash to meaningful commits before the merge.
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
M1-55 merged (`chat.Threads` has `Create`, `AppendToThread`, `Info`), else
`Verdict: BLOCKED dependency M1-55 missing`.

D1. `createMessage` flow (04 5.1 and 16.1):

1. Decode with the generated decoder (unknown members, duplicate keys and invalid UTF-8 are
   rejected there, M1-18).
2. `chat.Threads.Info(thread_id)`; missing gives `404`.
3. `authorize(actor, message.create, thread)`; invisible gives `404` identical to an unknown ID,
   `read` only gives `403`, read-only or archived channel gives `403 channel_read_only`. Apps need
   scope `messages:write`.
4. Validate the body (D2) and compute the request hash (D3).
5. Unit of work: look up `(author, client_msg_id)`. Found and same hash: answer `200` with the
   current representation of that message and `Idempotent-Replay: true`, no events. Found and other
   hash: `422 idempotency_key_reuse`. Not found: resolve mentions (D5); `AppendToThread` with
   `ReopenIfClosed = true` (origin `human`, `agent`, `app`); if `Reopened`, insert the system line
   (D6) with that sequence and call `AppendToThread` again for the message; insert the message and
   its `mentions` rows; publish `MessagePosted` (system line first, then the message); append
   `message.created` events (D7); commit.
6. A unique violation on `(author_principal_id, client_msg_id)` at insert rolls the whole unit of
   work back, including the thread sequence; then repeat step 5 lookup in a new transaction and
   answer as a replay or with `422`.
7. Answer `201` with `Message`.
The replay path still runs step 3: a principal who lost access gets `404`, never the original.

D2. Body rules (`MessageCreate`):

- `client_msg_id` must be a UUIDv7 (`kernel/ids`; the time inside is not checked, 04 5.1 point 5),
  else `400` (`/client_msg_id`, `not_uuid_v7`).
- `kind` absent means `text`. In M1 only `text` is enabled; `voice` and `card` give `400`
  (`/kind`, `not_enabled`). Non-empty `attachment_file_ids`, non-empty `components` or any `run_id`
  give `400` with the member as pointer and code `not_enabled` (06 "Fehler" item 3); a silently
  dropped member would lose an intent (W7). M2-15, M2-19, M3-07 and M3-09 lift the rejection
  (PLAN).
- `content_md` must contain a non-white-space character, else `400` (`/content_md`, `empty`); at
  most `message.max_content_bytes` UTF-8 bytes, else `413 payload_too_large` with `detail` naming
  `message.max_content_bytes`. Content is stored byte for byte (SEC-IN-4).
- `reply_to_message_id` must be a message of the same thread (deleted ones allowed), else `400`
  (`/reply_to_message_id`, `not_in_thread`); the same answer for an ID of another thread and an
  unknown ID, so nothing leaks.
- `silent` is stored; `clear_draft` is not stored and travels in `MessagePosted`.
- `mention_ids`, when present, must equal the resolved set (D5) as a set, else `400`
  (`/mention_ids`, `mention_mismatch`); more than `message.max_mentions` distinct handles give
  `400` (`/content_md`, `too_many_mentions`) (03 `messages`, "Inhaltsformat").

D3. Canonical request hash (`client_payload_sha256`, 04 5.1 point 3): SHA-256 over a versioned,
length-prefixed encoding, not over JSON text, so key order and white space cannot change it:
`"bothy-msg-v1"` then, each as `uvarint(len) || bytes`: the context (`"thread:" + thread_id` for
`createMessage`; for `createThread` see D8), `kind`, `content_md`, `reply_to_message_id` (empty
when absent), the attachment IDs in request order joined by `","` (empty in M1). Changing the
encoding later is a new version prefix, never an edit.

D4. Origin: `user` gives `human`, `agent` gives `agent`, `app` gives `app` (from `kernel/actor`).
`trigger_state`, `trigger_state_reason`, `trigger_state_at` stay NULL in M1.

D5. Mentions (03 `messages` "Inhaltsformat" and 07 4.6 are normative; this is the build view):

- The scanner implements the token rule of 03 exactly: `@` plus handle `[a-z0-9][a-z0-9._-]*`
  (case-insensitive), preceded by the start of the text or a character that is not a letter,
  digit, `_`, `.`, `-`, `@` or `/`; trailing `.`, `-` and `_` cut off. Code spans and code blocks
  (fenced and indented) as CommonMark 0.31.2 defines them are skipped; use a CommonMark parser
  for the code structure (pinned, license checked by `make licenses`) or a scanner that passes the
  code-span and code-block examples of the CommonMark specification as test fixtures.
- Each distinct handle is looked up once among active principals of kind `user` or `agent`
  (batched, scope item 7). A found person becomes a mention if `authorize(author, user.read,
  person)` allows or, for authors without `user.read` (agents, apps without `users:read`), if
  `authorize(person, thread.read, thread)` allows; a found agent if `authorize(author, agent.read,
  agent)` allows (07 4.6). Otherwise the token stays plain text. These checks are filters
  (`authorize.Filter(ctx)`, M1-28): a token that stays text writes no audit entry.
- `@everyone` and `@here` have no collective meaning; they resolve only like any other handle.
- One `mentions` row per resolved principal (`thread_id` copied), also for the author and for
  people without read access to the channel (03 `mentions`: stored, no notification later).
- `Message.mention_ids` lists the resolved IDs, sorted.

D6. System line on reopen (03 `messages`, R-DM-34; `Message.system_code` in `openapi.yaml`): a
message with `kind = 'system'`, `origin = 'system'`, `system_code = 'thread.reopened'`,
`content_md` empty, `author_principal_id` = the principal whose message reopened the thread,
`client_msg_id` = a new server UUIDv7, `silent = true`, no mentions, no `trigger_state`,
`thread_seq` directly before the reopening message. Clients render a localized sentence from the
code and `author` ("Reopened by a message from Sam"). Document the code list (one entry now) in
`internal/messaging/README.md`.

D7. Events (`kernel/eventlog.Appender`), one `message.created` per message, in this order inside
the transaction: `thread.reopened` (appended by chat), `message.created` for the system line,
`message.created` for the message. Envelope: `type`, `schema_version` 1, `actor {id, kind}`, scope
`{space_id, channel_id, thread_id}`, resource `{message, id, version: 1}`, `client_msg_id` (not for
the system line), receive action `message.read`, payload `{message: Message}` with
`attachments: []`, `reactions: []`, `components: []`, `pinned: false` and without `my_reactions`.
Truncation: if the serialized payload exceeds `sync.event_payload_max_bytes`, cut `content_md` to
the longest valid UTF-8 prefix that makes it fit, set `message.content_truncated = true` and the
envelope flag `truncated = true` (04 2.2 rule 2). REST answers are never cut. No JSON `null`.

D8. `createThread` (02 4.6): handler and action `thread.create` belong to `messaging`, because the
request carries a message and only `messaging` may compose `chat` with messages. The messaging use
case authorizes `thread.create` on the channel (`write`; read-only or archived channel gives
`403 channel_read_only`), so the proof of the route action (02 9.3) holds, and then calls
`chat.Threads.Create` (M1-55), which validates and writes without a rights check of its own.

- Validation that needs messages: `parent_message_id` must be an undeleted message whose thread is
  the main thread of the target channel; messaging loads it and passes `ParentMessage{MessageID,
  ThreadID}` to `Threads.Create`, which checks the main thread and the channel (M1-55 D1). A
  missing, deleted or foreign parent gives `400` (`/parent_message_id`, `not_in_main_thread`).
- Forum channel: `first_message` is required, else `400` (`/first_message`,
  `required_for_forum_post`). Text channel: `first_message` is optional.
- With `first_message`: one unit of work: `Threads.Create`, then `authorize(message.create)` on the
  new thread (descriptor with its known channel path, so the check does not depend on reading the
  uncommitted row), then the message flow of D1 step 5 on the new thread (sequence 1). Events:
  `thread.created`, then `message.created`. Idempotency by `first_message.client_msg_id`: the hash
  context is `"new-thread:" + channel_id + "|" + parent_message_id + "|" + title + "|" +` the sorted
  tag IDs joined by `","`. A replay with the same hash answers `200` with `ThreadCreated` (current
  thread and message) and `Idempotent-Replay: true`; another hash gives `422`.
- Without `first_message`: idempotency through `Idempotency-Key` (M1-20 middleware stores the `201`).
- The answer is `201` with `ThreadCreated`.

D9. Performance: the write path uses a fixed number of statements (one batched mention lookup, one
`mentions` insert with `unnest`), because M1-72 holds `perf.message_create_p99`.

## Tests (write them first)

Unit (`internal/messaging/domain`):

- `TestMentionScanner`: start of text, after punctuation, e-mail address not a mention, inline code,
  fenced and indented code blocks skipped, trailing dot cut, upper case handle, duplicates once,
  `@everyone` without such principal gives nothing.
- `TestRequestHash_StableAndSensitive`: same input same hash; each field change changes it; JSON
  key order of the request has no influence.
- `TestTruncateForEvent_UTF8Boundary`: multi-byte characters at the cut; result fits the limit.
- `TestOriginFromPrincipalKind`.

Integration (PostgreSQL 18, real modules, HTTP through the test server; every response checked by
the response validator):

- `TestCreateMessage_Replay200`: same request twice gives `201` then `200` with
  `Idempotent-Replay: true`, same `id` and `thread_seq`, exactly one `message.created` event.
- `TestCreateMessage_DifferentBody422`.
- `TestCreateMessage_ConcurrentSameClientMsgID`: two requests behind a barrier create one row; one
  answer `201`, the other `200` replay; the thread sequence has no gap.
- `TestCreateMessage_ThreadSeqGaplessUnderConcurrency`: many authors, one thread, committed
  sequences exactly 1..n.
- `TestCreateMessage_MaxContentBytes`: exactly `message.max_content_bytes` accepted; one byte more
  gives `413 payload_too_large`.
- `TestCreateMessage_LargePayloadTruncatedInEvent`: stored event payload within
  `sync.event_payload_max_bytes`, `truncated` and `content_truncated` set; the REST answer has the
  full content.
- `TestCreateMessage_Mentions`: visible person and readable agent resolved; agent without
  `agent.read` for the author stays text; `mention_ids` mismatch `400`; a mentioned person without
  channel access still gets a `mentions` row; an agent author mentions a person who can read the
  thread (resolved) and one who cannot (text, no row); more than `message.max_mentions` distinct
  handles give `400 validation_failed` with `errors[].code` `too_many_mentions`; `MessagePosted` carries `AuthorDeviceID` and, for
  `createThread`, `CreatedThread`.
- `TestCreateMessage_ReopensClosedThread`: closed thread; events in order `thread.reopened`,
  system `message.created`, user `message.created`; sequences n and n+1; system line fields as D6;
  `MessagePosted` twice with `ReopenedThread` on the user message.
- `TestCreateMessage_ReplySameThreadOnly`: reply into the thread accepted; ID of another thread and
  unknown ID give the same `400`.
- `TestCreateMessage_NotEnabledMembers400`: `kind` voice and card, attachments, components, run_id.
- `TestCreateMessage_OriginFromPrincipal`: person gives `human`, agent token gives `agent`
  (operator rule allows the write), app token with `messages:write` gives `app`; an `origin`
  member in the body is rejected as an unknown member (W7, M1-18).
- `TestCreateMessage_RightsAndLeaks`: read-only member `403`; outsider `404` identical to an
  unknown thread ID (ignoring `correlation_id` and `instance`); archived channel
  `403 channel_read_only` with reason `channel_archived`; replay after losing access `404`.
- `TestCreateMessage_MessagePostedVetoRollsBack`: a test subscriber registered through the wire
  test hook sees all fields; when it returns an error, no message, mention, thread sequence or
  event remains.
- `TestSeq_04_16_1_SendOnline`: with a `LISTEN event_log` connection open, `POST
  /api/v1/threads/{id}/messages` gives `201`; the notification payload is the new `seq`; the
  `sync` tail reads `message.created` with the echoed `client_msg_id`. Delivery to other clients is
  proven in M1-65.
- `TestCreateThread_ForumPost`: title source rules apply, first message has `thread_seq` 1, events
  `thread.created` then `message.created`, replay with the same `first_message` gives `200`, a
  changed title with the same `client_msg_id` gives `422`.
- `TestCreateThread_FromChannelMessage`: thread from a main thread message captures the effective
  operator; second thread for the same parent `409 conflict`; parent of another channel or deleted
  parent `400`; without `first_message` the `Idempotency-Key` replay works.
- `TestCreateThread_Validation`: forum without `first_message` `400`; text channel without parent
  `400`; text channel with tags `400`.
- `TestAZ_P1_RouteCount`: passes with the two new operations.

Commands that must pass:

```
cd services/server && go test -race ./internal/messaging/... ./internal/chat/... ./internal/identity/...
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

- [ ] `messages` and `mentions` exist as in 03 with exactly the documented deviations; the
      deferred FK from `threads.parent_message_id` exists.
- [ ] `createMessage` and `createThread` are enabled; generated code has no drift; AZ-P1 green.
- [ ] Replay, `422` and the concurrent case behave as 04 5.1.
- [ ] `thread_seq` gapless under concurrency; origin set by the server.
- [ ] Content limit enforced with `413`; events above `sync.event_payload_max_bytes` are cut with
      `truncated`.
- [ ] Mentions follow D5; the reply rule holds.
- [ ] A message in a closed thread reopens it and writes the system line of D6.
- [ ] `MessagePosted` carries the fields of the port type and can veto.
- [ ] The test for sequence diagram 04 16.1 is green up to the log tail.
- [ ] The module README describes D5; D5, D6 and D8 match 03, 07 4.6, 02 4.6 and
      `openapi.yaml` (changed only where proven wrong).
- [ ] `make check` is green.

## Definition of done

- Code, tests and documentation in the same change: `internal/messaging/README.md` (status, ports,
  tables, actions, events, in-transaction events, system codes), doc comments on every public
  symbol, comments explain why.
- No architecture decision beyond the rulings; otherwise an ADR (MADR 4, next free number).
- `services/server/CHANGELOG.md` entry under "Unreleased".
- `make check` green on the final commit.
- Local review (`make review` plus a code review over the diff); red team by a fresh read-only
  reviewer with `docs/quality/red-team-gate.md` over `git diff <base>...HEAD`, with attention to
  replay leaks, mention probing and payload limits; record the verdict line; on `NEIN` fix, test,
  re-check the delta.
- Merge locally into the base branch only with clean review, `Urteil: JA` and green
  `make check`; then remove the worktree.

## Report

Write `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits and files with line counts.
3. Test results with counts and the red output of the first failing tests.
4. Review and red team verdicts.
5. What is not proven (delivery over the gateway, M1-65; latency target, M1-72).
6. Only real owner decisions, each with a recommendation.

Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
