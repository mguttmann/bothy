# A1-02: Server: agent definitions

Status: ready | Depends on: none (specification merged) | Milestone: A1

## Goal

After this ticket the server stores versioned definitions of built-in agents as closed data and refuses every definition that would reach a surface of the Claude Code CLI below `host`. An owner writes a definition with `putAgentDefinition` inside `agent.update` (step-up, audit, `If-Match`), reads it and its versions with `agent.definition.read`, and the agent itself reads its current definition through its hosting runner; every write raises `agents.version` and sends the durable personal event `agent.definition_changed` to the agent. The validator of 26 4.2 and 4.3 answers one field error per finding with a JSON Pointer and a field code, and a shared corpus of definition fixtures holds the same rules for the runner (A1-05), the template gate (A1-06) and the interview (A1-07). `agent.tool_class.set` refuses to lower an agent below `host` while its definition has a `stdio` MCP server. Nothing of this changes any existing agent.

## Read first

- `docs/spec/26-builtin-agents.md`: 1, 2, 4 (all), 5.5 (the surfaces), 9, 11, 12 (BA-T01 to BA-T04), 13 (R-BA-04 to R-BA-06, R-BA-14), 14 (the `agents.definition.*` keys and `agents.mcp_stdio_runners`).
- `docs/spec/07-permissions.md` 2.11 and 2.13 (the row `agent.definition.read`, the refusals of `agent.tool_class.set`); `docs/spec/05-actions.md` 4.10 rows `agent.update`, `agent.tool_class.set` and R-AZ-36; `docs/spec/06-api.md` 7, 8 and R-API-29.
- `docs/spec/03-data-model.md` 4.4 `agents` and `agent_definitions`, R-DM-59; 03 section 6.3 (lock order).
- `docs/spec/09-agents.md` 2.1, 2.2 (`default_model` pattern), 18.1 (`allowed_tools` intersection), 18.2 (tool classes).
- `docs/spec/04-sync-protocol.md` 15.1 (personal durable events to an agent).
- ADR 0076, ADR 0049 (catalogue as the single source), ADR 0013 (spec first), ADR 0053 (no JSON null).
- Design note `docs/design/builtin-agents.md` D-A1-7, D-A1-17, D-A1-18 and the section "A definition is data"; `docs/decisions.d/A1-01.md`.
- Code on the base branch: `services/server/internal/agents` (`app/create.go`, `app/policy.go`, `app/store.go`, `app/read.go`, `README.md`), `services/server/internal/wire` (the configuration key files), `services/server/api/operations.yaml`, `services/server/migrations` and its schema tests (`agents_test.go`), `docs/api/openapi.yaml`, `docs/api/asyncapi.yaml`, `docs/spec/actions.yaml`, `tools/catalog`.

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
8. White label: examples, templates and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no internal script names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it: the catalogue rows of `docs/spec/actions.yaml` that R-AZ-36 assigns to this ticket and their generated copies (`make catalog`).
11. Additive only (26 section 11): every new column is nullable or defaults to "as before", every key defaults to off, and an installation that sets none of the new keys behaves exactly as before.

Note on rule 1: "stages" are the milestones and the stage A1 of the plan (PLAN P-27).

Product requirements implemented here:

12. The schema `agent_definition.v1` is closed at every level and holds exactly the members of 26 4.2 (R-BA-04).
13. Every refusal of 26 4.3 answers `400 validation_failed` with one `errors[]` entry per finding (JSON Pointer and field code `unknown_member`, `skill_frontmatter`, `inline_shell`, `import_outside_folder`, `secret_value_not_allowed`, `env_name_reserved`, `mcp_command_not_allowed`, `mcp_package_unpinned`, `tool_class_too_low`, `too_long`, `too_many`, `invalid_name`, `duplicate_name`), and nothing is stored (BA-T01, BA-T02).
14. Writing is `agent.update` (owner, step-up, audit) with `If-Match` of the current definition version; reading is `agent.definition.read` (owner or agent-self); a definition never appears in an event, a log line, telemetry or an audit entry (26 4.1).
15. `agent.tool_class.set` below `host` is refused with `409 conflict`, reason `definition_needs_host`, while the current definition has a `stdio` MCP server (R-BA-06, BA-T03).
16. Only built-in agents (`adapter_kind = 'claude_agent'`) have definitions; `putAgentDefinition` for another agent answers `409 conflict`, reason `not_a_builtin_agent`.

## Scope

All paths under `services/server/` unless they start with `docs/`, `tests/` or name the root `Makefile`.

Create:

