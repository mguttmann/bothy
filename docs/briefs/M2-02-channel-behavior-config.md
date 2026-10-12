# M2-02: Channel behaviour and agent configuration

Status: ready | Depends on: M1-69 | Milestone: M2

## Goal

After this ticket a person with `manage` on a channel reads and changes the channel's behaviour:
the mention switches `require_mention`, `thread_require_mention`, `free_response` and
`allow_mentioned_agents`, the switch `auto_thread`, the context window `history_backfill_limit`, the
channel prompt and the ordered skill references. A change needs a fresh step-up and `If-Match` on
`settings_version`, is audited, raises `settings_version` by one and writes
`channel.settings_changed`; when the prompt or the skills change it also writes the personal event
`agent.config_changed` to the channel's effective operator. Both events carry only the channel id
and the new version. The effective operator agent lists the channels it serves and reads their
configuration over REST (`agent.config.read`); nobody else reads it there. Prompt and skills are
readable only with `manage` or as the effective operator and never appear in events, logs, traces,
metrics, audit details or error bodies, which the SEC-T10 canary scenario proves with a canary
prompt. This is the server side of 09 "Channel switches" and of the adapter's configuration path in
09 "Overview" (row "Context of a run, channel prompt and skills"); the trigger decision that uses
the switches is M2-04, the project binding is M2-03.

## Read first

- `docs/spec/03-data-model.md`: 4.5 "Structure (`chat`)" tables `channels` (columns
  `require_mention`, `thread_require_mention`, `free_response`, `auto_thread`,
  `allow_mentioned_agents`, `history_backfill_limit`, `prompt`, `version`, `settings_version`; the
  paragraphs "Effective operator" and "Mention rule"), `channel_skill_bindings` (ordered, unique
  `(channel_id, skill_ref)`, the server never interprets a reference), `project_bindings` (only to see
  what M2-03 adds); table `event_log` (`audience_principal_id`, `receive_action`, the check
  `receive_action <> 'self' OR audience_principal_id IS NOT NULL`); 6.3 "Lock order";
  9 "Configuration values of this chapter" (`channel.prompt_max_bytes`,
  `channel.history_backfill_limit_default`, `channel.history_backfill_limit_max`).
- `docs/spec/05-actions.md`: 2.1 'Column "Permission"' (`manage`, `agent-self`); 2.3 "Effective level"
  points 1, 4 and 5; 2.4 'Columns "Owner", "Step-up", "Audit", "Permissions"'; 2.5 "Decision"; 2.6
  "Scopes for apps"; 4.5 rows `channel.behavior.read` and `channel.behavior.update`; 4.10 row
  `agent.config.read`; 6 "Durable event types and receive action" rows
  `channel.settings_changed` and `agent.config_changed`; 8 decisions R-AZ-01, R-AZ-03, R-AZ-04,
  R-AZ-09, R-AZ-14; 9 "Tests that hold this catalogue" points 1, 2 and 4.
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope" with payload rule 1 (prompt, skills
  and project binding never in events); 8.1 "Basic rule"; 9 "Delivery guarantee" point 3 (both events
  without `resource.version`); 15.1 "Connection and cursor" point 4 (`agent.config_changed` is a
  personal event to the agent).
- `docs/spec/09-agents.md`: 3.1 "Operator", 3.3 "Channel switches", 3.6 "Guest agents", 4.1
  "Overview", 4.4 "Events to the adapter" (row `agent.config_changed` and the paragraph after
  the table), 4.6 "Run context" (how M2-13 uses `history_backfill_limit`).
- `docs/spec/07-permissions.md`: 2.1 "Subjects" (agents and apps never hold `manage`), 2.9 "Agents as principals", 4.1 "Decisions and reasons" (`capability_missing`, `subject_kind_not_allowed`,
  `step_up_required`, `step_up_not_possible`), 4.2 "Flow" (requirement before step-up, audit of
  denials, filter mode), 4.3 "Visibility", 4.5 "Archived spaces and channels", 7.2 "Permission state (snapshot)", 7.4 "Query pattern" rules 1 and 3, 11.2 "Required tests" AZ-P1, AZ-P2, AZ-P4, AZ-P8,
  AZ-P12, AZ-P13.
- `docs/spec/10-backend-policy.md`: 5.2 "Targets that an agent operates" (last paragraph: guest
  channels are no I1 targets, so `channel.behavior.update` needs no I1 check).
- `docs/spec/06-api.md`: 2 "Basic rules" rows "No effect", "No `null`", "Unknown values"; 4
  "Authorization in the API" points 2 and 3; 5 "Errors" points 3, 4 and 7; 6 "Pagination" point 6;
  7 "Idempotency" (row "PUT and DELETE"); 8 "Concurrency" points 1, 2 and 5; 16 "Endpoints"
  tables "Channels" and "Agent runtime".
- `docs/spec/02-architecture.md`: 4.3 "Ports, events and sources per module" (`chat` port
  `AgentConfig`, no in-transaction event for behaviour), 4.6 "Actions per module" (`agent.config.read`
  belongs to `chat`), 8.1 "Flow", 8.2 "Lock order", 9.3 "Proof of the check" (lists count
  through `Scope`), 11.3 "Permission cache".
- `docs/spec/19-security.md`: SEC-IN-1, SEC-IN-2, SEC-IN-4 (6.1), SEC-AI-1 and SEC-AI-8 (11
  "Controls for agents"), SEC-LOG-1 to SEC-LOG-7 (9), SEC-AUD-5 (12), SEC-T10 (14 "Tests").
