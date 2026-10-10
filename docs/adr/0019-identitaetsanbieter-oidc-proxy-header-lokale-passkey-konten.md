---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung
---

# Identitätsanbieter: OIDC, Proxy-Header, lokale Passkey-Konten

## Context and Problem Statement

Das Produkt ist selbst gehostet und öffentlich. Betreiber haben sehr verschiedene Umgebungen: ein
Unternehmen mit Microsoft Entra ID, ein kleines Team hinter einem privaten Overlay-Netz mit
Identitäts-Kopfzeilen (z. B. `tailscale serve`), oder eine Installation ganz ohne Identitätsanbieter.
Das Produkt darf weder Tailscale noch einen bestimmten Anbieter voraussetzen (Baseline B9). Native
Apps auf iOS, Android, macOS und Windows müssen sich anmelden, ohne Geheimnisse in der App.

Wie melden sich Menschen an, und wie wird eine externe Identität einem Konto zugeordnet?

## Decision Drivers

- Kein Selbst-Registrieren; neue Menschen kommen über den Owner (Studie C).
- Keine Anbieter-Tokens und keine Client-Geheimnisse auf Geräten.
- Ein Token-Format für alle Wege, gebunden an Geräteschlüssel (ADR 0020).
- Sichere Zuordnung ohne veränderliche Merkmale wie E-Mail.
- Native Apps: Systembrowser, keine eingebetteten Web-Views (RFC 8252).
- Passkeys funktionieren in nativen Apple-Apps nur mit öffentlich erreichbarer Associated Domain
  (Baseline R-Passkeys).

## Considered Options

1. Port mit drei Treibern (OIDC, vertrauenswürdiger Proxy-Header, lokale Konten); der Server ist
   OIDC-Relying-Party und Autorisierungsdienst der eigenen Apps.
2. Nur OIDC; Instanzen ohne Anbieter bekommen einen mitgelieferten OIDC-Dienst.
3. Nur Identität aus dem Overlay-Netz (Proxy-Kopfzeilen).
4. Apps melden sich direkt beim Anbieter an und senden dessen Tokens an den Server.

## Decision Outcome

Gewählte Option: 1, weil sie alle drei Umgebungen abdeckt, ohne einen Dienst vorauszusetzen, und
weil der Server als Vermittler einheitliche, gerätegebundene Tokens ausstellt.

Festlegungen ([../spec/08-identitaet.md](../spec/08-identitaet.md)):

- OIDC: Authorization Code mit PKCE (`S256`) auf beiden Strecken (App zum Server, Server zum
  Anbieter); Server als vertraulicher Client mit `private_key_jwt` oder Secret aus dem
  Secret-Provider; Prüfung von `iss` gegen Mix-up (RFC 9700, RFC 9207).
- Zuordnung nur über `(issuer, subject)` oder einen einmaligen Einladungs-Code; nie über E-Mail.
- Proxy-Header nur, wenn alle Listener an Loopback gebunden sind; nur von vertrauten Gegenstellen;
  nur zur Anmeldung, optional an jede Anfrage gebunden.
- Lokale Konten: Passkey (Konsole, öffentliche Domains), Einladungs-Code, Geräte-Kopplung,
  Passwort mit TOTP als abschaltbarer Rückfall.
- Erster Owner nur per Server-Befehl am Host; vorher antwortet die API `503 setup_required`.

### Consequences

- Gut, weil kein Betreiber einen bestimmten Anbieter oder ein bestimmtes Netz braucht.
- Gut, weil Apps nur mit dem eigenen Server sprechen und keine Anbieter-Tokens halten.
- Gut, weil ein vergessener E-Mail-Abgleich als Angriffsweg (Kontoübernahme) ausgeschlossen ist.
- Schlecht, weil der Server Anmeldevorgänge, Rückleitungen und Einladungs-Codes selbst führen muss
  (mehr eigener Code mit Sicherheitsbezug, also mehr Tests).
- Schlecht, weil eine Sperre im Anbieter erst bei der nächsten Anmeldung wirkt
  (`auth.idp_recheck_interval`); die automatische Übernahme (SCIM oder Shared Signals) ist eine
  vorgeschlagene Folge-ADR.

### Confirmation

Tests ID-T01 bis ID-T11 und ID-T20 bis ID-T25 sowie ID-T27 in 08 (PKCE-Downgrade, `state`, `iss`, `alg`, Zuordnung
ohne E-Mail, `tid`-Liste, Erstinstallation, Loopback-Pflicht, Kopfzeilen-Bindung, Einladungs-Codes,
TOTP-Wiederholung, Antwortzeit, Argon2id-Untergrenze, vertraute Proxys).

## Pros and Cons of the Options

### 1. Drei Treiber, Server als Vermittler

- Gut, weil alle Zielumgebungen ohne Zusatzdienst abgedeckt sind.
- Gut, weil Geräte- und Token-Logik für alle Treiber gleich ist.
- Schlecht, weil der Server Teile eines Autorisierungsdienstes selbst implementiert.

### 2. Nur OIDC mit mitgeliefertem Dienst

- Gut, weil nur ein Anmeldeweg im Server.
- Schlecht, weil kleine Installationen einen weiteren Dienst betreiben, sichern und aktualisieren
  müssen.
- Schlecht, weil der Proxy-Fall trotzdem eine Brücke bräuchte.

### 3. Nur Identität aus dem Overlay-Netz

- Gut, weil sehr einfach, wo das Netz schon besteht.
- Schlecht, weil es das Produkt an ein Netz bindet (verboten durch Baseline B9).
- Schlecht, weil getaggte Geräte und öffentlicher Verkehr keine Identität tragen.

### 4. Apps direkt beim Anbieter

- Gut, weil der Server keinen Anmeldevorgang führt.
- Schlecht, weil Anbieter-Tokens auf Geräten liegen und der Server Tokens fremder Formate prüfen
  muss.
- Schlecht, weil Gerätebindung (DPoP) vom Anbieter abhängt.
- Schlecht, weil jeder Anbieter eigene Rückleitungen je Plattform verlangt.

## More Information

- RFC 8252 (Native Apps): https://www.rfc-editor.org/rfc/rfc8252.txt, abgerufen 2026-10-10 [belegt].
- RFC 9700 (OAuth 2.0 Security BCP): https://www.rfc-editor.org/rfc/rfc9700.html, abgerufen
  2026-10-10 [belegt].
- Microsoft zu Claims: https://learn.microsoft.com/en-us/entra/identity-platform/claims-validation,
  abgerufen 2026-10-10 [belegt].
- Tailscale Serve: https://tailscale.com/kb/1312/serve, abgerufen 2026-10-10 [belegt].
- Gerätebindung und Step-up: ADR 0020.
