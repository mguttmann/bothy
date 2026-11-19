# FIX-RUNNER-CONTRACT-TOLERANT The contract subset accepts stream members a newer CLI adds

The operator's update chain rejected Claude Code 2.1.328 on 2026-11-19: of the contract subset
only `test_recorded_stream_fields_equal_the_installed_cli` failed, because the CLI's schema of
`system/init` names one member more (`host_dropped_env`), inserted before members the
recording knows. The test compared the recorded list with the fresh one position by position,
so an added member the runner never reads failed the proof.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-CONTRACT-TOLERANT-1 | 2026-11-19 | Against a tested newer CLI (`CLAUDE_CODE_RUNNER_TESTED_VERSION`) the recorded field table is a floor (`tests/real_cli/record.py` `field_changes`): every member of a recorded message schema must stay in that message's schema, every recorded member of `background_task`, `usage` and `task_types` must stay, and `task_status`, `command_states` and the workflow type stay equal. A member the recording does not know passes and is named in a `UserWarning` of the test. At the pin the table stays exactly equal to the recording. | The stream reader ignores members it does not read, so an addition changes nothing the runner does; a lost or renamed member (a loss plus an addition) can drop a field the runner reads, and a new status or command state would end no step. The exact check at the pin keeps the recording itself honest. |
| D-FIX-RUNNER-CONTRACT-TOLERANT-2 | 2026-11-19 | `stream_fields_<v>.json` names in `written_only` the members of each message that only the recorded turn wrote (no schema names them); `record.py` writes it, and for 2.1.326 it was derived from the pinned CLI's file and the recorded turn with the same functions. These members are not held against a newer CLI's schema. | They were appended to the schema members in one list, so a comparison could not tell a member a newer schema dropped from one no schema ever named. What the runner reads of them (`isReplay`) stays held by the token tests of `test_pinned_cli.py`. |

## Not covered

- The table holds member names, not their types; a member that keeps its name and changes its
  type passes the field test and shows only in a real turn (level two) or in failed runs.
