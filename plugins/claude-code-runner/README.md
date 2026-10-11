# claude-code-runner

Status: geplant. Dieser Ordner enthält noch keinen Code.

## Zweck

Referenz-Adapter für Dev-Foren: Ein Forum mit Projektbindung ist ein Projekt, jeder Thread darin
ist eine Claude-Code-Sitzung. Der Runner ist ein eigenständiger Prozess auf dem Rechner, auf dem
die Projekte liegen. Er startet je Turn `claude -p` mit stream-json, füllt die Live-Box aus dem
Stream und gibt Nachträge über stdin in den laufenden Turn.

## Vertrag

- Allgemein: docs/spec/09-agenten.md Abschnitte 4 bis 16.
- Runner-spezifisch: docs/spec/09-agenten.md Abschnitt 18 (Prozessvertrag, Umgebung als
  Erlaubnisliste, eigene Prozessgruppe, Stopp per SIGINT, Sperre je Sitzung, Live-Box aus dem
  Stream, Nachträge, Kosten aus `modelUsage`, Fehlerpfade, Datei-Regeln).
- Backend-Richtlinie je Lauf: docs/spec/10-backend-richtlinie.md Abschnitt 10.2.
- Abschottung für mehrere Personen: docs/spec/21-dev-isolation.md.

## Feste Regeln

- Projektordner nur unter `runner.projects_root`; der Server nennt nie einen absoluten Pfad.
- Umgebung des Kindprozesses ist eine Erlaubnisliste; Anmelde-Variablen nur nach dem Soll der
  Aufruf-Marke.
- Ist-Modus vor dem Start mit `claude auth status` in der Lauf-Umgebung prüfen, Gegenprobe
  `apiKeySource`; bei Abweichung kein Turn.
- Der Runner liest, kopiert oder schreibt nie Anmeldedateien einer Laufzeit.
- Keine automatische Wiederholung eines abgebrochenen Turns.
- `--dangerously-skip-permissions` nur bei Werkzeugklasse `full` und nur, wenn die Schutzregeln aus
  docs/spec/07-rechte.md das erlauben.

## Geplanter Aufbau

| Teil | Inhalt |
|---|---|
| Hauptprozess | Gateway-Verbindung, Marke prüfen, Läufe verteilen, Heartbeat mit Projekt-Bereitschaft |
| Turn-Prozess | ein `claude -p` je Turn, abgekoppelt, eigene Prozessgruppe |
| Stream-Leser | Abbildung der Stream-Nachrichten auf das Lauf-Dokument |
| Zustand | `runner.state_dir`: Sitzungen, Sperren, Mitschnitte, Eingangsdateien |
| Tests | Fake-Claude (abgespielte Fixtures); Test gegen die installierte, gepinnte CLI-Version für erkannte Ereignisse und Fehlertexte; echte Läufe nur hinter einem Schalter |

## Vor dem Bau zu prüfen

- Kommt `system/init` bei `--input-format stream-json` vor der ersten Modellanfrage?
- Ist `/compact` im Druckmodus der gepinnten Version verfügbar?