- `docs/spec/08-identity.md`: 6.2 "Step-up flow".
- `docs/api/openapi.yaml`: operationIds `getChannelBehavior`, `putChannelBehavior`,
  `listServedChannels`, `getServedChannelConfig`; schemas `ChannelBehavior`, `ChannelBehaviorPut`,
  `ProjectBinding`, `AgentChannelConfig`, `AgentChannelConfigList`, `ChannelSettingsChangedEvent`,
  `ChannelSettingsChangedPayload`, `AgentConfigChangedEvent`, `AgentConfigChangedPayload`, `Channel`
  (carries `settings_version`), `FieldError`, `Problem`; parameters `ChannelIdPath`,
  `IfMatchRequired`; responses `PreconditionFailed`, `PreconditionRequired`.
- `docs/api/asyncapi.yaml`: messages `event.channel.settings_changed` and
  `event.agent.config_changed` (with `x-audience: self`). `docs/api/webhooks.md`: rows
  `channel.settings_changed` and `agent.config_changed` (delivery over webhooks is M2-26).
- `docs/spec/actions.yaml` on the base branch (M1-02): entries `channel.behavior.read`,
  `channel.behavior.update`, `agent.config.read`, `channel.settings_changed`
  (`resource_version: false`), `agent.config_changed` (`audience: self`, `receivers: agent`).
- ADRs 0013 (spec first), 0015 (errors, idempotency), 0017 (one authorize, 404 before 403), 0018,
  0020 (step-up), 0021 (audit separate from telemetry), 0022 (agents outside the server), 0049
  (action catalogue), 0052 (one envelope), 0053 (no JSON `null`), 0054 (provenance; the channel
  prompt is its own field, SEC-AI-1).
- `docs/DECISIONS.md`: D-W-100 (briefs written before M1 closed); section 10 "Rulings of the build milestones" (row format `D-<Ticket>-<n>`).
- `docs/security/threat-model.md`: T-AI-09 (prompt exfiltration, OWASP LLM07), T-AI-01.
  `docs/RISKS.md`: RK-01.
- `docs/observability/signals.md`: `bothy.authz.errors` with `bothy.reason = filter_mismatch`;
  `docs/observability/alerting.md`: `BothyAuthzError`; `docs/observability/privacy.md`: 6
  "Canary test (gate)" (the prompt is one of the canary fields).
- `docs/operations/configuration.md`: `channel.prompt_max_bytes`,
  `channel.history_backfill_limit_default`, `channel.history_backfill_limit_max`,
  `auth.step_up_max_age`, `database.tx_retry_max`, `http.max_json_body_bytes`,
  `authz.not_found_timing_tolerance`, `audit.deny_aggregate_window`.
- `docs/PLAN.md`: 3 "Way of working per ticket"; 8 "M2 Agent platform, extensions and terminal"
  (Goal, Out of scope, Exit criteria point 10, Red team T-AI-09); rows M2-02, M2-03, M2-04, M2-07,
  M2-08, M2-13, M2-14, M2-26; M1 rows M1-53, M1-54, M1-63, M1-65, M1-69, M1-70, M1-71.
- M1 briefs this ticket builds on: M1-53 (channel columns and `channel_skill_bindings` already in the
  channels migration, archived channels frozen with `409 conflict`), M1-54 (effective operator,
  `ChannelsOperatedBy`, `setChannelOperator`, `setSpaceOperator`), M1-28 (`Authorizer.Authorize`,
  `Scope`, `Filter(ctx)`, requirement atoms `Cap(manage)` and `ChannelOperator`, `ChannelFacts`),
  M1-18 (`StagePreconditions`, `FormatETag`, `ParseIfMatch`, problem writer, check log), M1-17
  (`api/operations.yaml`, route generator), M1-16 (`eventlog.Appender`, `eventlog.Event`), M1-24
  (`auditing.Recorder`, `Attrs`), M1-13 (`txn.UnitOfWork.Do`), M1-35 (step-up), M1-26
  (`kernel/textcheck`), M1-63 (`listEvents`, `sync.Admit`), M1-65 (`Authorizer.Observe`, delivery
  mode), M1-69 (`services/server/test/canary`), M1-70 (SQL visibility lint), M1-71
  (`services/server/test/harness`, `BuildMatrixWorld`).
- Dependencies: the header names M1-69 (canary scenario, D-W-106); M1-71, M1-65, M1-63, M1-54,
  M1-53, M1-52, M1-51, M1-49, M1-48, M1-35, M1-29, M1-28, M1-26, M1-24, M1-20, M1-18, M1-17, M1-16,
  M1-13 and M1-08 are implied through it. Design note 1 stops the build while one of them is
  missing.
- Code: `services/server/internal/chat` (README; spaces, channels and operators of M1-52 to M1-54),
  `internal/authz`, `internal/sync`, `internal/kernel/{authorize,auditing,eventlog,txn,textcheck,actor}`,
  `internal/platform/httpserver`, `internal/wire` (config keys, `configchecks.go`),
  `services/server/api/operations.yaml` and `services/server/api/README.md`,
  `services/server/test/harness`, `services/server/test/canary`, `services/server/test/integration`.

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

11. `channel.behavior.read` needs `manage` on the channel, without step-up and audit, rate-limit
    class `rl.read`; prompt and skills are readable only with `manage` (05 4.5, R-AZ-09). Agents and
    apps never hold `manage` (07 2.1, R-DM-20), so they never read through this path.
12. `channel.behavior.update` needs `manage`, step-up, audit of allow and deny, no permission change,
    rate-limit class `rl.admin`, and `If-Match` on `settings_version` (05 4.5 and 2.4; 06
    "Concurrency" points 1 and 2; R-AZ-03, R-AZ-04).
13. The behaviour lives in the typed columns of `channels` and the ordered rows of
    `channel_skill_bindings`; `history_backfill_limit` lies between zero and
    `channel.history_backfill_limit_max`, the prompt is at most `channel.prompt_max_bytes`;
    `settings_version` is the own counter for behaviour, prompt, skills and project, and events name
    only this number (03 `channels`, `channel_skill_bindings`; 09 3.3).
