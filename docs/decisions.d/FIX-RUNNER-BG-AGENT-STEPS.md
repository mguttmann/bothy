# FIX-RUNNER-BG-AGENT-STEPS Background mode: the agents a session launches are steps again

Reproduction condition: a runner runs its turns in background mode with the pinned CLI
2.1.326, and the session of a turn launches two agents with the Agent tool and
`run_in_background`. Result: the live topology shows only the run, without its agents; in print
mode the same kind of turn shows them. The turn record holds `steps.tasks` `{}`,
`steps.agents` `{}` and `steps.subagents` `0`, while its `steps.tools` holds the two Agent calls.
Cause: the live box makes a `subagent` or `workflow` step only from `system/task_*` messages,
which only the stream of a print turn carries. The session transcript, the live source of a
background turn, has no such record: it has the `tool_use` of the Agent call, its tool result
with `toolUseResult.status` `async_launched` and the agent's id, and later the notification the
session gives itself (`<task-notification>` with `<tool-use-id>`, `<status>` and `<usage>`).
The mapper reduced the tool result to `tool_use_id` and `is_error` and read nothing of the
notification, so no task message reached the live box (D-FEAT-RUNNER-BG-SESSIONS-7 named this
as a limit of the mode).

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-BG-AGENT-STEPS-1 | 2026-11-18 | The transcript mapper gives the task messages of the stream itself: `task_started` of a `local_agent` task for every call of the tool `Agent`, `task_started` of a `local_workflow` task for a tool result with `async_launched` and that `taskType`, and `task_notification` for the result of an Agent call that is not `async_launched` and for the session's own notification of a call this process started. The task is known by the id of its tool call; a notification that names only the CLI's task id (an agent that waits on its own background work, seen in the same turn) is matched by the `agentId` or `taskId` of the call's `async_launched` result. This replaces, for subagents of the Agent tool and workflows, the part of D-FEAT-RUNNER-BG-SESSIONS-7 that each is the one `tool_call` step of its tool. Evidence: `test_the_background_agents_of_a_real_turn_are_subagent_steps` with the records (structure only) of that turn, red before this change, and the tests of a foreground agent, a background workflow and a notification of another sender. | One place, the mapper, so the live box, the turn record and the adoption work from the same messages in both modes. The id of the tool call is the one id that the call, its result and its notification all carry; the agent's own id first appears in the result. A notification is read only when its origin is the session's own and only for a call this process made, so no other sender's text opens or ends a step, and only the part before the summary (task id, tool call, status) and the usage after the agent's result are read: the summary holds the agent's description, chosen by the model. |
| D-FIX-RUNNER-BG-AGENT-STEPS-2 | 2026-11-18 | Not covered: a background shell command stays the one `tool_call` step of its tool (superseded on 2026-11-25 by D-FIX-RUNNER-BG-VISIBILITY-1); the agents of a workflow, the steps inside a subagent and the usage before the end are not shown; an agent whose notification comes to a later session process is not ended by it; a notification `completed` of an agent that waits on its own background work ends its step, as in print mode, and a later one changes nothing. | The reader reports background work between turns as one task of its own (`background`), whose list would end a `background_job` step at once; the transcript of the main conversation carries no `workflow_progress` and no records of the agents. None of these is the reported fault. |

Deploy note: the runner's restart. A turn that runs at the restart is adopted and mapped again
from its launch offset; the adopter gives the task messages of that replay to the live box
before it reads on, so its agents get their steps then (a step the record has keeps its id),
and a later notification of an ended agent changes neither its status nor its usage.

Open minors (for the next run in this area of the runner):

- The test of a tool call in the summary passes without the split at `<summary>` too; a test
  needs a notification without `<tool-use-id>` in its header and a foreign one in its summary.
- `agentId` and `taskId` of a `toolUseResult` are mapped to every tool result of the record;
  the pinned CLI writes one result per record, so no harm is reproduced.
- Ruling 2 says a notification `completed` of an agent that waits on its own background work
  ends its step as in print mode; that was not checked against a print turn.
- The cause above names `<tool-use-id>` only; the notification by task id alone is in ruling 1.
- An agent that `SendMessage` resumes (FIX-RUNNER-SENDMESSAGE-OWN) gets no step for its further
  work: its notification names the `SendMessage` call, which opened no step, and its first step
  is already ended. Nothing is mapped to a wrong step; ruling 2 does not name the case yet.
- An adopter counts a subagent twice in `/metrics/subagents` when the runner died after it
  stored the step id of a task and before the server got the step: `restore` drops the task
  but keeps the stored count, and `replay_tasks` adds the task and counts it again. The step
  itself is right. `restore` lowers the count by the dropped subagent tasks; a replay that
  only skips the count is not enough, because with a stored offset before the task's start
  the later read of the transcript adds and counts the task again.
- No scenario test covers the call of `replay_tasks` in the adopter (`runturn.py`): replacing
  it with `pass` leaves the adoption and background scenarios green. A test adopts a turn
  whose agent started before the stored offset and sends its notification after it.
- The design row of agent steps in background mode says a step is open "from the call"; a
  `workflow` step opens only with the `async_launched` result of its call
  (`bgtranscript.py` `_task_results`). Its end is described right.
