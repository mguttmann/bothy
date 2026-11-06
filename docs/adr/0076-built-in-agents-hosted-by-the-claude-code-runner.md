---
status: accepted
date: 2026-11-06
decision-makers: Owner, planning lead
consulted: Red team of the design note
informed: all teams
---

# Built-in agents hosted by the Claude Code runner

Applies [ADR 0022](0022-agents-and-plugins-outside-the-server-process.md) (agents outside the
server process) and [ADR 0023](0023-signed-invocation-token-and-check-in-the-adapter.md) (signed
invocation token, check in the adapter) to agents that a person creates inside Bothy. Normative:
[26-builtin-agents.md](../spec/26-builtin-agents.md), [07-permissions.md](../spec/07-permissions.md)
2.13, [09-agents.md](../spec/09-agents.md) 18.18, [10-backend-policy.md](../spec/10-backend-policy.md)
10.3; design note: [builtin-agents.md](../design/builtin-agents.md); rulings:
[decisions.d/A1-01.md](../decisions.d/A1-01.md).

## Context and Problem Statement

Today every agent of an installation runs in a runtime that its operator installs and configures
on a host: the Claude Code runner for development forums, an adapter for another agent runtime for
the rest. A person who tries Bothy without such a runtime has nobody to talk to. The owner asked on
2026-11-05 for an agent that lives in Bothy: created in the apps through an interview or from a
template, running on local Claude through the existing runner, with its own instructions, skills
and MCP servers, without breaking a running installation. Where does such an agent run, where does
its definition live, how does it get a token, and how is a definition written by a person or a
model kept from widening what its tool class allows on the host?

## Decision Drivers

- Only Claude; no second runtime and no model credentials in the server (B10, I5).
- An agent created in an app must work without a person setting up a service, copying a token or
  editing files on the host.
- The tool class and SR2 stay the only limit of what an agent does on the host: no content of a
  definition, a template or a model draft may run host code or grant a tool below `host`.
- An installation that sets no new key behaves byte for byte as before; everything is additive and
  reversible.
- Everything instance-specific stays outside the repository; templates are generic.

## Considered Options

1. The Claude Code runner hosts built-in agents: one runner process holds a connection per hosted
   agent, fetches each agent's token over its own key-bound connection, and runs `claude -p` turns
   from a versioned definition that the server stores as closed data and the runner turns into
   files it writes itself.
2. A second runtime on the Claude Agent SDK, one process for all built-in agents.
3. One runner process per built-in agent, each with its own service unit and token file.
4. Definitions as files on the runner host (a folder per agent that the owner edits through the
   terminal), the server knowing only the agent record.
5. The server runs the model calls of built-in agents itself.

## Decision Outcome

Chosen: option 1. A built-in agent is an agent principal of `adapter_kind = 'claude_agent'` with
`agents.hosted_by` naming a Claude Code runner that reports `hosts_agents`. With
`runner.hosted_agents` the runner holds one connection, cursor, cache, heartbeat and state folder
per hosted agent and shares its slots. The server issues each hosted agent's token only to its
hosting runner, bound to the runner's DPoP key; the runner keeps it in memory. The definition
(instructions, skills, MCP servers by secret reference, settings) is a closed schema in
`agent_definitions`, written by the owner with `agent.update`; the runner writes `CLAUDE.md`, the
skills plugin with a frontmatter of `name` and `description` only, the MCP file and its own
restrictive settings, rebuilds them before every turn and removes every foreign control file.
`stdio` MCP servers need `host`. The interview is a short-lived request to a builder agent on the
pattern of the search assistant ([ADR 0075](0075-search-assistant-through-an-agent-over-the-full-text-search.md)).

### Consequences

- Good, because a built-in agent gets operator rule, live box, sessions, costs and the billing
  invariants without new code paths: it is an agent like every other.
- Good, because the runner, its process contract, its fixtures and the pinned CLI version stay the
  one Claude runtime; hosting is a key on a runner that already runs.
- Good, because no token or secret value passes a person or the server's storage, and the server
  never writes host files.
- Good, because the closed schema is checked by four writers with their own tests (server,
  template gate, interview, runner), so no single missed check opens the class.
- Bad, because one runner process carries all its hosted agents: a crash of the runner takes all of
  them offline at once, and they share one host user and one set of slots.
- Bad, because `scoped` stays an allow list without approval cards, and a definition cannot use
  anything of the CLI beyond the schema (hooks, subagents, own settings) below `host`; a later
  ruling may open more for `host` alone.
- Bad, because some behaviour of the pinned CLI in a headless turn is known only from its strings
  and help text; A1-05 probes each point and falls back to a narrower form where a probe fails
  (26 5.7).
- Neutral, because a built-in agent cannot serve a project forum in this stage (R-AG-39).

### Confirmation

- Server tests: one refusal per surface of 26 5.5 on every definition write; `stdio` below `host`
  refused; hosted tokens only for the hosting runner and revoked with the agent, a host change, the
  runner and the runner's token (BA-T01 to BA-T08).
- Runner tests: byte-equal fixtures without the new keys; materializer, rebuild and command line
  per class; the probe against the pinned CLI with a planted skill carrying hooks,
  `allowed-tools: Bash` and both inline shell forms; channel skills only in `host` turns
  (BA-T09 to BA-T14, BA-T19, BA-T20, BA-T22).
- Template gate with one failing fixture per surface; interview tests where a builder answer
  carrying each surface is refused (BA-T15, BA-T17).
- Creation flow that sets the class before the definition, also for a `stdio` template (BA-T16).
- End to end on the demo stack with a fake Claude (BA-T21).

## Pros and Cons of the Options

### The Claude Code runner hosts built-in agents

- Good, because of the drivers above.
- Neutral, because new keys, a table, a column, actions and events are needed; all are
  additive.

### A runtime on the Agent SDK

- Good, because the SDK offers programmatic control of sessions and tools.
- Bad, because it would be a second runtime with the same capabilities and none of the tests,
  fixtures and billing evidence the runner has, and a second process contract to keep.

### One runner process per agent

- Good, because agents would be isolated from each other as processes.
- Bad, because every new agent would need a service unit and a token copied to the host, which a
  person creating an agent in an app cannot do; a supervisor would be another program to ship.

### Definitions as files on the host

- Good, because the runtime reads files anyway.
- Bad, because the apps could not create or edit an agent, the server could not validate or
  version a definition, and every file the CLI reads would be open to anything a person writes.

### Model calls in the server

- Good, because there would be no runner to set up.
- Bad, because the server would hold model credentials and call a paid service, against B10, I5
  and ADR 0022.

## More Information

- The stdio rule, the closed schema and the runner settings come from the red team rounds of the
  design note against the strings and help text of the pinned CLI 2.1.288.
- Revisit when the pinned CLI changes a surface of 26 5.5, when built-in agents should serve
  project forums, or when `host` agents should get more of the CLI than `stdio` MCP servers.
