### Added

- Files, images, voice and video that Hermes sends with an answer arrive in the thread as
  attachments: each file passes Hermes' own media delivery policy, must fit the server's upload
  limit, goes through an upload session of the channel and then as one text message of the run
  (voice replies too, since the server takes a voice message only after its scan). The adapter
  reports `files_out`.
- Voice input: audio attachments of the triggering message without a transcript are transcribed
  with the profile's own speech to text, the transcript is the person's text and is stored on the
  server as the agent's transcript; without one Hermes' own note reaches the turn. The adapter
  reports `voice_input` while speech to text is on and finds a provider.

### Changed

- An image Hermes names by web address is sent as the address instead of being refused.
