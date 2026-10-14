# M8-08: Import overview in the console

Status: ready | Depends on: M6-04, M8-03 | Milestone: M8

## Goal

After this ticket the web console has the page "Import" of 01 5.5 (feature row "Import from Discord", web only): a list of import sources with kind, state and progress; a source page with the counters of the server and of the tool (`stats.server.*`, `stats.tool.*`), the items in state `failed`, and one row per mapped channel with its Discord ID, its Bothy channel, the stored `lead` (mirror or "Bothy leads"), the read-only state and whether the channel's operator equals the planned operator; and every owner step of `import.manage` that the command-line tool cannot do because a `cli` session never provides step-up (08 6.4, 22 8): create a source by uploading the mapping file (YAML, converted to JSON in the browser, server errors shown at their JSON pointers), start, pause, cancel and complete (each with confirmation and step-up), "Bothy leads" for a channel (confirmation that names the Discord step and the `switch` command, then `setImportLead` `bothy`, then the guided `setChannelOperator` to the planned operator with the I1 check) and "Discord leads" (revert: `setImportLead` `discord`, then remove the operator). The page refreshes after every action, on a refresh control and when the browser tab becomes visible; it reads the same `ImportSource` model that `import.progress` carries. `patrol` web tests run these flows in `make console-e2e`. Why: without these screens no source can be created or started and no channel can switch over, because those steps need step-up (22 8, 13.3, 13.4, D-W-124).

## Read first

- `docs/spec/01-product.md`: 3.7 (row "Import from Discord": web, `import.manage`), 5.2 (confirmation with step-up), 5.5 (console navigation with "Import"), 8, 9.
- `docs/spec/22-import-migration.md`: 5.2 (mapping file), 8 (operations, the `cli` paragraph), 11 (report columns), 12.1 (ownership list), 13 (flow per channel; steps done in the console), 14.
- `docs/spec/05-actions.md` (core contract, read only): 4.12 `import.read`, `import.manage`, rule 1.8, `channel.operator.set`, `channel.read_only.set`.
- `docs/spec/10-backend-policy.md` (I1 and the guided switch).
- `docs/spec/15-client-architecture.md` 3.1 rule 3 (console without gateway connection), 20 (row E2E: `patrol` for the console).
- `docs/api/openapi.yaml`: tag `Import`, `ImportSourceDiscordConfig` (M8-03), `setChannelOperator`, `getChannel`, `listAgents`.
- `docs/DECISIONS.md`: D-W-100, D-W-120, D-W-123, D-W-124; rows `D-M6-04-<n>`, `D-M8-03-<n>`; section 10 format.
- Briefs (merged names win, D-W-100): `M6-04-console-admin.md` (hosting `admin_ui` in the console, the step-up adapter, `patrol` web suites in `console-e2e`), `M6-02-console-app.md` (routes, HTTP layer, CSRF), `M6-03-console-csp.md` (`console-e2e`, the seed job, the CSP violation collector), `M4-25-settings-devices.md` (adding a device by pairing code, which the owner uses to approve `bothy import login`), `M4-28-admin-agents.md` (the guided operator switch with I1), `M8-03-import-server.md` (states, `409` reasons, lead semantics).
- Code on the base branch: `packages/admin_ui`, `packages/api_client`, `packages/l10n`, `apps/console`, `tools/console-e2e`, `infra/compose/test`.