- Migration `<next number>_agents_agent_definitions.sql` with the table of 03 `agent_definitions` (no change to any other table), its lock entry (`make migrations-lock`) and a schema test in `migrations/` like `agents_test.go`.
- `internal/agents/domain/definition/` (pure, no I/O): the types of `agent_definition.v1`, `Validate(def, Limits, ToolClass) []FieldError` with every rule of 26 4.2 and 4.3, the derived class requirement (`NeedsHost`: a `stdio` server) and the intersection helper of 26 4.4 item 2 for later tickets.
- `internal/agents/app/definitions.go`: `PutDefinition` (owner, step-up, `If-Match`, insert of the next version, `agents.version` raised, audit entry with the version and the names of the changed parts, the durable event `agent.definition_changed` with `definition_version` to the agent, all in one transaction with the agent row locked first), `GetDefinition`, `ListDefinitionVersions` (paginated, newest first, without the definition text).
- `tests/agent-definitions/` at the repository root: the shared corpus. `valid/<name>.json` and `invalid/<code>-<name>.json`, each `{"tool_class": ..., "definition": {...}, "expect": [{"pointer": ..., "code": ...}]}` (empty `expect` for valid ones), with at least one invalid fixture per surface of 26 5.5 and per field code of requirement 13, and a `README.md` that says the corpus is the conformance set of 26 4.3 for every writer (server, runner, template gate, interview).
- `docs/decisions.d/A1-02.md` with the rulings of this ticket.

Change:

- `internal/agents/app/policy.go` (or where `agent.tool_class.set` lives): the refusal of requirement 15.
- `internal/agents` port and wire: the read of the current definition for the agent itself (agent-self) and for owners.
- `internal/wire`: the configuration keys `agents.definition.instructions_max_bytes`, `agents.definition.max_skills`, `agents.definition.skill_description_max_chars`, `agents.definition.skill_body_max_bytes`, `agents.definition.max_skill_files`, `agents.definition.max_mcp_servers`, `agents.definition.max_allowed_tools`, `agents.definition.max_cost_usd_per_run` and `agents.mcp_stdio_runners` with the defaults of 26 14; `make config-reference`. Keep the rule `agents.` in `tools/configdoc/planned.yaml` while other `agents.*` keys are still planned; remove it in the ticket that brings the last one.
- `docs/api/openapi.yaml`: `getAgentDefinition`, `listAgentDefinitionVersions`, `putAgentDefinition` (paths of 26 9, `x-authz-action`, problem codes, `If-Match` and `ETag` as for other versioned resources), schemas `AgentDefinition` (closed, every limit named by its key in the description), `AgentDefinitionVersion`; `docs/api/asyncapi.yaml` and the event schema `AgentDefinitionChangedEvent`; `services/server/api/operations.yaml` (enable the three operations).
- `docs/spec/actions.yaml`: the action `agent.definition.read` and the event `agent.definition_changed` with exactly the columns of 07 2.13 and 26 9; `make catalog`, `make event-catalog`, `make contract-documents`, and every generated copy that `make codegen-check` holds (Go routes and constants, the Dart and Python clients).
- `services/server/architecture/modules.yaml`: the new action and event of the module `agents`; `internal/agents/README.md` (actions, events, the definition rules in one paragraph).
- `docs/spec/26-builtin-agents.md`: only where the build settles a detail the chapter leaves open (for example a field code that the corpus needs); never a rule weaker than the chapter.

## Out of scope

- `hosted_by`, `hosts_agents`, hosted tokens, `agent.host.set` (A1-03); the runner (A1-04, A1-05); templates and their gate (A1-06); the interview (A1-07); clients (A1-08, A1-09).
- Any field of the definition beyond 26 4.2, also for `host` (a later ruling).
- Resolving a `secret_ref`: the server never does.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/A1-02-agent-definitions`; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers; squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, DPoP keys, secret values and secret references resolved to values never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money, except a test behind the explicit switch of the runner's real CLI tests when this brief names one and the launcher allows it.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/26-builtin-agents.md`, stop with `Verdict: BLOCKED <reason>`.
- The validator is the server's own check; never trust a client, a template or a model draft to have checked anything.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red` and the failing gate. Confirm that `docs/spec/26-builtin-agents.md` and the table `agent_definitions` in 03 exist on the base, else `Verdict: BLOCKED specification missing`.

