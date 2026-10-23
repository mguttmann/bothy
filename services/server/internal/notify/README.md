# Module `notify`

Status: skeleton (M1-05), the decision per recipient as pure functions in `domain/` (M3-17), the notification levels and quiet hours with their tables, use cases, REST operations, archive cleanup and the reads of the push path (M3-18), and the push path (M3-19): the table `push_registrations` with its read port, the worker consumer `push`, the bundling window, the job `notify.push_send` with the provider port, automatic following, the badge and the badge sync. The provider adapters (M3-20, M3-21), registration and preview (M3-22), the gateway route and presence (M3-23) and the delivery measurement (M3-24) follow.
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

Migration `00059_notify_push_registrations.sql` creates the table exactly as 03 (provider, token, environment, app ID, preview token hash, delivery columns, the check that `disabled_at` and `disabled_reason` are set together, cascades from `users`, `devices` and `auth_sessions`, unique `(provider, token)`, the partial unique index on `preview_token_hash` and the indexes `(user_id) WHERE disabled_at IS NULL` and `(auth_session_id)`), retention class `security`. This ticket writes no rows except the delivery columns of a send; `registerPush` and `unregisterPush` come with M3-22. The token and `preview_token_hash` never leave the adapters of `notify`.

### Ports

- Incoming `PushRegistrations.Active(ctx, userIDs)`: the registrations with `disabled_at IS NULL` of the given people, grouped by person, ascending by `id`, without token. It does not authorize: the callers select the people. M3-22 adds the writes on the same port.
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
- M3-22 adds the writes of `PushRegistrations` (`registerPush`, `unregisterPush`), the preview token and endpoint (with `BadgeCounter.For`) and the deactivation on logout, device revocation and user disable.
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

`adapters/apns/apnstest` is an in-process HTTP/2 server with TLS on `127.0.0.1` that verifies the bearer JWT like APNs (`403 MissingProviderToken`, `403 InvalidProviderToken`), answers `405` and `404` to a wrong method or path, answers per token from a script and records every request and TLS connection. It imports no testing package (M3-24 uses it for the push handoff measurement) and is never linked into `cmd/bothy`. The tests of M3-20 and the integration tests in `test/integration/notify` (`withAPNs`) use it; M3-21 to M3-24 build on it.

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
`message_id`, `target_id`, `quiet` and `count`; the preview fields of 13 6.4 are absent until
M3-22 adds them with its preview builder.

### Release check of delayed pushes

`notify.push_send` with `ReleaseSeq > 0` reads, after the registration, age and access checks and
before the provider lookup, the read position `max(read_states.last_read_seq,
threads.unread_floor_seq)` and ends with the skip reason `read` when it reached `ReleaseSeq`
(`domain.ReleaseDelayed`). An immediate push reads nothing extra; nothing else is evaluated again.

### Role table

| `server.role` | `presence.View` | `LiveRoutes` | publisher for `notify.gateway_send` |
|---|---|---|---|
| `all` | the gateway's `PresenceView` | `app.NewLiveRoutes` | the gateway's `EphemeralPublisher` |
| `worker` | `presence.None` | `FailClosedRoutes` (M3-19) | none |
| `api` | not wired (no consumer `push`) | not wired | not wired (no job execution) |

A worker without presence logs `presence unavailable in this role: hints go by push only` once at
start. Until M7-17 separate roles and several api nodes reach desktops only by push (RK-37).

### Rulings

D-M3-23-1 to D-M3-23-12 in [docs/decisions.d/M3-23.md](../../../../docs/decisions.d/M3-23.md) ("M3-23 Desktop hints over the gateway").

## Port (root package `internal/notify`)

- Inbound: `Preferences`, `QuietHours`, `PushRegistrations`.
- Emits in-tx events: none.
- Subscribes to in-tx events: `identity.DeviceRevoked`, `identity.SessionEnded` (deactivate the registrations of the session), `chat.ChannelArchived`, `chat.SpaceArchived`, `messaging.MessagePosted` (automatic following, M3-19).
- Implements source ports and cross-cutting ports: kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `identity`, `chat`, `messaging`, `sync`, `authz`.

Reason: devices and time zone; target path for settings; mentions; read state; candidates with read permission for notifications.

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

## Notes for the build

- A push never carries content; before every dispatch the job checks `push.deliver` (05, R-AZ-11).
- The consumer `push` creates jobs and advances its cursor in the same transaction (02, 11.5).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [13-push-notifications.md](../../../../docs/spec/13-push-notifications.md)
