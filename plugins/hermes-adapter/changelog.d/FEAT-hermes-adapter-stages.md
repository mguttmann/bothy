### Added

- A long command that writes its progress as stage events (one JSON line per stage start, pass
  and end, as the coding chain of a Hermes profile does in `stages.jsonl`) can now show it in the
  Bothy run. The tool call registers the file with
  `python -m bothy_hermes_adapter stages register`; while the thread's run is open, each stage is
  a step of the run and the status line shows the last event. Lines that do not fit the format
  are skipped, and nothing of a line but its checked words reaches the run.
