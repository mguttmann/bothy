### Changed

- The pinned Claude Code version is 2.1.288 (was 2.1.259, which the API no longer serves current
  models to). The fixtures, the stream field recording and the captured API errors name and come
  from 2.1.288.

### Fixed

- A turn the API refuses because Claude Code is older than the model's minimum ends
  `runtime_error` with a box note that names the required version and says to update Claude
  Code; the runner reports `degraded` with `runtime_version_unsupported` at once and refuses
  further runs until a version probe finds a CLI at that version.
- `runner.default_model` and a binding's model in the CLI's notation with a context window
  (`<model>[1m]`) no longer fail `startRun` with `400 validation_failed`: the CLI gets the value
  unchanged, the server gets the model name without the suffix.
