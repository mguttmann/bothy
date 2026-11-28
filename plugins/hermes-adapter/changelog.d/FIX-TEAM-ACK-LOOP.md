### Fixed

- A thanks, an "I am on it" or a status in a team channel no longer starts a chain of runs: the
  quiet rules tell Hermes to mention a member only when it has to act on the reply or answer it
  (an answer it does not have to act on goes without the `@`), and the adapter's notice that a
  run had nothing to add to a message that mentioned the agent names the member who wrote it
  without the `@`, so it stays visible to the owner and starts no run.
