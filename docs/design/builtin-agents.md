# Built-in agents

Status: design of 2026-11-05 for the stage "A1 Built-in agents" and the rest of V1. Nothing in it
is built yet. This note records the goal and the decisions with their reasons. The normative text
is in [26-builtin-agents.md](../spec/26-builtin-agents.md) and the chapters it names, the decision in
[ADR 0076](../adr/0076-built-in-agents-hosted-by-the-claude-code-runner.md), the tickets in
[PLAN](../PLAN.md) sections 15 and 21 and their briefs in the [brief index](../briefs/README.md),
all written by A1-01 (rulings: [decisions.d/A1-01.md](../decisions.d/A1-01.md)). Where this note
and a chapter disagree, the chapter wins.

Why a design note and not an ADR: the decisions span the data model, the actions, the adapter
contract, the runner, the clients and the release work, and several of them are product rulings
rather than architecture. The repository keeps such cross-cutting drafts in `docs/design/` (for
example [new-project.md](new-project.md) and [search-assistant.md](search-assistant.md)); the
architecture part becomes one ADR in A1-01, and the rulings go to `docs/decisions.d/` with the
tickets that build them.

## 1. Goal

Owner request of 2026-11-05 (translated and shortened): Bothy should become interesting for people
outside the maintainer's own installation, without breaking that installation.

1. **An agent that lives in Bothy.** A person creates an agent inside Bothy, and it runs on local
   Claude through the existing Claude Code runner. No other agent runtime is needed to try Bothy.
   Only Claude; no other model vendors.
2. **Created through an interview.** Like the dialog "New project" and the search assistant,
   Bothy asks "you want to create an agent", runs a short interview and produces the agent with
   its instructions (`CLAUDE.md`), skills, MCP configuration and settings. Afterwards the person
   chats with it.
3. **Ready-made templates, strictly anonymous.** The repository ships generic templates. Nothing in
   them, or anywhere else in the repository, says which agents, profiles, companies, hosts,
   people or tickets any real installation uses.
4. **Finish V1.** The publication checklist, the placeholders such as `OWNER` and an external test
   from zero. The working name stays.

Constraints: there is one `main` for everyone; private data lives outside the repository. Every
change is backwards compatible and additive: a running installation with its runner, its adapter
for another agent runtime and its apps keeps working unchanged after an update, with no new
configuration.

### 1.1 Done means

1. An owner opens "New agent" in the desktop app, the phone app or the web console, picks a
   template or starts the interview, sees a preview of everything that will be created, edits it,
   confirms with step-up, and can chat with the new agent in a channel of their choice right away.
2. The agent's turns run in the Claude Code runner with the agent's own instructions, skills and
   MCP servers, in a folder of its own, with its tool class, its billing mode and a cost cap per
   run; costs appear in the existing statistics.
3. The templates pass a repository gate that proves them generic without the private deny list,
   and the private deny list scan passes over the whole repository.
4. An installation that sets none of the new keys behaves exactly as before: the runner's command
   line, heartbeat and served channels are byte-equal to today's fixtures, and the adapter for the
   other runtime is untouched.
5. Every V1 item of section 9 is done with evidence, or carries an owner decision.

## 2. What exists

| Part | Today | Gap for the goal |
|---|---|---|
| Agent records (09 2.1, 03 `agents`) | `agent.create` is an owner action with step-up; `adapter_kind` is free text; `operator_config` is a closed schema without secrets | no client calls `createAgent`; no place for instructions, skills or MCP per agent |
| Channel prompt and skills (03 `channels.prompt`, `channel_skill_bindings`) | belong to the channel; the adapter reads them with `agent.config.read` | the runner ignores the skills; nothing edits them in a client |
| Claude Code runner (09 18) | one process per agent token; tool class, model and default folder for the whole runner; `claude -p` with stream JSON | serves exactly one agent; passes no skills and no MCP configuration |
| Search assistant (09 10.6, 18.17) | a short-lived event to one agent, history kept by the client, one model call without tools, a JSON answer | answers search questions only |
| "New project" (01 6.9) | composes existing actions into one dialog with visible steps | no interview; offers only agents with `adapter_kind` `code_runner` (R-AG-39) |
| Templates | none | everything |
| Deny list (ADR 0042) | private list from outside the repository plus public generic rules, in `make check`, the pull request CI and on `main` | no gate that proves a template generic without the private list |

## 3. Runtime: hosted agents in the Claude Code runner

