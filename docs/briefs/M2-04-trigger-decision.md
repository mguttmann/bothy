# M2-04: Trigger decision as a pure function

Status: ready | Depends on: M2-02 | Milestone: M2
## Goal

After this ticket the package `internal/runs/domain` holds the rules of 09 "Operator and trigger rules" as side-effect-free functions with a documented decision table. Given the facts
about one new message (origin, kind, channel and thread, the operator, the channel switches, the
resolved agent mentions, the author's `trigger` permission, an active run in the thread and the
transcription mode), `Decide` returns exactly one outcome: no trigger (`trigger_state` stays
empty), a rejection at acceptance with a reason code of 09 3.5, an invocation of one candidate
(operator or guest, in the thread or in a new auto thread), or a follow-up to the active run.
`DecideCommand` applies 09 10.4 to agent commands, and `Settle` turns the result of the
`agent.invoke` (or `run.followup.create`) check into `trigger_state` and `trigger_state_reason`.
The functions read no database, call no `authorize`, use no clock and write nothing, so the rules
are proven by table tests: AG-01 to AG-06, the guest rule, and one case for every row of the
rejection table in 09 3.5. The tickets that apply the decision (M2-05 acceptance, M2-11
follow-ups, M2-16 commands, M3-09 voice) call these functions instead of re-implementing them.

## Read first

- `docs/spec/09-agents.md`: 1 "Principles" (point 4 fail closed); 2.2 "Features" (effective
  feature, R-AG-01, `voice_input`); 2.3 "Grants of an agent"; the whole of 3 "Operator and trigger rules": 3.1 "Operator", 3.2 "Origin (`messages.origin`)", 3.3 "Channel switches",
  3.4 "Decision per message" (steps 1 to 7 and the flowchart), 3.5 "Reasons for rejection
  at acceptance", 3.6 "Guest agents", 3.7 "Auto-thread from a channel message"; 6
  "Trigger state (`trigger_state`)"; 7 "Follow-ups during a run" (first paragraph and
  last sentence); 10.4 "Execution"; 13 "Voice messages"; 16.2 "Required scenarios" rows AG-01
  to AG-06; 19 "Decisions of this chapter" (R-AG-02, R-AG-03, R-AG-04, R-AG-05, R-AG-16,
  R-AG-22).
- `docs/spec/05-actions.md`: 2.3 "Effective level" (operator rule); 2.4 (column "Audit", value
  `denials`); 4.10 rows `agent.invoke`, `run.followup.create`, `command.invoke`; 5 "Agent invocation
  `agent.invoke`" points 1 to 8 and the paragraphs "Consequences on `allow`" and "Consequences on
  denial"; 8 "Decisions of this chapter" (R-AZ-13, R-AZ-19, R-AZ-20).
- `docs/spec/03-data-model.md`: `channels` (switch columns, "Effective operator" and
  "Mention rule" after the table), `threads` (`kind`, `operator_agent_id`, `title`,
  `title_source`, `parent_message_id`), `messages` (`origin`, `kind`, `trigger_state`,
  `trigger_state_reason` and the checks `trigger_state IS NULL OR origin = 'human'`,
  `trigger_state_reason IS NULL OR trigger_state IN ('failed','cancelled')`), `runs` (`state`,
  the partial unique index for at most one active run per thread), `agents` (`billing_mode`),
  `principals` (`status`); R-DM-11, R-DM-12, R-DM-14, R-DM-21.
- `docs/spec/07-permissions.md`: 2.3 "Levels" (`trigger` includes `write`), 2.9 "Agents as principals" (point 2 operator rule, point 3 agents never trigger), 4.1 "Decisions and reasons" (deny reasons
  `origin_not_human`, `agent_not_reachable`, `billing_mode_personal_not_holder`,
  `tool_class_owner_only`, `dev_environment_not_ready`, `project_not_ready`,
  `run_active_other_agent`, `run_active`, `level_insufficient`, `no_grant`), 4.6 "Resolving mentions", "SR4 Only humans trigger", 11.1 "Test matrix" rows AZ-M30 and AZ-M53 (owned by
  M2-08 and M2-27; this ticket mirrors them at decision level only).
- `docs/spec/02-architecture.md`: 4.1 "Rules" point 4 (`messaging` publishes `MessagePosted`,
  `runs` decides), 4.2 "Module list" (`runs` owns the trigger decision), 4.3 row `runs`, 4.4 row
  `runs`, 5 "Layers within a module" (domain imports only `kernel-core`), 6 "Code layout".
- `docs/spec/04-sync-protocol.md`: 15.3 "Invocation and run start" point 1 "Acceptance" and point 2
  "Delivery"; 15.4 "Follow-ups" point 1.
- `docs/spec/12-files.md`: 8.2 "Transcript" (values of `voice.transcription_mode`).
- `docs/spec/01-product.md`: 6.3 "Trigger state as icons".
- `docs/spec/00-glossary.md`: "Operator", "Operator rule", "Main thread", "Follow-up",
  "Trigger state", "Control command".
