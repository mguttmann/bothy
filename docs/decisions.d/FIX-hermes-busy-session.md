# FIX-hermes-busy-session A stop and a new run while the thread's Hermes session is busy

Found in operation: in `personal` mode a person stopped a run whose model request hung in
Hermes' rate-limit retry. Hermes logged `Unauthorized user: bothy-adapter-control`, so the
stop never reached the turn. Every run started on the thread afterwards ended `failed` with
`runtime_error` within a second, and the answers the still running turn produced found no run
("no active run in this thread"). Cause, read in the adapter and the Hermes source: the
installer wrote Hermes' allow list of the platform with the credential holder alone, and Hermes
judges every sender by it, also the control sender (09 17.7 layer 3). On the busy session
Hermes steered the new run's text into the running turn and admitted no turn for the event,
which `_dispatch` takes as a refused event (`TurnOutcome.FAILED`). Reproduced with the
Fake-Hermes rig against the pinned version (`tests/fake_hermes/test_busy_session.py`).

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-HBS-1 | 2026-11-23 | `switch-mode personal` writes `<KEY>_ALLOWED_USERS` as `<credential holder>,<control sender>`. | Without it Hermes refuses every command of the adapter in `personal`. The control sender is the adapter's own sender: no person's event carries it, so layer 3 still admits exactly the credential holder among persons. |
| D-FIX-HBS-2 | 2026-11-23 | `start_turn` waits until no processing runs on the thread's session (the platform's processing task, then the gateway's running-turn slot, polled every `background_poll_interval`) before `handle_message`. A stop of the run ends the wait. | A run's turn is a turn of its own (17.6). Steering it into a turn without a run gives the run no outcome, and the answer comes when no run is open; waiting keeps the run open and gives the message its own turn and answer. |

Not changed: a late answer of a turn whose run ended while no other run of the thread is open is
still dropped (D-HERMES-CRON-COMMANDS-5). A turn that never ends keeps the waiting run open until
a person stops it; with D-FIX-HBS-1 that stop reaches Hermes.
