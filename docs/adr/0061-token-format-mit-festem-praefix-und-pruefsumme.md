---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team C (Identität), Team H (Qualität und Repo), Team B (API)
informed: Team D (Agenten-Plattform)
---

# Token-Format mit festem Präfix und Prüfsumme

## Context and Problem Statement

Agenten, Apps, persönliche Werkzeuge und Eingangs-Webhooks melden sich mit langlebigen API-Tokens an
([08-identitaet.md](../spec/08-identitaet.md), Abschnitt 12). Solche Tokens landen erfahrungsgemäß
in Konfigurationsdateien, Logs und Commits. Ein Secret-Scanner findet sie nur, wenn sie sich von
anderen Zufallsdaten (Hashes, IDs) unterscheiden. Zufällige Funde und Tippfehler sollen den Server
nicht mit Datenbankabfragen belasten. Welches Format haben API-Tokens?

## Decision Drivers

- Geleakte Tokens erkennt ein Secret-Scanner zuverlässig und mit wenig Fehlalarmen (Gate
  `gitleaks`, [23-qualitaet.md](../spec/23-qualitaet.md)).
- Tippfehler und Zufallsfunde werden ohne Datenbankzugriff abgewiesen.
- Genug Zufall, dass Raten aussichtslos ist; gespeichert wird nur ein Hash (03, Grundsatz 9).
- Widerruf wirkt sofort; die Art des Tokens ist sichtbar.
- Der Produktname steht nur an gelisteten Stellen (ADR 0044, Ruling L6).

## Considered Options

- Festes Präfix, Art, Zufall in Base62 und Prüfsumme: `bothy_<art>_<zufall>_<prüfsumme>`
- Undurchsichtiger Zufall ohne Struktur
- Selbsttragende signierte Tokens (JWT oder PASETO)
- Capability-Tokens mit Einschränkung (Macaroons, Biscuit)
- Festes Präfix ohne Prüfsumme

## Decision Outcome

Gewählte Option: "Festes Präfix, Art, Zufall und Prüfsumme", weil Scanner das Präfix eindeutig
finden und die Prüfsumme Fehlalarme ohne Datenbank verwirft. Normativ in
[08-identitaet.md](../spec/08-identitaet.md), Abschnitt 12, und
[03-datenmodell.md](../spec/03-datenmodell.md), Tabelle `api_tokens`.

1. **Form.** `bothy_<art>_<zufall>_<prüfsumme>`. Art: `agt` (Agent), `app` (App), `pat`
   (persönlich), `whi` (Eingangs-Webhook), `hpc` (Kopplungs-Code eines Rechners für das Terminal,
   Nachtrag 2026-10-10, [24-terminal.md](../spec/24-terminal.md) Abschnitt 3.2). Zufall: 256 Bit aus einem kryptografischen
   Zufallsgenerator, in Base62 als genau 43 Zeichen. Base62 nutzt das Alphabet `0-9A-Za-z` in
   dieser Reihenfolge (Ziffernwert 0 bis 61). Prüfsumme: CRC-32 (IEEE) über die Zeichen des
   Zufallsteils, in Base62 als genau 6 Zeichen mit führenden Nullen.
2. **Prüfreihenfolge** bei jeder Anfrage: Form, Prüfsumme, dann Suche über
   HMAC-SHA-256 mit dem Server-Pepper, dann Ablauf, Widerruf und DPoP-Bindung. Ein Fehler in Form
   oder Prüfsumme ergibt `401` ohne Datenbankzugriff.
3. **Speicher.** Nur der HMAC (`token_hash`) und `token_prefix` (`bothy_<art>_` plus die ersten
   6 Zeichen des Zufallsteils) zum Wiedererkennen in der Konsole. Anzeige genau einmal bei
   `token.create`.
4. **Grenzen** wie in 08, Abschnitt 12: Pflicht-Ablauf bis `auth.api_token_max_ttl` bzw.
   `auth.personal_token_max_ttl`, DPoP-Bindung nach `auth.dpop_required_for`, kein Step-up, keine
   Owner-Rechte über persönliche Tokens.
5. **Scanner.** Die gitleaks-Regel `bothy-api-token` (`.gitleaks.toml`) erkennt genau dieses
   Format, mit genau 6 Zeichen Prüfsumme.

Die Prüfsumme ist kein Schutz gegen Fälschung, sondern ein Filter: CRC-32 erkennt Tippfehler und
zufällige Treffer; die Echtheit prüft der HMAC-Vergleich. Dasselbe Muster (Präfix, Unterstrich,
Base62, CRC-32 in 6 Zeichen) nutzt GitHub für seine Tokens [Q1, belegt].

### Consequences

- Gut, weil ein geleaktes Token im Repo (Gate `gitleaks`) und mit eigenen Mustern in
  Secret-Scanning-Diensten als Produkt-Token erkannt wird; der Unterstrich trennt das Präfix von
  Base62-Daten [Q1, belegt].
- Gut, weil Müll-Tokens (Scanner-Treffer, Tippfehler, Angriffe mit Zufallswerten) nie die
  Datenbank erreichen.
