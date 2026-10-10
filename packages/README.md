# packages

Status: geplant (M0). Noch kein Code. Ein Dart-Pub-Workspace für alle Pakete und Apps.

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

Abhängigkeitsregeln und Begründungen:
[docs/spec/15-client-architektur.md](../docs/spec/15-client-architektur.md), Abschnitt 3.

Grundregeln:

- `domain`, `api_client`, `auth`, `local_store`, `sync_engine` importieren kein Flutter.
- Kein Paket importiert aus `apps/`.
- Erzeugter Code (`api_client`, Tokens) wird eingecheckt und im Gate neu erzeugt und verglichen.
