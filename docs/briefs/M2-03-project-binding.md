# M2-03: Project binding and readiness

Status: ready | Depends on: M2-02 | Milestone: M2

## Goal

After this ticket an owner with a fresh step-up binds a forum to a development project and removes that binding again (`putProjectBinding`, `deleteProjectBinding`, action `channel.project.bind`). The binding is a row of `project_bindings` in the module `chat`: an opaque `workspace_ref` that the runner resolves, a display-only `repository_url`, a `default_branch`, a `runner_config` after the schema `project_runner_config.v1`, and a `ready` flag that only the serving runner sets through its heartbeat, by a port this ticket provides. `chat` feeds `ProjectBound` and `ProjectReady` into the facts of `authorize`, so `agent.invoke` in a bound forum is denied with `project_not_ready` until the runner has reported its workspace ready, and every binding change is announced only as a new `settings_version` (`channel.settings_changed`, `agent.config_changed`), never with its content. This is the function "Dev forum with project binding and runner agent" of 01 ("triggering only with `project_bindings.ready`") and point 7 of 05 "Agent invocation `agent.invoke`"; the heartbeat (M2-20), the trigger decision (M2-04), the delivery check (M2-08) and the Claude-Code runner (M2-34) build on it.

## Read first

- `docs/spec/03-data-model.md`: "Principles" points 7, 8, 9, 10, 12 and 13; table `project_bindings` (every row, PK, unique `lower(workspace_ref)`, FKs, retention `config`); table `channels` (`kind`, `archived_at`, `settings_version`, the formula "Effective operator"); `spaces.default_operator_agent_id`; `event_log` (`audience_principal_id`, `receive_action`, `resource_version`); `audit_events.details`; "Transaction rules" 6.3 "Lock order".
- `docs/spec/05-actions.md`: rules 1.1, 1.2 and 1.8; 2.1 (value `owner`), 2.3 "Effective level" point 1, 2.4 'Columns "Owner", "Step-up", "Audit", "Permissions"'; 4.5 rows `channel.project.bind`, `channel.behavior.read`, `channel.behavior.update`, `channel.operator.set`; 4.10 rows `agent.invoke`, `agent.heartbeat`, `agent.config.read`; section 5 "Agent invocation `agent.invoke`" (point 7 is the readiness rule; the plan row cites it as point 6, point 6 is about interactions); section 6 rows `channel.settings_changed` and `agent.config_changed`; section 8 R-AZ-01, R-AZ-03, R-AZ-04, R-AZ-09, R-AZ-10, R-AZ-14.
- `docs/spec/07-permissions.md`: 4.1 "Decisions and reasons" rows `owner_only`, `step_up_not_possible`, `step_up_required`, `project_not_ready`, `dev_environment_not_ready`, `channel_archived`; 4.2 "Flow"; 4.5 "Archived spaces and channels"; SR2 "Tool class", SR3 "Dev isolation", SR5; 10 "Examples" (Runner as operator of the forum Engineering/Projekt-A with project binding); 11.1 "Test matrix" rows AZ-M15 (owned by V2-02) and AZ-M19 (owned by M2-08); 11.2 "Required tests" AZ-P1, AZ-P4, AZ-P13.
- `docs/spec/09-agents.md`: 3.4 "Decision per message" point 7; 3.5 "Reasons for rejection at acceptance" row `project_not_ready`; 4.4 "Events to the adapter" row `agent.config_changed` and the payload rule below the table; 4.8 "Heartbeat and reported state" row `projects`; 5.2 "Reasons (`runs.state_reason`)" row `project_not_ready`; 18.1 "Project and thread"; 18.2 "Process contract" (`--model`, the flags per tool class); 21 "Configuration values of this chapter".
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope" (field `audience`, payload rule 1: project binding never in events); 9 "Delivery guarantee" point 3 (no `resource.version` for `channel.settings_changed` and `agent.config_changed`).
- `docs/spec/06-api.md`: 2 "Basic rules" row "Unknown values"; 4 "Authorization in the API" points 3 and 6; 5 "Errors" (`errors[]` with `pointer` forms, the closed code list); 7 "Idempotency" row "PUT and DELETE"; 8 "Concurrency" points 1, 3 and 5.
- `docs/spec/02-architecture.md`: 4.3 "Ports, events and sources per module" (row `chat`: `AgentConfig`, implements `authz.StructureSource`), 4.4 "Allowed dependencies" (`agents` may use `chat`), 4.6 "Actions per module" (`agent.config.read` belongs to `chat`), 8.1 to 8.3 (unit of work, lock order, in-transaction events).
- `docs/spec/19-security.md`: SEC-IN-1, SEC-IN-4, SEC-OUT-1, SEC-AUD-5, SEC-T07.
- `docs/spec/21-dev-isolation.md`: status paragraph and section 1 (until V2 only owners trigger dev projects).
- `docs/spec/01-product.md`: 3.1 "Structure and conversations" row "Dev forum with project binding and runner agent".
- `docs/api/openapi.yaml`: operations `putProjectBinding`, `deleteProjectBinding`, `getChannelBehavior`, `putChannelBehavior`, `listServedChannels`, `getServedChannelConfig`; schemas `ProjectBinding`, `ProjectBindingPut`, `ChannelBehavior` (member `project_binding`), `AgentChannelConfig`, `AgentChannelConfigList`, `Heartbeat` (member `project_readiness`), `ProjectReadiness`, `ChannelSettingsChangedEvent`, `ChannelSettingsChangedPayload`, `AgentConfigChangedEvent`, `AgentConfigChangedPayload`, `JsonObject`, `Problem`, `FieldError`; responses `Conflict`, `PreconditionFailed`; parameters `ChannelIdPath`, `IfMatchOptional`.
- `docs/api/asyncapi.yaml`: messages `event.channel.settings_changed` and `event.agent.config_changed` (`x-audience: self`). `docs/api/webhooks.md`: rows `channel.settings_changed` and `agent.config_changed`. `docs/spec/actions.yaml`: entry `channel.project.bind` and the event entries `channel.settings_changed`, `agent.config_changed`.
- ADRs 0013 (spec first), 0017 (one authorize function, 404 before 403), 0022 (agents outside the server process), 0046 (multi-user dev environment only with isolation), 0053 (no JSON `null`).
- `docs/DECISIONS.md`: D-W-100 (briefs written before M1 closed), section 10 (format of `D-<ticket>-<n>` rows).
- `docs/security/threat-model.md`: T-AI-02 (confused deputy), T-DEV-01 (dev runner as a back door to the host), T-AZ-01, T-AZ-04, T-IN-01.
- `docs/operations/configuration.md`: `audit.reason_max_chars`, `auth.step_up_max_age`, `database.tx_retry_max`, `http.max_json_body_bytes`, `audit.deny_aggregate_window`.
- `docs/PLAN.md`: section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope "09 without dev isolation", Out of scope "SR3 and dev environments (V2)", Red team); rows M2-02, M2-03, M2-04, M2-08, M2-20, M2-34 and V2-02.
- M1 briefs whose code this ticket uses: M1-28 (`authz` sources: `ChannelFacts.ProjectBound`, `ChannelFacts.ProjectReady`; guard `agent.invoke` step 6; decision to error mapping), M1-53 (D2 archived channels, D7 channel part of `StructureSource`, migration pattern), M1-54 (operator use cases, D2 order of checks, the world builder under `services/server/test/integration/`), M1-55 (thread part of `StructureSource`, `chat.Threads.Create`), M1-62 (structure of a chat use case), M1-17 (`api/operations.yaml`), M1-18 (W7 closed request schemas, field error codes, `If-Match` helpers), M1-13 (`txn.UnitOfWork`), M1-16 (`eventlog.Appender`), M1-24 (`auditing.Recorder`, deny aggregator), M1-26 (`kernel/textcheck`), M1-35 (step-up, `identitytest`), M1-07 and M1-08 (configuration keys and reference).
- Code: `internal/chat` (port, spaces, channels, operators, threads, behaviour of M2-02), `internal/authz` (decide, guards, sources), `internal/kernel/authorize`, `internal/kernel/auditing`, `internal/kernel/eventlog`, `internal/kernel/txn`, `internal/kernel/textcheck`, `internal/wire`, `internal/platform/pg/pgtest`, `services/server/api/operations.yaml`, `services/server/migrations/`.
- Dependencies: the header names only M2-02. Everything M2-02 depends on (M1-69 and its closure, among them M1-28, M1-35, M1-49, M1-51, M1-53, M1-54, M1-55, M1-65 and M1-71) is implied. This brief also needs M1-55 (thread paths in `StructureSource`), which is in this closure; Design note 0 checks it.

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

