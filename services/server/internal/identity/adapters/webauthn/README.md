# identity/adapters/webauthn

`Verifier` implements `app.PasskeyVerifier` with go-webauthn
([github.com/go-webauthn/webauthn](https://github.com/go-webauthn/webauthn), BSD-3-Clause, pinned
`v0.18.2` in `services/server/go.mod`). It is the only package of the server that imports the
library (vendor group `webauthn` in `architecture/modules.yaml`, M1-40).

- Relying party: RP ID and origins from `domain.ResolveRelyingParty` (`auth.webauthn_rp_id`,
  `auth.webauthn_origins`, defaults from `server.public_url`); http(s) origins go to `RPOrigins`,
  app origins such as `android:apk-key-hash:...` to `RPOpaqueOrigins`. The display name is the RP
  ID (docs/DECISIONS.md M1-40/2).
- Fixed policy (docs/spec/08-identitaet.md "2.5"): `userVerification = required`,
  `residentKey = preferred`, `attestation = none`, algorithms ES256 (-7), EdDSA (-8) and RS256
  (-257) only.
- No state: `CreationOptions` and `RequestOptions` put the challenge the use case stored as HMAC
  into the options; `VerifyCreation` and `VerifyAssertion` rebuild the session data from the
  checked challenge, the RP ID and required user verification. `VerifyAssertion` resolves the
  person through the lookup of the use case (allow list, revocation, person) and returns its error
  unchanged.
- Errors: every refusal of the library is wrapped in `app.ErrPasskeyRejected`; the library text
  stays in the chain for the log and never reaches an answer.

Tests: `webauthn_test.go` with the virtual authenticator of `virtualauthn` (round trip, every
refusal, the algorithms).
