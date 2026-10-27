### Fixed

- Demo agent: a short outage of the server (`429`, `502`, `503`, `504` or no HTTP answer) no
  longer ends the demo agents. `receive`, `heartbeat`, the sign-in and the watcher's live box
  patches wait with a growing backoff and try again, and any other failure ends only the agent it
  hit while the other agents go on.
