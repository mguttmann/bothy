# FIX-RUNNER-HOSTING-REVIEW Leftover hosted turns: checked records, a stop that fails, errors logged by kind

Follow-up to AUD-plugin-claude-code-runner-01. Two automated security reviews of that change, one
of its commit and one of the pushed branch, named five points in
`plugins/claude-code-runner/src/claude_code_runner`. Each was treated as a claim and read again in
the code of the main branch after the package had landed (commit `d060117fd0`; every `file:line`
below names that commit). Points 2 to 5 hold, and so does the first part of point 1; they are
fixed, each with a test that failed before. Two parts of point 1 do not hold and are recorded with
the reason. Normative text: 26 3.2 item 5 and 09 18.9 (addenda of 2026-11-12).

## The claims

| Claim | Holds | Where, in the code before the change |
|---|---|---|
| 1 a. A turn record is used for signals and lock names without a check | yes | `hosting.py:220` to `hosting.py:225` parse only `run_id` and `thread_id`. `hosting.py:232` builds the name of the session lock from `session_ref` as it stands, and `locks.py:57` to `locks.py:59` create the missing folders of that path and open the file: a reference with path characters gave a lock outside the lock folder. `hosting.py:229` and `hosting.py:244` hand `pid` and `pgid` of the record to the stop |
| 1 b. The identity is not checked again before a signal, because `verified=True` is passed | no | `stop.py:96` to `stop.py:102` ask `may_signal` before every `os.killpg`, and `adoption.py:200` runs the whole check there for as long as the leader lives. The flag only decides for a group that has lost its leader (`adoption.py:203` to `adoption.py:209`). The test `test_the_identity_is_checked_again_before_the_signal` passes on the code before the change as well |
| 1 c. A hosted turn can write into `hosted/<agent_id>/turns/` | no, for the folders the runner hands a turn | The folder is made with mode `0700` (`turn.py:289`, `statedir.py:102` to `statedir.py:106` for the agent's folder) inside a state folder that is refused when group or others have a permission on it (`config.py:338` to `config.py:345`). The working folder of a hosted turn lies under `runner.hosted_agents_root`, which is refused when it is equal to, inside or above the state folder (`config.py:469` to `config.py:471`); the added folder is `files/<thread_id>` (`runturn.py:967` to `runturn.py:969`) and the temporary folder `tmp/<run_id>` (`reply_upload.py:142`), both beside `turns`, not above it |
| 2. One failing stop ends the close of the hosted set | yes | `hosting.py:1280` and `hosting.py:1281`: `asyncio.gather` without `return_exceptions` raises at the first failure while the other stops still run, the set is not emptied, and the rest of the handler's close is skipped |
| 3. Two log lines carry the message and the traceback of an error | yes | `app.py:723` and `app.py:815` call `_LOG.exception`; the console script formats a record as its message followed by the traceback (`__main__.py:107`), which ends with the message of the error |
| 4. The identity check before a signal is weak | yes | `adoption.py:128` takes the session reference anywhere in the command line; nothing compares the `run_id` of a record with its file name (`hosting.py:223`); nothing ties the process to the agent |
| 5. One record stops the start for every agent | yes | `hosting.py:226` handles four kinds of error around the read only, and `hosting.py:236` only a lock timeout. `os.getpgid` raises `OverflowError` for a number above the system's type (`adoption.py:121` to `adoption.py:124` handle two other kinds), `UUID(5)` raises `AttributeError`, and a lock folder that is no folder raises `StateDirError` |

## Rulings

| No | Date | Ruling | Reason |
|---|---|---|---|
| FIX-RHR/1 | 2026-11-12 | One reader for every turn record, of the runner's own agent and of a hosted agent (`adoption.read_turn_record`, used by the adoption, by the list of the records of a handover and by `stop_leftover_turns`): the file is opened without following a link and without waiting and must be a regular file with a JSON object whose `run_id` is the UUID the file is named after and whose `thread_id` and `session_ref` are UUIDs in their canonical text (a session reference in upper case counts, as `choose_session` takes it). The member `session_ref` may be missing in one case only: a record of phase `preparing` or `waiting_lock` (`adoption.PHASES_BEFORE_START`), which a next runner removes by its phase (09 18.19.6). The record of a hosted turn always needs it (`hosting._leftover_record`). Anything else is unreadable: logged with `turn_record_unreadable`, left where it is, and nothing is built from it. | A record is a file on disk, and the names of both locks and several paths are built from it. The runner itself writes `run_id` and `thread_id` of every record as a canonical UUID, and the session reference of every record a turn writes (`RunTurn._persist`, with `runner.turn_handover` from the phase `transcribing` on), so the check changes nothing for those. The same lock name was built in the start of the runner's own agent (`app.py:619` before the change), so the check sits in the reader both starts share instead of in one of them. A named pipe in the place of a record held the start for good, which no handler of an error could end. With the key the runner also writes a record before `run.start` that carries the run, its thread and channel and the phase, and no session (`RunnerHandler._phase`). A next runner with the key removes such a record by its phase before any lock name or path would be built from it, and one without the key does not start while it is left, so it is read without a session. The first version of this change asked every record for a session reference. The review of the change found and reproduced what that broke: the next runner logged the record of a run in prepare as unreadable and left it, against 18.19.6, and `handover_records` no longer listed it, so a start without the key was no longer refused. |
| FIX-RHR/2 | 2026-11-12 | A process or group ID of a record is an integer above 1 that fits the system's type (`adoption.process_id`, at most `MAX_PROCESS_ID`). `process_matches` and `AdoptedGroup` take every other value as no process, so it counts as gone instead of raising. In the record of a hosted turn `pid` and `pgid` are either both such IDs or both missing (the record of a turn whose process had not started); otherwise that record is unreadable. No stop ever signals a group ID below 2, whatever its group object says of its identity (`stop._signal`, the one place that signals the group of a turn). | `os.getpgid` raises `OverflowError` above the type, and `AdoptedGroup` raised for a text or an infinite number, in the start of the own agent too. A hosted turn is always started directly, so its record carries both IDs or none; a record of the own agent written with `runner.turn_handover` may carry 0 before its CLI is known, so there the value counts as no process and the record stays readable. A signal to group 0 goes to the group of the caller. A group object without a group that counts as verified answers that it may be signalled: `AdoptedGroup` when it is built with `verified`, and `launch.KeeperGroup` for a record that matches but names no keeper (read in the code, not run with such a record); the stop then signals the runner's own group, which the test shows with a group object of its own. Every turn leads a group of its own, so the bound changes nothing for a turn. |
| FIX-RHR/3 | 2026-11-12 | The record of a hosted turn is acted on only at `hosted/<agent_id>/turns/<run_id>.json`, where the agent's folder is named by the canonical UUID and both folders are directories themselves, not links, that group and others have no permission on (`statedir.is_private_dir`, the state `ensure_private_dir` leaves; it creates them with mode `0700`). | That is where and how the runner writes it, so a record anywhere else is not one of its records. `ensure_private_dir` already refuses both folders in any other state when the runner writes into them. |
| FIX-RHR/4 | 2026-11-12 | A leftover hosted turn is signalled only when, beyond the identity check of 09 18.9, its command line as `ps` prints it now names the record's session as the whole argument of `--session-id` or `--resume` (`adoption.names_session`) and names the control folder of the agent whose state folder holds the record (`materialize.control_name`, the folder of the `--settings` file of every hosted turn, 26 5.3). The group object makes this check itself, first when the record is read and again before every signal (`hosting._LeftoverTurn`); the caller no longer passes `verified`. A process that fails the check counts as gone, as in 18.9. | With the reference anywhere in the command line a record could name any process whose command line contains a chosen text, with that process's own start time, and the runner stopped it. The session argument alone would still fit a session of Claude Code that a person runs with the same reference, so the process must also be a turn of this agent. Of the proofs the review offered, the command line is the one the runner can read on Linux and macOS alike: it cannot ask which process holds a lock, and it has no portable way to read the working folder of another process. The runner writes the session and the settings file into every hosted command line (`commandline.build_argv`), so a turn it started always passes; the scenario with a killed hosting runner proves that with the real command line. |
| FIX-RHR/5 | 2026-11-12 | `stop_leftover_turns` settles each record inside one handler: an unexpected error is logged with `turn_record_unreadable` and the kind of the error, the record stays, the locks taken for it are released, and the next record is handled. | The start of the runner must not depend on one file of one hosted agent. Releasing the locks belongs to it: a lock kept by the runner itself would block the thread's next run. A cancellation is no error and still ends the start. |
| FIX-RHR/6 | 2026-11-12 | `HostedAgents.close` lets every stop run to its end (`return_exceptions`), logs a failed one with `hosted_stop_failed`, the agent and the kind of the error, and empties the set in a `finally`. `HostedConnection.stop` waits for the end of its connection also when closing it failed, and cancels it after `sdk.http_timeout` as before, which stops its turns as the end of the adapter does. | Each stop waits until the turns of its agent are stopped (09 18.6). One failure ended that wait for all others and skipped the rest of the runner's close. The agent whose own close failed would have kept its connection, and with it its turns, until the process ended. |
| FIX-RHR/7 | 2026-11-12 | The error of a follow or a takeover that the runner started as a task is logged by its kind only, as a JSON line of level error (`adopted_follow_failed`, `turn_takeover_failed`, each with the run and the kind). The traceback goes to a line of level debug. | The message of an error may hold a path or a value, and the runner's log lines carry event codes and IDs, never content (`app.log`). `RunTurn._on_error` already logs the same error by its kind (`run_failed_unexpectedly`). |
| FIX-RHR/8 | 2026-11-12 | Claims 1 b and 1 c lead to no change of behaviour. A test pins each: `test_the_identity_is_checked_again_before_the_signal`, and the layout of a real hosted turn in `test_a_hosted_turn_answers_in_its_folder` (the record folder has mode `0700` and lies in none of the folders the turn was given). | See the table of the claims. For 1 c no mode keeps a turn of class `host` out: it runs commands as the runner's user. What a file in that folder can cause is bounded by FIX-RHR/1 to FIX-RHR/4 instead: a signal only for a turn of that agent, and a lock only in the lock folder. |

Amended by these rulings: AUD-CR1/2 (the log of a failed follow or takeover, FIX-RHR/7),
AUD-CR1/3 (which record is acted on and which process is signalled, FIX-RHR/1 to FIX-RHR/5) and
AUD-CR1/4 (a stop that fails, FIX-RHR/6).

## Evidence

Each test below failed against the code before the change, unless it says otherwise:

- Claim 1 a: `tests/unit/test_hosted_leftovers.py::test_a_forged_record_never_signals_the_foreign_process_it_names`
  (the foreign process got SIGINT) and
  `::test_a_forged_record_takes_no_lock_outside_the_lock_folder` (a lock file and a folder
  outside the lock folder), `::test_a_record_that_no_runner_wrote_is_unreadable_and_the_start_goes_on`
  and `::test_records_outside_a_private_folder_of_an_agent_are_unreadable`. For the runner's own
  agent: `tests/scenarios/test_crash.py::test_a_forged_record_takes_no_lock_outside_the_lock_folder`.
  The reader and the IDs: `tests/unit/test_adoption.py`. A named pipe in the place of a record
  held the read for good before the change: `test_a_link_or_a_pipe_in_the_place_of_a_record_is_unreadable`
  did not fail there but hung until it was ended from outside. The same read in
  `handover_records` was read in the code, not run against the code before
  (`test_the_records_of_a_handover_are_listed_by_the_same_reader`).
  No signal below group 2: `tests/unit/test_stop.py::test_a_group_id_below_two_is_never_signalled`.
- The record of a run before `run.start` (FIX-RHR/1, found by the review of this change): each
  of these failed against the first version of this change.
  `tests/unit/test_adoption.py::test_the_record_of_a_run_in_prepare_carries_no_session` and the
  record of a run in prepare in `::test_the_records_of_a_handover_are_listed_by_the_same_reader`;
  `tests/scenarios/test_launch_crash_points.py::test_the_record_of_a_run_in_prepare_is_removed_at_the_next_start`
  and `::test_without_the_key_the_record_of_a_run_in_prepare_refuses_the_start` (with the way
  out through `stop --turns`); and what the next runner logs of the record a real runner left, in
  `::test_handover_while_preparing_restarts_through_the_inbox` and
  `::test_handover_while_waiting_for_the_thread_lock`. The bound of the exception:
  `tests/unit/test_adoption.py::test_only_the_record_of_a_run_in_prepare_may_carry_no_session`,
  and for a hosted agent
  `tests/unit/test_hosted_leftovers.py::test_a_hosted_record_without_a_session_is_unreadable`.
- Claim 2: `tests/unit/test_hosting.py::test_close_stops_every_connection_when_one_stop_fails`,
  `::test_close_empties_the_set_when_it_is_cancelled` and
  `::test_a_connection_whose_close_fails_is_still_ended`.
- Claim 3: `tests/scenarios/test_crash.py::test_an_unexpected_error_while_following_an_adopted_turn_stops_it_and_fails_the_run`
  and `tests/scenarios/test_handover.py::test_an_unexpected_error_of_the_adopter_stops_the_turn_and_fails_the_run`:
  the injected error names a file, and neither that name nor a traceback is in what the console
  script writes.
- Claim 4: `tests/unit/test_hosted_leftovers.py::test_a_process_that_is_no_turn_of_the_agent_is_never_signalled`
  (the reference anywhere, a session without a control folder, a part of an argument, the
  argument of another flag, a turn of another agent).
- Claim 5: the cases of a process ID that is too large, a `run_id` that is no text and a file
  nested too deep in the tests of claim 1 a, and
  `tests/unit/test_hosted_leftovers.py::test_an_error_of_the_state_folder_at_one_record_does_not_stop_the_start`,
  `::test_an_unexpected_error_at_one_record_keeps_it_and_releases_its_locks`,
  `tests/scenarios/test_crash.py::test_a_record_with_a_process_id_out_of_range_does_not_stop_the_start`
  (before the change the start never reached its first heartbeat).
- No change for a turn the runner started: the unchanged scenarios of
  `tests/scenarios/test_crash.py`, `test_handover.py` and `test_hosted_turns.py`, among them
  `test_a_hosted_turn_left_by_a_crashed_runner_is_stopped_at_the_next_start`, which kills a
  hosting runner and lets the next one stop the turn by its real command line.
- Passing before and after, as the proof of what did not hold or did not change:
  `test_the_identity_is_checked_again_before_the_signal`,
  `test_a_record_written_before_its_process_started_is_removed` and the layout lines in
  `test_a_hosted_turn_answers_in_its_folder`.

## Limits and open minors

- Not changed: the identity rule for the turns of the runner's own agent (09 18.9). There the
  session reference may still stand anywhere in the command line, and nothing ties the process
  to the runner beyond its group and start time. With FIX-RHR/1 the reference is at least a
  UUID. The turn of the own agent has no control folder; a rule for it could name the prompt
  file of the run under `runner.state_dir`, which every command line carries. That changes the
  adoption of every turn, also of those handed over with a keeper
  (`launch.identity_matches`), and belongs in a ticket of its own with the whole handover
  suite.
- Not changed: what else the start of the own agent takes from a record (the stream path, the
  launch files, the working folder, and with `runner.turn_handover` the IDs of the keeper and
  its CLI, `launch.identity_matches` and `launch.KeeperGroup`), and an unexpected error there
  still ends the start, as recorded in AUD-plugin-claude-code-runner-01. Only a record that
  fails FIX-RHR/1 and a `pid` or `pgid` of the record itself that fails FIX-RHR/2 no longer do.
- Without `runner.turn_handover` the adoption does not ask a record for its phase, and it does
  not need to: `run` refuses to start while a record with a phase is left, and `stop --turns`
  reads those records with the key (`handover_records`, the same reader). Only a file put
  there between that check and the read of the same start would be settled as the record of a
  turn that is gone, with a session lock named after no session inside the lock folder, as
  before this change (read in the code, not run).
- A turn of class `host` can write any file its user can, also a record; see FIX-RHR/8.
- Not built: a proof by the lock or by the working folder of the process (FIX-RHR/4).
- `ps` joins the arguments of a command line with spaces, so the check of FIX-RHR/4 cannot
  tell an argument from the same words inside one longer argument.
- When the identity changes between the first check and a signal, the stop ends without a
  signal and the record is removed without the lock rule, as before this change.
- The console script logs from level info on and has no switch for level debug, so it does
  not write the traceback lines of FIX-RHR/7; a program that sets the level of the logger
  `claude_code_runner` gets them.
- Not changed: the SDK logs an unexpected error of a handler with its traceback
  (`bothy_sdk.adapter`), for a run of its own run loop too. That is a line of another
  component.
- With a close that failed, the stop of that agent takes `sdk.http_timeout`, because its
  connection is cancelled only after that wait. The stops of the other agents are not held up
  by it, but `HostedAgents.close` returns only when all of them are done.