**D-A1-1 A built-in agent is a normal agent principal that a Claude Code runner hosts.** It has
its own handle, avatar, grants, sessions and costs like every agent (09 2.1); its turns run in a
runner that the owner chose when creating it. Reason: everything that makes an agent useful in
Bothy (operator rule, live box, sessions per thread, costs, billing invariants) already works for
agent principals; a second kind of agent would duplicate it. The runner is the only Claude runtime
the product ships (09 18), and it already speaks the adapter contract.

**D-A1-2 The runner keeps the CLI, not the Agent SDK.** Turns stay `claude -p` with stream JSON
(09 18.2). Reason: the process contract, the live box mapping, follow-ups, stop, costs, the
billing mode proof and the fixtures exist for the CLI and the pinned version; the Agent SDK would
be a second runtime with the same capabilities and none of the tests. The pinned CLI already has
the flags this stage needs (`--mcp-config`, `--strict-mcp-config`, `--setting-sources`,
`--settings`, `--tools`, `--plugin-dir`, `--max-budget-usd`), read from `claude --help` of the
pinned version on 2026-11-05 and 2026-11-06; their behaviour in a headless turn is probed by A1-05 before it builds on them.

**D-A1-3 One runner process hosts many agents, switched on by a new key.** A new runner key
`runner.hosted_agents` (default off) lets the runner hold one connection per hosted agent next to
its own. Each hosted agent has its own token, cursor, channel cache, heartbeat and session state
under `<runner.state_dir>/hosted/<agent_id>/`; slots (`runner.max_parallel_runs`) are shared,
because they protect the host. Without the key the runner is the same program as today. Reason:
one process per agent would mean one service unit per agent on the host, which a person who
created an agent in an app cannot set up; a supervisor would be a second program to install and
update.

**D-A1-4 The server gives the token to the hosting runner, never to a person.** The agent record
gets the additive column `hosted_by` (the runner agent that hosts it, nullable). A runner with
`runner.hosted_agents` reports the capability `hosts_agents` in its heartbeat (09 2.2); only such a
runner can be chosen as host. The hosting runner fetches a token for each agent it hosts through a
new adapter operation, bound to the runner's own DPoP key, shown to nobody and replaced on
`agent.disable` or a host change. Reason: the server does not know the runner's host and must
never write files there; a token shown once in an app would have to be copied to the runner
host by hand, which defeats "create in the app". Keeping the token on the runner's authenticated,
key-bound connection adds no new secret path for people.

**D-A1-5 A new adapter kind `claude_agent`.** Hosted agents carry `adapter_kind = claude_agent`.
Reason: `code_runner` is the kind that may serve a bound project forum (R-AG-39) and that "New
project" offers; a hosted agent works in its own folder, not in a project, so it must not appear
there. A later ruling can allow a hosted agent to serve a project; this stage does not.

**D-A1-6 Where a hosted agent answers.** In any text channel or forum where it is the operator or
a guest with `write` (09 3), like every agent. The creation flow offers to create a channel for it
in a space the owner picks, with the agent as operator, so "chat with it right away" needs no
second dialog. Direct messages stay "later" (01).

## 4. Where the definition lives

**D-A1-7 The server stores a versioned agent definition; the runner turns it into files.** A new
table `agent_definitions` (one current version per agent, older versions kept for audit and undo)
holds:

| Part | Content | Limits |
|---|---|---|
| Instructions | the text that becomes `CLAUDE.md` in the agent folder | size limit as a configuration value; an `@` import that leaves the agent folder (absolute, `~` or `..`) is refused (D-A1-18) |
| Skills | name, description and a Markdown body per skill; optional extra Markdown files per skill | count and size limits as configuration values; the runner writes the `SKILL.md` frontmatter itself from name and description only; a body with its own frontmatter or with inline shell is refused (D-A1-18); text only, no binaries, no executables |
| MCP servers | name, transport (`http` with an `https` URL, or `stdio` with a command from an allow list of public package runners and a package pinned as `name@version`), arguments, and environment entries that are either plain non-secret values or secret references | never a secret value; a reference names a key in the runner's secret provider; an unpinned `stdio` package is refused; a `stdio` server makes the agent `host` (D-A1-18) |
| Settings | model, tool class wish, allowed tools for `scoped`, cost cap per run | validated against the tool class rules of 07; the allowed tools only narrow `runner.scoped_allowed_tools` (D-A1-18) |
| Origin | template slug and version, or "interview", or "manual" | display only |

Nothing else: the schema is closed. A definition has no field for settings, hooks, permission
rules, slash commands, subagents, output styles, workflows, plugin manifest entries, LSP servers,
monitors or MCP configuration beyond the row above, and the runner writes every control file of the
agent folder itself (D-A1-8). The full rule, with the surfaces of the pinned CLI it closes, is
D-A1-18.

