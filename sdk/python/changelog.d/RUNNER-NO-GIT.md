### Added

- `RunRefused(reason, summary=...)`: the summary is sent with `finishRun` and names the cause of
  the refusal in the thread.

### Fixed

- An unexpected error of a handler in `prepare` or `execute` ends the run `adapter_error` with a
  summary that names the kind of the error (never its message), instead of no cause at all.
