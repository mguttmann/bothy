# M2-19: Agent replies and announcements

Status: ready | Depends on: M2-09 | Milestone: M2

## Goal

Agents write into the chat only where their rights allow it, and nothing they write ever sets an agent in motion. An agent with its API token answers in the thread of its run and posts announcements without a human trigger (09 "Reports without a trigger": a new thread per report or a fixed thread) only where it has `write` through a grant or the operator rule; anywhere else it gets `404` (no read right) or `403` (read right only), which is AG-26. Every agent message carries `origin = agent` and an `author` of kind `agent`, set by the server and present in every representation (REST answer, replay, `message.created`, catch-up, gateway), so clients can show the origin badge (19 SEC-OUT-3); a client can never set `origin` or write a system line. No agent message creates a run, a follow-up, a `trigger_state` or an `agent.invocation`, with or without mentions, in free channels and in threads with an active run (AG-01 for `origin = agent`, 07 SR4, T-AI-07). `MessageCreate.run_id`, rejected with `400 validation_failed` and `errors[].code` `not_enabled` since M1-56, is enabled: an agent may name an active run of its own in the target thread, every other run is refused with one uniform answer, and a run cannot end while such an answer is being written. After this ticket the server side of AG-01 (agent part) and AG-26 is proven by tests that were red first, as M2-31 requires.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope, Out of scope, Exit criteria 1 and 10, Test strategy, Red team, Risks, block "agents and extensions"); the row M2-19 (this ticket); the rows M2-04, M2-05, M2-08 and M2-09 (what exists), M2-11, M2-13, M2-14, M2-15, M2-18, M2-20, M2-27, M2-29, M2-31 and M2-33 (who builds what next to this ticket); the rows M3-17 to M3-19 (notifications of agent messages) and M8-03 (read-only channels).
- Dependencies: the header names M2-09. Implied through it and therefore not in the header: M2-08, M2-07, M2-06, M2-05, M2-04, M2-02 and the M1 tickets M1-13, M1-16, M1-18, M1-19, M1-24, M1-28, M1-29, M1-31, M1-44, M1-51, M1-53, M1-54, M1-55, M1-56, M1-61, M1-63, M1-65 and M1-68 (`bothy.message.created`). Not in the closure, used only if merged: M1-57 (history reads), M1-70 (SQL visibility lint), M2-10 (shares the reason `run_not_active`), M2-11 (`run_followups`), M2-15 (cards).
- `docs/spec/09-agents.md`: 1 "Principles"; 2.3 "Grants of an agent" (operator rule, `write` through a grant for guests and report channels); 3.1 "Operator"; 3.2 "Origin (`messages.origin`)"; 3.4 "Decision per message" point 1; 3.6 "Guest agents"; 4.1 "Overview" (answers over the normal API with the agent token); 4.7 "Adapter operations" (row "Send reply": `message.create`, `client_msg_id`, `origin = agent`); 5.1 "State machine"; 5.4 "Live box content" (the anchor is the only message with `run_id`); 7 "Follow-ups during a run"; 11.5 "Approval buttons" (a card with the run as `run_id`, built by M2-15); 12 "Auto-title"; 14 "Reports without a trigger" (points 1 to 5 and the limit); 15 "Reliability" (row "Answer lost while sending", the adapter outbox); 16.2 "Required scenarios" (AG-01, AG-26); 17.9 "Reports from cron jobs".
- `docs/spec/05-actions.md`: rules 1.1, 1.2 and 1.8; 2.3 "Effective level" points 4, 5 and 6; 2.4 (column Audit, aggregated denials, R-AZ-04); 2.6 "Scopes for apps" (agents have the scopes implicitly); 4.6 row `thread.create`; 4.7 row `message.create` (`write`, `rl.message`, Audit `no`); 5 "Agent invocation `agent.invoke`" point 1.
- `docs/spec/07-permissions.md`: 2.9 "Agents as principals"; 3 "Principal context" (`acting_for`); 4.1 "Decisions and reasons" (`no_grant`, `level_insufficient`, `channel_archived`, `origin_not_human`); 4.5 "Archived spaces and channels"; 4.6 "Resolving mentions"; SR4 "Only humans trigger"; SR9 "Acting-for binding (confused deputy)" (exception: answers in the thread of the run); 10 "Examples" (the world); 11.1 rows AZ-M27, AZ-M28, AZ-M29, AZ-M30; 11.2 rows AZ-P1, AZ-P4, AZ-P13.
- `docs/spec/03-data-model.md` (core contract, read only): `messages` (`origin`, `kind`, `run_id` "for `kind = 'run'`", the checks `(kind = 'run') = (run_id IS NOT NULL)` and `trigger_state IS NULL OR origin = 'human'`, the unique pair `(author_principal_id, client_msg_id)`, the partial unique index on `run_id`), `runs` (`state`, `agent_id`, `thread_id`, the partial unique index of active runs, R-DM-14), `threads` (`operator_agent_id`, `title_source`), 6.3 "Lock order".
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope"; 5.1 "Messages: `client_msg_id`" (point 3: the parts of the canonical request); 10 "Ordering"; 15.3 "Invocation and run start"; 15.5 "Cancellation"; 16.5 "Agent run with follow-up" (the adapter posts its answer before `finish`).
- `docs/spec/06-api.md`: 2 "Basic rules" (rows "Unknown values", "No `null`"); 4 "Authorization in the API" points 3 and 6; 5 "Errors" points 3, 4 and 7; 7 "Idempotency" (row "Send a message").
- `docs/spec/02-architecture.md`: 4.1 "Rules" point 4 (source port or in-transaction event); 4.3 "Ports, events and sources per module" (rows `messaging`: `MessagePosted`; `runs`: subscribes `messaging.MessagePosted`); 4.4 "Allowed dependencies" (`messaging` may not use `runs`); 8.1 "Flow", 8.2 "Lock order", 8.3 "In-Tx events" (a subscriber may veto).
- `docs/spec/13-push-notifications.md`: 2 "Kinds of notifications and recipients" (rows `message`, `run_done`), 2.1 "Agent messages during a run", 2.2 "Report channels" (only to know what M3 consumes; nothing of it is built here).
- `docs/spec/19-security.md`: SEC-OUT-3, SEC-AI-1, SEC-IN-1.
- `docs/api/openapi.yaml`: operations `createMessage`, `createThread`, `listEvents`, `pollAgentEvents`, `startRun`, `awaitRunInput`, `finishRun`, `cancelRun`, `createGrant`, `setChannelOperator`, `archiveChannel`, `closeThread`; schemas `MessageCreate` (`run_id`: "Agents only; the run this answer belongs to."), `Message` (`origin`, `author`, `run_id`, `trigger_state`), `MessageOrigin`, `MessageKind`, `PrincipalRef`, `PrincipalKind`, `ThreadCreate` (`first_message`), `ThreadCreated`, `OperatorSet`, `MessageCreatedEvent`, `MessagePayload`, `Problem` (`x-problem-codes`), `FieldError`; response `Conflict`; header `Idempotent-Replay`.
- `docs/api/asyncapi.yaml`: message `event.message.created`.
- ADRs 0008, 0009, 0013, 0015, 0017, 0022, 0025, 0053, 0054.
- `docs/DECISIONS.md`: D-W-100; section 10 for the format of new rows.
- `docs/security/threat-model.md`: T-AI-04 (agent writes into foreign channels), T-AI-07 (loops), T-AI-01, T-TK-02. `docs/RISKS.md`: RK-01 ("agents never trigger agents"). No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- `docs/observability/signals.md` 4.2: `bothy.message.created` with `bothy.message.origin` (recorded by M1-68, which is in the closure; assert it).
- `docs/operations/configuration.md`: `ratelimit.rl.message.per_minute`, `ratelimit.rl.message.burst`, `database.tx_retry_max`, `audit.deny_aggregate_window`, `message.max_content_bytes`.
- M1 briefs: M1-13 (`txn.Publish`, subscriber veto returns the error unchanged, golden registrations), M1-16 (`eventlog.Appender`), M1-18 (problem writer, `StageValidate` and its `errors[].code` vocabulary), M1-19 (rate-limit classes per principal), M1-24 (deny aggregation), M1-28 (descriptor, `message.create` requirement `All(Level(write), CapIf(upload, HasAttachments))`, operator rule in the effective rights, decision to error mapping), M1-31 (response validator in `services/server/test/apitest`, test authenticator builders in `internal/platform/httpserver/httpservertest`), M1-44 (agent API tokens), M1-51 (agent records), M1-53 (`archiveChannel`), M1-54 (`setChannelOperator`, `OperatorSet.mode`), M1-55 (`chat.Threads`, title source: agents always give `auto`), M1-56 (`createMessage`, `createThread`, `MessagePosted`, D1 flow, D2 body rules with the `not_enabled` row for `run_id`, D3 request hash, D7 events), M1-61 (the `sync` subscriber of `MessagePosted`), M1-63 (`listEvents`), M1-65 (gateway delivery, test client).
- M2 briefs: `docs/briefs/M2-04-trigger-decision.md` (AG-01 at decision level), `M2-05-runs-state-machine.md` (acceptance subscriber `OnMessagePosted`, its first step "return unless `Origin = human`", `CreateRunAnchor`, event order of note 7), `M2-07-agent-consumers.md` (`pollAgentEvents`), `M2-08-invocation-delivery.md` (delivery, AZ-M30), `M2-09-run-lifecycle.md` (start, wait, finish, cancel, the end-of-run function and its locks), and `M2-10-run-document-live-box.md` if present (reason `run_not_active` for patches).
- Code on the base branch: `internal/messaging` (port, `domain`, `app`, `adapters/httpapi`), `internal/runs` (port, `domain`, `app`, `adapters/postgres`), `internal/authz`, `internal/kernel/authorize`, `internal/kernel/errs`, `internal/kernel/txn`, `internal/kernel/eventlog`, `internal/kernel/clock`, `internal/wire` (golden registrations, the test hook for `MessagePosted` subscribers of M1-56).

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

