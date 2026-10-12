# Belege des Mitschnitts (M2-32)

Diese Dateien schreibt `make -C plugins/hermes-adapter record` (nie von Hand). Sie belegen die
Antworten in docs/spec/09-agenten.md 17.10 gegen die Version in `tests/hermes_pin.toml`.

| Datei | Inhalt | Belegt |
|---|---|---|
| `<szenario>.json` | Szenario, Pin, `inputs_sha256`, normalisierte Aufrufe der Probe, Form der Modell-Anfragen mit Anmeldeart, Aufrufmuster | den Punkt, den das Szenario in `tests/recording/scenarios/` nennt |
| `observations.json` | Pin, `inputs_sha256`, Aufnahmetag, Plugin-Suche, Oberflächen, Egress, Aufrufe der Schlüsselbund-Attrappen, je Punkt die Fakten und das berechnete Urteil `holds` | alle sechs Punkte |

Regeln:

- Keine Fremdtexte: Systemprompts, Statustexte und Meldungen von Hermes stehen nur als
  `sha256:<hex>:<Länge>`; nur Szenario-Texte, Bezeichner und Platzhalter (`<id:n>`, `<tmp>`,
  `<home>`, `<host>`, `<user>`, `<ws>`, `<partial>`, `<cursor>`) stehen im Klartext.
- Keine Anmeldewerte: die Fake-Anmeldungen leben nur im Speicher und im Wegwerf-Home.
- Verglichen werden die Aufrufmuster (`pattern`) und in `observations.json` Plugin-Suche,
  Oberflächen, Egress und Punkte. Die Zahl der Wiederholungen von `send_draft` und
  Zwischen-Bearbeitungen einer gestreamten Nachricht hängt am Takt und zählt nicht.
  `set_status_text` ruft Hermes aus dem Agenten-Thread; seine Stellung zwischen den übrigen
  Aufrufen hängt an der Thread-Planung, daher steht im Muster nur sein letzter Aufruf, als
  letzter Eintrag.
- Damit das Muster nicht am Takt hängt, ist jede Textantwort der Szenarien getaktet (Pause nach
  jedem Stück), und Szenarien mit Werkzeugaufrufen außerhalb von `progress_hooks` schalten die
  Werkzeug-Fortschrittsmeldungen der Probe ab (`timing_problems` in
  `tests/hermes_harness/scenarios.py`).
- Ändert sich Pin, Probe, ein Szenario oder ein Harness-Modul, scheitern die Offline-Tests, bis
  neu aufgezeichnet ist (`inputs_sha256`).

Neu aufzeichnen: `make -C plugins/hermes-adapter record`, den Diff prüfen, 09 17.10 nachziehen.
