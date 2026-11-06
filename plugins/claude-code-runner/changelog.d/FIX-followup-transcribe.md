### Fixed

- A voice message sent as a follow-up into a running Claude Code turn reaches the turn with its
  transcript. The runner transcribed only the audio attachments of the message that started a
  run; a follow-up's audio came through as a file path alone, and the person had no way to get
  its content to the agent. Now an audio attachment of a follow-up goes through
  `runner.transcribe_cmd` before the follow-up is written, the transcript stands in the
  follow-up's line after its text and is stored on the server with `submitTranscript`; a failed
  transcription posts the note in the thread and the follow-up is written with its file. A stop
  ends a running transcription, and a later follow-up waits for the one being transcribed, so
  follow-ups keep their order (09 18.5, 18.14).
  The token of a follow-up is judged when the turn takes it up, so a long transcription no
  longer expires the voice follow-up or the follow-ups behind it, and a follow-up that fails
  its checks is neither stored nor transcribed.
