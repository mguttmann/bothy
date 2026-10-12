# M2-14: Acting-for binding (SR9)

Status: ready | Depends on: M2-09 | Milestone: M2

## Goal

An agent that uses the server API inside a run presents the run's invocation token in the request header `Invocation-Token`; the server checks it on every request (signature, audience, run active) and binds the request to the person who triggered the run, so that the agent has at most that person's rights at that moment: every action that reads content or writes into a channel is decided on the intersection of the agent's and the person's effective rights, and `Scope` gives every list, counter, snapshot and catch-up of that request only channels both may read. Only the exceptions of 07 "SR9" (replies in the run's thread, patches of the own run, `run.finish`, `interaction.respond`, `agent_session.report`) use the agent's own rights, and only for the own thread and run. Without the header the agent keeps its own rights and the console shows hint H1. This closes the confused deputy over the server API (T-AI-02, T-AI-04, SEC-AI-3): a person with `trigger` on Support can no longer make an agent that also reads Engineering summarise Engineering for them, and a revocation stops the agent's tool calls at once. After this ticket AZ-M44, AZ-M45 and AZ-P11 are green (M2 exit criterion 3).

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope, Out of scope, Exit criteria 3 and 10, Test strategy, Red team first paragraph: "confused deputy through the server API", Risks: prompt injection) and the block introduction "Block agents and extensions"; rows M2-06 to M2-09 (what exists), M2-14 (this ticket), M2-10, M2-12, M2-13, M2-15, M2-19 (operations of the SR9 exceptions and of run reads, built by others), M2-29, M2-30, M2-31 (SDKs and conformance suite), M3-14 (search in a run); M1 rows M1-28, M1-49, M1-70.
- Dependencies: the header names M2-09. Through it this ticket also builds on M2-08, M2-07, M2-06, M2-05, M2-04 and M2-02 and on the M1 tickets M1-12, M1-13, M1-18, M1-24, M1-25, M1-28, M1-29, M1-32, M1-33, M1-34, M1-44, M1-45, M1-48, M1-49, M1-51 to M1-56, M1-60, M1-61, M1-63 to M1-65 (all implied, so not in the header). M1-70 is not in the closure: do not rely on `services/server/test/leak` or the SQL visibility lint of M1-70; use them only if they are on the base branch. M1-71 is in the closure (through M2-02 and M1-69), so its shared test harness `services/server/test/harness` is on the base branch.
- `docs/spec/07-permissions.md`: 1 "Principles" (points 5 and 7); 2.1, 2.3, 2.7 "Owner", 2.9 "Agents as principals" (point 6); 3 "Principal context" (row `acting_for`); 4.1 "Decisions and reasons" (rows `acting_for_insufficient`, `invocation_token_invalid`); 4.2 "Flow" (step 4 `if p.acting_for`, step 5, the audit paragraph, filter mode); 4.3; "SR5 Administration only by humans with confirmation"; "SR7 Disabled means no permissions"; "SR9 Acting-for binding (confused deputy)" completely; 6 "Read horizon of agents" (H1); 7.2, 7.3, 7.4 "Query pattern" (rule 1: `Scope` with `acting_for`); 8.1 "Delivery filter" (row "Agent consumers"); 8.2 "Immediate effect on revocation" (point 4); 9.1 "Who has access?" (`via_agent`); 10 "Examples"; 11.1 rows AZ-M44, AZ-M45; 11.2 rows AZ-P8, AZ-P11; 12 (R-RE-06).
- `docs/spec/05-actions.md` (core contract, read only): 2.2 "Target of a resource", 2.3 "Effective level" (point 1: owner rights only in a session, with the exception of R-AZ-29; point 6: acting-for binding), 2.5 "Decision", 2.7 "Effect (`effect`)", tables 4.6, 4.7, 4.9 and 4.10 (rows `message.create`, `run.update`, `run.finish`, `interaction.respond`, `agent_session.report`, `run.start`, `run.token.renew`, `run.context.read`, `agent.events.consume`), 5 "Agent invocation `agent.invoke`" (claims `sub`, `run_id`, `thread_id`, `trigger_kind`; the paragraph "Check without a request"); 8 (R-AZ-14, R-AZ-29).
- `docs/spec/00-glossary.md`: row "Acting-for binding" (`acting_for`).
- `docs/spec/09-agents.md`: 4.5 "Invocation token" (header, claims, server check), 4.5.1 "Fresh token for a waiting run" (step 1 checks without expiry), 4.7 "Adapter operations", 5.1 "State machine" (active states).
- `docs/spec/06-api.md`: 3 "Authentication per principal kind" (row Agent), 4 "Authorization in the API", 5 "Errors" (generated table, row `invocation_token_invalid`), 7 "Idempotency".
- `docs/spec/02-architecture.md`: 4.1 "Rules" (rule 4: source ports), 4.3 "Ports, events and sources per module" (rows `identity`, `runs`), 4.4 "Allowed dependencies", 4.5 "Cross-cutting ports" (`kernel/actor`, `kernel/authorize`), 5 "Layers within a module", 6 "Code layout" (list of product-name places; `test/integration`), 9.1 "Chain in the api node" (step 7), 9.2, 9.3 "Proof of the check", 11.3 "Permission cache".
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope" (rule 3: no tokens in payloads), 8.1 "Basic rule", 15.1 to 15.3.
- `docs/spec/08-identity.md`: 7.1 "Overview" (row Invocation token), 12 "API tokens for agents, apps and tools".
- `docs/spec/11-extensions.md`: 11.2 "Hand-written layer" (rows `invocation`, `runs`).
- `docs/spec/14-search.md`: 4.3 "Permission filter" point 5 (search in a run, built by M3-14).
- `docs/spec/19-security.md`: SEC-AI-3, SEC-LOG-2, SEC-AUD-5.
- `docs/api/openapi.yaml`: `components.securitySchemes` `bearerAuth` and `dpopAuth`; `components.parameters` (style of `IdempotencyKey`); `x-problem-codes` of `Problem` (`invocation_token_invalid`, `forbidden`, `not_found`); responses `Forbidden`, `NotFound`; operations `getMessage`, `createMessage`, `finishRun`, `awaitRunInput`, `startRun`, `resumeRun`, `renewInvocationToken`, `pollAgentEvents`, `getSnapshot`, `listEvents`, `listMyMentions`, `listMyReadStates`, `listSpaces`, `listSpaceChannels`, `listChannelThreads`, `listThreadMessages`, `listThreadRuns`, `listActiveRuns`, `explainChannelAccess`, `createGrant`, `revokeGrant`, `setChannelOperator`; and, if enabled on the base branch, `patchRun`, `reportAgentSession`, `respondToInteraction`, `getRunContext`.
- ADRs 0008, 0013, 0017, 0018, 0021, 0022, 0023, 0053, 0067.
- `docs/DECISIONS.md`: D-RE-03, D-W-15, D-W-19, D-W-100; section 10 for the format of new rows (`D-M2-14-<n>`).
- `docs/security/threat-model.md`: TB4, T-AI-01, T-AI-02, T-AI-04, T-TK-05. `docs/RISKS.md`: RK-01. No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- `docs/observability/signals.md`: `bothy.authz.decisions`, `bothy.authz.errors` (no new metric here).
- `docs/operations/configuration.md`: `agent.invocation_token_ttl`, `auth.clock_leeway`, `authz.snapshot_cache_max_entries`, `audit.deny_aggregate_window` (no new key).
- Briefs: M1-18 (`StageAuthenticate`, `actor.Credentials`, check log `RecordCheck` with `CheckScope`, `httpservertest`), M1-24 (`auditing.Recorder`, `audit_details.v1`), M1-28 (`actor.ActingFor{UserID, RunID, ThreadID ids.ID; JTI string}`, the `ActingFor` intersection in `effective` and `Scope`, reason `acting_for_insufficient` for visibility, AZ-P8 reference, matrix world fakes in `internal/authz/adapters/fake`), M1-34 (`internal/identity/app/authenticator.go`, the authenticator never rejects on flags), M1-44 (API token path, no cache of token state), M1-49 (`via_agent`, `explainChannelAccess`), M1-63 (`Scope` in snapshot and catch-up, `unread_threads`), M1-65 (`authorize.Delivery(ctx)`), M2-06 (`app.InvocationTokens.Verify` with `VerifyInvocation{Audience, RunID, IgnoreExpiry}`; Design note 1: "M2-14 adds the port with its own DTOs"), M2-07 (REST poll in delivery mode), M2-08 (Design note 4: the re-check principal of the triggering person), M2-09 (`Decision.Detail`, the mapping of `invocation_token_invalid` with `reason` equal to `Detail`, the test world with a run of Alex).
- Code on the base branch: `internal/kernel/actor`, `internal/kernel/authorize`, `internal/kernel/auditing`, `internal/kernel/clock`, `internal/kernel/errs`, `internal/authz` (domain, app, fakes), `internal/identity` (port, `app/authenticator.go`), `internal/runs` (M2-05 to M2-09: port, `app/invocation.go`, repository), `internal/platform/httpserver` and `httpservertest`, `internal/platform/pg/pgtest`, `internal/identity/adapters/identitytest`, `internal/wire`, `architecture/modules.yaml`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. For every action of an agent in a run that reads content or writes into a channel: `eff(agent in run, target) = eff(agent, target) ∩ eff(triggering person, target)`, where `∩` is the lower level and the intersection of the capabilities (07 "SR9", 07 4.2 step 4, 05 2.3 point 6, 00 "Acting-for binding", SEC-AI-3, R-RE-06).
12. The agent presents the checked invocation token of the run in a request header whose name 06 fixes (07 "SR9"); the server checks it on every request (signature, `aud`, run active) and computes the person's rights at that moment, never from the token, so a revocation applies at once to the agent's tool calls (07 "SR9", 07 3 row `acting_for`, 07 8.2 point 4).
13. Only the agent's rights apply to replies in the run's thread, patches of the own run, `run.finish`, `interaction.respond` and `agent_session.report`, and only for the own thread and run (07 "SR9"; PLAN row M2-14).
14. `Scope` cuts its lists with those of the triggering person; every list, counter, the snapshot and the catch-up of an agent in a run contain only channels the person may read too (07 7.4 rule 1, D-W-19; PLAN row M2-14).
15. A target that is invisible only because of the intersection answers `404` exactly like an unknown ID, with deny reason `acting_for_insufficient` in the audit; a requirement that fails only because of the intersection answers `403 forbidden` with `reason: acting_for_insufficient` (07 4.1, 4.2 step 5, R-AZ-01, AZ-M44).
16. Without a token the agent acts with its own rights; the console shows hint H1 for the triggering people (07 6, 9.1 `via_agent`, AZ-M45).
17. Deliveries to an agent consumer are judged by the agent's own grants (07 8.1 row "Agent consumers").
18. Every denial of a request is audited (07 4.2, R-AZ-04); a token never appears in logs, traces, audit details, problem bodies or events (SEC-LOG-2, SEC-AUD-5, 04 2.2 rule 3).

