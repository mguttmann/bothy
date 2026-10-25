# Module `notify`

Status: skeleton (M1-05), the decision per recipient as pure functions in `domain/` (M3-17), the notification levels and quiet hours with their tables, use cases, REST operations, archive cleanup and the reads of the push path (M3-18), and the push path (M3-19): the table `push_registrations` with its read port, the worker consumer `push`, the bundling window, the job `notify.push_send` with the provider port, automatic following, the badge and the badge sync, the APNs adapter `adapters/apns` (M3-20), the FCM adapter `adapters/fcm` with Fake-FCM (M3-21), the desktop hints over the gateway (M3-23), the shared presence of several nodes (M7-17), and the push registration, the preview endpoint and the deactivation of registrations (M3-22). The delivery measurement (M3-24) follows.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Notification levels, quiet hours, push registrations, push consumer and dispatch.

## Code

- `internal/notify` (`doc.go`, `preferences.go`): the port as root package: `Preferences` and `QuietHours` with their DTOs `Target`, `PutPreference`, `Preference`, `QuietHoursInput`, `QuietHoursRule` (M3-18).
- `internal/notify/domain` (M3-17): the decision per recipient of 13 section 4 as side-effect-free functions: `ResolveLevel` (`level.go`), `InQuietHours` and `ParseLocalTime` (`quiet.go`), `Decide` (`decide.go`), `Flush` (`flush.go`), `ReleaseDelayed` (`delay.go`), the enums in `kinds.go`. It imports only the standard library without I/O and `kernel-core`; `TestDomainPurity` and `make arch` hold that. The golden table is `domain/testdata/decision_table.golden`.
- `internal/notify/domain` (M3-18): `TargetKind` and `PreferenceUnchanged` (`target.go`), `RuleInput`, `ValidateRuleInput` and `RuleUnchanged` (`rule.go`).
- `internal/notify/app` (M3-18): the use cases (`preferences.go`, `quiethours.go`), the two event builders (`events.go`), the in-transaction subscribers (`subscribers.go`) and the reads of the push path (`facts.go`).
- `internal/notify/adapters/postgres` (M3-18): the repository over the sqlc queries in `queries/`; `adapters/httpapi`: the seven REST operations of the group Preferences.
- `internal/notify` (M3-19, `registrations.go`): the port `PushRegistrations` with the DTO `PushRegistration` (no token).
- `internal/notify/domain` (M3-19): `KindCode` and `KindFromCode` (`codes.go`), the result classes with `ApplyResult` and `RetryDelay` (`result.go`), `RunActive` (`runstate.go`).
- `internal/notify/app` (M3-19): the push path `Push` (`consumer.go`) with the consumer `PushConsumer`, the event sources (`sources.go`), the run reads (`runs.go`), the facts per batch (`gather.go`), the window (`window.go`), the flush (`flush.go`), the send job (`send.go`), the badge and the badge sync (`badge.go`), the outgoing ports (`ports.go`) and the automatic following (`follow.go`).
- `internal/notify/adapters/postgres` (M3-19, `registrations.go`): `PushRegistrations.Active` and the outgoing port `RegistrationStore` over `push_registrations`; `adapters/jobs`: the five job kinds, the `Scheduler` over `platform/jobs` and the span wrapper `Traced`; `adapters/otelmetrics`: the push instruments; `adapters/fakepush`: `FakeProvider`, the test double of `PushProvider` (only tests use it).
- `internal/notify/adapters/fcm` (M3-21): the provider `fcm` of `PushProvider` (`Adapter`, `New`, `Deps`, `Endpoints`, `SecretResolver`, `Config`, `ServiceAccount`, `ParseServiceAccount`), with the access token cache and pause, the message builder and the answer table; `adapters/fcm/fcmtest`: Fake-FCM `Server`, the test double of both Google endpoints (only tests use it; `TestFakeNotInBinary` keeps it out of `cmd/bothy`).
- `internal/notify` (M3-22, `registrations.go`): `PushRegistrations` gains `Register`, `Unregister` and `Preview` with `RegisterCmd`, `RegisteredPush`, `PreviewRequest`, `PreviewView` and `Secret`, the redacting type of the preview token (it renders as `[redacted]` through fmt, slog and JSON; only `Reveal` hands out the value).
- `internal/notify/domain` (M3-22): `ValidateRegistration`, `RegistrationInput`, `RegistrationPolicy`, `DisableReasonForSession`, the field codes and the `disabled_reason` values (`registration.go`); `PreviewMode`, `ParsePreviewMode`, `PreviewKindAllowed` (`preview.go`); `Excerpt` (`excerpt.go`). `TestDomainPurity` holds these files too.
- `internal/notify/app` (M3-22): `Registrations`, the implementation of `PushRegistrations` (`registrations.go`, `preview.go`), `PreviewBuilder` (`previewbuilder.go`), the subscribers `OnSessionEnded` and `OnDeviceRevoked` (`deactivate.go`), `PushTargets` (`pushtargets.go`) and the outgoing port `RegistrationWriter` (`ports.go`).
- `internal/notify/adapters` (M3-22): `postgres/registrations_write.go` over `queries/registrations_write.sql` (`RegistrationWriter`); `httpapi/push.go` (`registerPush`, `unregisterPush`, `getPushPreview`); `otelmetrics.Fetch` (`bothy.push.fetch`).
- `app/` and `adapters/<kind>/`: `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Decision per recipient

`domain.Decide` takes the facts about one notification for one person and returns one outcome: `none` with a skip reason, `collect` into the open coalescing window, or `send`, quiet or normal, with one route per device. It reads no database, calls no `authorize`, reads no clock, loads no time zone and reads no configuration; the caller passes everything in (docs/DECISIONS.md D-M3-17-1). M3-18, M3-19 and M3-23 call these functions instead of re-implementing them.

### Decision table (13 4.1)

Evaluated in this order; the first row that decides wins.

| Step | Condition | Outcome | `SkipReason` |
|---|---|---|---|
| validate | input invalid (D-M3-17-7) | error, zero `Decision` | |
| allowed | `Allowed` is false | `none` | `not_allowed` |
| recipient | `message`, `mention` or `dm` and `Author`; `run_done`, `run_failed` or `input_needed` and not `RunStarter` | `none` | `not_recipient` |
| level | kind with thread and the effective level does not allow it | `none` | `level` |
| level | `admin` and not `Switches.AdminEnabled` | `none` | `admin_disabled` |
| activity | kind with thread and a device is active in the thread | `none` | `active_in_thread` |
| quiet | quiet hours hold and not (`input_needed` and `InputNeededBreaksQuietHours`) | sets `Quiet` for a send | |
| window | `message` and the window is open | `collect` | |
| route | no device has a route | `none` | `no_route` |
| send | otherwise | `send` | |

| Effective level | Kinds with thread it allows |
|---|---|
| `all` | `message`, `mention`, `dm`, `run_done`, `run_failed`, `input_needed` |
| `mentions` | the same except `message` |
| `mute` | only `input_needed` |

`security` passes the level step at every level, `admin` only by `notify.admin_enabled`; both have no thread, so no level, no activity in a thread and no window apply to them, quiet hours do (D-M3-17-2). The skip reasons are internal constants; M3-19 maps them to the attribute values of `bothy.push.skipped`. No skip reason stands for quiet hours: they make a notification quiet, never drop it (R-PN-05).

### Inputs and where they come from

| Field of `RecipientFacts` | Source |
|---|---|
| `Kind`, `ThreadID`, `Author`, `RunStarter` | the event the consumer `push` reads (M3-19, 13 2) |
| `Allowed` | `authorize(r, push.deliver, thread)` in M3-19 with current rights (13 4.2); false for a principal that is not an active human; for `security` and `admin` the recipient rule of 13 2 as M3-19 evaluates it |
| `Level.Thread`, `Level.Channel`, `Level.Space` | own rows of `notification_preferences` (M3-18) |
| `Level.ChannelDefault`, `Level.DirectMessage` | `chat` (`channels.default_notification_level`, `channels.kind = 'dm'`) |
| `Level.PersonalDefault`, `Switches` | `user_settings` through `sync`, the switches resolved with `notify.default_admin_enabled` and `notify.default_input_needed_breaks_quiet_hours` |
| `Level.InstanceDefault` | configuration key `notify.user_default_level` |
| `QuietHours` | `quiet_hours` (M3-18), each with its `*time.Location` loaded by the caller from `quiet_hours.time_zone` or, when NULL, `users.time_zone` from `identity` |
| `Devices` | active devices from `identity`, active `push_registrations` (M3-22), connection, focus and `idle` from the presence port of the gateway (M3-23) |
| `Window` | window state per (recipient, thread) of M3-19 |
| `Policy` | configuration keys `notify.coalesce_window` and `notify.mobile_delay_when_desktop_active` |
| `Now` | the caller's clock |

### Window rules (13 4.5, D-M3-17-5)

- The window of (recipient, thread) is open while `Now` is before `Window.End`; from `Window.End` on it is closed.
- A kind with thread that sends while no window is open sets `OpensWindowUntil = Now + notify.coalesce_window`, whatever its kind. A length of zero opens no new window, so every notification goes at once, except a `message` while a window opened earlier is still open.
- `message` in an open window gives `collect` without routes. `mention`, `dm`, `input_needed`, `run_done` and `run_failed` in an open window send at once and leave the window unchanged.
- `Flush` at the window end: `Collected` zero gives `nothing_collected`, `Unread` (03 `read_states`, "Unread") zero or less gives `read`; otherwise allowed, level, activity, quiet and route run again at the flush time, `Author` and `Window` are ignored, and a send carries `Count = Unread` and opens the next window. A flush that sends nothing leaves the window closed.

### Route rules (13 4.4, 4.6, D-M3-17-6)

- Active in the thread: a device with a connection that is not idle and whose focus contains the thread, on any platform. It stops the notification on every device.
- Desktop active: a `macos` or `windows` device with a connection that is not idle.
- One route per device, in the order of `Devices`: a connection (idle or not) gets `gateway` (`notification.created`); else a device with registrations gets `push_delayed` with `NotBefore = Now + notify.mobile_delay_when_desktop_active` if it is `ios` or `android`, the desktop is active and the delay is above zero, otherwise `push`; a device with neither gets nothing.
- No route on any device gives `none` with `no_route` and opens no window.
- `ReleaseDelayed(hintSeq, readSeq)` drops a delayed push exactly when `readSeq >= hintSeq`; a notification without a message (`hintSeq` zero or less) is always released.
- A delayed push keeps the `Quiet` flag of its decision (D-M3-17-4).

### Rulings

D-M3-17-1 to D-M3-17-8 in [docs/DECISIONS.md](../../../../docs/DECISIONS.md), section "M3-17 Notification decision".

## Notification levels and quiet hours (M3-18)

- Tables: `notification_preferences` (PK `(user_id, target_kind, target_id)`, no foreign key on the target, R-DM-16) and `quiet_hours` (weekday bit mask, local start and end, `time_zone` NULL for `users.time_zone`), both `ON DELETE CASCADE` from `users`, retention class `personal_sync`.
- Port `Preferences` (`listMyNotificationPreferences`, `putNotificationPreference`, `deleteNotificationPreference`) and port `QuietHours` (`listMyQuietHours`, `createQuietHoursRule`, `putQuietHoursRule`, `deleteQuietHoursRule`); actions `settings.read`, `notification_pref.update` ("self and read" on the target), `quiet_hours.update` ("self"); agents and apps get `403` with `subject_kind_not_allowed`.
- Events: every change writes one `notification_pref.updated` (`NotificationPrefPayload`, `deleted: true` on removal) or `quiet_hours.updated` (`QuietHoursPayload`, the complete list ascending by `id`), resource `user` without version, empty scope, audience the person; a no-op writes none.
- Subscriptions: `chat.ChannelArchived -> OnChannelArchived`, `chat.SpaceArchived -> OnSpaceArchived` remove the preferences of the archived target and everything below it in the archive's transaction, one event per row.
- `upsertPreference` (exported as `Service.UpsertPreference`) is one of the two write paths of `notification_preferences`; it neither authorizes nor locks. The other is `insertPreferenceIfAbsent` of the automatic following (M3-19, below), which never overwrites an own row.
- `Facts.LevelFacts(ctx, threadID, userIDs)`: the facts of 13 3.1 for many people in one preference query; the caller resolves with `domain.ResolveLevel`. `Facts.QuietRules(ctx, userIDs)`: the rules with their zones loaded (rule zone, else `users.time_zone`, a zone that no longer loads counts as UTC). Both are for M3-19 and do not authorize.
- Configuration: `notify.user_default_level`; the list of rules is bounded by `sync.event_payload_max_bytes`.
- Rulings: D-M3-18-1 to D-M3-18-11 in [docs/DECISIONS.md](../../../../docs/DECISIONS.md), section "M3-18 Notification preferences and quiet hours".

## Push path (M3-19)

### Table `push_registrations`

Migration `00059_notify_push_registrations.sql` creates the table exactly as 03 (provider, token, environment, app ID, preview token hash, delivery columns, the check that `disabled_at` and `disabled_reason` are set together, cascades from `users`, `devices` and `auth_sessions`, unique `(provider, token)`, the partial unique index on `preview_token_hash` and the indexes `(user_id) WHERE disabled_at IS NULL` and `(auth_session_id)`), retention class `security`. This ticket writes no rows except the delivery columns of a send; `registerPush` and `unregisterPush` write the rows since M3-22 (see "Registration and preview (M3-22)"). The token and `preview_token_hash` never leave the adapters of `notify`.

### Ports

- Incoming `PushRegistrations.Active(ctx, userIDs)`: the registrations with `disabled_at IS NULL` of the given people, grouped by person, ascending by `id`, without token. It does not authorize: the callers select the people. M3-22 added `Register`, `Unregister` and `Preview` on the same port.
- Outgoing ports of `notify/app`: `PushProvider` (`Name`, `Send(ctx, Registration, Notification)`; implemented by `adapters/apns` of M3-20, `adapters/fcm` of M3-21 and `adapters/fakepush`), `LiveRoutes` (the routes `gateway` and `push_delayed`; the default `FailClosedRoutes` returns an internal error, so the round rolls back), `Scheduler` (the jobs of `notify` over `platform/jobs`), `RegistrationStore` (loads a registration with its token, locks and writes its delivery columns, counts the active rows per provider) and `Metrics` (the push signals).
- `Notification` holds only IDs, the kind, the quiet flag, the count and the badge (`Badge`, `HasBadge`); the badge sync has the kind `KindSync` (`sync`) and carries `Cleared`, the complete list of threads read since the last sync. `InstanceID` is the instance record's ID, passed in by `internal/wire`.
- Reads of other modules, none of which authorizes: `authz.Recipients.Readers`, `sync.History` (`ResourceEvents`, `PersonalEvents`), `sync.ReadPositions` (`PositionsFor`, `PositionsOfUser`), `messaging.Messages` (`LastAnchorBefore`, `AnchorOf`, `LastAgentMessageBefore`, `Newest`), `messaging.Mentions.CountUnread`, `chat.Threads` (`Info`, `UnreadCandidates`), `identity.Directory` and `identity.Devices.ActiveForUsers`, and the kernel port `presence.View` (until M3-23 `NewPush` falls back to `presence.None`, because `internal/wire` passes no view).

### Consumer `push`

`PushConsumer` is the `sync.ConsumerHandler` of the worker consumer `push` (M2-01, ADR 0051). It inserts the jobs of each event in the round that moves its cursor, so a restart in the middle of a round neither doubles nor loses a hint, and it ignores every other event type. Sources and candidates:

| Event | Hint | Candidates |
|---|---|---|
| `message.created` | `dm` in a DM channel; else `mention`, and `message` unless the message is an agent message during an active run (13 2.1) | in a DM channel its readers from `authz.Recipients.Readers` (participants and grant holders, no owners); else the mentioned people; the readers of the channel from `authz.Recipients.Readers`; never the author, one hint per person and event, precedence `dm`, `mention`, `message`; nothing for a redacted event, `silent`, `origin` `import` or `system`, and the kinds `run` and `system` |
| `run.state_changed` | `input_needed` at `waiting_input`, `run_done` at `completed`, `run_failed` at `failed` to the starter; at `completed`, `failed` and `cancelled` one `message` to the other readers (the starter included at `cancelled`), in a DM channel one `dm` instead, all pointing at the last agent message of the run | `runs.triggered_by` and the readers of the channel; a run without agent message gives no `message` hint |
| `access.expiring` | `admin` with the grant or group as target | the audience of the event |
| `device.pending` | `security` with the device as target | the audience of the event, routed only to its other active devices |
| `read_state.updated` | none; the trigger of the badge sync | |

Only active humans are recipients. The facts of all candidates of one event are read with one call per port; every candidate with thread is decided with `authorize(r, push.deliver, thread)` in filter mode with the principal of a check without a request (R-AZ-29), `security` and `admin` with the recipient rule of R-AZ-30. `domain.Decide` then gives none, collect or send; a send inserts one `notify.push_send` per active registration of each push route, all with one `NotificationID` (the `nid`).

### Jobs

All five run on the queue `delivery` (02 12.2, 12.3):

- `notify.push_send`: arguments `PushSend` (IDs, the kind as `KindCode` 1 to 9, quiet, count, t0 as `DueAtUnixMicro`, attempt). Step 1 in a transaction: registration, age, rights and provider are checked and the badge is counted; step 2 outside any transaction under `push.max_concurrent_requests`: `PushProvider.Send` inside the span `push.send`; step 3 in a transaction: `domain.ApplyResult` on the locked row and, for a retry, the same job with the next attempt at `domain.RetryDelay`. At least once with one `nid`.
- `notify.coalesce_window`: the marker of an open bundling window of (recipient, thread), scheduled at its end; unique per recipient, thread and end, the open window is the latest pending marker; its handler does nothing.
- `notify.coalesce_flush`: exists exactly when a `message` hint was collected in the window; arguments recipient, thread and the window end, scheduled at the end; it sends one hint with the unread count, pointing at the newest message, or nothing.
- `notify.badge_sync`: unique per person and time (`UserID` and `at` tagged `river:"unique"`), one waiting per person (the consumer reads the latest pending one; a due one may already run, so a later falling read gets a follow-up after the interval); it waits for a running hold of an earlier sync, then counts the badge, collects `Cleared`, inserts the hold and one `notify.push_send` of kind `sync` per active `apns` or `fcm` registration of an active `ios` or `android` device.
- `notify.badge_sync_hold`: scheduled at now plus `notify.badge_sync_min_interval` after a sync, unique per person and end; a falling read during the hold schedules the next sync at its end; its handler does nothing.

### Automatic following

`Subscribers.OnMessagePosted`, an in-transaction subscriber of `messaging.MessagePosted` registered after those of `sync` and `runs`: a message of `origin` `human` in a thread of kind `thread` (not the main thread) inserts the author's row with level `all` for that thread through `insertPreferenceIfAbsent`, unless the person has an own row; only an inserted row appends one `notification_pref.updated`. A veto later in the unit of work removes row and event. `createThread` is covered, because it publishes `MessagePosted` for its first message.

### Badge

`BadgeCounter.For(ctx, userID)` counts the threads the person can read that are unread and hold an unread mention of the person, have the effective level `all`, or lie in a DM channel (13 8); every counted thread is confirmed with `push.deliver`. `notify.push_send` sets it on every `Notification`; M3-22 calls it for the preview. A `read_state.updated` that lowers the number schedules `notify.badge_sync`, at most one per person per `notify.badge_sync_min_interval`.

### Skip reasons

`bothy.push.skipped` counts with `bothy.reason`: `no_access` (`not_allowed`, or denied at send), `preference` (`level`, `admin_disabled`), `active_in_thread`, `no_registration` (`no_route`, or the registration gone, deactivated or on an inactive device at send), `read` (a flush with nothing unread), `expired` (older than `push.max_age`, or a retry past it) and `no_provider` (no provider registered for the registration). `not_recipient` and `nothing_collected` are not counted. Logs carry only `bothy.push.provider`, `bothy.job.kind`, `bothy.reason` and `error.type`, never a token, a person, a thread or a text.

### What later tickets plug in

- M3-20 (`adapters/apns`, see "APNs adapter") and M3-21 (`adapters/fcm`) implement `PushProvider` and map their answers onto the result classes; until a provider is registered, every `notify.push_send` ends with `no_provider`.
- M3-22 adds the writes of `PushRegistrations` (`registerPush`, `unregisterPush`), the preview token and endpoint (with `BadgeCounter.For`) and the deactivation on logout, device revocation and user disable (built, see "Registration and preview (M3-22)").
- M3-23 replaces `presence.None` with the gateway's `presence.View` and `FailClosedRoutes` with `notification.created` and the delayed push (built, see "Desktop hints over the gateway").
- M3-24 adds the delivery measurement of 13 9.

### Rulings

D-M3-19-1 to D-M3-19-22 in [docs/decisions.d/M3-19.md](../../../../docs/decisions.d/M3-19.md) ("M3-19 Hint jobs, automatic following and bundling").

## APNs adapter (M3-20)

`adapters/apns` implements `PushProvider` for registrations with `provider = apns` (13 5.1, 6.1, 7.1). `internal/wire` (`startAPNs` in `module_notify.go`) builds it only when APNs is enabled and only in the roles `worker` and `all`; the role `api` and an instance without APNs build none, read no secret, and their `apns` jobs end with `no_provider`.

### Configuration

APNs is enabled exactly when `push.apns.team_id`, `push.apns.key_id` and `push.apns.topics` are all set; then `push.apns.private_key_ref` is required. The cross-check `push_apns_complete` refuses a partial set, an empty or repeated topic and an enabled APNs without key reference; the loader refuses `push.apns.jwt_refresh_interval` outside 20 to 60 minutes in every role. At start `apns.New` resolves the key once and parses it (one PEM block `PRIVATE KEY`, PKCS #8, ECDSA on P-256); an error stops the start and names `push.apns.private_key_ref`. `bothy config check --secrets` resolves the key only with APNs enabled. Further keys: `push.ttl` (`apns-expiration`), `push.apns.retry_5xx_delay`, `push.apns.time_sensitive_enabled`.

### Endpoints and connection

`https://api.push.apple.com` for `environment = production`, `https://api.sandbox.push.apple.com` for `sandbox`, request `POST /3/device/<token>`. One egress client with `Options.HTTP2` serves both; its host policy allows exactly these two hosts on `https` port `443`, so no `egress.allowed_hosts` entry is needed. `net/http` keeps one multiplexed HTTP/2 connection per host open. `Deps.Endpoints` exists only for tests.

