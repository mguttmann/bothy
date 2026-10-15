# bothy_viz

Visualizations of the statistics (docs/spec/25-visualisierung.md): views, primitives, fixtures and harness, with the port `LiveFrameSource` for the live stream.

Status: skeleton (M4-01); M6-10 onward fills it. Kind: Flutter package, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): `design_system`, `api_client`, `domain` (arrows of the diagram), and through them `l10n`.
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- never depends on `sync_engine` or `local_store`, directly or through another member (rule 6).
- no native source (rule 4).

## Layout

| Path | Content |
|---|---|
| `lib/bothy_viz.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
