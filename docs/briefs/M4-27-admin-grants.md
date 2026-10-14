# M4-27: Administration: grant wizard, Who has access, View as person

Status: ready | Depends on: M4-26 | Milestone: M4

## Goal

After this ticket the administration in `packages/admin_ui` has the permission screens A2 to A4 of 01 5.2: the grant wizard in three steps (who, where, level and capabilities with validity and a required reason) that calls the server's preview before every confirmation and shows its plain-language text, the effects per principal, the invariant check (I1 and SR8 explained, not just rejected: "switch to organization first"), the hints (H1 read horizon, blocked triggering, group reach, missing expiry, a terminal grant that is not effective) and a denied delegation with its reason, then confirms with step-up; the grants list per target with revoke (immediate, the confirmation says so, and a required reason); "Who has access?" per space or channel with level, capabilities and the source chain (owner, direct, group, inherited, operator, participant, via agent with hint H1, blocked by a guard rule; disabled principals struck through); and "View as person" as the read-only snapshot of the spaces and channels a person sees, with the clear frame, its expiry and the audit note. Integration tests use the example of 07 9.3. Why: the owner must see the effect of a grant before giving it, including what an agent could leak (H1); a wizard without the preview would make permission changes a guess (07 9).

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-26 to M4-28, M4-37; section 12 rows M6-04, M6-07.
- `docs/spec/01-product.md`: 5.2 A2 "Create grant with plain-language preview", A3 "Who has access?", A4 "View as person"; 6.7; 9 (terms Access grant, Trigger agents, Approve actions).
- `docs/spec/07-permissions.md`: 2 "Grant model" (2.1 to 2.7, 2.12 hosts and `terminal`), 4.1 (decisions and reasons, delegation limits), 5 (SR1, SR2, SR3, SR8, SR10, SR12 as the preview reports them), 6 (read horizon, H1), 9 "Requirements for the admin interface" (9.1 to 9.5), 10 (examples), 11.2.
- `docs/spec/10-backend-policy.md` 5.4 and 11 (the I1 violation and what the guided flow of M4-28 adds).
- `docs/spec/05-actions.md` (`grant.create`, `grant.revoke`, `access.preview`, `access.explain`, `access.view_as` with their step-up and audit columns).
- `docs/api/openapi.yaml`: `listGrants`, `createGrant` (`GrantCreate`), `previewGrant` (`GrantPreview`, `AccessEffect`, `InvariantViolation`, `PreviewHint`), `getGrant`, `revokeGrant`, `explainSpaceAccess`, `explainChannelAccess` (`AccessExplanation`, `AccessSource`), `viewAsUser` (`ViewAsSnapshot`).
- `docs/DECISIONS.md`: D-W-94 (no German text in tracked files except allow-listed ones), D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M4-26-admin-people.md` (shell, repository, `ConfirmChange`, the step-up test), `docs/briefs/M4-25-settings-devices.md` (`StepUpDialog`); for the server side `docs/briefs/M1-48-authz-api.md`, `docs/briefs/M1-49-authz-explain.md`.

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
9. The preview text is the server's (`text`, `text_locale`); the client never composes its own sentence of the effect, only labels around it (07 9.3: templates in `authz`).
10. Test data in a language other than English is never copied into a tracked file: the test reads the example of 07 9.3 at run time (D-W-94).

Product requirements implemented here:

11. The wizard calls the preview before every confirmation and shows its text, effects, invariant violations and hints; the step-up follows afterwards (07 9.3).
12. An I1 violation is explained with the hint "switch to organization first", not just rejected; the guided switch is M4-28 (01 A2, 10 11).
13. A delegation limit gives `allowed = false` with `denial_reason`, shown as the reason; the confirm button stays disabled (07 9.3).
14. Every grant needs a reason; persons outside groups without an expiry get the `expiry_missing` hint (07 9.3, 9.5).
15. Revoking takes effect immediately, needs a reason (`revokeGrant` body), and the confirmation says so (07 9.5).
16. "Who has access?" shows per principal the effective level, capabilities and the source chain with the kinds of 07 9.1; entries only via an agent have level `none` and carry hint H1; disabled principals are struck through.
17. "View as person": owners only, with step-up; a read-only snapshot with the frame "Viewing as <name> (read-only). Nothing is sent. Logged." and the expiry; DM channels of the person stay invisible (07 9.2, SR6).

## Scope

Create in `packages/admin_ui/lib/src/`:

- `grants/` (`grant_wizard.dart` with the three steps, `subject_step.dart`, `target_step.dart`, `level_step.dart`, `preview_panel.dart` with text, effects, invariants and hints, `grants_list.dart`, `revoke_grant_dialog.dart` with the required reason).
- `access/` (`who_has_access.dart`, `source_chain.dart`, `view_as_person.dart`).
- Registration of these screens in the admin shell of M4-26 (entries Grants and the "Who has access?" action on spaces and channels).
- Fake server handlers through the registry of M4-17 for the operations above; the preview handler returns, for the example request of 07 9.3, the text read at run time from 07 (Design note 3).
- Tests under `packages/admin_ui/test/grants/` and `test/access/`, goldens, integration tests under `apps/client/integration_test/` (Tests).
- ARB keys for every label in `packages/l10n`.

Change:

- `packages/admin_ui/README.md`, `apps/client/README.md`.
- `docs/DECISIONS.md` section 10: rows `D-M4-27-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The guided flow "Grant a person access in a personal space" with `resolve_i1` (M4-28).
- Grants for hosts and the `terminal` capability in the host screen (M4-37 adds the wizard for the target host on top of this wizard).
- Browsing content as another person beyond the snapshot (07 9.2: added with the console session, M6).
- The list "Permission hints" (M6-07).
- The list "Expiring soon" with "Extend" (07 9.4; the server queries of M6-07).
- The "History" button of A3 (the audit view of M4-29 filtered by the target; the link from this view comes with the console administration, M6-04).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-27-admin-grants`; one writer per worktree.
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

1. **Wizard.** Step 1 who: a person, group, agent or app from the directory lists (`listUsers`, `listGroups`, `listAgents`; apps only if the shell has the Apps entry), search by name. Step 2 where: instance, space or channel from the server's lists (only targets the caller sees). Step 3: level (`read`, `write`, `trigger`), capabilities (`approve_actions`, `upload`, `export`, `manage`, and for host targets nothing here), `expires_at`, required `reason`. Every change of a step debounces and calls `previewGrant` with the full request; the panel shows `text` (in `text_locale`), the effects grouped per principal (added, removed, changed), `invariant_violations` (I1 with the conflicting agents and the hint "switch to organization first"; SR8), `hints` mapped to labels, and `allowed = false` with `denial_reason` through `problemText` (reason codes of 07 4.1). "Confirm" is enabled only when `allowed` is true and the reason is set; it calls `createGrant` through the step-up interceptor.

2. **Who has access and view as.** `explainSpaceAccess` and `explainChannelAccess` render rows with level, capabilities and the source chain (`AccessSource` kinds as plain labels with the grant's expiry and reason; `via_agent` with the agent and hint H1; `guard_blocked` with the rule); actions on a row: create grant and revoke. "View as person": `viewAsUser` with step-up, shown in a frame with the banner text of A4 and the expiry from `expires_at`; nothing in it is clickable to write.

3. **Example of 07 9.3.** The fake's preview handler reads the fenced example between the language-check markers of 07 9.3 at run time and returns it with `text_locale = de` for the request that matches the example (Sam, trigger, channel Marketing/Kampagnen, until 2026-12-31, agent Texter with H1 on Marketing/Intern); the integration test asserts that the panel shows that text unchanged and the H1 hint. No German sentence lands in a tracked test file (D-W-94).

4. **Rulings for docs/DECISIONS.md** (`D-M4-27-<n>`): preview on every step change (note 1); server text only (Rules); the run-time example (note 3).

## Tests (write them first)

Widget tests (fake repository, fake signer):

- `grant_wizard_test.dart`: the three steps; the preview is called with the full request after a change; confirm disabled without a reason or with `allowed = false`; I1 violation explained with the hint; delegation denial with its reason; step-up after confirm; revoke says it takes effect immediately.
- `who_has_access_test.dart`: every source kind of 07 9.1 rendered; `via_agent` with level `none` and H1; disabled principals struck through.
- `view_as_test.dart`: step-up, frame text, expiry, nothing writable.
- Goldens of the wizard (each step and the preview with hints), the access view and the view-as frame; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Integration test (`apps/client/integration_test/grant_preview_test.dart`, against `FakeServer`): the example of 07 9.3 end to end: Alex opens the wizard, chooses Sam, the channel and `trigger` until the date with a reason, sees the preview text of 07 9.3 and the H1 hint, confirms with step-up, and "Who has access?" then lists Sam with the direct source.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make docs`
- `make check`

## Acceptance criteria

- [ ] Widget and integration tests with the example of 07 9.3 are green; the example text is read at run time, not copied.
- [ ] The wizard shows the server's preview with effects, invariants and hints before the step-up; I1 is explained; delegation denials show their reason.
- [ ] "Who has access?" shows the source chains of 07 9.1 with H1; "View as person" is a read-only snapshot with step-up.
- [ ] Rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 07.
- `packages/admin_ui/README.md` (English) as in Scope.
- Rulings of Design note 4 in `docs/DECISIONS.md` section 10.
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
4. Where the integration test ran, and every field of the preview or explanation responses that the screens could not show because the schema on the base branch differs from 07 9.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
