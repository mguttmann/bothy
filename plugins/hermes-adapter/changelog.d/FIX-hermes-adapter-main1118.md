### Fixed

- On Hermes Agent main after the removal of its old session store, a new thread gets its
  automatic title from the Hermes session again. The adapter reads titles and the bind check
  through Hermes' storage interface and closes every handle; older Hermes versions keep working.
- An approval card shows the whole command, so a command longer than Hermes' preview can be
  approved again instead of offering Deny only.
- A question card of the agent says how long the agent waits for the answer. An answer after
  the question expired is no longer taken silently: the run says that the question expired and
  that the answer did not reach the agent.

### Changed

- Tested against Hermes Agent main `f7e2898aaefc` (new pin) and still against `a8b0bc15053c`.
