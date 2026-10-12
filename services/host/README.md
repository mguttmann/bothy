# services/host

Host service `bothy-host`: its own Go binary on connected hosts for the terminal.
Normative: [24 Terminal](../../docs/spec/24-terminal.md),
[ADR 0068](../../docs/adr/0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md).

It runs as a user without administrator permissions, connects to the server only outbound, is the
Noise peer of the device and starts one PTY per terminal session (macOS, Linux, Windows through
ConPTY).

## Status

With M2-38 there is the Go module `github.com/OWNER/bothy/services/host` with the Noise core. The
binary has no commands yet; M2-42 adds `install`, `pair`, `doctor`, `run`, `restart`,
`session-user` and `pty-helper`, the key storage and the local channel. M2-43 builds the control
and data connections, M2-45 the check of the terminal session token and the PTY.

## Layout

| Path | Content |
|---|---|
| `cmd/bothy-host/` | command dispatcher without commands (usage, exit codes 0, 1 and 2) |
| `internal/noise/` | handshake `Noise_NK_25519_ChaChaPoly_SHA256` over `github.com/flynn/noise`, prologue, transport with the limits of the Noise specification, hash of the ephemeral key for `cek` (24 5.2, 5.3) |
| `internal/frame/` | frames `start` to `replay_end` with byte order, content rules and directions (24 5.4), fuzzed parser |
| `internal/fingerprint/` | fingerprint of the host in Crockford Base32 with its display form (24 4.3) |
| `testdata/noise/` | public test vectors, recordings of Go and Dart, fingerprint vectors ([README](testdata/noise/README.md)) |
| `.go-arch-lint.yml` | architecture rules, maintained by hand, one component per package |

What the packages guarantee: no network, no process start, no logs; error texts name codes,
lengths and type bytes, never payload, plaintext, keys or tokens. After a failed handshake step or
a failed decryption the object refuses every further call.

## Commands

```sh
cd services/host
GOTOOLCHAIN=local go test -race ./...                         # tests, also TM-T01 and TM-T02
make recordings                                               # rewrite recordings/go.json and fingerprint.json
make release-build VERSION=$(cat VERSION) OUT=<directory>     # unsigned binaries for five platforms
GOTOOLCHAIN=local go test -run='^$' -fuzz=FuzzDecode -fuzztime=60s ./internal/frame
```

`release-build` builds `bothy-host_<version>_<os>_<arch>` (Windows with `.exe`) without cgo for
darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 and windows/amd64
([18 Release](../../docs/spec/18-release.md), row `host`). Signature, SBOM, checksums and
installation packages are added by M7-23; releases carry tags `host/v<version>`.
