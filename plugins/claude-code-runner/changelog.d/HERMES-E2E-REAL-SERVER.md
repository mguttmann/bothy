### Added

- An end-to-end suite runs the runner with Fake-Claude against the server of this tree (`make
  adapters-e2e`, CI job `test-adapters-e2e`): forum posts, mentions with `auto_thread`, replies
  that resume the session, a follow-up into the running turn, answer attachments, voice messages
  with a local transcriber, cancel and retry.
