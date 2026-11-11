# AUD-plugin-claude-code-runner-01 Stops, unexpected errors and leftover hosted turns of the Claude Code runner

Five findings of the audit of 2026-11-11 against the Claude Code runner
(`plugins/claude-code-runner/src/claude_code_runner`): `claude-code-runner-1` to
`claude-code-runner-4` and `architecture-2`. Each was read again in the code of the current main
branch before the change. Three are fixed (AUD-CR1/1 to AUD-CR1/4), two ask for a redesign and
are not built here (AUD-CR1/5). Normative text: 09 18.6 (addendum on the stop of a process), 18.9
(unexpected error of the runner), 18.18 item 5 and 26 3.2 item 5 (hosted turns after a crash).

| No | Date | Ruling | Reason |
|---|---|---|---|
| AUD-CR1/1 | 2026-11-12 | The stop of a turn process is state of that process: `RunTurn.stopping` starts empty with every process of a run, together with `ProcessState` (`_reset_process_state`, called for a process started directly and for one started through the launch handshake). The stop request of the run (`stop_reason`) stays state of the run, so after a requested stop still nothing starts. | `stopping` was set by the first stop of a run and never cleared, and it guards `_stop_group` and both stop branches of `_idle`. When the runner stopped a process after a `result` with an error (18.5: background work kept the process) and the end was a full context, the run went on with a compaction and a retry (18.9), and for those processes a stop request, a run the server ended and a further stop after an error sent no signal: the run worked on to its own end and only then ended `cancelled`. That breaks 18.6 ("a stop during ... a compaction or a restart applies to the process that runs") for a stop by a person, a revoked access and a deleted forum. |
| AUD-CR1/2 | 2026-11-12 | An error the runner handles nowhere while a run is driven (`RunTurn.run`, `take_over`, `adopted`) ends in `RunTurn._on_error`: the live process is stopped as in 18.6 (an own process like at a cancellation, an adopted group only when its identity still matches), the run is finished `failed` with `adapter_error` and the SDK's summary that names only the kind of the error, the error is logged by its kind (`run_failed_unexpectedly`) and raised again. The record and the run files go by the existing end of the run once no process is left. At a handover nothing is stopped or finished. `RunnerHandler` logs the error of a follow or takeover it started as a task, with its traceback, as the SDK does for an own run. | Only a cancellation was handled. Any other error left the process handle set, so the record stayed while the SDK ended the run `failed`: the CLI ran on with nobody reading it, out of reach of a stop (the run was no longer known), holding the thread and session locks, and under a keeper with stdin open for good. A follow or takeover of an adopted turn runs outside the SDK's run loop, so its run was not even finished and stayed `running`. The SDK's finish of an own run finds the run finished (`RunHandle.finish` ends a run once), so it is reported once. The task of a follow has no caller that could take the error, hence the log there. |
| AUD-CR1/3 | 2026-11-12 | Hosted turns stay without adoption (D-A1-05-9, D-AR-03-9), but no longer without an end: at every start, after the records of its own agent and before any hosted connection, the runner reads `hosted/<agent_id>/turns/*.json` (`hosting.stop_leftover_turns`, with or without `runner.hosted_agents`). A process that matches its record is stopped as in 18.6 without a finish and the record removed; a process that does not match is never signalled and its record is removed when the thread and session locks are free, else it stays for the next start (`adoption_lock_busy`). | A runner that crashed or was killed left the hosted turn running: nothing read its records, so the turn worked to its own end without a stop path and without its costs, held the thread's lock, and its record stayed for good and kept the run's `tmp/` and `outgoing/` folders out of the retention sweep. The server already ends such a run (09 5.5), so stopping the process loses nothing the run could still deliver. The identity rule and the lock rule are those of 18.9 for the runner's own records: any doubt counts as gone, and a busy lock shows a turn that lives on. |
| AUD-CR1/4 | 2026-11-12 | `HostedAgents.close` stops the connections of all hosted agents together instead of one after the other. | Each stop waits until the turns of its agent are stopped (up to `runner.stop_grace` plus `runner.kill_grace` for a turn that ignores the signals), so the waits added up over the agents and could pass the stop timeout of a service manager, whose kill then leaves the turns of AUD-CR1/3 behind. |
| AUD-CR1/5 | 2026-11-12 | Not built here (findings `claude-code-runner-4` and `architecture-2`): cutting `RunTurn` into collaborators with their own state (process supervision, follow-ups, answer delivery, costs, handover), its phases as an enumeration with explicit transitions, a shared base of `RunnerHandler` and `HostedHandler`, `turn_handover` and `handing_over` as members of the protocol `TurnRuntime`, and a lint limit for the size of a class or a module. | It is a redesign, not a fix: it moves nearly every method of the class that carries every run and the state they share, and it has to be carried by the whole scenario suite and a review of its own. `architecture-2` also names classes of the Hermes adapter and of both SDKs, and a size limit would be a gate of the repository root; other work packages of the same audit change those places at the same time. The audit ties no wrong behaviour to the cut itself, which follows D-M2-35-16 and D-M2-35-28; the one fault it had caused, the stop state shared by the processes of a run, is fixed by AUD-CR1/1. Proposed order for a ticket of its own: the process supervisor first (start, stop and record, owning `ProcessState` and the stop state), then follow-ups, then answers and costs, each step behind the unchanged scenario suite. |
| AUD-CR1/6 | 2026-11-12 | Test stand-ins: Fake-Claude plays a turn that fails after it started a background job (`api_error` together with `background_gate`); `tests/hosting_runner.py` is the hosting runner as a process of its own and `hosting_world` can follow the real time for it. | The fault of AUD-CR1/1 needs a failed turn whose process stays for background work and leaves an input without an answer, so that a retry follows; the fault of AUD-CR1/3 needs a runner that is really killed while a hosted turn runs, which an in-process runner cannot show. |

