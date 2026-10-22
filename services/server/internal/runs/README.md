# Module `runs`

Status: the trigger decision as pure functions in `domain/` (M2-04); runs, the state machine, the acceptance of a triggering message and the run reads (M2-05). The invocation tokens and the reference verifier of the adapter check (M2-06). The delivery check `runs.Delivery` with the fresh token and the job `runs.queued_expiry` (M2-08). Start, renewal, end and cancellation of runs, the revocation and disabled-agent subscribers and the jobs `runs.cancel_timeout` and `runs.stall_check` (M2-09). Patches (M2-10) and follow-ups (M2-11) build on `app.Open`, `app.Transition`, the end-of-run function `endRun` and `app.Service.Prepare`.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Trigger decision, runs and run document, steps, follow-ups, components, interactions, command execution, invocation tokens.

## Code

- `internal/runs` (`runs.go`, M2-05): the port. `Runs` is the read side (`Get`, `ListByThread`, `ListSteps`, `ListActive`, `ActiveForActor`) with the DTOs `Run`, `RunSummary`, `RunStep`, `Money`, the pages and the queries; `Register` declares `run.state_changed`. No other module changes run states.
- `internal/runs/domain` (M2-04, M2-05): the trigger decision of [09-agents.md](../../../../docs/spec/09-agents.md) section 3 as pure functions. `trigger.go` holds the input types and `Decide`, `trigger_command.go` holds `DecideCommand`, `trigger_settle.go` holds `Verdict`, `Settlement` and `Settle`, `trigger_reasons.go` holds `Reason` (one constant per code of 09 3.5) and the constants of `Outcome`, `Step`, `Check`, `TranscriptSource` and `TriggerState`. M2-05 adds `state.go` (the six states, `Active`, `Ended`, `CanTransition` with the ten pairs of 03 `runs`), `reason.go` (the reason table of 09 5.2 plus `user_disabled` with state and setter, `ValidateServerReason`, `NormalizeAdapterReason`, `OriginalReasonDetail`) and `triggerstate.go` (`TriggerOnOpen`, `TriggerOnTransition`, the mapping of 09 6). The package imports only the standard library and `kernel-core` (`kernel/ids`, `kernel/errs`, `kernel/textcheck`); it has no I/O, no clock, no randomness and never calls `authorize`.
- `internal/runs/app` (M2-05): `Service` with the lifecycle `Open` and `Transition`, the reads, `ActiveGroups` (the gauge), and `Acceptor`, the in-transaction subscriber of `messaging.MessagePosted`; `ports.go` holds `Repository` and its row types, `metrics.go` the outgoing `Metrics` port.
- M2-06: `domain/invocation.go` (claims, header constants, `ParseInvocationToken`, `CheckInvocationClaims`; it adds `kernel/jose` to the imports of the domain), `app/invocation.go` (`InvocationTokens` with `Issue` and `Verify`, the redacting `InvocationToken`) and `adapters/runstest` (the reference verifier, test support only).
- `internal/runs/adapters/postgres` (M2-05): the sqlc queries over `runs` and `run_steps` and `Repository`. `adapters/httpapi`: the four REST operations and the cursor codec. `adapters/otelmetrics`: the run metrics of signals.md 4.3 and the gauge `bothy.run.active`.

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

- Incoming (M2-05): `Runs` (reads). M2-08: `Delivery` (`delivery.go`: `Prepare`, `DeliveryInput`, `DeliveryOutcome`, `InvocationBearing`), called by the agent consumer engine of `gateway` (WebSocket and REST poll). Planned: start, resume, patch, finish, cancel (M2-09, M2-10), `Interactions` (M2-15), `CommandExec` (M2-16; `Autocomplete` and `AnswerAutocomplete` M2-17). Source port (M2-17): `AutocompleteRelay`.
- Emits durable events (M2-08): `agent.invocation` (audience the agent, receive action `agent.events.consume`, resource the run without version, no token).
- Emits in-tx events: none.
- Subscribes (M2-05): `messaging.MessagePosted` (the acceptance, `app.Acceptor`, registered after the subscriber of sync). Planned: `messaging.MessageDeleted`, `files.FileFinalized`, `agents.AgentDisabled`, `chat.ChannelReadOnlySet`, kernel `authorize.AccessChanged`.
- Uses the ports `chat.Threads` (`Info`, `Locate`, `Create`), `chat.TriggerFacts`, `agents.Facts` and `messaging.Messages` (`CreateRunAnchor`, `SetTriggerState`).
- Implements source ports and cross-cutting ports: `messaging.ComponentReader` (M2-15), kernel `personaldata.Participant` (M7-06).

