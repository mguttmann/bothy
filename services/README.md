# services

Server code. Today only planning, no code.

| Folder | Content | Chapter |
|---|---|---|
| `server/` | The Go server as a modular monolith: one binary with the roles `api` and `worker` | [02 Architecture](../docs/spec/02-architecture.md) |
| `host/` | Host service `bothy-host` for the terminal on connected hosts | [24 Terminal](../docs/spec/24-terminal.md) |

For the gates, every tracked `go.mod` under `services/` is a Go module (format, lint,
build, tests with coverage, vulnerabilities, licenses); details in
[docs/quality/gates.md](../docs/quality/gates.md). Releases of the server carry tags
`server/v<version>` and need `server/VERSION`, `server/CHANGELOG.md` and the target
`release-build` in `server/Makefile` (chapter 23, section 3).
