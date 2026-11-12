# Installation with Docker Compose

Status: draft M0. Guide for an instance on one host. Rules:
[16-operations.md](../spec/16-operations.md). Files: [infra/compose/](../../infra/compose/README.md).

Two equivalent paths: the setup wizard in one command (section 1.1) or the manual path
(sections 2 to 4). Both create the same instance.

Until the first release no registry holds the images `bothy-server` and `bothy-postgres` and
there is no install bundle, so the wizard and the registry of `.env.example` work from the first
release on. Until then an instance runs on images built from the source (section 1.2) and
follows the manual path.

## 1. Prerequisites

| Item | Requirement |
|---|---|
| Runtime | Docker Engine 26 or newer with Compose 2.26 or newer; on the Mac Docker Desktop or Colima |
| Architecture | linux/amd64 or linux/arm64 |
| Memory of the Docker VM (Mac, Windows) | enough for all services; ClamAV alone needs at least 3 GiB (vendor) |
| Disk | according to the model in [16-operations.md](../spec/16-operations.md) 13 |
| Network | egress to push services, OIDC provider, off-site target, ClamAV signatures |
| Edge | a reverse proxy with TLS (profile `direct`) or a tailnet (profile `tailscale`) |
| Off-site target | S3 bucket with Object Lock or SFTP with append-only access |

### 1.1 One command: setup wizard

With the prerequisites met (Docker and Compose installed), one command sets up the instance,
starts it and creates the first owner ([16-operations.md](../spec/16-operations.md), section 3.1):

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh | sh
```

The wizard asks for the public address, the edge (`direct` or `tailscale`), virus scan, file
storage (`fs` or `s3`), observability stack, off-site target (optional) and the first owner,
writes `.env` and `secrets/`, enters the gateway of the Compose network in
`BOTHY_TRUSTED_PROXIES` for the edge `direct` (section 5.1), starts the stack, waits for
readiness, checks the published port and shows the enrollment code once. Without a terminal (CI,
cloud-init) it takes all answers as options; the portability test passes these options too
([16-operations.md](../spec/16-operations.md), 3.1 and 11.1):

```sh
curl -fsSL https://github.com/OWNER/bothy/releases/download/v0.0.0/install.sh \
  | sh -s -- --yes --public-url https://chat.example.com --edge direct --scan on --storage s3 \
      --owner-handle alex --owner-name Alex