Reason: the definition is created and edited in the apps, so it must live where the apps write,
which is the server. The server already keeps prompts and skills of channels the same way
(`channels.prompt`, `channel_skill_bindings`), and `operator_config` stays closed and small as 09
2.2 requires. Secrets stay with the runtime (I5): the server stores only references.

**D-A1-8 The runner materializes the definition into an agent folder.** On start and on
`agent.config_changed` the runner fetches the definition (new adapter read, like
`agent.config.read`) and writes the agent folder `<runner.hosted_agents_root>/<agent_id>/` with
`CLAUDE.md` and `.claude/settings.local.json`, and next to it, outside the agent folder, the control
folder `<runner.hosted_agents_root>/<agent_id>.control/` with the runner's settings file, the
skills plugin and the MCP configuration file, whose secret references are resolved at turn start
into the turn's environment only. Every control file is written by the runner from the definition
under the closed schema of D-A1-18; none is copied from the definition as a file. Before every
turn the runner rebuilds the control files and removes foreign ones (D-A1-18). The agent folder is
the working folder of the agent's turns. The turn gets, in addition to 09 18.2:
`--settings <runner settings file>`, `--plugin-dir <skills plugin>`,
`--mcp-config <file> --strict-mcp-config`, `--setting-sources project,local`, under `scoped`
`--tools` with the intersected tool list (D-A1-18), and `--max-budget-usd <cap>`. Reason: the CLI
reads `CLAUDE.md` from the working folder already; a plugin folder and an explicit MCP file are the
documented ways to hand skills and servers to one session; strict MCP and restricted setting
sources keep the host user's own settings, hooks and servers out of a hosted agent's turn, which
matters in billing mode `personal`, where the turn runs as the host user; control files outside
the working folder are out of reach of the turn's file tools. Whether the user-level `CLAUDE.md` is also left out with these flags is probed in
A1-05; if it is not, A1-05 records the gap in the runner README and in the preview text of the
creation flow instead of claiming isolation.

**D-A1-9 Channel skills only behind their own key.** A new key `runner.channel_skills_dir`
(default unset) turns channel skills on. Only then does the runner add skills of
`channel_skill_bindings` to a turn, for its own agent and for hosted agents. Resolution: a
`skill_ref` of the form `claude-code:<name>` resolves to `<runner.channel_skills_dir>/<name>/`
(a folder with a `SKILL.md`, handed to the turn as one more `--plugin-dir`); `<name>` must match
`[a-z0-9-]{1,64}`, so a reference never leaves that folder. Every other reference belongs to
another adapter (03 says the server does not interpret `skill_ref`; the Hermes adapter stores
Hermes skill names there) and is skipped with a debug log only, no live box text. A
`claude-code:` reference without a matching folder is logged and named in the live box, not
fatal. Without the key the runner reads no bindings and its command line, heartbeat and live box
stay byte-equal to today (D-A1-22, D-A1-23); the byte-equal fixtures of A1-05 include a channel
with bindings of both forms and no key. Reason: the field exists and the runner ignores it today;
turning it on for the runner's own agent without a key would change a running installation (a
channel served by this runner may carry references meant for the other adapter); the interview
may suggest channel skills, so they have to work once the owner sets the key.

## 5. Interview

**D-A1-10 The interview follows the search assistant pattern.** A new owner operation sends the
interview so far (the person's answers and the assistant's earlier questions, kept by the client)
to the agent named by the new server key `agents.builder_agent_id` as a short-lived event. That
agent, a Claude Code runner with the new key `runner.agent_builder` on, answers with one model call
without tools (class `chat`, structured JSON output) either the next question or a finished draft.
Nothing is stored on the server until the owner confirms; the interview's cost estimate is shown
like the search assistant's. Reason: the pattern is built, tested and keeps a half-finished
interview out of the database; a normal thread run would leave a thread per attempt and would need
components, which the runner does not build. Without a builder agent, or when it is offline or
too slow, the flow offers templates and the manual form; nothing breaks.

**D-A1-11 The questions.** The builder asks only what the draft needs, one or two questions per
turn, and stops when it can fill every field:

1. What should the agent do, for whom, and what should it never do?
2. Where will people talk to it (a new channel, an existing one, which space)?
3. Which language and tone?
4. Does it need to read or change files, run commands, or only talk? (sets the tool class wish)
5. Which outside systems should it reach? (suggests only MCP servers that the templates of
   section 6 declare in their `template.yaml`, the template catalogue of D-A1-13, names the
   secrets the owner will have to add on the runner host, by name only, and says that a `stdio`
   server makes the agent `host`, D-A1-18)
