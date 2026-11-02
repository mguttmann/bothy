### Fixed

- Images and other files sent to a Hermes agent reach Hermes. The adapter passed only the text
  and the transcripts of voice messages into the turn, so a pasted screenshot arrived as an
  empty message. Now every attachment of the triggering message is placed in Hermes' media
  cache and handed over as Hermes' Discord platform does: images to its vision, other files as
  cached paths with Hermes' own note. A message of attachments only is no longer empty, and an
  attachment that cannot be passed on (scan pending or blocked, too large for Hermes, refused)
  reaches Hermes as a note with the reason.
