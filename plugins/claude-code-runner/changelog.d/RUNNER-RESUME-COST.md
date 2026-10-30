### Fixed

- A resumed session no longer books its old costs again. Claude Code reports the totals of the
  whole session on `--resume`; before the first process of a run that resumes the runner now reads the
  totals the CLI restores (the last `cost-state` line of the session transcript) and books only
  what the run added, per model, in amounts and in tokens, never below zero. This covers the
  first run of a session taken over from an import with `--sessions` or started outside the
  runner (it booked the whole old session for one turn) and a run after the session was
  continued outside the runner. No start value in the database is needed any more; a cost
  baseline of the import only applies when the transcript has no totals (09 18.7).