## Run lifecycle (M2-05)

- `Open` (inside the caller's unit of work): `INSERT ... ON CONFLICT` on the partial unique index of the active run of a thread `DO NOTHING` (R-DM-14; `ActiveRunExistsError` otherwise, nothing written), the anchor through `messaging.CreateRunAnchor`, `run.state_changed`, then `trigger_state = received`.
- `Transition`: the pair must be one of 03 `runs` (`409 invalid_state_transition` otherwise), the reason one of 09 5.2 for its setter (an unknown adapter reason becomes `adapter_error` with a status step `adapter_reason`), one compare-and-swap `UPDATE` on `state` and `version` (`ConcurrentChangeError` with the current run when it lost, `not_found` for an unknown run), `run.state_changed`, then `trigger_state` per 09 6 through `messaging.SetTriggerState`. The metrics are recorded after the commit (`txn.AfterCommit`).
- Lock order: `runs` before `messages`; the acceptance holds the thread row of the message.
- Every state change, the creation included, appends `run.state_changed`: resource `run` with the version of the run row, the receive action of reading a run, payload `RunStateChangedPayload` (the summary, the triggering message and the trigger state this change set).
- Acceptance (`Acceptor.OnMessagePosted`): only `origin = human`, `kind = text`, no DM; facts through the ports; `Decide`; a candidate without mention is checked in filter mode first (no trigger right: nothing written, no audit); then `agent.invoke` in request mode, which audits denials; the auto thread only after `allow`; a disagreement of `Decide` and `authorize` fails closed. Until `Bind` the acceptor writes nothing (D-M2-05-19).

## Delivery (M2-08)

`app.Service.Prepare` implements `runs.Delivery` in one unit of work, immediately before every hand-over of an invocation-bearing event to an agent (04 15.3 step 2, invariant I2 "before every invocation"):

1. A type outside `InvocationBearing` is an internal error; a type or trigger kind without a resolver (`interaction.created`, `command.invoked` and the trigger kinds `command` and `interaction` until M2-15 and M2-16) gives `Send = false` and an `Error` log line (fail closed).
2. The run row is locked (`LockRun`, `SELECT ... FOR UPDATE`); a missing run or a run of another agent is an internal error.
3. The delivery matrix (`domain.DeliveryMatrix`): terminal runs are skipped without a check; `agent.invocation` records its `jti` only while `queued`; `agent.followup` and `interaction.created` record it only in `waiting_input`; `command.invoked` never.
4. A `queued` run at `queued_at + run.queued_max_age` or later fails with `agent_unavailable` without a check, a token or an audit entry.
5. The resolver reads the trigger: for `agent.invocation` with `trigger_kind = message` the run's trigger and its triggering message now (origin, mention of the agent, `NewThread` when the message lives in another thread than the run, the auto thread); for `agent.followup` the author of the follow-up message of the payload.
6. `agent.invoke` in filter mode for `actor.Recheck(trigger)` (R-AZ-29) at the permission version read once through `instance.Versions` and pinned, on the run's thread with `ActiveRunAgentID` = the run's agent.
7. Denial: the authorize reason maps to a run reason (`domain.DeliveryReason`: I2, SR2, project, read-only and `principal_inactive` keep their codes as `billing_mode_personal_not_holder`, `tool_class_owner_only`, `project_not_ready`, `channel_read_only`, `user_disabled`; `agent_not_reachable` of an inactive agent is `agent_disabled`; everything else `access_revoked`). `agent.invocation` of a queued run fails the run through `Transition` (which sets `trigger_state = failed`); `agent.followup` fails the follow-up message and leaves the run; a started run stays. One `auditing.Recorder.Record` in the unit: action `agent.invoke`, decision `deny`, the exact authorize reason, resource the thread, attributes `run_id`, `agent_id`, `stage` (`delivery`), `event_type`, `trigger_kind`, the re-check principal as actor. After the commit `bothy.agent.invocation.denied` counts it.
8. Allow: the token of `InvocationTokens.Issue` with the claims of `domain.DeliveryGrant` (`level = trigger`, the configured `billing_mode` and `tool_class`, the pinned `permission_version`); `RecordDelivery` stores `invocation_jti` (when recorded, with `renewed_from_jti = NULL`) and `billing_mode_expected` and raises `version` by one, only when a column changes; the first delivery of `agent.invocation` of a queued run moves `trigger_state` from `received` to `queued`. `Send = true` with the token, which is a delivery field only.

The span `agent.dispatch` (PRODUCER, `bothy.run.id`, `bothy.agent.handle`, `bothy.decision`) covers every checked delivery. The tokens are bound in the signing keys step (`BindTokens`).

Acceptance (M2-08 additions): `Open` schedules `runs.queued_expiry` at `queued_at + run.queued_max_age` in the creating transaction, and the acceptance appends `agent.invocation` as its last write (`domain.InvocationPayload`: IDs, `trigger_kind = message`, the author as `triggered_by`, the billing mode, `settings_version`; no content). The acceptance keeps the audit of its denials in request mode of `authz` (D-M2-08-8).

Job `runs.queued_expiry` (`adapters/jobs`, queue `default`, args the run only): `ExpireQueued` locks the run; still queued and due, it fails with `agent_unavailable`; queued but not yet due (the key was raised), it schedules itself again; otherwise nothing.

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

- **Token verdict** (`domain.StartVerdict`, `domain.RenewVerdict`): the token is verified through `InvocationTokens.Verify` without its expiry (any failure is `invalid`); then, first match wins, the start token of a running run is a replay, the start token of a waiting run is `superseded` (consumed), an ended or running run is `already_started`, a `jti` other than `invocation_jti` is `superseded`, now after `exp` plus `auth.clock_leeway` is `expired`, the wrong endpoint is `invalid_state_transition`. A renewal accepts the current `jti` or `renewed_from_jti` unless the start consumed it and never checks the expiry.
- **Token facts:** `authorize.TokenFacts` carries the verdict and, for a transition, the reported mode, the configured mode, `billing_evidence.key_type`, the reported tool class and the claim `tool_class`; `authz` decides `run.start` and `run.token.renew` on them (`invocation_token_invalid` with the verdict as problem reason; the guard gives `billing_mode_mismatch` and `tool_class_mismatch`). An I3 mismatch fails the run in the unit, which commits, and answers 409 afterwards; `bothy.agent.billing_mode_mismatch` counts it.
- **Start:** `Transition` with `TransitionStore` stores `started_jti`, clears `renewed_from_jti`, stores `billing_mode_used`, `billing_evidence` and `model`; `external_session_ref` is `not_enabled` until M2-12.
- **Renewal:** a token with `trigger_kind` message, followup or command (the `agent.invocation` of a command run, M2-16) is renewed; interaction follows with M2-15. Steps 4 and 5 of 09 4.5.1 reuse the resolvers and the reason mapping of the delivery check: `agent.invoke` for `actor.Recheck(sub)` at the current permission version; a denial fails the run with the mapped reason, audits it with stage `renewal` and answers 403 `agent_invocation_denied`; an allow issues a fresh token and replaces `invocation_jti` through `RecordDelivery`, with `renewed_from_jti` set to the presented `jti` when it was current.
- **Wait and end:** `AwaitInput` moves `running -> waiting_input` with the status line. `Finish` applies `domain.FinishDecision` (the reason rules of D-M2-09-9; `costs` is `not_enabled` until M2-22); the same end state again changes nothing. Every end state goes through `endRun`, the one end-of-run function and the extension point of M2-11 and M2-15; an adapter's I3 mismatch writes one audit entry of `run.finish`.
- **Cancel:** `requestCancel` is the one function behind every cancellation (M2-16 `/stop` and M8-03 reuse it): it appends `run.cancel_requested` (personal to the agent, `requested_by` only for `user_cancelled`) and either ends the run at once or schedules `runs.cancel_timeout` at now plus `agent.cancel_grace` (`CancelTimeoutScheduler`, `adapters/jobs.CancelScheduler`). `Cancel` ends a queued run at once, requests otherwise, answers a cancelled run without change and audits the allow (`state_before`, `immediate`, `cancel_reason`, `no_change`); the reason text is checked and stored nowhere.
- **Revocation and disabled agent:** `Acceptor.OnAccessChanged` (phase 3.1, never calls `authorize`) locks the active runs of the affected starters, keeps those whose channel they lost (`domain.LostTriggerRuns`; every run for `user.disable`) and requests their cancellation with `access_revoked`, `user_disabled` or `channel_read_only` (`domain.CancelReasonFor`); the states stay. `Acceptor.OnAgentDisabled` ends the agent's active runs `cancelled` with `agent_disabled` at once.
- **Orphans:** `StallCheck` reads the started active runs and the persisted heartbeat of their agents (`agents.Heartbeats`) and fails `interrupted` and `stale` runs (`domain.OrphanVerdict`), one unit of work per run with the verdict decided again under the lock; nothing is retried.

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
  at once. An app command is `404` until M2-25, because `Resolve` hides it (D-M2-16-20); the
  use case would answer an empty list for an app owner and the requirement already decides it. Otherwise the payload of `command.autocomplete_requested`
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

## Invocation tokens (M2-06)

`app.InvocationTokens` issues and verifies the invocation tokens of invariant I3 (09 4.5, 05 section 5, ADR 0023). `internal/wire` builds it once per process in the signing keys step (`Runs.BindInvocationTokens`) over `instance.Keys`, the instance row and the configuration keys `agent.invocation_token_ttl` and `auth.clock_leeway`. It stays inside the module: every consumer up to M2-17 is in `runs`; M2-14 adds a port for other modules (D-M2-06-1).

- **Header:** exactly `{"alg":"EdDSA","kid":<kid>,"typ":"agent-invocation+jwt"}`, signed with the `active` key of purpose `invocation` through `kernel/jose`. A signer of another algorithm is an internal error.
- **Claims per trigger kind** (`domain.NewInvocationClaims`): `iss`, `aud`, `sub`, `jti`, `iat`, `exp`, `trigger_kind`, `thread_id`, `channel_id`, `space_id`, `level`, `billing_mode`, `tool_class` and `permission_version` always; `run_id` with `message_id` for `message` and `followup`, with `interaction_id` for `interaction`, with `command_id` for `command`; only `command_id` for `autocomplete`. `dev_environment_id` is never written (V2-02). A grant that breaks this table, an unknown enum value, a zero ID or a negative `permission_version` is a programming error: `Issue` returns an internal error and signs nothing (D-M2-06-2). `iat` and `exp` are NumericDate seconds (`exp = iat + agent.invocation_token_ttl`, both truncated), `aud` is one string, absent claims are omitted (D-M2-06-3).
- **Issuing:** a new UUIDv7 `jti` per call; `iss` is loaded once per process from `instance.State` (a failed load is retried); the signer is read fresh on every call, so a promotion in another process applies to the next token. `Issue` starts no transaction and stores nothing; only the `jti` goes back to the caller for `runs.invocation_jti` (M2-08).
- **Verification order** (`Verify`, used by M2-09): header (`alg`, `typ` byte for byte, a `kid`), the published key of purpose `invocation` the `kid` names (`next`, `active` or `retiring`; another purpose, a retired or unknown key fail), signature, payload (members of the kind with their JSON types, member names exact, unknown members ignored), `iss`, `aud`, `run_id` when expected, `iat` not after now plus `auth.clock_leeway` and `exp` after `iat`, and last the expiry with `auth.clock_leeway` unless `IgnoreExpiry` (D-M2-06-4 to D-M2-06-8).
- **Errors:** `403 invocation_token_invalid` with `reason` `invalid` for every failed check, `expired` only when everything else holds. An unreadable key store is passed through as it is (no verdict about the token). The detail names the failed step, never the token.
- **Never stored or logged:** `app.InvocationToken` renders as `[redacted]` through `fmt`, `slog`, JSON and text; only `Reveal` returns the compact JWS, for the envelope (SEC-LOG-2, SEC-SEC-3). Logs and errors may name `kid` and `jti`, never the token or a segment of it.
- **Reference verifier:** `adapters/runstest.ReferenceVerifier` performs the adapter check of 09 4.5 with the standard library only (independent of `kernel/jose`), fetches the JWKS through a client the caller passes and honours its `max-age`, reloads once per `adapter.jwks_refresh_min_interval` for an unknown `kid` and reports the results in the precedence of D-M2-06-9 (`ErrRefInvalid`, `ErrRefUnknownKey`, `ErrRefBillingModeMismatch`, `ErrRefToolClassMismatch`, `ErrRefExpired`, `ErrRefReplay`). It never enters the binary. M2-08 and M2-09 check delivered tokens with it; the SDK verifiers of M2-29 and M2-30 must agree with it on the cases of the M2-06 tests.

## Visibility of `runs` and `run_steps`

The tables have no `channel_id` (07 7.4 rule 1, D-M2-05-9): every query reads the rows of one thread or run whose visibility the use case checked first, or (the active-run lists) the active runs whose threads `chat.Threads.Locate` resolves, kept only when the channel is in `Scope.ChannelIDs` and `run.read` allows each row in filter mode (category `thread_scope` of `tools/lint/sql_visibility.yaml`); the gauge query has no principal.

## Dependencies

May use the ports of these modules: `instance`, `chat`, `agents`, `messaging`, `billing`.

Reason: sign the token; operator, mention rule, project binding; policy and session; anchor and `trigger_state`; post costs.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`runs`, `run_steps`, `run_followups`, `message_components`, `interactions`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`agent.invoke`, `run.read`, `run.overview.read`, `run.context.read`, `run.start`, `run.token.renew`, `run.update`, `run.finish`, `run.cancel`, `run.followup.create`, `component.update`, `interaction.create`, `interaction.respond`, `command.autocomplete`, `command.autocomplete.respond`, `command.invoke`.

## Events and WebSocket ops

- Durable: `run.state_changed`, `agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`, `command.invoked`. Since M2-05 the first of them is written, since M2-08 also `agent.invocation`, since M2-16 `command.invoked`, since M2-09 `run.cancel_requested`; the others come with their tickets.
- Ephemeral: `run.updated`, `command.autocomplete_requested`.
- WebSocket ops (client to server): `run.update`, `command.autocomplete_result`.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `runs.cancel_timeout` | scheduled per cancel request, `agent.cancel_grace` (M2-09) | `maintenance` |
| `runs.stall_check` | periodic, `agent.presence_timeout` (M2-09) | `maintenance` |
| `runs.component_expiry` | periodic, `component.default_ttl` | `maintenance` |
| `runs.invocation_hold_timeout` | scheduled, `file.invocation_hold_max` or `voice.transcript_hold_max` | `default` |
| `runs.queued_expiry` | scheduled, `run.queued_max_age` (M2-08) | `default` |

## Notes for the build

- State transitions are compare-and-swap on `state` and `version` (03, `runs`).
- Live box patches are stored without `event_log` and go out as ephemeral `run.updated` to focused connections (04, 11).
- The invocation token is created anew at every delivery, after `agent.invoke` and I2 with current permissions; if the check fails, the run becomes `failed` (fail closed, 04, 15.3).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [09-agents.md](../../../../docs/spec/09-agents.md), [10-backend-policy.md](../../../../docs/spec/10-backend-policy.md)
