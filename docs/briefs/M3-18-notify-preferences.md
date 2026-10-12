# M3-18: Notification preferences and quiet hours

Status: ready | Depends on: M1-61, M3-17 | Milestone: M3

## Goal

After this ticket the module `notify` owns the tables `notification_preferences` and `quiet_hours` and serves the seven operations of the group Preferences that read and write them: `listMyNotificationPreferences`, `putNotificationPreference`, `deleteNotificationPreference`, `listMyQuietHours`, `createQuietHoursRule`, `putQuietHoursRule` and `deleteQuietHoursRule`. A person sets a level `all`, `mentions` or `mute` (with an optional temporary `muted_until`) per space, channel or thread they can read, and keeps a list of quiet-hour rules with weekday bits, local start and end and an IANA time zone that falls back to `users.time_zone`. Every change writes one durable personal event (`notification_pref.updated` or `quiet_hours.updated`, `audience = self`), so all devices of the person converge in server commit order with "last write wins per row" (04 14). Archiving a space or a channel removes, in the same transaction, every preference that targets it or anything below it, and a target that is archived takes no new preference. On the read side the ticket builds what the push path needs: one batched query that yields, for one thread and many people, the level facts of 13 3.1 (own rows for thread, channel and space, the channel default `channels.default_notification_level` set by `channel.update`, the DM rule, the personal default `user_settings['notify.default_level']` and the instance default `notify.user_default_level`), proven against PostgreSQL through `ResolveLevel` of M3-17, and the quiet-hour rules of many people with their zones loaded. The consumer `push` of M3-19 calls these reads and the pure decision of M3-17; it never queries the two tables itself.

## Read first

