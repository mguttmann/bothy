# M1-61: Read state, drafts and personal settings

Status: ready | Depends on: M1-56 | Etappe: M1

## Goal

The `sync` module owns the personal sync tables `read_states`, `drafts` and `user_settings` and serves the twelve REST operations that read and write them. Every change writes one durable personal event (`audience = self`), so all devices of a person converge in server commit order: the read position only moves forward unless it is reset with `If-Match`, drafts follow last write wins with the `overwrote_newer` hint, settings follow last write wins per key. Sending a message with `clear_draft: true` removes the draft in the same transaction, and mentions raise the unread mention counter of the mentioned people. This is the cross-device sync of 01 'Offline, Sync, Geräte' and the personal data that the snapshot (M1-63) and the gateway (M1-64) deliver.

## Read first

- `docs/spec/04-sync-protokoll.md`: 'Geräteübergreifende Synchronisation' (rules per data kind), 'Umschlag (Envelope)', 'Zustellgarantie', 'Idempotente Schreibvorgänge', 'Bearbeiten und Löschen'.
- `docs/spec/03-datenmodell.md`: tables `read_states`, `drafts`, `user_settings`, `threads` (`last_message_seq`, `unread_floor_seq`), `mentions`, `messages`, `user_space_states`; 'Grundsätze' points 1, 10, 11, 12, 13; 'Transaktionsregeln' with 'Sperrreihenfolge'.
- `docs/spec/05-aktionen.md`: section 'Persönliche Daten, Benachrichtigung, Sync' (actions `read_state.update`, `read_state.mark_unread`, `draft.read`, `draft.write`, `settings.read`, `settings.update`); 'Spalte Recht' (`self`); 'Verbindliche Regeln' rules 1, 2 and 8; rows `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated` in 'Dauerhafte Ereignistypen und Empfangs-Aktion'; R-AZ-21.
- `docs/spec/01-produkt.md`: 'Einstellungen der Person' (normative keys, values, defaults).
- `docs/spec/13-push-benachrichtigungen.md`: 'Persönliche Schalter' (normative `notify.*` keys).
- `docs/spec/02-architektur.md`: 'Ports, Ereignisse und Quellen je Modul' (`sync` ports `ReadStates`, `Drafts`, `Settings`; subscription `messaging.MessagePosted`), 'Erlaubte Abhängigkeiten', 'Schichten im Modul', 'Transaktionen und Unit of Work', 'In-Tx-Ereignisse', 'Idempotenz im Unit of Work', 'Request-Pfad', '404 statt 403'.
- `docs/spec/06-api.md`: 'Grundregeln' (no `null`), 'Fehler', 'Idempotenz' (rows for PUT and DELETE), 'Nebenläufigkeit' points 1, 2 and 4, endpoint table 'Preferences'.
- `docs/spec/07-rechte.md`: 'Die Funktion authorize', 'Sichtbarkeit', 'Abfrage-Muster' (rule 3: SQL filter plus `authorize` per element).
- `docs/spec/12-dateien.md`: 'Anhängen und Agenten-Aufruf' point 4.
- `docs/spec/20-datenschutz.md`: test DP-T10.
- `docs/api/openapi.yaml`: operations `listMyReadStates`, `putReadState`, `markThreadUnread`, `putThreadDraft`, `deleteThreadDraft`, `putChannelDraft`, `deleteChannelDraft`, `listMyDrafts`, `listMySettings`, `putMySetting`, `deleteMySetting`, `getMySidebar`; schemas `ReadState`, `ReadStatePut`, `ReadStateList`, `Draft`, `DraftPut`, `DraftPutResult`, `DraftList`, `UserSetting`, `UserSettingPut`, `UserSettingList`, `SidebarStateList`, `ReadStatePayload`, `DraftPayload`, `DraftDeletedPayload`, `SettingsPayload`, `MessageCreate` (`clear_draft`), `ThreadCreate`.
- ADRs 0009, 0013, 0015, 0017, 0021 and, if present, 0052 and 0053.
- `docs/PLAN.md`: 'Arbeitsweise je Ticket', the M1 rows M1-56, M1-60, M1-62 and M1-63; `docs/quality/gates.md`; `CONTRIBUTING.md`.
- Code from earlier tickets under `services/server/`: `internal/sync` (appender, idempotency store), `internal/kernel/*`, `internal/chat` (threads, sidebar), `internal/messaging` (`MessagePosted`, mentions), `internal/authz`, `api/operations.yaml`, `architecture/modules.yaml`, `internal/wire`.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc comment (purpose, parameters, return value, errors, example where useful); comments explain why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a CHANGELOG entry per release, docs site kept current. A change without matching documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossar.md`, `03-datenmodell.md`, `04-sync-protokoll.md`, `05-aktionen.md`) is not edited by this ticket.

Product requirements implemented here:

11. The read position is monotonic: `last_read_seq = GREATEST(old, new)`; moving it back is the separate operation `markThreadUnread` with `If-Match` (04 'Geräteübergreifende Synchronisation', R-SY-13).
12. Drafts: last write wins in server order, every write raises `version`; a `base_version` older than the current version answers `overwrote_newer: true`; an empty draft is deleted (03 `drafts`, 04 same section, R-SY-12).
13. Settings: last write wins per key; only the keys and values of 01 'Einstellungen der Person' and 13 'Persönliche Schalter'; unknown keys and values outside the list are rejected (03 `user_settings`).
14. Every change writes a durable personal event with `audience = self`: `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated` (04, 05). Payloads carry `updated_by_device_id` of the writing device, so a device recognises its own echo (04 'Geräteübergreifende Synchronisation', last sentence).
15. Sending with `clear_draft: true` deletes the draft in the same transaction as the message (04, 12 'Anhängen und Agenten-Aufruf' point 4).
16. `read_states.mention_count` holds the unread mentions (03 `read_states`); `sync` maintains it through the subscription on `messaging.MessagePosted` (02 'Ports, Ereignisse und Quellen je Modul').
17. Every operation runs through `authorize` with its catalogue action; no read right gives `404`, never `403` (B7, 05 rules 1 and 2). Nobody but the owner can read a read state (20, DP-T10).
18. No JSON `null` in responses or events (06 'Grundregeln', 04 'Umschlag (Envelope)').

## Scope

Create, all paths under `services/server/`:

- Migration `migrations/<next>_sync_personal_data.sql` (goose; next sequential version after the newest existing file, same naming pattern; never edit an existing migration): tables `read_states`, `drafts` and `user_settings` exactly as in 03 (columns, types, defaults, checks, foreign keys with their `ON DELETE`, the partial unique indexes `drafts (user_id, thread_id) WHERE thread_id IS NOT NULL` and `drafts (user_id, channel_id) WHERE thread_id IS NULL`, the index `read_states (user_id, channel_id)`).
- `internal/sync/domain/readstate.go` (advance, mark unread, range checks), `internal/sync/domain/draft.go` (emptiness, validation, the `overwrote_newer` rule), `internal/sync/domain/settings.go` (the key registry: `type SettingSpec struct { Key string; Kind SettingKind; Values []string }`, `func KnownSetting(key string) bool`, `func ValidateSetting(key string, value json.RawMessage) error`).
- `internal/sync/app/readstates.go`, `drafts.go`, `settings.go`, `sidebar.go` and `subscribers.go` (with `OnMessagePosted`).
- Port additions in the root package `internal/sync` (own DTOs, no domain types; actor and device come from `kernel/actor` in the context):
  - `ReadStates`: `Advance(ctx, AdvanceReadState{ThreadID, LastReadSeq}) (ReadState, error)`, `MarkUnread(ctx, MarkUnread{ThreadID, LastReadSeq, IfMatchVersion}) (ReadState, error)`, `ListForChannel(ctx, channelID) ([]ReadState, error)`.
  - `Drafts`: `PutThreadDraft(ctx, PutDraft) (DraftPutResult, error)`, `PutChannelDraft(ctx, PutDraft) (DraftPutResult, error)`, `DeleteThreadDraft(ctx, threadID) error`, `DeleteChannelDraft(ctx, channelID) error`, `List(ctx) ([]Draft, error)`.
  - `Settings`: `List(ctx) ([]UserSetting, error)`, `Put(ctx, key string, value json.RawMessage) (UserSetting, error)`, `Delete(ctx, key string) error`, `ListSidebar(ctx) ([]SidebarState, error)` (the sidebar is read under `settings.read`, R-AZ-21).
- `internal/sync/adapters/postgres/queries/read_states.sql`, `drafts.sql`, `user_settings.sql` with the sqlc output and the repositories that implement the outgoing ports of `app`.
- `internal/sync/adapters/httpapi/personal.go`: the strict-server methods for the twelve operations named in 'Read first'.

Change:

- `api/operations.yaml`: move the twelve operations from `later` into the enabled list and rerun the route generator.
- `internal/wire`: register the handlers and the in-transaction subscription `messaging.MessagePosted -> sync.OnMessagePosted`; update the golden file of subscriptions (mechanism of M1-13) in the same commit.
- `internal/messaging` port, additive only: `MessagePosted` must carry `MessageID`, `ThreadID`, `ChannelID`, `ThreadSeq`, `AuthorID`, `AuthorDeviceID` (absent when the author has no device), `Origin`, `MentionedUserIDs` (people only, author excluded, without duplicates), `ClearDraft` and `CreatedThread` (true when the message is the first message of a thread created in the same transaction). Add what M1-56 did not provide and set it in the create use cases. Add `Mentions.CountAfter(ctx, userID, threadID uuid, afterThreadSeq int64) (int, error)` (mentions of the person in the thread whose message has `thread_seq > afterThreadSeq`; deleted messages have no mention rows, 03 `messages`) unless M1-60 already offers an equivalent method.
- `internal/chat` port, additive only, unless M1-52 or M1-55 already offer them: `Threads.InfoForShare(ctx, threadID) (ThreadInfo, error)` returning `ChannelID`, `SpaceID`, `ChannelKind`, `LastMessageSeq` and `UnreadFloorSeq` under `FOR SHARE` on the thread row, and `Structure.ListSidebarStates(ctx, userID) ([]SidebarState, error)`.
- `docs/spec/01-produkt.md`, section 'Einstellungen der Person': in the row `layout.column_widths` replace the value 'Objekt' with the shape from Design note 14.
- READMEs `internal/sync/README.md`, `internal/messaging/README.md`, `internal/chat/README.md`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md`.