- `docs/spec/23-quality.md`: "Documentation in code" (Go: revive rules `exported` and
  `package-comments`).
- `docs/api/openapi.yaml`: schemas `Message` (`trigger_state`, `trigger_state_reason`),
  `TriggerState`, `MessageOrigin`, `MessageKind`, `ChannelBehavior`; operation `createMessage`
  (description: invocation evaluated after storing); problem code `agent_invocation_denied`.
- ADRs 0008 (hexagonal modules and checked dependencies), 0017 (one authorization function), 0022
  (agents outside the server process), 0024 (backend policy with invariant).
- `docs/DECISIONS.md`: D-B7 (only `authz` holds permission logic), D-AG-02, D-AG-03, D-AG-04,
  D-AG-12, D-AG-15, D-W-100; section 10 "Rulings of the build milestones" (format of the rows you add).
- `docs/security/threat-model.md`: T-AI-07 (loops: only `origin = human` triggers, one active run
  per thread) and T-AI-08 (personal subscription used by a second person).
- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 5 "Test strategy" (row "Unit");
  section 8 "M2 Agent platform, extensions and terminal": "Goal", "Scope", "Exit criteria"
  point 1, "Test strategy", "Red team" (personal subscription triggered by another person over
  every path), rows M2-02, M2-03, M2-04, M2-05, M2-08, M2-11, M2-12, M2-16, M2-19, M2-31; section 9
  row M3-09.
- M1 briefs whose interfaces this decision mirrors (the domain imports none of them):
  `docs/briefs/M1-56-messaging-send.md` (in-transaction event `MessagePosted` with `Origin`,
  `ThreadKind`, `Kind`, `MentionedIDs`; mentions resolve only to active principals the author may
  see, D5), `docs/briefs/M1-55-chat-threads.md` (`chat.Threads.Info` returns `ThreadInfo` with
  `ChannelKind`, `Kind`, `OperatorAgentID`), `docs/briefs/M1-54-chat-operators.md` (pure function
  for the effective operator of a channel), `docs/briefs/M1-51-agents-records.md` (agent status
  and `billing_mode`), `docs/briefs/M1-28-authz-core.md` (the `agent.invoke` guard in
  `domain/guards.go`, `InvokeFacts{AgentID, TriggerOrigin, MentionsAgent, ActiveRunAgentID}`,
  `Decision{Outcome, Reason}`, filter mode, deferred entry `run.followup.create`),
  `docs/briefs/M1-05-server-skeleton.md` (layers, `kernel/ids`, `kernel/errs`).
- Dependencies: the header names M1-56 and M2-02, both minimal. Through them the closure contains
  M1-28, M1-51, M1-54 and M1-55, which this brief reads but does not repeat in the header. M2-02
  owns the API of the channel switches (read its brief in `docs/briefs/`, its plan row and the
  schema `ChannelBehavior`); the columns themselves exist since M1-53.
- Code on the base branch (merged names win, D-W-100): `services/server/internal/runs/doc.go`
  and `README.md` (port package only, no `domain/` yet), `services/server/internal/kernel/ids`
  (`ids.ID`), `services/server/internal/kernel/errs` (`errs.New`, `errs.CodeInternalError`),
  `services/server/architecture/modules.yaml` (module `runs`), `services/server/.go-arch-lint.yml`
  (component `runs-domain` may depend only on itself and `kernel-core`).

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
10. Specification first: a change to behaviour, API, event, permission or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. Only a message with `origin = 'human'` triggers; agents, apps, imports and system lines never
    do, and DM channels have no agents (09 3.2, 3.4 steps 1 and 2; 07 SR4; 05 section 5 points 1
    and 2).
12. The operator of a thread (`kind = 'thread'`) is its fixed `threads.operator_agent_id`; the
    operator of the main thread is the channel's effective operator at trigger time; without an
    operator no agent answers unless an allowed guest is mentioned (09 3.1, R-DM-12, R-DM-21,
    R-AG-02).
13. Mentioned agents E: more than one gives `multiple_agents_mentioned`; exactly one is the
    candidate if it is the operator or the guest rule holds, else `agent_not_allowed_here`; none
    makes the operator the candidate only if `free_response`, or main thread and not
    `require_mention`, or thread and not `thread_require_mention`; otherwise nothing happens (09
    3.4 step 3, 03 `channels` "Mention rule", R-AG-03).
14. Guest rule: a mentioned agent that is not the operator is a guest only if
    `allow_mentioned_agents` is set, it has at least `write` on the channel itself and its
    `billing_mode` is `organization`; a guest run has no agent session (09 3.6, 05 section 5 point
    3, R-AZ-19, R-AG-04, R-AG-22).
15. Without effective `trigger` on the channel the author gets `no_trigger_right` only for an
    explicit mention; otherwise nothing happens (09 3.4 step 4, R-AG-05).
