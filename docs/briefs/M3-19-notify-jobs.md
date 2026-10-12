# M3-19: Hint jobs, automatic following and bundling

Status: ready | Depends on: M1-38, M2-01, M2-09, M3-18 | Milestone: M3

## Goal

After this ticket hints arise in the worker, never in the write path: the worker consumer `push` (frame of M2-01, ADR 0051, 13 12 point 1) reads `message.created`, `run.state_changed`, `access.expiring`, `device.pending` and `read_state.updated` from the event log in `seq` order, determines the candidates of each event (readers of the channel through a new `authz` port, mentioned people, participants of a DM channel, the person who started a run, the audience of a reminder, the owner of a pending device), gathers the facts of each candidate in batches (the level facts and quiet-hour rules of M3-18, the personal switches, active devices, active push registrations, live connections through a presence port that M3-23 fills, the bundling window) and lets the pure decision of M3-17 (`Decide`) choose: no hint with a skip reason, collect into the open window, or send now. A send inserts one job `notify.push_send` per active push registration of the device (13 12 point 5) in the same transaction that moves the cursor, so a worker restart in the middle of a round neither doubles nor loses a hint. Agent messages written during an active run give no `message` hint; the end of the run gives exactly one hint per person: `run_done` (or `run_failed`) to the person who started it and `message` to the others with level `all`, both pointing at the last agent message of the run (13 2.1). A mention of a person without read right gives nothing and leaves no trace in jobs or logs (AZ-M48). Bundling per recipient and thread (13 4.5) keeps one unique scheduled River job per window: ten messages in one window give at most two hints, the second one with the unread count. The job `notify.push_send` checks `push.deliver` again with the rights of that moment (04 7.6), drops hints older than `push.max_age`, sends through a provider port `PushProvider` that M3-20 (APNs) and M3-21 (FCM) implement, applies the result classes of 13 7.3 to the registration and ends with a skip reason when no provider is registered for the registration. The ticket creates the table `push_registrations` exactly as 03 with a read port of the active registrations (D-W-108), and it builds automatic following (13 3.2): a person who writes in a thread, not in the main thread, follows it with level `all` in the transaction of the message, unless an own setting for that thread exists. It builds the source of `security` hints (R-AZ-31): `identity` writes the durable personal event `device.pending` in the transaction that registers a device with state `pending`, and the consumer maps it to one `security` hint to the other active devices of that human. Every push carries the badge number of 13 8, computed per recipient at send time, and a read that lowers the number gives at most one badge sync per person per `notify.badge_sync_min_interval` to the person's mobile registrations (D-W-108).

## Read first

