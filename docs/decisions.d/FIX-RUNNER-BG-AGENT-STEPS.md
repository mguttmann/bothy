# FIX-RUNNER-BG-AGENT-STEPS Background mode: the agents a session launches are steps again

On 2026-11-18 the production runner ran its turns in background mode with the pinned CLI
2.1.326. In a turn whose session launched two agents with the Agent tool and
`run_in_background`, the live topology showed only the run, without its agents; in print mode
the same kind of turn had shown them. The turn record of that run held `steps.tasks` `{}`,
`steps.agents` `{}` and `steps.subagents` `0`, while its `steps.tools` held the two Agent calls.
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
| D-FIX-RUNNER-BG-AGENT-STEPS-1 | 2026-11-18 | The transcript mapper gives the task messages of the stream itself: `task_started` of a `local_agent` task for every call of the tool `Agent`, `task_started` of a `local_workflow` task for a tool result with `async_launched` and that `taskType`, and `task_notification` for the result of an Agent call that is not `async_launched` and for the session's own notification of a call this process started. The task is known by the id of its tool call. This replaces, for subagents of the Agent tool and workflows, the part of D-FEAT-RUNNER-BG-SESSIONS-7 that each is the one `tool_call` step of its tool. Evidence: `test_the_background_agents_of_a_real_turn_are_subagent_steps` with the records (structure only) of that turn, red before this change, and the tests of a foreground agent, a background workflow and a notification of another sender. | One place, the mapper, so the live box, the turn record and the adoption work from the same messages in both modes. The id of the tool call is the one id that the call, its result and its notification all carry; the agent's own id first appears in the result. A notification is read only when its origin is the session's own and only for a call this process made, so no other sender's text opens or ends a step, and only the part before the agent's result (tool call, status) and the usage after it are read. |
| D-FIX-RUNNER-BG-AGENT-STEPS-2 | 2026-11-18 | Not covered: a background shell command stays the one `tool_call` step of its tool; the agents of a workflow, the steps inside a subagent and the usage before the end are not shown; an agent whose notification comes to a later session process stays open until the run ends. | The reader reports background work between turns as one task of its own (`background`), whose list would end a `background_job` step at once; the transcript of the main conversation carries no `workflow_progress` and no records of the agents. None of these is the reported fault. |

Deploy note: the runner's restart. A turn that runs at the restart is adopted and mapped again
from its launch offset, so its agents get their steps then.
