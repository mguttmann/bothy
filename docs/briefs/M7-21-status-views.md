# M7-21: Status page in apps and console

Status: ready | Depends on: M5-03, M6-04, M7-08 | Milestone: M7

## Goal

After this ticket people see the state of the instance where they work (01 status page row, 16 12): the desktop app, the mobile apps (iPhone and Android) and the console show the components with their states, open incidents and maintenance windows with their history, and the maintenance mode, from `getStatus` and the event `status.updated`; agent components appear only for people with `agent.read` on that agent (the server filters, the apps never widen); owners maintain components, incidents and their texts with `status.manage` in the administration of desktop and console (`packages/admin_ui`); the public status page is reachable without login only when `status.public_enabled` is set (`getPublicStatus`). When the server cannot be reached at all, the apps show "Server unreachable" instead of a stale "all good", because the status page runs in the server (16 12 "Limit"). Why: the status page is how users learn whether a problem is theirs or the instance's; it must look the same on every surface and never claim health it cannot know.

## Read first

- `docs/spec/16-operations.md` 12 (all rows, the limit), `docs/spec/01-product.md` (status page row, the screens of the administration and of the console that list status), `docs/adr/0065-maintenance-mode-read-only.md` item 6 (display of maintenance).
- `docs/api/openapi.yaml`: `getStatus`, `getPublicStatus`, `createStatusComponent`, `updateStatusComponent`, `deleteStatusComponent`, `createStatusIncident`, `addStatusIncidentUpdate`, the `status.updated` event; `docs/spec/05-actions.md` (`status.read`, `status.manage`, `status.public.read`).
- `docs/spec/15-client-architecture.md` (connection states, packages, 3.1 dependency rules), `docs/design/` (components and state roles).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125), D-W-121 (tests that need a server stack stay outside `apps/client/integration_test/`); section 10 (format of `D-M7-21-<n>` rows).
- Briefs (merged names win, D-W-100): `M7-09-status-page.md` (the read model and visibility), `M7-08-maintenance-mode.md` (the maintenance state in the read model), `M4-17-fake-server.md`, `M4-26-admin-people.md` (`admin_ui` structure), `M5-02-iphone-layout.md`, `M5-03-android-layout.md`, `M6-02-console-app.md`, `M6-03-console-csp.md` (`console-e2e`), `M6-04-console-admin.md` (console administration and its `patrol` web suites).
- Code on the base branch: `apps/client`, `apps/console`, `packages/admin_ui`, `packages/sync_engine`, `packages/l10n`, `packages/design_system`.

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
8. White label: test data uses the components of 16 12, an agent named "Lotse" or "Scout" and the people Alex, Sam and Robin.
9. Colours only from the state roles of the design tokens; no literal colour.
10. Specification first: a change to an operation goes into `docs/api/` first; the core contract is not edited by this ticket. Interface texts in German and English through `l10n`.
11. No cost (D-W-125).

Product requirements implemented here:

12. One status view in `admin_ui` (or a shared package the console may use under 15 3.1) for desktop and console; the phone apps reuse the same widgets in their layouts; the console uses neither `sync_engine` nor `local_store` (it refreshes `getStatus` on an interval, a named client value).
13. Agent components are shown exactly as the server returns them; no client-side widening.
14. Unreachable server: after the connection state reports the server unreachable, every status surface shows "Server unreachable" and hides the component states.
15. Owner actions require `status.manage`; the forms send only the fields of the operations; owner texts are Markdown rendered with the message renderer's safe subset.
16. Maintenance windows (incidents of kind `maintenance`) and the maintenance mode are shown distinctly from outages.

## Scope

- `packages/admin_ui`: status view and the owner forms (components, incidents, updates).
- `apps/client`: navigation entries on desktop, iPhone and Android; the "Server unreachable" state from the connection state; live updates through `status.updated`.
- `apps/console`: the status view and the administration forms; the `patrol` web suite joins `console-e2e`.
- `packages/l10n`: texts.
- Tests (below); goldens in light and dark.
- Docs: `apps/client/README.md`, `apps/console/README.md`, `packages/admin_ui/README.md`, `docs/quality/accessibility-checklist.md` rows for the new screens, CHANGELOGs under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-21-<n>`.

## Out of scope

- A separate public status website outside the server; push hints for incidents.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-21-status-views`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(admin_ui): ...`, `feat(console): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Widget and integration tests run against the fake server of M4-17 under `apps/client/test/` and the package tests; tests that need the Compose stack live outside `apps/client/integration_test/` (D-W-121); the console suite runs in `make console-e2e`.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **One view, three surfaces** (owner rule 2; Ruling for docs/DECISIONS.md). The view model takes a `StatusSource` port (apps: `getStatus` plus `status.updated` through `sync_engine`; console: `getStatus` on an interval) so the widgets stay the same; record the interval as a named client value with the rule of D-W-115.

2. **Rulings for `docs/DECISIONS.md`** (`D-M7-21-<n>`): the source port (note 1), the unreachable rule (requirement 14), the Markdown subset for owner texts.

## Tests (write them first)

- `packages/admin_ui`: widget tests for every component state, incidents, maintenance, empty state, and goldens in light and dark at text scale 1 and 2.
- `apps/client/test/`: with the fake server, an agent component is shown only when the server returns it; a server outage shows "Server unreachable"; a `status.updated` event updates the view.
- Owner forms: validation and the calls of the seven operations; a person without `status.manage` sees no form.
- Console: a `patrol` web test of the status view and of an incident flow in `make console-e2e`.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter test --coverage`, `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`, `cd apps/console && flutter analyze --fatal-infos && flutter test --coverage`
- `make console-e2e`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] Widget tests for components, incidents and maintenance on desktop, mobile and console.
- [ ] Agent components visible only with `agent.read`; "Server unreachable" on an outage.
- [ ] Public status only with `status.public_enabled`.
- [ ] READMEs, the accessibility checklist, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs updated in English.
- Rulings of Design note 2 recorded in `docs/DECISIONS.md` section 10.
- CHANGELOG entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make console-e2e`.
3. Review and red team verdicts and the findings fixed.
4. Golden images of the status view in both themes.
5. What is not proven (VoiceOver, TalkBack and Narrator by hand: the checklist rows are open for the release check).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
