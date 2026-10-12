# M2-13: Run context with origin and trust

Status: ready | Depends on: M2-11, M2-12 | Milestone: M2
## Goal

After this ticket an agent adapter loads the context of its own active run with `run.context.read` (`getRunContext`, `GET /api/v1/runs/{run_id}/context`) and receives `run_context.v1`: the triggering message complete and never cut, the earlier messages of the thread above the floor of its session, on the first run of a session the main-thread messages before the thread's parent message, and for every element its author, `origin`, kind, channel, `role` and a trust marker `trust` (`trigger`, `authorized`, `self`, `unverified`, `system`) that the server computes at the time of the request from the current rights. Contents are bounded by the channel's `history_backfill_limit`, by `agent.context_message_max_bytes` per message and by `agent.context_max_bytes` in total, cut from old to new. The server builds this context because only `authz` knows which authors may instruct the agent (ADR 0054, R-AG-06, D-AG-05); every adapter then passes `unverified` content to its runtime only as marked data, so instructions hidden in messages of people without `trigger`, of other agents, apps or imports cannot pass as the request of the triggering person (SEC-AI-1, T-AI-01, T-AI-03, RK-01). This is the server half of AG-20 and AG-21; the SDK formatting helper (M2-29), the reference adapters (M2-33, M2-34) and the conformance suite (M2-31) build on it, and it replaces reading the whole event stream in adapters (RK-19).

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" (Goal; Scope; Out of scope: files and voice messages in agent calls are M3; Exit criteria 1 and 10; Test strategy: fake agent, FakeServer, no model costs; Red team first paragraph: T-AI-01 to T-AI-09 after OWASP, "confused deputy through the server API"; Risks: prompt injection), block "agents and extensions" rows M2-09 to M2-14, M2-29, M2-31, M2-33, M2-34; section 9 rows M3-07 and M3-09; section 7 rows M1-57, M1-62, M1-69.
- `docs/spec/09-agents.md`: 1 "Principles" (points 1 to 4); 3.2 "Origin (`messages.origin`)"; 3.3 "Channel switches" (`history_backfill_limit`); 3.4 "Decision per message" point 5 (auto thread); 3.6 "Guest agents" (no session); 3.7 "Auto-thread from a channel message" (M stays in the main thread, the run belongs to the new thread); 4.1 "Overview" (context by REST); 4.4 "Events to the adapter" (payloads carry no content); 4.6 "Run context" completely; 4.9 "Order in the adapter: commitment first, then runtime" (context read before `run.start`); 5.1 "State machine"; 7 "Follow-ups during a run"; 9.1 "Lifecycle" (`session.mode` `new`, `resume`, `bind`); 10.4 "Execution" (command runs have no triggering message); 16.1 "Test doubles"; 16.2 "Required scenarios" (AG-20, AG-21, AG-28); 17.6 "Flow of an invocation" (how an adapter formats the context); 19 (R-AG-04, R-AG-06); 21 "Configuration values of this chapter".
- `docs/spec/03-data-model.md` (core contract, read only): `runs` (`thread_id`, `agent_id`, `agent_session_id`, `trigger_kind`, `triggering_message_id`, `triggered_by`, `state`, `queued_at`, the partial index of active runs, R-DM-14); `run_followups` (`state`); `agent_sessions` (`is_current`, `started`); `threads` (`kind`, `parent_message_id` and the sentence about moves, `operator_agent_id`); `messages` (`thread_seq`, `author_principal_id`, `origin`, `kind`, `content_md`, `reply_to_message_id`, `deleted_at`, `system_code`; "Deletion empties `content_md`"); `channels` (`history_backfill_limit`, effective operator).
- `docs/spec/05-actions.md` (core contract, read only): 1 rules 1 and 2; 2.1 (`agent-self`); 2.3 points 1, 4, 5 and 6; 2.4 (column "Audit", denials are always audited); 2.5; 4.10 row `run.context.read`.
- `docs/spec/04-sync-protocol.md` (core contract, read only): 8.3 point 5 (a loss of `trigger` cancels the starter's runs); 15.3 "Invocation and run start" point 4 (the adapter reads the context before it starts); 15.4 "Follow-ups".
- `docs/spec/07-permissions.md`: 2.9 "Agents as principals"; 3 "Principal context"; 4.1 (reasons `no_grant`, `not_self`, `subject_kind_not_allowed`); 4.2 "Flow" (filter mode, audit of denials, "no permission logic outside of `authz`"); 4.3 "Visibility"; 4.5 "Archived spaces and channels" point 1; "SR9 Acting-for binding (confused deputy)"; 6 "Read horizon of agents"; 7.2 last paragraph ("How agents read threads is decided by milestone M2"); 7.4 "Query pattern" rules 1 and 3; 11.2 AZ-P1, AZ-P4, AZ-P12.
- `docs/spec/02-architecture.md`: 4.3 "Ports, events and sources per module" (rows `runs`, `messaging`, `chat`, `agents`, `authz`); 4.4 "Allowed dependencies" (row `runs`: `instance`, `chat`, `agents`, `messaging`, `billing`; no `authz`, no `identity`, no `files`); 4.5 "Cross-cutting ports" (`kernel/authorize`); 5 "Layers within a module"; 9.2 "404 instead of 403"; 9.3 "Proof of the check".
- `docs/spec/06-api.md`: 4 "Authorization in the API"; 5 "Errors" (`not_found`, `forbidden`, `internal_error`; point 7, the closed code list); 16 "Endpoints", table "Runs" row `getRunContext`.
- `docs/spec/19-security.md`: 9 "Logs without secrets and without content"; 11 "Controls for agents" (SEC-AI-1, SEC-AI-3); SEC-T10 in the test table.
- `docs/spec/20-privacy.md`: 6 "Agents, LLM providers and processors" (Data minimization at invocation: thread, `history_backfill_limit` earlier messages, authors as handle and kind, no e-mail addresses).
- `docs/api/openapi.yaml`: operation `getRunContext` (parameter `RunIdPath`, responses `Unauthorized`, `Forbidden`, `NotFound`, `TooManyRequests`, `Error`); schemas `RunContext`, `RunContextMessage`, `RunContextAttachment`, `PrincipalRef`, `MessageOrigin`, `MessageKind`, `Message`, `Attachment`, `File`, `AgentInvocationPayload` (`message_id` description), `AgentFollowupPayload` (`message_id` description), `AgentSession`, `Problem` with `x-problem-codes`.
- ADRs: 0054 (complete, including "Confirmation"), 0017 (one authorization function, 404 instead of 403), 0022 (agents outside the server), 0025 (run as an object), 0053 (no JSON `null`), 0013 (spec first), 0008 (module rules).
- `docs/DECISIONS.md`: D-AG-05, D-K-D6, D-W-100, section 10 "Rulings of the build milestones" (row format `D-M2-13-<n>`).
- `docs/security/threat-model.md`: T-AI-01, T-AI-02, T-AI-03 and the STRIDE row "Prompt injection through content". `docs/RISKS.md`: RK-01, RK-19. `docs/OPEN-QUESTIONS.md`: no item concerns this ticket.
- `docs/operations/configuration.md`: `agent.context_max_bytes`, `agent.context_message_max_bytes`, `channel.history_backfill_limit_default`, `channel.history_backfill_limit_max`, `audit.deny_aggregate_window`.
- `docs/observability/signals.md`: `bothy.authz.errors` with `bothy.reason = filter_mismatch`; `docs/observability/alerting.md`: `BothyAuthzError`. No metric is named for the run context; this ticket adds none.
- M1 briefs whose interfaces this ticket uses: `docs/briefs/M1-28-authz-core.md` (`Authorizer.Authorize`, `Resource` with `OwnerID`, atom `AgentSelf`, filter mode `authorize.Filter(ctx)`, `Decision.Err`, snapshots without credentials for `ViewAs` and `ActingFor`, visibility of kind `run`), `M1-18-http-chain.md` (check log: responses of status 400 or higher pass without a check), `M1-17-api-server-stubs.md` (`api/operations.yaml`, row `getRunContext` under `later`), `M1-24-audit-chain.md` (recorder), `M1-44-identity-api-tokens.md` (agent tokens), `M1-51-agents-records.md`, `M1-54-chat-operators.md` (effective operator), `M1-55-chat-threads.md` (`Threads.Info`, `ThreadInfo`), `M1-56-messaging-send.md` (`Messages` port, the single representation function and `MessageView`, origins), `M1-57-messaging-history.md` (window queries, tombstone representation, `TestQueriesCarryVisibleChannelPredicate`), `M1-62-chat-thread-move.md` (`moveThread`, `parent_message_id` keeps pointing into the source channel), `M1-69-canary-test.md` (`services/server/test/canary`, `services/server/test/harness`), `M1-07-config-schema-loader.md` and `M1-08-config-reference.md` (keys in `internal/wire/configkeys_<module>.go`, chapter constants in `configchapters.go`), `M1-12-postgres-migrations.md` (`pgtest`).
- M2 briefs this ticket builds on: `docs/briefs/M2-02-channel-behavior-config.md` (`history_backfill_limit`, chat port `AgentConfig`), `M2-04-trigger-decision.md`, `M2-05-runs-state-machine.md` (port `Runs`, run descriptor, states, `runstest` helpers), `M2-07-agent-consumers.md` (`pollAgentEvents`), `M2-08-invocation-delivery.md`, `M2-09-run-lifecycle.md` (ruling: `OwnerID` of a run is the principal the action reserves it to, `runs.agent_id` for agent-self actions; test world), `M2-11-run-followups.md` (`run_followups`), `M2-12-agent-sessions.md` (`agent_sessions`, port `agents.Sessions`), and the neighbour `M2-14` row (SR9, `acting_for`).
- Code: `services/server/internal/runs` (README, port, `app`, `domain`, `adapters`), `internal/messaging`, `internal/chat`, `internal/agents`, `internal/authz`, `internal/kernel/authorize`, `internal/kernel/actor`, `internal/kernel/errs` (`errs.New`, `errs.CodeNotFound`, `errs.CodeInternalError`), `internal/kernel/ids` (`ids.ID`), `internal/wire` (`configkeys_*.go`, `configchapters.go`), `api/operations.yaml`, `test/harness`, `test/canary`. Read the merged state with `git log` before you start.
- Implied dependencies: the header names M2-11 (`run_followups`) and M2-12 (`agent_sessions`). Their closure brings M2-10, M2-09, M2-08, M2-07, M2-06, M2-05, M2-04, M2-02, M1-65, M1-63, M1-61, M1-56, M1-55, M1-54, M1-49, M1-44, M1-28 and everything below them. M1-69 is required by the brief of M2-02 (so it, M1-71 and M1-62 are merged once M2-02 is); see Boundaries.

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

11. `run.context.read` is reached only through `getRunContext` (`GET /api/v1/runs/{run_id}/context`): right `agent-self`, only for own active runs (states `queued`, `running`, `waiting_input`); rate-limit class `rl.agent_runtime`; Audit `no`, Step-up `no`, Permissions `no`; denials are audited by `authz` like every denial of a request (05 4.10 and 2.4, R-AZ-04; 06 16 "Runs"; openapi `getRunContext`).
12. The server builds `run_context.v1`; the adapter never derives trust itself and no rights logic exists outside `authz` (09 4.6, ADR 0054 decision drivers, R-AG-06, 05 rule 1.1, ADR 0017).
13. Parts: `trigger_message` is the triggering message, complete, with its attachments and never cut; `thread_context` holds messages of the thread T before M with `thread_seq` above `context_floor_seq`, newest kept, at most `history_backfill_limit`; `channel_context` exists only on the first run of a session in a thread with `parent_message_id` and holds main-thread messages before the parent message, at most `history_backfill_limit`; `truncated_before_seq` is set when older messages are missing; `context_floor_seq` is the `thread_seq` of the triggering message of the previous run of the same session, otherwise zero (09 4.6, ADR 0054 point 5).
14. In a session with `session.mode = resume` the agent's own messages are left out; for `new`, `bind` and guest runs they are included. Messages already delivered to this session as follow-ups (`run_followups` in `delivered` or `absorbed`) carry `already_delivered: true` (09 4.6, 09 9.1, 09 3.6).
15. Contents above `agent.context_message_max_bytes` are cut with `truncated: true`; the sum stays within `agent.context_max_bytes`, without `trigger_message`; cutting goes from old to new (09 4.6, 09 21, AG-21).
16. Every element carries `message_id`, `thread_seq`, `created_at`, `author` (`id`, `kind`, `handle`, `display_name`), `role` (`user` for people, `assistant` for this agent, `other` for other agents, apps, imports and system), `origin`, `kind`, `content_md`, `attachments` (`file_id`, name, MIME type, size, `scan_status`, transcript when present), `reply_to_message_id` and `trust`; plus its channel (Design note 14, ruling 10) (09 4.6, 19 SEC-AI-1, 20 section 6).
17. `trust` is set at the time of the request: `trigger` for the triggering person, `authorized` for another active person with effective `trigger` on the channel, `self` for this agent, `unverified` for people without `trigger`, other agents, apps and imports, `system` for `origin = system`. A person who lost `trigger` between two runs is `unverified` in the second context (09 4.6, ADR 0054 point 2 and "Confirmation").
18. File contents are always data and are fetched by the adapter with `file.read`; a transcript of a voice message carries the trust of its message (09 4.6, ADR 0054 point 4). This ticket lists attachments as the representation of messaging returns them; real attachments and transcripts arrive with M3.
19. No read right gives `404`, and an unknown run, a foreign invisible run and an own run that is no longer active answer exactly the same `404` (05 rules 1 and 2, R-AZ-01, 02 9.2, AZ-P4).
20. No message content, handle or display name appears in logs, traces, metrics, audit details or error responses (19 section 9, SEC-T10); no JSON `null` in the response (ADR 0053).

## Scope

All paths under `services/server/` unless noted. Names in code blocks are proposals; follow the style of the merged module where it differs and list the differences in the report.

Create:

- `internal/runs/domain/runcontext.go`: the pure rules of Design notes 4 to 8: types `Trust` (`trigger`, `authorized`, `self`, `unverified`, `system`) and `Role` (`user`, `assistant`, `other`); `Classify(m AuthorFacts, p RunParties, holders map[ids.ID]bool) (Role, Trust)`; `SessionMode` (`new`, `resume`, `bind`, `guest`) and `PlanContext(f PlanFacts) ContextPlan` (floor, own messages excluded, channel context wanted); `CutContent(s string, maxBytes int) (string, bool)`; `FitBudget(channel, thread []Item, maxTotal int) (keptChannel, keptThread []Item)`; `TruncatedBeforeSeq(...)`. Imports only `kernel-core` packages (02 section 5).
- `internal/runs/app/runcontext.go`: the use case `ReadRunContext` (Design note 1).
- `internal/runs/adapters/postgres/queries/runcontext.sql` (sqlc; regenerate): `GetRunForContext`, `PreviousSessionTrigger`, `DeliveredFollowupsOfSession` (Design notes 5, 9 and 11).
- `internal/runs/adapters/httpapi`: the strict-server method `GetRunContext` in the module's handler file.
- `internal/messaging/app/context.go` and `internal/messaging/adapters/postgres/queries/context.sql`: `ListContextWindow`, `GetContextMessage` (Design note 3).
- `internal/kernel/authorize/triggerholders.go`: the interface `TriggerHolders` (Design note 7).
- `internal/authz/app/triggerholders.go` implementing it, with its fake in `internal/authz/adapters/fake`.
- `internal/wire/configkeys_runs.go` (or the existing file that already holds keys of chapter 09): `agent.context_max_bytes` and `agent.context_message_max_bytes` (Design note 12).
- `test/integration/runcontext/` (component `testsupport`, harness `test/harness`): the integration tests below.

Change:

- `internal/runs` port: `Runs.Context(ctx context.Context, runID ids.ID) (RunContext, error)` with the DTOs `RunContext`, `ContextMessage`, `ContextAttachment` (Design note 10). The acting principal comes from `kernel/actor` in the context, never from a parameter.
- `internal/messaging` port: `Messages.ContextWindow(ctx, ContextQuery) (ContextPage, error)` and `Messages.ContextMessage(ctx, messageID ids.ID) (MessageView, bool, error)` (Design note 3).
- `internal/chat` port, only where missing: `ThreadInfo.ParentMessageID` (zero when absent); the internal read of the channel switches including `HistoryBackfillLimit` (Design note 2).
- `internal/agents` port, only where M2-12 did not export it: the read of the current session of a thread (`id`, `agent_id`, `started`) (Design note 5).
- `internal/wire`: hand the `authz` implementation of `authorize.TriggerHolders` to `runs`; construct `ReadRunContext` with the two configuration keys; add the chapter constant for `09-agents.md` to `configchapters.go` if missing.
- `api/operations.yaml`: move `getRunContext` from `later` to the enabled list; rerun the route generator.
- `docs/api/openapi.yaml`: `RunContextMessage.channel_id` (required); descriptions of `getRunContext`, `RunContext.channel_context`, `RunContext.truncated_before_seq`, `RunContextMessage.truncated` and `already_delivered` after Design note 10; the example of `getRunContext` with `channel_id`.
- `docs/spec/09-agents.md` 4.6 (English, like the chapter): the element field `channel_id` and the rules of rulings 3 to 9 and 12; `docs/spec/02-architecture.md` 4.5, row `kernel/authorize`: the port `TriggerHolders` (ruling 1).
- `test/canary/scenario.go` and `test/canary/README.md`: the context read in the scenario (a newly enabled operation that is missing there fails the canary test, M1-69).
- `TestQueriesCarryVisibleChannelPredicate` of messaging (M1-57) and, if present, the repository-wide list of `tools/lint/sql_visibility.py` (M1-70): the new queries with their reason (Design note 11).
- `docs/operations/configuration.md`: regenerated with `make config-reference`.
- READMEs of `runs`, `messaging`, `chat`, `authz` (and `agents` if its port changed); package doc of `kernel/authorize`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10.

This ticket creates no migration and no new top-level Makefile target.

## Out of scope

- SR9: `acting_for` from the invocation token in `Authorize` and `Scope` and its per-request intersection with the rights of the triggering person (M2-14). This ticket keeps every element inside the thread's current channel (Design note 6), the channel on which the triggering person needed `trigger` for the run; a loss of that right cancels the run (R-SY-11, M2-09).
- Formatting the context with trust markers for a runtime: the SDK helper (M2-29), the reference adapters for Hermes Agent and the Claude Code runner and their AG-20 adapter tests (M2-33, M2-34).
- The data-driven scenarios AG-20 and AG-21 with the fake agent in the conformance suite (M2-31).
- Real attachments, file contents, `file.read` and transcripts of voice messages (M3-05, M3-07, M3-09; the plan names no ticket that fills them into the run context, reported as a plan gap).
- Follow-ups and their states (M2-11), agent sessions and their API (M2-12), run start, renewal and end (M2-09), delivery of `agent.invocation` (M2-08).
- The context of runs with `trigger_kind` `command` or `interaction` (M2-16, M2-15; reported as a spec gap) and the content of follow-ups of the current run (M2-11; reported as a spec gap).
- Channel prompt and skills (`agent.config.read`, M2-02).
- The timing part of AZ-P4 across all routes, the repository-wide SQL lint (M1-70) and the Schemathesis contract tests (M1-71).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-13-run-context`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(runs): ...`, `feat(messaging): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- Start check: `make check` is green on the fresh worktree, else `Verdict: BLOCKED base branch red`. M2-11 (`run_followups`), M2-12 (`agent_sessions`), M1-62 (`moveThread`) and M1-69 (`test/canary`) are merged, else `Verdict: BLOCKED dependency <ticket> missing`: this ticket reads their tables and paths (the header names M2-11 and M2-12; M1-62 and M1-69 are in its closure).
- No agent adapter, agent runtime, model or third-party service takes part in any test: the tests play the adapter with an agent token over HTTP (`pollAgentEvents`, `getRunContext`); real runtimes run only behind an explicit switch (09 16.1), and this ticket has none.
- Configuration values in tests are set through the named keys of the test harness; no limit is hard-coded in production code.

## Design notes

1. **Use case `ReadRunContext`** (`Runs.Context`), a read without a unit of work and without locks; every part is recomputed on each request (ruling 13). Order:
    a. Load the run with `GetRunForContext` (`id`, `thread_id`, `agent_id`, `triggered_by`, `state`, `trigger_kind`, `triggering_message_id`, `agent_session_id`, `queued_at`). An unknown id returns the error value of a `not_found` decision (`Decision.Err()` of M1-28), so status, body and headers equal an invisible run (02 9.2). Without an `authorize` call this is allowed because the response status is 404 (M1-18 check log).
    b. `Authorize(ctx, p, run.context.read, Resource{Kind: run, ID: run.id, ThreadID: run.thread_id, OwnerID: run.agent_id})`. `OwnerID` is the agent because `run.context.read` is agent-self (05 2.1 "own run"; ruling of M2-09 note 1); reuse the descriptor helper of M2-09 if it exists. M1-28 decides: thread not readable gives `404`; readable by another agent gives `403 forbidden` with reason `not_self`; a person or an app gives `403 forbidden` with `subject_kind_not_allowed`. Every non-allow returns `Decision.Err()`; `authz` audits the denial (aggregated per `audit.deny_aggregate_window`). An allow writes no audit entry (Audit `no`).
    c. `state` not in `queued`, `running`, `waiting_input`: the same `404` as in step a (ruling 4). `queued` is active: the adapter reads the context before `run.start` (04 15.3 point 4, 09 4.9, AG-28).
    d. `trigger_kind` other than `message`: an internal error (`internal_error`), logged with run id and trigger kind only. Such runs cannot exist before M2-15 and M2-16, and `run_context.v1` has no place for their trigger (spec gap); a `message` run whose `triggering_message_id` is NULL (the message row is gone) gives the `404` of step a.
    e. Thread facts through `chat.Threads.Info(run.thread_id)`: C = the thread's current channel, `Kind`, `ParentMessageID`, `OperatorAgentID`; for a main thread the channel's effective operator (M1-54).
    f. Session facts and plan (note 5).
    g. The triggering message M through `messaging.Messages.ContextMessage` (note 3). Missing or not readable by the agent: the `404` of step a (ruling 12). M is in T, or M is the parent message of T (auto thread, 09 3.7).
    h. `thread_context` through `Messages.ContextWindow` (note 6), `channel_context` when the plan wants it (note 6).
    i. Cut and budget (note 8), then `truncated_before_seq` (note 8).
    j. Trust (note 4): collect the distinct authors of the kept elements and of M with `origin = human` other than `triggered_by`, call `TriggerHolders.WithTrigger(ctx, Resource{Kind: channel, ID: C}, authors)` once (note 7), classify every element.
    k. `already_delivered` (note 9).
    l. Build `RunContext` (note 10). The handler answers `200` with the schema `RunContext`.
    Any source error (database, `TriggerHolders`) ends the request with that error's kind; it never turns into a context with weaker trust (fail closed, 09 1 point 4).

2. **Where each fact comes from** (ports only, 02 4.4 row `runs`):

    | Fact | Source |
    |---|---|
    | run row, previous run of a session, follow-up states | this module's queries (note 11) |
    | C, thread kind, parent message, fixed operator | `chat.Threads.Info` (M1-55; add `ParentMessageID` to `ThreadInfo` if missing) |
    | effective operator of C | the `chat` port of M1-54 (as M2-05 uses it) |
    | `history_backfill_limit` of C | the internal, non-authorizing read of the channel switches that M2-05 uses for the trigger decision (M2-04 `Switches`); extend it by `HistoryBackfillLimit` if missing. If no such read exists, add `ChannelSwitches(ctx, channelID ids.ID) (ChannelSwitches, error)` to `chat.AgentConfig`; it never returns the prompt or the skills (R-AZ-09) |
    | current session of T (`id`, `agent_id`, `started`) | `agents.Sessions` (M2-12) |
    | messages with author `PrincipalRef`, origin, kind, attachments | `messaging` (note 3); `runs` may not use `identity` or `files` |
    | which authors have `trigger` on C | `kernel/authorize.TriggerHolders` (note 7) |

3. **Messaging port additions** (the queries live in `messaging`, the owner of `messages`, 03 principle 12):

    ```go
    // ContextQuery selects messages of one thread for a run context (09 4.6). The caller has
    // authorized run.context.read on the run of this thread in the same request.
    type ContextQuery struct {
        ThreadID        ids.ID
        AfterSeq        int64  // exclusive lower bound (context_floor_seq, or 0)
        BeforeSeq       int64  // exclusive upper bound (thread_seq of M or of the parent message)
        Limit           int    // keep the newest Limit messages; 0 returns none
        ExcludeAuthorID ids.ID // zero: none; the agent itself in session mode resume
    }
    // ContextPage holds the selected messages ascending by thread_seq with full content.
    type ContextPage struct {
        Items    []MessageView
        HasOlder bool // a selectable message with AfterSeq < thread_seq < first item exists
    }
    // ContextWindow returns the window; tombstones (deleted_at set) are never selected.
    ContextWindow(ctx context.Context, q ContextQuery) (ContextPage, error)
    // ContextMessage returns one message with full content (tombstones as stored); ok is false
    // when it does not exist or the actor may not read it.
    ContextMessage(ctx context.Context, messageID ids.ID) (MessageView, bool, error)
    ```

    Both use the single representation function of M1-56 with the full `content_md` (never `message.list_content_max_bytes`), check every returned item with `authorize(actor, message.read, message)` in filter mode (`authorize.Filter(ctx)`), and drop a denied item. In `ContextWindow` a denied item is a filter mismatch: drop it, log at error level with only action and resource kind, and increment `bothy.authz.errors` with `bothy.reason = filter_mismatch` (07 7.4 rule 3). `ListContextWindow` is one statement with `LIMIT` and an `EXISTS` for `HasOlder`, over the unique index `(thread_id, thread_seq)`, filtering `deleted_at IS NULL` and the excluded author in SQL, so the limit counts only selectable messages (ruling 8).

4. **Classification** (`domain.Classify`, ruling 3), first matching row wins:

    | Message | `role` | `trust` |
    |---|---|---|
    | `origin = system` | `other` | `system` |
    | `origin = import` | `other` | `unverified` |
    | `origin = app` | `other` | `unverified` |
    | `origin = agent`, author is the run's agent | `assistant` | `self` |
    | `origin = agent`, other author | `other` | `unverified` |
    | `origin = human`, author is `runs.triggered_by` | `user` | `trigger` |
    | `origin = human`, author in `WithTrigger` result | `user` | `authorized` |
    | `origin = human`, otherwise | `user` | `unverified` |

    Origin decides before the author, so an imported message whose author is mapped to Alex stays `unverified` and the system line `thread.reopened` (author is a person, 03 `messages`) stays `system`. The triggering message is classified by the same table. Trust is a separate field of the response; no content, author or client field can set it (09 3.2: `origin` is set only by the server).

5. **Session mode, floor and own messages** (`domain.PlanContext`, ruling 5). Facts: whether the run's agent operates T (fixed `operator_agent_id` of a non-main thread, else the effective operator of C), the current session S of T (`agents.Sessions`), and whether a run of this agent with `agent_session_id = S` and `queued_at` before this run exists (`PreviousSessionTrigger`).
    - `guest`: the agent does not operate T (09 3.6, R-AG-04): no session, floor zero, own messages included, channel context wanted.
    - `new`: no current session, or S with `started = false` (09 9.1): floor zero, own messages included, channel context wanted.
    - `bind`: S with `started = true` and no earlier run of this agent in S: same as `new`.
    - `resume`: S with `started = true` and an earlier run in S: own messages excluded (`ExcludeAuthorID` = agent), no channel context, floor = `thread_seq` of the triggering message of the latest earlier run in S that has a triggering message; if that message lies outside T (it is the parent message of an auto thread) or no such run exists, the floor is zero.
    - Use the function of M2-12 that computes `session.mode` for `agent.invocation` if it exists, so delivery and context never disagree; otherwise put this one into `internal/runs/domain` and let M2-12's code call it.
    Reason: 09 4.6 says a continued session knows what was delivered to it; a session that never started (`started = false`) or a guest knows nothing, and a `thread_seq` of the main thread is no floor inside T.

6. **Windows.**
    - `thread_context`: if M lies in T, `ContextWindow{ThreadID: T, AfterSeq: floor, BeforeSeq: thread_seq of M, Limit: history_backfill_limit, ExcludeAuthorID: per note 5}`. If M is the parent message of T (auto thread, 09 3.4 point 5 and 3.7), T has no message before M: `thread_context` is empty and the floor is zero.
    - `channel_context` (ruling 6): only when the plan wants it, T has `parent_message_id` P, and P lies in a thread of C (the thread's current channel). Window: `ContextWindow{ThreadID: thread of P (the main thread of C), AfterSeq: 0, BeforeSeq: thread_seq of P, Limit: history_backfill_limit}`; P itself is not included (09 4.6 "before the parent message"). When P lies in another channel (the thread was moved, 03 `threads.parent_message_id`, M1-62), `channel_context` is absent: content of the source channel must not reach people who read only the target (T-AI-01, SR9, 07 6). Forum posts have no parent and no channel context.
    - Both windows use the channel's `history_backfill_limit` separately; the value zero gives empty windows.

7. **`kernel/authorize.TriggerHolders`** (ruling 1, ruling 2):

    ```go
    // TriggerHolders answers which people have effective level trigger on a target at decision
    // time. Implemented by authz; used by runs for the trust marker of the run context (09 4.6).
    type TriggerHolders interface {
        // WithTrigger returns, ascending and without duplicates, the ids among people that are
        // active humans with effective level trigger on the path of r. It writes no audit entry
        // and no check-log entry. An error never means "has trigger".
        WithTrigger(ctx context.Context, r Resource, people []ids.ID) ([]ids.ID, error)
    }
    ```

    Implementation in `authz`: `r` must be of kind `channel` (else internal error). A person counts when `PrincipalSource` says kind `user` and status `active` (not `pending`, `disabled` or `deleted`), and the effective level on the channel path (07 4.2 step 4) is `trigger`: snapshot of that person without credentials (as for `ViewAs`, cached the same way), grants of the person and of the groups in effect, the archive cap of 07 4.5 point 1 (nobody has `trigger` in an archived channel or space), DM rule (participants have `write`). An active owner counts on every non-DM channel: trust asks whether the person may instruct the agent, and an owner triggers from any session (05 2.3 point 1); `ActingFor` and `ViewAs` play no role here. Agents and apps never count. One call per request; it reads the rights version like every REST decision (07 7.3). If an earlier M2 ticket already added an equivalent query to `kernel/authorize`, use it (extend it to a batch if needed) and add no second one; name it in the report.

8. **Cutting and budget** (`domain.CutContent`, `domain.FitBudget`, rulings 8 and 9).
    - Per element of `thread_context` and `channel_context`: if `content_md` is longer than `agent.context_message_max_bytes` bytes, cut it at the longest valid UTF-8 prefix within the limit and set `truncated: true`. `trigger_message` is never cut and never counted (plan row: "the triggering message is never truncated").
    - Budget: only the bytes of `content_md` after the per-element cut count. Order all candidates chronologically, `channel_context` first (it lies before the thread), then `thread_context`; walk from the newest backwards and keep elements while the running sum stays within `agent.context_max_bytes`; the first element that does not fit and every older one are dropped. The kept elements are a contiguous newest suffix ("truncation from old to new", AG-21).
    - `truncated_before_seq` (refers to T only): when `ContextWindow` reported `HasOlder` or the budget dropped elements of `thread_context`, set it to the `thread_seq` of the oldest kept `thread_context` element, or to the `thread_seq` of M when none is kept; otherwise absent. The adapter loads older messages with `thread.read` (09 4.6).
    - Arrays in the response are ascending by `thread_seq`.

9. **`already_delivered`**: only in mode `resume`. `DeliveredFollowupsOfSession(session S, message ids of the kept elements)` returns the `message_id` of `run_followups` rows in `delivered` or `absorbed` whose run has `agent_session_id = S`; those elements get `already_delivered: true`.

10. **Response and API changes** (rulings 10 and 11).
    - `RunContext`: `run_id`, `context_floor_seq`, `trigger_message`, `thread_context` (always present, possibly empty), `channel_context` (present, possibly empty, exactly when note 6 builds it; absent otherwise), `truncated_before_seq` (only when set).
    - `RunContextMessage`: the fields of requirement 16; `channel_id` new and required (the channel of the message, which is C for every element after note 6); `truncated` and `already_delivered` only when true; `reply_to_message_id` only when set; `attachments` always present. Attachments map from the messaging representation (`Attachment.file`): `file_id`, `original_name`, `mime_type`, `size_bytes`, `scan_status`; `transcript` stays absent until M3-09.
    - openapi: add `channel_id` to `RunContextMessage` (`required`, `Uuid`, description "Channel of the message (SEC-AI-1)"), extend the descriptions named in Scope, update the example; regenerate the types and check them with `make codegen-check`; keep the `summary` of `getRunContext` unchanged, and if a generated block of chapter 06 still changes, regenerate it with `make api-tables` (checked by `make api-checks`). 09 4.6 gets the same field in its element list.
    - No JSON `null`; absent fields are omitted (ADR 0053, encoder of M1-18).

11. **Queries and the visible-channel predicate** (07 7.4 rule 1). The queries of notes 1, 5 and 9 (`GetRunForContext`, `PreviousSessionTrigger`, `DeliveredFollowupsOfSession`) read rows of exactly one run or one thread whose visibility `authorize(run.context.read)` checked first, which is the exception "loading a resource that has already been checked"; `ListContextWindow` and `GetContextMessage` read one thread or one message of the checked run's thread, the parent's main thread, or the triggering message, and every item passes `authorize(message.read)` in filter mode (note 3). Add each to the allowlist of `TestQueriesCarryVisibleChannelPredicate` and, if present, to the exception list of `tools/lint/sql_visibility.py` (category `after_authorize`) with this reason. Name the rule in the READMEs of `runs` and `messaging`. This also settles the open point of 07 7.2 ("How agents read threads") for the run context: through the run, not through `Scope`, so an agent that operates a thread in a channel it cannot otherwise read still gets its context.

12. **Configuration.** Register `agent.context_max_bytes` and `agent.context_message_max_bytes` (integer, minimum one, defaults of 09 section 21, chapter `09-agents.md`) in the schema of `internal/wire` if absent; run `make config-reference` and keep `make config-doc-check` green. `channel.history_backfill_limit_default` and `channel.history_backfill_limit_max` belong to M2-02. Code comments and docs name the keys, never their values.

13. **Logs and telemetry.** The handler and the use case log no content, no handle, no display name and no message id list; errors carry the run id at most. Spans follow the request span of the chain; no new metric (signals.md names none). Extend the canary scenario of M1-69 with a context read whose thread holds canary content and an author with a canary display name: the values may appear only in the `200` body to the agent (a reader with the right), never in a sink.

14. **Rulings for `docs/DECISIONS.md`** (one row `D-M2-13-<n>` each, with date, reason and place):
    1. The trust facts come from a new kernel port `authorize.TriggerHolders` (`WithTrigger`), implemented by `authz`; `runs` never derives a level and never imports `authz` (05 rule 1.1, 02 4.4). 02 4.5 names the port.
    2. An active owner counts as `authorized` on every non-DM channel; people who are not active, agents and apps never count.
    3. Classification order of note 4: origin before author.
    4. Only own runs in `queued`, `running` or `waiting_input` have a context; any other state answers the same `404` as an unknown run.
    5. Session mode, floor and own messages after note 5; the floor is non-zero only in mode `resume` and only when the previous trigger lies in T; channel context exactly in modes `new`, `bind` and `guest`.
    6. `channel_context` only when the parent message lies in the thread's current channel; after a move it is absent; the parent message itself is not part of it.
    7. Tombstones (deleted messages) are not part of `thread_context` or `channel_context` and count towards no limit; the triggering message appears as stored.
    8. Byte accounting counts only `content_md` after the per-element cut at a UTF-8 boundary; the budget keeps a contiguous newest suffix across `channel_context` then `thread_context`; exclusions apply before `history_backfill_limit`.
    9. `truncated_before_seq` refers to T and is the `thread_seq` of the oldest kept element of `thread_context`, or of M when none is kept.
    10. `RunContextMessage.channel_id` (required) carries the channel of each element (SEC-AI-1).
    11. Optional fields appear only when they apply; `channel_context` is present (possibly empty) exactly when it is built.
    12. A triggering message the agent cannot read ends the request with `404`; a run with `trigger_kind` other than `message` is an internal error until M2-15 and M2-16 define its context.
    13. A context read opens no unit of work and takes no lock; each request recomputes every part, including trust.

## Tests (write them first)

Unit:

- `internal/runs/domain`: `TestClassify` with one row per line of the table in note 4, plus an import authored by the triggering person (`other`, `unverified`), a system line authored by Alex (`other`, `system`), and the triggering message (`user`, `trigger`).
- `internal/runs/domain`: `TestPlanContext`: guest; no session; session with `started = false` and an earlier failed run (`new`, floor zero); `bind`; `resume` with the previous trigger in T (floor equals its `thread_seq`, own messages excluded, no channel context); `resume` whose previous trigger is the parent message (floor zero); `resume` whose latest earlier run has no triggering message (the run before it decides).
- `internal/runs/domain`: `TestCutContent`: below, equal to and above the limit; a multi-byte character across the limit is not split; the result is valid UTF-8.
- `internal/runs/domain`: `TestFitBudget`: newest kept, oldest dropped; channel elements dropped before thread elements; the kept elements are contiguous; the sum never exceeds the limit; an element larger than the whole budget drops everything older too; the trigger is not an input.
- `internal/runs/domain`: `TestTruncatedBeforeSeq`: `HasOlder` only, budget drop only, nothing kept, nothing missing.
- `FuzzCutContent` in `internal/runs/domain`: arbitrary bytes and limits never produce invalid UTF-8 or a result above the limit.
- `internal/authz/app`: `TestTriggerHolders`: grant `trigger` on the channel (in), on the space (in), `write` only (out), group grant `trigger` (in), expired grant (out), active owner without grant (in), disabled person (out), pending person (out), an agent id (out), archived channel (out), archived space (out), duplicates in the input (once), unknown id (out), source error (error, no result).
- `internal/messaging/app`: `TestContextWindow`: newest `Limit` kept, ascending order, tombstones and the excluded author not selected and not counted, `HasOlder` exact at both edges, full content above `message.list_content_max_bytes`, filter-mode check with a denied item counted as `filter_mismatch`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain on `127.0.0.1:0`, harness `test/harness`, people and agent tokens from the identity test helpers, fake clock; world of M2-09: space Support with channel Allgemein whose operator is the agent Lotse, space Marketing with the agent Texter; Robin is the owner, Alex has `trigger` on Support, Sam has `trigger` on Allgemein unless a test changes it, a person with the handle `writer` (address at `example.com`) has `write` only, Texter has a `write` grant on Allgemein and Allgemein allows mentioned agents where a test needs a guest; a run is triggered by Alex and delivered through M2-08 and `pollAgentEvents`; messages with `origin` `agent`, `app` and `import` are written through the real path where the base has one, otherwise through a test helper of `messaging` that writes its own table, as M1-57 does for tombstones):

- `TestRunContext_TrustMarkers_AG20`: before Alex's trigger the thread holds messages of Sam (`authorized`), of `writer` (`unverified`), of Texter as guest (`unverified`, role `other`), of an app (`unverified`), an import authored by Sam (`unverified`, role `other`), a `thread.reopened` line (`system`), an earlier message of Lotse while the thread has no session (mode `new`; `self`, role `assistant`) and a message of Robin, who as owner has no grant (`authorized`); `trigger_message` has `trust = trigger` and role `user`. Every element has `author` with `id`, `kind`, `handle` and `display_name` and no e-mail address (AG-20 server part for authors without `trigger`, other agents and imports).
- `TestRunContext_SEC_AI_1`: every element of all three parts carries `author`, `origin`, `kind` and `channel_id` equal to Allgemein, `role` and `trust`; `reply_to_message_id` is present on a reply; `attachments` is an empty array.
- `TestRunContext_TrustAtRequestTime`: Sam's grant is revoked between two runs (ADR 0054 "Confirmation"): Sam's message is `authorized` in the first context and `unverified` in the second; a second read of the same run after the revocation also shows `unverified`.
- `TestRunContext_TriggerNeverTruncated`: with `agent.context_message_max_bytes` and `agent.context_max_bytes` set below the size of Alex's message, `trigger_message.content_md` is byte-equal to the stored content, has no `truncated`, and does not reduce the budget of the other parts.
- `TestRunContext_SizeLimits_AG21`: with small limits and a thread of messages whose sum exceeds `agent.context_max_bytes`, each long element is cut at `agent.context_message_max_bytes` on a UTF-8 boundary with `truncated: true`, the sum of `content_md` stays within `agent.context_max_bytes`, the oldest elements are missing, the newest are present, and `truncated_before_seq` equals the oldest kept `thread_seq`.
- `TestRunContext_BackfillLimit`: with `history_backfill_limit` below the number of earlier messages only the newest are present and `truncated_before_seq` is set; with the value zero `thread_context` is empty and `truncated_before_seq` equals the `thread_seq` of M.
- `TestRunContext_ResumeFloorAndAlreadyDelivered`: first run started and ended with an answer of Lotse and an `absorbed` follow-up (M2-11), session started (M2-12); second run in mode `resume`: `context_floor_seq` equals the `thread_seq` of the first trigger, Lotse's answers are absent, the follow-up has `already_delivered: true`, and no message at or below the floor appears.
- `TestRunContext_NewAndBindIncludeOwnMessages`: after `resetAgentSession` (mode `new`) and after `bindAgentSession` (mode `bind`) Lotse's earlier answers are present with `self`, the floor is zero and `channel_context` is built when the thread has a parent.
- `TestRunContext_GuestRun`: Texter mentioned as guest (`allow_mentioned_agents`): floor zero, own earlier messages present, no session needed, `channel_context` built for a thread with parent.
- `TestRunContext_AutoThreadFirstRun`: Alex mentions Lotse in the main thread with `auto_thread`; the run in the new thread has `trigger_message` = M from the main thread, `thread_context` empty, `channel_context` = the main-thread messages before M (newest within `history_backfill_limit`), all with `channel_id` Allgemein; a second run in that thread in mode `resume` has floor zero and sees the new thread's messages.
- `TestRunContext_ChannelContextAbsentAfterMove`: a thread created from a main-thread message of Allgemein is moved (`moveThread`, M1-62) to another Support channel with the same operator; the first run's context has no `channel_context`, and no content of Allgemein's main thread appears anywhere in the body.
- `TestRunContext_TombstonesOmitted`: a deleted earlier message is absent and does not count towards `history_backfill_limit`; a deleted triggering message appears with empty `content_md`.
- `TestRunContext_QueuedRunReadable`: the context is read while the run is `queued` (before `startRun`) and while `waiting_input`.
- `TestRunContext_NotFoundEquality`: an unknown run id, a run in Marketing that Lotse cannot read, and Lotse's own run after `finishRun` give `404` with equal status, body (without `correlation_id` and `instance`) and headers (without `Correlation-Id`) (AZ-P4 status and body part).
- `TestRunContext_ForeignAndHuman`: Texter with read on Allgemein asking for Lotse's run gets `403 forbidden` with `reason` `not_self`; Alex with her session gets `403 forbidden` with `reason` `subject_kind_not_allowed`; each denial produces one aggregated deny entry in the audit log; a successful read produces no audit entry (R-AZ-04, 05 Audit `no`).
- `TestRunContext_SchemaAndNoNull`: every `200` body of this file validates against `RunContext` of `docs/api/openapi.yaml` and contains no JSON `null`; absent optional fields are omitted.
- `TestRunContext_RouteMetadata`: the generated route table names `run.context.read` and `rl.agent_runtime` for `getRunContext`, and the route is registered (AZ-P1).
- `TestRunContext_NoContentInLogs`: with the harness log writer, a context read over a thread with distinct content and display names leaves none of them in the captured log output.
- `test/canary`: the scenario includes `getRunContext`; the canary test stays green (SEC-T10).

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/runs/... ./internal/messaging/... ./internal/chat/... ./internal/agents/... ./internal/authz/... ./internal/kernel/... ./internal/wire/... ./test/integration/runcontext/... ./test/canary/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make register-check`, `make config-doc-check`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `getRunContext` is enabled and registered with `run.context.read` and `rl.agent_runtime`; AZ-P1 passes.
- [ ] AG-20 (server part): authors without `trigger`, other agents, apps and imports are `unverified`; the triggering person is `trigger`, other active people with `trigger` (owners included) are `authorized`, the own agent is `self`, system lines are `system`; roles follow 09 4.6.
- [ ] Trust is computed per request: a person who lost `trigger` is `unverified` in the next context.
- [ ] AG-21: contents are cut at `agent.context_message_max_bytes`, the sum stays within `agent.context_max_bytes`, cutting goes from old to new, and `truncated_before_seq` marks the gap.
- [ ] The triggering message is never cut and never counted.
- [ ] SEC-AI-1: every element carries author, `origin`, kind and `channel_id`; `openapi.yaml` and 09 4.6 name `channel_id`.
- [ ] Floor, own messages, `already_delivered` and `channel_context` follow the session mode; after a thread move no content of the source channel is in the context.
- [ ] Only the own agent reads the context of its active run; unknown, invisible and ended runs give the same `404`; denials are audited, reads are not.
- [ ] No content, handle or display name in logs; the canary scenario includes the context read; no JSON `null`.
- [ ] Rulings recorded in `docs/DECISIONS.md` section 10; 09 4.6 and 02 4.5 updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `runs`, `messaging`, `chat` and `authz` (and `agents` if its port changed) updated (ports, the context use case, `TriggerHolders`, the query exceptions of note 11), package doc of `kernel/authorize`.
- Rulings of note 14 recorded in `docs/DECISIONS.md` section 10 (one row each with reason); `docs/spec/09-agents.md` 4.6, `docs/spec/02-architecture.md` 4.5 and `docs/api/openapi.yaml` updated in the same change.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