14. `channel.settings_changed` (resource `channel`, receive action `channel.read`) and the personal
    `agent.config_changed` (resource `channel`, audience the agent, receive action
    `agent.config.read`) carry only `channel_id` and `settings_version` and no `resource.version`
    (05 6; 04 2.2 rule 1 and 9 point 3; 09 4.4).
15. `agent.config.read` needs the caller to be the agent that is the channel's effective operator,
    rate-limit class `rl.read`; the operator reads prompt and skills over REST (05 4.10; 09 4.1).
16. No read permission gives `404 not_found`, identical to an unknown id; `403 forbidden` only with read
    permission (05 rule 1.2, R-AZ-01, AZ-P4). Lists filter in the query and then run `authorize` per
    element (07 7.4 rules 1 and 3); lists count through `Scope` (02 9.3).
17. A change without effect answers `200` with the current state, without new version and event,
    and the audit entry carries `no_change: true` (06 "Basic rules" row "No effect", R-API-27).
18. No JSON `null` in responses and events (ADR 0053); unknown request members give
    `400 validation_failed` (06 "Unknown values", SEC-IN-1).
19. Prompt and skill references never appear in logs, traces, metrics, audit details, error bodies
    or events (SEC-LOG-4, SEC-LOG-6, SEC-AUD-5, R-AZ-09); the SEC-T10 canary scenario carries a prompt (19
    "Tests", PLAN row M2-02). Prompts are no place for secrets (SEC-AI-8).

## Scope

All paths under `services/server/` unless noted. No migration: M1-53 created the behaviour columns,
`settings_version` and `channel_skill_bindings`. No new make target.

Create:

- `internal/chat/domain/behavior.go`: the value `Behavior` (the five switches, `auto_thread`
  included, `HistoryBackfillLimit`, `Prompt`, `Skills []string`), `BehaviorLimits{PromptMaxBytes,
  BackfillMax int}`, `ValidateBehavior(b Behavior, l BehaviorLimits) []FieldError` (use the
  field-error type the chat domain already has), `ChangedFields(old, new Behavior) []string`
  (codes of Design note 7) and `ConfigChanged(old, new Behavior) bool` (prompt or skills differ).
- `internal/chat/app/behavior.go`: the four use cases and the shared function of Design note 6
  that raises `settings_version` and appends the events.
- `internal/chat/adapters/postgres/queries/behavior.sql`: read of the behaviour columns and the
  skills of one channel, `FOR SHARE` on the space row, `FOR UPDATE` on the channel row, update of
  the behaviour columns with `settings_version = settings_version + 1`, delete and insert of the
  skill rows, the list queries of Design note 9; sqlc output and repository methods.
- `internal/chat/adapters/httpapi/behavior.go`: the strict-server methods for
  `getChannelBehavior`, `putChannelBehavior`, `listServedChannels`, `getServedChannelConfig`.
- `test/integration/channel_behavior_test.go`: the integration tests below on the shared harness.

Change:

- `internal/chat` (port): `AgentConfig` and its DTOs (Design note 2).
- `internal/authz` and, if the interface needs it, `internal/kernel/authorize`: `Scope` accepts
  `agent.config.read` (Design note 9); `Observe` treats `channel.settings_changed` as a structure
  event (Design note 11b); tests including the AZ-P8 property case.
- `internal/sync`: the admission rule for personal events with a receive action other than `self`
  (Design note 11a), only if the merged code lacks it; with a unit test either way.
- `internal/wire`: construct the `AgentConfig` use cases and hand them to the chat HTTP adapter;
  register `channel.prompt_max_bytes` in the chat config keys if M1-53 did not; add the cross-check
  of Design note 13 to `configchecks.go` if none exists.
- `api/operations.yaml`: move `getChannelBehavior`, `getServedChannelConfig`, `listServedChannels`
  and `putChannelBehavior` from `later` to `enabled` (sorted ascending); regenerate as
  `services/server/api/README.md` describes (`go generate ./api/...`).
- `docs/api/openapi.yaml` (repository root): descriptions only, no schema change. On
  `putChannelBehavior` the rules of Design note 5 with their field error codes, the event rule of
  Design note 6 and the absence of `Idempotency-Key`; on `listServedChannels` "only non-archived
  channels whose effective operator is the caller, ordered by channel id; other principals get
  `403 forbidden`"; on `getServedChannelConfig` "only the channel's effective operator". If the
  description of `FieldError.code` lists the code vocabulary (M1-18), add `duplicate` and
  `invalid_characters`.
- `test/canary/` (M1-69): `values.go`, `scenario.go`, `scan_test.go` and `README.md` as in Design
  note 14.
- `tools/lint/sql_visibility.yaml` (repository root, M1-70): exception entries of Design note 12,
  only if the lint is in the base branch.
- Documentation: `internal/chat/README.md` (port `AgentConfig`, operations, events, rulings, query
  exceptions), `internal/authz/README.md` (`Scope` for `agent.config.read`, the cache event),
  `internal/sync/README.md` if the admission rule changes, `test/canary/README.md`,
  `services/server/CHANGELOG.md` (Unreleased, Added), `docs/DECISIONS.md` section 10 rows
  `D-M2-02-<n>` for the rulings of Design note 15, `docs/security/threat-model.md` row T-AI-09
  column "Tests": SEC-T10.

## Out of scope

- Project binding: `putProjectBinding`, `deleteProjectBinding`, the table `project_bindings`, the
  member `ChannelBehavior.project_binding` and readiness (M2-03). M2-03 calls the shared function of
  Design note 6 for binding changes.
- Using the switches: the trigger decision of 09 "Decision per message" and the guest rule
  (M2-04); auto-thread on a trigger and the `channel.settings_version` member of `agent.invocation`
  (M2-08, M2-09).