16. Target: in the main thread with `auto_thread` the target is a new thread with
    `parent_message_id = M`, `operator_agent_id` = effective operator of the channel, `title = ''`,
    `title_source = 'auto'`; `trigger_state` stays on M (09 3.4 step 5, 3.7). Otherwise the target
    is the thread of M.
17. Active run in the target: of the candidate, M becomes a follow-up; of another agent,
    `run_active_other_agent` (09 3.4 step 6, 09 7, 05 section 5 point 8, R-AZ-20, R-DM-14).
18. The `agent.invoke` check runs after the decision; `allow` gives `trigger_state = received`, a
    denial gives `trigger_state = failed` with the reason; a rejection at acceptance creates no
    run (09 3.4 step 7, 09 6, 05 section 5 "Consequences on denial").
19. A voice message without a transcription path is rejected at acceptance with
    `voice_unsupported` (09 13, 12 8.2, R-AG-16).
20. A model command during an active run is rejected with `run_active`; a control command goes to
    the agent at once and is never a follow-up (09 10.4, 09 7 last sentence, 05 section 5 point 8,
    D-AG-12).
21. Every row of the rejection table 09 3.5 has a test case, and AG-01 to AG-06 and the guest rule
    are table tests (PLAN row M2-04).
22. The decision is a pure function in `internal/runs/domain`: no I/O, no `authorize`, no clock
    (PLAN row M2-04; 02 5 "Layers within a module", row "Domain").

## Scope

All paths under `services/server/` unless noted. This ticket creates no migration, enables no
operation, appends no event, schedules no job, records no metric and adds no configuration key.

Create:

- `internal/runs/domain/doc.go`: package comment of the domain layer of `runs` (what lives here,
  that it imports only `kernel-core`).
- `internal/runs/domain/trigger.go`: value types and `Decide` (design notes 2 to 6).
- `internal/runs/domain/trigger_command.go`: `DecideCommand` (design note 7).
- `internal/runs/domain/trigger_settle.go`: `Verdict`, `Settlement` and `Settle` (design note 8).
- `internal/runs/domain/trigger_reasons.go`: the type `Reason` with one constant per code of 09
  3.5, and the constants of `Outcome`, `Step`, `Check`, `TranscriptSource`.
- Tests (design of each in "Tests"): `internal/runs/domain/trigger_test.go`,
  `trigger_matrix_test.go`, `trigger_command_test.go`, `trigger_settle_test.go`,
  `trigger_spec_test.go`, `trigger_fuzz_test.go`.

Change:

- `internal/runs/README.md` (English, like the file): status line, section "Code" (`domain/` now
  exists), a new section "Trigger decision" with the decision table of design note 3, the
  input table of design note 2 (which module or port supplies each fact, for M2-05), the mapping
  table of design note 8 and the rulings of design note 12.
- `services/server/CHANGELOG.md`: entry under "Unreleased", "Added".
- `docs/spec/09-agents.md` (not core contract): in 3.4 add the position of the voice rule (design
  note 6) and, after step 7, the mapping of `authorize` deny reasons to `trigger_state_reason`
  (design note 8); in 3.5 a sentence that the codes of the table are the values of
  `messages.trigger_state_reason` and how other deny reasons appear. English, like the chapter.
- `docs/DECISIONS.md`, section 10: rows `D-M2-04-1` to `D-M2-04-5` (design note 12).

No new top-level Makefile target.

## Out of scope

- Applying the decision on `messaging.MessagePosted`: gathering the facts from `chat`, `agents`,
  `authz` and `runs`, creating the auto thread through the `chat` port, writing `trigger_state`,
  `trigger_state_reason` and `trigger_state_at` through `messaging` (`SetTriggerState`),
  `message.updated`, the run `queued` and the live box anchor (04 15.3 point 1): M2-05, the first
  ticket with `runs` tables and the mapping to `trigger_state`.
- The `agent.invoke` guard itself and its audit: M1-28 (built); the requirement of
  `run.followup.create`, which M1-28 keeps deferred, and the `run_followups` rows with `steer` and
  `queue`: M2-11.
- Delivery, invocation token, the re-check before each delivery (I2), `trigger_state = queued`
  and the metric `bothy.agent.invocation.denied`: M2-06 and M2-08.
- Agent sessions, including the end of the main-thread session with `agent_changed` after an
  operator change: M2-12.
- Command registration, visibility, `required_level`, the `command.invoke` requirement in `authz`
  and the HTTP answer for `run_active`: M2-16; app commands without `agent.invoke`: M2-25.
- Voice messages (`MessageCreate.kind = voice`), transcripts and the hold of an invocation until
  the transcript or a scan is final (`ReleaseInvocation`): M3-09 and M3-07.
- Agent replies (`origin = agent`) end to end and AG-26: M2-19; incoming webhooks and AZ-M53:
  M2-27; AZ-M30 against the real server: M2-08.
- Channel behaviour storage and its API (`getChannelBehavior`, `putChannelBehavior`): M2-02.
  Project binding readiness (`project_not_ready` passes through `Settle`): M2-03.
