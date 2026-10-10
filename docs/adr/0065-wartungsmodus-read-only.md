---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team F (Betrieb), Team B (API), Team D (Agenten-Plattform)
informed: Team E (Apps), Team C (Identität)
---

# Wartungsmodus read_only

## Context and Problem Statement

Ein Umzug auf ein anderes Ziel und ein Hauptversions-Upgrade von PostgreSQL kopieren den Stand einer
Instanz ([16-betrieb.md](../spec/16-betrieb.md), Abschnitte 10 und 11;
[cloud-migration.md](../operations/cloud-migration.md)). Während der Abschluss-Sicherung darf die
alte Instanz nichts mehr annehmen, was auf dem Ziel fehlen würde. Menschen sollen trotzdem lesen,
und laufende Agenten-Läufe sollen sauber enden. Der Kern-Vertrag legt die Schicht und den Fehler
fest ([05-aktionen.md](../spec/05-aktionen.md), Regel 1.9: `503 maintenance` mit `Retry-After`,
Lesen und Nachholen laufen weiter). Offen ist die genaue Bedeutung: Welche Aktion gilt als
schreibend, welche Schreibvorgänge bleiben erlaubt, und was verhindert, dass ein übersehener Pfad
doch schreibt?

## Decision Drivers

- Nach dem Umschalten entsteht kein Zustand mit fachlicher Wirkung, außer den festgelegten
  Ausnahmen; sonst fehlt er auf dem Ziel.
- Lesen, Nachholen und Datei-Abruf laufen weiter (05, Regel 1.9).
- Laufende Läufe dürfen enden, neue entstehen nicht ([PLAN.md](../PLAN.md), Bau-Ticket M7-08).
- Clients erkennen den Zustand und halten ihr Ausgangsfach (`Retry-After`).
- Durchsetzen statt annehmen: Ein übersehener Schreibpfad scheitert sichtbar.
- Jede Aktion ist eindeutig eingeordnet. Die Spalte "Recht" aus 05 reicht dafür nicht:
  `settings.read` und `settings.update` haben beide das Recht `self`.

## Considered Options

- Konfigurationswert `server.maintenance_mode = read_only`; Einordnung jeder Aktion im Katalog,
  feste Ausnahmen für den Abschluss von Läufen, Token-Erneuerung ohne Rotation und Zustellungen;
  lesende Transaktionen als zweite Linie
- Server ganz anhalten
- Nur die Datenbank auf lesend stellen (`default_transaction_read_only`)
- Ein Schalter in der Datenbank, den der Owner per API umlegt
- Wartung je Modul (zum Beispiel nur Nachrichten gesperrt)

## Decision Outcome

Gewählte Option: "`server.maintenance_mode = read_only` mit Einordnung im Katalog, festen Ausnahmen
und lesenden Transaktionen", weil nur sie Lesen erlaubt, Läufe enden lässt und jeden anderen
Schreibversuch an zwei Stellen abfängt.

1. **Schalter.** `server.maintenance_mode` (`off` oder `read_only`) ist Konfiguration des Betreibers
   und wirkt beim Start von `api` und `worker` (R-AR-15). Grund gegen einen Schalter in der
   Datenbank: Das Umlegen wäre selbst ein Schreibvorgang, der Schalter müsste sich selbst ausnehmen,
   und Wartung ist ein Schritt des Betreibers rund um eine Kopie, kein Produktschalter.
2. **Einordnung.** Jede Aktion im Katalog (ADR 0049) trägt `effect: read` oder `effect: write`;
   `write` heißt: Die Aktion kann dauerhaften Zustand ändern. Das ersetzt die Ableitung aus der
   Spalte "Recht" in 05, Regel 1.9.
3. **Schicht.** Nach dem Rate-Limit und vor `authorize` (05, Abschnitt 2.5) beantwortet sie jede
   Aktion mit `effect: write` ohne Ausnahme mit `503` und Problem-Code `maintenance` samt
   `Retry-After`. Sie schreibt dafür keinen Audit-Eintrag (das wäre ein Schreibvorgang und keine
   Rechte-Entscheidung); eine Kennzahl zählt die Ablehnungen.
