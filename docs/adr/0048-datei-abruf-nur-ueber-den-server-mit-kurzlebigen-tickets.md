---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team G (Plattformdienste), Team B (API), Team C (Sicherheit)
informed: Team E (Apps)
---

# Datei-Abruf nur über den Server mit kurzlebigen Tickets statt presigned GET

## Context and Problem Statement

Jeder Datei-Abruf muss durch `authorize` laufen, und ein Entzug wirkt sofort (B7,
[07-rechte.md](../spec/07-rechte.md), Abschnitte 8.1 und 8.2). Der S3-Treiber könnte Clients
vorab signierte Download-URLs geben und den Server entlasten. Medien-Player und "Öffnen mit"
senden aber eigene HTTP-Anfragen mit `Range` und können keinen DPoP-Beweis je Anfrage bilden
([08-identitaet.md](../spec/08-identitaet.md), Abschnitt 7.3). Wie bekommen solche Abrufe Zugriff,
ohne dass ein Link einen Entzug überlebt?

## Decision Drivers

- `authorize(principal, file.read, file)` zur Abrufzeit, für Original, Vorschau und Transkript.
- Ein Entzug (Freigabe, Sitzung, Gerät, Sperre) wirkt auf den nächsten Abruf.
- Kopfzeilen wie `Content-Disposition`, `nosniff` und Sandbox-CSP setzt nur der Server; die
  Admin-Konsole läuft im Browser ([12-dateien.md](../spec/12-dateien.md), Abschnitt 9.3).
- Der Speicher muss für Clients nicht erreichbar sein; der Dateisystem-Treiber hat gar keine URLs.
- Medien-Player brauchen einfache GET-Anfragen mit `Range`.
- Gleiches Verhalten für beide Treiber (ADR 0010).

## Considered Options

- Server liefert alle Bytes selbst; kurzlebige, an Prinzipal und Sitzung gebundene HMAC-Tickets
  für Player, bei jedem Abruf erneut geprüft
- Presigned GET-URLs des Objektspeichers mit kurzer Gültigkeit
- Server liefert selbst; Player bekommen das normale Zugriffstoken in der URL
- Server prüft, der Reverse-Proxy liefert die Bytes (`X-Accel-Redirect` oder ähnlich)
- Cookie-Sitzung für Medien-Abrufe

## Decision Outcome

Gewählte Option: "Server liefert alle Bytes selbst, mit kurzlebigen Tickets", weil sie als
einzige `authorize` bei jedem Abruf durchsetzt, ohne vom Speicher oder vom Rand abzuhängen. Die
Regel ist R-FI-03 in [12-dateien.md](../spec/12-dateien.md), Abschnitt 9.

1. `GET /api/v1/files/{id}/content`, `.../thumbnails/{variant}` und `.../transcript` liefern die
   Inhalte durch den Server, Bytes mit `Range` (genau ein Bereich). Es gibt keine presigned
   GET-URLs; der Port des Objektspeichers hat keine Operation dafür.
2. `POST /api/v1/files/{id}/tickets {variant}` (Aktion `file.read`) liefert `{url, expires_at}`.
   Das Ticket ist `base64url(payload || HMAC-SHA-256(key, payload))` mit `file_id`, `variant`,
   `principal_id`, `auth_session_id` und Ablauf `file.ticket_ttl`. Der Schlüssel kommt aus dem
   Secret-Provider (`file.ticket_key_ref`); zwei Schlüssel gelten gleichzeitig, damit ein Wechsel
   ohne Ausfall geht.
3. Bei jedem Abruf mit Ticket prüft der Server Signatur, Ablauf, ob die Sitzung noch gilt, und
   `authorize` mit den aktuellen Rechten. Jeder Fehler ergibt dieselbe `404` wie eine unbekannte ID.
4. Antworten mit Ticket tragen `Cache-Control: private, no-store`. Der Parameter `ticket` wird aus
   Logs und Telemetrie-Attributen entfernt; der Collector filtert ihn zusätzlich
   ([17-observability.md](../spec/17-observability.md)).
5. Dieselben Tickets nutzt die Push-Vorschau für das erste Bild (`image_url`,
   [13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md), Abschnitt 6.4).

HMAC statt einer signierten Marke mit den Server-Schlüsseln: Nur der Server prüft das Ticket;
ein symmetrischer Schlüssel ist kürzer und schneller, und niemand außerhalb braucht einen
öffentlichen Schlüssel.

### Consequences

- Gut, weil ein Ticket keinen Entzug überlebt: Sitzungs-Widerruf und Rechte-Entzug wirken auf den
  nächsten Abruf.