11. An agent writes a message (`message.create`) or creates a thread (`thread.create`) only with effective `write` on the target: from a grant (at most `write` and `upload`) or from the operator rule (effective operator of the channel, fixed operator of a thread). Without read right the answer is `404`, identical to an unknown thread; with read right only `403 forbidden` with `reason` `level_insufficient`; in an archived channel `403 channel_read_only` with `reason` `channel_archived`, also for the operator (05 2.3 points 4 and 5, 4.6, 4.7, rule 1.2; 07 2.9, 4.1, 4.5; 09 2.3; AG-26; AZ-M27 to AZ-M29).
12. `origin` is set by the server from the principal: an agent token gives `agent`; the request cannot carry `origin`, and `kind` cannot be `system` or `run` (09 3.2; 19 SEC-OUT-3; 06 2 "Unknown values"; SEC-IN-1).
13. An agent message never triggers: no run, no follow-up, no `trigger_state`, no `agent.invocation`, whatever it mentions and in whatever channel (09 3.2 and 3.4 point 1; 05 5 point 1; 07 SR4; 03 check `trigger_state IS NULL OR origin = 'human'`; AG-01; T-AI-07).
14. `MessageCreate.run_id` ("Agents only; the run this answer belongs to", openapi) is accepted only from an agent and only for an active run (`queued`, `running`, `waiting_input`) of that agent in the target thread; it is never written to `messages.run_id`, which names only the run of a live-box anchor (03 `messages`, 09 5.4). Design notes 3 to 6 fix the answers.
15. Announcements without a trigger: the agent writes with `message.create` into a channel where it has `write`, either one new thread per report (`thread.create`, title from the first line, `title_source = auto`) or a fixed thread; there is no run and no invocation token; the agent's messages count against `rl.message` of the agent; a human answer in such a thread triggers by the normal rules (09 14 points 1 to 5; 09 12 point 1).
16. Messages are idempotent by `(author_principal_id, client_msg_id)`; a repeated answer returns the original with `200` and `Idempotent-Replay: true`, also after the run ended (04 5.1; 06 7; 09 15 "Answer lost while sending").
17. No JSON `null` in answers and events; every response validates against `openapi.yaml` (ADR 0053; 06 2 "No `null`").

