# Module `runs`

Status: the trigger decision as pure functions in `domain/` (M2-04); runs, the state machine, the acceptance of a triggering message and the run reads (M2-05). The invocation tokens and the reference verifier of the adapter check (M2-06). The delivery check `runs.Delivery` with the fresh token and the job `runs.queued_expiry` (M2-08). Start, renewal, end and cancellation of runs, the revocation and disabled-agent subscribers and the jobs `runs.cancel_timeout` and `runs.stall_check` (M2-09). The booking of costs in the finish, the cost columns and the metric `bothy.run.cost` (M2-22). Components and interactions: the tables `message_components` and `interactions`, the port `Interactions`, the source ports of `messaging` in `adapters/messagingsource`, the job `runs.component_expiry` and the resolvers of `interaction.created` and of `agent.invocation` with `trigger_kind = interaction` (M2-15). Patches (M2-10) and follow-ups (M2-11) build on `app.Open`, `app.Transition`, the end-of-run function `endRun` and `app.Service.Prepare`. The hold of an agent invocation while an attachment of the triggering message is not final, `ReleaseInvocation`, the subscriber of `files.FileFinalized`, the job `runs.invocation_hold_timeout` and the owner's switch `supports_files_out` (M3-07). Commands and interactions of apps: the source port `AppDispatcher`, the app branches of `CommandExec`, `Interactions` and the autocompletion, `Interactions.DisableAuthoredBy` and the job `runs.app_dispatch` (M2-25). Voice messages at acceptance, the hold window of a pending transcript and the attachments of the run context (M3-09).
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Trigger decision, runs and run document, steps, follow-ups, components, interactions, command execution, invocation tokens.

## Code

- `internal/runs` (`runs.go`, M2-05): the port. `Runs` is the read side (`Get`, `ListByThread`, `ListSteps`, `ListActive`, `ActiveForActor`) with the DTOs `Run`, `RunSummary`, `RunStep`, `Money`, the pages and the queries; `Register` declares `run.state_changed`. No other module changes run states.
- `internal/runs/domain` (M2-04, M2-05): the trigger decision of [09-agents.md](../../../../docs/spec/09-agents.md) section 3 as pure functions. `trigger.go` holds the input types and `Decide`, `trigger_command.go` holds `DecideCommand`, `trigger_settle.go` holds `Verdict`, `Settlement` and `Settle`, `trigger_reasons.go` holds `Reason` (one constant per code of 09 3.5) and the constants of `Outcome`, `Step`, `Check`, `TranscriptSource` and `TriggerState`. M2-05 adds `state.go` (the six states, `Active`, `Ended`, `CanTransition` with the ten pairs of 03 `runs`), `reason.go` (the reason table of 09 5.2 plus `user_disabled` with state and setter, `ValidateServerReason`, `NormalizeAdapterReason`, `OriginalReasonDetail`) and `triggerstate.go` (`TriggerOnOpen`, `TriggerOnTransition`, the mapping of 09 6). The package imports only the standard library and `kernel-core` (`kernel/ids`, `kernel/errs`, `kernel/textcheck`); it has no I/O, no clock, no randomness and never calls `authorize`.
- `internal/runs/app` (M2-05): `Service` with the lifecycle `Open` and `Transition`, the reads, `ActiveGroups` (the gauge), and `Acceptor`, the in-transaction subscriber of `messaging.MessagePosted`; `ports.go` holds `Repository` and its row types, `metrics.go` the outgoing `Metrics` port.
- M2-06: `domain/invocation.go` (claims, header constants, `ParseInvocationToken`, `CheckInvocationClaims`; it adds `kernel/jose` to the imports of the domain), `app/invocation.go` (`InvocationTokens` with `Issue` and `Verify`, the redacting `InvocationToken`) and `adapters/runstest` (the reference verifier, test support only).
- M2-15: `interactions.go` (the port `Interactions` with `Create`, `Respond`, `UpdateComponent` and the DTOs `CreateInteraction`, `InteractionResult`, `RespondInteraction`, `UpdateComponent`, `Interaction`, `Component`); `domain/components.go` (`component.v1` as typed structs, `CheckCard` with the canonical bytes of the request hash, `CheckDefinition`, `SameDefinition`), `domain/values.go` (`CheckValues`, `SameValues`), `domain/interaction.go` (`EffectiveState`, `StateTransition`, `RouteInteraction`, `InteractionAuditDetails`, `InteractionCreatedPayload`); `app/components.go` (`CheckCard`, `AttachCard`, `ForMessages`, `UpdateComponent`), `app/interactions.go` (`Create`, `Respond`), `app/component_lifecycle.go` (`DisableRunComponents`, `Acceptor.OnMessageDeleted`, `ExpireComponents`) and `app/component_ports.go` (`ComponentRepository` and its row types); `adapters/messagingsource` (`Source`, the source ports of `messaging`), `adapters/jobs/component_expiry.go` (the job and `ComponentScheduler`), `adapters/postgres/components.go` with `queries/components.sql` and `queries/interactions.sql`, and the three handlers in `adapters/httpapi/interactions.go`. `FuzzCheckCard` and `FuzzCheckValues` sit next to the domain functions.
- `internal/runs/adapters/postgres` (M2-05): the sqlc queries over `runs` and `run_steps` and `Repository`. `adapters/httpapi`: the four REST operations and the cursor codec. `adapters/otelmetrics`: the run metrics of signals.md 4.3 and the gauge `bothy.run.active`.
- M2-25: `appdispatch.go` (the source port `AppDispatcher` with `AppWay`, `AppPost`, `AppAsk`, `AppOutcome` and the closed reason list `AppReasons`); `domain/appowner.go` (the target thread of an app command, the `CommandInvokedPayload` of an app owner, the map from `AppOutcome` to the next interaction state); `app/app_commands.go`, `app/app_interactions.go` and `app/app_autocomplete.go` (the app branches of `CommandExec` and `Interactions`), `app/app_dispatch.go` (the use case of the job), `app/components_authored.go` (`DisableAuthoredBy`); `adapters/jobs/app_dispatch.go` (the job `runs.app_dispatch`).

## Trigger decision

The tickets that apply the decision (M2-05 acceptance, M2-11 follow-ups, M2-16 commands, M3-09 voice) call these functions instead of re-implementing the rules. The domain chooses the candidate and the reason a person sees; the `agent.invoke` guard of `authz` re-checks operator and guest rule under the permission snapshot, and when the two disagree `authz` wins through `Settle` (D-M2-04-1).

### Decision table of `Decide`

Evaluated in this order; the first row that decides wins, and `Decision.Step` names it. E is the set of distinct IDs of `Mentioned`; the operator of the thread is `ThreadOperator` for `kind = 'thread'` and `ChannelOperator` for the main thread (09 3.1).

| Step | Condition | Outcome | Reason |
|---|---|---|---|
| validate | input invalid (D-M2-04-5) | error, zero `Decision` | |
| `origin` | `Origin` is not `human` | `none` | |
| `direct_message` | `ChannelKind` is `dm` | `none` | |
| `mentions` | E has more than one element | `reject` | `multiple_agents_mentioned` |
| `mentions` | E = {A}, A is the operator of the thread | candidate A, mentioned | |
| `mentions` | E = {A}, A is not the operator and the guest rule holds | candidate A, mentioned, guest | |
| `mentions` | E = {A}, otherwise | `reject` | `agent_not_allowed_here` |
| `mentions` | E empty, an operator exists and (`FreeResponse` or (main and not `RequireMention`) or (thread and not `ThreadRequireMention`)) | candidate operator, not mentioned | |
| `mentions` | E empty, otherwise | `none` | |
| `trigger_right` | not `AuthorHasTrigger` and the candidate was mentioned | `reject` | `no_trigger_right` |
| `trigger_right` | not `AuthorHasTrigger` and the candidate was not mentioned | `none` | |
| `thread_operator` | the candidate is the fixed operator of the thread (not a guest) and `ThreadOperatorOutside` (addendum of 2026-10-30) | `reject` | `operator_not_in_channel` |
| `voice` | `Voice` and no transcription path | `reject` | `voice_unsupported` |
| target | main thread and `AutoThread`: target is a new thread (`NewThread`); otherwise the thread of M | | |
| `active_run` | target is the thread of M, `ActiveRun` set, `ActiveRun.AgentID` is the candidate | `followup`, `RunID` set | |
| `active_run` | target is the thread of M, `ActiveRun` set, another agent | `reject` | `run_active_other_agent` |
| `invoke_check` | otherwise | `invoke` | |

- Guest rule (09 3.6, 05 section 5 point 3): a mentioned agent that is not the operator of the thread is a guest exactly when `AllowMentionedAgents`, its own `WriteOnChannel` and `BillingMode = organization` (R-AZ-19, R-AG-22). A mentioned operator is never a guest. The channel's current operator mentioned in a thread with another fixed operator is a guest candidate like any other agent (R-DM-12). `Active` is not part of the rule; step 7 checks it.
- Transcription path (09 13, D-M2-04-2): `voice.transcription_mode = service`, or `agent` and the candidate's effective `voice_input`; `off` has none. With a path, `Transcript` is `service` or `agent`.
- New thread (09 3.7): `ChannelID`, `ParentMessageID = M`, `OperatorAgentID` = the channel's effective operator (zero when none), `Title = ""`, `TitleSource = "auto"`. `trigger_state` stays on M (`TriggerMessageID`). A new thread has no active run, so step `active_run` does not apply to it.
- `Check` names the authorization that runs next: `CheckInvoke` (`agent.invoke`) for `invoke` and `control`, `CheckFollowUp` (`run.followup.create`, "write and `agent.invoke`") for `followup`, none for `none` and `reject`. Every outcome that can lead to a run or a delivery carries a check.
- `DecideCommand` (09 10.4): a control command gives `control` with `CheckInvoke`, also during a run, and is never a follow-up; a model command during an active run of any agent gives `reject` with `run_active`; otherwise `invoke` with `TriggerKind = "command"` and `CommandID`. App commands are not decided here (M2-25).

### Where each input comes from (for M2-05)

