# hermes-adapter

Status: Mitschnitt vorhanden (M2-32), Adapter geplant (M2-33).

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

- Nur öffentliche Hermes-Plugin-Oberflächen; keine Änderung an Hermes. Wo es keine gibt
  (Anmeldetyp, 17.8), kapselt eine Kompatibilitätsschicht die interne Funktion, ein Vertragstest
  gegen die gepinnte Version hält sie.
- Plattform-Schlüssel (`hermes_adapter.platform_key`) nach Inbetriebnahme nie ändern; sonst
  verliert jeder Thread seine Hermes-Sitzung.
- Globaler `display.busy_input_mode` ausdrücklich `steer` oder `queue` (der Schlüssel je
  Plattform wird nicht gelesen); bei `interrupt` meldet der Adapter `degraded`.
- `display.platforms.<platform_key>.tool_progress: off`, damit Hermes keine Fortschritts-Blasen
  als Nachrichten sendet; Schritte kommen aus `set_status_text`.
- Kein Start ohne gültige Aufruf-Marke; Ist-Modus nur am Schlüsseltyp erkennen, den Wert nie
  speichern, senden oder protokollieren.

## Mitschnitt (M2-32)

Vor dem Bau des Adapters hat M2-32 gemessen, wie die gepinnte Hermes-Version die Oberflächen von
Abschnitt 17 aufruft. Die gepinnte Version steht in `tests/hermes_pin.toml`; Tag, Commit und die
Antworten stehen in docs/spec/09-agenten.md 17.10. Kurz:

- `progress_hooks`: kein Aufruf von `format_tool_event`; Werkzeugschritte über `set_status_text`,
  Entwürfe kumulativ über `send_draft`, ohne Entwürfe ein abschließendes `edit_message`.
- `steer`: wirkt über den globalen Schlüssel und über `/steer`, nicht über den Schlüssel je Plattform.
- `credential_type`: keine öffentliche Oberfläche; die interne Auflösung stimmt in jedem Profil.
- `interrupt_outcome`: `/stop` über `handle_message` liefert `CANCELLED`.
- `session_switch`: `/new` mit Bestätigung, `/resume` im Thread, fremde Sitzungen nur mit `--all`
  für einen Admin-Absender; die Sitzungs-ID kommt aus dem Hook `pre_llm_call`.
- `own_access_policy`: `enforces_own_access_policy` mit `dm_policy = "allowlist"` wirkt.

Der Mitschnitt ist Teil des normalen Testlaufs (`make test`, jedes `make check`, das `plugins`
wählt). Er braucht Netz für die gepinnten Downloads (Hermes-Commit, Python-Pakete, Python) und
scheitert ohne Netz; er überspringt nie.

Neu aufzeichnen (nach einem neuen Pin oder einer Änderung an Probe, Szenarien oder Harness):

```
make -C plugins/hermes-adapter record
```

Danach den Diff in `tests/fixtures/recording/` prüfen und 09 17.10 nachziehen.

## Aufbau der Tests

| Teil | Inhalt |
|---|---|
| `tests/hermes_pin.toml` | gepinnte Hermes-Version (Tag, voller Commit, Python-Linie, Installationsart) |
| `tests/hermes_harness/` | Harness: Pin, Bereitstellung, Integrität, Wegwerf-Homes und Umgebung, Sandbox (macOS), Egress-Rekorder, Fake-Modell, Steuerkanal, Gateway-Prozessgruppe, Normalisierung, Beobachtungen, Szenario-Lauf, Fixtures, benannte Fristen |
| `tests/recording/probe_plugin/` | Probe-Plattform `recording_probe`; einziger Code mit Hermes-Import |
| `tests/recording/scenarios/` | Szenarien je Punkt von 17.10 |
| `tests/recording/test_recording.py` | Live-Mitschnitt gegen die gepinnte Version |
| `tests/fixtures/recording/` | Belege: ein Fixture je Szenario und `observations.json` |
| `tests/unit/` | Tests ohne Netz und ohne Hermes, auch für die eingecheckten Belege |

## Was der Harness zusichert und was nicht

- Der Checkout des gepinnten Commits liegt schreibgeschützt in einem Wegwerf-Ordner außerhalb des
  Repositorys; installiert wird aus einem `git archive`-Export in eine Wegwerf-Umgebung. Checkout
  und installierte Dateien sind nach dem Lauf unverändert, sonst scheitert er.
- Jeder Hermes-Prozess bekommt eine Umgebung aus einer Erlaubnisliste, eigene Homes, einen
  leeren Schlüsselring, verweigernde Programme für Schlüsselbund und Dienste und einen Proxy, der
  alles mit `403` ablehnt und nur das Ziel notiert. Modell und Anmeldung sind Fakes.
- Auf macOS läuft der Hermes-Prozessbaum zusätzlich in `sandbox-exec`: kein Netz außer den
  Loopback-Ports der eigenen Harness-Server, keine Signale und Prozess-Infos außerhalb, kein Lesen
  von Dateiinhalten und kein Schreiben im echten Home außer im Wegwerf-Ordner.
- Auf Linux gibt es keine Netz-Sandbox; die Zusicherung beruht darauf, dass keine echte Anmeldung
  erreichbar ist. Eine durchgerutschte Anfrage trüge nur Fake-Anmeldungen, die bei einem echten
  Anbieter nichts kosten.
- Der Harness öffnet nur Ports auf `127.0.0.1` mit Port 0 und beendet nur die eigene
  Prozessgruppe, nie Prozesse nach Namen.

## Was M2-33 übernimmt

M2-33 baut den Adapter auf diesem Harness: `load_pin`, `provision` und `release`,
`ThrowawayHomes`, `build_environment`, die Sandbox, `EgressRecorder`, `FakeModel` mit
`FakeCredentials`, `HermesGateway`, `snapshot_checkout`, `installed_digest` und
`assert_unchanged`. Statt der Probe läuft dann der Adapter; Pin, Isolation und Integritätsprüfung
bleiben.
