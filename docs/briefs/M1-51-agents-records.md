# M1-51: Agent records, backend policy and credential holder

Status: ready | Depends on: M1-31, M1-44 | Milestone: M1

## Goal

After this ticket agents exist as principals with their operating record: adapter kind,
description, operator configuration, configured backend policy (`personal` or `organization`),
credential holder and configured tool class, without any runtime parts (P-02). Owners create,
change, disable, enable and switch agents over the API with step-up, reason and audit, and every
change that could break I1 is checked in its own transaction. The module `agents` becomes the real
`authz.AgentPolicySource`, so the permission core decides with the actual policy instead of a fake.
This implements ADR 0024 for the data and administration side.

## Read first

- `docs/PLAN.md`, section 7: row M1-51, "Prerequisites from the core contract" (rows with
  `agent.enable` and "`agent.read` for the agent itself"), "Exit criteria" item 3, "Red team" item
  3; section 19 P-02; section 3 "Way of working per ticket". `docs/briefs/README.md`.
- `docs/adr/0024-backend-policy-personal-organization-with-invariant.md`.
- `docs/spec/10-backend-policy.md`: "3. Invariants", "5. I1 transactional" (5.1 to 5.4),
  "8. Expected and reported", "9. Switching (I4)", "11. Guided flow" (context only), "13. Display
  in the app", "14. Audit trail", "15. Tests" (BR-01, BR-03, BR-08).
- `docs/spec/09-agents.md`: "2. Agents as principals" (2.1 to 2.4).
- `docs/spec/07-permissions.md`: "2.9 Agents as principals", "2.11 Tool class of an agent",
  "4.3 Visibility" (agent visibility), "4.4 Invariants in the transaction", "SR1", "SR5",
  "SR7", "11.1 Test matrix" (AZ-M22).
- `docs/spec/05-actions.md`: rules 1.6 and 1.7, "2.4", "4.10" rows `agent.read` to
  `agent.credential_holder.set`, "6" row `agent.updated`.
- `docs/spec/03-data-model.md`: tables `agents`, `principals`, `users`, `api_tokens`;
  "1. Principles" (3, 8, 10, 13), "6.3 Lock order", "6.4 I1 check".
- `docs/spec/08-identity.md`: "12. API tokens for agents, apps and tools".
- `docs/spec/02-architecture.md`: "4.3" (module `agents`: port `Agents`, In-Tx event
  `AgentDisabled`, source `authz.AgentPolicySource`), "4.4" (`agents` may use `identity`, `authz`,
  `chat`), "5. Layers within a module", "8.1 Flow".
- `docs/spec/06-api.md`: "7. Idempotency", "8. Concurrency" (If-Match for agents, backend
  policy, credential holder), "16. Endpoints" group "Agents".
- `docs/api/openapi.yaml`: operations `listAgents`, `createAgent`, `getAgent`, `updateAgent`,
  `disableAgent`, `enableAgent`, `setAgentBillingMode`, `setAllAgentsBillingMode`,
  `setCredentialHolder`; schemas `Agent`, `AgentPage`, `AgentCreate`, `AgentUpdate`,
  `AgentRuntimeStatus`, `BillingMode`, `BillingModeSet`, `BillingModeSetAllResult`,
  `CredentialHolderSet`, `ReasonBody`, `ToolClass`, `AgentPayload`, `AgentUpdatedEvent`.