### JWT cache and pause

Per environment: one ES256 JWT (header `alg`, `kid`; claims `iss`, `iat`), created under the environment's lock by the first send that needs one and reused for `push.apns.jwt_refresh_interval`, so parallel sends create at most one per interval. Each creation resolves and parses the key again, so a rotated key is used from the next JWT on. `403 ExpiredProviderToken` forces one replacement and one retry; a second one within the interval is a configuration error. A configuration error (the other `403` reasons, a key that cannot be resolved or parsed) pauses the environment for one interval: sends return `config_error` without a request or a log record, then one send probes with a new JWT; an answer other than a configuration error ends the pause.

### Answer table

| Answer | Result |
|---|---|
| `200` | `accepted` |
| any `410` | `disable` `token_invalid` with `InvalidSince` = `timestamp` |
| `400 BadDeviceToken` | `disable` `token_invalid`, one Warn record that names the environment |
| `400 DeviceTokenNotForTopic` | `disable` `topic_mismatch` |
| `403 ExpiredProviderToken` | renew the JWT, retry once |
| any other `403` | `config_error`, pause, one Error record |
| `429 TooManyProviderTokenUpdates` | `program_error`, Error record |
| any other `429` | `throttled` |
| other `4xx` | `program_error`, Error record |
| `5xx` | `retry` after `push.apns.retry_5xx_delay` |
| other status | `program_error`, Error record |
| no answer | Go error, transient for the job |

