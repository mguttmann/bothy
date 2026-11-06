### Fixed

- A run that holds Hermes' background work no longer stays `running` for good when the turn
  Hermes starts with a result fails (for example the model refuses the request). The adapter now
  sees the end of every Hermes turn of the thread, also one that ends with an error: such a run
  ends `failed` with Hermes' error text as a message of the run, and a follow-up written
  meanwhile gets its turn in the follow-on run instead of waiting for a stop. A result turn that
  starts a moment before the run begins to hold its work is now covered as well.