- `docs/spec/13-push-notifications.md`: 2 "Kinds of notifications and recipients" (who is a candidate, to see what the read side serves); 2.2 "Report channels" (`channels.default_notification_level`, set by whoever may use `channel.update`); 3.1 "Effective level" (the seven steps, "If `muted_until` of the selected row lies in the future, `mute` applies"); 3.2 "Automatic following" (owned by M3-19; it reuses the write of this ticket); 3.3 "Personal switches" (`notify.default_level`); 3.4 "Quiet hours with time zone"; 12 "Execution in the server" point 2 ("A batched query on `notification_preferences` fetches the levels for all candidates and the three targets"); 13 "Endpoints and actions" (the rows of the seven operations and of `channel.update`); 14 "Tests" (rows "Decision table" and "Quiet hours in `Europe/Berlin`", owned by M3-17, and "Automatic following", owned by M3-19); 16 "Alignment with the core contract" rows 1 and 5; 17 "Configuration values of this chapter" (`notify.user_default_level`).
- `docs/spec/03-data-model.md` (core contract, read only): 1 "Principles" items 10 to 13; 4.2 `users` (`time_zone`); 4.5 `channels` (`kind`, `space_id` NULL exactly for `dm`, `default_notification_level`, `archived_at`), `spaces` (`archived_at`) and `threads`; 4.10 `user_settings`, `notification_preferences` (all columns, PK `(user_id, target_kind, target_id)`, no FK on the target, index `(target_kind, target_id)`, the paragraph "Effective level" after the table) and `quiet_hours` (all columns, check `weekdays BETWEEN 1 AND 127`, index `(user_id) WHERE enabled`); 6.3 "Lock order"; 8 R-DM-16 ("the application handles the cleanup on archiving").
- `docs/spec/04-sync-protocol.md` (core contract, read only): 2.2 "Envelope" (`scope`, `resource` without `version`, `audience`, payload rules 1 and 2: the appender rejects a payload above `sync.event_payload_max_bytes`); 9 "Delivery guarantee" point 3; 14 "Cross-device synchronization" (row "Notification, quiet hours": "per row, the last write wins"; the paragraph on lists of personal data that name only what the person can still read).
- `docs/spec/05-actions.md` (core contract, read only): 2.7 "Effect (`effect`)"; 4.9 rows `settings.read` ("notification levels, quiet hours"), `notification_pref.update` (resource space, channel, thread; right "self and read"), `quiet_hours.update` (resource user; right "self"), `push.deliver` (R-AZ-30, only to see what M3-19 builds); 6 rows `notification_pref.updated` and `quiet_hours.updated` (resource `user`, receive action `self`), `channel.updated` (`default_notification_level`); R-AZ-21.
- `docs/spec/00-glossary.md`: rows `notification_preference` and `quiet_hours`.
- `docs/spec/07-permissions.md`: 4.1 (reason `subject_kind_not_allowed`), 4.5 "Archived spaces and channels" (archiving is final; reading stays; management changes of an archived target end with `409 conflict`), 7.4 "Query pattern" rules 1, 3 and 6; 11.2 AZ-P1 and AZ-P4.
- `docs/spec/02-architecture.md`: 4.2 "Module list" (row `notify`); 4.3 "Ports, events and sources per module" (row `notify`: ports `Preferences`, `QuietHours`; subscriptions `chat.ChannelArchived`, `chat.SpaceArchived`; rows `chat`, `sync`, `identity`); 4.4 "Allowed dependencies" (row `notify`: `identity`, `chat`, `messaging`, `sync`); 5 "Layers within a module"; 8.1 to 8.3 (unit of work, in-transaction subscribers, veto); 13 "Configuration loading" item 5.
- `docs/spec/06-api.md`: 2 "Basic rules" (no `null`); 5 "Errors" points 3, 4 and 7; 7 "Idempotency" (rows "other POST and PATCH" and "PUT and DELETE", including "resources without a tombstone answer `404` after deletion"); 8 "Concurrency" points 3 and 4; 16 endpoint table "Preferences".
- `docs/spec/01-product.md`: 10.1 "When a notification arrives"; screen D9 (quiet hours "Mon-Fri 19:00-07:00, Sat-Sun all day", temporary muting); R-PR-05, R-PR-06.
- `docs/spec/20-privacy.md`: the rows `user_settings`, `notification_preferences` and `quiet_hours` of the data inventory (class `personal_sync`; quiet hours reveal working hours) and DP-T10 (pattern for "a foreign person's personal data is not retrievable").
- `docs/api/openapi.yaml`: the seven operations named in the Goal (tag Preferences; `createQuietHoursRule` takes `IdempotencyKey`); parameters `TargetKindPath`, `TargetIdPath`, `RuleIdPath`; schemas `NotificationLevel`, `NotificationPreference`, `NotificationPreferencePut`, `NotificationPreferenceList`, `QuietHoursRule`, `QuietHoursRuleInput`, `QuietHoursList`, `NotificationPrefPayload`, `QuietHoursPayload`, `NotificationPrefUpdatedEvent`, `QuietHoursUpdatedEvent`, `EventEnvelope`, `Problem` (`x-problem-codes`: `validation_failed`, `forbidden`, `not_found`, `conflict`), `FieldError`; response component `Conflict`; `updateChannel` with `ChannelUpdate.default_notification_level` and `clear_default_notification_level`. `docs/api/asyncapi.yaml`: messages `event.notification_pref.updated`, `event.quiet_hours.updated`.
- `docs/operations/configuration.md`: `notify.user_default_level`, `sync.event_payload_max_bytes`.
- ADRs 0008 (hexagonal modules), 0009 (event log), 0013 (spec first), 0015 (errors, idempotency), 0017 (one `authorize`, `404` instead of `403`), 0031 (push without content), 0050 (module register), 0052 (one envelope), 0053 (no JSON `null`).
- `docs/DECISIONS.md`: D-PN-01 to D-PN-04, D-K-G8 (`channels.default_notification_level`), D-K-G12 (no own read action), D-W-100, D-W-104, D-W-108 (R-AZ-30 and the push block order), section 10 "Rulings of the build milestones" (format of the rows `D-M3-18-<n>` you add). `docs/OPEN-QUESTIONS.md` OQ-05 (concerns M3-19, not this ticket). `docs/RISKS.md`: no RK row names this ticket. `docs/security/threat-model.md` T-PU-01 (only IDs leave the server; preferences never reach a push provider).
- `docs/PLAN.md`: 3 "Way of working per ticket"; 5 "Test strategy" (rows "Unit" and "Integration server"); 9 "M3 Platform services" ("Goal", "Exit criteria" point 1, "Test strategy"); the rows M3-17 (the pure decision this ticket feeds), M3-18 (this ticket, the contract), M3-19 (consumer, automatic following and bundling: the first reader of this ticket's reads), M3-22 and M3-23 (beside it); 7 rows M1-52, M1-53, M1-55, M1-61.
- Dependencies: the header names M1-61 and M3-17. Their closure contains every M1 ticket this brief uses: M1-60, M1-56, M1-55 (threads, `Threads.Info`), M1-53 (channels, `updateChannel` with the channel default, `archiveChannel`, `chat.ChannelArchived`), M1-52 (spaces, `archiveSpace`, `chat.SpaceArchived`), M1-51, M1-46, M1-44, M1-36, M1-35, M1-34, M1-33, M1-31 (`updateMyProfile` with `time_zone`), M1-29, M1-28 (`authorize`, filter mode, `Scope`), M1-26 (`identity.Directory`), M1-25 (catalogue constants), M1-20 (idempotency keys), M1-18 (HTTP chain, problem writer, request validation), M1-17 (`api/operations.yaml`), M1-16 (`eventlog.Appender`), M1-13 (unit of work, `txn.Subscribe`), M1-12 (`pgtest`, migrations), M1-08 and M1-07 (configuration schema and reference), M1-06 (module register check), M1-05, M1-03, M1-02, M1-01. Not in the closure and not needed: M1-19 (rate limits; the classes are declared in `openapi.yaml` and enforced where M1-19 is merged), M1-62 (moving a thread changes nothing here, because a thread preference names the thread), M1-63 (the snapshot carries no preferences), M1-70 (SQL visibility lint; Design note 15 says how to follow it where it is merged).
- Briefs (merged names win, D-W-100): `M3-17-notify-decision.md` (package `internal/notify/domain`: `Level`, `Preference`, `LevelFacts`, `EffectiveLevel`, `LevelSource`, `ResolveLevel`, `LocalTime`, `ParseLocalTime`, `QuietRule`, `InQuietHours`, `TestDomainPurity`, Design note 2 "Inputs" with the table of where each fact comes from, Design note 6 on `start_local = end_local`), `M1-61-sync-personal-data.md` (the pattern this ticket copies: personal events, no-op writes, `deleted: true` with the last version plus one and a restart at version 1, agent and app tokens rejected; the `sync` port `Settings`), `M1-53-chat-channels.md` (D1 `updateChannel` with `default_notification_level`, D2 archive with `ChannelArchived{ChannelID, SpaceID, ArchivedAt}`, D7 `StructureSource`), `M1-52-chat-spaces.md` (`ArchiveSpace`, `SpaceArchived{SpaceID}`, Out of scope "Notification preferences cleanup on archive"), `M1-55-chat-threads.md` (`Threads.Info`, `ThreadInfo`), `M1-31-identity-people.md` (`time_zone`: an IANA name accepted by `time.LoadLocation`, never `Local` or empty, `time/tzdata` imported), `M1-26-identity-principals.md` (`identity.Directory`, `UserView.TimeZone`), `M1-28-authz-core.md` (`authorize`, `authorize.Filter(ctx)`, `Scope`), `M1-16-event-log.md` (`eventlog.Event`, `Appender`, `AudiencePrincipalID`, payload size rule), `M1-13-unit-of-work.md` (`txn.Subscribe`, `testdata/txn_registrations.golden`), `M1-20-idempotency-keys.md`, `M1-17-api-server-stubs.md` (the `later` entries of the seven operations, ticket M3-18), `M1-12-postgres-migrations.md`, `M1-07-config-schema-loader.md` (`internal/wire/configkeys_*.go`, `TestConfigSchemaBuilds`), `M1-01-server-gates.md` (migration rules of `make migrations-lint`).
- Code on the base branch (all under `services/server/`): `internal/notify` (`doc.go`, `README.md`, `domain/` of M3-17), `internal/sync` (port `Settings`, `user_settings` queries), `internal/chat` (port `Structure`, `Threads`, `ChannelArchived`, `SpaceArchived`, the archive use cases), `internal/identity` (port `Directory`), `internal/kernel/authorize`, `internal/kernel/eventlog`, `internal/kernel/clock`, `internal/wire`, `api/operations.yaml`, `migrations/`, `architecture/modules.yaml`.

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

11. Tables exactly as 03: `notification_preferences` with PK `(user_id, target_kind, target_id)`, the polymorphic target without foreign key (R-DM-16) and the index `(target_kind, target_id)`; `quiet_hours` with the check `weekdays BETWEEN 1 AND 127` and the partial index `(user_id) WHERE enabled`; both `ON DELETE CASCADE` from `users(principal_id)`, retention class `personal_sync`.
12. Levels are set per space, channel or thread; `notification_pref.update` needs "self and read" on the target, `quiet_hours.update` "self", reads need `settings.read` (05 4.9, R-AZ-21). Agents and apps are refused (`403 forbidden` with `reason` `subject_kind_not_allowed`).
13. The effective level of a person for a thread is the first present value of 13 3.1: own row for the thread, the channel, the space; `channels.default_notification_level`; `all` in a DM channel; `user_settings['notify.default_level']`; `notify.user_default_level`. If the chosen row has `muted_until` in the future, `mute` applies (13 3.1, 03 `notification_preferences`). This ticket gathers the facts in one batched query for all candidates and the three targets (13 12 point 2) and resolves them only through `ResolveLevel` of M3-17.
14. The channel default is a column of `channels`, written by `channel.update` (`updateChannel`, M1-53) and reported by `channel.updated`; `notify` only reads it (13 2.2, D-K-G8).
15. Quiet hours: weekday bit mask with bit 0 = Monday, `start_local` and `end_local` as local `HH:MM`, `end_local <= start_local` crosses midnight, `time_zone` NULL means `users.time_zone`, `enabled` (13 3.4, 03 `quiet_hours`).
16. Every change writes exactly one durable personal event with `audience = self`: `notification_pref.updated` with `NotificationPrefPayload` (`deleted: true` on removal) or `quiet_hours.updated` with `QuietHoursPayload` (the complete list after the change); a write that changes nothing writes no event; the rule between devices is "last write wins per row" in server commit order (04 14, 05 6).
17. Archiving a space or channel removes the preferences of the archived target and of every channel and thread below it in the transaction of the archive (02 4.3 subscriptions `chat.ChannelArchived`, `chat.SpaceArchived`; R-DM-16).
18. Lists of personal data name only targets the person can still read; the rows stay stored (04 14; 07 7.4 rule 3). No foreign person can read or receive another person's preferences or rules; unknown and foreign IDs answer the same `404` (AZ-P4, R-AZ-01).
19. No JSON `null` in responses or events: an absent `muted_until` or `time_zone` is omitted (06 2, ADR 0053).

## Scope

All paths under `services/server/` unless they start with `docs/`. Where an M1 ticket already created a file named below, extend it. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0` (`make licenses`).

Create:

- `migrations/<next>_notify_notification_preferences.sql` and `migrations/<next + 1>_notify_quiet_hours.sql` (Design note 2), their lines in `migrations.sha256` through `make migrations-lock`.
- `internal/notify/preferences.go` (port): `Preferences`, `QuietHours` and their DTOs (Design note 3).
- `internal/notify/domain/target.go`: `TargetKind` (`space`, `channel`, `thread`) with `Valid`, `PreferenceUnchanged`; `internal/notify/domain/rule.go`: `RuleInput`, `ValidateRuleInput`, `RuleUnchanged` (Design notes 4 and 9). Standard library without I/O and `kernel-core` only, so `TestDomainPurity` of M3-17 keeps passing unchanged.
- `internal/notify/app/preferences.go` (list, put, delete, and the shared `upsertPreference` of Design note 6), `internal/notify/app/quiethours.go` (list, create, put, delete), `internal/notify/app/events.go` (the two event builders), `internal/notify/app/subscribers.go` (`OnChannelArchived`, `OnSpaceArchived`), `internal/notify/app/facts.go` (`LevelFacts`, `QuietRules`, Design notes 12 and 13).
- `internal/notify/adapters/postgres/queries/notification_preferences.sql`, `internal/notify/adapters/postgres/queries/quiet_hours.sql`, the sqlc output and the repositories that implement the outgoing ports of `app`.
- `internal/notify/adapters/httpapi/preferences.go`: the strict-server methods of the seven operations.

Change:

- `api/operations.yaml`: move the seven operations from `later` (ticket M3-18) into `enabled`; rerun the route generator (`make codegen-check` shows no diff afterwards).
- `internal/chat` port and adapters, additive only (Design note 10): `Structure.NotificationPath`, `Structure.LockNotificationPath`, `Structure.ChannelIDsOfSpace`, `Structure.ThreadIDsOfChannels`, unless M1-52, M1-53 or M1-55 already offer methods with the same result (then use those and list them in the report).
- `internal/sync` port and adapter, additive only: `Settings.ValuesFor` (Design note 11).
- `internal/identity` port and adapter, additive only: `Directory.Users` (Design note 11), unless `Directory` already returns `UserView` for a list of IDs.
- `internal/wire`: register the handlers; the in-transaction subscriptions `chat.ChannelArchived -> notify.OnChannelArchived` and `chat.SpaceArchived -> notify.OnSpaceArchived`; regenerate `testdata/txn_registrations.golden`; the key `notify.user_default_level` in `internal/wire/configkeys_notify.go` (create it if absent) with the default and allowed values of 13 17 (Design note 14).
- `architecture/modules.yaml`: only if `notify.depends_on` lacks `chat`, `sync` or `identity` (02 4.4 already allows all three); then regenerate `.go-arch-lint.yml` (`cd services/server && go generate ./tools/archgen`).
- `docs/spec/13-push-notifications.md` (not core contract) and `docs/api/openapi.yaml` as Design note 16 lists; new text in English.
- `docs/operations/configuration.md` regenerated (`make config-reference`; expect no diff, the key is already listed).
- READMEs `internal/notify/README.md`, `internal/chat/README.md`, `internal/sync/README.md`, `internal/identity/README.md`; `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` section 10 rows `D-M3-18-1` to `D-M3-18-9` (Design note 17).

This ticket adds no action, no event type, no job, no metric, no problem code and no top-level Makefile target. It changes nothing in `internal/notify/domain` that M3-17 built, except adding the two files above.

## Out of scope

- The pure decision (`Decide`, `Flush`, `ReleaseDelayed`, `ResolveLevel`, `InQuietHours`): M3-17; this ticket calls `ResolveLevel` and `InQuietHours` only in its tests.
- The worker consumer `push`, candidates from `authz`, the `push.deliver` check, the window state, reading the switches `notify.admin_enabled` and `notify.input_needed_breaks_quiet_hours`, the job `notify.push_send`, the table `push_registrations` and its read port, automatic following of 13 3.2 (which calls `upsertPreference` of this ticket in the transaction of the message, Design note 6) and the metric `bothy.push.skipped`: M3-19.
- `registerPush`, `unregisterPush`, the preview token and endpoint, `notify.preview_mode`, the subscriptions `identity.DeviceRevoked` and `identity.SessionEnded`: M3-22. APNs and FCM: M3-20, M3-21. `notification.created` and presence: M3-23.
- Writing `channels.default_notification_level`: `updateChannel` of M1-53 (D1) already does it; this ticket only reads the column and tests the order with it.
- Direct messages and `dm_participants` (V3); the DM step of 13 3.1 is tested with a fixture channel of kind `dm` written through the `pgtest` connection.
- Export and erasure of the two tables for data subject requests (`kernel/personaldata.Participant`, M7, P-14); the rows go with the account by `ON DELETE CASCADE`.
- Showing preferences and quiet hours in the apps (M4, M5); the snapshot carries neither (openapi `Snapshot`), clients load them with the two list operations.
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M3-18-notify-preferences`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(notify): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- Time only through `kernel/clock`; tests move a fake clock, they never sleep.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11). The two tables, the actions, the events and their receive action are in the core contract; build them as written.
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-17: follow its names in `internal/notify/domain` and list each difference.

