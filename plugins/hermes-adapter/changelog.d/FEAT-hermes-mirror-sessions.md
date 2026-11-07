### Added

- Sessions of a Hermes profile that do not start in Bothy (a session another program drives over
  `hermes acp`, a CLI session, a cron job) can show up in Bothy, each as a post of its own in one
  forum, with the prompt marked as the request, the agent's replies and the tools each turn
  called. Off by default: set `hermes_adapter.mirror_channel` and
  `hermes_adapter.mirror_platforms` in the platform section. The posts are read-only, and the
  mirror never holds or fails the session when the server is away.
