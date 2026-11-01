# Live topology: the agent tree of a run

Status: design of 2026-11-01 (ticket FEAT-topology-agent-tree). The normative text is in the
chapters it changes ([25-visualization.md](../spec/25-visualization.md) 1, 5.2, 6.1, 15;
[09-agents.md](../spec/09-agents.md) 5.6, 18.4). Rulings:
[decisions.d/FEAT-topology-agent-tree.md](../decisions.d/FEAT-topology-agent-tree.md)
(`FEAT-TAT/1` to `FEAT-TAT/9`).

## 1. Goal

Owner request (2026-11-01): a session in the live topology runs child agents (subagents of the
Agent tool, background shell jobs, workflow runs with several agents), but the topology shows one
flat card per run and none of the children. The topology must show an org chart per run: the run
on top, below it what it started and what those started, each with its label, state (running,
done, failed, cancelled), current tool and runtime, with lines from parent to child, so the owner
sees at a glance where the work split off.

Done means:

1. The live frame `stats.live` carries per run a bounded list of child nodes (`tasks`) with parent,
   kind, label, state, times and current tool; the label only for a receiver who may read the run.
2. The Claude Code runner reports the agents of a workflow as children of the workflow step, with
   label, state and current tool, and no longer puts the foreground tool calls of workflow agents
   at the root of the run.
3. Every client (macOS, Windows, iOS, Android, web console) draws the tree under each run card of
   the live topology: node cards with connecting lines, collapsible branches, finished nodes
   faded; the table form lists the children indented below their run.
4. The run graph (D14, "Graph" in the run header) shows the same workflow agents, because it reads
   the same run steps.

## 2. What the runner already knows (measured 2026-11-01)

Read from the stream recordings the runner keeps per turn (`<state>/sessions/<session>/<turn>.stream.jsonl`)
of the pinned CLI 2.1.288, 50 recorded turns:

| Signal | Stream | Reported before this change |
|---|---|---|
| Subagent of the Agent tool | `system/task_started` with `task_type = local_agent`, `tool_use_id`, `description`; its own tool calls as `assistant` messages with `parent_tool_use_id` | step `subagent` under the step of the Agent call (M2-51); its tools under the same call |
| Background or long shell command | `task_started` with `task_type = local_bash`, `is_backgrounded` | step `background_job` |
| Workflow run | `task_started` with `task_type = local_workflow`, `workflow_name`; `task_progress` with `workflow_progress`: the full list of phases (`workflow_phase`) and agents (`workflow_agent` with `index`, `label`, `phaseTitle`, `agentId`, `state` `start`, `progress` or `done`, `startedAt`, `lastToolName`, `tokens`, `toolCalls`, and the content previews `promptPreview` and `resultPreview`) | step `workflow`; the agents not at all |
| Shell commands of workflow agents | `task_started` with `owned_by_subagent = true`, a `tool_use_id` the main stream never names, mostly `is_backgrounded = false` | a `background_job` step at the root of the run per command (noise: one per command of every workflow agent) |
| End of a task | `task_notification` with `status` `completed`, `failed`, `stopped`; `usage` | end of the step |

The workflow journal files under the session folder (`.../workflows/<run>/journal.jsonl`) carry
the same agent list; the stream carries it already, live and without reading files of the CLI, so
the runner reads only the stream.

What reached the server before: the run steps above (`run_steps`, with `parent_step_id`). The live
frame read only the kind and name of the newest running step of each run (`LiveFacts`), so the
topology knew nothing of the children.

## 3. Data model

A **task node** of the live frame (`StatsLiveTask`) is one step of kind `subagent`, `workflow` or
`background_job` of an active run:

| Member | Content |
|---|---|
| `id` | step ID |
| `parent_id` | the task node it hangs under; absent: under the run itself |
| `kind` | `subagent`, `workflow`, `background_job` |
| `name` | step name: the adapter's type (`local_agent`, `workflow_agent`, `local_workflow`, `local_bash`, ...); never content |
| `state` | `running` (step `pending` or `running`), `completed`, `failed`, `cancelled` |
| `started_at`, `finished_at` | step times; the client computes the runtime against the frame's `at` |
| `tool` | current tool name: the newest running `tool_call` step owned by the node, else the `last_tool` the adapter reported in the step `detail` while the node runs |
| `label` | the step `title` (first line, at most 80 characters); only when the receiver may read the run (`run.read`), else absent |

