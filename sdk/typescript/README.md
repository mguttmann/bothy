# sdk/typescript

Status: geplant. Dieser Ordner enthält noch keinen Code.

## Zweck

TypeScript-SDK für Apps, Integrationen und Agenten-Adapter in JavaScript-Umgebungen.

## Aufbau (geplant)

| Schicht | Inhalt |
|---|---|
| Kern | Typen erzeugt mit openapi-typescript aus `docs/api/openapi.yaml`; Laufzeit-Client darauf aufbauend (Wahl im Bau-Ticket); im Gate erzeugt und kompiliert |
| Handschicht | Module `auth`, `gateway`, `invocation`, `runs`, `components`, `commands`, `outbox`, `retry`, `heartbeat`, `webhooks`, `testing` nach docs/spec/11-erweiterungen.md Abschnitt 11.2 |

## Leitlinien

- ESM; aktive LTS-Linien von Node.js.
- Im Browser nur Teile ohne Geheimnisse; Agenten- und App-Tokens gehören nie in einen Browser.
- Prüfung der Aufruf-Marke und der Webhook-Signaturen mit einer JOSE-Bibliothek, die EdDSA und eine
  Algorithmen-Erlaubnisliste kann (Kandidat und Beleg in docs/spec/11-erweiterungen.md
  Abschnitt 11.3).
- Fehler typisiert nach dem Problem-Format aus docs/spec/06-api.md.

## Testdoppel

`testing` liefert denselben FakeServer und dieselben Vertragsszenarien wie das Python-SDK.