- The run context that applies `history_backfill_limit` (M2-13).
- Agent consumers with durable cursor, `ack` and redelivery (M2-07); agent webhook subscriptions
  that carry `agent.config_changed` (M2-26); `ActingFor` filled from the invocation token (M2-14).
- Reference adapters that load the configuration (M2-33, M2-34).
- Setting operators (M1-54) and `resolve_i1` (M2-21); the read-only switch `setChannelReadOnly`
  (M8-03).
- The editing views in the apps and the console, including the hint of SEC-AI-8 (M4 to M6, 01 5.2).
- The canary run through the real collector (M1-73).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-02-channel-behavior-config`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(chat): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- No agent runtime and no model: agent principals in tests are harness agents that call the REST API with their own tokens; no adapter process, no call to a model or any third-party service.
- Test prompts and skill references are canary values or neutral sentences, never a secret (SEC-AI-8).
- Changes to `authz`, `sync` and the canary test stay within Design notes 9, 11 and 14, each with a test; no other behaviour of an M1 ticket changes.

## Design notes

1. Start check. Run `make check` on the fresh worktree; if it is red, stop with
   `Verdict: BLOCKED base branch red` and the failing gate. Confirm that the base branch has: the
   behaviour columns, `settings_version` and `channel_skill_bindings` (M1-53), the effective operator
   and `setChannelOperator` (M1-54), `services/server/test/harness` with `BuildMatrixWorld` (M1-71),
   `services/server/test/canary` (M1-69), `listEvents` (M1-63) and `Authorizer.Observe` (M1-65).
   A missing one ends with `Verdict: BLOCKED dependency <ticket> missing`; all of them are in the
   closure of the header M1-69 (D-W-106).

2. Port `AgentConfig` in `internal/chat` (02 4.3 names it). The actor comes from `kernel/actor` in
   the context, never from a parameter. Pseudo-Go, follow the naming of the existing chat port:

   ```go
   // AgentConfig reads and changes channel behaviour and serves the operator its configuration.
   type AgentConfig interface {
       GetChannelBehavior(ctx context.Context, channelID ids.ID) (ChannelBehavior, error)      // channel.behavior.read
       PutChannelBehavior(ctx context.Context, cmd PutChannelBehaviorCmd) (ChannelBehavior, error) // channel.behavior.update
       ListServedChannels(ctx context.Context) ([]ServedChannelConfig, error)                   // agent.config.read
       GetServedChannelConfig(ctx context.Context, channelID ids.ID) (ServedChannelConfig, error) // agent.config.read
   }
   type BehaviorSettings struct {
       RequireMention, ThreadRequireMention, FreeResponse, AutoThread, AllowMentionedAgents bool
       HistoryBackfillLimit int
       Prompt               string
       Skills               []string // ordered skill references
   }
   type ChannelBehavior struct {
       BehaviorSettings
       SettingsVersion int64 // the project binding joins with M2-03
   }
   type PutChannelBehaviorCmd struct {
       ChannelID               ids.ID
       ExpectedSettingsVersion int64 // from If-Match
       Settings                BehaviorSettings
   }
   type ServedChannelConfig struct {
       ChannelID, SpaceID ids.ID
       Behavior           ChannelBehavior
   }
   ```

   No in-transaction event: 02 4.3 lists none for behaviour, and no other module reacts in the same
   transaction.

3. `getChannelBehavior` (`channel.behavior.read`):
   a. `Authorize(actor, channel.behavior.read, Resource{Kind: channel, ID})`. An unknown or
      unreadable channel gives `404 not_found` with exactly the body and headers of an unknown id
      (AZ-P4); a readable channel without `manage` gives `403 forbidden` with
      `reason: capability_missing`. Owners pass on every non-DM channel (05 2.3 point 1); agents
      and apps always end in `403` or `404` here.
   b. Read the columns and the skills ordered by `position` in one read; answer `200` with
      `ChannelBehavior` and `ETag` from `FormatETag(settings_version)` (06 8.1). The member
      `project_binding` is absent until M2-03. Archived channels stay readable (07 4.5).

4. `putChannelBehavior` (`channel.behavior.update`), one unit of work (`txn.UnitOfWork.Do`) without
   the `RightsChange` option, in this order:
   a. The chain answers first: schema and unknown members give `400 validation_failed`
      (SEC-IN-1); a missing `If-Match` or `*` gives `428 precondition_required`
      (`StagePreconditions`, M1-18).
   b. `Authorize(actor, channel.behavior.update, Resource{Kind: channel, ID})`: unreadable gives
      `404 not_found`; without `manage` `403 forbidden` with `reason: capability_missing`, without
      asking for step-up first (07 4.2 "Requirement before step-up"); a person whose step-up is older
      than `auth.step_up_max_age` gets `401 step_up_required`; a personal token with `manage` (a
      token without a session) gets `403 forbidden` with `reason: step_up_not_possible` (R-AZ-03;
      agent and app tokens never hold `manage`); an owner's personal token acts with the person's
      grants only (R-AZ-14). `authz`
      records every denial (Audit `yes`, R-AZ-04, AZ-P13).
   c. Locks: read the channel's `space_id`, lock the `spaces` row `FOR SHARE`, then the `channels`
      row `FOR UPDATE` (parent before child, as `setSpaceOperator` changes the space before its
      inheriting channels, M1-54 design note D4; 03 6.3 point 2). Reason: `setSpaceOperator` and `setChannelOperator` update these rows, so the
      effective operator read after the locks is the one at commit, and the audience of
      `agent.config_changed` (note 6) is exact. Deadlocks are retried by the unit of work
      (`database.tx_retry_max`).
   d. Compare `If-Match` with the locked `settings_version` (`ParseIfMatch`; a weak, list or
      non-decimal value counts as a mismatch). A mismatch gives `412 version_conflict` with
      `current` set to the `ChannelBehavior` at the locked version. Comparing after the lock makes
      two parallel requests with the same `If-Match` end in one `200` and one `412`.
   e. Validation (note 5): all failing fields in one `400 validation_failed` answer.
   f. State: an archived channel or a channel of an archived space gives `409 conflict` (ruling of
      M1-53: `manage` actions pass `authorize` on archived channels and chat rejects every state
      change). A read-only channel accepts the change (05 rule 1.8 keeps `manage` actions open).
   g. No-op: if the five switches, `history_backfill_limit`, the prompt (byte-equal) and the skills
      (same order) equal the stored state, answer `200` with the current state and the unchanged
      `ETag`; no version, no event; the audit allow entry carries `no_change: true` (06 "No
      effect").
   h. Write: update the behaviour columns and `settings_version = settings_version + 1`;
      `channels.version` stays, because the `Channel` object carries no behaviour field (M1-53
      design note D1) and `settings_version` is the own counter (03 `channels`); no
      `channel.updated`. When the skills changed, delete the channel's `channel_skill_bindings` rows
      and insert the request order with positions counting from zero.
   i. Events (note 6) and audit (note 7) in the same unit of work.
   j. Answer `200` with the new `ChannelBehavior` and its `ETag`.
   No permission change: `permission_version` stays and no `access.changed` is written (05 column
   Permissions `no`). No I1 check: guest channels are no I1 targets (10 5.2 last paragraph), and the
   operator does not change here. The operation has no `Idempotency-Key` (06 7, PUT is idempotent by
   HTTP semantics); a repeat after success gets `412 version_conflict` whose `current` equals the
   requested state.

5. Validation (Ruling for docs/DECISIONS.md where 03 leaves the answer open):
    - `history_backfill_limit` from zero to `channel.history_backfill_limit_max` inclusive (03
      `channels`). Below zero is a schema error (`minimum: 0`, code `too_small` of M1-18); above the
      maximum gives pointer `/history_backfill_limit`, code `too_large`. The check uses the configured
      value at request time. A stored value above a later lowered maximum is read back unchanged; a
      PUT that keeps it fails until the value is lowered.
    - `prompt`: UTF-8 length in bytes up to `channel.prompt_max_bytes` (03); above gives pointer
      `/prompt`, code `too_long`, as `400 validation_failed` and not `413 payload_too_large`, because
      it is a field rule (06 5 point 4). The prompt is stored unchanged: no trimming, no character
      check (it is a multi-line instruction, not a display text of SEC-IN-4). No error `detail` ever
      contains prompt text (06 5 point 3).
    - `skills`: each reference non-empty (`/skills/<i>`, `required`), without C0 or C1 control or
      bidi control characters (`invalid_characters`, the shared validator `kernel/textcheck` of M1-26
      that M1-53 uses for names), and not repeated in the list (`duplicate`, mirrors the unique index
      `(channel_id, skill_ref)`). References are stored as sent and never interpreted (03). No count or
      length limit beyond `http.max_json_body_bytes` (spec gap reported).

6. Events (05 6; 04 2.2 and 9 point 3; 09 4.4), appended through `eventlog.Appender` in the work
   phase, in this order:
    - `channel.settings_changed` on every effective change: scope `{space_id, channel_id}`, resource
      `{kind: channel, id}` without version, receive action `channel.read`, no audience, payload
      `ChannelSettingsChangedPayload {channel_id, settings_version}`.
    - `agent.config_changed` only when the prompt or the skills changed (Ruling: 05 6 and 09 4.4
      name prompt, skills and project binding; a switch-only change reaches the operator through
      `channel.settings_changed`, which it receives by the operator rule) and only when the channel
      has an effective operator after the locks of note 4c: audience that agent, receive action
      `agent.config.read`, the same scope and resource, payload
      `AgentConfigChangedPayload {channel_id, settings_version}`. A channel without effective
      operator gets no `agent.config_changed`.
    - Payloads never carry prompt, skills or project binding (04 2.2 rule 1, R-AZ-09); a unit test of
      the event builder proves the payload has exactly the two members.
    - One function in `internal/chat/app` raises `settings_version` and appends these events, with a
      flag "configuration changed" that decides on `agent.config_changed`; M2-03 calls it for project
      binding changes, because `settings_version` counts the project too (03 `channels`).

7. Audit (05 2.4; 07 4.2 "Allow decisions of an action with audit `yes` are written by its use case";
   19 SEC-AUD-5): one allow entry per successful request through `auditing.Recorder.Record` in the
   unit of work: action `channel.behavior.update`, resource kind `channel` and the channel id;
   details `space_id`, `previous_settings_version`, `settings_version`, `changed` (list of codes from
   the closed set `require_mention`, `thread_require_mention`, `free_response`, `auto_thread`,
   `allow_mentioned_agents`, `history_backfill_limit`, `prompt`, `skills`), the new values of the
   five switches and of `history_backfill_limit`, and `skills_count`. A no-op entry has
   `no_change: true` and equal versions. Never the prompt text and never a skill reference (free
   text, SEC-AUD-5). Ruling for docs/DECISIONS.md.

8. Permissions in short (05 4.5 and 4.10; 07 4.2 and 4.3): `channel.behavior.read` and
   `channel.behavior.update` evaluate `Cap(manage)` on the channel path; `agent.config.read`
   evaluates the atom `ChannelOperator` of M1-28 (kind `agent` and the channel's effective
   operator; its failure reason is `subject_kind_not_allowed`). Visibility comes first: whoever
   cannot read the channel gets `404 not_found`. The operator rule gives the operator at least
   `write`, so it always sees its channels (07 2.9). Apps never pass: they hold no `manage` and are
   not agents (05 2.6, 11 "Never allowed").

9. `listServedChannels` (`agent.config.read`):
   a. Lists count through `Scope` (02 9.3), and M1-28 allows `Scope` only for read-level actions.
      Extend `Scope` for `agent.config.read` (Ruling for docs/DECISIONS.md): liveness and pending
      rules as for every call; for a principal of kind `agent`, `ChannelIDs` are the non-archived
      channels whose effective operator it is (`StructureSource.ChannelsOperatedBy`, M1-28 and
      M1-54), after the `ActingFor` intersection that every `Scope` applies (filled from M2-14);
      `SpaceIDs` are their spaces; `FullyVisibleSpaceIDs` is empty and `InstanceWide` false. For
      every other kind `Scope` returns the `forbidden` error with reason `subject_kind_not_allowed`,
      the reason the requirement gives, so a person gets `403 forbidden`. `Scope` records
      `(agent.config.read, outcome)` in the check log of M1-18.
   b. chat loads the channels with `id = ANY($channel_ids) AND space_id = ANY($space_ids) AND
      archived_at IS NULL` and their skills with `channel_id = ANY($channel_ids)`, ordered by
      channel id ascending (Ruling). Both predicates come from the same `Scope` (07 7.4 rule 1).
   c. Per item `Authorize(agent.config.read, channel)` in filter mode (`authorize.Filter(ctx)`, no
      denial audit); an item that fails is dropped, logged without ids and counted in
      `bothy.authz.errors` with `bothy.reason = filter_mismatch` (07 7.4 rule 3).
   d. `200` with `AgentChannelConfigList {items}`, each item `AgentChannelConfig {channel_id,
      space_id, behavior}`; unpaginated (06 6 point 6; the schema has no cursor). Archived channels
      never appear.

10. `getServedChannelConfig` (`agent.config.read`): `Authorize(agent.config.read, channel)`. Not
    readable gives `404 not_found` (AZ-P4); readable by a caller that is not the effective
    operator (a person, an app, another agent with a grant) gives `403 forbidden` with
    `reason: subject_kind_not_allowed`. Then read as in note 3b; `200` with `AgentChannelConfig` and
    `ETag` from `settings_version`. An archived channel stays readable for its operator (07 4.5).
    The fixed operator of a thread whose channel now has another effective operator reads nothing
    here (05 4.10 "agent-self and effective operator"; Ruling, spec gap reported). No audit (05).

11. Delivery of the two events (05 6; 04 8.1; 02 11.3):
    a. Admission: 05 6 requires `authorize(receiver, receive_action, resource)` for every receiver
       and, for personal events, that the receiver is the audience. `agent.config_changed` is the
       first personal event whose receive action is not `self` (03 `event_log.receive_action`).
       The single admission rule of `internal/sync` (`Admit`, used by `listEvents` of M1-63 and the
       hub of M1-65) must require both for such an event: the receiver is the audience, and
       `authorize` with `agent.config.read` on the channel allows in filter mode. M1-63's design
       admits audience events without the action; if the merged code does so, add the second
       condition (Ruling). Effect: an agent that stopped operating the channel before it catches
       up does not receive the event.
    b. Gateway permission cache: `authz` channel facts carry the mention switches (`ChannelFacts` of
       M1-28) that `agent.invoke` evaluates, and in delivery mode M1-65 refreshes cached paths only
       on the structure events of 04 8.1. Add `channel.settings_changed` to the events on which
       `Authorizer.Observe` advances the structure generation (Ruling; spec gap reported for
       04 8.1), so delivery-time decisions from M2-08 on never use stale switches.

12. SQL visibility (07 7.4 rule 1, AZ-P12): `channels` and `channel_skill_bindings` are content
    tables of the lint of M1-70. The list queries of note 9b carry the predicate. The single-channel
    reads after an allowing `authorize` (`getChannelBehavior`, `getServedChannelConfig`, the locked
    read of `putChannelBehavior`) go into `tools/lint/sql_visibility.yaml` with category
    `after_authorize` and a reason, if the lint is in the base branch; otherwise list them in the
    chat README so M1-70 adds them. Writes are outside the lint.

13. Configuration: `channel.prompt_max_bytes`, `channel.history_backfill_limit_default` and
    `channel.history_backfill_limit_max` are typed chat keys of chapter 03. Register
    `channel.prompt_max_bytes` in the chat config keys of `internal/wire` if M1-53 did not (integer,
    minimum one, default from 03 9). If no cross-check holds `channel.history_backfill_limit_default`
    at most `channel.history_backfill_limit_max`, add it to `internal/wire/configchecks.go` with a
    test, so a new channel never starts with a value its own PUT would reject. Run
    `make config-doc-check`.

14. Canary (SEC-T10; the plan row; M1-69 leaves the prompt to the ticket that builds its way):
    - `values.go`: a new class `manage_only` for the prompt and skill references. Allowed only in
      `2xx` bodies of `getChannelBehavior`, `putChannelBehavior`, `listServedChannels` and
      `getServedChannelConfig`, and in `current` of a `412 version_conflict` answer of
      `putChannelBehavior` (the caller passed `manage` before the version check). Never in logs,
      traces, metrics or audit rows, never in any other response (including `getChannel`,
      `listSpaceChannels`, `getSnapshot` and every error body), never in any event frame, catch-up
      page or snapshot of any harness connection, including readers of the channel and the operator
      agent.
    - `scenario.go` on the matrix world: the owner sets a canary prompt and a canary skill reference
      on Support/Allgemein (fresh step-up); Lotse calls `listServedChannels` and
      `getServedChannelConfig` and pages `listEvents`; Alex (reads Support, no `manage`) is an
      additional observer for this class with a gateway connection, `listEvents` and `getSnapshot`,
      and also sends `getChannelBehavior` (`403`) and a PUT with a canary body (`403`); the owner
      sends a stale PUT (`412`), a PUT whose canary prompt exceeds `channel.prompt_max_bytes`
      (`400`) and a switch-only PUT. The coverage test then finds the four operations.
    - `scan_test.go`: rows of `TestSEC_T10_ScanAllowMatrix` for the new class per sink, and a case
      where a `manage_only` value inside an event frame of a reader is a finding.
    - `README.md`: the class and why it is stricter than `content`.

15. Rulings for `docs/DECISIONS.md` section 10 (one row `D-M2-02-<n>` each, with reason): check order
    and locks of note 4; the validation codes and the stored-value rule of note 5; the event rule of
    note 6 (switch-only changes without `agent.config_changed`, audience fixed after the locks,
    order of the two events, shared function for M2-03); the audit details of note 7; `Scope` for
    `agent.config.read`, list order and archived channels of note 9; no channel configuration for a
    fixed thread operator of note 10; the admission rule and the cache event of note 11;
    `project_binding` absent until M2-03.

## Tests (write them first)

Unit:

- `internal/chat/domain`: `TestValidateBehavior` (table: backfill zero, at the configured maximum
  and one above; prompt at `channel.prompt_max_bytes` and one byte above; empty skill reference,
  control and bidi characters, duplicate reference; several failures reported together; limits come
  from `BehaviorLimits`, never literals), `TestChangedFields` (every field alone and combined, order
  of skills counts), `TestConfigChanged` (prompt or skills: true; switches or backfill only: false).
- `internal/chat/app`: `TestSettingsChangedEvents` (with configuration change and operator: two
  events in order; without operator: one; payloads have exactly `channel_id` and `settings_version`;
  no resource version; audience and receive actions as note 6).
- `internal/authz`: `TestScope_AgentConfigRead` (an agent gets exactly its non-archived operated
  channels and their spaces; an archived operated channel is absent; a person and an owner get
  `forbidden` with `subject_kind_not_allowed`; the check log holds one `agent.config.read` entry;
  an `ActingFor` principal gets the intersection), `TestAZ_P8_ScopeAgentConfigRead` (the property
  test of M1-28 extended: in random worlds the `Scope` set for `agent.config.read` equals the
  non-archived channels on which `Authorize(agent.config.read)` allows), and
  `TestAuthzObserve_SettingsChangedAdvancesGeneration` (a cached path with the old
  `require_mention` is recomputed after `channel.settings_changed`).
- `internal/sync`: `TestAdmit_PersonalEventWithReceiveAction` (`agent.config_changed` with audience
  Lotse: admitted for Lotse while it operates the channel, not admitted after the operator changed,
  never admitted for another principal; personal events with `self` unchanged).
- `internal/wire`: the config tests cover `channel.prompt_max_bytes` and the cross-check of note 13.

Fuzz:

- `FuzzValidateBehavior` in `internal/chat/domain`: arbitrary prompt bytes and skill lists never
  panic, and every accepted input satisfies the limits and the skill rules.

Integration (PostgreSQL 18 via `pgtest`, the real `internal/wire` graph and HTTP chain through the
shared harness `test/harness` of M1-71 with `BuildMatrixWorld`: owner, Alex, Sam, Robin; spaces
Support, Marketing, Engineering, Research; agents Lotse (operator of Support), Texter (operator of
Marketing), Runner (operator of Engineering/Werkzeuge), Assistent; grants G-1 to G-5; every response
checked by the response validator; extra grants through `createGrant`, never through SQL):

- `TestR_AZ_09_BehaviorReadableOnlyWithManage`: the owner gets `200` with all members and `ETag`;
  Robin with an added grant `read` plus `manage` on Support/Allgemein gets `200`; Alex (G-1,
  `trigger` without `manage`) gets `403 forbidden` with `reason: capability_missing`; Lotse (operator
  rule, `write`) gets `403 forbidden`; Sam (no read on Support) gets `404 not_found`.
- `TestAZ_P4_ChannelBehaviorForeignEqualsUnknown`: `getChannelBehavior` by Sam on
  Support/Allgemein and `getServedChannelConfig` by Texter on Research/Notizen, each versus a random
  UUIDv7: equal status, headers and body after removing `correlation_id` and `instance`, and median
  response times within `authz.not_found_timing_tolerance`.
- `TestPutChannelBehavior_Updates`: the owner with fresh step-up and `If-Match` of the current
  version changes every field: `200`, `settings_version` plus one, `ETag` equal to it, columns and
  skill rows (positions from zero, request order) as sent, `channels.version` unchanged, no
  `channel.updated`, `permission_version` unchanged, no `access.changed`.
- `TestPutChannelBehavior_SettingsVersionPerChange`: three different changes raise
  `settings_version` by one each; an identical PUT answers `200` with the same version and `ETag`,
  writes no event and one audit entry with `no_change: true`.
- `TestPutChannelBehavior_Preconditions`: without `If-Match` `428 precondition_required`; a stale
  version `412 version_conflict` whose `current` is the stored `ChannelBehavior`; a repeat of a
  successful PUT with its old `If-Match` gets `412` with `current` equal to the requested state.
- `TestPutChannelBehavior_StepUpAndRights`: the owner with a step-up older than
  `auth.step_up_max_age` (harness clock) gets `401 step_up_required` and nothing changes; Alex gets
  `403 forbidden` (`capability_missing`), not `401`; Lotse's agent token gets `403 forbidden`
  (`capability_missing`, agents never hold `manage`); with personal tokens enabled as the tests of
  M1-44 do, the owner's personal token gets `404 not_found` (it carries only the person's grants,
  none here, R-AZ-14) and Robin's personal token, after Robin got `manage` on Support/Allgemein,
  gets `403 forbidden` with `reason: step_up_not_possible`; Sam gets `404 not_found`.
- `TestPutChannelBehavior_Validation`: each rule of note 5 gives `400 validation_failed` with its
  pointer and code; the problem body never contains the submitted prompt; values exactly at the
  configured limits are accepted; an unknown member gives `400 validation_failed`.
- `TestPutChannelBehavior_ArchivedChannel`: after `archiveChannel` a PUT gives `409 conflict` and
  `getChannelBehavior` still answers `200`.
- `TestPutChannelBehavior_Events`: a prompt change on Support/Allgemein writes exactly
  `channel.settings_changed` then `agent.config_changed` with consecutive `seq`; scope, resource
  without version, receive actions and audience (Lotse) as note 6; both payloads validate against
  `ChannelSettingsChangedEvent` and `AgentConfigChangedEvent`; no JSON `null`. A switch-only change
  writes only `channel.settings_changed`. On a channel whose operator is set to `none` a prompt
  change writes only `channel.settings_changed`.
- `TestR_AZ_09_PromptNeverInEvents`: after several changes with a unique prompt and skill
  reference, no row of `event_log` (payload as text) and no row of `audit_events` (details as text)
  contains either value.
- `TestPutChannelBehavior_Audit`: one allow entry per successful request with the details of
  note 7; a PUT by Alex and one with a stale step-up leave deny entries (`capability_missing`,
  `step_up_required`) after the aggregator flush of M1-24.
- `TestPutChannelBehavior_ConcurrentPuts`: two PUTs with the same `If-Match` started behind a
  barrier: exactly one `200` and one `412`; `settings_version` raised once; one pair of events.
- `TestPutChannelBehavior_ConcurrentOperatorChange`: a prompt change on Support/Allgemein in
  parallel with `setSpaceOperator` of Support to Texter: both end with a defined status (no deadlock
  reaches the client) and the audience of `agent.config_changed` is the operator that is effective
  at the event's position in `seq` order.
- `TestListServedChannels_OnlyOperatedChannels`: Lotse lists exactly Support/Allgemein and
  Support/Eskalation, each with `space_id` and the full `behavior`, ordered by channel id; Texter
  with an added `read` grant on Support/Allgemein lists only Marketing/Kampagnen; an archived channel
  operated by Lotse is absent; the owner and Alex get `403 forbidden` with
  `reason: subject_kind_not_allowed`.
- `TestListServedChannels_FollowsOperatorChange`: after `setChannelOperator` on Support/Eskalation
  to Runner, Lotse's next list lacks it and Runner's has it, in the next request.
- `TestGetServedChannelConfig_Rights`: Lotse on Support/Allgemein gets `200` with `ETag`; Texter
  with a `read` grant there gets `403 forbidden` (`subject_kind_not_allowed`); Texter on
  Research/Notizen gets `404 not_found`; the owner gets `403 forbidden`; after a thread in
  Support/Allgemein was created with operator Lotse and the channel's operator was set to Runner,
  Lotse gets `403 forbidden` there (note 10).
- `TestAgentConfigChanged_Delivery`: after a prompt change, Lotse's `listEvents` and Lotse's gateway
  connection carry both events; Alex's carry `channel.settings_changed` only; Sam's carry neither.
- `TestAgentConfigChanged_ReceiveActionAtDelivery`: a prompt change, then `setChannelOperator` to
  Runner, then Lotse's first `listEvents` from before the change: the `agent.config_changed` with
  audience Lotse is not delivered.
- `TestAZ_P1_RouteCount` (existing): passes with the four new routes; AZ-P2 stays green with both
  event types.

Contract: the Schemathesis contract tests of M1-71 (part of `make check`) read the enabled list and
cover the four operations; they stay green.

End-to-end (canary, `test/canary`, M1-69 extended as note 14):

- SEC-T10: `TestSEC_T10_CanaryAcrossAllOutputs` with the prompt and skill canaries ends with zero
  findings; `TestSEC_T10_ScenarioCoversEveryEnabledOperation` finds the four operations;
  `TestSEC_T10_ScanAllowMatrix` has the new class rows and the reader-event finding case.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/chat/... ./internal/authz/... ./internal/sync/... ./internal/wire/... ./test/integration/... ./test/canary/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make register-check`, `make config-doc-check`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `getChannelBehavior`, `putChannelBehavior`, `listServedChannels` and `getServedChannelConfig`
      are enabled and registered; AZ-P1 and AZ-P2 pass; generated code has no drift.
