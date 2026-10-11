# services/host

Host service `bothy-host`: its own Go binary on connected hosts for the terminal. Today only
planning, no code.

It runs as a user without administrator permissions, connects to the server only outbound, is the
Noise peer of the device and starts one PTY per terminal session (macOS, Linux, Windows through
ConPTY). Normative: [24 Terminal](../../docs/spec/24-terminal.md),
[ADR 0068](../../docs/adr/0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md).

Releases carry tags `host/v<version>` and need `host/VERSION`, `host/CHANGELOG.md` and the
target `release-build` in `host/Makefile` ([18 Release](../../docs/spec/18-release.md)).
