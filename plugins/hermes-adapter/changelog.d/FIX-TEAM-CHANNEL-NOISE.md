### Fixed

- Status lines of Hermes, such as a memory plugin's "recalled N memories" line, are shown as
  progress of the run and are no longer posted as messages (in a team channel they were mirrored
  for everyone).

### Changed

- In a team channel the agent is told to mention a member only to hand it work, to name members
  it only refers to without the `@`, and to answer `NOTHING_TO_ADD` when it has nothing new to
  contribute, which the server does not show in the team channel.