The run carries `tasks` (absent when empty) and `tasks_truncated` (absent unless nodes were left
out by a limit below).

**Parent.** Task steps hang under the tool call that started them (D-RV-3), and the tools of a
subagent hang under the same call. The server resolves the task parent by walking up the steps:
a task step is the answer; a tool call with exactly one task child stands for that child (the
merge rule of 09 5.6 item 4), unless that child is the node whose parent is being resolved; any
other tool call is passed through; no parent is the run. Workflow agents hang directly under their
workflow step.

**Limits** (server, configuration of 25 15):

- `stats.live_max_tasks` (default 25) nodes per run; running nodes first, then the newest finished;
  a kept node always keeps its ancestors. `0` switches the tree off.
- `stats.live_task_max_depth` (default 4): deeper nodes are left out.
- `stats.live_task_keep` (default `2m`): a finished node stays in the frame this long after
  `finished_at`, then it is gone (the "collapse after a while").
- The payload bound `sync.event_payload_max_bytes` drops the trees of the oldest runs before it
  drops any run (`tasks_truncated` on each run that lost its tree).

## 4. How the runner derives it

- A `task_progress` of a known workflow task with `workflow_progress` adds one step per
  `workflow_agent` (key: workflow task and `agentId`, else `index`): kind `subagent`, name
  `workflow_agent`, title the `label`, parent the workflow step, detail
  `{"type": "workflow_agent", "phase": <phaseTitle>, "last_tool": <lastToolName>, "usage": {...}}`.
  `done` ends it `completed`; `failed` and `error` end it `failed`; every other state is running.
  A changed `lastToolName` or usage replaces the detail (the patcher throttles to
  `agent.run_update_min_interval`). `promptPreview` and `resultPreview` are content and never
  leave the runner.
- When a workflow task ends, its agent steps still open end with the workflow's status.
- A task with `owned_by_subagent = true` whose `tool_use_id` names no known step and that is not
  backgrounded gets no step (a foreground command of a workflow agent; the agent's node shows it as
  its current tool). A backgrounded one keeps its step as before.
- The workflow agent keys are part of the turn record, so an adopted turn reuses its steps.

## 5. Privacy

Chapter 25 rule 3 ("no content") had one exception added: the label of a task node is a step
title, which is run content. It is sent only to a receiver who holds `run.read` on the run, checked
per receiver with the scope of `run.read` (the same check as the run graph, D-RV-10); for anyone
else the node has no label and the client shows the kind and position ("Subagent 2"). Tool names,
kinds, states and times are not content (rule 3 already allows tool names). The content previews
of workflow agents never reach the server.

## 6. Client

One shared drawing in `packages/bothy_viz`, used by every app and the console:

- Under each run card an indented tree (an org chart read top to bottom): node cards with kind
  icon, label (or "Subagent 2"), state as color, icon and word, runtime and the current tool chip;
  elbow lines from the parent to each child.
- A node with children has a toggle that collapses its branch; the choice survives frames.
- Finished nodes are drawn faded; the server removes them after `stats.live_task_keep`.
- Live updates: the run card and its tree are one cached widget per run; a frame rebuilds only
  runs whose model changed (25 7.3), so an unchanged tree does not flicker. Runtimes tick with the
  frame (`at`), not with a client timer.
- Phone: the same vertical tree in the narrower lane.
- Table form: one row per run, then one row per task node, its agent cell indented by depth and
  named by label or kind and position; the columns state, step (current tool) stay.
- Tap on a node opens the run's thread, like the card.

## 7. Not in this change

- A "waiting" state for a child: the stream has no signal that a subagent waits; a run waiting for
  a person stays the run's `waiting_input`.
- Phases of a workflow as nodes of their own: the phase is in the agent's detail, not a node.
- Children of other adapters (Hermes reports no subagents in the pinned version).
- An index for the read of the task steps: the read is bounded by `run.max_steps_persisted` per
  run and the runs of the frame; it runs once per `stats.live_interval` only while someone watches.