- Interactions (05 section 5 point 6): M2-15. Auto title: M2-18.
- Running AG-01 to AG-06 against the real server: M2-31.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-04-trigger-decision`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(runs): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- This ticket has unit tests only: no listener, no container, no PostgreSQL, no network. The one file a test reads is `docs/spec/09-agents.md` in the working tree.
- Production code in `internal/runs/domain` imports only standard library packages without I/O and `kernel-core` (`kernel/ids`, `kernel/errs`); `make arch` enforces the layer. No `context.Context`, no logger, no clock, no randomness in production code of this package.
- No real model call and no third-party service; nothing in this ticket talks to an agent or adapter.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. Purity and place. The decision lives in `internal/runs/domain` (02 4.2: `runs` owns
   "trigger decision"; 02 5: the domain layer holds pure functions and imports only
   `kernel-core`). Its input types are value types of this package that mirror the data other
   modules hand over through their ports (02 5, row "Domain": no foreign module). The functions are
   deterministic, never mutate their input (copy slices before sorting or deduplicating) and
   never call `authorize`: every permission fact is an input that the caller gathers from `authz`
   (D-B7, ADR 0017). The operator rule and the guest rule therefore appear twice by design: here
   they choose the candidate and the reason the person sees (09 3.4), and the `agent.invoke` guard
   of M1-28 re-checks them under the permission snapshot (05 section 5 point 3). When the two disagree
   (a concurrent change), `authz` wins through `Settle` (note 8).

2. Inputs. Signatures (names may follow the merged code, D-W-100; the fields are binding):

   ```go
   type Origin string      // "human", "agent", "app", "import", "system" (03 messages.origin)
   type ChannelKind string // "text", "forum", "dm" (03 channels.kind)
   type ThreadKind string  // "main", "thread" (03 threads.kind)
   type BillingMode string // "personal", "organization" (03 agents.billing_mode, the target)
   type VoiceMode string   // "off", "agent", "service" (voice.transcription_mode, 12 8.2)

   // AgentFacts are the facts about one agent that the decision needs.
   type AgentFacts struct {
       ID             ids.ID
       Active         bool        // principals.status = 'active'
       BillingMode    BillingMode // agents.billing_mode
       WriteOnChannel bool        // own effective level on the channel is at least write
       VoiceInput     bool        // effective feature voice_input (09 2.2, R-AG-01)
   }

   type Switches struct { // 03 channels; history_backfill_limit has no influence here
       RequireMention, ThreadRequireMention, FreeResponse, AutoThread, AllowMentionedAgents bool
   }

   type ActiveRun struct{ RunID, AgentID ids.ID } // a run in queued, running or waiting_input

   type MessageTrigger struct {
       MessageID, ThreadID, ChannelID ids.ID
       Origin           Origin
       Voice            bool          // messages.kind = 'voice'
       ChannelKind      ChannelKind
       ThreadKind       ThreadKind
       ThreadOperator   *AgentFacts   // threads.operator_agent_id; nil when NULL; main: always nil
       ChannelOperator  *AgentFacts   // effective operator of the channel now; nil when none
       Switches         Switches
       Mentioned        []AgentFacts  // resolved agent mentions of M; people are not in it
       AuthorHasTrigger bool          // the author has effective trigger on the channel
       ActiveRun        *ActiveRun    // active run of the thread of M; nil when none
       VoiceMode        VoiceMode
   }
   ```

   Where each fact comes from (write this table into the README for M2-05): `Origin`, `MessageID`,
   `ThreadID`, `ChannelID`, `ThreadKind`, `Voice` and the mention IDs from `messaging.MessagePosted`
   (M1-56); M1-56 resolves a mention only to an active principal the author may see (07 4.6), so a
   disabled agent is never in `Mentioned`. `ChannelKind` and `ThreadOperator` from
   `chat.Threads.Info` (M1-55). `ChannelOperator` from the effective-operator function of `chat`
   (M1-54; 03 `channels` "Effective operator"). `Switches` from the channel behaviour (M2-02;
   schema `ChannelBehavior`). `Active` and `BillingMode` from `agents` (M1-51). `AuthorHasTrigger`
   and `WriteOnChannel` from `authz` (07 2.3, 07 2.9; `WriteOnChannel` is the effective level of
   05 section 5 point 3, "at least `write`", which includes the operator rule; 09 2.3 says
   "grant with `write`" for the usual guest, and the core contract wins). `VoiceInput` from
   `agents` (owner permission `supports_voice` and adapter report
   `voice_input`). `ActiveRun` from `runs` (M2-05). `VoiceMode` from the configuration key
   `voice.transcription_mode`.

3. `Decide(in MessageTrigger) (Decision, error)`: the decision table, evaluated in this order;
   the first row that decides wins, and `Decision.Step` names it:

   | Step | Condition | Outcome | Reason |
   |---|---|---|---|
   | validate | input invalid (note 10) | error, zero `Decision` | |
   | `origin` | `Origin` is not `human` | `none` | |
   | `direct_message` | `ChannelKind` is `dm` | `none` | |
   | `mentions` | E (distinct IDs of `Mentioned`) has more than one element | `reject` | `multiple_agents_mentioned` |
   | `mentions` | E = {A}, A is the operator of the thread (09 3.1) | candidate A, mentioned | |
   | `mentions` | E = {A}, A is not the operator and the guest rule holds (note 5) | candidate A, mentioned, guest | |
   | `mentions` | E = {A}, otherwise | `reject` | `agent_not_allowed_here` |
   | `mentions` | E empty, an operator exists and (`FreeResponse` or (main and not `RequireMention`) or (thread and not `ThreadRequireMention`)) | candidate operator, not mentioned | |
   | `mentions` | E empty, otherwise | `none` | |
   | `trigger_right` | not `AuthorHasTrigger` and the candidate was mentioned | `reject` | `no_trigger_right` |
   | `trigger_right` | not `AuthorHasTrigger` and the candidate was not mentioned | `none` | |
   | `voice` | `Voice` and no transcription path (note 6) | `reject` | `voice_unsupported` |
   | target | main thread and `AutoThread`: target is a new thread (note 4); otherwise the thread of M | | |
   | `active_run` | target is the thread of M, `ActiveRun` set, `ActiveRun.AgentID` is the candidate | `followup`, `RunID` set | |
   | `active_run` | target is the thread of M, `ActiveRun` set, another agent | `reject` | `run_active_other_agent` |
   | `invoke_check` | otherwise | `invoke` | |

   `Step` takes the values `origin`, `direct_message`, `mentions`, `trigger_right`, `voice`,
   `active_run`, `invoke_check` and, in `DecideCommand`, `command`; the rows "validate" and
   "target" decide no outcome of their own.
   The operator of the thread is `ThreadOperator` for `kind = 'thread'` and `ChannelOperator` for
   the main thread (09 3.1); a later operator change of the channel never changes the operator of
   an existing thread (R-DM-12). A new thread has no active run, so step `active_run` does not
   apply to it. A message without trigger stays a normal message even while a run is active (09
   7, first paragraph).

4. `Decision` (output; every field documented in code):

   ```go
   type Decision struct {
       Outcome          Outcome          // none, reject, invoke, followup, control
       Step             Step             // the row of the table that decided
       Reason           Reason           // only with OutcomeReject
       TriggerKind      string           // "message" or "command" (03 runs.trigger_kind)
       TriggerMessageID ids.ID           // M; carries trigger_state, also with a new thread (3.7)
       CommandID        ids.ID           // DecideCommand only
       Agent            *AgentFacts      // the candidate: invoke, followup, control
       Guest            bool             // guest run without agent session (3.6, R-AG-04)
       Mentioned        bool             // the candidate was mentioned explicitly
       NewThread        *NewThread       // target is a new thread; nil: the thread of M
       RunID            ids.ID           // followup: the active run
       Check            Check            // "" or CheckInvoke or CheckFollowUp
       Transcript       TranscriptSource // voice only: "agent" or "service"
   }

   type NewThread struct { // 09 3.7
       ChannelID       ids.ID
       ParentMessageID ids.ID // = M
       OperatorAgentID ids.ID // effective operator of the channel; zero when none
       Title           string // always ""
       TitleSource     string // always "auto"
   }
   ```

   `Check` tells the caller which authorization runs next: `CheckInvoke` (`agent.invoke`) for
   `invoke` and `control`, `CheckFollowUp` (`run.followup.create`, "write and `agent.invoke`") for
   `followup`, empty for `none` and `reject`. The domain keeps no action names of the catalogue;
   the app layer maps `Check` to the action constants. Every outcome that can lead to a run or a
   delivery carries a check, so no path of this package invokes an agent without `agent.invoke`
   (T-AI-08; M2 "Red team": personal subscription triggered by another person over every path).

5. Guest rule (09 3.6, 05 section 5 point 3): a mentioned agent A that is not the operator of the
   thread is a guest candidate if and only if `AllowMentionedAgents`, `A.WriteOnChannel` and
   `A.BillingMode == organization` (R-AZ-19, R-AG-22: a personal agent is never a guest, because
   I1 does not count guest channels among its targets, 03 6.4 "I1 check"). An operator who is
   mentioned is never a guest, even when the guest conditions also hold. The channel's current
   operator mentioned in a thread whose fixed operator is another agent is not the operator of
   that thread, so the guest rule decides for it (R-DM-12). After a guest run ends, a message
   without mention triggers the operator again (09 3.6 last sentence), which the table yields
   because a guest is chosen only by mention. `A.Active` is not part of the guest rule (09 3.4
   checks activity in step 7); note 8 turns a denial for an inactive candidate into
   `agent_disabled`.

6. Voice rule (09 13; Ruling for docs/DECISIONS.md, `D-M2-04-2`). 13 says "Rejection at
   acceptance" but 3.4 does not place it. It runs after step `trigger_right` and before the target is
   chosen, because the transcription path depends on the candidate (`voice_input` of the agent in
   mode `agent`) and because a message that cannot be understood must create neither an auto
   thread nor a follow-up. Transcription path: `VoiceMode == service`, or `VoiceMode == agent` and
   the candidate's `VoiceInput`; `off` has none. With a path, `Transcript` is `service` or `agent`
   (M3-09 and M3-07 use it for the hold of 04 15.3 point 1). A voice message that would not
   trigger at all stays `none`.

7. `DecideCommand(in CommandTrigger) (Decision, error)` for commands owned by an agent (09 10.4):

   ```go
   type CommandTrigger struct {
       CommandID, ThreadID ids.ID
       Owner     AgentFacts // the agent that registered the command
       Control   bool       // commands.control
       ActiveRun *ActiveRun // active run of the thread; nil when none
   }
   ```

   `Control` true: outcome `control` with `CheckInvoke`, also during a run, never a follow-up (09
   7 last sentence, 05 section 5 point 8). `Control` false and `ActiveRun` set (any agent):
   `reject` with `run_active`. Otherwise `invoke` with `TriggerKind = "command"` and `CommandID`
   set (03 `runs.triggering_command_id`). App commands are not decided here (05 section 5, last
   paragraph before "Consequences on `allow`"; M2-25).

8. `Settle(d Decision, v Verdict) (Settlement, error)` (09 3.4 step 7, 09 6; Ruling for
   docs/DECISIONS.md, `D-M2-04-3`):

   ```go
   type Verdict struct { Allowed bool; Reason string } // outcome of the check of note 4
   type Settlement struct {
       Accepted bool
       State    TriggerState // "received", "failed", or "" (stays NULL; also for commands)
       Reason   Reason
   }
   ```

   For `none` and `reject` the verdict is ignored: `none` gives `State ""`, `reject` gives
   `failed` with its reason. For `invoke`, `followup` and `control`: `Allowed` gives `Accepted`
   and, for message decisions, `received`. A denial gives `failed` (message decisions) and the
   reason by this table, because the closed deny list of 07 4.1 (`kernel/auditing.Reason`, M1-24)
   has no `agent_disabled` and the M1-28 guard answers `agent_not_reachable` for an inactive
   agent, while 09 3.5 shows the person "agent disabled":

   | Deny reason from `authorize` (07 4.1) | `trigger_state_reason` |
   |---|---|
   | `agent_not_reachable`, candidate `Active` false | `agent_disabled` |
   | `agent_not_reachable`, candidate `Active` true | `agent_not_allowed_here` |
   | `level_insufficient`, `no_grant` | `no_trigger_right` |
   | every other reason (`billing_mode_personal_not_holder`, `project_not_ready`, `run_active_other_agent`, `tool_class_owner_only`, `dev_environment_not_ready`, `channel_read_only`, ...) | unchanged |

   A denial with an empty reason is an internal error (`errs.CodeInternalError`). The audit of the
   denial is written by `authz` (05 2.4, `denials`); `Settle` only names the reason the person sees.

9. Descriptor of the acceptance check (Ruling for docs/DECISIONS.md, `D-M2-04-4`). The caller
   builds the `agent.invoke` resource on the thread of M (also when the target is a new thread)
   with `InvokeFacts.AgentID` = the candidate, `TriggerOrigin = human`, `MentionsAgent =
   Decision.Mentioned` and `ActiveRunAgentID` zero for a new thread. Reason: 05 section 5 point 3
   names "the effective operator of the channel in the main thread and for new threads", and the mention
   rule that chose the candidate is the one of the main thread. What the re-check before each
   delivery uses (04 15.3 point 2) belongs to M2-08 and is not decided here; do not change the
   M1-28 guard in this ticket.

10. Invalid input (Ruling for docs/DECISIONS.md, `D-M2-04-5`). `Decide` and `DecideCommand`
    validate first and return an error that wraps `errs.CodeInternalError` with the zero
    `Decision` (outcome `none`, no check) for: an unknown value of `Origin`, `ChannelKind`,
    `ThreadKind`, `BillingMode` or `VoiceMode`; `ThreadOperator` set on a main thread (R-DM-21); a
    main thread in a forum channel (R-DM-11); an operator on a DM channel (03 `channels` checks);
    a zero ID in `MessageID`, `ThreadID`, `ChannelID`, a mentioned agent, an operator or
    `ActiveRun`. Fail closed (09 1 point 4): a broken input never yields `invoke`, `followup` or
    `control`. The error text names the field, never a value from a client.

11. `trigger_state` semantics. `none` keeps `trigger_state` NULL, so the check
    `trigger_state IS NULL OR origin = 'human'` of 03 `messages` holds for every non-human
    message; `reject` and a denied check give `failed` with a reason, and only then is a reason
    set (`trigger_state_reason IS NULL OR trigger_state IN ('failed','cancelled')`); an allowed
    check gives `received` (09 6, 04 15.3 point 1). This package never yields `queued`,
    `working`, `done`, `cancelled` or `followup_delivered` (M2-08, M2-09, M2-11).

12. Rulings for `docs/DECISIONS.md` section 10, one row each with reason and place:
    `D-M2-04-1` the decision is split into `Decide`/`DecideCommand` (steps 1 to 6 of 09 3.4,
    13 and 10.4) and `Settle` (step 7), and the domain never calls `authorize` (note 1);
    `D-M2-04-2` position of the voice rule (note 6); `D-M2-04-3` mapping of deny reasons (note 8);
    `D-M2-04-4` descriptor of the acceptance check (note 9); `D-M2-04-5` invalid input (note 10).
    Rulings 2 and 3 also go into 09 3.4 and 3.5 (scope, "Change").

## Tests (write them first)

Unit tests in `internal/runs/domain` (PLAN 5, row "Unit"). The labels below come from the world of
07 10 "Examples": space Support with operator Lotse (`organization`), Texter (`organization`, with
`write` on Support/Allgemein in the guest cases), Assistent (`personal`, with `write` on
Support/Allgemein in the guest cases); Alex has `trigger` on Support, Sam only `write`. In a pure
test they are facts, not database rows. Test IDs from 09 16.2 stand in the test names.

- `TestDecide_AG01_NonHumanOriginNeverTriggers` (AG-01: "Message with `origin` `agent`, `app`,
  `import`, `system` in a free-response channel" gives no run): for each of the four origins in a channel with
  `free_response`, operator Lotse present, author with `trigger`, once without mention, once with
  Lotse mentioned and once with two agents mentioned: outcome `none`, step `origin`, no reason, no
  check. Mirrors AZ-M30 (Lotse mentions Texter with `origin = agent`) and AZ-M53 (app origin,
  `free_response`) at decision level.
- `TestDecide_DirectMessageNeverTriggers`: DM channel, human origin, agent mentioned: `none`, step
  `direct_message`.
- `TestDecide_AG02_SwitchMatrix` (AG-02: switches of 3.3 x main/thread x with/without mention give
  exactly the candidates of 3.4), in `trigger_matrix_test.go`: every combination of
  `RequireMention`, `ThreadRequireMention`, `FreeResponse`, `AutoThread`, `AllowMentionedAgents`
  x main thread and thread x the mention variants none, the operator, an eligible guest, a guest
  without `write`, a `personal` guest with `write`, and two agents; author with `trigger`, no active
  run. The expected result comes from an oracle written in the test from the 03 `channels`
  "Mention rule" formula and the guest rule of 05 section 5 point 3 as plain boolean
  expressions (no call into production helpers): candidate, `Guest`, `Mentioned`, reason, and the
  target (new thread exactly for main thread with `AutoThread`).
- `TestDecide_AG03_MultipleAgentsMentioned` (AG-03: `failed`, `multiple_agents_mentioned`): two
  agents including the operator; two guests; an author without `trigger` still gets
  `multiple_agents_mentioned` (step 3 before step 4); the same agent twice in `Mentioned` counts
  once and is not a rejection.
- `TestDecide_AG04_GuestNotAllowed` (AG-04: guest with `allow_mentioned_agents = false` or without
  `write` gives `agent_not_allowed_here`): both cases, plus Assistent (`personal`) with `write` and
  guests allowed (R-AG-22), plus the channel's current operator mentioned in a thread whose fixed
  operator is another agent with guests not allowed (R-DM-12); an author without `trigger`
  mentioning a not allowed agent gets `agent_not_allowed_here` (step 3 before step 4).
- `TestDecide_AG05_AutoThread` (AG-05: new thread, run there, `trigger_state` on M): main thread
  with `AutoThread`: `invoke`, `NewThread{ChannelID, ParentMessageID: M, OperatorAgentID: channel
  operator, Title: "", TitleSource: "auto"}`, `TriggerMessageID == M`; without operator and a
  guest mentioned: `NewThread.OperatorAgentID` zero, `Guest` true; main thread without
  `AutoThread`: target is the thread of M; a thread (`kind = thread`) with `AutoThread`: target is
  the thread of M; main thread with `AutoThread` and an active run of another agent in the main
  thread: still `invoke` into the new thread.
- `TestDecide_AG06_ActiveRunOtherAgent` (AG-06: guest mentioned during the operator's run gives
  `run_active_other_agent`): that case; the operator as candidate during a guest run gives the
  same; the operator during its own run gives `followup` with `RunID` and `CheckFollowUp`; a
  guest mentioned during its own guest run gives `followup`.
- `TestDecide_GuestRule` (the guest rule as table test, PLAN row): every combination of
  `AllowMentionedAgents` x `WriteOnChannel` x `BillingMode` x "is the operator": a guest exactly
  for allowed, `write`, `organization`, not operator; `Guest` false for the operator even when the
  guest conditions hold; after a guest run (no active run) a message without mention in a thread
  without `ThreadRequireMention` chooses the operator.
- `TestDecide_TriggerRight` (R-AG-05): mention without `trigger` gives `no_trigger_right`; no
  mention with `FreeResponse` and without `trigger` gives `none`; without `trigger` and with an
  active run of the operator gives `none`, not a follow-up (09 7).
- `TestDecide_OperatorRules` (09 3.1): a thread uses its fixed operator and ignores a different
  `ChannelOperator`; the main thread uses `ChannelOperator`; a thread without operator and without
  mention in a `FreeResponse` channel gives `none`.
- `TestDecide_Voice` (09 13): `off` gives `voice_unsupported`; `agent` with candidate
  `VoiceInput` gives `invoke` with `Transcript = agent`; `agent` without it gives
  `voice_unsupported`; `service` gives `Transcript = service`; a voice message without trigger
  stays `none`; a voice message that would be a follow-up and has no path gives
  `voice_unsupported`.
- `TestDecide_InvalidInput` (note 10): each listed case returns an error whose public code is
  `internal_error` and a decision with outcome `none` and no check.
- `TestDecideCommand` (09 10.4): control command during a run of the same and of another agent
  gives `control` with `CheckInvoke`; model command during any active run gives `run_active`;
  model command without run gives `invoke`, `TriggerKind = "command"`, `CommandID` set.
- `TestSettle` (note 8): allow gives `received` and `Accepted`; each row of the mapping table;
  passthrough of `billing_mode_personal_not_holder`, `project_not_ready`,
  `run_active_other_agent`, `tool_class_owner_only`; `none` and `reject` ignore the verdict;
  command decisions give `State ""`; a denial with empty reason gives an error.
- `TestRejectionTableCoversSpec` (PLAN row: every row of the rejection table in 09 has a test
  case), in `trigger_spec_test.go`: walks up from the package directory to the repository root,
  reads `docs/spec/09-agents.md`, takes the first cell of every row under the heading "3.5 Reasons for rejection at acceptance" and asserts that (a) the set equals the set of `Reason`
  constants of this package and (b) each code has a named case in a map in the test that produces
  it through `Decide`, `DecideCommand` or `Settle`: `multiple_agents_mentioned`,
  `agent_not_allowed_here`, `no_trigger_right`, `run_active_other_agent` (from `Decide`),
  `agent_disabled`, `billing_mode_personal_not_holder`, `project_not_ready` (from `Settle`),
  `voice_unsupported` (from `Decide`), `run_active` (from `DecideCommand`). A row added to 3.5
  later fails this test until it has a case.
- `FuzzDecide` (seed corpus from the table cases; runs in `go test`): builds a `MessageTrigger`
  from the fuzz input, skips inputs for which `Decide` returns an error, and asserts: non-human
  origin or DM gives `none`; a reason is set exactly with `reject` and is one of the 3.5 codes;
  `invoke` and `followup` always carry a check; an `invoke` or `followup` candidate is the
  operator of the thread or the only mentioned agent; a guest is `organization`, has
  `WriteOnChannel` and the channel allows guests; `NewThread` only for main thread with
  `AutoThread`; `followup` only when the active run belongs to the candidate; `Settle` with a
  denial never yields `received`; two calls with the same input give equal results and the input
  is unchanged.

No integration, contract or end-to-end test: this ticket has no I/O. M2-05 adds the PostgreSQL
tests when it applies the decision, and M2-31 runs AG-01 to AG-06 against the real server.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/runs/...`
- `make arch`, `make lint`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `internal/runs/domain` exists with `Decide`, `DecideCommand` and `Settle`, a doc comment on
      every public symbol, and imports only the standard library and `kernel-core`; `make arch`
      and `make lint` pass.
- [ ] AG-01 to AG-06 pass as table tests (`TestDecide_AG01_*` to `TestDecide_AG06_*`), the AG-02
      matrix covers every switch combination of 3.3 for main thread and thread, with and without
      mention.
- [ ] The guest rule passes as a table test, including "a `personal` agent is never a guest" and
      "a mentioned operator is never a guest".
- [ ] Every row of the rejection table in 09 3.5 has a test case; `TestRejectionTableCoversSpec`
      reads the table from `docs/spec/09-agents.md`.
- [ ] `Settle` maps deny reasons as in design note 8; invalid input never yields an invocation.
- [ ] `FuzzDecide` seed corpus passes in `go test`.
- [ ] The runs README holds the decision table, the input sources and the mapping; 09 3.4 and 3.5
      name the voice position and the mapping; `D-M2-04-1` to `D-M2-04-5` are in
      `docs/DECISIONS.md`; the CHANGELOG has the entry.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `runs` updated (status, code layout, the decision table of note 3, the input sources of note 2, the mapping of note 8).
- Rulings of note 12 recorded in `docs/DECISIONS.md` (one row each with reason); rulings 2 and 3 also in `docs/spec/09-agents.md`.
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
