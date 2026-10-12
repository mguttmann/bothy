# Noise-Testdaten

Gemeinsame Testdaten des Host-Dienstes (`services/host/internal/noise`,
`services/host/internal/fingerprint`) und des Dart-Pakets `packages/bothy_noise`
([24 Terminal](../../../../docs/spec/24-terminal.md), Abschnitte 4.3, 5.2 und 14). Nichts davon
wird ausgeliefert; die Dateien enthalten nur öffentliche Testschlüssel.

## Testvektoren (TM-T01)

Die beiden Dateien liegen unverändert, aber mit `gzip -n -9` komprimiert unter `vectors/`. Grund
(docs/DECISIONS.md D-M2-38-15): Die Kompression hält den Inhalt Byte für Byte unter
`repo.max_file_kb`, schützt ihn vor den Hygiene-Hooks und hält die veröffentlichten Testschlüssel
aus den Textregeln der Geheimnis-Scanner. Die Prüfsumme gilt für den entpackten Inhalt;
`TestVectorProvenance` hält diese Tabelle gegen die Dateien.

| Datei | Quelle | Commit | Lizenz | SHA-256 des Inhalts |
|---|---|---|---|---|
| `vectors/cacophony.txt.gz` | https://github.com/haskell-cryptography/cacophony, Pfad `vectors/cacophony.txt` (früher `centromere/cacophony`, GitHub leitet um) | `8ee9d41e34a1a596cfa3ab12aa4069ff87dc1247` | `Unlicense` | `3bde7c09a6f349ee11c825c50fcc02649f8f02a47c857a459206b357f9386cae` |
| `vectors/noise-c-basic.txt.gz` | https://github.com/rweather/noise-c, Pfad `tests/vector/noise-c-basic.txt` | `cfe25410979a87391bb9ac8d4d4bef64e9f268c6` | `MIT` | `e826749cf90efda26be85410381f4b1f75552c61c7c0dcad7f9ffb4d639e4e45` |

Lizenztexte: `vectors/LICENSE.cacophony` (Unlicense, Widmung an die Gemeinfreiheit) und
`vectors/LICENSE.noise-c` (MIT, Datei `COPYING` des Projekts). Abgerufen am 2026-10-12.

Laderegeln in Go und Dart: Das Protokoll eines Eintrags ist `protocol_name`, sonst `name`, sonst
`Noise_<pattern>_<dh>_<cipher>_<hash>`. Gewählt werden die Einträge gleich
`Noise_NK_25519_ChaChaPoly_SHA256` ohne PSK und ohne `fallback`; `NoisePSK_NK_25519_ChaChaPoly_SHA256`
aus noise-c (PSK einer älteren Revision) fällt heraus. `resp_prologue` fehlt, dann gilt
`init_prologue`. Die ersten zwei Nachrichten sind der Handshake, der Rest Transport, abwechselnd
Initiator und Responder. Jede Datei muss mindestens einen Eintrag liefern.

## Aufzeichnungen (TM-T02)

`recordings/go.json` schreibt Go, `recordings/dart.json` schreibt Dart. Jede Sprache vergleicht
ihre eigene Datei Byte für Byte mit einer frischen Erzeugung und spielt die Datei der anderen
Sprache als Initiator und als Responder nach; dazu prüfen beide, dass ein falscher Prolog, ein
falscher statischer Schlüssel und vertauschte Sitzungs- und Rechner-ID scheitern. Inhalt: drei
Szenarien (`open_shell`, `attach_replay`, `limits`) mit festen Schlüsseln (SHA-256 von
`noise-recording/<generator>/<szenario>/<rolle>`), dem Handshake und den Rahmen aus 24 5.4.

Neu schreiben, wenn sich das Format oder die Erzeugung ändert:

```sh
make -C services/host recordings
(cd packages/bothy_noise && UPDATE_RECORDINGS=1 flutter test test/recordings_test.dart)
```

## Fingerabdruck-Vektoren

`fingerprint.json` (`make -C services/host recordings`, geprüft von `TestFingerprintFileCurrent`
in Go und `fingerprint vectors match` in Dart) enthält zuerst den bekannten Vektor aus 24 4.3,
dann acht Einträge mit den Schlüsseln SHA-256 von `fingerprint/noise/<i>` und
`fingerprint/auth/<i>` für `i` von 1 bis 8. Felder: `noise_public` (X25519-Schlüssel, hex),
`ed25519_public` (roher Ed25519-Schlüssel, hex) und `fingerprint` (Anzeigeform). Die Feldnamen
meiden die Stichwörter der gitleaks-Regel `generic-api-key`. Der Server (M2-41) prüft seine eigene
Berechnung gegen dieselbe Datei.