- Gut, weil ein schneller HMAC genügt: Bei 256 Bit Zufall ist Raten aussichtslos, ein langsamer
  Passwort-Hash würde nur jede Anfrage verlangsamen.
- Schlecht, weil der Arbeitsname im Präfix steht; ein Umbenennen braucht eine Übergangszeit, in der
  der Server alte Präfixe bis zum Ablauf der Tokens annimmt (ADR 0044 führt die Stelle).
- Schlecht, weil Tokens nicht selbsttragend sind: Jede Anfrage braucht eine Suche (mit Cache).
  Dafür wirkt ein Widerruf sofort.

### Confirmation

- `tools/ci/tests/test_gitleaks_rules.py`: Die Regel `bothy-api-token` findet ein zur Laufzeit
  gebautes Token mit 43 Zeichen Zufall und 6 Zeichen Prüfsumme.
- Test ID-T23 ([08-identitaet.md](../spec/08-identitaet.md), Abschnitt 15): nur Hash gespeichert,
  einmal angezeigt, falsche Prüfsumme und abgelaufene Tokens abgelehnt, DPoP-pflichtiges
  Agenten-Token ohne Beweis abgelehnt.
- Unit-Test des Formats: Erzeugen und Parsen ergeben dasselbe; eine Prüfsumme mit führenden Nullen
  hat 6 Zeichen; jede Änderung eines Zeichens im Zufallsteil fällt an der Prüfsumme auf.
- Test: Ein Token mit falscher Prüfsumme wird ohne Datenbankabfrage abgelehnt (Abfragezähler des
  Test-Repositorys bleibt 0).
- Bau-Ticket M1-44 in [PLAN.md](../PLAN.md). Built (2026-10-13): `TestFormat_RoundTrip`,
  `TestFormat_ChecksumVectors`, `TestParse_Rejects`, `TestGenerate_ReadsExactly32BytesFromCryptoRand`
  and `TestTokenPrefix` in `services/server/internal/identity/domain/apitoken`, the vectors
  recomputed by `test_checksum_vectors_match_go`, and `TestIDT23_WrongChecksumRejectedWithoutLookup`
  (lookup counter stays 0). Identity does not cache token lookups (one indexed query per request);
  authz caches the token liveness with its rights snapshot per rights version, and every
  revocation raises that version, so a revocation holds at the next request in every process.
- Kind `hpc` (M2-41, 2026-10-15): the vector of random part `EXAMPLEhostpairingcode000000000000000000000`
  in `testdata/checksum_vectors.json`; `TestHostPairingCode_MintAndHash`,
  `TestHashHostPairingCode_RejectsWithoutLookup` and `TestHostPairingCode_MatchesGitleaksRule` in
  `services/server/internal/identity/app`; `test_pairing_code_with_valid_checksum_is_detected` with
  the pinned gitleaks binary; `TestTMT20_WrongChecksumNoDatabase` (no unit of work and no query for
  a wrong checksum or kind) and `TestTMT20_PairingCodeRejections` in the terminal module.

## Pros and Cons of the Options

### Präfix, Art, Zufall, Prüfsumme

- Gut, weil erkennbar, schnell zu verwerfen und ohne Zustand im Format.
- Schlecht, weil die Form mehr Regeln hat als reiner Zufall.

### Undurchsichtiger Zufall

- Gut, weil einfach.
- Schlecht, weil Scanner solche Werte nicht von Hashes oder IDs unterscheiden; GitHub nennt genau
  das als Grund für sein neues Format [Q1, belegt].

### JWT oder PASETO

- Gut, weil der Server ohne Suche prüfen könnte.
- Schlecht, weil ein Widerruf eine Sperrliste oder kurze Laufzeiten braucht; langlebige
  Agenten-Tokens sollen aber sofort widerrufbar sein.
- Schlecht, weil die Tokens lang sind und Inhalte (Ansprüche) lesbar in Logs tragen.

### Macaroons oder Biscuit

- Gut, weil der Inhaber Rechte selbst weiter einschränken kann.
- Schlecht, weil Rechte im Produkt aus Freigaben und Scopes im Server kommen (07, 11); eine zweite
  Rechte-Sprache im Token widerspräche der einen Autorisierungsfunktion (ADR 0017).

### Präfix ohne Prüfsumme

- Gut, weil kürzer.
- Schlecht, weil jeder Treffer mit passendem Präfix eine Datenbankabfrage auslöst und Scanner mehr
  Fehlalarme melden; laut GitHub beseitigt erst die Prüfsumme Fehlalarme beim Scannen ohne
  Datenbank fast ganz [Q1, belegt].

## More Information

- Quelle (abgerufen 2026-10-10): Q1 GitHub Blog, "Behind GitHub's new authentication token
  formats",
  https://github.blog/engineering/platform-security/behind-githubs-new-authentication-token-formats/
  [belegt].
- Verwandt: ADR 0020 (DPoP), ADR 0038 (lokale Gates), ADR 0044 (Arbeitsname).