```

`TS_AUTHKEY` for the edge `tailscale` comes from a hidden prompt or, without a terminal, from the
environment, never from an option. The target folder is `--dir` (default `./bothy`); the stack
then runs in `bothy/infra/compose` (`cd bothy/infra/compose && docker compose ps`). The wizard
never calls `sudo` and changes nothing outside its folder; a second run in the same folder keeps
`.env` and `secrets/`. With a bundle that is already downloaded (`gh release download
v<version>`), `--bundle bothy-compose-<version>.tar.gz` replaces the download; the checksum
check stays.

Whoever wants to check the script before running it downloads it with `SHA256SUMS` and the
signature and checks both like any release artifact ([release.md](release.md), section 5); then
`sh install.sh`. The wizard does not create supplied secrets (push keys, access to the off-site
target, alert URLs). They go into `secrets/` as in the manual path (section 2, item 3); then
`docker compose up -d` in the target folder.

### 1.2 From source (until the first release)

The stack reads its two images as `${BOTHY_REGISTRY}/bothy-server:${BOTHY_VERSION}` and
`${BOTHY_REGISTRY}/bothy-postgres:pg18-${BOTHY_VERSION}`. Until a release publishes them, build
both on the host of the instance from a checkout of the repository. This needs git, GNU make and
Docker with the buildx plugin ([CONTRIBUTING.md](../../CONTRIBUTING.md), "Development setup").
Each image gets a local name and the commit as its tag:

```sh
git clone https://github.com/OWNER/bothy.git
cd bothy
tag="src-$(git rev-parse --short HEAD)"
make image IMAGE_NAME=localhost/bothy-server IMAGE_TAG="$tag"
make postgres-image POSTGRES_IMAGE_NAME=localhost/bothy-postgres POSTGRES_IMAGE_TAG="pg18-$tag"
```

Both targets build for `linux/amd64` and `linux/arm64` and load the image of the host's platform
into the local image store; that needs a buildx builder that builds several platforms
(`make image` prints how to create one). `IMAGE_PLATFORMS=linux/arm64` (or `linux/amd64`) on
both commands builds the host's platform only. The server image also builds the admin console,
so its first build takes a while.

Then follow the manual path from section 2 in `infra/compose/` of this checkout, with these two
values in `.env` (the tag as printed by `echo "$tag"`):

```
BOTHY_REGISTRY=localhost
BOTHY_VERSION=src-<commit>
```

`localhost` is no registry that publishes images: Compose takes both images from the local
store, and a start without them fails instead of fetching something else.

Update to a newer state of the source: check that state out, build both images again (the new
commit gives a new tag), set `BOTHY_VERSION` to it and follow [upgrade.md](upgrade.md) without
`docker compose pull`, which has nothing to pull from; `docker compose up -d` replaces the
containers. The images of the previous tag stay in the local store until someone removes them,
so the rollback of [upgrade.md](upgrade.md), section 3, is the previous value of `BOTHY_VERSION`.

## 2. Preparation (manual path)

1. Get the repository state of the desired version (tag `server/vX.Y.Z`) and change into
   `infra/compose/`.
2. Copy `.env.example` to `.env` and set: `BOTHY_REGISTRY`, `BOTHY_VERSION`,
   `BOTHY_PUBLIC_URL`, `COMPOSE_PROFILES`, `BOTHY_BACKUP_OFFSITE_TARGET`.
3. Create the directory `secrets/` (`chmod 700`) and place the supplied secrets there
   ([configuration.md](configuration.md) 5): at least `offsite/`, for push `apns_key.p8` and
   `fcm_service_account.json`, for alerts the three webhook URLs, for telemetry the keys
   of the chosen overlay.
4. Check: `docker compose config --quiet`.
5. Profile `direct` only: `docker compose up --no-start` (creates networks and containers, starts
   nothing), read the gateway according to section 5.1 and set
   `BOTHY_TRUSTED_PROXIES=<gateway>/32` in `.env` (IPv6 `/128`). The start in section 3 recreates
   the containers with the new value. Without this step, the server rejects every request over
   the published port (section 5.1).

## 3. Start

```
docker compose up -d
```

Order that Compose enforces: `secrets-init` (once), `postgres`, `migrate`
(once; allowed on the empty database without a backup), then `api`, `worker`, `backup`,
`otel-collector` and the profiles.

First owner (only via a server command, Baseline 3):

```
docker compose exec api bothy setup bootstrap-owner --handle <handle> --display-name "<Name>"
```

The command prints an enrollment code once; with it, the owner redeems their first device in the
app ([08-identity.md](../spec/08-identity.md), section 4). If all owners later lose their
devices, `docker compose exec api bothy setup issue-enrollment-code --user <handle>`
issues a code for recovery. Access to the host is the root of trust of the instance
([16-operations.md](../spec/16-operations.md), section 4.3): shell and container runtime only for
people who could also be owners.

## 4. Check after the start

| Check | Command or place | Expectation |
|---|---|---|
| Services | `docker compose ps` | all `healthy` or `running`; `secrets-init`, `migrate` with exit 0 |
| Readiness | `docker compose exec api bothy healthcheck --url http://127.0.0.1:8081/readyz` | exit 0 |
| Secrets | output of `secrets-init` | fingerprints; escrow notice followed |
| First backup | `docker compose exec backup bothy-backupd backup pg_full` (then `logical` and, with the driver `fs`, `objects`) | exit 0, `bothy-backupd: backup pg_full succeeded`; run `succeeded` in the admin console |
| First restore test | `docker compose exec backup bothy-backupd restore-test` | exit 0, all checks green |
| Status page | app, Status section | all components `operational` |

With the profile `observability` the alerts `BothyBackupStale` and `BothyRestoreTestStale` fire
while a backup kind (`physical` for `pg_full` and `pg_diff`, `logical`, `objects`) or the restore
test has not succeeded yet: they treat a missing last success like one that is too old
([alerting.md](../observability/alerting.md) 3.2). The first backup of every kind and the first
restore test end them. With the driver `s3` the kind `objects` mirrors the file bucket into the
off-site bucket: with an off-site target run it by hand as well; without one every run of it
fails with `s3_mirror_not_configured` (docs/decisions.d/M7-02.md D-M7-02-5), so the alert stays
for `objects` until a target is set or the alert is silenced for that kind in Alertmanager.

## 5. Edge

### 5.1 Profile `direct`

The server is reachable on `127.0.0.1:BOTHY_HTTP_PUBLISH_PORT` of the host. A reverse proxy
terminates TLS and forwards, including WebSocket upgrades on `/api/v1/gateway`. The server does
not see the proxy as the peer, but the gateway of the Compose network. Its address
belongs in `BOTHY_TRUSTED_PROXIES` (sets `server.trusted_proxies`):

```
docker network inspect <project>_default --format '{{(index .IPAM.Config 0).Gateway}}'
```

Without this entry, the server rejects every plaintext connection (SEC-TLS-1), and rate limits per
client IP are wrong. Login: OIDC or local accounts; header identity is disabled in this
profile.

The proxy sends the address of the client in `X-Forwarded-For` and must remove a `Forwarded`
header from the request, which it would otherwise pass on as the client sent it (nginx:
`proxy_set_header Forwarded "";` next to the line that sets `X-Forwarded-For`). A proxy that sends
`Forwarded` (RFC 7239) instead removes `X-Forwarded-For`. The server reads one of the two headers:
a request that arrives with both counts as a request of the proxy itself, so everyone who sends
such requests shares one rate limit ([16-operations.md](../spec/16-operations.md), section 4.3).

