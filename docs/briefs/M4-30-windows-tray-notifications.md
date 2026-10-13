# M4-30: Windows: notification area and local notifications

Status: ready | Depends on: M4-25, M3-23 | Milestone: M4

## Goal

After this ticket the Windows app keeps running in the notification area when its window is closed, keeps its gateway connection there, and shows local notifications from the server's `notification.created` events (13 5.3, 15 10.3): one toast per notification with title "<Channel> · <Thread title>" and text "<Author>: <excerpt>" from the event's preview fields (or the placeholder with the locally known channel name when the preview mode gives none), quiet notifications without sound, the slot of a thread replaced by the next notification of that thread, a click that opens the thread at the message through the router, and nothing while the thread is open and focused. A closed app gets no notifications, and the settings say so; "Start at login" is a device setting, off by default. The same presenter shows `notification.created` on macOS while the app is connected (13 5.3). The tray icon has Open and Quit, and quitting ends the connection. Why: Windows has no push in the M milestones (R-Push); the notification area is the only way a Windows person learns that an agent needs them while the window is closed.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-18, M4-25, M4-30, M4-34; section 9 row M3-23.
- `docs/spec/01-product.md`: 10 "Notifications" (10.1 table and quiet hours, 10.2 content and placeholder, 10.3 row Windows and row "all": no notification for the open thread in the foreground).
- `docs/spec/13-push-notifications.md`: 2 (kinds), 3.4 (quiet hours: silent), 4.4 "Active on a device" (`idle`), 4.5 (coalescing: one slot per thread), 4.6 "Delivery path per device", 5.3 "Windows and desktop through the gateway", 6.3 "Gateway event `notification.created`" (payload `nid`, `kind`, `thread_id`, `message_id`, `quiet`, `count` and the preview fields of 6.4), 6.4 (preview fields and `notify.preview_mode`), 8 (badge), 15 (R-PN-01).
- `docs/spec/15-client-architecture.md`: 2 (Windows 10 2004 for package identity), 10 (table rows macOS and Windows), 10.3 "Windows" (`tray_manager`, `window_manager`, `flutter_local_notifications`, package identity from MSIX, start at login), 11 (deep link paths), 21 (R-CL-12).
- `docs/spec/04-sync-protocol.md` 6.2 (`focus` with `idle`) and 2.1 (ephemeral events).
- `docs/api/openapi.yaml` and `docs/api/asyncapi.yaml`: the payload schema of `notification.created`.
- `docs/DECISIONS.md`: D-M3-17-2, D-M3-17-6, D-W-100, D-W-114, D-W-116 (this ticket depends on M4-25 for the settings entry); section 10.
- Briefs: `docs/briefs/M4-18-desktop-shell.md` (bootstrap, router, platform folder `windows/`), `docs/briefs/M4-25-settings-devices.md` (settings sections), `docs/briefs/M4-14-sync-catch-up.md` (ephemeral events through the dispatcher), `docs/briefs/M4-13-sync-connection.md` (`FocusManager` with `idle`); for the server side `docs/briefs/M3-23-desktop-notices.md`.

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
9. No new configuration key and no new personal setting key: "Start at login" is a device setting kept locally (01 11 has no key for it; 15 10.3 calls it a setting).
10. Specification first: 13 4.4 says 15 defines when a client reports `idle`; 15 does not yet; this ticket adds the definition of Design note 3 to 15 10 in the same change.

Product requirements implemented here:

11. Closing the window on Windows hides it to the notification area; the app keeps its gateway connection there; "Quit" in the tray menu ends the app and the connection (15 10.3).
12. Local notifications only from `notification.created` (R-PN-01); the client decides nothing about who is notified.
13. Content: title "<Channel> · <Thread title>", text "<Author>: <excerpt>" from the preview fields; with `notify.preview_mode = none` or no preview fields, the placeholder "New message" with the channel name known locally (01 10.2).
14. `quiet: true` shows the toast without sound (quiet hours, 13 3.4); `count` replaces the thread's previous toast with the unread count (13 4.5).
15. No toast for the thread that is open and focused in the foreground (01 10.3 row "all").
16. A click opens the thread at the message (15 11).
17. A closed app gets no notifications; the settings say so (13 5.3, 01 10.3).
18. "Start at login" is off by default and kept per device.

## Scope

Create in `apps/client/` (and in `packages/sync_engine` where named):

