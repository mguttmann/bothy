# FIX-RUNNER-LIMIT-KEEPS-RUN An account error no longer ends a run that holds background work

Owner report: a long session of the Claude Code runner ended although its background work was
still running, and nothing went on until a person wrote. Normative text: 09 18.5 and 18.9
(addenda of 2026-11-13), R-AG-58, the keys `runner.account_retry_*` of 09 21.

## Evidence

What the CLI writes, read in the file of the pinned CLI 2.1.321 on 2026-11-13 (09 18.19.12,
"Kind of an API error"):

- A turn whose request the API refuses ends with an `assistant` message with
  `is_api_error_message` and a `result` with `is_error` and `terminal_reason` `api_error` (both
  lines as the CLI wrote them against the local stand-in for the API: `tests/real_cli/captured`).
  The message names the kind of the error in its member `error`, an enumeration of the CLI's
  message schema. Four of its values are errors of the account or of the API's capacity:

  | `error` | When the CLI gives it | Text of the CLI | What the runner reads besides `error` |
  |---|---|---|---|
  | `rate_limit` | an answer with status 429; a used-up usage window also carries `api_error` `usage_limit_reached` | for a used-up usage window "You've hit your" with the name of the limit (for example "5-hour limit") and a reset time in the time zone of the host | `api_error_params.rate_limit_info.resetsAt` (seconds since the epoch) |
  | `authentication_failed` | no login, or an answer with status 401 or 403 | without a login "Not logged in" with the advice to log in | nothing |
  | `oauth_org_not_allowed` | an answer with status 401 or 403 that says OAuth authentication is not allowed for the organization | "Your organization has disabled Claude subscription access for Claude Code" with advice | nothing |
  | `overloaded` | an answer with status 529 | not read | nothing |

- None of the four says anything about the turn: the same request can succeed a moment later
  without any change to the session.

The reproduction is the scenario
`tests/scenarios/test_account_errors.py::test_a_usage_limit_at_a_task_notification_keeps_the_work_and_tries_again`
with Fake-Claude: a turn starts a background job and answers, the CLI starts a turn by itself
for the task's notification, and that turn ends with the API error of a used-up usage window.
Against the code before this change the runner logged `turn_stopped_after_error` and stopped the
process with its task, and the run ended `failed` with `runtime_error` (the red run is quoted
under "Tests").

The cause, in the code before this change (commit `5784fc70d8`):

- `processstate.py:162` to `processstate.py:171` (`ProcessState._error_result`): every error
  `result` of a turn the runner can write to sets the failure mark, whatever the error was.
- `processstate.py:294` to `processstate.py:298` (`ProcessState.decide`): with the mark, stdin
  closes at once, and `runner.stop_grace` later the answer is `Decision.STOP`.
- `runturn.py:1425` (`RunTurn._read` calls `_idle` at the `result`) and `runturn.py:1453` to
  `runturn.py:1458` (`RunTurn._idle`): on `STOP` the runner logs `turn_stopped_after_error` and
  stops the group (`runturn.py:1493`, `stop.py:109`: SIGINT, SIGTERM, SIGKILL). Every background
  task of the process ends with it.
- `failures.py:261` to `failures.py:284` and `recovery.py:218` to `recovery.py:232`: the end is
  classified `runtime_error` (a login that is gone: `runtime_not_authenticated`), and no end of
  that kind starts anything. Nothing goes on until a person writes.

That rule is D-FIX-runner-auth-failed-held-1 and was right for the error it was made for: a turn
that failed must not hold the run for background work. It is wrong for an error that says
nothing about the turn.

Run against the pinned CLI 2.1.321 and a local stand-in for the API (no login, no model call;
`tests/real_cli/test_refused_request.py`): after a request refused with `429` the process stays
with stdin open and its background task keeps running; a further input line starts a turn of its
own in the same process and is answered; the request of that turn still carries the input of the
failed turn, in the same user message as the new line. The same holds when the failed turn is
one the CLI started by itself: the job ends, the CLI starts a turn for the task's notification,
that request is refused, and the request of the runner's retry line carries the notification
(`<task-notification>`) next to the line.

## Rulings

