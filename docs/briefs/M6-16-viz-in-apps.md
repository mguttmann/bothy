# M6-16: Visualizations in the desktop app, console and mobile

Status: ready | Depends on: M5-03, M6-04, M6-06, M6-12, M6-14, M6-15 | Milestone: M6

## Goal

After this ticket the six views of `bothy_viz` (M6-12) run with real data in all three hosts (25 6, 7.1; ADR 0069): in the desktop app under "Live and statistics" (screen D12) with the live stream through `sync_engine` (`stats.subscribe` on the gateway, frames delivered as the ephemeral event `stats.live`), with TV mode (D13); in the mobile app as screen M10 under Me with one tab per view except TV mode, density `touch`, the live stream only in the foreground (the app is suspended in the background, 15 7.2), tapping a run card opens the thread; and in the console under "Live" with the frame polled from `getStatsLive` every `viz.live_poll_interval` and TV mode. Each host implements the two ports of M6-11 (`LiveFrameSource`, `StatsQuerySource`), shows "stale" after `viz.stale_after` without a new frame, shows costs in TV mode only with `viz.tv_show_costs`, and offers "Open in Grafana" only when `telemetry.dashboard_url` is delivered (M6-06), opening it in the system browser or a new tab, never in a WebView or iframe. A static check proves that `apps/client` contains no WebView package and no `HtmlElementView` with an iframe (VZ-T16). Integration tests with the fake server and fixtures, `patrol` web tests of the console and `patrol` tests on iOS and Android for all views of M10 with the table view and detaching in the background are green. Why: the visualizations are a product function of every app (R-PR-13 pillar "statistics"), built once in `bothy_viz` and fed by the transport each host has (R-CL-13, R-VZ-06).

## Read first

