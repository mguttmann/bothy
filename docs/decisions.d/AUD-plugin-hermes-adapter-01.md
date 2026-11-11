# AUD-plugin-hermes-adapter-01 Card answers and voice follow-ups of a Hermes run

Three findings of the audit of 2026-11-11 against the Hermes adapter (`fn-hermes-adapter-1` to
`fn-hermes-adapter-3`), all in `plugins/hermes-adapter/src/bothy_hermes_adapter/handler.py` and all
on the path of a run that a person answers or adds to while it is open. Each was reproduced
against the server of this tree before the change (`tests/realserver`): after a click on a card
the run document carried no token counts, a typed answer to a card ended `deferred` and so
triggered a follow-on run, and a voice message sent during a run was steered as an empty text.
Normative text: 09 section 7, 17.3 (row "Interaction"), 17.5, 17.6 and 17.12 points 4 and 5.

| No | Date | Ruling | Reason |
|---|---|---|---|
| AUD-HA1/1 | 2026-11-11 | The handler of an answer to a card (`on_interaction`, and `on_followup` for a typed answer) returns only when the run's `execute` returned, which is after the run's own `finishRun`, or when the run waits for its next card (`_until_settled`). The hold of background work no longer releases it: `_hold` does not set the turn's `settled` event, and `_until_settled` no longer returns for a turn that is held or about to end. | The SDK ends a resumed run `completed`, without metrics and costs, when the handler of the answer returns while the run neither ended nor waits (`bothy_sdk.adapter`, the end of handler work). The hold released the handler at its start, and the platform reads Hermes' registries in a worker thread right after, so the SDK's end won: no costs, no token counts, no title, and with a delegation or a background process open the run ended at once (09 17.12, R-AG-44). A card shown by a turn during the hold ended the run the same way. Waiting for `execute` itself, not for a flag set before the end goes out, leaves no window between the two. |
| AUD-HA1/2 | 2026-11-11 | A typed answer to an open card is acknowledged by what Hermes took: the text that resolved a clarify, and at `steer` the text steered into the blocked turn in front of a refused approval or confirmation, is `delivered` at the handover and `absorbed` when the turn completed, the same way as a steered follow-up (`_answer_card`, the turn's `steered` list); the answer to a card that a turn showed while the run holds work is `absorbed` when the run completes. An answer Hermes did not take stays `queued` for the follow-on run of 09 section 7: at `queue` the text to an approval or a confirmation, and an answer no prompt waited for. | The server defers every follow-up left `queued` or `delivered` at the end of the run and starts a follow-on run with it, so an answer the turn took was processed twice. At `queue` no follow-up enters a turn (17.5), so that text has not been processed and the follow-on run is the intended way. Absorbing at the end of the turn, not at the handover, keeps one rule for every follow-up Hermes takes: after a failed turn it is a new request. |
| AUD-HA1/3 | 2026-11-11 | A follow-up with an attachment of any type, audio too, is never passed into a turn of the running run (not steered, no turn of its own during the hold) and never acknowledged; the follow-on run takes it as its triggering message, where its files reach Hermes and its audio is transcribed. As the answer to an open card it only releases the card: a clarify gets its text (an empty one is Hermes' form of no answer), an approval is denied, a confirmation cancelled, nothing is steered and nothing acknowledged. This removes the `audio/*` exception of D-FIX-HIMG-3. | A voice message has no text and follow-ups are not transcribed, so it reached the turn as an empty text and was acknowledged: no run carried what the person said. Audio beside a text lost the audio the same way. One rule for every file is the rule of D-FIX-HIMG-3 without its exception. With AUD-HA1/2 an acknowledged card answer with a file would lose the file, so such an answer is not acknowledged, and its text is not steered either, or the follow-on run would bring it a second time. |
| AUD-HA1/4 | 2026-11-11 | Not built: transcribing the audio of a follow-up before it is steered, as the Claude Code runner does (FIX-followup-transcribe). | It needs the order of follow-ups behind a running transcription, the token's age at the handover and a note path for a failed transcription (six rulings in the runner). The follow-on run already transcribes the message as its trigger, so nothing is lost; the cost is that a voice message waits for the end of the run. |
| AUD-HA1/5 | 2026-11-11 | The fake runtime of the unit and real-server tests gives the event loop away in `background_work` (`tests/adapter_kit.py`), as the platform does when it reads Hermes' registries in a worker thread. | With a fake that answered at once the run's own end always went out first, so no test without Hermes could show AUD-HA1/1. |
| AUD-HA1/6 | 2026-11-11 | `execute` does not return while a stop of its run is being carried out by the handler of a card answer: when the turn was dropped by that handler, it waits until the turn is interrupted and its work cancelled (`_Turn.abandoned`, set at the end of `_abandon`). | Since the resume the SDK's task of the run is the handler of the answer, so a stop reaches that handler, which ends the run `cancelled` after it stopped the work. With AUD-HA1/1 that handler now lives as long as the run, also through the hold, so every stop of a resumed run takes this way. `execute` saw the dropped turn and returned at once, and the SDK ended the run `completed` whenever stopping the work took longer than that (a `/stop` through Hermes and killed processes do): a stopped run showed `completed`, and its open follow-ups were deferred into a follow-on run instead of discarded (09 section 7). Reproduced with a fake runtime that stops the work slower than the hold polls, for a held run and for a turn still running, which had this race before this change. |

Evidence, each test failed before the change (of the card answer with a file, the approval case):

- AUD-HA1/1: `tests/realserver/test_e2e_flows.py::test_run_of_a_clicked_card_ends_with_its_usage`
  (the server of this tree), `tests/fake_hermes/test_background_work.py::test_clicked_card_then_delegation_keeps_the_run_and_ends_with_costs`
  (the pinned Hermes), `tests/unit/test_handler_runs.py::test_an_answered_card_ends_the_run_with_usage_costs_and_title`,
  `tests/unit/test_background_work.py::test_run_of_an_answered_card_holds_its_background_work` and
  `test_card_of_a_followup_turn_keeps_the_held_run_open`.
- AUD-HA1/6: `tests/unit/test_background_work.py::test_stop_of_a_run_resumed_by_a_card_answer_ends_it_cancelled`
  (it fails without the wait in `execute`; with the fake runtime only, no test with Hermes).
- AUD-HA1/2: `tests/realserver/test_e2e_flows.py::test_typed_answer_to_a_card_starts_no_second_run`,
  `tests/fake_hermes/test_interaction.py::test_typed_answer_to_a_card_is_absorbed`,
  `tests/unit/test_handler_runs.py::test_followup_answers_each_card_kind`.
- AUD-HA1/3: `tests/realserver/test_e2e_flows.py::test_voice_followup_is_left_to_the_follow_on_run`,
  `tests/fake_hermes/test_interaction.py::test_voice_followup_is_not_steered_into_the_turn`,
  `tests/unit/test_media.py::test_followup_with_a_file_is_left_to_the_follow_on_run`,
  `tests/unit/test_background_work.py::test_voice_followup_while_held_gets_no_turn_of_its_own`,
  `tests/unit/test_handler_runs.py::test_card_answer_with_a_file_is_left_to_the_follow_on_run`.

## Open minors

- Read in the code, not run: a stop of a run whose resumed handler already returned because the
  run waits for a second card is ended by the SDK itself (`Adapter._on_cancel` finds no running
  task), so the adapter's interrupt of the Hermes turn does not run for it.
- Read in the code, not run: a card answer whose message the server does not hand out
  (`getMessage` fails) answers nothing, and Hermes' prompt stays open until Hermes' own timeout.