| No | Date | Ruling | Reason |
|---|---|---|---|
| FIX-RLKR/1 | 2026-11-13 | An account error is an API error message of the turn itself (`is_api_error_message`, no `parent_tool_use_id`) whose member `error` is `rate_limit`, `authentication_failed`, `oauth_org_not_allowed` or `overloaded`, followed by the turn's error `result` with no answer of the model in between. The runner's names of the classes are `usage_limit`, `not_logged_in`, `subscription_disabled` and `overloaded`. The text of the error is never read. Place: `account.py` (`ACCOUNT_ERRORS`, `account_error`). | These are the four errors the owner named. The member is an enumeration of the CLI's message schema, while the texts are built from parts (09 18.19.12) and name the time zone of the host. An error of a subagent is that task's, and the words of the model may quote any error text (D-M2-35-18). Every other value of the enumeration (`server_error`, `billing_error`, `invalid_request` and the rest) stays an error of the turn. |
| FIX-RLKR/2 | 2026-11-13 | The wait applies to a turn process the runner can write to that holds or held background work (`BackgroundWork.seen`): a turn the CLI starts for a task's notification, also the one of the last task, when the task list is already empty; the turn of a follow-up written to a held process; and a turn that fails after it started background work. A process that never had background work ends at the first account error as before, and its summary says so (FIX-RLKR/7). For a process the runner cannot write to (stdin closed, a compaction, a turn adopted with reading rights only) nothing changes: its account error is handled like any other error `result`. | The brief asks for the smallest behaviour that keeps the work, and allows a turn without background work to fail at once. Such a turn loses nothing but the request itself, a person has just written and sees the reason, and an error of the setup (a wrong key, a host that is logged out) still answers at once. The notification turn of the last task has to be included: when it ends the run, the result of finished work is never reported. |
| FIX-RLKR/3 | 2026-11-13 | During the wait the error sets no failure mark: stdin stays open, no signal is sent, the run stays `running`, progress patches go on while a task runs, and a follow-up of a person is written at once as before and is itself a try. `/status_line` of the run document says "Waiting for the Claude Code account", the class of the error, the reset time when the CLI gave one, and when the next try comes; it is sent again after every further error and every retry and removed when the wait ends. Log codes: `account_wait_started` (class, running tasks), `account_retry` (number, class, whether the line was written), `account_wait_ended`, `account_wait_gave_up`. | The contract has what is needed: a run in `running` with a status line that the adapter may set and every client shows in the live box (09 5.3, 5.4). A new run state, wait reason or box note code would change the contract, the server and three clients for a state that lasts minutes. The wait reasons of 5.2.1 belong to a `queued` run. |
| FIX-RLKR/4 | 2026-11-13 | A retry is one input line of the runner's own words, written like every further input (`RunTurn.offer_input`, 09 18.19.7: recorded with its `uuid` before it is written). It says that it is an automatic retry and no message of a person, why the previous turn ended, and to go on where that turn stopped without starting work again that runs or is done. Its turn's answer is posted like any other. Place: `account.py` (`retry_input`), `runturn.py` (`RunTurn._account_retry`). | A turn the CLI starts by itself for a task's notification has no input line the runner could write again, and a turn of the same process cannot be forced otherwise. The input of the failed turn is still part of the conversation and is sent again with the line (run against the pinned CLI, see above), so one form of retry serves a message, a follow-up and a notification. The refused request had no effect, so nothing happens twice (09 1 point 5). |
| FIX-RLKR/5 | 2026-11-13 | Owner decision: the first retry comes `runner.account_retry_first` (default `1m`) after the error, the second `runner.account_retry_second` (`2m`) after the first retry failed, every further one `runner.account_retry_interval` (`3m`) after the one before; each pause counts from the last error `result` or, when no answer came, from the last retry line. No retry is written while a turn runs or an input is queued. The reset time of a usage limit is shown and never used as a waiting time. `runner.account_retry_max_wait` (`20m`) bounds the wait from its first error; `0s` switches the retry off. | Such an error can clear within a minute or two, so the runner tries again that soon and never waits until the end of the usage window. The default of the bound is several times the pauses, so an account that needs a few tries to come back is still met, and it is short enough that a person who watches gets an honest end in the same sitting instead of a run that shows `running` for hours on an account that does not come back. Each try is one refused request. |
| FIX-RLKR/6 | 2026-11-13 | The wait ends when a turn of the process is answered: a `result` without error that reports at least one model turn. A success `result` with `num_turns` 0 changes nothing. It also ends at the bound: the error then counts as an error of the turn from that moment, so stdin closes, the process is stopped `runner.stop_grace` later (`turn_stopped_after_error`) and the run ends by 18.9 as before, with the reason `runtime_error`, or `runtime_not_authenticated` for a login that is gone. Any other error `result` during the wait is the turn's own error and ends the wait the same way at once. A stop by a person, a run the server ended, a stop of the runner and a handover work at every moment as before; a process that ends by itself during the wait ends the run by its last error. | The CLI writes a success `result` without a model turn for a turn it ends without a request to the model (09 18.19.12; the turns of a resume after lost work, FIX-runner-hold-background, are such), which proves nothing about the account. After the bound nothing new is needed: D-FIX-runner-auth-failed-held-1 already says how a failed turn of a held process ends. The wait is a state of the read loop, not a task of its own, so every existing check of the loop (stop, server end, handover) runs unchanged between two polls. |
| FIX-RLKR/7 | 2026-11-13 | A run that an account error ended names it in its `summary`: the class in words, the reset time in UTC when the CLI gave one (from `api_error_params.rate_limit_info.resetsAt`, never from the text), how often the runner tried again and that `runner.account_retry_max_wait` bounds that, or, for a process without background work, that nothing was kept and the run ended at once; then the lost tasks and what the next message does, as before. Place: `causes.py` (`turn_end_cause`), `account.py` (`AccountError.describe`). The reason stays `runtime_error` or `runtime_not_authenticated`. | A person saw only "runtime error" and a box note without a text, because the CLI names the limit in the `result` text, which is no member of `errors`. The runner's own words carry no name of the account, of the organization or of the host; the text of the CLI is not repeated, as for the version error (D-RUNNER-CLI-PIN-2). The reason `usage_limited` (09 5.2) belongs to the usage window of 18.19.10 with its wake-up and its notification and is not used for an end without one. |
| FIX-RLKR/8 | 2026-11-13 | The wait lives in `ProcessState` (`AccountWait`, `Decision.RETRY`, `Decision.GIVE_UP`) and is written by `apply` like every other stream-derived value; what the runner adds goes through `account_retried`, `give_up` and `process_ended`. A runner that adopts a turn with full rights replays the lines and so starts the wait again at the adoption, with the first pause and the whole bound. The turn record gets no new member. | 09 18.19.7: every value the stdin and stop rule reads lives in that object, and the class test over every stream and checkpoint holds the adopter to the old runner's decisions (it now includes a stream with an account error). Restarting at the adoption follows the rule of the replay for every window: held longer, never shorter. |
| FIX-RLKR/9 | 2026-11-13 | Amended by this ticket: D-FIX-runner-auth-failed-held-1 (the stop `runner.stop_grace` after an error `result` does not apply while an account error is tried again) and D-FIX-runner-auth-failed-held-2 (a turn that failed for the account is tried again inside the run; any other failed turn is not). R-AG-17 and 09 1 point 5 are untouched: no run that was aborted is started again, the run is still `running`. The usage mark of 18.19.10 keeps its precedence in `ProcessState._error_result`; the stream reader still skips `rate_limit_event` until AR-06. | The older rulings named the login as their example and no other account error. The stage AR design wakes a thread at the reset of the usage window; that stays the answer for an account that is really used up, and AR-06 decides how its mark and this short retry meet (noted in the brief of AR-06). |
| FIX-RLKR/10 | 2026-11-13 | Test stand-ins: an API error fixture of Fake-Claude carries the members the pinned CLI writes (`members`: `error`, `api_error_status` and the rest), also the existing fixture of a missing login; `account_gate` keeps the account unusable until the test opens it, for the turn of every further line and for the notification turn of the job's end. The scenarios of an ordinary error in a held process (`tests/scenarios/test_handover.py`, variants c and d of AR-T02) use the new fixture `server_error`. The stand-in for the API can send response headers with an error. | A fixture without the member `error` would have kept the old tests green on a message the CLI does not write. The two handover scenarios are about an error of the turn, which a missing login no longer is in a held process. Without `x-should-retry: false` the CLI tries a `429` again by itself for minutes before the turn sees it. |
| FIX-RLKR/11 | 2026-11-13 | The texts of the runner give what the pinned CLI writes, read in its file or run against the stand-in, and the reproduction as tests. They say nothing about one account or installation: no plan, and no count, date or duration of its use. `tools/docs/tests/test_runner_texts.py` (`ONE_ACCOUNT`) reads every tracked file of the runner, the rulings of its changelog fragments and chapter 09 for the wordings of that class. | AGENTS.md, "This repository is public": no billing or plan details and nothing about the machines and tools somebody uses privately. The red team of this ticket found both in the first draft of its texts, and the white-label scan holds terms, not this class. The test lives with the tools tests, like the one of the host service (`test_host_service_texts.py`): they run on every change, also on one that touches only a ruling, which starts no job of the runner (`tools/ci/components.py`). |
| FIX-RLKR/12 | 2026-11-13 | Two rules the review of the pull request asked for. (a) The bound is decided like a retry, only while no turn runs and no input is queued (`ProcessState.decide`): a turn that is open when `runner.account_retry_max_wait` passes is read to its `result` first; an answer ends the wait, a further account error gives up at once. 09 18.9, the key in 09 21 and the texts of the key say so. (b) A success `result` without a model turn (`num_turns` 0) read after the account error that ended a process does not classify its end as `completed`: `classify_turn_end` then goes on with the table and `runtime_error` (`TurnEvidence.account_failed`, set from `ProcessState.account_error`). An answered turn after such an error still gives `completed`, and a requested stop still `stopped`. This amends D-M2-35-7. | (a) The read loop decides between turns: giving up in the middle of a turn would stop a try that may be answered a moment later. The bound can therefore be passed by the length of one open turn, which the text did not say. (b) FIX-RLKR/6 already says that such a `result` proves nothing about the account and leaves the wait as it is, and 09 18.9 says that a process that ends by itself during the wait ends the run by its last error. The classification read only the last `result`: a process that ended during the wait after such a `result` ended the run `completed`, without the error and without a word about the lost work (reproduced, see "Tests"). The same held for a stop after the bound whenever the process wrote no `result` at the signal. |

