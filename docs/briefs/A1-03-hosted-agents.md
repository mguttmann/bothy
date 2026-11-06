# A1-03: Server: hosted agents

Status: ready | Depends on: none (specification merged) | Milestone: A1

## Goal

After this ticket the server knows which Claude Code runner hosts which built-in agent and gives each built-in agent's token only to that runner. `createAgent` accepts `hosted_by` together with `adapter_kind = 'claude_agent'`; a runner reports the capability `hosts_agents` with its class ceiling in the heartbeat; `agent.host.set` re-hosts an agent; `listHostedAgents` and `issueHostedAgentToken` serve the hosting runner; every token of a hosted agent is revoked with the agent, a host change, the runner and every token of the runner with the same key. People never get such a token, built-in agents never serve project forums, and the billing rule of 10 10.3 holds. Existing agents and adapters see no change.

## Read first

- `docs/spec/26-builtin-agents.md`: 1, 2, 3 (all), 9, 10, 11, 12 (BA-T04 to BA-T08), 13 (R-BA-01 to R-BA-03, R-BA-12, R-BA-14), 14 (`agents.hosted_token_ttl`).
- `docs/spec/07-permissions.md` 2.9, 2.11, 2.13, SR1, SR2; `docs/spec/05-actions.md` 4.3 (`token.create`, `token.revoke`), 4.10 (`agent.create`, `agent.disable`, `agent.heartbeat`), R-AZ-36; `docs/spec/06-api.md` 7 (one-time secrets), R-API-29.
- `docs/spec/03-data-model.md` 4.2 `api_tokens`, 4.4 `agents` (`hosted_by`, its checks and index), R-DM-58; `docs/spec/08-identity.md` 12 (API tokens, DPoP binding).
- `docs/spec/09-agents.md` 2.1, 2.2 (the row `hosts_agents`), 4.3, 4.8 (heartbeat, closed capability keys), 18.16 (R-AG-39); `docs/spec/10-backend-policy.md` 10.3 and BR-14.
- `docs/spec/04-sync-protocol.md` 15.1 (identify, `4012` on revocation).
- ADR 0076, ADR 0061 (token format), ADR 0049.
- Design note `docs/design/builtin-agents.md` D-A1-1 to D-A1-6, D-A1-19 and backlog items 4, 5 and 8; `docs/decisions.d/A1-01.md` D-A1-01-4, D-A1-01-9, D-A1-01-13.
- Code on the base branch: `services/server/internal/agents` (`app/create.go`, `app/lifecycle.go`, `app/heartbeat.go`, `app/heartbeats.go`, `app/switch.go`, `README.md`), `services/server/internal/identity` (tokens: create, revoke, DPoP binding), `services/server/internal/chat` (`putProjectBinding`, `setChannelOperator` and the R-AG-39 check), `services/server/internal/authz`, `services/server/migrations`, `docs/api/openapi.yaml`, `docs/api/asyncapi.yaml`, `docs/spec/actions.yaml`.

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
8. White label: examples, templates and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no internal script names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it: the catalogue rows of `docs/spec/actions.yaml` that R-AZ-36 assigns to this ticket and their generated copies (`make catalog`).
11. Additive only (26 section 11): every new column is nullable or defaults to "as before", every key defaults to off, and an installation that sets none of the new keys behaves exactly as before.

Note on rule 1: "stages" are the milestones and the stage A1 of the plan (PLAN P-27).

Product requirements implemented here:

12. `agents.hosted_by` is set exactly when `adapter_kind = 'claude_agent'`, names an active `code_runner` agent that is not hosted itself and whose last heartbeat reports `hosts_agents`; otherwise `400 validation_failed` with `host_not_capable` on `/hosted_by` (26 3.1, BA-T07).
13. The heartbeat accepts the capability `hosts_agents` with the value `chat`, `scoped` or `host`; a hosted agent's class above its host's ceiling is refused with `409 conflict`, reason `host_tool_class_ceiling`, by `agent.tool_class.set` and `agent.host.set` (26 3.1 item 3).
14. `issueHostedAgentToken` issues a token only to the runner named in `hosted_by`, bound to the `dpop_jkt` of the calling token, with `expires_at` after `agents.hosted_token_ttl`, revoking older tokens of that agent from the same runner, with one audit entry without the value; every other caller gets `404`; a repeated request with the same `Idempotency-Key` answers `409 conflict` with `one_time_secret` (26 3.3, R-API-29, BA-T05).
15. Revocation of 26 3.3 item 3 in the same transaction as its cause, with the connections of the revoked tokens closed with `4012` (BA-T06).
16. `token.create` for a built-in agent answers `409 conflict`, reason `hosted_agent_token`; a built-in agent never serves a bound forum (`operator_cannot_serve_project`, R-AG-39, BA-T08).
17. 10 10.3: `personal` for a hosted agent only with the runner's `credential_holder_id`; every other combination and a runner change that would break it answer `409 conflict`, reason `hosted_holder_mismatch` (BR-14).

