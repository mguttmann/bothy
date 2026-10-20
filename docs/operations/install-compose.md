# Installation mit Docker Compose

Status: Entwurf M0. Anleitung für eine Instanz auf einem Host. Regeln:
[16-betrieb.md](../spec/16-betrieb.md). Dateien: [infra/compose/](../../infra/compose/README.md).

Hinweis: Die Images `bothy-server` und `bothy-postgres` entstehen mit dem ersten Release. Bis
dahin ist diese Anleitung der Vertrag, den das Release und der Portabilitätstest erfüllen.

## 1. Voraussetzungen

| Punkt | Anforderung |
|---|---|
| Laufzeit | Docker Engine 26 oder neuer mit Compose 2.26 oder neuer; auf dem Mac Docker Desktop oder Colima |
| Architektur | linux/amd64 oder linux/arm64 |
| Speicher der Docker-VM (Mac, Windows) | genug für alle Dienste; ClamAV allein braucht mindestens 3 GiB (Hersteller) |
| Platte | nach dem Modell in [16-betrieb.md](../spec/16-betrieb.md) 13 |
| Netz | Ausgang zu Push-Diensten, OIDC-Anbieter, Off-site-Ziel, ClamAV-Signaturen |
| Rand | ein Reverse-Proxy mit TLS (Profil `direct`) oder ein Tailnet (Profil `tailscale`) |
| Off-site-Ziel | S3-Bucket mit Object Lock oder SFTP mit Append-only-Zugang |

## 2. Vorbereitung

1. Repository-Stand der gewünschten Version holen (Tag `server/vX.Y.Z`) und in `infra/compose/`
   wechseln.
2. `.env.example` nach `.env` kopieren und setzen: `BOTHY_REGISTRY`, `BOTHY_VERSION`,
   `BOTHY_PUBLIC_URL`, `COMPOSE_PROFILES`, `BOTHY_BACKUP_OFFSITE_TARGET`.
3. Verzeichnis `secrets/` anlegen (`chmod 700`) und gelieferte Geheimnisse ablegen
   ([configuration.md](configuration.md) 5): mindestens `offsite/`, für Push `apns_key.p8` und
   `fcm_service_account.json`, für Alarme die drei Webhook-URLs, für Telemetrie die Schlüssel
   des gewählten Overlays.
4. Prüfen: `docker compose config --quiet`.

## 3. Start

```
docker compose up -d
```

Reihenfolge, die Compose durchsetzt: `secrets-init` (einmalig), `postgres`, `migrate`
(einmalig; auf der leeren Datenbank ohne Sicherung erlaubt), dann `api`, `worker`, `backup`,
`otel-collector` und die Profile.

Erster Owner (nur per Server-Befehl, Baseline 3):

```
docker compose exec api bothy setup bootstrap-owner --handle <handle> --display-name "<Name>"
```

Der Befehl gibt einmalig einen Einladungs-Code aus; damit löst der Owner in der App sein erstes
Gerät ein ([08-identitaet.md](../spec/08-identitaet.md), Abschnitt 4). Haben später alle Owner ihre
Geräte verloren, stellt `docker compose exec api bothy setup issue-enrollment-code --user <handle>`
einen Code zur Wiederherstellung aus. Der Zugang zum Host ist die Vertrauenswurzel der Instanz
([16-betrieb.md](../spec/16-betrieb.md), Abschnitt 4.3): Shell und Container-Laufzeit nur für
Personen, die auch Owner sein dürften.

## 4. Prüfung nach dem Start

| Prüfung | Befehl oder Ort | Erwartung |
|---|---|---|
| Dienste | `docker compose ps` | alle `healthy` oder `running`; `secrets-init`, `migrate` mit Exit 0 |
| Bereitschaft | `docker compose exec api bothy healthcheck --url http://127.0.0.1:8081/readyz` | Exit 0 |
| Geheimnisse | Ausgabe von `secrets-init` | Fingerabdrücke; Hinweis auf Hinterlegung befolgt |
| Erste Sicherung | `docker compose exec backup bothy-backupd run --now full` | Lauf `succeeded` in der Admin-Konsole |
| Erster Wiederherstellungstest | `docker compose exec backup bothy-backupd run --now restore-test` | alle Prüfungen grün |
| Statusseite | App, Bereich Status | alle Komponenten `operational` |

## 5. Rand

### 5.1 Profil `direct`

Der Server ist auf `127.0.0.1:BOTHY_HTTP_PUBLISH_PORT` des Hosts erreichbar. Ein Reverse-Proxy
terminiert TLS und leitet weiter, auch WebSocket-Upgrades auf `/api/v1/gateway`. Der Server sieht
dabei nicht den Proxy, sondern das Gateway des Compose-Netzes als Gegenstelle. Dessen Adresse
gehört in `BOTHY_TRUSTED_PROXIES` (setzt `server.trusted_proxies`):

```
docker network inspect <projekt>_default --format '{{(index .IPAM.Config 0).Gateway}}'
```

Ohne diesen Eintrag weist der Server jede Klartext-Verbindung ab (SEC-TLS-1), und Rate-Limits je
Client-IP stimmen nicht. Anmeldung: OIDC oder lokale Konten; Header-Identität ist in diesem Profil
gesperrt.

### 5.2 Profil `tailscale`

Siehe [tailscale.md](tailscale.md).

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
   for your proxy, the gateway of the network. `BOTHY_TRUSTED_PROXIES` therefore names the subnet of the network
   `<project>_default`, not only its gateway:

   ```
   docker network inspect <project>_default --format '{{(index .IPAM.Config 0).Subnet}}'
   ```

5. `edge` restarts when an api node is recreated (for example by an upgrade), because nginx
   resolves the node names only at start.
6. Both api nodes share the key store volume `signing-keys` (ADR 0066) and run with
   `cluster.mode = multi`, as the worker does.
7. The idle timeouts of `edge` follow the defaults of `terminal.idle_timeout` plus
   `terminal.idle_grace` on the stream paths and of `terminal.host_heartbeat_interval` times
   `terminal.host_heartbeat_miss_limit` elsewhere. If you change these keys, change
   `infra/compose/edge/nginx.conf` and the timeouts of your own proxy with them.

## 6. Mac-Hinweise

1. Docker läuft in einer VM; Volumes liegen in deren Platte. Die VM braucht genug Speicher und
   Platte; beides lässt sich nur bei gestoppter VM ändern.
2. Bei Colima startet die VM nicht von selbst nach einem Neustart des Macs, wenn kein Dienst sie
   startet. Der Betreiber richtet den Autostart ein (z. B. `brew services start colima`
   [vermutet, nicht geprüft]).
3. Ruhezustand unterbricht alles. Ein Server-Mac braucht "Ruhezustand verhindern".

## 7. Aktualisieren und Entfernen

Aktualisieren: [upgrade.md](upgrade.md). Entfernen ohne Daten: `docker compose down`. Mit allen
Daten: `docker compose down --volumes`; vorher eine Abschluss-Sicherung und das Secrets-Bündel
prüfen, denn danach sind Datenbank, Dateien und lokale Sicherungen weg.
