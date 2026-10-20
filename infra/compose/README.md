# infra/compose

Docker Compose stack of a Bothy instance. Rules: [docs/spec/16-operations.md](../../docs/spec/16-operations.md).
Guide: [docs/operations/install-compose.md](../../docs/operations/install-compose.md).

## Files

| File | Content |
|---|---|
| `compose.yaml` | all services, profiles, networks, volumes |
| `.env.example` | template for `.env`; only placeholders and default values, no secrets |
| `compose.multi-node.yaml` | overlay with two api nodes behind the example edge (profile `direct` only; section "Several api nodes") |
| `edge/nginx.conf` | example edge (nginx) of the overlay: hash routing of the terminal stream paths, timeouts, no access log |
| `secrets/` | not in the repository: secrets delivered by the owner as files (catalogue in [configuration.md](../../docs/operations/configuration.md) 5) |
| `install.sh` | source of the setup wizard (POSIX `sh`, [16-operations.md](../../docs/spec/16-operations.md) 3.1); the release ships it rendered with version, checksum and download address next to the bundle (`make release-build`), not inside it; tests in [test/install/](test/install/README.md) (`make install-test`) |
| `smoke.sh` | the Compose smoke test (`make portability`, M1-73); test only |
| `alert-setup.sh` | guided setup of the alert routes (POSIX `sh`, M7-20): reads a webhook URL from standard input and writes `secrets/alertmanager_*` with mode `0600`, printing only the fingerprint ([alerting-setup.md](../../docs/operations/alerting-setup.md)); tests in `tools/observability/tests/test_alert_setup.py` |
| `test/` | the test overlay, its variables and the collector overlay `file` of the smoke test; the test overlays `test/compose.terminal.yaml` and `test/compose.terminal-twonode.yaml`, the host image `test/host/Dockerfile` and the TLS front `test/terminal/` of the terminal conformance suite (M2-48); test only, never for an installation, not in the release bundle ([test/README.md](test/README.md)) |

The stack also mounts `../otel` (Collector) and `../observability` (Prometheus, Alertmanager,
Loki, Tempo, Grafana), read-only. The release bundle therefore contains both folders
(docs/spec/18-release.md, section 2).

## Short form

One command: the setup wizard `install.sh` in this folder (flow in
[docs/spec/16-operations.md](../../docs/spec/16-operations.md), section 3.1); it is published
with every release of the component `bothy`. The manual path:

```
cp .env.example .env          # set values
mkdir -m 700 secrets          # store the delivered secrets
docker compose config --quiet # check
docker compose up --no-start  # create networks; in the profile direct set BOTHY_TRUSTED_PROXIES now
                              # (docs/operations/install-compose.md, section 5.1)
docker compose up -d
```

## Profiles

| Profile | Services | Note |
|---|---|---|
| (always) | `secrets-init`, `postgres`, `migrate`, `worker`, `backup`, `otel-collector` | |
| `direct` | `api` on `127.0.0.1:${BOTHY_HTTP_PUBLISH_PORT}` | TLS and login through a reverse proxy or OIDC |
| `tailscale` | `tailscale`, `api-tailscale` | header identity; see [tailscale.md](../../docs/operations/tailscale.md) |
| `scan` | `clamav` | needs a lot of memory (vendor: at least 3 GiB); clamd's socket lies in the volume `clamav-socket`, mounted read-only in `worker` |
| `s3` | `seaweedfs` | S3 driver on one host; set `BOTHY_STORAGE_DRIVER=s3` |
| `observability` | `prometheus`, `alertmanager`, `loki`, `tempo`, `grafana` | together with `BOTHY_OTEL_EXPORTER=local` |

Exactly one of `direct` and `tailscale`.

The profile `scan` belongs to the default `file.scan.mode = required`: without it, completed
uploads stay pending until clamd answers ([12-files.md](../../docs/spec/12-files.md) 5.3, fail
closed). An installation without a virus scanner sets `file.scan.mode` to `off`.

## Several api nodes

```
docker compose -f compose.yaml -f compose.multi-node.yaml config --quiet
docker compose -f compose.yaml -f compose.multi-node.yaml up -d
```

