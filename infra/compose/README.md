# infra/compose

Docker-Compose-Stack einer Bothy-Instanz. Regeln: [docs/spec/16-betrieb.md](../../docs/spec/16-betrieb.md).
Anleitung: [docs/operations/install-compose.md](../../docs/operations/install-compose.md).

## Dateien

| Datei | Inhalt |
|---|---|
| `compose.yaml` | alle Dienste, Profile, Netze, Volumes |
| `.env.example` | Vorlage für `.env`; nur Platzhalter und Standardwerte, keine Geheimnisse |
| `secrets/` | nicht im Repository: vom Owner gelieferte Geheimnisse als Dateien (Katalog in [configuration.md](../../docs/operations/configuration.md) 5) |

Der Stack bindet außerdem `../otel` (Collector) und `../observability` (Prometheus, Alertmanager,
Loki, Tempo, Grafana) nur lesend ein.

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
| `scan` | `clamav` | braucht viel Speicher (Hersteller: mindestens 3 GiB) |
| `s3` | `seaweedfs` | S3-Treiber auf einem Host; `BOTHY_STORAGE_DRIVER=s3` setzen |
| `observability` | `prometheus`, `alertmanager`, `loki`, `tempo`, `grafana` | dazu `BOTHY_OTEL_EXPORTER=local` |

Genau eines von `direct` und `tailscale`.

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

## Geprüft

`docker compose config --quiet` (Docker Compose v5.6.0) mit `.env.example` und allen Profilen am
2026-10-10: gültig. Ein Start war nicht möglich, weil die Bothy-Images noch nicht existieren und
auf dem Prüfrechner kein Docker-Daemon lief. `secrets-init`, `bothy-backupd`, `bothy healthcheck`,
`migrate --bootstrap` und die TLS-Verbindung von pgBackRest sind Verträge für Bau-Tickets.
