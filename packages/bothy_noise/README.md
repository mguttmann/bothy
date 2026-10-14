# bothy_noise

Pure Dart package with the end-to-end layer of the terminal
([24 Terminal](../../docs/spec/24-terminal.md), sections 4.3, 5.2 to 5.4): handshake
`Noise_NK_25519_ChaChaPoly_SHA256` as initiator, transport, prologue, frames and fingerprint.
Desktop app, mobile app and web console use the same code.

## Guarantees

- No `package:flutter`, no `dart:io`, no `dart:html`, no network and no storage in `lib/`;
  the only dependency is `cryptography` (X25519, ChaCha20-Poly1305, SHA-256, HMAC)
  ([15 Client architecture](../../docs/spec/15-client-architecture.md), 3.1 rule 7 and 23.1).
- The same bytes as the host service in Go: the public test vectors (TM-T01) and the
  recordings of both languages in both roles (TM-T02) under
  [`services/host/testdata/noise/`](../../services/host/testdata/noise/README.md).
- A wrong prologue, a wrong static key or swapped session and host IDs make the handshake
  fail. After a failed step or a failed decryption every further call throws
  `NoiseError.broken`.
- Error texts name kind, operation and lengths, never payload, plaintext, keys or tokens.
- `encrypt` takes its nonce at the call; send the messages in the order of the calls.

## Libraries

| Import | Content |
|---|---|
| `package:bothy_noise/bothy_noise.dart` | `NkInitiator`, `NkResult`, `NoiseTransport`, `uuidToBytes`, `buildPrologue`, `generateEphemeralKeyPair`, `ephemeralKeyHash`, frames (`TerminalFrame`, `encodeFrame`, `decodeFrame`), `crockfordBase32`, `hostKeyFingerprint`, constants, errors |
| `package:bothy_noise/testing.dart` | only `NkResponder`, the host side for tests and for M4-35 as a fake host |

## Flow on the device

1. `generateEphemeralKeyPair()`, then send `ephemeralKeyHash` of the public key as
   `client_ephemeral_key_hash` when opening (15 23.2, step 2).
2. `NkInitiator.start` with the pinned `noise_public_key`, the session and host IDs
   (`uuidToBytes`) and the same key pair; send `writeMessage1(token)`.
3. `readMessage2` returns the payload, the `NoiseTransport` and the handshake hash; after that
   frames with `encodeFrame` and `transport.encrypt`, `StartFrame` first.

Still missing: pin storage, view and renderer (M4-35).

## Commands

```sh
flutter pub get                # once, at the repository root (Dart workspace, M4-01)
cd packages/bothy_noise
flutter analyze --fatal-infos
flutter test --coverage
UPDATE_RECORDINGS=1 flutter test test/recordings_test.dart   # rewrite recordings/dart.json
```

Since M4-01 the package is a member of the Dart workspace at the repository root: one
`pubspec.lock` there, and Flutter comes from `environment.flutter` of the root `pubspec.yaml`
(docs/DECISIONS.md D-M4-01-1, D-M4-01-2). The package uses Flutter only as its test runner.
Allowed dependencies and the checks of `make arch`: [tools/arch-check](../../tools/arch-check/README.md)
(no Flutter, no `dart:io` in `lib/`, `cryptography` as the only runtime dependency; 15 3.1
rule 7).