- `docs/spec/13-push-notifications.md`: 1 "Goals"; 2 "Kinds of notifications and recipients" (the table and "No notification is created for"), 2.1 "Agent messages during a run", 2.2 "Report channels"; 3.2 "Automatic following"; 3.3 "Personal switches"; 4 "Decision per recipient" (4.1 to 4.6); 5.4 "Later" (the port `PushProvider`); 6.1 and 6.2 (which IDs a push carries; the payload itself is M3-20 and M3-21); 7 "Error handling" (7.1 to 7.3); 8 "Badge and sync" (the badge number and the badge sync, Design note 16); 9 "Delivery measurement" (row t0); 12 "Execution in the server" (all five points); 14 "Tests" (rows "Revocation between event and sending", "Mention of a person without read permission", "Ten messages in one thread within one window", "Run with three agent messages", "Automatic following"); 15 R-PN-02, R-PN-04, R-PN-06, R-PN-09; 17 "Configuration values of this chapter".
- `docs/spec/03-data-model.md` (core contract, read only): `push_registrations` (all columns, checks, unique keys, indexes, retention `security`); `notification_preferences` (PK, needed for following); `event_log` (columns, the index `(resource_kind, resource_id)`); `event_consumer_cursors`; `messages` (`origin`, `kind`, `silent`, `run_id` only for `kind = 'run'`, `thread_seq`); `runs` (`triggered_by`, `state`, the partial unique index "at most one active run per thread", R-DM-14); `read_states` and `threads` (`last_message_seq`, `unread_floor_seq`, the formula "Unread"); `mentions`; `devices` (`platform`, `state`); the `event_log` index `(audience_principal_id, seq)`; 6.3 "Lock order"; 9 `push.max_consecutive_failures`; R-DM-27.
- `docs/spec/04-sync-protocol.md` (core contract, read only): 3.2 "Pattern with a counter row" (a reader that sees `seq = k` sees every smaller number: commit order is the log order); 7.6 "Consumers in the worker (push, webhooks)"; 10 "Ordering".
- `docs/spec/05-actions.md` (core contract, read only): 4.9 row `push.deliver`; 5 "Check without a request" and R-AZ-29 (Design note 7); 6 rows `message.created`, `run.state_changed`, `access.expiring`, `device.pending`, `read_state.updated`, `notification_pref.updated`, `notification.created`; R-AZ-11; R-AZ-30; R-AZ-31.
- `docs/spec/07-permissions.md`: 7.4 "Query pattern" rule 1 (exception list for system reads); 9.4 "Expiry reminder" (who receives `access.expiring`); 10 and 11.1 (the test world); AZ-M48 and AZ-P5 in 11.
- `docs/spec/02-architecture.md`: 4.3 "Ports, events and sources per module" (rows `notify`, `authz`, `messaging`, `sync`, `identity`, `gateway`); 4.4 "Allowed dependencies" (row `notify`, "Tiers"); 4.5 "Cross-cutting ports (`internal/kernel`)"; 5 "Layers within a module"; 8.1 to 8.5; 11.5 "Worker consumers (push, webhooks)"; 12.1 to 12.4 (job arguments only IDs, queue `delivery`, maintenance mode, job catalogue row `notify.push_send`, rules for jobs); 13 item 5.
- `docs/spec/09-agents.md` 5.4 "Live box content" (the anchor message is the only message with `run_id`, silent) and 3.2 "Origin (`messages.origin`)".
- `docs/api/openapi.yaml` (read only): schemas `NotificationKind`, `MessagePayload`, `Message` (`thread_seq`, `origin`, `kind`, `silent`, `mention_ids`, `run_id`), `RunStateChangedPayload`, `RunSummary` (`triggered_by`, `agent_id`, `thread_id`, `channel_id`, `state`), `RunState`, `AccessExpiringPayload`, `DevicePendingEvent`, `DevicePendingPayload`, `DevicePlatform`, `ReadStatePayload`, `ReadState`, `PushRegistration`, `PushProvider`, `PushEnvironment`, `NotificationCreatedPayload`. `docs/api/asyncapi.yaml`: messages `event.message.created`, `event.run.state_changed`, `event.access.expiring`, `event.device.pending`, `event.read_state.updated`.
- `docs/operations/configuration.md`: `notify.coalesce_window`, `notify.mobile_delay_when_desktop_active`, `notify.user_default_level`, `notify.default_admin_enabled`, `notify.default_input_needed_breaks_quiet_hours`, `notify.badge_sync_min_interval`, `push.max_age`, `push.retry_base`, `push.retry_cap`, `push.max_concurrent_requests`, `push.max_consecutive_failures`, `consumer.batch_size`, `consumer.poll_interval`.
- `docs/observability/signals.md`: 4.4 rows `bothy.push.sent`, `bothy.push.send.duration`, `bothy.push.skipped`, `bothy.push.registrations.active`; 5 span `push.send`; 5.3 attributes `bothy.push.provider`, `bothy.outcome`, `bothy.reason`, `bothy.job.kind`; 7 "Logs".
- ADRs 0008 (hexagonal modules), 0017 (one `authorize`), 0031 (push without content), 0049 (action catalogue), 0050 (module register), 0051 (worker consumers of the event log).
- `docs/DECISIONS.md`: D-PN-01 to D-PN-04, D-PN-07, D-W-100, D-W-104 (R-AZ-25 to R-AZ-29), D-W-108 (this ticket's table, read port and provider port; R-AZ-30; point 6: R-AZ-31, the writer and mapping of `device.pending`, the badge and the badge sync, M7-06 for privacy deadline reminders), section 10 "Rulings of the build milestones" (format of the rows `D-M3-19-<n>`). `docs/OPEN-QUESTIONS.md` OQ-05 (automatic following and quiet hours, default "yes, as in chapter 13", due before this ticket). `docs/RISKS.md` RK-37 (presence per api node). `docs/security/threat-model.md` 5.12 "Push", T-PU-01, T-AZ-02, T-AZ-03.
- `docs/PLAN.md`: 3 "Way of working per ticket"; 5 "Test strategy"; 9 "M3 Platform services" ("Goal", "Exit criteria" points 1 and 2 with AZ-M48 and AZ-P5 for push, "Red team" item "content in push payloads", "Dependencies"); the rows M3-17 and M3-18 (inputs), M3-19 (this ticket, the contract), M3-20, M3-21 (provider port), M7-06 (reminder of a privacy request deadline), M3-22 (writes of `push_registrations`), M3-23 (presence, `notification.created`, delayed push), M3-24 (measurement); 8 rows M2-01 and M2-09; 19 P-11, P-13.
- Dependencies: the header names M1-38, M2-01, M2-09 and M3-18 (D-W-108: M1-38 because the source of `security` hints is written where a device registers as `pending`). Their closure brings what this brief uses: M1-38 (pending devices; tests create them through `CompleteLogin` of M1-37 with `TrustUnconfirmed`), M3-17 (`internal/notify/domain`), M2-08, M2-07 (`pollAgentEvents`), M2-05 (runs, anchor messages, `run.state_changed`), M1-71, M1-69, M1-68 (metric registry `internal/platform/otel/metrics`, `make signal-check`), M1-66 (focus with `idle`), M1-65, M1-63, M1-61 (`read_states`, `sync.Settings`), M1-60 (mentions), M1-56 (`createMessage`, `MessagePosted`), M1-55 (`chat.Threads.Info`), M1-53, M1-52, M1-49 (explanation candidates in `authz`), M1-48, M1-45, M1-44, M1-37, M1-34 (`auth_sessions`), M1-33 (`devices`, `identity.Devices`), M1-31, M1-28 (`authorize`), M1-26 (`identity.Directory`), M1-25 (catalogue), M1-16 (`event_log`, `sync.Event`), M1-14 (`platform/jobs`), M1-13 (unit of work, `txn.Subscribe`), M1-12 (`pgtest`, migrations), M1-07 and M1-08 (configuration), M1-06, M1-05, M1-01. Not in the closure and not needed: M1-30 (the job that writes `access.expiring`; tests append that event through the appender), M2-19 (agent answers naming `run_id`; agents post through M1-56 with `origin = agent`), M1-57, M1-70 (SQL visibility lint; Design note 15 says how to follow it where merged), M2-11.
- Briefs (merged names win, D-W-100): `M3-17-notify-decision.md` (all of it: `Decide`, `Flush`, `RecipientFacts`, `Device`, `Connection`, `Window`, `Policy`, `Switches`, `Decision`, `Route`, the `Via` and `SkipReason` constants, `TestDomainPurity`, its Out of scope list naming this ticket), `M3-18-notify-preferences.md` (`Facts.LevelFacts`, `Facts.QuietRules`, `upsertPreference`, the event builder of `notification_pref.updated`, `sync.Settings.ValuesFor`, `identity.Directory.Users`, `chat.Structure.NotificationPath`), `M2-01-worker-event-consumers.md` (`sync.ConsumerPush`, `sync.ConsumerHandler`, `HandleEvent`, `Consumers.Register`, the round, note 6 "no durable events in a round", `TestConsumerCrashMidRoundNoDuplicates`), `M2-09-run-lifecycle.md` (`startRun`, `awaitRunInput`, `finishRun`, `cancelRun`, the one end-of-run function), `M2-05-runs-state-machine.md` (`CreateRunAnchor`: anchor `kind = run`, `origin = agent`, `silent = true`; every state change writes `run.state_changed`, `queued` included), `M2-08-invocation-delivery.md` (how `runs.Delivery.Prepare` builds the human without request as principal, R-AZ-29), `M1-56-messaging-send.md` (`MessagePosted` with `ThreadKind`, `Origin`, `MentionedIDs`, `Silent`, `CreatedThread`), `M1-14-background-jobs.md` (`jobs.Args`, `Register`, `HandlerOptions`, `Inserter.InsertTx`, `InsertOptions.ScheduledAt`, `ValidateArgsType`: only `ids.ID`, `[]ids.ID` and `int64` fields), `M1-28-authz-core.md` (`Authorizer`, `Resource`, `Filter(ctx)`, `Scope`), `M1-49-authz-explain.md` (the explanation candidate queries and the random world generator of AZ-P8), `M1-33-identity-devices-dpop.md` (`identity.Devices`, `Register` with `RegisterDevice.State` `pending` or `active`, `DeviceState`), `M1-37-identity-bootstrap.md` (`CompleteLogin` with `TrustUnconfirmed`: the only path that registers a `pending` device), `M1-38-identity-device-approval.md` (pending devices; pairing creates active devices), `M1-60-messaging-mentions-templates.md` (`Mentions.CountUnread`, `UnreadMentionQuery`, `ThreadMentionCount`), `M3-20-apns-adapter.md` and `M3-21-fcm-adapter.md` (Design note 5: badge, sync form, `cleared`), `M1-61-sync-personal-data.md` (`sync.ReadStates`, the unread formula), `M1-66-gateway-focus-typing.md`, `M1-68-core-metrics.md`, `M1-12-postgres-migrations.md`, `M1-13-unit-of-work.md`, `M1-07-config-schema-loader.md`.
- Code on the base branch (all under `services/server/`): `internal/notify` (`domain/` of M3-17, `app/` and `adapters/` of M3-18), `internal/sync` (consumer frame of M2-01, appender, `Settings`, `ReadStates`), `internal/authz`, `internal/kernel/authorize`, `internal/messaging`, `internal/chat`, `internal/identity`, `internal/platform/jobs`, `internal/platform/otel/metrics`, `internal/wire`, `migrations/`, `architecture/modules.yaml`, `tools/archgen`.

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

11. Hints arise in the worker consumer `push` from the event log, never in the write path; producers do not know `notify`; one round inserts the jobs with `InsertTx` and moves the cursor in the same transaction, so every hint arises exactly once (13 12 point 1, R-PN-09, 02 11.5, 04 7.6, ADR 0051).
12. Candidates per event (13 2, 13 12 point 2): `message` to every active human with read right on the channel except the author; `mention` to every mentioned active human except the author; `dm` to the participants of a DM channel except the author; `run_done`, `run_failed` and `input_needed` only to `runs.triggered_by`; `security` only to the human who owns the device of a `device.pending`, routed only to the other active devices of that human; `admin` only to the people 07 9.4 names for the reminder. No hint for edits, deletions, reactions, pins, typing, live-box patches, `silent` messages and messages with `origin IN ('import','system')` (13 2 "No notification is created for", R-PN-11).
13. Agent messages of the agent of an active run in its thread give no `message` hint; at the end of the run exactly one hint per person: `run_done` to the starter and `message` to the others with level `all`, both pointing at the last agent message of the run; a mention by the agent goes at once (13 2.1, R-PN-02).
14. Every decision uses the pure functions of M3-17 with facts of the decision time: `push.deliver` with current rights for kinds with a thread, the recipient rule of R-AZ-30 for `security` and `admin`; a mention without read right gives no hint and leaves no trace (13 4.2, AZ-M48, R-AZ-30).
15. Bundling per recipient and thread with `notify.coalesce_window` as a unique scheduled River job: the first hint goes at once and opens the window, further `message` hints are collected, one hint with the unread count goes at the window end; the urgent kinds go at once (13 4.5, R-PN-06).
16. One job `notify.push_send` per active push registration of a routed device (13 12 point 5); before sending it checks `push.deliver` (or R-AZ-30) again with current rights (04 7.6, R-AZ-11); a hint older than `push.max_age` is dropped (13 7.3); at most `push.max_concurrent_requests` provider requests at a time per process (13 12 point 5).
17. Result classes of 13 7.1 to 7.3 change the registration: success resets `failure_count` and sets `last_success_at`; invalid tokens deactivate with their `disabled_reason`; transient failures raise `failure_count` and deactivate with `repeated_failures` at `push.max_consecutive_failures`; retries follow `push.retry_base`, `push.retry_cap` and a provider's `Retry-After` (13 7.2, 7.3).
18. A push carries only IDs, the kind, flags, a count and the badge number, never text; device tokens never appear in logs (ADR 0031, T-PU-01, 13 7.3 last point).
19. `push_registrations` exactly as 03; this ticket reads it and writes only the delivery columns (`last_success_at`, `failure_count`, `disabled_at`, `disabled_reason`); creation, removal, the preview token and the deactivation on logout, device revocation and user disable are M3-22 (D-W-108).
20. Automatic following: a human who writes in a thread with `kind = 'thread'` or creates it, and has no own setting for that thread, gets `notification_preferences(user, 'thread', id, level = 'all')` and one `notification_pref.updated` in the transaction of the message; never in the main thread (13 3.2, R-PN-04, OQ-05 default).
21. `identity` writes the durable personal event `device.pending` (resource `device`, receive action self, audience the human who owns the device, payload only `device_id` and `platform`) in the transaction that registers a device with state `pending` (R-AZ-31, 05 6).
22. Badge (13 8): every push carries the badge number of its recipient, computed at send time from `mentions`, `read_states.last_read_seq`, the effective level and the thread's channel kind through read ports of the owning modules, counting only threads the recipient can read; when a read lowers the number, the person's mobile registrations get at most one badge sync per `notify.badge_sync_min_interval`, without text (13 8, D-W-108).

## Scope

All paths under `services/server/` unless they start with `docs/` or `infra/`. Where an earlier ticket already created a file named below, extend it. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0` (`make licenses`).

Create:

- `migrations/<next>_notify_push_registrations.sql` (Design note 2) and its line in `migrations.sha256` through `make migrations-lock`.
- `internal/notify/registrations.go` (port): `PushRegistrations` with `Active` and the DTO `PushRegistration` (Design note 3).
- `internal/kernel/presence/presence.go`: `View`, `Connection`, `None` (Design note 10).
- `internal/notify/domain/codes.go` (`KindCode`, `KindFromCode`), `internal/notify/domain/result.go` (`ResultClass`, `SendResult`, `RegistrationState`, `ApplyResult`, `RetryDelay`), `internal/notify/domain/runstate.go` (`RunActive`) (Design notes 6, 12, 13); standard library and `kernel-core` only, so `TestDomainPurity` of M3-17 passes unchanged.
- `internal/notify/app/consumer.go` (`PushConsumer`, the handler of the consumer `push`), `internal/notify/app/sources.go` (classification of the five event types, Design note 4), `internal/notify/app/runs.go` (active run and last agent message, Design note 5), `internal/notify/app/gather.go` (facts per batch of candidates, Design note 8), `internal/notify/app/window.go` (Design note 9), `internal/notify/app/flush.go` (the flush use case), `internal/notify/app/send.go` (the send use case of `notify.push_send`, Design notes 11 to 14), `internal/notify/app/follow.go` (`OnMessagePosted`, Design note 17), `internal/notify/app/badge.go` (`BadgeCounter` and the badge sync, Design note 16), `internal/notify/app/ports.go` (outgoing ports `Scheduler`, `PushProvider`, `LiveRoutes`, `RegistrationStore`, Design notes 9, 10, 11).
- `internal/notify/adapters/postgres/queries/push_registrations.sql`, the sqlc output and the repository implementing `PushRegistrations` and `RegistrationStore`.
- `internal/notify/adapters/jobs/jobs.go`: the args `PushSend`, `CoalesceWindow`, `CoalesceFlush`, `BadgeSync`, `BadgeSyncHold`, their handlers and the `Scheduler` over `platform/jobs` (Design notes 9, 11 and 16).
- `internal/notify/adapters/fakepush/fakepush.go`: `FakeProvider`, a test double of `PushProvider` with programmable results per registration and a record of every call (Design note 12); used only by tests.
- No `internal/notify/adapters/otelmetrics`: M1-68 is in the closure, so the metric `Spec`s go into its registry `internal/platform/otel/metrics` (Design note 15).

Change:

- `internal/platform/jobs` (additive): `Inserter.InsertUniqueTx`, `InsertResult`, `Inserter.PendingTx` (Design notes 9 and 16), with tests against PostgreSQL.
- `internal/authz` port and adapter (additive): `Recipients.Readers` (Design note 6).
- `internal/sync` port and adapter (additive): `History.ResourceEvents`, `History.PersonalEvents`, `ReadStates.PositionsFor` and `ReadStates.PositionsOfUser` (Design notes 5, 9 and 16).
- `internal/chat` port and adapter (additive): `Threads.UnreadCandidates` (Design note 16).
- `internal/messaging` port and adapter (additive): `Messages.LastAnchorBefore`, `Messages.AnchorOf`, `Messages.LastAgentMessageBefore`, `Messages.Newest` (Design notes 5 and 9).
- `internal/identity` port and adapter (additive): `Devices.ActiveForUsers` (Design note 8), unless the merged port already returns the active devices of many people; the writer of `device.pending` in the use case `Register` of `internal/identity/app/devices.go` (Design note 4 d).
- `internal/notify/app/preferences.go` of M3-18: add `insertPreferenceIfAbsent` beside `upsertPreference` (Design note 17); the `Facts` adapter of M3-18: add `LevelFactsOfUser` (Design note 16).
- `internal/wire`: register the consumer `push` with `sync.Consumers.Register(sync.ConsumerPush, ...)` (replace the comment M2-01 left); register the five job kinds on queue `delivery`; the in-transaction subscription `messaging.MessagePosted -> notify.OnMessagePosted` after the subscribers of `runs` and `sync`; wire `presence.None` as `presence.View`, the default `LiveRoutes` and an empty provider set; the configuration keys of Design note 14; regenerate `testdata/txn_registrations.golden` and the jobs golden file if one exists.
- `architecture/modules.yaml`: `authz` in `notify.depends_on`; `internal/kernel/presence` in the kernel port component; regenerate `.go-arch-lint.yml` (`cd services/server && go generate ./tools/archgen`).
- Docs (Design note 18; new text in English, like the existing text): `docs/spec/13-push-notifications.md`, `docs/spec/02-architecture.md` 4.3, 4.4, 4.5, 12.3, `docs/architecture/c4-3-server-components.mmd` and its `.svg` (`make diagrams`), `docs/observability/signals.md` 4.4, `docs/operations/configuration.md` regenerated (`make config-reference`).
- READMEs of `notify` (keep the line `May use the ports of these modules: ...` in its form and add `authz`, because M1-06 compares it with `architecture/modules.yaml`), `authz`, `sync`, `messaging`, `chat`, `identity`, `internal/platform/jobs`, `internal/kernel` (if it lists the packages); `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` section 10 rows `D-M3-19-1` to `D-M3-19-15` (Design note 19).

This ticket adds no operation, no action, no durable or ephemeral event type, no problem code, no metric name and no top-level Makefile target. It writes the durable event `device.pending` that the core contract defines (R-AZ-31) and adds the job kinds `notify.coalesce_window`, `notify.coalesce_flush`, `notify.badge_sync` and `notify.badge_sync_hold` (02 12.3 in the same change) and the `bothy.reason` values of Design note 15 (signals.md in the same change).

## Out of scope

- The decision itself (`Decide`, `Flush`, `ReleaseDelayed`, `ResolveLevel`, `InQuietHours`): M3-17; preferences, quiet hours and their reads: M3-18. This ticket calls them and changes neither, except the additions named in the Scope.
- APNs and FCM payloads, headers, priorities, `interruption-level`, `push.ttl`, the provider JWT and OAuth tokens, the pause of a provider after a configuration error and its alarm: M3-20 and M3-21. They implement `PushProvider` (Design note 12); this ticket registers no production provider.
- `registerPush`, `unregisterPush`, the preview token, the preview endpoint, `notify.preview_mode`, the subscriptions `identity.DeviceRevoked` and `identity.SessionEnded` and the deactivation on logout, device revocation and user disable: M3-22 (on the table of this ticket).
- `notification.created` over the gateway, the gateway's implementation of `presence.View`, the delayed push to mobile devices while a desktop is active and its read check with `ReleaseDelayed`: M3-23 (it replaces `presence.None` and the default `LiveRoutes`, Design note 10). RK-37 (presence per api node) stays with M3-23 and M7.
- The measurement of 13 9 and `perf.push_handoff_p95`: M3-24. `bothy.push.fetch`: M3-22.
- Hints of kind `admin` for privacy request deadlines: M7-06 emits that reminder as a personal durable event and maps it in the consumer `push` (PLAN row M7-06, D-W-108); this ticket builds `admin` only from `access.expiring`.
- The `badge` of the preview endpoint (13 6.4): M3-22, which calls `BadgeCounter.For` of this ticket (Design note 16).
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M3-19-notify-jobs`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(notify): ...`, `feat(jobs): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Never log, print or persist a device token outside `push_registrations`; logs carry at most a short hash prefix of it. No network calls except package downloads for pinned tools and Go modules.
- No real push service in any test: providers are `fakepush.FakeProvider`; nothing in this ticket costs money.
- Never lower a threshold, disable a gate or delete a test.
- Time only through `kernel/clock`; tests move a fake clock and call job handlers directly, they never sleep for a window.
- Never move the cursor of the consumer `push` by hand outside a test database (M2-01).
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11). The principal for `push.deliver` (Design note 7) and the source of `security` hints (Design note 4 d) are settled by R-AZ-31 and are not a reason to stop.
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-17 and M3-18: follow their names in `internal/notify` and list each difference.

