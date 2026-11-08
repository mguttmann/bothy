### Changed

- A run whose turn completed now says so in its collapsed line: "Run ended; nothing is waiting,
  so this thread continues only when someone sends a message" (`run_ended_idle`). Before, a turn
  that promised to go on later while no background task ran ended without a sign, and nothing
  woke the thread again.
- A restart or stop of the runner without `runner.turn_handover` now names in the thread how
  many background tasks it ended with a run's turn; their results are lost.
