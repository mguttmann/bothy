# infra/compose

Docker Compose stack of a Bothy instance. Rules: [docs/spec/16-operations.md](../../docs/spec/16-operations.md).
Guide: [docs/operations/install-compose.md](../../docs/operations/install-compose.md).

## Files

| File | Content |
|---|---|
| `compose.yaml` | all services, profiles, networks, volumes |
| `.env.example` | template for `.env`; only placeholders and default values, no secrets |
| `secrets/` | not in the repository: secrets delivered by the owner as files (catalogue in [configuration.md](../../docs/operations/configuration.md) 5) |
| `install.sh` | source of the setup wizard (POSIX `sh`, [16-operations.md](../../docs/spec/16-operations.md) 3.1); the release ships it rendered with version, checksum and download address next to the bundle (`make release-build`), not inside it; tests in [test/install/](test/install/README.md) (`make install-test`) |
| `smoke.sh` | the Compose smoke test (`make portability`, M1-73); test only |
| `test/` | the test overlay, its variables and the collector overlay `file` of the smoke test; test only, not in the release bundle ([test/README.md](test/README.md)) |

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
