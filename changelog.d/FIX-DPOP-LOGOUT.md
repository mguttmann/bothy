### Fixed

- Client: the apps no longer log out when a token refresh arrives at the server with an expired
  device proof, for example after the computer slept while the refresh was underway. The app
  signs the proof again and retries once; if that fails too, it keeps the session and tries
  again later. A refresh request now also times out after a short while instead of waiting
  through sleep.
