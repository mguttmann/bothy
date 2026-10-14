# design_system

Design tokens (generated from `design/`), the theme with `ThemeExtension`, the components of docs/design/components.md, the Phosphor icons and the Atkinson Hyperlegible fonts.

Status: skeleton (M4-01); M4-04 fills it. Kind: Flutter package, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): `l10n`, `domain` (arrows of the diagram).
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- no native source (rule 4).

## Layout

| Path | Content |
|---|---|
| `lib/design_system.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
