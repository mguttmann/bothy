# M3-17: Notification decision as a pure function

Status: ready | Depends on: M1-05 | Milestone: M3

## Goal

After this ticket the package `internal/notify/domain` holds the per-recipient decision of 13
"Decision per recipient" as side-effect-free functions with a documented decision table.
`ResolveLevel` applies the seven steps of 13 3.1 "Effective level" including a temporary
`muted_until`; `InQuietHours` applies 13 3.4 "Quiet hours with time zone" to the local wall time of
each rule's zone; `Decide` takes the facts about one hint for one person (kind, the result of the
`push.deliver` check, the level sources, the personal switches, the quiet-hour rules, the person's
active devices with their live connection, focus and push registrations, the bundling window of
the thread and the two window lengths) and returns exactly one outcome: no hint with a skip
reason, collect into the open window, or send now, quiet or normal, with one route per device
(`notification.created` over the gateway, push now, or push after the desktop delay). `Flush`
decides the one hint at the end of a bundling window, and `ReleaseDelayed` drops a delayed push
the person has read in the meantime. The functions read no database, call no `authorize`, use no
clock, load no time zone and read no configuration, so the rules are proven by tables: a golden
table of every combination of kind, effective level, `muted_until`, quiet hours, the two personal
switches, activity in the thread, active desktop and bundling window; quiet hours in
`Europe/Berlin` on both changeover days and across midnight; and `input_needed` under `mute`
arriving quietly in quiet hours. The tickets of the push path (M3-18 settings, M3-19 consumer and
bundling, M3-23 desktop delivery) call these functions instead of re-implementing them.

## Read first

- `docs/spec/13-push-notifications.md`: 1 "Goals" (points 2 to 4); 2 "Kinds of notifications and recipients" (the table, the paragraph "No notification is created for"), 2.1 "Agent messages during a run", 2.2 "Report channels"; 3.1 "Effective level"; 3.2 "Automatic following"
  (owned by M3-19); 3.3 "Personal switches"; 3.4 "Quiet hours with time zone"; the whole of 4
  "Decision per recipient": 4.1 "Flow" (flowchart), 4.2 "Permissions", 4.3 "Level and kind", 4.4
  "Active on a device", 4.5 "Coalescing", 4.6 "Delivery path per device"; 6.3 "Gateway event
  `notification.created`"; 9 "Delivery measurement" (row t0); 12 "Execution in the server" (point 3: the
  decision is a pure function in the domain of `notify`); 14 "Tests" (rows "Decision table",
  "Quiet hours in `Europe/Berlin`", "`input_needed` with `mute`", and the rows owned by M3-19 and
  M3-23 that this ticket mirrors at decision level); 15 "Decisions of this chapter" (R-PN-01
  to R-PN-06); 17 "Configuration values of this chapter".
- `docs/spec/03-data-model.md`: `notification_preferences` (`level`, `muted_until`, the paragraph
  "Effective level" after the table, R-DM-16), `quiet_hours` (`weekdays`, `start_local`,
  `end_local`, `time_zone`, `enabled`, check `weekdays BETWEEN 1 AND 127`), `users.time_zone`,
  `channels.default_notification_level`, `user_settings` (`notify.*` keys), `devices` (`platform`,
  `state`), `push_registrations` (`disabled_at`), `threads.last_message_seq`, `messages.thread_seq`,
  `read_states` (`last_read_seq` and the formula "Unread" with `threads.unread_floor_seq`).
- `docs/spec/04-sync-protocol.md`: 6.1 "Endpoint and version" (a device holds exactly one
  connection), 6.2 "Ops" (row `focus` with `idle`), 7.6 "Consumers in the worker (push, webhooks)"
  (every job checks `push.deliver` again before it sends).
- `docs/spec/05-actions.md`: 4.9 rows `push.deliver`, `notification_pref.update`,
  `quiet_hours.update`, `settings.read`; 6, table of ephemeral events, row `notification.created`.
- `docs/spec/07-permissions.md`: 11 test row AZ-M48 (a mention without read right gives no push; owned
  by M3-19, mirrored here at decision level).
- `docs/spec/01-product.md`: 10.1 "When a notification arrives"; screen D9 (quiet hours "all day",
  temporary muting "until today 18:00"); R-PR-05, R-PR-06.
- `docs/spec/02-architecture.md`: 4.2 "Module list" (row `notify`), 4.3 and 4.4 (rows `notify`), 5
  "Layers within a module" (domain imports only `kernel-core`), 6 "Code layout", 11.5
  "Worker consumers (push, webhooks)".
- `docs/spec/23-quality.md`: "Documentation in code" (Go: revive rules `exported` and
  `package-comments`).
- `docs/api/openapi.yaml`: schemas `NotificationKind`, `NotificationLevel`, `NotificationPreference`
  (`muted_until`), `QuietHoursRule` (`weekdays` 1 to 127, `start_local` and `end_local` as `HH:MM`,
  `time_zone`), `NotificationCreatedPayload` (`quiet`, `count`, optional `thread_id`). Read only;
  this ticket enables no operation. `docs/api/asyncapi.yaml`: message `event.notification.created`.
- `docs/operations/configuration.md`: `notify.user_default_level`, `notify.coalesce_window`,
  `notify.mobile_delay_when_desktop_active`, `notify.default_input_needed_breaks_quiet_hours`,
  `notify.default_admin_enabled` (this ticket reads none of them; the caller passes their values).
- ADRs 0008 (hexagonal modules and checked dependencies), 0031 (push without content), 0051
  (worker consumers of the event log).
