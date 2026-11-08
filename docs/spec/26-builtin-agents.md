# 26 Built-in agents

Status: draft A1 (stage "A1 Built-in agents", [PLAN](../PLAN.md) section 21). Normative for agents
that a person creates inside Bothy and that a Claude Code runner hosts: the hosting, the agent
definition, its materialization in the runner, the interview, the templates and their gate, and the
creation and editing flow in the apps. Decision:
[ADR 0076](../adr/0076-built-in-agents-hosted-by-the-claude-code-runner.md). Design note with the
reasons per decision (D-A1-1 to D-A1-25): [builtin-agents.md](../design/builtin-agents.md); where
the note and this chapter differ, this chapter applies. Rulings of the specification ticket:
[decisions.d/A1-01.md](../decisions.d/A1-01.md).

Related chapters: agents as principals, adapter contract and the Claude Code runner
([09-agents.md](09-agents.md) 2, 4, 18); tool class and guard rules
([07-permissions.md](07-permissions.md) 2.11, 2.13, SR2); backend policy
([10-backend-policy.md](10-backend-policy.md) 10.3); tables ([03-data-model.md](03-data-model.md)
`agents`, `agent_definitions`); threats ([threat model](../security/threat-model.md) 5.17, 6.9).

Nothing in this chapter is built before the tickets A1-02 to A1-10. Actions, operations and events
that this chapter names enter `docs/spec/actions.yaml`, `docs/api/openapi.yaml` and
`docs/api/asyncapi.yaml` with the ticket that builds them, with exactly the values of
[07-permissions.md](07-permissions.md) 2.13 and section 9 below (R-BA-14).

## 1. Principles

1. **A built-in agent is a normal agent.** It is an agent principal (09 2.1) with its own handle,
   avatar, grants, sessions, costs, tool class and backend policy. The operator rule, the trigger
   rules, the live box, follow-ups, costs and the invariants I1 to I5 apply to it unchanged.
2. **Only Claude, only the Claude Code runner.** Its turns run in a Claude Code runner (09 18) as
   `claude -p` with stream JSON, the CLI and the pinned version of 09 18.2. Bothy ships no second
   agent runtime and no other model vendor for built-in agents.
3. **A definition is data, not configuration of the CLI.** The server stores what the agent is
   (instructions, skills, MCP servers, settings) in a closed schema; the runner writes every
   file the CLI reads from it. No content of a definition, a template or an interview draft is
   ever interpreted by the CLI as code or as a permission grant (section 4.3, R-BA-04).
4. **The server never touches the host.** It stores no secret value and no model credential (I5,
   B10): an MCP server names its secrets by reference, and the runner resolves them. The server
   never writes files on a runner host and never shows a hosted agent's token to a person.
5. **Off until asked for, additive, reversible.** Every new key defaults to off, every new
   column is nullable or defaults to "as before", and an installation that sets none of the new
   keys behaves exactly as before (section 11).

## 2. Terms

| Term | Meaning |
|---|---|
| Built-in agent | An agent with `adapter_kind = 'claude_agent'` whose turns a hosting runner runs from its definition. Also called a hosted agent. |
| Hosting runner | The Claude Code runner agent named in `agents.hosted_by` of a built-in agent. It is an agent of `adapter_kind = 'code_runner'` that reports the capability `hosts_agents`. |
| Agent definition | The versioned, closed description of a built-in agent: instructions, skills, MCP servers, settings and origin (section 4, 03 `agent_definitions`). |
| Agent folder | `<runner.hosted_agents_root>/<agent_id>/`, the working folder of the agent's turns. |
| Control folder | `<runner.hosted_agents_root>/<agent_id>.control/`, next to the agent folder and outside it: the files the runner writes for the CLI. |
| Builder agent | The agent named in `agents.builder_agent_id` that answers interview requests (section 6). |
| Template | A generic, versioned starting point for a definition under `templates/agents/<slug>/` or in the private folder `agents.templates_dir` (section 7). |

## 3. Hosting

### 3.1 Records

1. A built-in agent is created with `agent.create` and carries `adapter_kind = 'claude_agent'` and
   `agents.hosted_by` (03). The two go together: `hosted_by` is set exactly when `adapter_kind` is
   `claude_agent` (R-DM-58). No other agent may have `hosted_by`.
2. `hosted_by` names an active agent with `adapter_kind = 'code_runner'` that is not itself hosted
   and whose last heartbeat reports the capability `hosts_agents` (09 2.2). Any other value is
   refused with `400 validation_failed` and the field code `host_not_capable` on `/hosted_by`.
3. The capability `hosts_agents` carries the highest tool class the runner loads for a hosted
   agent (`chat`, `scoped` or `host`, its key `runner.hosted_max_tool_class`). The server refuses
   `agent.tool_class.set` for a hosted agent above that class with `409 conflict`, reason
   `host_tool_class_ceiling`, and `agent.host.set` to a runner whose ceiling is below the agent's
   class with the same reason. A runner without `runner.hosted_agents` reports no `hosts_agents`.
4. The host changes only through `agent.host.set` (owner, step-up, audit; 07 2.13). The change
   revokes every token of the agent that the old host holds (3.3) in the same transaction and
   writes the durable event `agent.hosting_changed` to the old and the new host.
5. `adapter_kind = 'claude_agent'` cannot serve a bound forum: R-AG-39 accepts only
   `code_runner`, and the dialog "New project" (01 6.9) never offers a built-in agent. A later
   ruling may open project forums to built-in agents; this stage does not.

### 3.2 One runner process, many agents

1. With `runner.hosted_agents` (default `false`) the runner hosts every active agent whose
   `hosted_by` is its own agent. It lists them with `listHostedAgents` (action
   `agent.hosted.list`, agent-self) at start, on `agent.hosting_changed` and every
   `runner.channels_resync_interval` as the fallback for a missed event.
2. For each hosted agent the runner holds its own gateway connection with the hosted agent's
   token and the consumer `default`, its own cursor, channel cache, heartbeat and session state.
   State lives under `<runner.state_dir>/hosted/<agent_id>/` (mode `0700`): cursor, channel cache,
   turn records and session records, never a token (3.3). Each hosted agent sends its own heartbeat
   with its own `reported_tool_class` (3.1 item 3), the runner's `billing_modes_supported` and the
   capabilities of the runner except `hosts_agents`.
3. Slots are shared: `runner.max_parallel_runs` bounds the turns of the runner's own agent and of
   all hosted agents together, because they protect one host. `run_slots` and `waiting_runs`
   (09 5.2.1) are reported per agent with the shared totals.
4. Without `runner.hosted_agents` the runner reads no hosted list, opens no further connection and
   its command line, heartbeat and served channels are byte-equal to the fixtures of the version
   before this stage (section 11, BA-T20).

### 3.3 Tokens of hosted agents

1. **Issue.** The hosting runner fetches a token for a hosted agent with
   `issueHostedAgentToken` (action `agent.hosted_token.issue`), authenticated with its own
   DPoP-bound agent token. `authorize` allows it only when the target is an active agent whose
   `hosted_by` equals the calling agent, and the caller is active and reports `hosts_agents`;
   every other case answers `404` like an unknown agent. The server creates an `api_tokens` row
   for the hosted agent with `created_by` the hosting runner, `dpop_jkt` equal to the `dpop_jkt`
   of the calling token (a calling token without `dpop_jkt` is refused with `409 conflict`, reason
   `dpop_required`) and `expires_at` after `agents.hosted_token_ttl`, revokes every older
   active token of the hosted agent created by the same runner, writes one audit entry (agent,
   host, token id, never the value; the token row holds the prefix, which is no code value of
   the audit details, decisions.d/A1-03.md D-A1-03-9) and returns the token once.
2. **Keep.** The runner keeps the token in memory only, never in a file, a log, the turn record or
   the environment of a turn, and fetches a new one before `expires_at` and after its own restart.
   Because the token is bound to the runner's DPoP key, a copy of it without that key connects to
   nothing.
