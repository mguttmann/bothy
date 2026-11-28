# FIX-RUNNER-BG-VISIBILITY Background mode: background commands are steps, a lost login waits

Owner's word (2026-11-25): everything that runs locally out of Bothy must be visible in Live. A
read-only diagnosis of the same day found two gaps of the Claude Code runner in background mode.

Gap C (the trigger). Reproduction: runner log of a run (one channel of the live instance) on
2026-11-25 05:07 UTC: `account_wait_started` (`not_logged_in`, `background_tasks` 1),
`account_retry` 1, `counter_check_failed` `request_without_login`, `run_finished` `failed`
`billing_mode_mismatch`. The transcript of its session shows why: a request record with
the login beta at 05:06:36 was refused by the CLI itself (`Not logged in`, synthetic,
`authentication_failed`); the runner waited and wrote its retry line at 05:07:37; the request
record of that turn (05:07:38) lists no `oauth-2025-04-20` and the model answered it with a
Bash call with `run_in_background`; the next request record (05:07:41) lists the beta again.
The counter-check (`RunTurn._read_background` before this change) stopped the session at that
answer, and the session took its background command with it. The same line stands seven times
in the runner log since FIX-RUNNER-BG-INBOX-AUTH (18.11.).

Gap A (the display). Cause: `TranscriptMapper` made task messages only from calls of the Agent
tool and from workflow results. A Bash call with `run_in_background` was one `tool_call` step
that ended with its launch result; between the turns the reader reported the one task
`background` (`BACKGROUND_TASK`), which only set `/metrics/background_jobs`. Live showed the run
card without a step for the running command.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-BG-VISIBILITY-1 | 2026-11-25 | The mapper gives `task_started` of a `local_bash` task for every `tool_use` of the tool `Bash` with `input.run_in_background` true, known by the id of the tool call and titled with `livebox.step_title` of the command (its first non-empty line). The launch result keeps the task open when it names `toolUseResult.backgroundTaskId` (that id is noted, so a notification naming the task id alone ends it too), ends it `failed` when it is an error and `completed` otherwise. The session's own notification (`<tool-use-id>` or `<task-id>`, `<status>`) ends it like an agent's. The live box makes it a `background_job` step under the call's `tool_call` step (code it already had). This supersedes the line "a background shell command stays the one `tool_call` step of its tool" of D-FIX-RUNNER-BG-AGENT-STEPS-2. | The end of a single background command is in the transcript: the recording `background_2.1.326` and the live transcript of 2026-11-25 hold the notification with `<task-id>`, `<tool-use-id>` and `<status>` and the producer `session-task`. The title is the one the call's `tool_call` step already shows, so nothing new leaves the host: steps have no further cut or redaction in the runner, and the step title of a tool call has been the command's first line since M2. The brief asked for the first command line rather than the call's `description` (which print mode uses). |
| D-FIX-RUNNER-BG-VISIBILITY-2 | 2026-11-25 | While the session list says `busy` between two turns, `background_tasks_changed` lists the open background commands of the process (`TranscriptMapper.open_jobs`, with `task_id` and `description`) instead of `background`, and only when no command is open the one task `background`. The reader reports the list again at every change while busy, and the empty list once the list says `idle`, which ends every `background_job` step still open `completed` and forgets the open commands, so a later busy phase does not list them again. | So `/metrics/background_jobs` counts the commands, the step of a command whose notification came is no longer listed, and a command the CLI stopped without a notification (a stop leaves no transcript marker) does not stay running in Live forever. During a turn the list is not reported, as before: the transcript has no record of the CLI's task list. |
| D-FIX-RUNNER-BG-VISIBILITY-3 | 2026-11-25 | The counter-check of 18.3 runs before `ProcessState.apply` (`RunTurn._counter_check`). An output without a proven login while a wait for the account runs (`ProcessState.account`) or after the last account error of the process was `not_logged_in` (`ProcessState.login_lost`, cleared by an answered turn on a proven login) is withheld: no live box change, no answer, no work, no live cost, and no ledger entry (so it answers no input and is never the result the end of the process is judged by; a wait given up after withheld answers ends with the account error, found by the review of the first version); its turn end does not end the wait (`apply(..., proven=False)`); the session and its background work are kept; `counter_check_withheld` is logged once per phase, `counter_check_login_back` at the next request with the beta. Every other output without a proven login still stops the session and ends the run `billing_mode_mismatch`, or tries the turn once more under the rules of FIX-RUNNER-LOGIN-BLIP-RETRY (`RunTurn._login_unproven`, unchanged; the two fixes were built side by side and joined on 2026-11-28). | The safety rule stays whole: nothing the session puts out without a proven login is accepted. What changes is only the reaction in the one situation the live log shows, a brief loss of the login that the account rules of 18.9 already handle: the wait's retries and its bound `runner.account_retry_max_wait` (default 20 minutes) decide, and a session whose login does not come back ends with the account error after that bound. The task messages of a withheld turn (a background command it starts) still reach the live box: the command runs on the host, and it must be visible (owner's word); its step then hangs at the root of the run, because the withheld call has no `tool_call` step. |

Not covered (reason in one sentence each):

- Gap B, processes started with `--detach` outside the session, stays a rule for the agents
  (brief).
- During a turn, `/metrics/background_jobs` is not updated (the transcript has no task list);
  the step of each command is visible from its call.
- An adopter replays the transcript with `proven` true, so a withheld turn end before the
  stored offset ends the adopter's wait; the stored `bg.login` keeps the counter-check closed
  for the output after the offset. No such case was seen; the window is one runner restart
  during a withheld turn.

Open minors (for the next run in this area of the runner):

- `login_lost` can be set without a running wait (a `not_logged_in` message after which the
  turn goes on, not seen with the pinned CLI); the withholding is then not bound by
  `runner.account_retry_max_wait`, only by the end of the session.
- An adopter during a withheld phase replays with `proven` true and may end the run
  `billing_mode_mismatch` again (fails closed; see "Not covered").
- During a withheld phase the runner keeps writing retry lines up to the bound; each costs a
  model request if the CLI answers it on an unproven credential.
- SonarQube Cloud on pull request 1798 (quality gate passed): cognitive complexity
  (python:S3776) of `runturn.py` `_read` (the counter-check and the withheld branch added to
  the one loop over mapped messages) and `processstate.py` `ProcessState.apply` (the
  `proven` flag), and of `runturn.py` `_computer_finish` (CU code already on main). A split
  of `runturn.py` is a ticket of its own, as for FIX-RUNNER-CLI-PIN-326.

Deploy note: the runner only (no server, app or schema change). A turn that runs at the
restart is adopted and mapped again from its launch offset, so its open background commands
get their steps then.
