# M7-13: Mobile app distribution and minimum version

Status: ready | Depends on: M5-11, M7-12 | Milestone: M7

## Goal

After this ticket the phone apps have their release path and old apps meet a clear barrier instead of an error. `make -C apps/client release-ios` builds the iOS archive and `make -C apps/client release-android` the signed Android App Bundle from the publisher values of `distribution.yaml` (M7-12); the release workflow uploads the iOS build to TestFlight (groups beta and stable) and the bundle to the Play test tracks (internal for beta, the next track for stable) only with the owner's publisher credentials, which are paid accounts and stay an owner decision (OQ-08, D-W-125). Until then the chain is proven with a test publisher: an unsigned iOS archive whose structure and entitlements are checked, and an Android bundle signed with a throwaway upload key and verified. In the app, the values `client.min_version.<platform>` and `client.recommended_version.<platform>` of `getDiscovery` (18 9, R-RL-08) take effect: below the minimum version the app does not connect and shows the update path of its channel (TestFlight, store, appcast or `.appinstaller` from its build configuration); below the recommended version it shows a notice; a gateway close with `4007 protocol_version_unsupported` shows "Update required" instead of an error (18 9 item 1). Why: exit criterion 4 of M7 requires the minimum version to take effect; it is the instance operator's only lever to retire vulnerable app versions (18 11 item 3), and it must be built before the first store build exists.

## Read first

