# Built-in agents

Status: design of 2026-11-05 for the stage "A1 Built-in agents" and the rest of V1. Nothing in it
is built yet. This note records the goal and the decisions with their reasons; the normative text
goes into the chapters it names, written by ticket A1-01 (section 11) together with an ADR and the
briefs. Where this note and a chapter disagree after A1-01, the chapter wins.

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
`--plugin-dir`, `--max-budget-usd`), read from `claude --help` of the pinned version on
2026-11-05; their behaviour in a headless turn is probed by A1-05 before it builds on them.

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
| Instructions | the text that becomes `CLAUDE.md` in the agent folder | size limit as a configuration value |
| Skills | name, description and the `SKILL.md` text per skill; optional extra text files per skill | count and size limits as configuration values; text only, no binaries, no executables |
| MCP servers | name, transport (`http` with an `https` URL, or `stdio` with a command from an allow list of public package runners and a package pinned as `name@version`), arguments, and environment entries that are either plain non-secret values or secret references | never a secret value; a reference names a key in the runner's secret provider; an unpinned `stdio` package is refused; a `stdio` server makes the agent `host` (D-A1-18) |
| Settings | model, tool class wish, allowed tools for `scoped`, cost cap per run | validated against the tool class rules of 07; the allowed tools only narrow `runner.scoped_allowed_tools` (D-A1-18) |
| Origin | template slug and version, or "interview", or "manual" | display only |

Reason: the definition is created and edited in the apps, so it must live where the apps write,
which is the server. The server already keeps prompts and skills of channels the same way
(`channels.prompt`, `channel_skill_bindings`), and `operator_config` stays closed and small as 09
2.2 requires. Secrets stay with the runtime (I5): the server stores only references.

