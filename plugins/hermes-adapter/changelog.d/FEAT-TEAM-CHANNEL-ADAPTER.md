### Added

- Hermes agents can be members of a team channel. The adapter runs a `team` run like a message
  run in the member's work thread and tells the agent its handle, the other members, who wrote
  the input and whether it was mentioned. A request of another member, also one that arrives
  while the agent works, is marked "von Mitglied @x" and never counts as the owner's word or as
  an approval; the owner's messages are passed on as before. The agent hands work on by
  mentioning a handle in its final answer, and every request of the run carries the run's own
  token, so the server can hand that answer on.
