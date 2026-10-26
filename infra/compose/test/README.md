# infra/compose/test: Compose smoke test

Test files of the Compose smoke test (brief M1-73, plan P-06, exit criterion 8 of milestone M1).
**Test only, never for an installation**: `make release-build` leaves this directory out of the
release bundle, and a test (`test_release_bundle_excludes_smoke_overlay`) holds that.

`infra/compose/smoke.sh` (run by `make portability` without `PORTABILITY_TAG`) starts the server core from the real
`bothy-server` image with the unchanged installation file `infra/compose/compose.yaml` plus the
overlay of this directory and proves the path from first setup to live delivery over WebSocket.
It is the only end-to-end proof that the image, the Compose contract and the server modules work
together.

## Files

| File | Content |
|---|---|
| `compose.smoke.yaml` | test overlay: the server services on the local image, the PostgreSQL 18 image of `pgtest` pinned by digest, no published port, `backup` disabled, the collector with the overlay `file`, the services `smoke-probe` and `smoke-scan` |
| `smoke.env` | Compose interpolation variables (`--env-file`); no secrets, environment `staging` |
| `compose.console-e2e.yaml` | overlay of the console E2E test (M6-03, `make console-e2e`, `tools/console-e2e/README.md`): the TLS sidecar `tls`, the seed `console-seed` (`bothy-smoke seed`) and the browsers `console-browser`, all in the network namespace of `api`; `server.public_url` `https://localhost:8443` and the WebAuthn relying party `localhost`; the webhook receiver `webhook-receiver` of phase 2 (M6-08) on the Compose network, HTTPS on 8443 with the per-run certificate, no published port |
| `console-e2e.env`, `console-e2e/Caddyfile`, `console-e2e/receiver.Caddyfile` | the variables of that overlay (read after `smoke.env`), the configuration of the sidecar and of the webhook receiver (every POST answered 200, method, path and headers of each request in the JSON access log) |
| `compose.console-e2e-admin.yaml` | overlay of phase 2 of the console E2E test (M6-04, M6-08): `api` serves the bundle of the patrol suites with a step-up of 3 s; `api` and `worker` list the receiver in `webhook.allowed_hosts` and `egress.allowed_hosts` (`webhook-receiver:8443;allow_private`) and trust the test CA through `SSL_CERT_FILE` (the system roots of the server image plus the test CA, written by the runner); the release image and the installation bundle stay unchanged |
| `compose.console-e2e-terminal.yaml` | overlay of phase 3 of the console E2E test (M6-18, section "Console E2E test, phase 3" below): `api` serves the bundle of the terminal test and joins the internal network `consolehosts`; the host service container `console-host` of the host image of M2-48; `E2E_MODE` and the test CA for `console-browser` |
| `compose.alert-e2e.yaml`, `alert-e2e/rules.yaml` | overlay of the alert path test (M7-20, `make alert-e2e`, `tools/observability/alert_e2e.py`): Prometheus with a configuration derived from the real one plus the test rules (one `page`, one `ticket` alert), Alertmanager unchanged without a published port, the stub `alert-stub` for the external receivers and the metric of the test rules, the probe `alert-probe` in the network namespace of `api` ([alerting.md](../../../docs/observability/alerting.md) 5) |
| `compose.e2e.yaml`, `e2e.env`, `e2e/Caddyfile` | overlay of the desktop E2E stack (M4-34, `infra/compose/e2e.sh`, section "Desktop E2E stack (M4-34)" below): the TLS front `e2e-tls`, the world job `e2e-world` (`bothy-smoke e2e-world`) and the Fake-Agent `e2e-agent`, all in the network namespace of `api`; the only published port is the front on `127.0.0.1`; its variables (read after `smoke.env`) and the configuration of the front |
| `otel/file.yaml` | collector overlay `file`: the three signals as OTLP JSON lines into the volume `otel-smoke-out`, shorter `tail_sampling` and `batch` waits |

The probe is the Go package `services/server/test/smoke` and its command `cmd/bothy-smoke`, built
as the target `smoke` of `services/server/Dockerfile` (same builder and runtime base as the server
image; the production stage stays last).

