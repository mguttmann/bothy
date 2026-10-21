# Modul `runs`

Status: the trigger decision as pure functions in `domain/` (M2-04); runs, the state machine, the acceptance of a triggering message and the run reads (M2-05). The invocation tokens and the reference verifier of the adapter check (M2-06). The delivery check `runs.Delivery` with the fresh token and the job `runs.queued_expiry` (M2-08). Start, renewal, end and cancellation of runs, the revocation and disabled-agent subscribers and the jobs `runs.cancel_timeout` and `runs.stall_check` (M2-09). Patches (M2-10) and follow-ups (M2-11) build on `app.Open`, `app.Transition`, the end-of-run function `endRun` and `app.Service.Prepare`.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Auslöse-Entscheidung, Läufe und Lauf-Dokument, Schritte, Nachträge, Komponenten, Interaktionen, Befehlsausführung, Aufruf-Marken.

## Code

- `internal/runs` (`runs.go`, M2-05): the port. `Runs` is the read side (`Get`, `ListByThread`, `ListSteps`, `ListActive`, `ActiveForActor`) with the DTOs `Run`, `RunSummary`, `RunStep`, `Money`, the pages and the queries; `Register` declares `run.state_changed`. No other module changes run states.
- `internal/runs/domain` (M2-04, M2-05): the trigger decision of [09-agenten.md](../../../../docs/spec/09-agenten.md) section 3 as pure functions. `trigger.go` holds the input types and `Decide`, `trigger_command.go` holds `DecideCommand`, `trigger_settle.go` holds `Verdict`, `Settlement` and `Settle`, `trigger_reasons.go` holds `Reason` (one constant per code of 09 3.5) and the constants of `Outcome`, `Step`, `Check`, `TranscriptSource` and `TriggerState`. M2-05 adds `state.go` (the six states, `Active`, `Ended`, `CanTransition` with the ten pairs of 03 `runs`), `reason.go` (the reason table of 09 5.2 plus `user_disabled` with state and setter, `ValidateServerReason`, `NormalizeAdapterReason`, `OriginalReasonDetail`) and `triggerstate.go` (`TriggerOnOpen`, `TriggerOnTransition`, the mapping of 09 6). The package imports only the standard library and `kernel-core` (`kernel/ids`, `kernel/errs`, `kernel/textcheck`); it has no I/O, no clock, no randomness and never calls `authorize`.
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

## Port (Wurzelpaket `internal/runs`)

- Incoming (M2-05): `Runs` (reads). M2-08: `Delivery` (`delivery.go`: `Prepare`, `DeliveryInput`, `DeliveryOutcome`, `InvocationBearing`), called by the agent consumer engine of `gateway` (WebSocket and REST poll). M2-09 adds start, resume, renewal, await, finish and cancel to `Runs`; M2-10 adds `Runs.Patch` and the delivery reads `LiveBox`. Planned: `Interactions` (M2-15), `CommandExec` (M2-16).
- Gibt dauerhafte Ereignisse aus (M2-08): `agent.invocation` (audience the agent, receive action `agent.events.consume`, resource the run without version, no token).
- Gibt In-Tx-Ereignisse aus: keine.
- Subscribes (M2-05): `messaging.MessagePosted` (the acceptance, `app.Acceptor`, registered after the subscriber of sync); M2-09: kernel `authorize.AccessChanged` and `agents.AgentDisabled`; M2-12: `agents.SessionAttached` (`Acceptor.OnSessionAttached`). Planned: `messaging.MessageDeleted`, `files.FileFinalized`, `chat.ChannelReadOnlySet`.
- Uses the ports `chat.Threads` (`Info`, `Locate`, `Create`), `chat.TriggerFacts`, `agents.Facts`, `agents.Sessions` (`ForInvocation`, `AttachOnStart`, `CheckStartModel`; M2-12) and `messaging.Messages` (`CreateRunAnchor`, `SetTriggerState`, `TriggerMessageOf`, `AppendSystemLine`).
- Implementiert Quellen-Ports und Querschnitts-Ports: `agents.RunSource` (M2-12, `adapters/postgres.RunSource`), `messaging.ComponentReader` (M2-15), kernel `personaldata.Participant` (M7-06).

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

## Run lifecycle after the delivery (M2-09)