## Scope

All paths under `services/server/` unless noted. Names of M1 and of M2-04 to M2-09 below are the expected ones; where the merged code names them differently, follow the code (Boundaries).

Create:

- `internal/runs/domain/answer.go`: `type AnswerRun struct{ ID, ThreadID, AgentID ids.ID; State State }` and the pure function `AnswerVerdict(run AnswerRun, found bool, agentID, threadID ids.ID) error`, returning nil, `ErrAnswerRunMismatch` or `ErrAnswerRunNotActive` (Design note 4). `State` is the run state type of M2-05.
- `internal/runs/app/answer.go`: `checkAnswer(ctx context.Context, ev messaging.MessagePosted) error` on the merged app type of `runs`; it loads the run through the repository, calls `AnswerVerdict` and maps the domain errors to `kernel/errs` (Design note 4).
- `internal/runs/adapters/postgres/queries/run_answer.sql`: `LockRunForAnswer` (`SELECT id, thread_id, agent_id, state FROM runs WHERE id = $1 FOR SHARE`) and the regenerated sqlc code; the repository method behind it.
- Tests as listed under "Tests (write them first)", in the test packages the merged modules already use for unit and integration tests.

Change:

- `internal/messaging` port: `MessagePosted` gets `RunID` (optional, the same optional-ID form the merged struct uses for `ReplyToID`), documented as "the run this message belongs to: for `kind = run` the anchored run (`messages.run_id`); for an agent message of any other kind the run the request named in `run_id`, after the body rules of Design note 3; absent otherwise".
- `internal/messaging/domain` (body rules of M1-56 D2): `run_id` is no longer `not_enabled`; a `run_id` from a principal that is not an agent gives `400 validation_failed` with pointer `/run_id` and `errors[].code` `run_mismatch`; `first_message.run_id` in `createThread` gives the same with pointer `/first_message/run_id` (Design note 5). The `not_enabled` rows for `kind = card`, `components`, `kind = voice` and `attachment_file_ids` stay as they are on the base.
- `internal/messaging/app`: `createMessage` puts the validated `run_id` of an agent into `MessagePosted.RunID`; `CreateRunAnchor` (M2-05) sets `RunID` to the anchored run. Nothing else in the write path changes; `run_id` is not part of the request hash (Design note 6).
- `internal/runs/app` acceptance subscriber `OnMessagePosted` (M2-05 `accept.go`, extended by M2-08; follow the merged file): a first branch for agent messages that calls `checkAnswer` and then returns without any trigger work (Design note 2). No new subscription: `internal/wire/testdata/txn_registrations.golden` stays unchanged.
- The M1-56 test that asserts `not_enabled` for `run_id` (`TestCreateMessage_NotEnabledMembers400` or its merged name): change only the `run_id` row to the new rule of Design note 3; keep every other row and the test.
- `docs/api/openapi.yaml` (descriptions only, no schema change): `MessageCreate.run_id` names the rule of Design notes 3 and 4 (agents only; an active run of the calling agent in this thread; validated, not stored; `400 validation_failed` with `errors[].code` `run_mismatch`, `409 conflict` with `reason` `run_not_active`; not part of the canonical request of `client_msg_id`); `Message.run_id`: "Set only for `kind = run`: the run whose live box this anchor holds."; `ThreadCreate` description: `first_message.run_id` is never valid because a new thread has no run. Then the server code generation (M1-17) and the API gates.
- `docs/spec/09-agents.md` (English, like the chapter, not core contract): 4.7 row "Send reply": optional `run_id` only for an active run of the agent itself in the target thread, otherwise `400 validation_failed` with `errors[].code` `run_mismatch`, or for an ended run `409 conflict` with `reason` `run_not_active`; checked, not stored; the adapter sends replies before `run.finish` (04 16.5) and sends a reply refused after the end of the run again without `run_id`. 14 point 1: "without `run_id`".
- `docs/security/threat-model.md`: T-AI-04, column Mitigations: `rl.message` instead of `rl.agent_runtime` (05 4.7 gives `message.create` the class `rl.message`, 09 14 point 4 says the same); add AG-26 to its tests; add AG-01 to the tests of T-AI-07.
- `internal/messaging/README.md` and `internal/runs/README.md`: the `run_id` rule, `MessagePosted.RunID`, the answer branch of `OnMessagePosted`, the `FOR SHARE` lock and why (Design note 7); `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` section 10 rows `D-M2-19-<n>` for every Ruling below.
- If `tools/lint/sql_visibility.py` is on the base (M1-70): list `LockRunForAnswer` as an exception of category `after_authorize` with the reason of Design note 4e.