## Tests

Each of these failed against the code before the change, unless it says otherwise. The red run
of the scenarios ended with "the run ended failed (runtime_error; 1 background task was still
running and was stopped; its result is lost ...) before it showed the wait".

- The reproduction and the fix, `tests/scenarios/test_account_errors.py`:
  `test_a_usage_limit_at_a_task_notification_keeps_the_work_and_tries_again` (the process and the
  task live, the status line names the class and the reset time, one retry after the pause, its
  answer is posted, the run completes in one process),
  `test_every_account_error_keeps_the_process_and_is_tried_again` (the four classes),
  `test_the_second_retry_comes_after_the_first_one_failed` (both pauses, measured in Fake-Claude),
  `test_a_followup_that_meets_the_limit_is_answered_by_the_retry`,
  `test_the_notification_of_the_last_task_is_tried_again`.
- The bound: `test_the_bound_ends_the_run_with_the_reason` (summary, lost task, task step
  `cancelled`, `account_wait_gave_up`, then `turn_stopped_after_error`) and
  `tests/scenarios/test_background.py::test_login_lost_while_background_runs_ends_the_run_with_its_cause`
  (a login that stays gone ends `runtime_not_authenticated` after the bound).
- Stop and shutdown: `test_stop_during_the_wait_ends_the_run_at_once`,
  `test_a_stop_of_the_runner_during_the_wait_ends_the_turn`; takeover:
  `tests/scenarios/test_handover.py::test_a_wait_for_the_account_survives_the_handover`.
