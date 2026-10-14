# M8-11: Thread, message, pin and file items of the import interface

Status: ready | Depends on: M1-59, M2-18, M3-07, M3-13, M8-03 | Milestone: M8

## Goal

After this ticket `applyImportBatch` writes the content of a source: a batch with `thread_origin_key` concerns exactly one thread (`thread:<id>` or `channel:<id>` for the main thread of a text channel) and is one transaction; its optional first item creates the thread (title source decided by the placeholder rule of M2-18, operator from `planned_operator` of the stored mapping, times and done state from the source, starter message link, forum tags) or, when the thread is already `posted`, applies only the mirrored changes of 22 12.2 step 3 (title only while `title_source = auto`, `closed` and `open`, `last_activity_at`); its message items become messages with `origin = import`, `silent = true`, no `mentions` rows, no `trigger_state`, the author mapped through the posted user item, `created_at` and `edited_at` from the source, `thread_seq` appended in request order, `import_item_id` set, reply links inside the thread and attachments from posted file items; its pin items pin messages with the importing owner as `pinned_by`. At the end of the batch the server sets `threads.unread_floor_seq` to the thread's `last_message_seq` and appends exactly one `thread.updated` (when something changed) and one `import.progress`, never `message.created`, never a notification, push, run or `agent.invocation` (04 15.8, 22 6.4, R-IM-05). `createImportFile` reserves the file ID of an attachment under its origin key (`pending`), returns an upload session of the importing owner without quota whose scan jobs run with the lowest priority, returns the same ID on every repeat, and marks the item `posted` when the bytes are complete (22 6.5, 7.2 item 3). Why: the import tool (M8-04) and the read mirror (M8-05) copy history and new Discord messages through this path, and imported content must never trigger, notify or appear as unread (22 16 rows "Import never triggers", "No notification").

## Read first