6. Which recurring tasks deserve a skill?
7. Which model, and how much may one run cost at most?

**D-A1-12 The result is a draft that the owner sees and edits before anything is created.** The
draft has the same shape as a definition (section 4) plus name, handle, description and the
channel choice. The client shows a preview with every part: instructions, each skill, each MCP
server with the secret names it needs, tool class, billing mode, model and cost cap. The server
validates the draft again on create with the same rules as a manual definition and never trusts
model output: a draft field outside the schema, a command outside the allow list, an unpinned
`stdio` package, a secret value in place of a reference, a skill body with frontmatter or inline
shell, or an `@` import that leaves the agent folder is refused with a field error (the closed
schema of D-A1-18). The builder's structured output schema itself has no field for settings,
hooks, permission rules, commands, subagents or plugin parts, so a crafted answer cannot carry
them; the server check holds even if it did.
Creating composes existing and new actions in the order of 01 6.9: `agent.create` (step-up), the
definition, the hosting assignment, then the channel and operator when asked for, each step
visible with its result.

**D-A1-13 The builder writes no files and holds no tools.** Reason: the builder reads only the
owner's own answers and the template catalogue; it has no need for tools, and a tool-free call
cannot be turned against the host by a crafted answer.

## 6. Templates

**D-A1-14 Templates are data written for the repository, never exported from an installation.**
One folder per template under `templates/agents/<slug>/` with `template.yaml` (name, description,
tool class wish, model hint, required secret names, MCP servers, skills list, version),
`CLAUDE.md`, `skills/<name>/SKILL.md` (frontmatter `name` and `description` only, D-A1-18) and
nothing else. The server ships them in its image
(copied by a `go generate` step, held equal by `make codegen-check`, 26 7.2) and lists them through a new read
operation. An installation may add private templates from a folder outside the repository, named
by the new server key `agents.templates_dir` (default empty), the same way `.env` and
`compose.override.yaml` stay outside. Reason: copying a real agent and scrubbing it is exactly how
private names leak; writing generic templates from scratch keeps every template inside the review
and the gates. Topics are generic (for example a writing helper, a code reviewer working in a
folder, a research summarizer, a meeting notes helper); none mirrors an agent of a real
installation.

**D-A1-15 A template gate that works without the private list.** A new check in `make check` and
the pull request CI validates every template against its schema and fails on: a host, URL or
e-mail outside the example domains, an absolute path, an IP address, a person's name outside the
example persons (Alex, Sam, Robin), a secret-looking value, an MCP command outside the allow list,
an unpinned `stdio` package, a `SKILL.md` frontmatter key other than `name` and `description`,
inline shell in any template text, an `@` import that leaves the template folder, any file other
than `template.yaml`, `CLAUDE.md` and `skills/<name>/SKILL.md` (so no `.claude/`, `.mcp.json`,
`.lsp.json`, plugin manifest, hooks, commands, agents or output styles; the closed schema of
D-A1-18), and every finding of the generic deny rules (`--allow-missing` run
over `templates/`). The private deny list then runs over templates like over every other file, in
`make denylist`, the pull request job and `denylist-main`. Reason: forks and outside contributors have no private list; the
template gate must hold for them on its own, and the private list adds the instance-specific terms.

**D-A1-16 The private list is extended before the first template lands.** Terms that the
maintainers know to be private but that the list does not hold yet are added to the private list
and its CI secret by the owner before A1-06 merges. Reason: a scan only proves what the list
contains (ADR 0042).

## 7. Permissions and costs

**D-A1-17 Only owners create and edit built-in agents, with step-up.** `agent.create` and
`agent.update` stay owner actions with step-up (05); writing a definition is part of
`agent.update`; the interview operation is owner-only and rate limited like the search assistant.
Reason: an agent with tools runs on a host with the host user's rights; that is an owner decision
today and stays one.

