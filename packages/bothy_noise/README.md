# bothy_noise

Reines Dart-Paket mit der Ende-zu-Ende-Schicht des Terminals
([24 Terminal](../../docs/spec/24-terminal.md), Abschnitte 4.3, 5.2 bis 5.4): Handshake
`Noise_NK_25519_ChaChaPoly_SHA256` als Initiator, Transport, Prolog, Rahmen und Fingerabdruck.
Desktop-App, mobile App und Web-Konsole nutzen denselben Code.

## Zusicherungen

- Kein `package:flutter`, kein `dart:io`, kein `dart:html`, kein Netz und kein Speicher in `lib/`;
  einzige Abhängigkeit ist `cryptography` (X25519, ChaCha20-Poly1305, SHA-256, HMAC)
  ([15 Client-Architektur](../../docs/spec/15-client-architektur.md), 3.1 Regel 7 und 23.1).
- Gleiche Bytes wie der Host-Dienst in Go: die öffentlichen Testvektoren (TM-T01) und die
  Aufzeichnungen beider Sprachen in beiden Rollen (TM-T02) unter
  [`services/host/testdata/noise/`](../../services/host/testdata/noise/README.md).
- Ein falscher Prolog, ein falscher statischer Schlüssel oder vertauschte Sitzungs- und
  Rechner-ID lassen den Handshake scheitern. Nach einem gescheiterten Schritt oder einer
  gescheiterten Entschlüsselung wirft jedes weitere Aufrufen `NoiseError.broken`.
- Fehlertexte nennen Art, Vorgang und Längen, nie Nutzlast, Klartext, Schlüssel oder Marken.
- `encrypt` nimmt seine Nonce beim Aufruf; die Nachrichten in der Reihenfolge der Aufrufe senden.

## Bibliotheken

| Import | Inhalt |
|---|---|
| `package:bothy_noise/bothy_noise.dart` | `NkInitiator`, `NkResult`, `NoiseTransport`, `uuidToBytes`, `buildPrologue`, `generateEphemeralKeyPair`, `ephemeralKeyHash`, Rahmen (`TerminalFrame`, `encodeFrame`, `decodeFrame`), `crockfordBase32`, `hostKeyFingerprint`, Konstanten, Fehler |
| `package:bothy_noise/testing.dart` | nur `NkResponder`, die Rechner-Seite für Tests und M4-35 als Schein-Rechner |

## Ablauf im Gerät

1. `generateEphemeralKeyPair()`, dann `ephemeralKeyHash` des öffentlichen Schlüssels als
   `client_ephemeral_key_hash` beim Öffnen senden (15 23.2, Schritt 2).
2. `NkInitiator.start` mit dem gepinnten `noise_public_key`, Sitzungs- und Rechner-ID
   (`uuidToBytes`) und demselben Schlüsselpaar; `writeMessage1(marke)` senden.
3. `readMessage2` liefert Nutzlast, `NoiseTransport` und Handshake-Hash; danach Rahmen mit
   `encodeFrame` und `transport.encrypt`, als erstes `StartFrame`.

Was noch fehlt: Pin-Speicher, Ansicht und Renderer (M4-35), der Pub-Workspace (M4-01).

## Befehle

```sh
cd packages/bothy_noise
flutter analyze --fatal-infos
flutter test --coverage
UPDATE_RECORDINGS=1 flutter test test/recordings_test.dart   # recordings/dart.json neu schreiben
```

Flutter kommt aus der Zeile `environment.flutter` der `pubspec.yaml` (docs/DECISIONS.md
D-M2-38-12); das Paket selbst nutzt Flutter nur als Testläufer.
