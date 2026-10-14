# Noise test data

Shared test data of the host service (`services/host/internal/noise`,
`services/host/internal/fingerprint`) and of the Dart package `packages/bothy_noise`
([24 Terminal](../../../../docs/spec/24-terminal.md), sections 4.3, 5.2 and 14). None of it is
shipped; the files contain only public test keys.

## Test vectors (TM-T01)

Both files are stored unchanged, but compressed with `gzip -n -9`, under `vectors/`. Reason
(docs/DECISIONS.md D-M2-38-15): the compression keeps the content byte for byte below
`repo.max_file_kb`, protects it from the hygiene hooks and keeps the published test keys out of
the text rules of the secret scanners. The checksum applies to the unpacked content;
`TestVectorProvenance` holds this table against the files.

| File | Source | Commit | License | SHA-256 of the content |
|---|---|---|---|---|
| `vectors/cacophony.txt.gz` | https://github.com/haskell-cryptography/cacophony, path `vectors/cacophony.txt` (formerly `centromere/cacophony`, GitHub redirects) | `8ee9d41e34a1a596cfa3ab12aa4069ff87dc1247` | `Unlicense` | `3bde7c09a6f349ee11c825c50fcc02649f8f02a47c857a459206b357f9386cae` |
| `vectors/noise-c-basic.txt.gz` | https://github.com/rweather/noise-c, path `tests/vector/noise-c-basic.txt` | `cfe25410979a87391bb9ac8d4d4bef64e9f268c6` | `MIT` | `e826749cf90efda26be85410381f4b1f75552c61c7c0dcad7f9ffb4d639e4e45` |

License texts: `vectors/LICENSE.cacophony` (Unlicense, dedication to the public domain) and
`vectors/LICENSE.noise-c` (MIT, the project's file `COPYING`). Retrieved on 2026-10-12.

Loading rules in Go and Dart: the protocol of an entry is `protocol_name`, otherwise `name`,
otherwise `Noise_<pattern>_<dh>_<cipher>_<hash>`. Selected are the entries equal to
`Noise_NK_25519_ChaChaPoly_SHA256` without PSK and without `fallback`;
`NoisePSK_NK_25519_ChaChaPoly_SHA256` from noise-c (PSK of an older revision) drops out. If
`resp_prologue` is missing, `init_prologue` applies. The first two messages are the handshake,
the rest is transport, alternating initiator and responder. Each file must yield at least one
entry.

## Recordings (TM-T02)

Go writes `recordings/go.json`, Dart writes `recordings/dart.json`. Each language compares its
own file byte for byte with a fresh generation and replays the other language's file as initiator
and as responder; in addition both check that a wrong prologue, a wrong static key and swapped
session and host IDs fail. Content: three scenarios (`open_shell`, `attach_replay`, `limits`)
with fixed keys (SHA-256 of `noise-recording/<generator>/<scenario>/<role>`), the handshake and
the frames of 24 5.4.

Write them again when the format or the generation changes:

```sh
make -C services/host recordings
(cd packages/bothy_noise && UPDATE_RECORDINGS=1 flutter test test/recordings_test.dart)
```

## Fingerprint vectors

`fingerprint.json` (`make -C services/host recordings`, checked by `TestFingerprintFileCurrent`
in Go and `fingerprint vectors match` in Dart) first holds the known vector of 24 4.3, then eight
entries with the keys SHA-256 of `fingerprint/noise/<i>` and `fingerprint/auth/<i>` for `i` from
1 to 8. Fields: `noise_public` (X25519 key, hex), `ed25519_public` (raw Ed25519 key, hex) and
`fingerprint` (display form). The field names avoid the keywords of the gitleaks rule
`generic-api-key`. The server (M2-41) checks its own computation against the same file.
