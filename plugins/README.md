# plugins

Status: geplant. Dieser Ordner enthält noch keinen Code.

## Zweck

Referenz-Adapter und Referenz-Apps. Jedes Programm hier läuft außerhalb des Servers und nutzt nur
die öffentliche API (ADR 0022). Nichts in diesem Ordner wird in den Server gelinkt oder von ihm
geladen.

## Inhalt

| Ordner | Art | Zweck | Stand |
|---|---|---|---|
| `hermes-adapter/` | Agenten-Adapter | Plattform-Plugin für das Open-Source-Framework Hermes Agent | geplant |
| `claude-code-runner/` | Agenten-Adapter | Startet Claude-Code-Sitzungen je Thread in Dev-Foren | geplant |
| weitere Ordner je Referenz-App | App | Beispiele aus docs/spec/11-erweiterungen.md Abschnitt 10 (Code-Hosting, Tickets) | geplant, Ordner entsteht mit dem Bau-Ticket |

## Regeln für alle Programme hier

- Vertrag: docs/spec/09-agenten.md (Agenten) bzw. docs/spec/11-erweiterungen.md (Apps).
- Nur über das SDK (`sdk/`) oder die öffentliche API; keine Server-internen Pakete.
- Keine Geheimnisse im Repo, keine Zugangsdaten in Protokollen; Konfiguration über Umgebung oder
  Datei, Geheimnisse über den Secret-Provider des jeweiligen Programms.
- Eigene Versionsnummer (SemVer) und eigenes Changelog je Ordner.
- Tests mit den Testdoppeln aus docs/spec/09-agenten.md Abschnitt 16; echte Modellaufrufe nur
  hinter einem ausdrücklichen Schalter.
- White Label: keine Firmen-, Personen- oder Hostnamen; Beispiele mit example.com.
