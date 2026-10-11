# Modul generic-vm

Richtet einen vorhandenen Linux-Host für Bothy ein. Basis aller VM-Ziele; die Module `azure`,
`aws` und `hetzner` rufen es nach dem Anlegen ihres Hosts auf.

## Eingaben

Gemeinsame Eingaben siehe [variables.tf](variables.tf). Zusätzlich: `ssh_host`, `ssh_user`.

## Geplante Ressourcen

| Teil | Inhalt |
|---|---|
| cloud-init bzw. Bereitstellung per SSH | Docker Engine und Compose-Plugin aus den Paketquellen von Docker, Benutzer `bothy`, Verzeichnis für den Stack |
| Compose-Bündel | `infra/compose` und `infra/otel`, `infra/observability` der Version `bothy_version`, Signatur geprüft ([18-release.md](../../../../docs/spec/18-release.md) 6) |
| `.env` | aus den Eingaben erzeugt; enthält keine Geheimnisse |
| Systemdienst | startet `docker compose up -d` beim Boot |
| Rand | `edge_mode = reverse_proxy`: Caddy oder ein anderer Proxy auf dem Host mit TLS; `tailscale`: Profil `tailscale`; `private`: kein öffentlicher Port |

## Ausgaben

Siehe [infra/tofu/README.md](../../README.md). `database_endpoint` und `object_storage_bucket` bleiben
leer, außer die Eingaben verweisen auf externe Dienste.

## Offen

Welches Werkzeug den Host per SSH einrichtet (cloud-init für neue Hosts, ein Provider für
vorhandene Hosts), entscheidet das Bau-Ticket.