3. **Revoke.** All tokens of a hosted agent are revoked, and its connections closed with `4012`,
   when: the hosted agent is disabled (`agent.disable`); its host changes (3.1 item 4); the hosting
   runner is disabled; or a token of the hosting runner is revoked, which revokes the hosted
   tokens created by that runner with the same `dpop_jkt` in the same transaction. A person never
   sees, copies or creates a hosted token; `token.create` for a hosted agent is refused with
   `409 conflict`, reason `hosted_agent_token`.

### 3.4 Where a built-in agent answers

In every text channel or forum where it is the effective operator, or a guest with `write`
(09 3), like every agent. The creation flow offers to create a channel in a space the owner picks
with the agent as operator (section 8). Direct messages stay "later" (01).

## 4. Agent definition

### 4.1 Storage and versions

1. The server stores definitions in `agent_definitions` (03): one row per version, the highest
   version is the current one, older versions stay for audit and undo. Only built-in agents have
   definitions.
2. Writing a definition is part of `agent.update` (owner, step-up, audit): `putAgentDefinition`
   with `If-Match` of the current version (absent only for the first version) stores a new
   version, raises `agents.version`, writes one audit entry with the version and the names of the
   changed parts (never their text) and the durable personal event `agent.definition_changed`
   with `definition_version` to the agent. Undo writes the content of an older version as a new
   version.
3. Reading is `agent.definition.read`: the owner, and the agent itself (its hosting runner through
   the hosted connection). Definitions never appear in an event, a log line or telemetry.
4. Only built-in agents have a definition; `putAgentDefinition` for another agent answers
   `409 conflict`, reason `not_a_builtin_agent`.

### 4.2 Schema `agent_definition.v1`

A JSON object with exactly these members (`additionalProperties: false` at every level); every
string is valid UTF-8 without control or bidi characters except line breaks inside Markdown text.
All limits are configuration values (section 14).

| Member | Content | Rules |
|---|---|---|
| `instructions` | Markdown, becomes `CLAUDE.md` in the agent folder | at most `agents.definition.instructions_max_bytes`; text rules of 4.3 |
| `skills[]` | `name`, `description`, `body`, optional `files[]` | at most `agents.definition.max_skills`; `name` matches `^[a-z0-9][a-z0-9-]{0,63}$`, unique; `description` one line, at most `agents.definition.skill_description_max_chars`; `body` Markdown, at most `agents.definition.skill_body_max_bytes`; `files[]` at most `agents.definition.max_skill_files`, each `name` a flat file name matching `^[a-z0-9][a-z0-9._-]{0,63}\.md$` other than `SKILL.md` and `content` Markdown within the same byte limit; text rules of 4.3 for `body` and every `content` |
| `mcp_servers[]` | `name`, `transport`, then per transport | at most `agents.definition.max_mcp_servers`; `name` matches `^[a-z0-9][a-z0-9-]{0,63}$`, unique |
| `mcp_servers[]` with `transport = http` | `url`, optional `headers[]` | `url` is `https` with a host, without user information or fragment; `headers[]` each `name` and either `value` or `secret_ref` |
| `mcp_servers[]` with `transport = stdio` | `command`, `package`, optional `args[]`, optional `env[]` | `command` is one of `agents.mcp_stdio_runners`; `package` is pinned as `name@version` with an exact version (no range, no tag); `args[]` strings, none starting with `-` before the package; `env[]` each `name` matching `^[A-Z_][A-Z0-9_]*$`, not one of the names of 4.3 item 5, and either `value` or `secret_ref` |
| `settings` | `model`, `tool_class_wish`, `allowed_tools[]`, `max_cost_usd_per_run` | `model` the pattern of `default_model` (09 2.2); `tool_class_wish` one of `chat`, `scoped`, `host`; `allowed_tools[]` tool names (built-in names or `mcp__<server>__<tool>` of a listed server), distinct, none starting with `-`, at most `agents.definition.max_allowed_tools`; `max_cost_usd_per_run` a decimal above zero and at most `agents.definition.max_cost_usd_per_run` |
| `origin` | `kind`, optional `template_slug`, `template_version` | `kind` one of `template`, `interview`, `manual`; display only |

A `secret_ref` names a key in the secret provider of the hosting runner (16, the `secret_ref`
form of `runner.anthropic_api_key_ref`); the server never resolves it. The runner resolves only a
`file:` reference whose file lies, also after following every link, in
`<runner.hosted_secrets_dir>/<agent_id>/` of the same agent (5.1 item 3); an `env:` reference,
another file and every reference without that key are out of scope.

Optional are only `files[]`, `headers[]`, `args[]`, `env[]`, `settings.model` (absent: the runner's
default model), `settings.allowed_tools` (absent: no narrowing; empty: no named tool),
`settings.max_cost_usd_per_run` (absent: no cap), `origin.template_slug` and
`origin.template_version`; every other member is required. The runner places `package` directly
after `command` and every `args[]` entry after it, so no argument stands before the package
(decisions.d/A1-02.md).

### 4.3 Refusals

The server refuses a definition that breaks one rule with `400 validation_failed`, one
`errors[]` entry per finding with the JSON Pointer and the field code below, and stores nothing.
The same check runs on every definition write, on every interview draft (section 6) and, as the
template gate, on every template (section 7); the runner repeats it before every turn (5.6).

1. **Closed schema** (`unknown_member`): any member outside 4.2. There is no member for settings
   files, hooks, permission rules, environment of the CLI, command helpers, slash commands,
   subagents, output styles, workflows, rules, plugin manifests, language servers, monitors or MCP
   configuration beyond 4.2.
2. **Frontmatter** (`skill_frontmatter`): a skill `body` or file `content` whose first line is
   `---`. The runner writes the frontmatter of `SKILL.md` itself from `name` and `description`
   only (5.2).
3. **Inline shell** (`inline_shell`): in `instructions`, a skill `body` or a file `content`, the
   form `` !`<command>` `` at the start of the text or after white space, or a fenced code block
   opened with `` ```! ``. The pinned CLI runs both as shell when a skill loads; its strings
   (version 2.1.288, read on 2026-11-06) hold these two patterns, which the check covers at least:

    ```text
    /```!\s*\n?[\s\S]*?\n?```/g
    (?<=^|\s)!`[^`]+`
    ```

4. **Imports** (`import_outside_folder`): an `@` import in any text whose path is absolute,
   starts with `~` or contains a `..` segment.
5. **Secrets** (`secret_value_not_allowed`): a plain `value` of a header named `Authorization`,
   `Cookie` or `Proxy-Authorization`, or of a header or environment entry whose name contains
   `KEY`, `TOKEN`, `SECRET`, `PASSWORD`, `PASSWD`, `CREDENTIAL`, `AUTH` or `COOKIE`
   (case-insensitive). Such entries need a `secret_ref`. A secret pasted as the value of an
   innocently named entry is not detectable by name; the preview shows every plain value, and the
   residual risk is T-BA-02. Environment names that the runner sets itself (`PATH`,
   `HOME`, `USER`, `LOGNAME`, `SHELL`, `TMPDIR`, `CLAUDE_CONFIG_DIR`, `CLAUDE_PRODUCT_RUN`, every
   `ANTHROPIC_*`, `CLAUDE_CODE_*`, `LD_*`, `DYLD_*` and `NODE_OPTIONS`) are refused with
   `env_name_reserved`.
6. **MCP commands** (`mcp_command_not_allowed`, `mcp_package_unpinned`): a `command` outside
   `agents.mcp_stdio_runners`, a bare command without a package, or a package without an exact
   version.
7. **Class** (`tool_class_too_low`): a definition with a `stdio` MCP server for an agent whose
   `tool_class` is below `host` (4.4).
8. **Limits** (`too_long`, `too_many`) and names (`invalid_name`, `duplicate_name`).

