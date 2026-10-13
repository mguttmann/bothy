# M4-28: Administration: agents, backend switch, costs and tokens

Status: ready | Depends on: M4-27, M2-21 | Milestone: M4

## Goal

After this ticket the administration in `packages/admin_ui` has A5 of 01 5.2: the agents list with the badges "Personal" and "Organization", an agent page with the adapter, the presence, the configured mode next to the reported mode of the heartbeat and the warnings of 10 8 (mismatch, a mode the adapter cannot prove, unknown login type), the operated targets, the credential holder, switching the backend per agent and for all agents with a required reason and step-up (switching to organization always possible; to personal with a holder and, for all agents, an explicit list proposed from the last global switch), the explanation of why Personal is not possible right now (I1 with the conflicting principals), the guided flow of 10 11 that grants a person access in a personal space by switching the agent to organization in the same request (`resolve_i1` in `createGrant`, `addGroupMember` and `addOwner`; BR-07 from the interface's view), and the agent's API tokens (list, create with the secret shown once, revoke). The cost figures of A5 come from the statistics of the server (`getCostStats`, M6-05) and join the agent page with the statistics views of M6 (D-W-116). Why: the backend policy decides who pays and who may trigger (10 1); the owner must see what an adapter really reports and must be guided through the only safe way to widen a personal agent's audience.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-26 to M4-28; section 8 rows M2-21, M2-22; section 12 rows M6-04, M6-05.
- `docs/spec/01-product.md`: 5.2 A5 "Agents with backend switch and costs", 6.7 (costs), 9 (terms Personal, Organization, Estimate).
- `docs/spec/10-backend-policy.md`: 3 (invariants I1 to I4), 5.4 (the `409 i1_violation` response with `conflicts`), 8 "Expected and reported" (fields and warnings), 9 "Switching (I4)" (9.1 actions and bodies, 9.2 effects), 11 "Guided flow", 12.2 (display of costs), 13 "Display in the app", 15 (BR-07, BR-08).
- `docs/spec/09-agents.md` 2.1 and 4.8 (agent records, heartbeat and reported state).
- `docs/spec/08-identity.md` (API tokens of agents: prefix, shown once, revoke) and `docs/adr/0061-token-format-with-a-fixed-prefix-and-checksum.md`.
- `docs/spec/05-actions.md` (`agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `token.create`; rule 1.6 `resolve_i1`).
- `docs/api/openapi.yaml`: `listAgents`, `getAgent`, `setAgentBillingMode`, `setAllAgentsBillingMode`, `setCredentialHolder`, `listServedChannels` or the field that names the operated targets, `listTokens`, `createToken`, `revokeToken`, `createGrant`, `addGroupMember`, `addOwner` with `resolve_i1` (added by M2-21), `previewGrant` (`InvariantViolation` with `organization_ready`), `listAuditEvents` (the last global switch).
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-116 (this ticket depends on M4-27 for the grant wizard of the guided flow; the cost figures of A5 follow with M6); section 10.
- Briefs: `docs/briefs/M4-27-admin-grants.md` (wizard and preview panel), `docs/briefs/M4-26-admin-people.md` (shell, repository, `ConfirmChange`, step-up test, group members, owners), `docs/briefs/M4-07-core-components.md` (`DsBillingBadge`, `DsCostText`); for the server side `docs/briefs/M2-21-billing-switch-i4.md`, `docs/briefs/M2-20-heartbeat-presence.md`, `docs/briefs/M1-51-agents-records.md`.

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
9. No permission logic: the switch, the holder change and the tokens appear according to the server's answers (owner-only actions answer `403` otherwise).
10. Specification first: the guided flow is one request with `resolve_i1` (05 rule 1.6), the field M2-21 adds to `GrantCreate`, `GroupMemberAdd`, `OwnerAdd` and `OperatorSet`; never two requests.

Product requirements implemented here:

11. The badge shows "Personal" or "Organization" with icon and text on the list and the agent page (10 13, components.md 10).
12. The reported mode from the heartbeat stands next to the configured mode; the warnings of 10 8 appear with their texts; an adapter that follows the configured mode per run shows no reported mode.
13. Switching needs a reason and step-up; switching to organization is always possible and clears the holder; switching to personal needs a holder; "all to personal" needs an explicit list (10 9.1).
14. A rejected switch to personal (`409 i1_violation`) explains which principals besides the holder may trigger, without names the caller may not read (10 5.4).
15. Guided flow: when the grant preview reports an I1 conflict, the wizard offers "Switch <agent> to Organization and grant" with a reason; one request with `resolve_i1 = {switch_to_organization: [agent_ids], reason}`; both changes or none; afterwards the agent page shows whether the adapter still reports the old mode and how to switch it (10 11, BR-07). The same option exists when adding a group member or an owner hits I1.
16. Tokens of the agent: list with prefix and expiry, create with the secret shown once, revoke with step-up.

## Scope

Create in `packages/admin_ui/lib/src/agents/`:

- `agents_list.dart`, `agent_page.dart` (adapter, presence, configured and reported mode, warnings, targets, holder), `switch_mode_dialog.dart` (per agent), `switch_all_dialog.dart` (with the proposed list from the audit), `holder_dialog.dart`, `i1_explanation.dart` (from `409 i1_violation` or the preview's violations), `agent_tokens.dart` (list, create, revoke), `guided_flow.dart` (Design note 2).
- Changes in `packages/admin_ui/lib/src/grants/preview_panel.dart` (M4-27) and `packages/admin_ui/lib/src/groups/member_editor.dart` and `people/owners_section.dart` (M4-26): the guided option when the preview or the response reports an I1 conflict.
- Registration of the Agents entry in the admin shell.
- Fake server handlers through the registry of M4-17 for the operations above, with the all-or-nothing rule of BR-07 and BR-08.
- Tests under `packages/admin_ui/test/agents/`, goldens, integration test under `apps/client/integration_test/` (Tests).
- ARB keys for every new text in `packages/l10n` (the texts of 10 8, 10 11 and 10 13 as keys, 10 13 says so).

Change:

- `packages/admin_ui/README.md`, `apps/client/README.md`.
- `docs/DECISIONS.md` section 10: rows `D-M4-28-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The reverse way back to personal as a guided flow (10 11: not a guided flow; the screen lists what would violate I1).
- Cost figures on the agent page (`getCostStats` of M6-05, added with the statistics views of M6), price table and cost reconciliation (M2-22, M2-28; the console administration M6-04).
- Presence history and the agent's commands list (no operation in the API for a history; commands appear in the composer, M4-21).
- Creating and disabling agents beyond what A5 shows (`createAgent`, `disableAgent` in the console administration, M6-04).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-28-admin-agents`; one writer per worktree.
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

1. **Agent page.** `getAgent` gives the configured mode, holder, tool class, reported mode, `billing_modes_supported` and the evidence; presence from the engine's `EphemeralStore` in the desktop app or from `getAgent` in the console (the screen takes a presence stream as a parameter). Warnings are pure functions over these fields with tests per row of 10 8. The targets the agent operates come from the field or operation the API offers for them.

2. **Guided flow.** In the grant wizard's preview panel, an `InvariantViolation` with `code = i1_violation` shows the plain-language explanation of 10 11 step 2 (with the extra sentence when `organization_ready` is false) and a button "Switch to Organization and grant"; it asks for the reason and sends `createGrant` with `resolve_i1 = {switch_to_organization: [agent_id, ...], reason}` through the step-up interceptor. Success shows the agent page state (step 5 of 10 11). A failure of either part leaves nothing changed (the server's transaction); the screen shows the problem. `addGroupMember` and `addOwner` get the same option on `409 i1_violation`.

3. **Switch all.** "All to organization" needs only a reason; "All to personal" needs a holder and the explicit agent list, proposed from the last `agent.billing_mode.set_all` in the audit (`listAuditEvents` filtered by action) and editable; BR-08 (one violating agent changes nothing) is shown with the server's conflicts.

4. **Tokens.** `listTokens` for the agent, `createToken` shows the secret once in a copyable code view (pattern of M4-26 Design note 4) and never stores it; `revokeToken` with step-up.

5. **Rulings for docs/DECISIONS.md** (`D-M4-28-<n>`): presence as a parameter (note 1); guided flow from the preview and the two other actions (note 2); the proposed list (note 3); token secrets shown once (note 4).

## Tests (write them first)

Widget tests (fake repository, fake signer):

- `agent_page_test.dart`: badges; reported next to configured; every warning of 10 8; no reported mode for per-run adapters.
- `switch_test.dart`: reason required; step-up; to organization clears the holder; to personal needs a holder; `409 i1_violation` explained with the conflicts.
- `guided_flow_test.dart` (BR-07 from the interface): an I1 conflict in the grant preview offers the guided option; one request with `resolve_i1`; both changes appear; a failing request changes nothing and shows the problem; the same for adding a group member and an owner.
- `switch_all_test.dart`: proposed list from the audit; BR-08 shows the conflicts and changes nothing.
- `tokens_test.dart`: token secret shown once and not kept; revoke with step-up.
- Goldens of the list and the agent page with warnings, the switch dialogs and the guided flow; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Integration test (`apps/client/integration_test/guided_flow_test.dart`, against `FakeServer`): a personal agent operates the Support space; Alex grants Sam `trigger` there; the preview shows the I1 conflict; Alex takes the guided option with a reason and step-up; afterwards the agent is organization, Sam has the grant, and the agent page warns that the adapter still reports personal until the fake's heartbeat changes.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] The integration test of the guided flow is green (BR-07 from the point of view of the interface).
- [ ] Badges "Personal" and "Organization"; reported mode next to the configured mode with the warnings of 10 8.
- [ ] Switching per agent and for all agents with reason and step-up; I1 rejections explained.
- [ ] Agent tokens with the secret shown once and revoke with step-up.
- [ ] Rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 10.
- `packages/admin_ui/README.md` (English) as in Scope.
- Rulings of Design note 5 in `docs/DECISIONS.md` section 10.
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
4. Where the integration test ran, and the shape of `resolve_i1` as found on the base branch.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
