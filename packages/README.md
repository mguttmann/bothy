# packages

Status: geplant (M0); als erstes Paket gibt es `bothy_noise` (M2-38). Ein Dart-Pub-Workspace für alle Pakete und Apps folgt mit M4-01; bis dahin trägt `bothy_noise` seine eigene `pubspec.lock`.

| Paket | Art | Zweck | Plattformen |
|---|---|---|---|
| `design_system` | Flutter | erzeugte Tokens (`design/build/dart/`), Theme mit `ThemeExtension`, Komponenten aus [docs/design/components.md](../docs/design/components.md), Phosphor-Icons, Schriften Atkinson Hyperlegible | alle |
| `l10n` | Flutter | ARB-Dateien de und en, erzeugte Lokalisierung; Test gegen Gedankenstriche und Emoji in UI-Texten | alle |
| `domain` | Dart | Entitäten, Wertobjekte, reine Regeln (z. B. Ungelesen, Auslöser-Status, Bruchindex-Sortierung) | alle |
| `api_client` | Dart, erzeugt | REST-Client aus dem OpenAPI-Dokument (openapi-generator `dart-dio`); nie von Hand ändern | alle |
| `auth` | Dart | Anmeldung, Token-Speicher als Port, DPoP-Beweise, Refresh mit Sperre, Step-up | alle |
| `device_key` | Flutter-Plugin | Schlüssel im sicheren Speicher (Secure Enclave, Android Keystore, TPM, WebCrypto), ES256 | iOS, macOS, Android, Windows, Web |
| `local_store` | Dart | drift-Datenbank, Schema-Spiegel, Migrationen, Verschlüsselung (SQLite3MultipleCiphers) | iOS, macOS, Android, Windows |
| `sync_engine` | Dart | Gateway, Nachholen, Ausgangsfach, Abgleich, Lauf-Dokumente, Fokus | iOS, macOS, Android, Windows |
| `admin_ui` | Flutter | Verwaltungs-Screens für Desktop-App, Konsole und Notfall-Aktionen mobil | alle |
| `bothy_viz` | Flutter | Visualisierungen der Statistik (Live-Topologie, Backend-Ansicht, Zustellung, Executive-Seite, Timeline, TV-Modus), Fixtures und Harness ([25-visualisierung.md](../docs/spec/25-visualisierung.md)) | alle |
| `bothy_noise` | Dart, gebaut | Noise-Handshake `Noise_NK_25519_ChaChaPoly_SHA256`, Transport-Verschlüsselung, Rahmen und Fingerabdruck des Terminals, geprüft gegen die öffentlichen Testvektoren und die Aufzeichnungen des Host-Dienstes ([24-terminal.md](../docs/spec/24-terminal.md), Abschnitt 5; [README](bothy_noise/README.md)) | alle |
| `terminal_ui` | Flutter | Terminal-Ansicht mit Renderer `xterm`, Sitzungen, Schlüssel-Pinning über Ports, Sondertasten-Leiste ([15-client-architektur.md](../docs/spec/15-client-architektur.md), Abschnitt 23) | alle |

Abhängigkeitsregeln und Begründungen:
[docs/spec/15-client-architektur.md](../docs/spec/15-client-architektur.md), Abschnitt 3.

Grundregeln:

- `domain`, `api_client`, `auth`, `local_store`, `sync_engine`, `bothy_noise` importieren kein Flutter.
- `bothy_viz` und `terminal_ui` hängen nicht von `sync_engine` und `local_store` ab (die Konsole nutzt beide).
- Kein Paket importiert aus `apps/`.
- Erzeugter Code (`api_client`, Tokens) wird eingecheckt und im Gate neu erzeugt und verglichen.