## Design notes

1. **Place and dependencies** (02 4.3, 4.4; ADR 0008). Everything lives in `notify`. It reads target paths and the channel default through the `chat` port, the personal default through the `sync` port and time zones through the `identity` port; all three edges are allowed by 02 4.4 ("target path for settings", "devices and time zone"). `notify` never reads a table of another module (03 1 item 12). The `domain` layer gets only pure helpers (Design notes 4 and 9); time zone loading, configuration and SQL stay in `app` and `adapters`.

2. **Migrations** (goose; sequential numbers and slugs as `make migrations-lint` requires; take the next free numbers when you rebase before the merge and renumber yours if a parallel ticket took them).

    a. `<next>_notify_notification_preferences.sql`: `user_id uuid NOT NULL`, `target_kind text NOT NULL CHECK (target_kind IN ('space','channel','thread'))`, `target_id uuid NOT NULL`, `level text NOT NULL CHECK (level IN ('all','mentions','mute'))`, `muted_until timestamptz` (NULL allowed), `version bigint NOT NULL DEFAULT 1`, `updated_at timestamptz NOT NULL DEFAULT now()`; PK `(user_id, target_kind, target_id)`; FK `user_id -> users(principal_id) ON DELETE CASCADE`; no FK on `target_id` (R-DM-16); index `(target_kind, target_id)`.

    b. `<next + 1>_notify_quiet_hours.sql`: `id uuid NOT NULL` (PK), `user_id uuid NOT NULL`, `weekdays smallint NOT NULL CHECK (weekdays BETWEEN 1 AND 127)`, `start_local time NOT NULL`, `end_local time NOT NULL`, `time_zone text` (NULL allowed), `enabled boolean NOT NULL DEFAULT true`, `version bigint NOT NULL DEFAULT 1`; FK `user_id -> users(principal_id) ON DELETE CASCADE`; index `(user_id) WHERE enabled`.

    Both tables are new and empty, so constraints and indexes are created with them. No column, CHECK or index that 03 does not name: the list of one person's rules reads few rows without an extra index. Privileges for the application role follow the existing migrations (`grep -n GRANT migrations/*.sql`).

