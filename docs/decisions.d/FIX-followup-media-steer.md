# FIX-followup-media-steer Follow-ups with files reach a running Hermes turn at once

Owner report of 2026-11-18: a follow-up with an image sent to a Hermes agent during an open run
did not reach the agent until the run ended. The follow-up stayed `queued` without any log line
of the adapter. Cause in the code: `HermesHandler.on_followup` returned for every follow-up with
an attachment (`message.attachments`), so the server's follow-on run took it as its triggering
message after the end of the run (D-FIX-HIMG-3 of 2026-11-02, widened to audio by AUD-HA1/3 of
2026-11-11). Before 2026-11-02 the adapter steered the text of such a follow-up and dropped the
file. Hermes steers text only (`agent.steer`, `_hm_text_only` of its gateway) and gives a
message with media a turn of its own after the current one. The Claude Code runner already
stores the files of a follow-up and names them in the line it writes (09 18.5). Normative text:
09 17.5 (last paragraph), 17.6, 17.12 point 5, the key `hermes_adapter.followup_file_wait`, and
for the app 01 6.3 and components.md section 5.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FMS-1 | 2026-11-18 | A follow-up with files is steered into the running turn at once, at `steer`, when the turn's last model reply called a tool, as a text follow-up is; it is `delivered` at the handover and `absorbed` when the turn completed. Before that the adapter reads the message again every `hermes_adapter.background_poll_interval` until the server's check of every attachment but audio ended, at most `hermes_adapter.followup_file_wait` (default `2m`, as `runner.followup_file_wait`). | The person expects the agent to see what they sent while it works. The server delivers a follow-up when it is stored, also while a file is still checked, and an image stays `pending` until its cleaned original is stored (12 3.4, 12 10), so without the wait a fresh screenshot would never be ready. |
| D-FMS-2 | 2026-11-18 | Every non-audio file of a follow-up takes the trigger's path (`_load_files`, shared with `_media`): the same scan states, Hermes' `gateway.max_inbound_media_bytes`, `downloadFile`, Hermes' own `cache_media_bytes_async` and the inline rule for text files. The steered text is a line per cached file with its path in Hermes' media cache and what the agent can do with it (an image: look at it with its image analysis tool; a video: a video or media tool; any other file: its file tools; a text file within the inline limit: its content is included), then the notes of files not passed on, the inlined texts and the person's text, or Hermes' no-text wording. The path is the host path Hermes' cache returns, as Hermes' own placeholder for queued media names it. | Hermes steers text only, so a path is the one way the running turn reaches a file; Hermes' tools (`vision_analyze` for images, `read_file` for documents) take a local path. Naming the tool by its purpose, not its name, keeps the text right for a profile whose toolsets differ. One loading path keeps one set of trust and size rules. |
| D-FMS-3 | 2026-11-18 | A file that may still come leaves the follow-up `queued`, so the server's follow-on run takes the whole message as its triggering message: audio (transcribed only there, AUD-HA1/4 stands), a file whose check had not ended when the wait ended (`scan_pending`), one the server did not hand out (`unavailable`). The rest of the follow-up is still steered, with a note per such file that the message comes again with it in a new turn; a follow-up with nothing but such files (a voice message without text) is not steered. A file the scan blocked, one over Hermes' limit and one Hermes refused to cache stay notes, and the follow-up is acknowledged. | The person's words reach the agent at once, and nothing is lost: the follow-on run brings the file. The run state is kept per message, not per file, so the message stays open as a whole; the note tells the agent why it sees the text twice. A blocked or too large file would only be a note in the follow-on run as well. |
| D-FMS-4 | 2026-11-18 | Follow-ups of a run are passed on one after the other in the order the adapter got them (`_Turn.followup_order`), so a text sent after a picture waits while the picture is checked and loaded. The answer to an open card keeps its own path (AUD-HA1/2, AUD-HA1/3). | Without the order the later text would overtake the picture (as D-FIX-FT-3 of the runner). |
| D-FMS-5 | 2026-11-18 | While the run holds background work (17.12 point 5), and for a follow-up the turn did not take that later becomes a turn of its own, the follow-up's turn carries its files as media (`TurnRequest.media`) with the notes and inlined texts in its text, as a trigger's turn does; a follow-up with a file that may still come stays `queued`. | A turn of its own is a normal Hermes turn, where Hermes' vision and document notes work as on a trigger. |
| D-FMS-6 | 2026-11-18 | Every follow-up the adapter does not steer gets one log line `run <id>: follow-up <id> not steered code=<reason>` (`not_readable`, `stopped`, `audio`, `scan_pending`, `unavailable`, `own_turn`, `busy_mode_<mode>`, `turn_answered`, `turn_ended`), and one steered but left queued gets `steered, left queued code=<reason>`. IDs and codes only (SEC-LOG-2). | The reported case left no trace in the log. |
| D-FMS-7 | 2026-11-18 | The app shows what became of a follow-up without a new server field: a message sent while a run of the thread was active (after its `queued_at`, before its `finished_at`) is a follow-up of that run. One still `received` while a run is active shows "Wartet auf Ende des Laufs" / "Waiting for the run to end" with the queued icon; one `cancelled` with `user_cancelled` by the stop of that run shows "Verworfen: Lauf gestoppt" / "Discarded: the run was stopped" with the stop icon. Both are display states of `DsTriggerState` the server never sends. The server keeps 09 section 7: after a stop by a person the open follow-ups are `discarded` with `trigger_state` `cancelled` and `user_cancelled`. | `received` had the same look as a received trigger, which read as delivered, and a discarded follow-up read like the stopped trigger. The run summaries the app holds already give the window; a run the local table does not hold leaves the stored state, which is still visible, never silent. |

Evidence (each test fails on the previous handler or app):

- D-FMS-1, D-FMS-2, D-FMS-4: `plugins/hermes-adapter/tests/unit/test_media.py::test_image_followup_is_steered_at_once_with_its_path`,
  `test_followup_with_files_only_is_steered_with_hermes_no_text_wording`,
  `test_followup_waits_for_the_check_of_its_files`, and against the pinned Hermes
  `tests/fake_hermes/test_interaction.py::test_image_followup_is_steered_into_the_turn`.
- D-FMS-3: `test_file_not_ready_after_the_wait_is_left_to_the_follow_on_run`,
  `test_blocked_file_of_a_followup_is_steered_as_a_note`, `test_voice_followup_is_left_to_the_follow_on_run`.
- D-FMS-5: `tests/unit/test_background_work.py::test_image_followup_while_held_is_a_turn_of_its_own_with_the_image`.
- D-FMS-6: `test_followup_at_queue_is_logged_with_its_reason` and the log assertions above.
- D-FMS-7: `apps/client/test/agents/trigger_state_test.dart` (the three follow-up tests) and
  `packages/design_system/test/components/trigger_state_test.dart`.

## Open minors

- Not built: translating the cached path to the path a sandboxed terminal backend sees
  (Hermes' `to_agent_visible_cache_path`, used in its own document notes). Hermes' image tool
  reads host paths; a profile with a Docker or SSH terminal backend may not open a steered
  document with its file tools.
- Not built: the button "Send again" that 09 section 7 names for a follow-up discarded by a
  stop; the app shows the state only.
- A follow-up moved to a follow-on run and still `received` there shows "Waiting for the run to
  end" until that run takes it.
