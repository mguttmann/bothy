### Fixed

- Background mode: a follow-up written into a running turn is no longer held by the host
  user's own `crossSessionInbound` setting. With any value of that setting in the host user's
  Claude Code `settings.json`, the settings file of a run now sets `accept`, which goes before
  it; without a value the file sets none, as before. On a host whose settings held `hold`,
  every follow-up was held, stayed `delivered`, and the server deferred it into runs of their
  own and offered it to each later run again, so the same message was answered several
  times. On such a host a runner session now takes every message of every sender, and the
  runner logs each one it did not write.