Error records carry only `bothy.push.provider` and `error.type` (the Apple reason of the table or `other`); never a token, the JWT, the key, the topic or an ID.

### Payload allow list

Structs, not maps: `aps` with `alert` (`title-loc-key` `PUSH_TITLE`, `loc-key` `PUSH_BODY_<KIND>`), `mutable-content: 1`, `thread-id`, `category` (`<KIND>`), `sound` `default` (absent when quiet), `badge` (always, also `0`) and `interruption-level` (`passive` when quiet, `time-sensitive` only for `input_needed` with the switch, else `active`); the object `b` with `v`, `nid`, `k`, `i`, `t` and `m`. Thread and message keys are absent when the hint has none. The badge sync is exactly `{"aps":{"badge":<n>}}` with `apns-priority: 5`. Before any request: another provider or environment, an unknown kind, a zero `nid` or instance, `HasBadge` false or a payload above Apple's 4096 bytes give `program_error`; an empty or non-hex token `disable` `token_invalid`; an `app_id` outside `push.apns.topics` `disable` `topic_mismatch`.

### Fake-APNs

`adapters/apns/apnstest` is an in-process HTTP/2 server with TLS on `127.0.0.1` that verifies the bearer JWT like APNs (`403 MissingProviderToken`, `403 InvalidProviderToken`), answers `405` and `404` to a wrong method or path, answers per token from a script and records every request and TLS connection. It imports no testing package (M3-24 uses it for the push handoff measurement) and is never linked into `cmd/bothy`. The tests of M3-20 and the integration tests in `test/integration/notify` (`withAPNs`) use it; M3-21 to M3-24 build on it. `Request.AcceptedAt` is the time at which the Fake accepted a request with `200`, after reading it and verifying the JWT and right before writing the status (t1 of 13 section 9 for `perf/push_handoff`, M3-24); it is zero for every other answer.

