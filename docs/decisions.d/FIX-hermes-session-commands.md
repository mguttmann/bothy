# FIX-hermes-session-commands An owner's session command typed as a message, and a paused session

Found in operation (24.11.2026, a production profile, thread `01a1b423-1b57-78db-9906-7d58ce2a10bc` in
channel `01a18834-bc4e-71d3-b16b-774dd6c29a71`): the run of 22.11. 09:00 ended `failed`
`interrupted` when Hermes restarted during an adapter rollout, so Hermes fenced its turn
`unknown`. After the reboot of 24.11. Hermes paused the session `20261107_045242_60c46534`
(`gateway.log`: `paused: unknown_execution`). The first run (12:23) got Hermes' pause notice as
its answer; every later run (12:24, 14:21, 14:23, 14:24 `/discharge`, 14:25 `/discard`, 14:25)
ended `completed` after 30 to 60 ms without a message (`runs` and `messages` of the production
database, read only). Cause, read in the adapter and in the pinned Hermes source: the adapter
hands every person's text to Hermes with `allow_gateway_control` off (D-M2-33-8), and
`MessageEvent.is_command` is false for such an event, so `/discard` was a plain message that
Hermes admitted into the paused queue. Hermes' `admit_message` answers only the first message of a
pause episode (`pause_notice`, `pause_notified`) and every later one with `None`, so the adapter
saw a completed processing without a reply. The "Gateway message origin" header in Hermes is the
busy-steer injection (`gateway/run_busy.py`), not the adapter's, and played no part. Reproduced
with the Fake-Hermes rig against the pin (`tests/fake_hermes/test_paused_session.py`: SIGKILL of
the gateway during a held turn, restart, then runs on the thread).

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-HSC-1 | 2026-11-24 | A run sends `/discard`, `/reset`, `/new` or `/stop` as the Hermes command instead of a turn exactly when: `trigger_kind = message`; the token's `tool_class` is `host`; the run context's `trigger_message` has `origin = human`, an author of kind `user` equal to the token's `sub` and no attachments; its text, stripped and with the name lower-cased, is exactly `/<one of the four>`. Everything else stays a turn. Place: `session_commands.py`, `handler.py` (`prepare`, `_session_command`). | The adapter has no read of a person's owner role (`user.read` is for people and apps only, 05). For tool class `host` the server lets only an active owner trigger the agent and checks it at acceptance and before every delivery (07 SR2, `authz/domain/guards.go` `toolClassOwnerOnly`); the token is signed per delivery, so the claim proves the owner. SR2's other way, a ready dev environment of the person's own, is not set by the server yet (`DevEnvironmentReady` is never true); the dev environment milestone must revisit this rule. A team input is excluded as a whole: a member's reply has `origin = agent`, and an owner's copy has the sponsor as authority (3.9.2), so neither is the run's person's own word for this purpose. |
| D-FIX-HSC-2 | 2026-11-24 | The command goes out through `Runtime.run_command` from the control sender, Hermes' confirmation of `/new` and `/reset` is answered "once" (`CommandRequest.confirm`) and the text `slash_confirm.resolve` returns joins the command's replies (Hermes sends a confirmed command's reply through no `send`; review finding of 2026-11-24), each reply text is posted as the run's answer, no reply gives `COMMAND_FAILED`, and the run ends `completed` through the usual end (session report, hold of work Hermes starts itself, such as the saved messages `/discard` releases). | The same path and sender as every other Hermes command of the adapter (17.4); Hermes judges the control sender by its allow list, which carries it since D-FIX-HBS-1. Ending through `_end_turn` reports a new session after `/reset` and covers the turns the released queue runs when they start in time. |
| D-FIX-HSC-3 | 2026-11-24 | A run's turn that Hermes reports complete without any `pre_llm_call` of the thread while the run was open and without any answer of the run gets the answer `NOT_ANSWERED` and ends `completed`. | Hermes keeps no readable pause state the adapter could ask; "no turn and no reply" is exactly the paused admission. A silent reply passes `pre_llm_call` and stays silent. |

Not changed: the registered command table keeps `discard` refused, so the apps offer no
`/discard`; a typed one is the only way, and only for an owner. Saved messages Hermes runs after
`/discard` once the run has ended have no run: their tool calls are refused and their answers are
posted as reports (17.12), as before.

Open minors for the next run in this area:

- The rule of D-FIX-HSC-1 relies on SR2 alone; once the server sets a ready dev environment, a
  non-owner with one could reach the four commands for a run of their own (needs the claim
  `dev_environment_id` in the SDK's claims and a check here).
- `/discard` releases the saved messages; when their turns start after the command run ended
  (race with `_hold`), they run without tools. Not reproduced as harmful; the test accepts both.
- A typed `/reset` or `/new` in a run of session mode `bind` is reported as `bind_failed`
  (`session_missing`), because the new session is neither the bound one nor its parent.
- A typed `/stop` waits for a free session like every run, so it reaches only an idle session;
  stopping a running turn stays the server's `/stop`.
- Whether Hermes replays the saved messages after `/discard` with their old `reply_to` (then
  `send` refuses them with "no active run") is not verified.
- A turn Hermes starts itself between the wait for a free session and the dispatch can queue the
  run's message: the run gets `NOT_ANSWERED`, the answer comes later. Not reproduced.
- On an agent of tool class `chat` or `scoped` a typed `/discard` stays text and the session
  stays paused; `NOT_ANSWERED` still names `/discard` and `/reset` there, where only the server's
  `/new` helps (red team r1).
- `run_command` with `confirm` waits for the pending confirmations of every thread, not only its
  own (`platform.py`, red team r1; no harm reproduced).
- D-M2-33-8 ("never runs a Hermes command") and the row "Hermes commands" of 09 17.3 ("Every other
  gateway command is refused") carry no reference to D-FIX-HSC-1 yet (red team r1).
- Open minors of FIX-hermes-busy-session still stand (read for this run, none touched).
