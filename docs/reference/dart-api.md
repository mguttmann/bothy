# Dart API reference

The API documentation of the Dart packages is generated from their doc comments with
`dart doc` on every build of the documentation site (`make docs`,
`tools/docs/dart_api_docs.py`); it is never tracked, so it cannot drift from the code. A public
symbol without a doc comment fails `make lint` (`public_member_api_docs`), and a warning of
`dart doc` fails `make docs`. The apps `apps/client` and `apps/console` are not libraries and
have no API pages. Layout and dependency rules of the workspace:
[15 Client architecture](../spec/15-client-architecture.md), sections 3 and 3.1.

| Package | Purpose |
|---|---|
| [design_system](../api/dart/design_system/index.html) | Design tokens (generated), theme, components, icons and fonts. |
| [l10n](../api/dart/l10n/index.html) | ARB files in German and English and the generated localization. |
| [domain](../api/dart/domain/index.html) | Entities, value objects and pure rules, without Flutter. |
| [api_client](../api/dart/api_client/index.html) | REST client generated from the OpenAPI document, never changed by hand. |
| [auth](../api/dart/auth/index.html) | Login, tokens, DPoP proofs, step-up and refresh, without Flutter. |
| [device_key](../api/dart/device_key/index.html) | Keys in the secure storage of each platform (Flutter plugin, Pigeon). |
| [local_store](../api/dart/local_store/index.html) | drift database, schema mirror, migrations and encryption, without Flutter. |
| [sync_engine](../api/dart/sync_engine/index.html) | Gateway, catch-up, outbox, reconciliation and run documents, without Flutter. |
| [admin_ui](../api/dart/admin_ui/index.html) | Administration screens for the desktop app, the console and emergency actions on mobile. |
| [bothy_viz](../api/dart/bothy_viz/index.html) | Visualizations of the statistics: views, primitives, fixtures and harness. |
| [bothy_noise](../api/dart/bothy_noise/index.html) | Noise handshake NK, transport encryption and frames of the terminal, without Flutter. |
| [terminal_ui](../api/dart/terminal_ui/index.html) | Terminal view: renderer, sessions, key pinning and the extra keys bar. |
