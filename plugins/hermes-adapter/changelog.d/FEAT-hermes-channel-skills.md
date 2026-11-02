### Added

- Channel skills and prompts (FEAT-hermes-channel-skills): `channel_skill_bindings` and
  `channel_prompts` of the platform section apply with Hermes' Discord semantics, keyed by Bothy
  channel or thread ID; the bound skills are loaded at the start of a thread's session, and the
  channel prompt is appended to the server's context.
- Thread actions: `python -m bothy_hermes_adapter thread close|reopen|title` and
  `bothy_hermes_adapter.threads.thread_action` set a thread's title, mark it done and reopen it
  as the profile's agent.
