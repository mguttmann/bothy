# apps

Status: built (M4 to M7). Both apps are members of the Dart workspace at the repository root
(`pubspec.yaml`, one `pubspec.lock`). The README of each app says which tickets built it and
what it holds; [distribution](distribution/README.md) holds the publisher values and the
templates of the update files.

| App | Purpose | Platforms | Technology | Built by |
|---|---|---|---|---|
| [client](client/README.md) | Chat with agents, live box, notifications, offline, administration on the desktop, emergency actions on mobile | macOS, Windows, iOS, Android | Flutter (ADR 0027) | M4 (desktop), M5 (phones) |
| [console](console/README.md) | Administration, statistics and terminal in the browser, no chat | Web | Flutter Web (ADR 0028) | M6 |

Both apps use the packages of [`packages/`](../packages/README.md). Architecture:
[docs/spec/15-client-architecture.md](../docs/spec/15-client-architecture.md). Usage and screens:
[docs/spec/01-product.md](../docs/spec/01-product.md). Design: [docs/design/](../docs/design/README.md).

Shared rules (checked by `make arch`, `tools/arch-check/rules.yaml`):

- No permission logic in the apps; the server decides (15 1 principle 1).
- Platform code only in `client/{ios,macos,android,windows}` and `packages/device_key` (15 3.1
  rule 4); the console has its web folder `console/web`.
- The console never depends on `sync_engine` or `local_store`, also not through another
  package (15 3.1 rule 3).
- The branding of an instance comes from the server at run time and from the build
  configuration of the instance at build time (ADR 0044); the repository holds only an example
  with `example.com`.

Platform tests (native code, integration tests on a device or a desktop) are listed per member
in `platform-tests.json` and run with `make apps-platform` on the host system and an attached
Android device (CI jobs `apps-macos`, `apps-ios`, `apps-windows`, `apps-android-26` and
`apps-android-36`). The members with platform tests are `client`, `packages/device_key` and
`packages/bothy_viz/harness`.