- `docs/spec/01-product.md` wireframe A5.
- Code already merged: `internal/authz` (snapshot, `authorize`, phase hook with I1 and
  `VerifyInvariants`, declared `AgentPolicySource`, fake-source harness), `internal/identity`
  (M1-26 `Directory`, M1-31 people and owners, M1-34 sessions, M1-35 step-up, M1-44 API tokens),
  `internal/kernel/*`, `internal/platform/pg/pgtest`, `services/server/api/operations.yaml`.
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/quality/red-team-gate.md`.

## Fixed requirements that apply

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

Note on rule 3: In this repository `docs/api/openapi.yaml` is normative and the server interface is
generated from it (ADR 0013); the route test keeps both identical.

6. Test first: every behaviour gets a test that fails before the code exists (red), then passes
   (green). Keep the red output for the report.
7. No en dash or em dash characters (U+2013, U+2014) in any file or commit message.
8. White label: no company, person, host or domain names except Alex, Sam, Robin, the agents of
   07 "10. Examples" and `example.com`; no real model, profile or machine names.
9. Limits and durations only through named keys from `docs/operations/configuration.md`; no bare
   numbers in production code or documentation.
10. Specification first: API and chapter changes below land before the code that uses them.
11. Product requirements implemented here: I1 at every policy, holder and enable change (10 "5.1",
    05 rule 1.6), I4 (switching only by an owner with step-up and reason, audited, global or per
    agent; to `personal` only if I1 holds afterwards), I5 and B10 (no credentials in the server;
    nothing in this ticket stores a key, token value or subscription data), `agent.read` for the
    agent itself, `agent.updated` carrying the badge change, disabling revokes the agent's tokens
    (09 "2.1").

## Scope

The specification is in place (planning, spec first): `BillingModeSetAll` as the body of
`setAllAgentsBillingMode` and `minLength: 1` on `reason` of `BillingModeSet` and
`CredentialHolderSet` in `docs/api/openapi.yaml`; 09 "2.2" with the table of
`agent_operator_config.v1`; 10 "5.1" (I1 over active agents, `agent.enable` checks this agent) and
"5.2"; 05 row `agent.updated` (also at creation); 06 2 "No effect". Change them only where the
implementation proves them wrong; run `make api-lint`, `make api-checks` and the M1-17 generation
target; `make codegen-check` shows no diff.

Code (paths under `services/server/`):

- Migration (next free sequential goose version in `migrations/`): table `agents` exactly as 03
  (all columns, checks, FKs, indexes) plus the trigger that requires `principals.kind = 'agent'`.
- `internal/agents` (port): interface `Agents` with `List`, `Get`, `Create`, `Update`, `Disable`,
  `Enable`, `SetBillingMode`, `SetBillingModeAll`, `SetCredentialHolder`, `SetToolClass`; DTOs;
  In-Tx event
  `AgentDisabled{AgentID ids.ID}` (no subscriber in M1; `runs` and `status` subscribe later).
- `internal/agents/domain`: agent entity, backend policy value with transitions, tool class order
  `chat < scoped < host` and effective class, `OperatorConfigV1` validation, presence view.
- `internal/agents/app`: one use case per operation (authorize, unit of work with the
  `RightsChange` option where "Permissions" = `yes`, domain, repository, events, audit).
- `internal/agents/adapters/postgres` (sqlc queries and repository),
  `internal/agents/adapters/httpapi` (handlers of the ten operations),
  `internal/agents/adapters/authzsource` (implements `authz.AgentPolicySource`).
- `internal/identity` port additions, only where the base lacks them: create a principal of kind
  `agent` (handle and display-name rules of M1-26, case-insensitive handle uniqueness), set
  principal status with `disabled_at`, update display name, revoke all API tokens of a principal.
  Implement them in identity's adapters with tests.
- `internal/wire`: register handlers and replace any placeholder `AgentPolicySource` with the
  agents implementation; `services/server/api/operations.yaml`: enable the ten operations
  (including `setAgentToolClass`).
- Docs: `internal/agents/README.md` (ports, tables, actions, events, rules),
  `services/server/CHANGELOG.md` under Unreleased, rulings in `docs/DECISIONS.md`.

## Out of scope

- Heartbeat, presence events, reported state and the mismatch alarm (M2-20); runs, invocation
  token, I2 and I3 (M2); costs (M2-22); consumers, commands, agent sessions (M2).
- Operators of spaces and channels and the I1 integration test with all real sources (M1-54).
- Creating API tokens for agents (M1-44, merged; `token.create`).
- Guided flow `resolve_i1` (needs the console; its schema field is not in the agent operations).

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on branch `feat/M1-51-agents-records`.
- Conventional Commits with scope (`feat(agents): ...`, `feat(identity): ...`, `docs(api): ...`)
  and DCO sign-off.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except the CHANGELOG and docs entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

Catalog facts (05 "4.10"): every mutating action is owner only, step-up, audited;
"Permissions" = `yes` for `agent.create`, `agent.disable`, `agent.enable`, `agent.billing_mode.set`,
`agent.billing_mode.set_all`, `agent.credential_holder.set`; `agent.update` is not a permission
change.
All use class `rl.admin`; `agent.read` uses `rl.read`.

Visibility and errors:

- `agent.read`: owner, the agent itself (`agent-self`), or a principal with `read` on a channel the
  agent serves (07 "4.3"). Not visible gives 404 with the body of an unknown ID; visible but not
  owner gives 403 (`owner_only`); owner without fresh step-up gives 401 `step_up_required`; a
  personal token of an owner has no owner permissions (SR5).
- `listAgents`: cursor pagination by principal ID; candidates from the table, each item filtered by
  `authorize(p, agent.read, agent)` (07 "7.4" rule 3); no totals.

Record and representation:

- `Create` (Idempotency-Key): principal (kind `agent`, status `active`) through the identity port,
  then the `agents` row with `billing_mode = organization`, `tool_class = chat`,
  `reported_runtime_state = unknown`, `version = 1`. Handle taken (also by a human, case
  insensitive) gives `409 conflict`. Display name with control or bidi characters gives `400
  validation_failed` (SEC-T07, part agents) through identity's existing validator.
- `adapter_kind`: required, pattern `^[a-z0-9][a-z0-9_-]*$` (examples `hermes`, `code_runner`,
  `custom`).
- `operator_config` schema `agent_operator_config.v1`: JSON object, `additionalProperties: false`,
  optional keys `default_model` (string, pattern `^[A-Za-z0-9._:/@-]+$`), `supports_followups`,
  `supports_components`, `supports_voice`, `supports_title`, `supports_files_out` (booleans; an
  absent key means allowed, because the owner switches features off, 09 "2.2"). Never secrets.
  Invalid config gives `400 validation_failed` with the JSON pointer.
- `Agent` response: `status` from `principals.status`; `presence` from the `reported_*` columns:
  `state`, `reported_billing_mode`, `runtime_version`, `last_heartbeat_at`,
  `billing_modes_supported` (from `reported_details`), `reported_tool_class`, and
  `billing_mode_matches = false` only when a reported mode exists and differs. `ETag` and
  `If-Match` use `agents.version`; every change of the record or of the agent's display name bumps
  it. No `null`.
- Event `agent.updated` (payload `AgentPayload` with the full agent, resource
  `{kind: agent, id, version}`, empty scope) after every successful mutation including create;
  there is no `agent.created` type in 05. Receive action `agent.read`, so it also reaches the agent
  itself.

Backend policy (10 "9"):

- `SetBillingMode` (If-Match, `BillingModeSet`): to `organization` always allowed;
  `credential_holder_id` becomes NULL and a given holder gives `400`. To `personal`: holder required
  and must be an active human, else `400 validation_failed` with pointer
  `/credential_holder_id`. Same target as current (mode and holder) returns `200` with the current
  agent and writes nothing. Sets `billing_mode_changed_at`, `billing_mode_changed_by`.
- `SetBillingModeAll` (no If-Match, `BillingModeSetAll`): `organization` without `agent_ids`
  affects every agent not yet on `organization`, including disabled agents (emergency switch);
  with `agent_ids` only those. `personal` requires `agent_ids` and `credential_holder_id`. Unknown
  IDs give `400` (pointer `/agent_ids/<index>`). One transaction, rows locked in ascending ID, all
  or nothing (BR-08). Response `changed_agent_ids` (only actually changed agents). One audit entry
  with the list of agents, old and new values and the reason.
- `SetCredentialHolder` (If-Match): only for `personal` agents, else `409 conflict`; holder must be
  an active human.
- Every policy, holder and enable change runs with `RightsChange` naming the affected agents, so
  the authz phase hook bumps `permission_version`, computes diffs (usually none for humans; the
  bump invalidates every cache that holds policy facts) and runs I1. A violation rolls everything
  back and returns `409` `i1_violation` with the problem body M1-29 defines; the deny entry is
  written in its own transaction (07 "4.4").
- Audit details per 10 "14": old and new mode, old and new holder, reason; never names or values.

Disable and enable:

- `Disable` (`ReasonBody`, Idempotency-Key): status `disabled` and `disabled_at` through identity,
  revoke every API token of the agent through identity, publish `AgentDisabled`, `agent.updated`,
  `RightsChange` with the agent. Disabling a disabled agent succeeds without change: `200` with
  the agent, no event, no permission version, audit with `no_change: true` (06 2 "No effect").
  Gateway connections close through the existing revocation path (M1-65).
- `Enable` (`ReasonBody`): back to `active`; I1 is checked for this agent. Enabling an active agent
  succeeds without change in the same way. Tokens are not restored; the owner creates new ones.
- `SetToolClass` (`setAgentToolClass`, `PUT /agents/{agent_id}/tool-class`, `agent.tool_class.set`,
  owner, step-up, `If-Match`, body `ToolClassSet` with `tool_class` and `reason`): sets
  `agents.tool_class`, bumps `version`, `agent.updated`, `RightsChange` with the agent (SR2 reads
  the effective class, 07 2.11), audit with old and new class and the reason. The same class
  succeeds without change (06 2 "No effect").
- I1 considers active agents only: `AgentPolicySource` reports personal agents whose principal
  status is `active`. Reason: a disabled agent cannot be triggered, and `agent.enable` exists in
  the I1 list (05 rule 1.6) precisely because enabling can create a violation that arose while the
  agent was disabled. If the merged I1 code queries agents differently, adapt the source, not the
  invariant, and keep AZ-P7 green.

`AgentPolicySource` (implement the interface as declared in the base branch; these facts must be
available): status, `billing_mode`, `credential_holder_id`, effective tool class (higher of
`tool_class` and `reported_tool_class`), `billing_modes_supported`, readiness of a dev environment
(always "none ready" until V2), and the list of active personal agents for I1. A shared contract
test runs against the authz fake and this implementation.

Rulings to record in `docs/DECISIONS.md` (the others are in 05, 06, 09 and 10; link them):
`adapter_kind` pattern; lock order of `SetBillingModeAll`.

## Tests (write them first)

App tests use the authz fake-source harness (fake structure: Assistent serves Research/Notizen);
integration tests use `pgtest` with real authz, identity and agents and a fake `StructureSource`
where channels are needed (the `channels` table arrives with M1-53). Test names carry the IDs.

- `TestOperatorConfigV1Validation`: unknown key, wrong type, bad `default_model` rejected; empty
  object and full object accepted.
- `TestBillingPolicyTransitions` and `TestEffectiveToolClass` (domain tables).
- `TestAZM22_SwitchToPersonalWithForeignTriggerIsRejected`: Alex has `trigger` on a served channel
  (fake structure); Owner sets Assistent to `personal`: `409 i1_violation`, nothing changed, deny
  audited in its own transaction.
- `TestBR01_SwitchToPersonalWithInstanceTriggerGrant` (integration, no channels needed): Sam has
  `trigger` on the instance; switching to `personal` fails as above.
- `TestBR01_CredentialHolderChangeIsChecked`: a second owner exists, holder change to a person who
  is not the sole owner fails with `i1_violation`.
- `TestBR01_EnableIsChecked`: Assistent `personal`, disabled; meanwhile Sam gets `trigger` on the
  instance (allowed, agent inactive); `enableAgent` fails with `i1_violation` and the agent stays
  disabled.
- `TestBR03_ParallelChangesSerialize`: a policy switch to `personal` and a parallel grant that only
  together break I1: exactly one succeeds.
- `TestBR08_SetAllPersonalWithOneViolatorChangesNothing`: list [A, B], B violates: `409`, A and B
  unchanged, problem lists B.
- `TestSetAllOrganizationWithoutListSwitchesAll` and `TestSetAllPersonalRequiresListAndHolder`.
- `TestCreateAgent`: principal and record exist, defaults as specified, `agent.updated` in
  `event_log`, audit allow, `permission_version` bumped; repeat with the same Idempotency-Key
  returns the stored response.
- `TestCreateAgentHandleConflictCaseInsensitive` (`Alex` vs existing user `alex`) and
  `TestCreateAgentRejectsBidiDisplayName`.
- `TestAgentMutationsOwnerStepUpAudit`: table over all seven mutating operations: visible
  non-owner `403`, invisible `404`, owner without fresh step-up `401`, owner personal token `403`,
  owner with step-up success with one audit entry.
- `TestUpdateAgentIfMatch`: missing `428`, stale `412` with `current`, fresh `200`, version + 1.
- `TestSetBillingModeOrganizationClearsHolder`, `TestSetBillingModeHolderMustBeActiveHuman`,
  `TestPolicyReasonRequired` (empty reason `400`), `TestPolicyUnchangedWritesNothing`.
- `TestDisableRevokesTokens`: an agent token works before and gives `401` after `disableAgent`;
  `AgentDisabled` was published in the transaction (test subscriber); repeat gives `409`.
- `TestAgentReadsItself`: with its own token the agent reads itself including `billing_mode` and
  `credential_holder_id`; another agent it does not serve is `404`.
- `TestListAgentsVisibility`: owner sees all; Sam with `read` on a channel served by Lotse sees
  Lotse only; Robin sees none.
- `TestAgentUpdatedCarriesBadge`: after each switch the `agent.updated` payload carries the new
  `billing_mode`, and `authorize(agent, agent.read, itself)` allows its delivery.
- `TestAgentPolicySourceContract`: shared contract suite against fake and real source, including
  "disabled personal agents are not in the I1 list".
- Every response validates against openapi.yaml in the test build and contains no `null`.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/agents/... ./internal/identity/... ./internal/authz/...
make codegen-check
make api-lint
make migrations-lint
make check
```