### Rulings

D-M3-20-0 to D-M3-20-8 in [docs/decisions.d/M3-20.md](../../../../docs/decisions.d/M3-20.md) ("M3-20 APNs adapter with Fake-APNs").

## Desktop hints over the gateway (M3-23)

### Live routes

`app.NewLiveRoutes` implements `LiveRoutes` inside the unit of work of the round (the consumer
round and the flush job); both methods only insert jobs, so jobs and cursor commit together:

- `Gateway`: one job `notify.gateway_send` per hint and recipient, inserted unique (the other
  connected devices of the same hint add nothing).
- `Delayed`: one `notify.push_send` per registration of the route, scheduled at the end of
  `notify.mobile_delay_when_desktop_active` with `DueAt` at the same time and `ReleaseSeq` =
  `Hint.MessageSeq`, the thread sequence of the message the hint points at.

### Job `notify.gateway_send`

Step 1 in a unit of work: older than `push.max_age` gives `expired`; a recipient who is no longer
an active human, a denied `push.deliver` on the thread (principal of a human without a request,
filter mode) or a failed recipient rule of R-AZ-30 for `security` and `admin` gives `no_access`;
the same function as the send step of `notify.push_send` decides (`mayDeliver`). Step 2 outside
any transaction: without a publisher in the process (role `worker`) the skip `no_registration`
with one Warn line per process, otherwise `notification.created` once to the recipient (audience
self; resource the thread, or the recipient for a hint without a thread). A connection that ended
meanwhile gets nothing and no fallback push. The payload holds `nid`, `kind`, `thread_id`,
`message_id`, `target_id`, `quiet` and `count`, and the preview fields of the recipient's
`notify.preview_mode` from the preview builder of M3-22 (see "Preview fields of
`notification.created`").

### Release check of delayed pushes

`notify.push_send` with `ReleaseSeq > 0` reads, after the registration, age and access checks and
before the provider lookup, the read position `max(read_states.last_read_seq,
threads.unread_floor_seq)` and ends with the skip reason `read` when it reached `ReleaseSeq`
(`domain.ReleaseDelayed`). An immediate push reads nothing extra; nothing else is evaluated again.

### Role table

| `server.role` | `presence.View` | `LiveRoutes` | publisher for `notify.gateway_send` |
|---|---|---|---|
| `all`, `cluster.mode = single` | the gateway's `PresenceView` over the hub | `app.NewLiveRoutes` | the gateway's `EphemeralPublisher` |
| `all`, `cluster.mode = multi` | the table-backed view of `gateway_presence` | `app.NewLiveRoutes` | the gateway's publisher (own hub, then `NOTIFY ephemeral`) |
| `worker`, any mode | the table-backed view of `gateway_presence` | `app.NewLiveRoutes` | the gateway's cluster publisher (`NOTIFY ephemeral` to the api nodes) |
| `api` | not wired (no consumer `push`) | not wired | not wired (no job execution) |

Since M7-17 (13 12 point 4, docs/decisions.d/M7-17.md D-M7-17-7) the worker reads the live
connections of every api node from `gateway_presence` through the gateway's adapter (the owner of
the table) and sends `notification.created` to the node that holds the connection, so separate
roles and several api nodes get desktop hints and the delayed push exactly as role `all`.
`notify` itself is unchanged: it sees only `presence.View` and an `EphemeralPublisher`.

