# M4-33: Channel menu: notification level and export

Status: ready | Depends on: M4-20, M4-30, M3-11 | Milestone: M4

## Goal

After this ticket the desktop app lets a person decide how loud each place is and take a channel's history with them: the menu of a space, a channel and a thread (in the sidebar, the post list and the thread header) sets the notification level `all`, `mentions` or `mute`, optionally muted until a time, or "inherit", and shows the inherited level with its source; the notification settings page D9 shows the personal default (`notify.default_level`), the preview mode (`notify.preview_mode`), the quiet hours rules with time zone and "all day", the switch `notify.input_needed_breaks_quiet_hours`, `notify.admin_enabled` for owners, and the tree of spaces, channels and followed threads with their levels and visible inheritance; and the channel menu offers "Export" only where the server gives the capability `export`, asks for step-up, and saves the streamed history of a chosen time range as a file. The effective level follows 13 3.1 exactly, computed for display only (the server decides every notification). Why: a person who cannot quiet a busy channel mutes the whole app, and an export must never be possible without the capability and a fresh confirmation (07 2.4, 05).

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-19, M4-25, M4-30, M4-33, M4-34; section 9 rows M3-11, M3-18.
- `docs/spec/01-product.md`: 5.1 D9 "Notification settings", 6.7, 10 "Notifications" (10.1 table), 11 (note on `notify.*`).
- `docs/spec/13-push-notifications.md`: 3 "Settings" (3.1 "Effective level", 3.2 "Automatic following" shown as "Following", 3.3 personal switches, 3.4 quiet hours with the "all day" rule of D-M3-17-4), 17 (defaults).
- `docs/spec/03-data-model.md`: `notification_preferences` (`muted_until`), `quiet_hours` (bit mask, `start_local`, `end_local`, `time_zone`, `enabled`), `channels.default_notification_level`, `user_settings`.
- `docs/spec/04-sync-protocol.md` 14 (`notification_pref.updated`, `quiet_hours.updated`: last write wins per row).
- `docs/spec/07-permissions.md` 2.4 (capability `export`) and `docs/spec/05-actions.md` (`export.channel` with step-up and audit).
- `docs/spec/12-files.md` and the export section the M3-11 brief names (format and range of the stream).
- `docs/api/openapi.yaml`: `listMyNotificationPreferences`, `putNotificationPreference` (`NotificationPreferencePut`), `deleteNotificationPreference`, `listMyQuietHours`, `createQuietHoursRule`, `putQuietHoursRule`, `deleteQuietHoursRule`, `listMySettings`, `putMySetting`, `exportChannel` (range, NDJSON of `Message`), `getChannel` (`ChannelView.capabilities`).
- `docs/DECISIONS.md`: D-M3-17-4, D-W-100, D-W-114, D-W-116 (this ticket depends on M4-20 for the thread header menu and on M4-30 for the notification stream, the fake's notification decision and the settings entry of M4-25); section 10.
- Briefs: `docs/briefs/M4-19-sidebar-post-list.md` (sidebar rows, post list, menus), `docs/briefs/M4-20-thread-view.md` (thread header), `docs/briefs/M4-25-settings-devices.md` (the Notifications entry `notifications_entry.dart`, the `StepUpDialog`), `docs/briefs/M4-30-windows-tray-notifications.md` (`SyncEngine.notifications`, `FakeWorld.notify`), `docs/briefs/M4-12-local-store.md` (`notification_preferences`, `quiet_hours`, the `level` and `capabilities` columns), `docs/briefs/M4-15-sync-outbox.md` (executors); for the server side `docs/briefs/M3-11-channel-export.md`, `docs/briefs/M3-18-notify-preferences.md`.

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. No permission logic: "Export" appears only when the channel's `capabilities` from the server contain `export`; the step-up is the server's demand (15 1).
10. No new personal setting key; only the `notify.*` keys of 13 3.3. Specification first: D9 shows a row "Runs I triggered" that 13 3.3 has no key for; this ticket shows it as an explanation of 01 10.1 (questions always, done and failed by level) and says so in 01 D9 in the same change.

Product requirements implemented here:

11. Level per space, channel and thread: `all`, `mentions`, `mute`, with an optional `muted_until`; "inherit" deletes the own row (13 3.1).
12. The displayed effective level follows the order of 13 3.1 (thread, channel, space, channel default, DM, personal default, instance default) and `muted_until`; the source of an inherited level is shown ("inherits: All").
13. Threads that the server follows automatically show "Following" (13 3.2).
14. Personal switches `notify.default_level`, `notify.preview_mode`, `notify.input_needed_breaks_quiet_hours`, `notify.admin_enabled` (owners) (13 3.3).
15. Quiet hours rules with weekdays, start, end, time zone (default the person's), enabled; "all day" stores `00:00` to `00:00` (13 3.4, D-M3-17-4).
16. Every change goes through the outbox and syncs to the person's other devices (04 14).
17. Export only with the capability `export` and step-up; the client saves the stream unchanged for the chosen range and shows progress; nothing of it stays in the app.

## Scope

Create in `apps/client/lib/`:

- `notifications_settings/` (`notification_settings_page.dart` D9, `level_tree.dart` with inheritance, `quiet_hours_editor.dart`, `effective_level.dart` pure function of 13 3.1 for display, `providers.dart`).
- `menus/` (`space_menu.dart`, `channel_menu.dart`, `thread_menu.dart` with the level submenu and "Mute until", and "Export" in the channel menu), `export/channel_export.dart` (range dialog, save, progress, Design note 3).
- `sync/notify_ops.dart`: outbox executors for the preference and quiet hours operations, registered in the bootstrap.
- Fake server handlers through the registry of M4-17 for the operations above, with automatic following on a write in a thread and with an export stream.
- Tests under `test/notifications_settings/`, `test/menus/`, `test/export/`, goldens, integration tests under `integration_test/` (Tests).
- ARB keys for every new text in `packages/l10n`.

Change:

- The sidebar rows and post rows of M4-19 and the thread header of M4-20, only to open the menus; `apps/client/lib/settings/notifications_entry.dart` of M4-25 to open the page D9.
- `docs/spec/01-product.md` 5.1 D9 (the row "Runs I triggered" as an explanation).
- `apps/client/README.md`; `docs/DECISIONS.md` section 10: rows `D-M4-33-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The decision who is notified (server, M3) and showing notifications (M4-30, M5).
- Channel settings for operators (default level of a channel: administration in the console, M6).
- Exports of other kinds (audit export M4-29; personal data export M7).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-33-channel-menu`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Effective level for display.** `effectiveLevel({threadPref, channelPref, spacePref, channelDefault, isDm, personalDefault, instanceDefault, now})` returns the level and its source, applying `muted_until` of the selected row; the instance default comes from the server where the API gives it (`getInstance` or the snapshot) and otherwise from the reference default of `notify.user_default_level` with a note in the report. It decides nothing about delivery; it labels the tree and the menus.

2. **Menus.** Level submenu with the three levels, "Mute until" with fixed choices (1 hour, until tomorrow 08:00 in the person's time zone, until a chosen date) plus "Inherit"; changes as `putNotificationPreference` or `deleteNotificationPreference` through the outbox; the local row updates at once.

3. **Export.** Visible only with `export` in the channel's `capabilities`; the dialog asks for the range (from, to) and a file path; the request runs through the step-up interceptor; the response streams to the file unchanged with a byte progress; cancel aborts the stream and deletes the partial file; nothing is cached. `403` and `404` show the server's reason.

4. **Rulings for docs/DECISIONS.md** (`D-M4-33-<n>`): display-only effective level and the source of the instance default (note 1); the "Mute until" choices (note 2); export streaming without cache (note 3); the D9 row explanation (Rules).

## Tests (write them first)

Widget and unit tests:

- `test/notifications_settings/effective_level_test.dart`: every step of the order of 13 3.1 with and without `muted_until`, DM channels, personal and instance defaults; the source label.
- `test/notifications_settings/quiet_hours_test.dart`: weekdays mask, across midnight, "all day" as `00:00` to `00:00`, time zone default.
- `test/menus/level_menu_test.dart`: setting, muting until, inheriting; "Following" for an automatically followed thread.
- `test/export/channel_export_test.dart`: hidden without `export`; step-up asked; the file equals the stream; cancel deletes the partial file.
- Goldens of D9 and the menus; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Integration tests (`integration_test/`, against `FakeServer`):

- `notification_level_test.dart`: the level takes effect according to 13 "Effective level": Alex mutes a channel, `FakeWorld.notify` of M4-30 (which follows 13 3.1) sends no `notification.created` for a message there, then Alex sets the thread to `all` and the next message notifies; the change appears on a second simulated device.
- `export_test.dart`: export with capability and step-up saves the file; without the capability the menu has no entry.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Integration tests: the level takes effect according to 13 "Effective level"; export only with capability `export` and step-up.
- [ ] Levels per space, channel and thread with mute-until and inheritance; D9 with the personal switches and quiet hours.
- [ ] Changes sync across devices through the outbox; 01 D9 updated; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 13.
- `apps/client/README.md` (English) as in Scope.
- 01 D9 updated; rulings of Design note 4 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the integration tests ran, the source of the instance default level, and the export format found on the base branch.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
