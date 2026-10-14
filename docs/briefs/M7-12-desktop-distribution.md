# M7-12: Desktop app distribution (ADR 0037)

Status: ready | Depends on: M4-34, M7-10 | Milestone: M7

## Goal

After this ticket an organization can build, sign and auto-update the desktop app with its own identity (18 8, R-RL-05, R-RL-06): `apps/distribution` holds the template `distribution.yaml` with every publisher value (bundle and package IDs, team ID, Sparkle feed URL and public EdDSA key, MSIX publisher, APNs topic, Firebase project, optional crash report target); `make -C apps/client release-apple` builds the macOS app as a Developer ID edition with Sparkle 2 (EdDSA-signed archive, appcast entry with channel `beta`, phased rollout attribute), and `make -C apps/client release-windows` builds and signs an MSIX with `stable.appinstaller` and `beta.appinstaller` per channel. Notarization (`xcrun notarytool`, `xcrun stapler`) and the real signatures run only with the owner's publisher credentials in the environments `release-apple` and `release-windows`; they are paid accounts and stay an owner decision (OQ-08, D-W-125). Until then everything is built and proven with a test publisher: a self-signed code signing identity on macOS, a self-signed certificate on Windows, a throwaway EdDSA key for Sparkle. With the test publisher, a beta build replaces an older one automatically on both platforms, `codesign --verify --deep --strict` and `signtool verify /pa` are green, and the trial runs in CI on GitHub-hosted `macos-15` and `windows-2025` runners. Why: R-RL-05 makes every organization its own publisher; the chain has to exist and work before any organization brings its accounts, and app updates are corrected forward only (R-RL-07), so the update path must be proven.

## Read first

- `docs/spec/18-release.md`: 5 (channels, rules 1 and 4), 8 (route table and consequences), 10 (rollback), 13 (R-RL-03, R-RL-05 to R-RL-07).
- `docs/adr/0037-app-distribution-and-auto-update-channels.md` (all, "Confirmation"), `docs/operations/release.md` 4, `docs/OPEN-QUESTIONS.md` OQ-08.
- `docs/spec/15-client-architecture.md` 2 (minimum OS versions: macOS 12, Windows 10 2004), 10 (Windows package identity), 3.1 (platform code only in `apps/client` and `device_key`).
- `.github/workflows/release.yml` (jobs `build-apple`, `build-windows`, environments), `.github/workflows/ci.yml` (jobs `apps-macos`, `apps-windows` of M4-01), `tools/ci/check_runners.py`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), the trial release and the test publisher (D-W-127); section 10 (format of `D-M7-12-<n>` rows).
- Briefs (merged names win, D-W-100): `M4-01-workspace-arch.md` (platform jobs, "Building and signing release artifacts ... M7"), `M4-18-desktop-shell.md` (branding and bundle IDs "M7"), `M4-30-windows-tray-notifications.md` (package identity with MSIX), `M4-34-desktop-e2e-perf.md`, `M7-10-server-release-chain.md` (signing of files, `SHA256SUMS`, trial pattern).
- Code on the base branch: `apps/client` (macos and windows runners, `pubspec.yaml`), `apps/README.md`.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: the template uses placeholders (`com.example.bothy`, `https://updates.example.com/appcast.xml`, `CN=Example Publisher`); no real team ID, bundle ID of an organization, company, host or personal name.
9. Every tool is pinned (Sparkle release by version and checksum, actions by SHA); no moving tag.
10. Specification first: a change to the distribution rules updates `docs/spec/18-release.md` 8 and `docs/operations/release.md` 4 in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): no Apple Developer Program membership, no Windows certificate or Artifact Signing account, no paid runner. The owner's credentials are never required for `make check` or for the trial.
12. Never tag, never create a release, never upload to an update host or a store (D-W-125).

Product requirements implemented here:

13. One build moves from beta to stable without a rebuild (18 5 rule 1): the appcast element `sparkle:channel` is removed for stable, `stable.appinstaller` points at the same MSIX.
14. Every publisher value comes from `distribution.yaml`; a missing value fails the build and names the key; the template validates against a JSON Schema in `apps/distribution`.
15. The release jobs sign and notarize only with the environment secrets; without them the job stops with a clear message before signing, never with a silent unsigned artifact.
16. The Sparkle archive carries an EdDSA signature that `sign_update` produced and the appcast verifies; the MSIX carries the package identity that local notifications need (M4-30).

## Scope