## Scope

All paths under `services/server/` unless they start with `docs/` or name the root `Makefile`.

Create:

- Migration `<next number>_agents_hosted_by.sql`: the column `agents.hosted_by` with its foreign key, the two checks and the partial index of 03; an expand step only. The check `(adapter_kind = 'claude_agent') = (hosted_by IS NOT NULL)` would fail on an existing row with that adapter kind; the value is new with this stage, so the migration first counts such rows and fails with a clear message naming the agent IDs when there are any (no data change by the migration).
- `internal/agents/app/hosting.go`: `SetHost`, `ListHosted`, `IssueHostedToken` and the revocation hooks; `internal/agents/app/hosting_test.go`.
- `docs/decisions.d/A1-03.md`.

Change:

- `internal/agents/app/create.go`: `hosted_by` on create with the checks of requirement 12; `adapter_kind = 'claude_agent'` without `hosted_by` is `400 validation_failed` on `/hosted_by`.
- `internal/agents` heartbeat: accept `hosts_agents` (closed key list of 09 2.2) and keep it in `reported_details.capabilities`; a hosted agent's own heartbeat never carries it.
- `internal/agents` policy: the ceiling check of requirement 13 and the billing rule of requirement 17 in `agent.billing_mode.set`, `agent.credential_holder.set`, `agent.host.set` and the runner's own mode or holder changes.
- `internal/identity`: token issue for a hosted agent through a port that only `agents` calls after `authorize`; the cascade of requirement 15 on `token.revoke` of a runner token and on `agent.disable` of a runner or a hosted agent; `token.create` refusal of requirement 16.
- `internal/chat`: no new rule is expected (R-AG-39 already admits only `code_runner`); add the test of BA-T08.
- Durable event `agent.hosting_changed` (payload `agent_id`) to the old and the new host on create, host change and disable of a hosted agent.
- `internal/wire`: the key `agents.hosted_token_ttl`; `make config-reference`.
- `docs/api/openapi.yaml` (`createAgent` member `hosted_by`, `setAgentHost`, `listHostedAgents`, `issueHostedAgentToken`, the heartbeat capability, the problem reasons), `docs/api/asyncapi.yaml` (`agent.hosting_changed`), `services/server/api/operations.yaml`, `docs/spec/actions.yaml` (`agent.host.set`, `agent.hosted.list`, `agent.hosted_token.issue`, the event; columns of 07 2.13), `make catalog`, `make event-catalog`, `make contract-documents` and every generated copy of `make codegen-check`.
- `services/server/architecture/modules.yaml`, `internal/agents/README.md`, `internal/identity/README.md`.

## Out of scope