## Prerequisites

- Docker Engine 26.0.0 or newer (volume `subpath`) and Docker Compose 2.26.0 or newer (`!reset`,
  `!override`); nothing else on the host (16 "Portability test in CI").
- The server image: `make image` loads `bothy-server:<services/server/VERSION>-local`, the default
  of `SMOKE_SERVER_IMAGE`. Without it the script stops with exit code 2 and names `make image`; it
  never pulls or builds the server image itself.
- `TMPDIR` must lie in a directory the Docker runtime shares with its VM: the probe writes to and
  the scanner reads from bind mounts below it. Docker Desktop shares the default temp directory of
  macOS; with colima point `TMPDIR` into the home directory.

A missing Docker, a Docker runtime that does not answer, an older Engine or Compose and a missing
image each exit with code 2 and a message that names the fix (P-09: nothing is skipped).

## Variables

Every variable of the harness starts with `SMOKE_`; the defaults live only in `smoke.sh`.

| Variable | Default | Meaning |
|---|---|---|
| `SMOKE_SERVER_IMAGE` | the tag of `make image` (`IMAGE_NAME:IMAGE_TAG` of the `Makefile`) | server image to test |
| `SMOKE_KEEP` | `0` | `1` keeps the stack and the temp directory and prints the commands that remove them |
| `SMOKE_WAIT_TIMEOUT_S` | `240` | bound of every wait of the stack (`up --wait`, `stop`, `down`) |
| `SMOKE_TELEMETRY_SETTLE_S` | `5` | wait between the stop of `api` and `worker` and the stop of the collector; at least `decision_wait` + `batch.timeout` + `flush_interval` of `otel/file.yaml` (a test holds it) |
| `SMOKE_LOG_TAIL` | `200` | log lines per service printed on a failure |

`smoke.sh` exports `SMOKE_RUN_ID`, `SMOKE_SECRETS_INPUT`, `SMOKE_OUT`, `SMOKE_TMP` and
`SMOKE_CANARY` for the overlay; they are not inputs. Every exported `BOTHY_*`, `COMPOSE_*` and
`OTEL_*` variable of the calling shell is removed first, and Compose always reads `smoke.env`, so
the `.env` of a real installation is never read.

## Steps

Each step is announced as `smoke: <step>` on stderr. On a failure the script prints the step,
`compose ps --all` and the last `SMOKE_LOG_TAIL` lines of `api`, `worker`, `migrate` and
`otel-collector`, cleans up and exits 1.

1. Preflight: Docker, Engine and Compose versions, the image.
2. Workspace: `mktemp -d` with `secrets-input` (empty, mode 0700: no owner-supplied secrets),
   `out`, `otel`, `logs`; run ID, project name `bothy-smoke-<run ID>`, canary `smk` plus 24
   lowercase hex characters from `/dev/urandom`.
3. `compose config --quiet`; `compose build smoke-probe` (with the build argument `VERSION` of the
   builder stage); `compose pull` of the PostgreSQL and collector images.
4. `compose up --wait api worker otel-collector`: `secrets-init` (`bothy secrets init`),
   `postgres`, `migrate` (`bothy migrate up --bootstrap`), `otel-out-init`; both one-shot
   containers must exit 0.
5. `secrets-init` and `migrate` run a second time and exit 0 (idempotence; output into
   `logs/rerun.log`).
6. Probe `precheck`: discovery says `setup_required: true`, `GET /api/v1/me` answers
   `503 setup_required` (ID-T06).
7. `bothy setup bootstrap-owner --handle alex --display-name "Alex <canary>"` in the running `api`
   container; the code goes to `out/setup.out` (mode 0600, never printed, never scanned), stderr to
   `logs/setup.err` (scanned). A second identical call must fail (ID-T07). No restart.