2. **Validator shape.** One pass over the decoded JSON with a strict decoder (unknown members are findings, not decode errors, so every unknown member gets its own pointer). Limits come from `Limits`, filled from the configuration keys; no number in code. Text rules apply to `instructions`, every skill `body` and every file `content`:
    - frontmatter: the first line of a body or file content is `---` (after an optional byte order mark);
    - inline shell: an exclamation mark followed by a backtick-quoted command on one line, at the start of the text or after white space, and a fence line that opens with three backticks directly followed by an exclamation mark (the two forms of 26 4.3 item 3, at least the two patterns of the pinned CLI quoted there); both count also inside other fences, because the CLI does not care;
    - imports: a token `@<path>` at the start of a line or after white space whose path starts with `/`, `~` or contains a `..` segment (also with `\` as separator);
    - names: skill and server names `^[a-z0-9][a-z0-9-]{0,63}$`, file names `^[a-z0-9][a-z0-9._-]{0,63}\.md$` and never `SKILL.md`; a description is one line;
    - control and bidi characters (the set the runner already refuses for process arguments, 09 18.2) are refused in every name, description, URL, argument, header and environment entry.

3. **Secrets.** A header named `Authorization`, `Cookie` or `Proxy-Authorization`, or a header or environment entry whose upper-cased name contains one of the words of 26 4.3 item 5, must carry `secret_ref` instead of `value`. `secret_ref` has the form the runner's secret provider accepts (`runner.anthropic_api_key_ref` in 09 21 is the model); the server checks only the form. Reserved environment names of 26 4.3 item 5 give `env_name_reserved`.

4. **MCP.** `http`: `url` parses as `https` with a host, no user information, no fragment. `stdio`: `command` is one of `agents.mcp_stdio_runners`; `package` is `<name>@<exact version>` where the version is a semantic version without a range operator, a wildcard or a tag (`latest`, `next`); `args` never start with `-` before the package argument; the server derives that the agent needs `host` and refuses below it with `tool_class_too_low` on `/mcp_servers/<i>/transport`.

5. **Storage.** The agent row is locked first (03 6.3), then the version is computed as the current maximum plus one and inserted; `If-Match` absent is allowed only when no version exists (`428 precondition_required` otherwise, as other versioned resources do), a stale one answers `412 version_conflict` with `current`. `origin_kind` copies `definition.origin.kind`. Undo is a client concern: it reads an older version and writes it as a new one.

6. **Audit and privacy.** The audit entry of `agent.update` names the agent, the new version and the changed top-level parts (`instructions`, `skills`, `mcp_servers`, `settings`), never their content. Add the definition text to the canary test of `test/canary` if it has a scenario for agent writes, so that a canary string in `instructions` appears in no log, event or telemetry.

7. **Read for the agent.** `agent.definition.read` as agent-self returns only the current version of the calling agent; another agent gets `404`. A1-03 makes the hosting runner call it with the hosted agent's token, so no new path is needed here.

8. **Tool class refusal.** `agent.tool_class.set` loads the current definition in its transaction; when the target class is below `host` and `NeedsHost` holds, it answers `409 conflict`, reason `definition_needs_host`, and writes the audit `deny` like other refusals.

9. **Rulings for `docs/decisions.d/A1-02.md`** (one row each): the exact text rules and regular expressions; the corpus format; the version and `If-Match` behaviour; anything the build settled that 26 leaves open.

## Tests (write them first)

- Go unit, `internal/agents/domain/definition`: `TestCorpus` reads every file of `tests/agent-definitions/` and asserts exactly its `expect` list (pointer and code, order free); one table test per rule of design notes 2 to 4 with the boundary of every limit (at the limit passes, one above fails).
- Go unit, `internal/agents/app`: put as owner with step-up stores version 1, a second put with the right `If-Match` stores version 2, a stale `If-Match` gives `412`, a missing one on an existing definition gives `428`; a non-owner gets the refusal of 07 2.13 and the audit `deny`; a definition for a non built-in agent gives `not_a_builtin_agent`; `agent.tool_class.set` to `scoped` with a `stdio` server gives `definition_needs_host` (BA-T03).
- Integration (container runtime): `putAgentDefinition` through the HTTP API with a real step-up, `getAgentDefinition` as owner and as the agent itself (agent token), `404` for another agent; `agent.definition_changed` reaches the agent's consumer with `definition_version` only; the event payload and the audit entry contain no definition text (BA-T01, BA-T04 definition part).
- Migration schema test of `agent_definitions` (columns, check, primary key, foreign keys, no other index).
- API contract tests that `make check` runs (G1 to G10) stay green with the new operations.

```sh
cd services/server && go test -race -count=1 ./internal/agents/... ./migrations/...
make catalog && make catalog-check
make config-reference && make config-doc-check
make docs
make check
```

## Acceptance criteria

- [ ] Every fixture of `tests/agent-definitions/` gives exactly its expected findings in the Go validator.
- [ ] One refusal test per surface of 26 5.5 and per field code of requirement 13 is green (BA-T01, BA-T02).
- [ ] `putAgentDefinition`, `getAgentDefinition` and `listAgentDefinitionVersions` work with the permissions, step-up, audit and `If-Match` rules above (BA-T04 definition part).
- [ ] `agent.tool_class.set` below `host` with a `stdio` server is refused (BA-T03).
- [ ] No definition text in any event, log line, telemetry or audit entry.
- [ ] The catalogue, the API documents, the configuration reference and every generated copy are current; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `services/server/internal/agents/README.md` and `tests/agent-definitions/README.md` describe the definitions and the corpus.
- An entry `changelog.d/A1-<nn>.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/<ticket>.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0076 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: every surface of 26 5.5 is refused by the server; no member outside the schema is stored; a secret named like one cannot be stored as a plain value; `stdio` below `host` is impossible by both paths (definition write and class change); no definition text leaks into events, logs or audit.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: whether the corpus covers a surface of a later CLI version (RK-14)), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