A finding of the shape or of a character carries the code of the request validation of the API
(`required`, `invalid_type`, `invalid_enum`, `invalid`, `too_large`, `too_small`) or of the text
check (`invalid_utf8`, `control_character`, `bidi_control`). The exact rules and patterns are the
rulings of [decisions.d/A1-02.md](../decisions.d/A1-02.md); the corpus `tests/agent-definitions`
at the repository root is the conformance set of every writer (A1-02).

### 4.4 Tool class and definition

1. A new built-in agent gets `chat` unless the owner picks more in the preview; the class is the
   agent's `tool_class`, set with `agent.tool_class.set` (owner, step-up, audit, 07 2.11), never
   by the definition. `tool_class_wish` only preselects the preview.
2. `scoped` uses the tool list of the definition, intersected with `runner.scoped_allowed_tools`
   of the hosting runner in the order of the runner key (the rule of `allowed_tools` of a project
   binding, 09 18.1): the definition only narrows, the host operator keeps the ceiling.
3. MCP servers are never loaded under `chat`; the runner keeps `--disallowedTools "mcp__*"` and the
   preview says so. Under `scoped` only `http` servers are loaded.
4. A `stdio` MCP server starts a local process with the rights of the host user, so it needs
   `host`: the server refuses the definition below `host` (4.3 item 7) and the preview offers
   `agent.tool_class.set` to `host`, after which SR2 limits triggering to owners. The apps run
   that class step before they write such a definition, on create and on edit (8.2, 8.4).
5. `agent.tool_class.set` below `host` for an agent whose current definition has a `stdio` server
   is refused with `409 conflict`, reason `definition_needs_host`; the owner first removes the
   server (R-BA-06). The runner also refuses a turn that would start a `stdio` server under a token
   class below `host` with `tool_class_mismatch` (5.6), so a definition changed behind the server's
   back cannot widen the class either.
6. The runner has no approval cards: `scoped` stays an allow list without interactive approvals
   in this stage, and the preview says so.

## 5. Materialization in the runner

### 5.1 Keys and folders

1. `runner.hosted_agents_root` is required with `runner.hosted_agents`. The runner refuses to
   start when it is equal to, inside or above `runner.state_dir` or `runner.projects_root`, when
   it or a folder above it holds a `CLAUDE.md`, or when it is writable by group or others. Reason:
   a `host` turn of one hosted agent must not reach the runner's state through its working folder,
   and a `CLAUDE.md` above the root would reach every agent's prompt (compare T-TM-13).
2. The agent folder `<root>/<agent_id>/` (mode `0700`) is the working folder of every turn of the
   agent. The control folder `<root>/<agent_id>.control/` (mode `0700`) lies outside it, out of
   reach of the file tools of a `chat` or `scoped` turn.
3. `runner.hosted_secrets_dir` (default empty) holds the secrets of hosted agents, one subfolder
   `<agent_id>/` per agent, filled by the host operator. A definition's `secret_ref` resolves only
   to a file in its own agent's subfolder (4.2), read below the real path of the folder, so a
   subfolder that is a link resolves nothing; never to the runner's environment, another
   agent's subfolder or a credential of the runner itself (`agent_token_ref`, `dpop_key_ref`,
   `anthropic_api_key_ref`). Empty, no reference of a definition resolves. With
   `runner.hosted_agents` the runner refuses to start when the key is not an absolute folder (a
   link is refused), when group or others may write it, when it is equal to, inside or above
   `runner.hosted_agents_root`, `runner.state_dir` or `runner.projects_root`, or when a `file:`
   credential of the runner lies in it. Reason: a definition sends the value as a header or an
   environment value to a server of the owner's choice, also under `scoped`, so without a scope
   it could carry out any secret of the runner user (red team of A1-05).

### 5.2 Files the runner writes

On start, on `agent.definition_changed` and before every turn, the runner fetches the current
definition (`agent.definition.read` with the hosted token) and writes, never copying a file from
the definition:

| File | Content |
|---|---|
| `<agent folder>/CLAUDE.md` | `instructions` |
| `<agent folder>/.claude/settings.local.json` | the runner settings of 5.4, only when the turn uses `--setting-sources` (5.3) |
| `<control>/settings.json` | the runner settings of 5.4 |
| `<control>/plugin/.claude-plugin/plugin.json` | `name` (`bothy-agent-<agent_id>`) and `version` (the definition version), nothing else |
| `<control>/plugin/skills/<name>/SKILL.md` | YAML frontmatter with exactly `name` and `description`, written by a YAML serializer as quoted scalars (a line break in a value cannot start a new key), then `body` |
| `<control>/plugin/skills/<name>/<file name>` | each `files[]` entry, flat, under its validated name |
| `<control>/mcp.json` | the `http` servers, and under `host` also the `stdio` servers, with `secret_ref` entries unresolved |
| `<control>/turns/<run_id>.mcp.json` | per turn: `mcp.json` with every `secret_ref` resolved within the agent's scope of 5.1 item 3, mode `0600`, deleted when the turn ends and swept at start; a reference out of scope or unreadable ends the run with the cause `mcp_reference_unavailable` and writes no file |

Secret values reach a turn only through the per-turn MCP file, never through the environment of
the turn and never through a file in the agent folder or in the turn's `TMPDIR` (T-BA-02).

### 5.3 Command line of a hosted turn

A hosted turn uses the argument list of 09 18.2 with the hosted agent's model (`settings.model`,
else `runner.default_model`) and the flags of its tool class, with these changes:

| Class | Tool flags | Added directly after `--permission-prompts none`, before `--add-dir` |
|---|---|---|
| `chat` | `--tools` gets `Skill` and `--allowedTools Skill` follows it when the definition has skills, else `--tools` stays `""`; `--disallowedTools "mcp__*"` stays | `--restricted --settings <control>/settings.json`, with skills `--plugin-dir <control>/plugin`, then `--strict-mcp-config` |
| `scoped` | `--tools <baseline and intersected tools>` before `--allowedTools`; `--allowedTools` names the intersected tools of 4.4 item 2 plus `Skill` when the definition has skills | `--restricted --settings <control>/settings.json --plugin-dir <control>/plugin --mcp-config <turn MCP file> --strict-mcp-config` |
| `host` | unchanged | `--setting-sources project,local --settings <control>/settings.json --plugin-dir <control>/plugin --mcp-config <turn MCP file> --strict-mcp-config` |

Every class also gets `--max-budget-usd <settings.max_cost_usd_per_run>` when the definition sets
it, directly before `--append-system-prompt-file`. Under `scoped` the baseline of `--tools` is what
09 18.2 grants every `scoped` turn already (`Read`, `Grep`, `Glob` and `Bash`, which `dontAsk`
limits to read-only commands) plus `Skill` when the definition has skills, followed by the
intersected tools. `--mcp-config` takes several values, so it is always followed by
`--strict-mcp-config`, never by a positional argument; `--plugin-dir` and `--mcp-config` are left
out when there is nothing to load. `--strict-mcp-config` stands in every class, also under `chat`
without `--mcp-config`, because `--restricted` alone still starts the MCP servers of the host
user's configuration ("add --strict-mcp-config to skip MCP servers too", help text of the pinned
CLI). `dontAsk` refuses a tool that is not allowed in advance, so `Skill` is named in
`--allowedTools` wherever `--tools` carries it. `--restricted` keeps user, project and local
settings files out and confines the file tools to the working folder (help text of the pinned CLI
2.1.288, read on 2026-11-06); its fallback is in 5.7. A test compares the whole list per class (BA-T13).

### 5.4 Runner settings

The settings file holds exactly: `disableSkillShellExecution: true`, `disableAllHooks: true`,
`disableWorkflows: true` and `permissions.deny` with edit and write rules for the control files
inside the working folder (`CLAUDE.md`, `CLAUDE.local.md`, `.claude/**`, `.mcp.json`,
`.lsp.json`, `.claude-plugin/**`). It holds no allow rule, no `env`, no command helper (among them
`apiKeyHelper`, `proxyAuthHelper`, `otelHeadersHelper`, `awsAuthRefresh`, `awsCredentialExport`,
`gcpAuthRefresh`, `statusLine`, `fileSuggestion`), no MCP switch, no plugin or marketplace key and
no `additionalDirectories`. The pinned CLI marks `disableAllHooks`,
`disableSkillShellExecution` and `disableWorkflows` as restrictive in its settings merge, so a lower
source cannot turn them off.