This ticket touches no API operation, no table, no action, no event and no server configuration key.

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
9. No configuration key is added.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Every `import.manage` step (create, start, pause, cancel, complete, lead change) and `channel.operator.set` shows its confirmation with the plain-language effect and then the step-up (01 5.2; 05 4.12).
12. The page shows counters only, never message content, file names or names of Discord users (22 11, 14); channel names (Discord and Bothy) are structure and may be shown.
13. "Bothy leads" names the order of 22 13.3 in its confirmation: Discord locked by the owner, `bothy import discord switch ... --discord-locked` succeeded; then the lead, then the operator, which must be the planned operator (otherwise the confirmation warns that imported threads keep the planned operator, R-IM-08).
14. "Discord leads" sets the lead (which sets read-only and cancels active runs, M8-03) and then offers removing the operator (22 13.4 step 1).
15. Completion is offered only when every imported channel has `lead = bothy`; the server's `409 conflict` with `reason` `lead_not_bothy` is shown in plain language; the confirmation of completion warns that the mapping in `import_items` stays until the retention class `import` is deleted and that after that deletion a repeated import is no longer idempotent (22 13.5).
16. The console has no gateway connection (15 3.1 rule 3); the page refreshes through `getImport` after actions, on a refresh control and on visibility; no timer polling without a configuration key.

## Scope

Create:

