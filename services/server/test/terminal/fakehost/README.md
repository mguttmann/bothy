# Package `test/terminal/fakehost`

Status: built (M2-44). Component `testsupport`, never imported by production code.

The Fake-Host of the terminal tests (PLAN, row "Test doubles"; M2-44 Design note 13):

- `Host` (`Connect`): the control connection of an active host through `adapters/hosttest`
  (login with the Ed25519 key of the host, `report`, the heartbeat loop) and one data connection
  per `session.open`, run by a script per session: `Accept` (message 1 is the session token,
  verified against the key set of `hello` and `heartbeat_ack`: signature, `typ`, `aud`, `sid`,
  `exp`, a `jti` never seen; message 2; `session.started` on the next device frame; then echo),
  `Pair` (like `Accept` without `session.started`), `Refuse` (close with a code and a
  `close_detail`), `End` (`session.ended`), `EndAfterHandshake` (the close order of the host
  service after message 2, M2-45: message 2, then on the first device frame `session.ended` with a
  reason and detail while the data connection stays open until the server closes it), `Ignore`
  (no data connection) and `Stall` (connect, never read). Every op and frame is recorded, and each
  data connection keeps the `hello` of the server.
- `DialDevice`: the device stream with the access token and a DPoP proof of a harness client
  (`harness.Client.Proof`), or a deliberately broken `identify` for the negative tests.
- `Timers`: the fake of `terminal/app.Timers` that the tests fire, so no test sleeps for
  `terminal.open_timeout`.

Every text frame in both directions passes `api/specschema.ValidateChannelFrame` (G9).

What the fakes are not: the device sends the compact session token as message 1, a test framing
only. There is no Noise, no PTY and no handshake; the relay forwards opaque bytes and cannot tell
the difference. Neither fake replaces the real host service `bothy-host` (M2-45) or the Go test
device of the conformance and chaos suite (M2-48), which prove the handshake with real Noise.