11. Both operations use the action `channel.project.bind`: Permission `owner`, Owner `yes`, Step-up `yes`, Audit `yes`, Permissions `no`, rate-limit class `rl.admin` (05 4.5, openapi `x-authz-action`). Only owners set a project binding (R-AZ-10); step-up actions only by humans in a session (R-AZ-03, 05 2.4); owner permissions never through a personal API token (R-AZ-14).
12. A binding exists only for a channel with `kind = 'forum'`, at most one per forum (PK `channel_id`); `workspace_ref` is opaque, unique case-insensitively and never an absolute path of the server; `repository_url` is display only and the server never fetches it; `runner_config` follows the schema `project_runner_config.v1` and holds no secrets (03 `project_bindings`, principles 8 and 9; 09 18.1).
13. `ready` starts as `false`; the runner reports readiness in its heartbeat and the server sets `project_bindings.ready` (03 `project_bindings.ready`; 09 4.8 row `projects` and 18.1; openapi `Heartbeat.project_readiness`).
14. In a forum with a binding, `agent.invoke` requires `project_bindings.ready = true`, otherwise `forbidden` with reason `project_not_ready`; SR3 applies on top, so until V2 only owners trigger there and non-owners get `dev_environment_not_ready` (05 section 5 point 7; 07 4.1 and SR3; 09 3.5; 21 section 1).
15. The binding never appears in events. A change raises `channels.settings_version` and is announced by `channel.settings_changed` (receive action `channel.read`) and `agent.config_changed` (receive action `agent.config.read`, audience the agent), both carrying only `channel_id` and `settings_version` and no `resource.version` (04 2.2 payload rule 1 and 9 point 3; 05 section 6; 09 4.4; 03 `channels.settings_version`).
16. Last write wins: `If-Match` is optional on `putProjectBinding` and checked when present (`412 version_conflict` with `current`); the `ETag` is `"<version>"`; every change raises `version` by one (06 8 points 1 and 3; 03 principle 10).
17. PUT and DELETE are idempotent by HTTP semantics and take no `Idempotency-Key`; a resource without tombstone answers `404` after its deletion (06 7 row "PUT and DELETE").
18. No read permission gives `404`, identical to an unknown ID; `403` only with read permission (05 rule 1.2, R-AZ-01). Archived forums and forums of archived spaces are frozen (07 4.5).
19. The mandatory reason has no control or bidi characters and at most `audit.reason_max_chars` characters; it is the only free text in the audit details (SEC-AUD-5, SEC-IN-4). Unknown request members give `400 validation_failed` (SEC-IN-1, 06 2 "Unknown values").
20. The binding is readable with `manage` through `channel.behavior.read` (`ChannelBehavior.project_binding`) and by the serving agent through `agent.config.read` (05 4.5 and 4.10; R-AZ-09; 02 4.6).