**D-A1-18 Tool class `chat` by default; `host` only under SR2.** A new agent gets `chat` unless the
owner picks more in the preview. `scoped` needs the explicit tool list of the definition (MCP tools
as `mcp__<server>__<tool>` entries); `host` follows SR2 (07) unchanged. The tool list of the
definition is always intersected with `runner.scoped_allowed_tools` of the hosting runner, in the
order of the runner key, the same rule as `allowed_tools` of a project binding (09 18.1): the
definition only narrows (nothing else in it grants tools, see "A definition is data" below), and
the host operator keeps the ceiling of what `scoped` may run on the host (09 18.2). MCP servers are never loaded under `chat`; there the runner keeps
`--disallowedTools "mcp__*"`, and the preview says so. Under `scoped` only `http` MCP servers are
loaded. A `stdio` MCP server runs a local process with the rights of the host user, so it makes the
effective tool class `host`: the server derives the class from the definition, refuses to store a
definition with a `stdio` server for an agent whose `tool_class` is below `host`, and offers in
the preview to set `host` with `agent.tool_class.set` (owner, step-up, audit, 07 2.11); from then
on SR2 limits triggering to owners. The runner starts no `stdio` server in a turn whose token
carries a class below `host` and refuses that turn with `tool_class_mismatch` (09 4.5, 18.2), so a
definition changed behind the server's back cannot widen the class either. Every `stdio` package is
pinned as `name@version`; a bare runner command or an unpinned package is refused. Reason: 09 18.2
and 07 2.11 already define what each class may do (`scoped`: named tools with their own, minimal
credentials, one named, limited effect); the definition must not widen them, and an arbitrary
package started as the host user is the `host` case. The runner has no approval cards, so `scoped`
stays an allow list without interactive approvals in this stage, and the preview says that too.

**A definition is data, not configuration of the CLI.** Below `host` this is what keeps the class
ceiling: any content of a definition, a template or an interview draft that the CLI interprets as
code or as a permission grant would let a `scoped` or `chat` agent run host code or widen its tools
beyond `runner.scoped_allowed_tools` without SR2. In this stage the rule holds for every class, so
there is one schema; `host` adds only `stdio` MCP servers, and a later ruling may open more for
`host` alone. Surfaces of the pinned CLI 2.1.288, found in the strings of the binary and in its
`claude --help` on 2026-11-06:

