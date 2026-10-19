# Tests of the setup wizard install.sh

Tests of [infra/compose/install.sh](../../install.sh), the setup wizard of
[docs/spec/16-betrieb.md](../../../../docs/spec/16-betrieb.md), section 3.1. They run the script
against stubs: no container of Bothy starts, nothing goes to the network, nothing outside a work
folder is written.

## How to run

| Command | Where it runs |
|---|---|
| `make install-test` | `sh infra/compose/test/install/run.sh` in the container `debian:trixie-slim@sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f` (dash as `sh`, `script` from util-linux), the repository mounted read-only at `/src` |
| `INSTALL_TEST_NATIVE=1 make install-test` | the same runner on the host |
| `sh infra/compose/test/install/run.sh test_gateway_ipv6` | only the named tests |

The runner writes only into a folder from `mktemp -d` under `${TMPDIR:-/tmp}` and removes it at the
end. It prints `ok` or `not ok` per test (with the output of a failed test), then a summary line.
Exit codes: `0` all tests passed or were skipped, `1` a test failed, `2` a precondition of the runner
is missing (for example `perl`, which it uses to run each case without a controlling terminal).

On the host the only skipped test is `test_terminal_prompts` when `script` is not the one from
util-linux (macOS has a BSD `script`); it prints a visible `SKIP` line. In the container nothing is
skipped.

## How it works

`run.sh` renders `install.sh` the way the release build does: it replaces only the three built-in
lines with a test version, the SHA-256 of a fixture bundle and the base address
`https://github.com/example/bothy/releases/download/v<test version>/`. The fixture bundle
`bothy-compose-<test version>.tar.gz` has the top directory `bothy-<test version>/` with a small
`infra/compose/compose.yaml` and the real `infra/compose/.env.example` of the repository, in which
`BOTHY_VERSION` is a test server version and `BOTHY_REGISTRY` is `ghcr.io/example`: both differ from
the script's version, so a test sees that they pass through unchanged.

Each `test_*` function of `cases.sh` runs in its own empty folder, with a clean environment and
`PATH` set to `stubs/` plus links to the system tools of `/usr/bin` and `/bin` without `docker`,
`curl`, `cosign` and `sudo`, so a real docker CLI is never reachable, also not on a host that has
one.

| Stub | Behavior |
|---|---|
| `stubs/docker` | answers `version`, `info`, `network inspect` and the `compose` calls of `install.sh` (`version`, `config --quiet`, `config --format json`, `up --no-start`, `up -d`, `port api 8080`, `exec -T` of `bothy healthcheck`, `bothy setup bootstrap-owner` and `tailscale status`); variables `STUB_*` in the header of the file set versions, memory, architecture, gateways, readiness and an existing owner |
| `stubs/curl` | serves download URLs from a fixture folder by their last path segment and records them, exits `22` for a missing file; prints `STUB_DISCOVERY_STATUS` (default `200`) for the discovery URL |
| `stubs/cosign` | exits with `STUB_COSIGN_EXIT` (default `0`) |
| `stubs/sudo` | records the call and fails: the runner fails every test in which `install.sh` called `sudo` |

Every stub appends its arguments to the log of the test, which a failed test prints.

## What the tests cover

| Area | Tests |
|---|---|
| Script | `#!/bin/sh`, `sh -n`, each built-in line exactly once, no `OWNER` and no placeholder left after rendering, no `sudo` call, no cheaply detectable bashism, no en or em dash (ShellCheck runs in `make check`, not here) |
| Options | `--help` exits `0`; unknown option and bad values exit `2` with `install.sh: options:`; `--opt=value` |
| Preflight | docker missing, Engine 25, Compose 2.25 (with `v` and suffix): abort with `install.sh: preflight:` and nothing written; a foreign non-empty target folder is refused |
| Bundle | download address exactly `<base address>bothy-compose-<version>.tar.gz`; wrong checksum on the download and with `--bundle`; `--bundle` without download; cosign called with the identity of the release workflow, a failing signature aborts, without cosign or with a key-signed signature of the private phase the command is named; a member outside the top directory is refused; the unrendered source aborts in step `bundle` |
| Prompts | without a terminal a missing `--public-url`, `--owner-handle`, `--owner-name` or `TS_AUTHKEY` aborts naming it; with a terminal (pseudo terminal of `script`) the answers reach `.env` |
| Write | `.env` mode `0600`, `secrets/` mode `0700`, no line but the answers changed (no secret except `TS_AUTHKEY`), `BOTHY_VERSION` and `BOTHY_REGISTRY` from the bundle, `--storage s3`, scan default by memory and the memory note |
| Start | order of the Compose calls in the compose folder without a project option; `BOTHY_TRUSTED_PROXIES` from the gateway of the network named by `docker compose config --format json` (IPv4, IPv4 and IPv6, none); discovery not `200` aborts naming `BOTHY_TRUSTED_PROXIES`; readiness bounded by `--ready-timeout` |
| Owner and finish | the enrollment code is shown once on the output and is in no file of the target folder; an existing owner is skipped; next steps and the warning `local backup only`; edge tailscale clears `TS_AUTHKEY` after the login or warns |
| Second run | `.env` and `secrets/` unchanged (content and mode), an operator value of `BOTHY_TRUSTED_PROXIES` stays, the default value is replaced by the gateway |