## Scope

All paths under `services/server/` unless they start with `docs/`. This ticket creates no new top-level Makefile target.

Create:

- `migrations/<next number>_chat_project_bindings.sql` (goose, numbering as the existing files): table `project_bindings` exactly as 03 defines it (columns, defaults, PK `channel_id`, FK `channel_id -> channels(id) ON DELETE CASCADE`, FK `created_by -> principals(id) ON DELETE RESTRICT`, unique index on `lower(workspace_ref)`), privileges for the application role following the pattern of the existing migrations (`grep -n GRANT migrations/*.sql`), and a Down section. The rule "only `kind = 'forum'`" cannot be a CHECK across tables; the use case enforces it (Design note 3) and a comment in the file says so.
- `internal/chat/domain/project_binding.go`: the value types `ProjectBinding`, `RunnerConfig`, `ProjectBindingInput`, the pure validation `ValidateProjectBinding(in ProjectBindingInput, limits BindingLimits) []FieldError` (Design note 4; use the field error type the chat domain already returns for `400 validation_failed`) and the pure change rule `PlanBindingPut(current *ProjectBinding, in ProjectBindingInput) BindingChange` returning the kind (`created`, `replaced`, `unchanged`), the next row and whether `ready` is reset (Design note 5).
- `internal/chat/app/project_binding.go`: the use cases `PutProjectBinding`, `DeleteProjectBinding` and `ApplyProjectReadiness`.
- `internal/chat/adapters/postgres/queries/project_bindings.sql`: lock, read, insert, update and delete of a binding, the readiness update, the readiness reset for a set of channels, and the project facts for `StructureSource`; regenerate the sqlc output.
- Tests named in "Tests" below, including `test/integration/project_binding_test.go` (cross-module, all real modules).

Change:

- `internal/chat` (port): a new interface `ProjectBindings` with DTOs and doc comments (pseudo-Go; follow the conventions already used in `internal/chat`; the actor comes from `kernel/actor` in the context):

```go
// ProjectBindings binds forums to development projects (05 `channel.project.bind`) and takes the
// readiness a runner reports (09 4.8). All methods join an ambient unit of work when one exists.
type ProjectBindings interface {
    PutProjectBinding(ctx context.Context, cmd PutProjectBindingCmd) (ProjectBinding, error)
    DeleteProjectBinding(ctx context.Context, channelID ids.ID) error
    ApplyProjectReadiness(ctx context.Context, agentID ids.ID, reports []ProjectReadiness) error
}
type PutProjectBindingCmd struct {
    ChannelID       ids.ID
    WorkspaceRef    string
    RepositoryURL   *string      // absent member: nil
    DefaultBranch   *string
    RunnerConfig    RunnerConfig // absent member: empty config
    Reason          string
    ExpectedVersion *int64       // from If-Match; nil when the header is absent
}
type RunnerConfig struct { Model *string; AllowedTools []string }
type ProjectReadiness struct { ChannelID ids.ID; Ready bool } // openapi `ProjectReadiness`
```

- The chat adapter that implements `authz.StructureSource` (find it with `grep -rn "StructureSource" internal/chat`): fill `ChannelFacts.ProjectBound` and `ChannelFacts.ProjectReady` from `project_bindings` in every method that returns channel facts (Design note 8).
- The chat operator use cases of M1-54 (`SetChannelOperator`, `SetSpaceOperator`): reset readiness of bound forums whose effective operator changes (Design note 7).
- The behaviour and agent configuration read models of M2-02 (`getChannelBehavior`, `putChannelBehavior`, `listServedChannels`, `getServedChannelConfig`): add `project_binding` when the forum has a binding (Design note 10).
- `internal/chat/adapters/httpapi`: strict-server methods for `putProjectBinding` and `deleteProjectBinding`, including the `ETag` of the `200` answer.
- `api/operations.yaml`: move `putProjectBinding` and `deleteProjectBinding` from `later` to `enabled` (sorted); regenerate `api/rest` and `api/routes` with the generator documented in `api/README.md`.
- `internal/wire`: wire the `ProjectBindings` implementation and the HTTP handlers; register the configuration key of Design note 4 next to `channel.name_max_chars` (find it with `grep -rn "channel.name_max_chars" internal`), with `Chapter` naming `09-agents.md` (add a constant `chapter09` to `internal/wire/configchapters.go` if it does not exist). No module consumes `ApplyProjectReadiness` yet; M2-20 wires `agents` to it.
- `docs/api/openapi.yaml` (spec first, Design note 11): new schema `ProjectRunnerConfig`, referenced from `ProjectBinding.runner_config` and `ProjectBindingPut.runner_config`; `minLength: 1` on `ProjectBindingPut.reason`; response `'409'` (`Conflict`) on `deleteProjectBinding`; descriptions of both operations state the rules of Design notes 3, 5 and 6. Rerun `make api-lint`, `make api-checks` and `make codegen-check`.
- `docs/spec/09-agents.md`: 18.1 defines `project_runner_config.v1` (Design note 11); 21 gets the row of the new key `project.binding_text_max_chars`; regenerate `docs/operations/configuration.md` with `make config-reference`.
- `services/server/test/canary` and `services/server/test/harness` (M1-69 and M1-71, in the closure through M2-02; Design note 13).
- `tools/lint/sql_visibility.yaml`, only if M1-70 is merged (Design note 12).
- Documentation: `internal/chat/README.md` (port `ProjectBindings`, table `project_bindings`, action `channel.project.bind`, readiness and its reset, the query exceptions), `services/server/CHANGELOG.md` (Unreleased), `docs/DECISIONS.md` section 10 rows `D-M2-03-1` to `D-M2-03-8` (Design note 15).