- A process that ends during the wait (FIX-RLKR/12):
  `test_a_process_that_ends_during_the_wait_ends_the_run_by_the_account_error`. With a `result`
  without a model turn after the error it failed with "assert 'completed' == 'failed'"; without
  one it was green before. The cases of `tests/unit/test_failures.py` hold the classification.
- What stays: `test_an_ordinary_error_while_background_runs_still_ends_the_run` (green before and
  after), `test_a_turn_without_background_work_fails_at_once_and_says_why` (the end was the same
  before, the summary was missing), `test_the_retry_is_off_with_a_bound_of_zero`.
- The rule itself: `tests/unit/test_processstate.py` (the schedule to the bound on a fake clock,
  a `result` without a model turn, an ordinary error, the model's words and a subagent's error,
  stdin closed, the replay, and the class test with the stream `background_account_error`),
  `tests/unit/test_account.py`, `tests/unit/test_causes.py`, `tests/unit/test_config.py`.
- The texts (FIX-RLKR/11), `tools/docs/tests/test_runner_texts.py`:
  `test_texts_of_the_runner_describe_no_account_in_use` (green on the texts before this
  change; it failed on the first draft of this ticket's texts: the README, `account.py`,
  Fake-Claude and three of its fixtures, the scenarios, this file and chapter 09) and
  `test_a_sentence_is_found_in_every_kind_of_text` for the reader itself, on a made-up
  repository.
