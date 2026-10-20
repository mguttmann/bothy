# infra/compose

Docker-Compose-Stack einer Bothy-Instanz. Regeln: [docs/spec/16-betrieb.md](../../docs/spec/16-betrieb.md).
Anleitung: [docs/operations/install-compose.md](../../docs/operations/install-compose.md).

## Dateien

| Datei | Inhalt |
|---|---|
| `compose.yaml` | alle Dienste, Profile, Netze, Volumes |
| `.env.example` | Vorlage für `.env`; nur Platzhalter und Standardwerte, keine Geheimnisse |
| `compose.multi-node.yaml` | overlay with two api nodes behind the example edge (profile `direct` only; section "Several api nodes") |
| `edge/nginx.conf` | example edge (nginx) of the overlay: hash routing of the terminal stream paths, timeouts, no access log |
| `secrets/` | nicht im Repository: vom Owner gelieferte Geheimnisse als Dateien (Katalog in [configuration.md](../../docs/operations/configuration.md) 5) |
| `install.sh` | source of the setup wizard (POSIX `sh`, [16-betrieb.md](../../docs/spec/16-betrieb.md) 3.1); the release ships it rendered with version, checksum and download address next to the bundle (`make release-build`), not inside it; tests in [test/install/](test/install/README.md) (`make install-test`) |
| `smoke.sh` | the Compose smoke test (`make portability`, M1-73); test only |
| `test/` | the test overlay, its variables and the collector overlay `file` of the smoke test; the test overlays `test/compose.terminal.yaml` and `test/compose.terminal-twonode.yaml`, the host image `test/host/Dockerfile` and the TLS front `test/terminal/` of the terminal conformance suite (M2-48); test only, never for an installation, not in the release bundle ([test/README.md](test/README.md)) |

Der Stack bindet außerdem `../otel` (Collector) und `../observability` (Prometheus, Alertmanager,
Loki, Tempo, Grafana) nur lesend ein. The release bundle therefore contains both folders
(docs/spec/18-release.md, section 2).

## Kurzform

```
cp .env.example .env          # Werte setzen
mkdir -m 700 secrets          # gelieferte Geheimnisse ablegen
docker compose config --quiet # prüfen
docker compose up -d
```

## Profile

| Profil | Dienste | Hinweis |
|---|---|---|
| (immer) | `secrets-init`, `postgres`, `migrate`, `worker`, `backup`, `otel-collector` | |
| `direct` | `api` auf `127.0.0.1:${BOTHY_HTTP_PUBLISH_PORT}` | TLS und Anmeldung über Reverse-Proxy oder OIDC |
| `tailscale` | `tailscale`, `api-tailscale` | Header-Identität; siehe [tailscale.md](../../docs/operations/tailscale.md) |
| `scan` | `clamav` | braucht viel Speicher (Hersteller: mindestens 3 GiB); clamd's socket lies in the volume `clamav-socket`, mounted read-only in `worker` |
| `s3` | `seaweedfs` | S3-Treiber auf einem Host; `BOTHY_STORAGE_DRIVER=s3` setzen |
| `observability` | `prometheus`, `alertmanager`, `loki`, `tempo`, `grafana` | dazu `BOTHY_OTEL_EXPORTER=local` |

Genau eines von `direct` und `tailscale`.

## Several api nodes

```
docker compose -f compose.yaml -f compose.multi-node.yaml config --quiet
docker compose -f compose.yaml -f compose.multi-node.yaml up -d
```

The overlay `compose.multi-node.yaml` runs `api` and `api-2` with `BOTHY_CLUSTER__MODE=multi` (the
worker too) and puts the example edge `edge` (nginx, image tag `BOTHY_NGINX_VERSION`) in front of
them. Rules ([16-betrieb.md](../../docs/spec/16-betrieb.md) 4.4,
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

## Sicherheitsregeln in dieser Datei

1. Ports nur auf `127.0.0.1`. PostgreSQL, Loki, Tempo und SeaweedFS hängen nur am Netz `internal`
   ohne Ausgang und sind nie am Host veröffentlicht.
2. Geheimnisse nie in Umgebungsvariablen; jeder Dienst bindet nur sein Unterverzeichnis des
   Volumes `secrets` ein (`subpath`, Docker Engine 26 oder neuer).
3. `api`, `worker`, `migrate`: Dateisystem nur lesend, alle Capabilities entfernt,
   `no-new-privileges`. Nur der Sidecar `tailscale` bekommt `NET_ADMIN` und `/dev/net/tun`
   (Kernel-Netz, siehe [tailscale.md](../../docs/operations/tailscale.md) 2).
4. Images der Hilfsdienste sind auf Versionen gepinnt; Bothy-Images auf `BOTHY_VERSION`.
5. Container-Logs werden vom Log-Treiber rotiert.
6. clamd is reached only over its Unix socket (19 SEC-TLS-5, M3-04): the volume `clamav-socket`
   is mounted at `/run/clamav` in `clamav` and read-only in `worker`, in no other service;
   `clamav` publishes no port, exposes none and has no `TCPSocket` in
   [clamav/clamd.conf](clamav/clamd.conf), which also sets the mandatory options of
   [12-dateien.md](../../docs/spec/12-dateien.md) 5.4. Its health check pings over the socket.

## Geprüft

`docker compose config --quiet` (Docker Compose v5.6.0) mit `.env.example` und allen Profilen am
2026-10-10: gültig. Ein Start war nicht möglich, weil die Bothy-Images noch nicht existieren und
auf dem Prüfrechner kein Docker-Daemon lief. `secrets-init`, `bothy-backupd`, `bothy healthcheck`,
`migrate --bootstrap` und die TLS-Verbindung von pgBackRest sind Verträge für Bau-Tickets.

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
