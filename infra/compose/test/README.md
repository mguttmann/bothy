# infra/compose/test: Compose smoke test

Test files of the Compose smoke test (brief M1-73, plan P-06, exit criterion 8 of milestone M1).
**Test only, never for an installation**: `make release-build` leaves this directory out of the
release bundle, and a test (`test_release_bundle_excludes_smoke_overlay`) holds that.

`infra/compose/smoke.sh` (run by `make portability`) starts the server core from the real
`bothy-server` image with the unchanged installation file `infra/compose/compose.yaml` plus the
overlay of this directory and proves the path from first setup to live delivery over WebSocket.
It is the only end-to-end proof that the image, the Compose contract and the server modules work
together.

## Files

| File | Content |
|---|---|
| `compose.smoke.yaml` | test overlay: the server services on the local image, the PostgreSQL 18 image of `pgtest` pinned by digest, no published port, `backup` disabled, the collector with the overlay `file`, the services `smoke-probe` and `smoke-scan` |
| `smoke.env` | Compose interpolation variables (`--env-file`); no secrets, environment `staging` |
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

Not proven here (M7-14 and M7): the profiles `scan` and `s3`, files and EICAR, search, the status
page, backup and restore with `bothy-postgres` and `bothy-backupd`, the other architecture than the
host's (the run uses the image of the host platform), the `tailscale` profile, released images and
the CI wiring of `portability.yml`. The probe's own output (`out/`) is not scanned.

## Debugging

`SMOKE_KEEP=1 make portability` keeps the stack and the temp directory; the script prints the
`docker compose ... down` and `rm -rf` commands that remove them. With the project name from the
output, `docker compose --project-name bothy-smoke-<run ID> ... logs api` shows a service. Never
print `out/setup.out` or `out/canaries.txt`: they hold the enrollment code and the tokens.
