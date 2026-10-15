# packages

Status: every package exists as a member of the Dart workspace (M4-01); `bothy_noise` is built
(M2-38), the others are skeletons that their tickets fill. One pub workspace at the repository
root (`pubspec.yaml`) resolves all packages and both apps together into one `pubspec.lock`
(docs/spec/15-client-architektur.md, section 3; R-CL-02); no Melos, the tasks run through the
root `Makefile`. No package is published (`publish_to: none`).

| Package | Kind | Purpose | Filled by |
|---|---|---|---|
| [`design_system`](design_system/README.md) | Flutter | generated tokens (`design/build/dart/`), theme with `ThemeExtension`, components from [docs/design/components.md](../docs/design/components.md), Phosphor icons, Atkinson Hyperlegible fonts | M4-04 |
| [`l10n`](l10n/README.md) | Flutter | ARB files de and en, generated localization; test against dashes and emoji in interface texts | M4-06 |
| [`domain`](domain/README.md) | Dart | entities, value objects, pure rules (for example unread state, trigger state, fractional index ordering) | M4-08 |
| [`api_client`](api_client/README.md) | Dart, generated | REST client from the OpenAPI document (openapi-generator `dart-dio`); never changed by hand | M4-02 |
| [`auth`](auth/README.md) | Dart | login, token store as a port, DPoP proofs, refresh under a lock, step-up | M4-09 |
| [`device_key`](device_key/README.md) | Flutter plugin | keys in secure storage (Secure Enclave, Android Keystore, TPM, WebCrypto), ES256 | M4-10, M4-11 |
| [`local_store`](local_store/README.md) | Dart | drift database, schema mirror, migrations, encryption (SQLite3MultipleCiphers) | M4-12 |
| [`sync_engine`](sync_engine/README.md) | Dart | gateway, catch-up, outbox, reconciliation, run documents, focus | M4-13 to M4-16 |
| [`admin_ui`](admin_ui/README.md) | Flutter | administration screens for the desktop app, the console and emergency actions on mobile | M4-25 onward |
| [`bothy_viz`](bothy_viz/README.md) | Flutter | visualizations of the statistics, fixtures and harness ([25-visualisierung.md](../docs/spec/25-visualisierung.md)) | M6-10 onward |
| [`bothy_noise`](bothy_noise/README.md) | Dart, built | Noise handshake `Noise_NK_25519_ChaChaPoly_SHA256`, transport encryption, frames and fingerprint of the terminal ([24-terminal.md](../docs/spec/24-terminal.md), section 5) | M2-38 |
| [`terminal_ui`](terminal_ui/README.md) | Flutter | terminal view with the renderer `xterm`, sessions, key pinning through ports, extra keys bar ([15-client-architektur.md](../docs/spec/15-client-architektur.md), section 23) | M4-35 |

Dependency rules and reasons: [docs/spec/15-client-architektur.md](../docs/spec/15-client-architektur.md),
section 3.1. `make arch` checks them for every member on every run of the gate
([tools/arch-check](../tools/arch-check/README.md)):

- A member depends on and imports only the members the diagram of 15 3.1 reaches from it
  (rule 8); a new edge changes 15 3.1 and `tools/arch-check/rules.yaml` in the same change.
- `domain`, `api_client`, `auth`, `local_store`, `sync_engine` and `bothy_noise` import no
  Flutter (`package:flutter/`, `dart:ui`) and do not depend on the Flutter SDK (rules 1 and 7).
- `bothy_viz` and `terminal_ui` do not depend on `sync_engine` and `local_store` (rules 6 and 7;
  the console uses both packages).
- No package imports from `apps/` (rule 2); native sources only in `device_key` (rule 4).
- Every `pubspec.yaml` under `apps/` and `packages/` is a member of `rules.yaml` and of the root
  `workspace:` list (rule 9).

Documentation in code: every public symbol has a doc comment (`public_member_api_docs`, an error
in `make lint`; generated code excluded); `make docs` generates the API pages with `dart doc`
([Dart API reference](../docs/reference/dart-api.md)). Generated code (`*.g.dart`,
`*.freezed.dart`, `*.drift.dart`, `lib/src/generated/`) is checked in, regenerated and compared
in the gate, and counts toward neither the doc and style lint nor coverage.

A new package: create it under `packages/<name>/` with `resolution: workspace`, the exact
`environment` lines of the root `pubspec.yaml`, an `analysis_options.yaml` that includes the
root file, a README, `lib/<name>.dart` and a test under `test/`; add it to the root
`workspace:` list, to `tools/arch-check/rules.yaml` and to the diagram of 15 3.1, and run
`flutter pub get` at the root.
