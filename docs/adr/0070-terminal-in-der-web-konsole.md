---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Builder-Check (frischer Prüfer), Team Sicherheit und Rechte
informed: Team Apps und Design
---

# Terminal in der Web-Konsole

Ersetzt [ADR 0028](0028-web-ziel-nur-fuer-admin-und-statistik-konsole.md) teilweise: Die
Web-Konsole bekommt neben Verwaltung und Statistik das Terminal. Alles andere aus ADR 0028 gilt
weiter, insbesondere: kein Chat im Browser.

## Context and Problem Statement

ADR 0028 beschränkt das Web-Ziel auf Verwaltung und Statistik, um Offline-Speicher, Push und die
Steuerung von Agenten aus dem Browser herauszuhalten. Der Owner verlangt das Terminal ausdrücklich
auch in der Web-Konsole (Nachtrag vom 2026-10-10) und eine Passkey-Bestätigung beim Öffnen. Die
Konsole erlaubt Step-up bisher auch per erneuter OIDC-Anmeldung oder Passwort mit TOTP
([08-identitaet.md](../spec/08-identitaet.md), Abschnitt 6.2). Ein Browser hat Erweiterungen und
XSS als Angriffsfläche. Darf die Konsole ein Terminal öffnen, und unter welchen Bedingungen?

## Decision Drivers

* Owner-Vorgabe: Terminal in Desktop, Handy und Web-Konsole.
* Passkey-Bestätigung beim Öffnen ohne Ersatzweg.
* Ein Terminal im Browser ist Fernzugriff; XSS in der Konsole darf keinen Rechner öffnen.
* Keine zweite UI-Technik; gleiches Widget wie in den Apps.
* Kein Chat im Browser (ADR 0028 bleibt).

## Considered Options

* Terminal in der Konsole mit Passkey-Pflicht (`userVerification = required`) und gehärteter CSP
* Kein Terminal in der Konsole; Terminal nur in den nativen Apps
* Terminal in der Konsole mit jedem Step-up-Weg der Konsole

## Decision Outcome

Gewählt: "Terminal in der Konsole mit Passkey-Pflicht und gehärteter CSP", weil sie die Vorgabe des
Owners erfüllt und die schwächeren Step-up-Wege der Konsole für das Terminal ausschließt.

1. `terminal.open` und `terminal.attach` in der Konsole nur mit Step-up per WebAuthn-Passkey mit
   `userVerification = required`, frisch und je Öffnung verbraucht. Kein Ersatzweg; ohne Passkey
   zeigt die Konsole den Weg zur Einrichtung ([08-identitaet.md](../spec/08-identitaet.md),
   Abschnitt 6.3).
2. Gleiche CSP wie die übrige Konsole: `connect-src 'self'` für den Strom, Trusted Types,
   `frame-ancestors 'none'`. Das Terminal rendert mit dem Flutter-Widget `xterm` in den Canvas; es
   erzeugt kein HTML aus Terminal-Ausgaben [abgeleitet aus der Bauart von Flutter Web]. Kontrollen
   SEC-TERM-* in [19-sicherheit.md](../spec/19-sicherheit.md).
3. Der Noise-Schlüssel des Geräts lebt nur im Speicher des Tabs; gepinnte Rechner-Schlüssel im
   Speicher des Origins, gelöscht bei Abmeldung. Das Pinning fängt in der Konsole nur einen
   Schlüsselwechsel ab, den ein ehrlicher Server meldet; darum kostet das Löschen keinen Schutz.
4. Das schnelle Wiederanhängen ohne Step-up (`terminal.quick_reattach_window`) gilt in der Konsole
   nicht: Ohne Geräteschlüssel beweist nur das Cookie die Anfrage. Jedes Anhängen braucht einen
   frischen Passkey.
5. Kein Chat, kein Offline-Speicher, kein Push in der Konsole (ADR 0028 unverändert).

### Consequences

* Gut, weil Owner ohne App, z. B. direkt nach der Installation, auf einen Rechner kommen.
* Gut, weil ein Angreifer mit einer gestohlenen Konsolen-Sitzung ohne Passkey kein Terminal öffnet.
* Schlecht, weil Personen ohne Passkey in der Konsole kein Terminal haben. Das ist gewollt.
* Schlecht, weil das Pinning in der Konsole nicht gegen einen bösartigen Server-Betreiber schützt:
  Er liefert den Code der Konsole und kann eine Fassung ohne Pin-Prüfung schicken. Gegenmittel
  dort ist nur `host_service.require_local_confirm` ([24-terminal.md](../spec/24-terminal.md),
  Abschnitt 5.1; Bedrohungsmodell T-TM-01).
* Schlecht, weil nach jedem Funkloch ein neuer Passkey nötig ist; in der Konsole ist das
  hinnehmbar, weil sie am Schreibtisch läuft.
* Schlecht, weil bösartige Browser-Erweiterungen eine geöffnete Sitzung mitlesen können
  (Restrisiko im Bedrohungsmodell, T-TM-07); Empfehlung: eigenes Browser-Profil für die Verwaltung.

### Confirmation

* ID-T28 in [08-identitaet.md](../spec/08-identitaet.md): Konsole mit Passwort-Step-up oder
  Passkey ohne `userVerification = required` bekommt kein Terminal; Wiederanhängen in der Konsole
  ohne Passkey scheitert auch innerhalb von `terminal.quick_reattach_window` (TM-T12).
* SEC-T05 (CSP im Headless-Browser) läuft mit geöffnetem Terminal.
* Routentest der Konsole aus ADR 0028 bleibt: keine Routen für Kanäle, Threads oder Nachrichten.

## Pros and Cons of the Options

### Passkey-Pflicht und gehärtete CSP

* Gut, weil Owner-Vorgabe erfüllt und Passkey an den Origin gebunden ist (Phishing-Schutz).
* Schlecht, weil ein weiterer Pfad im Browser zu prüfen ist.

### Kein Terminal in der Konsole

* Gut, weil kleinste Angriffsfläche.
* Schlecht, weil es der Owner-Vorgabe widerspricht.

### Jeder Step-up-Weg der Konsole

* Gut, weil auch ohne Passkey nutzbar.
* Schlecht, weil Passwort mit TOTP und OIDC-Anmeldung phishbar sind und die Vorgabe "Passkey"
  verfehlen.

## More Information

* Verwandt: [ADR 0028](0028-web-ziel-nur-fuer-admin-und-statistik-konsole.md),
  [ADR 0068](0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md), ADR 0020.
* Ruling T4 der Leitung.
