# M2-15: Components and interactions

Status: ready | Depends on: M1-57, M1-58, M2-09 | Milestone: M2
## Goal

After this ticket an agent posts a card (`kind = card`) with buttons, selects and forms, optionally bound to its own active run through `MessageCreate.run_id`; the components live in `message_components` of the module `runs`, appear in every representation of the message, and expire, get disabled at the end of their run and vanish with their message. A human with `trigger` operates a component through `createInteraction`: the server checks `agent.invoke` (I2, SR2), the component's state and expiry, `required_capability` and `restricted_to`, applies the single-use rule per message under row locks so that exactly one of two concurrent clicks wins, validates selected options and form values against the definition, stores the interaction idempotently by `client_interaction_id`, writes `interaction.created` for the owning agent (and starts a new run when no run waits), and updates the card for every reader through `message.updated`. The agent changes its own components with `updateComponent` and confirms an interaction with `respondToInteraction`. Approval cards (09 11.5) work end to end: "Always allow" only for owners, approval clicks are audited, and the token delivered with `interaction.created` resumes a waiting run. This is the function "Buttons, selects, forms on messages" of 01 and the server half of threat T-AI-05: the card header the clients draw comes only from fields the server sets (SEC-OUT-4), and no form value leaves the server outside the response to the person and the event to the agent (SEC-T10).

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope, Out of scope, Exit criteria points 1, 3 and 10, Test strategy, Red team with "another person triggering a personal subscription through every path (... interaction ...)" and "imitating a card header", Risks "prompt injection"); block "agents and extensions": rows M2-05, M2-08, M2-09, M2-11, M2-13, M2-14, M2-15, M2-16, M2-19, M2-20, M2-25, M2-31; M1 rows M1-56, M1-57, M1-58, M1-69, M1-70, M1-71; M4 row M4-21 (client half of SEC-T14).
- Dependencies: the header names M1-57 (message representation of reads), M1-58 (`message.updated` writer, `MessageDeleted`, `chat.Threads.LockForMessageChange`) and M2-09. Implied through M2-09 and therefore not in the header: M2-08 (`runs.Delivery`, trigger resolvers, acceptance), M2-07 (agent consumers), M2-06 (token issuer), M2-05 (`runs`, `Open`, `Transition`, the deferred FK of `runs.triggering_interaction_id`), M2-04, M2-02, M1-65, M1-64, M1-63, M1-61, M1-60, M1-56 (`createMessage`, `MessagePosted`), M1-55, M1-54, M1-53, M1-52, M1-51 (agents, `operator_config`), M1-49, M1-48, M1-46 (`kernel/designkeys`), M1-45, M1-44, M1-35, M1-32, M1-31, M1-29, M1-28 (`authorize`), M1-26 (`kernel/textcheck`), M1-25, M1-24 (audit), M1-20, M1-19, M1-18, M1-17, M1-16, M1-14, M1-13 and the gate tickets M1-01 to M1-08, and through M2-02 also M1-69 (`services/server/test/canary`) and M1-71 (`services/server/test/harness`, `BuildMatrixWorld`, contract tests). Design note 1 still checks M1-57, M1-58, M1-69 and M1-71 at the start and stops the build while one of them is missing. Used if merged: M1-70 (SQL visibility lint), M2-11 (follow-up acceptance, Design note 15 c).
- `docs/spec/09-agents.md`: 11 "Components" completely (11.1 "Kinds (`component.v1`)", 11.2 "Who may interact", 11.3 "Single-use rule per message", 11.4 "Flow of an interaction", 11.5 "Approval buttons", 11.6 "Questions"); 2.2 "Features" (`supports_components`); 4.4 "Events to the adapter" (row `interaction.created`); 4.5 "Invocation token" and 4.5.1; 4.6 "Run context"; 4.7 "Adapter operations" (rows "Question, approval", "Change components", "Answer interaction"); 5.1 "State machine"; 7 "Follow-ups during a run"; 8 "Stop" point 5; 16.2 "Required scenarios" (AG-15, AG-16, AG-31); 19 "Decisions of this chapter" (R-AG-01, R-AG-08, R-AG-11, R-AG-12); 21 "Configuration values of this chapter".
- `docs/spec/03-data-model.md` (core contract, read only): `message_components` (all columns, the paragraph "Single-use rule per message"), `interactions`, `runs` (`trigger_kind`, `triggering_interaction_id`, the checks on the `triggering_*` columns), `messages` (`kind = 'card'`, the check `(kind = 'run') = (run_id IS NOT NULL)`, the paragraph "Deletion empties ..."), `audit_events` (`details`), principle 8 "JSON", 6.3 "Lock order", 6.5 "Redaction on deletion".
- `docs/spec/04-sync-protocol.md` (core contract, read only): 2.2 "Envelope" (payload rules 1 to 5), 5.1 "Messages: `client_msg_id`" (last sentence: interactions use the same procedure), 9 "Delivery guarantee" point 3, 13.2 "Outbox", 15.3 "Invocation and run start" step 5, 15.4 "Follow-ups" point 5.
- `docs/spec/05-actions.md` (core contract, read only): 1 rules 1, 2, 6 (every trigger of an agent, also an interaction, checks `agent.invoke`) and 8; 2.2 "Target of a resource" (`component`, `interaction`); 2.3 "Effective level"; 2.6 "Scopes for apps" (`components:write`); 3 "Rate-limit classes" (`rl.interaction`); 4.10 rows `component.update`, `interaction.create`, `interaction.respond`, `agent.invoke`; 5 "Agent invocation `agent.invoke`" (points 1 to 8 and the paragraph on app components); 6 rows `message.updated`, `interaction.created`, `agent.invocation`; 8 R-AZ-01, R-AZ-04, R-AZ-12, R-AZ-18.
- `docs/spec/07-permissions.md`: 2.3 "Levels", 2.4 "Capabilities" (`approve_actions`), 4.1 "Decisions and reasons" (`component_inactive`, `component_restricted`, `capability_missing`, `run_active`, `run_active_other_agent`), 4.2 "Flow" (order, audit of denials, deferred entries), 4.3 "Visibility", SR9 (exception `interaction.respond`), 8.1 "Delivery filter", 8.2 "Immediate effect on revocation" point 6, 10 "Examples", 11.1 "Test matrix" rows AZ-M42, AZ-M43.
- `docs/spec/06-api.md`: 2 "Basic rules" (rows "No effect", "Unknown values", "No `null`"), 4 "Authorization in the API" points 3 and 6, 5 "Errors" (points 3, 4, 7 and the generated table: `component_used`, `invalid_state_transition`, `agent_invocation_denied`, `feature_disabled`), 7 "Idempotency" (row "Interaction", `x-idempotency-exempt` reason `client_id`), 8 "Concurrency" point 2 (components need `If-Match`), 16 endpoint tables "Interactions" and "Agent runtime".
- `docs/spec/02-architecture.md`: 4.1 "Rules" point 4 (source port, in-transaction event), 4.3 "Ports, events and sources per module" (`messaging` declares `ComponentReader`; `runs` port `Interactions`, implements `messaging.ComponentReader`, subscribes `messaging.MessageDeleted`), 4.4 "Allowed dependencies", 4.7 "Event types and WebSocket ops per module" (the sentence on `message.updated` produced by `messaging`), 12.1 "Usage", 12.3 "Job catalogue" (row `runs.component_expiry`), 12.4 "Rules for jobs".
- `docs/spec/19-security.md`: SEC-IN-1, SEC-IN-2, SEC-IN-4, SEC-IN-8, SEC-OUT-3, SEC-OUT-4, SEC-AI-4, SEC-AUD-5, SEC-LOG-4, SEC-T10, SEC-T14.
- `docs/spec/01-product.md`: 3.2 "Agents" (row "Buttons, selects, forms on messages"), 6.5 "Components".
- `docs/api/openapi.yaml`: operations `createInteraction` (`x-idempotency-exempt: client_id`), `updateComponent`, `respondToInteraction`, `createMessage`, `getMessage`, `listThreadMessages`, `resumeRun`; schemas `Component`, `ComponentCreate`, `ComponentUpdate`, `ComponentType`, `ComponentRestriction`, `Capability`, `InteractionCreate`, `Interaction`, `InteractionResponse`, `InteractionRespond`, `InteractionCreatedPayload`, `InteractionCreatedEvent`, `MessageCreate` (`kind`, `run_id`, `components`), `Message`, `MessageKind`, `MessagePayload`, `MessageUpdatedEvent`, `AgentInvocationPayload` (`trigger_kind`, `interaction_id`), `FieldError`, `Problem`; responses `BadRequest`, `Forbidden`, `NotFound`, `Conflict`, `PreconditionFailed`, `PreconditionRequired`; parameters `ComponentIdPath`, `InteractionIdPath`, `IfMatchRequired`, `IdempotencyKey`; headers `Idempotent-Replay`, `ETag`.
- `docs/api/asyncapi.yaml`: messages `event.interaction.created` (`x-receive-action: agent.events.consume`, `x-audience: self`), `event.message.updated`, `event.agent.invocation`. `docs/api/webhooks.md` 7 "Event types for apps and agents" (row `interaction.created`).
- ADRs 0013 (spec first), 0017, 0023 (invocation token), 0025, 0052 (one envelope), 0053 (no JSON `null`), 0054.
- `docs/DECISIONS.md`: D-AG-10 (single-use rule, "Always allow" only owners), D-K-D10, D-K-E9, D-W-29 (dash check with `tools/hooks/text_rules.py`), D-W-100; section 10 "Rulings of the build milestones" (row format `D-<Ticket>-<n>`).
- `docs/OPEN-QUESTIONS.md`: OQ-01 (default: strict I1 without owner exception; the I2 test of this ticket needs an I1-bypassing fixture because of it).
- `docs/security/threat-model.md`: T-AI-05 (approval obtained through cards), T-AI-03, T-PL-01, T-AI-02. `docs/RISKS.md`: RK-01.
- `docs/observability/privacy.md` 6 "Canary test (gate)" (form values are canary fields) and the table of contents that never go into telemetry ("form values of components"). No metric of signals.md belongs to this ticket.
- `docs/operations/configuration.md`: `component.default_ttl`, `component.max_per_message`, `component.custom_id_max_chars`, `component.form_max_fields`, `interaction.ack_timeout`, `sync.event_payload_max_bytes`, `message.max_content_bytes`, `http.max_json_body_bytes`, `ratelimit.rl.interaction.per_minute`, `ratelimit.rl.interaction.burst`, `database.tx_retry_max`, `audit.deny_aggregate_window`, `run.queued_max_age`, `agent.invocation_token_ttl`.
- `docs/design/icons.md` ("Rules": icons only through semantic names) and `design/tokens/icon.tokens.json` (`ds.icons.glyphs`).
- M1 briefs whose interfaces you use: M1-13 (`txn.UnitOfWork.Do`, `txn.InUnit`, `txn.Subscribe`, `txn.Publish`, `database.tx_retry_max`), M1-14 (`jobs.Inserter.InsertTx`, `InsertOptions.ScheduledAt`, `jobs.Register`), M1-16 (`eventlog.Appender`, `eventlog.Event`), M1-17 (`api/operations.yaml`, route generator), M1-18 (`StagePreconditions`, `FormatETag`, `ParseIfMatch`, problem writer with `errors[]` and `reason`), M1-20 (idempotency middleware), M1-24 (`auditing.Recorder.Record`, `audit_details.v1`), M1-25 (`authorize.Registry.Event`, `VerifyRegistrations`), M1-26 (`kernel/textcheck`), M1-28 (Design notes "Public contract in `kernel/authorize`", "Requirement table", "Deferred entries", "Guards", "Decision to error mapping"), M1-46 (`kernel/designkeys` generator), M1-51 (agents port, `operator_config`), M1-56 (D1 to D3, D7: flow, body rules, request hash, event form), M1-57 (message representation), M1-58 (`MessageDeleted`, `RedactPayload`, `LockForMessageChange`, `message.updated`), M1-69 (canary scenario), M1-71 (shared harness).
- M2 briefs this ticket extends (the briefs of M2-05, M2-08 and M2-09 in `docs/briefs/`): M2-05 Design notes 3 to 5 (`Open`, `OpenCmd`, `Transition`, anchor), M2-08 Design notes 1 to 9 and 14 (port `Delivery`, delivery matrix row `interaction.created`, resolver table, re-check principal, claims, audit at delivery, acceptance), M2-09 Design note 6 f (end-of-run function and its extension point) and the test `TestAG31_ConsumedStartToken`.
- Code (merged names win, D-W-100): `services/server/architecture/modules.yaml` on `main` (`runs` owns `message_components` and `interactions` and may use `instance`, `chat`, `agents`, `messaging`, `billing`), `services/server/internal/runs/README.md` (port `Interactions`, job `runs.component_expiry`), `internal/runs`, `internal/messaging`, `internal/chat`, `internal/agents`, `internal/authz`, `internal/kernel/{authorize,actor,auditing,eventlog,txn,textcheck,designkeys,errs,ids}`, `internal/platform/jobs`, `internal/wire`, `services/server/api/operations.yaml` (entries `createInteraction`, `updateComponent`, `respondToInteraction` under `later` with ticket M2-15), `services/server/test/harness`, `services/server/test/canary`.

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