## Design notes

1. **Place and dependencies** (02 4.3, 4.4, 5; ADR 0008). Everything of this ticket lives in `notify`, except the additive port methods of `authz`, `sync`, `messaging` and `identity`, the unique insert of `platform/jobs` and the new kernel port `presence`. `notify` reads candidates through `authz` (13 12 point 2: "from `authz`"), which 02 4.4 does not list for `notify` yet. Ruling for docs/DECISIONS.md: add `authz` to "May use" of `notify` in 02 4.4 (reason: "candidates with read permission for notifications", English text), to `notify.depends_on` in `architecture/modules.yaml`, the edge `notify --> authz` to `docs/architecture/c4-3-server-components.mmd` and to the dependency line of the `notify` README; the edge points down (02 4.4 "Tiers"), and `make register-check` keeps the four equal. `notify` never imports `runs` or `gateway`: run facts come from the event payloads and the log (Design note 5), live connections through `presence.View` (Design note 10).

2. **Migration** (goose; the next free number at your local merge, renumber if a parallel ticket took it). `<next>_notify_push_registrations.sql` creates `push_registrations` exactly as 03: `id uuid NOT NULL` (PK), `user_id uuid NOT NULL`, `device_id uuid NOT NULL`, `auth_session_id uuid NOT NULL`, `provider text NOT NULL CHECK (provider IN ('apns','fcm','unifiedpush','wns'))`, `token text NOT NULL`, `environment text NOT NULL DEFAULT 'production' CHECK (environment IN ('production','sandbox'))`, `app_id text NOT NULL`, `preview_token_hash bytea`, `created_at timestamptz NOT NULL DEFAULT now()`, `last_success_at timestamptz`, `failure_count integer NOT NULL DEFAULT 0`, `disabled_at timestamptz`, `disabled_reason text CHECK (disabled_reason IN ('token_invalid','device_revoked','logout','topic_mismatch','sender_mismatch','repeated_failures'))`; check `(disabled_at IS NULL) = (disabled_reason IS NULL)`; FK `user_id -> users(principal_id) ON DELETE CASCADE`, `device_id -> devices(id) ON DELETE CASCADE`, `auth_session_id -> auth_sessions(id) ON DELETE CASCADE`; UNIQUE `(provider, token)`; unique partial index on `preview_token_hash WHERE preview_token_hash IS NOT NULL`; indexes `(user_id) WHERE disabled_at IS NULL` and `(auth_session_id)`. The table is new and empty, so constraints and indexes are created with it. Privileges for the application role follow the existing migrations. No column, check or index that 03 does not name.