## Scope

All paths under `services/server/` unless they start with `docs/`. Names of M2-05 to M2-09 below are the expected ones; where the merged code names them differently, follow the code (Boundaries).

Create:

- `internal/authz/domain/actingfor.go`: the closed mode table `ActingForModeOf(a authorize.Action) ActingForMode` with the modes `ActingForBound`, `ActingForOwnException`, `ActingForUnbound` (Design note 5), and the pure function `ActingForApplies(mode ActingForMode, af actor.ActingFor, path Path, r authorize.Resource, delivery bool) bool` (whether the intersection applies). `Path` stands for the resolved path type of M1-28's domain.
- `internal/authz/domain/testdata/actingfor_modes.golden`: the exception and unbound action ids, one per line.
- `internal/runs/domain/actingfor.go`: pure `ActingForVerdict(c InvocationClaims, run RunBinding, found bool, agentID ids.ID) (ActingForResult)` with `RunBinding{ID, AgentID, TriggeredBy, ThreadID ids.ID; State string}` and `ActingForResult{Status string; UserID, RunID, ThreadID ids.ID; JTI string}` (Design note 3). Domain imports only `kernel-core`, so the status is a runs-local string type with the three values of note 2.
- `internal/runs/app/actingfor.go`: use case `ResolveActingFor(ctx context.Context, agentID ids.ID, token string) (ActingForResult, error)` over `InvocationTokens.Verify` (M2-06) and the repository read of note 3c.
- `internal/runs/adapters/postgres/queries/run_binding.sql`: `GetRunBinding :one` selecting `id, agent_id, triggered_by, thread_id, state` of one run by primary key; regenerate sqlc.
- `internal/runs/adapters/actingfor/source.go`: type `Source` that implements `identity.ActingForSource` structurally (only `kernel` types in the signature) by calling the use case and mapping `ActingForResult` to `actor.ActingFor`.
- `test/integration/actingfor/` (component `testsupport`): the cross-module integration tests of this ticket and the golden file `testdata/sr9_scope_operations.golden`.

