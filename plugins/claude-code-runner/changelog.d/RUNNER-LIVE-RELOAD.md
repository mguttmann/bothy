### Added

- Optional transcription of voice messages and audio attachments (09 18.14): with
  `runner.transcribe_cmd` set, every audio attachment of the triggering message without a
  server transcript is given to that local program (file path appended, no shell, time limit
  `runner.transcribe_timeout`); its stdout becomes the transcript in the trigger part and is
  delivered with `submitTranscript`. Without a transcriber, or when it fails, the thread gets a
  note that Claude Code received only the audio file. `capabilities.voice_input` is true exactly
  when a transcriber is configured.
- Configuration keys `runner.channels_resync_interval` (default `5m`),
  `runner.transcribe_cmd` (default empty, off) and `runner.transcribe_timeout` (default `5m`).

### Fixed

- Operator and binding changes reach a running runner (09 18.1): `access.changed` and the
  channel and space structure events reload the served channels, a timer reloads them every
  `runner.channels_resync_interval` as a fallback, and a changed readiness is reported at once.
  Before, a forum that got the runner as operator stayed "not ready" until a restart.