11. Components of the types `button`, `select` and `form` with the definition fields of `component.v1`; at most `component.max_per_message` per message, `custom_id` up to `component.custom_id_max_chars`, forms up to `component.form_max_fields` fields; button styles only from the roles `primary`, `secondary`, `success`, `danger`, `ghost`, never a free colour; definitions checked with the limits and closed field sets of the schema (09 11.1, 03 `message_components`, SEC-IN-8, D-K-E9).
12. Submitted options and form values are checked against the definition: `400 validation_failed` with `errors[].code` `interaction_values_invalid` and the `pointer` on the field (09 11.1; plan row "form values checked against the definition").
13. Single-use rule per message: in one transaction `SELECT ... FROM message_components WHERE message_id = $m AND single_use AND state = 'active' ORDER BY id FOR UPDATE`; the chosen component, if present and not expired, becomes `used` (`used_by`, `used_at`, `version + 1`), every other locked row `disabled`; otherwise `409 component_used`; components without `single_use` stay active (03 `message_components`, 09 11.3, R-AG-11, AG-15).
14. `interaction.create` needs `trigger` on the channel and, for components of an agent, `agent.invoke` (I2 for personal agents, SR2), an active, unexpired component, the `required_capability` if set, and `restricted_to`: `any`; `run_starter` (the person who started the run, or an owner); `owner` (only owners) (09 11.2, 05 4.10 and 5 point 6, 07 2.4, AZ-M42, AZ-M43, AG-16).
15. A component with `required_capability = approve_actions` has `restricted_to = run_starter` unless the agent sets it explicitly (SEC-AI-4).
16. Approval cards are messages `kind = card` bound to the run, with the buttons of 09 11.5; "Always allow" (`restricted_to = owner`) is refused for every non-owner (09 11.5, R-AG-12, D-AG-10, AG-16).
17. The card header is built by clients from server fields only (author and origin of the message, the component's `run_id`, `required_capability`, `restricted_to`); an agent cannot set or change them through text, definition or update (SEC-OUT-4, SEC-OUT-3, T-AI-05, SEC-T14).
18. Interactions are idempotent by `(principal_id, client_interaction_id)`: a repeat answers `200` with the original and `Idempotent-Replay: true`, a different request with the same ID `422 idempotency_key_reuse` (04 5.1, 06 7, 03 `interactions`).
19. `interaction.create` is audited on allow when the component requires `approve_actions`; every denial is audited (05 4.10 column Audit, R-AZ-04, 07 4.2).
20. `interaction.created` goes only to the agent that owns the component (`audience`), with receive action `agent.events.consume` and without `resource.version`; before every delivery `agent.invoke` is checked again with current rights, and the delivery to a waiting run carries the token that resumes it (05 6, 04 15.3 step 5, 07 8.1, 09 5.1, AG-31).
21. A change of a component's state or definition is visible to every reader through `message.updated`, produced by `messaging` (05 6 row `message.updated`, 02 4.7).
22. `component.update` only by the author of the message (agent-self, app-self) with `If-Match` on the component version; `interaction.respond` only by the owner of the component, state only forward (05 4.10, 06 8 point 2, 09 4.7).
23. `MessageCreate.components` and `kind = card` are enabled; until this ticket they gave `400 validation_failed` with `errors[].code` `not_enabled` (M1-56 D2).
24. Deleting a message deletes its components (03 `messages`); the open components of a run are disabled when the run ends (09 8 point 5, 03 `message_components` index "for disabling at the end of the run"); expired components become `expired` (`component.default_ttl`, 02 12.3).
25. The canary scenario contains a form value; no form value appears in logs, traces, metrics, audit details, error responses or events to principals other than the agent that owns the component (19 SEC-T10, privacy.md 6, SEC-LOG-4, SEC-AUD-5).
26. No JSON `null` in answers and events (ADR 0053); rate-limit classes `rl.interaction` (`interaction.create`), `rl.write` (`component.update`), `rl.agent_runtime` (`interaction.respond`) (05 3, 4.10).

## Scope

All paths under `services/server/` unless they start with `docs/`, `design/` or `tools/`. Names of M2-05 to M2-09 below are the expected ones; where the merged code names them differently, follow the code (Boundaries). This ticket creates no new top-level Makefile target.

Create:

- `migrations/NNNNN_runs_message_components.sql` and `migrations/NNNNN_runs_interactions.sql` (the next free five-digit numbers, ADR 0007; Design note 3), with their lines in `migrations.sha256`.
- `internal/runs/interactions.go` (port): the interface `Interactions` (02 4.3) and its DTOs (Design note 2).
- `internal/runs/domain/components.go`: `component.v1` types per type, `CheckCard` (Design notes 4 and 5), defaults, the effective state at a time, the transitions of `updateComponent` (Design note 13), the canonical bytes of a card (Design note 6 c).
- `internal/runs/domain/values.go`: `CheckValues(definition, values)` per type (Design note 9).
- `internal/runs/domain/interaction.go`: the routing of an interaction to a run (Design note 10), the audit details builder (Design note 17), the payload size check.
- `internal/runs/app/interactions.go` (`Create`, `Respond`), `internal/runs/app/components.go` (`UpdateComponent`, `Check` and `Attach` behind the source port, `ForMessages`), `internal/runs/app/component_lifecycle.go` (`DisableRunComponents`, `OnMessageDeleted`, the expiry use case).
- `internal/runs/adapters/messagingsource/`: the implementation of `messaging.ComponentReader` and `messaging.ComponentWriter` on top of `runs/app` (02 4.1 point 4: the higher module implements the source port in an adapter).
- `internal/runs/adapters/postgres/queries/components.sql` and `queries/interactions.sql` with regenerated sqlc code.
- `internal/runs/adapters/jobs/component_expiry.go`: args `ComponentExpiry{MessageID ids.ID}` with kind `runs.component_expiry`, queue `maintenance`, and its handler (Design note 15 a).
- `internal/runs/adapters/httpapi`: the strict-server methods of `createInteraction`, `updateComponent`, `respondToInteraction`.
- Fuzz targets `FuzzCheckCard` and `FuzzCheckValues` next to the domain functions.
- Tests of the section "Tests".

Change:

- `internal/messaging` (port, domain, app, adapters): declare the source ports `ComponentReader` and `ComponentWriter` and the port methods `Messages.LockCard` and `Messages.ComponentsChanged` (Design note 2); the card rules and the flow of `createMessage` (Design note 6); the request hash version for cards (Design note 6 c); every `Message` representation (REST answers of `createMessage`, `getMessage`, `listThreadMessages`, `updateMessage`, payloads of `message.created` and `message.updated`) reads `components` through `ComponentReader` in one batched call (Design note 7).
- `internal/kernel/authorize`: `Resource.Interaction *InteractionFacts` (Design note 8 a) and, if not yet present, the rows of the decision mapping in Design note 8 d, each with a doc comment.
- `internal/authz`: the requirement of `interaction.create` instead of `Deferred` (and its id out of the golden list of deferred actions), point 6 of 05 5 in the `agent.invoke` guard, the mention rule and point 8 for interactions (Design note 8).
- `internal/runs/app` (code of M2-05, M2-08, M2-09): `OpenCmd.TriggeringInteractionID` and `trigger_kind = interaction` in `Open`; the resolvers of the delivery table for `interaction.created` and for `agent.invocation` with `trigger_kind = interaction`, and the payload builder of `agent.invocation` for that trigger kind (Design note 11); the extension point of the end-of-run function calls `DisableRunComponents` (Design note 15 b). Adapt `TestDelivery_UnregisteredResolverFailsClosed` of M2-08 so that it keeps proving the case for `command.invoked` only; never delete it.
- `internal/kernel/designkeys` and its generator (M1-46): the list of semantic icon names from `ds.icons.glyphs` of `design/build/tokens.resolved.json` next to `space-icons` (Design note 4).
- `internal/wire`: the source ports for `messaging`; subscription `messaging.MessageDeleted -> runs.OnMessageDeleted`; the job `runs.component_expiry` for the roles `worker` and `all`; registration of `interaction.created` for module `runs` through `authorize.Registry.Event`; the three handlers; regenerate the golden files of subscriptions, jobs and event registrations.
- `internal/platform/config` schema: `component.default_ttl`, `component.max_per_message`, `component.custom_id_max_chars`, `component.form_max_fields` only if absent (defaults from 03 and 09 21); regenerate `docs/operations/configuration.md` with `make config-reference` when the schema text changes.
- `api/operations.yaml`: move `createInteraction`, `updateComponent`, `respondToInteraction` from `later` to `enabled`; rerun the route generator (M1-17).
- `docs/api/openapi.yaml`: add `'409': $ref: '#/components/responses/Conflict'` to `updateComponent` and `respondToInteraction` (Design notes 12 and 13); descriptions of `MessageCreate.run_id` (cards bind their components to the run; other kinds stay `not_enabled` until M2-19), `MessageCreate.components`, `Component.definition` and `ComponentUpdate.definition` (name 09 11.1), `InteractionCreate.values` (shape per type, Design note 9). Then `make api-lint`, `make api-checks`, `make api-tables` and the server code generation.
- `docs/spec/09-agents.md` (in English, like the chapter): 11.1 (the definition rules of Design note 4, the value shapes of Design note 9), 11.2 (owner restriction reason, mention rule and `run_active` for interactions, the check at delivery, Design notes 8 b and 8 c), 11.3 (non-single-use components), 11.4 (dispatch state, events of a new run, Design notes 10 to 12), 11.6 (follow-up answer, Design note 15 c), 2.2 (the server enforces the owner's switch, Design note 6 b).
- `docs/spec/07-permissions.md` 4.1: rows `component_restricted` (also `restricted_to = owner`), `component_inactive` (answered as `409 component_used`), `run_active` (also an interaction on a component without run while the agent's run is active).
- `docs/spec/02-architecture.md`: 4.3 (rows `messaging` and `runs`: `ComponentWriter`), 4.7 (the sentence on `message.updated` also for component changes through `Messages.ComponentsChanged`), 12.3 (row `runs.component_expiry` scheduled per card, Design note 15 a).
- `services/server/test/canary`: `scenario.go`, `values.go` and `README.md` as Design note 18.
- READMEs of `runs`, `messaging`, `authz`, `kernel/designkeys` (if it has one) and `test/canary`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M2-15-<n>` (Design note 20).

## Out of scope

- Components and interactions of apps: delivery of `interaction.created` to an app, app components without `agent.invoke` and I2 (R-AZ-18), the scope `components:write` in use: M2-25. Until then a card from an app principal gives `400 validation_failed` with `errors[].code` `not_enabled` (Design note 6 a).
- The delivery path itself (`runs.Delivery.Prepare`, transports, token issuing, the delivery matrix): M2-08; this ticket only adds the resolvers. Run start, resume, renewal, `awaitRunInput`, `finishRun`, `cancelRun` and the end-of-run function: M2-09.
- Follow-ups (`run_followups`, `steer`, `queue`): M2-11; this ticket provides `DisableRunComponents` for the follow-up answer of 09 11.6 and wires it only if M2-11 is merged.
- Run context for runs started by an interaction (`run_context.v1` has only `trigger_message`): M2-13 (spec gap reported). SR9 (`acting_for`, the token in requests): M2-14.
- Slash commands and their cards: M2-16; autocomplete: M2-17. `MessageCreate.run_id` for kinds other than `card` (agent answers in a run): M2-19.
- The heartbeat half of the capability `components` (`reported_details.capabilities.components`): M2-20; this ticket enforces only the owner's switch `supports_components`.
- The client side of SEC-OUT-4 and SEC-T14 (header drawn from server fields, disabled components with a hint, "Expired"): M4-21 and M4-07.
- The display "not confirmed" after `interaction.ack_timeout` (09 11.4): no field and no event carries it to clients today; the key is not consumed (spec gap reported).
- Per-field length limits for labels, placeholders, option and choice lists beyond the keys of 09 21: not specified (spec gap reported); this ticket enforces the existing keys, the closed field sets and the event size (Design note 4).
- Redaction of `interaction.created` payloads when the card message is deleted: 03 6.5 covers only resource kind `message` (spec gap reported); the interaction rows are deleted with the components.
- `kernel/personaldata.Participant` for interactions (data subject requests): M7-06. Fencing over several nodes: M7-17. Conformance suite with the fake agent (AG-15, AG-16 data driven): M2-31. Reference adapters building cards: M2-33, M2-34.
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M2-15-components-interactions`; one writer per worktree.
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
- The same holds for the merged code of M2-05 to M2-09: follow its names; where it lacks a function this brief expects from it (for example the resolver table or the end-of-run extension point), add it in the module that owns it and report it.
- No agent runtime, no model and no third-party service in any test: agents are harness agents that call the REST API with their own tokens and receive events through the gateway test client of M1-64 or the REST poll of M2-07. Real runtimes run only behind an explicit switch (09 16.1), and this ticket has none.
- Test cards, labels and form values are canary values or neutral sentences in English; never a secret, never a real service name.
- Never log, print or persist a form value, a component label, an invocation token or a `jti` outside the tables and events the specification names, not even in test output; tests compare values in memory.
- Changes to `authz`, `messaging`, the M2-05 to M2-09 code of `runs` and the canary test stay within Design notes 6 to 8, 11, 15 and 18, each with a test; no other behaviour of an earlier ticket changes.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red` and the failing gate. Confirm that the base branch has: M2-09 (`startRun`, `resumeRun`, `awaitRunInput`, `finishRun` enabled, the end-of-run function), M2-08 (`runs.Delivery`, the resolver table), M2-05 (`runs` with the column `triggering_interaction_id` and no FK on it, `Open`), M1-57 (`getMessage`, `listThreadMessages`), M1-58 (`message.updated` from `updateMessage`, `MessageDeleted`, `chat.Threads.LockForMessageChange`), M1-69 (`services/server/test/canary`) and M1-71 (`services/server/test/harness` with `BuildMatrixWorld`). A missing one ends with `Verdict: BLOCKED dependency <ticket> missing`; M1-57 and M1-58 are named in the header, and M1-69 and M1-71 are in the dependency closure of this ticket through M2-09.

2. **Module boundaries and ports** (02 4.1, 4.3, 4.4, 4.7). `runs` owns `message_components` and `interactions`; `messaging` owns `messages` and the request of `createMessage`; `messaging` may not import `runs`. Therefore:
   a. `messaging` declares two source ports in its port package and `runs` implements both in `internal/runs/adapters/messagingsource`; `internal/wire` connects them. Pseudo-Go, follow the naming style of the merged port:

   ```go
   // ComponentInput is one member of MessageCreate.components as received; pointers mean "given".
   type ComponentInput struct {
       CustomID, Type     string
       Definition         json.RawMessage
       SingleUse          *bool
       RequiredCapability string     // "" when absent
       RestrictedTo       string     // "" when absent
       ExpiresAt          *time.Time
   }
   // CardInput is the card part of a createMessage request.
   type CardInput struct {
       AuthorID   ids.ID
       AuthorKind string  // user, agent, app
       ThreadID   ids.ID
       RunID      ids.ID  // MessageCreate.run_id; zero when absent
       Components []ComponentInput
   }
   // CheckedCard is a card that passed every check without database access.
   type CheckedCard struct {
       Canonical []byte    // input of the request hash (Design note 6 c)
       Input     CardInput
   }
   // ComponentWriter is declared by messaging and implemented by runs (02 4.1 point 4).
   type ComponentWriter interface {
       // Check validates the components of a card request without database access.
       Check(ctx context.Context, in CardInput) (CheckedCard, error)
       // Attach stores the components of a new card message in the ambient unit of work.
       Attach(ctx context.Context, messageID ids.ID, card CheckedCard) error
   }
   // Component mirrors the schema Component.
   type Component struct { /* every field of the schema, zero values for absent fields */ }
   // ComponentReader is declared by messaging and implemented by runs (02 4.3).
   type ComponentReader interface {
       // ForMessages returns the components of the given messages ordered by position;
       // it reads in the ambient transaction when there is one.
       ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]Component, error)
   }
   ```

   b. `messaging.Messages` gains `LockCard(ctx, messageID) (CardFacts, error)` (`SELECT ... FOR UPDATE` of the message row in the ambient unit of work; `CardFacts{MessageID, ThreadID, ChannelID, SpaceID, AuthorID ids.ID; AuthorKind, Kind string; Deleted bool; Version int64}`; a missing message gives `not_found`) and `ComponentsChanged(ctx, messageID) (int64, error)` (raises `messages.version` by one and appends `message.updated` with the representation of Design note 7; returns the new version). Reason: one event type has one producer (02 4.7: `message.updated` for `trigger_state` is produced by `messaging` through `SetTriggerState`; component changes follow the same path), and 05 6 names "component state" as content of `message.updated`, whose catalogue entry has `resource_version: true`, so the version must rise (03 `messages.version` "state change"). Ruling for docs/DECISIONS.md: component changes raise `messages.version`.
   c. Port `runs.Interactions` (02 4.3):

   ```go
   type Interactions interface {
       Create(ctx context.Context, cmd CreateInteraction) (InteractionResult, error)   // interaction.create
       Respond(ctx context.Context, cmd RespondInteraction) (Interaction, error)        // interaction.respond
       UpdateComponent(ctx context.Context, cmd UpdateComponent) (Component, error)     // component.update
   }
   type CreateInteraction  struct { ComponentID, ClientInteractionID ids.ID; Values json.RawMessage /* nil means {} */ }
   type InteractionResult  struct { Interaction Interaction; Replay bool }
   type RespondInteraction struct { InteractionID ids.ID; State string; ResponseText *string }
   type UpdateComponent    struct { ComponentID ids.ID; IfMatch int64; State *string; Definition json.RawMessage }
   ```

   The actor comes from `kernel/actor` the way the merged `runs` port passes it (M2-09 passes it as a parameter); `Interaction` and `Component` mirror the schemas. Ruling for docs/DECISIONS.md: `ComponentWriter` next to `ComponentReader`, because component definitions are content that `MessagePosted` never carries (M1-56) and a card must be rejected before its message exists; update 02 4.3.

3. **Migrations** (03 `message_components`, `interactions`; ADR 0007).
    - `message_components` exactly as 03: all columns with types, defaults and nullability; checks `type IN ('button','select','form')`, `restricted_to IN ('any','run_starter','owner')`, `state IN ('active','used','expired','disabled')`; FKs `message_id -> messages(id) ON DELETE CASCADE`, `run_id -> runs(id) ON DELETE SET NULL`, `used_by -> principals(id) ON DELETE RESTRICT`; unique `(message_id, custom_id)`; partial indexes `(expires_at) WHERE state = 'active'` and `(run_id) WHERE state = 'active'`. Retention class `content` (03).
    - `interactions` exactly as 03: checks `state IN ('received','dispatched','acknowledged','failed','rejected')`; FKs `component_id -> message_components(id) ON DELETE CASCADE`, `message_id -> messages(id) ON DELETE CASCADE`, `principal_id -> principals(id) ON DELETE RESTRICT`; unique `(principal_id, client_interaction_id)`; indexes `(component_id)` and `(state) WHERE state IN ('received','dispatched')`. Retention class `content`. Same file: `ALTER TABLE runs ADD CONSTRAINT runs_triggering_interaction_id_fkey FOREIGN KEY (triggering_interaction_id) REFERENCES interactions(id) ON DELETE SET NULL` (left out by M2-05).
    - A migration test proves every check, unique and FK, including that deleting a component cascades to its interactions and sets `runs.triggering_interaction_id` to NULL without breaking the `triggering_*` check of `runs`.

4. **`component.v1`** (09 11.1; Ruling for docs/DECISIONS.md for every point the chapter leaves open; write them into 09 11.1 in English). The definition is a JSON object with a closed field set per type (unknown keys rejected, SEC-IN-1); decode it into typed Go structs in `runs/domain`.
    - `button`: `label` (required string), `style` (one of `primary`, `secondary`, `success`, `danger`, `ghost`; default `secondary`), `icon` (optional; a semantic icon name from `ds.icons.glyphs`, read from the list in `kernel/designkeys`; an emoji, a glyph file name or a free name is rejected, docs/design/icons.md "Rules"), `value` (required string).
    - `select`: `placeholder` (optional string), `options` (required, at least one; each `label` required, `value` required and unique within the select, `description` optional), `min_values` and `max_values` (optional integers; defaults `1` and `1`; `0 <= min_values <= max_values`, `max_values >= 1`, `max_values` at most the number of options).
    - `form`: `title` (required string), `submit_label` (optional string), `fields` (required, at least one, at most `component.form_max_fields`; each `name` required, matching `^[a-z][a-z0-9_]*$` and unique within the form, `label` required, `type` one of `text`, `textarea`, `number`, `boolean`, `choice`, `required` boolean (default false), `max_length` optional integer at least one and only for `text` and `textarea`, `choices` required for `choice` and only there: a non-empty list of unique strings).
    - Every display text of a definition (`label`, `placeholder`, option `label` and `description`, `title`, `submit_label`, field `label`, every entry of `choices`) and every `value` passes `kernel/textcheck` (no control or bidi characters); reason: these texts are drawn as controls next to the server header, and SEC-IN-4 rejects such characters in display texts (T-AI-05). `custom_id` passes `kernel/textcheck` too, is not empty and has at most `component.custom_id_max_chars` characters.
    - No per-field length limit exists in the specification besides the keys above (spec gap reported); the whole card is bounded by the event size (Design note 6 a). Do not invent a number or a key.
    - Errors are `400 validation_failed` with one `errors[]` entry per finding, `pointer` below `/components/<index>/...`, and `code` `required` (missing), `too_long` (`custom_id` above its key, a card event above `sync.event_payload_max_bytes`), `limit_reached` (more than `component.max_per_message` components or `component.form_max_fields` fields) or `invalid` for every other rule (unknown key, wrong JSON type, unknown style or icon, duplicate `custom_id`, option value or field name, bad bounds, rejected characters). `detail` names the rule, never the offending text. Ruling for docs/DECISIONS.md: the generic code `invalid` next to the codes 06 5 point 3 names.

5. **Component fields set by the server** (03 `message_components`, SEC-AI-4). `position` = index in the request plus one; `single_use` default `true`; `required_capability` one of `approve_actions`, `upload`, `export`, `manage` (`terminal` exists only on the target `host`, 07 2.4, and gives `invalid`); `restricted_to` default `run_starter` when `required_capability = approve_actions`, else `any`; `expires_at` = the given time if it lies after the request time (else `invalid`), otherwise request time plus `component.default_ttl`; `run_id` = `MessageCreate.run_id` (Design note 6 b); `state = active`; `version = 1`; `used_by`, `used_at` empty. `ComponentCreate` has no member for `position`, `run_id`, `state`, `used_by` or `version`; the decoder rejects them as unknown members.

6. **`createMessage` with a card** (M1-56 D1 and D2 extended; 09 4.7 row "Question, approval"; 09 11.5; SEC-OUT-4).
   a. Body rules after the decoder: `components` non-empty requires `kind = card` (else `400`, pointer `/kind`, code `invalid`); `kind = card` requires at least one component (pointer `/components`, `required`); only principals of kind `agent` send cards (`user`: pointer `/components` or `/kind`, code `invalid`; `app`: code `not_enabled` until M2-25); `run_id` is accepted only with `kind = card` and keeps `not_enabled` for every other kind (M2-19); `content_md` keeps the M1-56 rule (non-empty), because an approval must describe its effect in the agent's text (T-AI-05). Then `ComponentWriter.Check` (Design notes 4 and 5). Finally the size rule: build the `message.created` payload of the card with all components; if it exceeds `sync.event_payload_max_bytes` even after the content cut of M1-56 D7, answer `400` with pointer `/components` and code `too_long` (04 2.2 rule 2: the appender rejects instead of cutting, and components cannot be cut).
   b. In the unit of work, after the replay lookup of M1-56 D1 and after the message row is inserted, `ComponentWriter.Attach`: lock the run of `run_id` with `FOR SHARE`; it must exist, have `runs.agent_id` equal to the author, `runs.thread_id` equal to the message's thread and a state in `queued`, `running`, `waiting_input`, else `400` with pointer `/run_id` and code `invalid` (one code for every case, so nothing about other agents' runs leaks); read the author's `agents.operator_config` through the `agents` port: `supports_components = false` gives `403 feature_disabled` (09 2.2: the owner switches the capability off; R-AG-01; Ruling for docs/DECISIONS.md: the server enforces the owner's part, the adapter's report is M2-20); insert the rows of Design note 5; schedule `runs.component_expiry` (Design note 15 a). The share lock makes a concurrent end of the run either come first (the card is rejected) or see the new components (Design note 15 b). `messages.run_id` stays NULL: 03 allows it only for `kind = 'run'`; the run of a card lives in `message_components.run_id` (Ruling for docs/DECISIONS.md; spec gap reported).
   c. Request hash (M1-56 D3: "Changing the encoding later is a new version prefix, never an edit"): requests with neither `components` nor `run_id` keep `bothy-msg-v1` byte for byte; requests with either use `bothy-msg-v2`: the v1 fields in v1 order, then `run_id` (empty when absent), then `CheckedCard.Canonical`, each as `uvarint(len) || bytes`. `Canonical` is built from the typed, validated form as received (defaults not applied): per component in request order `custom_id`, `type`, the definition re-encoded from its typed struct (fixed field order, so key order and white space of the request do not matter), `single_use` (empty, `true`, `false`), `required_capability`, `restricted_to`, `expires_at` (RFC 3339 UTC with milliseconds, or empty). A replay of a message stored before this ticket still computes v1. Ruling for docs/DECISIONS.md; M2-19 reuses v2 for `run_id` on answers. Spec gap reported: 04 5.1 point 3 does not list components and `run_id`.
   d. Order inside the unit of work: thread sequence and message insert (M1-56), `Attach`, `MessagePosted` (the acceptance of M2-08 ignores `origin = agent`), `message.created` with the components, commit. `201` with `Message` including `components`; a replay `200` with the current components.

7. **Message representation** (05 6, `Message.components` is required). Every builder of `Message` in `messaging` (REST and event form, M1-56 D7, M1-57, M1-58) fills `components` from `ComponentReader.ForMessages` with one call per answer or page, never one per message; tombstones keep `components: []` (M1-58 `RedactPayload`, 04 2.2 rule 5). `Component` contains no form values (they live only in `interactions`), so `message.*` events never carry them. The components query reads rows of messages that `authorize` already checked or that `messaging` loaded for an event of the same transaction; if the SQL visibility lint of M1-70 is merged, list the queries of this ticket in its exception list with this reason (07 7.4 rule 1).

8. **Authorization** (07 4.2: all rights logic in `authz`; 05 5 point 6).
   a. `kernel/authorize`: add to `Resource`

   ```go
   // InteractionFacts carries the component of interaction.create, and of the re-check before a
   // delivery that an interaction caused (05 5 point 6, 04 15.3 step 2).
   type InteractionFacts struct {
       Stage              string    // "accept" (interaction.create) or "deliver" (runs.Delivery)
       OwnerKind          string    // kind of the card's author: agent or app
       ComponentState     string    // active, used, expired, disabled, from the locked row
       ExpiresAt          time.Time
       RequiredCapability string    // "" when none
       RestrictedTo       string    // any, run_starter, owner
       ComponentRunID     ids.ID    // message_components.run_id; zero without run
       ComponentRunActive bool      // that run is queued, running or waiting_input
       RunStarterID       ids.ID    // runs.triggered_by of that run; zero without run
   }
   ```

   b. `interaction.create` (catalogue: resource `component`, right `trigger`): requirement `Level(trigger)` instead of `Deferred`, plus the guard. Descriptor: `{Kind: component, ID, ThreadID, OwnerID: card author, Invoke: &InvokeFacts{AgentID: card author, TriggerOrigin: "human", ActiveRunAgentID: agent of the thread's active run or zero}, Interaction: &InteractionFacts{Stage: "accept", ...}}`. `OwnerKind = app` is an internal error until M2-25 replaces it (apps cannot post cards before, note 6 a). For `OwnerKind = agent` the guard is the `agent.invoke` guard of M1-28 with these additions, in this order: points 1 to 5 unchanged except that the mention rule of point 3 counts as met when `Interaction` is set (the card is the agent's own address to the person; a guest agent still needs `allow_mentioned_agents`, its own `write` and `organization`); then point 6: at stage `accept` a state other than `active`, an `ExpiresAt` at or before the request time, or `ComponentRunID` set with `ComponentRunActive` false gives `component_inactive`; a missing `RequiredCapability` among the effective capabilities on the path gives `capability_missing`; `restricted_to = run_starter` with the principal neither `RunStarterID` nor effective owner, and `restricted_to = owner` with the principal not effective owner, give `component_restricted`; then point 7 unchanged; then point 8: `ActiveRunAgentID` set and not the card's agent gives `run_active_other_agent`; at stage `accept` `ComponentRunID` zero and `ActiveRunAgentID` equal to the card's agent gives `run_active`. Rulings for docs/DECISIONS.md: the mention rule for interactions; `run_active` for a run-less component while the agent's run is active (as a model command, 05 `command.invoke`; a second run cannot exist, R-DM-14, and a follow-up is a message, 05 5 point 8); a component of an ended run is inactive.
   c. Stage `deliver` (the re-check of Design note 11) evaluates `RequiredCapability` and `RestrictedTo` with the rights of that moment and skips the state check and the `run_active` rule: the component is `used` by this very interaction and the run is the one the event goes to. Ruling for docs/DECISIONS.md.
   d. Decision to HTTP (M1-28 "Decision to error mapping" extended; 06 4 points 3 and 6; 06 5):

   | `authorize` outcome and reason | Answer |
   |---|---|
   | `not_found` (any reason) | `404 not_found`, identical to an unknown component |
   | `forbidden` `level_insufficient`, `capability_missing`, `component_restricted`, `subject_kind_not_allowed`, `not_self`, `view_as_read_only` | `403 forbidden` with `reason` |
   | `forbidden` `channel_read_only`, `channel_archived` | `403 channel_read_only` (with `reason` for archived) |
   | `forbidden` `component_inactive` | `409 component_used` |
   | `forbidden` `origin_not_human`, `agent_not_reachable`, `billing_mode_personal_not_holder`, `tool_class_owner_only`, `project_not_ready`, `run_active`, `run_active_other_agent` | `403 agent_invocation_denied` with `reason` |

   Reuse the `agent_invocation_denied` row if M2-09 added it for `renewInvocationToken`. Ruling for docs/DECISIONS.md: `component_inactive` keeps its reason in the audit and is answered `409 component_used`, so a lost race answers the same whether it is decided by `authorize` or by the lock (03, 06 5); update 07 4.1.
   e. `component.update` and `interaction.respond` keep the M1-28 requirement `Any(AgentSelf, AppSelf)` with `OwnerID` = the card's author; their visibility is the component path (05 2.2). `interaction.respond` is an SR9 exception (07 SR9); nothing here depends on M2-14.

9. **Values** (`CheckValues`, pure; 09 11.1; Ruling for docs/DECISIONS.md for the shapes; describe them in `InteractionCreate.values` and 09 11.1). `values` absent means `{}`.
    - `button`: `values` must be `{}`.
    - `select`: `{"selected": [<option value>, ...]}`: every entry a `value` of the definition, no duplicates, count between `min_values` and `max_values`.
    - `form`: an object keyed by field `name`; unknown keys rejected; `required` fields present (`text` and `textarea` with at least one non-white-space character); `text` and `textarea` strings, `text` without line breaks, at most `max_length` characters when set; `number` a finite JSON number; `boolean` a JSON boolean; `choice` one of `choices`.
    - Every finding is `400 validation_failed` with `errors[].code` `interaction_values_invalid` and `pointer` `/values/<name>` (form), `/values/selected/<index>` or `/values/selected` (select), `/values` (button); `detail` names the rule, never the value (SEC-T10).
    - Values are stored exactly as sent after the check. The `interaction.created` payload that carries them must fit `sync.event_payload_max_bytes`; otherwise `400` with pointer `/values` and code `too_long` (04 2.2 rule 2; the agent has no other way to read the values).

10. **`createInteraction`** (09 11.2 to 11.4; 03; 04 5.1). Order and answers:
    a. Before the handler: authentication, `rl.interaction`, the decoder (unknown members, duplicate keys, invalid UTF-8 give `400`); `client_interaction_id` must be a UUIDv7, else `400` with pointer `/client_interaction_id` and code `not_uuid_v7` (as M1-56 D2). No `Idempotency-Key` (`x-idempotency-exempt: client_id`).
    b. Read the component, its message and thread without locks; an unknown component gives `404`.
    c. Unit of work, locks in the order of note 16: `chat.Threads.LockForMessageChange(thread)`; the run rows (the component's run and the thread's active run, ascending `id`, `FOR UPDATE`); `messaging.Messages.LockCard` (a deleted or missing message gives `404`); the single-use group with the exact statement of 03 when the chosen component is `single_use`, otherwise the chosen row `FOR UPDATE`.
    d. Replay: look up `(principal, client_interaction_id)` now, under the locks, so a concurrent twin waiting on the same rows finds the committed row. Found: `authorize(principal, message.read, card message)`; not allowed gives the `404` of an unknown component; allowed and the same `component_id` with `values` equal as JSON (key order ignored) gives `200` with the current `Interaction` and `Idempotent-Replay: true`, no write, no event; anything else gives `422 idempotency_key_reuse`. A unique violation on insert (only possible if the locks were bypassed) rolls back and answers like this step in a new unit of work. Ruling for docs/DECISIONS.md: 03 `interactions` has no request hash, so the comparison uses the stored `component_id` and `values`.
    e. `authorize(principal, interaction.create, descriptor of note 8 b)` with the facts of the locked rows; denials map by note 8 d and are audited by the authz layer (R-AZ-04).
    f. `CheckValues` against the locked definition (note 9).
    g. Writes, in this order: single-use: the chosen row `used` with `used_by`, `used_at`, `version + 1`, every other locked active row `disabled` with `version + 1`; non-single-use: the component row unchanged (no version, no `message.updated`; the interaction row records who and when; Ruling for docs/DECISIONS.md); insert the interaction (`id` UUIDv7, `state = received`); the run of note 10 h; `messaging.Messages.ComponentsChanged(card)` when a component changed; the events of note 10 i; the audit entry of note 17.
    h. The run (Ruling for docs/DECISIONS.md): `ComponentRunID` set: that run (it is active, note 8 b). Otherwise no active run exists in the thread (note 8 b refused every other case): open a new run through `Open` of M2-05 with `TriggerKind = interaction`, `TriggeringInteractionID` = the new interaction, `TriggeredBy` = the principal, `AgentID` = the card's author, `ThreadID` = the card's thread; `Open` writes the anchor, `run.state_changed` and schedules `runs.queued_expiry` (M2-08); no `trigger_state` changes (M2-05 note 4). Under the thread lock `ErrActiveRunExists` cannot occur; if it does, answer like note 8 b would (`run_active` or `run_active_other_agent`).
    i. Events in this order: `message.updated` (card, through `ComponentsChanged`), the events of `Open` when a run was opened, `interaction.created`, and for a new run `agent.invocation` last (M2-08 note 10 f). Both personal events go to the card's agent. Reason for writing both for a new run: `agent.invocation` announces the run and carries the start-bearing token (09 4.4, 5.1), while the values reach the agent only through `interaction.created`; `run_context.v1` has no place for them (spec gap reported). `interaction.created` comes first, so the values are in the adapter's inbox before the start-bearing event (ADR 0023). Ruling for docs/DECISIONS.md.
    j. Answer `201` with `Interaction`: `custom_id` from the component, `thread_id` of the card, `run_id` = the component's run or the run opened for it (absent otherwise), `values`, `state = received`, `created_at`; never `response`.

11. **Delivery** (M2-08 notes 2, 3, 5 and 7 extended; 04 15.3 steps 2 and 5; 07 8.1).
    a. Resolver `interaction.created`: run = `interaction.run_id` of the payload; trigger = `interactions.principal_id`; triggering object `interaction_id`; `InvokeFacts{AgentID: run.agent_id, TriggerOrigin: "human", ActiveRunAgentID: run.agent_id}` and `InteractionFacts` with stage `deliver` read from the component now. The delivery matrix row of M2-08 applies (as `agent.followup`: record the `jti` only for a run in `waiting_input`). On allow in the same transaction: `interactions.state` from `received` to `dispatched` (Ruling for docs/DECISIONS.md: the server sets `dispatched` when it hands the event to a transport). On denial: `interactions.state = failed`, the run unchanged, audit and counter as M2-08 note 3 f and 8 (Ruling for docs/DECISIONS.md). If the interaction no longer exists (its message was deleted), `Send = false` without audit.
    b. Resolver `agent.invocation` with `runs.trigger_kind = interaction`: trigger = `runs.triggered_by`; triggering object = `runs.triggering_interaction_id`; `InteractionFacts` with stage `deliver` from that interaction's component; when it was deleted, the check runs without `Interaction` facts (points 1 to 5, 7 and 8 still re-check the person, I2 and SR2; Ruling for docs/DECISIONS.md).
    c. Claims as M2-08 note 7 with `trigger_kind = interaction` and `interaction_id`. The `agent.invocation` payload for this trigger kind has `trigger_kind = interaction`, `interaction_id`, no `message_id`, and `triggered_by` = the `PrincipalRef` of the person; `runs` may not use `identity` (02 4.4), so take it from the request principal if `kernel/actor` carries handle and display name, else through the same `messaging` port function M2-08 uses for message authors, extended to a principal ID; report the choice.
    d. `AG-31` with a real interaction (Tests) replaces the stand-in delivery that M2-09 used.

12. **`respondToInteraction`** (05 `interaction.respond`, `rl.agent_runtime`, Audit `no`; 09 4.7 "state only forward"; 06 2 "No effect").
    a. Load the interaction, its component and message; unknown gives `404`. `authorize(agent, interaction.respond, {Kind: interaction, ID, ThreadID, OwnerID: card author})`.
    b. Body `InteractionRespond`: `state` one of `acknowledged`, `failed`, `rejected`; `response.text` optional, `kernel/textcheck` is not applied (content), at most `message.max_content_bytes` UTF-8 bytes (Ruling for docs/DECISIONS.md: a hint is message-like text), else `400` with pointer `/response/text` and code `too_long`.
    c. Under `FOR UPDATE` of the interaction row: from `received` or `dispatched` store `state`, `response`, `updated_at`; the same final state again answers `200` with the current interaction and writes nothing; any other transition gives `409 invalid_state_transition`. No event (05 6 has no type for it; the card shows the result through `updateComponent`, 09 11.4). `Idempotency-Key` through the M1-20 middleware.
    d. Add the `409` response to the operation in `openapi.yaml` (06 2 "No effect" requires it; spec gap reported). Answer `200` with `Interaction` including `response`.

13. **`updateComponent`** (05 `component.update`, `rl.write`, Audit `no`; 06 8 point 2; 09 11.4 "card shows result").
    a. `If-Match` missing or `*`: `428 precondition_required` before any lookup (`StagePreconditions`). Load; unknown gives `404`; `authorize(agent, component.update, ...)` (note 8 e).
    b. Unit of work: `LockCard` (deleted gives `404`); for a state change of a `single_use` component lock the whole single-use set of the message (`ORDER BY id FOR UPDATE`), else the component row; `version` not equal to `If-Match` gives `412 version_conflict` with `current` = the `Component`.
    c. Transitions (Ruling for docs/DECISIONS.md): `used` and `expired` are final (an expired-by-time `active` counts as `expired`); `active -> disabled` allowed; `disabled -> active` allowed only before `expires_at` and, for a `single_use` component, only while no `single_use` component of the message is `used` (the single-use rule must not allow a second decision, 09 11.3); every other change of `state` gives `409 invalid_state_transition`; add the `409` response to the operation in `openapi.yaml`. `definition` may change in every state; it must pass Design note 4 for the component's type (`type`, `custom_id`, `single_use`, `required_capability`, `restricted_to`, `run_id` cannot change: `ComponentUpdate` has no such members, SEC-OUT-4).
    d. No change (same state, same definition after decoding): `200` with the current `Component` and its `ETag`, no version, no event (06 2 "No effect"). Otherwise `version + 1`, `ComponentsChanged`, `200` with `Component` and `ETag: "<version>"`. `Idempotency-Key` through the middleware.

14. **SEC-OUT-4 and T-AI-05 (server half).** The header fields are set only by the server: the message author and `origin = agent` (M1-56), `kind = card`, `Component.run_id` (validated as the author's own active run in the same thread, note 6 b), `required_capability` and `restricted_to` (typed enums, note 5). The agent's text stays in `content_md` and in the definition texts; no member of a definition can name an author, a run, a capability or a system code (closed field sets); `kind = system` is never accepted from a client (SEC-OUT-3, M1-56). The client half (drawing the header, SEC-T14) is M4-21.

15. **Lifecycle of components.**
    a. Expiry (Ruling for docs/DECISIONS.md and row in 02 12.3: job `runs.component_expiry`, module `runs`, scheduled per card message at its earliest `expires_at` with `InsertTx` and `ScheduledAt`, queue `maintenance`, args only the message ID, 02 12.1; instead of a periodic job whose interval `component.default_ttl` would leave components `active` for up to a whole period). `Attach` and a reactivation through `updateComponent` schedule it. The handler, in one unit of work: `LockCard` (missing: done); lock the `active` components of the message with `expires_at <= now` (`ORDER BY id FOR UPDATE`); set them `expired` with `version + 1`; `ComponentsChanged` once if any changed; schedule itself again for the next `expires_at` of a remaining active component. Repeated execution is harmless (02 12.4 rule 1). Correctness never depends on the job: `authorize` treats a passed `expires_at` as expired (note 8 b).
    b. End of run (09 8 point 5; 03 index `(run_id) WHERE state = 'active'`; Ruling for docs/DECISIONS.md: at every end state, not only after a stop): the extension point of the M2-09 end-of-run function calls `DisableRunComponents(ctx, runID)`: the active components with that `run_id`, grouped by message in ascending message `id`; per message `LockCard`, the components `ORDER BY id FOR UPDATE`, `disabled` with `version + 1`, `ComponentsChanged`. The run row is already locked by `Transition`, so the order of note 16 holds. If an end path of M2-08 (denial of a `queued` run at delivery, `runs.queued_expiry`) does not go through the end-of-run function on your base, call `DisableRunComponents` there as well and list it in the report: a card may already be bound to a `queued` run (note 6 b).
    c. Follow-up answer (09 11.6 "Whatever arrives first counts; the card then becomes `disabled`"; 04 15.4 point 5): when a follow-up message is accepted for a run in `waiting_input`, `DisableRunComponents` runs in that transaction. If M2-11 is merged in the base, call it from its follow-up acceptance and add a test; otherwise name it in the report as a follow-up (plan gap reported).
    d. Message deletion (03 `messages` "Deletion ... deletes ... components"; M1-58 hands `MessageDeleted` to `runs`): `OnMessageDeleted` deletes the message's components (`DELETE ... WHERE message_id = $1`; interactions follow by cascade, `runs.triggering_interaction_id` becomes NULL). No event from `runs`: `message.deleted` and the redacted `message.*` payloads with `components: []` cover it (04 2.2 rule 5). A scheduled expiry job finds nothing and ends.

16. **Locks and concurrency** (03 6.3; Ruling for docs/DECISIONS.md). Domain rows in this order across tables: `threads` (only where a run may be opened or a message is written), `runs` (ascending `id`), `messages` (ascending `id`), `message_components` (ascending `id`), `interactions`; the counters last (unit of work). Every path of this ticket follows it, and so do M2-05 (`Transition`: `runs` then `messages`) and M1-58 (delete: thread, message, then the `MessageDeleted` subscriber). The one place where a `runs` lock comes after a `messages` row is `Attach` (note 6 b): it takes the share lock after the new card row is inserted, and a row that no other transaction can see yet cannot take part in a deadlock. Deadlocks that remain are retried by the unit of work (`database.tx_retry_max`). AG-15 holds because both clicks serialize on the single-use set; the loser sees the winner's committed rows and gets `component_inactive`, answered `409 component_used`.

17. **Audit** (05 4.10 Audit "yes for `approve_actions`, otherwise denials"; 07 4.2; 19 SEC-AUD-5). Denials of the three actions are written by the authz layer, aggregated per `audit.deny_aggregate_window`; do not add a second entry. An allowed `interaction.create` on a component with `required_capability = approve_actions` writes one entry in its transaction through `auditing.Recorder.Record`: action `interaction.create`, decision `allow`, resource `component` with its ID, details `interaction_id`, `message_id`, `run_id` (when set), `component_type`, `required_capability`, `restricted_to`, and `choice` only for a button whose `value` is one of `once`, `session`, `always`, `deny` (09 11.5); never `custom_id`, a label, a value of another component or a form value (they are agent or human text, SEC-AUD-5). Ruling for docs/DECISIONS.md: the detail set. Other allowed interactions, `component.update` and `interaction.respond` write no allow entry.

18. **Canary** (19 SEC-T10; privacy.md 6; the plan row; M1-69 left form values to the ticket that builds their way). Extend `services/server/test/canary`:
    - `values.go`: form values are class `content`, scoped to the card's channel.
    - `scenario.go` on the matrix world: Lotse (harness agent) posts in Support/Allgemein a card with a button, a select and a form whose labels and option labels are canaries; Alex submits the form with a canary text value and a canary textarea value (`201`), then a form with a canary in a `choice` field that is not among `choices` (`400` with `interaction_values_invalid`); Lotse calls `respondToInteraction` with a canary `response.text` and `updateComponent` with a canary label; the scenario also sends `createInteraction` as Robin (`404`) and `updateComponent` without `If-Match` (`428`). The three operations are now enabled, so `TestSEC_T10_ScenarioCoversEveryEnabledOperation` must pass without an exemption.
    - Expected: no canary in sinks 1 to 4 (logs at level debug, traces, metrics, audit rows), in error bodies (the `400` echoes no value), or in frames, pages and snapshots of the observers Sam and Robin; the form values appear only in the `201` answer to Alex and in the `interaction.created` delivered to Lotse.
    - `README.md`: the new field and where it may appear.

19. **Events** (05 6; 04 2.2, 9 point 3; ADR 0052, 0053):

    | Type | Producer | `scope` | `resource` | `audience` | Receive action | Payload |
    |---|---|---|---|---|---|---|
    | `interaction.created` | `runs` | `{space_id, channel_id, thread_id}` of the card | `{kind: interaction, id}` without `version` (03 `interactions` has no version) | the card's agent | `agent.events.consume` | `InteractionCreatedPayload{interaction}` with `values`, without `response` |
    | `message.updated` | `messaging` through `ComponentsChanged` | `{space_id, channel_id, thread_id}` | `{kind: message, id, version: new}` | none | `message.read` | `MessagePayload{message}` with `components` |
    | `agent.invocation` (new run) | `runs` (builder of M2-08) | target thread | `{kind: run, id}` without `version` | the card's agent | `agent.events.consume` | `AgentInvocationPayload` with `trigger_kind = interaction`, `interaction_id` |

    `actor` = the person (or the agent for `updateComponent`, none for the jobs), `correlation_id` of the request. Every payload validates against its schema and contains no JSON `null`.

20. **Rulings for `docs/DECISIONS.md`** (section 10, one row `D-M2-15-<n>` each, with reason and place): component changes raise `messages.version` through `ComponentsChanged` (note 2 b); `ComponentWriter` (note 2 c); the open points of `component.v1` and the generic code `invalid` (note 4); the card rules and `403 feature_disabled` for `supports_components = false` (note 6 a, b); the run of a card in `message_components.run_id` (note 6 b); request hash `bothy-msg-v2` (note 6 c); the interaction rules of `authorize`: mention rule, `run_active`, inactive component of an ended run, stage `deliver`, owner restriction as `component_restricted`, `component_inactive` answered `409 component_used` (note 8); value shapes (note 9); replay by `component_id` and `values` (note 10 d); non-single-use components unchanged on use (note 10 g); new run and event order (notes 10 h, 10 i); `dispatched` and `failed` set by the delivery and the check without a deleted interaction (note 11); respond rules and the bound of `response.text` (note 12); component transitions (note 13); expiry job per card (note 15 a); disabling at every end of run and at a follow-up answer (note 15 b, c); lock order (note 16); audit details (note 17).

## Tests (write them first)

The test world is 07 "Examples" through `BuildMatrixWorld` of M1-71: spaces Support, Marketing, Engineering, Research; agents Lotse (operator of Support, `organization`, `chat`), Texter (operator of Marketing, `organization`, `scoped`), Runner (`host`), Assistent (operator of Research, `personal`, holder the Owner); the Owner; G-1 (Alex, `trigger`, `upload`, `approve_actions` on Support), G-2 (Sam, `trigger`, `upload` on Marketing), G-4 (group Leser with Sam, `read` on Support/Allgemein). Agents act through their harness tokens; runs are started and set to `waiting_input` through `startRun` and `awaitRunInput` with the tokens delivered by M2-08. The test clock of `kernel/clock` drives time. Go test names carry the test IDs with `_` instead of `-`.

Unit:

- `internal/runs/domain`: `TestCheckCard` (a table with one case per rule of Design notes 4 and 5: unknown definition key, missing `label`, style outside the roles, icon not a glyph name, emoji icon, duplicate option value, `min_values` above `max_values`, more fields than `component.form_max_fields`, more components than `component.max_per_message`, `custom_id` longer than `component.custom_id_max_chars`, bidi character in a label, `max_length` on a `boolean`, `choices` missing on a `choice`, `required_capability = terminal`, `expires_at` in the past, SEC-AI-4 default `run_starter`, positions, defaults), each with the exact pointer and code; `TestCanonicalCard` (key order and white space give the same bytes; every field change gives different bytes; defaults are not applied); `TestCheckValues` (button with values, select below `min_values` and above `max_values`, unknown option, duplicate selection, form with unknown key, missing required field, white-space-only required text, line break in `text`, too long text, string for `number`, a number outside the range of a 64-bit float, number for `boolean`, choice outside `choices`; every error has code `interaction_values_invalid` and a pointer, and no `detail` contains the value); `TestEffectiveState` (active before and expired at `expires_at`); `TestComponentTransitions` (every cell of Design note 13 c); `TestInteractionRouting` (component run, no run and no active run, run-less component with the agent's active run, active run of another agent); `TestAuditDetails` (`choice` only for `once`, `session`, `always`, `deny`; never `custom_id` or labels).
- `internal/authz`: `TestInteractionCreate_RequirementNotDeferred` (the golden list of deferred actions no longer contains `interaction.create`; AZ-P3 stays green); `TestInteractionCreate_ReasonOrder` (inactive before capability before restriction; I2 and SR2 before point 6; project after it; `run_active_other_agent` and `run_active` last); `TestAZ_M42_Decision` and `TestAZ_M43_Decision` at the decision level; `TestInteractionCreate_OwnerPassesRestrictions`; `TestInteractionCreate_MentionRuleMet` (`thread_require_mention` set, no mention, operator card: allowed); `TestInteractionCreate_StageDeliver` (state `used` passes, capability and restriction are evaluated, `run_active` is not); `TestDecisionMapping_Interactions` (every row of Design note 8 d).
- `internal/messaging/domain`: `TestRequestHash_V1UnchangedForPlainMessages` (golden bytes from M1-56 still match) and `TestRequestHash_V2ForCards` (sensitive to `run_id` and every component field).
- Fuzz: `FuzzCheckCard` and `FuzzCheckValues` never panic and never accept a definition or value that `TestCheckCard` and `TestCheckValues` would reject.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP and gateway through the real chain, every response checked against `openapi.yaml`):

- `TestMigration_ComponentsAndInteractions`: checks, uniques, FKs and cascades of Design note 3.
- `TestCreateMessage_Card`: Lotse posts a card with a button, a select and a form bound to its running run: `201`; the rows carry the defaults of Design note 5; `messages.run_id` is NULL and every `message_components.run_id` is the run; `message.created` and `getMessage` and `listThreadMessages` show the components in position order; one `runs.component_expiry` job is scheduled at the earliest `expires_at`.
- `TestCreateMessage_CardRules`: components without `kind = card`, a card without components, a card from a person, `run_id` on a text message (`not_enabled`), a run of Texter, a run of another thread and an ended run (all three the same `invalid` on `/run_id`), and a card whose event would exceed `sync.event_payload_max_bytes` (`too_long`): exact status, pointer and code; nothing is stored.
- `TestCreateMessage_CardSupportsComponentsOff`: with `operator_config.supports_components = false` for Lotse, `403 feature_disabled`.
- `TestCreateMessage_CardReplayAndHash`: the same card twice gives `201` then `200` with `Idempotent-Replay: true` and the same components; the same `client_msg_id` with a changed component gives `422 idempotency_key_reuse`; a plain text message replays as before.
- `TestCreateInteraction_Button`: Alex clicks a single-use button on a card of Lotse's run in `waiting_input`: `201` with `custom_id`, `thread_id`, `run_id`, `state = received`; the button is `used` with `used_by` Alex, its siblings `disabled`; events in the order of Design note 10 i; `message.updated` raises `messages.version` by one; the interaction payload validates against `InteractionCreatedPayload`.
- `TestAG_15_ConcurrentClicksOneWins` (AG-15: two people click different buttons of one card at the same time; exactly one wins, the rest is `disabled`): on a card of Lotse with two single-use buttons (`restricted_to = any`, no `required_capability`), Alex and the Owner click different buttons behind a barrier: exactly one `201` and one `409 component_used`; one `used`, all others `disabled`; one interaction row; one `interaction.created`; one `message.updated` for the card; the loser's denial is in the audit with reason `component_inactive`.
- `TestAG_16_AlwaysAllowNonOwner403` (AG-16: "Always allow" by a non-owner gives `403`): Lotse posts the approval card of 09 11.5 for Alex's run (four buttons with the values, capabilities and restrictions of the table); Alex clicks "Always allow": `403 forbidden` with `reason` `component_restricted`, nothing changes, one deny audit entry; Alex then clicks "Allow once": `201`, the other three are `disabled`, one allow audit entry with `choice = once`; on a second card the Owner clicks "Always allow": `201`, audit `choice = always`.
- `TestAZ_M42_RestrictedToRunStarter` (AZ-M42: Alex on a card with `restricted_to = run_starter` whose run the Owner started gives `forbidden` `component_restricted`): the Owner triggers Lotse; Lotse starts the run and posts the card; Alex (who holds `approve_actions`) gets `403 forbidden` with `reason` `component_restricted`.
- `TestAZ_M43_ApproveActionsMissing` (AZ-M43: Sam on a card with `approve_actions` in Marketing without `approve_actions` gives `forbidden` `capability_missing`): Sam triggers Texter; Texter posts an approval card for Sam's run; Sam gets `403 forbidden` with `reason` `capability_missing`.
- `TestCreateInteraction_FormValuesValidated` (plan row: form values checked against the definition): a valid form gives `201` with the values stored as sent; each invalid case of `TestCheckValues` over HTTP gives `400 validation_failed` with `interaction_values_invalid` and the pointer; values whose event would exceed `sync.event_payload_max_bytes` give `too_long` on `/values`.
- `TestCreateInteraction_SelectValues`: within and outside `min_values` and `max_values`.
- `TestCreateInteraction_Replay`: the same request twice gives `201` then `200` with `Idempotent-Replay: true`, one row, one event; different values with the same `client_interaction_id` give `422`; two identical requests behind a barrier give one `201` and one `200`; a replay after Alex lost read access gives `404`.
- `TestCreateInteraction_RightsAndLeaks`: Robin (no right on Support) gets `404` identical in status, body and headers (except `correlation_id` and `instance`) to an unknown component ID; Sam (read through Leser) gets `403 forbidden` with `level_insufficient`; an archived channel gives `403 channel_read_only` with `reason` `channel_archived`; an agent token gets `403`.
- `TestCreateInteraction_Expired`: the clock passes `expires_at`, the job has not run: `409 component_used`; the job then sets `expired`, raises the versions and writes one `message.updated`.
- `TestCreateInteraction_I2PersonalAgent` (Red team: a personal subscription triggered by another person through an interaction): a raw SQL statement on the `pgtest` connection gives Alex `trigger` on Research while Assistent stays `personal` with the Owner as holder (a fixture that simulates an I1 defect, OQ-01); Alex clicks a card of Assistent: `403 agent_invocation_denied` with `reason` `billing_mode_personal_not_holder`; the component stays `active`; one deny audit entry.
- `TestCreateInteraction_NewRun`: a run-less card of Lotse, no active run: `201`; a new run `queued` with `trigger_kind = interaction`, `triggering_interaction_id`, `triggered_by` Alex; events in the order of Design note 10 i; Lotse's consumer receives `interaction.created` and then `agent.invocation` with tokens whose claims carry `trigger_kind = interaction` and `interaction_id`; `startRun` with the `agent.invocation` token gives `200`.
- `TestCreateInteraction_ActiveRunRules`: a run-less card of Lotse while Lotse's run is active gives `403 agent_invocation_denied` with `run_active`; while a run of another agent is active, `run_active_other_agent`.
- `TestAG_31_InteractionResumesWaitingRun` (AG-31 end to end: only the token of the `interaction.created` delivery sets a waiting run to `running`): Alex triggers Lotse; Lotse starts with S, posts an approval card bound to the run and calls `awaitRunInput`; `renewInvocationToken(S)` and `resumeRun(S)` give `403 invocation_token_invalid` with `superseded`; Alex clicks "Allow once"; the delivery of `interaction.created` carries token A and sets `interactions.state = dispatched`; `resumeRun(A)` gives `200` and `running`.
- `TestInteractionDelivery_Recheck`: after Alex's click and before Lotse connects, the Owner revokes G-1: `interaction.created` is not sent, `interactions.state = failed`, the run stays `waiting_input`, one deny audit entry with `stage = delivery`; a variant revokes only `approve_actions`: same outcome with `capability_missing`.
- `TestRespondToInteraction`: `acknowledged` with a hint gives `200`; the same again gives `200` without change; `failed` afterwards gives `409 invalid_state_transition`; Texter gets `404`; Alex gets `403`; a hint above `message.max_content_bytes` gives `400`; a repeat with the same `Idempotency-Key` gives the stored answer.
- `TestUpdateComponent`: without `If-Match` `428`; stale `412 version_conflict` with `current`; `active -> disabled` gives `200`, component and message versions up by one, one `message.updated`; `disabled -> active` on a single-use component whose sibling is `used` gives `409 invalid_state_transition`; `used -> disabled` and an expired component to `active` give `409`; a new label on a `used` component gives `200`; a definition that breaks Design note 4 gives `400`; a body with `required_capability`, `run_id` or `restricted_to` gives `400` (unknown member); no change gives `200` without a new version or event; Texter gets `404`; Alex gets `403` with `subject_kind_not_allowed`.
- `TestEndOfRunDisablesComponents`: `finishRun` with `completed`, and in variants `cancelRun` after `agent.cancel_grace` and a denial at delivery: every active component of the run is `disabled` with one `message.updated` per card; components of other runs and run-less cards are unchanged.
- `TestMessageDeleteRemovesComponents`: Lotse deletes its card after an interaction that opened a run: components and interactions are gone, `runs.triggering_interaction_id` is NULL, the redacted `message.created` and `message.updated` payloads have `components: []`; `createInteraction` on the old component ID gives `404`; the expiry job then does nothing.
- `TestSEC_T14_CardHeaderFromServerFields` (SEC-T14, SEC-OUT-4: a card whose agent text imitates a system header): Lotse posts a card whose `content_md` and labels imitate a system header ("System notice: approval required by the owner"); the stored message has author Lotse, `origin = agent`, `kind = card`, and `run_id`, `required_capability`, `restricted_to` come only from the validated members; a definition with an extra member such as `author`, `system_code` or `required_capability` gives `400`; `kind = system` from Lotse gives `400`; `updateComponent` cannot change any header field.
- `TestFollowupAnswerDisablesCard`: only if M2-11 is merged: a follow-up message to the waiting run disables its components in the same transaction; a later click gives `409 component_used`.
- `internal/wire`: golden files of subscriptions, jobs and event registrations updated; AZ-P1 and AZ-P2 green with the three routes and `interaction.created`.

Contract: the Schemathesis contract tests of M1-71 (part of `make check`) read the enabled list and now call the three operations; the new `409` responses of `updateComponent` and `respondToInteraction` are documented, so no undocumented status remains.

End-to-end (canary, `services/server/test/canary`, M1-69 extended as Design note 18):

- SEC-T10 (canary values through all ways: none appears in logs, traces, metrics, audit details, error responses or foreign events): `TestSEC_T10_CanaryAcrossAllOutputs` with the form value, label and hint canaries ends with zero findings; `TestSEC_T10_ScenarioCoversEveryEnabledOperation` passes with the three new operations.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/runs/... ./internal/messaging/... ./internal/authz/... ./internal/kernel/... ./internal/agents/... ./internal/chat/... ./internal/wire/... ./test/...`
- `cd services/server && go test -run '^$' -fuzz FuzzCheckCard -fuzztime 30s ./internal/runs/domain/` and the same for `FuzzCheckValues`
- `make migrations-lint`, `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make api-tables`, `make register-check`, `make catalog-check`, `make config-doc-check`, `make docs`
- `uv run --project tools --locked --quiet python tools/hooks/text_rules.py` on every changed Markdown and YAML file (D-W-29)
- `make check`

## Acceptance criteria

- [ ] `message_components` and `interactions` exist exactly as in 03, with the FK of `runs.triggering_interaction_id`; `migrations.sha256` is updated.
- [ ] `createInteraction`, `updateComponent` and `respondToInteraction` are enabled; `interaction.create` is no longer deferred; AZ-P1, AZ-P2 and AZ-P3 are green.
- [ ] `MessageCreate.components` and `kind = card` are enabled for agents; cards from apps keep `not_enabled` until M2-25; a card binds its components to the author's own active run through `run_id`.
- [ ] AG-15: of two concurrent clicks on one card exactly one wins, the rest is `disabled`, and the loser gets `409 component_used`.
- [ ] AG-16: "Always allow" by a non-owner gives `403` with `component_restricted`; an owner can choose it.
- [ ] AZ-M42: Alex on a `run_starter` card of the Owner's run gets `forbidden` `component_restricted`.
- [ ] AZ-M43: Sam without `approve_actions` gets `forbidden` `capability_missing` on an approval card in Marketing.
- [ ] Form values and selections are checked against the definition with `interaction_values_invalid` and a pointer; definitions are checked against `component.v1` with the keys of 09 21.
- [ ] The card header comes from server fields only (SEC-OUT-4, SEC-T14 server half).
- [ ] Interactions are idempotent by `client_interaction_id` (`200` replay, `422` on different content).
- [ ] `interaction.create` is audited on allow for `approve_actions` with the details of Design note 17; denials are audited once.
- [ ] `interaction.created` reaches only the owning agent after the re-check of `agent.invoke`; its token resumes a waiting run (AG-31 end to end); a new run starts when no run waits.
- [ ] Components expire, are disabled at every end of their run and are deleted with their message; every visible change produces `message.updated`.
- [ ] The canary scenario contains a form value and finds no leak.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `runs`, `messaging`, `authz` and `test/canary` updated (ports `Interactions`, `ComponentReader`, `ComponentWriter`, `LockCard`, `ComponentsChanged`, the card rules and error codes, the request hash version, the interaction guard and its reason order, the lock order of Design note 16, the job `runs.component_expiry`, the canary field).
- Rulings of Design note 20 recorded in `docs/DECISIONS.md` (one row each with reason).
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
