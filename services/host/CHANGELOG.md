# Changelog

All notable changes to the host service (`services/host`, binary `bothy-host`) are documented in
this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this component adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version
is the [`VERSION`](VERSION) file next to this one; release tags are `host/v<version>`
(docs/spec/18-release.md).

## [Unreleased]

### Added

- Go module `github.com/OWNER/bothy/services/host` with `VERSION`, the golangci-lint v2
  configuration and the hand-maintained go-arch-lint rules of the module.
- Binary `bothy-host` with an empty command table, usage text and the exit codes 0, 1 and 2;
  M2-42 registers the commands.
- `make release-build VERSION=<x.y.z> OUT=<dir>`: unsigned binaries without cgo for darwin/arm64,
  darwin/amd64, linux/amd64, linux/arm64 and windows/amd64; `make recordings` rewrites the
  recordings and fingerprint vectors.
- `internal/noise`: the handshake `Noise_NK_25519_ChaChaPoly_SHA256` over `github.com/flynn/noise`
  (initiator and responder), the prologue `"bothy-terminal/1"` || session ID || host ID, the
  transport with the limits of the Noise specification, the hash of the ephemeral key for the
  claim `cek`, and a state that refuses every call after a failure.
- `internal/frame`: the frames `start` to `replay_end` of docs/spec/24-terminal.md 5.4 with byte
  order, body rules, directions and a fuzzed parser.
- `internal/fingerprint`: the host key fingerprint in Crockford base32 and its display form.
- Test data under `testdata/noise/`: the public test vectors of cacophony and noise-c (TM-T01),
  the recordings of Go and Dart (TM-T02) and the fingerprint vectors.
