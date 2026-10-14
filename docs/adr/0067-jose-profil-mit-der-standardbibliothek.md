---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
---

# JOSE-Profil mit der Standardbibliothek statt einer JOSE-Bibliothek

## Context and Problem Statement

Der Server baut und prüft kompakte JWS an mehreren Stellen: Zugriffstokens (EdDSA, Zweck
`access_token`), Aufruf-Marken für Agenten (Zweck `invocation`), Prüfpunkte des Audit-Logs (Zweck
`audit_checkpoint`), DPoP-Beweise und Step-up-Signaturen der Geräte (`ES256`, `EdDSA`), ID-Tokens
der OIDC-Anbieter (`RS256`, `PS256`, `ES256`, `EdDSA`) und Client-Assertions für `private_key_jwt`
([08-identitaet.md](../spec/08-identitaet.md), Abschnitte 2.2, 6.2, 7.2 und 7.3;
[19-sicherheit.md](../spec/19-sicherheit.md), Abschnitt 12). Diese Stellen liegen in verschiedenen
Modulen (`identity`, `instance`, `audit`, `runs`), und Domain-Pakete dürfen nur Pakete aus
`kernel-core` importieren, keine Fremdbibliotheken ([02-architektur.md](../spec/02-architektur.md),
Abschnitt 5). Zwei Bau-Tickets derselben Welle (PLAN M1-32 und M1-33) hätten ohne Festlegung
jeweils eine eigene Lösung gewählt. Womit baut und prüft der Server JWS?

## Decision Drivers

