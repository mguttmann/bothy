# Module `runs`

Status: skeleton (M1-05) and the trigger decision as pure functions in `domain/` (M2-04). Interfaces, use cases and tables come with the tickets of the milestone; M2-05 applies the decision.
The normative source is [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); where they differ, the chapter applies.

## Purpose

Trigger decision, runs and run document, steps, follow-ups, components, interactions, command execution, invocation tokens.

## Code

- `internal/runs` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `internal/runs/domain` (M2-04): the trigger decision of [09-agenten.md](../../../../docs/spec/09-agenten.md) section 3 as pure functions. `trigger.go` holds the input types and `Decide`, `trigger_command.go` holds `DecideCommand`, `trigger_settle.go` holds `Verdict`, `Settlement` and `Settle`, `trigger_reasons.go` holds `Reason` (one constant per code of 09 3.5) and the constants of `Outcome`, `Step`, `Check`, `TranscriptSource` and `TriggerState`. The package imports only the standard library and `kernel-core` (`kernel/ids`, `kernel/errs`); it has no I/O, no clock, no randomness and never calls `authorize`.
- `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

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

- Inbound: `Runs` (read, context, start, resume, patch, end, cancel, overview), `Delivery` (delivery preparation with I2, guard rules and token), `Interactions`, `CommandExec` (including autocompletion).
- Emits in-tx events: none.
- Subscribes to in-tx events: `messaging.MessagePosted`, `messaging.MessageDeleted`, `files.FileFinalized` (write the held-back invocation), `agents.AgentDisabled`, `chat.ChannelReadOnlySet`, kernel `authorize.AccessChanged`.
- Implements source ports and cross-cutting ports: `messaging.ComponentReader`, kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `instance`, `chat`, `agents`, `messaging`, `billing`.

Reason: sign the token; operator, mention rule, project binding; policy and session; anchor and `trigger_state`; post costs.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`. The domain layer imports only `kernel-core`.
Checked by go-arch-lint (ADR 0008).

## Tables

`runs`, `run_steps`, `run_followups`, `message_components`, `interactions`.

Only this module writes and reads these tables via SQL (03-datenmodell.md, principle 12).

## Actions (05-aktionen.md)

`agent.invoke`, `run.read`, `run.overview.read`, `run.context.read`, `run.start`, `run.update`, `run.finish`, `run.cancel`, `run.followup.create`, `component.update`, `interaction.create`, `interaction.respond`, `command.autocomplete`, `command.autocomplete.respond`, `command.invoke`.

## Events and WebSocket ops

- Durable: `run.state_changed`, `agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`, `command.invoked`.
- Ephemeral: `run.updated`, `command.autocomplete_requested`.
- WebSocket ops (client to server): `run.update`, `command.autocomplete_result`.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `runs.cancel_timeout` | periodic, `agent.cancel_grace` | `maintenance` |
| `runs.stall_check` | periodic, `run.stall_after` | `maintenance` |
| `runs.component_expiry` | periodic, `component.default_ttl` | `maintenance` |
| `runs.invocation_hold_timeout` | scheduled, `file.invocation_hold_max` or `voice.transcript_hold_max` | `default` |

## Notes for the build

- State transitions are compare-and-swap on `state` and `version` (03, `runs`).
- Live box patches are stored without `event_log` and go out as ephemeral `run.updated` to focused connections (04, 11).
- The invocation token is created anew at every delivery, after `agent.invoke` and I2 with current permissions; if the check fails, the run becomes `failed` (fail closed, 04, 15.3).

## Chapters

[02-architektur.md](../../../../docs/spec/02-architektur.md), [09-agenten.md](../../../../docs/spec/09-agenten.md), [10-backend-richtlinie.md](../../../../docs/spec/10-backend-richtlinie.md)