- [ ] The prompt is readable only with `manage` (or as effective operator through
      `agent.config.read`) and appears in no event (R-AZ-09); the problem body of a failed request
      never contains it.
- [ ] `settings_version` rises by exactly one per effective change; a no-op changes nothing and is
      audited with `no_change: true`; `channels.version` stays.
- [ ] A change needs fresh step-up and `If-Match`; it is audited with the details of note 7 and
      denials are audited.
- [ ] `channel.settings_changed` and, for prompt or skill changes, `agent.config_changed` to the
      effective operator are written in this order with only `channel_id` and `settings_version`.
- [ ] The agent lists and reads only channels it operates; others get `403 forbidden` or
      `404 not_found` as note 10; invisible equals unknown (AZ-P4).
- [ ] `agent.config_changed` reaches only its audience and only while it passes
      `agent.config.read`; the gateway cache follows `channel.settings_changed`.
- [ ] The SEC-T10 canary scenario carries the prompt and a skill reference and finds no leak.
- [ ] Rulings recorded in `docs/DECISIONS.md` section 10; T-AI-09 names SEC-T10 as its test.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `chat`, `authz`, `sync` (if changed) and `test/canary` updated (port `AgentConfig`, operations, events, the `Scope` extension, the cache event, the query exceptions of note 12).
- Rulings of note 15 recorded in `docs/DECISIONS.md` section 10 (one row each with date, decision, reason and place).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`), with attention to T-AI-09: can any principal without `manage` obtain prompt or skills through a route, error body, event, snapshot, log, trace or audit row, and can an agent read the configuration of a channel it does not operate; on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