## Acceptance criteria

- [ ] BR-01 for switching, holder change and enable, BR-03 for a policy switch, BR-08 and AZ-M22
      (with fake structure) are green.
- [ ] Switching needs owner, step-up and reason and is audited with old and new values.
- [ ] `agent.read` works for the agent itself; visibility follows 07 "4.3".
- [ ] `agent.updated` follows every mutation and carries the badge (`billing_mode`).
- [ ] Disabling revokes the agent's tokens and publishes `AgentDisabled`; disabling a disabled and
      enabling an active agent succeed without change.
- [ ] `setAgentToolClass` sets the class with owner, step-up, reason and audit; a test
      `TestSetToolClass` covers change, same class and missing `If-Match`.
- [ ] The agents module is the wired `AgentPolicySource`; AZ-P7 and AZ-P8 stay green.
- [ ] No credential, key or subscription value is stored or logged (review of the diff).
- [ ] The implementation matches the specification (changed only where proven wrong); `make api-lint`, `make api-checks`,
      `make codegen-check`, `make migrations-lint` are green; the ten operations are enabled.
- [ ] `make check` is green.

## Definition of done

Code and tests as above; `internal/agents/README.md` and the identity README updated; doc comment
on every public symbol; ADR 0024 is implemented, write a new ADR only for an architecture decision
beyond the rulings listed; `services/server/CHANGELOG.md` entry under Unreleased; rulings in
`docs/DECISIONS.md`; `make check` green; an independent review and the red team
(`docs/quality/red-team-gate.md`) over the diff with the verdict recorded; merge locally into the
base branch only if both are green, then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits,
files with line counts, test results with counts (including the red runs), what is not proven,
follow-ups that could not be done in scope with reason. Then write `{{DONE_PATH}}` with
`exit=0 sha=<sha>` or `exit=1 <reason>`.
