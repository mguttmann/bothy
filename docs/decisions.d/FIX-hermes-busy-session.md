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
| D-FIX-HBS-2 | 2026-11-23 | The run's first turn waits (`Runtime.session_free`, called by the handler before the session switch and the turn) until no processing runs on the thread's session: the platform's processing task, then the gateway's running-turn slot, polled every `background_poll_interval`. While it waits the run covers nothing of that session: tools refused, answers, cards and turn ends not the run's, a follow-up left queued (`session_busy`). A stop ends the wait; an abort during it ends the run `failed` with its reason without a turn. | A run's turn is a turn of its own (17.6). Steering it into a turn without a run gives the run no outcome, and the answer comes when no run is open; waiting keeps the run open and gives the message its own turn and answer. |

Not changed: a late answer of a turn whose run ended while no other run of the thread is open is
still dropped (D-HERMES-CRON-COMMANDS-5). A turn that never ends keeps the waiting run open until
a person stops it; with D-FIX-HBS-1 that stop reaches Hermes.

Open minors for the next run in this area (red team of 2026-11-23):

- `check` shows the allow list only as `set` or `empty`; a `personal` profile switched before
  this fix (no control sender on the list) does not stand out.
- Windows of the same class not reproduced: between the wait and the turn's start (a `/new` or
  `/resume` of the session switch), follow-up turns of a run that holds work (`_followup_turn`
  does not wait), and a thread without a known source after a gateway restart (the wait returns
  at once).
- The historical brief `docs/briefs/M2-33-hermes-adapter.md` still says the list holds exactly
  the credential holder (archive, left as is).
- A card of the busy turn during the wait is covered only by the unit test through `_active`.