| Field of `MessageTrigger` | Source |
|---|---|
| `MessageID`, `ThreadID`, `ChannelID`, `Origin`, `ThreadKind`, `Voice`, the IDs of `Mentioned` | `messaging.MessagePosted` (M1-56; `Kind`, `MentionedIDs`). Mentions resolve only to active principals the author may see (07 4.6), so a disabled agent is never in `Mentioned`. |
| `ChannelKind`, `ThreadOperator` | `chat.Threads.Info` (`ThreadInfo.ChannelKind`, `OperatorAgentID`; M1-55) |
| `ChannelOperator` | the effective operator of the channel (`chat` domain `EffectiveOperator`, M1-54; 03 `channels` "Effective operator") |
| `Switches` | the channel behaviour of `chat` (M2-02; schema `ChannelBehavior`) |
| `AgentFacts.Active`, `AgentFacts.BillingMode` | `agents` (M1-51: principal status, `agents.billing_mode`) |
| `AgentFacts.VoiceInput` | `agents`: owner permission `supports_voice` and adapter report `voice_input` (09 2.2, R-AG-01) |
| `AuthorHasTrigger`, `AgentFacts.WriteOnChannel` | `authz` (07 2.3, 07 2.9; `WriteOnChannel` is the agent's own effective level of at least `write` of 05 section 5 point 3, which includes the operator rule) |
| `ActiveRun` | `runs` itself (M2-05: the run in `queued`, `running` or `waiting_input` of the thread) |
| `ThreadOperatorOutside` | `runs` itself (FIX-run-retry-and-orphan-operator): the thread operator is not `ChannelOperator`, not the agent of `ActiveRun`, has no `write` of its own on the channel (`thread.create` on the channel for the agent, filter mode) and `CompletedRunOfAgent` finds no completed run of it in the thread |
| `VoiceMode` | configuration key `voice.transcription_mode` |

The acceptance check (D-M2-04-4) builds the `agent.invoke` resource on the thread of M, also when the target is a new thread, with `InvokeFacts.AgentID` = the candidate, `TriggerOrigin = human`, `MentionsAgent = Decision.Mentioned` and `ActiveRunAgentID` zero for a new thread.

### `Settle`: verdict to `trigger_state`

`none` keeps `trigger_state` NULL; `reject` gives `failed` with its reason, whatever the verdict. For `invoke`, `followup` and `control` an allow gives `Accepted` and, for message decisions, `received`; a denial gives `failed` and the reason by this table (D-M2-04-3). Command decisions never set a trigger state. A denial with an empty reason is an internal error.

| Deny reason from `authorize` (07 4.1) | `trigger_state_reason` |
|---|---|
| `agent_not_reachable`, candidate `Active` false | `agent_disabled` |
| `agent_not_reachable`, candidate `Active` true | `agent_not_allowed_here` |
| `level_insufficient`, `no_grant` | `no_trigger_right` |
| every other reason (`billing_mode_personal_not_holder`, `project_not_ready`, `run_active_other_agent`, `tool_class_owner_only`, `dev_environment_not_ready`, `channel_read_only`, ...) | unchanged |

This package never yields `queued`, `working`, `done`, `cancelled` or `followup_delivered` (M2-08, M2-09, M2-11). The audit of a denial is written by `authz`.

### Rulings

- D-M2-04-1: the decision is split into `Decide`/`DecideCommand` (steps 1 to 6 of 09 3.4, 13 and 10.4) and `Settle` (step 7); the domain never calls `authorize`.
- D-M2-04-2: the voice rule runs after step `trigger_right` and before the target.
- D-M2-04-3: the mapping of deny reasons above.
- D-M2-04-4: the descriptor of the acceptance check above.
- D-M2-04-5: invalid input (an unknown value of `Origin`, `ChannelKind`, `ThreadKind`, `BillingMode` or `VoiceMode`; `ThreadOperator` on a main thread; a main thread in a forum channel; an operator on a DM channel; a zero ID) returns an error with code `internal_error` and the zero `Decision`, which never invokes. The error names the field, never a value.

## Port (root package `internal/runs`)

- Incoming (M2-05): `Runs` (reads; `Retry`, the use case of `retryRun` with the rights of `agent.invoke`, 09 5.7, in `app/retry.go`). M2-08: `Delivery` (`delivery.go`: `Prepare`, `DeliveryInput`, `DeliveryOutcome`, `InvocationBearing`), called by the agent consumer engine of `gateway` (WebSocket and REST poll). M2-09: start, resume, renewal, wait, finish and cancel in `Runs`; M2-10 adds `Runs.Patch` and the delivery reads `LiveBox`. M2-15: `Interactions` (`interactions.go`: `Create` for `createInteraction`, `Respond` for `respondToInteraction`, `UpdateComponent` for `updateComponent`; the principal comes from the context). M2-16: `CommandExec` (`Autocomplete` and `AnswerAutocomplete` M2-17). M2-25: `Interactions.DisableAuthoredBy`, called by `apps` at `app.suspend` and `app.uninstall`. M7-08: `RunningRuns` (`maintenance.go`: `Running`, the runs of an agent in state `running` with their threads, one read of `RunningRunsOfAgent` over `runs_state_agent_active_idx`), which the maintenance stage of `platform/httpserver` and the op `run.update` of the gateway read through `internal/wire` for the exceptions of ADR 0065 item 4; it never authorizes. In maintenance mode `Delivery.Prepare` holds every delivery whose run is not a running run of the agent, and every `command.invoked` (`DeliveryOutcome.Hold`, nothing checked or written, D-M7-08-13). Source ports: `AutocompleteRelay` (M2-17), `AppDispatcher` (M2-25, implemented by `apps/adapters/appdispatch`).
- Emits durable events (M2-08): `agent.invocation` (audience the agent, receive action `agent.events.consume`, resource the run without version, no token). M2-15: `interaction.created` (audience the card's agent, receive action `agent.events.consume`, resource the interaction without version, payload `InteractionCreatedPayload` with the values and without `response`). M2-25: `command.invoked` and `interaction.created` of app commands and app components with the owning app as audience, the same receive action (an app passes it with an active installation and `events:subscribe`, decided by `authz` in filter mode) and without `run_id`.
- Emits in-tx events: none.
- Subscribes (M2-05): `messaging.MessagePosted` (the acceptance, `app.Acceptor`, registered after the subscriber of sync). M2-09: kernel `authorize.AccessChanged` and `agents.AgentDisabled`. M2-15: `messaging.MessageDeleted` (`Acceptor.OnMessageDeleted`, registered after them). `chat.ChannelReadOnlySet` (M8-03): see "Run lifecycle after the delivery". M2-12: `agents.SessionAttached` (`Acceptor.OnSessionAttached`). M2-28: `billing.CostsReconciled` (`Acceptor.OnCostsReconciled`, registered last). Planned: `files.FileFinalized`.
- Uses the ports `chat.Threads` (`Info`, `Locate`, `Create`; M2-15 `LockForMessageChange`), `chat.TriggerFacts`, `agents.Facts` (M2-15 `RunFacts.ComponentsAllowed`), `agents.Sessions` (`ForInvocation`, `AttachOnStart`, `CheckStartModel`; M2-12; `AddCost`, M2-22), `billing.Costs` (`ValidateReports`, `BookRun`; M2-22) and `messaging.Messages` (`CreateRunAnchor`, `SetTriggerState`, `TriggerMessageOf`; M2-12 `AppendSystemLine`; M2-15 `LockCard`, `ComponentsChanged`, `PrincipalOf`; M2-25 `Authors`).
- Implements source ports and cross-cutting ports: `agents.RunSource` (M2-12, `adapters/postgres.RunSource`), `messaging.ComponentReader` and `messaging.ComponentWriter` (M2-15, `adapters/messagingsource`), kernel `personaldata.Participant` (M7-06).

## Run lifecycle (M2-05)

- `Open` (inside the caller's unit of work): `INSERT ... ON CONFLICT` on the partial unique index of the active run of a thread `DO NOTHING` (R-DM-14; `ActiveRunExistsError` otherwise, nothing written), the anchor through `messaging.CreateRunAnchor`, `run.state_changed`, then `trigger_state = received`.
- `Transition`: the pair must be one of 03 `runs` (`409 invalid_state_transition` otherwise), the reason one of 09 5.2 for its setter (an unknown adapter reason becomes `adapter_error` with a status step `adapter_reason`), one compare-and-swap `UPDATE` on `state` and `version` (`ConcurrentChangeError` with the current run when it lost, `not_found` for an unknown run), `run.state_changed`, then `trigger_state` per 09 6 through `messaging.SetTriggerState`. The metrics are recorded after the commit (`txn.AfterCommit`).
- Lock order: `runs` before `messages`; the acceptance holds the thread row of the message.
- Every state change, the creation included, appends `run.state_changed`: resource `run` with the version of the run row, the receive action of reading a run, payload `RunStateChangedPayload` (the summary, the triggering message and the trigger state this change set).
- Acceptance (`Acceptor.OnMessagePosted`): only `origin = human`, `kind = text`, no DM; facts through the ports; `Decide`; a candidate without mention is checked in filter mode first (no trigger right: nothing written, no audit); then `agent.invoke` in request mode, which audits denials; the auto thread only after `allow`; a disagreement of `Decide` and `authorize` fails closed. Until `Bind` the acceptor writes nothing (D-M2-05-19).

## Delivery (M2-08)

`app.Service.Prepare` implements `runs.Delivery` in one unit of work, immediately before every hand-over of an invocation-bearing event to an agent (04 15.3 step 2, invariant I2 "before every invocation"):

1. A type outside `InvocationBearing` is an internal error; a type or trigger kind without a resolver (`command.invoked` and the trigger kind `command` until M2-16) gives `Send = false` and an `Error` log line (fail closed).
2. The run row is locked (`LockRun`, `SELECT ... FOR UPDATE`); a missing run or a run of another agent is an internal error. The run of `agent.invocation` and `agent.followup` is the event's resource; the run of `interaction.created`, whose resource is the interaction, is the `run_id` of its payload (M2-15).
3. The delivery matrix (`domain.DeliveryMatrix`): terminal runs are skipped without a check; `agent.invocation` records its `jti` only while `queued`; `agent.followup` and `interaction.created` record it only in `waiting_input`; `command.invoked` never.
4. A `queued` run at `queued_at + run.queued_max_age` or later fails with `agent_unavailable` without a check, a token or an audit entry.
5. The resolver reads the trigger: for `agent.invocation` with `trigger_kind = message` the run's trigger and its triggering message now (origin, mention of the agent, `NewThread` when the message lives in another thread than the run, the auto thread); for `agent.followup` the author of the follow-up message of the payload. M2-15: for `interaction.created` the person of the interaction (`interactions.principal_id`), the interaction as triggering object and the component's facts read now (`authorize.InteractionFacts`, stage `deliver`); an interaction that no longer exists (its card was deleted) gives `Send = false` without a check or an audit entry. For `agent.invocation` with `trigger_kind = interaction` the run's `triggered_by`, `triggering_interaction_id` (or, once the column is NULL after a deletion, the `interaction_id` of the stored payload) and the facts of that interaction's component; without its interaction the check runs without them.
6. `agent.invoke` in filter mode for `actor.Recheck(trigger)` (R-AZ-29) at the permission version read once through `instance.Versions` and pinned, on the run's thread with `ActiveRunAgentID` = the run's agent.
7. Denial: the authorize reason maps to a run reason (`domain.DeliveryReason`: I2, SR2, project, read-only and `principal_inactive` keep their codes as `billing_mode_personal_not_holder`, `tool_class_owner_only`, `project_not_ready`, `channel_read_only`, `user_disabled`; `agent_not_reachable` of an inactive agent is `agent_disabled`; everything else `access_revoked`). `agent.invocation` of a queued run fails the run through `Transition` (which sets `trigger_state = failed`); `agent.followup` fails the follow-up message and leaves the run; `interaction.created` sets the interaction `failed` and leaves the run (M2-15); a started run stays. The failed queued run goes through the end-of-run function `endRun` (M2-15), so its components are disabled. One `auditing.Recorder.Record` in the unit: action `agent.invoke`, decision `deny`, the exact authorize reason, resource the thread, attributes `run_id`, `agent_id`, `stage` (`delivery`), `event_type`, `trigger_kind`, the re-check principal as actor. After the commit `bothy.agent.invocation.denied` counts it.
8. Allow: the token of `InvocationTokens.Issue` with the claims of `domain.DeliveryGrant` (`level = trigger`, the configured `billing_mode` and `tool_class`, the pinned `permission_version`); `RecordDelivery` stores `invocation_jti` (when recorded, with `renewed_from_jti = NULL`) and `billing_mode_expected` and raises `version` by one, only when a column changes; the first delivery of `agent.invocation` of a queued run moves `trigger_state` from `received` to `queued`; an allowed `interaction.created` moves the interaction from `received` to `dispatched` (an interaction the agent already answered keeps its state, M2-15). `Send = true` with the token, which is a delivery field only.

The span `agent.dispatch` (PRODUCER, `bothy.run.id`, `bothy.agent.handle`, `bothy.decision`) covers every checked delivery. The tokens are bound in the signing keys step (`BindTokens`).

Acceptance (M2-08 additions): `Open` schedules `runs.queued_expiry` at `queued_at + run.queued_max_age` in the creating transaction, and the acceptance appends `agent.invocation` as its last write (`domain.InvocationPayload`: IDs, `trigger_kind = message`, the author as `triggered_by`, the billing mode, `settings_version`; no content). The acceptance keeps the audit of its denials in request mode of `authz` (D-M2-08-8).

Job `runs.queued_expiry` (`adapters/jobs`, queue `default`, args the run only): `ExpireQueued` locks the run; still queued and due, it fails with `agent_unavailable` through `endRun` (M2-15); queued but not yet due (the key was raised), it schedules itself again; otherwise nothing.

## Command execution (M2-16)

`CommandExec.Invoke` (`commands.go`, `app/command_exec.go`, handler `adapters/httpapi/commands.go`)
runs `invokeCommand` in one unit of work: `agents.Commands.Resolve` (404 for a command not usable
in the channel), the target thread (with `thread_id` a thread of the channel; without it the main
thread of a text channel for a control command and for a model command only without
`auto_thread`, else `400` `/thread_id` `required`), the thread row lock for a model command
(`chat.Threads.LockForMessageChange`, the lock order of the acceptance), the active run of the
thread and `DecideCommand`, then `command.invoke` and `agent.invoke` in request mode (the command
addresses its owner like a mention; the active-run rule is `command.invoke`'s), the option values
(`domain.ValidateOptionValues`) and the execution:

- control command: `command.invoked` with audience the owner, receive action
  `agent.events.consume`, resource `command` without version, `run_id` exactly when the owner has an
  active run in the thread (R-AZ-25); no run, no anchor, no follow-up;
- model command: `Open` with `trigger_kind = command` and `triggering_command_id`, then
  `agent.invocation` with `command_invocation_id`, `command_id`, `command_name` and the options in
  definition order, without `message_id`.

A refusal maps to `403 agent_invocation_denied` with the reason, `403 channel_read_only` or `404`.
The invoker is rendered by `messaging.Messages.PrincipalRefs`. At delivery (`app/delivery_command.go`)
`command.invoked` takes its own path without a run lock and without recording a jti: `agent.invoke`
for the re-check principal of the invoker on the thread of the payload, a denial sends nothing and
writes one audit entry with `command_id`, `stage = delivery` and `event_type`; an allow issues a
token with `trigger_kind = command`, `command_id` and the payload's `run_id` (a control grant may
lack a run, `InvocationGrant.Control`). `agent.invocation` of a command run has its own resolver
(the payload's `command_id` as the triggering object) and otherwise follows M2-08.

## Run lifecycle after the delivery (M2-09)

The port `runs.Runs` gains `Start`, `Resume`, `RenewToken`, `AwaitInput`, `Finish` and `Cancel` (REST `startRun`, `resumeRun`, `renewInvocationToken`, `awaitRunInput`, `finishRun`, `cancelRun` in `adapters/httpapi`); the principal comes from the context. Every operation runs in one unit of work with the run row locked first (`LockRun`); several runs are locked in ascending id.

- **Token verdict** (`domain.StartVerdict`, `domain.RenewVerdict`): the token is verified through `InvocationTokens.Verify` without its expiry (any failure is `invalid`); then, first match wins, the start token of a running run is a replay, the start token of a waiting run is `superseded` (consumed), an ended or running run is `already_started`, a `jti` other than `invocation_jti` is `superseded`, now after `exp` plus `auth.clock_leeway` is `expired`, the wrong endpoint is `invalid_state_transition`. A renewal accepts the current `jti` or `renewed_from_jti` unless the start consumed it and never checks the expiry. A token of `trigger_kind = interaction` (M2-15) is re-checked like the delivery it came with: `agent.invocation` for the interaction that opened the run, `interaction.created` for any other interaction of the run (`agent.invocation` also once `triggering_interaction_id` is NULL); an `interaction.created` token whose interaction was deleted is not renewed (`403 invocation_token_invalid` with `reason` `invalid`), the token of a run whose opening interaction was deleted is checked without the component facts and renewed on allow (D-M2-15-22).
- **Token facts:** `authorize.TokenFacts` carries the verdict and, for a transition, the reported mode, the configured mode, `billing_evidence.key_type`, the reported tool class and the claim `tool_class`; `authz` decides `run.start` and `run.token.renew` on them (`invocation_token_invalid` with the verdict as problem reason; the guard gives `billing_mode_mismatch` and `tool_class_mismatch`). An I3 mismatch fails the run in the unit, which commits, and answers 409 afterwards; `bothy.agent.billing_mode_mismatch` counts it.
- **Start:** `Transition` with `TransitionStore` stores `started_jti`, clears `renewed_from_jti`, stores `billing_mode_used`, `billing_evidence` and `model`; since M2-12 the start attaches the agent session (see "Agent sessions (M2-12)"; ruling D-M2-09-6 is superseded).
- **Renewal:** a token with `trigger_kind` message, followup, command (the `agent.invocation` of a command run, M2-16) or interaction (M2-15, below) is renewed. Steps 4 and 5 of 09 4.5.1 reuse the resolvers and the reason mapping of the delivery check: `agent.invoke` for `actor.Recheck(sub)` at the current permission version; a denial fails the run with the mapped reason, audits it with stage `renewal` and answers 403 `agent_invocation_denied`; an allow issues a fresh token and replaces `invocation_jti` through `RecordDelivery`, with `renewed_from_jti` set to the presented `jti` when it was current.
- **Wait and end:** `AwaitInput` moves `running -> waiting_input` with the status line. `Finish` applies `domain.FinishDecision` (the reason rules of D-M2-09-9) and books the reported `costs` (see "Costs in the finish (M2-22)"); the same end state again changes nothing and books nothing. Every end state goes through `endRun`, the one end-of-run function and the extension point of M2-11 and M2-15 (M2-15 calls `DisableRunComponents` there); an adapter's I3 mismatch writes one audit entry of `run.finish`.
- **Cancel:** `requestCancel` is the one function behind every cancellation (M2-16 `/stop` and M8-03 reuse it): it appends `run.cancel_requested` (personal to the agent, `requested_by` only for `user_cancelled`) and either ends the run at once or schedules `runs.cancel_timeout` at now plus `agent.cancel_grace` (`CancelTimeoutScheduler`, `adapters/jobs.CancelScheduler`). `Cancel` ends a queued run at once, requests otherwise, answers a cancelled run without change and audits the allow (`state_before`, `immediate`, `cancel_reason`, `no_change`); the reason text is checked and stored nowhere.
- **Read-only channel (M8-03):** `Acceptor.OnChannelReadOnlySet` (in the unit of `channel.read_only.set` or `setImportLead`) does nothing when the flag is lifted; when the channel becomes read-only it reads the threads of the channel through `ChannelThreads` (`chat.Structure.ThreadIDsOfChannels`), locks their active runs in ascending id (`LockActiveRunsByThreads`) and calls `requestCancel` with `channel_read_only` for each (05 `channel.read_only.set`, 22 13.4).
- **Revocation and disabled agent:** `Acceptor.OnAccessChanged` (phase 3.1, never calls `authorize`) locks the active runs of the affected starters, keeps those whose channel they lost (`domain.LostTriggerRuns`; every run for `user.disable`) and requests their cancellation with `access_revoked`, `user_disabled` or `channel_read_only` (`domain.CancelReasonFor`); the states stay. `Acceptor.OnAgentDisabled` ends the agent's active runs `cancelled` with `agent_disabled` at once.
- **Orphans:** `StallCheck` reads the started active runs and the persisted heartbeat of their agents (`agents.Heartbeats`) and fails `interrupted` and `stale` runs (`domain.OrphanVerdict`), one unit of work per run with the verdict decided again under the lock; nothing is retried.

## Components and interactions (M2-15)

`runs` owns `message_components` and `interactions`; `messaging` owns `messages` and the request of `createMessage` and may not import `runs`. Rulings D-M2-15-1 to D-M2-15-30 in docs/decisions.d/M2-15.md; the behaviour is 09 section 11.

**Ports.** Incoming `runs.Interactions` (`Create`, `Respond`, `UpdateComponent`). The source ports of `messaging` are implemented in `adapters/messagingsource`: `Source.Check` and `Source.Attach` (`messaging.ComponentWriter`, over `app.Service.CheckCard` and `AttachCard`) and `Source.ForMessages` (`messaging.ComponentReader`, one statement for all messages of an answer or page, in the ambient transaction when there is one). `internal/wire` builds the `Source` before the messaging module, which receives it, and binds it in `NewRuns`; an unbound source fails closed with an internal error (D-M2-15-25). The outgoing ports are `ComponentRepository` (`app/component_ports.go`, implemented by `adapters/postgres`), `CardMessages` (`messaging.Messages`: `LockCard`, `ComponentsChanged`, `PrincipalOf`), `ThreadLocker` (`chat.Threads.LockForMessageChange`) and `ComponentExpiryScheduler` (`adapters/jobs.ComponentScheduler` over `jobs.Inserter.InsertTx`, bound by `Runs.BindJobs`). Without `Deps.Components` the components are off: a card is refused, `ForMessages` returns none.

**Card rules and error codes.** `CheckCard` (no database access) applies `component.v1` and the server fields: closed member sets per type, semantic icon names of `kernel/designkeys`, `kernel/textcheck` on every display text, `value` and `custom_id`, the keys `component.max_per_message`, `component.custom_id_max_chars`, `component.form_max_fields` and `component.default_ttl`; `400 validation_failed` with one `errors[]` entry per finding below `/components/<index>/...` and the codes `required`, `too_long`, `limit_reached` or `invalid` (`detail` names the rule, never the text). It returns the canonical bytes of the request hash `bothy-msg-v2` and the components as they will be stored (normalized definition with defaults). `AttachCard` (inside the unit of work of `createMessage`, after the message row) checks the request again, locks the run of `run_id` with `FOR SHARE` (an active run of the author in the message's thread, with the answers of M2-19: `400 validation_failed` with `run_mismatch` on `/run_id`, or `409 conflict` with `run_not_active` for an ended own run; D-M2-15-29), refuses `supports_components = false` with `403 feature_disabled` (`agents.RunFacts.ComponentsAllowed`; an app author has no agents row and is not asked, M2-25), inserts the rows (`state = active`, `version = 1`, `run_id` the card's run; `messages.run_id` stays NULL) and schedules `runs.component_expiry` at the earliest `expires_at`. The body rules before it (`kind`, people and apps, `run_id` on other kinds) and the size rule belong to `messaging`.

**`createInteraction` (`Create`).** `client_interaction_id` must be a UUIDv7 (`not_uuid_v7`). The component and its card are read without locks (unknown: `404`); one unit of work takes the locks below, answers a repetition (the person must still read the card, else `404`; the same `component_id` and `values` equal as JSON give `200` with `Idempotent-Replay`, anything else `422 idempotency_key_reuse`), decides `interaction.create` with `authorize.InteractionFacts` at stage `accept` from the locked rows (denials are audited by authz and mapped by `Decision.Err`: `component_inactive` as `409 component_used`), runs `CheckValues` (`interaction_values_invalid`) and the payload size rule (`too_long` on `/values`), and writes: the single-use rule (chosen `used` with `used_by` and `used_at`, the rest of the locked group `disabled`, each `version + 1`; a component without `single_use` stays unchanged, D-M2-15-13), the interaction in `received`, `ComponentsChanged` when a component changed, the run of `domain.RouteInteraction` (the component's run, or `Open` with `trigger_kind = interaction`, `TriggeringInteractionID`, the person as `TriggeredBy`), `interaction.created`, `agent.invocation` last for a new run (`triggered_by` through `Messages.PrincipalOf`) and the allow audit of an `approve_actions` component (`domain.InteractionAuditDetails`: IDs, `component_type`, `required_capability`, `restricted_to`, `choice` only for `once`, `session`, `always`, `deny`). A unique violation of the idempotency key (`ErrInteractionTaken`, only when the locks were bypassed) is answered like a repetition in a new unit of work.

**`respondToInteraction` (`Respond`).** First the state (`invalid_enum` on `/state` outside `acknowledged`, `failed`, `rejected`) and `response.text` at most `message.max_content_bytes` UTF-8 bytes (`too_long` on `/response/text`, no text check); then the interaction is read and `authorize` runs on the interaction path with the card's author as owner; under `FOR UPDATE` of the interaction: from `received` or `dispatched` the state and response, the same final state again without a write, every other transition `409 invalid_state_transition`; no event.

**`updateComponent` (`UpdateComponent`).** `authorize` on the component path (agent-self, app-self), the definition through `CheckDefinition`; in the unit of work `LockCard`, the whole single-use set of the card for a state change of a `single_use` component (else the component row), `412 version_conflict` with the current component when `If-Match` differs, `domain.StateTransition` (`409 invalid_state_transition`); no change answers without a write; a change raises the component version, calls `ComponentsChanged` and, for a reactivation, schedules the expiry job again. `428` for a missing `If-Match` comes from the HTTP chain.

**Lock order** (03 section 6.3, D-M2-15-19). Domain rows in this order across tables: `threads` (only where a run may be opened or a message is written), `runs` (ascending `id`), `messages` (ascending `id`), `message_components` (ascending `id`), `interactions`; the counters last (unit of work). `Create` follows it (thread, the component's run and the thread's active run, `LockCard`, the single-use group or the component), and so do `UpdateComponent`, `DisableRunComponents` (the run is locked by `Transition`, then per card `LockCard` and its components), `ExpireComponents` and `Respond`. The one later `runs` lock is the share lock of `AttachCard` after the new card row, which no other transaction can see yet. Deadlocks that remain are retried by the unit of work (`database.tx_retry_max`). Two concurrent clicks on one card serialize on the single-use group; the loser sees the winner's rows and gets `409 component_used` (AG-15).

**Delivery and renewal.** The resolver table holds `interaction.created` (`resolveInteraction`) and `agent.invocation` with `trigger_kind = interaction` (`resolveInvocationInteraction`); the check at stage `deliver` evaluates `required_capability` and `restricted_to` and skips the state and `run_active`. An allow sets the interaction `dispatched`, a denial `failed` with the run unchanged; see "Delivery (M2-08)" above. `renewInvocationToken` rebuilds the delivery of an interaction token the same way (D-M2-15-22).

**End of run, follow-up answer and deletion.** `DisableRunComponents(ctx, runID)` disables the active components of a run in the caller's unit of work, grouped by card in ascending message id, with one `message.updated` per card. `endRun` calls it at every end state (D-M2-15-21), and the denial of a queued run at delivery and `runs.queued_expiry` go through `endRun`; `Followups.store` (M2-11) calls it when a follow-up answers a run in `waiting_input` (09 11.6). `Acceptor.OnMessageDeleted`, the in-transaction subscriber of `messaging.MessageDeleted`, deletes the components of the deleted message; their interactions follow by cascade and `runs.triggering_interaction_id` becomes NULL; no event of its own (the tombstone carries `components: []`).

**Job `runs.component_expiry`** (`adapters/jobs/component_expiry.go`, queue `maintenance`, args `ComponentExpiry{MessageID}` only, registered in every role, run by `worker` and `all`): scheduled per card at its earliest `expires_at`, never periodic. `ExpireComponents` locks the card (missing or deleted: done), locks its active components with `expires_at` at or before now in ascending id, sets them `expired` with `version + 1`, calls `ComponentsChanged` once if any changed and schedules itself for the next `expires_at`. Repeated execution is harmless; correctness never depends on the job, because `authorize` treats a passed `expires_at` as expired.

**Visibility.** `message_components` and `interactions` have no `channel_id`: every query names the components of messages that `authorize` checked, that `messaging` loaded for an event of the same transaction, or that a job or the end of a run names (07 7.4 rule 1, `tools/lint/sql_visibility.yaml`). Form values live only in `interactions`; they leave the server only in the answers of `createInteraction`, in `interaction.created` and in the answer of `respondToInteraction` to the card's agent or app, and for an app with `delivery.interactions_url` in the signed request to that address (M2-25; SEC-T10).

## Autocompletion (M2-17)

`CommandExec.Autocomplete` and `CommandExec.AnswerAutocomplete` (`autocomplete.go`,
`app/autocomplete.go`, `domain/autocomplete.go`, handlers `adapters/httpapi/autocomplete.go`) serve
`autocompleteCommand` and `answerAutocomplete`; the gateway op `command.autocomplete_result` calls
`AnswerAutocomplete` too. Neither holds a transaction, writes a row or appends a durable event, and
neither creates a run or a cost entry (R-AG-14).

- `Autocomplete`: `agents.Commands.Resolve` (404 for an unknown, removed, not covered or shadowed
  command), the thread of the request (404 when unknown or of another channel, before `authorize`),
  the channel facts, the target thread (`domain.AutocompleteTarget`: the given thread, else the main
  thread of a text channel, else none in a forum, R-AZ-26), then `command.autocomplete` in request
  mode at the permission version read once (`instance.Versions`) and the query rules
  (`domain.ValidateAutocompleteQuery`). An agent owner without a target thread gets an empty list
  at once. An app owner takes the app branch (section "Commands and interactions of apps
  (M2-25)" below). Otherwise, for an agent owner, the payload of `command.autocomplete_requested`
  (`400 validation_failed` with the field code `too_long` and an empty pointer above
  `sync.event_payload_max_bytes`), a fresh token with
  `trigger_kind = autocomplete`, no `run_id`, the command and the target thread
  (`domain.AutocompleteGrant`, target billing mode and tool class of the agent), the request in the
  registry, and the source port `AutocompleteRelay` (implemented by
  `gateway/adapters/autocompleterelay`). Not sent: an empty list at once. Sent: the first accepted
  answer, or an empty list after `command.autocomplete_timeout`; the end of the request context
  expires the request.
- `AnswerAutocomplete`: an unknown or forgotten request is `404` without `authorize`; then
  `command.autocomplete.respond` on the command at the place of the request with the owner as
  `OwnerID`, the answer rules (`domain.ValidateAutocompleteAnswer`, at most
  `command.autocomplete_max_choices`; the request stays open after a `400`), `410` for an expired
  request, `204` without effect for an answered one, otherwise the compare-and-swap
  `open -> answered` that wakes the waiter.
- The registry of open requests is a mutex-guarded map in the memory of the process
  (`autocompleteBox`); every transition is a compare-and-swap of `domain.AutocompleteRequest`.
  Answered and expired requests stay until `expires_at` plus `command.autocomplete_timeout` and are
  swept by every call (no goroutine). A restart forgets every request (at most once, 04 section 9).
  With `cluster.mode = multi`, before M7-17 distributes requests and answers, only owner
  connections and REST answers on the same api node are reached; elsewhere the person gets an empty
  list and the answer `404` (OQ-07, RK-37).
- Residual risk (T-AI-02, 07 SR9): an adapter that reads the API with its own rights while it
  computes suggestions could return names from channels the person cannot read; the server cannot
  filter free-form choices. SR9 binds requests only within a run; M2-14 decides about autocomplete
  tokens as `acting_for`.
- Keys: `command.autocomplete_timeout`, `command.autocomplete_max_choices` (bound with the relay in
  `internal/wire`, `BindAutocomplete`), `sync.event_payload_max_bytes`, `agent.invocation_token_ttl`.
- Registrations: `runs.Register` declares the ephemeral `command.autocomplete_requested` and the op
  `command.autocomplete_result`, whose handler the gateway runs on `CommandExec`.

## Commands and interactions of apps (M2-25)

An installed app owns commands and components like an agent, and none of them ever reaches an
agent: no run, no invocation token, no `trigger_state`, no `agent.invoke` and no I2 (09 sections
10.4 and 11, 11 section 9, 05 section 5, R-AZ-18). The branch is chosen by the owner kind: an owner
with an agents row is an agent, every other owner is an app. Rulings: docs/decisions.d/M2-25.md.

**Source port `AppDispatcher`** (`appdispatch.go`; 02 4.1 point 4). `runs` declares it and
`apps/adapters/appdispatch` implements it, because only `apps` reads the manifest, the event log and
the egress client of the interaction address; `runs` imports neither `apps` nor `sync`.
`internal/wire` binds it into `runs`.

- `Way(ctx, appID)` gives an `AppWay`: `inactive` without an active installation,
  `interactions_url` when the manifest declares `delivery.interactions_url`, `gateway` otherwise.
  The way follows the manifest, not a live connection, so it is the same on every api node (OQ-07).
- `Post(ctx, AppPost{AppID, EventID, ReplyThreadID})` sends a stored `command.invoked` or
  `interaction.created` to the interaction address and writes a `200` reply as a message of the app
  into `ReplyThreadID` (zero when there is none).
- `Ask(ctx, AppAsk{AppID, Event, Deadline})` sends an ephemeral `command.autocomplete_requested`
  and returns the choices of a valid `200` answer.
- `AppOutcome` holds exactly one of `Answered`, `Deferred` (a `202`: the app answers later through
  the API) and a `Reason` of the closed list `AppReasons`: `inactive`, `no_url`, `denied`,
  `timeout`, `transport`, `blocked`, `redirect`, `http_status`, `too_large`, `invalid_reply`,
  `no_thread`, `reply_refused`; `Choices` only for an answered `Ask`. Errors are returned only for
  faults of the server (database, programming); every answer of the app is an outcome.

**App commands** (`CommandExec.Invoke`, `app/app_commands.go`).

1. `agents.Commands.Resolve` (`404` for an unknown, removed, not covered or shadowed command); a
   given `thread_id` must be a thread of the channel (`404`, before `authorize`).
2. Target thread (`domain/appowner.go`): the given thread, else the main thread of a text channel,
   else none in a forum. No `auto_thread`: it exists to give an agent's run a thread.
3. `command.invoke` in request mode on the channel with the target thread, `Resource.OwnerID` the
   thread's creator and `CommandFacts{OwnerID: the app, OwnerKind: "app", RequiredLevel}`. `authz`
   decides the visibility of the place, the app-owner guard (`AppOwnerReachable` on the facts of
   the owning app: an active installation, `commands:write` and `write` on the channel; false gives
   `404 not_found` like an unknown command), `required_level` and the read-only and archive rules;
   never the `agent.invoke` guard. The answers map as for agent commands: `404 not_found`,
   `403 channel_read_only`, `403 agent_invocation_denied` with `reason`. Agents and apps may run
   an app command whose level they hold (spec gap reported).
4. Option values through `domain.ValidateOptionValues` (`400 validation_failed`).
5. One unit of work, joined with the idempotency stage: `command.invoked` (audience the app,
   receive action `agent.events.consume`, resource `command` without version, payload without
   `run_id`) and the job `runs.app_dispatch` through `jobs.Inserter.InsertTx`. No run, no token,
   no `trigger_state` and no `run_active` rule; `control` is passed through and changes nothing.
6. `202` with `CommandInvocation{invocation_id, command_id, channel_id, thread_id (with a target
   thread), accepted: true}` and without `run_id`. A replay with the same `Idempotency-Key` answers
   the stored answer without a second event or job.

**App cards.** `messaging` accepts `kind = card` from an app (`message.create` with `write` and
`messages:write`); `AttachCard` applies every card rule above except `supports_components`, which
is not consulted for an app, and stores the components without a run. An app has no run, so a
`run_id` on its card gives `400 validation_failed` with `run_mismatch` on `/run_id` (the answer
of M2-19). Posting needs no `components:write`; without it the app's components are inactive.
`updateComponent` and `respondToInteraction` by the app need app-self with `components:write` and
the card's author as `OwnerID` (`403 scope_missing` without the scope). The card header is the
author and `origin = app`, set by the server.

**Interactions on app components** (`Interactions.Create`, `app/app_interactions.go`).
`InteractionFacts{Stage: "accept", OwnerKind: "app"}` without `Invoke`, with the card's author
as `OwnerID`; `authz` checks `Level(trigger)`, the read-only and archive rules, then state, expiry
and the app-owner guard with `components:write` (any of them false: `component_inactive`,
answered `409 component_used`), `required_capability` (`capability_missing`) and `restricted_to`
(`owner` and `run_starter` pass only owners, because no run exists; `component_restricted`).
Replay, locks, the single-use rule, the value checks and `message.updated` are those of agent
cards. No run is opened or resumed; the unit appends `interaction.created` with the app as
audience (never `agent.invocation`) and inserts `runs.app_dispatch` with kind `interaction`. The
answer is `201` with `state = received` and without `run_id`.

**Job `runs.app_dispatch`** (`adapters/jobs/app_dispatch.go`, use case `app/app_dispatch.go`, queue
`default`, run by `worker` and `all`, paused in maintenance mode because it may write a reply).
Inserted for every app command and every interaction on an app component, whatever the way of the
app. The args are IDs only: the event, the app, the invoking principal and the installation of an
invoking app, the channel, the thread, and the command or the interaction (whichever is set gives
the kind; the kind of the invoker is read from the directory). The handler, in this order:

1. `Way`: `gateway` returns (the event reaches the app through its connection, `listEvents` or a
   webhook subscription, and an interaction stays `received` until `respondToInteraction`);
   `inactive` sets an interaction `failed` (compare-and-swap from `received`) and returns.
2. Re-check with the rights of this moment in filter mode, without audit: `command.invoke` with
   the descriptor of the invocation, or `interaction.create` at stage `deliver`. A person is the
   principal of a check without a request (`actor.Recheck`, R-AZ-29, which `authz` admits for
   `command.invoke` and `interaction.create` too); an agent or app that ran an app command is
   checked as itself, with its kind and installation from the args. A denial sends nothing, sets
   an interaction `failed` and logs the reason `denied`.
3. Interaction: compare-and-swap `received -> dispatched`; `dispatched` (a rerun) continues;
   `acknowledged`, `failed` or `rejected` (set by `respondToInteraction`) end the job without a
   request.
4. `Post` outside every transaction, with the thread of the args as `ReplyThreadID`.
5. Outcome to state (`domain/appowner.go`), each a compare-and-swap from `dispatched` in its own
   short transaction: answered gives `acknowledged`, deferred keeps `dispatched`, every reason
   gives `failed`; the component stays `used`. A command stores nothing; `apps` logs the outcome.
6. After the request the handler always returns nil: one request, no repetition (09 section 1
   point 5). A fault before the request (a database error, the keys; `Post` without
   `runs.ErrAfterRequest`) returns an error and is retried within
   River's default attempts, and a shutdown returns `ctx.Err()`; a repeated request carries the
   same `webhook-id`, which the app uses as its idempotency key.

Whichever of the job and `respondToInteraction` leaves `received` or `dispatched` first wins; the
job's final change starts only from `dispatched`. The outcome reaches the person only through the
stored state (spec gap reported).

**Autocompletion for app commands** (`app/app_autocomplete.go`). After `command.autocomplete` in
request mode (the app-owner guard as for `command.invoke`), the payload is built and the open
request registered as for agents, without a token. `Way`: `interactions_url` calls `Ask` with the
deadline the earlier of the end of the person's wait (`command.autocomplete_timeout`) and now plus
`app.interaction_timeout`; answered gives the choices through the compare-and-swap to `answered`,
deferred keeps waiting for `answerAutocomplete` or the op `command.autocomplete_result` until
`command.autocomplete_timeout`, every other outcome expires the request at once with an empty
list. `gateway` offers the event through `AutocompleteRelay.Ask` with an empty token to one live
connection of the app (the live phase of an app starts with `ready`; until the contract gap of
D-M2-25-24 is closed an app has no live connection, so this gives an empty list). `inactive` gives an empty
list at once. The use case holds no transaction, so the request to the app runs inside the
person's request; the person never waits longer than `command.autocomplete_timeout`. The app
answers with `command.autocomplete.respond` (app-self, `commands:write`).

**`DisableAuthoredBy`** (`Interactions.DisableAuthoredBy(ctx, authorID)`,
`app/components_authored.go`). `apps` calls it at `app.suspend` and `app.uninstall` inside its
unit of work; it calls no `authorize`, because the owner action was authorized. It reads the cards
with `active` components without a run (`run_id IS NULL`), keeps those whose author is `authorID`
(one batched `messaging.Messages.Authors`, which reads only the author and checks nothing), then
per card in ascending message id `LockCard`, the card's active components `FOR UPDATE` in
ascending id, each `disabled` with `version + 1`, and one `ComponentsChanged` (one
`message.updated`). It returns the number of disabled components. The locks follow the lock order
above, so a click racing it either wins or finds the component `disabled` (`409 component_used`).
`app.resume` does not bring the components back.

## Live box (M2-10)

The port `runs.Runs` gains `Patch` (REST `patchRun` in `adapters/httpapi/patch.go` and the gateway
op `run.update` call the same function) with the DTOs `PatchOp`, `PatchRun` and `PatchResult`;
`RunUpdated` is the payload of the ephemeral `run.updated`. The port `runs.LiveBox` serves the
delivery reads of the gateway. `app.Service` implements both; `Service.BindPublisher` binds the hub
as the `EphemeralPublisher` once the gateway is built (until then patches are stored and publish
nothing).

**Order of checks** (`app.Service.Patch`, one unit of work for steps 3 to 6; D-M2-10-1):

1. Load the run without a lock and authorize `run.update` on it (agent-self, `OwnerID` =
   `runs.agent_id`): an unknown or unreadable run gives `404 not_found`, another agent's run or a
   person or an app as actor `403 forbidden`; authz audits denials, an allowed patch writes no
   audit entry.
2. Static checks without the database (`domain.ParseOps`): every op through the path table and the
   value rules; `400 validation_failed` names every failing op in op order.
3. Lock the row (`LockRun`). A state other than `queued`, `running` or `waiting_input` gives
   `409 conflict` with `reason` `run_not_active`.
4. A `base_version` other than `runs.version` gives `409 base_version_conflict` with the document
   in `current` (the projection of `getRun`).
5. Dynamic checks against the locked state (`domain.Apply`); the first failure gives
   `400 validation_failed` naming that op and rolls back the whole patch.
6. Write in the lock order of 03 (`Repository.WritePatch`): one `UPDATE runs` with the changed
   columns, `version = version + 1` and `stall_warned_at = NULL` (D-M2-10-2), then the deletes,
   updates and inserts of `run_steps`; the follow-up states through `FollowupStates`. No
   `event_log` row, no `event_seq`, no `audit_seq`, no `NOTIFY`.
7. After the unit of work returned without error: publish `run.updated` (type, schema version 1,
   actor the agent, scope of the run's thread, resource `{run, id, new version}`, payload with the
   normalized ops) and count `bothy.run.steps` per step the adapter added (summary steps excluded).
   When the encoded payload reaches `sync.event_payload_max_bytes`, it is published without ops
   and with `Truncated`, and the gateway sends the whole document. A publish error is logged with
   the run id only and never reaches the adapter.

**Path table** (`domain/patch.go`, 09 5.3); everything else gives `patch_path_forbidden` at
`/ops/<i>`, a step or message id that is not a UUIDv7 `invalid_format` at `/ops/<i>/path`:

| Path | Ops | Stored in |
|---|---|---|
| `/status_line` | `add`, `replace`, `remove` | `runs.status_line` |
| `/last_text_excerpt` | `add`, `replace` | `runs.last_text_excerpt`, cut |
| `/model` | `replace` | `runs.model` |
| `/metrics/<field>` (`context_pct`, `tokens_in`, `tokens_out`, `tokens_cache_read`, `tokens_cache_write`, `tokens_cache_write_1h`, `subagents`, `background_jobs`, `api_retries`) | `add`, `replace` | `runs.metrics` |
| `/metrics/last_api_retry` | `add`, `replace`, `remove` | `runs.metrics` |
| `/cost` | `replace` | `runs.cost_micros`, `currency`, `cost_is_estimate` |
| `/plan` | `add`, `replace`, `remove` | `runs.plan` |
| `/steps/<step_id>` | `add` (no `ordinal`) | new `run_steps` row |
| `/steps/<step_id>/status`, `/title`, `/detail`, `/finished_at` | `replace` | `run_steps` column |
| `/followups/<message_id>/state` | `replace` | `run_followups.state` through `FollowupStates` |

Values follow D-M2-10-3 and D-M2-10-4: closed objects (`unknown_member`), no JSON `null`
(`invalid_type`), integers from 0 to 2^53 - 1, `context_pct` from 0 to 100, `is_estimate` must be
`true`, unique plan item ids, an `ordinal` in a step value is `patch_path_forbidden`; display texts
pass `kernel/textcheck.DisplayText` and give its rule name; `last_text_excerpt` and `detail` are
agent output and are only cut. No field error detail repeats a value.

**Set semantics** (`domain.Apply`, D-M2-10-5): `add` and `replace` set the member also when it is
absent, `remove` of an absent member is a no-op, and an empty text stores NULL like `remove`. A
step `add` with an id the run has gives `patch_target_exists`; a member `replace` on an unknown
step, a follow-up the run does not have or an unknown `parent_step_id` (not of the run and not
added earlier in the patch) gives `patch_target_missing`. The normalized ops are one per
effective change (`add` when the member was absent, `replace` when present, `remove` when cleared)
with the stored values as `getRun` encodes them; no-ops are dropped. Applied with RFC 6902 to the
document at `base_version` they give the document at the new version. One stored patch is one
version step even when every op is a no-op.

**Cutting** (`domain/truncate.go`, D-M2-10-6): `CutExcerpt` keeps the last `run.excerpt_max_chars`
runes of `last_text_excerpt`. `CutDetail` returns the canonical JSON of a detail when it fits
`run.step_detail_max_bytes`; otherwise it sets the top-level member `truncated: true` and shortens
the string leaves, the longest first, at a rune boundary; if it still does not fit, the stored
value is `{"truncated":true}`. Equal input gives equal output.

**Summary step** (`domain/steps_summary.go`, D-M2-10-7): after the step adds of a patch, while the
run has more than `run.max_steps_persisted` steps, the finished step (`completed`, `failed`,
`cancelled`) with the lowest ordinal that has no child and is not the summary step is removed and
counted into the summary step (kind `status`, name `steps_summarized`, empty title, status
`completed`, `detail` `{"summarized": {"count", "by_kind"}}`, a server UUIDv7, the lowest ordinal
removed so far). Running steps and parents stay, because `parent_step_id` cascades on delete. A
step added in the same patch may be removed too; its ops never reach clients. Without a candidate
the first step `add` gives `limit_reached`. The normalized ops carry `remove /steps/<id>` per
removed stored step and the `add` of the summary step or the `replace` of its `detail` (and of
its `ordinal` when it moves lower). `Repository.PatchSteps` reads the steps under the lock with the
detail of the summary steps only. Since M2-51 (09 5.3 addendum, R-DM-53, D-M2-51-3) there is one
summary step per parent: a removed step goes into the summary step whose `parent_step_id` equals
its own (the root counts as one parent), a new summary step carries that parent, and a finished
step whose only child is its own summary step is removed with it, both counts moving into the
summary of its parent, so finished subtrees collapse upward. Summary steps stored before stay at
the root. The first removal into a parent without a summary step frees no slot (the new summary
step takes it), so a run whose running steps sit under many parents reaches `limit_reached`
earlier than with one root summary.

**Progress and stall** (M2-51, 09 5.5): `runs.last_progress_at` is set by every successful start or
resume (`TransitionRun` with `started_jti`) and by every stored patch (`PatchRun`, together with
`stall_warned_at = NULL`); a patch that cleared a notice ends its normalized ops with
`remove /stall_warned_at`. After its orphan verdicts, `StallCheck` marks every `running` run whose
`last_progress_at` is older than `run.stall_after` and that has no notice: one unit of work per
run locks the row, compares the version it read (`MarkRunStalled`) and publishes `run.updated`
with `add /stall_warned_at` after the commit. `waiting_input` is never marked, and a run without
`last_progress_at` neither (no backfill, D-M2-51-2). `stall_warned_at` is part of `Run`,
`RunSummary` and `run.state_changed`.

**Delivery reads** (`app/livebox.go`): `LiveBox.Documents(threadIDs)` returns the documents of the
active runs of the threads, `LiveBox.Document(runID)` the document of one run while it is active.
They do not call `authorize`: the gateway checked `gateway.focus` per thread and admits every
recipient with `run.read` before it sends (07 7.4 rule 1, exception "loading a resource that has
already been checked"). Their queries are `runs.ActiveRunsOfThreads`, `runs.GetRun` and
`runs.AllSteps`, listed with `runs.PatchSteps` in `tools/lint/sql_visibility.yaml`. Each read joins
one unit of work, so a document is never torn by a concurrent patch.

**Follow-ups** (`app/followups.go`, D-M2-10-8): the outgoing port `FollowupStates` (`States`,
`SetState`) reads and writes the follow-up states a patch names. `Followups` implements it over
`run_followups` (M2-11, see "Follow-ups" below); `NoFollowups` remains only for a service without
a follow-up repository, where every follow-up op gives `patch_target_missing`.

**Configuration keys:** `run.excerpt_max_chars`, `run.step_detail_max_bytes` and
`run.max_steps_persisted` (registered in `internal/wire/configkeys_runs.go`),
`sync.event_payload_max_bytes`, `database.tx_retry_max` (the unit of work retries `40P01` and
`55P03`), and through the transports `http.max_json_body_bytes`, `sync.ws_max_frame_bytes` and
`ratelimit.rl.agent_runtime.per_minute` and `.burst`.

## Answers with `run_id` (M2-19)

- `Acceptor.OnMessagePosted` has a first branch for answers: an event with `RunID` and a `Kind`
  other than `run` must have `Origin = agent` (anything else is an internal error, because
  messaging never builds it); for a kind other than `system` it calls `checkAnswer`
  (`app/answer.go`). Afterwards every message whose origin is not human returns without trigger
  work, as before: no run, no follow-up, no `SetTriggerState`, no audit (AG-01, 07 SR4).
- `checkAnswer` reads the run with `Repository.LockRunForAnswer` (`queries/run_answer.sql`,
  `SELECT id, thread_id, agent_id, state ... FOR SHARE`) and decides with the pure
  `domain.AnswerVerdict`: unknown, another thread or another agent is `ErrAnswerRunMismatch`
  (`400 validation_failed`, `/run_id`, `run_mismatch`, one detail for every case, equal to the
  body rule of messaging); an own run in an end state is `ErrAnswerRunNotActive` (`409 conflict`,
  `reason` `run_not_active`). Ownership comes before the state, so `run_id` cannot probe runs
  outside the authorized thread (05 rule 1.2). There is no `authorize` call for the run: the
  message was authorized as `message.create` on its thread, and a run's path is its thread.
- A valid `run_id` changes nothing on the run (no `version`, no event, no step).
- Lock and order: messaging holds the thread row (`AppendToThread`), then the answer check takes
  the run row `FOR SHARE`, the same order as the acceptance (thread, then run). `finishRun`,
  `cancelRun`, the cancel timeout and the stall check update the run row under `FOR UPDATE`, so
  they wait for the answer's commit, or the answer waits for theirs and then sees the end state.
  Every answer stored with a `run_id` therefore commits while its run is active, and its
  `message.created` has a lower `seq` than the `run.state_changed` of the end. A deadlock with a
  path that locks the run first and a thread later is retried by the unit of work
  (`database.tx_retry_max`).
- `LockRunForAnswer` is an `after_authorize` exception of the SQL visibility lint
  (`tools/lint/sql_visibility.yaml`).

## Follow-ups (M2-11)

A human message that would trigger the agent of the thread's active run becomes a follow-up of
that run (09 section 7, AG-13). `app.Followups` (from `Service.Followups`) is the only code that
changes a follow-up state (D-M2-11-1); it joins the unit of work of its caller.

- **Acceptance** (`Followups.Accept`, inside the unit of `createMessage`): the acceptance reads
  the active run with its row lock (`LockActiveRunOfThread`) before `domain.Decide`. A decision
  `followup` authorizes `run.followup.create` for the author (a candidate without mention in
  filter mode first: a person without `trigger` gets nothing; then request mode, authz audits the
  denial). A denial sets `trigger_state = failed` with the reason of `domain.Settle`; an allow
  inserts the row `queued`, raises `runs.version`, sets `received` and appends `agent.followup`
  when `domain.FollowupEventNeeded` for the effective mode (D-M2-11-2).
- **Effective mode** (`domain.EffectiveFollowupMode`): the owner's `supports_followups`
  (`agents.RunFacts.FollowupsPermitted`, absent allows) and the adapter's persisted report
  (`FollowupsReported`, `steer` or `queue`); anything else is `none`. With `steer` the event goes
  out in every active state, with `queue` and `none` only to a run in `waiting_input`.
- **States** (`domain.FollowupState`): `queued`, `delivered`, `absorbed`, `deferred`,
  `discarded`. The adapter's patch (`FollowupStates.SetState`, M2-10's port) moves forward only
  (`domain.AdvanceFollowup`; 409 `invalid_state_transition` otherwise); the first move out of
  `queued` sets `followup_delivered` (D-M2-11-3).
- **Delivery** (`Prepare` with `agent.followup`): only an open row of this run whose message is not
  deleted is checked (`ForDelivery`); otherwise nothing is sent, without token or audit. A denial
  fails the message, discards the row and raises `runs.version` (`RefuseDelivery`, D-M2-11-4).
- **End of a run** (`Followups.OnRunEnded`, called by `endRun` after the transition; every end
  goes through `endRun`, D-M2-11-10): the open rows are locked in ascending `message_id` and split
  by `domain.PlanRunEnd`: deleted messages are discarded, after `user_cancelled` the rest are
  discarded with `cancelled` / `user_cancelled`, after every other end they become `deferred` in
  thread order (D-M2-11-5). The first candidate whose author passes `agent.invoke` (re-check
  principal, filter mode, pinned permission version, no active run, audit stage `run_end`)
  triggers the follow-on run through `Open` and stays `deferred`; the rest move to the new run as
  `queued` with `received` (D-M2-11-6).
- **Deleted message** (`Acceptor.OnMessageDeleted`, subscriber of `messaging.MessageDeleted`): an
  open follow-up becomes `discarded` and raises `runs.version`; the tombstone keeps its trigger
  state (D-M2-11-7).
- **Versions** (D-M2-11-8): acceptance, refused delivery and deletion raise `runs.version` without
  `run.updated`; the run document lists the follow-ups (`runs.Run.Followups`, REST keyed by
  message id).
- **Queries** (`adapters/postgres/queries/run_followups.sql`): every read names the rows of one
  run or one message whose run the use case authorized or changes in the same unit; they stand
  under `after_authorize` in `tools/lint/sql_visibility.yaml`.

## Hold of the agent invocation (M3-07)

A message that triggers an agent while one of its attachments is not final (12 section 2
"final") keeps `trigger_state = received` and gets no `agent.invocation` yet (04 15.3 point 1,
12 section 10, R-FI-09). `runs` reads the attachment facts only through the port
`files.Attachments` (`Deps.Files`; nil means nothing is ever pending).

- **Hold** (`app/hold.go`, `invokeOrHold`): the last step of the acceptance and of the follow-on
  run of M2-11. A message created with attachments (`MessagePosted.HasAttachments`) asks
  `Attachments.Pending`; a message without attachments costs no `files` call; the follow-on run
  asks unconditionally (it has no `MessagePosted`). Pending: `domain.HoldUntil` gives one clock
  reading plus `file.invocation_hold_max` (`voice.transcript_hold_max` when only a transcript is
  pending, M3-09), `Open` stores it in `runs.invocation_held_until`
  (R-DM-44), no `agent.invocation` is written and the job `runs.invocation_hold_timeout` is
  scheduled at that time (`Deps.Holds`). The release time is fixed at acceptance, so a later
  change of the key does not move held runs. Everything else of the acceptance (anchor,
  `run.state_changed`, `received`, `runs.queued_expiry`) is unchanged. Follow-ups
  (`agent.followup`) are never held.
- **Release** (`Service.ReleaseInvocation(ctx, runID, cause)`, inside the caller's unit of work):
  the run row is locked; for `domain.ReleaseFinal` `Pending` is read after the lock;
  `domain.ReleaseVerdict` releases only a `queued` run whose column is set, for `ReleaseFinal`
  only when nothing is pending. The release clears the column and raises `runs.version` by one
  (no `run.state_changed`), then appends `agent.invocation` with the facts of now (the scope of
  the run's thread, the agent's billing mode, the channel's `settings_version`, `triggered_by`)
  and without `actor` and `correlation_id` (`Deps.Uncorrelated` removes the trace span). It never
  authorizes (`Prepare` checks `agent.invoke` before the hand-over) and never touches
  `trigger_state`. The first release clears the column, so a run gets exactly one
  `agent.invocation`; a run that ended while held keeps its column and is never released. A
  release by `ReleaseTimeout` logs one Info line with `bothy.run.id` only; the line names both
  windows, `file.invocation_hold_max` and `voice.transcript_hold_max` (M3-09).
- **Subscriber** (`Acceptor.OnFileFinalized`, in-transaction subscriber of
  `files.FileFinalized`): `Attachments.MessageOf` of the file; for an attached file the queued
  held runs of its message are locked in ascending id and released with `ReleaseFinal`, so the
  final state of the last attachment, its `file.updated` and the `agent.invocation` commit
  together. Unbound or without `files` it does nothing.
- **Job** `runs.invocation_hold_timeout` (`adapters/jobs/hold.go`, args `InvocationHoldTimeout{RunID}`,
  queue `default`, `HoldScheduler` over `jobs.Inserter.InsertTx`; use case
  `Service.InvocationHoldTimeout`): in one unit of work, a run that is not queued or not held is
  nothing; before the stored release time the job schedules itself again at that time; else
  `ReleaseInvocation` with `ReleaseTimeout`. Repeated execution has no second effect.
- **Owner's switch `supports_files_out`** (09 section 2.2): the acceptance checks, before every
  other branch, a message of an agent with attachments: `agents.RunFacts.FilesOutPermitted`
  `false` returns `feature_disabled` (403), which vetoes the unit of work of the message; a
  missing key allows. The adapter's capability `files_out` is not evaluated here.
- **Queries** (`adapters/postgres/queries/hold.sql`): `LockHeldRunsOfMessage` (the held-run
  lookup, served by the partial index of the active runs) and `ReleaseHeldInvocation` (the
  release UPDATE); the run lock is `LockRun` of `delivery.sql`. The reads stand under
  `system_job` in `tools/lint/sql_visibility.yaml`.

## Voice messages (M3-09)

A voice message (`MessagePosted.Kind = voice`, 09 section 13, R-AG-16) is routed at acceptance:

- **Acceptance** (`app/accept.go`): `OnMessagePosted` takes `Origin = human` with `Kind` `text`
  or `voice`. For a voice message only, `MessageTrigger.Voice` is set and `VoiceMode` is the
  effective mode of the author from `files.Transcripts.Mode` (`Deps.Transcripts`, the narrow port
  `VoiceModes`; nil means `off`); `VoiceInput` of every `AgentFacts` is
  `domain.EffectiveVoiceInput` of the owner's `supports_voice` (`agents.RunFacts.VoicePermitted`,
  nil allows) and the reported `voice_input` (`VoiceReported`), taken from the one `RunFacts`
  read. A text message reads no mode, so its path costs no extra statement.
- **Rejection**: without a transcription path `domain.Decide` rejects with `voice_unsupported`;
  the acceptance stores `trigger_state = failed` with that reason, without `agent.invoke` in
  request mode and so without an audit entry (a rejection of the decision function, like
  `multiple_agents_mentioned`). A person without trigger who did not mention the agent stays
  unanswered (the filter check of R-AG-05 comes first). A voice message that triggers nobody keeps
  no trigger state.
- **Mode agent**: the invocation is written at once (the recording is final since the send); the
  adapter reads the file with `file.read` and stores the text with `submitTranscript`.
- **Mode service and the hold window**: `files` sets `transcript_status = pending` at the send, so
  `Attachments.Pending` answers `PendingTranscript` and `invokeOrHold` holds the invocation for
  `domain.HoldWindow(reason == PendingTranscript, file.invocation_hold_max,
  voice.transcript_hold_max)` (03 R-DM-44, R-DM-47). `ReleaseInvocation` with `ReleaseFinal`
  releases only on `PendingNone`; `ReleaseTimeout` releases with the transcript still pending,
  and its Info line names both keys. The follow-on run of M2-11 uses `invokeOrHold` as well;
  follow-ups (`agent.followup`) are not held.
- **Run context**: every element lists the attachments of its message (one
  `files.Attachments.ForMessages` call for the flagged messages that are still elements after the
  budget); `transcript` is present only while `transcript_status = done`. It sits inside its
  element and so carries the trust of its message (09 4.6, ADR 0054 point 4), while file contents
  stay marked data.
- **Configuration**: `voice.transcript_hold_max` (`app.Config.TranscriptHoldMax`, positive).
- Tests: `app/voice_test.go` (`TestVoice_TextMessageReadsNoMode`, `TestVoice_Routing`,
  `TestVoice_HoldWindowAndRelease`, `TestVoice_ConfigNeedsTranscriptHoldMax`),
  `app/runcontext_internal_test.go` (`TestContextAttachments_Transcript`), `domain/hold_test.go`
  (`TestHoldWindow`, `TestEffectiveVoiceInput`).

## Costs in the finish (M2-22)

`FinishRun.Costs` carries the reports of `RunFinish.costs` as `runs.CostReport` (a DTO of the port with the fields of `billing.CostReport`, because a port imports only `kernel`); `app` maps them to `billing.CostReport`. `billing` owns the price set, the computation and `cost_entries`; `runs` owns the order of the unit of work and the run columns ([10-backend-policy.md](../../../../docs/spec/10-backend-policy.md) 12.1).

1. Before the unit of work: `billing.Costs.ValidateReports`; an error is `400 validation_failed` with the pointers under `/costs` (`underCosts`) and changes nothing. A finish with costs in a world without billing (`Deps.Costs` nil) is internal.
2. In the unit, after `LockRun`, `authorize` and `FinishDecision`: a repeat of the same end state answers the run and books nothing (D-M2-22-5). A run without `billing_mode_used` and a non-empty `costs` is `400 validation_failed` (`invalid`, `/costs`) and rolls back.
3. One clock reading `at` is `finished_at` (through `TransitionStore.At` of `endRun`) and `occurred_at` of every entry (D-M2-22-6).
4. `book` passes the copies: `agent_id`, `thread_id`, `agent_session_id` and `billing_mode_used` of the run, `channel_id` and `space_id` from `chat.Threads.Info` now; `billing.Costs.BookRun` locks the price rows `FOR SHARE` and inserts the entries. Lock order: run row, price rows, session row.
5. Cost columns (`app.CostColumns` in `TransitionStore.Cost`): `cost_micros` is the sum in the currency of the first entry, `currency` that currency, `cost_is_estimate = true`, written in the update of the transition, so `version` rises once and the one `run.state_changed` carries the booked cost; other currencies stay only in `cost_entries` (D-M2-22-7). A finish without `costs` or with an empty list leaves the columns unchanged.
6. `addSessionCosts`: with a session, `agents.Sessions.AddCost` once per currency of the booking (D-M2-22-8).
7. `countCosts`: after the commit (`txn.AfterCommit`), never inside, the metrics port `CostBooked` records `bothy.run.cost` once per entry with a `BookedCost`: the value `amount_micros / 10^6` (unit `{currency_unit}`, a float counter) and the attributes `bothy.agent.handle`, `bothy.billing_mode`, `bothy.currency` (pattern source `SourceCurrencies`, D-M2-22-21) and `bothy.cost.estimate`; `adapters/otelmetrics` implements it.

Runs the server ends (`stale`, `interrupted`, `cancel_timeout`, `agent_disabled`, `access_revoked` without a later `finishRun` transition) book nothing (D-M2-22-9). The reported values are the consumption of this run; the session baseline is never subtracted (D-M2-22-10).

## Agent sessions (M2-12)

`agents` owns `agent_sessions` and every session rule; `runs` owns `runs.agent_session_id` and therefore the question whether a run used a row (09 section 9, 02 4.4). Every call joins the caller's unit of work after the run row (03 6.3: rows of `runs` before rows of `agent_sessions`).

- **Session mode** (`domain/session_mode.go`, `DecideSession`): a run of an agent that does not operate the thread is a guest run without session (09 3.6, R-AG-04); for the operator no row gives `new` without `agent_session`, a row with `started = false` gives `new` with the row (the adapter reuses its reference, REUSE L14), a started row some run already used gives `resume`, a started row no run used gives `bind` (rebinding, import binding). The operator is `ThreadInfo.OperatorAgentID` for a thread of kind `thread` and the channel's effective operator (`chat.TriggerChannel.EffectiveOperatorID`) for a main thread (03 R-DM-12, R-DM-21).
- **Payload:** `domain.InvocationPayload(in, session)` takes the decision as a required argument, so every writer of `agent.invocation` decides it; it refuses a missing mode, a guest decision with a row, and `resume` or `bind` without a row. An operator payload carries `agent_session` (the bytes of `agents.SessionJSON`, the schema `AgentSession`, omitted without a row) and `session_mode`; a guest payload carries neither.
- **Acceptance:** `emitInvocation` (the only writer of `agent.invocation`) calls `agents.Sessions.ForInvocation` for an operator run after `Open`, with the thread, the agent and the triggering person; "used" comes from `Repository.SessionUsed` (a run of the thread with `agent_session_id` = the row). Without `Deps.Sessions` every operator run gets `new` without a row (test worlds; `internal/wire` always sets it).
- **Held invocation (M3-07):** a held `agent.invocation` decides the session when it is released (`ReleaseInvocation` calls `emitInvocation`), with the run row already locked.
- **Start (`AttachOnStart`):** the `queued -> running` path of `startRun`, after the token checks, `authorize` and the compare-and-swap, calls `agents.Sessions.AttachOnStart` with the run, its thread and agent, `external_session_ref` (`""` when absent) and `model`; a returned row is stored with `Repository.SetRunSession` before the run document is read, so the answer shows `agent_session_id`. A validation error of the reference is returned unchanged (`400 validation_failed`, pointer `/external_session_ref`). `resumeRun` never calls it: the run keeps its row. Before the transition, `startRun` and `resumeRun` validate `model` through `agents.Sessions.CheckStartModel` (`400 validation_failed`, pointer `/model`). Without `Deps.Sessions` a reference is ignored.
- **Source `agents.RunSource`:** `adapters/postgres.RunSource` (`NewRunSource(pool)`) implements `ActiveRun(ctx, threadID)`: the run of the thread in `queued`, `running` or `waiting_input` (R-DM-14) with its row locked `FOR UPDATE` for the rest of the unit of work, so a report locks the run before its session row; outside a unit of work it is internal. `internal/wire` wires it into the agents use cases.
- **Subscriber `Acceptor.OnSessionAttached`** (In-Tx event `agents.SessionAttached`, registered after the other subscribers of runs): `SetRunSession(ev.RunID, ev.SessionID)` and, when `ev.BindFailed`, exactly one `messaging.Messages.AppendSystemLine` with the code `agent_session.resume_failed` and the run's agent as author; the line never reopens a closed thread (R-DM-42) and never triggers (SR4). Until `Bind` it does nothing.
- **Subscriber `Acceptor.OnCostsReconciled`** (In-Tx event `billing.CostsReconciled` of the job `billing.reconcile`, M2-28): in ascending `id` it sets `cost_micros`, `currency` and `cost_is_estimate` of each run whose values differ with `version + 1` (`Repository.SetCostTotal`, query `SetRunCostTotal`, unchanged rows are not touched), then hands the session totals to `agents.Sessions.ApplyCostTotals` (lock order runs before `agent_sessions`). No durable event: clients see the reconciled amounts on the next read. Until `Bind` it does nothing.
- **Repository:** `SetRunSession(ctx, runID, sessionID)` writes only `runs.agent_session_id` (no version change, no event; `not_found` for an unknown run); `SessionUsed(ctx, threadID, sessionID)` reads whether a run of the thread used the row (a row belongs to one thread, so the thread prefix of `runs_thread_queued_idx` serves it).

## Invocation tokens (M2-06)

`app.InvocationTokens` issues and verifies the invocation tokens of invariant I3 (09 4.5, 05 section 5, ADR 0023). `internal/wire` builds it once per process in the signing keys step (`Runs.BindInvocationTokens`) over `instance.Keys`, the instance row and the configuration keys `agent.invocation_token_ttl` and `auth.clock_leeway`. It stays inside the module: every consumer up to M2-17 is in `runs`; M2-14 adds a port for other modules (D-M2-06-1).

- **Header:** exactly `{"alg":"EdDSA","kid":<kid>,"typ":"agent-invocation+jwt"}`, signed with the `active` key of purpose `invocation` through `kernel/jose`. A signer of another algorithm is an internal error.
- **Claims per trigger kind** (`domain.NewInvocationClaims`): `iss`, `aud`, `sub`, `jti`, `iat`, `exp`, `trigger_kind`, `thread_id`, `channel_id`, `space_id`, `level`, `billing_mode`, `tool_class` and `permission_version` always; `run_id` with `message_id` for `message` and `followup`, with `interaction_id` for `interaction`, with `command_id` for `command`; only `command_id` for `autocomplete`. `dev_environment_id` is never written (V2-02). A grant that breaks this table, an unknown enum value, a zero ID or a negative `permission_version` is a programming error: `Issue` returns an internal error and signs nothing (D-M2-06-2). `iat` and `exp` are NumericDate seconds (`exp = iat + agent.invocation_token_ttl`, both truncated), `aud` is one string, absent claims are omitted (D-M2-06-3).
- **Issuing:** a new UUIDv7 `jti` per call; `iss` is loaded once per process from `instance.State` (a failed load is retried); the signer is read fresh on every call, so a promotion in another process applies to the next token. `Issue` starts no transaction and stores nothing; only the `jti` goes back to the caller for `runs.invocation_jti` (M2-08).
- **Verification order** (`Verify`, used by M2-09): header (`alg`, `typ` byte for byte, a `kid`), the published key of purpose `invocation` the `kid` names (`next`, `active` or `retiring`; another purpose, a retired or unknown key fail), signature, payload (members of the kind with their JSON types, member names exact, unknown members ignored), `iss`, `aud`, `run_id` when expected, `iat` not after now plus `auth.clock_leeway` and `exp` after `iat`, and last the expiry with `auth.clock_leeway` unless `IgnoreExpiry` (D-M2-06-4 to D-M2-06-8).
- **Errors:** `403 invocation_token_invalid` with `reason` `invalid` for every failed check, `expired` only when everything else holds. An unreadable key store is passed through as it is (no verdict about the token). The detail names the failed step, never the token.
- **Never stored or logged:** `app.InvocationToken` renders as `[redacted]` through `fmt`, `slog`, JSON and text; only `Reveal` returns the compact JWS, for the envelope (SEC-LOG-2, SEC-SEC-3). Logs and errors may name `kid` and `jti`, never the token or a segment of it.
- **Reference verifier:** `adapters/runstest.ReferenceVerifier` performs the adapter check of 09 4.5 with the standard library only (independent of `kernel/jose`), fetches the JWKS through a client the caller passes and honours its `max-age`, reloads once per `adapter.jwks_refresh_min_interval` for an unknown `kid` and reports the results in the precedence of D-M2-06-9 (`ErrRefInvalid`, `ErrRefUnknownKey`, `ErrRefBillingModeMismatch`, `ErrRefToolClassMismatch`, `ErrRefExpired`, `ErrRefReplay`). It never enters the binary. M2-08 and M2-09 check delivered tokens with it; the SDK verifiers of M2-29 and M2-30 must agree with it on the cases of the M2-06 tests.

## Acting-for resolution (M2-14, SR9)

An agent that calls the server API inside a run sends the run's invocation token in the header `Invocation-Token`; the authenticator of `identity` resolves it through `adapters/actingfor.Source` (only `kernel` types in the signature, so it implements `identity.ActingForSource` structurally; `internal/wire` holds the assertion) over the use case `app.ActingFor.ResolveActingFor` and the pure `domain.ActingForVerdict`:

- `InvocationTokens.Verify` with the agent of the request as audience, `IgnoreExpiry` and no run: every verdict (`invocation_token_invalid`) gives `invalid`. A failure of the key store or the instance row, and of the run read, is returned as an error (the authenticator answers 503, never an actor without the binding).
- Claims without a `run_id` (autocomplete, a command without an active run) give `invalid`: SR9 binds a run.
- `GetRunBinding(run_id)`: no row, another agent, `sub` not `triggered_by` (for `trigger_kind` `message` and `command` only; an interaction or follow-up token names the person who acted, D-M2-15-30) or the claim `thread_id` not the run's `thread_id` give `invalid`; `queued`, `running`, `waiting_input` give `bound`, `completed`, `failed`, `cancelled` give `run_inactive`, both with `UserID = triggered_by`, the run, its current thread and the `jti`; any other stored state gives `invalid` (fail closed). `invalid` carries zero IDs.
- Expiry and the currency of the `jti` are not checked: a run outlives `agent.invocation_token_ttl`, the binding always takes `UserID` from `triggered_by`, never from `sub`, and only narrows the agent's rights.
- Nothing is cached: a revocation or the end of a run counts on the next request in every process.
- Query exception: `GetRunBinding` reads one run by primary key without the visible-channel predicate, on the pool, outside a unit of work and without a lock, during the authentication of a request and before any `authorize`; nothing of the row leaves the server (07 section 7.4 rule 1, exception "resolution for authorize"; category `authorize_resolution` in `tools/lint/sql_visibility.yaml`). It is the port `app.RunBindings`, separate from `Repository`.

## Run context (M2-13)

- Port: `Runs.Context(ctx, runID)` returns `RunContext` (`ContextMessage`, `ContextAuthor`,
  `ContextAttachment`) for `getRunContext` (`run.context.read`, agent-self, `rl.agent_runtime`);
  `adapters/httpapi/runcontext.go` converts it to schema `RunContext` without JSON `null`.
- Use case `app/runcontext.go` (`Context`): `Repository.Run`, `authorize` with the run's agent as
  owner, the same `404` for unknown, invisible and inactive runs and for a triggering message the
  agent cannot read; then `chat.Threads.Info` (current channel, kind, parent message, fixed
  operator), `chat.TriggerFacts.TriggerChannel` (effective operator, `history_backfill_limit`),
  the session plan over `runs.agent_session_id` (`agents.RunSessions`), the windows through `messaging.Messages.ContextWindow` and `ContextMessage`,
  cutting and budget, one `authorize.TriggerHolders.WithTrigger` call for the trust marks and the
  delivered follow-ups. No unit of work, no lock; logs carry the run id at most.
- Pure rules in `domain/runcontext.go`: `Classify` (role and trust, origin before author),
  `PlanContext` (mode through `DecideSession`, floor, own messages, channel context),
  `CutContent`, `FitBudget`, `TruncatedBeforeSeq`.
- Collaborators in `app.Deps`: `ContextMessages` (messaging), `TriggerHolders` (authz through
  `kernel/authorize`), `RunSessions` (agents) and `ContextRows` (`adapters/postgres`,
  queries `EarlierSessionRun` and `DeliveredFollowupsOfSession` in `queries/runcontext.sql`);
  `internal/wire` takes the first three from the values it already wires and refuses one that does
  not implement its port.
- Configuration keys: `agent.context_max_bytes`, `agent.context_message_max_bytes`.
- Rulings: docs/decisions.d/M2-13.md.

## Visibility of `runs`, `run_steps` and `run_followups`

The tables have no `channel_id` (07 7.4 rule 1, D-M2-05-9): every query reads the rows of one thread or run whose visibility the use case checked first, or (the active-run lists) the active runs whose threads `chat.Threads.Locate` resolves, kept only when the channel is in `Scope.ChannelIDs` and `run.read` allows each row in filter mode (category `thread_scope` of `tools/lint/sql_visibility.yaml`); the gauge query has no principal. The run-context queries `EarlierSessionRun` and `DeliveredFollowupsOfSession` read rows of the one run, thread and session row whose `run.context.read` the use case authorized first (category `after_authorize`, M2-13).

## Dependencies

May use the ports of these modules: `instance`, `chat`, `agents`, `messaging`, `billing`, `files`.

Reason: sign the token; operator, mention rule, project binding; policy and session; anchor and `trigger_state`; post costs; write the held invocation on `files.FileFinalized` and read the attachment state (M3-07).

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`runs`, `run_steps`, `run_followups`, `message_components`, `interactions`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`agent.invoke`, `run.read`, `run.overview.read`, `run.context.read`, `run.start`, `run.token.renew`, `run.update`, `run.finish`, `run.cancel`, `run.followup.create`, `component.update`, `interaction.create`, `interaction.respond`, `command.autocomplete`, `command.autocomplete.respond`, `command.invoke`.

## Events and WebSocket ops

- Durable: `run.state_changed`, `agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`, `command.invoked`. Since M2-05 the first of them is written, since M2-08 also `agent.invocation`, since M2-16 `command.invoked`, since M2-09 `run.cancel_requested`, since M2-11 `agent.followup`, since M2-15 `interaction.created`; the others come with their tickets.
- Ephemeral: `run.updated`, `command.autocomplete_requested`.
- WebSocket ops (client to server): `run.update`, `command.autocomplete_result`.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `runs.cancel_timeout` | scheduled per cancel request, `agent.cancel_grace` (M2-09) | `maintenance` |
| `runs.stall_check` | periodic, `agent.presence_timeout` (M2-09); also the stall marking (M2-51) | `maintenance` |
| `runs.component_expiry` | scheduled per card message at its earliest `expires_at`, not periodic (M2-15) | `maintenance` |
| `runs.invocation_hold_timeout` | scheduled, `file.invocation_hold_max` (M3-07) or `voice.transcript_hold_max` | `default` |
| `runs.queued_expiry` | scheduled, `run.queued_max_age` (M2-08) | `default` |
| `runs.app_dispatch` | `InsertTx` on `command.invoked` and `interaction.created` for an app, one request, no retry after it (M2-25) | `default` |

## Notes for the build

- State transitions are compare-and-swap on `state` and `version` (03, `runs`).
- Live box patches are stored without `event_log` and go out as ephemeral `run.updated` to focused connections (04, 11).
- The invocation token is created anew at every delivery, after `agent.invoke` and I2 with current permissions; if the check fails, the run becomes `failed` (fail closed, 04, 15.3).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [09-agents.md](../../../../docs/spec/09-agents.md), [10-backend-policy.md](../../../../docs/spec/10-backend-policy.md)

## Statistics source (M6-05)

`runs.StatsSource` (`stats.go`, implemented by `app.Stats` over
`adapters/postgres/queries/stats.sql`) serves the statistics of `stats`: `RunStats` counts the ended
runs per bucket of `finished_at` by end state and `state_reason` and computes the duration
percentiles with `percentile_cont`; `ToolStats` counts the tool calls per tool name with failures
and the median duration. Because `runs` has no `channel_id` (D-M2-05-9), the caller passes the
threads of the visible channels; the queries stand in `tools/lint/sql_visibility.yaml` with the
category `system_aggregate`. The triggering person never leaves.

## Timeline source (M6-14)

`runs.TimelineSource` (`timeline.go`, implemented by `app.Stats` over the queries `TimelineRuns`
and `TimelineAgents` of `adapters/postgres/queries/stats.sql`) serves the timeline and the backend
view of `stats`: the runs of a thread set whose interval from `queued_at` to `finished_at` (now
while unfinished) intersects a range, ordered by `queued_at`, then ID, with a limit; and the
distinct agents of those runs. The caller passes the threads of the visible channels, as for the
statistics source; both queries stand in `tools/lint/sql_visibility.yaml` with the category
`system_aggregate`. Only interval, state and agent leave, never the triggering person or a title.

## Personal data (M7-06)

`adapters/postgres.PersonalData` (`NewPersonalData(pool)`) is the participant of runs in the privacy requests (`kernel/personaldata`, docs/spec/20-privacy.md sections 2, 4.4 and 5.2). Tables: `runs`, `run_steps`, `run_followups`, `message_components`, `interactions`.

- Export: `runs.jsonl`, one line per run the person triggered (`triggered_by`): ID, thread and agent ID, trigger kind, state, `queued_at`, `started_at`, `finished_at`, `cost_micros`, `currency`, `cost_is_estimate` and the numeric members of the metrics (`docs/api/gdpr-export/v1/lines/runs.schema.json`). Left out are the status line, the last text excerpt, the summary, the plan, the state reason, the last API retry and the run steps: the agent writes them from the whole thread, so they can carry contents of other people (Art. 15(4) GDPR). `runs` has no channel column and the participant reads no table of chat, so a run names only its thread ID. `interactions.jsonl`, one line per own interaction: ID, component and message ID, state, `created_at` and the submitted values, never the response of the app or agent (`lines/interactions.schema.json`).
- Erase: nothing in any step. The rows go with their thread or message (20 section 2); with `privacy.account_deletion_content = delete` the message deletion deletes the components of each deleted message, and their interactions follow by cascade.
- The two reads (`PersonalDataRuns`, `PersonalDataInteractions`) carry no visibility predicate: they run in the job `privacy.export` without a principal and are named by the person.
