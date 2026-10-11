# apps

Status: geplant (M0). Noch kein Code.

| App | Zweck | Plattformen | Technik |
|---|---|---|---|
| [client](client/README.md) | Chat mit Agenten, Live-Box, Benachrichtigungen, Offline, Verwaltung auf Desktop, Notfall-Aktionen mobil | iOS, Android, macOS, Windows | Flutter (ADR 0027) |
| [console](console/README.md) | Verwaltung und Statistik im Browser, kein Chat | Web | Flutter Web (ADR 0028) |

Beide Apps nutzen die Pakete aus [`packages/`](../packages/README.md). Architektur:
[docs/spec/15-client-architektur.md](../docs/spec/15-client-architektur.md). Bedienung und Screens:
[docs/spec/01-produkt.md](../docs/spec/01-produkt.md). Gestaltung: [docs/design/](../docs/design/README.md).

Gemeinsame Regeln:

- Keine Rechte-Logik in den Apps; der Server entscheidet (B7).
- Plattform-Code nur in `client/{ios,macos,android,windows}` und `packages/device_key`.
- Branding einer Instanz kommt zur Laufzeit vom Server und zur Bauzeit aus der Build-Konfiguration
  der Instanz (ADR 0044); im Repo liegt nur ein Beispiel mit `example.com`.
