# hermes-adapter

Status: geplant. Dieser Ordner enthält noch keinen Code.

## Zweck

Referenz-Adapter für Hermes Agent (https://github.com/NousResearch/hermes-agent), ein quelloffenes
Agenten-Framework. Der Adapter ist ein Hermes-Plattform-Plugin (`kind: platform`). Er verbindet
ein Hermes-Profil als Agenten-Prinzipal mit dem Server: Aufrufe kommen über das Gateway, Antworten,
Live-Box und Komponenten gehen über die öffentliche API.

Hermes ist ein Adapter unter mehreren. Der Server hängt nicht von Hermes ab.

## Vertrag

- Allgemein: docs/spec/09-agenten.md Abschnitte 4 bis 16.
- Hermes-spezifisch: docs/spec/09-agenten.md Abschnitt 17 (Abbildung der
  `BasePlatformAdapter`-Methoden, Sitzungsschlüssel über die Thread-ID, `busy_input_mode`,
  Prüfung der Aufruf-Marke vor `handle_message`, Erlaubnisliste, Ist-Modus).
- Backend-Richtlinie: docs/spec/10-backend-richtlinie.md Abschnitt 10.1.

## Feste Regeln

- Nur öffentliche Hermes-Plugin-Oberflächen; keine Änderung an Hermes.
- Plattform-Schlüssel (`hermes_adapter.platform_key`) nach Inbetriebnahme nie ändern; sonst
  verliert jeder Thread seine Hermes-Sitzung.
- `busy_input_mode` ausdrücklich `steer` oder `queue`; bei `interrupt` meldet der Adapter
  `degraded`.
- Kein Start ohne gültige Aufruf-Marke; Ist-Modus nur am Schlüsseltyp erkennen, nie den Wert
  lesen, speichern oder senden.

## Geplanter Aufbau

| Teil | Inhalt |
|---|---|
| `plugin.yaml` | Manifest des Hermes-Plugins |
| Adapter-Modul | einziges Modul mit Hermes-Import; Unterklasse von `BasePlatformAdapter` |
| Logik-Module | Abbildung, Kontext-Format, Erlaubnisliste; ohne Hermes-Import, eigenständig testbar |
| Installer | schreibt Plugin-Konfiguration und Profil-Einstellungen, prüft sie mit Hermes' eigenem YAML-Leser vorher und nachher |
| Tests | Logik-Tests; Fake-Hermes-Tests im Unterprozess mit gepinnter Hermes-Version, Wegwerf-`HERMES_HOME` und Fake-Modell-Endpunkt |

## Vor dem Bau zu prüfen

Siehe docs/spec/09-agenten.md Abschnitt 17.10.