- `packages/admin_ui/lib/src/import/`: `ImportRepository` (REST through `api_client`), `ImportController` (state from `ImportSource` snapshots, the same payload as `import.progress`), screens `ImportListPage`, `ImportSourcePage`, `ImportCreateDialog` (file picker, YAML to JSON with the `yaml` package already in the workspace or the one the merged `admin_ui` uses, size limited by the server's request limit, errors at pointers), `ImportChannelTable`, the confirmation dialogs of requirements 11, 13 to 15 (Design notes 1 to 4).
- `apps/console/lib/admin/import/`: the route `import` and its page frame hosting the `admin_ui` screens with the console's step-up adapter (M6-04 note 2).
- `apps/console/integration_test/admin/import_test.dart`: `patrol` web flows (note 5).
- Tests named under "Tests".

Change:

- `packages/admin_ui` navigation model (entry "Import" for owners only), `packages/l10n` ARB files (English and German texts of every new string), `apps/console` routes and README, `tools/console-e2e` seed (note 5), root `CHANGELOG.md` under `[Unreleased]`; `docs/spec/22-import-migration.md` 8 (the console page names the steps; no new rule) only if wording needs it; `docs/DECISIONS.md` section 10 rows `D-M8-08-1` to `D-M8-08-4`.

## Out of scope

- The import tool, its login and the server (M8-03, M8-04, M8-10).
- A desktop or mobile import screen (01 3.7: web only).
- Editing a stored mapping (there is no operation; a changed mapping means a new source before the first `--apply`, 22 13.2).
- Deleting `import_items` by retention (M7).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-08-import-console`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(admin-ui): ...`, `feat(console): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the stack publishes its ports only on `127.0.0.1`.
- Never contact Discord or any other real service: the e2e seed writes import data through the API with an owner device session, never through SQL and never through a Discord call.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Docker, the pinned Flutter and the pinned browsers must be available for `make console-e2e`; the ticket must run it once on the build host or stop with `Verdict: BLOCKED <tool> missing`.
- Never lower a threshold, disable a gate, delete a test or relax the CSP.
- No permission logic in the client (15 1 principle 1): the entry and the buttons follow the server's answers (`403` hides nothing that the server did not refuse).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Model and refresh** (15 3.1 rule 3; Ruling for docs/DECISIONS.md). `ImportController` holds the latest `ImportSource` per ID and replaces it with every snapshot it receives, newest `stats` wins; a snapshot comes from `getImport` (after an action, the refresh control, `visibilitychange` to visible) or, in a future client with a gateway, from `import.progress`, whose payload is the same `ImportSource`. No periodic timer.

2. **Create** (22 5.2, 8). The dialog takes a name and a mapping file; the YAML is parsed strictly (duplicate keys are an error) and converted to JSON; nothing is stored in browser storage; `createImport` with `kind = discord` after confirmation and step-up; `400 validation_failed` errors are listed with their pointer translated to the YAML path (`/channels/2/planned_operator` becomes `channels[2].planned_operator`). After success the page shows the next steps: run a dry run, then start the source here, then `bothy import discord --apply`.

3. **Channel table and lead actions** (22 12.1, 13.3, 13.4; Ruling for docs/DECISIONS.md). Rows from the stored mapping's `channels` with a target other than `skip`; Bothy channel name, read-only and operator from `getChannel` of the posted channel item's target (found through `listImportItems` with kind `channel`); not yet imported channels show "not imported" and no lead action (the server would answer `409 conflict` with `reason` `channel_not_imported`). "Bothy leads": confirmation of requirement 13, step-up, `setImportLead` `bothy`; then the operator dialog of M4-28 preselected with the planned operator (I1 check and guided switch as there); if the owner cancels it, the row shows "Bothy leads, no operator". "Discord leads": confirmation (read-only for everyone, active runs cancelled, messages written in Bothy stay in Bothy and are counted by `bothy import discord revert`), step-up, `setImportLead` `discord`, then the offer to remove the operator.

4. **State actions** (M8-03 note 3). Buttons follow the state: `draft` start, cancel; `running` pause, cancel, complete; `paused` start, cancel, complete (complete only while every imported channel has `lead = bothy`, requirement 15; `409 conflict` with `reason` `lead_not_bothy` is still handled when the state changed after loading); `completed` and `failed` none. Each with confirmation, step-up and the server's answer; `409 conflict` with `reason` `import_state` and `409 conflict` with `reason` `lead_not_bothy` in plain language. The step-up adapter of M6-04 repeats the request once with the same `Idempotency-Key`.

5. **`patrol` web tests** (15 20; SEC-T05). The seed job of `console-e2e` creates, through the API with an owner device session, nothing import-specific; the test itself: (a) Robin opens Import, uploads a white-label mapping (two channels, one forum), sees a pointer error for a wrong handle, fixes it, creates the source; (b) starts it with the virtual authenticator; (c) the test's helper (an owner device session in the test runner, the way `bothy import` would act) posts a structure batch for both channels and sets the cursor counters; (d) after refresh the counters and two rows "mirror, read-only" appear; (e) "Bothy leads" on the first channel with step-up, then the operator dialog sets Lotse; the row shows "Bothy leads, operator ok"; (f) "Discord leads" on it again; the row is read-only; (g) complete is refused while one channel is mirrored; after "Bothy leads" on both, the completion confirmation shows the retention warning of requirement 15 and complete succeeds; (h) pause and cancel of a second source, each with step-up. Every flow runs with the CSP violation collector of M6-03 active; any violation fails the suite.

6. **Rulings for `docs/DECISIONS.md`** (`D-M8-08-<n>`): refresh without polling (note 1); YAML conversion in the browser and pointer translation (note 2); the lead actions and the guided operator step (note 3); the state buttons (note 4).

## Tests (write them first)

- `packages/admin_ui/test/import/`: controller tests with sequences of `ImportSource` snapshots (state changes, counters, a snapshot with an older `stats` than the shown one is still applied because the server is the source); widget and golden tests of the list, the source page, the channel table, the create dialog and every confirmation (light and dark, text scaling 1 and 2, de and en); a test that no message content, file name or Discord user name appears even when `stats` carries unknown members.
- `apps/console/integration_test/admin/import_test.dart`: the flows of Design note 5, red first.
- The route test of M6-02 still finds no content route.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/console && flutter analyze --fatal-infos && flutter test --coverage`
- `make console-e2e` on the build host
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Widget tests cover the snapshots of `import.progress` (`ImportSource`) and every screen.
- [ ] `patrol` web tests create, start, change leads, refuse and then perform completion, each with step-up, under the enforced CSP.
- [ ] No content, file name or person name of the source appears in the console.
- [ ] ARB files, README, CHANGELOG and rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol; comments explain why.
- `apps/console/README.md` and the `admin_ui` README updated in English.
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under `[Unreleased]`.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make console-e2e`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