This ticket creates no migration, no new operation (`createMessage` and `createThread` are enabled since M1-56), no new action, event, job, metric or configuration key and no new top-level make target.

## Out of scope

- Cards, components, `kind = card`, `message_components.run_id`, `restricted_to` and approval cards (M2-15); if M2-15 is already merged, see Design note 9.
- SR9, `acting_for` and the invocation token in the request header (M2-14); this ticket reads no token header.
- The run context, the trust marks of agent messages in the context of other runs and `role` (M2-13); follow-ups and `run_followups` (M2-11); agent sessions (M2-12); titles set by the operator agent with `thread.title.update`, AG-19 (M2-18).
- Files from agents (`attachment_file_ids`, `+upload`, capability `files_out`) (M3-07); voice messages (M3-09).
- Notifications of agent messages: no `message` hint during an active run, `run_done` pointing at the last agent message, `channels.default_notification_level` of report channels (M3-17 to M3-19).
- App messages, incoming webhooks and AG-01 for `origin = app` (M2-24, M2-27); imports and AG-01 for `origin = import`, read-only channels (`channels.read_only`) (M8-03).
- Reports from cron jobs of Hermes Agent (`cron_deliver_env_var`, `standalone_sender_fn`, 09 17.9) (M2-33); the SDK outbox for answers (M2-29); the conformance suite with the fake agent (M2-31).
- The live-box anchor (M2-05), start, finish and cancel of runs (M2-09), patches and `run_not_active` for patches (M2-10).
- Rendering of the origin badge in the apps (M4).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-19-agent-messages`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(runs): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for M2-02 and M2-04 to M2-09: their merged names win over the expected names here; list each difference. If the base has no acceptance subscriber of `MessagePosted` in `runs` or no `startRun` and `finishRun`, a dependency is missing: stop with `Verdict: BLOCKED <reason>`.
- No real agent runtime and no model call: the adapter in the tests is plain Go HTTP code with an agent API token; no third-party service.
- Time only through `kernel/clock`; tests move a fake clock and never sleep for a window.
- Invocation tokens and API tokens never appear in logs, problem bodies, event payloads or test failure messages.
- A mutation used to prove a red run (Tests, last paragraph) is never committed.

## Design notes

1. **Where the rules live.** `authz` already decides who may write (M1-28: requirement of `message.create` and `thread.create`, operator rule, limits for agents, archived channels); this ticket changes no rights code and proves it end to end. `messaging` sets `origin`, applies the body rules and carries `run_id` in `MessagePosted`. `runs` checks the named run, because `messaging` may not import `runs` (02 4.4) and `runs` already subscribes `MessagePosted` (02 4.3). Ruling for docs/DECISIONS.md: the check of `run_id` is a branch of the `runs` subscriber of `MessagePosted` that vetoes the unit of work (02 4.1 point 4, 8.3), not a new source port.

2. **`createMessage` for an agent**, extending M1-56 D1 and M2-05 note 7, in this order:
   a. Decode with the generated decoder: a member `origin` or `system_code` gives `400 validation_failed` (`unknown_member`); `kind` other than `text`, `voice` or `card` gives `400 validation_failed` (`invalid_enum`), so neither `system` nor `run` can be sent (19 SEC-OUT-3: system lines come only from the server).
   b. `chat.Threads.Info(thread_id)`; unknown gives `404`.
   c. `authorize(agent, message.create, thread)`. No read right gives `404` with exactly the body and headers of an unknown thread id (AZ-M28, AZ-P4); read right without `write` gives `403 forbidden` with `reason` `level_insufficient`; an archived channel or space gives `403 channel_read_only` with `reason` `channel_archived`. Every denial is recorded by the authz layer, aggregated per `audit.deny_aggregate_window` (05 2.4, AZ-P13); an allow writes no audit entry (Audit `no`).
   d. Body rules (M1-56 D2 plus Design note 3).
   e. Unit of work as M1-56 D1 step 5: lookup of `(author, client_msg_id)` first; a replay answers the original and publishes nothing, so no run check runs on a replay (Design note 6). A new message: `AppendToThread` (reopens a done thread with the system line of M1-56 D6, author the agent, `origin = system`), insert with `origin = agent`, `trigger_state` NULL, mentions after 07 4.6, `message.created` with `actor {id, kind: agent}` (M1-56 D7), then `MessagePosted` with `Origin = agent`, `AuthorKind = agent` and `RunID`.
   f. `runs.OnMessagePosted`, first branch: `Origin = agent` and `Kind` neither `run` nor `system` and `RunID` set: `checkAnswer` (note 4); an error vetoes and rolls everything back, including the thread sequence. Then, for every message with `Origin` other than `human`, return without any trigger work, as M2-05 note 6 step 1 already does (no run, no follow-up, no `SetTriggerState`, no audit). A message with `Origin` other than `agent` and `RunID` set and `Kind` other than `run` is an internal error, because `messaging` never builds it.
   g. Commit; `201` with the stored `Message`.

3. **Body rules for `run_id`.** Absent: nothing to check. Present on a request whose principal is not an agent (person, app): `400 validation_failed`, pointer `/run_id`, `errors[].code` `run_mismatch` and the `detail` of note 4b (a run belongs to an agent, so it can never be theirs). Present on an agent request: carried unchanged into `MessagePosted.RunID`; the run itself is checked in `runs` (note 4).

4. **The answer check** (`checkAnswer`, `AnswerVerdict`).
   a. `LockRunForAnswer(run_id)` with `FOR SHARE` (note 7).
   b. Not found, `runs.thread_id` different from `ev.ThreadID`, or `runs.agent_id` different from `ev.AuthorID`: `ErrAnswerRunMismatch`, mapped to `400 validation_failed` with pointer `/run_id`, `errors[].code` `run_mismatch` and the same `detail` for every case. Reason: an unknown run, a run in another thread and a run of another agent answer alike, so `run_id` cannot probe runs outside the authorized thread (05 rule 1.2).
   c. Ownership first, state second: only for an own run in this thread does an end state (`completed`, `failed`, `cancelled`) give `ErrAnswerRunNotActive`, mapped to `409 conflict` with `reason` `run_not_active`. Active means `queued`, `running` or `waiting_input`, as in 03 `runs` (R-DM-14) and 13 2.1. Ruling for docs/DECISIONS.md: `run_mismatch` and `run_not_active` with these statuses; `run_not_active` is the same reason M2-10 uses for patches of an ended run.
   d. No `authorize` call for the run: a run of the thread is readable by every principal with `write` on that thread (05 2.2: a run's path is its thread), and runs outside the thread answer like unknown ones. Ruling for docs/DECISIONS.md.
   e. `LockRunForAnswer` reads one row by primary key inside the unit of an authorized `message.create`, and only a verdict leaves the module; this is its reason as an `after_authorize` exception of the SQL visibility lint (07 7.4 rule 1) where that lint exists.
   f. A valid `run_id` changes nothing on the run: no new `version`, no `run.state_changed`, no step, no follow-up, no `message.updated` of the triggering message.

5. **`createThread` with `first_message.run_id`**: always `400 validation_failed` with pointer `/first_message/run_id` and `errors[].code` `run_mismatch`, checked in the body rules before the unit of work, because a thread created by the request has no run. Ruling for docs/DECISIONS.md.

6. **Not stored; idempotency.** `messages.run_id` stays NULL for every message except the anchor (03 check `(kind = 'run') = (run_id IS NOT NULL)`), so `Message.run_id` of an answer is absent in the `201`, the replay, events and history. `run_id` is not part of `client_payload_sha256`: 04 5.1 point 3 names the parts of the canonical request (thread, content, reply, attachments), and M1-56 D3 keeps its version prefix. Consequences: a replay of an answer returns the original with `200` and `Idempotent-Replay: true` even after its run ended (09 15); an answer refused with `run_not_active` stored nothing, so the adapter can send the same `client_msg_id` again without `run_id`, which then is a normal agent message. Ruling for docs/DECISIONS.md: `run_id` is validated, carried in `MessagePosted` and never stored; it is not part of the request hash. The gap in the core contract goes into the report (spec gap).

7. **Lock and order.** The answer check takes the run row `FOR SHARE` after `messaging` holds the thread row (M1-55 `AppendToThread`), the same order as the acceptance of M2-05 (thread, then run). `finishRun`, `cancelRun`, the cancel timeout and the stall check of M2-09 update the run row and so wait for the answer's commit, or the answer waits for theirs and then sees the end state. Result: every answer stored with a `run_id` commits while its run is active, and its `message.created` has a lower `seq` than the `run.state_changed` with the end state (04 10). Deadlocks with an end-of-run path that locks the run first and a thread later (M2-11's next run) are retried by the unit of work (`database.tx_retry_max`). Ruling for docs/DECISIONS.md.

8. **Announcements (09 14).** A report channel is a normal channel; the server has no flag for it (13 2.2).
   a. New thread per report: `createThread` with `title` (the adapter takes the first line of the report) and `first_message`, authorized as `thread.create` on the channel (`write`) and then `message.create` on the new thread (M1-56 D8). The thread gets `title_source = auto` because an agent set the title (M1-55, 09 12 point 1), `created_by` the agent and `operator_agent_id` = the effective operator of the channel, none if the channel has none.
   b. Fixed thread: `createMessage` into a thread the adapter keeps.
   c. Both write with the agent's own rights: a grant with `write` or the operator rule (09 2.3: report channels need a grant unless the agent operates them).
   d. No run, no invocation token, no `run_id`; `rl.message` counts per agent principal (05 4.7, M1-19).
   e. A human answer in a report thread is decided by M2-05 like any other human message: with the agent as fixed operator of the thread it triggers by the switches of 09 3.3; in a channel without operator nothing triggers without an allowed guest mention.
   f. The limit of 09 14 ("the server checks only where the agent writes, not what it knows") is documented in the messaging README; nothing in the server checks content.

9. **Cards (M2-15).** This ticket handles `run_id` on `kind = text` (and on any kind the base enables). If M2-15 is merged first and already validates `run_id` for `kind = card`, keep one check: generalize the merged function to the rules of notes 3 to 7 instead of adding a second one, and list it in the report. M2-15 takes `MessagePosted.RunID` into `message_components.run_id`.

10. **SR9.** Answers in the thread of the run are an exception of SR9: only the agent's rights apply (07 SR9). This ticket does not read `acting_for` and does not intersect rights; if M2-14 is merged, its tests stay green unchanged.

11. **Rights version, events, audit.** No action here raises the rights version or writes `access.changed`; no new event type; `message.created` and `thread.created` as M1-56 and M1-55 write them. Denials are audited by the authz layer; allows are not (05 4.7 and 4.6, Audit `no`).

12. Rulings for `docs/DECISIONS.md` section 10, one row each with reason: notes 1, 4c, 4d, 5, 6 and 7. Facts already in the spec (`origin` from the principal, agents never trigger, the operator rule, title source `auto` for agents) are not repeated there.

## Tests (write them first)

Unit:

- `internal/messaging/domain`: `TestCreateRules_RunID`: absent passes; agent request carries it; a person and an app give `400 validation_failed` with `/run_id` and `run_mismatch`; `createThread` with `first_message.run_id` gives `/first_message/run_id` and `run_mismatch` for every principal kind; the `not_enabled` rows for `card`, `components`, `voice` and `attachment_file_ids` are unchanged where the base still has them.
- `internal/runs/domain`: `TestAnswerVerdict` with a row per case: not found; another thread; another agent; own run in each of `queued`, `running`, `waiting_input` (nil); own run in each of `completed`, `failed`, `cancelled` (`ErrAnswerRunNotActive`); an ended run of another agent gives `ErrAnswerRunMismatch` (ownership before state).
- `internal/runs/app`: `TestOnMessagePosted_AnswerBranch` with a fake repository: `Origin = agent`, `Kind = text`, `RunID` set calls `LockRunForAnswer` once and maps the verdicts to the problem of note 4 (`validation_failed` with the field error; `conflict` with `reason` `run_not_active`); `Kind = run` and `Kind = system` never call it; an agent message without `RunID` calls nothing and writes nothing; `Origin = human` with `RunID` and `Kind = text` returns an internal error; no branch for agent messages calls the trigger decision or `SetTriggerState`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain, every response checked by the response validator of `test/apitest`, people and agent tokens from the identity test helpers, fake clock). World after 07 10, without the grant G-5 of 07 11.1, so that Lotse writes in Support only through the operator rule unless a test grants more: space Support with default operator Lotse (`organization`), channels Allgemein (text, `free_response` and `allow_mentioned_agents` set through M2-02) and Eskalation (text, `auto_thread` off through M2-02); forum Reports in Support with operator mode `none` and a grant `write` on it for Lotse; forum Bulletin in Support that inherits Lotse; space Marketing with default operator Texter (`organization`), text channel Kampagnen and forum Ideas; Texter has a grant `write` on Support/Allgemein for the guest cases; Robin is the owner, Alex has `trigger` on Support, Sam `trigger` on Marketing. A run of Lotse is triggered by a message of Alex, delivered through `pollAgentEvents` and started with `startRun` with the delivered token, as in the tests of M2-09.

- `TestAG01_AgentMessagesNeverTrigger` (AG-01 for `origin = agent`): table over Lotse writing in Allgemein without mention, mentioning `@lotse`, mentioning `@texter`; Texter (guest) mentioning `@lotse`; Lotse writing in a thread where its own run is `running` (a human message there would be a follow-up); Texter writing in that thread (a human mention there would give `run_active_other_agent`); Lotse in the main thread with `auto_thread`. Each gives `201` with `origin = agent` and no `trigger_state`; afterwards no new `runs` row, no `agent.invocation` in `event_log`, no `message.updated`, no new thread, the active run unchanged (`state`, `version`), and no `run_followups` row if that table exists on the base. The stored row has `trigger_state` NULL.
- `TestAG26_AgentWithoutWrite` (AG-26): Lotse without any grant on Marketing: `createMessage` into a thread of Kampagnen gives `404` with the same status, body (ignoring `correlation_id` and `instance`) and headers as an unknown thread id, and `createThread` on forum Ideas gives `404` like an unknown channel id (AZ-M28); after `createGrant` of `read` on space Marketing for Lotse, both give `403 forbidden` with `reason` `level_insufficient`; nothing is stored; one aggregated deny entry per principal, action and reason (`no_grant`, `level_insufficient`). After `archiveChannel` of Allgemein, Lotse as its operator gets `403 channel_read_only` with `reason` `channel_archived`.
- `TestAgentWritesWithGrantOrAsOperator`: Lotse writes in Support/Allgemein through the operator rule (as AZ-M29) and in forum Reports through its grant (as AZ-M27); a guest run of Texter in a thread of Allgemein (Alex mentions `@texter`) lets Texter answer with `run_id`; after its grant on Allgemein is revoked, Texter's next answer gives `404`; a run of Lotse in the main thread of Eskalation (Alex mentions `@lotse`, `auto_thread` off): after `setChannelOperator` of Eskalation to Texter, Lotse's answer in that main thread gives `404`, because the operator rule follows the operator and a main thread has no fixed operator (05 2.3 point 4, R-DM-21).
- `TestReport_NewThreadPerReport`: Lotse calls `createThread` on forum Reports with a title and `first_message`: `201` `ThreadCreated`; `title_source = auto`, `operator_agent_id` absent, `created_by` Lotse; the first message has `origin = agent`, `thread_seq` 1; events `thread.created` then `message.created`; no run, no `agent.invocation`; a replay with the same `first_message.client_msg_id` gives `200` with `Idempotent-Replay: true`.
- `TestReport_FixedThread`: Lotse posts three reports into one thread of forum Bulletin: three messages, consecutive `thread_seq`, no run.
- `TestReport_HumanReplyTriggersNormally`: Alex answers in Lotse's report thread in Bulletin: one run of Lotse in that thread (fixed operator, default switches); Alex answers in the report thread in Reports: no run and no `trigger_state`; with `@lotse` there: `trigger_state = failed` with `agent_not_allowed_here` (guests not allowed in Reports).
- `TestAgentMessage_RateLimitClass`: with `ratelimit.rl.message.burst` and `ratelimit.rl.message.per_minute` set low in the test configuration, Lotse's message beyond the burst gives `429 rate_limited` with `Retry-After`; a message of Alex at the same moment succeeds (own bucket).
- `TestAgentMessage_OriginEverywhere` (SEC-OUT-3): for one answer of Lotse, `origin = agent` and `author {kind: agent, handle: lotse}` in the `201`, in the replay `200`, in the `message.created` row of `event_log`, in `listEvents` read by Alex and in the gateway dispatch to Alex's connection (M1-65 test client); also in `getMessage` and `listThreadMessages` if M1-57 is on the base; `bothy.message.created` counts it with `bothy.message.origin = agent` (M1-68 is in the closure). No JSON `null` anywhere; the event payload validates against `MessageCreatedEvent`.
- `TestAgentMessage_CannotForgeOriginOrKind`: bodies with `origin: human`, with `system_code`, with `kind: system` and with `kind: run` give `400 validation_failed`; nothing is stored.
- `TestAgentMessage_ReopensDoneThread`: `closeThread`, then an answer of Lotse: events `thread.reopened`, system `message.created` (author Lotse, `origin = system`, `system_code = thread.reopened`), then Lotse's `message.created`; no run.
- `TestAnswer_RunIDActiveRun`: Lotse answers with `run_id` while the run is `running`, after `awaitRunInput` while it is `waiting_input`, and in another thread for a run of Lotse that is still `queued`: each `201`; the `Message` has no `run_id`; a test subscriber registered through the wire test hook sees `MessagePosted.RunID`; the run's `state` and `version` are unchanged; the triggering message has no new version. Red before the change: `400 validation_failed` with `not_enabled`.
- `TestAnswer_RunIDMismatch`: Lotse posts into a thread of Allgemein with an unknown UUIDv7, with its own active run of another thread, and with the active guest run of Texter in this thread; Alex posts with that run of Texter: each `400 validation_failed` with pointer `/run_id` and `errors[].code` `run_mismatch`; the four bodies are equal ignoring `correlation_id` and `instance`; nothing stored, `thread_seq` of the thread unchanged, no `message.created`.
- `TestAnswer_RunIDEndedRun`: after `finishRun` with `completed`, `failed` (`runtime_error`) and `cancelled` (via `cancelRun` and the adapter's confirmation): `409 conflict` with `reason` `run_not_active`; nothing stored; the same `client_msg_id` without `run_id` then gives `201`.
- `TestAnswer_ReplayAfterRunEnded`: an answer with `run_id` stored while `running`; `finishRun`; the same request again gives `200` with `Idempotent-Replay: true` and the original; no second event.
- `TestAnswer_FirstMessageRunIDRejected`: `createThread` with `first_message.run_id` set to Lotse's active run gives `400 validation_failed` with pointer `/first_message/run_id` and `run_mismatch`; no thread is created.
- `TestAnswer_ConcurrentWithFinish`: many rounds of an answer with `run_id` and `finishRun(completed)` released behind a barrier: either the answer is `201` and its `message.created` has a lower `seq` than the `run.state_changed` with `completed`, or the answer is `409 conflict` `run_not_active` and nothing is stored; never an answer with `run_id` committed after the end event; no deadlock reaches the client.

Contract: every response of `createMessage` and `createThread` in these tests validates against `Message`, `ThreadCreated` and `Problem`; problems carry `code` and, where named above, `reason` or `errors[]`.

Red team focus of the milestone (PLAN 8 "Red team": confused deputy through the server API; imitating a card header) and T-AI-04 and T-AI-07 are covered by `TestAG01_AgentMessagesNeverTrigger`, `TestAG26_AgentWithoutWrite`, `TestAgentMessage_CannotForgeOriginOrKind` and `TestAnswer_RunIDMismatch`; name them in the red team request.

Red runs for M2-31 (its acceptance: every scenario was red before it was built, shown in the report of the ticket that builds it): run `TestAG01_AgentMessagesNeverTrigger` and `TestAG26_AgentWithoutWrite` first and keep the output. Where a case is already green on the base, because M1-28, M1-56 or M2-05 built the guard, prove that the test can fail: apply a one-line local mutation to the guard it covers (for example let the acceptance branch treat `origin = agent` like `human`, or give agents `write` without grant and operator rule), run the test, keep the red output, revert the mutation, and record in the report which case needed it.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/messaging/... ./internal/runs/... ./internal/authz/... ./internal/kernel/... ./internal/wire/... ./api/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`
- `make docs`
- `make check`

