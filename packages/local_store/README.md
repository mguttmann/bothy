# local_store

drift database: schema mirror, local tables, migrations and encryption (SQLite3MultipleCiphers), without Flutter.

Status: skeleton (M4-01); M4-12 fills it. Kind: pure Dart package, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): `domain` (arrows of the diagram).
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- no import of `package:flutter/`, `dart:ui` (rule 1).
- no native source (rule 4).

## Layout

| Path | Content |
|---|---|
| `lib/local_store.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
