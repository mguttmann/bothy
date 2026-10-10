# services

Server-Code. Heute nur Planung, kein Code.

| Ordner | Inhalt | Kapitel |
|---|---|---|
| `server/` | Der Go-Server als modularer Monolith: ein Binary mit den Rollen `api` und `worker` | [02 Architektur](../docs/spec/02-architektur.md) |

Für die Gates ist jedes getrackte `go.mod` unter `services/` ein Go-Modul (Format, Lint,
Build, Tests mit Abdeckung, Schwachstellen, Lizenzen); Einzelheiten in
[docs/quality/gates.md](../docs/quality/gates.md). Releases des Servers tragen Tags
`server/v<version>` und brauchen `server/VERSION`, `server/CHANGELOG.md` und das Ziel
`release-build` in `server/Makefile` (Kapitel 23, Abschnitt 3).
