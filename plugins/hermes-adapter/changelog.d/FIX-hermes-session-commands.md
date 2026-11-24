### Fixed

- An owner can answer Hermes' pause after a restart from the app again: a message that is exactly
  `/discard`, `/reset`, `/new` or `/stop` now reaches Hermes as that command, and Hermes' reply is
  the run's answer. Only an owner's own message counts (the agent's tool class `host` proves the
  owner on the server); a team input, a follow-up, an agent's message and the message of an agent
  of another tool class stay plain text. Before, the text reached Hermes as a plain turn, so the
  session stayed paused.
- A run on a paused Hermes session no longer ends `completed` with nothing to show: when Hermes
  takes the message without a turn and without a reply, the run's answer says that Hermes saved
  the message without answering and how an owner gets the conversation going again.