3. **Read port of the active registrations** (02 4.3 row `notify`: `PushRegistrations`; D-W-108). In the root package, without `authorize` (callers select the people; the doc comment says so) and without the token:

    ```go
    // PushRegistrations reads the push targets of people (03 push_registrations).
    // M3-22 adds the writes on the same port.
    type PushRegistrations interface {
        // Active returns the registrations with disabled_at IS NULL of the given people,
        // grouped by person, each list in ascending id.
        Active(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]PushRegistration, error)
    }
    type PushRegistration struct {
        ID, UserID, DeviceID ids.ID
        Provider, Environment, AppID string
        CreatedAt, LastSuccessAt     time.Time // LastSuccessAt zero when NULL
        FailureCount                 int
    }
    ```

    The token and `preview_token_hash` never leave the `notify` adapters; the send use case reads the token through the outgoing port `RegistrationStore` (Design note 11). `Active` uses the index `(user_id) WHERE disabled_at IS NULL`.

4. **Event sources** (13 2, 12 point 1; Ruling for docs/DECISIONS.md). `PushConsumer.HandleEvent` (the `sync.ConsumerHandler` of M2-01) looks only at the event type and ignores every other type (the cursor still moves):

    a. `message.created` (payload `MessagePayload`): nothing for `silent`, `origin IN ('import','system')`, `kind IN ('run','system')`. In a DM channel (`ChannelKind = dm` from `chat.Threads.Info`): kind `dm` to every participant except the author. Otherwise: kind `mention` to every person of `mention_ids` except the author; kind `message` to every reader except the author and the people who got `mention` for this event, unless the message is an agent message during an active run (Design note 5). One hint per person and event; precedence `dm`, then `mention`, then `message`. Only active humans are recipients (`identity.Directory`; agents, apps and inactive people are dropped before any fact is gathered). `ThreadID` and the message ID come from the payload, the channel from the envelope scope.

    b. `run.state_changed` (payload `RunStateChangedPayload`): `waiting_input` gives `input_needed` to `run.triggered_by`; `completed` gives `run_done` to `run.triggered_by`; `failed` gives `run_failed` to `run.triggered_by`; `queued` and `running` give nothing. At `completed`, `failed` and `cancelled` the readers of the run's channel with the kind `message` get the run's single `message` hint (13 2.1), excluding the starter at `completed` and `failed` (who got `run_done` or `run_failed`) and including the starter at `cancelled` (13 2 gives no run kind for `cancelled`, so the starter is one of "the others"). Both point at the last agent message of the run; if the run wrote no agent message, `run_done`, `run_failed` and `input_needed` point at the thread only (no message ID) and no `message` hint arises. Ruling for docs/DECISIONS.md: the treatment of `failed` and `cancelled` for the others, because 13 2.1 says "At the end of the run" without naming the end states.

    c. `access.expiring` (payload `AccessExpiringPayload`, personal event): kind `admin` to `audience_principal_id`, if that principal is an active human; target ID `grant_id` for `kind = grant`, `group_id` for `kind = group_membership`. 07 9.4 names exactly the receivers of this event (subject, `granted_by`, the active owners, `added_by`), which is the recipient rule of R-AZ-30 for these reminders.

    d. `device.pending` (payload `DevicePendingPayload`, personal event, R-AZ-31): kind `security` to `audience_principal_id`, if that principal is an active human; target ID `device_id`, no thread, no message. The routes are the other active devices of that human: `Devices.ActiveForUsers` returns only `devices.state = 'active'`, so the pending device itself never gets the hint; no other person is a candidate. The recipient rule of R-AZ-30 for `security` at decision and send time: the recipient is an active human and the audience of the event, and the registration's device is active and is not the `device_id` of the payload. The writer (05 6 row `device.pending`): the use case `Register` of `identity.Devices` (M1-33, `internal/identity/app/devices.go`) appends one `device.pending` through `kernel/eventlog.Appender` in its ambient unit of work exactly when `RegisterDevice.State` is `pending`, with resource `{kind: device, id: device_id}`, audience the device's `UserID`, receive action self, payload only `device_id` and `platform`, and type and actions from the generated catalogue constants. `Register` is the only insert of a device row, and the only caller with `pending` is `CompleteLogin` of M1-37 with `TrustUnconfirmed` (pairing of M1-38 creates active devices), so every pending device gets exactly one event and an active one none. A veto later in the unit of work removes the device and the event. Ruling for docs/DECISIONS.md.

    e. `read_state.updated` (payload `ReadStatePayload`, personal event): no hint; it is the trigger of the badge sync of Design note 16.

5. **Agent messages during a run, by commit order** (13 2.1; 04 3.2; Ruling for docs/DECISIONS.md). 03 stores no run on an agent answer (`messages.run_id` is set only for the anchor, M2-19), and the consumer may process a message after the run already ended, so the run state of the moment of processing is not the answer. The log order is the commit order, and per thread `thread_seq` follows it (the thread row lock of `AppendToThread`), so:

    a. An agent message M (`origin = agent`, author A, thread T, `thread_seq` s, log `seq` m) is during an active run exactly when `messaging.Messages.LastAnchorBefore(T, s)` returns an anchor of run R and the last `run.state_changed` of R with `seq < m` (`sync.History.ResourceEvents` with resource `run` R, type `run.state_changed`, before m) has a state in `queued`, `running`, `waiting_input` (`domain.RunActive`) and `run.agent_id = A`. At most one run is active per thread (R-DM-14), so the newest anchor before M is the only candidate. No anchor, no retained event or another agent: not during a run, normal `message` hints.

    b. The last agent message of run R, ended by the event e (`seq` e): `messaging.Messages.AnchorOf(R)` gives the anchor's `thread_seq` a; then `messaging.Messages.LastAgentMessageBefore(T, A, a, before)` with `before` starting above every sequence returns the newest message of A in T with `a < thread_seq < before`, `origin = agent`, `kind IN ('text','voice','card')` and not deleted; it is the answer if `sync.History.ResourceEvents` (resource `message`, type `message.created`, before e) finds its creation event, else repeat with `before` = its `thread_seq`. Messages written after the end are skipped this way; the loop ends at the anchor.

    c. Both reads use the index `(resource_kind, resource_id)` of `event_log` and the thread index of `messages`; none filters by rights, because the consumer reads like the log tail and every hint is checked with `push.deliver` (07 7.4 rule 1, Design note 15).

    New port methods (additive, never `authorize`, doc comments say so):

    ```go
    // sync: History reads retained events of one resource in commit order for worker consumers.
    type History interface {
        ResourceEvents(ctx context.Context, q ResourceQuery) ([]Event, error) // ascending seq
    }
    type ResourceQuery struct {
        ResourceKind string
        ResourceID   ids.ID
        Types        []string
        Before       int64 // only seq < Before
    }
    // messaging.Messages additions
    LastAnchorBefore(ctx context.Context, threadID ids.ID, beforeThreadSeq int64) (AnchorRef, bool, error)
    AnchorOf(ctx context.Context, runID ids.ID) (AnchorRef, bool, error)
    LastAgentMessageBefore(ctx context.Context, threadID, agentID ids.ID, afterThreadSeq, beforeThreadSeq int64) (MessageRef, bool, error)
    Newest(ctx context.Context, threadID ids.ID) (MessageRef, bool, error) // newest not deleted, kind not run or system
    type AnchorRef struct { MessageID, RunID ids.ID; ThreadSeq int64 }
    type MessageRef struct { ID ids.ID; ThreadSeq int64 }
    ```

6. **Candidates through `authz`** (13 12 point 2; Ruling for docs/DECISIONS.md). New port in the `authz` root package:

    ```go
    // Recipients answers who may be a recipient of a hint in a channel (13 12 point 2).
    type Recipients interface {
        // Readers returns the active humans that a source of 07 makes able to read the
        // channel: owners (not for DM channels, SR6), grants on the instance, the space or
        // the channel to the human or to a valid group of the human, and the participants
        // of a DM channel; ascending, a superset that authorize decides exactly.
        Readers(ctx context.Context, channelID ids.ID) ([]ids.ID, error)
    }
    ```

    It reuses the explanation candidate queries of M1-49 in `internal/authz/adapters/postgres` (no copy). The exact answer per candidate is `authorize(r, push.deliver, thread)` in filter mode (`authorize.Filter(ctx)`, no denial audit), so a superset never leaks: a candidate the guards exclude gets `Allowed = false`. Mentions and run starters do not go through `Readers`; they are candidates by name and get the same `authorize`.

7. **The principal of a recipient** (05 5 "Check without a request", R-AZ-29, R-AZ-14, R-AZ-30, R-AZ-31; Ruling for docs/DECISIONS.md). `push.deliver` evaluates the rights of a human without a request of that human, at decision time and again at send time. Build that principal exactly as `runs.Delivery.Prepare` of M2-08 builds the human without request: no session, device or token, blocks only from the person's status, owner status from the current owner list. 05 section 5 lists "`push.deliver` before each hint decision and each push send, and the recipient rule of R-AZ-30 for hints without a thread" as checks without a request, so R-AZ-29 applies (R-AZ-31); an owner who reads a channel only as owner gets hints like any reader. The ruling adds nothing to anyone's rights: it is the read right the person has in a session of the same moment. For `security` and `admin` the fact `Allowed` is: the recipient is the audience of the event and an active human at that moment (R-AZ-30), for `security` with the device rule of Design note 4 d.

