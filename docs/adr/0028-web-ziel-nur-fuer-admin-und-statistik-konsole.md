---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team Apps und Design
consulted: Team Sicherheit und Rechte, Team Betrieb
informed: alle Teams
---

# Web-Ziel nur für Admin- und Statistik-Konsole

Status-Hinweis (2026-10-10): Teilweise ersetzt durch
[ADR 0070](0070-terminal-in-der-web-konsole.md). Die Konsole bekommt zusätzlich das Terminal mit
Passkey-Pflicht; alles andere dieser Entscheidung gilt weiter, insbesondere kein Chat im Browser.

## Context and Problem Statement

Betreiberinnen und Owner brauchen Verwaltung und Statistik auch ohne installierte App, etwa vom
Rechner eines Kollegen oder direkt nach der Installation einer Instanz. Ein Chat im Browser wäre
technisch möglich. Er bringt aber einen weiteren Client mit Offline-Speicher, Push, Sitzungen und
Angriffsfläche (XSS, Erweiterungen im Browser) in ein Produkt, dessen Inhalte Agenten-Sitzungen mit
Werkzeugmacht steuern. Was bietet das Web-Ziel an?

## Decision Drivers

- Verwaltung und Statistik ohne App erreichbar (Baseline B5)
- kleine Angriffsfläche für die Steuerung von Agenten
- ein Client weniger mit Offline-Speicher, Push und Sitzungsverwaltung
- gleiche Bausteine wie die Apps (Design-System, API-Client, Verwaltungs-Screens)
- Barrierefreiheit im Browser

## Considered Options

1. Flutter Web nur für Verwaltung und Statistik (Konsole)
2. Voller Chat-Client im Browser (Flutter Web)
3. Kein Web-Ziel; Verwaltung nur in den Apps
4. Konsole in eigener Web-Technik (z. B. TypeScript mit einem SPA-Framework)

## Decision Outcome

Chosen option: "Flutter Web nur für Verwaltung und Statistik", weil es die Verwaltung ohne App
erreichbar macht, dieselben Pakete wie die Apps nutzt (`design_system`, `api_client`, `auth`,
`admin_ui`) und den Chat mit seinem Offline-Speicher, Push und Agenten-Auslösung aus dem Browser
heraushält. Flutter Web passt laut Doku zu "app-centric experiences" wie einer Konsole, nicht zu
textlastigen Seiten (https://docs.flutter.dev/platform-integration/web/faq, abgerufen 2026-10-10
[belegt laut Recherche]).

### Consequences

- Good, because die Konsole keine lokale Datenbank, kein Ausgangsfach und keinen Push braucht.
- Good, because Verwaltungs-Screens nur einmal entstehen (`packages/admin_ui`).
- Good, because WebAuthn-Passkeys im Browser ohne Associated Domains funktionieren.
- Bad, because Personen ohne App nicht chatten können; Links aus der Konsole öffnen die App.
- Bad, because Flutter Web Barrierefreiheit erst nach `SemanticsBinding.instance.ensureSemantics()`
  einschaltet (https://docs.flutter.dev/ui/accessibility/web-accessibility, abgerufen 2026-10-10
  [belegt laut Recherche]); die Konsole schaltet sie immer ein.
- Neutral, because der JavaScript-Build genutzt wird; Wasm hilft heute nur in Chromium
  (https://docs.flutter.dev/platform-integration/web/wasm, abgerufen 2026-10-10 [belegt]).

### Confirmation

- Architekturtest: `apps/console` hängt nicht von `sync_engine` und `local_store` ab.
- Routentest der Konsole: es gibt keine Route für Kanäle, Threads oder Nachrichten.
- Ein Widget-Test prüft, dass die Konsole beim Start die Semantik einschaltet.

## Pros and Cons of the Options

### Flutter Web nur Konsole

- Good, because geteilte Pakete, kleine Angriffsfläche, kein Offline-Client im Browser.
- Bad, because kein Chat ohne App.

### Voller Chat im Browser

- Good, because Zugang ohne Installation.
- Bad, because Push, Offline-Speicher, Schlüssel im Browser und Agenten-Auslösung aus einer
  Umgebung mit Erweiterungen und XSS-Risiko.
- Bad, because ein weiterer Client mit Tests, Golden-Bildern und Barrierefreiheit.

### Kein Web-Ziel

- Good, because noch weniger Fläche.
- Bad, because Verwaltung direkt nach der Installation und von fremden Rechnern fehlt.

### Konsole in eigener Web-Technik

- Good, because SEO und Text-Rendering des Browsers.
- Bad, because zweite UI-Technik, zweites Design-System, doppelte Verwaltungs-Screens.

## More Information

- Screens der Konsole: [01-produkt.md](../spec/01-produkt.md), Abschnitt 5.5.
- Anmeldung der Konsole (Passkeys, DPoP mit WebCrypto): [15-client-architektur.md](../spec/15-client-architektur.md), Abschnitte 8 und 9.
- Wird ein Chat im Browser später gewünscht, braucht er eine neue ADR mit Bedrohungsmodell.