- Definitions (A1-02); anything in the runner (A1-04, A1-05); clients (A1-08, A1-09).
- A hosted agent serving a project forum (a later ruling).
- Moving existing tokens; existing agents keep their tokens and rows.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/A1-03-hosted-agents`; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers; squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, DPoP keys, secret values and secret references resolved to values never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money, except a test behind the explicit switch of the runner's real CLI tests when this brief names one and the launcher allows it.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/26-builtin-agents.md`, stop with `Verdict: BLOCKED <reason>`.
- The token value appears only in the one response of `issueHostedAgentToken`; it is stored as a hash like every API token (ADR 0061) and never in an idempotency record (R-API-26).

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red` and the failing gate. Confirm that `docs/spec/26-builtin-agents.md` and `agents.hosted_by` in 03 exist on the base, else `Verdict: BLOCKED specification missing`.

2. **Authorization of the issue.** `agent.hosted_token.issue` is agent-self of the caller with the target in the path: `authorize` resolves the target agent and allows only when its `hosted_by` equals the caller's principal, the target is active and the caller is active and its stored capabilities report `hosts_agents`; every other case is the `404` of an unknown agent (07 2.13 item 3), audited as a denial like other refusals.

3. **Token row.** Kind `agt` (ADR 0061), `principal_id` the hosted agent, `created_by` the hosting runner, `dpop_jkt` copied from the calling token (a caller without `dpop_jkt` is refused with `409 conflict`, reason `dpop_required`, because a hosted token must be key-bound), `expires_at` = now plus `agents.hosted_token_ttl`, `name` `hosted by <runner handle>`. In the same transaction every active token of the hosted agent with the same `created_by` is revoked (`revoked_by` the runner), so at most one hosted token per agent and runner is active.

4. **Cascade.** A hosted token is one whose `created_by` is the agent named in its holder's `hosted_by` at issue time. Revocation by the rules of 26 3.3 item 3: `agent.disable` of the hosted agent revokes its tokens as for every agent; `agent.host.set` revokes the tokens `created_by` the old host; `agent.disable` of the runner and `token.revoke` of a runner token revoke the hosted tokens `created_by` the runner whose `dpop_jkt` equals that of the revoked runner token (all of the runner's on disable). The gateway closes the connections of every revoked token with `4012` through the existing revocation path.

5. **Ceiling.** The ceiling is the value of `hosts_agents` in the host's last stored heartbeat. A host that stops reporting it keeps its hosted agents; their runs fail on the runner side (A1-04), and `agent.host.set` to it is refused with `host_not_capable`.

6. **Billing.** Requirement 17 is one function in `agents` used by every action it names; the I1 machinery of SR1 stays as it is and runs after it.

7. **Events.** `agent.hosting_changed` is a personal durable event to a runner (receive action `agent.hosted.list`); the payload carries only `agent_id`. `agent.updated` also follows `hosted_by`.

8. **Rulings for `docs/decisions.d/A1-03.md`**: the token name, the `dpop_required` refusal, the cascade query, the migration precheck.

## Tests (write them first)

- Go unit and integration (container runtime), `internal/agents`, `internal/identity`, `internal/chat`:
    - BA-T05: issue by the hosting runner (token works only with the runner's DPoP key), by another runner (`404`), for an agent hosted elsewhere (`404`), for a disabled agent (`404`), by a person (`404`); the audit entry has the prefix, never the value; a repeated `Idempotency-Key` answers `one_time_secret`.
    - BA-T06: each revocation cause of design note 4 revokes exactly the tokens it names and closes their gateway connections with `4012`; the runner's own tokens are untouched by a hosted agent's disable.
    - BA-T07: `createAgent` with `hosted_by` naming an agent without `hosts_agents`, a hosted agent, itself, a non-runner; `claude_agent` without `hosted_by`; `hosted_by` with another adapter kind.
    - BA-T08: `putProjectBinding` and `setChannelOperator` on a bound forum with a built-in agent answer `operator_cannot_serve_project`.
    - BA-T04 hosting part: each action of 07 2.13 owned here refuses a non-owner or a wrong agent as the table says.
    - Ceiling: `agent.tool_class.set` above the ceiling and `agent.host.set` to a lower ceiling give `host_tool_class_ceiling`.
    - BR-14 of 10.
    - `token.create` for a built-in agent gives `hosted_agent_token`.
- Migration test: the column, checks, foreign key and index; the precheck fails with the agent IDs on a database with a fixture row of adapter kind `claude_agent`.

```sh
cd services/server && go test -race -count=1 ./internal/agents/... ./internal/identity/... ./internal/chat/... ./migrations/...
make catalog && make catalog-check
make config-reference && make config-doc-check
make check
```

## Acceptance criteria

- [ ] BA-T04 (hosting part) to BA-T08 and BR-14 are green.
- [ ] A hosted token is issued only to its hosting runner, key-bound, short-lived, audited without its value, and revoked with every cause of 26 3.3 item 3.
- [ ] Existing agents and their tokens are unchanged by the migration and the new code (an integration test with a runner and a Hermes-like agent created before the migration).
- [ ] The catalogue, the API documents, the configuration reference and every generated copy are current; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `internal/agents/README.md` and `internal/identity/README.md` describe hosting and hosted tokens.
- An entry `changelog.d/A1-<nn>.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/<ticket>.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0076 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: no path gives a hosted token to anyone but its hosting runner; every revocation cause revokes in the same transaction; no token value in logs, audit or idempotency records; built-in agents cannot serve project forums; `personal` cannot leave the runner's holder.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the behaviour of a real runner with the new operations (A1-04)), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