8. **Gathering facts** (13 12 points 2 to 4; M3-17 Design note 2). Per event and kind, for all candidates at once: `Facts.LevelFacts(threadID, userIDs)` and `Facts.QuietRules(userIDs)` of M3-18; the switches `notify.admin_enabled` and `notify.input_needed_breaks_quiet_hours` through `sync.Settings.ValuesFor` with the defaults `notify.default_admin_enabled` and `notify.default_input_needed_breaks_quiet_hours` (a stored value that is not a boolean counts as unset and logs one Warn line without person data); active devices through `identity.Devices.ActiveForUsers(ctx, userIDs) (map[ids.ID][]DeviceRef, error)` with `DeviceRef{ID ids.ID; Platform string}` (`devices.state = 'active'`); registrations through `PushRegistrations.Active`, kept only for active devices; live connections through `presence.View.Connections`; the window through `Scheduler.WindowEnd` (Design note 9); `Policy` from `notify.coalesce_window` and `notify.mobile_delay_when_desktop_active`; `Now` one `kernel/clock` reading per event. `Author` is always false (the author is no candidate) and `RunStarter` is true exactly for the run kinds. `Decide` of M3-17 then runs per candidate; an error of `Decide` is an internal error that rolls the round back (M2-01 note 7), because a bad fact is a bug.

9. **Window state as River jobs** (13 4.5 "unique, scheduled job per (recipient, thread)"; M3-17 Design note 7; Ruling for docs/DECISIONS.md).

    a. `platform/jobs` gets two additive functions, River stays inside the package: `Inserter.InsertUniqueTx(ctx, args Args, opts *InsertOptions) (InsertResult, error)` with `InsertResult{Inserted bool; ScheduledAt time.Time}` inserts unless a job of the same kind with equal args is in a not finalized state (`available`, `pending`, `retryable`, `running`, `scheduled`), and returns the standing job's `ScheduledAt` when it does not insert (River unique jobs by args and state [13 Q1]); `Inserter.PendingTx(ctx, args Args) (scheduledAt time.Time, ok bool, err error)` reads the same not finalized job by kind and args in the ambient transaction. Both refuse to run outside a unit of work like `InsertTx`.

    b. Two job kinds of `notify` on queue `delivery` (02 12.2: they are part of push, so they keep running in maintenance mode like `notify.push_send`), args only `UserID, ThreadID ids.ID`: `notify.coalesce_window` marks an open window and is scheduled at its end (its handler does nothing); `notify.coalesce_flush` exists exactly when a `message` hint was collected in that window and is scheduled at the same end.

    c. In the consumer: `Window{End}` is the `ScheduledAt` of a pending `notify.coalesce_window` of (r, T) when it is after `Now`, else zero. A `send` with `OpensWindowUntil` inserts the window job unique at that time before its push jobs; if a late marker of the previous window still stands (`Inserted` false with a time not after `Now`), the hint goes without a new window. A `collect` inserts `notify.coalesce_flush` unique at `Window.End`; a duplicate means it was already collected.

    d. The flush handler, one unit of work at its scheduled time: gather the facts of r for T (Design note 8), `FlushState{Collected: 1, Unread: u}` with `u = threads.last_message_seq - max(read_states.last_read_seq, threads.unread_floor_seq)` (03 "Unread"; `chat.Threads.InfoForShare` of M1-61 for `LastMessageSeq` and `UnreadFloorSeq`, and the new `sync.ReadStates.PositionsFor(ctx, threadID, userIDs) (map[ids.ID]int64, error)`, absent row read as 0), `domain.Flush`; a `send` carries `Count = u`, points at `messaging.Messages.Newest(T)` (no message ID if none), has `DueAt` = the window end, inserts its push jobs and the next window job as in c.

10. **Presence and live routes** (13 12 point 4, 4.4, 4.6 point 1; RK-37; Ruling for docs/DECISIONS.md). `notify` and `gateway` are on the same top layer of 02 4.4, so the presence port is a cross-cutting kernel port, like `eventlog.EphemeralPublisher` that the gateway implements:

    ```go
    // Package presence: live gateway connections of people (13 12 point 4).
    // The gateway implements View (M3-23); until then internal/wire wires None.
    type View interface {
        // Connections returns, per person, one entry per device with a live connection.
        Connections(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]Connection, error)
    }
    type Connection struct {
        DeviceID ids.ID
        Focus    []ids.ID // threads of the last focus op (04 6.2)
        Idle     bool     // idle of the last focus op
    }
    // None reports no connection for anyone: nobody is active in a thread, no desktop is active.
    type None struct{}
    ```

    `gather.go` maps each `Connection` onto the `domain.Device` with the same ID (a connection of an unknown or inactive device is ignored). With `None` no decision yields the routes `gateway` or `push_delayed`. For those routes the outgoing port `LiveRoutes` (`Gateway(ctx, Hint, domain.Route) error`, `Delayed(ctx, Hint, domain.Route) error`) receives them; the default implementation of this ticket returns an internal error (fail closed, the round rolls back) and M3-23 replaces it with `notification.created` and the delayed push. Add the package to 02 4.5 (implemented by `gateway`, used by `notify`).

11. **The job `notify.push_send`** (13 12 point 5, 7.3, 9; 04 7.6; 02 8.5, 12.1, 12.4; Ruling for docs/DECISIONS.md).

    a. Args `PushSend{NotificationID, RegistrationID, RecipientID, ThreadID, MessageID, TargetID ids.ID; Cleared []ids.ID; Kind, Quiet, Count, DueAtUnixMicro, Attempt int64}`: `Kind` is `domain.KindCode` (fixed codes 1 to 8 in the order of `NotificationKind` and 9 for the badge sync `app.KindSync`, never renumbered), `Cleared` only for the badge sync (Design note 16), `Quiet` 0 or 1, `DueAt` is t0 of 13 9 (the `occurred_at` of the triggering event, or the window end of a flush). One `NotificationID` (UUIDv7, the `nid`) per hint and recipient, shared by the jobs of that hint's registrations. Queue `delivery`.

    b. Handler, step 1 in a unit of work: load the registration through `RegistrationStore` (with token); not found, `disabled_at` set or its device no longer active: skip `no_registration`; `now - DueAt > push.max_age`: skip `expired`; `push.deliver` for kinds with thread or the R-AZ-30 rule for `security` and `admin` with the rights of now (Design note 7), and for the badge sync only that the recipient is an active human: denied gives skip `no_access`; no provider registered for `provider`: skip `no_provider` (the job ends without error); the badge number of `RecipientID` through `BadgeCounter.For` (Design note 16). Commit.

    c. Step 2 outside any transaction (02 8.5), under a process-wide semaphore of `push.max_concurrent_requests`: `PushProvider.Send` with a deadline from the provider adapter; span `push.send`, `bothy.push.send.duration`, then `bothy.push.sent` with the outcome of Design note 13.

    d. Step 3 in a unit of work: `domain.ApplyResult` on the registration row locked `FOR UPDATE`, and for a retry `InsertTx` of the same args with `Attempt + 1` at the time of `domain.RetryDelay`; the handler then returns nil. A crash between step 2 and step 3 makes River run the job again, so a push may reach a device twice with the same `nid`; the app replaces by thread ID (13 6.1, 5.2). Ruling: at least once with one `nid`, because a provider call cannot join a transaction.

12. **Provider port** (13 5.4; D-W-108). An outgoing port of `notify/app`, implemented by `adapters/apns` (M3-20), `adapters/fcm` (M3-21) and `adapters/fakepush`:

    ```go
    // PushProvider sends one hint to one registration (13 5.4).
    type PushProvider interface {
        Name() string // value of push_registrations.provider it serves
        Send(ctx context.Context, reg Registration, n Notification) (domain.SendResult, error)
    }
    type Registration struct {
        ID, UserID, DeviceID         ids.ID
        Provider, Environment, AppID string
        Token                        string // never logged (13 7.3)
        CreatedAt                    time.Time
    }
    // Notification holds only IDs, the kind, flags, a count and the badge (ADR 0031).
    type Notification struct {
        ID, InstanceID, ThreadID, MessageID, TargetID ids.ID // zero when absent
        Kind     string   // NotificationKind, or KindSync for the badge sync of 13 8
        Quiet    bool
        Count    int64
        Badge    int64    // badge number of 13 8 at send time
        HasBadge bool     // true for every hint and every sync this ticket sends
        Cleared  []ids.ID // KindSync only: threads read since the last sync, complete or nil
    }
    // KindSync marks the badge sync of 13 8: no thread, no message, no text.
    const KindSync = "sync"
    ```

    These are the final names that M3-20 and M3-21 use (their Design note 5): a hint carries `Badge` with `HasBadge` (`aps.badge`, `data.badge`); the sync form is `Kind == KindSync` with `ID`, `InstanceID`, `Badge`, `HasBadge` and `Cleared`, and zero `ThreadID`, `MessageID`, `TargetID`, `Quiet` and `Count`. APNs sends it as `{"aps":{"badge":<n>}}` and ignores `Cleared`; FCM sends `k = "sync"`, `badge` and `cleared` (the IDs comma-separated, in the given order) with priority `normal`. `Cleared` is never a prefix: this ticket passes the complete list, and M3-21 leaves it out entirely when it does not fit the FCM data limit (D-M3-21-6). `InstanceID` is the instance record's ID, passed in by `internal/wire` at startup (`notify` may not use `instance`). A Go error from `Send` counts as a transient failure. `internal/wire` builds the provider set from the registered adapters, one per `Name`; this ticket registers none in production.