### Rulings

D-M3-23-1 to D-M3-23-12 in [docs/decisions.d/M3-23.md](../../../../docs/decisions.d/M3-23.md) ("M3-23 Desktop hints over the gateway").

## Adapter `fcm` (M3-21)

`internal/notify/adapters/fcm` implements `PushProvider` for registrations with `provider = fcm` (`Name()` returns `fcm`). It sends one FCM HTTP v1 data message per hint and returns a `domain.SendResult`; the job `notify.push_send` does the rest (13 5.2, 6.2, 7.2). It imports the own `app` and `domain`, `kernel` and the standard library, and `platform/egress` and `platform/secrets`; no Firebase SDK and no `golang.org/x/oauth2`, because both would open HTTP clients past the egress client.

### Configuration

- `push.fcm.project_id`: FCM is enabled exactly when it is set. The cross-check `push_fcm_project_id` refuses a value that is not only lower-case letters, digits and hyphens starting with a letter, because it becomes a path segment of the send URL.
- `push.fcm.service_account_ref`: the service account JSON in the secret provider, default `file:<secrets.dir>/fcm_service_account.json`. `bothy config check --secrets` resolves it only when FCM is enabled.
- `push.ttl` (bounded to between 1 second and the FCM limit of 28 days) becomes `android.ttl`; `push.retry_cap` is the length of the pause.
- `internal/wire/push_fcm.go` builds the adapter only when FCM is enabled and only in the roles `worker` and `all` (the start part `startFCM` of `module_notify.go`), and adds it to the provider set of M3-19. `New` resolves and parses the service account (an error stops the start and names `push.fcm.service_account_ref`, never a value) and sends no request. Disabled: no adapter, no secret read, and jobs for `fcm` registrations end with `no_provider`.