The overlay `compose.multi-node.yaml` runs `api` and `api-2` with `BOTHY_CLUSTER__MODE=multi` (the
worker too) and puts the example edge `edge` (nginx, image tag `BOTHY_NGINX_VERSION`) in front of
them. Rules ([16-operations.md](../../docs/spec/16-operations.md) 4.4,
[install-compose.md](../../docs/operations/install-compose.md) 5.3):

1. Profile `direct` only. With `tailscale` neither `edge` nor `api-2` starts: `tailscale serve`
   has no hash rule, so that profile runs one api node.
2. The api nodes publish no port; only `edge` publishes on
   `127.0.0.1:${BOTHY_HTTP_PUBLISH_PORT}`. The operator's TLS edge stays in front of it.
3. `edge/nginx.conf` routes `/api/v1/terminal/sessions/{session_id}/stream` and
   `/api/v1/hosts/sessions/{session_id}/stream` by consistent hashing over the session ID to the
   same api node; everything else may land on either node. An edge of your own needs the same
   rule, otherwise both sides of a session close with `4102`.
4. `BOTHY_TRUSTED_PROXIES` names the subnet of the network `<project>_default` (field `Subnet` of
   `docker network inspect <project>_default`), because the server sees the address of `edge`.
5. Both api nodes mount the same key store `signing-keys` (ADR 0066); `api-2` takes image,
   command, hardening, volumes and health check from `api` through `extends`.
6. `edge` restarts when an api node is recreated, because nginx resolves the node names only at
   start.
7. `edge` runs as a non-root user with a read-only root file system, without capabilities, with
   `no-new-privileges` and writes no access log.

## Security rules in this file

1. Ports only on `127.0.0.1`. PostgreSQL, Loki, Tempo and SeaweedFS are attached only to the
   network `internal` without egress and are never published on the host.
2. Secrets never in environment variables; each service mounts only its own subdirectory of the
   volume `secrets` (`subpath`, Docker Engine 26 or newer).
3. `api`, `worker`, `migrate`: read-only file system, all capabilities dropped,
   `no-new-privileges`. Only the sidecar `tailscale` gets `NET_ADMIN` and `/dev/net/tun`
   (kernel networking, see [tailscale.md](../../docs/operations/tailscale.md) 2).
4. Images of the auxiliary services are pinned to versions; Bothy images to `BOTHY_VERSION`.
5. Container logs are rotated by the log driver.
6. clamd is reached only over its Unix socket (19 SEC-TLS-5, M3-04): the volume `clamav-socket`
   is mounted at `/run/clamav` in `clamav` and read-only in `worker`, in no other service;
   `clamav` publishes no port, exposes none and has no `TCPSocket` in
   [clamav/clamd.conf](clamav/clamd.conf), which also sets the mandatory options of
   [12-files.md](../../docs/spec/12-files.md) 5.4. Its health check pings over the socket.

## Checked

`docker compose config --quiet` (Docker Compose v5.6.0) with `.env.example` and all profiles on
2026-10-10: valid. A start was not possible because the Bothy images do not exist yet and no
Docker daemon was running on the checking host. `secrets-init`, `bothy-backupd`, `bothy healthcheck`,
`migrate --bootstrap` and the TLS connection of pgBackRest are contracts for build tickets.

Since M1-73 the smoke test `smoke.sh` (`make portability`, [test/README.md](test/README.md))
proves the server part of the stack with the real `bothy-server` image: this file unchanged plus
the test overlay, `secrets-init`, `migrate up --bootstrap`, `api`, `worker` and the collector,
first setup, enrollment, live delivery over WebSocket, catch-up, revocation and the canary scan of
the collector export and the container logs. It runs on the official PostgreSQL 18 image without
`backup`; `bothy-postgres`, `bothy-backupd`, the profiles `scan`, `s3`, `tailscale` and
`observability` and the full portability test stay for M7 (M7-14).

Checks of the overlay and the example edge (`services/server/test/edge`, needs Docker):
`TestComposeMultiNodeOverlay` renders compose.yaml with the overlay and `.env.example` for the
profiles `direct` and `tailscale`; `TestEdgeExample_Config` runs `edge/nginx.conf` in the pinned
image and compares its directives and timeouts with the defaults in
[configuration.md](../../docs/operations/configuration.md); `TestEdgeExample_HashRoutesBothStreamPaths`
proves that both stream paths of a session reach the same node.