13. **Result classes** (13 7.1 to 7.3; Ruling for docs/DECISIONS.md). `domain.SendResult{Class ResultClass; DisableReason string; InvalidSince time.Time; RetryAfter time.Duration}` and pure `ApplyResult(state RegistrationState, res SendResult, now time.Time, maxFailures int) RegistrationChange`:

    | Class | Registration | Hint | `bothy.outcome` |
    |---|---|---|---|
    | `accepted` | `last_success_at = now`, `failure_count = 0` | done | `accepted` |
    | `disable` | `disabled_at = now`, `disabled_reason` = `token_invalid`, `topic_mismatch` or `sender_mismatch`; unchanged when `InvalidSince` is set and the row was created after it (13 7.1 row `410`) | done | `rejected_token` |
    | `retry` | `failure_count + 1`; at `push.max_consecutive_failures` `disabled_at = now` with `repeated_failures` | retried unless deactivated or past `push.max_age` | `provider_error` |
    | `throttled` | unchanged (the limit is the server's, not the token's) | retried | `throttled` |
    | `config_error`, `program_error` | unchanged | dropped (a retry cannot help; the adapter raises its alarm, M3-20, M3-21) | `rejected_other` |

    `RetryDelay(attempt int64, base, cap, retryAfter time.Duration, jitter float64) time.Duration` is `min(cap, base * 2^(attempt-1))` scaled by `jitter` in `[0.5, 1]` (the caller draws it), at least `retryAfter`; base `push.retry_base`, cap `push.retry_cap`. A retry whose time is past `DueAt + push.max_age` is dropped as `expired`.

14. **Configuration.** In the schema (if absent) with the defaults of 13 17 and 03 9: `notify.coalesce_window`, `notify.mobile_delay_when_desktop_active`, `notify.default_admin_enabled`, `notify.default_input_needed_breaks_quiet_hours`, `notify.badge_sync_min_interval` (in `internal/wire/configkeys_notify.go` of M3-18), `push.max_age`, `push.retry_base`, `push.retry_cap`, `push.max_concurrent_requests`, `push.max_consecutive_failures` (in `internal/wire/configkeys_push.go`, created if absent); `TestConfigSchemaBuilds` lists them; `make config-reference` and `make config-doc-check` show no diff. The provider keys `push.ttl`, `push.apns.*` and `push.fcm.*` stay with M3-20 and M3-21.

15. **Observability and queries** (signals.md; 17 rules; Ruling for docs/DECISIONS.md). Declare `bothy.push.sent`, `bothy.push.send.duration`, `bothy.push.skipped` and `bothy.push.registrations.active` as `Spec`s in the registry of M1-68 (`make signal-check`). `bothy.push.skipped` counts one per recipient and hint decided `none` and one per job that ends without sending, with `bothy.reason`: `no_access` (`not_allowed`, or denied at send), `preference` (`level`, `admin_disabled`), `active_in_thread`, `no_registration` (`no_route`, or the registration gone at send), `read` (a flush with nothing unread), `expired`, `no_provider`; `not_recipient` and `nothing_collected` are not counted (no hint existed for anyone). In signals.md 4.4 replace the list of `bothy.push.skipped` by these seven values, which removes `quiet_hours`: quiet hours make a hint quiet and never drop it (R-PN-05), so that value could never occur. `bothy.push.registrations.active` is an observable gauge per `bothy.push.provider` over the active rows. Logs carry only `bothy.push.provider`, `bothy.job.kind`, `bothy.reason` and `error.type`, never a token, a person, a thread or a text. The reads of Design notes 5 and 6 and the registration reads have no visibility predicate by design; where the SQL visibility lint of M1-70 is merged and flags them, add exception entries with category `log_tail` (the `sync` history read), `system_job` (the `messaging` reads, the `authz` candidate read and the registration reads of the jobs), each with its reason; never widen an existing entry.

16. **Badge and badge sync** (13 8; D-W-108; Ruling for docs/DECISIONS.md).

    a. `BadgeCounter.For(ctx, userID ids.ID) (int64, error)` in `internal/notify/app/badge.go` counts the threads T for r that r can read and that hold an unread mention of r, or are unread with effective level `all`, or are unread in a DM channel (13 8). Unread at position p: `LastMessageSeq > max(p, UnreadFloorSeq)` (03 "Unread"); p is `read_states.last_read_seq`, 0 without a row. Reads, never foreign tables: the visible channels through `authz` `Scope` (M1-28) for the principal of Design note 7 and the action `push.deliver`; the threads of these channels that can be unread through the new `chat.Threads.UnreadCandidates(ctx, channelIDs []ids.ID) ([]ThreadPosition, error)` with `ThreadPosition{ThreadID, ChannelID ids.ID; ChannelKind string; LastMessageSeq, UnreadFloorSeq int64}` (only threads with `last_message_seq > unread_floor_seq`, never `authorize`, doc comment says so); the positions through the new `sync.ReadStates.PositionsOfUser(ctx, userID ids.ID, threadIDs []ids.ID) (map[ids.ID]int64, error)`; unread mentions through `messaging.Mentions.CountUnread` of M1-60 with `PrincipalID` r, the visible channels and the positions; effective levels through the new `Facts.LevelFactsOfUser(ctx, userID ids.ID, threadIDs []ids.ID) (map[ids.ID]domain.LevelFacts, error)` beside `LevelFacts` of M3-18 and `domain.ResolveLevel`. Every thread that counts is confirmed with `authorize(r, push.deliver, thread)` in filter mode, so a thread r cannot read never counts. `notify.push_send` calls `For` in step 1 (Design note 11 b) and sets `Badge` and `HasBadge` of every `Notification`; M3-22 calls the same function for the preview.

    b. The badge sync. The consumer `push` handles `read_state.updated` of r for thread T (Design note 4 e): the read lowers the number exactly when T counts for r at the previous position and does not count at the new `read_state.last_read_seq`. The previous position is the `last_read_seq` of the newest earlier `read_state.updated` of r for T (`sync.History.ResourceEvents` with resource `thread` T, type `read_state.updated`, before the event, keeping only events with audience r), else 0. A mark as unread never lowers it. When it falls, the consumer schedules one sync of r: if `PendingTx(BadgeSync{UserID: r})` finds one, nothing (it will carry this read); else `InsertUniqueTx(BadgeSync{UserID: r, AfterSeq: seq - 1})` at the `ScheduledAt` of a pending `notify.badge_sync_hold` of r when that is after `Now`, else at `Now`.

    c. Two job kinds of `notify` on queue `delivery`: `notify.badge_sync` with args `BadgeSync{UserID ids.ID; AfterSeq int64}` and `notify.badge_sync_hold` with args `BadgeSyncHold{UserID ids.ID}` (handler does nothing). `BadgeSync` tags `UserID` with `river:"unique"`, and `InsertUniqueTx` and `PendingTx` compare only tagged fields when an args type tags any (River unique by args), else all fields, so a person has at most one pending sync. The `notify.badge_sync` handler, one unit of work: `BadgeCounter.For(r)`; `Cleared` = the threads of the `read_state.updated` events of r with `seq > AfterSeq` (new `sync.History.PersonalEvents(ctx, PersonalQuery{AudienceID ids.ID; Types []string; After int64}) ([]Event, error)` on the index `(audience_principal_id, seq)`, never `authorize`) that do not count now, distinct, in event order; insert `BadgeSyncHold{UserID: r}` unique at `Now + notify.badge_sync_min_interval`; then one `notify.push_send` with `Kind` `KindSync`, `Cleared` and a new `NotificationID` per active registration of r with provider `apns` or `fcm` on an active device with platform `ios` or `android` (13 8: mobile devices only). No registration gives no job. So at most one sync per person per `notify.badge_sync_min_interval`, and a number that does not fall gives none.

17. **Automatic following** (13 3.2, R-PN-04, OQ-05; Ruling for docs/DECISIONS.md). `notify.OnMessagePosted(ctx, messaging.MessagePosted)`, an in-transaction subscriber registered after those of `runs` and `sync`: return unless `Origin = human` and `ThreadKind = thread`; then `insertPreferenceIfAbsent(ctx, AuthorID, thread ThreadID, level all)`: `INSERT ... ON CONFLICT (user_id, target_kind, target_id) DO NOTHING RETURNING ...`; only an inserted row appends `notification_pref.updated` with the builder of M3-18 (`version = 1`). M3-18 Design note 6 names `upsertPreference` for this; an upsert would overwrite an existing `mute` or `mentions` of the person, which 13 3.2 forbids ("no own setting for this thread yet"), so this ticket adds the insert-if-absent sibling in the same file and documents both as the only write paths. The subscriber runs in the message's unit of work: a veto later in the unit removes the row and the event. Creating a thread with a first message is covered, because `createThread` publishes `MessagePosted` for that message in the new thread. A different answer to OQ-05 would change only this function.