### Endpoints and connection

One egress client per adapter, purpose `push_fcm`, HTTP/1.1 with keep-alive, `RedirectNever`. Its host policy allows exactly `https` on port `443` to `fcm.googleapis.com` and `oauth2.googleapis.com`, so no `egress.allowed_hosts` entry is needed; deadline and answer size are `egress.timeout` and `egress.max_response_bytes`. Sends go to `POST https://fcm.googleapis.com/v1/projects/<push.fcm.project_id>/messages:send` with `authorization: Bearer <access token>` and `content-type: application/json`. The token URL is fixed at `https://oauth2.googleapis.com/token`; a service account whose `token_uri` is present and differs is refused at start. `Deps.Endpoints` replaces both base URLs for tests only; `internal/wire` never sets it, and a test that sets it still runs every egress check.

### Access token cache

- The assertion is signed with `jose.RS256Signer` of `kernel/jose`: header `alg = RS256`, `typ = JWT`, `kid` = `private_key_id`; claims `iss` = `client_email`, `scope = https://www.googleapis.com/auth/firebase.messaging`, `aud` = the token URL, `iat` and `exp` = `iat` plus one hour (13 18 Q17). It is posted with the JWT bearer grant (`grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer`, `assertion`).
- Every token request resolves and parses the service account again, so a rotated key is used from the next token on without a restart (the secret provider caches for `secrets.cache_ttl`).
- A token is reused while now is before its time of issue plus half its `expires_in`. One mutex guards the cache and the token request runs under it, so at most one token request is in flight; a failed request stores nothing.
- A `401 UNAUTHENTICATED` from the send endpoint drops the token, gets a new one once and sends the same message once more; a second `401` is a configuration error.

### Pause

A configuration error of the credentials (`401` twice, `403 PERMISSION_DENIED`, `404 NOT_FOUND`, a token request refused with a `4xx`, a failed resolve or parse of the secret) drops the token, writes one Error record and pauses the adapter for `push.retry_cap`. During the pause `Send` returns `config_error` without a request and without a log record. The first send at or after its end probes once with a new token: any other result ends the pause with one Info record, a configuration error starts the next one. `SENDER_ID_MISMATCH` and `THIRD_PARTY_AUTH_ERROR` never pause; a token endpoint answering `429`, `5xx` or not at all gives a transient Go error and no pause. The pause lives in the process.

### Answer table

| Answer | Result |
|---|---|
| `200` | `accepted` |
| `404 UNREGISTERED`, `400 INVALID_ARGUMENT` with a violation on `message.token` or with the `FcmError` code `INVALID_ARGUMENT` and no violation on another field | `disable`, `token_invalid` |
| `400 INVALID_ARGUMENT` otherwise, any other `400` | `program_error`, Error record |
| `403 SENDER_ID_MISMATCH` | `disable`, `sender_mismatch`, Error record (configuration alert) |
| `401 THIRD_PARTY_AUTH_ERROR` | `config_error`, Error record, no pause |
| `401` otherwise | renew and retry once; a second `401` is `config_error`, pause |
| `403` otherwise, `404` otherwise | `config_error`, Error record, pause |
| `429` | `throttled`, `RetryAfter` = `Retry-After`, at least one minute |
| `5xx` | `retry`, `RetryAfter` = `Retry-After` when present, else zero (`domain.RetryDelay` applies) |
| any other status | `program_error`, Error record |
| no answer, egress refusal, oversized body, token request without answer or with `429` or `5xx` | Go error (transient) |

`Retry-After` is read as delta seconds or as an HTTP date; negative, unreadable or past values count as absent. Error records are `fcm configuration error`, `fcm program error` and `fcm sender mismatch` and carry exactly `bothy.push.provider = fcm` and `error.type` (the FCM error code, else `error.status`, else the OAuth `error` of a refused token request, else `other`). No log, span, metric or error text carries the device token, the assertion, the access token, key material, the client e-mail, the project ID, an ID of the hint or Google's `error.message`.

### Payload allow list

The message is built from structs with JSON tags, never from a map: `message` holds only `token`, `android` (`priority`, `ttl`) and `data`. `data` holds only `v`, `nid`, `k`, `i`, `t`, `m`, `badge` and `quiet`, every value a string: `t` and `m` are absent without thread or message, `badge` is always present, `Count` and `TargetID` are not carried. `priority` is `high`, `normal` for quiet hints and the sync form; `ttl` is `push.ttl` in whole seconds plus `s`. The sync form (`KindSync`) has `v`, `nid`, `k = "sync"`, `i`, `badge` and `cleared` (comma-separated, complete or absent). No `notification`, `collapse_key`, `apns` or `webpush` key exists. Before any request, a wrong provider, an unknown kind, a zero `nid` or instance ID or a missing badge give `program_error`, and an empty or non-printable token gives `disable` with `token_invalid`; `data` above the FCM limit of 4096 bytes gives `program_error` and is never truncated. `TestPayloadMatchesChapterExample` compares the message with the JSON example of 13 6.2; `testdata/payload_message.golden` holds its exact bytes for the client tests of M5-05.

### Fake-FCM