Change:

- `internal/kernel/actor`: `ActingFor` gains `Status ActingForStatus` with the constants `ActingForBound` (`"bound"`), `ActingForRunInactive` (`"run_inactive"`), `ActingForInvalid` (`"invalid"`); the zero value (empty `Status`) means "no header". Add `(ActingFor) Present() bool`. `Credentials` gains `InvocationTokens []string` (every value of the header). Doc comments name 07 "SR9" and Design note 2.
- `internal/platform/httpserver`: `StageAuthenticate` fills `Credentials.InvocationTokens` from `r.Header.Values("Invocation-Token")`; the header stays out of every log and span (the allowlist of SEC-LOG-2 must not gain it; add it to the denylist test of the access log if one exists).
- `internal/identity` port: declare `ActingForSource { ResolveActingFor(ctx context.Context, agentID ids.ID, token string) (actor.ActingFor, error) }` (02 4.1 rule 4, source port of a lower module). `internal/identity/app/authenticator.go`: the steps of Design note 2.
- `internal/authz`: `decide.go`, `effective.go`, `visibility.go`, `app/authorizer.go`, `app/scope.go`, `app/cache.go` for Design note 4; the AZ-P8 slow reference gains SR9 (Design note 4i); the matrix world fakes gain a helper that builds an agent principal with `ActingFor`.
- `internal/wire`: bind `runs/adapters/actingfor.Source` as the `identity.ActingForSource` of the authenticator, with the compile-time assertion `var _ identity.ActingForSource = (*actingfor.Source)(nil)` in `internal/wire` (Design note 3h).
- `docs/api/openapi.yaml`: `components.parameters.InvocationToken` (`name: Invocation-Token`, `in: header`, `schema: {type: string}`, description per Design note 1); one sentence in the descriptions of `bearerAuth` and `dpopAuth` naming the header and SR9; the `invocation_token_invalid` entry of `x-problem-codes` extended for the header with the reasons `invalid` and `run_inactive` and its `client_action` for both. Then `make api-lint`, `make api-checks`, `make api-tables` (regenerates the table of 06 5) and the server code generation.
- `docs/spec/06-api.md` (English): 3 row "Agent", column "Headers per request": "within a run additionally `Invocation-Token: <invocation_token>` (07 SR9)"; 4 a new point 8 "Acting-for binding" with the rules of Design notes 2, 4 and 5 in two to four sentences.
- `docs/spec/07-permissions.md` (English): 3 row `acting_for` names the header; 4.1 rows `invocation_token_invalid` (also every request with `Invocation-Token`, `reason` `invalid` or `run_inactive`) and `acting_for_insufficient` (both decisions as built); 4.2 step 1 the two status checks and step 4 the exceptions, the two unbound actions and the delivery mode; "SR9" one paragraph with the binding check of note 3, the exception rule of note 5, the triggering person's rights of note 4e and the residuals of note 9.
- `docs/spec/02-architecture.md` (English): 4.3 row `identity` "declares `ActingForSource` (invocation token of the request, SR9)", row `runs` column "Implements sources" `identity.ActingForSource`; 9.1 step 7 "and the invocation token from `Invocation-Token` through `identity.ActingForSource` (SR9)".
- `docs/spec/11-extensions.md` (English): 11.2 row `runs` "sets `Invocation-Token` on every request during a run (07 SR9); `403 invocation_token_invalid` with `reason: run_inactive` ends the work on the run".
- `docs/spec/09-agents.md` (English): 4.7 one sentence that every request of the adapter inside a run carries `Invocation-Token` (07 SR9).
- `docs/security/threat-model.md`: add this ticket's test names to the test column of T-AI-02 and T-AI-04.
- If `tools/lint/sql_visibility.yaml` (M1-70) is on the base branch: add `GetRunBinding` to its exception list with the reason of note 3f.
- READMEs of `authz`, `identity`, `runs` and `platform/httpserver`; package doc of `kernel/actor`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M2-14-<n>` for every Ruling below.

This ticket creates no migration, enables no operation in `api/operations.yaml`, adds no configuration key, no event, no job, no metric and no new top-level make target.

## Out of scope

- The SDKs setting the header on every request of a run (07 "SR9": "The SDKs ... automatically bind the token ..."): M2-29 (Python) and M2-30 (TypeScript); the reference adapters: M2-33 and M2-34. This ticket defines the header and its answers.
- Search of an agent in a run (14 4.3 point 5, D-W-19): M3-14; it relies on the `Scope` intersection made exact here.
- End-to-end tests of the exceptions on routes enabled by other tickets: `patchRun` and the gateway op `run.update` (M2-10), `reportAgentSession` (M2-12), `respondToInteraction` (M2-15), agent replies with `MessageCreate.run_id` (M2-19), and the bound read `getRunContext` (M2-13). This ticket decides them in `authz` (unit tests) and adds the end-to-end case only for those already enabled on the base branch (Tests).
- Attachments of an agent reply (`file.upload`, M3): see the spec gap in the report.
- The AZ-P5 property test and the AZ-P12 lint (M1-70); the conformance suite with the fake agent (M2-31).
- Token issuing and verification (M2-06), the delivery re-check principal (M2-08), the run lifecycle and the cancel on revocation (M2-09).
- "Who has access" itself (M1-49) and its console screens (M6).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-14-acting-for`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(authz): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for M2-05 to M2-09: their merged names win over the expected names here (for example the request principal type `actor.Actor` or `actor.Principal`, the verifier `InvocationTokens.Verify`, `Decision.Detail`); list each difference. If the merged M2-08 has no re-check principal for the triggering person (its Design note 4), build the triggering person's snapshot with the semantics of note 4e here and say so in the report.
- No real agent runtime and no model call: the adapter in the tests is plain Go HTTP code with an agent API token; no third-party service.
- Time only through `kernel/clock`; tests move a fake clock, they never sleep for a window.
- Invocation tokens and API tokens never appear in logs, audit details, problem bodies, event payloads or test failure messages (print a hash prefix of the `jti` if needed).

