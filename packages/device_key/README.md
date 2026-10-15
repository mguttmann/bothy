# device_key

Device keys in the secure storage of each platform (Secure Enclave, Android Keystore, TPM, WebCrypto), ES256; a federated Flutter plugin with a Pigeon interface.

Status: skeleton (M4-01); M4-10 (macOS, iOS) and M4-11 (Windows) fills it. Kind: Flutter plugin, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): none: no arrow of the diagram leaves this package.
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- native sources only in `ios/`, `macos/`, `android/`, `windows/`, `linux/`, `web/` (rule 4).

## Layout

| Path | Content |
|---|---|
| `lib/device_key.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