- Against the installed pinned CLI, behind the switch of the real-CLI suite:
  `tests/real_cli/test_refused_request.py`, and the members the runner now reads in
  `tests/real_cli/test_pinned_cli.py` (`READ_TOKENS`).

## Limits and open minors

- Not built: a wait for a process that never had background work. Its turn ends the run at the
  first account error, with the reason in the summary, and the next message continues the
  session. Extending the wait to it is the condition `held` in `ProcessState._error_result`; it
  is a product decision, because a first turn with a wrong key or on a host that is logged out
  would then show the wait for `runner.account_retry_max_wait` before it fails.
- `authentication_failed` is also what the CLI writes for a refused API key. In a held process
  such a turn is tried again until the bound, although the key cannot change while the process
  runs.
- Run against the pinned CLI is a request refused with `429` only (`rate_limit`). The further
  members of a used-up usage window and the other three classes are read in the CLI's file,
  and the fixtures are built from it; an `overloaded` error reaches the turn only after the
  CLI has tried the request again by itself.
- A background task that is itself an agent can fail on the same account error. The runner
  retries the turn of the main loop only; the agent learns of the failed task from its
  notification.
- After a handover the adopter starts the wait again, so the bound counts from the adoption and
  the retries from one. A runner that restarts more often than the bound never gives up.
- A turn adopted without a named pipe (09 18.9, reading rights only) gets neither the wait nor
  the account sentences in its summary, as it gets no failure mark.
- The echo of a retry line counts as an input of a person for the nudge mark and the count of
  autonomous inputs in `ProcessState` (09 18.19.11, 5.8.6). Nothing reads those two before AR-05
  and AR-07; the ticket that reads them flags the retry line as neither (noted in the briefs of
  AR-05, AR-06 and AR-07).
- The status line is one text for every reader: the reset time is UTC, not the reader's zone. A
  turn that sets the status line during the wait (a transcription, an upload of answer files)
  replaces it until the next error or retry. For a voice follow-up that is the `result` of its
  own turn: the follow-up is written once its transcription ended, and its turn ends with an
  answer (the wait ends) or with a further account error (the line is sent again).
- The bound is not hard: it is decided while no turn runs and no input is queued (FIX-RLKR/12),
  so a turn that is open when it passes is read to its `result` first.
- Fake-Claude writes an error `result` at SIGINT also between two turns. Whether the pinned CLI
  does so is not established; the classification no longer rests on it (FIX-RLKR/12 b).
- A run that waits holds its slot of `runner.max_parallel_runs` and its thread for up to the
  bound; a person can stop it.
- Hosted agents run through the same `RunTurn` and get the same wait (read in the code, no
  scenario of its own).
- Not changed: `docs/design/autonomous-runs.md` section 2.3 describes the runner at a named
  commit; one sentence there now points here.
- `tools/docs/tests/test_runner_texts.py` holds wordings: it recognizes no number, date or
  duration, so a review still reads the texts of a change against the rule. A ruling is read
  while the changelog fragment of its ticket exists, so until the next release of the runner.
  The older list for the texts about the pin (`tests/unit/test_versions.py` of the runner)
  stays where it is and still runs only with the tests of the runner.
- Seen once in four runs of the whole unit and scenario suite under load, in a scenario this
  change does not touch:
  `tests/scenarios/test_error_paths.py::test_session_missing_starts_new_session_fail_open[bind]`
  ended `failed` instead of `completed`. It passed in the other three runs and in 108 runs of
  its own, some of them six at a time. The cause is not established. Read in the code, not
  shown: that module leaves `agent.invocation_token_ttl` of the FakeServer at its default,
  while the scenario clock runs faster than real time and the run starts two CLI processes.
