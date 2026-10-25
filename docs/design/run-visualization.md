# Run visualization: run graph and control room

Status: design of 2026-10-24, approved scope of the owner: a short design with the decisions,
then build tickets. This note explains and justifies; the normative text is in the chapters it
changes ([01-product.md](../spec/01-product.md) D3, D10, D14, M3, M11, M12;
[03-data-model.md](../spec/03-data-model.md) `runs`, `run_steps`;
[04-sync-protocol.md](../spec/04-sync-protocol.md) 11.3;
[09-agents.md](../spec/09-agents.md) 5.3, 5.5, 5.6, 18.4;
[25-visualization.md](../spec/25-visualization.md) 1, 7.1). Rulings:
[decisions.d/run-visualization.md](../decisions.d/run-visualization.md) (`D-RV-1` to `D-RV-12`).
Tickets: M2-51 (server and runner), M4-39 (desktop), M5-14 (iPhone and Android) in the
[plan](../PLAN.md).

## 1. Goal

Owner request (2026-10-24): "I want to really see how things work, also when a run starts several
agents, see what is happening right now. Not only text, but visual."

Done means:

1. A person who may read a run (`run.read`) opens a **run graph**: the agent as the root, its
   subagents, workflows and background jobs as children in a tree on a time axis, each node with
   state color, duration, cost where known and the current tool; the graph grows live while the
   run is active.
2. A person opens a **control room**: every active run the person may see (`run.overview.read`)
   as a live card that says what the run does now, whether it is stalled and what it costs; each
   card opens its run graph.
3. Both views have a list form with the same content for screen readers.
4. No new action, no new permission rule, no new gateway op and no new event type (one optional
   member of the op `focus`, D-RV-8).

## 2. What exists

| Part | Today | Gap for the goal |
|---|---|---|
| Run document (`run_document.v1`, 04 11.1, 09 5.3) | steps with `kind`, `parent_step_id`, `status`, `started_at`, `finished_at`; at most `run.max_steps_persisted` steps; older finished leaves are merged into one summary step | the summary step sits at the root, so merging flattens the tree; `stall_warned_at` is not in the API document |
| Live box (01 D3, 09 5.4) | state, duration, steps with indentation, counts of subagents and jobs | a list of the newest steps, no time axis, no overview of parallel work |
| Claude Code runner (09 18.4) | `tool_call` steps get `parent_step_id` from `parent_tool_use_id`; tasks become steps `subagent` or `background_job` | task steps have no parent, so a subagent and the tools it calls are not connected; no step is ever `workflow`; no usage per task |
| Hermes adapter (09 17.3) | `tool_call` steps from status sentences | no signal about subagents in the pinned version: the graph of a Hermes run is flat |
| Running now (01 D10, M4-23) | rows from `listActiveRuns` and `run.state_changed` | no current tool, no stall, no way into the structure of a run |
| Live topology (01 D12, 25 6.1) | run cards for owners and managers from `stats.live` | by rule without content (no titles) and only with `stats.read`; not a view for every person |
| Stall "no progress" (09 5.5) | not built: no time of the last progress is kept (D-M2-09-19) | the control room cannot say "stalled" |

## 3. Decisions

Each decision has a ruling ID in the fragment; the chapters carry the normative text.

**D-RV-1 Form of the run graph: an indented trace view on a time axis.** One row per node in
tree order (depth first, children by `ordinal`), indented by depth; to the right of the label a
bar from `started_at` to `finished_at` (or "now") on a shared time axis that starts at the run's
start. Reason: the tree and the time axis are both readable at once, the layout does not jump when
nodes arrive, it scales to hundreds of nodes with a virtualized list, the same form works on a
phone, and the list fallback is the same rows without bars. A free node-link diagram was rejected:
its layout moves on every new node, it needs zoom and pan for more than a few dozen nodes and it
has no time axis.

**D-RV-2 One derivation, shared by all apps.** The graph is computed from the run document only
(`getRun`, `listRunSteps`, `run.updated`), by one pure function in `packages/bothy_viz`
(`lib/src/run_graph/`), which every app uses. The rules are normative in 09 5.6. Reason: desktop,
iPhone and Android must draw the same tree from the same document; a second derivation would drift.
The server sends no graph of its own: the document already carries everything, and a server view
would be a second source of truth for the same steps.

