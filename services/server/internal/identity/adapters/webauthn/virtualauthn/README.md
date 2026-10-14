# identity/adapters/webauthn/virtualauthn

A software WebAuthn authenticator for tests (M1-40; also for M1-71 and the console tests). An
`Authenticator` holds one key in memory (ES256, EdDSA, RS256, or ES384 to test a refused
algorithm) and answers creation and request options with the JSON a browser sends:
`clientDataJSON` with type, challenge and origin, authenticator data with the RP ID hash, the flags
and the counter, attestation format `none`, ECDSA signatures in ASN.1 DER.

Knobs (exported fields, changed between calls): `Origin`, `RPID`, `NoUserVerification`, `Counter`
and `FreezeCounter`, `BackupEligible` and `BackupState`, `Discoverable` (the assertion carries the
user handle), `SignWith` (another key signs), `Reuse` (a second `Create` returns the same
credential). It never touches the network or a real authenticator and is used only by tests.