## Design notes

1. **Header** (Ruling for docs/DECISIONS.md; 07 "SR9" leaves the name to 06, which has none). Name `Invocation-Token`, value the compact JWS exactly as delivered in `EventEnvelope.invocation_token`. No product name (02 6 lists the only places for it, and M1-46 tests that list), no `X-` prefix, like `Idempotency-Key` and `Correlation-Id`. It is declared once as `components.parameters.InvocationToken` and named in the descriptions of `bearerAuth` and `dpopAuth`, not as a parameter of each operation: like `Authorization` it is an input of step 7 of 02 9.1 for every route, and the schema validation of step 13 ignores undeclared headers. The DPoP proof does not cover it; it needs no binding of its own because it never adds a right (note 3e).

2. **Authentication** (02 9.1 step 7, `internal/identity/app/authenticator.go`), after the existing paths of M1-34 and M1-44:
   a. No value of `Invocation-Token`: `ActingFor` stays zero.
   b. Values but no `Authorization` header: `401 unauthenticated` (a run binding needs an authenticated agent; this counts as failed credentials for `rl.public_auth` like every other `401` of this step, 02 9.1).
   c. More than one value, an empty value, or a principal whose kind is not `agent` (people with a session, personal tokens, apps): `ActingFor{Status: invalid}` without calling the source; the token's `aud` can only name an agent (05 5).
   d. Exactly one value and an agent: `ActingForSource.ResolveActingFor(ctx, agentID, token)`; its result goes into the actor. An error of the source (database) is `503 unavailable`, never an actor without the binding (07 1 point 5, fail closed).
   e. The authenticator never rejects because of the status; `authorize` decides and audits (note 4), as M1-34 does for session and device flags. No cache of the result: a revocation and the end of a run must count on the next request in every process (M1-44, AZ-P9).

