### Fixed

- CI: the client tests that send a long draft, a long post of a forum or a voice message wait
  for the message in the outbox instead of a fixed number of settle rounds. The send first
  writes a file with real IO, which a loaded runner did not always finish in time, so these
  tests failed once and passed on their rerun (issue #238).