8. Probe `run`: discovery after setup; enrollment with the code, a DPoP key and a step-up key
   (`platform: macos`, the probe stands in for a native app); `getMe`; gateway connection A
   (`hello`, `identify` with the DPoP proof for the nonce of `hello`, `ready`, heartbeats); space,
   channel, a main-thread message, a thread from it and three messages, delivered on A in `seq`
   order with their `client_msg_id`; A closed with `1000`; three messages while offline;
   `listEvents` after the cursor (REST catch-up); connection B with `cursor` and `sync_epoch`: the
   missed events in order, none at or below the cursor, `catchup.complete`, then a live message;
   step-up with the device key; `revokeDevice` closes B with `4012` and no dispatch arrives after
   the answer; the old access token gets `401`. The probe writes `out/canaries.txt` (mode 0600):
   the canary, both tokens, the enrollment code and the principal ID.
9. Flush: `compose stop api worker` (graceful shutdown flushes the SDK, 02 15.3), wait
   `SMOKE_TELEMETRY_SETTLE_S`, `compose stop otel-collector`.
10. Collect: `compose logs` of all services into `logs/compose.log`; `compose cp` of the collector
    export into `otel/`.
11. Scan (`smoke-scan`, no network, the temp directory read-only): no value of `canaries.txt` in
    any file below `otel/` and `logs/`; a positive control first proves the scanner finds a
    canary; the export must hold a span of the route of `createMessage`, a log record and a metric
    data point of `service.name` `bothy-server`, so an empty export cannot pass. A hit names file,
    line and label, never the value.
12. Cleanup (always, unless `SMOKE_KEEP=1`): `compose down --volumes --remove-orphans --rmi local`,
    removal of the probe image and of the temp directory; the script then asserts that no
    container, volume or network with the project label remains.

Exit codes: 0 pass, 1 smoke failure, 2 precondition missing.

## Why the probe shares the network namespace of api

Inside a container the server sees the traffic of a published port coming from the bridge
gateway, not from loopback, and SEC-TLS-1 allows plaintext HTTP only from loopback; widening
`server.trusted_proxies` for a test would weaken the rule under test. The probe therefore runs with
`network_mode: service:api` and dials `127.0.0.1:8080`, like the `tailscale` sidecar of the
installation, and the server listens on `127.0.0.1:8080` only. No host port is published (P-10).
DPoP proofs name `server.public_url` (`https://chat.example.com`), not the loopback address the
probe dials (08 7.3). The WebSocket client sends no `Origin` header (SEC-CORS-3).

## What is proven and what is not

Proven: the real image starts with the installation's Compose contract (secrets layout, roles,
migrations, health checks, read-only file systems), the setup lock and the first owner, enrollment
with device keys and DPoP, the REST writes of the chat core, ordered live delivery, REST and
gateway catch-up, step-up, revocation with `4012`, the canary scan of the collector export and the
container logs.

Not proven here: the profiles `scan` and `s3`, files and EICAR, search, the status page, backup
and restore with `bothy-postgres` and `bothy-backupd`, the other architecture than the host's (the
run uses the image of the host platform), the `tailscale` profile and released images. The
portability test proves them with a release (`make portability` with `PORTABILITY_TAG`,
[tests/portability/README.md](../../../tests/portability/README.md), M7-14). The probe's own output
(`out/`) is not scanned.

## Debugging

`SMOKE_KEEP=1 make portability` keeps the stack and the temp directory; the script prints the
`docker compose ... down` and `rm -rf` commands that remove them. With the project name from the
output, `docker compose --project-name bothy-smoke-<run ID> ... logs api` shows a service. Never
print `out/setup.out` or `out/canaries.txt`: they hold the enrollment code and the tokens.

## Terminal conformance suite (M2-48)

The end-to-end packages of the terminal suite (`services/server/test/terminal/e2e/...`,
[README](../../../services/server/test/terminal/README.md)) run one Compose stack per test package
from the files of this directory. **Test only, never for an installation**; like everything here
they are not in the release bundle.

