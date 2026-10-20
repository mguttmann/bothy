---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Builder-Check (frischer Prüfer)
informed: alle Teams
---

# Terminal über einen Host-Dienst mit Noise-Ende-zu-Ende-Strom und Invariante I6

## Context and Problem Statement

Der Owner verlangt ein Terminal als Pflichtbestandteil (Nachtrag vom 2026-10-10): Shell und
Sitzungen eines Coding-Agenten auf angebundenen Rechnern, in Desktop, Handy und Web-Konsole, mit
eigenem Recht je Rechner, erneuter Passkey-Bestätigung beim Öffnen, Zugang nur über ein privates
Netz oder einen Ende-zu-Ende-Tunnel, Audit, Leerlauf-Abbruch und einem eigenen Benutzer ohne
Administratorrechte auf dem Rechner. Das Produkt darf kein Tailscale voraussetzen (Anforderung 21),
der Server ist Autorisierungspunkt (B7), Agenten-Inhalte und Zugangsdaten bleiben aus dem Server
heraus (B10, I5). Ein Terminal erlaubt jede Nutzung der LLM-Anmeldung des Rechners; Anthropic
erlaubt Abo-Anmeldungen nur für die eigene Nutzung
(https://code.claude.com/docs/en/legal-and-compliance, abgerufen 2026-10-10 [belegt]). Wie kommt
ein Terminal sicher vom Rechner auf das Gerät, und wer darf es öffnen?

## Decision Drivers

* Kein eingehender Port auf dem Rechner; funktioniert hinter NAT, im privaten Netz und in der Cloud.
* Der Server entscheidet jede Öffnung, liest aber keinen Inhalt.
* Geprüfter, kleiner Standard mit öffentlichen Testvektoren statt eigener Kryptografie.
* Keine Hintertür am Rechte-Modell vorbei; Agenten und Apps öffnen nie ein Terminal.
* Persönliche Abos nur für ihren Inhaber, auch auf Rechnern mit mehreren Ownern.
* Ehrliches Schutzziel: was die Verschlüsselung leistet und was nicht.

## Considered Options

* Host-Dienst mit ausgehender WebSocket-Verbindung, Server als Relais, Noise `NK` Ende-zu-Ende
* SSH zum Rechner über das private Netz (Tailscale oder VPN), Server gibt nur Zugangsdaten aus
* Host-Dienst mit TLS-Strom, den der Server terminiert und weiterleitet (kein Ende-zu-Ende)
* WebRTC-Datenkanal zwischen Gerät und Rechner mit Signalisierung über den Server

## Decision Outcome

Gewählt: "Host-Dienst mit ausgehender WebSocket-Verbindung, Server als Relais, Noise `NK`", weil
nur diese Option ohne eingehenden Port und ohne Tailscale-Pflicht auskommt, den Server zur
einzigen Prüfstelle macht und den Inhalt trotzdem vor Relais, Logs und Datenbank schützt.
Normativ: [24-terminal.md](../spec/24-terminal.md).

1. **Host-Dienst** `bothy-host` (eigenes Go-Binary, `services/host`) läuft als Benutzer ohne
   Administratorrechte, prüft das selbst (`doctor`) und spricht nur ausgehend mit dem Server. Die
   Sitzungen laufen als eigener Sitzungsbenutzer, der die Schlüssel des Rechners nicht lesen kann;
   eine zweite lebende Steuerverbindung mit denselben Schlüsseln sperrt den Rechner.
2. **Noise** `Noise_NK_25519_ChaChaPoly_SHA256` (Revision 34, https://noiseprotocol.org/noise.html,
   abgerufen 2026-10-10 [belegt]). Das Gerät kennt den statischen Schlüssel des Rechners aus der
   Kopplung. Prolog mit Protokollversion, Sitzungs-ID und Rechner-ID.
3. **Sitzungs-Marke** im ersten Handshake-Paket: EdDSA-JWS des Servers, genau einmal gültig, an den
   ephemeren Schlüssel des Geräts gebunden (`cek`). Die PTY startet erst nach der ersten gültigen
   Transportnachricht. Grund: Die erste Nachricht von `NK` ist laut Spezifikation wiederholbar
   (Abschnitt 7.7 [belegt]).
4. **Rechte**: eigenes Ziel `host` mit nur dem Zusatzrecht `terminal`, nur für Menschen und
   Gruppen; Owner implizit. Schutzregel SR10: Nicht-Owner nur auf dem eigenen Rechner, der
   Server-Host nur für Owner.
5. **Invariante I6**: Bei `llm_credentials = personal` öffnet nur der Inhaber der Anmeldung ein
   Terminal, auch kein anderer Owner; nur eine Einzel-Freigabe an ihn. Transaktional geprüft und bei
   jedem Öffnen.
6. **Step-up** je Öffnung frisch und verbraucht.

Abgrenzung zu [ADR 0024](0024-backend-richtlinie-personal-organization-mit-invariante.md) und
Ruling L22: I1 kennt keine Owner-Ausnahme, weil Owner auf jedem Ziel `trigger` erben und ein Agent
über viele Ziele erreichbar ist. Ein Terminal ist Zugriff auf genau einen Rechner, keine vererbte
Stufe. Ohne Owner-Ausnahme könnte eine Instanz mit zwei Ownern keinen persönlichen Rechner haben.
Die Ausnahme schränkt Owner ein und erweitert nichts; L22 bleibt für I1 unverändert.

### Consequences

* Gut, weil der Rechner keinen Port öffnet und das Produkt ohne privates Netz funktioniert.
* Gut, weil Relais, Proxy, Logs, Datenbank-Abzug und ein mitlesender api-Prozess keinen Inhalt sehen.
* Gut, weil jede Öffnung durch `authorize` läuft und im Audit steht.
* Schlecht, weil ein bösartiger Server-Betreiber einen eigenen Schlüssel verteilen und Sitzungen
  erlauben kann. Gegenmittel: Schlüssel-Pinning je Gerät mit harter Warnung, Audit jeder
  Neukopplung, Option `host_service.require_local_confirm`; im Bedrohungsmodell als Restrisiko
  geführt (Ruling T3).
* Schlecht, weil mehrere Server-Knoten Hash-Routing im Proxy brauchen, damit beide Enden einer
  Sitzung auf demselben Knoten landen (Ruling T6).
* Schlecht, weil das Produkt den Inhalt einer LLM-Anmeldung nicht prüfen kann; I6 hängt an der
  Erklärung des Owners. Der Host-Dienst prüft das Vorhandensein einer Abo-Anmeldung und verweigert
  den Start bei Widerspruch.

### Confirmation

* Konformität mit öffentlichen Noise-Testvektoren (cacophony, noise-c; 24 Abschnitt 5.2) in Go
  und Dart (TM-T01) und Kreuztest Go gegen Dart (TM-T02). Seit M2-38: TM-T01 hält
  `TestVectorsTMT01` in `services/host/internal/noise` und `test/vectors_test.dart` in
  `packages/bothy_noise`; TM-T02 halten `TestRecordingsTMT02Dart` und
  `TestRecordingsTMT02Negative` in Go und `test/recordings_test.dart` in Dart, über die
  Aufzeichnungen beider Sprachen in beiden Rollen.
* Trennung der Benutzer und Sperre bei zweiter Steuerverbindung (TM-T22, TM-T23). Since M2-43
  TM-T23 holds `TestDuplicate_TMT23` in `services/server/internal/terminal/adapters/wsrelay`.
* Since M2-42 the host side of TM-T13, TM-T22, TM-T24, TM-T26, TM-T27 and the TM-T29 parts (a),
  (b), (c), (d), (e), (f), (h), (i), (j) holds the platform suite of `services/host/test/platform`
  (`TestPlatform_TMT13`, `TestPlatform_TMT22`, `TestPlatform_TMT24`, `TestPlatform_TMT26`,
  `TestPlatform_TMT27`, `TestPlatform_TMT29_Parts`, `TestPlatform_TMT21_SessionUserAddRemove`)
  on disposable GitHub-hosted runners; the unit tests `TestHD1` to `TestHD11`,
  `TestEntryScanSemantics`, `TestReportNamesRule`, the peer checks of `internal/localchan` and
  `TestHostPairAgainstServer` in `services/server/test/terminal` hold the rules on every build
  host. Until a recorded platform run proves a platform, HD10 fails there with
  `peer_check_unavailable` and the host gives no terminal (docs/DECISIONS.md D-M2-42-19).
* Wiedereinspiel- und Marken-Tests im Host-Dienst (TM-T03 bis TM-T05). Since M2-45 they hold in
  `services/host/internal/session`: `TestTMT03Replay`, `TestTMT04TokenFailures`,
  `TestTMT05NoStart`, `TestVerifyOrder` (the check order of 24 5.3 with one row per step) and the
  fuzz target `FuzzVerifySessionToken`. The host module verifies session tokens with its own
  smaller implementation of the ADR 0067 profile, tested with the Ed25519 example of RFC 8037
  A.4 (`TestVerifyRFC8037Vector`) and the negative cases of `TestVerifyOrder` (`alg`, `typ`,
  `crit`, `jwk`, extra dots, `null` and missing claims) (docs/DECISIONS.md D-M2-45-5).
* Since M2-45 the host side of TM-T21, TM-T25, TM-T28 (c) and (d) and the TM-T29 parts with a
  session holds the platform suite of `services/host/test/platform` (`TestPlatform_TMT21_Sessions`,
  `TestPlatform_TMT25`, `TestPlatform_TMT28c`, `TestPlatform_TMT28d`, `TestPlatform_TMT29a_Session`,
  `TestPlatform_TMT29c_Open`, `TestPlatform_TMT29f_Operation`, `TestPlatform_TMT29g_Token`,
  `TestPlatform_TMT29k`, `TestPlatform_TMT29l`) on disposable GitHub-hosted runners
  (docs/DECISIONS.md D-M2-45-18).
* Kanarien-Test über Relais, Logs, Datenbank und Telemetrie (TM-T06).
* Rechte-Testmatrix AZ-M54 bis AZ-M60 und I6-Tests (TM-T09) in `make test`.
* Red Team Terminal am Ende von M2, M4, M5, M6 (Bedrohungsmodell, Abschnitt 6.7).

## Pros and Cons of the Options

### Host-Dienst, Relais, Noise `NK`

* Gut, weil ausgehend, ohne Netz-Voraussetzung, Ende-zu-Ende verschlüsselt.
* Gut, weil `github.com/flynn/noise` (BSD-3-Clause, https://pkg.go.dev/github.com/flynn/noise,
  abgerufen 2026-10-10 [belegt]) `HandshakeNK` und `Prologue` mitbringt.
* Schlecht, weil ein eigenes Paket für den Dart-Handshake nötig ist.

### SSH über das private Netz

* Gut, weil bewährt und mit vorhandenen Werkzeugen nutzbar.
* Schlecht, weil der Rechner einen Port öffnet und das Produkt ein privates Netz voraussetzt
  (Anforderung 21).
* Schlecht, weil der Server die Öffnung nicht mehr selbst entscheidet, sondern nur Schlüssel ausgibt;
  Entzug wirkt erst, wenn der Schlüssel abläuft.

### TLS bis zum Server, Server leitet weiter

* Gut, weil einfach.
* Schlecht, weil der Server jeden Tastendruck und jede Ausgabe im Klartext sieht; ein Dump eines
  api-Prozesses oder ein Log-Fehler legt Shell-Inhalte offen.

### WebRTC-Datenkanal

* Gut, weil direkte Verbindungen den Server entlasten.
* Schlecht, weil STUN und TURN als weitere Dienste nötig sind und das Relais trotzdem gebraucht wird.
* Schlecht, weil der Browser-Stack und die Bibliotheken der Plattformen eine große Angriffsfläche
  haben.

## More Information

* Verwandt: ADR 0020 (Step-up), ADR 0024 (I1), ADR 0046 (Dev-Abschottung), ADR 0067 (JOSE-Profil),
  [ADR 0070](0070-terminal-in-der-web-konsole.md) (Terminal in der Web-Konsole).
* Rulings T1 bis T6, T9, T10 der Leitung (DECISIONS).
* Überprüfung: vor der Veröffentlichung (externer Test P2) und beim Wechsel der Noise-Bibliothek.