| Surface | What the CLI does with it | Closed by |
|---|---|---|
| `SKILL.md` frontmatter | `hooks` ("Hooks registered while this skill is active. Same shape as settings.json `hooks`"), `allowed-tools` ("Tools available to the model while this file is active"), `model`, `effort`, `shell`, `context: fork` with `agent`, `paths`, `disable-model-invocation`, `user-invocable`, and internal keys such as `mcpServers`, `agents`, `settings`, `lspServers`, `monitors` | schema: the definition stores name, description and body; the runner writes the frontmatter with `name` and `description` only |
| Inline shell in skill and command text | `` !`cmd` `` and a fenced block opened with `` ```! `` run as shell when the skill loads | schema: refused in every text; runner setting `disableSkillShellExecution` ("Disable inline shell execution in skills and custom slash commands from user, project, or plugin sources") as a second layer |
| Settings files the turn reads (`--setting-sources project,local`) | `hooks`; `permissions` with allow rules, `defaultMode` and `additionalDirectories`; `env`; the command helpers `apiKeyHelper`, `otelHeadersHelper`, `awsAuthRefresh`, `awsCredentialExport`, `gcpAuthRefresh`, `statusLine`, `fileSuggestion`; `enableAllProjectMcpServers`, `enabledMcpjsonServers`; `enabledPlugins`, `extraKnownMarketplaces` | no settings field in the schema; the runner writes the only settings (below) and removes foreign files |
| Discovered folder content | `.claude/skills`, `.claude/commands` (same frontmatter and inline shell as skills), `.claude/agents` (subagent frontmatter with tools, permission mode, `mcpServers` and `hooks`), `.claude/workflows`, `.claude/rules`, `.mcp.json`, `CLAUDE.local.md` | not in the schema; removed on rebuild; `.mcp.json` is also ignored under `--strict-mcp-config` |
| Plugin folder (`--plugin-dir`) | `.claude-plugin/plugin.json` and the default component paths `hooks/hooks.json`, `.mcp.json`, `.lsp.json` (starts language servers), `monitors/monitors.json` ("unsandboxed, same trust tier as hooks"), `commands/`, `agents/`, `output-styles/` | the runner writes `plugin.json` with name and version only and `skills/` only; the plugin lives in the control folder outside the working folder |
| `@` imports in `CLAUDE.md` | read files into the prompt; imports outside the project wait for an approval flag of the host user (`hasClaudeMdExternalIncludesApproved`) | schema: an import that leaves the agent folder is refused |

Settings the runner writes, into its settings file passed with `--settings` and with the same
content into the folder's `.claude/settings.local.json`: `disableSkillShellExecution: true`,
`disableAllHooks: true` ("Disable all hooks and statusLine execution: the hooks defined in settings
files and by installed plugins"), `disableWorkflows: true`, and deny rules for edits of the
control files in the working folder (`CLAUDE.md`, `CLAUDE.local.md`, `.claude/`, `.mcp.json`,
`.lsp.json`, `.claude-plugin/`; rule syntax to verify in A1-05). It writes no allow rules (the tool
list stays on the command line, intersected as above), no `env`, no command helper, no MCP switch
and no plugin or marketplace key. The binary marks `disableAllHooks`, `disableSkillShellExecution`
and `disableWorkflows` as restrictive in its settings merge, so a lower source cannot turn them
back off; A1-05 verifies that. `allowManagedPermissionRulesOnly` is not usable: it is read from
managed settings only and would also drop the runner's own `--allowedTools`. So `allowed-tools` in
frontmatter is closed by the schema, and under `scoped` the runner also passes `--tools` with the
intersected built-in tool list, so no frontmatter can name a tool the turn does not have (to verify
in A1-05).

Rebuild: before every turn the runner checks the control folder and the working folder against the
definition, rewrites every control file it owns, and removes every file of the surfaces above that
it did not write, anywhere in the working folder; the agent's other files stay. A definition that
fails the schema (written behind the server's back) is refused with `tool_class_mismatch`, like the
`stdio` case, because it claims more than its class.

Each of the four writers enforces the rule with its own test: the server on every definition write
(A1-02, one refusal test per surface), the template gate (D-A1-15, A1-06, a fixture template per
surface), the interview (D-A1-12, A1-07, a builder answer carrying each surface is refused), and the
runner (A1-05: materializer and rebuild tests, the settings file content, and a probe against the
pinned CLI with a planted skill carrying `hooks`, `allowed-tools: Bash` and both inline shell forms,
expecting no hook run, no `Bash` and the placeholder "[shell command execution disabled by policy]").
To verify in A1-05, because the strings do not settle them: whether `disableAllHooks` outside
managed settings also drops hooks from skill frontmatter, whether a plugin's `bin/` folder reaches
the turn's `PATH`, whether `--tools` holds against a skill's `allowed-tools`, whether the CLI
reloads a settings file changed during a turn, and whether `--setting-sources` accepts an empty
list, which would let the turn read no folder settings at all.

**D-A1-19 Billing follows 10 unchanged.** A new agent is `organization` by default (I1). `personal`
is offered only when the creating owner is the credential holder and the only owner, and the
hosting runner reports `personal` in `billing_modes_supported`; I1 to I5 apply as they are.
Reason: the invariants are the product's answer to the vendor's terms; built-in agents get no
exception.

**D-A1-20 Cost cap per run, no monthly budget in this stage.** The definition carries a cost cap
per run that the runner passes as `--max-budget-usd`; a run stopped by it ends with a visible
reason. A monthly budget per agent is left for later. Reason: the CLI enforces a per-run cap
reliably; a monthly budget needs a server-side check at acceptance against the cost ledger and a
policy for running sessions, which is a feature of its own; costs per agent are already in the
statistics (01, 25).

## 8. Protecting a running installation

**D-A1-21 Additive only.** Every new column is nullable or has a default that means "as before";
every new table, operation, event, capability and configuration key is new; no existing field
changes meaning. Migrations follow expand and contract with one version back (R-OP-12), and this
stage writes no contract step. Reason: an installation that updates the server image must keep
running with its old runner, old adapters and old apps, and must be able to go back one version.

**D-A1-22 Off until asked for.** The runner hosts agents only with `runner.hosted_agents`, answers
the interview only with `runner.agent_builder`, reads channel skills only with
`runner.channel_skills_dir` (D-A1-9), and the server sends interviews only with
`agents.builder_agent_id`; all default off. Clients show "New agent" only to owners and only when
at least one runner reports `hosts_agents`. Reason: an update must not change what a running
installation does; turning the feature on is one key on one runner.

**D-A1-23 Byte-equal fixtures prove it.** The runner tests keep today's command line, heartbeat
and served channel fixtures for a configuration without the new keys, unchanged; the adapter for
the other runtime gets no change in this stage; the apps tolerate the new fields and operations
missing on an older server (they hide the entry). Reason: "backwards compatible" is a claim only
with a test against today's artifacts.

**D-A1-24 Rollback.** Turning `runner.hosted_agents` off takes hosted agents offline (their runs
end like with any offline agent) and leaves the runner's own agent as before; their definitions
stay on the server. Going back one server version leaves the new tables unread. Reason: an owner
can undo the feature without losing data or touching the database.

## 9. One `main`, no public branch

**D-A1-25 There is one `main`; private data is configuration outside the repository.** No
separate public branch, fork or export. Everything instance-specific comes in through files the
repository never tracks: `.env`, `compose.override.yaml`, the runner configuration, the secret
provider, private templates (`agents.templates_dir`) and the runtime profiles of other agent
runtimes. Reason: a second branch would drift and would need its own scans, merges and releases;
[ADR 0002](../adr/0002-two-repos-public-product-private-ops-repo.md) and
[ADR 0042](../adr/0042-deny-list-scanner-with-the-list-from-the-ops-repo.md) already separate
product and operations. `compose.override.yaml` is in `.gitignore` since this note, so a normal
clone cannot commit it by accident.

## 10. V1 completion

V1 is the publication milestone ([PLAN 15](../PLAN.md), [public-switch.md](../quality/public-switch.md)).
The repository is public already; what is open, as checked on 2026-11-05:

| Item | What is open | Who |
|---|---|---|
| Deny list on `main` | `denylist-main` fails on the latest squash commits because author and sign-off use different identities | owner decision on the identity (section 12, item 1), then a session updates the allow list and the landing step |
| `ci` on `main` | the iOS app job fails on `main` | session |
| V1-01 Enable list | the section `later` of the operation list holds operations of a built milestone; the gate checks only the form of the entries | session: build them or rule them out with a reason, and make the gate check the rule |
| V1-02 Penetration test | no engagement yet | owner (cost), section 12 item 3 |
| V1-03 Placeholders | `OWNER` in the Go module path, `CODEOWNERS`, documentation and install examples; the acceptance criterion "no `OWNER` anywhere" cannot hold, because SQL, Windows and SDK identifiers use the word | owner names the target (section 12 item 2); then one mechanical session ticket with a narrower criterion, run when few branches are open |
| V1-04 Checklist part A | the evidence of the steps before the first push is not filed | session: run the scans again on a fresh clone and file the evidence |
| V1-05 Parts B to E | release environments, ruleset (pull request required, signed commits, pinned actions), dependency bot app, fork probe, DCO probe, first signed release and the public portability test | owner for settings, tags and public packages; session for the probes and the release preparation |
| V1-06 Name | the name stays; the trademark check is open | owner (cost) |
| Install command | `release_prep` does not rewrite every page that shows the install command | session |
| External test from zero | no recorded run of "a stranger, a fresh host, only the README" | session prepares the script and a fresh-host run; a third person runs it (owner arranges) |
| Showcase | images and video predate recent features, and built-in agents will be the first thing a visitor should see | session (Android, console, Grafana); iOS and macOS on a Mac when it is free |
| README "How it is used today" | describes the maintainer's installation in general terms | session: rewrite it around what a visitor can try, keep it generic |

These rows become tickets V1-07 to V1-12 in section 11; V1-01 to V1-06 keep their plan rows.

## 11. Ticket cut

Stage **A1 Built-in agents**. Every ticket is one pull request from `main`; A1-01 wrote the
normative text, the ADR, the plan section and the briefs of the others, so the later tickets run
from briefs in `docs/briefs/` like every milestone.

The ticket table, the waves and the launch arguments live in [PLAN](../PLAN.md) section 21 (A1-01
to A1-10) and section 15 (V1-07 to V1-12, which run from their plan rows without a brief, P-28);
the briefs are in the [brief index](../briefs/README.md). Waves: A1-02 and A1-03; then A1-04,
A1-06 and A1-07; then A1-05 and A1-08; then A1-09; then A1-10. V1-08 and V1-09 can run at any
time; V1-07 after the owner's ruling. The launch arguments leave out A1-01 because it is merged
before every other ticket starts.

## 12. Owner decisions

Each with the recommendation this design builds on until the owner rules otherwise.

1. **Commit identity.** Goal 3 conflicts with the earlier ruling on the maintainer's commit
   identity. Recommendation: from now on use the hosting service's no-reply
   address for author and sign-off, add it to the allow list of authors (operations repository
   and CI secret), and leave the existing history as it is; rewriting a public `main` is
   irreversible and breaks every clone.
2. **Target of `OWNER` (V1-03).** Recommendation: a neutral organization, before V1-03, so no
   personal account name ends up in the Go module path.
3. **Penetration test (V1-02).** Cost. Recommendation: commission it before people outside the
   project are invited to install Bothy.
4. **Trademark check (V1-06).** Cost and legal. Recommendation: run it before the first release
   with signed images.
5. **Repository settings and first release.** Release environments, ruleset, dependency bot,
   signed tags, making the image packages public (not reversible). Recommendation: do them in one
   session after V1-07 and V1-08 are green, with a probe pull request before turning on signed
   commits.
6. **Test from zero by a third person.** Recommendation: after A1-10 and V1-09, one person who has
   never seen the project, a fresh host, only the README.

## 13. Backlog (minors from the design review)

Collected, not polished; the next run in this area (A1-01) takes them. Resolved by A1-01: each
item names where, and [decisions.d/A1-01.md](../decisions.d/A1-01.md) records the ruling.

1. V1-07 to V1-12 have no plan rows and no briefs yet: A1-01 adds their rows to `docs/PLAN.md`
   and writes their briefs (or records that they run in the operations runner as tasks without a
   brief). Resolved: PLAN section 15 rows V1-07 to V1-12; they run from their rows without a brief
   (P-28).
2. Owner decision 1 (section 12) recommends the no-reply address, while the operations runner
   currently sets the maintainer's commit identity as author; when the owner rules, the operations
   runner changes in the same step. Resolved: open as OQ-25 with V1-07, which names that step.
3. The start arguments of section 11 leave out A1-01 although the table lists it as a dependency;
   that is correct because A1-01 is merged first, and A1-01 states it in one sentence. Resolved:
   PLAN section 21 "Dependencies" and section 11 above.
4. Tokens of hosted agents sit next to each other in `<runner.state_dir>/hosted/` (D-A1-3, lines
   84 to 85): a `host` turn of one hosted agent can read the others, and concurrent turns as the
   same user may expose MCP secrets in the process environment; A1-01 decides tokens in memory or
   in the secret provider, `runner.hosted_agents_root` never under `runner.state_dir` (in the
   way 09 18.10 keeps attachment roots apart), and a threat model entry. Resolved: tokens in memory
   only (26 3.3), no token under `runner.state_dir`, `runner.hosted_agents_root` refused inside,
   equal to or above `runner.state_dir` (26 5.1), MCP secrets only in a per-turn file (26 5.2),
   T-BA-01 and T-BA-02.
5. "replaced on `agent.disable`" (D-A1-4, line 96) should say revoked; A1-01 adds that blocking or
   revoking the host runner revokes the tokens of all its hosted agents, that the token operation
   goes through `authorize` with `hosted_by` equal to the caller, and its audit. Resolved: 26 3.3
   and 07 2.13.
6. Error paths of creation (D-A1-12, lines 211 to 213): what stays when `agent.create` succeeds
   but the definition or the host assignment fails, and whether the interview operation (it costs
   money) is audited and logged; A1-01 states both with a test each. Resolved: 26 8.3 with BA-T16,
   26 6.1 item 5 with BA-T15.
7. Agent folder under `--setting-sources project,local` (D-A1-8, line 147): hooks a `scoped` agent
   writes into its own `.claude/` and the removal of foreign files are now part of D-A1-18 (deny
   rules, rebuild before every turn, tests); what stays for the A1-05 probe is whether `CLAUDE.md`
   files of the folders above the agent folder are loaded (compare T-TM-13). Resolved: the runner
   refuses a `runner.hosted_agents_root` with a `CLAUDE.md` above it (26 5.1), and the probe with its
   fallback is in 26 5.7.
8. "the credential holder" in D-A1-19 (line 333) is ambiguous: A1-01 writes the
   `credential_holder` of the hosting runner, which equals that of the hosted agent. Resolved: 10
   10.3 and R-BA-12.
9. Commit identity of the commits of this note (review minor 7): no finding of the diff; it stays
   with owner decision 1 (section 12). Resolved: OQ-25.
10. `agent.tool_class.set` can set an agent whose definition has a `stdio` server below `host`
    again (D-A1-18, lines 269 to 275); the runner catches it with `tool_class_mismatch`, so there
    is no harm, but the preview or the action should refuse the downgrade or say what it breaks;
    A1-01 decides which, with a test (delta review of round 2, minor 1). Resolved: the action
    refuses the downgrade (26 4.4 item 5, R-BA-06, BA-T03).
11. Encoding of name, description and extra files (red team round 3, finding 1). Resolved: the
    runner writes the frontmatter with a YAML serializer, the schema keeps `description` on one
    line and skill names in kebab case, extra files are flat `*.md` names (26 4.2, 5.2, BA-T11).
12. Surface table incomplete (round 3, finding 2): MCP `headersHelper`, `proxyAuthHelper`, the
    hooks module of a plugin, developer modifications. Resolved: 26 5.4 and 5.5 name them.
13. `--restricted` of the pinned CLI (round 3, finding 3). Resolved: used below `host` with a
    fallback (26 5.3, 5.7, R-BA-07).
14. `--tools` under `scoped` drops read-only shell and skills (round 3, finding 4). Resolved: the
    class baseline plus `Skill` stays in `--tools` (26 5.3, R-BA-07).