## Out of scope

- The heartbeat operation `sendHeartbeat`, presence and the mapping of `Heartbeat.project_readiness` onto `ApplyProjectReadiness` (M2-20); this ticket provides and tests the port.
- The trigger decision as a pure function and its rejection table (M2-04), runs and `trigger_state` with reason `project_not_ready` on a message (M2-05), the re-check before every delivery and the run reason `project_not_ready` (M2-08), the problem code `403 agent_invocation_denied` of triggering REST operations (M2-09 `renewInvocationToken`, M2-15 interactions, M2-16 commands). This ticket makes `authorize(agent.invoke)` decide on readiness.
- AZ-M19 end to end with invocation token and delivery (M2-08); AZ-M15 to AZ-M17, dev environments, `DevEnvironmentReady` and `dev_environment_id` (V2-01, V2-02).
- Channel behaviour, prompt, skills, mention rules, their operations and the two event types themselves (M2-02); this ticket adds the binding to them.
- Resolving `workspace_ref` under `runner.projects_root`, detecting readiness, applying `runner_config` to the command line (M2-34); setting operators (M1-54) apart from the readiness reset of Design note 7.
- `kernel/personaldata.Participant` for `project_bindings.created_by` (M7, P-14); client and console views of the binding (no plan row yet, see the report).
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M2-03-project-binding`; one writer per worktree.
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
- M2-02 was briefed in parallel with this ticket. Where its merged code names the behaviour read model, the served-channel queries or the helper that appends `channel.settings_changed` and `agent.config_changed` differently from this brief, follow the code and list each difference in the report.
- No real agent runtime, runner, coding agent or model call in any test: readiness enters only through `ApplyProjectReadiness`, called by the test the way M2-20 will call it.
- The server never opens `repository_url` (03); no test resolves or contacts it. Example values use `example.com` hosts only.

## Design notes

0. Start check. Run `make check` on the fresh worktree before changing anything; if it is red, stop with `Verdict: BLOCKED base branch red` and the failing gate. Confirm that M2-02 is merged (`getChannelBehavior` is in `enabled` of `api/operations.yaml`) and that the chat `StructureSource` adapter resolves thread paths (M1-55, `grep -rn "func .*Thread(" internal/chat/adapters`); otherwise stop with `Verdict: BLOCKED dependency <ticket> missing`.

1. Authorization. Both operations build the descriptor `{kind: channel, id: channel_id}` and call `authorize(actor, channel.project.bind, descriptor)` before any other check except the schema validation of the request chain (M1-18 `StageValidate` runs before the handler). `authz` decides (M1-28): unknown channel or no read permission gives `not_found` (`404`, same body and headers as an unknown ID); read permission without being an effective owner gives `forbidden` with reason `owner_only` (`403`); an agent operating the forum is visible through the operator rule and also gets `403 forbidden` with `reason: owner_only`; an owner's personal API token has no owner permissions (SR5, R-AZ-14) and gets `404` without a grant on the forum or `403 forbidden` with `reason: owner_only` with one; an owner whose step-up is older than `auth.step_up_max_age` gets `401 step_up_required`. Denials are audited by the authz layer through the deny aggregator (R-AZ-04, AZ-P13); nothing else in this ticket writes a denial.

2. Order of checks for `putProjectBinding` (fixed, so tests are deterministic):
   a. Load the channel (unknown gives `404`), then `authorize` (note 1).
   b. `kind` is not `forum`: `400 validation_failed`, field error pointer `#path/channel_id`, code `not_a_forum` (the code M1-53 uses for forum tags; pointer form from 06 5 point 3).
   c. Body validation of note 4: `400 validation_failed` with every field error at once.
   d. Open the unit of work. Lock the `channels` row `FOR UPDATE`; the channel or its space archived gives `409 conflict` (07 4.5, ruling D2 of M1-53).
   e. Lock the `project_bindings` row `FOR UPDATE` when it exists. `If-Match` present and no row: `412 version_conflict` without `current` (RFC 9110: the condition is false without a current representation). `If-Match` present and not equal to `version`: `412 version_conflict` with `current` = the stored `ProjectBinding`. Use the `If-Match` helpers of M1-18.
   f. Apply note 5. A unique violation of the index on `lower(workspace_ref)` (SQLSTATE `23505` on exactly that index) gives `409 conflict` without `reason`; any other database error stays an internal error.
   g. On a change: raise `channels.settings_version` by one in the locked row (never `channels.version`), append the events of note 9, record the audit of note 6. Answer `200` with the `ProjectBinding` after the change and `ETag: "<version>"`.
   Lock order across this ticket (03 6.3 point 2): `spaces`, then `channels`, then `project_bindings`; several rows of one table in ascending id. Deadlocks and lock timeouts are retried by the unit of work (`database.tx_retry_max`).

3. `deleteProjectBinding`: steps a, b (`400` for a non-forum) and d as in note 2; no binding gives `404 not_found` (06 7: a resource without tombstone answers `404` after its deletion, so a repeated DELETE answers `404`); otherwise delete the row, raise `channels.settings_version`, append the events of note 9, record the audit of note 6 and answer `204`. The forum stays a forum; from the commit on, `agent.invoke` there no longer checks readiness (05 section 5 point 7 applies only to bound forums).