The port `runs.Runs` gains `Start`, `Resume`, `RenewToken`, `AwaitInput`, `Finish` and `Cancel` (REST `startRun`, `resumeRun`, `renewInvocationToken`, `awaitRunInput`, `finishRun`, `cancelRun` in `adapters/httpapi`); the principal comes from the context. Every operation runs in one unit of work with the run row locked first (`LockRun`); several runs are locked in ascending id.

- **Token verdict** (`domain.StartVerdict`, `domain.RenewVerdict`): the token is verified through `InvocationTokens.Verify` without its expiry (any failure is `invalid`); then, first match wins, the start token of a running run is a replay, the start token of a waiting run is `superseded` (consumed), an ended or running run is `already_started`, a `jti` other than `invocation_jti` is `superseded`, now after `exp` plus `auth.clock_leeway` is `expired`, the wrong endpoint is `invalid_state_transition`. A renewal accepts the current `jti` or `renewed_from_jti` unless the start consumed it and never checks the expiry.
- **Token facts:** `authorize.TokenFacts` carries the verdict and, for a transition, the reported mode, the configured mode, `billing_evidence.key_type`, the reported tool class and the claim `tool_class`; `authz` decides `run.start` and `run.token.renew` on them (`invocation_token_invalid` with the verdict as problem reason; the guard gives `billing_mode_mismatch` and `tool_class_mismatch`). An I3 mismatch fails the run in the unit, which commits, and answers 409 afterwards; `bothy.agent.billing_mode_mismatch` counts it.
- **Start:** `Transition` with `TransitionStore` stores `started_jti`, clears `renewed_from_jti`, stores `billing_mode_used`, `billing_evidence` and `model`; since M2-12 the start attaches the agent session (see "Agent sessions (M2-12)"; ruling D-M2-09-6 is superseded).
- **Renewal:** steps 4 and 5 of 09 4.5.1 reuse the resolvers and the reason mapping of the delivery check: `agent.invoke` for `actor.Recheck(sub)` at the current permission version; a denial fails the run with the mapped reason, audits it with stage `renewal` and answers 403 `agent_invocation_denied`; an allow issues a fresh token and replaces `invocation_jti` through `RecordDelivery`, with `renewed_from_jti` set to the presented `jti` when it was current.
- **Wait and end:** `AwaitInput` moves `running -> waiting_input` with the status line. `Finish` applies `domain.FinishDecision` (the reason rules of D-M2-09-9; `costs` is `not_enabled` until M2-22); the same end state again changes nothing. Every end state goes through `endRun`, the one end-of-run function and the extension point of M2-11 and M2-15; an adapter's I3 mismatch writes one audit entry of `run.finish`.
- **Cancel:** `requestCancel` is the one function behind every cancellation (M2-16 `/stop` and M8-03 reuse it): it appends `run.cancel_requested` (personal to the agent, `requested_by` only for `user_cancelled`) and either ends the run at once or schedules `runs.cancel_timeout` at now plus `agent.cancel_grace` (`CancelTimeoutScheduler`, `adapters/jobs.CancelScheduler`). `Cancel` ends a queued run at once, requests otherwise, answers a cancelled run without change and audits the allow (`state_before`, `immediate`, `cancel_reason`, `no_change`); the reason text is checked and stored nowhere.
- **Revocation and disabled agent:** `Acceptor.OnAccessChanged` (phase 3.1, never calls `authorize`) locks the active runs of the affected starters, keeps those whose channel they lost (`domain.LostTriggerRuns`; every run for `user.disable`) and requests their cancellation with `access_revoked`, `user_disabled` or `channel_read_only` (`domain.CancelReasonFor`); the states stay. `Acceptor.OnAgentDisabled` ends the agent's active runs `cancelled` with `agent_disabled` at once.
- **Orphans:** `StallCheck` reads the started active runs and the persisted heartbeat of their agents (`agents.Heartbeats`) and fails `interrupted` and `stale` runs (`domain.OrphanVerdict`), one unit of work per run with the verdict decided again under the lock; nothing is retried.

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
detail of the summary step only.