- `docs/DECISIONS.md`: D-PN-01 to D-PN-04, D-PN-07, D-PR-01, D-W-100; section 10 "Rulings of the build milestones" (format of the rows you add).
- `docs/OPEN-QUESTIONS.md`: OQ-05 (quiet hours make hints silent, not muted; default "yes, as in
  chapter 13", answer due before M3-19). This ticket builds the default; a different answer changes
  one function.
- `docs/security/threat-model.md`: T-PU-01 (push metadata; only IDs leave the server).
  `docs/RISKS.md`: no RK row names this ticket; RK-37 (presence state per api node) concerns the
  presence port of 13 12 point 4 (M3-23) and multi-node operation (M7), not the decision, which
  gets the connections as facts.
- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 5 "Test strategy" (row "Unit");
  section 9 "M3 Platform services": "Goal", "Exit criteria" point 1, "Test strategy" ("golden table
  of the notification decision"), "Red team" (content in push payloads), rows M3-17 (this
  ticket), M3-18, M3-19, M3-20 to M3-24 (who builds what next).
- Dependencies: the header names M1-05; its closure adds M1-01. Nothing of M2 or of the other M1
  tickets is needed, because the decision imports no foreign module. Read
  `docs/briefs/M1-05-server-skeleton.md` (layers, `kernel/ids`, `kernel/errs`, the pattern of
  `TestCodesMatchChapter06` for finding the repository root) and, for the source of `focus` and
  `idle`, `docs/briefs/M1-66-gateway-focus-typing.md`.
- Code on the base branch (merged names win, D-W-100): `services/server/internal/notify/doc.go` and
  `README.md` (port package only, no `domain/` yet), `services/server/internal/kernel/ids` (`ids.ID`,
  `IsZero`), `services/server/internal/kernel/errs` (`errs.New`, `errs.CodeInternalError`,
  `errs.As`, the field `(*errs.Error).Code`), `services/server/architecture/modules.yaml` (module
  `notify`), `services/server/.go-arch-lint.yml` (component `notify-domain` may depend only on
  itself and `kernel-core`).

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

11. The decision per recipient follows 13 4.1 in this order: the `push.deliver` result, kind and
    effective level, activity in the thread, quiet hours, bundling window, route per device; it is
    a pure function in the domain of `notify` that gets facts and returns a decision (13 12 point
    3, PLAN row M3-17, R-PN-01).
12. The effective level is the first present value of the seven steps of 13 3.1; if the chosen own
    row has `muted_until` in the future, `mute` applies (13 3.1, 03 `notification_preferences`).
13. Level and kind: `all` allows every kind, `mentions` every kind except `message`, `mute` only
    `input_needed` (own run), `security` and `admin`; `admin` depends only on
    `notify.admin_enabled`; `input_needed` and `security` come at every level (13 2, 13 4.3,
    R-PN-03, R-PR-05).
14. Recipients of 13 2: the author of a message never gets a hint for it; `run_done`, `run_failed`
    and `input_needed` go only to the person who triggered the run; only active humans are
    recipients (13 2, 2.1, 4.2).
15. A device of the person with a gateway connection whose focus contains the thread and that does
    not report `idle` means no hint for that thread on any device (13 4.4).
16. Quiet hours: now converted into the zone of the row gives weekday and time of day; same-day and
    across-midnight windows with weekday bits as in 13 3.4 points 2 to 4; the server never computes
    the boundaries as instants. In quiet hours a hint is quiet, never dropped; the only exception is
    `input_needed` with `notify.input_needed_breaks_quiet_hours` (13 3.4, R-PN-05, R-PR-06).
17. Bundling per recipient and thread with `notify.coalesce_window`: the first hint goes at once
    and opens the window, further `message` hints are collected, one hint with the unread count
    goes at the window end; `mention`, `dm`, `input_needed`, `run_done`, `run_failed` and
    `security` always go at once (13 4.5, R-PN-06).
18. Route per active device: a live connection gets `notification.created`; otherwise each active
    push registration gets a push; with an active desktop, pushes to mobile devices wait
    `notify.mobile_delay_when_desktop_active` and are dropped if read meanwhile; a device with
    neither gets nothing (13 4.4, 4.6).
19. The decision carries only IDs, kinds, flags, counts and times, never text (ADR 0031, T-PU-01).
20. Golden table test of all combinations; quiet hours in `Europe/Berlin` on both changeover days
    and across midnight; `input_needed` with `mute` quiet in quiet hours (PLAN row M3-17, 13 14).
21. No I/O, no `authorize`, no clock, no time zone loading, no configuration reading in
    `internal/notify/domain` (PLAN row M3-17; 02 5 "Layers within a module", row "Domain").

## Scope

All paths under `services/server/` unless noted. This ticket creates no migration, enables no
operation, appends no event, schedules no job, records no metric and adds no configuration key.
Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`, like the existing
`internal/notify/doc.go` (`make licenses`).

Create:

- `internal/notify/domain/doc.go`: package comment of the domain layer of `notify` (what lives
  here, that it imports only `kernel-core` and the standard library without I/O, and that callers
  pass facts in). If the merged code already has this file with a package comment only (M1-05 adds
  such files when go-arch-lint needs them), extend it instead.
- `internal/notify/domain/kinds.go`: `Kind`, `Level`, `Platform`, `LevelSource`, `Outcome`,
  `SkipReason`, `Via` with their constants, `Valid` methods, `Kind.HasThread`, `Kind.Urgent`,
  `Platform.Desktop`, `Platform.Mobile` and the list functions `Kinds`, `Levels`, `Platforms`
  (design notes 2 and 3).
- `internal/notify/domain/level.go`: `Preference`, `LevelFacts`, `EffectiveLevel`, `ResolveLevel`
  (design note 4).
- `internal/notify/domain/quiet.go`: `LocalTime`, `ParseLocalTime`, `LocalTime.String`, `QuietRule`,
  `InQuietHours` (design note 6).
- `internal/notify/domain/decide.go`: `Switches`, `Connection`, `Device`, `Window`, `Policy`,
  `RecipientFacts`, `Route`, `Decision`, `Decide` (design notes 2, 3, 5, 7, 8).
- `internal/notify/domain/flush.go`: `FlushState`, `Flush` (design note 7).
- `internal/notify/domain/delay.go`: `ReleaseDelayed` (design note 9).
- Tests (design of each in "Tests"): `level_test.go`, `quiet_test.go`, `decide_test.go`,
  `flush_test.go`, `delay_test.go`, `table_test.go`, `contract_test.go`, `purity_test.go`,
  `fuzz_test.go`, and `testdata/decision_table.golden`.

Change:

- `internal/notify/README.md` (English, like the file): status line, section "Code" (`domain/` now
  exists), a new section "Decision per recipient" with the decision table of design note 3, the
  input table of design note 2 (which module or port supplies each fact, for M3-18, M3-19 and
  M3-23), the window rules of design note 7, the route rules of design note 8 and the rulings of
  design note 12. Keep the line `May use the ports of these modules: ...` and the section
  `## Tables` exactly as they are; M1-06 parses them.
- `services/server/CHANGELOG.md`: entry under "Unreleased", "Added".
- `docs/spec/13-push-notifications.md` (not core contract; English, like the chapter): in 2
  after the table that a message in a DM channel gives `dm` and never `message` or `mention`, also
  when it mentions r (design note 5); in 3.4 after point 3 the sentence for `start_local =
  end_local` (design note 6); in 4.4 that hints without thread (`security`, `admin`) know no "Active
  in the thread" (design note 5); in 4.5 that hints without thread open and use no window (design
  note 5), the window rules of design note 7 as new points 4 and 5, and in point 2 the unread
  count by the formula of 03 `read_states` instead of the shortened difference; in 4.6 that no
  route on any device gives no hint and no window (design note 8).