## Out of scope

- `putMySidebarSpace` and the table `user_space_states` (M1-52); this ticket only reads them through the chat port.
- Moving threads and the subscription `chat.ThreadMoved` that updates `read_states.channel_id` and `drafts.channel_id` (M1-62).
- `GET /api/v1/events`, the snapshot and the per-recipient filter for personal events (M1-63); gateway delivery (M1-64, M1-65).
- The mention list and templates (M1-60); notification preferences and quiet hours (`notify`, M3).
- Checking that `drafts.file_ids` belong to the person and are attachable: `sync` has no dependency on `files` (02 'Erlaubte Abhängigkeiten'); attachability is checked when the message is sent (12 'Anhängen und Agenten-Aufruf').
- Export and deletion of these tables for data subject requests (`kernel/personaldata.Participant`, M7, P-14).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-61-sync-personal-data`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(sync): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).

## Design notes

Read state

1. `putReadState`: first `authorize(actor, read_state.update, thread)` (unknown or invisible thread gives `404`). Then `0 <= last_read_seq <= threads.last_message_seq`, otherwise `400 validation_failed` with `errors: [{pointer: /last_read_seq, code: out_of_range}]`. Store `GREATEST(old, new)`. If the stored value does not change, answer `200` with the current state, write no event and keep `version`. A missing row is the implicit state `last_read_seq = 0`, `mention_count = 0`, `version = 0`; advancing a missing row to `0` writes nothing.
2. On a real change: `version = version + 1`, `updated_by_device_id` = device of the actor, `updated_at = now()`, `mention_count = Mentions.CountAfter(user, thread, new last_read_seq)`, `channel_id` from `Threads.InfoForShare`, one `read_state.updated`.
3. `markThreadUnread`: `If-Match` is required (`428 precondition_required` when missing; use the M1-18 helpers). The ETag of a read state is its `version` in the format of 06 'Nebenläufigkeit' point 1; a missing row has version `0`. Mismatch gives `412 version_conflict` with the current `ReadState` in `current`. The body value must be at most the current `last_read_seq`, otherwise `400 validation_failed` (`out_of_range`; advancing is `putReadState`). An equal value answers `200` without event. Otherwise store exactly that value, raise `version`, recount `mention_count`, write the event and answer with the `ETag` header. The middleware of M1-20 handles `Idempotency-Key`.
4. Unread counts are never stored: unread is `threads.last_message_seq - GREATEST(last_read_seq, threads.unread_floor_seq)` (03 `read_states`), computed by readers (M1-63).
5. `OnMessagePosted` runs in the message transaction. Skip mention counting when `Origin = import` (imported content never counts as unread, 04 'Schreiben eines Imports'). For each id in `MentionedUserIDs`, in ascending id order (03 'Sperrreihenfolge'), upsert `read_states (user, thread)` with `mention_count = mention_count + 1`, `version = version + 1`, `channel_id = ChannelID`, `updated_by_device_id` absent (a server-side change), and write `read_state.updated` for that person. The server does not move the author's own read position; clients advance it over REST (04: personal data changes over REST). Record as ruling.
6. `listMyReadStates` (`settings.read` on the own user): rows of the given channel only if the actor can read the channel (visible set from `Authorizer.Scope`), each row additionally checked with `authorize(actor, thread.read, thread)` and dropped on deny (07 'Abfrage-Muster' rule 3). An invisible and an unknown channel give the same `200` with `items: []`. Every query on the three tables has the predicate `user_id = <actor>`.

Drafts

7. `putThreadDraft` and `deleteThreadDraft`: `authorize(actor, draft.write, thread)`. `putChannelDraft` and `deleteChannelDraft`: `authorize(actor, draft.write, channel)`; the channel must have `kind = forum` (a text channel drafts on its main thread), otherwise `400 validation_failed` with a `detail`; `reply_to_message_id` is not allowed on a channel draft (`400`, pointer `/reply_to_message_id`). A read-only channel answers `403 channel_read_only` from `authorize` (05 rule 8; `draft.write` needs `write`).
8. Validation: `content_md` at most `message.max_content_bytes` bytes (`400 validation_failed`, pointer `/content_md`, code `too_long`; if M1-56 answers an over-long message with another status or code, use exactly that and say so in the report); `file_ids` without duplicates and at most `message.max_attachments` entries, each a UUIDv7 (`400`, pointer `/file_ids/<index>`); `reply_to_message_id` must name a message of the same thread, checked through the messaging port, otherwise `400`, pointer `/reply_to_message_id`, code `not_in_thread`, identical for unknown and foreign ids.
9. Empty draft: `content_md` without any non-whitespace character and no `file_ids`. A PUT of an empty draft deletes the row like DELETE, writes `draft.deleted` if a row existed, and answers `200` with `draft` as the final state (`content_md` empty, `file_ids` empty, `version` = last version plus one, or `1` if none existed) and `overwrote_newer` per note 10. Record as ruling.
10. `overwrote_newer = row exists AND (base_version absent OR base_version < row.version)`. Create: new UUIDv7 `id`, `version = 1`; update: `version + 1`; `channel_id` of a thread draft from `Threads.InfoForShare`. Concurrent first writes must not fail on the unique index: use `INSERT ... ON CONFLICT ... DO UPDATE` on the partial unique index and compute `overwrote_newer` from the version seen under the row lock.
11. Deleting: delete if present and write `draft.deleted` with `DraftDeletedPayload`; answer `204` also when there was no draft (06 'Idempotenz').
12. `clear_draft` in `OnMessagePosted`: if `ClearDraft`, then when `CreatedThread` is true and the channel is a forum, delete the author's channel draft (`thread_id IS NULL`) of that channel; otherwise delete the author's draft of `ThreadID`. Write `draft.deleted` when a row was deleted. A rollback of the message transaction keeps the draft.
13. `listMyDrafts` (`draft.read`): own drafts whose channel is still readable (visible set plus `authorize(actor, thread.read, thread)` for thread drafts or `authorize(actor, channel.read, channel)` for channel drafts). Drafts of lost channels stay stored (class `personal_sync`) but are not listed.

Settings

14. The registry holds exactly the keys of 01 'Einstellungen der Person' and 13 'Persönliche Schalter'. Enumerations (`appearance.theme`, `appearance.density`, `appearance.reduce_motion`, `notify.default_level`, `notify.preview_mode`) accept exactly the listed strings. Booleans (`composer.enter_sends`, `privacy.crash_reports`, `privacy.client_metrics`, `notify.input_needed_breaks_quiet_hours`, `notify.admin_enabled`) accept JSON `true` or `false`. `layout.column_widths` accepts a JSON object whose member names match `^[a-z][a-z0-9_]*$` and whose values are non-negative integers (logical pixels); write this shape into 01 in the same change. JSON `null` is never a valid value.
15. `putMySetting`: unknown key `404 not_found` (the path names no resource); invalid value `400 validation_failed`, pointer `/value`. A value equal to the stored one answers `200` without event and without version change. Otherwise insert with `version = 1` or update with `version + 1`, set `updated_by_device_id`, write `settings.updated` with `SettingsPayload {setting}`.
16. `deleteMySetting`: unknown key `404`; known but unset `204`; set: delete the row, write `settings.updated` with `SettingsPayload {setting: {key, value: last stored value, version: last version + 1}, deleted: true}`, answer `204`. Defaults are neither stored nor returned; clients apply the defaults of 01 and 13 because some depend on the device or the instance (`composer.enter_sends`, `privacy.crash_reports`). A later PUT of the key starts again at `version = 1`; clients drop the local entry on `deleted: true`, so the new entry applies. Record as ruling.
17. `listMySettings`: all stored rows of the person.
18. `getMySidebar` (`settings.read`): the person's `user_space_states` from `Structure.ListSidebarStates`, only for spaces that are not archived and that the actor may read (`space.read`), as `SidebarStateList`.

Events

19. All durable, `audience_principal_id` = the person, appended through `kernel/eventlog.Appender` in the same unit of work, receive action and event types from the generated catalogue constants (M1-02, M1-25):

| Type | `resource` | `scope` | Payload |
|---|---|---|---|
| `read_state.updated` | `{kind: thread, id: thread_id}` without `version` | space, channel, thread | `ReadStatePayload` |
| `draft.updated` | `{kind: draft, id, version}` | space, channel, thread (thread absent for channel drafts) | `DraftPayload` |
| `draft.deleted` | `{kind: draft, id, version: last + 1}` | as `draft.updated` | `DraftDeletedPayload` |
| `settings.updated` | `{kind: user, id: user_id}` without `version` | empty | `SettingsPayload` |

20. Why `resource.version` is absent on `read_state.updated` and `settings.updated`: the resource kinds `thread` and `user` have their own object versions, and a per-read-state or per-key version in the same field would make clients drop later updates (04 'Zustellgarantie' point 3). Clients compare the version inside the payload. Record as ruling and report it as a spec gap of 04.
21. If an encoded `DraftPayload` exceeds `sync.event_payload_max_bytes`, cut `content_md` at a UTF-8 boundary so that the payload fits and set `truncated: true`, with the same mechanism M1-56 uses for `message.created`; clients reload through `listMyDrafts`.

Rights, concurrency, limits

22. Agents and apps cannot use these operations (they act on a person's own data). `authorize` must answer `403 forbidden` with reason `subject_kind_not_allowed` for an agent or app token. If the requirement step of M1-28 does not yet do this for `self` actions, add the case in `internal/authz` with a unit test; rights logic lives only in `authz`.
23. `Threads.InfoForShare` takes `FOR SHARE` on the thread row, so a concurrent move (M1-62 updates that row) either commits first and the copied `channel_id` is the new channel, or waits. Several `read_states` rows are written in ascending `user_id`.
24. Rate limits and idempotency keys come from the HTTP chain (M1-19, M1-20); do not reimplement them.
25. Configuration keys used (all listed in `docs/operations/configuration.md`): `message.max_content_bytes`, `message.max_attachments`, `sync.event_payload_max_bytes`. If the schema of M1-07 lacks one, add it from its chapter table and run `make config-doc-check`.

Rulings for `docs/DECISIONS.md`: notes 5 (author's read position), 9 (empty draft PUT), 14 (shape of `layout.column_widths`), 16 (defaults on the client, DELETE semantics) and 20 (no `resource.version` on two personal event types).

## Tests (write them first)

Unit, `internal/sync/domain`:

- `TestAdvanceKeepsMaximum`, `TestAdvanceRejectsSeqAboveLastMessage`, `TestMarkUnreadOnlyMovesBack`.
- `TestOverwroteNewer`: table with no row; row and base absent; base equal; base older; base newer.
- `TestDraftEmptiness`: whitespace only, files only, reply only.
- `TestDraftValidation`: content over `message.max_content_bytes`, too many and duplicate `file_ids`, reply on a channel draft.
- `TestSettingValues`: per key valid and invalid values, including `null`, numbers for booleans and nested objects in `layout.column_widths`.
- `TestSettingRegistryMatchesSpec`: reads the tables in `docs/spec/01-produkt.md` 'Einstellungen der Person' and `docs/spec/13-push-benachrichtigungen.md` 'Persönliche Schalter' (find the repository root from the test file path), extracts keys and enumerated values and fails on any difference to the registry.

Unit, `internal/sync/app`, with fakes for the chat and messaging ports, `kernel/authorize` and the repositories:

- `TestOnMessagePostedImportDoesNotCount`, `TestOnMessagePostedSkipsAuthor`, `TestOnMessagePostedWritesInIDOrder`, `TestOnMessagePostedClearDraftThreadAndForumPost`.

Integration (PostgreSQL 18 via `internal/platform/pg/pgtest`, real modules through `internal/wire`, HTTP through the real chain, people with active devices and DPoP proofs from the identity test helpers of M1-33 and M1-34):

- `TestPutReadStateMonotonic`: advance to 10, then to 5: the value stays 10, the second call writes no event and keeps `version`.
- `TestReadStateEventIsPersonal`: the `event_log` row has `type = read_state.updated`, `audience_principal_id` = the person, the scope ids of the thread, a payload valid against `ReadStatePayload`, `updated_by_device_id` = the calling device, no JSON `null`.
- `TestMarkThreadUnreadPreconditions`: missing `If-Match` gives `428`; stale gives `412` with `current`; success returns `ETag` and writes the event.
- `TestMarkThreadUnreadTwoDevices`: two devices send the same `If-Match` in parallel: exactly one `200` and one `412`.
- `TestMentionCount`: Sam mentions Alex in two messages: Alex has `mention_count = 2` and two events; Alex advances past the first message: `1`; past both: `0`.
- `TestDraftsTwoDevicesServerOrder`: device A puts without base: version 1, `overwrote_newer = false`; device B puts with base 1: version 2, false; A puts with base 1: version 3, `overwrote_newer = true`.
- `TestDraftsConcurrentFirstWrite`: two parallel first PUTs without base: both `200`, one row, versions 1 and 2, exactly one response with `overwrote_newer = true`.
- `TestClearDraftSameTransaction`: a draft exists; `createMessage` with `clear_draft: true` removes it; `draft.deleted` and `message.created` carry the same `correlation_id`; a unit of work that runs `OnMessagePosted` and then rolls back keeps the draft.
- `TestClearDraftForumPost`: `createThread` in a forum with `first_message.clear_draft: true` deletes the channel draft.
- `TestEmptyDraftPutDeletes`, `TestDraftReadOnlyChannel` (`403 channel_read_only`), `TestDraftEventTruncated` (content near `message.max_content_bytes`: payload within `sync.event_payload_max_bytes`, `truncated = true`).
- `TestSettingsPutValidation` (`404` unknown key, `400` invalid value), `TestSettingsPutSameValueNoEvent`, `TestSettingsDeleteWritesDeletedEvent`, `TestSettingsDeleteUnset204`.
- `TestDPT10ForeignReadStateNotRetrievable`: Sam sees none of Alex's read states through any of the twelve operations; Alex's events carry `audience_principal_id` = Alex (the per-recipient filter is tested in M1-63).
- `TestListMyReadStatesInvisibleChannel`: a foreign and an unknown channel give identical status, body and headers.
- `TestListMyDraftsHidesLostChannels`: after a grant is revoked the draft is no longer listed and is still stored.
- `TestGetMySidebarVisibleOnly`.
- `TestPersonalOperationsRejectAgentToken`: `403` on each of the twelve operations for an agent token.
- If the response validator of G8 exists, all integration tests run with it.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/sync/... ./internal/messaging/... ./internal/chat/... ./internal/authz/... ./internal/wire/...`
- `make codegen-check`, `make arch`, `make migrations-lint`, `make api-lint`, `make config-doc-check`, `make register-check`
- `make check`