**Delivery reads** (`app/livebox.go`): `LiveBox.Documents(threadIDs)` returns the documents of the
active runs of the threads, `LiveBox.Document(runID)` the document of one run while it is active.
They do not call `authorize`: the gateway checked `gateway.focus` per thread and admits every
recipient with `run.read` before it sends (07 7.4 rule 1, exception "loading a resource that has
already been checked"). Their queries are `runs.ActiveRunsOfThreads`, `runs.GetRun` and
`runs.AllSteps`, listed with `runs.PatchSteps` in `tools/lint/sql_visibility.yaml`. Each read joins
one unit of work, so a document is never torn by a concurrent patch.

**Follow-ups** (`app/followups.go`, D-M2-10-8): the outgoing port `FollowupStates` (`States`,
`SetState`) reads and writes the follow-up states a patch names. `internal/wire` binds
`NoFollowups` until M2-11 creates `run_followups`, so every follow-up op gives
`patch_target_missing`; M2-11 replaces it and adds the forward-only rule and
`trigger_state = followup_delivered`.

**Configuration keys:** `run.excerpt_max_chars`, `run.step_detail_max_bytes` and
`run.max_steps_persisted` (registered in `internal/wire/configkeys_runs.go`),
`sync.event_payload_max_bytes`, `database.tx_retry_max` (the unit of work retries `40P01` and
`55P03`), and through the transports `http.max_json_body_bytes`, `sync.ws_max_frame_bytes` and
`ratelimit.rl.agent_runtime.per_minute` and `.burst`.
## Agent sessions (M2-12)

`agents` owns `agent_sessions` and every session rule; `runs` owns `runs.agent_session_id` and therefore the question whether a run used a row (09 section 9, 02 4.4). Every call joins the caller's unit of work after the run row (03 6.3: rows of `runs` before rows of `agent_sessions`).

- **Session mode** (`domain/session_mode.go`, `DecideSession`): a run of an agent that does not operate the thread is a guest run without session (09 3.6, R-AG-04); for the operator no row gives `new` without `agent_session`, a row with `started = false` gives `new` with the row (the adapter reuses its reference, REUSE L14), a started row some run already used gives `resume`, a started row no run used gives `bind` (rebinding, import binding). The operator is `ThreadInfo.OperatorAgentID` for a thread of kind `thread` and the channel's effective operator (`chat.TriggerChannel.EffectiveOperatorID`) for a main thread (03 R-DM-12, R-DM-21).
- **Payload:** `domain.InvocationPayload(in, session)` takes the decision as a required argument, so every writer of `agent.invocation` decides it; it refuses a missing mode, a guest decision with a row, and `resume` or `bind` without a row. An operator payload carries `agent_session` (the bytes of `agents.SessionJSON`, the schema `AgentSession`, omitted without a row) and `session_mode`; a guest payload carries neither.
- **Acceptance:** `emitInvocation` (the only writer of `agent.invocation`) calls `agents.Sessions.ForInvocation` for an operator run after `Open`, with the thread, the agent and the triggering person; "used" comes from `Repository.SessionUsed` (a run of the thread with `agent_session_id` = the row). Without `Deps.Sessions` every operator run gets `new` without a row (test worlds; `internal/wire` always sets it).
- **Start (`AttachOnStart`):** the `queued -> running` path of `startRun`, after the token checks, `authorize` and the compare-and-swap, calls `agents.Sessions.AttachOnStart` with the run, its thread and agent, `external_session_ref` (`""` when absent) and `model`; a returned row is stored with `Repository.SetRunSession` before the run document is read, so the answer shows `agent_session_id`. A validation error of the reference is returned unchanged (`400 validation_failed`, pointer `/external_session_ref`). `resumeRun` never calls it: the run keeps its row. Before the transition, `startRun` and `resumeRun` validate `model` through `agents.Sessions.CheckStartModel` (`400 validation_failed`, pointer `/model`). Without `Deps.Sessions` a reference is ignored.
- **Source `agents.RunSource`:** `adapters/postgres.RunSource` (`NewRunSource(pool)`) implements `ActiveRun(ctx, threadID)`: the run of the thread in `queued`, `running` or `waiting_input` (R-DM-14) with its row locked `FOR UPDATE` for the rest of the unit of work, so a report locks the run before its session row; outside a unit of work it is internal. `internal/wire` wires it into the agents use cases.
- **Subscriber `Acceptor.OnSessionAttached`** (In-Tx event `agents.SessionAttached`, registered after the other subscribers of runs): `SetRunSession(ev.RunID, ev.SessionID)` and, when `ev.BindFailed`, exactly one `messaging.Messages.AppendSystemLine` with the code `agent_session.resume_failed` and the run's agent as author; the line never reopens a closed thread (R-DM-42) and never triggers (SR4). Until `Bind` it does nothing.
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
- `GetRunBinding(run_id)`: no row, another agent, `sub` not `triggered_by` or the claim `thread_id` not the run's `thread_id` give `invalid`; `queued`, `running`, `waiting_input` give `bound`, `completed`, `failed`, `cancelled` give `run_inactive`, both with `UserID = triggered_by`, the run, its current thread and the `jti`; any other stored state gives `invalid` (fail closed). `invalid` carries zero IDs.
- Expiry and the currency of the `jti` are not checked: a run outlives `agent.invocation_token_ttl`, every token of a run names the same person, and the binding only narrows the agent's rights.
- Nothing is cached: a revocation or the end of a run counts on the next request in every process.
- Query exception: `GetRunBinding` reads one run by primary key without the visible-channel predicate, on the pool, outside a unit of work and without a lock, during the authentication of a request and before any `authorize`; nothing of the row leaves the server (07 section 7.4 rule 1, exception "resolution for authorize"; category `authorize_resolution` in `tools/lint/sql_visibility.yaml`). It is the port `app.RunBindings`, separate from `Repository`.

## Visibility of `runs` and `run_steps`

The tables have no `channel_id` (07 7.4 rule 1, D-M2-05-9): every query reads the rows of one thread or run whose visibility the use case checked first, or (the active-run lists) the active runs whose threads `chat.Threads.Locate` resolves, kept only when the channel is in `Scope.ChannelIDs` and `run.read` allows each row in filter mode (category `thread_scope` of `tools/lint/sql_visibility.yaml`); the gauge query has no principal.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `chat`, `agents`, `messaging`, `billing`.

Grund: Marke signieren; Bediener, Erwähnungsregel, Projektbindung; Richtlinie und Sitzung; Anker und `trigger_state`; Kosten buchen.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`runs`, `run_steps`, `run_followups`, `message_components`, `interactions`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`agent.invoke`, `run.read`, `run.overview.read`, `run.context.read`, `run.start`, `run.token.renew`, `run.update`, `run.finish`, `run.cancel`, `run.followup.create`, `component.update`, `interaction.create`, `interaction.respond`, `command.autocomplete`, `command.autocomplete.respond`, `command.invoke`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `run.state_changed`, `agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`, `command.invoked`. Since M2-05 the first of them is written, since M2-08 also `agent.invocation`, since M2-09 `run.cancel_requested`; the others come with their tickets.
- Kurzlebig: `run.updated`, `command.autocomplete_requested`.
- WebSocket-Ops (Client an Server): `run.update`, `command.autocomplete_result`.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `runs.cancel_timeout` | terminiert je Abbruch-Anfrage, `agent.cancel_grace` (M2-09) | `maintenance` |
| `runs.stall_check` | periodisch, `agent.presence_timeout` (M2-09) | `maintenance` |
| `runs.component_expiry` | periodisch, `component.default_ttl` | `maintenance` |
| `runs.invocation_hold_timeout` | terminiert, `file.invocation_hold_max` bzw. `voice.transcript_hold_max` | `default` |
| `runs.queued_expiry` | terminiert, `run.queued_max_age` (M2-08) | `default` |

## Hinweise für den Bau

- Zustandsübergänge sind Compare-and-swap auf `state` und `version` (03, `runs`).
- Live-Box-Patches speichern ohne `event_log` und gehen kurzlebig als `run.updated` an fokussierte Verbindungen (04, 11).
- Die Aufruf-Marke entsteht bei jeder Zustellung neu, nach `agent.invoke` und I2 mit aktuellen Rechten; scheitert die Prüfung, wird der Lauf `failed` (fail closed, 04, 15.3).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [09-agenten.md](../../../../docs/spec/09-agenten.md), [10-backend-richtlinie.md](../../../../docs/spec/10-backend-richtlinie.md)
