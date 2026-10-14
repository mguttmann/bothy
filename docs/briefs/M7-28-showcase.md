# M7-28: Showcase: images, video, landing pages and release template

Status: ready | Depends on: M5-13, M6-16, M6-18, M7-25, M7-27 | Milestone: M7

## Goal

After this ticket whoever sees the project for the first time understands in one minute what it does (PLAN 13 goal, P-22): `make showcase` regenerates, by script from the demo instance of M7-27, screenshots of all apps the build host can run next to the demo stack (desktop macOS, iPhone, Android, console, TV mode, terminal, Grafana plugin; the Windows app follows on a host of the owner, Boundaries) in both themes and a short video of one flow (thread, live box, approval, terminal, live topology); a manifest names script, commit and demo seed per image, and images carry no metadata (no EXIF, no text chunks). The README becomes a landing page (the value in one sentence with the product description of 01, images, the quick start with the one command of M7-29, a reading path), the docs site gets a landing page, `.github/release-template.md` is the template of the release pages, every release section of every changelog starts with a summary for users before the technical entries (a check in `make check` fails a section without one), and the architecture graphics under `docs/architecture/` render with `make diagrams` and are readable on the docs site in light and dark (contrast of lines and texts checked with the contrast gate of M4-03). A one-minute test proves the result: a fresh, read-only reviewer without prior knowledge reads only the README and the docs landing page and answers three fixed questions (What is it? For whom? How do I start it?) correctly; questions, answers and time go into the report. The README names no company, no person except Alex, Sam and Robin, and no host (deny list). Why: exit criterion 8 of M7; the showcase is the first impression of V1 and must show real behaviour from a real instance, never real data.

## Read first

- `docs/PLAN.md` 13 (goal, exit criterion 8, red team: company or personal data in demo data, images and video), decisions P-22, P-26; `docs/spec/01-product.md` (product description, screens), `docs/spec/25-visualization.md` 6 (views), `docs/spec/24-terminal.md` (terminal view).
- `docs/quality/push-discipline.md` (release notes), `docs/spec/18-release.md` 4 (changelog and release notes), ADR 0045 (docs site), `docs/design/` (themes), `docs/architecture/README.md` (diagram conventions), `docs/quality/gates.md` (`diagrams`, `tokens`, `denylist`, `linkcheck`), `docs/quality/public-switch.md` (what V1 checks later).
- `README.md`, `docs/index.md`, `mkdocs.yml`, every `CHANGELOG.md` of the components (`tools/ci/components.py` lists them).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), D-W-121 (tests that need a server stack stay outside `apps/client/integration_test/`); section 10 (format of `D-M7-28-<n>` rows).
- Briefs (merged names win, D-W-100): `M7-27-demo-instance.md` (`setup demo`, `demo.yaml`), `M4-38-terminal-e2e-perf.md`, `M5-13-mobile-terminal-e2e.md`, `M6-16-viz-in-apps.md`, `M6-18-console-terminal.md` (the surfaces and their test harnesses that can drive screens), `M7-25-grafana-plugin.md`, `M7-29-setup-wizard.md` (the documented command), `M4-03-accent-variants.md` (contrast gate).

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
8. White label: only the demo data of M7-27; no company, no person except Alex, Sam and Robin, no host, no real address, no window title or desktop background that shows the build host; every image is checked against the deny list.
9. Images and video are produced only by script from the demo instance; nothing is edited by hand; the Apple app images come from a macOS host against the demo instance, never from real data.
10. Specification first: if the product description of 01 changes, 01 changes in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): the build host and free local tools only. GitHub-hosted macOS runners have no Docker engine and Windows runners run only Windows containers, so the demo stack cannot run next to the apps there (M4-34 Design note 6).
12. Never tag, never publish a release, never deploy the docs site (D-W-125).

Product requirements implemented here:

13. `make showcase` regenerates all images per platform it can run on (`make showcase PLATFORM=<platform>`); a manifest (`docs/assets/showcase/manifest.json`) names per image the script, the commit and the demo seed; a check fails when an image is not in the manifest or the manifest names a missing image.
14. Images and video carry no metadata: a check fails on EXIF, PNG text chunks or container metadata beyond codec data.
15. Every release section of every component changelog starts with a summary for users before the technical entries; the check runs in `make check` and fails a section without one (the template and the `release-prep` of M7-10 produce the placeholder).
16. Every graphic under `docs/architecture/` renders with `make diagrams` and is readable in light and dark on the docs site: lines and texts meet the contrast gate against both backgrounds.
17. `make docs` and `make linkcheck` stay green; `make denylist` passes over README, docs landing page, the manifest and the image files (with the OCR-free rule the gate supports: file names, metadata and any embedded text layers).

