### Fixed

- When the adapter's connection to the server fails after the gateway started (for example the
  discovery or the first key fetch), the adapter now reports a retryable error to Hermes, which
  reconnects the platform in the background. Before, the agent stayed offline until the gateway
  was restarted.
- A control command such as `/context` or `/version` that Hermes fails now gets a short notice in
  the thread instead of no reply at all.
- A report or a command reply longer than `hermes_adapter.max_message_bytes` now ends with the
  note `(cut)`, and the gateway log names the cut. Before, the text was cut without a sign.
- A short loss of the server connection at the end of a run (session report, title, an edit of
  the answer, a card's answer) no longer fails a finished run, and any unexpected failure of a run
  now stops the background work the run started.
- A run that never starts (a refused or superseded start) no longer leaves its prepared context
  in the adapter's memory.
- Unexpected failures of the adapter loop and of the adapter's background tasks are logged with
  their stack (files, lines and functions, never the error's text), and a stop of the loop is
  logged as a warning.
- The external job and stage helpers can now be run in a durable install: `install` writes the
  starter `run.py` into the plugin directory, and the README and the notice of a job without a
  run name it.

### Changed

- The README describes the size limit of the turn's context (`hooks.output_spill.max_chars`) and
  recommends running the gateway under a supervisor.
