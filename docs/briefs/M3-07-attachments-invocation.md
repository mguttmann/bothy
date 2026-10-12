# M3-07: Attachments and release of the agent invocation

Status: ready | Depends on: M1-58, M2-11, M3-04 | Milestone: M3
## Goal

After this ticket a person, an agent or an app sends a message with up to `message.max_attachments` of its own files that were uploaded for the channel of that message and whose upload session is completed: `createMessage` and `createThread` (`first_message`) accept `MessageCreate.attachment_file_ids` (until now `400 validation_failed` with `errors[].code` `not_enabled`, M1-56), require `+upload`, bind every file to exactly one message in `message_attachments` with the copies `thread_id` and `channel_id` of that message, set `messages.has_attachments`, and refuse every other file with `409 conflict` and `reason` `file_not_attachable` (12 10, SEC-UP-9, R-DM-22, R-DM-43). An attached file is read through the channel of its message, appears in every representation of the message, follows its thread when the thread moves (same transaction) and goes with its message when the message is deleted (same transaction, `has_attachments` back to `false`). A message that triggers an agent while one of its attachments is not final keeps `trigger_state = received` and writes no `agent.invocation`: the transaction that makes the last attachment final writes it (`ReleaseInvocation`, subscriber of `files.FileFinalized`), and the job `runs.invocation_hold_timeout` writes it after `file.invocation_hold_max` with the attachments in their current state, exactly once either way (04 15.3 point 1, 12 10, R-FI-09, RK-43). No agent ever receives an unchecked byte; after a timeout it receives an invocation whose attachments it can see as `pending`, never their content. The attachment facts this ticket writes are what the search coordinator (M3-14) filters on.

## Read first