4. Validation (pure, `domain.ValidateProjectBinding`; field error codes from the vocabulary of M1-18 plus `invalid_characters` of M1-53; the shared validator `kernel/textcheck` of M1-26 rejects C0 and C1 control characters and bidi controls, SEC-IN-4, SEC-T07):
    - New configuration key `project.binding_text_max_chars` (Server, positive integer): the maximum length in Unicode characters of `workspace_ref`, `repository_url`, `default_branch`, `runner_config.model` and each entry of `runner_config.allowed_tools`; longer gives `too_long`. Choose the default so that a `workspace_ref` of that many characters in UTF-8 stays inside the PostgreSQL B-tree entry limit of the unique index on `lower(workspace_ref)`, and write that reason into the row of 09 section 21. Without this key an over-long `workspace_ref` would reach the index and end as an internal error. Ruling for docs/DECISIONS.md.
    - `workspace_ref` (`/workspace_ref`): not empty (`required`); no control or bidi characters (`invalid_characters`); `invalid_format` when it has leading or trailing white space, starts with `/`, `\`, `~` or `-`, starts with a drive letter and colon (`C:`), or has a path segment `..` when split at `/` and `\`. Reason: 03 says "never an absolute path of the server"; the runner still confines the resolved path under `runner.projects_root` including symbolic links (09 18.1), this check is defense in depth. Relative refs with `/` inside (for example `team/website`) stay allowed.
    - `repository_url` (`/repository_url`, optional): an absolute URL with scheme `https` or `http` (the link schemes clients render, SEC-OUT-1) and a host, without user information (no credentials in the database, 03 principle 9), else `invalid_format`; no control or bidi characters.
    - `default_branch` (`/default_branch`, optional): `invalid_format` when empty, when it contains white space or `..`, or starts with `-`; no control or bidi characters.
    - `runner_config` (`/runner_config`): schema `ProjectRunnerConfig` (note 11); unknown members give `unknown_member` from the request validator. `model` and each `allowed_tools` entry: not empty and not starting with `-` (`invalid_format`; the runner passes them as process arguments, 09 18.2), no control or bidi characters; duplicate tools are rejected by `uniqueItems`.
    - `reason` (`/reason`): not empty after trimming (`required`), at most `audit.reason_max_chars` characters (`too_long`), no control or bidi characters (SEC-AUD-5).
    - The whole body stays within `http.max_json_body_bytes` (M1-18 answers `413` otherwise).

5. PUT semantics (pure, `domain.PlanBindingPut`):
    - No row: insert with `ready = false`, `version = 1`, `created_by` = actor, `created_at = now()`; kind `created`.
    - Row exists: the PUT replaces the whole binding (HTTP PUT): an absent optional member clears the column (`repository_url` and `default_branch` NULL, `runner_config` `{}`); `created_by` and `created_at` stay. If the stored values equal the new ones (`runner_config` compared as `jsonb`, so the order of `allowed_tools` counts): kind `unchanged`, no write, no version change, no event, `ready` untouched; answer `200` with the stored binding and write the audit allow entry with `op = unchanged` (every allowed audited action is recorded, as M1-54 does for a no-op). Reason: a client that retries a lost PUT must not reset readiness.
    - Otherwise kind `replaced`: write the new values, `version + 1`, and set `ready = false`. Reason: the runner reported readiness for the old configuration; 05 section 5 point 7 must fail closed until the runner, told by `agent.config_changed`, reports the new workspace ready. Ruling for docs/DECISIONS.md.

6. Audit, in the same transaction through `auditing.Recorder`: action `channel.project.bind`, resource the channel, details exactly `channel_id`, `op` (`created`, `replaced`, `unchanged`, `deleted`), `version` (after the change; absent for `deleted`), `ready_reset` (boolean, PUT only) and `reason` (PUT only; DELETE has no body). Never `workspace_ref`, `repository_url`, `default_branch` or `runner_config`: SEC-AUD-5 allows no other free text, and these are names and configuration, not codes. A `404` of DELETE without a binding writes no audit entry.

7. Readiness.
    - `ApplyProjectReadiness(ctx, agentID, reports)` is the only writer of `ready` besides the resets of this note. It runs in the ambient unit of work of the caller (M2-20's heartbeat) or opens its own. A channel id that occurs twice in `reports` returns an `errs` validation error with pointer `/project_readiness` (M2-20 maps it to `400 validation_failed`) and changes nothing.
    - Lock, in ascending channel id: the `spaces` rows of reported channels that inherit their operator `FOR SHARE`, the `channels` rows `FOR SHARE`, the `project_bindings` rows `FOR UPDATE`; compute the effective operator only after these locks (03 `channels` formula). Reason: `setChannelOperator` and `setSpaceOperator` update these rows, so the share locks serialize a report against an operator change.
    - A report is applied only when the channel has a binding, the channel and its space are not archived, and the effective operator of the channel equals `agentID` (09 4.8: "runner only"; 09 18.1). Every other entry is ignored without error, so the result never tells an agent anything about forums it does not serve. An applied report that changes `ready` raises `project_bindings.version` by one; an equal value writes nothing.
    - A readiness change writes no event, no audit entry and does not raise `channels.settings_version`. Reason: readiness is runtime state of the runner, not configuration; 04 2.2 forbids its content in events anyway; a `settings_version` change on each flip would make every pending `putChannelBehavior` fail with `412 version_conflict` and change the ETag of `getServedChannelConfig` without a configuration change; `agent.heartbeat` audits only changes of the reported mode and the tool class (05 4.10). Owners see `ready` on their next `getChannelBehavior`. Ruling for docs/DECISIONS.md.
    - Reset on operator change: in `SetChannelOperator` and `SetSpaceOperator` (M1-54), after the operator columns are written and inside the same transaction, set `ready = false` and `version = version + 1` for every bound forum whose effective operator changed and whose `ready` is `true` (for a space: its inheriting, non-archived channels). Reason: readiness was reported by an agent that no longer serves the forum. No extra event: M1-54 already writes `channel.updated` and `access.changed`. Ruling for docs/DECISIONS.md.

8. `agent.invoke` facts. The chat `StructureSource` adapter fills `ChannelFacts.ProjectBound` (a `project_bindings` row exists) and `ChannelFacts.ProjectReady` (its `ready`) with a `LEFT JOIN` in every query that returns channel facts, reading in the ambient transaction when one exists. The guard order stays as M1-28 built it (step 6 after SR2): bound and not ready gives `project_not_ready`; bound, ready, principal not an effective owner and `DevEnvironmentReady` false gives `dev_environment_not_ready` (SR3 rule 1; `DevEnvironmentReady` stays false until V2). Owners trigger a ready bound forum. Nothing in this ticket raises `permission_version` or writes `access.changed` (05: Permissions `no`): structure facts are read fresh per call in REST mode (M1-28 "Resolution"). The delivery mode of M1-65 (`authorize.Delivery`, in the closure through M2-02), whose cached resolved paths are invalidated only by structure events, is in `internal/authz`: make the `agent.invoke` fact gathering read the channel facts fresh in that mode as well, because readiness changes write no event (note 7); 07 SR2 states the reason: readiness changes without a permission transaction, and the invocation is the only place where all facts are current. If `ChannelFacts` lacks the two fields in the merged code, add them to the interface in `internal/authz` and to its fakes.

9. Events on `created`, `replaced` and `deleted` (through `kernel/eventlog.Appender`, in this order, the way M2-02 appends them for `putChannelBehavior`):
    - `channel.settings_changed`: scope `{space_id, channel_id}`, resource `{kind: channel, id: channel_id}` without `version`, payload `ChannelSettingsChangedPayload {channel_id, settings_version}`, receive action `channel.read`.
    - `agent.config_changed`: same scope, resource and payload (`AgentConfigChangedPayload`), receive action `agent.config.read`, audience the forum's effective operator (`event_log.audience_principal_id`). When the forum has no effective operator, append no `agent.config_changed`, because no receiver exists; if M2-02 handles this case differently, follow M2-02 and report it.
    - No JSON `null` (ADR 0053); no binding value anywhere in the envelope or payload (04 2.2 rule 1).

10. Read models. `ChannelBehavior.project_binding` (in the answers of `getChannelBehavior` and `putChannelBehavior`) and `AgentChannelConfig.behavior.project_binding` (in `listServedChannels` and `getServedChannelConfig`) carry the `ProjectBinding` exactly when the forum has a binding; otherwise the member is absent, never `null`. The permissions stay those of M2-02 (`manage` for behaviour; the serving agent for its config). The ETag of these operations is `settings_version`, which notes 2 and 3 raise.

11. Schema `project_runner_config.v1` (03 and openapi name it; no file defines it yet). Define it in 09 18.1 and as `ProjectRunnerConfig` in `docs/api/openapi.yaml`: an object with `additionalProperties: false` (required for every object reachable from a request body, W7 of M1-18; the M1-17 generator check fails on an open one) and the optional members `model` (string, `minLength: 1`) and `allowed_tools` (array of strings with `minLength: 1`, `uniqueItems: true`), description "Schema `project_runner_config.v1`. No secrets." The server checks only this shape and the rules of note 4; it does not interpret tool names. The runner never widens its configured tool class with this list: it derives its flags from the class and narrows them by the list (09 18.2; M2-34 builds that). The 03 rule "within `agents.tool_class`" is therefore enforced by the runner. Ruling for docs/DECISIONS.md.

12. SQL visibility lint. If `tools/lint/sql_visibility.yaml` exists (M1-70), every new read of `project_bindings` either carries a predicate `channel_id = ANY(<param>)` or gets an exception entry: the `StructureSource` facts with category `authorize_resolution`, the binding read after `authorize` in PUT, DELETE and the read models with category `after_authorize`; each with a reason. The readiness lock query filters `channel_id = ANY(<reported ids>)`. Writes are outside the lint (07 7.4 rule 1).

13. Canary scenario. `services/server/test/canary` exists (M1-69, in the closure through M2-02), and its test `TestSEC_T10_ScenarioCoversEveryEnabledOperation` fails for the two new operations: add both to the scenario with canary values in `workspace_ref`, the path of `repository_url` (host `git.example.com`), `default_branch`, `runner_config.model` and `reason`. Allowed sinks: the `200` answer of the PUT, `getChannelBehavior`, `getServedChannelConfig` and `listServedChannels` answers, and the audit `reason` (the only free text SEC-AUD-5 allows); everything else (logs, traces, metrics, event log, other audit fields, error bodies) must stay free of them.

14. Concurrency. Two PUTs on one forum serialize on the `channels` row. Two PUTs with the same `workspace_ref` on different forums: the unique index lets exactly one commit, the other gets `409 conflict`. A PUT and a readiness report on the same forum serialize on the `project_bindings` row (the PUT holds the `channels` row `FOR UPDATE`, the report takes it `FOR SHARE` first, so the lock order is the same).

15. Rulings for `docs/DECISIONS.md` section 10 (one row each with date, decision, reason, location): `D-M2-03-1` schema `project_runner_config.v1` (note 11); `D-M2-03-2` configuration key `project.binding_text_max_chars` (note 4); `D-M2-03-3` format rules of `workspace_ref`, `repository_url`, `default_branch` and the runner config strings (note 4); `D-M2-03-4` PUT replaces the whole binding, an identical PUT is a no-op that keeps `ready` (note 5); `D-M2-03-5` every effective PUT change and every change of the effective operator resets `ready` (notes 5 and 7); `D-M2-03-6` readiness reports only from the effective operator of a bound, non-archived forum, other entries ignored, no event, no audit, no `settings_version` change (note 7); `D-M2-03-7` status codes: non-forum `400` with `not_a_forum`, archived `409 conflict`, DELETE without binding `404`, `If-Match` without binding `412` (notes 2 and 3); `D-M2-03-8` no `agent.config_changed` without an effective operator (note 9), unless M2-02 decided otherwise.

## Tests (write them first)

Go test names carry the specification ID where one exists, so `go test -run AZ_P1` finds it.

Unit (`internal/chat/domain`, table tests; limits read from the configuration in the test, never hard-coded):

- `TestValidateProjectBinding_WorkspaceRef`: accepted `website` and `team/website`; rejected with the code of note 4: empty, white space only, leading or trailing space, `/srv/website`, `\srv\website`, `C:\srv\website`, `~website`, `-website`, `team/../website`, `..\website`, a value one character over `project.binding_text_max_chars` (`too_long`), one at exactly the limit accepted.
- `TestSEC_T07_ProjectBindingRejectsControlAndBidi`: every code point listed by `kernel/textcheck` is rejected in `workspace_ref`, `repository_url`, `default_branch`, `runner_config.model`, an `allowed_tools` entry and `reason`, each with code `invalid_characters` and the right pointer.
- `TestValidateProjectBinding_RepositoryURL`: `https://git.example.com/team/website` and `http://git.example.com/team/website` accepted; `ftp://git.example.com/x`, `javascript:alert(1)`, `git.example.com/team/website` (no scheme), `https://alex@git.example.com/x` (user information) and `https:///x` (no host) rejected with `invalid_format`.
- `TestValidateProjectBinding_DefaultBranchAndRunnerConfig`: `main` and `release/2026` accepted; empty, `my branch`, `a..b`, `-main` rejected; `model` empty or `-x` rejected; `allowed_tools` entry `-rf` rejected.
- `TestValidateProjectBinding_Reason`: blank gives `required`; `audit.reason_max_chars` plus one gives `too_long`.
- `TestPlanBindingPut`: no row gives `created` with `ready` false and version 1; equal values give `unchanged` and keep `ready` true; any changed member (including only the order of `allowed_tools`, or clearing `repository_url` by omitting it) gives `replaced`, version plus one and `ready_reset` true; `created_by` and `created_at` kept.

