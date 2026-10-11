# apps/console

Status: geplant (M0). Noch kein Code.

Web-Konsole für Verwaltung und Statistik, gebaut mit Flutter Web (ADR 0028). Kein Chat im Browser;
Links auf Threads öffnen die App.

## Zweck

- Personen, Gruppen, Freigaben mit Klartext-Vorschau, "Wer hat Zugriff?", "Als Person ansehen".
- Agenten mit Backend-Richtlinie und Kosten, Apps und Webhooks, Import, Instanz und Branding,
  Statusseite.
- Audit-Log mit Prüfung der Hash-Kette, Statistik.

Screens: [01-produkt.md](../../docs/spec/01-produkt.md), Abschnitte 5.2 und 5.5. Die Screens kommen
aus `packages/admin_ui` und sind dieselben wie in der Desktop-App.

## Technik

- Pakete: `design_system`, `l10n`, `domain`, `api_client`, `auth`, `admin_ui`, `device_key`
  (WebCrypto). Nicht: `sync_engine`, `local_store` (Architekturtest).
- Anmeldung mit WebAuthn-Passkeys oder OIDC; DPoP mit nicht extrahierbarem WebCrypto-Schlüssel;
  Step-up per Passkey ([15-client-architektur.md](../../docs/spec/15-client-architektur.md),
  Abschnitte 8 und 9).
- JavaScript-Build; Semantik beim Start eingeschaltet (Barrierefreiheit).
- Ausgeliefert vom Server unter einem eigenen Pfad, gleiche Herkunft wie die API (Betrieb:
  [16-betrieb.md](../../docs/spec/16-betrieb.md)).

## Plattformen

Aktuelle Versionen von Chromium-Browsern, Firefox und Safari.