- `docs/PLAN.md`: 3 "Way of working per ticket"; 5 "Test strategy" (row "Test doubles": clamd container); 9 "M3 Platform services" (Goal, Scope, Out of scope, Exit criteria 1 and 2, Test strategy, Red team "a file across space boundaries in the context of another agent (SEC-UP-9)", Dependencies, Risks "a long outage of clamd holds back invocations with attachments up to `file.invocation_hold_max`"); the rows M3-03 and M3-04 (what exists), M3-07 (this ticket, the contract), M3-05 and M3-06 (beside it, not in its closure), M3-08, M3-09, M3-12, M3-14, M3-15 (who builds on it); 8 rows M2-05, M2-07, M2-08, M2-11, M2-13, M2-19, M2-31; 7 rows M1-56, M1-57, M1-58, M1-62, M1-70; 14 row M8-03.
- Dependencies: the header names M1-58, M2-11 (follow-ups and the follow-on run) and M3-04. Implied through them and therefore not in the header: M2-08, M1-62, M3-03 (tables `files`, `file_upload_sessions`, `file_upload_parts`, `getFile`, the `file.updated` builder, `authorize.FileFacts`, `file.read` no longer deferred), M3-01 (object store), M2-07 (`pollAgentEvents`), M2-06, M2-05 (`Open`, acceptance), M2-04, M2-02, M1-71 (shared test harness), M1-70 (`test/leak` and the SQL visibility lint `tools/lint/sql_visibility.py`), M1-69 (`test/canary`), M1-68 (instrument registry), M1-65, M1-64, M1-63, M1-61 (drafts and `clear_draft`), M1-60, M1-56, M1-55, M1-54, M1-51, M1-28, M1-24, M1-16, M1-14, M1-13, M1-12 and the rest of their closure. Not in the closure but used if merged (each place says how): M1-57 (history), M1-59 (reactions and pins in the delete path), M2-13 (run context), M2-15 (the owner's switch pattern), M2-19 (agent answer branch), M3-05 (download), M3-06 (images).
- `docs/spec/12-files.md`: 1 "Goals and limits" point 1; 2 "States of a file" (definitions "servable" and "final", the rule that every state change writes `file.updated`); 3.4 "Completion and verification pass" step 3 (a completed upload may be attached at once); 3.7 "Purpose of a file"; 10 "Attaching and agent invocation" completely; 12 "Retention, deletion, deduplication" row "Message deleted"; 15 "Tests" rows "Attaching the same file a second time" and "Agent invocation with an attachment while clamd is down"; 16 R-FI-08, R-FI-09; 18 `file.invocation_hold_max`.
- `docs/spec/03-data-model.md` (core contract, read only): 1 "Principles" item 12; 4.5 `threads` (paragraph "A move changes `channel_id`"); 4.6 `messages` (`has_attachments`, its CHECK, the paragraph "Deletion empties `content_md`, sets `has_attachments` to `false`"); 4.7 `files`, `file_upload_sessions`, `message_attachments` (all columns and the paragraphs after the table); 4.8 `runs` (including `invocation_held_until`, R-DM-44, Design note 2, and the partial index of active runs `(state, agent_id)`); 6.3 "Lock order"; 6.5 "Redaction on deletion"; 8 R-DM-22, R-DM-43, R-DM-44; 9 `message.max_attachments`.
- `docs/spec/04-sync-protocol.md` (core contract, read only): 2.2 "Envelope" (`actor` absent for jobs without principal, payload rules 4 and 5); 9 "Delivery guarantee" point 3; 15.3 "Invocation and run start" points 1 and 2; 15.4 "Follow-ups" points 1, 2 and 4 (Design note 11 e); 17 R-SY-20.
- `docs/spec/05-actions.md` (core contract, read only): 2.2 "Target of a resource" (rows `file`, the paragraph "An attached file always follows its message"); 4.7 row `message.create` ("with attachments additionally `+upload`"); 4.8 rows `file.read`, `file.delete`; 5 "Agent invocation `agent.invoke`" (only to see that the release writes no new check); 6 rows `agent.invocation`, `file.updated`, `message.created`.
- `docs/spec/02-architecture.md`: 4.1 "Rules" point 4; 4.3 "Ports, events and sources per module" (rows `files`, `messaging`, `runs`); 4.4 "Allowed dependencies" (rows `files`, `messaging`, `runs`, "Tiers"); 8.1 to 8.3 (subscribers, veto); 8.5 "What never happens in a transaction"; 12.3 "Job catalogue" (rows `runs.invocation_hold_timeout`, `files.purge_objects`); 12.4 "Rules for jobs"; 13 "Configuration loading" item 5.
- `docs/spec/09-agents.md`: 2.2 "Features" (row "Sending files", `supports_files_out`); 2.3 "Grants of an agent"; 3.4 "Decision per message" point 7; 4.6 "Run context" (`attachments` with `scan_status`, the paragraph "The adapter loads attachments with `file.read`"); 4.7 row "Send file"; 5.2 `agent_unavailable`; 6 "Trigger state (`trigger_state`)" (`received` until the first delivery).
- `docs/spec/07-permissions.md`: 4.1 (`capability_missing`), 4.3 "Visibility", 7.4 "Query pattern" rules 1 and 3; 11.2 AZ-P4, AZ-P12. `docs/spec/19-security.md` SEC-UP-9. `docs/spec/20-privacy.md` 4.2 "Deleting a message", 4.3 "Files". `docs/spec/14-search.md` 4.3 to 4.5 (the consumer of `message_attachments` and `has_file`). `docs/spec/06-api.md` 5 "Errors" points 3, 5 and 7.
- `docs/api/openapi.yaml`: operations `createMessage`, `createThread`, `getFile`, `moveThread`, `deleteMessage`, `pollAgentEvents`, `listEvents`, `updateAgent` (and `getMessage`, `listThreadMessages`, `downloadFile`, `createFileTicket`, `getRunContext` where their tickets are merged); schemas `MessageCreate`, `ThreadCreate`, `Message`, `Attachment`, `File`, `FilePayload`, `FileUpdatedEvent`, `MessageCreatedEvent`, `AgentInvocationPayload`, `AgentInvocationEvent`, `Problem` (`x-problem-codes`: `conflict`, `validation_failed`, `forbidden`, `feature_disabled`), `FieldError`. `docs/api/asyncapi.yaml`: messages `event.file.updated`, `event.agent.invocation`.
- ADRs 0007, 0008, 0017, 0023, 0033, 0050, 0052, 0053, 0054.
- `docs/DECISIONS.md`: D-FI-07, D-FI-08, D-LS-14, D-K-A6, D-K-G6 (the mechanism `files.FileFinalized` plus `runs.invocation_hold_timeout` instead of a port call), D-W-100, D-W-104 (R-DM-43, R-DM-44 and the further amendments), D-W-106, section 10 (format of the rows `D-M3-07-<n>`).
- `docs/security/threat-model.md` T-AI-01 ("files only in their own channel (SEC-UP-9)"), T-AI-03, T-UP-01; `docs/RISKS.md` RK-43.
- `docs/operations/configuration.md`: `file.invocation_hold_max`, `message.max_attachments`, `run.queued_max_age`, `database.tx_retry_max`; `docs/operations/upgrade.md` 4 "Rules for migrations (Expand/Contract)" (M3, M4, M5, M6).
- Briefs (merged names win, D-W-100): `M3-04-clamd-scan.md` (`files.FileFinalized`, `domain.Final`, `domain.Deliverable`, the outcome write of its Design note 6, `clamdtest` with the fake clamd server), `M3-03-upload-sessions.md` (Design notes 15 and 16: visibility of kind `file`, `FileFacts`, the `file.updated` builder in `internal/files/app/events.go`, audience before attaching), `M2-08-invocation-delivery.md` (acceptance in `OnMessagePosted`, Design notes 10 f and 14: the order inside acceptance and the `agent.invocation` builder; `runs.Delivery`; the job `runs.queued_expiry`), `M2-05-runs-state-machine.md` (`Open`, `OpenCmd`, `Transition`, `SetTriggerState`), `M2-07-agent-consumers.md` (`pollAgentEvents`), `M1-58-messaging-edit-delete.md` (the delete use case, its steps 3 to 9 and the lock order of its note 4), `M1-62-chat-thread-move.md` (`chat.ThreadMoved`, the subscribers of `messaging` and `sync`), `M1-56-messaging-send.md` (D1, D2, D3, D7, D8, D9, `MessagePosted`), `M1-55-chat-threads.md` (`AppendToThread` returns `Appended.Thread`), `M1-28-authz-core.md` (`Resource.HasAttachments`, `CapIf(upload, HasAttachments)`), `M1-13-unit-of-work.md` (`txn.Subscribe`, `txn.Publish`, `testdata/txn_registrations.golden`), `M1-14-background-jobs.md` (`jobs.Args`, `jobs.Register`, `Inserter.InsertTx`, `InsertOptions.ScheduledAt`), `M1-16-event-log.md` (`eventlog.Appender`, `AudiencePrincipalID`), `M1-51-agents-records.md` (`operator_config`, `OperatorConfigV1`), `M1-12-postgres-migrations.md` (`pgtest`, migration numbering), `M1-61-sync-personal-data.md` (note 12: `clear_draft` deletes the draft in the message transaction), `M1-70-leak-tests.md` (SQL lint rules 1 to 7 and the exception categories), `M1-01-server-gates.md` (migration rules of `make migrations-lint`), `M2-11-run-followups.md` (note 8 c: the follow-on run uses the invoke step of the acceptance). If merged: `M2-15-components-interactions.md` (note 6 b: the owner's switch `supports_components`). Consumers: `M3-14-search-coordinator.md` Design note 15, `M3-15-file-text-extraction.md` Design note 10, `M3-06-image-metadata-thumbnails.md` Design note 16, `M2-13-run-context.md` requirement 18.
- Code (all under `services/server/`): `internal/files` (port, `domain`, `app/events.go`, `app/get.go`, the outcome write of M3-04, `adapters/postgres`, `adapters/jobs`, `adapters/clamd/clamdtest`), `internal/messaging` (create, delete, representation, `MessagePosted`), `internal/runs` (acceptance, `Open`, the `agent.invocation` builder, `adapters/jobs`), `internal/chat` (`ThreadMoved`, `Threads.Info`), `internal/agents` (port), `internal/kernel/authorize`, `internal/authz` (visibility of kind `file`), `internal/wire`, `migrations/`, `architecture/modules.yaml`, `tools/archgen`.

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
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. Attaching (12 10 points 1 to 3, SEC-UP-9, R-DM-22): every file was uploaded by the author (`uploaded_by`), has `purpose = 'attachment'`, `scope_channel_id` equal to the channel of the message, an upload session in state `completed`, `upload_state` other than `failed`, no `deleted_at` and no attachment yet; otherwise `409 conflict` with `reason` `file_not_attachable`. At most `message.max_attachments` files per message; a file hangs on at most one message (unique index on `message_attachments(file_id)`).
12. `message.create` with attachments needs `write` and `+upload` on the thread's channel (05 4.7 row `message.create`); `attachment_file_ids` is accepted by `createMessage` and by `createThread` in `first_message` and no longer answers `400 validation_failed` with `errors[].code` `not_enabled` (06 5 point 3, M1-56 D2).
13. `message_attachments` exactly as 03 4.7 with the copies `thread_id` and `channel_id` (R-DM-43): `files` writes them at attach from the message that `messaging` inserted in the same transaction and passes through the `files` port; `files` changes `channel_id` in the transaction of `moveThread` (subscriber of `chat.ThreadMoved`); no foreign key on the copies; `files` never reads `messages` and never uses `files.scope_channel_id` for the copy (03 1 item 12).
14. `messages.has_attachments` with the CHECK `deleted_at IS NULL OR NOT has_attachments`: `true` exactly when the message was created with at least one attachment and is not deleted; `messaging` sets it in the INSERT and resets it in the UPDATE that deletes the message; in that same transaction the attachment rows go and the files get `deleted_at` (03 `messages`, 12 12, 20 4.2 and 4.3).
15. Hold (04 15.3 point 1; 12 10 "Invocation only after the check"; R-FI-09): a triggering message with an attachment that is not final (12 2 "final") writes no `agent.invocation` at acceptance and keeps `trigger_state = received`; the transaction that makes the last attachment final writes it (`files.FileFinalized`, subscriber in `runs`, `ReleaseInvocation`); the job `runs.invocation_hold_timeout` writes it after `file.invocation_hold_max` with the attachments in their current state; every run gets exactly one `agent.invocation`.
16. A released invocation carries no file content; the agent learns attachment states only through reads that `authorize` checks (`getFile`, the message representation, the run context) and gets bytes only when `scan_status IN ('clean','skipped')` (12 1 point 1, 09 4.6, 04 2.2 rule 4).
17. An attached file is read through the channel and thread of its message (05 2.2 rows `file`); `files.scope_channel_id` counts only before attaching and never changes (03 `threads`, `files`).
18. `file.updated` of an attached file carries the scope of its message and no audience; before attaching it goes to the uploader only (05 6 row `file.updated`; M3-03 Design note 16).
19. Every representation of a message (`Message.attachments` in REST answers, `message.created`, `message.updated`) lists its attachments in position order with the `File` object; no JSON `null` (ADR 0053); a deleted message lists none (04 2.2 rule 5).
20. An agent sends files only with `+upload` (09 2.3) and only while the owner's switch `supports_files_out` in `agents.operator_config` is not `false` (09 2.2); otherwise `403 feature_disabled` (06 5 point 5).
21. Lock order (03 6.3, 02 8.2): thread row, then message rows, then `files` rows in ascending `id`, then `message_attachments`, then `runs`; no path takes them in another order.

## Scope

All paths under `services/server/` unless they start with `docs/` or `infra/`. Where M3-03, M3-04, M2-05, M2-08, M1-56 or M1-58 already created a file named below, extend it.

Create:

- `migrations/<next>_files_message_attachments.sql`, `migrations/<next + 1>_messaging_has_attachments.sql`, `migrations/<next + 2>_messaging_has_attachments_validate.sql`, `migrations/<next + 3>_runs_invocation_held_until.sql` (Design note 2), their lines in `migrations.sha256` through `make migrations-lock`.
- `internal/files/attachments.go` (port): `Attachments`, `AttachCmd`, `Attachment`, `AttachmentRef` (Design note 3).
- `internal/files/domain/attach.go`: `Attachable(AttachFacts) bool`, `EventTarget(attached bool) Target` and `BecameFinal(before, after StateFacts) bool` (Design notes 4 and 10), standard library and `kernel-core` only.
- `internal/files/app/attach.go` (the five port methods), `internal/files/app/resource.go` (`fileResource`, Design note 9), `internal/files/app/filechange.go` (`recordFileChange`, Design note 10, unless the merged code already has one function that appends `file.updated` for every state change; then extend that one), `internal/files/app/subscribers.go` (`OnThreadMoved`).
- `internal/files/adapters/postgres/queries/attachments.sql` and the regenerated sqlc code.
- `internal/runs/domain/hold.go`: `HoldUntil`, `ReleaseCause` (`ReleaseFinal`, `ReleaseTimeout`), `ReleaseVerdict` (Design notes 11 and 12).
- `internal/runs/app/hold.go`: `invokeOrHold`, `ReleaseInvocation`, `OnFileFinalized`, the hold timeout use case; `internal/runs/adapters/jobs/hold.go` (args `InvocationHoldTimeout{RunID ids.ID}`, kind `runs.invocation_hold_timeout`, handler, scheduler over `jobs.Inserter`); `internal/runs/adapters/postgres/queries/hold.sql`.

Change:

- `internal/messaging`: port (`MessagePosted.HasAttachments`, the create command with `AttachmentFileIDs`), `domain` (body rules), `app` (create, delete, representation), postgres queries (`has_attachments` in the INSERT, in the delete UPDATE and in the reads) (Design notes 5 to 7).
- `internal/runs`: `OpenCmd.InvocationHeldUntil` of M2-05's `Open`; the acceptance subscriber `OnMessagePosted` (M2-05 `accept.go`, extended by M2-08; follow the merged file) for the hold (Design note 11) and the agent branch of Design note 15.
- `internal/files`: `app/events.go` (scope and audience, Design note 10), `app/get.go` and every other caller of `file.read` in `files` (Design note 9), the outcome write of M3-04 (publish moves into `recordFileChange`).
- `internal/authz`: only if the merged visibility of kind `file` lacks the attached row of M3-03 Design note 15 (read on the path of the hints `ChannelID` and `ThreadID`): add it with a test.
- `internal/agents` port: only if no read returns `operator_config` (M2-15 adds one where it is merged): `Agents.OperatorConfig(ctx context.Context, agentID ids.ID) (OperatorConfigV1, error)` with the type of M1-51, its query and a test.
- `internal/wire`: subscriptions `files.FileFinalized -> runs.OnFileFinalized` and `chat.ThreadMoved -> files.OnThreadMoved` (after `messaging.OnThreadMoved` and `sync.OnThreadMoved`); job `runs.invocation_hold_timeout` (queue `default`, roles `worker` and `all`); the `files.Attachments` port for `messaging` and `runs`; config keys of Design note 16 in `configkeys_files.go` and the messaging key file; regenerate `testdata/txn_registrations.golden` and the jobs golden file if one exists.
- `architecture/modules.yaml`: `files` in `runs.depends_on`; regenerate `.go-arch-lint.yml` (`cd services/server && go generate ./tools/archgen`).
- `tools/lint/sql_visibility.yaml` at the repository root, where M1-70 created it: the exceptions of Design note 16.
- The M1-56 test that asserts `not_enabled` for `attachment_file_ids` (`TestCreateMessage_NotEnabledMembers400` or its merged name): change only the attachments row to the new rule; keep the other rows.
- `docs/spec/02-architecture.md` 4.3 and 4.4, `docs/architecture/c4-3-server-components.mmd` and its `.svg` (Design note 1); `docs/spec/12-files.md` 2 and 10, `docs/spec/09-agents.md` 2.2, `docs/api/openapi.yaml` (descriptions only) (Design note 17).
- `docs/operations/configuration.md` regenerated (`make config-reference`).
- READMEs of `files`, `messaging`, `runs` (and `agents` if its port changed); `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` section 10 rows `D-M3-07-<n>` (Design note 18).

This ticket enables no operation in `api/operations.yaml` (`createMessage`, `createThread`, `getFile`, `moveThread` and `deleteMessage` are enabled), adds no action, no event type, no metric and no top-level Makefile target.

## Out of scope

- Upload sessions and `getFile` itself (M3-03); the scan pass, `files.scan`, `domain.Final` and the in-transaction event `files.FileFinalized` (M3-04); download, fetch tickets, `409 file_not_ready` and `409 file_blocked` (M3-05); metadata removal and thumbnails (M3-06). This ticket only changes their call sites where Design notes 9 and 10 say so.
- Deleting objects of deleted files, the jobs `files.purge_objects` and `files.cleanup_pending`, never attached files, `deleteFile`, the redaction of `file.updated` payloads of deleted files and DP-T01 for files (M3-08); this ticket sets `files.deleted_at` and leaves the objects to the purge job.
- Voice messages, `kind = voice` (stays `400 validation_failed` with `errors[].code` `not_enabled`), transcripts and `voice.transcript_hold_max` (M3-09); it extends the hold condition of Design note 11.
- Reply attachments of the Claude Code runner (M3-12); files of the Hermes adapter (plan gap of M2-33); import files and `IngestFile` (M8-03).
- Search over attachments and `has_file` (M3-14); text extraction and its deletion with the file (M3-15).
- The attachment part of AG-20 in the conformance suite (its line `AG-20 attachment M3-07` in `testdata/deferred.golden` of M2-31): it needs M2-31, M2-13 and M3-05, none of which is in this ticket's closure; M3-09 closes it (D-W-108).
- Holding `agent.followup` of a follow-up whose attachments are not final: not held (Design note 11 e; the core contract defines the hold only for `agent.invocation`, 04 15.3 point 1, 12 10). Bytes stay protected by the download rule of 12 9.3. The follow-on run of M2-11 is held like an acceptance (Design note 11 d).
- The adapter half of the capability `files_out` (heartbeat `capabilities.files_out`, M2-20, which is not in this ticket's closure); this ticket enforces only the owner's switch, as M2-15 does for `supports_components`. Reason: the capability gates what an agent sends, and an adapter that sends a file shows by the send that it can; the owner's switch is the half that can forbid it (09 2.2 "Reason").
- Ending runs whose triggering message was deleted (M2); the deletion journal (M7-07); client rendering of attachments (M4, M5); notifications, which never wait for attachments (12 10 last sentence).
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M3-07-attachments-invocation`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(files): ...`, `feat(runs): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Never log, print or persist an invocation token. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11). The attachment facts are in the core contract since D-W-104 (03 `message_attachments` and `messages`, R-DM-43), and so is the marker of a held invocation (03 `runs.invocation_held_until`, R-DM-44); build them as written.
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-01, M3-03, M3-04 and M2-11 (and M3-05, M3-06, M2-13, M2-19 where they are merged): follow their names (repositories, the `file.updated` builder, the outcome write, `FileFinalized`, `Open`, the acceptance file, the `agent.invocation` builder) and list each difference.
- clamd runs in tests only as the fake clamd server of `clamdtest` on a Unix socket in a `mktemp -d` directory, and in the one test marked "real clamd" as the throwaway container of `clamdtest` (pinned image, no signature download, loopback only); stop only containers you started, by id. No real third-party service, no agent runtime and no model call: the adapter side is `pollAgentEvents` with an agent token.
- Time only through `kernel/clock`; tests move a fake clock and call job handlers directly, they never sleep for a window.
- The SQL visibility lint of M1-70 is on the base branch: every new reading query on a content table carries the visibility predicate or an exception entry with category and reason (Design note 16); never widen an existing exception.

## Design notes

1. **Dependencies and ports** (02 4.1, 4.3, 4.4; ADR 0008, ADR 0050). `runs` subscribes `files.FileFinalized` (02 4.3 row `runs`) and asks `files` whether the attachments of a message are final, so `runs` imports the `files` port; 02 4.4 does not list that edge yet. Ruling for docs/DECISIONS.md: add `files` to "May use" of `runs` in 02 4.4 (reason: "write the held invocation on `files.FileFinalized`; read the attachment state"), to `runs.depends_on` in `architecture/modules.yaml`, the edge `runs --> files` to `docs/architecture/c4-3-server-components.mmd` (render with `make diagrams`) and to the dependency line of the `runs` README; `make register-check` holds the four equal (M1-06 C7, C11, C12). The edge points down (`files` is below `messaging`, `runs` above it), so the graph stays acyclic. Ruling for docs/DECISIONS.md: the source port `AttachmentLocator` of 02 4.3 is not built, because the copies of R-DM-43 give `files` the path of every attached file in its own table; remove it from the rows `files` and `messaging` of 02 4.3.

2. **Migrations** (goose; sequential numbers and slugs of `make migrations-lint`; use the next free numbers when you rebase before the merge and renumber yours if a parallel ticket took them).

    a. `<next>_files_message_attachments.sql`: table `message_attachments` exactly as 03 4.7: `message_id uuid NOT NULL`, `file_id uuid NOT NULL`, `position smallint NOT NULL`, `thread_id uuid NOT NULL`, `channel_id uuid NOT NULL`; PK `(message_id, position)`; FK `message_id -> messages(id) ON DELETE CASCADE`, FK `file_id -> files(id) ON DELETE RESTRICT`; no foreign key on `thread_id` or `channel_id` (R-DM-43); UNIQUE `(file_id)`; index `(thread_id)`. The table is new and empty, so constraints and indexes are created with it. No column, CHECK or index that 03 does not name.

    b. `<next + 1>_messaging_has_attachments.sql`: `ALTER TABLE messages ADD COLUMN has_attachments boolean NOT NULL DEFAULT false` (constant default, no rewrite, upgrade.md M3) and `ADD CONSTRAINT messages_has_attachments_deleted_check CHECK (deleted_at IS NULL OR NOT has_attachments) NOT VALID` (M5). `<next + 2>_messaging_has_attachments_validate.sql`: `VALIDATE CONSTRAINT` of it. Remove the deviation comment of M1-56 in the documentation of `messaging` (its migration stays unchanged).

    c. `<next + 3>_runs_invocation_held_until.sql`: `ALTER TABLE runs ADD COLUMN invocation_held_until timestamptz` (NULL allowed, no default, upgrade.md M3). The column is core contract (03 `runs`, R-DM-44, D-W-104): set by the INSERT of `Open` exactly when acceptance holds the `agent.invocation` of the run, to the latest release time; set to `NULL` by `ReleaseInvocation` in the transaction that writes the event, which raises `runs.version` by one like every stored change of a run (03 `runs.version`; as M2-08 does for `invocation_jti`) but writes no `run.state_changed`; `NULL` for every run that was never held; a run that ends while held keeps the value and is never released. No new index: the held-run lookup of Design note 13 a filters the partial index of active runs `(state, agent_id) WHERE state IN ('queued','running','waiting_input')` (03 `runs`), because a held run is always `queued` and active runs are few. No CHECK beyond 03.

3. **Port `files.Attachments`** (02 4.3 row `files`: `Attachments`). Every method runs inside the caller's unit of work (internal error otherwise) and never calls `authorize`, because the caller authorized the action (doc comments say so):

    ```go
    // Attachments binds uploaded files to messages and answers attachment facts
    // (12 section 10, 03 message_attachments).
    type Attachments interface {
        // Attach checks every file against 12 10 point 1 and inserts one row per file in
        // request order with the thread and channel of the message just inserted.
        Attach(ctx context.Context, cmd AttachCmd) ([]Attachment, error)
        // ForMessages returns the attachments of the given messages in position order.
        ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]Attachment, error)
        // DeleteForMessage removes the attachments of a message deleted in the same unit of
        // work and marks their files deleted (12 12).
        DeleteForMessage(ctx context.Context, messageID ids.ID) error
        // MessageOf reports where a file is attached; ok is false for an unattached file.
        MessageOf(ctx context.Context, fileID ids.ID) (ref AttachmentRef, ok bool, err error)
        // Pending reports whether an attachment of the message is not final (12 2).
        Pending(ctx context.Context, messageID ids.ID) (bool, error)
    }
    type AttachCmd struct {
        MessageID, ThreadID, ChannelID, AuthorID ids.ID
        FileIDs []ids.ID // request order; position = index + 1
        Pointer string   // JSON pointer of the list in the request body
    }
    type Attachment struct { Position int; File File }
    type AttachmentRef struct { MessageID, ThreadID, ChannelID ids.ID; Position int }
    ```

    `messaging.MessagePosted` gets `HasAttachments bool` (doc comment: "the message was created with at least one attachment"); M2-05's `OpenCmd` gets `InvocationHeldUntil time.Time` (zero: not held), written by the INSERT of `Open`.

4. **Attach** (12 10 points 1 to 3; Ruling for docs/DECISIONS.md for c and d).

    a. `SELECT ... FOR UPDATE OF files` on the `files` rows of the distinct IDs in ascending `id`, joined with the state of their upload session (the session row is not locked: `completed` is final for a session, M3-03); then, in a later statement, the `message_attachments` rows of these files. Locking first makes a concurrent attach of the same file and the outcome write of M3-04 wait for this transaction.

    b. `domain.Attachable(AttachFacts{...})` is true exactly when the row exists, `deleted_at` is NULL, `uploaded_by` equals `AuthorID`, `purpose = 'attachment'`, `scope_channel_id` equals `ChannelID`, the session state is `completed`, `upload_state <> 'failed'` and no attachment row exists. Blocked files (`infected`, `error`) and files whose scan still runs are attachable, as 12 10 point 1 and 12 3.4 step 3 say.

    c. Walk the list in request order; the first entry that fails ends the call with `409 conflict` with `reason` `file_not_attachable` and `detail` = `<Pointer>/<index>`; an ID listed a second time fails as already attached (12 10 point 3). Unknown, foreign, deleted, other-channel, open, aborted, expired and failed files give the same status, code, reason and headers, so the answer tells nothing about files of other people.

    d. Insert the rows (`position` = index + 1, `thread_id` and `channel_id` from the command). A unique violation on `(file_id)` is the last line and maps to the same `409` with `detail` = `Pointer`; the unit of work rolls back, so the message is not stored and its thread sequence is given back (03 6.2).

    e. Return the attachments with the attached form of `File` (Design note 6).

5. **Sending** (M1-56 D1, D2, D8; Ruling for docs/DECISIONS.md for the empty content).

    a. Body rules: more than `message.max_attachments` IDs give `400 validation_failed` with `errors[].code` `limit_reached` and pointer `/attachment_file_ids` (`/first_message/attachment_file_ids` in `createThread`). The `not_enabled` rule for `attachment_file_ids` goes. With at least one attachment `content_md` may be empty or white space only (a picture without caption); without attachments the M1-56 rule (`errors[].code` `empty`) stays. `kind = voice` stays `not_enabled`.

    b. `authorize(actor, message.create, Resource{..., HasAttachments: len(ids) > 0})` at the place M1-56 authorizes (for `createThread` the check of `message.create` on the new thread): without `upload` the answer is `403 forbidden` with `reason` `capability_missing`.

    c. In the unit of work, after the replay lookup (a replay attaches nothing and answers the stored message with its attachments): `AppendToThread`; INSERT the message with `has_attachments = (len(ids) > 0)`; mentions; then `files.Attachments.Attach` with the message id, its `thread_id`, the `channel_id` of the row just inserted (the channel `AppendToThread` returned under the thread lock), the author and the list; then `message.created` and `MessagePosted` (with `HasAttachments`) in the merged order. The request hash of M1-56 D3 already holds the attachment IDs in request order; its encoding does not change. A send with `clear_draft: true` removes the files from the author's draft because M1-61 deletes that draft in the same transaction (12 10 point 4; M1-61 note 12); nothing else is needed.

    d. A message without attachments costs no extra statement (M1-56 D9, `perf.message_create_p99`): no `files` call and no flag query. Every caller of the create use case (sends, agent replies, voice, import) inherits the flag and the attach.

6. **Representation** (`Message.attachments`, 04 2.2 rule 5). The single representation function of M1-56 fills `attachments` from `files.Attachments.ForMessages`, called only for messages with `has_attachments = true`, once per request or page (one batched call per page where the merged history of M1-57 builds pages); `message.created` and the `201` use the result of `Attach`. Each `Attachment` has `position` and the `File` of the `files` representation (M3-03) with two rules: Ruling for docs/DECISIONS.md: `scope_channel_id` is absent once the file is attached, because after a move it names a channel the reader may not see (05 2.2: the channel of the message counts); `sha256` only when `upload_state = 'complete'` (M3-03). A deleted message shows `attachments: []` (tombstone of M1-57 and M1-58). Messages carry the file state at the time of the read; later changes arrive as `file.updated` (note 10). Editing (M1-58) never changes attachments (03 `messages`).

7. **Deleting a message** (M1-58 steps 3 to 9; 03 `messages`, 12 12, 20 4.2 and 4.3).

    a. Step 4 becomes `UPDATE messages SET content_md = '', has_attachments = false, deleted_at = now(), deleted_by = caller, version = version + 1 ...` and remembers whether the flag was `true`.

    b. After revisions and mentions (and reactions and pins of M1-59 where merged) and only when the flag was `true`: `files.Attachments.DeleteForMessage`: lock the files of the message's attachments `FOR UPDATE` in ascending `id`, delete the `message_attachments` rows of the message, set `files.deleted_at` to the clock time. Lock order thread, message, files, `message_attachments` (Design note 14).

    c. Ruling for docs/DECISIONS.md: no `file.updated` for these files (12 2 names state changes, and `message.deleted` already removes the message with its attachments from every client); objects stay until `files.purge_objects` of M3-08 removes them within `privacy.object_delete_delay`; `getFile` already answers `404` for a file with `deleted_at` (M3-03). Keep the `deleted_at` write in one function of `files/app` and name it in the README, because M3-08 and M3-15 add their deletions to it.

8. **Moving a thread** (03 `threads`, 4.7; R-DM-43). `files.OnThreadMoved(ctx, chat.ThreadMoved)`: `UPDATE message_attachments SET channel_id = <ToChannelID> WHERE thread_id = <ThreadID>` (index `(thread_id)`); no version change, no event; `files.scope_channel_id` stays. Registered in `internal/wire` after `messaging.OnThreadMoved` and `sync.OnThreadMoved`, so the move transaction locks thread, messages, the rows of `sync`, then `message_attachments`. A send into the thread holds the thread row (M1-55 `AppendToThread`) before it inserts attachment rows, so a send and a move of the same thread run one after the other and end with `message_attachments.channel_id` = `messages.channel_id` = `threads.channel_id`. A write is not linted by the SQL visibility lint (07 7.4 rule 1).

9. **Reading an attached file** (05 2.2; M3-03 Design note 15). One function `fileResource(f File, ref *AttachmentRef) authorize.Resource` builds every `file.read` descriptor in `files`: unattached `{Kind: file, ID, ChannelID: scope_channel_id, OwnerID: uploaded_by, File: &FileFacts{Purpose, Attached: false}}`; attached `{Kind: file, ID, ChannelID: ref.ChannelID, ThreadID: ref.ThreadID, OwnerID: uploaded_by, File: &FileFacts{Purpose: "attachment", Attached: true}}`. `getFile` loads the attachment row by `file_id` next to the file row and uses it; where M3-05 (`downloadFile`, `createFileTicket`, `downloadThumbnail`) or M3-06 are merged, switch their call sites to `fileResource` too. Foreign and unknown IDs keep answering the same `404` (AZ-P4).

10. **`file.updated` and `FileFinalized`** (12 2, 12 10; 05 6; M3-03 Design note 16; M3-04 Design note 6).

    a. `recordFileChange(ctx, before, after File)` is the one place that appends `file.updated` for a change of `upload_state`, `scan_status`, thumbnails or transcript. It reads the attachment row of the file (the caller holds the file row lock): unattached gives `AudiencePrincipalID` = uploader and scope `{space_id, channel_id}` of `scope_channel_id`; attached gives no audience and scope `{space_id, channel_id, thread_id}` of the message, with `channel_id` and `thread_id` from the copies and `space_id` from `chat.Threads.Info(thread_id)` (`domain.EventTarget`). Receive action `file.read`, resource `{kind: file, id}` without `version`, payload `FilePayload` with the form of note 6; the actor stays as each caller sets it today (the request actor for the abort of M3-03, none for the jobs of M3-04 and M3-06).

    b. In the same function: `txn.Publish(ctx, files.FileFinalized{FileID})` when `domain.BecameFinal(before, after)` (not final before, final after, 12 2). Ruling for docs/DECISIONS.md: the publish moves out of M3-04's outcome write into this function, so every finalizing write publishes exactly once: M3-04's outcome, M3-03's abort, and M3-06's thumbnail job and its failure transaction where M3-06 is merged (an image is final only after them, M3-06 Design note 16). Route every merged writer of these columns through `recordFileChange`.

11. **Hold at acceptance** (04 15.3 point 1; 09 3.4 point 7; M2-08 Design note 10 f). `invokeOrHold` replaces the last step of the acceptance, which wrote `agent.invocation`:

    a. Before `Open`: `pending := ev.HasAttachments && files.Attachments.Pending(ctx, ev.MessageID)`; the attachments were bound earlier in the same unit of work and their file rows are locked by it, so the answer is current.

    b. `domain.HoldUntil(now, pending, file.invocation_hold_max)`: zero when not pending, else `now + file.invocation_hold_max`, with one `kernel/clock` reading taken by `invokeOrHold`; pass it as `OpenCmd.InvocationHeldUntil` and use the same value as `ScheduledAt` of the hold job. Ruling for docs/DECISIONS.md: the release time is fixed at acceptance, so a later change of `file.invocation_hold_max` does not move held runs.

    c. Not held: append `agent.invocation` as M2-08 does. Held: no `agent.invocation`; insert `InvocationHoldTimeout{RunID}` with `jobs.Inserter.InsertTx` and `ScheduledAt` = the release time. Everything else of the acceptance (run `queued`, anchor, `SetTriggerState(received)`, `runs.queued_expiry`) stays as M2-08 built it.

    d. Every path that writes the first `agent.invocation` of a run with a triggering message goes through `invokeOrHold`: the acceptance of `createMessage` and of `createThread` with `first_message`, and the follow-on run of M2-11 (M2-11 note 8 c builds it with the invoke step of the acceptance; that path has no `MessagePosted`, so it asks `files.Attachments.Pending` for its triggering message unconditionally, one query per follow-on run).

    e. Follow-ups are not held (Ruling for docs/DECISIONS.md): a follow-up of 04 15.4 whose attachments are not final gets its `agent.followup` as M2-11 writes it. Reason: the core contract holds only `agent.invocation` (04 15.3 point 1, R-SY-20); the adapter reads attachments only through `file.read`, which never serves bytes before `scan_status IN ('clean','skipped')` (12 9.3, 09 4.6), so no unchecked byte reaches the agent; holding a follow-up would need a marker on `run_followups` that 03 does not have and would reorder the follow-ups of a run. Add the sentence to 12 10 (Design note 17).

12. **`ReleaseInvocation(ctx, runID, cause)`** in `internal/runs/app` (04 15.3 point 1 names it; D-K-G6), always inside the ambient unit of work:

    a. Lock the run `FOR UPDATE`. For cause `ReleaseFinal` read `files.Attachments.Pending(triggering_message_id)` after the lock (a new statement, so it sees every finalization committed before the lock was granted); for `ReleaseTimeout` the state of the files does not matter.

    b. `domain.ReleaseVerdict(state, heldUntil, cause, pending)`: release only when `state = 'queued'`, `invocation_held_until` is set and, for `ReleaseFinal`, nothing is pending. A held run that ended meanwhile (`runs.queued_expiry`, a cancel) keeps its column and is never released (Ruling for docs/DECISIONS.md).

    c. Release: `UPDATE runs SET invocation_held_until = NULL, version = version + 1 WHERE id = $1 AND state = 'queued' AND invocation_held_until IS NOT NULL`; then append `agent.invocation` with the builder of M2-08 Design note 14 and the facts of now: scope of the run's thread now (`chat.Threads.Info`; the thread may have moved while held), current target `billing_mode`, current `settings_version`, `triggered_by` from the `messaging` port; audience the agent, receive action `agent.events.consume`, resource `{kind: run, id}` without `version`. Ruling for docs/DECISIONS.md: the envelope has no `actor` and no `correlation_id`, because in every path of this ticket the writer is a job without principal (the scan job of M3-04, the thumbnail job of M3-06, the hold job; 04 2.2 row `actor`, 03 `event_log.actor_kind`); the trigger stays in `triggered_by`. A later finalizing write inside a request (the transcript of M3-09) sets the actor of its unit as usual.

    d. Never `authorize` here: `runs.Delivery.Prepare` of M2-08 checks `agent.invoke` with the rights of the moment immediately before the hand-over (04 15.3 point 2; the human without request as principal, R-AZ-29). The release never touches `trigger_state`; the first successful delivery moves it to `queued` (M2-08).

    e. Exactly once: both causes serialize on the run row and the column is cleared by the first release; a second attempt finds it `NULL`. A cause `ReleaseTimeout` logs one Info line with `bothy.run.id` only.

13. **Subscriber and job.**

    a. `runs.OnFileFinalized(ctx, files.FileFinalized)`: `files.Attachments.MessageOf(e.FileID)`; not attached: return. Otherwise `SELECT id FROM runs WHERE state = 'queued' AND triggering_message_id = $1 AND invocation_held_until IS NOT NULL ORDER BY id FOR UPDATE` (served by the partial index of active runs, note 2 c; the run may live in another thread than the message, 09 3.7) and `ReleaseInvocation(ctx, id, ReleaseFinal)` for each. It runs in the finalizing transaction (02 8.3), so the last attachment's final state, its `file.updated` and the `agent.invocation` commit together.

    b. Job `runs.invocation_hold_timeout` (02 12.3: module `runs`, scheduled, queue `default`), args only the run id (02 12.1). Handler in one unit of work: run not `queued` or column `NULL`: return `nil`; `now` before the stored release time (clock skew between nodes): insert itself again at that time and return `nil`; else `ReleaseInvocation(ctx, id, ReleaseTimeout)`. Repeated execution has no second effect (02 12.4 rule 1); database errors go to River's retry (02 12.4 rule 5).

14. **Concurrency and lock order** (03 6.3; 02 8.2; deadlocks are retried by the unit of work, `database.tx_retry_max`).

    - Send: thread, message (insert), files ascending, `message_attachments` (insert), `runs` (insert), the message again (`SetTriggerState`, already locked).
    - Delete: thread, message, files ascending, `message_attachments` (delete).
    - Move: thread, target channel and space (share, M1-62), messages, `read_states`, `drafts`, `message_attachments`.
    - Finalizing write (M3-04, M3-06): file, session, then the run (subscriber of note 13 a).
    - Hold job: the run only; it reads files without a lock.
    - A file attached and finalized at the same time: the attach locks the file first, so the finalizing write waits and then sees the attachment and the held run; or the finalizing write commits first, so the attach reads a final file and acceptance does not hold. Two attachments finalized in parallel: both subscribers lock the same run; the second reads `Pending` after the first committed and releases, without waiting for the timeout.

15. **Agents and the switch `supports_files_out`** (09 2.2, 2.3; Ruling for docs/DECISIONS.md). In `runs.OnMessagePosted`, before every other branch, for `Origin = agent` with `HasAttachments`: read the author's `operator_config` (`OperatorConfigV1` of M1-51) through the `agents` port; `supports_files_out = false` returns a `kernel/errs` error with code `feature_disabled`, which vetoes the unit of work (02 8.3) and which the `createMessage` and `createThread` handlers render as `403 feature_disabled` like any other error of the use case, so neither message nor attachment rows stay; a missing key allows (09 2.2). The check lives in `runs` because `messaging` and `files` may not use `agents` (02 4.4), the same place M2-15 note 6 b chose for `supports_components`. Then the merged agent branch (M2-19 `checkAnswer` where merged) runs as before. The heartbeat half is not evaluated (Out of scope). Add the sentence to 09 2.2 row "Sending files".

16. **Configuration and observability.** Add to the schema, with chapter defaults (02 13 item 5): `file.invocation_hold_max` (chapter 12) and `message.max_attachments` (chapter 03) if absent; `TestConfigSchemaBuilds` lists them; regenerate `docs/operations/configuration.md`. No new metric (`docs/observability/signals.md` has none for this path); logs carry only `bothy.run.id`, `bothy.file.id` and `bothy.message.id`, never file names (12 14). SQL visibility lint (M1-70, `tools/lint/sql_visibility.py`, AZ-P12): every new reading query on a content table carries the predicate or gets one exception entry in `tools/lint/sql_visibility.yaml` with its category and a reason: `after_authorize` for the file lock read of `Attach` (keyed by the listed IDs and the author after `message.create` was allowed), `ForMessages` (messages the caller already read), `Pending` (the attachments of one message whose creation or run was already decided) and the lock read of `DeleteForMessage` (after `message.delete`); `authorize_resolution` for the attachment read by `file_id` that `getFile` uses to build the resource and that `MessageOf` and `recordFileChange` reuse; `system_job` for the run lock of `ReleaseInvocation`, the held-run lookup and the read of the hold job. The `UPDATE` of `OnThreadMoved` and the inserts are writes and are not linted (rule 2). `make sql-visibility` stays clean.

17. **Specification and API** (rule 10; nothing in 00, 03, 04 or 05 changes; texts in English like the chapters, YAML in English):

    - `docs/spec/12-files.md` 10: the order and the single answer of note 4 (one `409 conflict` with `reason` `file_not_attachable` for every cause, `detail` with the pointer; blocked and pending files are attachable); `limit_reached`; empty content with attachments; `+upload` and the switch `supports_files_out`; `scope_channel_id` absent once attached; the release with `runs.invocation_held_until` (R-DM-44), exactly once, without `actor`, release time fixed at acceptance; follow-ups (`agent.followup`) are not held, the follow-on run of deferred follow-ups is (note 11 d and e); deletion with the message without `file.updated`. 12 2 last paragraph: the write that appends `file.updated` publishes `files.FileFinalized` when the file becomes final; attached files carry the scope of their message without audience.
    - `docs/spec/09-agents.md` 2.2 row "Sending files": "If `supports_files_out` is off, the server rejects messages of an agent with attachments with `403 feature_disabled`."
    - `docs/spec/02-architecture.md` 4.3 and 4.4 as note 1.
    - `docs/api/openapi.yaml` (descriptions only): `MessageCreate.attachment_file_ids` (own files uploaded for this channel with a completed session; at most `message.max_attachments`; needs `upload`; otherwise `409 conflict` with `reason` `file_not_attachable` or `400 validation_failed` with `errors[].code` `limit_reached`), `MessageCreate.content_md` (may be empty only with attachments), `createMessage` (an agent invocation with attachments waits until they are final or `file.invocation_hold_max`), `File.scope_channel_id` (only before the file is attached), `Message.attachments` (position order). Then `make codegen-check`, `make api-lint`, `make api-checks`, and `make api-tables` if a generated block of 06 changes.

18. **Rulings for `docs/DECISIONS.md`** (section 10, one row `D-M3-07-<n>` each, with reason and place): the edge `runs -> files` and no `AttachmentLocator` (note 1); one `409` for every attach failure with the pointer in `detail` and the lock before the check (note 4); empty content with attachments (note 5 a); `scope_channel_id` absent once attached (note 6); no `file.updated` on deletion with the message and one `deleted_at` function (note 7 c); `FileFinalized` published by `recordFileChange` (note 10 b); the file lock `FOR UPDATE OF files` without the session row (note 4 a); release time fixed at acceptance from one clock reading (note 11 b); the follow-on run asks `Pending` unconditionally (note 11 d); follow-ups are not held (note 11 e); ended runs are never released (note 12 b); no `actor` on a released invocation and `runs.version` plus one at the release (note 12 c); the hold job reschedules itself when early (note 13 b); no new index for the held-run lookup (note 2 c); the switch `supports_files_out` in the `runs` subscriber with `403 feature_disabled` (note 15); the categories of the SQL lint exceptions (note 16). The column `runs.invocation_held_until` itself is core contract (R-DM-44) and needs no row.

## Tests (write them first)

Test world: 07 10 and 11.1 (space Support with channels Allgemein and Eskalation, operator Lotse, `organization`, `chat`; G-1 Alex `trigger` with `upload` on Support; group Leser with Sam, G-4 `read` on Support/Allgemein; G-5 Lotse `write` and `upload` on Support; Robin without a grant on Support; the Owner). Files are uploaded through the real chain with `createUpload`, `uploadPart` and `completeUpload` (M3-03) and scanned by calling the `files.scan` handler of M3-04 directly; the scanner is the fake clamd server of `clamdtest` (answers `stream: OK`, the EICAR reply, or is stopped), except the test marked "real clamd". The adapter side is `pollAgentEvents` with Lotse's agent token (M2-07); the fake clock drives `file.invocation_hold_max` (small value in the test configuration).

Unit:

- `internal/files/domain`: `TestAttachable` (one row per condition of note 4 b: foreign uploader, purpose `avatar`, other channel (SEC-UP-9), session `open`, `aborted`, `expired`, `upload_state = 'failed'`, deleted, already attached; `infected`, `error` and `pending` scan are attachable); `TestEventTarget` (unattached: audience uploader, scope channel; attached: no audience, scope of the message); `TestBecameFinal` (every pair of 12 2 states; only not final to final is true; a deleted clean file is not deliverable, M3-04 `TestDeliverableAndFinal` stays).
- `internal/runs/domain`: `TestHoldUntil` (not pending: zero; pending: the given clock reading plus `file.invocation_hold_max`); `TestReleaseVerdict` (state `queued`, `running`, `failed`, `cancelled` times held or not times cause `final` or `timeout` times pending or not; release only for queued, held, and final without pending or timeout).
- `internal/messaging/domain`: `TestAttachmentBodyRules` (over `message.max_attachments` gives `limit_reached` with pointer `/attachment_file_ids`, in `createThread` `/first_message/attachment_file_ids`; empty content with one attachment passes, without gives `empty`; `kind = voice` still `not_enabled`).
- `internal/messaging/app` with a fake `files` port: `TestCreate_NoFilesCallWithoutAttachments` (no `Attach`, no `ForMessages` for a message without attachments; one `ForMessages` per page of messages with the flag).

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain, M3-01's file system driver in a `mktemp -d` root):

- `TestMigrationMessageAttachments`: columns, types, PK, both FKs with their `ON DELETE`, UNIQUE `(file_id)`, index `(thread_id)`, no FK on `thread_id` or `channel_id`; `TestMigrationHasAttachments` (column, default `false`, constraint validated after the second migration); `TestMigrationRunsInvocationHeldUntil` (nullable `timestamptz` column without default, `NULL` on existing rows, no new index); a second `migrate up` changes nothing.
- `TestAttach_SendWithAttachments`: Alex sends a message with two files in Support/Allgemein: `201` with `attachments` in request order (`position` 1 and 2), each `File` without `scope_channel_id` and without JSON `null`; two rows with `thread_id` and `channel_id` equal to the message's; `has_attachments = true`; the `message.created` payload validates against `MessageCreatedEvent` and lists both; a message without files has `has_attachments = false` and `attachments: []`. Variant with `clear_draft: true` after Alex saved a draft of the thread with both file IDs (`putThreadDraft`): the draft is gone in the same transaction (12 10 point 4, M1-61).
- `TestAttach_SecondAttach409` (plan, 12 15 row "Attaching the same file a second time"): F is attached to M1; a new message with F answers `409 conflict` with `reason` `file_not_attachable`; M2 is not stored and the next message of the thread gets the next `thread_seq` without a gap.
- `TestAttach_NotAttachable_SEC_UP_9`: a file of Alex uploaded for Support/Eskalation sent in Allgemein (SEC-UP-9), Lotse's file sent by Alex, an unknown ID, a file with an open session, an aborted upload, an upload failed by `checksum_mismatch`, a row with `purpose = 'avatar'` (fixture through the `pgtest` connection), the same ID twice in one request: each `409 conflict` with `reason` `file_not_attachable`, identical status, code, reason and headers, `detail` naming the entry; nothing stored.
- `TestAttach_ConcurrentSameFile`: two sends with F at the same time: exactly one `201`, one `409`.
- `TestAttach_LimitCapabilityNotEnabled`: over `message.max_attachments` gives `400 validation_failed` with `errors[].code` `limit_reached`; Robin with a `write` grant without `upload` on Support/Eskalation (granted by the Owner in the test) gets `403 forbidden` with `reason` `capability_missing`; the M1-56 `not_enabled` answer for `attachment_file_ids` is gone.
- `TestAttach_AttachmentOnlyAndPending`: a message with only a file and empty `content_md` is `201`; a file whose scan has not run yet is attachable and shows `scan_status: pending`.
- `TestAttach_ReplayAndHash`: the same request again answers `200` with `Idempotent-Replay: true`, the same attachments and no new row; the same `client_msg_id` with the files in another order answers `422 idempotency_key_reuse`.
- `TestAttach_CreateThreadFirstMessage`: Alex calls `createThread` in Support/Allgemein with `parent_message_id` (his message in the main thread) and `first_message.attachment_file_ids` (files uploaded for Allgemein): thread, message and attachment rows are created in one transaction, the rows carry the new thread's `thread_id`; over `message.max_attachments` and a foreign file answer with the `/first_message/attachment_file_ids` pointer.
- `TestAttachedFile_ReadPath` (Alex's message in a thread of Allgemein): Sam (`read` on Allgemein) gets `404` for Alex's file before it is attached and `200` after (`getFile`, no `scope_channel_id`); Robin always `404`, identical to an unknown ID; after the Owner moves the thread to Support/Eskalation Sam gets `404` and the Owner `200` (the copies decide). With M3-05 merged, `downloadFile` and `createFileTicket` follow the same rule.
- `TestAttachedFile_UpdatedEventScope`: F attached while pending; the scan outcome writes `file.updated` without audience in the scope `{space_id, channel_id, thread_id}` of the message; Sam receives it through `listEvents`; a file of Alex that is never attached sends its `file.updated` to Alex only.
- `TestFileFinalized_OncePerTransition`: a test subscriber counts `files.FileFinalized`: once for clean, infected and checksum failure, none for a retry round of M3-04, once for an abort of an unattached upload; with M3-06 merged, once for an image after its thumbnail transaction and never in the scan for that type.
- `TestHasAttachments_Lifecycle` (plan): `true` after a send with a file, `false` without, `false` after `deleteMessage`, with the `message_attachments` rows gone and `files.deleted_at` set in the same transaction (a test subscriber of `MessageDeleted` that vetoes leaves flag, rows and file unchanged); `getFile` then answers `404` for Alex and Sam; a second delete is `204` without writes.
- `TestHasAttachments_CheckRejectsDeletedWithFlag` (`pgtest`): a direct `UPDATE messages SET deleted_at = now(), content_md = ''` on a message with `has_attachments = true` fails the CHECK.
- `TestMoveThread_MovesAttachmentCopies` (plan): the Owner moves a thread with attachments from Allgemein to Eskalation: every `message_attachments.channel_id` of the thread equals the target in the same transaction; `thread_id` unchanged; unattached files keep `scope_channel_id`; a test subscriber registered after `files.OnThreadMoved` that vetoes rolls back the move and leaves the copies.
- `TestMoveThread_ConcurrentSendWithAttachment`: many parallel sends with attachments into the thread while it moves: at the end `message_attachments.channel_id` = `messages.channel_id` = `threads.channel_id` for every row.
- `TestHold_ReleasedOnClean` (plan, 12 15 row "Agent invocation with an attachment while clamd is down"): `auto_thread` on Allgemein; the fake clamd is stopped; Alex mentions Lotse with F (pending): `201` with `trigger_state = received`; the run in the new thread is `queued` with `invocation_held_until` = the fake clock at the send plus `file.invocation_hold_max`; one `runs.invocation_hold_timeout` job scheduled at that time; Lotse's `pollAgentEvents` shows no `agent.invocation`, and `trigger_state` stays `received` across further polls and a clock step below the limit. The fake clamd starts; the due `files.scan` makes F clean: in that transaction exactly one `agent.invocation` is appended, the column is `NULL` and `runs.version` is one higher; the next poll delivers it with `invocation_token` and `trigger_state` becomes `queued` (M2-08); the hold job, run after the limit, writes nothing.
- `TestHold_TimeoutReleasesPending` (plan): the fake clamd stays stopped; the clock passes `file.invocation_hold_max`; the hold job appends exactly one `agent.invocation` (no `actor`, scope of the run's thread, payload valid against `AgentInvocationPayload`); F is still `pending`; Lotse's `getFile` of F answers `200` with `scan_status: pending` (with M3-05 merged, `downloadFile` answers `409 file_not_ready`; with M2-13 merged, `getRunContext` lists the attachment with `scan_status` `pending`); when F becomes clean later, no second `agent.invocation` appears.
- `TestHold_RealClamdStoppedThenClean` (real clamd, `clamdtest` container): the same sequence as `TestHold_ReleasedOnClean` with the container stopped and started (M3-04 `TestScanClamdStoppedStaysPending` pattern).
- `TestHold_NoHoldWhenFinal`: F is clean before the send: `agent.invocation` in the message's transaction, column `NULL`, no hold job.
- `TestHold_LastOfTwoAttachments`: F1 and F2 pending; F1 clean: still held; F2 clean: released once. Variant: both outcome writes in parallel: exactly one `agent.invocation`, before the limit.
- `TestHold_BlockedOrFailedAttachmentReleases`: an EICAR reply makes F `infected` (final): released at once; Lotse's `getFile` shows `infected`.
- `TestHold_SameTransaction`: a test subscriber of `files.FileFinalized` registered after `runs.OnFileFinalized` vetoes once: neither the clean state of F, nor its `file.updated`, nor `agent.invocation` is stored; the retry stores all three.
- `TestHold_RunEndedWhileHeld`: with `run.queued_max_age` below `file.invocation_hold_max` in the test configuration, `runs.queued_expiry` (M2-08) fails the held run with `agent_unavailable`; later finalization and the hold job write no `agent.invocation`.
- `TestHold_ThreadMovedWhileHeld`: the thread of a held run moves to Eskalation; the released `agent.invocation` has the scope of Eskalation.
- `TestAgentAttachments_FilesOutSwitch`: Lotse uploads a file in Support and sends it in its thread: `201`; after the Owner sets `operator_config.supports_files_out = false` with `updateAgent`, the next send with a file gives `403 feature_disabled` and stores nothing; a send without files still passes.
- With M1-57 merged: `TestHistory_ShowsAttachments` (`getMessage` and `listThreadMessages` show the attachments; a deleted message shows none). For M2-11: a follow-on run whose first deferred message has a pending attachment is held like an acceptance and released once; a follow-up with a pending attachment to a running run gets its `agent.followup` at once (not held, note 11 e).
- `internal/wire`: the golden registrations show `files.FileFinalized -> runs.OnFileFinalized` and `chat.ThreadMoved -> files.OnThreadMoved` after the subscribers of `messaging` and `sync`; the job `runs.invocation_hold_timeout` on queue `default`; `TestConfigSchemaBuilds` lists `file.invocation_hold_max` and `message.max_attachments`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/files/... ./internal/messaging/... ./internal/runs/... ./internal/chat/... ./internal/agents/... ./internal/authz/... ./internal/wire/... ./migrations/...`
- `make migrations-lock` (once after creating the migrations), `make migrations-lint`, `make codegen-check`, `make arch`, `make register-check`, `make api-lint`, `make api-checks`, `make config-reference` (no diff afterwards), `make config-doc-check`, `make sql-visibility`
- `make diagrams`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `createMessage` and `createThread` accept `attachment_file_ids` with `+upload` (no longer `400 validation_failed` with `errors[].code` `not_enabled`); a second attach of the same file answers `409 conflict` with `reason` `file_not_attachable`, and so does every file that 12 10 point 1 refuses, including a file of another channel (SEC-UP-9).
- [ ] `message_attachments` exists as 03 4.7 says; every row carries `thread_id` and `channel_id` of its message; after `moveThread` the rows of the thread carry the target channel in the same transaction, a rolled back move leaves them, and a concurrent send ends consistent.
- [ ] `messages.has_attachments` is `true` after a send with a file, `false` without, and `false` after deleting, with the attachment rows gone and the files deleted in the same transaction; the CHECK rejects a deleted message with the flag.
- [ ] With clamd stopped, a message with an attachment that triggers Lotse keeps `trigger_state = received` and writes no `agent.invocation` until the attachment is `clean` (written in that transaction) or until `file.invocation_hold_max` (with the attachment still `pending`); each run gets exactly one `agent.invocation`, and `runs.invocation_held_until` is set exactly while it is held (R-DM-44).
- [ ] An attached file is read through its message's channel, `file.updated` carries the message's scope without audience, and every representation of a message lists its attachments.
- [ ] An agent with `supports_files_out = false` gets `403 feature_disabled` for a message with attachments.
- [ ] 02 4.3 and 4.4, `architecture/modules.yaml`, the C4 diagram, 12 2 and 10, 09 2.2, the openapi descriptions, `configuration.md`, the READMEs, the CHANGELOG and the rows `D-M3-07-<n>` are updated; `make register-check`, `make arch` and `make sql-visibility` are green.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `files` (table `message_attachments` with retention class `content`, port `Attachments`, the subscriber of `chat.ThreadMoved`, `recordFileChange` and the publish of `FileFinalized`, the `deleted_at` function, query exceptions), `messaging` (flag `has_attachments`, attach in the create path, attachments in the representation, the delete step) and `runs` (dependency on `files`, the hold, `ReleaseInvocation`, the subscriber, the job, the switch `supports_files_out`) updated; `agents` if its port changed.
- Rulings of Design note 18 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