### 5.2 Profile `tailscale`

See [tailscale.md](tailscale.md).

### 5.3 Several api nodes (overlay)

The overlay `infra/compose/compose.multi-node.yaml` runs two api nodes, `api` and `api-2`, behind
an example edge (nginx in the image `nginxinc/nginx-unprivileged`, tag `BOTHY_NGINX_VERSION`):

```
docker compose -f compose.yaml -f compose.multi-node.yaml up -d
```

1. Profile `direct` only. The profile `tailscale` runs one api node, because `tailscale serve` has
   no hash rule; with it the overlay starts neither `edge` nor `api-2`.
2. Only `edge` publishes a port, on `127.0.0.1:BOTHY_HTTP_PUBLISH_PORT`; the api nodes publish
   none. Your TLS reverse proxy stays in front of `edge` and forwards to that port as in 5.1.
3. `edge` routes `/api/v1/terminal/sessions/{session_id}/stream` and
   `/api/v1/hosts/sessions/{session_id}/stream` by consistent hashing over the session ID to the
   same api node, where the relay pairs them. A side that lands on another node closes after
   `terminal.open_timeout` with `4102` and counts in `bothy.terminal.relay.unpaired`; check that
   metric after changing the edge.
4. The server sees the address of `edge`, which appends to `X-Forwarded-For` the address it sees
   for your proxy, the gateway of the network, and removes `Forwarded` (5.1). In front of `edge`
   your proxy therefore sends the address of the client in `X-Forwarded-For`, and
   `BOTHY_TRUSTED_PROXIES` names the subnet of the network `<project>_default`, not only its
   gateway:

   ```
   docker network inspect <project>_default --format '{{(index .IPAM.Config 0).Subnet}}'
   ```

5. `edge` restarts when an api node is recreated (for example by an upgrade), because nginx
   resolves the node names only at start.
6. Both api nodes share the key store volume `signing-keys` (ADR 0066) and run with
   `cluster.mode = multi`, as the worker does. What crosses the nodes and how failures show is
   in [multi-node.md](multi-node.md).
7. The idle timeouts of `edge` follow the defaults of `terminal.idle_timeout` plus
   `terminal.idle_grace` on the stream paths, of `agents.interview.timeout` plus
   `http.request_timeout` on `/api/v1/agents/interview` and of `terminal.host_heartbeat_interval`
   times `terminal.host_heartbeat_miss_limit` elsewhere. If you change these keys, change
   `infra/compose/edge/nginx.conf` and the timeouts of your own proxy with them.

### 5.4 Panel plugin "Bothy Live" in Grafana

The profile `observability` starts Grafana with the panel plugin `bothy-live-panel` and the
dashboard `bothy-live` ("Bothy / Live"); nothing else is to switch on
([25-visualization.md](../spec/25-visualization.md), section 8.3):

1. The plugin build comes with the bundle in
   `infra/observability/grafana/plugins/bothy-live-panel/`; Compose mounts it read-only into
   Grafana. An upgrade of the product brings the plugin version of that release.
2. The plugin is unsigned until V1 (ruling T11). Grafana loads it only because
   `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS` in `compose.yaml` names exactly its ID. Do not
   widen this setting (no `*`, no further IDs, no override file): `make ops-check` rejects it
   (SEC-VZ-5), and every unsigned plugin is code that runs in the browser of every Grafana user.
3. Check: Grafana on `127.0.0.1:${BOTHY_GRAFANA_PUBLISH_PORT}` (default `3300`), Dashboards,
   folder Bothy, "Bothy / Live"; TV mode is `/d/bothy-live?kiosk`. Grafana's log names
   `bothy-live-panel` as unsigned and permitted (`docker compose logs grafana | grep -i unsigned`)
   and no other plugin.
4. In a checkout instead of the bundle the folder is empty until
   `make -C plugins/grafana-bothy-live stack-install` puts the release ZIP of the plugin's
   `VERSION` there; Grafana then shows the panels as missing until it is restarted.

## 6. Mac notes

1. Docker runs in a VM; volumes live on its disk. The VM needs enough memory and
   disk; both can only be changed while the VM is stopped.
2. With Colima, the VM does not start by itself after a restart of the Mac if no service
   starts it. The instance operator sets up the autostart (for example `brew services start colima`
   [assumed, not checked]).
3. Sleep interrupts everything. A server Mac needs "Prevent automatic sleeping".

## 7. Update and remove

Update: [upgrade.md](upgrade.md). Remove without data: `docker compose down`. With all
data: `docker compose down --volumes`; check a final backup and the secrets bundle first,
because afterwards the database, files and local backups are gone.

## 8. Moving from Discord

An installation that takes over an existing Discord server with agent bots follows
[discord-migration.md](discord-migration.md): dry run, pilot, parallel operation and switch-over
channel by channel.
