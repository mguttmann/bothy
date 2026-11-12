### Added

- Configuration keys `run.status_line_max_chars` (default `500`) and `run.summary_max_chars`
  (default `2000`): the status line of a run (`patchRun`, the gateway op `run.update` and
  `awaitRunInput`) and the summary of `finishRun` keep their first characters up to these
  lengths. A longer text is shortened, not refused.

### Fixed

- A long status line of an adapter no longer keeps a run from ending. A status line above
  `sync.event_payload_max_bytes` was stored whole; from then on `finishRun`, `awaitRunInput`
  and `disableAgent` answered `500 internal_error` for that run, the cancel timeout and the
  stall check failed, and its thread stayed busy. A `finishRun` with a summary of that length
  failed the same way. A run that already holds such a status line can end again: its
  `run.state_changed` goes out without `status_line` and `summary` and with `truncated: true`
  in its envelope, and `getRun` returns the texts.
