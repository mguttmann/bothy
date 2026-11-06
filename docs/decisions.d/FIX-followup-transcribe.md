# FIX-followup-transcribe Voice follow-ups reach a running Claude Code turn as text

Owner report: a voice message sent to a Claude Code thread while a run was open reached the
agent without a transcript. Read in the live runner log: `followup_written` for the message and
no `attachment_transcribed` or `attachment_not_transcribed`; the file stayed at transcript status
`none`. Cause in the code: `RunTurn.run` transcribed only the attachments of the trigger
(`_transcribe` before the first input), and `offer_followup` passed a follow-up's attachments
only as file entries; 09 18.14 point 1 said follow-ups are not transcribed. Long runs with
background work keep the turn open for a long time, so this was frequent. Normative text: 09
18.5 (attachments of a follow-up) and 18.14 points 1 to 4.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-FIX-FT-1 | 2026-11-06 | An `audio/*` attachment of a follow-up that the runner stored is transcribed exactly like one of the trigger: the same transcriber, status line, `submitTranscript` with the `ETag` of `getFile`, and the same note in the thread when there is no transcript. Place: `RunTurn._transcribe_items`, shared with `_transcribe`. | One path for both, so the follow-up gets what the trigger gets. |
| D-FIX-FT-2 | 2026-11-06 | The files of a follow-up are stored and transcribed after the wait for its files and before the write, without holding the write lock (`_prepare_followup`); a stop cancels the transcriber and the follow-up is not written. | The lock also guards closing stdin; a transcription can take up to `runner.transcribe_timeout`. |
| D-FIX-FT-3 | 2026-11-06 | Follow-ups are written in the order they were offered: each waits for the one offered before it to be written or left (`_FollowupOrder.tail`). A redelivery of a follow-up that is being prepared is left; the first offer writes it. | Without the order a text sent after a voice message would overtake it while the voice message is transcribed. |
| D-FIX-FT-4 | 2026-11-06 | The transcript stands in the follow-up's line after the text as `Transcript of attachment "<name>":` and the text, before the `Attachments:` list; a `not transcribed: <code>` line stays indented under its file in the list. | The same form as the trigger part of the context (`bothy_sdk.context`), inline because a follow-up comes from the run's triggering person. |
| D-FIX-FT-5 | 2026-11-06 | Every audio attachment of a follow-up is given to the local transcriber, also one whose server transcript is `done` or `pending` (mode `service`); the server refuses that `submitTranscript` and the refusal is only logged. | `getMessage` carries no transcript text, so without the local run the turn would get only the file. Reading the server transcript of a follow-up is not built (no reported case). |

Not changed: the Hermes adapter, which has its own rules for the attachments of follow-ups (09
17, D-FIX-HIMG-3), and attachments of context messages.
