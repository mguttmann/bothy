# Changelog

All notable changes to the host service (`services/host`, binary `bothy-host`) are documented in
this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this component adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version
is the [`VERSION`](VERSION) file next to this one; release tags are `host/v<version>`
(docs/spec/18-release.md).

## [Unreleased]

### Added

- `internal/control` (M2-43): the control connection to `wss://<server>/api/v1/hosts/connect`
  with the EdDSA proof of `identify`, `report` first after every `ready`, heartbeats, the key set of
  purpose `terminal`, the reconnect policy per close code with the new keys
  `host_service.reconnect_backoff_base` and `host_service.reconnect_backoff_cap`, and the refused
  start over one connection (one `report`, close `1000`, exit `78`). `run` uses it instead of
  `NoControl` once the host is paired; `startup.Events` passes every later `ready` to the start
  sequence and stops `run` after `4110`, `4112` and `4114`. Recordings
  `testdata/control/host_frames.json` and `server_frames.json`; the fake server `controltest`.
- Go module `github.com/OWNER/bothy/services/host` with `VERSION`, the golangci-lint v2
  configuration and the hand-maintained go-arch-lint rules of the module.
- Binary `bothy-host` with a command table, usage text and the exit codes 0, 1 and 2.
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
- Commands `install`, `session-user add|remove`, `restart`, `pair`, `doctor`, `run` and
  `pty-helper` with the exit codes `0`, `1`, `2` and `78` (refused start) (M2-42).
- The configuration file with strict decoding and the new key `host_service.auth_status_timeout`;
  the state directory with atomic writes and DPAPI on Windows.
- `doctor`: the checks HD1 to HD11 with the report entries by the rule for names, the local
  output, the entry scan per session user and the readiness per session user.
- The local channel to the PTY helpers with its peer check (socketpair over socket activation,
  named pipes on Windows), the admin endpoint and a fuzzed codec.
- Installation plans for Linux, macOS and Windows with the service definitions of the exit policy,
  `session-user add` and `remove`, and the drive root step on Windows.
- `pair` against `pairHost`: local key generation, fingerprint check and storage after
  acceptance.
- A fake `claude` for the tests of HD6 and the platform suite behind the build tag `platform`
  (`make platform-test`, workflow `host-platform.yml`).
