# Changelog

All notable changes to the Dart package `bothy_noise` are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this package adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). The package is not published
(`publish_to: none`); it ships inside the apps.

## [Unreleased]

### Added

- Handshake `Noise_NK_25519_ChaChaPoly_SHA256` as initiator (`NkInitiator`) on the primitives of
  `cryptography`, the prologue `"bothy-terminal/1"` || session ID || host ID, the transport
  (`NoiseTransport`) with the limits of the Noise specification and a state that refuses every
  call after a failure.
- `uuidToBytes`, `buildPrologue`, `generateEphemeralKeyPair` and `ephemeralKeyHash` for
  `client_ephemeral_key_hash`.
- The frames `start` to `replay_end` (`TerminalFrame`, `encodeFrame`, `decodeFrame`) with the
  same rules as the host service.
- `crockfordBase32` and `hostKeyFingerprint`.
- `package:bothy_noise/testing.dart` with the responder `NkResponder` for tests and fake hosts.
- Tests against the public vectors of cacophony and noise-c (TM-T01) and the cross test with the
  Go recording in both roles (TM-T02).
