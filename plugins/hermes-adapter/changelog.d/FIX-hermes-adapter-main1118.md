### Fixed

- On Hermes Agent main after the removal of its old session store, a new thread gets its
  automatic title from the Hermes session again. The adapter reads titles and the bind check
  through Hermes' storage interface and closes every handle; older Hermes versions keep working.
- An approval card shows the whole command when it fits into one message, so a command longer
  than Hermes' preview can be approved again instead of offering Deny only; a command too long
  for the message is still Deny only.
- A question card of the agent says how long the agent waits for the answer. An answer after
  the question expired is not taken by Hermes, and while the run is open the adapter posts a
  message that the question expired and the answer did not reach the agent.
- On Hermes Agent main a message of a person Hermes' allow list refuses no longer leaves the
  run open: it ends as failed.

### Changed

- Tested against Hermes Agent main `f7e2898aaefc` (new pin) and still against `a8b0bc15053c`.
