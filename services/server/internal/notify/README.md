# Modul `notify`

Status: Gerüst (M1-05) plus the decision per recipient as pure functions in `domain/` (M3-17). Interfaces, use cases, tables and the push consumer come with M3-18 to M3-24.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Benachrichtigungsstufen, Ruhezeiten, Push-Registrierungen, Push-Verbraucher und Versand.

## Code

- `internal/notify` (`doc.go`): Port als Wurzelpaket, bisher nur mit Paketkommentar. Schnittstellen, DTOs und In-Tx-Ereignisse kommen mit dem Ticket, das sie umsetzt.
- `internal/notify/domain` (M3-17): the decision per recipient of 13 section 4 as side-effect-free functions: `ResolveLevel` (`level.go`), `InQuietHours` and `ParseLocalTime` (`quiet.go`), `Decide` (`decide.go`), `Flush` (`flush.go`), `ReleaseDelayed` (`delay.go`), the enums in `kinds.go`. It imports only the standard library without I/O and `kernel-core`; `TestDomainPurity` and `make arch` hold that. The golden table is `domain/testdata/decision_table.golden`.
- `app/` und `adapters/<art>/` entstehen mit dem ersten Anwendungsfall. Ihre Komponenten erzeugt `tools/archgen` aus `architecture/modules.yaml`; ein Paket außerhalb dieser Schichten scheitert in `make arch`.

## Decision per recipient (Entscheidung je Empfänger)

`domain.Decide` takes the facts about one hint for one person and returns one outcome: `none` with a skip reason, `collect` into the open bundling window, or `send`, quiet or normal, with one route per device. It reads no database, calls no `authorize`, reads no clock, loads no time zone and reads no configuration; the caller passes everything in (docs/DECISIONS.md D-M3-17-1). M3-18, M3-19 and M3-23 call these functions instead of re-implementing them.

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

`security` passes the level step at every level, `admin` only by `notify.admin_enabled`; both have no thread, so no level, no activity in a thread and no window apply to them, quiet hours do (D-M3-17-2). The skip reasons are internal constants; M3-19 maps them to the attribute values of `bothy.push.skipped`. No skip reason stands for quiet hours: they make a hint quiet, never drop it (R-PN-05).

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
- A kind with thread that sends while no window is open sets `OpensWindowUntil = Now + notify.coalesce_window`, whatever its kind. A length of zero opens no new window, so every hint goes at once, except a `message` while a window opened earlier is still open.
- `message` in an open window gives `collect` without routes. `mention`, `dm`, `input_needed`, `run_done` and `run_failed` in an open window send at once and leave the window unchanged.
- `Flush` at the window end: `Collected` zero gives `nothing_collected`, `Unread` (03 `read_states`, "Ungelesen") zero or less gives `read`; otherwise allowed, level, activity, quiet and route run again at the flush time, `Author` and `Window` are ignored, and a send carries `Count = Unread` and opens the next window. A flush that sends nothing leaves the window closed.

### Route rules (13 4.4, 4.6, D-M3-17-6)

- Active in the thread: a device with a connection that is not idle and whose focus contains the thread, on any platform. It stops the hint on every device.
- Desktop active: a `macos` or `windows` device with a connection that is not idle.
- One route per device, in the order of `Devices`: a connection (idle or not) gets `gateway` (`notification.created`); else a device with registrations gets `push_delayed` with `NotBefore = Now + notify.mobile_delay_when_desktop_active` if it is `ios` or `android`, the desktop is active and the delay is above zero, otherwise `push`; a device with neither gets nothing.
- No route on any device gives `none` with `no_route` and opens no window.
- `ReleaseDelayed(hintSeq, readSeq)` drops a delayed push exactly when `readSeq >= hintSeq`; a hint without a message (`hintSeq` zero or less) is always released.
- A delayed push keeps the `Quiet` flag of its decision (D-M3-17-4).

### Rulings

D-M3-17-1 to D-M3-17-8 in [docs/DECISIONS.md](../../../../docs/DECISIONS.md), section "M3-17 Notification decision".

## Port (Wurzelpaket `internal/notify`)

- Eingehend: `Preferences`, `QuietHours`, `PushRegistrations`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: `identity.DeviceRevoked`, `identity.SessionEnded` (Registrierungen der Sitzung deaktivieren), `chat.ChannelArchived`, `chat.SpaceArchived`.
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `chat`, `messaging`, `sync`.

Grund: Geräte und Zeitzone; Ziel-Pfad für Einstellungen; Erwähnungen; Gelesen-Stand.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`notification_preferences`, `quiet_hours`, `push_registrations`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`notification_pref.update`, `quiet_hours.update`, `push.register`, `push.unregister`, `push.deliver`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `notification_pref.updated`, `quiet_hours.updated`.
- Kurzlebig: `notification.created`.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `notify.push_send` | Verbraucher `push` | `delivery` |

## Hinweise für den Bau

- Push trägt nie Inhalt; vor jedem Versand prüft der Job `push.deliver` (05, R-AZ-11).
- Der Verbraucher `push` erzeugt Jobs und rückt seinen Cursor in derselben Transaktion vor (02, 11.5).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [13-push-benachrichtigungen.md](../../../../docs/spec/13-push-benachrichtigungen.md)
