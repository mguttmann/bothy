### Fixed

- Text the agent writes in front of a tool call (such as "Now I fix the docstring and run the
  tests.") no longer appears as an agent message between the real answers. It shows as the run's
  progress in the live box, the same way as a draft, and only the turn's answer becomes a message.
  This holds with and without draft streaming, and for Hermes' heartbeats and other mid-turn
  notices too.
