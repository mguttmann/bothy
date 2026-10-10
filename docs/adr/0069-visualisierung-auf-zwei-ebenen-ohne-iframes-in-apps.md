---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Builder-Check (frischer Prüfer), Team Betrieb, Team Apps und Design
informed: alle Teams
---

# Visualisierung auf zwei Ebenen ohne iframes in Apps

## Context and Problem Statement

Der Owner verlangt Visualisierungen, die beeindrucken: eine Live-Topologie der Agenten-Läufe, eine
Ansicht der Backend-Richtlinie, Zustellung und Sync live, eine Executive-Seite mit SLO-Ampel, eine
abspielbare Tages-Timeline und einen TV-Modus, in Apps und Konsole und zusätzlich in Grafana, beide
in derselben Designsprache. Er fragt, ob Grafana-Grafiken in die App eingespielt werden können.
Die Statistik der App darf nicht vom Observability-Stack abhängen (R-OB-08, 17 Regel 7), und viele
Werte (Zustell-Latenz, Wiederverbinden, Annahme beim Push-Anbieter) stehen in keiner Fachtabelle.
Wie bauen wir die Visualisierungen, ohne Rechte, Datenschutz und Unabhängigkeit vom Stack zu
verlieren?

## Decision Drivers

* R-OB-08: Statistik und Statusseite funktionieren ohne Observability-Stack.
* Die Rechte der Instanz (`stats.read`, `cost.read.aggregate`, sichtbare Kanäle) gelten auch für
  Grafiken.
* Keine Werte je Person (20, Abschnitt 9).
* Eine Designsprache, Tokens aus einer Quelle (ADR 0030).
* Owner-Vorgabe: in iOS, Android und Desktop keine iframes; Grafana-Einbettung höchstens optional in
  der Web-Konsole.
* Clickjacking: Grafana schützt sich standardmäßig gegen Einbettung.

## Considered Options

* Zwei Ebenen: native Visualisierungen aus eigener Messung in Apps und Konsole, dazu ein eigenes
  Grafana-Panel-Plugin aus OTel-Metriken; Einbettung nur optional in der Konsole
* Grafana in allen Clients einbetten (iframe in der Konsole, WebView in den Apps)
* Nur native Visualisierungen, kein Grafana-Plugin
* Nur Grafana; die Apps verlinken auf Grafana

## Decision Outcome

Gewählt: "Zwei Ebenen", weil nur diese Option Rechte, Unabhängigkeit vom Stack und die Vorgabe des
Owners zugleich erfüllt.

1. Ebene a: Paket `packages/bothy_viz` (Flutter) mit sechs Ansichten. Daten nur aus der eigenen
   Statistik-API und dem Gateway-Ereignis `stats.live`. Der Server misst selbst über
   `kernel/measure` und schreibt Minuten-Aggregate nach `stats_minutes`; SLI-Schwellen und Ziele
   werden aus den Sloth-Dateien erzeugt; die Ampel heißt "eigene Messung" (Ruling T7).
2. Ebene b: Plugin `plugins/grafana-bothy-live` (ID `bothy-live-panel`), erzeugt mit
   `@grafana/create-plugin`, Daten aus Prometheus und Tempo über Grafana; bis V1 unsigniert und nur
   mit seiner ID freigegeben (Ruling T11).
3. Gemeinsame Designsprache: neue Rollen `color.viz.*` in `design/tokens/` und eine
   TypeScript-Ausgabe von `tools/design-tokens` für das Plugin.
4. Keine iframes oder WebViews mit Grafana in iOS, Android, macOS und Windows. In der Web-Konsole ist
   eine Einbettung optional und standardmäßig aus (`console.grafana_embed_url` leer); nur mit
   `allow_embedding`, Auth-Proxy, `frame-ancestors`, COEP/CORP am Proxy und `frame-src` genau dieses
   Origins.
5. Statt einer Ansicht für ein Werkzeug, das persönliche Abos rotiert, gibt es eine produktneutrale
   Backend-Ansicht (Ruling T8).

Einzelheiten: [25-visualisierung.md](../spec/25-visualisierung.md).

### Consequences

* Gut, weil die Statistik in jeder Installation gleich funktioniert, auch ohne Grafana.
* Gut, weil Grafiken denselben Rechten folgen wie die Daten und keine Werte je Person zeigen.
* Gut, weil Betreiber mit Grafana dieselbe Optik im Betrieb bekommen.
* Schlecht, weil zwei Umsetzungen derselben Ansichten zu pflegen sind (Dart und TypeScript).
* Schlecht, weil die eigene Messung und Prometheus leicht abweichen können (verschiedene
  Messpunkte, verlorene Zähler bei einem Absturz); die Ampel sagt das ausdrücklich.
* Schlecht, weil ein unsigniertes Plugin eine Ausnahme in Grafana braucht und in Grafana Cloud nicht
  läuft, bis signiert ist.

### Confirmation

* VZ-T01 bis VZ-T16 in [25-visualisierung.md](../spec/25-visualisierung.md), Abschnitt 12.
* VZ-T03: Generator der SLI-Grenzen gegen die Sloth-Dateien im Gate.
* VZ-T16: statische Prüfung, dass `apps/client` kein WebView-Paket und kein iframe-Element nutzt.
* SEC-T19 in [19-sicherheit.md](../spec/19-sicherheit.md): `frame-src` nur mit gesetzter
  `console.grafana_embed_url`.

## Pros and Cons of the Options

### Zwei Ebenen

* Gut, weil R-OB-08 und die Rechte der Instanz gelten.
* Gut, weil Grafana dort bleibt, wo es hingehört: beim Betrieb.
* Schlecht, weil mehr Bauarbeit.

### Grafana überall einbetten

* Gut, weil eine Umsetzung genügt.
* Schlecht, weil die Statistik vom Stack abhinge (verletzt R-OB-08).
* Schlecht, weil Grafana die Rechte der Instanz nicht kennt und eine zweite Anmeldung braucht.
* Schlecht, weil `allow_embedding` den Clickjacking-Schutz von Grafana abschaltet und WebViews in
  Apps eine weitere Angriffsfläche sind; vom Owner ausgeschlossen.

### Nur native Visualisierungen

* Gut, weil eine Umsetzung genügt.
* Schlecht, weil Betreiber im Betrieb ohne passende Ansichten bleiben; der Owner verlangt das Plugin.

### Nur Grafana mit Links

* Gut, weil am wenigsten Bauarbeit.
* Schlecht, weil die App ohne Statistik bliebe, sobald kein Stack läuft, und Grafana keine Rechte
  je Bereich kennt.

## More Information

* Verwandt: [ADR 0028](0028-web-ziel-nur-fuer-admin-und-statistik-konsole.md),
  [ADR 0030](0030-design-tokens-als-eine-quelle-mit-generator.md),
  [ADR 0034](0034-opentelemetry-ueber-collector-lokaler-standard-stack.md),
  [ADR 0035](0035-slos-mit-fehlerbudgets-und-burn-rate-alarmen.md),
  [ADR 0044](0044-arbeitsname-und-branding-als-konfiguration.md).
* Grafana-Belege (abgerufen 2026-10-10): https://grafana.com/developers/plugin-tools/,
  https://grafana.com/docs/grafana/latest/administration/plugin-management/plugin-sign/,
  https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/.
* Rulings T7, T8 und T11 der Leitung.
