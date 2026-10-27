### Fixed

- `TestDevicePairing/TestPairingCodeNeverLogged` no longer fails when the six digits of the
  device verification code occur by chance inside the random hex of a key ID, an HMAC or a UUID
  in an audit entry or a log line (main run 40491299995, code `137420`). The test now looks for
  each code as a whole token (no letter or digit before or after it); a code stored as a value,
  an attribute or plain text still fails the test.
