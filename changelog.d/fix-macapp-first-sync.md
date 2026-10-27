### Fixed

- A device after its first sync names the agents (thread header, running view) instead of
  showing their IDs: the full snapshot reads every agent the local data does not name yet.
- The main history of a text channel loads the channel's newest threads when it opens, so a
  new device shows the thread lines and can open the threads created before its first sync.