### 5.5 Surfaces the definition never reaches

The CLI interprets these places as code or as permission grants; the closed schema (4.3) keeps the
definition out of each, and the runner owns or removes each file (5.6). Read from the strings and
the `--help` of the pinned CLI 2.1.288 on 2026-11-05 and 2026-11-06 (design note D-A1-18 and the
red team rounds of the design note).

| Surface | What the CLI does with it | Closed by |
|---|---|---|
| `SKILL.md` frontmatter | `hooks`, `allowed-tools`, `model`, `effort`, `shell`, `context` with `agent`, `paths`, `disable-model-invocation`, `user-invocable`, internal keys such as `mcpServers`, `agents`, `settings`, `lspServers`, `monitors` | the runner writes `name` and `description` only (5.2); `skill_frontmatter` |
| Inline shell in skill and command text | `` !`cmd` `` and `` ```! `` blocks run as shell | `inline_shell`; `disableSkillShellExecution` |
| Settings files | `hooks`, `permissions` (allow rules, `defaultMode`, `additionalDirectories`), `env`, command helpers (5.4), MCP switches, plugins and marketplaces | no settings member; the runner writes the only settings; `--restricted` below `host` |
| Folder content | `.claude/skills`, `.claude/commands`, `.claude/agents`, `.claude/workflows`, `.claude/rules`, `.claude/hooks`, `.mcp.json`, `.lsp.json`, `CLAUDE.local.md` | not in the schema; removed on rebuild (5.6); `.mcp.json` also ignored under `--strict-mcp-config` |
| Plugin folder | `.claude-plugin/plugin.json`, `hooks/hooks.json` with a hooks module, `.mcp.json`, `.lsp.json`, `monitors/monitors.json`, `commands/`, `agents/`, `output-styles/`, `bin/` | the runner writes `plugin.json` with name and version and `skills/` only; everything else in the plugin folder is removed |
| MCP configuration | an `http` server with `headersHelper` (a script) | the runner writes `url` and `headers` only |
| Developer modifications of the CLI | hooks-like code that `disableAllHooks` turns off | `disableAllHooks` |
| `@` imports in `CLAUDE.md` | read files into the prompt; outside the project only after an approval flag of the host user | `import_outside_folder` |

### 5.6 Rebuild and the runner's own check

Before every turn the runner:

1. checks the current definition against 4.2 and 4.3 with its own copy of the rules; a definition
   that fails, or one with a `stdio` server under a token class below `host`, ends the run
   `failed` with `tool_class_mismatch` before `run.start`, because it claims more than its class;
2. rewrites every file of 5.2 it owns and compares the control folder with the definition;
3. removes every file and folder of the surfaces of 5.5 that it did not write, anywhere in the
   agent folder and the control folder (any `.claude/` content except its own
   `settings.local.json`, any `.mcp.json`, `.lsp.json`, `CLAUDE.local.md` and `.claude-plugin/`),
   and leaves the agent's other files in place;
4. refuses a turn for a hosted agent without a definition with `adapter_error` and the cause
   `definition_missing` in the summary (09 18.9, D-RUNNER-NO-GIT-5).

### 5.7 Points the build probes against the pinned CLI

The strings of the binary do not settle these; A1-05 probes each headless against the pinned
version with fake and planted files, records the result in the runner README and in this section,
and applies the fallback when the probe fails:

| Point | Expected | Fallback when it does not hold |
|---|---|---|
| `--restricted` with `--settings`, `--plugin-dir` and the working folder's `CLAUDE.md` under `chat` and `scoped` | settings and plugin skills apply, `CLAUDE.md` of the working folder is read | `--setting-sources project,local` with `.claude/settings.local.json` for every class |
| A skill outside the plugin folder (user or project skills of the host user) | not reachable through `Skill` | below `host` the runner loads no skills and leaves out `Skill`; the preview says so |
| A plugin skill under `chat` and under `scoped` with `dontAsk`, `--tools` and `--allowedTools` naming `Skill` | the skill loads and its text reaches the turn | below `host` the runner loads no skills and leaves out `Skill`; the preview says so |
| `--strict-mcp-config` without `--mcp-config` under `chat`, with an MCP server in the host user's configuration | no MCP server process starts | below `host` the runner passes `--mcp-config` with an empty server list before `--strict-mcp-config` |
| `--tools` against `allowed-tools` of a planted skill | no tool outside `--tools` | skills only under `host`, as above |
| `disableAllHooks` against hooks of a planted skill or plugin | no hook runs | the closed schema alone; recorded as a residual risk |
| A plugin `bin/` folder | not on the turn's `PATH` | the runner never writes `bin/` and removes it (5.6) |
| The user-level `CLAUDE.md` of the host user and a `CLAUDE.md` above the agent folder | not loaded | named in the runner README and in the preview of the creation flow; the runner keeps the start refusal of 5.1 item 1 |
| A settings file changed during a turn | not reloaded with wider rights | the deny rules of 5.4 and the rebuild before the next turn |
| A read-only shell command under `dontAsk` reading another process's environment | refused | recorded as a residual risk of T-BA-02; secrets stay out of every environment (5.2) |
| `Read` and a read-only shell command (`cat`) under `scoped` reading the per-turn MCP file of its own turn and of a concurrent turn in the control folder | refused, the path lies outside the working folder | below `host` the runner loads only MCP servers without a `secret_ref`; the preview says so, and the gap is recorded as a residual risk of T-BA-02 |
| The end of a turn stopped by `--max-budget-usd` | a `result` the runner recognizes | the run ends `failed` with `runtime_error` and the cause `cost_cap_reached` in the summary either way (5.8) |

Results (A1-05, pinned CLI 2.1.288, `tests/real_cli/test_hosted_probe.py`): the free level of the
probe ran on 2026-11-06 and passed: `--help` names every flag of 5.3 (`--tools` takes several
values), and the CLI holds the three switches of 5.4, the `result` subtype
`error_max_budget_usd` that a turn stopped by `--max-budget-usd` ends with, and the placeholder
"[shell command execution disabled by policy]". The probe turns of each row (one real turn per
row, behind the owner's switch for paid turns) have not run yet, so no row is proven and no
fallback is taken; the runner builds the expected behaviour of each row. The runner recognises
the cost cap by that subtype and, as the fallback of the last row, by a cost at or above the cap
(decisions.d/A1-05.md).

### 5.8 Cost cap

`settings.max_cost_usd_per_run` becomes `--max-budget-usd`. A run stopped by it ends `failed` with
`runtime_error` and the cause `cost_cap_reached` in the summary of `finishRun`, which the apps show
next to the reason (R-AG-35); its costs are reported as for every run (09 18.7). A monthly budget
per agent is not part of this stage.

### 5.9 Channel skills behind their own key

`runner.channel_skills_dir` (default empty, off) turns on skills of `channel_skill_bindings` for
turns of class `host` only, of the runner's own agent and of hosted agents. A `skill_ref` of the
form `claude-code:<name>` with `<name>` matching `^[a-z0-9-]{1,64}$` resolves to
`<runner.channel_skills_dir>/<name>/` (a folder with a `SKILL.md`), handed to a `host` turn as one
more `--plugin-dir`; every other reference belongs to another adapter and is skipped with a debug
log line only. A `claude-code:` reference without a folder is logged and named in the live box, not
fatal. A turn of class `chat` or `scoped` loads no channel skill: the runner adds no `--plugin-dir`
for it, logs the skipped references and names them in the live box once per run with the reason
that channel skills need `host` (R-BA-13, BA-T22).

Reason for `host` only: a channel skill folder is the host operator's own content, not data that 4.3
checks; its frontmatter (`allowed-tools`, `hooks`), its inline shell and any plugin part in it would
widen a `chat` or `scoped` turn, and which skill a channel gets is decided by a person with `manage`
on the channel, not by an owner (T-BA-03). A `host` turn already runs with the rights of the host
user and only owners trigger it (SR2), so a channel skill adds nothing such a turn could not do.
It still acts in that turn: a member with `manage` on a channel can bind a channel skill there,
and it then runs in a `host` turn that an owner starts in the channel, with no right the member
lacked before but with content the owner did not choose. The runner README tells the operator
so before the key is set.
The key is new in this stage, so no running installation loads channel skills today and the rule
changes no existing setup; a `host` agent of the runner loads them as soon as the operator sets the
key.

Without the key the runner reads no bindings, and its command line, heartbeat and live box are
byte-equal to the fixtures, also for a channel with bindings of both forms (BA-T20).

## 6. Interview

### 6.1 Request and answer

1. `interviewAgent` (action `agent.interview`, owner, `rl.trigger`, effect `write`, so the
   maintenance mode refuses it like every audited action, 05 2.7) carries the
   interview so far, kept by the client: the person's answers and the builder's earlier
   questions, at most `agents.interview.max_turns` turns of at most
   `agents.interview.answer_max_chars` characters each. The server sends the ephemeral event
   `agent.interview_requested` to one live gateway connection of the builder agent named in
   `agents.builder_agent_id` (consumer `default`, else the most recently attached one), with the
   history, `want_draft`, the template catalogue of 7.4, the definition limits and `expires_at`,
   without a token, and waits at most `agents.interview.timeout`. The request's own deadline is
   `agents.interview.timeout` plus `http.request_timeout` instead of `http.request_timeout`
   (02 14.2), so the wait ends before the request does. An interview that does not fit
   `sync.event_payload_max_bytes` is `400 validation_failed` with `too_large` on `/turns`
   (decisions.d/A1-07.md).
2. The builder answers once with `answerAgentInterview` (action `agent.interview.respond`,
   agent-self, only the builder the request was sent to, `rl.agent_runtime`): either
   `{"kind": "questions", "questions": [...]}` with one or two questions, or
   `{"kind": "draft", "draft": {...}}` with a draft of the shape of 4.2 plus `name`, `handle`,
   `description` and a channel suggestion; and `costs` (09 4.10). A second answer is
   `409 conflict` with `already_answered`, an answer after `expires_at` `410 gone`, and every
   principal but the builder the request was sent to gets `404`.
3. The server validates a draft with the rules of 4.3 and the catalogue check of 7.4, never
   trusting model output: a draft that
   fails is answered with the field errors and the person can ask the builder to fix it. A builder
   answer that reaches a surface of 5.5 (a frontmatter, an inline shell form or an `@` import
   outside the folder), carries a text that is no display text or pairs `kind` with a missing or
   the other member is refused as a whole with `400` to the builder and the fallback
   `builder_invalid` for the person. A member outside the answer schema, the remaining surface of
   5.5, never gets that far: the request validation of the API refuses it with `400`, and the
   request stays open until `expires_at`. It stores nothing of the interview;
   the answer shows the cost as an estimate like the search assistant (D-IS-8) and books no cost
   entry.
4. Fallbacks: no `agents.builder_agent_id`, no live connection, no valid answer within the
   deadline. The answer then names the reason (`builder_not_configured`, `builder_offline`,
   `builder_timeout`, `builder_invalid`) and the apps offer the templates and the manual form.
5. Audit and logs: every request writes one audit entry `agent.interview` with the outcome, the
   fallback reason, the number of turns and the cost estimate, never a question, an answer or a
   draft; one log line names the same counts (R-BA-09). Reason: an interview spends money on the
   builder's account.

### 6.2 The builder in the runner

With `runner.agent_builder` (default `false`) the runner answers `agent.interview_requested` when
it is the agent of `agents.builder_agent_id`:

1. One `claude -p` without tools (the flags of `chat`, 09 18.2) with `--output-format json`, the
   model `runner.agent_builder_model`, the time limit `runner.agent_builder_timeout` and never past
   `expires_at`; the billing rules of the search assistant (09 18.17 item 1); a private folder below
   `runner.state_dir` as working folder, no session, removed afterwards.
2. A fixed instruction of the package asks for one JSON object of the answer shape of 6.1 item 2;
   the answer schema has no member for settings files, hooks, permission rules, commands,
   subagents or plugin parts (4.3 item 1). The history and the catalogue follow as a data block
   marked as untrusted content.
3. The runner parses the last JSON object of the reply, checks it against the answer schema and
   the rules of 4.3 with its own copy, and sends nothing for an answer that fails; the request then
   falls back on the server.
4. The builder writes no file and holds no tool (D-A1-13).

### 6.3 Questions

The builder asks only what the draft needs, one or two questions per turn, and stops when it can
fill every member: what the agent does, for whom and what it must never do; where people talk to
it; language and tone; whether it reads or changes files, runs commands or only talks (the tool
class wish); which outside systems it reaches (only MCP servers that templates declare, 7.4; it
names the secrets the owner adds on the runner host by name only and says that a `stdio` server
makes the agent `host`); which recurring tasks deserve a skill; which model and which cost cap per
run.

## 7. Templates

### 7.1 Layout

One folder per template under `templates/agents/<slug>/` (`<slug>` matching
`^[a-z0-9][a-z0-9-]{0,63}$`) with exactly `template.yaml`, `CLAUDE.md` and
`skills/<name>/SKILL.md` per skill (frontmatter with `name` and `description` only). Schema
`agent_template.v1` of `template.yaml`: `name`, `description`, `version`, `tool_class_wish`,
`model_hint`, `required_secrets[]` (names only), `mcp_servers[]` (the shape of 4.2) and
`skills[]` (names of the skill folders). A template becomes a definition draft one to one, with
`origin.kind = template`.

`name`, `description`, `version`, `tool_class_wish` and `skills` are required; `model_hint`
(becomes `settings.model`), `required_secrets` and `mcp_servers` are optional, and no other key is
allowed. A `secret_ref` of a template is `env:<NAME>` with `NAME` listed in `required_secrets`.
The draft takes `CLAUDE.md` as `instructions` and, per skill, the frontmatter `description` and
the text after the frontmatter as `body`; it is checked with 4.3 for the tool class
`tool_class_wish` (decisions.d/A1-06.md).

### 7.2 Shipping and private templates

1. A `go generate` step copies the shipped templates into the server module, where the binary
   embeds them, and `make codegen-check` holds the copy equal to `templates/agents/` (Go embeds
   only files below its module). `listAgentTemplates` (action `agent.templates.read`, owner)
   returns them.
2. `agents.templates_dir` (default empty) names a folder outside the repository with private
   templates of the same layout, read at start. A private template whose slug equals a shipped one
   is skipped with the log code `template_slug_taken`; one that fails the schema is skipped with
   `template_invalid` (log attribute `bothy.reason`); a file at the root of the folder is
   ignored, and a folder that cannot be read stops the start. Private templates never enter the
   repository (D-A1-25).
3. `listAgentTemplates` answers the shipped templates by slug, then the private ones by slug,
   each with `source` (`shipped` or `private`), `name`, `description`, `version`,
   `required_secrets` and the draft as `definition` (schema `AgentTemplate`).

### 7.3 Template gate

`make templates-check`, part of `make check` and of the pull request CI, validates every template
under `templates/agents/` and fails on:

1. a file other than those of 7.1, among them `.claude/`, `.mcp.json`, `.lsp.json`, a plugin
   manifest, hooks, commands, agents and output styles;
2. a schema error of `template.yaml` or a `SKILL.md` frontmatter key other than `name` and
   `description`;
3. any refusal of 4.3 applied to the template as a definition draft;
4. a host, URL or e-mail address outside the example domains, an absolute path, an IP address, a
   person's name outside the example persons (Alex, Sam, Robin), a secret-looking value;
5. every finding of the generic deny rules (`denylist_scan.py --allow-missing` over
   `templates/`).

The private deny list runs over templates like over every other file, in `make denylist`, the pull
request job and `denylist-main`. The gate's tests carry one failing fixture template per surface of
5.5 and per rule above (`tools/templates/tests/fixtures/<rule>/`). Item 3 uses a port of the
server's rules (`tools/templates/definition_rules.py`) that runs the corpus `tests/agent-definitions`;
the person rule of item 4 is a heuristic over person positions (decisions.d/A1-06.md D-A1-06-1).
The terms the maintainers know to be private are added to the private list
and its CI secret by the owner before the first template lands (D-A1-16, PLAN section 21 "Owner").

### 7.4 Catalogue for the builder

The template catalogue sent with an interview request holds slug, name, description, tool class
wish and the MCP servers of each template (shipped and private). The builder suggests MCP servers
only from it; the server checks the draft against it: a drafted server matches a catalogue server
only if its whole entry except `name` is exactly equal. For `stdio` that is transport, command,
package, the full argument list (same order and count) and every `env[]` entry; for `http` it is
transport, URL and every `headers[]` entry. An `env[]` or `headers[]` entry is equal when it has
the same name and the same `value` or the same `secret_ref`; a switch between `value` and
`secret_ref` is a difference. The argument list compares as a list; `env[]` and `headers[]`
compare as sets by name (a repeated name in one list is `duplicate_name`), so their order does
not matter but a missing or an extra entry is a difference. Any other field could redirect
traffic or swap credentials (an argument or an environment value can carry an endpoint, a
`secret_ref` names another key). A server without such a match is the field error
`mcp_not_in_catalogue` at its JSON Pointer (6.1 item 3).

### 7.5 Topics

Shipped templates are written for the repository from scratch, never exported from an
installation, and generic: for example a writing helper, a code reviewer in a folder, a research
summarizer, a meeting notes helper. None mirrors an agent, profile, company, host, person or
internal script of a real installation.

## 8. Creation and editing in the apps

### 8.1 Entry

Desktop, phone and web console show "New agent" only to owners and only when the server answers
the operations of this chapter and at least one active runner reports `hosts_agents`; on an older
server or without such a runner the entry is hidden. The apps read this from the agent list they
already have: `getAgent` and `listAgents` show an owner the stored capability of each runner as
`presence.hosts_agents` (`AgentRuntimeStatus`); a runner that does not report it, every other
caller, `agent.updated` and the heartbeat answer carry no such member (addendum of 2026-11-07,
A1-03b).

### 8.2 Steps

The dialog offers a template, the interview or the manual form, then a preview of the whole draft:
instructions, each skill, each MCP server with the secret names it needs, tool class (with what
the class does not load, 4.4 and 5.7), billing mode, model and cost cap, all editable, and the
hosting runner, editable until `agent.create` succeeded and read-only afterwards (8.3). "Create"
runs, in this order and each visible with its state (01 6.9 pattern): `agent.create` with
`hosted_by` and `adapter_kind = 'claude_agent'` (step-up); `agent.tool_class.set` when the class
is not `chat`; `putAgentDefinition`; `agent.billing_mode.set` when `personal` is chosen
(10 10.3); on request `channel.create` in the chosen space and `channel.operator.set` with the new
agent. A step-up prompt appears where an action needs one. The class comes before the definition
because the server refuses a definition with a `stdio` server for an agent below `host` (4.3
item 7); until the definition is stored the agent is "not set up" and runs no turn (8.3), so the
raised class serves nothing in between.

### 8.3 Failure of a step

Every step is its own action with its own audit entry; nothing is rolled back. When a step after
`agent.create` fails, the agent stays with `chat`, without definition or without channel, as far as
the steps went. The dialog shows the failed step with one sentence for its cause and lets the owner
edit the preview. "Try again" compares each step's target from the edited fields with what the
agent has (class, current definition, billing mode, channel and operator) and runs, in the order
of 8.2, every step whose target is not reached yet, with the class rule of 8.4: a class above the
agent's current one is set before the definition, a lower one after it. So a class raised in the
preview after a refused definition is set before the definition is written again, and a class
lowered after a stored definition with a `stdio` server waits until the definition without it
is stored (R-BA-06); it creates nothing twice.
"Close" keeps what exists. Once `agent.create` succeeded, the hosting runner is shown but not
editable in the preview; moving the agent to another runner is `agent.host.set` of 8.4, so
"Try again" never finishes on a runner other than the one shown.
The agent list marks a built-in agent without a definition as "not set up" with "Continue setup",
"Change runner" (`agent.host.set` of 8.4, 01 6.20 item 6) and "Disable". Its hosting runner
serves no turn for it (5.6 item 4). A test per step holds that a failure leaves exactly the
earlier steps and their audit entries (BA-T16).

### 8.4 Edit and remove

Owners edit instructions, skills, MCP servers and settings with the same preview and step-up
(`putAgentDefinition`), change the host (`agent.host.set`), and disable or delete through the
existing actions (`agent.disable`; 09 2.1). The version list offers undo (4.1 item 2): "Restore"
opens the older version in the same preview and saves it as a new version like an edit, with a
`stdio` server in it preselecting `host`, so the class rule below holds for it too. When an
edit or a restore also changes the class, saving runs two visible steps: a raised class is set
before the definition is written (`agent.tool_class.set`, then `putAgentDefinition`), so a newly
added `stdio` server is accepted; a lowered class is set after it (`putAgentDefinition`, then
`agent.tool_class.set`), so a removed `stdio` server no longer blocks the downgrade of R-BA-06. A
failed step keeps the earlier one and offers "Try again" as in 8.3.

## 9. Actions, operations and events

Permissions, step-up, audit and rate-limit classes: [07-permissions.md](07-permissions.md) 2.13.

| Operation | Method and path | Action |
|---|---|---|
| `createAgent` (additive member `hosted_by`) | `POST /api/v1/agents` | `agent.create` |
| `getAgentDefinition`, `listAgentDefinitionVersions` | `GET /api/v1/agents/{agent_id}/definition`, `GET /api/v1/agents/{agent_id}/definition/versions` | `agent.definition.read` |
| `putAgentDefinition` | `PUT /api/v1/agents/{agent_id}/definition` | `agent.update` |
| `setAgentHost` | `PUT /api/v1/agents/{agent_id}/host` | `agent.host.set` |
| `listHostedAgents` | `GET /api/v1/agents/me/hosted` | `agent.hosted.list` |
| `issueHostedAgentToken` | `POST /api/v1/agents/me/hosted/{agent_id}/token` | `agent.hosted_token.issue` |
| `interviewAgent` | `POST /api/v1/agents/interview` | `agent.interview` |
| `answerAgentInterview` | `POST /api/v1/agent-interview-requests/{request_id}/answer` | `agent.interview.respond` |
| `listAgentTemplates` | `GET /api/v1/agent-templates` | `agent.templates.read` |

| Event | Kind | Resource | Receivers |
|---|---|---|---|
| `agent.definition_changed` | durable, personal | agent | the agent itself (receive action `agent.definition.read`); payload `definition_version` only |
| `agent.hosting_changed` | durable, personal | agent | the old and the new hosting runner (receive action `agent.hosted.list`); payload `agent_id` only |
| `agent.interview_requested` | ephemeral | agent | one gateway connection of the builder agent; no token |

`getAgentDefinition` takes the optional query `version`: owners read an older version for undo
(8.4), the agent itself reads only its current version.

`agent.updated` keeps its meaning and also follows `hosted_by` and a new definition version
(`agents.version`).

`getAgent` and `listAgents` (`agent.read`) add for an owner the member `hosts_agents` to the
`presence` of a runner that reports the capability (8.1, A1-03b); the action, its right and every
other reader stay as they were.

## 10. Billing

Built-in agents follow 10 unchanged; 10 10.3 holds the hosted case. A new agent is
`organization` (I1 default). `personal` is offered only when the creating owner is the only owner,
the hosting runner is `personal` with the same `credential_holder_id` as the one the hosted agent
gets, and it reports `personal` in `billing_modes_supported`. The `credential_holder_id` of a
hosted agent in `personal` always equals that of its hosting runner (R-BA-12).

## 11. Protecting a running installation

1. **Additive only.** Every new column is nullable or defaults to "as before"; every new table,
   operation, event, capability and key is new; no existing field changes meaning. Migrations are
   expand steps with one version back (R-OP-12); this stage writes no contract step.
2. **Off until asked for.** `runner.hosted_agents`, `runner.agent_builder`,
   `runner.channel_skills_dir` and `agents.builder_agent_id` default to off; the entry "New agent"
   needs a runner that reports `hosts_agents`.
3. **Byte-equal fixtures.** Without the new keys the runner's command line, heartbeat and served
   channels equal the fixtures of the version before this stage; the adapter for another agent
   runtime gets no change in this stage; the apps tolerate the new fields and operations missing
   on an older server.
4. **Rollback.** Turning `runner.hosted_agents` off takes the hosted agents offline (their runs end
   like those of any offline agent) and leaves the runner's own agent as before; their definitions
   stay. Going back one server version needs `runner.hosted_agents` off on every runner first: the
   older server knows no capability `hosts_agents`, so it refuses such a heartbeat as an unknown
   member (09 4.8), while a runner with the key off sends the heartbeat of the version before this
   stage (item 3). The older server then leaves the new table and column unread.

## 12. Tests

| ID | Test | Expected |
|---|---|---|
| BA-T01 | `putAgentDefinition` with one member per surface of 5.5 (frontmatter in a body, both inline shell forms, an `@` import with an absolute, a `~` and a `..` path, an unknown member at every level) | `400 validation_failed`, one field error each, nothing stored (A1-02) |
| BA-T02 | definition with a `stdio` server for an agent below `host`; unpinned package; command outside the allow list; plain secret value; reserved environment name | refused with the field codes of 4.3 (A1-02) |
| BA-T03 | `agent.tool_class.set` below `host` while the definition has a `stdio` server | `409 conflict`, `definition_needs_host` (A1-02) |
| BA-T04 | a person who is not an owner calls each action of 07 2.13 | `404` or `403` as 07 2.13 says, audit `deny` (A1-02, A1-03, A1-06, A1-07) |
| BA-T05 | `issueHostedAgentToken` by the hosting runner, by another runner, for an agent hosted elsewhere, for a disabled agent | token only in the first case; `404` otherwise; audit entry without the value (A1-03) |
| BA-T06 | revoke a token of the hosting runner; disable the runner; disable the hosted agent; change the host | every hosted token of that runner and key revoked, connections closed with `4012` (A1-03) |
| BA-T07 | `createAgent` with `hosted_by` naming an agent without `hosts_agents`, a hosted agent, itself; `claude_agent` without `hosted_by` | `host_not_capable` or `validation_failed` (A1-03) |
| BA-T08 | `putProjectBinding` and `setChannelOperator` on a bound forum with a built-in agent | `operator_cannot_serve_project` (R-AG-39, A1-03) |
| BA-T09 | runner with `runner.hosted_agents`: two hosted agents, shared slots, separate cursors and state folders; no token in any file under `runner.state_dir` or `runner.hosted_agents_root` | as 3.2 and 3.3 (A1-04) |
| BA-T10 | runner start with `runner.hosted_agents_root` inside or above `runner.state_dir` or `runner.projects_root`, with a `CLAUDE.md` above it, writable by others | start refused, naming the key (A1-05) |
| BA-T11 | materializer: files of 5.2 from a definition; frontmatter with a line break in `description`; a file name with `/` or `..`; settings file content exactly 5.4; a `secret_ref` naming the runner's own token, an `env:` variable, another agent's file or a link out of the agent's subfolder of `runner.hosted_secrets_dir` | as 5.2 and 5.4; injection and path escape impossible; an out-of-scope reference ends the run with `mcp_reference_unavailable` without a per-turn file (A1-05) |
| BA-T12 | rebuild: planted files of every surface of 5.5 in the agent folder and the control folder | removed before the turn; the agent's other files stay (A1-05) |
| BA-T13 | command line per class against 5.3 with fake Claude; `stdio` server under a token below `host`; a definition failing 4.3 | exact list; `tool_class_mismatch` before `run.start` (A1-05) |
| BA-T14 | probe against the pinned CLI with a planted skill carrying `hooks`, `allowed-tools: Bash` and both inline shell forms, and every point of 5.7 | no hook, no `Bash`, the placeholder "[shell command execution disabled by policy]"; results recorded in 5.7 (A1-05, real CLI behind its switch) |
| BA-T15 | interview: question turn, draft turn, a draft with an MCP server outside the catalogue (`mcp_not_in_catalogue`), a catalogue `stdio` server with only its endpoint argument changed, a catalogue `stdio` server with only the value of one `env[]` entry changed, a builder answer carrying each surface of 5.5, each fallback, timeout | draft validated; both changed servers `mcp_not_in_catalogue`; surfaces refused; fallback reason named; one audit entry and one log line without content (A1-07) |
| BA-T16 | creation flow: a failure at each step after `agent.create`; a template with a `stdio` server created with class `host`; a definition refused with `tool_class_too_low`, the class raised in the preview, "Try again"; a step after the definition failing for a definition with a `stdio` server under `host`, the server removed and the class set to `chat` in the preview, "Try again"; an edit that adds a `stdio` server and raises the class, and one that removes it and lowers the class; the hosting runner field after `agent.create` succeeded | earlier steps and their audit entries stay, nothing more; the hosting runner is read-only after `agent.create`, so "Try again" runs on the runner shown; `agent.tool_class.set` runs before `putAgentDefinition` when the class rises and after it when it falls, so the `stdio` cases are stored without `tool_class_too_low` or `definition_needs_host`; "Try again" runs the missing steps in the order of 8.2 and creates nothing twice (A1-08, A1-09) |
| BA-T17 | template gate: one failing fixture template per surface of 5.5 and per rule of 7.3; the shipped templates | each fixture fails with its rule; shipped templates pass (A1-06) |
| BA-T18 | private templates folder with a taken slug and an invalid template | skipped with their log codes (A1-06) |
| BA-T19 | cost cap reached in a turn (fake Claude) | run `failed`, `runtime_error`, cause `cost_cap_reached` in the summary, costs reported (A1-05) |
| BA-T20 | runner without the new keys, with channels carrying `channel_skill_bindings` of both forms | command line, heartbeat and served channels byte-equal to the fixtures before this stage (A1-04, A1-05) |
| BA-T21 | end to end on the demo stack with fake Claude: create from a template and from an interview, chat, costs shown, `runner.hosted_agents` off | as 8.2; with the key off the runner behaves as before (A1-10) |
| BA-T22 | runner with `runner.channel_skills_dir` and a channel skill folder carrying `allowed-tools: Bash`, `hooks`, both inline shell forms and a `hooks/` folder, bound to channels served by a `chat`, a `scoped` and a `host` turn (the runner's own agent and a hosted agent) | the folder reaches only the `host` turns as `--plugin-dir`; the `chat` and `scoped` command lines carry no `--plugin-dir` for it and their live box names the skipped reference (A1-05) |

## 13. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-BA-01 | A built-in agent is an agent principal of `adapter_kind = 'claude_agent'` hosted by a Claude Code runner named in `agents.hosted_by`; the runner keeps the CLI (3.1, 3.2). | Everything that makes an agent useful already works for agent principals; the runner is the only Claude runtime the product ships and speaks the adapter contract (D-A1-1 to D-A1-3, D-A1-5). |
| R-BA-02 | One runner process hosts many agents behind `runner.hosted_agents`, with a connection, cursor, cache, heartbeat and state folder per agent and shared slots (3.2). | A process per agent would need a service unit per agent, which a person creating an agent in an app cannot set up (D-A1-3). |
| R-BA-03 | Tokens of hosted agents go only to the hosting runner, bound to its DPoP key, kept in memory only, revoked with the hosted agent, a host change, the runner or the runner's token (3.3). | The server must not write files on the host, and a token shown in an app would be copied by hand; keeping it on the runner's key-bound connection adds no secret path for people (D-A1-4; design backlog items 4 and 5). |
| R-BA-04 | A definition is closed data: no member reaches a settings file, hook, permission rule, command, subagent, plugin part or command helper of the CLI; the runner writes every control file itself, frontmatter by a serializer with `name` and `description` only, extra files flat, and rebuilds before every turn (4.2, 4.3, 5.2, 5.6). | Below `host` any such content would let a `chat` or `scoped` agent run host code or widen its tools without SR2 (D-A1-7, D-A1-8, D-A1-18; red team r3 finding 1). |
| R-BA-05 | `stdio` MCP servers need `tool_class = 'host'`; `scoped` loads only `http` servers; `chat` loads none; the definition's tool list only narrows `runner.scoped_allowed_tools` (4.4). | A local process as the host user is the `host` case of 07 2.11; the host operator keeps the ceiling (D-A1-18). |
| R-BA-06 | `agent.tool_class.set` below `host` is refused while the current definition has a `stdio` server (4.4 item 5). | A silent downgrade would leave an agent that fails every turn with `tool_class_mismatch`; the owner removes the server first (design backlog item 10). |
| R-BA-07 | Below `host` a hosted turn runs with `--restricted`, `--settings` of the runner and an explicit `--tools` whose baseline is what 09 18.2 grants `scoped` already, plus `Skill` only with skills, then also named in `--allowedTools`; every class gets `--strict-mcp-config`; `host` uses `--setting-sources project,local`; each point of 5.7 has a probe and a fallback (5.3, 5.7). | `--restricted` drops user, project and local settings and confines file tools; an explicit `--tools` keeps the class baseline instead of removing read-only shell and skills (red team r3 findings 3 and 4); `--restricted` alone still starts the host user's MCP servers, and `dontAsk` refuses a tool not allowed in advance (red team of A1-01, minors 2 and 3). |
| R-BA-08 | Secret values reach a turn only through a per-turn MCP file of mode `0600` in the control folder, never through the environment (5.2), and only from the agent's own subfolder of `runner.hosted_secrets_dir` (5.1 item 3). | Concurrent turns of one host user can read each other's environment; a file outside the working folder is out of reach of `chat` and `scoped` file tools (design backlog item 4, T-BA-02). |
| R-BA-09 | The interview is one owner request with its own deadline to the builder agent, short-lived, nothing stored, every draft validated like a definition, audited and logged with counts only (6.1). | The search assistant pattern is built and tested; an interview costs money, so it is audited, but its content is the owner's text and stays out of audit and logs (D-A1-10, D-A1-12; design backlog item 6). |
| R-BA-10 | Templates are written for the repository, shipped in the image, listed by an owner read, held by a template gate that works without the private deny list; private templates come from `agents.templates_dir` (7). | Exporting a real agent is how private names leak; forks have no private list (D-A1-14 to D-A1-16). |
| R-BA-11 | Creation composes visible steps, each its own action and audit entry, without rollback; at creation, on "Try again" and on edit a raised class is set before and a lowered class after the definition; "Try again" runs the steps not reached yet in that order; a built-in agent without a definition is "not set up" and never served (8.2 to 8.4). | The pattern of "New project" (01 6.9); a rollback across actions would need a second write path (design backlog item 6); the server refuses a `stdio` server below `host` (4.3 item 7) and a downgrade while one is there (R-BA-06), so any other order cannot store such a definition (red team of A1-01, finding 1). |
| R-BA-12 | A hosted agent in `personal` has the `credential_holder_id` of its hosting runner; a runner without one hosts only `organization` agents (10). | The turn runs with the login of the runner's host user; I1 to I5 apply as they are (D-A1-19; design backlog item 8). |
| R-BA-13 | Channel skills only behind `runner.channel_skills_dir`, resolved from `claude-code:<name>`, and only in turns of class `host`; below `host` they are skipped and named in the live box (5.9). | A served channel may carry references of another adapter; turning them on without a key would change a running installation (D-A1-9); a channel skill folder is not checked like a definition and would widen a `chat` or `scoped` turn, while `host` adds nothing to a turn of the host user that only owners trigger (red team of A1-01, finding 2, decisions.d/A1-01.md). |
| R-BA-14 | Actions, operations and events of this chapter enter the catalogue and the API documents with the ticket that builds them, with the values of 07 2.13 and section 9. | A catalogue entry generates Go constants and route checks; an entry without its code would be a second truth until the build (decisions.d/A1-01.md). |

## 14. Configuration values of this chapter

Column "Place": Server = configuration reference of the server; Runner = configuration of the
Claude Code runner.

| Name | Place | Default | Meaning |
|---|---|---|---|
| `agents.builder_agent_id` | Server | empty | agent that answers interview requests (6); empty means no interview, templates and the manual form only |
| `agents.templates_dir` | Server | empty | folder of private templates outside the repository (7.2); empty means shipped templates only |
| `agents.interview.timeout` | Server | `90s` | longest wait for the builder's answer; the deadline of `interviewAgent` is this plus `http.request_timeout` instead of `http.request_timeout` alone (6.1) |
| `agents.interview.max_turns` | Server | `12` | most turns of the interview history in one request |
| `agents.interview.answer_max_chars` | Server | `4000` | longest turn of the interview in one request, and longest question of the builder |
| `agents.hosted_token_ttl` | Server | `24h` | lifetime of a token for a hosted agent (3.3) |
| `agents.mcp_stdio_runners` | Server | `["npx", "uvx"]` | commands a `stdio` MCP server may use (4.2) |
| `agents.definition.instructions_max_bytes` | Server | `65536` | longest `instructions` |
| `agents.definition.max_skills` | Server | `32` | most skills of a definition |
| `agents.definition.skill_description_max_chars` | Server | `300` | longest skill description, one line |
| `agents.definition.skill_body_max_bytes` | Server | `65536` | longest skill body and extra skill file |
| `agents.definition.max_skill_files` | Server | `8` | most extra files of one skill |
| `agents.definition.max_mcp_servers` | Server | `16` | most MCP servers of a definition |
| `agents.definition.max_allowed_tools` | Server | `64` | most entries of `settings.allowed_tools` |
| `agents.definition.max_cost_usd_per_run` | Server | `50` | highest cost cap per run a definition may set |
| `runner.hosted_agents` | Runner | `false` | host the agents whose `hosted_by` is this runner (3.2) |
| `runner.hosted_agents_root` | Runner | empty | root of the agent and control folders; required with `runner.hosted_agents`; never inside, equal to or above `runner.state_dir` or `runner.projects_root` (5.1) |
| `runner.hosted_max_tool_class` | Runner | `scoped` | highest tool class loaded for a hosted agent, reported as the value of `hosts_agents` (3.1) |
| `runner.hosted_secrets_dir` | Runner | empty | folder with one subfolder `<agent_id>/` per hosted agent; a definition's `secret_ref` resolves only to a file there; empty means no reference of a definition resolves (5.1) |
| `runner.mcp_stdio_runners` | Runner | `["npx", "uvx"]` | commands the runner starts for a `stdio` MCP server; a definition naming another command is refused before the turn (5.6) |
| `runner.agent_builder` | Runner | `false` | answer interview requests when this runner is the builder agent (6.2) |
| `runner.agent_builder_model` | Runner | `sonnet` | model of the builder call (6.2) |
| `runner.agent_builder_timeout` | Runner | `80s` | time limit of one builder call; below the server's `agents.interview.timeout` (6.2) |
| `runner.channel_skills_dir` | Runner | empty | folder of channel skills for `claude-code:<name>` references, loaded only in `host` turns; empty means channel skills off (5.9) |