## Scope

- `tools/showcase/` (scripts per surface: drive the demo through the harnesses of the client E2E tickets, capture, strip metadata, write the manifest; video capture of one flow), with tests.
- `docs/assets/showcase/` (images, video, manifest).
- `README.md` landing page; `docs/index.md` landing page; `.github/release-template.md`; the changelog template and the summary check (`tools/docs/` or `tools/ci/` with tests); every component `CHANGELOG.md` gets the summary line in its existing release sections if any exist.
- `docs/architecture/`: light and dark rendering through `make diagrams`, the contrast check with the gate of M4-03.
- `docs/quality/push-discipline.md` (reference to the release template and the summary rule).
- Docs: `docs/quality/gates.md`, `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-28-<n>`.

## Out of scope

- Publishing the docs site or the repository (V1); marketing texts beyond the README and the landing page.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-28-showcase`; one writer per worktree.
- Conventional Commits with the component as scope (for example `docs(showcase): ...`, `feat(tools): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). The demo stack publishes only on `127.0.0.1`.
- Platform images the build host cannot produce (on a macOS build host with a Docker engine: the Windows app) are not a stop: the manifest leaves them out, the report lists them as not produced, and they follow on a host of the owner that runs the app next to the demo stack (owner decision, D-W-128).
- Image files stay small enough for `repo.max_file_kb`; never raise that limit.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Capture by harness** (P-26). Reuse the E2E harnesses of M4-38 (desktop), M5-13 (mobile), M6-16 and M6-18 (console, `patrol` web) to navigate the demo and take screenshots through the test frameworks' screenshot APIs, so window chrome and the host desktop never appear; the Grafana plugin through Grafana's rendering of a local stack; TV mode in the desktop app and console.

2. **Where the images come from** (D-W-125: no paid runner). `make showcase PLATFORM=<p>` runs on the build host for every platform it can run next to the demo stack (on macOS with a Docker engine: the macOS app, the iPhone simulator, the Android emulator, the console, TV mode, the terminal and the Grafana plugin). The session verifies the manifest and metadata checks locally and commits the files. Record the platforms produced and the ones left out.

3. **One-minute test** (PLAN 13 goal). Start a fresh, read-only reviewer (a new agent session without the repository context, given only the rendered README and docs landing page), ask exactly: "What is it?", "For whom?", "How do I start it?"; record the answers and the time; correct means the answers match the product description of 01, its audience, and the documented command of M7-29.

4. **Rulings for `docs/DECISIONS.md`** (`D-M7-28-<n>`): capture method (note 1), CI images (note 2), the summary check rule, the metadata rule.

## Tests (write them first)

- `tools/showcase/tests/`: manifest check (missing and extra images), metadata check (an image with EXIF or a text chunk fails), video metadata check.
- Changelog summary check tests: a release section without a summary fails, with one passes, `Unreleased` is exempt.
- Diagram contrast: a test that the light and dark renderings of every architecture graphic pass the contrast gate.

Commands that must pass, from the repository root:

- the tests of `tools/showcase` and the summary check
- `make showcase PLATFORM=<platform>` for the platforms the build host supports
- `make diagrams`, `make tokens`, `make denylist`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `make showcase` regenerates all images the build host can produce, and the report lists the platforms left out (Boundaries); a manifest names script, commit and demo seed per image; images without metadata.
- [ ] Every release section of every changelog starts with a user summary (check in `make check`).
- [ ] Every graphic under `docs/architecture/` renders and is readable in light and dark (screenshots of both themes in the report).
- [ ] One-minute test: the three questions answered correctly; questions, answers and time in the report.
- [ ] `make docs` and `make linkcheck` green; the README names no company, no person except Alex, Sam and Robin, and no host.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `tools/showcase/README.md` written in English.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
- `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of the showcase runs per platform.
3. Review and red team verdicts and the findings fixed.
4. The one-minute test: questions, answers, time; screenshots of the docs site in both themes.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
