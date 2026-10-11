# packages

Status: planned (M0). No code yet. One Dart pub workspace for all packages and apps.

| Package | Kind | Purpose | Platforms |
|---|---|---|---|
| `design_system` | Flutter | generated tokens (`design/build/dart/`), theme with `ThemeExtension`, components from [docs/design/components.md](../docs/design/components.md), Phosphor icons, fonts Atkinson Hyperlegible | all |
| `l10n` | Flutter | ARB files de and en, generated localization; test against en and em dashes and emoji in UI texts | all |
| `domain` | Dart | entities, value objects, pure rules (for example unread, trigger state, fractional index sorting) | all |
| `api_client` | Dart, generated | REST client from the OpenAPI document (openapi-generator `dart-dio`); never change by hand | all |
| `auth` | Dart | login, token storage as a port, DPoP proofs, refresh with a lock, step-up | all |
| `device_key` | Flutter plugin | keys in secure storage (Secure Enclave, Android Keystore, TPM, WebCrypto), ES256 | iOS, macOS, Android, Windows, Web |
| `local_store` | Dart | drift database, schema mirror, migrations, encryption (SQLite3MultipleCiphers) | iOS, macOS, Android, Windows |
| `sync_engine` | Dart | gateway, catch-up, outbox, reconciliation, run documents, focus | iOS, macOS, Android, Windows |
| `admin_ui` | Flutter | administration screens for the desktop app, the console and emergency actions on mobile | all |
| `bothy_viz` | Flutter | visualizations of the statistics (live topology, backend view, delivery, executive page, timeline, TV mode), fixtures and harness ([25-visualization.md](../docs/spec/25-visualization.md)) | all |
| `bothy_noise` | Dart | Noise handshake `Noise_NK_25519_ChaChaPoly_SHA256`, transport encryption and frames of the terminal, checked against the official test vectors ([24-terminal.md](../docs/spec/24-terminal.md), section 5) | all |
| `terminal_ui` | Flutter | terminal view with renderer `xterm`, sessions, key pinning through ports, extra keys bar ([15-client-architecture.md](../docs/spec/15-client-architecture.md), section 23) | all |

Dependency rules and their rationale:
[docs/spec/15-client-architecture.md](../docs/spec/15-client-architecture.md), section 3.

Basic rules:

- `domain`, `api_client`, `auth`, `local_store`, `sync_engine`, `bothy_noise` do not import Flutter.
- `bothy_viz` and `terminal_ui` do not depend on `sync_engine` and `local_store` (the console uses both).
- No package imports from `apps/`.
- Generated code (`api_client`, tokens) is checked in, and the gate regenerates and compares it.