**D-RV-3 Parent references come from the adapter; the runner sets them for tasks.** The server
keeps `parent_step_id` as today (set at `add`, never replaced, must name a step of the run). The
Claude Code runner additionally sets `parent_step_id` of a task step (`subagent`, `workflow`,
`background_job`) to the step of the tool call that started the task (the task's `tool_use_id`),
and keeps setting `parent_step_id` of a tool call from `parent_tool_use_id`. The graph then draws a
tool call that has exactly one task child as that task's node (merge rule in 09 5.6), so a subagent
and its tools form one subtree. Reason: the stream links tasks and tools only through tool use
IDs; the runner holds the mapping from tool use ID to step ID, the server does not. Where the
stream carries no `tool_use_id` for a task, the task stays at the root; the graph is then less
deep, never wrong. Hermes reports no such signal in the pinned version; its graph stays flat until
an adapter version reports one.

**D-RV-4 Workflows.** A task whose `task_type` names a workflow becomes a step of kind `workflow`
(today every non-agent task is a `background_job`). The value of `task_type` is taken from a
recording of the pinned Claude Code version in M2-51; until it is recorded no step is `workflow`.
Phases of a workflow appear as children only where the stream reports them as tasks with a
`tool_use_id`; this change adds no phase source of its own. Reason: no guessed values in the
mapping table (09 18.4 marks every row by its source).

**D-RV-5 Cost per node: the root carries the run cost; a child only a reported cost.** The root
shows `cost` of the run with the rules of the live box (only for `organization`, "Estimate" while
`is_estimate`). A child shows tokens when its step `detail` carries `usage.tokens`, and a cost
only when it carries `usage.cost_micros` from the adapter and the run's `billing_mode_used` is
`organization`, always with "Estimate" (the same amount rule as the root and 01 6.7). The runner fills `usage` from the usage of
`task_notification` where the stream reports it. Reason: the server books costs per run (10), and a
split of the run cost over subagents by tokens would be wrong as soon as subagents use different
models; showing nothing is honest, showing an invented share is not.

**D-RV-6 Node limits.** The server limit stays `run.max_steps_persisted` per run. The summary step
of 09 5.3 is now kept per parent: finished leaves are merged into one summary step under their own
parent (the root counts as one parent), so a long run keeps its tree instead of moving old work to
the root. A finished step whose only child is its own summary step counts as a leaf: it is removed
with its summary step, and both counts go into the summary step of its own parent, so finished
subtrees collapse upward and the bound stays reachable. In the client, a node with more than `ui.livebox_visible_steps` finished children shows
the newest ones and one row "n earlier steps" that expands on demand; a running node and every
ancestor of a running node are never collapsed. Reason: the server bound already exists and is
tested; the client rule reuses the live box value, so no new configuration value is needed.

**D-RV-7 The control room is "Running now" with cards.** D10 keeps its place (sidebar entry,
Cmd+Shift+R, the tab "Running" on mobile) and gets a switch "Cards" and "List"; cards are the
default. A card shows agent, path, state, duration, current tool, open subagents, workflows and
jobs, a stall notice, cost with the suffix rules of D10, Stop where `run.cancel` is allowed, and
opens the run graph. A run that ends leaves the cards, as it leaves the list today. Reason: one
place for active runs, one permission, no new navigation item to learn.

**D-RV-8 Live data of the cards through `focus`, one new optional member.** The control room puts
the threads of the cards that are visible on the screen into the new optional `focus` member
`background_thread_ids` (04 6.2), within `sync.max_focused_threads` together with the threads the
app already shows. For these threads the server sends the whole document once and then
`run.updated` (04 11.3), but no typing; they never count as "Active in the thread" and are not
stored in the presence focus (13 4.4, 03 `gateway_presence`). `idle` stays one flag per connection
for the window state (15 section 10), so the notification rules are unchanged. A card outside the
budget, or scrolled out of view, shows the summary from `listActiveRuns` and `run.state_changed`
with the note "live details paused". Reason: `idle` cannot serve here: with `idle = true` the
desktop would stop counting as "Desktop active" and the thread the person reads would lose "Active
in the thread"; with `idle = false` every card thread would suppress notifications although its
messages are not on the screen. A separate list costs one optional member, no new op, no new event
and no new receive rule; the load is bounded by the existing focus limit; a card that nobody sees
needs no live data. A dedicated overview event was rejected because it would need a new op, a new
receive rule in 05 and a new fan-out path for the same data the focus already delivers.

**D-RV-9 Stall detection is built.** The row "No progress" of 09 5.5 is built in M2-51:
`runs.last_progress_at` is set by `run.start` and by every stored patch; `runs.stall_check` sets
`stall_warned_at` for a run in `running` whose `last_progress_at` is older than `run.stall_after`,
raises `version` and sends the change as `run.updated`; the next stored patch clears it (already
built, D-M2-10-2). `stall_warned_at` becomes a member of the API document and of `RunSummary`.
The Claude Code runner turns `tool_progress` into a patch of `/metrics/progress_events` at most
once per `runner.progress_patch_interval`, so a long tool that is still working is not stalled.
Reason: the control room must say "stalled" from a server fact, not from a client clock; a run
waiting for a person (`waiting_input`) is not stalled.

**D-RV-10 Permissions are those of today.** The graph reads with `run.read` (`getRun`,
`listRunSteps`, `run.updated` with focus); the control room lists with `run.overview.read`, which is
filtered by `run.read`; Stop needs `run.cancel` as before. No new action and no change to 05 or 07.
Reason: the graph shows exactly the run document a person may already read in the live box.

**D-RV-11 Surfaces.** Desktop (macOS, Windows): run graph D14 and control room D10. iPhone and
Android: run graph M12 and control room M11, in density `touch`, the graph as the same rows with a
narrower time axis (the full axis in landscape). Web console: neither view. Reason: the console has
no chat and no run views (ADR 0028, 01 5.5); its live view is the live topology D12 with its own
rules, and a card there still opens the app by deep link.

**D-RV-12 Accessibility and boundaries.** Each view has a switch "As list": the run graph as an
outline list (tree semantics with level, position, state as text, duration, cost), the control room
as the rows of D10. Screen readers hear the end of a node and state changes politely, at most once
per `a11y.live_announce_min_interval`, never every patch. Reduced motion: nodes appear without
animation. The run graph and the control room are not views of chapter 25 level a: they show run
content under `run.read`, so the content rules of 25 section 1 (no titles, no names) do not apply to
them, and they do not read `stats.*`. Reason: WCAG 2.2 AA as in 01 8; the boundary keeps the
statistics rules unchanged.

## 4. Server and client

| Part | Server (M2-51) | Client (M4-39, M5-14) |
|---|---|---|
| Parent of task steps | runner sets `parent_step_id` from `tool_use_id`; kind `workflow`; `usage` in `detail` | merge rule, tree, time axis |
| Summary step | one per parent | row "n steps summarized" under its parent |
| Stall | `last_progress_at`, `runs.stall_check`, `stall_warned_at` in `Run` and `RunSummary`, `run.updated`; runner progress patch | stall notice on node and card |
| Live data | `focus` member `background_thread_ids`: `run.updated`, no typing, not in presence | visible cards in `background_thread_ids`, `idle` from the window state |
| Limits | `run.max_steps_persisted` | collapse above `ui.livebox_visible_steps` per parent |
| Permissions | none new | Stop only where the server allows `run.cancel` |

## 5. Not in this change

- A graph across runs (one run starting another run in another thread). Runs are independent
  objects; a link between runs needs a field of its own and a reason to exist.
- Replay of a finished run with a time slider. The finished graph is shown complete; the daily
  timeline of 25 6.5 stays the replay view.
- Per node costs estimated by the client.
- The run graph in the web console and in TV mode.
- A Hermes signal for subagents; it follows when an adapter version reports one.