* RFC 8725 verlangt, dass der Aufrufer die erlaubten Algorithmen festlegt und keine anderen
  benutzt werden (Abschnitt 3.1 "Perform Algorithm Verification"), und empfiehlt einen expliziten
  Typ (Abschnitt 3.11 "Use Explicit Typing")
  (https://www.rfc-editor.org/rfc/rfc8725.html, abgerufen 2026-10-10) [belegt].
* Eine Implementierung für alle Module; Signieren und Prüfen dürfen nicht auseinanderlaufen.
* Nutzbar aus Domain-Paketen, also ohne Fremdbibliothek.
* Kleine Angriffsfläche: Gebraucht werden nur die kompakte Form, wenige Algorithmen und JWK-
  Fingerabdrücke nach RFC 7638; JWE, JSON-Serialisierung und Schlüssel aus Kopfzeilen nicht.
* Wenig Abhängigkeiten in einem Sicherheitspfad (Lieferkette, SEC-SC-1).

## Considered Options

* Eigenes JOSE-Profil mit der Go-Standardbibliothek im Paket `internal/kernel/jose`
* Eine JOSE-Bibliothek (zum Beispiel `github.com/go-jose/go-jose`) in den Adaptern
* Eine umfassende JOSE-Bibliothek (zum Beispiel `github.com/lestrrat-go/jwx`) in den Adaptern
* Jedes Modul baut seine JWS selbst

## Decision Outcome

Gewählt: "Eigenes JOSE-Profil mit der Standardbibliothek in `internal/kernel/jose`", weil es als
einzige Option aus Domain-Paketen nutzbar ist, genau das gebrauchte Profil abdeckt und die
Algorithmus-Liste bei jedem Aufruf erzwingt.

Profil (normativ zusammen mit 08):

1. Nur die kompakte Serialisierung von JWS (RFC 7515). Kein JWE, keine JSON-Serialisierung.
2. Jeder Aufruf nennt die erlaubten Algorithmen und den erwarteten `typ`. `none`, `HS*` und jeder
   nicht genannte Algorithmus werden abgelehnt, ebenso Kopfzeilen mit `crit`, `jku`, `x5u`, `x5c`
   oder `zip`. Algorithmen des Profils: `EdDSA` (Ed25519), `ES256` (Signatur als `R || S` mit
   64 Byte nach RFC 7518, Abschnitt 3.4, nie DER), `RS256` und `PS256` (nur Prüfen von ID-Tokens und
   Signieren von Client-Assertions).
3. Den Prüfschlüssel liefert immer der Aufrufer (`kid` aus seiner Schlüsselliste). Einzige
   Ausnahme ist der öffentliche `jwk` im Kopf eines DPoP-Beweises (RFC 9449); ein `jwk` mit
   privaten Teilen wird abgelehnt.
4. JWK-Fingerabdrücke nach RFC 7638 für `EC` (P-256), `OKP` (Ed25519) und `RSA`.
5. Signieren nimmt einen Signer über die Eingabe der Signatur; private Schlüssel bleiben beim
   Aufrufer (für Server-Schlüssel der Port `instance.Keys`, ADR 0066).

Wer was baut: PLAN M1-33 legt das Paket mit Parsen, Prüfen (`EdDSA`, `ES256`), Fingerabdrücken und
dem Zusammensetzen kompakter JWS an; M1-34 setzt damit die Zugriffstokens zusammen; M1-41 ergänzt
`RS256` und `PS256` (RSA-Schlüssel mit 2048 bis 8192 Bit nur über `ParseVerificationJWK`, nie für
Geräteschlüssel) und beim Signieren der Client-Assertion die Kopfzeile `x5t#S256`, die Entra für
Zertifikate verlangt; Prüfen liest sie nicht. Jede Erweiterung des Profils (ein Algorithmus, eine Kopfzeile) braucht eine
Änderung dieser ADR.

### Consequences

* Gut, weil jeder JWS-Pfad dieselbe Algorithmus-Prüfung durchläuft und kein Modul eine zweite
  Implementierung einführt.
* Gut, weil Domain-Pakete JWS prüfen können, ohne die Schichtregel zu brechen.
* Gut, weil keine Fremdbibliothek im Anmeldepfad steht.
* Schlecht, weil eigener Code im Kryptografie-Umfeld ein Risiko ist. Gegenmittel: nur Aufbau und
  Prüfung des Formats sind eigener Code, alle kryptografischen Verfahren kommen aus der
  Standardbibliothek; Testvektoren aus den RFCs, Negativtests und ein Fuzz-Test (Confirmation).
* Schlecht, weil ein späterer Bedarf an JWE oder weiteren Algorithmen eine neue Entscheidung
  braucht. Das ist gewollt.

### Confirmation

* Unit-Tests in `internal/kernel/jose` mit den Beispielen aus RFC 7515 Anhang A.3 (`ES256`),
  RFC 8037 Anhang A.4 (`Ed25519`) und RFC 7638 Abschnitt 3.1 (Fingerabdruck).
* Negativtests: `alg` gleich `none`, `HS256` oder nicht in der erlaubten Liste, falscher `typ`,
  `ES256`-Signatur in DER-Form, `crit` im Kopf, `jwk` mit privatem Teil, zusätzliche Punkte oder
  Zeichen außerhalb von base64url; alle abgelehnt.
* Go-Fuzz-Test des Parsers mit Seed-Korpus im normalen Testlauf (PLAN M1-33).
* Das Architektur-Gate (`make arch`) erlaubt `internal/kernel/jose` nur Importe der
  Standardbibliothek ([02-architektur.md](../spec/02-architektur.md), Abschnitt 7.2).

## Pros and Cons of the Options

### Eigenes Profil mit der Standardbibliothek

* Gut, weil klein, prüfbar und aus jeder Schicht nutzbar.
* Gut, weil die Liste erlaubter Algorithmen ein Pflichtparameter jeder Prüffunktion ist.
* Schlecht, weil Format-Code selbst geschrieben und gepflegt wird.

### `go-jose` in den Adaptern

* Gut, weil verbreitet und auf JOSE spezialisiert.
* Schlecht, weil Domain-Pakete sie nicht importieren dürfen; die Prüfung läge dann in Adaptern,
  getrennt von den Regeln, die sie braucht.
* Schlecht, weil sie mehr Formate und Algorithmen kann, als das Produkt braucht.

### `jwx` in den Adaptern

* Gut, weil sie JWT, JWK-Mengen und Caches mitbringt.
* Schlecht, weil sie noch mehr Umfang mitbringt als nötig und dieselbe Schichtgrenze trifft.

### Jedes Modul baut selbst

* Gut, weil ohne Abstimmung möglich.
* Schlecht, weil mehrere Implementierungen auseinanderlaufen; eine fehlende Algorithmus-Prüfung an
  einer Stelle genügt für einen Angriff.

## More Information

* Verwandt: ADR 0020 (DPoP), ADR 0066 (Signierschlüssel), ADR 0021 (Audit-Prüfpunkte).
* Normative Stellen: [08-identitaet.md](../spec/08-identitaet.md), Abschnitte 7.2 und 7.3;
  [02-architektur.md](../spec/02-architektur.md), Abschnitt 4.5.