- `apps/distribution/`: `distribution.yaml` template, its JSON Schema, a README (how an organization fills it, where it lives in the ops repository of an instance), the appcast and `.appinstaller` templates.
- `apps/client/Makefile`: `release-apple VERSION=<v> OUT=<dir>` (macOS app; the iOS archive is M7-13) and `release-windows VERSION=<v> OUT=<dir>`, both reading `DISTRIBUTION_FILE`; a `trial-apple` and `trial-windows` path that uses the test publisher (Design note 1).
- `tools/distribution/` scripts (render values into Info.plist, the MSIX manifest and the appcast; create the test publisher) with tests under `tools/distribution/tests/`.
- `.github/workflows/release.yml`: the jobs `build-apple` and `build-windows` call the new targets with the publisher secrets of their environments.
- `.github/workflows/ci.yml`: a trial step in the jobs `apps-macos` and `apps-windows` (or new jobs) that runs the update test of Design note 2 on pull requests that change `apps/distribution`, `apps/client` release files or `tools/distribution`; `make runners-check` and `make lint` stay green.
- Docs: `docs/spec/18-release.md` 8 (as built), `docs/operations/release.md` 4 (publisher steps with real accounts, what the owner must provide), `apps/client/README.md`, `apps/distribution/README.md`, root `CHANGELOG.md` and `apps/client/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-12-<n>`.

## Out of scope

- iOS TestFlight, Android tracks and the minimum version in the app (M7-13).
- Real Developer ID signing, notarization, Artifact Signing or an OV certificate (owner accounts, OQ-08).
- Hosting the appcast and the `.appinstaller` files (the publisher's update host).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-12-desktop-distribution`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(client): ...`, `ci(release): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). The local update host of the test binds only `127.0.0.1` on a random port.
- The macOS part needs a macOS host with Xcode and the Windows part a Windows host; run the part your build host supports locally and prove both in the CI jobs on `macos-15` and `windows-2025`; a part that neither the build host nor CI could run is a stop: `Verdict: BLOCKED <platform> trial not run`.
- Test publisher keys and certificates live only in a temporary keychain or certificate store of the run and are removed at the end; never import them into a login keychain of a person.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Test publisher** (D-W-127; OQ-08). macOS: create a self-signed code signing certificate in a temporary keychain, sign the app with the hardened runtime, `codesign --verify --deep --strict` must pass. `spctl --assess` accepts only an Apple-issued Developer ID with notarization; with the test publisher the trial runs it and records its rejection as expected, and the release job runs it as a required check with the real identity (ruling; report it as not proven). Windows: a self-signed certificate whose subject equals the MSIX `Publisher`, imported into the run's `TrustedPeople` and `Root` stores, `signtool verify /pa` must pass.

2. **Update test** (ADR 0037 confirmation). macOS: build version `N` and `N+1` (same build moves channels), serve the appcast with channel `beta` and the EdDSA-signed `N+1` archive from a local HTTP server, start `N` with the feed and an automatic check, and assert that after the update the installed bundle reports `N+1`; drive it with Sparkle's own updater (for example the `sparkle` command line tool shipped with the pinned Sparkle release, or a test harness that calls `SPUUpdater`), not by copying the bundle. Windows: install `N` through `beta.appinstaller`, point the file at `N+1`, trigger the App Installer update path (on launch with `HoursBetweenUpdateChecks="0"` or the documented package manager call with the `.appinstaller`), and assert the installed package version `N+1`. Record the mechanism used per platform in the ruling.

3. **Channels** (18 5). macOS: one appcast; beta entries carry `sparkle:channel` beta; promotion removes the element (a script with a test). Windows: two `.appinstaller` files; promotion points `stable.appinstaller` at the same MSIX.

4. **Rulings for `docs/DECISIONS.md`** (`D-M7-12-<n>`): test publisher and the `spctl` limit (note 1), update mechanisms (note 2), promotion scripts (note 3), the schema of `distribution.yaml`.

## Tests (write them first)

- `tools/distribution/tests/`: schema validation of the template (missing key fails with its name), rendering into Info.plist, MSIX manifest and appcast, the promotion scripts.
- Platform trials (CI and the build host where possible): the update tests of Design note 2; `codesign --verify --deep --strict` and `signtool verify /pa` green with the test publisher; the appcast signature verifies with the public EdDSA key.

Commands that must pass, from the repository root:

- `uv run --project tools --locked pytest -q tools/distribution`
- `make -C apps/client trial-apple` on a macOS host, `make -C apps/client trial-windows` on a Windows host (each where available; CI runs both)
- `make lint`, `make runners-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] With a test publisher, a beta build replaces an older one automatically on macOS (Sparkle) and Windows (App Installer).
- [ ] `codesign --verify --deep --strict` and `signtool verify /pa` are green with the test publisher; `spctl` runs in the release job with the real identity and its test-publisher rejection is recorded.
- [ ] The template `distribution.yaml` with schema exists; every publisher value comes from it.
- [ ] The release jobs sign only with environment secrets and stop clearly without them.
- [ ] 18, the runbook, READMEs, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `apps/distribution/README.md` and `apps/client/README.md` written in English.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
- CHANGELOG entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`, and the CI trial results on `macos-15` and `windows-2025`.
3. Review and red team verdicts and the findings fixed.
4. The update mechanisms used and their evidence.
5. What is not proven (Developer ID, notarization, `spctl` acceptance, SmartScreen reputation with a real certificate).
6. Owner steps with a recommendation (accounts per OQ-08); follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