Unit (`internal/authz`): the existing `TestAgentInvoke_*` cases of M1-28 for "project not ready" and "dev environment not ready" stay green unchanged; if note 8 adds the fields or a delivery-mode re-read, add a table case per change.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain with the response validator, people and sessions from `identitytest`; world of 07 "Examples" from the M1-54 world builder in `test/integration/`, extended additively where missing: owner, Alex with `trigger` and `manage` on space Engineering, Sam without any grant on Engineering (as in the world of 07), Robin with `trigger` on the forum Engineering/Projekt-A; agents Runner (organization, fixed operator of the forum Engineering/Projekt-A) and Lotse (operator of Support); a second forum Engineering/Projekt-B with Runner as operator; a text channel Engineering/Werkzeuge; every response checked against `docs/api/openapi.yaml`):

- `TestPutProjectBinding_OwnerOnlyWithStepUp`: owner with fresh step-up gives `200` with `ready` false, `version` 1 and `ETag: "1"`; owner with step-up older than `auth.step_up_max_age` gives `401 step_up_required`; Alex (read, `manage`) gives `403 forbidden` with `reason: owner_only`; Sam (no read on Engineering) gives `404` with status, body and headers equal to those for a random channel id after removing `correlation_id` and `instance`; Runner's agent token gives `403 forbidden` with `reason: owner_only`; the owner's personal API token without grants gives `404`. Rows unchanged after every denial.
- `TestDeleteProjectBinding_OwnerOnlyWithStepUp`: the same principals against DELETE with the same statuses; the owner gets `204`.
- `TestAZ_P13_ProjectBindingDenialsAudited`: after flushing the deny aggregator as the M1-24 tests do, the denials of Alex, Sam and the stale step-up stand in `audit_events` with `decision = deny` and reasons `owner_only`, `no_grant` and `step_up_required`.
- `TestPutProjectBinding_CreateReplaceNoOp`: create, then an identical PUT gives `200` with the same `version`, no new `event_log` row and an audit entry with `op = unchanged`; after `ApplyProjectReadiness` has set `ready` to true, the identical PUT keeps it true; a PUT that omits `repository_url` clears it (absent in the answer, no `null`), raises `version` by one and sets `ready` false.
- `TestPutProjectBinding_NotAForum`: PUT and DELETE on Engineering/Werkzeuge give `400 validation_failed` with pointer `#path/channel_id` and code `not_a_forum`.
- `TestPutProjectBinding_ArchivedForum409`: after `archiveChannel` on Projekt-B, PUT and DELETE give `409 conflict`; same for a forum of an archived space.
- `TestPutProjectBinding_WorkspaceRefTakenCaseInsensitive409`: `Website` on Projekt-B after `website` on Projekt-A gives `409 conflict`; after deleting the binding of Projekt-A it succeeds.
- `TestPutProjectBinding_ConcurrentSameRef`: two PUTs of the same `workspace_ref` on Projekt-A and Projekt-B behind a barrier: exactly one `200`, one `409 conflict`, one row.
- `TestPutProjectBinding_IfMatch`: without `If-Match` `200`; with the current version `200`; with an old version `412 version_conflict` whose `current` validates as `ProjectBinding`; `If-Match` on a forum without binding `412` without `current`.
- `TestPutProjectBinding_UnknownRunnerConfigMember`: `runner_config: {"api_key": "x"}` gives `400 validation_failed` with pointer `/runner_config/api_key` and code `unknown_member`; nothing stored.
- `TestDeleteProjectBinding_Twice`: first `204`, row gone; second `404 not_found`, no event, no audit entry.
- `TestProjectBinding_EventsAndSettingsVersion`: a create raises `channels.settings_version` by one and leaves `channels.version` unchanged; exactly `channel.settings_changed` then `agent.config_changed` with consecutive `seq`, both with resource `{kind: channel, id}` without `version`, payload `{channel_id, settings_version}`, receive actions `channel.read` and `agent.config.read`, `audience_principal_id` = Runner on the second; both validate against their schemas; no JSON `null`; the canary strings of `workspace_ref`, `repository_url`, `default_branch` and `runner_config.model` appear in no `event_log` row and in no audit detail (the only free text there is the `reason`). Same for a replace and a delete. A forum the test creates with operator mode `none` and no space default gets only `channel.settings_changed`.
- `TestProjectBinding_AuditDetails`: the allow entry of a create has exactly the keys `channel_id`, `op`, `version`, `ready_reset`, `reason`; a delete has `channel_id` and `op`.
- `TestProjectBinding_ReadModels`: owner and Alex (`manage`) see `project_binding` in `getChannelBehavior`; Runner sees it in `getServedChannelConfig` and in its `listServedChannels` item for Projekt-A; Projekt-B without binding has no `project_binding` member; the `ETag` of `getChannelBehavior` changes after a PUT.
- `TestAgentInvoke_ProjectNotReady` (05 section 5 point 7; the `authorize` part of AZ-M19): a forum thread of Projekt-A created through `chat.Threads.Create` with operator Runner; `authorize(owner, agent.invoke, {thread, Invoke{AgentID: Runner, TriggerOrigin: "human"}})` is `forbidden` with reason `project_not_ready`; after `ApplyProjectReadiness(Runner, [{Projekt-A, true}])` it is `allow` and `permission_version` is unchanged; after a PUT that changes `default_branch` it is `project_not_ready` again; after DELETE it is `allow` without any readiness report.
- `TestAgentInvoke_ProjectBindingNonOwner`: Robin (`trigger` on Projekt-A), with an operator of tool class `chat` so that SR2 does not apply (Runner keeps the default `chat` unless the world builder raised it; then the test uses an organization agent with tool class `chat` as fixed operator of a forum it creates): not ready gives `project_not_ready`; ready gives `dev_environment_not_ready` (SR3; 21 section 1).
- `TestApplyProjectReadiness_OnlyEffectiveOperator`: a report by Lotse for Projekt-A changes nothing; a report by Runner sets `ready`, raises `project_bindings.version` by one, leaves `channels.settings_version` unchanged and writes no `event_log` row and no audit entry; repeating it writes nothing; reports for an unbound forum, an archived bound forum and a random id are ignored without error.
- `TestApplyProjectReadiness_DuplicateChannelRejected`: the same channel twice with different values returns a validation error and changes nothing.
- `TestOperatorChangeResetsReadiness`: ready binding on Projekt-A; `setChannelOperator` to another organization agent sets `ready` false and raises `version`; a later report by Runner is ignored and one by the new operator applies. Same through `setSpaceOperator` for a bound forum that inherits the space default.
- `TestApplyProjectReadiness_ConcurrentOperatorChange`: a readiness report by Runner and `setChannelOperator` to another agent behind a barrier, repeated with both orders: no deadlock reaches the caller, and afterwards `ready` is `true` only if the effective operator is Runner.
- `TestAZ_P1_RouteCount`: the route count test passes with the two new operations; both carry `channel.project.bind` and `rl.admin`.