- `docs/DECISIONS.md`, section 10: rows `D-M3-17-1` to `D-M3-17-7` (design note 12).

No new top-level Makefile target.

## Out of scope

- Tables `notification_preferences` and `quiet_hours`, their migrations, the operations
  `listMyNotificationPreferences`, `putNotificationPreference`, `deleteNotificationPreference`,
  `listMyQuietHours`, `createQuietHoursRule`, `putQuietHoursRule`, `deleteQuietHoursRule`, the
  events `notification_pref.updated` and `quiet_hours.updated`, cleanup at archive, validation of
  `time_zone` names on write and the batched query of the three targets (13 12 point 2): M3-18,
  which proves the order of 13 3.1 against PostgreSQL through `ResolveLevel`.
- The worker consumer `push` (13 12, ADR 0051) and everything around one decision: which event
  gives which kind to which candidate (13 2 and 2.1: no `message` hint for agent messages during a
  run, exactly one `run_done` to the starter and `message` to others with `all`, nothing for
  edits, `silent`, `import` and `system`), candidates from `authz`, the `push.deliver` check before
  every send (04 7.6), automatic following (13 3.2), the window state per recipient and thread as
  a unique scheduled River job, the unread count, reading the configuration keys and the personal
  switches, loading time zones with `time.LoadLocation`, the job `notify.push_send` and the metric
  `bothy.push.skipped`: M3-19 (with AZ-M48 end to end, "ten messages in one window" and
  "exactly one `run_done`").
- APNs and FCM payloads, `interruption-level`, priorities, `time-sensitive` with
  `push.apns.time_sensitive_enabled`, `push.ttl`, `push.max_age`, the error tables: M3-20 and
  M3-21. The badge and the badge sync of 13 8 are not part of the decision.
- Push registrations, the preview token and endpoint, `notify.preview_mode`: M3-22.
- `notification.created` over the gateway, the presence port that supplies connections, focus and
  `idle` (13 12 point 4), the job of the desktop delay and its read check through
  `ReleaseDelayed`: M3-23 (M1-66 stores focus and `idle`).