- Gut, weil der Objektspeicher in privaten Netzen bleiben kann und kein Bucket öffentlich
  erreichbar sein muss (SEC-UP-11 in [19-sicherheit.md](../spec/19-sicherheit.md)).
- Gut, weil beide Treiber denselben Weg haben; Clients sehen keinen Unterschied.
- Schlecht, weil jeder Download durch den Server läuft (Bandbreite, offene Verbindungen bis
  `http.transfer_timeout`). Für einen Team-Chat mit `file.max_upload_bytes` ist das tragbar;
  Abrufe sind mit `rl.read` gedrosselt.
- Schlecht, weil eine Ticket-URL bis `file.ticket_ttl` ein Bearer-Geheimnis ist: Wer sie in dieser
  Zeit bekommt (zum Beispiel über "Öffnen mit" an eine fremde App), kann abrufen, solange der
  Prinzipal das Recht noch hat.
- Neutral, weil Uploads weiter optional presigned sein dürfen (R-FI-02); dort prüft der Server
  Größe und Prüfsumme am Ende.

### Confirmation

- Tests aus [12-dateien.md](../spec/12-dateien.md), Abschnitt 15: Ticket mit `Range` ergibt `206`;
  nach Rechte-Entzug, nach Sitzungs-Widerruf und nach Ablauf je `404`; fremde und unbekannte ID
  ergeben dieselbe `404`.
- Architektur-Regel: Nur `internal/files/adapters/s3` importiert das S3-SDK, und der Port
  `ObjectStore` hat keine Operation für signierte Download-URLs (Prüfung der Port-Schnittstelle im
  Treiber-Konformitätstest).
- Kanarien-Test aus [17-observability.md](../spec/17-observability.md), Abschnitt 10: Ein Ticket
  taucht in keinem Export auf.
- SEC-T03 (Kopfzeilen für HTML und SVG: `attachment`, `nosniff`, Sandbox-CSP).
- Bau-Ticket M3-05 in [PLAN.md](../PLAN.md).

## Pros and Cons of the Options

### Server liefert selbst, mit Tickets

- Gut, weil Rechte, Kopfzeilen und Protokollierung an einer Stelle liegen.
- Schlecht, weil der Server Bandbreite trägt.

### Presigned GET-URLs

- Gut, weil Downloads den Server nicht belasten.
- Schlecht, weil eine URL bis zu ihrem Ablauf gilt, auch nach einem Entzug; der Server sieht den
  Abruf nicht und kann ihn nicht prüfen.
- Schlecht, weil der Speicher für Clients erreichbar sein muss und die Kopfzeilen beim Speicher
  liegen.
- Schlecht, weil der Dateisystem-Treiber keine URLs hat; zwei Abrufwege für zwei Treiber.

### Zugriffstoken in der URL

- Gut, weil kein neuer Mechanismus nötig ist.
- Schlecht, weil Zugriffstokens an den Geräteschlüssel gebunden sind (DPoP) und ohne Beweis
  nicht gelten; ein Token ohne Bindung würde das ganze Token-Modell schwächen.
- Schlecht, weil Tokens in URLs in Logs von Proxys und Playern landen und für die ganze API gelten.

### Reverse-Proxy liefert die Bytes

- Gut, weil der Server nur prüft und die Bandbreite beim Proxy liegt.
- Schlecht, weil der Mechanismus proxy-spezifisch ist und nicht mit jedem Rand der Instanz geht
  ([16-betrieb.md](../spec/16-betrieb.md), Abschnitt 4.3); der Proxy müsste den Speicher lesen
  können. Als spätere Optimierung bleibt er möglich, weil die Prüfung im Server bleibt.

### Cookie-Sitzung für Medien

- Gut, weil Player Cookies oft mitsenden.
- Schlecht, weil die nativen Apps keine Cookie-Sitzung haben; Cookies gibt es nur für die
  Web-Konsole (08, Abschnitt 7.5). Ein zweites Sitzungsmodell für Apps wäre mehr Angriffsfläche
  als ein eng gebundenes Ticket.

## More Information

- Entscheidung R-FI-03 in [12-dateien.md](../spec/12-dateien.md); Endpunkte in
  [06-api.md](../spec/06-api.md), Abschnitt 10.
- Verwandt: ADR 0010 (Objektspeicher), ADR 0017 (eine Autorisierungsfunktion, 404 statt 403),
  ADR 0020 (DPoP), ADR 0047 (Bild-Metadaten).