Evidence. Each test failed against the code before the change, the tests of `stop_leftover_turns`
because the function did not exist:

- AUD-CR1/1: `tests/realserver/test_e2e_flows.py::test_cancel_reaches_the_retry_after_a_stop_after_an_error`
  (the server of this tree) and
  `tests/scenarios/test_stop.py::test_stop_reaches_a_process_after_a_stop_after_an_error` (the
  compaction and the retry, each started directly and through the launch handshake).
- AUD-CR1/2: `tests/scenarios/test_error_paths.py::test_an_unexpected_error_stops_the_turn_before_the_run_fails`,
  `tests/scenarios/test_handover.py::test_an_unexpected_error_of_the_adopter_stops_the_turn_and_fails_the_run`
  and `tests/scenarios/test_crash.py::test_an_unexpected_error_while_following_an_adopted_turn_stops_it_and_fails_the_run`.
- AUD-CR1/3: `tests/scenarios/test_hosted_turns.py::test_a_hosted_turn_left_by_a_crashed_runner_is_stopped_at_the_next_start`
  (a runner process killed with SIGKILL) and `tests/unit/test_hosted_leftovers.py`.
- AUD-CR1/4: `tests/unit/test_hosting.py::test_close_stops_every_connection_together`.

## Limits and open minors

- Not built: adoption of hosted turns. A hosted turn that outlived its runner is stopped, also
  when it could have completed; its run ends by the server's rule and the next message starts a
  new run. Adoption needs the hosted agent's own connection and token before its records are
  read (D-A1-05-9).
- The `tmp/` and `outgoing/` folders of a stopped hosted run are not removed at the start; the
  retention sweep of the agent's state folder takes them after the agent's next turn.
- Read in the code, not run: in the short windows in which a CLI may run while the turn holds no
  handle of it (an adopter that settles a recorded launch, and a launch between its go byte and
  the handle of its group), an unexpected error finishes the run but stops nothing, and the
  record is removed as before this change.
- A start whose record can be finished from the log runs that finish inside the start itself; an
  unexpected error there still ends the start of the runner. The run is finished and the record
  removed first, so the next start goes on.