| File | Content |
|---|---|
| `compose.terminal.yaml` | overlay of the suite: `api` and `worker` read the server keys of the suite profile from `TERMSUITE_SERVER_ENV`; `api` listens on `:8080` inside its namespace, trusts `TERMSUITE_TRUSTED_PROXIES` as its proxies and publishes no port; the TLS front `tls-front`; the service `termhost` (profile `termhost`, so `up` never starts it) for the host containers; the internal network `termhosts` |
| `compose.terminal-twonode.yaml` | overlay of the profile `twonode` (TM-T15), last after M2-47's `compose.multi-node.yaml`: `api-2` on the server image of the suite with the same rendered environment as `api`, and `edge` without a published port |
| `host/Dockerfile` | the host image: Debian with systemd as process 1 and the marker file `/etc/termsuite-disposable` of the platform suite; no `bothy-host` inside |
| `terminal/nginx.conf.tmpl` | configuration of the TLS front; the suite renders it with the timeouts of the profile (stream paths from `terminal.idle_timeout` and `terminal.idle_grace`, everything else from `terminal.host_heartbeat_interval` and `terminal.host_heartbeat_miss_limit`, as M2-47's edge computes them) |
| `terminal/routing-single.conf`, `routing-hash.conf`, `routing-split.conf` | the routing modes of the front: `single` sends everything to `api`; `hash` to M2-47's example edge `edge`; `split` the device stream paths to `api`, the host data paths to `api-2` and the rest to `edge` (TM-T15 without hash routing) |

### Stack

The suite combines, in this order, `compose.yaml` unchanged, `test/compose.smoke.yaml` of M1-73,
for the profile `twonode` `compose.multi-node.yaml`, then `test/compose.terminal.yaml` and for
`twonode` last `test/compose.terminal-twonode.yaml`; every call passes `--project-directory
infra/compose` and `--env-file infra/compose/test/smoke.env`, as `smoke.sh` does. The project
name is `termsuite-<package>-<8 random hex>`. `COMPOSE_PROFILES=direct` keeps the services of the
smoke test inactive, and `BOTHY_PUBLIC_URL` is the instance origin `https://chat.example.com:8443`.

- **Server image built from the worktree.** Each e2e package builds `services/server/Dockerfile`
  from the repository root with `docker buildx build --load` for the platform of the Docker host,
  with the build arguments `VERSION` (from `services/server/VERSION`) and `REVISION` (`git
  rev-parse HEAD`), and tags it `termsuite-server:<first 12 hex of the image ID>`; that tag is
  `SMOKE_SERVER_IMAGE`. An unchanged tree is a cache hit of BuildKit, and parallel packages and
  worktrees arrive at their own content-addressed tags, so no lock is needed. The suite never
  pulls a server image.
- **TLS front.** `tls-front` runs `nginxinc/nginx-unprivileged` at `BOTHY_NGINX_VERSION` of M2-47
  as the uid and gid of the test process, read-only, without capabilities and with
  `no-new-privileges`. It serves `chat.example.com:8443` with a certificate of a test CA that the
  suite creates per stack, and is the only published port: `127.0.0.1` with a random host port
  (`docker compose port tls-front 8443`). On `termhosts` it carries the alias `chat.example.com`.
  The suite switches the routing mode by replacing `routing.conf` in `TERMSUITE_FRONT_DIR` and
  reloading nginx in the container.
- **Networks.** `termhosts` is `internal` and carries only `tls-front` and the host containers, so
  a host service reaches nothing but the TLS front; the server services stay on the networks of
  the installation. `TERMSUITE_TRUSTED_PROXIES` is the subnet of `<project>_default`, read after
  `compose up --no-start` with `docker network inspect`, before `compose up --detach --wait`.
- **Host image.** `termsuite-host:<first 12 hex of the SHA-256 of host/Dockerfile>`, built with
  BuildKit when the tag is missing. Base `debian:trixie-slim` pinned by digest, APT sources on one
  timestamp of `snapshot.debian.org` (recorded in the Dockerfile,
  [toolchain.md](../../../docs/quality/toolchain.md)). It runs as root because systemd is process
  1 and `bothy-host install` creates real users and units; `termhost` is `privileged` because
  systemd needs a writable cgroup tree and the host service creates a cgroup per session. Only
  this test service is privileged. Each host of a case is one container `compose run --detach
  --name <project>-<case>-<host> termhost`; the suite copies `bothy-host` and the Fake-`claude`,
  built from the worktree, into it, adds the test CA to the trust store and runs `bothy-host
  install`, `pair` and `doctor` as an operator would.
- **Removal.** At the end of each test binary: the host containers, then `compose down --volumes
  --remove-orphans`; the suite then asserts that no container, volume or network with the project
  label remains, and that no file under `infra/compose` changed while the stack ran.

### Variables

Every variable of the suite starts with `TERMSUITE_`; the suite sets them for each Compose call,
after it removed every exported `BOTHY_*`, `COMPOSE_*`, `OTEL_*`, `SMOKE_*` and `TERMSUITE_*`
variable of the calling shell. They are not inputs.

| Variable | Read by | Meaning |
|---|---|---|
| `TERMSUITE_SERVER_ENV` | `compose.terminal.yaml` (`api`, `worker`), `compose.terminal-twonode.yaml` (`api-2`) | `env_file` with the server keys of the profile of `services/server/test/terminal/profiles.yaml` as `BOTHY_*` variables (mode 0600 in the temporary folder) |
| `TERMSUITE_TRUSTED_PROXIES` | `api`, `api-2` | `BOTHY_SERVER__TRUSTED_PROXIES`: the subnet of the project's default network, where the TLS front connects from |
| `TERMSUITE_FRONT_DIR` | `tls-front` | folder mounted read-only with the rendered `nginx.conf`, `routing.conf`, the certificate and the key of the front |
| `TERMSUITE_UID`, `TERMSUITE_GID` | `tls-front` | uid and gid of the test process, so the front reads its key file of mode 0600 |
| `TERMSUITE_HOST_IMAGE` | `termhost` | the tag of the host image |

The smoke overlay's `SMOKE_SERVER_IMAGE`, `SMOKE_SECRETS_INPUT`, `SMOKE_RUN_ID`, `SMOKE_OUT`,
`SMOKE_TMP` and `SMOKE_CANARY` are set by the suite as `smoke.sh` sets them.

### Prerequisites

Docker Engine and Compose at the minimum versions above, `docker buildx`, a Docker host on cgroup
version 2 that starts privileged containers, and `kernel.yama.ptrace_scope` of at least `1` on the
Docker host (below it the host service refuses to start with HD10 code `ptrace_scope_low`). The
preflight of the suite checks each and fails with a message that names the fix; nothing is
skipped, and the suite never changes a kernel setting.

### Cached images

The server and host images stay as local build cache under their content-addressed tags, one per
tree and Dockerfile state. List and remove them with:

```sh
docker image ls 'termsuite-server'
docker image ls 'termsuite-host'
docker image rm termsuite-server:<tag> termsuite-host:<tag>
```

The next run of the suite builds a removed image again.

## Console E2E test, phase 3 (M6-18)

Phase 3 of `make console-e2e` ([tools/console-e2e/README.md](../../../tools/console-e2e/README.md),
brief M6-18 Design notes 5 and 6) opens a terminal in the web console on a real host service.
`tools/console-e2e/console_e2e.py` adds `compose.console-e2e-terminal.yaml` to the files of phase 1
(`compose.yaml`, `compose.smoke.yaml`, `compose.console-e2e.yaml`) in place of the admin overlay.
**Test only, never for an installation.**

- **Host service container.** `console-host` runs the host image of the terminal suite above
  (`host/Dockerfile`, the tag `termsuite-host:<first 12 hex of the SHA-256 of the Dockerfile>`,
  built with BuildKit when missing and removed again by the run that built it), privileged, as
  root and with the same `tmpfs` as `termhost`, for the same reasons. The runner copies
  `bothy-host` and the Fake-claude in (built from `services/host` by
  `tools/console-e2e/host-bin.Dockerfile`, the builds of M2-48's `BuildHostBinaries`), adds the test
  CA of the run, which the overlay mounts read-only as the only file, to the trust store and runs
  `bothy-host install` and `pair` as an operator would.
- **Network.** The internal network `consolehosts` carries only `api` and `console-host`. The TLS
  sidecar of the console test shares the network namespace of `api` and listens on `:8443`; the
  host service pairs with `https://api:8443`, a name the certificate of the test CA names, so it
  talks TLS to the sidecar and never plaintext to the server (24 2.2, SEC-TLS-1), and it reaches
  nothing else. No port is published.
- **Server keys.** The overlay sets only `console.assets_dir` (the bundle of
  `apps/console/integration_test/terminal/terminal_bundle.dart`); `terminal.quick_reattach_window`
  and `auth.step_up_max_age` keep their defaults, so the attach of the test falls inside the
  window.

| Variable | Read by | Meaning |
|---|---|---|
| `E2E_TERMINAL_BUNDLE` | `api` | the exported bundle of the target `console-e2e-terminal-web` of `services/server/Dockerfile`, mounted read-only as `console.assets_dir` |
| `E2E_HOST_IMAGE` | `console-host` | the tag of the host image |
| `E2E_CERTS` | `console-host`, `console-browser` | the directory of the test CA of the run; only `ca.pem` is mounted |
| `E2E_MODE` | `console-browser` | `terminal` runs the terminal suites |

The runner sets them; they are not inputs.

## Desktop E2E stack (M4-34)

`infra/compose/e2e.sh` (run by `make e2e-desktop` and `make perf-desktop`, brief M4-34 Design
note 1, [15-client-architecture.md](../../../docs/spec/15-client-architecture.md) 20) starts the
server core like the smoke test, with `compose.e2e.yaml` and `e2e.env` on top, and runs the suite
of the desktop app (`apps/client/tool/e2e_suite.sh`, [apps/client/README.md](../../../apps/client/README.md))
against it. The app runs on the host, outside Compose. **Test only, never for an installation.**

- **TLS front.** The server accepts plaintext only from loopback (SEC-TLS-1). `e2e-tls` (Caddy,
  pinned by digest) shares the network namespace of `api`, terminates TLS with a leaf of a test CA
  of the run (`localhost`, `127.0.0.1`; the CA key is deleted once the leaf is signed) and passes
  on to `127.0.0.1:8080`. `api` publishes only the front's port, a free port on `127.0.0.1` that
  the script picks; `server.public_url` is `https://localhost:<port>`, the address the app and the
  services in the namespace use alike.
- **World.** After `bootstrap-owner` (Alex, locale `en`) the job `e2e-world` prepares the world
  through the public API: Sam, a space with a text channel and a forum, the agents
  `support-agent` and `stream-agent` with their tokens, the operator and the mention behavior of
  the forum. It writes `e2e.json` (the input of the suite) and `e2e-agent.json` (the credentials
  of the agents) with mode `0600` into the run's temp directory.
- **Fake-Agent.** `e2e-agent` runs `tools/ci/desktop_e2e_agent.py` with the Fake-Agent of
  `sdk/python` (an image built from a pinned Python base, requirements with `--require-hashes`)
  and plays both agents over REST until the run ends; the script waits for its readiness line.
- **Server keys.** The overlay turns off the file scan and raises the rate limits of every class
  for `api` and `worker`, so the suite measures the app and not the limiter; every key exists in
  [configuration.md](../../../docs/operations/configuration.md) (a test holds that).
- **Cleanup.** The run removes the project with its volumes and local images, the images
  `bothy-smoke:<run>` and `bothy-e2e-agent:<run>` and its temp directory; `E2E_KEEP=1` keeps
  them and prints the removal command.

| Variable | Read by | Meaning |
|---|---|---|
| `E2E_PORT` | `api`, `e2e-tls` | the free loopback port of the front |
| `E2E_CERTS` | `e2e-tls`, `e2e-agent` | the directory of the test CA and the leaf of the run |
| `E2E_AGENT_CONTEXT` | `e2e-agent` | the build context with the exported requirements |
| `E2E_UID_GID` | `e2e-tls`, `e2e-agent` | the user of the host, so the files of the run stay the host's |
| `SMOKE_OUT` | `e2e-world`, `e2e-agent` | the directory of `e2e.json` and `e2e-agent.json` |

The script sets them; they are not inputs. Its inputs (`E2E_SERVER_IMAGE`, `E2E_KEEP`,
`E2E_WAIT_TIMEOUT_S`, `E2E_LOG_TAIL`) and exit codes are in its header. Tests without Docker:
`tools/ci/tests/test_e2e_script.py` and `tools/ci/tests/test_desktop_e2e_agent.py`.
