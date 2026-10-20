# Package `test/terminal/fakehost`

Status: built (M2-44; Noise and `ControlAttempt` M2-46). Component `testsupport`, never imported
by production code.

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
  (no data connection), `Stall` (connect, never read) and `NoiseAccept` (M2-46, below). Every op
  and frame is recorded, and each data connection keeps the `hello` of the server.
- `DialDevice`: the device stream with the access token and a DPoP proof of a harness client
  (`harness.Client.Proof`), or a deliberately broken `identify` for the negative tests;
  `DeviceParams.ClientIP` sends `X-Forwarded-For`, so the stream comes from another client address
  (the harness trusts the loopback peer as its reverse proxy).
- Noise (`noise.go`, M2-46 Design note 13): `NewNoiseKey` returns the login key and a real static
  X25519 key pair (its public half is the `noise_public_key` of `pairHost`; `NewKey` stays for the
  tests without a handshake), `Host.UseNoise` gives it to the Fake-Host, and the script
  `NoiseAccept` runs the responder of `Noise_NK_25519_ChaChaPoly_SHA256` with the prologue of 24
  section 5.2: message 1 carries the session token (verified like `Accept`, plus its claim `cek`
  against the initiator's ephemeral key), message 2 carries `{"v":1,...}`, the frame `start`
  triggers `session.started`, every `data` frame is answered with `Script.Output` (or echoed), and
  `close` sends `session.ended` with `client_closed`. `NewEphemeral` gives the device key whose
  `Hash` is the `client_ephemeral_key_hash`; `Device.Handshake` runs the initiator and returns a
  `NoiseStream` (`Send`, `RecvFrame`); `StartFrame`, `DataFrame`, `ResizeFrame` and `CloseFrame`
  encode the frames of 24 section 5.4. `Host.Plain` returns the frames the host decrypted, and
  `Wire` of `DataConn` and `Device` returns every raw binary message sent and received (what
  passed the relay). The library is `github.com/flynn/noise` at the version `services/host/go.mod`
  pins, a test-only dependency of the server module, because `services/host/internal/noise` cannot
  be imported from another module.
- `ControlAttempt`: one control connection from a client address of choice with a valid or a
  given proof, returning `ready` or the close code (the rate-limit tests of M2-46).
- `Timers`: the fake of `terminal/app.Timers` that the tests fire, so no test sleeps for
  `terminal.open_timeout`.

Every text frame in both directions passes `api/specschema.ValidateChannelFrame` (G9).

What the fakes are not: with the scripts other than `NoiseAccept` the device sends the compact
session token as message 1, a test framing only, without Noise; the relay forwards opaque bytes and
cannot tell the difference. There is no PTY. Neither fake replaces the real host service
`bothy-host` (M2-45) or the Go test device of the conformance and chaos suite (M2-48).
