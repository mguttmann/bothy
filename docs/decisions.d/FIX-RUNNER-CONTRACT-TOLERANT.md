# FIX-RUNNER-CONTRACT-TOLERANT The contract subset accepts stream members a newer CLI adds

The operator's update chain rejected Claude Code 2.1.328 on 2026-11-19: of the contract subset
only `test_recorded_stream_fields_equal_the_installed_cli` failed, because the CLI's schema of
`system/init` names one member more (`host_dropped_env`), inserted before members the
recording knows. The test compared the recorded list with the fresh one position by position,
so an added member the runner never reads failed the proof.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-CONTRACT-TOLERANT-1 | 2026-11-19 | Against a tested newer CLI (`CLAUDE_CODE_RUNNER_TESTED_VERSION`) the recorded field table is a floor (`tests/real_cli/record.py` `field_changes`): every member of a recorded message schema must stay in that message's schema, every recorded member of `background_task`, `usage` and `task_types` must stay, and `task_status`, `command_states` and the workflow type stay equal. A member the recording does not know passes and is named in a `UserWarning` of the test. At the pin the table stays exactly equal to the recording. | The stream reader ignores members it does not read, so an addition changes nothing the runner does; a lost or renamed member (a loss plus an addition) can drop a field the runner reads, and a new status or command state would end no step. The exact check at the pin keeps the recording itself honest. |
| D-FIX-RUNNER-CONTRACT-TOLERANT-2 | 2026-11-19 | `stream_fields_<v>.json` names in `written_only` the members of each message that only the recorded turn wrote (no schema names them); `record.py` writes it, and for 2.1.326 it was derived from the pinned CLI's file and the recorded turn with the same functions. These members are not held against a newer CLI's schema. | They were appended to the schema members in one list, so a comparison could not tell a member a newer schema dropped from one no schema ever named. Of what the runner reads of them, `isReplay` is held by the token tests of `test_pinned_cli.py`; `workflow_progress` and `owned_by_subagent` are not (see the open minors), as before this change. |
| D-FIX-RUNNER-CONTRACT-TOLERANT-3 | 2026-11-21 | Amends D-FIX-RUNNER-CONTRACT-TOLERANT-1 for the task types: against a tested newer CLI a recorded task type fails only when the runner maps it to a step kind of its own (`READ_TASK_TYPES` of `tests/real_cli/record.py`: `local_agent` and the workflow type `local_workflow`; `test_livebox.py` holds that exactly these get a step kind of their own in `LiveBox`). Any other recorded type the CLI no longer names is named in a `UserWarning` of the test, like an added type. Message members, `background_task`, `usage`, the value lists and the workflow type keep the rule of D-FIX-RUNNER-CONTRACT-TOLERANT-1; at the pin the table stays exactly equal. | The update chain rejected 2.1.329 (2026-11-20) and 2.1.330 (2026-11-21) only because their task type table no longer names `local_memory_import`; their field tables lose nothing else and change no compared value. The runner reads no other type: `livebox` gives every type besides those two a `background_job` step named by the type, so a type that no longer occurs changes nothing it does, and a new type is handled the same way. Losses of message members stay strict because `READ_TOKENS` is not a complete list of the members the runner reads (see the open minors). The list is named in the test program, not imported from the runner, because the chain runs these tests on the code of the running runner. |

## Not covered

- The table holds member names, not their types; a member that keeps its name and changes its
  type passes the field test and shows only in a real turn (level two) or in failed runs.

Open minors (for the next run in this area of the runner):

- `workflow_progress` and `owned_by_subagent` (read in `livebox.py`) are members only the
  recorded turn wrote and are missing from `READ_TOKENS` of `test_pinned_cli.py`, so no test of
  the contract subset holds them against a newer CLI.
- `field_changes` names no message schema that a newer CLI adds beyond `record.SCHEMAS`; it can
  only report additions inside the recorded messages.
- The `state` values of the workflow agent entries (`livebox._AGENT_END`) are not part of the
  field table, so neither the pin nor a tested CLI is held against them.
- `test_livebox.py` holds `READ_TASK_TYPES` against the recorded task types and one invented
  type; if `LiveBox` came to map a type of its own that the recording does not name, the list
  would stay short unnoticed.
- The version row of spec 09 section 18.2 lists the pins only up to 2.1.325; the code pins
  2.1.326 (D-FIX-RUNNER-CLI-PIN-326-1).