3. **Resolution in `runs`** (`app/actingfor.go`, pure rules in `domain/actingfor.go`):
   a. `InvocationTokens.Verify(ctx, token, VerifyInvocation{Audience: agentID, IgnoreExpiry: true})` (M2-06 note 5: `alg`, `typ`, `kid` of purpose `invocation`, signature, `iss`, `aud`, `iat`). Any error gives `invalid`.
   b. Claims without `run_id` (`trigger_kind = autocomplete`, and `command.invoked` when the owner of the control command had no active run in the thread; 05 5, R-AZ-25) give `invalid`. Ruling for docs/DECISIONS.md: SR9 binds a run, and "run active" cannot hold without one.
   c. `GetRunBinding(run_id)`: no row, `agent_id` not the agent, `triggered_by` not `sub`, or `thread_id` not the claim `thread_id` give `invalid`.
   d. State `queued`, `running` or `waiting_input` (the active states, 09 5.1, R-DM-14) gives `bound`; `completed`, `failed`, `cancelled` give `run_inactive`. Both return `UserID = runs.triggered_by`, `RunID`, `ThreadID = runs.thread_id` (unchanged by a thread move) and `JTI`; `invalid` returns zero IDs.
   e. Ruling for docs/DECISIONS.md: expiry and the currency of the `jti` are not checked. 07 "SR9" names signature, `aud` and an active run; a run outlives `agent.invocation_token_ttl`; every token of a run names the same person (`runs.triggered_by`); and the token never adds a right (the intersection only narrows, the exceptions only keep the agent's own rights), so a leaked or older token gives its holder nothing the agent token alone would not give.
   f. `GetRunBinding` reads `runs` without the visible-channel predicate: it loads one run by primary key for authentication, before any `authorize`, and nothing of it leaves the server (07 7.4 rule 1 exception "resolution for `authorize`"). Name it with this reason in the `runs` README and, if present, in `tools/lint/sql_visibility.yaml`.
   g. The read uses the pool, no unit of work and no lock; the request's own unit of work starts later.
   h. Ruling for docs/DECISIONS.md: `identity` declares the source port (the authenticator owns the actor, 02 4.3), `runs` implements it in `adapters/actingfor` with only `kernel` types in the signature, and `internal/wire` holds the compile-time assertion; no new module edge in 02 4.4 or `modules.yaml` (M2-06 note 1: the consumer outside `runs` gets a port with its own DTOs).

4. **Decision in `authz`** (07 4.2; pure in `domain`, facts in `app`):
   a. Liveness and pending checks of step 1 stay first.
   b. Status `invalid`: every action, and `Scope`, ends with `forbidden`, reason `invocation_token_invalid`, `Detail` `invalid`, before the path is resolved, so an unknown and an invisible ID answer alike.
   c. Status `run_inactive`: every action except the unbound ones of note 5 ends the same way with `Detail` `run_inactive`; `Scope` returns that error. Ruling for docs/DECISIONS.md: a failed binding check is a failed token check (07 "SR9": "The token is checked on every request"), answered with the existing problem code and a new `reason`, so the adapter learns that the run ended. This includes the exceptions: after its run ended, `finishRun` with the header answers `403` with `run_inactive` instead of M2-09's idempotent answer; without the header M2-09's answers stay.
   d. Status `bound`: resolve the path, then `ActingForApplies` decides whether the intersection of M1-28 applies: not in delivery mode (note 6), not for unbound actions, not for an own exception (note 5); otherwise yes.
   e. The triggering person's rights (core contract: 05 section 5 "Check without a request" with 05 2.3 points 1 and 6, R-AZ-29; the marker extension and the cache key are a ruling for docs/DECISIONS.md): computed like the re-check principal of M2-08, a principal of kind `user` with the person's ID and no session, device, token, `view_as` or `acting_for`; liveness from the principal status only (SR7: `disabled`, `deleted` and `pending` people have no rights, so every bound action ends `acting_for_insufficient`); owner status from the current owner list without a session. Reason: the run exists because `agent.invoke` accepted the person with their rights, and without owner status every run of an owner (AZ-M19, every `personal` and `host` agent) could not even read its own context. The snapshot cache key keeps this variant apart from every token principal of the same person (M2-08 note 4). Extend the marker of M2-08 so it is allowed for snapshots of an `ActingFor` person as well as for the actions it already admits (R-AZ-29).
   f. Reasons: a target that is visible with the agent's own rights but not with the intersection gives `not_found` with `acting_for_insufficient` (M1-28 builds this); a requirement that holds with the agent's own rights but fails with the intersection gives `forbidden` with `acting_for_insufficient` instead of the atom's reason (07 4.1: "`not_found` or `forbidden`"). If the agent alone already fails, the agent's own reason stays.
   g. Mapping: `Decision.Err` of M2-09 maps `forbidden` with `invocation_token_invalid` to the problem code `invocation_token_invalid` with `reason` equal to `Detail`; `forbidden` with `acting_for_insufficient` maps to `403 forbidden` with `reason: acting_for_insufficient` (M1-28 row "every other reason"); `not_found` never carries a reason.
   h. `Scope` (07 7.4 rule 1): with status `bound` and outside delivery mode, `ChannelIDs` and `SpaceIDs` are the intersection of the agent's and the person's lists, `FullyVisibleSpaceIDs` the spaces fully visible to both, `InstanceWide` true only if both are; ascending order; the check log records `(a, allow)` as before.
   i. AZ-P8: the slow reference of M1-28 gains SR9 written independently from this note (mode table, exceptions, statuses, delivery mode, the person's rights of 4e), and the random worlds gain agents with `ActingFor` principals.

5. **Mode table** (Ruling for docs/DECISIONS.md; 07 "SR9" names the exceptions partly in prose and limits SR9 to actions that read content or write into a channel, without a closed list):
    - `ActingForOwnException`: `message.create` ("replies in the thread of the run"), `run.update` ("patches to its own run"), `run.finish` (also `awaitRunInput`), `interaction.respond`, `agent_session.report`. "Own" means: the resolved thread of the resource equals `ActingFor.ThreadID`, and for a resource of kind `run` also `r.ID` equals `ActingFor.RunID` (so `run.finish` on an earlier run of the same thread is bound). Then only the agent's rights apply; otherwise the action is bound.
    - `ActingForUnbound`: `run.start` (also `resumeRun`) and `run.token.renew`. They neither read content nor write into a channel; their own token in the body is checked by 09 4.5 and 4.5.1, and renewal step 4 repeats `agent.invoke` for the person (M2-09). With a header they answer exactly as without one, also with status `run_inactive` (so 09 "409 `already_started`" and D-W-15 stay true); status `invalid` still denies (note 4b).
    - `ActingForBound`: every other catalogue action. A new action is bound until its ticket classifies it (fail closed). Actions without a path in the channel tree (`agent.heartbeat`, `agent.events.consume`, `agent.triggerers.read`, `self.read`, `instance.read`) are bound too; the intersection changes nothing for them because their requirement needs no level on a path, and a test pins that.
   The golden file holds the two non-default sets; `TestActingForModes_Golden` compares, and `TestActingForExceptionsNamedInChapter07` reads the "SR9" section of `docs/spec/07-permissions.md` and asserts that every backticked action id of its exception sentence is in `ActingForOwnException`.

6. **Delivery mode** (Ruling for docs/DECISIONS.md, 07 8.1 row "Agent consumers": "like people, according to their own grants"): in `authorize.Delivery(ctx)` decisions and `Scope` ignore `ActingFor` completely. The REST poll of M2-07 admits events in delivery mode with the request principal, so a header on `pollAgentEvents` must not narrow the consumer. `Filter(ctx)` (per-element checks of lists, `listEvents`) stays bound.

7. **Audit** (07 4.2, R-AZ-04, SEC-AUD-5). Denials are recorded by the authz layer as for every request, aggregated per `audit.deny_aggregate_window`. Ruling for docs/DECISIONS.md: a denial whose principal carries `ActingFor` has the details `acting_for_user_id` and `run_id` (UUIDs; absent for `invalid`) and, for `invocation_token_invalid`, `token_verdict` with the `Detail` code (the key M2-09 uses). Allows write nothing new. Filters write nothing (AZ-P13).

8. **Gateway** (Ruling for docs/DECISIONS.md). SR9 binds REST requests only. The gateway has no request header, and an agent's gateway ops are patches of the own run (`run.update`, an exception), acknowledgements (`ack`) and autocomplete answers, none of which reads content in a run's name. Gateway connections therefore carry no `ActingFor`.

9. **What SR9 does not cover** (write it into the `authz` README and the SR9 paragraph of 07): the server cannot tell a request inside a run that omits the header from one outside a run (AZ-M45); the SDKs and reference adapters close this by always sending it (M2-29, M2-30, M2-33, M2-34). Knowledge a runtime keeps across runs is outside the API (H1, T-AI-01). An owner who triggered with a personal API token (no owner rights at acceptance) is intersected as owner (the residual named in R-AZ-29 and M2-08 note 4).

10. **Idempotency and concurrency.** Authentication reads the run before the idempotency stage (02 9.1 steps 7 and 10); a replay returns the stored answer of the same agent as M1-20 defines. A run that ends between authentication and the handler is decided with the status read at authentication; the next request sees the new state. No lock is taken.

11. **Performance.** Each request with the header costs one Ed25519 verification and one primary-key read; the intersection costs one more snapshot lookup in the cache. Add `BenchmarkAuthorizeCachedActingFor` next to `BenchmarkAuthorizeCached` (reports `p99-ns`, asserts nothing).

12. Rulings for docs/DECISIONS.md, one row each with reason: notes 1, 3b, 3e, 3h, 4c (with the new `reason` `run_inactive`), 4e (only the marker extension and the cache key; the rule itself is R-AZ-29), 5, 6, 7, 8. Facts already in the spec (the intersection, the exception list, `acting_for_insufficient` for both decisions, `Scope` with `acting_for`) are not repeated there.

## Tests (write them first)

Unit (`internal/authz`, fake sources and the matrix world of 07 10 and 11.1 from M1-28; Lotse additionally has `read` on space Engineering where a case says so):

- `TestActingForModes_Golden` and `TestActingForExceptionsNamedInChapter07` (note 5).
- `TestAZM44_ActingForInsufficient`: Lotse with `ActingFor` Alex (status `bound`, run in Support/Allgemein), Lotse has `read` on Engineering: `message.read` on a message in Engineering/Projekt-A gives `not_found` with `acting_for_insufficient`; `Scope(message.read)` contains no channel of Engineering.
- `TestAZM45_WithoutRun`: the same without `ActingFor` gives `allow`.
- `TestActingFor_ForbiddenReason`: Alex `read` and Lotse `write` on Engineering/Werkzeuge: `message.create` in a thread there gives `forbidden` `acting_for_insufficient`; Alex and Lotse both `read`: `forbidden` `level_insufficient`.
- `TestActingFor_Exceptions`: for each of `message.create`, `run.update`, `run.finish`, `interaction.respond`, `agent_session.report`: on the own thread (and own run) after Alex lost every grant, `allow` with Lotse's rights; on a thread in a channel Alex cannot read, `not_found` `acting_for_insufficient`; `run.finish` on an earlier run of the same thread is bound.
- `TestActingFor_Unbound`: `run.start` and `run.token.renew` give the same decision and reason with and without `ActingFor`, for statuses `bound` and `run_inactive`.
- `TestActingFor_StatusInvalid`: a bound, an exception, an unbound action and `Scope` give `forbidden` `invocation_token_invalid` with `Detail` `invalid`; an unknown resource ID gives the same decision as a known one.
- `TestActingFor_StatusRunInactive`: bound and exception actions and `Scope` give `invocation_token_invalid` with `Detail` `run_inactive`; unbound actions as without.
- `TestActingFor_DeliveryModeIgnores`: in `Delivery(ctx)` decisions and `Scope` equal those without `ActingFor`; in `Filter(ctx)` they are intersected.
- `TestActingFor_OwnerTriggerer`: the Owner triggers; the intersection equals Lotse's own rights; after `owner.remove` the person's grants alone count; the cache entry of this snapshot differs from the Owner's personal-token principal.
- `TestActingFor_TriggererInactive`: Alex `disabled` or `pending`: every bound action `not_found` `acting_for_insufficient`.
- `TestActingFor_FreshRights`: a revocation of G-1 (new rights version) makes the next decision deny without any change of `ActingFor`.
- `TestActingFor_PathlessActionsUnchanged`: for `agent.heartbeat`, `agent.events.consume`, `agent.triggerers.read`, `self.read`, `instance.read` the decision with and without `ActingFor` is equal.
- `TestScope_ActingForIntersection`: each of `ChannelIDs`, `SpaceIDs`, `FullyVisibleSpaceIDs`, `InstanceWide` intersected, ascending; a channel created in Engineering without a new rights version stays out.
- `TestDecisionErr_ActingFor`: problem code and `reason` for both `Detail` values; `acting_for_insufficient` as `403 forbidden` with `reason`; `not_found` without reason.
- `TestAZP11_ActingForProperty` (AZ-P11): random worlds from a seeded `math/rand/v2` source (seed printed on failure, overridable with `BOTHY_TEST_SEED`) with people, agents, groups, grants with expiries, operators, runs and statuses; for every content read (`message.read`, `thread.read`, `channel.read`, `run.read`, `message.revisions.read`) an `allow` for an agent with a `bound` `ActingFor` implies that the person may read the same path, and for every channel write (`message.create` outside the own thread, `message.update`, `message.delete`, `reaction.add`, `thread.create`, `thread.close`) that the person has the required level and capabilities there; `Scope` of the agent is a subset of the person's; decisions equal the extended AZ-P8 reference (note 4i).
- `TestAZP8_CacheMatchesReference` stays green with the SR9 dimension.

Unit (`internal/identity`, fake source):

- `TestAuthenticator_InvocationHeader`: no header gives zero `ActingFor`; an agent with one value calls the source with its ID and the token and copies the result; a person with a session, a personal token and an app token with the header give `invalid` without a source call; two values and an empty value give `invalid` without a call; the header without `Authorization` gives `401 unauthenticated`; a source error gives `503 unavailable`.

Unit (`internal/runs`):

- `TestActingForVerdict`: no run, other agent, `sub` not `triggered_by`, other `thread_id`, claims without `run_id`: `invalid`; `queued`, `running`, `waiting_input`: `bound` with `UserID`, `RunID`, `ThreadID`, `JTI`; `completed`, `failed`, `cancelled`: `run_inactive`.
- `TestResolveActingFor`: `Verify` is called with the agent as audience and `IgnoreExpiry`; a bad signature, the token of another agent, an access token (`at+jwt`) and a terminal-type JWS give `invalid`; an expired token of an active run and an older (renewed) token of the same run give `bound`.

Integration (`test/integration/actingfor`; PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain with `httpservertest` of M1-18, people and agent tokens from the identity test helpers, fake clock; world as M2-09's: Robin is the Owner, space Support with channels Allgemein and Eskalation operated by Lotse (`organization`, `chat`) with G-5 (`write`, `upload` on Support), Alex with G-1 (`trigger` on Support), spaces Engineering (forum Projekt-A with a post of Robin, text channel Werkzeuge) and Marketing (Texter); Lotse gets `read` on space Engineering through `createGrant` by Robin; Alex posts in Support/Allgemein, the run is delivered through `pollAgentEvents` and started with `startRun`, so Lotse holds Alex's token T):

- `TestAZM44_EndToEnd`: `getMessage` of the Projekt-A post with `Invocation-Token: T` gives `404` with the same status, body and headers as a random UUIDv7; one aggregated deny entry `message.read` with `acting_for_insufficient`, `acting_for_user_id` Alex and `run_id`.
- `TestAZM45_EndToEnd`: the same request without the header gives `200`; `explainChannelAccess` of Engineering/Projekt-A by Robin lists Alex with level `none` and a `via_agent` source naming Lotse (hint H1).
- `TestAZP11_WritesEndToEnd`: Robin gives Lotse `write` on Engineering/Werkzeuge; `createMessage` there with T gives `404` while Alex has no grant and `403 forbidden` with `reason: acting_for_insufficient` after Robin gives Alex `read` there; without the header `201`. Robin lowers G-1 to `read` (revoke and new grant): Lotse's reply in the run's own thread with T gives `201` (exception), a message into another thread of Support/Allgemein with T gives `403 forbidden` with `reason: acting_for_insufficient`.
- `TestSR9_EveryScopedList`: a test-only decorator around the wired `kernel/authorize.Authorizer` records which operation called `Scope`. Every enabled GET operation of `api/operations.yaml` that an agent may call is requested as Lotse without and with T (path parameters from a fixture map: Engineering space, Projekt-A channel and thread, the post, Lotse's own IDs, and Robin's run in Engineering/Werkzeuge named at the end of this item). For every operation that called `Scope`: with T no response body (all pages) contains an ID of Engineering (space, channels, threads, messages, runs); without the header at least the operations `listSpaces`, `getSnapshot`, `listEvents`, `listMyMentions` and `listActiveRuns` do contain one (the test is not vacuous); path-addressed requests on Engineering answer with T like an unknown ID. The set of operations that called `Scope` equals `testdata/sr9_scope_operations.golden`; a new list operation fails the test until it is added. Robin triggers Runner in Engineering/Werkzeuge first, so an active run exists there, and mentions Lotse in Projekt-A, so a mention exists.
- `TestSR9_UnreadCounters`: `getSnapshot` of Lotse without the header lists threads of Engineering in `unread_threads`; with T it lists none of them, and `spaces`, `channels` and `active_runs` contain nothing of Engineering. If the merged snapshot computes no unread threads for agents (no read states of their own), assert that and the other members, and say so in the report; the counters of people are covered by M1-63.
- `TestSR9_FinishOwnRunAfterRevocation`: Robin revokes G-1 (the run gets `run.cancel_requested` with `access_revoked`, M2-09); `getMessage` of a Support message outside the run's thread with T gives `404`; `finishRun(cancelled, access_revoked)` with T gives `200` (exception).
- `TestSR9_ForeignRunIsBound`: Robin sets Lotse as operator of Engineering/Werkzeuge (`setChannelOperator`) and triggers it there (run R2, started by Lotse with its own token of R2); `finishRun(completed)` of R2 with Alex's T in the header gives `404` like an unknown run; without the header `200`.
- `TestSR9_RunInactive`: after `finishRun(completed)` of Alex's run, `getMessage` of a Support message with T gives `403 invocation_token_invalid` with `reason: run_inactive`; `startRun` of that run with T in the header and the body gives `409 already_started` as without the header.
- `TestSR9_InvalidHeader`: a token with a changed signature byte, Texter's token, an access token of Alex, a token with `trigger_kind = autocomplete` issued through `InvocationTokens.Issue`, two header values and an empty value: each `403 invocation_token_invalid` with `reason: invalid` on `getMessage`; Alex's own session with T gives the same; T without `Authorization` gives `401 unauthenticated`; the deny entries carry `token_verdict` `invalid` and no token.
- `TestSR9_ExpiredTokenStillBinds`: after the clock passes `exp` of T plus `auth.clock_leeway`, T still binds (Projekt-A `404`, own thread `200`).
- `TestSR9_RevocationImmediate`: two `Authorizer` instances with warm caches on the same database (REST mode); after `revokeGrant` of G-1 both deny Lotse's request with T at once.
- `TestPollAgentEvents_HeaderDoesNotNarrowConsumer`: a `message.created` in Projekt-A reaches Lotse through `pollAgentEvents` with T in the header exactly as without it (07 8.1).
- `TestStartRun_HeaderUnaffected`: `startRun`, `resumeRun` and `renewInvocationToken` answer with T in the header exactly as M2-09's tests expect without it.
- `TestSR9_ExceptionRoutesEnabled`: for each of `patchRun`, `reportAgentSession`, `respondToInteraction` that is enabled on the base branch, the own-run case with T after Alex lost every grant succeeds and the foreign-run case gives `404`; for `getRunContext`, if enabled, T after Alex lost every grant gives `404`. List the routes not yet enabled in the report.
- `TestSR9_TokenNeverLogged`: with the log capture of `httpservertest`, every request above: the token, its signature segment and its payload segment appear in no log record, span attribute, problem body or `audit_events` row.
- `TestWire_ActingForSource`: the wired authenticator uses the `runs` source; the server starts.

Contract: problems of the integration tests validate against `Problem` and carry the `code` and `reason` named above; no JSON `null`.

Benchmark: `BenchmarkAuthorizeCachedActingFor` reports `p99-ns` (note 11).

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/authz/... ./internal/identity/... ./internal/runs/... ./internal/kernel/... ./internal/platform/httpserver/... ./internal/wire/... ./test/integration/actingfor/...`
- `cd services/server && go test -race -run '^TestAZ' -v ./internal/authz/... ./test/integration/actingfor/...`
- `cd services/server && go test -run '^$' -bench 'BenchmarkAuthorizeCached' -benchtime 20000x ./internal/authz/app`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make api-tables`, `make register-check`, `make semgrep`
- `make docs`
- `make check`

## Acceptance criteria

- [ ] AZ-M44: Lotse in Alex's run reading Engineering/Projekt-A gets `404` identical to an unknown ID, audited with `acting_for_insufficient`; green at decision level and end to end.
- [ ] AZ-M45: without the header Lotse reads it (`200`), and "Who has access" shows Alex with `via_agent` (hint H1).
- [ ] AZ-P11: no agent in a run reads or writes content the triggering person may not read or write (property test with the SR9 reference, write and read cases end to end).
- [ ] The exceptions of 07 "SR9" apply only on the own thread and run; `run.start` and `run.token.renew` answer as without the header; a failed requirement caused only by the intersection gives `403` with `reason: acting_for_insufficient`.
- [ ] `Scope` gives an agent in a run only channels the person may read too, for every list operation that calls `Scope` (golden list), including the unread counters of `getSnapshot`.
- [ ] The token is checked on every request: an invalid token gives `403 invocation_token_invalid` `invalid`, a token of an ended run `run_inactive`; an expired token of an active run still binds; a revocation counts on the next request in every process.
- [ ] Consumer deliveries ignore the header (07 8.1).
- [ ] `Invocation-Token` is in openapi.yaml, 06, 07, 02, 09 and 11; the problem table is regenerated; no token in logs, traces, audit or problems.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `authz`, `identity`, `runs` and `platform/httpserver` updated (the header, the authentication steps, the source port, the mode table, the delivery-mode rule, the query exception of note 3f, the residuals of note 9); package doc of `kernel/actor`.
- Rulings of note 12 recorded in `docs/DECISIONS.md` section 10 (one row each with reason); the changes to 02, 06, 07, 09, 11, `openapi.yaml` and the threat model of the Scope made in the same change.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`) with attention to the M2 red team item "confused deputy through the server API": tokens of other agents, ended runs, autocomplete tokens, owner triggers, the exceptions on foreign threads, lists and counters; on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`, the `p99-ns` of both benchmarks with the machine description.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example the exception routes not yet enabled on the base branch, and the SDK side of the header).
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found; at least: `file.upload` for attachments of a reply in the own thread, and the binding of autocomplete answers without a run (note 3b), each with a concrete proposal for the owner.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