- The delivery measurement of 13 9 and `perf.push_handoff_p95`: M3-24. Showing quiet hints on
  devices: M4 and M5.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-17-notify-decision`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(notify): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- This ticket has unit tests only: no listener, no container, no PostgreSQL, no network. Tests read only `docs/spec/13-push-notifications.md`, `docs/spec/03-data-model.md` and `docs/api/openapi.yaml` in the working tree, the package's own `testdata/` and its own source files (purity test).
- Production code in `internal/notify/domain` imports only standard library packages without I/O and `kernel-core` (`kernel/ids`, `kernel/errs`); `make arch` enforces the layer and `TestDomainPurity` the rest. No `context.Context`, no logger, no clock (`time.Now`, `time.Since`, `time.Until`), no time zone loading (`time.LoadLocation`, `time.LoadLocationFromTZData`), no randomness, no configuration access in production code of this package.
- No push provider, no device, no real time zone file outside the Go toolchain: tests load `Europe/Berlin` with `time.LoadLocation` after a blank import of `time/tzdata` in a test file (with a comment saying why), so they do not depend on the host's zone database.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. Purity and place. The decision lives in `internal/notify/domain` (13 12 point 3; 02 5: the
    domain layer holds pure functions and imports only `kernel-core`). Its inputs are value types
    of this package that mirror what the consumer of M3-19 gathers through ports; the domain imports
    no other module. The functions are deterministic, never mutate their input (copy slices before
    sorting) and take `Now` as an input. They never call `authorize`: the `push.deliver` result is
    the fact `Allowed`, gathered by the caller with current rights (13 4.2, 04 7.6, D-B7). They
    never load a time zone: each quiet-hour rule carries its `*time.Location`, which the caller
    loads from `quiet_hours.time_zone` or, when NULL, `users.time_zone`. They never read
    configuration: the caller passes the values of `notify.coalesce_window`,
    `notify.mobile_delay_when_desktop_active` and `notify.user_default_level`, and the personal
    switches already resolved with their defaults (`notify.default_admin_enabled`,
    `notify.default_input_needed_breaks_quiet_hours`).

2. Inputs. Signatures (names may follow the merged code, D-W-100; the fields are binding):

    ```go
    type Kind string        // NotificationKind: message, mention, dm, run_done, run_failed, input_needed, security, admin
    type Level string       // NotificationLevel: all, mentions, mute
    type Platform string    // 03 devices.platform: ios, android, macos, windows, web, cli
    type LocalTime int      // minutes after local midnight, 0 to 1439 (HH:MM of QuietHoursRule)

    type Preference struct { // one own row of notification_preferences
        Level      Level
        MutedUntil time.Time // zero when NULL
    }

    type LevelFacts struct { // sources of 13 3.1 for the thread of the hint
        Thread, Channel, Space *Preference // own rows; nil when absent
        ChannelDefault         Level       // channels.default_notification_level; "" when NULL
        DirectMessage          bool        // channels.kind = 'dm'
        PersonalDefault        Level       // user_settings['notify.default_level']; "" when unset
        InstanceDefault        Level       // value of notify.user_default_level; required
    }

    type Switches struct { // 13 3.3, resolved with their defaults by the caller
        AdminEnabled                bool // notify.admin_enabled
        InputNeededBreaksQuietHours bool // notify.input_needed_breaks_quiet_hours
    }

    type QuietRule struct { // one row of quiet_hours
        Weekdays uint8          // bit 0 = Monday, 1 to 127
        Start    LocalTime      // start_local
        End      LocalTime      // end_local; End <= Start crosses midnight
        Location *time.Location // zone of the row or users.time_zone; loaded by the caller
        Enabled  bool
    }

    type Connection struct { // the live gateway connection of a device (one per device, 04 6.1)
        Focus []ids.ID // threads of the last focus op
        Idle  bool     // idle of the last focus op
    }

    type Device struct { // an active device of the recipient (devices.state = 'active')
        ID            ids.ID
        Platform      Platform
        Connection    *Connection // nil when not connected
        Registrations []ids.ID    // active push registrations (disabled_at IS NULL)
    }

    type Window struct{ End time.Time } // window of (recipient, thread); zero: none; open while Now < End

    type Policy struct {
        CoalesceWindow time.Duration // notify.coalesce_window
        MobileDelay    time.Duration // notify.mobile_delay_when_desktop_active
    }

    type RecipientFacts struct { // one hint for one person r
        Kind       Kind
        Now        time.Time
        ThreadID   ids.ID      // zero for security and admin
        Allowed    bool        // push.deliver for r on the thread; for security and admin the recipient rule of 13 2
        Author     bool        // r wrote the message the hint points at
        RunStarter bool        // r is runs.triggered_by of the run the hint points at
        Level      LevelFacts  // kinds with thread only
        Switches   Switches
        QuietHours []QuietRule // every row of r, enabled or not
        Devices    []Device    // every active device of r
        Window     Window      // kinds with thread only
        Policy     Policy
    }
    ```

    Where each fact comes from (write this table into the README for M3-18, M3-19 and M3-23): `Kind`,
    `ThreadID`, `Author` and `RunStarter` from the event the consumer `push` reads (M3-19, 13 2);
    `Allowed` from `authorize(r, push.deliver, thread)` in M3-19 (13 4.2; it is false for a
    principal who is not an active human); the own rows of `LevelFacts` from
    `notification_preferences` (M3-18), `ChannelDefault` and `DirectMessage` from `chat`,
    `PersonalDefault` and the switches from `user_settings` through `sync`, `InstanceDefault` from
    `notify.user_default_level`; `QuietHours` from `quiet_hours` (M3-18) with the zone of the row or
    `users.time_zone` from `identity`; `Devices` from `identity` (active devices), `push_registrations`
    (M3-22) and the presence port of the gateway (M3-23); `Window` from the window state of M3-19;
    `Policy` from the configuration.

3. `Decide(in RecipientFacts) (Decision, error)`: the decision table of 13 4.1, evaluated in this
    order; the first row that decides wins:

    | Step | Condition | Outcome | `SkipReason` |
    |---|---|---|---|
    | validate | input invalid (note 10) | error, zero `Decision` | |
    | allowed | `Allowed` is false | `none` | `not_allowed` |
    | recipient | `message`, `mention` or `dm` and `Author`; `run_done`, `run_failed` or `input_needed` and not `RunStarter` | `none` | `not_recipient` |
    | level | kind with thread and the effective level does not allow it (table below) | `none` | `level` |
    | level | `admin` and not `Switches.AdminEnabled` | `none` | `admin_disabled` |
    | activity | kind with thread and a device is active in the thread (note 8) | `none` | `active_in_thread` |
    | quiet | quiet hours hold (note 6) and not (`input_needed` and `InputNeededBreaksQuietHours`) | sets `Quiet` for a send | |
    | window | `message` and the window is open (note 7) | `collect` | |
    | route | no device has a route (note 8) | `none` | `no_route` |
    | send | otherwise | `send` | |

    | Effective level | Kinds with thread it allows |
    |---|---|
    | `all` | `message`, `mention`, `dm`, `run_done`, `run_failed`, `input_needed` |
    | `mentions` | the same except `message` |
    | `mute` | only `input_needed` |

    `security` passes the level step at every level, `admin` only by its switch; both have no
    thread, so no level is resolved for them. Output (every field documented in code):

    ```go
    type Decision struct {
        Outcome          Outcome        // none, collect, send
        Skip             SkipReason     // only with none
        Kind             Kind
        Effective        EffectiveLevel // kinds with thread once the level step ran; zero before it and for security and admin
        Quiet            bool           // send only: deliver without sound (13 3.4)
        Count            int64          // send only: 1, or the unread count of a Flush
        OpensWindowUntil time.Time      // send only: end of the window this hint opens; zero otherwise
        Routes           []Route        // send only: in the order of Devices
    }

    type Route struct {
        DeviceID      ids.ID
        Via           Via       // gateway, push, push_delayed
        Registrations []ids.ID  // push and push_delayed: all active registrations of the device
        NotBefore     time.Time // push_delayed: Now + MobileDelay; zero otherwise
    }
    ```

    The skip reasons are internal Go constants of this package (`not_allowed`, `not_recipient`,
    `level`, `admin_disabled`, `active_in_thread`, `no_route`, and for `Flush` `nothing_collected`
    and `read`), not API values. Mapping them to the attribute values of `bothy.push.skipped`
    (`docs/observability/signals.md`) is M3-19's work; no skip reason of this package stands for
    quiet hours, because quiet hours make a hint quiet and never drop it (R-PN-05).

4. `ResolveLevel(f LevelFacts, now time.Time) (EffectiveLevel, error)` with
    `EffectiveLevel{Level Level; Source LevelSource; Muted bool}` (13 3.1; Ruling for
    docs/DECISIONS.md, `D-M3-17-3`). The first present source wins: `Thread`, `Channel`, `Space`
    (sources `thread`, `channel`, `space`), `ChannelDefault` (`channel_default`), `DirectMessage`
    gives `all` (`direct_message`), `PersonalDefault` (`personal_default`), `InstanceDefault`
    (`instance_default`). Only an own row has `muted_until`: if the chosen row's `MutedUntil` is
    strictly after `now`, the level is `mute` and `Muted` is true; once `now` reaches
    `MutedUntil` the row's own `level` applies again. A `muted_until` on a row that was not chosen has no effect
    ("of the selected row"). A row stored as `mute` stays `mute` after its `muted_until` passed.

5. Recipients and kinds without thread (13 2, 4.3; Ruling for docs/DECISIONS.md, `D-M3-17-2`). The
    domain does not classify events into kinds; it guards the recipient column of 13 2 so that a
    wrong caller fails closed: the author never gets `message`, `mention` or `dm` for their own
    message, and `run_done`, `run_failed` and `input_needed` reach only the starter of the run,
    which also makes "`input_needed` (own run)" of 13 4.3 hold. `security` and `admin` have no
    thread: no level, no activity in a thread and no bundling window apply to them (`ThreadID` and
    `Window` are ignored), but quiet hours do, because 13 3.4 names `input_needed` as the only
    exception. A `dm` hint requires `LevelFacts.DirectMessage`, and `message`, `mention` and the run
    kinds require a channel that is not a DM channel (13 2: a message in a DM channel is `dm`,
    also when it mentions r, because both kinds have the same level and go at once; 09 3.4: DM
    channels trigger no agent); a mismatch is invalid input. 05 4.9 defines `push.deliver` on a
    thread only, so for `security` and `admin` the fact `Allowed` is the recipient rule of 13 2 as
    the caller evaluates it at decision time (M3-19); the domain treats both cases alike.

6. Quiet hours (13 3.4; Ruling for docs/DECISIONS.md, `D-M3-17-4`). `InQuietHours(rules
    []QuietRule, now time.Time) (bool, error)` converts `now` into each enabled rule's `Location`
    and takes the weekday (Monday is bit 0) and the wall time in seconds after midnight; start and
    end are whole minutes. `Start < End`: quiet if the weekday's bit is set and `Start <= time <
    End`. `End <= Start`: quiet if the weekday's bit is set and `time >= Start`, or the previous
    weekday's bit is set and `time < End` (Monday's previous day is Sunday, bit 6). `Start == End`
    is therefore a window of 24 hours from `Start` on each set weekday, which is how screen D9
    "all day" is stored (`00:00` to `00:00`). Any matching enabled rule makes the person quiet;
    disabled rules are ignored; no rule means not quiet. No boundary is ever computed as an
    instant, so a rule inside the skipped hour of a spring changeover matches nothing that day and
    one inside the repeated hour of an autumn changeover matches twice. `ParseLocalTime` accepts
    exactly the `HH:MM` pattern of `QuietHoursRule` (`00:00` to `23:59`) and `LocalTime.String`
    writes it back. For a delayed push (note 8) the `Quiet` flag of the decision stays: the gateway
    copy of the same hint went out with it at once, and the delay is at most
    `notify.mobile_delay_when_desktop_active`.

7. Bundling and `Flush` (13 4.5; Ruling for docs/DECISIONS.md, `D-M3-17-5`). The window of (r,
    thread) is open while `Now` is before `Window.End`; at `Now == Window.End` it is closed.
    `Decide` for a kind with thread and no open window sends and sets `OpensWindowUntil = Now +
    CoalesceWindow` (the first hint opens the window, whatever its kind). `message` in an open
    window gives `collect` without routes. `mention`, `dm`, `input_needed`, `run_done` and
    `run_failed` in an open window send at once and leave the window unchanged (`OpensWindowUntil`
    zero); they replace the same slot on the device through the thread ID (13 6.1, adapters). With
    `CoalesceWindow` zero no window opens, so every hint goes at once. `Count` of an immediate hint
    is 1. `Flush(in RecipientFacts, s FlushState) (Decision, error)` with
    `FlushState{Collected int; Unread int64}` runs at the window end: `in.Kind` must be `message`;
    `Collected` zero gives `none` with `nothing_collected`; `Unread` (03 `read_states` "Unread",
    computed by the caller) zero or less gives `none` with `read`; otherwise the steps allowed,
    level, activity, quiet and route of note 3 run again at the flush time (rights, level and quiet
    hours may have changed; `Author` and `Window` are ignored, a bundle has no single author) and a
    send carries `Count = Unread` and opens the next window (`OpensWindowUntil = Now +
    CoalesceWindow`). A flush that sends nothing leaves the window closed. Reason: 13 1 point 3 "One
    notification per thread and window" holds under a steady stream of messages, and the first message
    after a quiet period still goes at once.

8. Activity and routes (13 4.4, 4.6; Ruling for docs/DECISIONS.md, `D-M3-17-6`). A device is
    active in the thread if it has a `Connection`, `Idle` is false and `Focus` contains `ThreadID`;
    any such device, of any platform, stops the hint on all devices. The desktop is active if a
    device with platform `macos` or `windows` has a `Connection` with `Idle` false. Routes, one per
    device in the order of `Devices`: a device with a `Connection` (idle or not) gets `gateway`; a
    device without one but with registrations gets `push_delayed` with `NotBefore = Now +
    MobileDelay` if it is `ios` or `android`, the desktop is active and `MobileDelay` is above zero,
    else `push`; a device with neither gets no route and is left out. `web` and `cli` follow the
    same rules (they have no registrations in practice). No route at all gives `none` with
    `no_route`, and such a decision opens no window.

9. `ReleaseDelayed(hintSeq, readSeq int64) bool` (13 4.4 "Desktop active"; part of `D-M3-17-6`):
    true sends the delayed push, false drops it. `hintSeq` is the `thread_seq` of the message the
    hint points at (for a flush hint `threads.last_message_seq` at the flush); `readSeq` is the read
    position of 03 `read_states` (the greater of `last_read_seq` and `threads.unread_floor_seq`).
    The push is dropped exactly when `readSeq >= hintSeq`. A hint without a message (`hintSeq` zero
    or less: `security`, `admin`, or a run hint without a message) is always released.

10. Invalid input (Ruling for docs/DECISIONS.md, `D-M3-17-7`). `ResolveLevel`, `InQuietHours`,
    `Decide` and `Flush` validate first and return an error made with
    `errs.New(errs.CodeInternalError, ...)` and the zero `Decision` (outcome `none`, no routes) for:
    an unknown `Kind`, `Level` or `Platform`; a zero `Now`; a negative `CoalesceWindow` or
    `MobileDelay`; a kind with thread and a zero `ThreadID`; `security` or `admin` with a non-zero
    `ThreadID`; an empty or unknown `InstanceDefault` for a kind with thread; the DM mismatch of
    note 5; a zero or duplicate device ID, a zero registration ID or a zero thread ID in a focus; a
    rule with `Weekdays` outside 1 to 127, a time outside 0 to 1439 or a nil `Location`; for
    `Flush` a kind other than `message` or a negative `Collected`. Fail closed: a broken input never
    yields `send` or `collect`. The error text names the field, never a value.

11. No content. `Decision` and `Route` hold only enum strings of this package, `ids.ID`, `bool`,
    `int64` and `time.Time`; no field can carry a name, a text or a token (ADR 0031, T-PU-01, the
    M3 red team item "content in push payloads"). `TestDecisionCarriesNoText` holds this.

12. Rulings for `docs/DECISIONS.md` section 10, one row each with reason and place:
    `D-M3-17-1` the decision is split into `ResolveLevel`, `InQuietHours`, `Decide`, `Flush` and
    `ReleaseDelayed`, takes facts and `Now` in and never calls `authorize`, reads a clock, loads a
    zone or reads configuration (note 1); `D-M3-17-2` recipient guards and kinds without thread
    (note 5); `D-M3-17-3` `muted_until` strictly after now and only on the chosen row (note 4);
    `D-M3-17-4` quiet-hour details: minute times, second precision, `start_local = end_local` as 24
    hours, zone loaded by the caller, quiet flag kept for delayed pushes (note 6); `D-M3-17-5`
    window rules and `Flush` (note 7); `D-M3-17-6` activity, routes and `ReleaseDelayed` (notes 8
    and 9); `D-M3-17-7` invalid input (note 10). Rulings 2, 4 (24 hours), 5 and 6 (no route) also go
    into 13 (Scope, "Change").

## Tests (write them first)

Unit tests in `internal/notify/domain` (PLAN 5, row "Unit"). The recipient is Alex; Sam wrote the
message or started the run in the "not the recipient" cases; Robin gets the `admin` hints. Alex's
active devices are named by platform: `phone` (`ios`, one registration), `tablet` (`android`, one
registration), `desktop` (`macos`), `laptop` (`windows`) and `console` (`web`). Durations in tests
are fixture values of `Policy`, not the configuration defaults.

- `TestResolveLevel_Order` (13 3.1): one case per step, each removing the sources above it:
  thread row, channel row, space row, channel default, DM channel gives `all`, personal default,
  instance default; asserts `Level` and `Source`; a DM channel with a channel default uses the
  default (step 4 before 5).
- `TestResolveLevel_MutedUntil`: chosen thread row `all` with `MutedUntil` one second after now
  gives `mute` and `Muted`; equal to now gives `all`; in the past gives `all`; a space row muted
  into the future under a thread row `mentions` gives `mentions`, not muted; a row stored as `mute`
  with `MutedUntil` in the past stays `mute`.
- `TestInQuietHours_EuropeBerlinChangeoverDays` (PLAN row, 13 14): rule every day (`127`), `22:00`
  to `07:00`, `Europe/Berlin`. Spring changeover (Sunday 2026-03-29, `02:00` CET becomes `03:00`
  CEST): `2026-03-28T20:59:59Z` (Saturday 21:59:59) not quiet, `2026-03-28T21:00:00Z` (22:00)
  quiet, `2026-03-29T00:59:59Z` (01:59:59 CET) quiet, `2026-03-29T01:00:00Z` (03:00 CEST) quiet,
  `2026-03-29T04:59:59Z` (06:59:59) quiet, `2026-03-29T05:00:00Z` (07:00) not quiet. Autumn
  changeover (Sunday 2026-10-25, `03:00` CEST becomes `02:00` CET): `2026-10-24T19:59:59Z` not
  quiet, `2026-10-24T20:00:00Z` quiet, `2026-10-25T00:30:00Z` (02:30 CEST) quiet,
  `2026-10-25T01:30:00Z` (02:30 CET) quiet, `2026-10-25T05:59:59Z` (06:59:59 CET) quiet,
  `2026-10-25T06:00:00Z` (07:00 CET) not quiet. Counting every minute from 12:00 UTC on the
  Saturday to 12:00 UTC on the Sunday gives 480 quiet minutes in spring and 600 in autumn.
- `TestInQuietHours_RuleInsideChangeoverHour`: rule Sunday only (`64`), `02:15` to `02:45`,
  `Europe/Berlin`: no minute of the UTC day 2026-03-29 is quiet; on the UTC day 2026-10-25 exactly
  60 minutes are quiet, among them `00:30:00Z` and `01:30:00Z`, not `00:45:00Z` and not
  `01:14:59Z`.
- `TestInQuietHours_AcrossMidnightAndWeekdayBits` (13 14 "across midnight, weekday bits"), all
  rules in `Europe/Berlin`: rule Friday only (`16`), `22:00` to `07:00`: `2026-10-23T21:30:00Z` (Friday 23:30) quiet,
  `2026-10-24T04:30:00Z` (Saturday 06:30) quiet, `2026-10-24T05:00:00Z` (Saturday 07:00) not,
  `2026-10-24T21:00:00Z` (Saturday 23:00) not, `2026-10-23T04:30:00Z` (Friday 06:30) not; rule
  Sunday only (`64`), `22:00` to `07:00`: `2026-10-26T04:30:00Z` (Monday 05:30) quiet,
  `2026-10-27T04:30:00Z` (Tuesday 05:30) not; rule Monday to Friday (`31`), `12:00` to `13:00`:
  `2026-10-26T11:00:00Z` quiet, `2026-10-26T11:59:59Z` quiet, `2026-10-26T12:00:00Z` not,
  `2026-10-24T10:30:00Z` (Saturday) not.
- `TestInQuietHours_WholeDayZonesAndDisabled`: rule Saturday and Sunday (`96`), `00:00` to `00:00`:
  `2026-10-23T21:59:59Z` not, `2026-10-23T22:00:00Z` (Saturday 00:00) quiet,
  `2026-10-25T22:59:59Z` (Sunday 23:59:59) quiet, `2026-10-25T23:00:00Z` (Monday 00:00) not; the
  same rule in `UTC` and in `Europe/Berlin` differs at `2026-10-23T22:30:00Z`; a disabled matching
  rule is ignored; no rules gives false.
- `TestParseLocalTime`: `00:00`, `07:00`, `23:59` parse and print back; `24:00`, `7:00`, `07:60`,
  `07:00:00` and the empty string are errors.
- `TestDecide_InputNeededMutedQuietHours` (PLAN row, 13 14 "`input_needed` with `mute`"): Alex
  started the run; level `mute` stored on the thread, and separately `all` with `MutedUntil` in the
  future; in quiet hours: `send`, `Quiet` true, routes present; with
  `InputNeededBreaksQuietHours`: `Quiet` false; outside quiet hours: `Quiet` false; the same hint for
  Sam, who did not start the run: `none` with `not_recipient`.
- `TestDecide_AZM48_NotAllowed` (mirrors AZ-M48 at decision level): a `mention` with `Allowed`
  false gives `none` with `not_allowed`, no routes, no window, no effective level; the same for
  every other kind.
- `TestDecide_Recipients` (13 2): `Author` with `message`, `mention`, `dm` gives `not_recipient`;
  not `RunStarter` with `run_done`, `run_failed`, `input_needed` gives `not_recipient`; `security`
  and `admin` ignore both flags.
- `TestDecide_LevelAndKind` (13 4.3): each kind with thread at each effective level against the
  table of note 3; `security` at every level sends; `admin` sends at every level with the switch
  and gives `admin_disabled` without it.
- `TestDecide_ActiveInThread` (13 4.4, mirrors the M3-23 case "Thread in focus on the desktop"):
  the desktop focusing the thread, not idle, gives `none` with `active_in_thread`; the phone
  focusing it does the same; an idle focus or a focus on another thread does not; `security` and
  `admin` are never `active_in_thread`; a `message` muted and active gives `level` (order).
- `TestDecide_Routes` (13 4.4, 4.6, mirrors the M3-23 case "Desktop active"): connected devices get
  `gateway` also when idle; with the desktop or the laptop connected and not idle, the phone and
  the tablet without connection get `push_delayed` with their registrations and `NotBefore = Now +
  MobileDelay`; an idle desktop delays nothing; `MobileDelay` zero gives `push`; the console with a
  connection gets `gateway`; a device with neither connection nor registration is left out; no
  route at all gives `none` with `no_route` and no window; routes keep the order of `Devices`.
- `TestDecide_Window` (13 4.5): no window: `send` with `OpensWindowUntil = Now + CoalesceWindow`;
  `message` in an open window: `collect`, no routes; each of `mention`, `dm`, `input_needed`,
  `run_done`, `run_failed` in an open window: `send`, `OpensWindowUntil` zero; `Now ==
  Window.End`: closed, `send` and opens; `security` and `admin` with a `Window.End` in the future:
  `send`, no window; `CoalesceWindow` zero: never a window.
- `TestDecide_QuietHours`: every kind in quiet hours is `Quiet` except `input_needed` with the
  switch; the switch changes nothing for other kinds; outside quiet hours nothing is `Quiet`.
- `TestDecide_InvalidInput` (note 10): each listed case returns an error `err` for which
  `e, ok := errs.As(err)` gives `ok` true and `e.Code == errs.CodeInternalError`, and the zero
  `Decision` (outcome `none`, no routes); the same for `Flush`, `ResolveLevel` and `InQuietHours` with their cases.
- `TestFlush` (note 7): `Collected` zero gives `nothing_collected`; `Unread` zero gives `read`;
  `Collected` 3 and `Unread` 4 gives `send` with `Count` 4, routes and `OpensWindowUntil = Now +
  CoalesceWindow`, `Quiet` by the quiet hours at the flush time; the thread level changed to
  `mentions` gives `level`; a device active in the thread gives `active_in_thread`; `Allowed` false
  gives `not_allowed`; `Author` true is ignored; kind `mention` is an error.
- `TestFlush_TenMessagesInOneWindow` (mirrors the M3-19 case of 13 14 "Ten messages"): the
  test keeps the window state as a caller would; ten `message` hints at `Now` plus 0 to 45 seconds
  with `CoalesceWindow` one minute give one `send` and nine `collect`; `Flush` at the window end
  with `Collected` 9 and `Unread` 10 gives the second and last `send` with `Count` 10.
- `TestReleaseDelayed`: `hintSeq` 5 with `readSeq` 4 releases, with 5 and 6 drops; `hintSeq` 0
  releases.
- `TestDecisionTableGolden` (PLAN row "golden table test of all combinations", 13 14 row
  "Decision table"), in `table_test.go`: enumerates every combination of kind (8 values in
  the order of `Kinds`), stored level of a thread row (`all`, `mentions`, `mute`), `muted_until` of
  that row (`n`: NULL, `y`: one hour after `Now`), quiet hours (`n`: no rule, `y`: one enabled rule
  every day `22:00` to `07:00` in `Europe/Berlin`), `InputNeededBreaksQuietHours`, `AdminEnabled`,
  active in thread (the phone connected, focus on the thread, not idle), desktop active (the
  desktop connected, focus on another thread, not idle) and window open (`Window.End` 30 seconds
  after `Now`), each `n` before `y`. Fixed: `Now = 2026-10-23T21:30:00Z` (Friday 23:30 in Berlin),
  `Allowed` true, `Author` false, `RunStarter` true, `DirectMessage` exactly for `dm`, devices
  `phone` (`ios`, one registration) and `desktop` (`macos`, no registration), `CoalesceWindow` one
  minute, `MobileDelay` 30 seconds, `ThreadID` zero for `security` and `admin`. With
  `active_in_thread=n` the phone has no connection, with `desktop_active=n` the desktop has none,
  and with `window_open=n` `Window` is zero; `InstanceDefault` is `mentions`, and the thread row
  is always present, so the stored level is the chosen row. One line per combination in
  `testdata/decision_table.golden`, after one header comment line:

    ```text
    kind=<k> level=<l> muted=<y|n> quiet_hours=<y|n> breaks_quiet=<y|n> admin_enabled=<y|n> active_in_thread=<y|n> desktop_active=<y|n> window_open=<y|n> -> outcome=<o> skip=<r|-> effective=<l|-> quiet=<y|n> count=<n> window_until=<+Ns|-> routes=<device:via[@+Ns],...|->
    ```

    The test fails on any difference and names the first differing line; `go test
    ./internal/notify/domain -run TestDecisionTableGolden -update` rewrites the file, and the diff
    is reviewed. Each line is also checked against an oracle written in the test from 13 2, 3.1,
    3.4, 4.3, 4.4, 4.5 and 4.6 as plain boolean expressions (no call into production helpers), so
    a wrong golden file cannot pass.

- `TestKindsLevelsPlatformsMatchContract`, in `contract_test.go`: walks up from the package
  directory to the directory that contains `docs/spec`, then asserts that `Kinds()` equals the
  `enum` of `NotificationKind` in `docs/api/openapi.yaml` and the first column of the table in 13
  "2. Kinds of notifications and recipients", `Levels()` the `enum` of `NotificationLevel`, and
  `Platforms()` the `CHECK (platform IN (...))` list in the table `devices` of
  `docs/spec/03-data-model.md` (not the one of the table `hosts`). A line scan is enough: the
  `enum:` line of the schema block in `openapi.yaml`, the table rows and the check text in the
  chapters; no YAML library. A new value in one of them fails this test until the domain follows.
- `TestDomainPurity`, in `purity_test.go`: parses the non-test files of the package with `go/parser`
  and asserts that imports are only `time`, `slices`, `cmp`, `math`, `strconv`, `strings`, `fmt`,
  `errors` and the module's `internal/kernel/ids` and `internal/kernel/errs`, and that no call of
  `time.Now`, `time.Since`, `time.Until`, `time.Sleep`, `time.After`, `time.Tick`,
  `time.NewTimer`, `time.LoadLocation` or `time.LoadLocationFromTZData` exists.
- `TestDecisionCarriesNoText` (note 11): walks the fields of `Decision`, `Route` and
  `EffectiveLevel` with `reflect` and fails on any field of kind `string` that is not one of the
  package's enum types, on any map and on any interface.
- `FuzzDecide` (seed corpus from the table cases; runs in `go test`): builds `RecipientFacts` from
  the fuzz input (kind, levels, flags, minute offsets within the week of the golden `Now`, one
  quiet rule, up to four devices), skips inputs for which `Decide` returns an error, and asserts:
  `none` exactly when `Skip` is set; `send` always has routes and `Count` 1; `collect` only for
  `message` in an open window and without routes; an effective `mute` sends no kind with thread
  except `input_needed`; `mentions` never sends or collects `message`; `Quiet` only when
  `InQuietHours` is true and never for `input_needed` with the switch; a device active in the
  thread means `none` for kinds with thread; `push_delayed` only for `ios` and `android` without
  connection while a desktop is active; `OpensWindowUntil` only for kinds with thread without an
  open window; two calls give equal results and the input is unchanged.

No integration, contract or end-to-end test: this ticket has no I/O. M3-18, M3-19 and M3-23 add the
PostgreSQL and gateway tests when they apply the decision.

Run `FuzzDecide` once locally with `go test -run=^$ -fuzz=FuzzDecide -fuzztime=<duration of your
choice> ./internal/notify/domain` and report duration and result; commit any crashing input as a
seed after the fix.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/notify/...`
- `make arch`, `make lint`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `internal/notify/domain` exists with `ResolveLevel`, `InQuietHours`, `Decide`, `Flush` and
      `ReleaseDelayed`, a doc comment on every public symbol, and imports only the standard library
      without I/O and `kernel-core`; `make arch`, `make lint` and `TestDomainPurity` pass.
