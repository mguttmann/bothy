### Fixed

- CI: the wire test of the default operator agent that gets a new channel live stops reading at
  the message. It read the agent's frames until its three-minute patience ran out on every run,
  since the agent never acknowledges and the server redelivers, and only then started its control
  part, which failed once with a 4000 close of the dial (issue #238).
