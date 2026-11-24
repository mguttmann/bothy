### Fixed

- A run no longer ends `failed` with `runtime_error` at once when the thread's Hermes session is
  still busy, for example with a turn that a stop did not end or one in a long rate-limit retry.
  The run's turn now starts when the session is free; until then the run stays open, and the
  person's message gets its own answer.
- A stop now reaches Hermes in `personal` mode: `switch-mode personal` writes the adapter's
  control sender into Hermes' allow list next to the credential holder. Before, Hermes refused
  the control sender's `/stop`, `/new`, `/resume` and control commands, so a stopped run's turn
  went on. A profile switched before needs `switch-mode personal` again and a gateway restart.