- [ ] `TestDecisionTableGolden` covers every combination of kind, stored level, `muted_until`,
      quiet hours, both personal switches, activity in the thread, active desktop and open
      window; the golden file matches and every line agrees with the oracle.
- [ ] Quiet hours in `Europe/Berlin` pass on both changeover days (including the rule inside the
      changeover hour) and across midnight with weekday bits.
- [ ] `input_needed` with `mute` (stored or by `muted_until`) is sent quietly in quiet hours and
      normally with `notify.input_needed_breaks_quiet_hours`.
- [ ] The effective level follows 13 3.1 with `muted_until`; level and kind follow 13 4.3; the
      recipient guards, activity in the thread, routes with the desktop delay, the window rules,
      `Flush` and `ReleaseDelayed` behave as in design notes 4 to 9.
- [ ] Kinds, levels and platforms equal `openapi.yaml`, 13 and 03
      (`TestKindsLevelsPlatformsMatchContract`); the decision carries no text.
- [ ] Invalid input never yields `send` or `collect`; the `FuzzDecide` seed corpus passes.
- [ ] The notify README holds the decision table, the input sources, the window and route rules;
      13 2, 3.4, 4.4, 4.5 and 4.6 carry the rulings; `D-M3-17-1` to `D-M3-17-7` are in
      `docs/DECISIONS.md`; the CHANGELOG has the entry.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `notify` updated (status, code layout, the decision table of note 3, the input sources of note 2, the window rules of note 7, the route rules of note 8).
- Rulings of note 12 recorded in `docs/DECISIONS.md` (one row each with reason); rulings 2, 4, 5 and 6 also in `docs/spec/13-push-notifications.md`.
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
