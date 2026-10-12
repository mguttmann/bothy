# services

Server-Code und Host-Dienst des Terminals.

| Ordner | Inhalt | Kapitel |
|---|---|---|
| `server/` | Der Go-Server als modularer Monolith: ein Binary mit den Rollen `api` und `worker` | [02 Architektur](../docs/spec/02-architektur.md) |
| `host/` | Host-Dienst `bothy-host` für das Terminal auf angebundenen Rechnern; heute der Noise-Kern (Handshake, Transport, Rahmen, Fingerabdruck) und das Binary ohne Befehle | [24 Terminal](../docs/spec/24-terminal.md) |

Für die Gates ist jedes getrackte `go.mod` unter `services/` ein Go-Modul (Format, Lint,
Build, Tests mit Abdeckung, Schwachstellen, Lizenzen); Einzelheiten in
[docs/quality/gates.md](../docs/quality/gates.md). Releases des Servers tragen Tags
`server/v<version>` und brauchen `server/VERSION`, `server/CHANGELOG.md` und das Ziel
`release-build` in `server/Makefile` (Kapitel 23, Abschnitt 3); die des Host-Dienstes tragen
`host/v<version>` mit `host/VERSION`, `host/CHANGELOG.md` und `release-build` in
`host/Makefile`.
