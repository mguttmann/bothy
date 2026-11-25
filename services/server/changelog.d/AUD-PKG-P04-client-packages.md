### Changed

- A thread move writes `thread.moved` before `thread.moved_out` (consecutive `seq`), so a client
  that sees both channels keeps the thread and its messages; webhook receivers of both events see
  the same order. Moves written before this change keep the old order in the event log.
