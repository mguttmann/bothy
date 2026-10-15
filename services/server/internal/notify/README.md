# Module `notify`

Status: skeleton (M1-05), the decision per recipient as pure functions in `domain/` (M3-17), and the notification levels and quiet hours with their tables, use cases, REST operations, archive cleanup and the reads of the push path (M3-18). Push registrations, the push consumer and delivery come with M3-19 to M3-24.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Notification levels, quiet hours, push registrations, push consumer and dispatch.

## Code

- `internal/notify` (`doc.go`, `preferences.go`): the port as root package: `Preferences` and `QuietHours` with their DTOs `Target`, `PutPreference`, `Preference`, `QuietHoursInput`, `QuietHoursRule` (M3-18).
- `internal/notify/domain` (M3-17): the decision per recipient of 13 section 4 as side-effect-free functions: `ResolveLevel` (`level.go`), `InQuietHours` and `ParseLocalTime` (`quiet.go`), `Decide` (`decide.go`), `Flush` (`flush.go`), `ReleaseDelayed` (`delay.go`), the enums in `kinds.go`. It imports only the standard library without I/O and `kernel-core`; `TestDomainPurity` and `make arch` hold that. The golden table is `domain/testdata/decision_table.golden`.
- `internal/notify/domain` (M3-18): `TargetKind` and `PreferenceUnchanged` (`target.go`), `RuleInput`, `ValidateRuleInput` and `RuleUnchanged` (`rule.go`).
- `internal/notify/app` (M3-18): the use cases (`preferences.go`, `quiethours.go`), the two event builders (`events.go`), the in-transaction subscribers (`subscribers.go`) and the reads of the push path (`facts.go`).
- `internal/notify/adapters/postgres` (M3-18): the repository over the sqlc queries in `queries/`; `adapters/httpapi`: the seven REST operations of the group Preferences.
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
- `upsertPreference` (exported as `Service.UpsertPreference`) is the single write path of `notification_preferences`; it neither authorizes nor locks. M3-19 calls it for the automatic following of 13 3.2 inside the message transaction.
- `Facts.LevelFacts(ctx, threadID, userIDs)`: the facts of 13 3.1 for many people in one preference query; the caller resolves with `domain.ResolveLevel`. `Facts.QuietRules(ctx, userIDs)`: the rules with their zones loaded (rule zone, else `users.time_zone`, a zone that no longer loads counts as UTC). Both are for M3-19 and do not authorize.
- Configuration: `notify.user_default_level`; the list of rules is bounded by `sync.event_payload_max_bytes`.
- Rulings: D-M3-18-1 to D-M3-18-11 in [docs/DECISIONS.md](../../../../docs/DECISIONS.md), section "M3-18 Notification preferences and quiet hours".

## Port (root package `internal/notify`)

- Inbound: `Preferences`, `QuietHours`, `PushRegistrations`.
- Emits in-tx events: none.
- Subscribes to in-tx events: `identity.DeviceRevoked`, `identity.SessionEnded` (deactivate the registrations of the session), `chat.ChannelArchived`, `chat.SpaceArchived`.
- Implements source ports and cross-cutting ports: kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `identity`, `chat`, `messaging`, `sync`.

Reason: devices and time zone; target path for settings; mentions; read state.

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
| `notify.push_send` | consumer `push` | `delivery` |

## Notes for the build

- A push never carries content; before every dispatch the job checks `push.deliver` (05, R-AZ-11).
- The consumer `push` creates jobs and advances its cursor in the same transaction (02, 11.5).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [13-push-notifications.md](../../../../docs/spec/13-push-notifications.md)