3. **Ports** (02 4.3 row `notify`). Own DTOs in the root package, no domain types; actor and device come from `kernel/actor` in the context; every method runs in a unit of work and calls `authorize` itself:

    ```go
    // Preferences reads and writes the caller's notification levels (13 3.1).
    type Preferences interface {
        List(ctx context.Context) ([]Preference, error)
        Put(ctx context.Context, cmd PutPreference) (Preference, error)
        Delete(ctx context.Context, target Target) error
    }
    // QuietHours reads and writes the caller's quiet-hour rules (13 3.4).
    type QuietHours interface {
        List(ctx context.Context) ([]QuietHoursRule, error)
        Create(ctx context.Context, in QuietHoursInput) (QuietHoursRule, error)
        Put(ctx context.Context, ruleID ids.ID, in QuietHoursInput) (QuietHoursRule, error)
        Delete(ctx context.Context, ruleID ids.ID) error
    }
    type Target struct { Kind string; ID ids.ID }            // kind: space, channel, thread
    type PutPreference struct { Target Target; Level string; MutedUntil time.Time } // zero: none
    type Preference struct { Target Target; Level string; MutedUntil time.Time; Version int64 }
    type QuietHoursInput struct { Weekdays int; StartLocal, EndLocal string; TimeZone string; Enabled bool } // "": NULL
    type QuietHoursRule struct { ID ids.ID; Weekdays int; StartLocal, EndLocal string; TimeZone string; Enabled bool; Version int64 }
    ```

    The handlers in `adapters/httpapi` map the generated types to these DTOs and back; zero `MutedUntil` and empty `TimeZone` are omitted in JSON (no `null`).

4. **Writing a preference** (`putNotificationPreference`, action `notification_pref.update`; Ruling for docs/DECISIONS.md `D-M3-18-1` and `D-M3-18-2`). In one unit of work, in this order:

    a. Path and lock: `chat.Structure.LockNotificationPath(ctx, kind, id)` (Design note 10) resolves the target and takes `FOR SHARE` on the thread row (kind `thread`), then the channel row (kinds `thread` and `channel`), then the space row (if the channel has one), in that order, the same order `moveThread` of M1-62 uses (thread first, then channel and space). An unknown ID, or an ID of another kind than `target_kind`, gives `not_found`.

    b. `authorize(actor, notification_pref.update, <target>)` with the resource of the target kind (`space`, `channel` or `thread` with the path hints of the lock result). An agent or app token gives `403 forbidden` with `reason` `subject_kind_not_allowed` (as M1-61 note 22 built for `self` actions); no read right gives `404 not_found`, identical to an unknown ID (AZ-P4). Every thread can be a target, the main thread of a text channel included: the automatic following of 13 3.2 skips the main thread, an explicit setting does not.

    c. Archived target: if the space, the channel or the channel's space is archived (`NotificationPath.Archived`), answer `409 conflict` with `reason` `target_archived` and write nothing. Reason: the cleanup of Design note 7 runs once, at archive time; a row written later would stay forever on a target that can never get a message (07 4.5 point 2 treats management writes on archived targets the same way). The `FOR SHARE` locks of step a make this exact: an archive holds the channel or space row `FOR UPDATE` until it commits, so a put either commits before the archive (and its row is removed by the cleanup) or sees `Archived` after it.

    d. Normalize: `muted_until` is truncated to whole microseconds (the precision of `timestamptz`) and stored as given, also when it lies in the past; a past value has no effect, because `ResolveLevel` applies it only while it is in the future (Ruling `D-M3-18-3`). Reason: a client in an outbox may send a mute that has expired meanwhile; refusing it would fail the entry for no gain.

    e. Write with one statement: `INSERT ... ON CONFLICT (user_id, target_kind, target_id) DO UPDATE SET level = EXCLUDED.level, muted_until = EXCLUDED.muted_until, version = notification_preferences.version + 1, updated_at = now() WHERE (notification_preferences.level, notification_preferences.muted_until) IS DISTINCT FROM (EXCLUDED.level, EXCLUDED.muted_until) RETURNING ...`. A new row has `version = 1`. No returned row means the stored values are equal: read the row and answer `200` with it, without event and without version change (`domain.PreferenceUnchanged` documents the comparison). Two parallel first writes therefore end in one row; with equal values only one of them writes an event.

    f. On a change: append `notification_pref.updated` (Design note 8) and answer `200` with `NotificationPreference`.