18. **Specification** (rule 10; nothing in 00, 03, 04 or 05 changes; all new text in English, like the existing text):

    - `docs/spec/13-push-notifications.md`: 2.1, a paragraph: an agent message is during a run when the newest anchor before it belongs to a run that was active at its commit (Design note 5); at `failed` and `cancelled` the others get `message` as at `completed`, at `cancelled` including the starter; a run without agent message gives no `message` hint. 3.2, a sentence: following inserts only when no own row exists. 4.5, a paragraph: the window as the two unique jobs `notify.coalesce_window` and `notify.coalesce_flush` (Design note 9), the flush hint points at the newest message of the thread. 7.3, a paragraph with the table of Design note 13 and the at-least-once rule with one `nid`. 12 point 1: the five source events, `device.pending` as the source of `security` hints (R-AZ-31) and `read_state.updated` as the trigger of the badge sync; 8, a paragraph: the badge computation and the sync jobs of Design note 16; point 2: candidates through `authz` `Recipients.Readers`; point 4: `presence.View` in `internal/kernel/presence`; point 5: the jobs of Design note 11 and `no_provider`.
    - `docs/spec/02-architecture.md`: 4.3 rows `notify` (subscribes `messaging.MessagePosted` for following), `authz` (port `Recipients`), `sync` (port `History`), `messaging`, `chat` and `identity` (the read additions), `identity` (writes `device.pending`); 4.4 row `notify` (Design note 1); 4.5 row `kernel/presence`; 12.3 rows `notify.coalesce_window`, `notify.coalesce_flush`, `notify.badge_sync` and `notify.badge_sync_hold` (module `notify`, trigger consumer `push` or the job `notify.badge_sync`, queue `delivery`, rule in 13).
    - `docs/observability/signals.md` 4.4 row `bothy.push.skipped` (Design note 15).

19. **Rulings for `docs/DECISIONS.md`** (section 10, one row each with date, decision, reason and place): `D-M3-19-1` the edge `notify -> authz` and `Recipients.Readers` as a superset decided by `authorize` (notes 1, 6); `D-M3-19-2` the five source events, precedence `dm`, `mention`, `message`, one hint per person and event, the writer of `device.pending` in `Register` and the `security` mapping (note 4); `D-M3-19-3` run end hints for `completed`, `failed` and `cancelled` (note 4 b); `D-M3-19-4` agent messages during a run and the last agent message by commit order (note 5); `D-M3-19-5` the principal of R-AZ-29 for `push.deliver` (note 7); `D-M3-19-6` facts per batch, invalid stored switches count as unset (note 8); `D-M3-19-7` window state as two unique River jobs, `InsertUniqueTx` and `PendingTx`, the late marker rule (note 9 a to c); `D-M3-19-8` the flush hint: unread formula, newest message, t0 at the window end (note 9 d); `D-M3-19-9` `presence.View` in the kernel with `None` and `LiveRoutes` failing closed (note 10); `D-M3-19-10` the job steps, kind codes, one `nid` per hint, at least once (note 11); `D-M3-19-11` the provider port and `no_provider` (note 12); `D-M3-19-12` the result classes, `throttled` without failure count, the `410` timestamp rule (note 13); `D-M3-19-13` the skip reasons of `bothy.push.skipped` and the query exceptions (note 15); `D-M3-19-14` insert-if-absent for following (note 17); `D-M3-19-15` the badge count, the badge sync with its two unique jobs, `KindSync` and the complete `Cleared` list (notes 12, 16). Rulings 3, 4, 7, 8, 10, 11, 12 and 15 also go into 13 (note 18).

## Tests (write them first)

Test world: 07 10 and 11.1 (space Support with channels Allgemein and Eskalation, operator Lotse; G-1 Alex `trigger` on Support; group Leser with Sam, G-4 `read` on Support/Allgemein; G-5 Lotse `write` and `upload` on Support; Marketing with G-2 Sam `trigger` and operator Texter; Robin without a grant on Support; the DM between Alex and Sam; the Owner). Devices are created with the identity test helpers; registrations are fixture rows written through the `pgtest` connection, because `registerPush` comes with M3-22: Alex `phone` (`ios`, provider `apns`), Sam `phone` (`android`, provider `fcm`), the Owner `phone` (`ios`, provider `apns`). Providers are `fakepush.FakeProvider` registered as `apns` and `fcm`. Events are produced by the real use cases through the HTTP chain, except `access.expiring`, which the test appends through the `sync` appender (M1-30 is not in the closure). The consumer runs through the M2-01 runner in role `worker` with the jobs runtime in role `api` (insert only), as M2-01 note 15 does; job handlers are called directly after moving the fake clock. Durations and limits are fixture values of the test configuration. A job is identified by its args; "hints for X" counts the distinct `NotificationID`s of jobs with `RecipientID = X`.