4. **Ausnahmen.**
   - Abschluss laufender Läufe: Für einen Agenten mit einem Lauf im Zustand `running` bleiben
     `run.update`, `run.finish`, `agent.heartbeat` und `agent.events.consume` (`ack`) erlaubt, dazu
     `message.create` und `file.upload` im Thread dieses Laufs (die Antwort). `run.cancel` bleibt
     für Menschen erlaubt, damit der Owner hängende Läufe beenden kann; der Job
     `runs.cancel_timeout` läuft weiter. `run.start` ist gesperrt: Läufe in `queued` und
     `waiting_input` warten und überleben die Kopie, weil ihr Zustand in der Datenbank liegt.
   - Token-Erneuerung: `auth.token.refresh` gibt ein neues Zugriffstoken aus, ohne das Refresh-Token
     zu rotieren und ohne zu schreiben. Grund: Sonst endet das Lesen nach `auth.access_token_ttl`;
     eine Rotation nach der Abschluss-Sicherung fehlte auf dem Ziel und meldete die Person dort ab.
     Refresh-Tokens sind an den Geräteschlüssel gebunden; ohne Rotation bleibt die Sender-Bindung,
     die RFC 9700 als Alternative zur Rotation nennt ([08-identitaet.md](../spec/08-identitaet.md),
     Abschnitt 7.4).
   - Sicherheitsreaktionen: Der Widerruf einer Refresh-Familie bei erkannter Wiederverwendung läuft
     wie immer, und die Notfall-Aktionen `auth.step_up`, `session.revoke`, `device.revoke`,
     `token.revoke` und `user.disable` bleiben erlaubt. Grund: Ein gestohlenes Gerät muss auch im
     Wartungsfenster sperrbar sein. Fällt eine solche Aktion nach die Abschluss-Sicherung,
     wiederholt der Owner sie auf dem Ziel.
   - Zustellungen: Im `worker` laufen die Verbraucher `push` und `webhooks` und ihre Jobs weiter
     (ADR 0051), ebenso `runs.cancel_timeout`. Alle anderen Warteschlangen und periodischen Jobs
     setzen aus.
   - `gateway.connect` bleibt erlaubt; die Fencing-Spalte der Agenten-Verbraucher im Modus
     `multi` darf es weiter schreiben, weil sie auf dem Ziel keine Bedeutung hat.
5. **Zweite Linie.** Im Modus `read_only` öffnet die Unit of Work jede Transaktion als `READ ONLY`,
   außer für die Ausnahmen aus Punkt 4. PostgreSQL lehnt dann `INSERT`, `UPDATE`, `DELETE` und DDL
   mit SQLSTATE `25006` ab [Q1, belegt]; der Server antwortet `503 maintenance`.
   Nebenbei-Schreibungen wie `last_used_at` von Tokens und Sitzungen entfallen im Modus
   ausdrücklich.
6. **Anzeige.** Statusseite und Konsole melden die Wartung, abgeleitet aus der Konfiguration ohne
   Schreibvorgang. Apps zeigen bei `503 maintenance` einen Hinweis statt eines Fehlers und senden
   ihr Ausgangsfach nach `Retry-After` erneut.

Für eine Wiederherstellung auf einen Zeitpunkt ist der Modus nicht gedacht: Dort wird die Datenbank
ersetzt, und `api` und `worker` stehen still
([backup-restore.md](../operations/backup-restore.md), Abschnitt 5.2).

### Consequences

- Gut, weil Menschen während des Umzugs lesen, suchen und Dateien abrufen und Läufe ihre Antwort
  noch abgeben.
- Gut, weil der Stand nach dem Abwarten der Läufe dem Stand auf dem Ziel entspricht; nur
  Zustellungen können danach noch Zeilen ändern.
- Gut, weil ein übersehener Schreibpfad an PostgreSQL scheitert statt still Daten zu erzeugen, die
  auf dem Ziel fehlen.
- Schlecht, weil der Katalog ein weiteres Merkmal bekommt und die Ausnahmen für Agenten vom
  Laufzustand abhängen; die Schicht braucht dafür eine Abfrage beim Modul `runs`.
- Schlecht, weil Ein- und Ausschalten einen Neustart brauchen; Clients verbinden mit `4013` neu.
- Schlecht, weil Zustellungen nach der Abschluss-Sicherung auf dem Ziel ein zweites Mal entstehen
  können (Push doppelt, Webhook mit gleicher `webhook-id`); Zustellung ist ohnehin mindestens
  einmal.
