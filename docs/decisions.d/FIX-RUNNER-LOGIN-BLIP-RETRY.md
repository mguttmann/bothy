# FIX-RUNNER-LOGIN-BLIP-RETRY One retry after a request without the login

Owner report (2026-11-24): runs in background mode ended `failed` with `billing_mode_mismatch`
although nothing about the account had changed. Normative text: 09 18.3 (addendum of
2026-11-24) and the exception in 09 18.9 "Classification of a process end".

## Evidence

- The runner log of the production host has five pairs of `counter_check_failed` with the code
  `request_without_login` followed by `run_finished` `failed` `billing_mode_mismatch`, two of
  them for runs that started in the same second.
- The transcripts of those two sessions hold, right after the prompt, one `api-request` record
  whose `params.betas` lack `oauth-2025-04-20`; the request was answered. Requests of other
  sessions two seconds before and two seconds after list the beta, and the next request of
  the same sessions lists it again. So the CLI sent one request in the moment the stored login
  was rewritten on the host.
- In one of the two sessions the answer to that request was a text only. In the other it was
  a `Bash` tool call: its `assistant` record was written before the tool ran, and the tool
  result written when the session was stopped says its outcome is unknown. So a stopped
  attempt may have acted, and its record shows that once its process is gone.

## Rulings

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-RUNNER-LOGIN-BLIP-RETRY-1 | 2026-11-24 | When the counter-check of 18.3 fails with `request_without_login`, the session is stopped as before and nothing of it is posted. Once per run the turn is then tried again: the runner waits `LOGIN_RETRY_WAIT` (10 seconds), runs the pre-check `claude auth status` of the start again, and resumes the same session with the inputs no `result` answered (log `login_retry_wait` with `seconds`, then `login_retry_started`). The retry gets the counter-check like every process; a second failure ends the run `billing_mode_mismatch`. A refused pre-check ends the run with its reason. | The owner approved one retry for this case on 2026-11-24. Each attempt is checked on its own, so what the billing guard accepts does not change: an answer is posted only after a request that showed the stored login, and the pre-check before the retry refuses an API key, a missing login and every unknown method as at the start. |
| D-FIX-RUNNER-LOGIN-BLIP-RETRY-2 | 2026-11-24 | The retry is refused, and the run ends at once as before with the warning `login_retry_refused`, when the attempt may have acted or is not this runner's: an adopted turn (`adopted`), a run that retried before (`retried_before`), a stop or a run the server ended (`stopping`), an output of the process accepted before the failed check (`output_accepted`), a process still alive after the stop (`session_alive`), a tool call in the transcript after the launch offset, in the main conversation or a side chain, where a file or line that cannot be read counts as a call (`tool_called`), and no unanswered input (`nothing_to_repeat`). | A prompt must never run twice once it has acted. The CLI writes the record of a tool call before it runs the tool (evidence above), so after the stop the transcript shows every call that may have run. An output accepted before means an earlier request showed the login and its turn may have run tools. |
| D-FIX-RUNNER-LOGIN-BLIP-RETRY-3 | 2026-11-24 | The wait is a constant of the runner (`LOGIN_RETRY_WAIT`), not a configuration key. The check `no_request_record` and the print-mode check of `apiKeySource` keep their behaviour. | The gap in the evidence was about two seconds, and nothing on the host needs a different value. A session that writes no request record at all is not the moment of a rewrite. |

Evidence for the rulings: `tests/scenarios/test_bg_inbox_auth.py` (one request without the
login is tried once more and answered; a second one ends the run; a tool call as the first
output is never run again; a login gone after the wait ends the run `runtime_not_authenticated`;
a stop during the wait ends it `cancelled`; a turn that acted before keeps the old end; an
adopted turn is refused with `adopted`) and
`tests/unit/test_bgtranscript.py` (`tool_called`).

What stays open: the counter-check reads the request records alone, so it cannot tell which
credential answered the one request without the login; that request and its answer happened
before the check, with or without this retry.

Minors noted in the review, left for the next run in this area: `login_retried` is not part of
the turn record (no effect, an adopted turn never retries); the stopped attempt's one request
is not added to the run's costs; a cancellation of the run by the SDK itself during the wait
is untested (the server's stop during the wait is tested).