- `docs/spec/18-release.md`: 5 (channels: iOS TestFlight groups, Android tracks), 8 (routes, consequences), 9 (all), 10, 11 item 3, 12 (the eight keys), 13 (R-RL-03, R-RL-07, R-RL-08).
- `docs/spec/04-sync-protocol.md` 6.3 and 6.4 (`identify` with versions, close `4007`), `docs/spec/15-client-architecture.md` (startup, connection states, 2 minimum OS versions), `docs/spec/01-product.md` 5.6 (E1 to E7, login and server address).
- `docs/api/openapi.yaml` (`getDiscovery`, the schema with `platform`, `min_version`, `recommended_version`).
- `docs/adr/0037-app-distribution-and-auto-update-channels.md`, `docs/operations/release.md` 4 and 6, `docs/OPEN-QUESTIONS.md` OQ-08.
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), the test publisher (D-W-127), D-W-121 (tests that need a server stack stay outside `apps/client/integration_test/`); section 10 (format of `D-M7-13-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-46-instance-settings.md` (discovery delivers `min_client_versions`), `M4-13-sync-connection.md` (close codes, `4007` stops for an update), `M4-17-fake-server.md`, `M5-02-iphone-layout.md`, `M5-03-android-layout.md`, `M5-11-mobile-e2e-perf.md` (`e2e-mobile`, device suites), `M7-12-desktop-distribution.md` (`distribution.yaml`, test publisher pattern).
- Code on the base branch: `apps/client`, `packages/sync_engine`, `packages/auth`, `packages/l10n`, `apps/distribution`, `services/server/internal/instance`.

This ticket touches no table, no action and no event.

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
8. White label: placeholders like `com.example.bothy` and `https://apps.example.com`; no real team ID, store listing, company, host or personal name.
9. Versions compare as SemVer; the keys are the eight of 18 12; code comments name the key, never a value.
10. Specification first: a change to the rules updates `docs/spec/18-release.md` and `docs/spec/15-client-architecture.md` in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English, interface texts in German and English through `l10n`.
11. No cost (D-W-125): no Apple Developer Program, no Play Console account, no Firebase project is required for `make check`, the tests or the trial.
12. Never tag, never create a release, never upload to a store or a test service (D-W-125).

Product requirements implemented here:

13. Below `client.min_version.<platform>` the app opens no gateway connection and sends no authenticated request except `getDiscovery`; it shows the update path of its channel; the outbox and the local data stay.
14. Below `client.recommended_version.<platform>` the app works normally and shows a dismissible notice once per version.
15. A close `4007` shows "Update required" with the update path and stops reconnecting (04 6.4); no generic error.
16. The update path comes from the build configuration (`distribution.yaml` values compiled in): iOS the TestFlight or store link, Android the store link, macOS the appcast via Sparkle, Windows the `.appinstaller`.
17. Upload steps run only in the environment `release-apple` or a new environment `release-android` with the owner's secrets; without them they stop with a clear message.

## Scope

- `apps/client`: the version gate in the startup and connection flow (a domain check in `packages/domain` or `sync_engine` as the merged layering allows, the screens or notices in `apps/client/lib`), interface texts in `packages/l10n`.
- `apps/client/Makefile`: `release-ios` and `release-android` (reading `DISTRIBUTION_FILE`), `trial-ios` (unsigned archive, structure check) and `trial-android` (throwaway upload key, `bundletool validate` and a signature check with `jarsigner -verify` or `apksigner` on an APK set).
- `apps/distribution`: the mobile values in the template and schema (if M7-12 left them out).
- `.github/workflows/release.yml`: iOS archive and TestFlight upload in `build-apple`; a job `build-android` with environment `release-android`; `tools/ci/check_runners.py` knows the new environment; `make runners-check` green.
- `services/server/internal/instance`: only if the discovery does not yet return the per-platform rules exactly as `openapi.yaml` describes (M1-46 builds it); a test of the response either way.
- Docs: `docs/spec/18-release.md` 8 and 9 as built, `docs/spec/15-client-architecture.md` (the version gate), `docs/operations/release.md` 4, `apps/client/README.md`, CHANGELOGs under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-13-<n>`.

## Out of scope

- The App Store submission and production tracks beyond the test tracks (owner, after OQ-08).
- Desktop distribution (M7-12).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-13-mobile-distribution`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(client): ...`, `ci(release): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Widget and integration tests run against the fake server of M4-17 under `apps/client/test/`; tests that need the Compose stack live outside `apps/client/integration_test/` (D-W-121).
- The iOS trial needs a macOS host with Xcode and the Android trial the Android SDK; run what the build host supports and prove the rest in the CI jobs on GitHub-hosted runners; a trial neither could run is a stop: `Verdict: BLOCKED <platform> trial not run`.
- Throwaway keys live only in the temporary directory of the run.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Version gate** (18 9; Ruling for docs/DECISIONS.md). The app reads its own version from the build, fetches `getDiscovery` before login and on every reconnect, and compares per platform; a comparison with an unparsable value treats the rule as absent and logs once. The gate runs before `identify`, so an old app never reaches `4007`; `4007` remains the second line for protocol versions.

2. **Test publisher on mobile** (D-W-127). iOS: `flutter build ipa --no-codesign` or `xcodebuild archive` without signing, then check bundle ID, version, build number and entitlements against `distribution.yaml`; signing and upload need an Apple account (owner). Android: a throwaway upload keystore created with `keytool`, the AAB signed and validated; the upload with Play credentials is owner-only.

3. **Rulings for `docs/DECISIONS.md`** (`D-M7-13-<n>`): version gate placement (note 1), test publisher on mobile (note 2), the environment `release-android`.

## Tests (write them first)

- Dart unit tests: SemVer comparison including pre-releases; the gate decision per platform.
- Widget and integration tests with the fake server (under `apps/client/test/`): below the minimum the app does not connect and shows the update path; below the recommended version the notice; `4007` shows "Update required" and stops reconnecting; the outbox survives.
- `tools/distribution/tests/`: template and schema with the mobile values.
- Trials: `trial-ios` and `trial-android` on the build host where possible and in CI.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- `make -C apps/client trial-ios` on a macOS host, `make -C apps/client trial-android` where the Android SDK is installed (CI runs both)
- `make lint`, `make runners-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] An app below the minimum version does not connect and shows the update path; `4007` shows a notice instead of an error.
- [ ] Below the recommended version the app shows a notice and keeps working.
- [ ] `release-ios` and `release-android` build from `distribution.yaml`; the trials pass; uploads run only with the owner's secrets.
- [ ] 18, 15, the runbook, READMEs, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `apps/client/README.md` updated in English.
- Rulings of Design note 3 recorded in `docs/DECISIONS.md` section 10.
- CHANGELOG entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of the trials per platform.
3. Review and red team verdicts and the findings fixed.
4. Screenshots of the update screen and the notice in both themes (from widget goldens or the simulator).
5. What is not proven (TestFlight and Play uploads, store review).
6. Owner steps with a recommendation (accounts per OQ-08); follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
