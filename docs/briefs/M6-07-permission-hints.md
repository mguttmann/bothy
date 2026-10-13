# M6-07: Permission hints and expiry overview

Status: ready | Depends on: M1-30, M6-04 | Milestone: M6

## Goal

After this ticket owners (and managers for their spaces) see two lists that 07 9.4 and 9.5 require, in the desktop administration and in the console: "Permission hints" and "Expiring soon". The server computes them in `authz` and serves them through two new operations with the read action `access.hints.read` (R-AZ-34, already in the core contract): `listPermissionHints` returns, per instance or per space, the hint H1 for every agent A and every person P in its trigger set T(A) that may not read a channel K of its read horizon H(A) ("P can learn content of K through A", 07 6), grants to disabled principals, grants `trigger` whose triggering is blocked by SR2 or SR3, and agents in mode `personal` with their trigger set; `listExpiringAccess` returns grants and group memberships that expire within `authz.expiry_reminder_lead` and API tokens of agents and apps that expire within the same lead, each with subject, target, expiry and the action "Extend" (revoke plus a new grant with a new reason, 07 9.4) or "Renew token". Both lists are computed from the same permission state as `authorize` (no second rule set), never name content, and show only what the caller may administer. An integration test proves the H1 hint with an agent whose read horizon is larger than the permissions of a triggering person. Why: H1 is a hint, not a block (07 6), so the owner must see it somewhere outside the grant wizard; expiring access must not end unnoticed (07 9.4).

## Read first