## Acceptance criteria

- [ ] The migration creates `read_states`, `drafts` and `user_settings` exactly as in 03; `make migrations-lint` is green and a second `bothy migrate up` changes nothing.
- [ ] The twelve operations are in the enabled list of `api/operations.yaml` and registered; the route count test (AZ-P1) passes.
- [ ] `putReadState` never lowers `last_read_seq`; `markThreadUnread` without `If-Match` gives `428`, with a stale one `412` with `current`.
- [ ] Draft writes from two devices end in server order with `overwrote_newer` exactly as in note 10, also for concurrent first writes.
- [ ] `clear_draft: true` removes the draft in the message transaction; a rollback keeps it.
- [ ] Unknown setting keys give `404`, invalid values `400`; the registry test against 01 and 13 passes.
- [ ] Every real change writes exactly one personal event of the right type (plus one per mentioned person); a no-op writes none; payloads validate against their schemas and carry `updated_by_device_id` when a device wrote.
- [ ] DP-T10 passes; invisible and unknown ids give identical responses.
- [ ] Agent and app tokens are rejected with `403` on all twelve operations.
- [ ] The golden file of in-transaction subscriptions contains `messaging.MessagePosted -> sync`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `sync`, `messaging` and `chat` updated (status, ports, tables, actions, events, subscriptions).
- `docs/spec/01-produkt.md` updated for `layout.column_widths`; rulings recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-vorlage-madr-4.md`, headings English, text German, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Merge locally into the base branch only if review, red team and `make check` are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