- `docs/spec/22-import-migration.md`: 5.1.1 (threads: starter message, forum posts, title, times, done rule, operator), 5.3 (mentions become text), 6 (6.1 types, 6.2 fields, 6.3 content is the tool's job, 6.4 no notifications, 6.5 attachments items 3 and 5, 6.6 pins and tags), 7.2 (items 1 to 4), 8, 12.2 step 3, 16 (rows "Import never triggers", "No notification", "Archive rule", "Stored, then error"), 17 (R-IM-04, R-IM-05, R-IM-08, R-IM-11), 18 (K-IM-3, K-IM-4).
- `docs/spec/03-data-model.md` (core contract, read only): "What the import sets differently" after `import_items`, `messages` (`origin`, `silent`, `trigger_state`, `import_item_id`, `client_msg_id`, `thread_seq`, `has_attachments`, `search_language`), `threads` (`operator_agent_id`, `title_source`, `parent_message_id`, `created_at`, `last_activity_at`, `closed_at`, `unread_floor_seq`, `last_message_seq`), `thread_tags`, `message_attachments`, `pins`, `read_states`, `files`, `file_upload_sessions`, R-DM-12.
- `docs/spec/04-sync-protocol.md` 15.8; `docs/spec/05-actions.md` (core contract, read only) rule 1.8, 4.12 `import.apply`, 5 rule 1 (imports never trigger).
- `docs/spec/12-files.md`: 3 (upload sessions, completion, 3.4 the processing path), 10 (attachments), 11 ("Import attachments").
- `docs/spec/09-agents.md` 12 (auto-title), `docs/spec/14-search.md` (language of imported messages).
- `docs/api/openapi.yaml`: `applyImportBatch`, `createImportFile`, `ImportBatch`, `ImportObject`, `ImportFileCreate`, `ImportFileSlot`, `UploadSession`, `uploadPart`, `completeUpload`, `ThreadUpdatedEvent`.
- `docs/DECISIONS.md`: D-IM-05, D-IM-07, D-K-I3, D-K-I4, D-W-100, D-W-120, D-W-124; the rows `D-M8-03-<n>`; section 10 format.
- Briefs (merged names win, D-W-100): `M8-03-import-server.md` (batch framework, ports `chat.ImportStructure`, `identity.ImportPeople`, origin keys, `source_changed`), `M1-55-chat-threads.md` (`chat.Threads`, thread tags), `M1-56-messaging-send.md` (insert path, `thread_seq`, the deferred FK), `M1-59-messaging-reactions-pins.md` (pins), `M2-18-auto-title.md` (`title.Placeholders.IsPlaceholder`, `SourceAtCreation`), `M3-03-upload-sessions.md` (sessions, parts, completion, quota), `M3-04-clamd-scan.md` (scan jobs), `M3-07-attachments-invocation.md` (`message_attachments`, `has_attachments`, the attach rules), `M3-13-search-vectors.md` (`search_language` on every insert, the static insert tests).
- Code on the base branch (all under `services/server/`): `internal/importer`, `internal/messaging`, `internal/chat`, `internal/files`, `internal/notify`, `internal/runs`, `internal/wire`, `api/operations.yaml`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Discord IDs are invented digit strings.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.

Product requirements implemented here:

11. Imported messages: `origin = import`, `kind = text`, `silent = true`, no `mentions` rows, no `trigger_state`, `client_msg_id` generated by the server, `import_item_id` set, `created_at` and `edited_at` from the data, `thread_seq` appended in request order (22 6.2, 03).
12. `origin = import` never triggers: no run, no `agent.invocation`, no `agent.followup`, no follow-up, also in a channel that has an operator and is not read-only (05 5 rule 1, 22 6.4 item 4).
13. No notification: no push, no notification row, no `message.created`; one `thread.updated` per batch and thread; `threads.unread_floor_seq` = end of the batch (04 15.8, 22 6.4).
14. Threads: title cut to `thread.title_max_chars`, `title_source` by the placeholder rule, `operator_agent_id` from `planned_operator` (never the effective operator), times and done state from the data, `parent_message_id` only when the starter message is posted, forum tags from posted tag items (22 5.1.1, R-IM-08, R-IM-11).
15. A posted thread changes only by the mirror rules of 22 12.2 step 3; a posted message or pin never changes (22 7.2 item 1, 12.4).
16. Files: `pending` reserves the file ID before the bytes; a repeat with the same key returns the same ID; `posted` when complete; no quota; the normal scan and processing path; `uploaded_by` is the mapped author (22 6.5, 7.2 item 3, 12 11).
17. Read-only channels accept these writes because they come through `import.apply` (05 rule 1.8).

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `internal/importer/domain/content.go` and `app/content.go`: validation and application of thread, message and pin items in a thread batch (Design notes 1 to 5).
- `internal/importer/app/files.go` and the handler `createImportFile` (note 6); the `files` subscriber that marks a file item `posted` (note 6 d).
- Ports in the owning modules (root packages, implemented in their `app`, note 7): `chat.ImportThreads` (create a thread with import fields, apply mirror changes, set `unread_floor_seq`, append `thread.updated`), `messaging.ImportMessages` (append imported messages, link replies, attach files, pin), `files.ImportFiles` (reserve a file with an upload session of import mode, report completion).
- Tests named under "Tests".

Change:

- `internal/importer/app/batch.go` of M8-03: remove the `not_enabled` answers for `thread`, `message`, `pin` and `thread_origin_key`.
- `api/operations.yaml`: enable `createImportFile`; regenerate.
- `docs/api/openapi.yaml`: component schemas `ImportThreadData`, `ImportMessageData`, `ImportPinData` (note 2); `ImportBatch.thread_origin_key` description (`thread:<id>` or `channel:<id>`); `createImportFile` description (owner's upload session, no quota); `make api-lint`, `make api-checks`, `make codegen-check`.
- `internal/files`: import mode of upload sessions (no quota, `created_by` the importing owner, `uploaded_by` the author) and scan jobs with the lowest job priority for import files (note 6 c).
- `docs/spec/12-files.md` 11 and `docs/spec/02-architecture.md` 12.2 (the priority instead of a separate queue, note 6 c); `docs/spec/22-import-migration.md` 7.2 (the mirror changes of a posted thread, note 3).
- READMEs of `importer`, `chat`, `messaging`, `files`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M8-11-1` to `D-M8-11-7`.

This ticket adds no table, no migration, no action, no event type, no configuration key and no top-level make target.

## Out of scope

- Converting Discord content to Markdown, quote lines for replies across threads and jump links for failed attachments: the tool does it (M8-04, 22 6.3, 6.5 item 6); the server stores the given Markdown.
- Earlier session rows on threads (M8-06).
- Reactions (never taken over, 22 6.6); keeping edits in sync (22 12.4).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-11-import-content-items`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(importer): ...`, `feat(messaging): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest`, clamd only as the test container of M3-04 (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Never contact Discord or any other real service: the server never calls Discord (R-IM-01), and every test uses only fixtures and fakes (D-W-124).
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `t.TempDir()`, removed at the end.
- Never read or print secrets. Logs carry no content, no file name and no Discord ID.
- Time only through `kernel/clock`; tests use a fake clock.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Shape of a thread batch** (22 8; Ruling for docs/DECISIONS.md). `thread_origin_key` is `thread:<digits>` or `channel:<digits>` (the main thread of a posted text channel; a forum channel has no main thread, `400 validation_failed`). For `thread:` keys the first item may be the thread item with the same key; it must be posted before or created by this batch, else `409 conflict` with `reason` `thread_not_imported`. All other items are `message` or `pin` items of this thread; any other kind is `400 validation_failed` at `/items/<i>/item_kind`. Message items keep request order; the server appends them in that order and never sorts (the tool sends ascending Discord IDs, 22 6.2).

2. **Data schemas** (`openapi.yaml`, spec first, all `additionalProperties: false`):
    - `ImportThreadData{channel_origin_key, name, created_at, last_activity_at, closed (boolean), closed_at?, starter_message_origin_key?, tag_origin_keys[]?}`.
    - `ImportMessageData{author_origin_key, content_md, created_at, edited_at?, reply_to_origin_key?, attachment_file_ids[]?}`; `content_md` within `message.max_content_bytes` (no split, 22 6.3).
    - `ImportPinData{message_origin_key}`.

3. **Thread item** (22 5.1.1, 12.2 step 3, 03; Ruling for docs/DECISIONS.md).
    a. New: channel from the posted channel item (must be a channel of this source); title = `name` cut to `thread.title_max_chars` (rune-safe); `title_source = auto` when `title.Placeholders.IsPlaceholder(name)` of M2-18, else `manual`; `operator_agent_id` = the agent of `planned_operator` of that channel in the stored mapping (resolved through `identity.ImportPeople.Resolve`; `none` gives NULL), never the channel's effective operator (R-IM-08); `created_at`, `last_activity_at` from the data; `closed` with `closed_at` (the tool applies the archive rule of 22 5.1.1, R-IM-11); `parent_message_id` = target of `starter_message_origin_key` when that message item is posted and lies in the main thread of the same channel, else NULL; `thread_tags` from posted `forum_tag` items of the same channel. `kind = thread`. No `thread.created` event (the batch writes `thread.updated`, note 5).
    b. Posted: compare with the stored thread and apply only: title when `title_source = auto` and the cut name differs (the source of the title stays `auto`); `closed` true with `closed_at`, or false (reopen) when the stored thread is closed; `last_activity_at` when later. Nothing else changes; the item's `content_sha256` rule of M8-03 note 5 b still applies. Update 22 7.2 with this sentence: "A posted thread applies the changes of section 12.2 step 3; every other posted item answers with its target and changes nothing."

4. **Message item** (22 6.2, 6.4, 03; Ruling for docs/DECISIONS.md).
    a. Author: the target of the posted user item `author_origin_key`; a `skipped` or missing user item is `409 conflict` with `reason` `author_not_imported` (the tool skips messages of skipped authors, 22 5.3).
    b. Insert through `messaging.ImportMessages` in the batch transaction: the fields of requirement 11; `search_language` as M3-13 defines it for a message written by that author in that channel (the static insert test of M3-13 must pass); no `MessagePosted` in-tx event, so neither `runs` nor `notify` sees it (requirement 12, 13); `edited_at` without a revision row.
    c. `reply_to_message_id` = target of `reply_to_origin_key` when that item is posted in the same thread; else `400 validation_failed` at the pointer (the tool writes a quote line instead, 22 6.2).
    d. Attachments: every ID in `attachment_file_ids` must be the target of a `posted` file item of this source whose file was uploaded for this channel; they attach through the attach path of M3-07 (`message_attachments` with the copies `thread_id` and `channel_id`, `has_attachments`), without `+upload` checks of the author (the import is authorized by `import.apply`); at most `message.max_attachments`.
    e. The new message's ID is the item's target (`target_kind = message`).

5. **End of the batch** (04 15.8, 22 6.4). When at least one message, pin or thread change was written: set `threads.unread_floor_seq` = `last_message_seq`, raise the thread version, append exactly one `thread.updated` with the thread scope; then `import.progress` and the stats counters (`server.threads`, `server.messages`, `server.pins`). A batch that only returned posted items writes no event at all (22 7.2 item 1).

6. **`createImportFile`** (22 6.5, 7.2 item 3, 12 11; Ruling for docs/DECISIONS.md).
    a. Authorize `import.apply` on the source; `channel_id` must be a channel of this source; `author_id` must be the target of a posted user item of this source; `size_bytes` at most `file.max_upload_bytes`; origin key `file:<digits>`.
    b. New key: write the item `pending` with `target_kind = file` and the reserved file ID, create the `files` row in its pending state and an upload session in import mode in the same transaction: `created_by` = the importing owner (so the tool, logged in as that owner, may upload parts and complete with the normal `uploadPart` and `completeUpload` of M3-03), `uploaded_by` = the author, no quota check, `scope_channel_id` = the channel. Response `ImportFileSlot{item, upload}`.
    c. Processing after completion is the normal path of 12 3.4 (checksum, type, scan, metadata, thumbnails). 12 11 asks for a separate queue `files_bulk`; this ticket instead enqueues the scan and processing jobs of import files on the queue `files` with the lowest River job priority, so live uploads (default priority) are always fetched first, and needs no new queue and no new configuration key. Update 12 11 and 02 12.2 with this rule.
    d. Repeat with a `pending` key: return the same file ID; when its upload session is still open, return it; when it expired or was aborted, open a new session for the same file ID. Repeat with a `posted` key: return the item without `upload`. When the upload completes (the in-tx event of `files` for a completed upload, or the port call the merged code offers), the importer marks the item `posted` in the same transaction. A file whose scan ends `infected` stays `posted` as an item but cannot be attached (M3-07 rules); the tool reports it as failed with the jump link.

7. **Ports** (03 principle 12; Ruling for docs/DECISIONS.md). `chat.ImportThreads.Create(ctx, ImportedThread) (ids.ID, error)`, `ApplyMirror(ctx, threadID, MirrorChange) (changed bool, err error)`, `FinishBatch(ctx, threadID) error` (floor, version, `thread.updated`); `messaging.ImportMessages.Append(ctx, threadID, []ImportedMessage) ([]ids.ID, error)`, `Pin(ctx, messageID, pinnedBy ids.ID) error` (no per-message event); `files.ImportFiles.Reserve(ctx, ReserveSpec) (FileSlot, error)`, `SlotFor(ctx, fileID)`. Names may differ in merged code (D-W-100); the behaviour may not.

## Tests (write them first)

- `internal/importer/domain`: `TestThreadBatchShape` (note 1 rules); `TestMirrorChange` (table: auto title renamed, manual title untouched, archive and reopen, older `last_activity_at` ignored).
- Integration tests with `pgtest` and the HTTP chain (seed of M8-03 plus a posted text channel with `lead = bothy`, not read-only, operator Lotse; a read-only forum channel with `lead = discord`; users Alex, Sam and a placeholder):
    - `TestImportNeverTriggers` (plan acceptance, 22 16): a message batch in the channel with operator Lotse creates no run, no `agent.invocation`, no follow-up, no `trigger_state`.
    - `TestImportNoNotification` (22 16): no notification row, no push job, no `mentions` row, no `message.created` in the event log; exactly one `thread.updated` per batch; `unread_floor_seq` equals `last_message_seq`; Alex's unread count for the thread is 0.
    - `TestThreadItemFields`: placeholder name gives `auto`, a real name `manual`; operator from `planned_operator` while the channel has no operator; times and `closed_at` from the data; starter message link only when posted.
    - `TestMirrorChangesOfPostedThread` (plan acceptance): rename with `auto` changes the title, with `manual` not; archive and reopen; `thread.updated` only when changed.
    - `TestRepeatBatchWritesNothing` (plan acceptance): the same thread batch twice gives the same targets and no event the second time.
    - `TestStoredThen502ThreadBatch`: the response is dropped after commit; the retry gives the same message IDs and the same `thread_seq` values.
    - `TestMessageOrderAndSeq`: `thread_seq` follows request order, not `created_at`.
    - `TestAuthorNotImported`, `TestReplyTargetRules`, `TestAttachmentRules` (a file of another channel, a `pending` file, more than `message.max_attachments`).
    - `TestPinsImported`: `pinned_by` is the importing owner; no per-message event.
    - `TestReadOnlyChannelAcceptsImport`: the read-only forum accepts the batch.
    - `TestCreateImportFileSameID` (plan acceptance): three calls with one key return one file ID; an aborted session gets a new session for the same ID; after completion the item is `posted` and the call returns no `upload`; no quota is charged.
    - `TestImportFileScanPriority`: the scan job of an import file has the lowest priority; a live upload enqueued later is fetched first by the worker.
    - `TestSearchLanguageOnImportedMessage` (M3-13 static and behaviour test pass).

Commands that must pass, from the repository root:

- `cd services/server && go test ./internal/importer/... ./internal/messaging/... ./internal/chat/... ./internal/files/... -race -count=1`
- `make arch`, `make api-lint`, `make api-checks`, `make codegen-check`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] `origin = import` never creates a run or an invocation, also in a channel with an operator.
- [ ] No push, no `mentions`, no `message.created`; one `thread.updated` per batch; imported content is not unread.
- [ ] Repeating a batch writes nothing; a dropped response after commit gives no duplicate.
- [ ] A file item reserves one ID across repeats and becomes `posted` when complete.
- [ ] Mirrored title and done state follow the source by the rules of 22 12.2.
- [ ] OpenAPI, 12, 02, 22, READMEs, CHANGELOG and rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every exported Go symbol; comments explain why.
- READMEs of the touched modules updated in English.
- Rulings of Design notes 1, 3, 4, 6 and 7 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` entry under `[Unreleased]`.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