`adapters/fcm/fcmtest` (no `testing` import) is an HTTP/1.1 server with TLS on `127.0.0.1:0` that serves both endpoints. The token handler verifies the assertion (header, claims, lifetime at most one hour, signature with `rsa.VerifyPKCS1v15`) and issues random tokens; the send handler checks method, path and bearer, records each request and answers a script per device token with the FCM error JSON. `Start`, `URL`, `TokenURL`, `RootCAs`, `EgressRule` (`127.0.0.1:<port>;allow_private`), `ServiceAccountJSON`, `Script`, `ScriptToken`, `Requests`, `TokenRequests`, `RevokeTokens`, `SetPublicKey` and `Close`; tests trust it through `egress.Deps.RootCAs` and never turn off certificate verification.

### Rulings

The rows `D-M3-21-<n>` in [docs/decisions.d/M3-21.md](../../../../docs/decisions.d/M3-21.md).

## Registration and preview (M3-22)

### Registration

`Registrations.Register` serves `registerPush` (13 6.4 and 13):

1. `authorize(actor, push.register, device {ID: actor.DeviceID, OwnerID: actor.ID})`: another
   person's device is `not_found`, a principal without a session (agent, app, API token) is
   `forbidden` with `step_up_not_possible`, a waiting device `device_pending`.
2. `identity.Devices.Get` for the platform of the device, then `domain.ValidateRegistration`
   against the `RegistrationPolicy` that `internal/wire` builds in every role from
   `apns.Config.Enabled`, `fcm.Config.Enabled`, `push.apns.topics` and
   `push.registration_field_max_bytes`, without building an adapter or reading a secret:
   `unifiedpush` and `wns` give `validation_failed` with `not_enabled` at `/provider`; `apns` or
   `fcm` not enabled give `feature_disabled` with the reason `push_provider_disabled`; then the
   field rules in one `validation_failed` (platform `invalid` at `/provider`, `sandbox` only for
   `apns` at `/environment`, `too_long` above the byte limit, else `pattern_mismatch` at `/token`
   for an `apns` token that is not an even number of hexadecimal digits or an `fcm` token that is
   not printable ASCII without white space, an `apns` `app_id` outside the topics `invalid` at
   `/app_id`). An absent environment is `production`; an `apns` token is stored in lower case.
3. One unit of work: `LockByProviderToken` locks the row of a known `(provider, token)`, active or
   disabled and of any person, and `Delete` removes it (the takeover; its pending
   `notify.push_send` jobs end with `no_registration`); `DisableOthersOfApp` sets `token_invalid`
   on the other active rows of the device with the same provider and `app_id` (one current token
   per app installation), locked in ascending `id`; the preview token is minted through
   `identity.Tokens.NewPushPreviewToken` and `Insert` writes the row with the caller's person,
   device and session and the HMAC of the token.
4. When the insert hits the unique key `(provider, token)` because a concurrent registration
   committed first (`ErrRegistrationTaken`), the unit of work runs once more; a second loss
   answers `conflict`.

