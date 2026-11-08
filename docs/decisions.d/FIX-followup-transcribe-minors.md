# FIX-followup-transcribe-minors Review minors of the follow-up transcription (issue 574)

The review of FIX-followup-transcribe (pull request 571) left six minors in the follow-up path
of the Claude Code runner. Each was checked against `main` first; all six were still present
and each behaviour has a scenario test in
`plugins/claude-code-runner/tests/scenarios/test_voice.py` (the stdin rule also a unit test in
`tests/unit/test_processstate.py`). Normative text: 09 18.5 and 18.14 points 2 and 4.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-FTM-1 | 2026-11-08 | The transcriber of a follow-up is cancelled when no follow-up can be written any more: a stop, the server ended the run (`server_ended` or an ended patch), or the run ended (phase `ended`). The phase `closed` alone does not cancel it. The transcription of the trigger is still cancelled by a stop only. | A run that ended without a stop let the transcriber run until `runner.transcribe_timeout`. `closed` also lies between the processes of a recovery, whose next process takes the held follow-up with its transcript (D-FIX-FTM-3); a cancel there would only transcribe it again. |
| D-FIX-FTM-2 | 2026-11-08 | Follow-ups offered to the open turn and not yet written or left (`_FollowupOrder.offered`) keep stdin open: `ProcessState.decide` returns no close while one is in flight, at the `result` and after `runner.followup_grace`. A failed turn still closes stdin at its error `result`. | A transcription longer than the window closed stdin under the follow-up, which then became a run of its own. The wait is bounded by `runner.followup_file_wait` and `runner.transcribe_timeout` per file. |
| D-FIX-FTM-3 | 2026-11-08 | Held follow-ups are offered in tasks beside the read of the new process (`_offer_held`), with their places in the order taken before any other follow-up can come. A follow-up prepared and then held between the processes of a recovery keeps its prepared files and transcripts (`_FollowupOrder.prepared`) and is not transcribed again. | Offered inline, a transcription delayed the read of the turn's output; a held follow-up was prepared once per process, so its transcript was produced and submitted twice. |
| D-FIX-FTM-4 | 2026-11-08 | A redelivery of a follow-up that is being prepared waits for the follow-up before it before it releases its place. | It released its place at once, so a later follow-up could overtake the one still being transcribed. The case needs a second invocation of the same message while the first is prepared; the inbox of the SDK makes that unlikely, so the test calls the turn directly. |
| D-FIX-FTM-5 | 2026-11-08 | The note "Voice message not transcribed" of a follow-up is posted after its line is written; a follow-up that is not written gets none (its own run transcribes it again and notes it there). The status line "Transcribing a voice message" is set by the first running transcription of the run and removed when the last one ends (`RunTurn.transcribing`). | The note came during the preparation, also for a follow-up that was then deferred; two parallel transcriptions removed each other's status line. |
| D-FIX-FTM-6 | 2026-11-08 | The lines of 09 18.5, 18.14 and the runner README that pull request 571 added longer than 100 characters are wrapped. Older long lines (tables) are left as they are. | The Markdown style of the repository. |

D-FIX-FT-1 (same note as the trigger) and D-FIX-FT-3 (a redelivery is left) stay; D-FIX-FTM-5
and D-FIX-FTM-4 add when the note is posted and how the redelivery keeps the order.