- `docs/spec/25-visualization.md`: 1, 4 rule 5, 5.1 (subscription; console polling), 6 (views; 6.6 TV mode desktop and console only), 7.1 (ports and who implements them; mobile only in the foreground), 9.1 (no WebView or iframe in apps; "Open in Grafana" with `telemetry.dashboard_url` in the system browser), 12 (VZ-T16), 15 (`viz.live_poll_interval`, `viz.stale_after`, `viz.tv_rotation_interval`, `viz.tv_show_costs`).
- `docs/spec/01-product.md`: 5.1 D12, D13; 5.3 M10; 5.5 (console navigation "Live").
- `docs/spec/15-client-architecture.md`: 3.1 rule 6 (`LiveFrameSource` from `sync_engine` in the apps, REST in the console), 7.1 (building blocks of `sync_engine`), 7.2 (`Suspended`), 21 (R-CL-13).
- `docs/spec/04-sync-protocol.md` (core contract, read only): 6.2 (ops `stats.subscribe`, `stats.unsubscribe`), 6.3 (`resume` restores subscriptions), 12 item 4.
- `docs/spec/19-security.md` SEC-VZ-1, SEC-T19 (the static check part).
- `docs/api/openapi.yaml` `getStatsLive`, `StatsLiveFrame`; `docs/api/asyncapi.yaml` `op.stats.subscribe`, `op.stats.unsubscribe`, `event.stats.live`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-16-<n>` rows) and the rows of M6-06, M6-11, M6-12, M6-15.
- Briefs (merged names win, D-W-100): `M6-11-viz-package.md` (ports, stale rule, fixtures), `M6-12-viz-views.md` (view contract, `VizHostOptions`, TV mode overlay), `M6-15-stats-live.md` (subscription, errors, `resume`), `M6-14-stats-queries.md`, `M6-06-stats-views.md` (`telemetry_dashboard_url`; its `AdminStatsSource` serves A7, not the six views), the briefs of M4-13 and M4-17 (gateway session and fake server), `M5-02-iphone-layout.md` and `M5-03-android-layout.md` (phone shell, Me tab, lifecycle), `M6-02-console-app.md`, `M6-03-console-csp.md` (`console-e2e`).
- Code on the base branch: `packages/bothy_viz`, `packages/sync_engine`, `packages/admin_ui`, `apps/client`, `apps/console`.

This ticket touches no API operation, no table, no action, no event and no configuration key.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names. Interface texts come from `packages/l10n`.
9. Intervals only through named configuration values (`viz.*` from `AppConfig`); code comments name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. The views of M6-12 in the desktop app (D12, TV mode D13), the mobile app (M10, no TV mode) and the console ("Live", TV mode), each with the table view.
12. The desktop and mobile apps take the live stream from `sync_engine` (`stats.subscribe`); the console polls `getStatsLive` every `viz.live_poll_interval`; `bothy_viz` stays free of transport code (15 3.1 rule 6).
13. On mobile the stream runs only in the foreground: the subscription ends when the app is suspended and is restored on `resume` (04 6.3) or a new `stats.subscribe` after `identify`.
14. "Stale" after `viz.stale_after` without a frame; TV mode ends on a key or a click and shows costs only with `viz.tv_show_costs`.
15. No WebView, no iframe in `apps/client` (VZ-T16, SEC-VZ-1); "Open in Grafana" only with `telemetry.dashboard_url` and outside the app.
16. A tapped run card opens the thread by deep link in the apps; the console shows "Open in the app" (ADR 0028).

## Scope

Create:

- `packages/sync_engine/lib/src/stats/`: the subscription manager (`subscribe(spaceId)`, `unsubscribe()`, the subscription carried across `resume`, re-sent after `identify`, ended by the server's `error` with the code passed on), and the delivery of `stats.live` frames to a stream; pure Dart, no Flutter (Design note 1).
- `apps/client/lib/viz/`: the app implementations of `LiveFrameSource` (over the `sync_engine` manager, with the stale rule of M6-11) and of `StatsQuerySource` (the query operations of M6-11 over the `api_client` with DPoP, M6-11 "Out of scope"); the desktop page "Live" with the view tabs and TV mode, the mobile M10 screen with tabs and without TV mode, foreground handling (Design notes 2 and 3).
- `apps/console/lib/viz/`: the console `LiveFrameSource` by polling `getStatsLive` every `viz.live_poll_interval` (stopped while the tab is hidden, Page Visibility), the console `StatsQuerySource` over the cookie client of M6-02, the "Live" route with TV mode (Design note 4).
- `tools/arch-check` or a pytest in `tools/ci/tests`: the static check of VZ-T16 over `apps/client` (Design note 5).
- Tests (Tests).

Change:

- `packages/admin_ui` navigation entry "Live" (desktop administration and console) as 15 3.1 assigns it to `admin_ui`.
- `apps/client` desktop sidebar group "Live and statistics" (D12) and the mobile Me tab entry "Live" (M10).
- `tools/console-e2e`: the `patrol` web suite of the console views.
- `apps/client/platform-tests.json`: the `patrol` M10 suites for iOS and Android.
- `packages/l10n` ARB files; READMEs of `sync_engine`, `apps/client`, `apps/console`; `docs/spec/15-client-architecture.md` 7.1 (the stats subscription building block); root `CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M6-16-1` to `D-M6-16-5`.

## Out of scope

- The optional Grafana embedding in the console (M6-17).
- The Grafana panel plugin (M7).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-16-viz-in-apps`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(sync-engine): ...`, `feat(client): ...`, `feat(console): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the stack publishes its ports only on `127.0.0.1`.
- Tests bind only `127.0.0.1` with port 0 and use the fake server of M4-17 with the fixtures of `bothy_viz` (the fake server serves `stats.subscribe` and `getStatsLive` from the fixtures; extend it if it lacks them). Stop only processes and containers you started, by PID or container ID; simulators by UDID, emulators by serial. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- The device suites need macOS with Xcode and an iOS simulator, and an Android emulator; the console suite Docker and the pinned browsers; where missing, `make apps-platform` prints its SKIP line; the ticket must run `make console-e2e` once on the build host or stop with `Verdict: BLOCKED <tool> missing`; the report names where the device suites ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Subscription in `sync_engine`** (04 6.2, 6.3, 12 item 4; 25 5.1; Ruling for docs/DECISIONS.md). The manager keeps the desired subscription (none, instance, or a space ID). When the session is live it sends `stats.subscribe` once; on `resumed` it expects the server to have restored it (04 6.3) and does not resend; after `identify` (a new session) it resends; on `error` with code `forbidden`, `not_found`, `feature_disabled` or `rate_limited` related to `stats.subscribe` it clears the desired subscription and emits the error; `unsubscribe` sends `stats.unsubscribe`. Frames arrive as ephemeral `stats.live` events on a stream; nothing is written to `local_store` (live frames are ephemeral and must not be persisted).