- `docs/spec/07-permissions.md`: 2.5 (validity of a grant), 2.6 (managers within their scope), 2.9 to 2.11 (agents, tool class), 4.3, 6 (read horizon H(A), trigger set T(A), hint H1 and where it appears), 7 (permission state), 9.1 (source kind `via_agent`), 9.3 (hints in the preview), 9.4 (expiry reminder and "Extend"), 9.5 ("Permission hints" list: H1, grants to disabled principals, `trigger` without possible triggering (SR2, SR3), agents with `personal` and their trigger set), SR1, SR2, SR3.
- `docs/spec/05-actions.md` and `actions.yaml` (core contract, read only): `access.hints.read` (R-AZ-34), `grant.read`, `grant.revoke`, `grant.create`, `token.list`, `token.create` (renewal), `token.revoke`, `access.explain`.
- `docs/spec/03-data-model.md` (core contract, read only): `grants` (`expires_at`, `expiry_reminded_at`), `group_memberships`, `api_tokens` (`expires_at`), `agents` (`billing_mode`).
- `docs/spec/08-identity.md` 12 (API tokens, their expiry).
- `docs/spec/01-product.md` 5.2 (administration in desktop and console).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); D-W-123 adds R-AZ-34; section 10 (format of `D-M6-07-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-28-authz-core.md` (snapshot, `Scope`), `M1-29-authz-invariants.md`, `M1-30-authz-expiry.md` (expiry job, `access.expiring`), `M1-49-authz-explain.md` (`via_agent` source, H1 in explain and preview), `M1-44-identity-api-tokens.md` (tokens), `M1-54-chat-operators.md` (operators of a channel), `M6-04-console-admin.md` (console hosting of `admin_ui`), the briefs of M4-27 and M4-28 (grant wizard, agents and tokens screens).
- Code on the base branch: `services/server/internal/authz`, `internal/identity` (tokens), `internal/chat` (operators), `internal/agents`, `packages/admin_ui`, `apps/console`, `apps/client` (administration).

This ticket touches no table, no event and no configuration key.

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
9. Windows only through named configuration keys (`authz.expiry_reminder_lead`); code comments name the key, never the number.
10. Specification first: the two operations go into `docs/api/openapi.yaml` first; 07 9.5 names them. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Hints from the same permission state that `authorize` uses; no second rule set (07 7; B7).
12. H1 per (agent, person, channel) exactly as 07 6 defines it; a hint, never a block.
13. The list "Permission hints" contains exactly the four kinds of 07 9.5.
14. "Expiring soon": grants, memberships and agent and app tokens within `authz.expiry_reminder_lead`; "Extend" is a revoke plus a new grant with a new reason (07 9.4); tokens are renewed by creating a new token (shown once) and revoking the old one.
15. Scope: the instance for owners; a space for its managers, limited to targets within their `manage` scope (07 2.6); a channel or space the caller cannot read gives `404`, readable without `manage` `403`.
16. No content and no names beyond what the administration lists already show (principal display names, target names).

## Scope

All server paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `internal/authz/domain/hints.go`: pure computation of the four hint kinds from a permission state, the operator assignments and the agent facts (Design note 1).
- `internal/authz/app/hints.go` and `expiring.go`: the use cases with `authorize(p, access.hints.read, instance | space)`, the scope limit, paging with the cursor codec of M1-18 (Design notes 2 and 3).
- Ports where missing: `identity.TokenExpiry` (agent and app tokens expiring before a time, without the token values), `chat.Operators` (operators per channel, M1-54) as used by M1-49.
- `internal/authz/adapters/httpapi/hints.go`: handlers `listPermissionHints` and `listExpiringAccess`.
- `packages/admin_ui/lib/src/hints/`: the two list screens with filters (kind, space), the "Explain" link to A3, "Extend" through the grant wizard of M4-27 with the old grant prefilled and a new reason required, "Renew token" through the token screen of M4-28 (Design note 4).
- Tests (Tests).

Change:

- `docs/api/openapi.yaml`: operations `listPermissionHints` (`GET /api/v1/access/hints`, query `space_id`, `kind`, cursor, limit) and `listExpiringAccess` (`GET /api/v1/access/expiring`, query `space_id`, cursor, limit), both `x-authz-action: access.hints.read`, `x-rate-limit-class: rl.read`; schemas `PermissionHint` (kind `h1_read_horizon`, `grant_to_disabled`, `trigger_blocked`, `agent_personal`; the IDs of agent, person, channel, grant, rule and the trigger set where the kind has them), `PermissionHintPage`, `ExpiringAccessItem` (kind `grant`, `membership`, `token`; IDs, `expires_at`), `ExpiringAccessPage`; `make api-lint`, `make api-checks`, `make codegen-check`; `api/operations.yaml` enables them.
- `apps/client` desktop administration navigation and `apps/console` routes: entries "Permission hints" and "Expiring soon" (console navigation of 01 5.5 under "Groups, grants").
- `packages/l10n` ARB files (hint texts from templates, like the preview of 07 9.3).
- Docs: `docs/spec/07-permissions.md` 9.4 and 9.5 (the operations and the list contents), `internal/authz/README.md`, `packages/admin_ui/README.md`, root and server `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M6-07-1` to `D-M6-07-5`.

## Out of scope

- Changing a hint into a block (07 6 decides: hint).
- Emails or pushes about expiry (07 9.4: the event `access.expiring` exists; no email).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-07-permission-hints`; one writer per worktree.
- Conventional Commits with the module or package as scope (for example `feat(authz): ...`, `feat(admin-ui): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL only through `pgtest`; client tests use the fake server of M4-17. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID.
- Never read or print secrets; token values never leave identity (the port returns only IDs and expiry).
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **H1 and the other kinds** (07 6, 9.5; Ruling for docs/DECISIONS.md). For each agent A: H(A) = channels A may read (its grants plus the operator rule), from the permission state of A; T(A) = people with effective `trigger` on a target A operates and no guard rule blocking it (SR2, SR3), from the states of those people. For each P in T(A) and K in H(A) with P not allowed to read K: one hint `h1_read_horizon (A, P, K)`. Reuse the function M1-49 uses for the `via_agent` source; if M1-49 computed it inline, move it to `authz/domain` with its tests and call it from both. `grant_to_disabled`: grants whose subject is a disabled principal. `trigger_blocked`: grants with level `trigger` where SR2 or SR3 blocks every triggering for that subject on that target. `agent_personal`: agents with `billing_mode = personal` with their trigger set (the I1 holder and anyone else who may trigger, which I1 forbids; the hint shows the set the owner must keep at one person).

2. **Scope and cost** (07 2.6). For a space: only agents operating a channel of the space, only channels of the space in H(A), only grants on targets within the caller's `manage` scope. The computation runs per request over the permission states held by the cache of M1-28; measure it with the test world scaled to 50 agents and 500 people and record the time; if it exceeds the request deadline of 02, page by agent and compute per page (the cursor carries the agent position).

3. **Expiring** (07 9.4). Grants and memberships with `expires_at` within `now + authz.expiry_reminder_lead`, not revoked; tokens of agents and apps (not personal tokens) with `expires_at` in the same window, from `identity.TokenExpiry`; ordered by expiry. For a space manager only grants on targets within the scope and memberships of groups whose grants fall in the scope.

4. **Screens** (07 9.3, 9.4). Each hint shows a sentence from a template in the caller's language ("Sam can learn content of Engineering/Intern through agent Texter."), the kind as icon and text, and actions "Explain" (A3 for the channel) and, for grants, "Revoke" or "Extend". "Extend" opens the grant wizard with the subject, target, level and capabilities of the old grant, an empty reason and a new expiry; confirming creates the new grant, then revokes the old one, both with step-up (one step-up covers both within `auth.step_up_max_age`).

5. **Rulings for `docs/DECISIONS.md`** (`D-M6-07-<n>`): one H1 function for explain and hints (note 1); the definitions of the other three kinds (note 1); scope and paging (note 2); the expiring window and token kinds (note 3); "Extend" as create then revoke (note 4).

## Tests (write them first)

Server (`pgtest`, full chain):

- `TestHints_H1` (PLAN row): agent Texter operates Marketing and may read Marketing/Kampagnen and Marketing/Intern; Sam has `trigger` on Marketing/Kampagnen only; `listPermissionHints` for the owner contains exactly one `h1_read_horizon (Texter, Sam, Intern)`; granting Sam `read` on Intern removes it; the same check through `explainChannelAccess` still shows `via_agent` (one function).
- `TestHints_OtherKinds`: a grant to a disabled person, a `trigger` grant blocked by SR2 (tool class) or SR3 (dev environment not ready, where SR3 data exists; otherwise the test uses the SR2 case only and the report says so), an agent in mode `personal` with its trigger set.
- `TestHints_Scope`: Sam as manager of Support sees only Support's hints; with `space_id = Engineering` `404` or `403` as 07 4.1 decides; Robin without `manage` gets `403`.
- `TestExpiring`: grants, memberships and tokens inside and outside `authz.expiry_reminder_lead` with a fake clock; personal tokens never listed; token values never in the response.
- `TestHints_Paging` and the measured time of Design note 2.

Client (widget and integration with the fake server): the two screens in light and dark, text scaling 1 and 2, de and en; "Extend" creates then revokes with one step-up; "Explain" opens A3.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/authz/... ./internal/identity/... ./internal/chat/... ./internal/wire/...`
- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `make codegen-check`, `make api-lint`, `make api-checks`, `make arch`, `make register-check`, `make catalog-check`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Integration test with an agent whose read horizon is larger than the permissions of a triggering person gives the H1 hint.
- [ ] The list contains exactly the four kinds of 07 9.5, from the permission state of `authorize`; scope and errors follow 07 2.6 and 4.1.
- [ ] "Expiring soon" lists grants, memberships and agent and app tokens within `authz.expiry_reminder_lead`; "Extend" creates then revokes.
- [ ] The operations are in `openapi.yaml`; the screens are in the desktop administration and the console.
- [ ] 07 9.4 and 9.5, READMEs, ARB files, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Go and Dart symbol; comments explain why.
- READMEs of `authz` and `admin_ui` updated in English.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
- Root and server `CHANGELOG.md` entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The measured computation time of the hints for the scaled world.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