**D-A1-8 The runner materializes the definition into an agent folder.** On start and on
`agent.config_changed` the runner fetches the definition (new adapter read, like
`agent.config.read`) and writes `<runner.hosted_agents_root>/<agent_id>/` with `CLAUDE.md`, a local
plugin folder holding the skills, and an MCP configuration file with the secret references
resolved at turn start into the turn's environment only. That folder is the working folder of the
agent's turns. The turn gets, in addition to 09 18.2: `--plugin-dir <skills plugin>`,
`--mcp-config <file> --strict-mcp-config`, `--setting-sources project,local` and
`--max-budget-usd <cap>`. Reason: the CLI reads `CLAUDE.md` from the working folder already; a
plugin folder and an explicit MCP file are the documented ways to hand skills and servers to one
session; strict MCP and restricted setting sources keep the host user's own settings, hooks and
servers out of a hosted agent's turn, which matters in billing mode `personal`, where the turn runs
as the host user. Whether the user-level `CLAUDE.md` is also left out with these flags is probed in
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
`stdio` package or a secret value in place of a reference is refused with a field error.
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
`CLAUDE.md`, `skills/<name>/SKILL.md` and nothing else. The server ships them in its image
(copied by `make codegen`, held equal by `make codegen-check`) and lists them through a new read
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
an unpinned `stdio` package, and every finding of the generic deny rules (`--allow-missing` run
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
definition only narrows, and the host operator keeps the ceiling of what `scoped` may run on the
host (09 18.2). MCP servers are never loaded under `chat`; there the runner keeps
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

Stage **A1 Built-in agents**. Every ticket is one pull request from `main`; A1-01 writes the
normative text, the ADR, the plan section and the briefs of the others, so the later tickets run
from briefs in `docs/briefs/` like every milestone.

| Ticket | Goal | Done when | Depends on |
|---|---|---|---|
| A1-01 | Specification, ADR, plan section and briefs for A1 | chapters 01, 03, 05, 06, 07, 09, 10 and the configuration tables carry sections 3 to 9 as normative text; ADR "Built-in agents hosted by the Claude Code runner"; plan section A1 and briefs A1-02 to A1-10 pass the brief format, rules and dependency checks; `make docs`, `make catalog-check` and `make config-doc-check` green | none |
| A1-02 | Server: agent definitions | table, versioning, owner write inside `agent.update`, adapter read, schema validation with field errors, secret references only, size limits as configuration values, `agent.config_changed` on change, audit; migration additive; tool class derived from the definition (D-A1-18): a `stdio` MCP server needs `tool_class = 'host'` and is refused below it, only `http` servers under `scoped`, `stdio` packages pinned as `name@version`; tests for every refusal | A1-01 |
| A1-03 | Server: hosted agents | `hosted_by`, capability `hosts_agents`, host choice on create, token for the hosting runner bound to its key, replaced on disable and host change, `claude_agent` refused by R-AG-39 paths; tests for every refusal | A1-01 |
| A1-04 | Runner: host many agents | `runner.hosted_agents` off by default; one connection, cursor, cache and state folder per hosted agent; shared slots; today's fixtures byte-equal without the key | A1-03 |
| A1-05 | Runner: run the definition | agent folder from the definition, skills plugin, strict MCP file, setting sources, cost cap, allowed tools of the definition intersected with `runner.scoped_allowed_tools`, no `stdio` MCP server in a turn below `host` (the turn is refused with `tool_class_mismatch`), tests for both, channel skills behind `runner.channel_skills_dir` with the `claude-code:` resolution of D-A1-9; byte-equal fixtures without the key include a channel with bindings of both forms; a probe test against the pinned CLI records what each flag does headless, including whether the user-level `CLAUDE.md` is left out and whether `--mcp-config` in the chosen argument order swallows following arguments | A1-02, A1-04 |
| A1-06 | Templates and their gate | `templates/agents/` with generic templates, schema, the template gate in `make check` and CI, read operation, `agents.templates_dir`; private list extended first (D-A1-16) | A1-02 |
| A1-07 | Interview | owner operation and short-lived event, `agents.builder_agent_id`, `runner.agent_builder`, structured answer, fallback without a builder, cost estimate shown | A1-02 |
| A1-08 | Clients: "New agent" | desktop, phone and console: template or interview, preview and edit, step-up, visible steps, channel with the agent as operator; hidden without a hosting runner | A1-03, A1-06, A1-07 |
| A1-09 | Clients: edit and remove | edit instructions, skills, MCP servers and settings with preview and step-up; disable and delete through the existing actions | A1-08 |
| A1-10 | End to end and documentation | an end-to-end test on the demo stack with a fake Claude: create from a template and from an interview, chat, cost shown, runner key off restores today's behaviour; guide for operators; README section for visitors | A1-05, A1-09 |
| V1-07 | Deny list on `main` green | the identity rule of section 12 item 1 in the allow list and the landing step; `denylist-main` green on the next commit | owner decision |
| V1-08 | `ci` on `main` green | the iOS job passes on `main` | none |
| V1-09 | Install command everywhere | `release_prep` rewrites every page with the install command, held by a test | none |
| V1-10 | Test from zero | a script and a recorded run on a fresh host following only the README; findings fixed | A1-10, V1-09 |
| V1-11 | Showcase current | showcase regenerated with built-in agents; manifest check green | A1-10 |
| V1-12 | Public texts | README, roadmap and site texts describe what a visitor can try; no description of a real installation | A1-10 |

Waves: A1-01 alone; then A1-02 and A1-03; then A1-04, A1-06 and A1-07; then A1-05 and A1-08; then
A1-09; then A1-10. V1-08 and V1-09 can run at any time; V1-07 after the owner's decision.

Launch arguments for the build workflow, once A1-01 has merged and the briefs exist (the brief
file names are fixed by A1-01; these are the planned ones):

```json
{
  "parallel": 3,
  "tickets": [
    {"id": "A1-02", "brief": "docs/briefs/A1-02-agent-definitions.md", "deps": []},
    {"id": "A1-03", "brief": "docs/briefs/A1-03-hosted-agents.md", "deps": []},
    {"id": "A1-04", "brief": "docs/briefs/A1-04-runner-hosting.md", "deps": ["A1-03"]},
    {"id": "A1-05", "brief": "docs/briefs/A1-05-runner-definition.md", "deps": ["A1-02", "A1-04"]},
    {"id": "A1-06", "brief": "docs/briefs/A1-06-agent-templates.md", "deps": ["A1-02"]},
    {"id": "A1-07", "brief": "docs/briefs/A1-07-agent-interview.md", "deps": ["A1-02"]},
    {"id": "A1-08", "brief": "docs/briefs/A1-08-new-agent-clients.md", "deps": ["A1-03", "A1-06", "A1-07"]},
    {"id": "A1-09", "brief": "docs/briefs/A1-09-edit-agent-clients.md", "deps": ["A1-08"]},
    {"id": "A1-10", "brief": "docs/briefs/A1-10-builtin-agents-end-to-end.md", "deps": ["A1-05", "A1-09"]}
  ]
}
```

A1-01 itself runs as a task without a brief, pointing at this note.

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

Collected, not polished; the next run in this area (A1-01) takes them.

1. V1-07 to V1-12 have no plan rows and no briefs yet: A1-01 adds their rows to `docs/PLAN.md`
   and writes their briefs (or records that they run in the operations runner as tasks without a
   brief).
2. Owner decision 1 (section 12) recommends the no-reply address, while the operations runner
   currently sets the maintainer's commit identity as author; when the owner rules, the operations
   runner changes in the same step.
3. The start arguments of section 11 leave out A1-01 although the table lists it as a dependency;
   that is correct because A1-01 is merged first, and A1-01 states it in one sentence.
4. Tokens of hosted agents sit next to each other in `<runner.state_dir>/hosted/` (D-A1-3, lines
   84 to 85): a `host` turn of one hosted agent can read the others, and concurrent turns as the
   same user may expose MCP secrets in the process environment; A1-01 decides tokens in memory or
   in the secret provider, `runner.hosted_agents_root` never under `runner.state_dir` (09 18.10),
   and a threat model entry.
5. "replaced on `agent.disable`" (D-A1-4, line 96) should say revoked; A1-01 adds that blocking or
   revoking the host runner revokes the tokens of all its hosted agents, that the token operation
   goes through `authorize` with `hosted_by` equal to the caller, and its audit.
6. Error paths of creation (D-A1-12, lines 195 to 197): what stays when `agent.create` succeeds
   but the definition or the host assignment fails, and whether the interview operation (it costs
   money) is audited and logged; A1-01 states both with a test each.
7. Agent folder under `--setting-sources project,local` (D-A1-8, line 136): a `scoped` agent with
   write access could write hooks into its own `.claude/settings.json`, and the runner does not yet
   promise to remove foreign files when it rebuilds the folder; the A1-05 probe covers both, and
   `CLAUDE.md` files of the folders above (compare T-TM-13).
8. "the credential holder" in D-A1-19 (line 262) is ambiguous: A1-01 writes the
   `credential_holder` of the hosting runner, which equals that of the hosted agent.
9. Commit identity of the commits of this note (review minor 7): no finding of the diff; it stays
   with owner decision 1 (section 12).