2. **App source and views** (25 7.1; D12, M10). The app `LiveFrameSource` wraps the manager and applies the stale rule of M6-11 (no frame for `viz.stale_after`). The desktop page hosts the view tabs and a TV mode button (`allowTvMode: true`); the mobile M10 screen hosts the tabs without TV mode (`allowTvMode: false`), density `touch`; run card taps go to `/t/{thread_id}`. "Open in Grafana" passes `onOpenGrafana` only when M6-06's `telemetry_dashboard_url` is present.

3. **Foreground only on mobile** (25 7.1; 15 7.2). Entering `Suspended` ends the session, so the server ends the subscription; returning to the foreground resumes; if the resume window passed and `identify` runs, the manager resubscribes. The view shows "stale" in between and never shows a frame older than `viz.stale_after` as live.

4. **Console source** (25 5.1; R-VZ-06; Ruling for docs/DECISIONS.md). A timer every `viz.live_poll_interval` calls `getStatsLive` with the scope; hidden tabs (`document.visibilityState`) stop polling; a `403 feature_disabled` stops polling and shows the text; errors back off up to `viz.stale_after` and the view shows "stale". TV mode in the console uses the same overlay as the desktop.

5. **Static check** (VZ-T16; SEC-VZ-1; Ruling for docs/DECISIONS.md). A check in the gate fails when `apps/client/pubspec.yaml` (or its resolved dependencies in `pubspec.lock`) contains a WebView package (a list: `webview_flutter`, `flutter_inappwebview`, `webview_windows`, `desktop_webview_window` and any package whose name contains `webview`), when any Dart file of `apps/client` uses `HtmlElementView` or creates an iframe element, or when a native folder of `apps/client` references `WKWebView`, `android.webkit.WebView` or `WebView2`. Run it in `make arch` (the M4-01 architecture tool or a separate script called there); test it with fixtures.

6. **Rulings for `docs/DECISIONS.md`** (`D-M6-16-<n>`): the subscription manager and its resend rules (note 1); no persistence of frames (note 1); mobile foreground behaviour (note 3); console polling with visibility (note 4); the static WebView check and its list (note 5).

## Tests (write them first)

- `packages/sync_engine/test/stats/`: subscribe once when live; no resend after `resumed`; resend after `identify`; errors clear the subscription and pass the code; `unsubscribe` sends the op; frames never reach the local database (a fake store records no write).
- Integration with the fake server (`apps/client/integration_test/viz/`): desktop D12 shows frames from the fixtures and "stale" when the fake server stops sending; TV mode starts, rotates, hides costs by default, shows them with `viz.tv_show_costs`, ends on a key and a click; "Open in Grafana" only with the URL.
- `patrol` on iOS and Android (`apps/client/integration_test/phone/viz_test.dart`): M10 with every view and its table view; the app goes to the background and the fake server sees the subscription end; back in the foreground frames resume; a run card opens the thread.
- `patrol` web in `make console-e2e`: "Live" polls `getStatsLive` against the Compose stack, shows the views and the table views, TV mode ends on a key; no CSP violation.
- VZ-T16: the static check passes on the real `apps/client` and fails on fixtures with a WebView dependency, an `HtmlElementView` with an iframe and a native `WKWebView`.

Commands that must pass, from the repository root:

- `cd packages/sync_engine && dart test`; `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`; `cd apps/console && flutter test --coverage`
- on macOS with Xcode and with an attached Android emulator: `make apps-platform`; otherwise the SKIP lines
- `make console-e2e` on the build host
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] VZ-T16 is green: no WebView and no iframe in `apps/client`.
- [ ] Integration tests with the fake server and fixtures pass; the `patrol` web test of the console passes; `patrol` on iOS and Android shows all views of M10 with the table view and detaches in the background.
- [ ] TV mode ends with a key or a click and shows costs only with `viz.tv_show_costs`; "stale" after `viz.stale_after`; "Open in Grafana" only with `telemetry.dashboard_url`.
- [ ] READMEs, 15 7.1, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol; comments explain why.
- READMEs of `sync_engine`, `apps/client` and `apps/console` updated in English.
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make console-e2e`.
3. Review and red team verdicts and the findings fixed.
4. Where the device suites ran.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