Unit, `internal/notify/domain` (M3-17's `TestDomainPurity` passes over the new files):

- `TestKindCodesStable`: `KindCode` of the eight kinds in the order of `NotificationKind` gives 1 to 8 and `KindSync` gives 9; `KindFromCode` inverts it; 0 and 10 are errors.
- `TestApplyResult`: one row per line of the table of Design note 13: `accepted` resets `failure_count` from 3 to 0 and sets `last_success_at`; `disable` with each reason; `disable` `token_invalid` with `InvalidSince` before `CreatedAt` changes nothing; `retry` at `failure_count` `push.max_consecutive_failures - 1` deactivates with `repeated_failures`; `throttled` keeps the count; `config_error` and `program_error` change nothing and do not retry.
- `TestRetryDelay`: attempts 1 to 10 with base and cap from the fixture: doubling, capped, never below `retryAfter`, jitter bounds.
- `TestRunActive`: `queued`, `running`, `waiting_input` active; `completed`, `failed`, `cancelled` not.

Unit, `internal/notify/app` with fakes for every port (`authz`, `sync`, `messaging`, `chat`, `identity`, `presence.View`, `Scheduler`, `PushProvider`, `RegistrationStore`, the clock):

- `TestClassify_MessageCreated`: precedence `dm` over `mention` over `message`; author never a candidate; agents, apps and inactive people dropped; `silent`, `import`, `system`, anchor (`kind = run`) and `kind = system` give nothing; a `message.updated` gives nothing (13 2, R-PN-11).
- `TestClassify_RunStateChanged`: each state against Design note 4 b, including the starter at `cancelled` and no `message` hint for a run without agent message.
- `TestClassify_AccessExpiring`: audience a human gives `admin` with `grant_id` or `group_id`; audience an agent or an inactive human gives nothing.
- `TestClassify_DevicePending`: audience an active human gives `security` with `device_id` as target and no thread; an inactive human gives nothing; the pending device is never a route.
- `TestAgentMessageDuringRun`: with fake anchors and fake history: active state before the message's `seq` suppresses; a terminal state before it does not; a run of another agent does not; no anchor does not; a run that ended after the message's `seq` still suppresses (the processing-time state is ignored).
- `TestLastAgentMessage`: newest message before the end event wins; a message created after the end is skipped; none gives no message ID.
- `TestGather_OneCallPerPortPerEvent`: 50 readers give one call each to `LevelFacts`, `QuietRules`, `ValuesFor`, `ActiveForUsers`, `Active`, `Connections`; a stored switch `"yes"` counts as unset.
- `TestLiveRoutes_DefaultFailsClosed`: with a fake `presence.View` that reports a connection, the default `LiveRoutes` returns an internal error and the round rolls back; with `presence.None` no decision of the golden combinations yields `gateway` or `push_delayed`.
- `TestSend_Steps`: registration disabled, device revoked, too old, `push.deliver` denied, no provider: no `Send`, the right skip reason; `Send` never runs inside a transaction (the fake unit of work asserts it); the semaphore never lets more than `push.max_concurrent_requests` calls run at once (100 parallel jobs with a blocking fake).
- `TestNotificationCarriesNoText`: walks `Notification` and the args `PushSend`, `CoalesceWindow`, `CoalesceFlush`, `BadgeSync`, `BadgeSyncHold` with `reflect`: only `ids.ID`, `[]ids.ID`, `int64`, `bool` and the kind string of `Notification`; `jobs.ValidateArgsType` accepts the five args types.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`):

- `TestMigrationPushRegistrations`: columns, types, defaults, both checks, the three FKs with `ON DELETE CASCADE`, UNIQUE `(provider, token)`, the unique partial index on `preview_token_hash`, the two indexes; a second `migrate up` changes nothing; deleting a device removes its rows.
- `TestPushRegistrationsActive`: only rows with `disabled_at IS NULL`, grouped and ordered; the DTO has no token.
- `TestJobsUniqueInsertAndPending` (`platform/jobs`): a second unique insert of equal args returns `Inserted = false` and the first `ScheduledAt`; other args insert; with a field tagged `river:"unique"` args that differ only in untagged fields do not insert; `PendingTx` finds the job until it completes and not after; outside a unit of work both return an internal error.
- `TestTenMessagesInOneWindow_AtMostTwoHints` (PLAN row, 13 14): Alex creates thread T in Support/Allgemein (so he follows it with `all`) and marks T read up to his own first message (`putReadState` with `last_read_seq` 1, so the unread count starts at zero); the Owner writes ten messages in T at fake clock offsets 0 to 45 seconds within one `notify.coalesce_window`: the consumer creates one hint for Alex (one `notify.push_send` with `Count` 1 for his phone), one `notify.coalesce_window` and one `notify.coalesce_flush` at the window end; running the window and flush handlers at the end creates the second and last hint with `Count` 10. Variant: Alex reads T up to the tenth message before the end (`putReadState`): the flush creates nothing and counts `read`. Variant: a mention of Alex inside the window goes at once and leaves the window unchanged.
- `TestRunWithThreeAgentMessages_ExactlyOneRunDone` (PLAN row, 13 14): Sam sets `all` on T through `putNotificationPreference`; Alex mentions Lotse in T and the consumer runs: Sam gets one `message` hint for Alex's message, which opens Sam's window; the fake clock moves past `notify.coalesce_window` and the window and flush handlers run. Then Lotse polls (`pollAgentEvents`), starts the run (`startRun`), writes three messages in T with its agent token and finishes with `completed` (`finishRun`); the consumer runs again only after all of it committed. Alex gets exactly one hint, kind `run_done`, pointing at the third agent message; Sam gets exactly one further hint, kind `message`, pointing at the same message; no other hint for the three messages exists. Variants: `failed` gives `run_failed` to Alex and `message` to Sam; a cancel through `cancelRun` gives `message` to Alex and Sam; a fourth message of Lotse after the end, written once the fake clock has moved past `notify.coalesce_window` again, gives normal `message` hints and does not move the pointer; a run that waits for input (`awaitRunInput`) gives one `input_needed` to Alex, also when Alex stored `mute` on T.
- `TestAgentMentionDuringRun_GoesAtOnce`: Lotse's second message in the run mentions Sam and the consumer runs: Sam gets `mention` at once, which opens Sam's window; after the fake clock has moved past `notify.coalesce_window` the run ends and Sam still gets exactly one `message` at the end.
- `TestAZM48_MentionWithoutReadRightGivesNothing` (PLAN row, AZ-M48): Sam writes in Marketing/Kampagnen and mentions Alex: the mention row is stored (M1-60); for Alex there is no `notify.push_send`, no window job, no flush job and no log record naming him; `bothy.push.skipped` with `no_access` rose by one.
- `TestRestartMidRound_NoDuplicateNoLoss` (PLAN row; pattern of M2-01 `TestConsumerCrashMidRoundNoDuplicates`): writers produce messages in Allgemein while the consumer runs; a test hook in `HandleEvent` records `pg_backend_pid()` after inserting the jobs of some events and blocks; the test ends that backend with `pg_terminate_backend`; subtests also for a handler error, a panic and a cancelled context. A fresh runner reaches the head; for every message and recipient that the decision admits there is exactly one hint and exactly one `notify.push_send` per registration; no hint is missing.
- `TestRevocationBetweenEventAndSend` (13 14): Sam (level `all` on T) gets a hint; before the job runs the Owner removes Sam from group Leser; the job sends nothing and counts `no_access`.
- `TestPushSend_ProviderResults`: through the job with `FakeProvider` programmed per result class: the registration columns and the follow-up job as in Design note 13; a `Retry-After` of the fake is respected; a hint whose next retry is past `push.max_age` is dropped as `expired`; the token never appears in captured logs, only a hash prefix.
- `TestPushSend_NoProvider` (D-W-108): only `apns` is registered; Sam's `fcm` registration: the job ends without error, no call, skip `no_provider`.
- `TestPushSend_TooOld`: the clock passes `DueAt + push.max_age` before the job runs: no call, `expired`.
- `TestAdminHintFromAccessExpiring`: an appended `access.expiring` to Alex gives one `admin` hint with the `grant_id` as target and no thread; with `notify.admin_enabled = false` stored for Alex none (`preference`); an `access.expiring` to Lotse gives none.
- `TestSecurityHintFromDevicePending` (R-AZ-31): Alex has the active `phone`; a login of Alex through `CompleteLogin` with `TrustUnconfirmed` registers a second device as `pending`: its unit of work wrote exactly one `device.pending` with audience Alex, resource `device` and a payload that validates against `DevicePendingEvent` and holds only `device_id` and `platform` (no name, no key, no app version); the consumer gives Alex exactly one `security` hint with the `device_id` as target, one `notify.push_send` for the registration of Alex's `phone` and none for the pending device; Sam and the Owner get nothing; a device registered as `active` writes no `device.pending`; with Alex disabled before the job runs the job counts `no_access`.
- `TestBadgeCount` (13 8, table over `BadgeCounter.For` with Alex): an unread mention in a thread with level `mentions` counts; an unread thread with level `all` counts; an unread thread with level `mentions` and no mention does not; an unread DM thread counts; a read thread does not; a thread whose messages lie below `unread_floor_seq` does not; a thread in Marketing where Sam mentioned Alex (stored by M1-60, Alex cannot read it) never counts; after the Owner removes Alex's read right a counted thread stops counting; the `Badge` of the `Notification` that `FakeProvider` records equals `For` at the fake send time and `HasBadge` is true.
- `TestBadgeSync` (13 8, fake clock): Alex has a counted unread thread T in Support/Allgemein and reads it through `putReadState`: one `notify.badge_sync` at once, then one `notify.push_send` with `KindSync` for his `phone`, recorded with the lower `Badge`, `Cleared` = T and no thread, message or quiet flag; a second falling read before `notify.badge_sync_min_interval` ends creates one sync at the end of the interval and none before; a read that does not lower the number (a thread with level `mentions` and no mention, or a second read of T) and `markThreadUnread` give no sync; Sam's `fcm` registration gets the sync with `Cleared` when Sam reads.
- `TestDMHint`: a fixture DM channel of Alex and Sam (written through the `pgtest` connection, as M3-18 does): Sam's message gives Alex one `dm` hint, also when it mentions Alex.
- `TestAutomaticFollowing` (13 14 "Automatic following"): the Owner creates thread T2; Alex's first answer in T2 creates his row with `all` and one `notification_pref.updated` (audience Alex) in the same transaction; his second answer writes nothing; his message in the main thread of Allgemein writes nothing; with Alex's stored `mute` on T2 nothing changes; Lotse's message writes nothing; `createThread` by Alex makes him follow the new thread; a test subscriber of `messaging.MessagePosted` registered after `notify` that vetoes once leaves neither row nor event.
- `TestAZP5_PushOnlyReadableChannels` (PLAN M3 exit criterion 2, AZ-P5 for push): in worlds of the random generator of AZ-P8 (M1-49) with random messages and mentions, no `notify.push_send` exists for a recipient who cannot read the thread at decision time, and every recipient of a `message` hint is in `Recipients.Readers` and allowed by `authorize`.
- `TestRegistrationsActiveGauge`: with an SDK manual reader, `bothy.push.registrations.active` per provider equals the active rows.
- `internal/wire`: the consumer `push` is registered; the five job kinds on queue `delivery`; the golden registrations show `messaging.MessagePosted -> notify.OnMessagePosted` after the subscribers of `runs` and `sync`; `TestConfigSchemaBuilds` lists the keys of Design note 14; `make register-check` passes with the edge `notify -> authz`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/notify/... ./internal/authz/... ./internal/sync/... ./internal/messaging/... ./internal/identity/... ./internal/platform/jobs/... ./internal/kernel/... ./internal/wire/... ./migrations/...`
- `make migrations-lock` (once after creating the migration), `make migrations-lint`, `make codegen-check`, `make arch`, `make register-check`, `make config-reference` (no diff afterwards), `make config-doc-check`, `make signal-check`
- `make diagrams`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `push_registrations` exists exactly as 03; `PushRegistrations.Active` returns the active registrations without token; `make migrations-lint` is green.
- [ ] The consumer `push` creates hint jobs from `message.created`, `run.state_changed`, `access.expiring` and `device.pending` in the round that moves its cursor; a worker restart in the middle of a round creates no hint twice and loses none.
- [ ] Ten messages in one window give at most two hints, the second with the unread count; a read thread gives no flush hint.
- [ ] A run with three agent messages gives exactly one `run_done` to the starter and one `message` to the others with `all`, both pointing at the last agent message; agent mentions go at once.
- [ ] A mention without read right gives no job and no trace (AZ-M48); no hint reaches a person who cannot read the thread (AZ-P5 for push); a revocation between event and send stops the push.
- [ ] `notify.push_send` checks rights, age and provider before sending, sends outside transactions within `push.max_concurrent_requests`, and applies every result class of 13 7.3 to the registration; without a registered provider it ends with `no_provider`.
- [ ] Automatic following inserts `all` with one `notification_pref.updated` only in threads, only for humans and only without an own setting.
- [ ] `identity` writes one `device.pending` per device registered as `pending`; it gives one `security` hint to the other active devices of that human and none to anyone else.
- [ ] Every push carries the badge number of 13 8 counted only over readable threads; a read that lowers it gives at most one badge sync per person per `notify.badge_sync_min_interval` to mobile registrations, and none when it does not fall.
- [ ] `presence.View` with `presence.None` and the default `LiveRoutes` exist for M3-23; the provider port exists for M3-20 and M3-21.
- [ ] 13, 02, the C4 diagram, signals.md (`quiet_hours` removed from `bothy.push.skipped`), `configuration.md`, the READMEs, the CHANGELOG and the rows `D-M3-19-1` to `D-M3-19-15` are updated; `make register-check`, `make arch` and `make signal-check` are green.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `notify` updated in English (status, ports `PushRegistrations` and the outgoing ports, the table `push_registrations` with retention class `security`, the consumer `push` with its sources and candidates, the jobs `notify.push_send`, `notify.coalesce_window`, `notify.coalesce_flush`, `notify.badge_sync`, `notify.badge_sync_hold`, following, the badge, the skip reasons, what M3-20 to M3-23 plug in), keeping the line `May use the ports of these modules: ...` and the section `## Tables` in their form for M1-06 (with `authz` added); READMEs of `authz`, `sync`, `messaging`, `chat`, `identity` and `internal/platform/jobs` name their additions (`identity`: the writer of `device.pending`).
- Rulings of Design note 19 recorded in `docs/DECISIONS.md` (one row each with reason); rulings 3, 4, 7, 8, 10, 11, 12 and 15 also in `docs/spec/13-push-notifications.md`.
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