The handler marks the answer as a one-time secret (`idempotency.MarkOneTimeSecret`), so a replay
with the same `Idempotency-Key` answers `409 conflict` with the reason `one_time_secret` and the
idempotency row holds no part of the token. `Registrations.Unregister` locks the row with
`OwnerOf`, answers `not_found` for an unknown one, authorizes `push.unregister` on the kind
`push_registration` with the person as owner (another person's row is `not_found` too) and
deletes it. Neither use case writes an audit entry or an event.

| Port method of `RegistrationWriter` | Query in `queries/registrations_write.sql` |
|---|---|
| `LockByProviderToken` | `LockRegistrationByProviderToken` |
| `Delete` | `DeleteRegistration` |
| `DisableOthersOfApp` | `DisableOtherRegistrationsOfApp` |
| `Insert` | `InsertRegistration` |
| `ByPreviewHash` | `GetRegistrationByPreviewHash` |
| `OwnerOf` | `GetRegistrationForOwner` |
| `DisableOfSessions` | `DisableRegistrationsOfSessions` |
| `DisableOfDevice` | `DisableRegistrationsOfDevice` |
| `OfDevices` | `ListRegistrationsOfDevices` |

No query returns a token or `preview_token_hash`; the device token never leaves the adapters of
`notify`.

### Preview token

`bothy_pvw_<random>_<checksum>`, the kind `pvw` of `identity`'s `domain/apitoken` (ADR 0061),
minted by `identity.Tokens.NewPushPreviewToken`; `push_registrations.preview_token_hash` holds its
HMAC-SHA-256 with the pepper of `auth.token_pepper_ref`. It crosses the port only as `Secret` and
leaves the server only in the `201` of `registerPush`. It has no expiry and no refresh: it ends
with its registration, its session, its device or its person. The authenticator refuses it on
every other operation.

### Preview endpoint

`Registrations.Preview` serves `getPushPreview` (13 6.4), in this order; the first failure ends
the request:

1. `domain.PreviewKindAllowed(kind)`, otherwise `validation_failed` with `invalid` at
   `#query/kind`;
2. exactly one `Authorization` value with the scheme `DPoP` (from `actor.CredentialsFrom`, the
   route has the flag `OwnCredential`), otherwise `401 unauthenticated`;
3. `identity.Tokens.HashPushPreviewToken` (form, kind and checksum, no database query), then
   `ByPreviewHash` over the active rows: a malformed, unknown or deactivated token is
   `401 unauthenticated` with the challenge `invalid_token`; from here on `bothy.push.fetch`
   counts once with the registration's provider, whatever the answer;
4. `identity.Auth.SessionActor` of the registration's person and session: another device is
   `401 unauthenticated`, a session without an active refresh family `401 session_revoked`;
5. `identity.DPoP.Verify` against the actor's `KeyThumbprint` (the `dpop_jkt` of the family), with
   `ath` over the preview token; a stale nonce adds `DPoP-Nonce` to the answer;
6. the message through `PreviewBuilder.Message` (unknown, deleted, `run` or `system`: `404`
   without `authorize`), then `message.read` on it and `push.deliver` on its thread for the
   session actor in normal mode: `unauthenticated` is `401 session_revoked`, every other denial
   `404`, identical to an unknown message;
7. the mode, the badge (`BadgeCounter.For` of the push path) and the fields of the builder.

Logout, device revocation and user disable deactivate the registration in their own transaction
(see "Deactivation"), so their tokens already fail at step 3; steps 4 and 6 catch what ends while
the row stays active, such as an expired session.

### Preview modes and excerpt

`PreviewBuilder.Mode` reads the stored `notify.preview_mode` (`sync` settings); a value that is
not exactly `full`, `names` or `none` counts as `notify.default_preview_mode`.
`PreviewBuilder.Fields` gives nothing for `none`; for `names` the space, channel and thread names
of `chat.Structure.PreviewNames` (each only when set) and the author's display name from
`identity.Directory`; for `full` also `domain.Excerpt` over `content_md` with the display names of
the mentioned principals, the number of attachments and the first image (`files.Attachments`,
read only when the message has attachments). The preview endpoint turns the first image into
`image_url`, a fetch ticket of the session actor for the thumbnail `small`; any error of the
ticket leaves it out. `domain.Excerpt` replaces mentions of mentioned principals with their
display names, strips Markdown (fences, inline code, emphasis, strong, strike, links, images,
autolinks, heading, quote, list and table markers, escapes; HTML stays literal), removes control
and bidi control characters, folds white space and cuts above `notify.preview_max_chars` code
points with U+2026.

### Preview fields of `notification.created`

Step 1 of `notify.gateway_send` calls the same builder after its `push.deliver` check (or the
rule of R-AZ-30), only for a kind that `domain.PreviewKindAllowed` accepts with a set message:
`none` builds nothing, an unknown, deleted, `run` or `system` message builds nothing and the
event still goes out. There is no `message.read` check here (the job has no request, R-AZ-29) and
never an `image_url` (a fetch ticket needs a session); `attachment_count` is filled for `full`. A
failed read fails the job, which the queue runs again.

### Deactivation

The in-transaction subscribers `Subscribers.OnSessionEnded` and `Subscribers.OnDeviceRevoked`,
registered in `internal/wire` after the other subscribers of these events, disable the active
rows in the revoking transaction: `OnSessionEnded` the rows of the ended sessions with
`domain.DisableReasonForSession` of the session reason (`device_revoked` and `user_disabled` give
`device_revoked`, every other reason `logout`), `OnDeviceRevoked` every row of the device with
`device_revoked`. `user.disable` reaches both through the events of `identity`. A row that is
already disabled keeps its first reason; `disabled_at` is the time the event carries. A session
that only expires publishes nothing: its rows stay active and the preview answers
`401 session_revoked`.

### Push targets of `Device`

`PushTargets` implements the source port `identity.PushTargets` that `identity` declares, so the
schema `Device` lists the registrations of a device (active and disabled, ascending by `id`,
without token or hash) without `identity` importing `notify`. One `OfDevices` query per answer; no
`authorize`, because `identity` reads only devices it authorized.

### Signal

`bothy.push.fetch` (counter, attribute `bothy.push.provider`): one per preview request whose
token identified an active registration, whatever the answer after that; nothing for a malformed
or unknown token. It is the receipt t2 of 13 9. Logs, spans and errors of the registration and
the preview never carry a device token, a preview token, the `Authorization` or `DPoP` header, a
ticket URL, a name or a text; a failed image ticket other than a missing or unready thumbnail writes one Warn line with `error.type` only.

### Rulings

D-M3-22-0 to D-M3-22-12 in [docs/decisions.d/M3-22.md](../../../../docs/decisions.d/M3-22.md) ("M3-22 Push registration and preview endpoint").

## Retention (M7-05)

`adapters/postgres.Registrations` is also the participant of notify in the job `retention.security`:
push registrations whose `disabled_at` lies before now minus `retention.security_after_end`.

## Port (root package `internal/notify`)

- Inbound: `Preferences`, `QuietHours`, `PushRegistrations` (`Active` of M3-19; `Register`, `Unregister` and `Preview` of M3-22).
- Emits in-tx events: none.
- Subscribes to in-tx events: `identity.DeviceRevoked`, `identity.SessionEnded` (deactivate the registrations of the session), `chat.ChannelArchived`, `chat.SpaceArchived`, `messaging.MessagePosted` (automatic following, M3-19).
- Implements source ports and cross-cutting ports: kernel `personaldata.Participant`, `identity.PushTargets` (M3-22).

## Dependencies

May use the ports of these modules: `identity`, `chat`, `messaging`, `sync`, `authz`, `files`.

Reason: devices and time zone; target path for settings; mentions; read state; candidates with read permission for notifications; attachments and fetch tickets for the preview.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`notification_preferences`, `quiet_hours`, `push_registrations`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`notification_pref.update`, `quiet_hours.update`, `push.register`, `push.unregister`, `push.deliver`.

## Events and WebSocket ops

- Durable: `notification_pref.updated`, `quiet_hours.updated`.
- Ephemeral: `notification.created`.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `notify.push_send` | consumer `push`, `notify.coalesce_flush`, `notify.badge_sync` | `delivery` |
| `notify.gateway_send` | consumer `push`, `notify.coalesce_flush` (gateway route, M3-23) | `delivery` |
| `notify.coalesce_window` | consumer `push`, `notify.coalesce_flush` | `delivery` |
| `notify.coalesce_flush` | consumer `push` | `delivery` |
| `notify.badge_sync` | consumer `push` | `delivery` |
| `notify.badge_sync_hold` | `notify.badge_sync` | `delivery` |
| `retention.security` (participant, M7-05) | periodic every `retention.job_interval` | `maintenance` |

## Notes for the build

- A push never carries content; before every dispatch the job checks `push.deliver` (05, R-AZ-11).
- The consumer `push` creates jobs and advances its cursor in the same transaction (02, 11.5).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [13-push-notifications.md](../../../../docs/spec/13-push-notifications.md)

## Statistics source (M6-05)

`notify.StatsSource` (`stats.go`, implemented by `app.Stats` over
`adapters/postgres/queries/stats.sql`) serves `getErrorStats`: `PushStats` counts per bucket and
provider the registrations active in the bucket and those disabled in it for a provider reason
(`token_invalid`, `topic_mismatch`, `sender_mismatch`, `repeated_failures`; a logout or a revoked
device is no failure). The registrations have no channel, so the stats use case calls it only for an
owner on the instance; no person, device or token leaves.