## Acceptance criteria

- [ ] AG-01 for `origin = agent`: no agent message creates a run, a follow-up, a `trigger_state` or an `agent.invocation`, with or without mentions, in a free channel and in a thread with an active run.
- [ ] AG-26: an agent without `write` gets `403 forbidden` (`level_insufficient`) with read right and `404` identical to an unknown id without it, for messages and report threads; the denials are audited.
- [ ] Agents write answers and announcements with a grant or as operator; a report thread has `title_source = auto`, no run, and a human answer in it triggers by the normal rules; agent messages count against `rl.message`.
- [ ] Agent messages carry `origin = agent` and an `author` of kind `agent` in every representation; `origin`, `system_code`, `kind = system` and `kind = run` cannot be sent.
- [ ] `MessageCreate.run_id` is enabled: an active own run in the thread gives `201`; any other run gives `400 validation_failed` with `run_mismatch` and one uniform body; an ended own run gives `409 conflict` with `reason` `run_not_active`; `run_id` is not stored; a replay is unaffected; `first_message.run_id` is rejected.
- [ ] Every answer stored with `run_id` commits before the end event of its run.
- [ ] `openapi.yaml`, 09, the threat model, the READMEs and `docs/DECISIONS.md` describe the behaviour; the report contains the red runs of AG-01 (agent part) and AG-26.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `messaging` and `runs` updated (the `run_id` rule, `MessagePosted.RunID`, the answer branch of `OnMessagePosted`, the lock of note 7, the limit of note 8f).
- Rulings of note 12 recorded in `docs/DECISIONS.md` (one row each with reason); the changes to 09, `openapi.yaml` and the threat model of the Scope made in the same change.
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
