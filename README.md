# hermes-chat (Arbeitstitel)

Eigene Discord-ähnliche Chat-App für Hermes (iPhone, macOS, Windows, nur Tailscale). Planung läuft.

## Quick start

One command on a Linux or macOS host with Docker Engine 26 and Compose 2.26 or newer (on Windows
inside WSL 2). The setup wizard checks the host, downloads the bundle of its version and checks
its SHA-256 (and its signature when `cosign` is installed), asks for the public address, the edge
(reverse proxy or Tailscale) and the first owner, starts the stack and prints the one-time
enrollment code for the app ([specification](docs/spec/16-betrieb.md), section 3.1):

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

The version in the address is the product version (`VERSION`); `make release-prep COMPONENT=bothy`
updates it, and `make install-options` checks it. Without a terminal, every answer is an option
after `sh -s --` (`--yes`, `--public-url`, `--edge`, `--scan`, `--storage`, `--observability`,
`--offsite`, `--owner-handle`, `--owner-name`, `--dir`, `--bundle`, `--ready-timeout`; `--help`
lists them). The [Compose guide](docs/operations/install-compose.md) describes the same install
step by step.
