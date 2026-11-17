### Fixed

- Background mode: a follow-up written into a running turn is no longer held by the host
  user's own `crossSessionInbound` setting. When that setting is `hold` in the host user's
  Claude Code `settings.json`, the settings file of a run now sets `accept`, which goes before
  it; with any other value or none the file sets none, as before, so a `refuse` there is
  never overridden. On a host whose settings held `hold`,
  every follow-up was held, stayed `delivered`, and the server deferred it into runs of their
  own and offered it to each later run again, so the same message was answered several
  times. On such a host a runner session now takes every message of every sender, and the
  runner logs each one it did not write.
