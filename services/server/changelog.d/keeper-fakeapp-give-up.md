### Fixed

- The test fake app (`test/apps/fakeapp`) closes the connection without an answer when the
  client gives up during `Hang` or `Delay`. Before, its handler returned and net/http wrote 200
  with an empty body; a TLS client sends close_notify before it closes its socket, so it could
  still read that 200 as the answer of the request it had cancelled (flaky `TestFakeAppScript`,
  and `TestAppInteractionNoAnswer` logging `invalid_reply` instead of `timeout`).
