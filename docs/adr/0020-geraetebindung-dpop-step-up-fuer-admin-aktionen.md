---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung
---

# Gerätebindung, DPoP, Step-up für Admin-Aktionen

## Context and Problem Statement

Tokens landen in Logs, Absturzberichten, Proxys und Sicherungen von Geräten. Ein gestohlenes
Bearer-Token öffnet bis zu seinem Ablauf alles, was die Person darf. Verwaltungsaktionen (Freigaben,
Owner, Agenten-Richtlinie, Signierschlüssel) sollen auch dann nicht möglich sein, wenn jemand eine
gültige Sitzung übernimmt. Passkeys als einziger Faktor scheitern in nativen Apple-Apps ohne
öffentlich erreichbare Domain (Baseline R-Passkeys). Die Admin-Konsole läuft im Browser.

Wie binden wir Tokens an Geräte, und wie bestätigt ein Mensch Verwaltungsaktionen?

## Decision Drivers

- Ein gestohlenes Token allein darf nichts nützen.
- Funktioniert hinter TLS-beendenden Proxys (Reverse-Proxy, `tailscale serve`).
- Funktioniert in nativen Apps ohne öffentliche Domain.
- Bestätigung durch den Menschen am Gerät, nicht durch Code im Prozess.
- Browser-Konsole ohne Tokens in JavaScript.

## Considered Options

1. Geräteschlüssel im sicheren Speicher: ein DPoP-Schlüssel für jede Anfrage (RFC 9449) und ein
   Step-up-Schlüssel mit Nutzerbestätigung je Nutzung; Konsole mit Sitzung nach dem
   Backend-for-Frontend-Muster und Step-up per Passkey.
2. Bearer-Tokens mit kurzer Lebensdauer und Refresh-Rotation.
3. Mutual TLS mit Client-Zertifikaten.
4. Passkeys (WebAuthn) überall, auch als Gerätebindung.

## Decision Outcome

Gewählte Option: 1, weil sie Tokens auf Anwendungsebene bindet (unabhängig davon, wo TLS endet),
ohne öffentliche Domain auskommt und Step-up an Biometrie oder PIN des Geräts knüpft.

Festlegungen ([../spec/08-identitaet.md](../spec/08-identitaet.md)):

- Zwei nicht exportierbare Schlüssel je nativem Gerät: `dpop` (ohne Abfrage je Nutzung) und
  `step_up` (Biometrie oder PIN je Nutzung).
- Zugriffstoken mit `cnf.jkt` und `sid`, kurz gültig; DPoP-Beweis mit Server-Nonce, `ath`,
  `jti`-Prüfung; Bearer-Nutzung eines DPoP-Tokens wird abgelehnt.
- Refresh-Tokens rotierend in Familien, an den DPoP-Schlüssel der Familie gebunden; Wiederverwendung
  widerruft Familie und Sitzung. Getrennte Familien für App-Erweiterungen, die eigenständig
  erneuern.
- Step-up gilt `auth.step_up_max_age` für die Sitzung, in der er erbracht wurde; nur Menschen.
- Konsole: Cookie mit `__Host-`, `Secure`, `HttpOnly`, `SameSite=Strict`; CSRF-Kopfzeile und
  `Origin`-Prüfung; Step-up per Passkey, sonst erneute OIDC-Anmeldung mit `max_age=0`.

### Consequences

- Gut, weil Token-Diebstahl ohne Schlüssel wertlos ist.
- Gut, weil Proxys mit TLS-Ende die Bindung nicht brechen.
- Gut, weil Verwaltung eine körperliche Bestätigung am Gerät braucht.
- Schlecht, weil jede Anfrage eine Signatur und der Server eine `jti`-Prüfung und Nonce-Verwaltung
  braucht.
- Schlecht, weil die Plattformen unterschiedlich sind; ob Windows Hello den Step-up-Schlüssel als
  `ES256` liefert, ist offen.
- Schlecht, weil die Konsole einen zweiten Sitzungsweg (Cookie) hat, der eigene Tests braucht.

### Confirmation

Tests ID-T12 bis ID-T19 und ID-T26 in 08 (DPoP-Fehlerfälle, Refresh-Wiederverwendung, getrennte
Familien, Schlüsselbindung der Familie, Step-up-Fälle, Konsole-Cookie und CSRF, Geräte-Widerruf);
Schutzregel SR5 in 07 mit AZ-M35 und AZ-M36.

## Pros and Cons of the Options

### 1. Geräteschlüssel mit DPoP und Step-up-Schlüssel

- Gut, weil standardisiert (RFC 9449) und unabhängig vom Transport.
- Gut, weil Schlüssel im sicheren Speicher nicht kopierbar sind.
- Neutral, weil Hardware-Bindung ohne Attestierung nicht beweisbar ist; Attestierung wird
  gespeichert und kann später Pflicht werden.
- Schlecht, weil mehr Logik in Clients und Server.

### 2. Bearer mit kurzer Lebensdauer

- Gut, weil einfach und überall unterstützt.
- Schlecht, weil ein gestohlenes Token bis zum Ablauf voll nutzbar ist und ein gestohlenes
  Refresh-Token neue Tokens holt, bis die Rotation den Diebstahl bemerkt.

### 3. Mutual TLS

- Gut, weil die Bindung im Transport liegt.
- Schlecht, weil TLS-beendende Proxys das Zertifikat nicht durchreichen, ohne dass der Server einer
  Kopfzeile vertrauen muss.
- Schlecht, weil Zertifikatsverteilung auf Mobilgeräte und im Browser aufwendig ist.

### 4. Passkeys überall

- Gut, weil phishing-resistent und standardisiert.
- Schlecht, weil native Apple-Apps eine öffentlich erreichbare Associated Domain brauchen; private
  Instanzen hätten keinen Weg.
- Schlecht, weil WebAuthn keine Bindung einzelner API-Anfragen liefert.

## More Information

- RFC 9449 (DPoP): https://www.rfc-editor.org/rfc/rfc9449.html, abgerufen 2026-10-10 [belegt].
- RFC 9700, Abschnitt 2.2.2 (Refresh-Tokens öffentlicher Clients sender-gebunden oder rotierend),
  abgerufen 2026-10-10 [belegt].
- RFC 10017 (Browser-Anwendungen, BFF-Muster und Cookie-Attribute):
  https://datatracker.ietf.org/doc/rfc10017/, abgerufen 2026-10-10 [belegt].
- WebSocket-Variante des DPoP-Beweises: [../spec/04-sync-protokoll.md](../spec/04-sync-protokoll.md),
  R-SY-01.