- Schlecht, weil im Fenster keine neuen Geräte angemeldet werden und Abmelden nur lokal wirkt.
- Schlecht, weil Notfall-Widerrufe nach der Abschluss-Sicherung auf dem Ziel fehlen; das Runbook
  des Umzugs muss den Owner daran erinnern, sie dort zu wiederholen.
- Neutral, weil 08 eine Ausnahme von der Rotation bekommt, die nur in diesem Modus gilt.

### Confirmation

- Bau-Ticket M7-08 in [PLAN.md](../PLAN.md): Eine Nachricht ergibt `503` mit `Retry-After`; ein
  laufender Lauf kann enden; neue Läufe entstehen nicht; die Statusseite zeigt Wartung.
- Tabellen-Test über den Katalog: Jede Aktion hat `effect`; jede Aktion mit `effect: write` ohne
  Ausnahme ergibt im Modus `503 maintenance`, keine Aktion mit `effect: read` tut es.
- Test der zweiten Linie: Ein Test-Handler mit einer nicht eingeordneten Schreibung scheitert im
  Modus mit `503`, und die Datenbank ist unverändert.
- Test der Erneuerung: Im Modus liefert `auth.token.refresh` ein neues Zugriffstoken und dasselbe
  Refresh-Token; Zeilen von Sitzung und Refresh-Familie sind vorher und nachher gleich.
- Test im `worker`: Im Modus laufen nur Zustell-Jobs und `runs.cancel_timeout`.
- Test der Sicherheitsreaktionen: Im Modus wirkt `device.revoke` mit Step-up (Verbindung des Geräts
  schließt mit `4012`), und eine Wiederverwendung eines Refresh-Tokens widerruft die Familie.
- Probe des Umzugs ([cloud-migration.md](../operations/cloud-migration.md), Abschnitt 3): Das
  Manifest der Abschluss-Sicherung gleicht dem Stand am Ende des Fensters, abgesehen von den
  Zustell-Tabellen.

## Pros and Cons of the Options

### Konfiguration mit Einordnung, Ausnahmen und zweiter Linie

- Gut, weil Lesen bleibt, Läufe enden und zwei unabhängige Stellen Schreiben verhindern.
- Schlecht, weil die meiste Arbeit dieser Optionen: Katalog-Merkmal, Ausnahmen, Modus der
  Unit of Work, Token-Pfad.

### Server ganz anhalten

- Gut, weil am einfachsten und sicher ohne Schreibvorgang.
- Schlecht, weil niemand lesen kann, Clients "Server nicht erreichbar" statt Wartung zeigen und
  laufende Läufe abbrechen; ihre Arbeit und ihre Kosten gehen verloren.

### Nur die Datenbank auf lesend

- Gut, weil PostgreSQL es durchsetzt [Q1, belegt].
- Schlecht, weil Fehler erst nach begonnener Arbeit im Handler auftreten und ohne `Retry-After`
  ankommen; Ausnahmen für Läufe bräuchten trotzdem eine Einordnung je Aktion. Darum nur zweite
  Linie.

### Schalter in der Datenbank

- Gut, weil ohne Neustart.
- Schlecht, weil das Umlegen selbst schreibt und der Schalter sich selbst ausnehmen müsste; ein
  Owner-Schalter in der Oberfläche lädt dazu ein, ihn ohne Kopie-Ablauf zu nutzen.

### Wartung je Modul

- Gut, weil feiner steuerbar.
- Schlecht, weil eine Kopie den ganzen Stand braucht; Teil-Sperren lassen Schreibwege offen und
  machen den Vergleich der Manifeste unmöglich.

## More Information

- Quelle (abgerufen 2026-10-10): Q1 PostgreSQL 18, `SET TRANSACTION` (Lese-Modus) und Fehlercode
  `25006 read_only_sql_transaction`, https://www.postgresql.org/docs/18/sql-set-transaction.html und
  https://www.postgresql.org/docs/18/errcodes-appendix.html [belegt].
- Problem-Code `maintenance`: [06-api.md](../spec/06-api.md), Abschnitt 5.
- Verwandt: ADR 0015 (Fehlerformat), ADR 0020 (DPoP), ADR 0049 (Aktionskatalog), ADR 0051
  (Worker-Verbraucher), ADR 0058 (Sicherung), ADR 0059 (Sync-Epoche).