Contract and generators: `make api-lint` and `make api-checks` accept the changed `openapi.yaml`; `make codegen-check` shows no drift; the M1-17 check "request schemas closed" passes with `ProjectRunnerConfig`; `make config-doc-check` shows `project.binding_text_max_chars` in the reference.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/chat/... ./internal/authz/... ./internal/kernel/... ./internal/wire/... ./migrations/... ./test/integration/...`
- `make migrations-lint`, `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make register-check`, `make config-doc-check`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `putProjectBinding` and `deleteProjectBinding` are enabled and registered with `channel.project.bind` and `rl.admin`; AZ-P1 passes.
- [ ] Only an owner in a session with fresh step-up changes a binding: non-owner with read `403 forbidden` with `reason: owner_only`, without read `404` identical to an unknown id, stale step-up `401 step_up_required`, agent and personal tokens never succeed; allows and denials are audited (AZ-P13).
- [ ] A binding exists only on non-archived forums, `workspace_ref` is unique case-insensitively, and the validation rules of note 4 hold, including the closed `project_runner_config.v1`.
- [ ] `authorize(agent.invoke)` in a bound forum gives `project_not_ready` while `ready` is false and `allow` for an owner once the serving runner reported it ready, without a new `permission_version`; non-owners get `dev_environment_not_ready`.
- [ ] `ready` is written only by `ApplyProjectReadiness` from the forum's effective operator and is reset by every effective PUT change and every change of the effective operator.
- [ ] Every binding change raises `channels.settings_version` and appends `channel.settings_changed` and `agent.config_changed` with only `channel_id` and `settings_version`; no binding value appears in events or in the audit details apart from the reason.
- [ ] `getChannelBehavior`, `putChannelBehavior`, `listServedChannels` and `getServedChannelConfig` carry `project_binding` exactly for bound forums.
- [ ] 09 18.1 defines `project_runner_config.v1`, 09 21 and the configuration reference list `project.binding_text_max_chars`, `openapi.yaml` has `ProjectRunnerConfig` and `409` on `deleteProjectBinding`, and `docs/DECISIONS.md` has the rows `D-M2-03-1` to `D-M2-03-8`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `chat` and, if note 8 changed it, `authz` updated (port `ProjectBindings`, table `project_bindings`, readiness and its reset, the query exceptions of note 12).
- Rulings of note 15 recorded in `docs/DECISIONS.md` (one row each with reason).
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
