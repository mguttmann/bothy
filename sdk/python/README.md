# sdk/python

Status: geplant. Dieser Ordner enthält noch keinen Code.

## Zweck

Python-SDK für Agenten-Adapter (darunter der Hermes-Adapter) und Apps.

## Aufbau (geplant)

| Schicht | Inhalt |
|---|---|
| Kern | erzeugt mit openapi-python-client aus `docs/api/openapi.yaml`; nie von Hand geändert; im Gate erzeugt und geprüft |
| Handschicht | Module `auth`, `gateway`, `invocation`, `runs`, `components`, `commands`, `outbox`, `retry`, `heartbeat`, `webhooks`, `testing` nach docs/spec/11-erweiterungen.md Abschnitt 11.2 |

## Leitlinien

- Asynchrone API zuerst, synchrone Hülle für Skripte.
- Unterstützt werden die CPython-Versionen, die laut python.org gepflegt werden.
- Wenige Abhängigkeiten, verträglich mit dem Umfeld von Hermes Agent (Prüfung im Fake-Hermes-Test).
- Prüfung der Aufruf-Marke nur mit `EdDSA` und `typ = agent-invocation+jwt`
  (docs/spec/09-agenten.md Abschnitt 4.5).
- Fehler typisiert nach dem Problem-Format aus docs/spec/06-api.md.

## Testdoppel

`testing` liefert einen FakeServer im Prozess (127.0.0.1, Port 0) mit Fehlerinjektion und die
Vertragsszenarien aus docs/spec/09-agenten.md Abschnitt 16, damit jeder Adapter sie gegen sich
laufen lassen kann.