- `lib/notifications/notification_presenter.dart` (port and the `flutter_local_notifications` implementation for Windows and macOS, Design note 2), `lib/notifications/notification_router.dart` (from an event to a toast or nothing), `lib/notifications/toast_content.dart` (pure mapping of the payload to title and text).
- `lib/desktop/tray.dart` (tray icon, menu Open and Quit, `tray_manager`), `lib/desktop/window_behavior.dart` (close hides on Windows, `window_manager`), `lib/desktop/start_at_login.dart` (device setting, Design note 4).
- `windows/runner/` changes for the tray icon resource and the activation of a toast click (forwarded to the router like a deep link of M4-18).
- `lib/settings/notifications_note.dart`: the sentence "Notifications arrive only while the app runs" and the "Start at login" switch, shown in the Notifications entry of the settings of M4-25 (`lib/settings/notifications_entry.dart` changes to host them).
- In `packages/sync_engine`: the dispatcher of M4-14 forwards the ephemeral `notification.created` (payload of 13 6.3) to a stream `SyncEngine.notifications` without storing it (it has no local table); and the fake of M4-17 gains `FakeWorld.notify(...)`, which decides per recipient with the effective level of 13 3.1 (thread, channel, space, channel default, personal default, instance default, `muted_until`) and the kinds of 01 10.1, then emits `notification.created` to the connected devices of the recipient by the 13 4.6 rule (connected device: event; no connection: nothing); M4-33 uses the same decision. Tests for both in `packages/sync_engine/test/`.
- Tests under `test/notifications/` and `test/desktop/`, integration tests under `integration_test/` (Tests).
- ARB keys for every new text in `packages/l10n`.

Change:

- `apps/client/pubspec.yaml` (`tray_manager`, `window_manager`, `flutter_local_notifications`, `launch_at_startup` or the package the license gate allows; only if `make licenses` and `make osv` stay green), `apps/client/README.md` (behaviour in the notification area, limits without package identity).
- `docs/spec/15-client-architecture.md` 10 (the `idle` definition, Design note 3; what works without package identity until MSIX in M7).
- `docs/DECISIONS.md` section 10: rows `D-M4-30-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- MSIX packaging, package identity, signing and the `.appinstaller` (M7); WNS (13 5.4: later).
- APNs and the Notification Service Extension on macOS (M5); notification settings (M4-33).
- Badge numbers on the taskbar (13 8 sends badges only to mobile devices).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-30-windows-tray-notifications`; one writer per worktree.
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

1. **Router.** `NotificationRouter.onEvent(payload)`: drop when the thread of the payload is open and the window is focused and not `idle` (the server normally does not send then, 13 4.4; this guards a race); otherwise map with `toast_content.dart` and call the presenter with a tag per thread (the replacement slot) and `silent = quiet`. Kinds without a thread (`security`, `admin`) use a fixed tag per kind and open the settings or administration route.

2. **Presenter.** `NotificationPresenter.show(ToastContent c, {required String tag, required bool silent})` and `cancelThread(tag)`; the `flutter_local_notifications` implementation initializes with the app name from the build branding of M4-18 and, on Windows, with the AUMID and GUID the package needs for unpackaged apps; features that need package identity (`cancel`, `getActiveNotifications`) are used only when available and otherwise skipped with a log line without content. Clicking a toast delivers the payload (`thread_id`, `message_id`) to the router.

3. **Idle.** The app reports `idle: true` in `focus` while its window is hidden to the notification area, minimized, or not the foreground window of the system, and `idle: false` as soon as it is in the foreground (no timer and no borrowed key). Record it in 15 10 (13 4.4 delegates the definition to 15) and as a ruling.

4. **Start at login.** A device setting stored with the platform package (registry run key for the current user on Windows, a login item on macOS), off by default; never in `user_settings`.

5. **Rulings for docs/DECISIONS.md** (`D-M4-30-<n>`): the guard against the open thread (note 1); behaviour without package identity (note 2); the `idle` definition (note 3); start at login per device (note 4); the macOS use of the presenter while connected (Goal, 13 5.3).

## Tests (write them first)

Unit and widget tests:

- `test/notifications/toast_content_test.dart`: title and text from the preview fields; the placeholder with the local channel name when the preview is missing; `count` in the text; no content in logs.
- `test/notifications/router_test.dart`: open and focused thread gives no toast; a quiet notification is silent; the same thread replaces its slot; a click navigates to the thread at the message.
- `test/desktop/window_behavior_test.dart`: closing hides on Windows and keeps the engine connected; Quit disposes the engine.
- `test/desktop/idle_test.dart`: `focus` carries `idle: true` when hidden, minimized or in the background, and `false` in the foreground.

Integration tests (`integration_test/notifications_windows_test.dart`, against `FakeServer`, on Windows; the macOS run checks the presenter path only):

- With the app running and the window hidden to the notification area, a message from Sam in a channel of Alex makes the fake send `notification.created`, and the presenter shows one toast with the expected title and text (a recording wrapper around the real presenter).
- After Quit, the fake sees no connection for the device and creates nothing (13 4.6 item 3); after a restart the unread counters show the message.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on Windows and on macOS: `make apps-platform`; elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`, `make osv`
- `make check`

## Acceptance criteria

- [ ] Integration test on Windows: a notification while the app is running, none when it has quit (R-Push).
- [ ] The window closes to the notification area and the connection stays; Quit ends both.
- [ ] Toast content follows 01 10.2 with the placeholder; quiet, replacement per thread and the open-thread rule hold.
- [ ] 15 10 defines `idle`; "Start at login" is a device setting, off by default; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 13 or 15.
- `apps/client/README.md` (English) as in Scope.
- 15 10 updated; rulings of Design note 5 in `docs/DECISIONS.md` section 10.
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
4. Where the Windows integration test ran (Windows build, packaged or not), which presenter features were unavailable without package identity, and the packages chosen with their licenses.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