5. **Deleting and listing preferences** (`deleteNotificationPreference`, `listMyNotificationPreferences`).

    a. Delete runs steps a and b of note 4 (an archived target is allowed, so a person can always clear a row), then `DELETE ... WHERE user_id = <actor> AND target_kind = $1 AND target_id = $2 RETURNING ...`. A deleted row writes `notification_pref.updated` with `{preference: <last stored values with version = last version + 1>, deleted: true}`; no row answers `204` without event (06 7 "PUT and DELETE"). A later put starts again at `version = 1`, as M1-61 rules for settings (Ruling `D-M3-18-4`).

    b. List (`settings.read` on the own user): every row with `user_id = <actor>`, ordered by `target_kind`, then `target_id`; each row is checked with `authorize(actor, space.read | channel.read | thread.read, <target>)` in filter mode `authorize.Filter(ctx)` (no denial audit, 07 4.2) and dropped on deny or when the target no longer resolves. Rows of targets the person lost stay stored and appear again when the right returns (04 14). There is no SQL visibility predicate to compare with (the table holds the person's own rows), so a dropped row is not a `filter_mismatch` and raises no alarm.

6. **`upsertPreference` for M3-19** (13 3.2). Steps d to f of note 4 are one function `upsertPreference(ctx, userID, target, level, mutedUntil)` in `internal/notify/app/preferences.go`, documented as the single write path of `notification_preferences`; it neither authorizes nor locks the path (its callers do). M3-19 calls it for automatic following inside the message transaction ("creates in the same transaction ... and writes `notification_pref.updated`"); the README names it.

7. **Cleanup on archive** (R-DM-16; 02 4.3; Ruling `D-M3-18-5`). Two in-transaction subscribers, registered in `internal/wire`:

    a. `OnChannelArchived(ctx, chat.ChannelArchived)`: targets are the channel and every thread of it (`chat.Structure.ThreadIDsOfChannels(ctx, []ids.ID{ChannelID})`). `OnSpaceArchived(ctx, chat.SpaceArchived)`: the space, every channel of it, archived or not (`chat.Structure.ChannelIDsOfSpace`), and every thread of those channels.

    b. `DELETE FROM notification_preferences WHERE (target_kind = 'space' AND target_id = $space) OR (target_kind = 'channel' AND target_id = ANY($channels)) OR (target_kind = 'thread' AND target_id = ANY($threads)) RETURNING ...`, then one `notification_pref.updated` with `deleted: true` (version last plus one) per deleted row, audience the row's person, appended in ascending `(user_id, target_kind, target_id)` order in the archive's unit of work, with the actor of that unit. A veto later in the unit rolls everything back (02 8.3).

    c. Concurrency: the archive holds the channel or space row `FOR UPDATE`; every put and delete holds the path `FOR SHARE` before it touches a preference row (note 4 a), so no other writer touches the rows of the archived path while the cleanup runs, and no deadlock arises (the cleanup takes no thread or channel row lock).

8. **Events** (04 2.2, 05 6, 04 14; Ruling `D-M3-18-6`). Both durable, through `kernel/eventlog.Appender` in the same unit of work, with type, action and receive action from the generated catalogue constants:

    | Type | `resource` | `scope` | `audience` | Payload |
    |---|---|---|---|---|
    | `notification_pref.updated` | `{kind: user, id: <person>}` without `version` | empty | the person | `NotificationPrefPayload` |
    | `quiet_hours.updated` | `{kind: user, id: <person>}` without `version` | empty | the person | `QuietHoursPayload` (all rules of the person after the change, ascending `id`) |

    `resource.version` is absent because the resource kind `user` has no version of its own (04 9 point 3); clients compare the version inside the payload, as for `settings.updated` (M1-61). The scope is empty for the same reason as `settings.updated`: the events are personal data of the person, routed by `audience`, not by a place. 03 gives neither table an `updated_by_device_id`, so the payload has none; a device that receives its own change sees a version it already holds and applies nothing (last write wins per row). `actor` and `correlation_id` come from the unit of work as usual.

9. **Quiet hours** (13 3.4; `quiet_hours.update`; Rulings `D-M3-18-7` and `D-M3-18-8`).

    a. Every write (`createQuietHoursRule`, `putQuietHoursRule`, `deleteQuietHoursRule`) first runs `authorize(actor, quiet_hours.update, <own user>)` (agent and app tokens: `403 forbidden` with `reason` `subject_kind_not_allowed`), then takes `SELECT pg_advisory_xact_lock(hashtextextended('notify.quiet_hours:' || <user_id>::text, 0))` before it reads or writes a row, so the writes of one person run one after the other and every `quiet_hours.updated` carries the complete list as of its own commit. Without the lock two parallel creates would each read the list without the other's new row, and the last event would miss one rule. A hash collision only serializes two people; it never changes a result.

    b. Validation (`domain.ValidateRuleInput` plus the zone load in `app`): `weekdays` 1 to 127, `start_local` and `end_local` by `ParseLocalTime` of M3-17 (the schema validator of M1-18 already refuses other forms); `start_local = end_local` is allowed and means a whole day from `start_local` (M3-17 Design note 6, screen D9 "all day"). `time_zone`, when present, must be an IANA name that `time.LoadLocation` accepts, never `Local` and never empty, with `time/tzdata` imported in `app` (the server image has no zone database, M1-31); otherwise `400 validation_failed` with `errors[].code` `unknown_time_zone` and pointer `/time_zone`. Absent `time_zone` stores NULL (the rule follows `users.time_zone`).

    c. Size: after the change the encoded `QuietHoursPayload` must not exceed `sync.event_payload_max_bytes`, measured as the appender of M1-16 measures it; otherwise `400 validation_failed` with `errors[].code` `limit_reached` and an empty pointer, and nothing is stored. Reason: 04 2.2 rule 2 makes the appender refuse a larger payload, and the payload must carry the complete list; this bounds the number of rules by an existing key instead of a new one.

    d. Create: new UUIDv7 `id`, `version = 1`, `201` with `QuietHoursRule`; the middleware of M1-20 handles `Idempotency-Key`. Put: replaces the whole rule (an absent `time_zone` sets NULL); unknown or foreign `rule_id` (the query has the predicate `user_id = <actor>`) gives `404 not_found`; equal values (`domain.RuleUnchanged`) answer `200` without event and without version change; otherwise `version + 1` and `200`. Delete: the row goes, `204`, event; an unknown, foreign or already deleted rule gives `404` (06 7: a resource without tombstone answers `404` after deletion).

    e. List (`settings.read`): all rules of the person, ascending `id`, disabled ones included, `time_zone` omitted when NULL.

10. **`chat` port additions** (Design note 1; additive; doc comments say they never authorize):

    ```go
    // NotificationPath is where a notification preference target sits (13 3.1).
    type NotificationPath struct {
        Kind                     string // space, channel, thread
        SpaceID                  ids.ID // zero for a DM channel and its threads
        ChannelID                ids.ID // zero for a space
        ThreadID                 ids.ID // zero unless Kind is thread
        ChannelKind              string // text, forum, dm; "" for a space
        DefaultNotificationLevel string // channels.default_notification_level; "" when NULL or for a space
        Archived                 bool   // the space, the channel or the channel's space is archived
    }
    NotificationPath(ctx context.Context, kind string, id ids.ID) (NotificationPath, error)     // errs not_found
    LockNotificationPath(ctx context.Context, kind string, id ids.ID) (NotificationPath, error) // FOR SHARE: thread, channel, space
    ChannelIDsOfSpace(ctx context.Context, spaceID ids.ID) ([]ids.ID, error)                   // archived ones included, ascending
    ThreadIDsOfChannels(ctx context.Context, channelIDs []ids.ID) ([]ids.ID, error)            // ascending
    ```

    `NotificationPath` and `LockNotificationPath` with an ID of another kind return `not_found`. They read in the ambient transaction. Where the merged `Threads.Info` (M1-55) or `Threads.InfoForShare` (M1-61) already give part of this, reuse their queries inside `chat`; the port stays one method per need.

11. **`sync` and `identity` additions** (additive; no `authorize`, because `notify` asks for facts of people it already selected; doc comments say so):

    a. `sync.Settings.ValuesFor(ctx context.Context, userIDs []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error)`: stored values only (no defaults, 04 14); one query `WHERE user_id = ANY($1) AND key = ANY($2)`. M3-19 reuses it for the two switches.

    b. `identity.Directory.Users(ctx context.Context, ids []ids.ID) (map[ids.ID]UserView, error)`: unknown IDs and non-users absent, like `Principals`.

12. **`LevelFacts` for M3-19** (13 3.1, 13 12 point 2; Ruling `D-M3-18-9`). `func (s *Facts) LevelFacts(ctx context.Context, threadID ids.ID, userIDs []ids.ID) (map[ids.ID]domain.LevelFacts, error)` in `internal/notify/app/facts.go`: one `chat.Structure.NotificationPath(thread)`; one query `SELECT ... FROM notification_preferences WHERE user_id = ANY($1) AND ((target_kind = 'thread' AND target_id = $2) OR (target_kind = 'channel' AND target_id = $3) OR (target_kind = 'space' AND target_id = $4))` (the space condition left out for a DM channel); one `sync.Settings.ValuesFor(userIDs, ["notify.default_level"])`; and the configuration value. It fills `Thread`, `Channel`, `Space` (nil when absent), `ChannelDefault` (`""` when NULL), `DirectMessage` (`ChannelKind = dm`), `PersonalDefault` (`""` when unset; a stored value outside `all`, `mentions`, `mute` counts as unset and logs one Warn line without the value and without the person's ID) and `InstanceDefault` (`notify.user_default_level`). It never resolves the level itself: `ResolveLevel` of M3-17 does, at the time the caller passes. Unknown user IDs get the facts without own rows; an unknown thread gives `not_found`. It is an `app` function of `notify`, not a port method, because only `notify` (M3-19) calls it.

13. **`QuietRules` for M3-19** (13 3.4). `func (s *Facts) QuietRules(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]domain.QuietRule, error)`: all rules of the people (enabled or not, `InQuietHours` ignores disabled ones), with `Location` loaded from `quiet_hours.time_zone` or, when NULL, from `users.time_zone` through `identity.Directory.Users`. Each distinct zone name is loaded once per call. A name that no longer loads (a zone removed from the database) falls back to `time.UTC` and logs one Warn line without the zone name and without any ID. Reason: the rule stays effective instead of disappearing, and UTC is the documented default of `users.time_zone` (03).

14. **Configuration.** `notify.user_default_level` (chapter 13, default `mentions`, allowed `all`, `mentions`, `mute`) in the configuration schema if absent (02 13 item 5); `TestConfigSchemaBuilds` lists it; `make config-reference` and `make config-doc-check` show no diff. `sync.event_payload_max_bytes` is read through the existing typed `sync` configuration. No new key.

15. **Rights, limits, SQL lint.** Rate-limit classes come from `openapi.yaml` (`rl.read`, `rl.write`) and are enforced by the chain of M1-19 where it is merged; do not reimplement them. Every query on the two tables carries `user_id = <actor>` or the explicit list of people (Design notes 7, 12, 13). Where the SQL visibility lint of M1-70 (`tools/lint/sql_visibility.py`, part of `make check`) is merged and flags a new reading query, add one exception entry in `tools/lint/sql_visibility.yaml` with its category and reason: `authorize_resolution` for the path reads of `NotificationPath` and `LockNotificationPath` (they resolve the target for `authorize`; `LevelFacts` reuses the same query), `after_authorize` for `ChannelIDsOfSpace` and `ThreadIDsOfChannels` (they run inside an archive that `channel.archive` or `space.archive` allowed); the two `notify` tables hold no content and carry `user_id` predicates. Never widen an existing entry. Logs carry no person data: no level, no time zone, no time of day, and IDs only with the attributes of `docs/observability/signals.md`.

16. **Specification and API** (rule 10; nothing in 00, 03, 04 or 05 changes; all new text in English, existing text stays as it is):

    - `docs/spec/13-push-notifications.md` 3.1, new paragraph after the order: preferences are set on spaces, channels and threads the person can read (the main thread included); an archived target takes no new preference (`409 conflict` with `reason` `target_archived`), deleting stays possible; `muted_until` is stored as given and has no effect once passed; an unchanged write writes no event; deleting writes `notification_pref.updated` with `deleted: true`, and a later write starts at version 1; archiving a space or channel removes the preferences of it and of everything below it in the same transaction, with one event per removed row; lists name only targets the person can still read.
    - 13 3.4, new paragraph after point 4: `time_zone` must be an IANA zone name (`400 validation_failed` with `errors[].code` `unknown_time_zone`), absent means `users.time_zone`; a put replaces the whole rule; every write sends the complete list in `quiet_hours.updated`, and a write that would make that list larger than `sync.event_payload_max_bytes` is refused with `400 validation_failed` and `errors[].code` `limit_reached`; a zone that no longer loads counts as UTC.
    - 13 12 point 2, one sentence: `notify` gathers the facts for all candidates of a thread in one query and resolves the level with the pure function of point 3.
    - `docs/api/openapi.yaml`: add `'409': $ref: '#/components/responses/Conflict'` to `putNotificationPreference`; descriptions (English) on `putNotificationPreference` (readable target, archived target `409 conflict` with `reason` `target_archived`, `muted_until` semantics), `deleteNotificationPreference` (`204` also without a stored row), `listMyNotificationPreferences` (only readable targets), `createQuietHoursRule` and `putQuietHoursRule` (`unknown_time_zone`, `limit_reached`, whole-rule replace), `QuietHoursRule.time_zone` and `QuietHoursRuleInput.time_zone` (IANA name; absent: the person's time zone), `QuietHoursPayload.rules` (ascending `id`). Then `make api-lint`, `make api-checks` and `make codegen-check`.

17. **Rulings for `docs/DECISIONS.md`** (section 10, one row each with date, decision, reason and place): `D-M3-18-1` the path lock order thread, channel, space with `FOR SHARE` and every thread as a target (note 4 a, b); `D-M3-18-2` no new preference on an archived target, `409 conflict` with `reason` `target_archived`, delete allowed (note 4 c); `D-M3-18-3` `muted_until` stored as given in microseconds, the single-statement upsert with no-op detection (note 4 d, e); `D-M3-18-4` delete event with last version plus one and restart at 1 (note 5 a); `D-M3-18-5` cleanup on archive with one event per row in key order (note 7); `D-M3-18-6` event resource `user` without version, empty scope, no device field (note 8); `D-M3-18-7` the per-person advisory lock and the complete list in ascending `id` (note 9 a, e); `D-M3-18-8` zone validation with `unknown_time_zone`, the list bound by `sync.event_payload_max_bytes` with `limit_reached`, the UTC fallback (notes 9 b, 9 c, 13); `D-M3-18-9` `LevelFacts` and `QuietRules` as `app` functions with one query each and the port additions of `chat`, `sync` and `identity` (notes 10 to 13). Rulings 2, 3, 5, 7 and 8 also go into 13 (note 16).

## Tests (write them first)

Test world: 07 10 and 11.1 (space Support with channels Allgemein and Eskalation; G-1 Alex `trigger` on Support; group Leser with Sam, G-4 `read` on Support/Allgemein; Robin without a grant on Support; the Owner). Threads are created with `createThread` (M1-56) by Alex; T is a thread in Support/Allgemein, M the main thread of Allgemein. Requests go through the real HTTP chain with DPoP proofs from the identity test helpers of M1-33 and M1-34; durations and `notify.user_default_level` are fixture values of the test configuration, not the defaults.

Unit, `internal/notify/domain` (M3-17's `TestDomainPurity` must still pass over the new files):

- `TestTargetKind`: `space`, `channel`, `thread` valid; `dm`, `user`, `""` invalid.
- `TestPreferenceUnchanged`: equal level and `muted_until` (both zero, both equal after microsecond truncation) unchanged; other level, set versus zero, other instant changed.
- `TestValidateRuleInput`: weekdays 0 and 128 rejected, 1 and 127 accepted; `07:00` to `07:00` accepted; `24:00` and `7:00` rejected through `ParseLocalTime`; `Local` and the empty zone rejected; `RuleUnchanged` for equal and for each single differing field.

Unit, `internal/notify/app` with fakes for the `chat`, `sync` and `identity` ports, `kernel/authorize`, the appender and the repositories:

- `TestLevelFacts_OneQueryForAllCandidates` (13 12 point 2): `LevelFacts` for 50 user IDs calls the preference repository once, `NotificationPath` once and `ValuesFor` once; a DM path leaves the space condition out; a stored personal default `"loud"` counts as unset.
- `TestQuietRules_ZoneSources`: a rule with zone uses it; a rule without zone uses the person's `TimeZone`; an unloadable stored name gives `time.UTC` and one Warn log line without the name; each zone name is loaded once per call.
- `TestOnChannelArchived_DeletesAndEmitsPerRow`, `TestOnSpaceArchived_CollectsChannelsAndThreads`: the targets passed to the repository; one event per returned row in ascending key order, `deleted: true`, audience the row's person.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`):

- `TestMigrationNotifyTables`: columns, types, defaults, PK of both tables, both FKs `ON DELETE CASCADE`, the checks on `target_kind`, `level` and `weekdays`, no FK on `target_id`, index `(target_kind, target_id)`, partial index `(user_id) WHERE enabled`; a second `migrate up` changes nothing; deleting a `users` row removes the person's rows.
- `TestEffectiveLevelOrder` (PLAN row, 13 3.1), through `LevelFacts` and `domain.ResolveLevel` at the fake clock's now: Alex sets `mute` on T, `all` on Allgemein, `mentions` on Support (`putNotificationPreference`); the Owner sets `default_notification_level = mute` on Allgemein (`updateChannel` of M1-53); Alex sets `notify.default_level = all` (`putMySetting` of M1-61); the instance default is `mentions`. Expected source and level after each removal: thread `mute`; after `deleteNotificationPreference` on T: channel `all`; on Allgemein: space `mentions`; on Support: channel default `mute`; after the Owner clears it (`clear_default_notification_level: true`): personal default `all`; after `deleteMySetting`: instance default `mentions`. DM step: a fixture channel of kind `dm` (no space) with a thread, written through the `pgtest` connection, gives `all` with source `direct_message`, and a channel default on it wins (step 4 before 5). One call for Alex, Sam and Robin returns three facts in one query, each with its own sources.
- `TestEffectiveLevel_MutedUntil`: Alex puts `all` with `muted_until` one hour after now on T: `mute`, `Muted`; the clock passes it: `all`; `muted_until` on the space row while T has a row: no effect.
- `TestPutNotificationPreference_Lifecycle`: first put `200`, `version 1`, no `muted_until` member; the `event_log` row has type `notification_pref.updated`, `audience_principal_id` Alex, resource `{user, Alex}` without version, empty scope, a payload valid against `NotificationPrefPayload`, no JSON `null`; the same body again: `200`, version 1, no new event; another level: version 2, one event; `muted_until` with nanoseconds: stored and returned in microseconds, an equal value afterwards is a no-op.
- `TestDeleteNotificationPreference`: delete of a stored row `204` and one event with `deleted: true` and `version = last + 1`; a second delete `204` without event; a later put starts at version 1.
- `TestNotificationPreference_Concurrent`: two parallel identical first puts: both `200`, one row with version 1, exactly one event; two parallel puts with different levels: one row with version 2, two events, the stored level equals the payload of the event with the higher `seq`.
- `TestNotificationPreference_TargetChecks` (AZ-P4): Robin's put and delete on T, on Allgemein and on Support give `404 not_found`, identical in status, body and headers to an unknown ID; a channel ID sent as `target_kind = thread` gives the same `404`; Alex can set a level on M; an agent token gets `403 forbidden` with `reason` `subject_kind_not_allowed` on each of the seven operations (the human-only rule of M1-61).
- `TestListMyNotificationPreferences_ReadableOnly` (04 14): Sam has rows on T and Allgemein; after the Owner revokes G-4 both are missing from Sam's list and still stored; after a new grant they appear again; the list is ordered by kind and ID.
- `TestArchiveCleanup_Channel` (PLAN row "cleanup on archiving"): Alex has rows on Allgemein, T, Support and Eskalation, Sam on Allgemein and T; the Owner archives Allgemein (`archiveChannel` with step-up): in the archive transaction the rows on Allgemein and T are gone, the rows on Support and Eskalation stay; one `notification_pref.updated` with `deleted: true` per removed row, audience its person, in key order, with the Owner as actor; a test subscriber of `chat.ChannelArchived` registered after `notify` that vetoes once leaves archive and rows unchanged.
- `TestArchiveCleanup_Space` (PLAN row): the Owner archives Support (`archiveSpace`): every row on Support, Allgemein, Eskalation and their threads is gone, with one event each; rows of another space stay.
- `TestArchivedTarget_PutConflictDeleteAllowed`: after the archive, Alex's put on Allgemein, on T and (after `archiveSpace`) on Support gives `409 conflict` with `reason` `target_archived`; delete answers `204`; reading the list still works.
- `TestArchive_RaceWithPut`: in 20 rounds a put on T runs in parallel with `archiveChannel` of Allgemein; after each round no row targets Allgemein or T, and every put answered either `200` (its row then removed with an event) or `409`.
- `TestQuietHours_Lifecycle`: create (`Idempotency-Key`) `201`, `version 1`, `time_zone` `Europe/Berlin`; the event `quiet_hours.updated` has resource `{user, Alex}` without version, empty scope, payload valid against `QuietHoursPayload` with the complete list in ascending `id`; a replay with the same key answers the stored `201` with `Idempotent-Replay: true` and adds no row; put with equal values `200` without event; put without `time_zone` clears it (response without the member) and raises the version; delete `204` with the list without the rule; a second delete `404`; Sam's put and delete of Alex's rule `404`, identical to an unknown ID.
- `TestQuietHours_Validation`: `time_zone` `Mars/Olympus`, `Local` and `""` give `400 validation_failed` with `errors[].code` `unknown_time_zone` and pointer `/time_zone`; `weekdays` 0 and `start_local` `24:00` give `400 validation_failed` from the schema; `00:00` to `00:00` is stored.
- `TestQuietHours_ListBoundByPayloadSize`: with a small `sync.event_payload_max_bytes` in the test configuration, creates succeed until the next list would exceed it; that create gives `400 validation_failed` with `errors[].code` `limit_reached` and stores nothing; a put that keeps the size passes.
- `TestQuietHours_ConcurrentCreates`: 10 parallel creates by Alex: 10 rows, 10 events, and the event with the highest `seq` lists all 10 rules.
- `TestQuietRules_UserTimeZone` (13 3.4 against PostgreSQL): Alex sets `time_zone` `Europe/Berlin` with `updateMyProfile` (M1-31) and a rule Friday (`16`) `22:00` to `07:00` without zone; `QuietRules` plus `InQuietHours` is true at `2026-10-23T21:30:00Z` and false at `2026-10-23T19:30:00Z`; after a put that sets `time_zone = UTC` on the rule the result at `2026-10-23T21:30:00Z` is false and at `2026-10-23T22:30:00Z` true.
- `TestPersonalDataNotRetrievable` (pattern of DP-T10): Sam sees none of Alex's preferences or rules through any of the seven operations; every event of this ticket carries `audience_principal_id` of its person (the per-recipient filter itself is M1-63's test).
- `internal/wire`: the golden registrations show `chat.ChannelArchived -> notify.OnChannelArchived` and `chat.SpaceArchived -> notify.OnSpaceArchived`; `TestConfigSchemaBuilds` lists `notify.user_default_level`; the route count test (AZ-P1) passes with the seven new routes.
- If the response validator of G8 exists, all integration tests run with it.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/notify/... ./internal/chat/... ./internal/sync/... ./internal/identity/... ./internal/wire/... ./migrations/...`
- `make migrations-lock` (once after creating the migrations), `make migrations-lint`, `make codegen-check`, `make arch`, `make register-check`, `make api-lint`, `make api-checks`, `make config-reference` (no diff afterwards), `make config-doc-check`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] The migrations create `notification_preferences` and `quiet_hours` exactly as 03; `make migrations-lint` is green and a second `migrate up` changes nothing.
- [ ] The seven operations are enabled in `api/operations.yaml` and registered; AZ-P1 passes; agent and app tokens get `403 forbidden` with `reason` `subject_kind_not_allowed`.
- [ ] The effective level follows the order of 13 "Effective level" against PostgreSQL through `LevelFacts` and `ResolveLevel`: thread, channel, space, channel default, DM, personal default, instance default, with `muted_until` on the chosen row; one preference query serves all candidates of a thread.
- [ ] Every real change writes exactly one `notification_pref.updated` or `quiet_hours.updated` with `audience = self`, resource `user` without version and a payload valid against its schema; a no-op writes none; `deleted: true` on removal; the quiet-hour event carries the complete list, also under concurrent writes.
- [ ] Archiving a channel or a space removes, in the archive transaction, every preference of the target and of everything below it with one event per row; a vetoed archive keeps them; an archived target refuses new preferences with `409 conflict` and `reason` `target_archived`.
- [ ] Unknown and foreign targets and rules give identical `404`; invalid zones give `400 validation_failed` with `errors[].code` `unknown_time_zone`; a list above `sync.event_payload_max_bytes` gives `400 validation_failed` with `errors[].code` `limit_reached`; lists name only readable targets.
- [ ] 13 3.1, 3.4 and 12, the openapi descriptions and the `409` response, the READMEs of `notify`, `chat`, `sync` and `identity`, the CHANGELOG and the rows `D-M3-18-1` to `D-M3-18-9` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `notify` updated in English (status, ports `Preferences` and `QuietHours`, the two tables with retention class `personal_sync`, actions, events, the two subscriptions, `upsertPreference`, `LevelFacts`, `QuietRules`, the rulings of Design note 17), keeping the line `May use the ports of these modules: ...` and the section `## Tables` as they are for M1-06; READMEs of `chat`, `sync` and `identity` name their port additions.
- Rulings of Design note 17 recorded in `docs/DECISIONS.md` (one row each with reason); rulings 2, 3, 5, 7 and 8 also in `docs/spec/13-push-notifications.md`.
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
