# apps

Status: skeletons (M4-01). Both apps are members of the Dart workspace at the repository root
(`pubspec.yaml`, one `pubspec.lock`) and show an empty `MaterialApp` without text until their
tickets fill them.

| App | Purpose | Platforms | Technology | Filled by |
|---|---|---|---|---|
| [client](client/README.md) | Chat with agents, live box, notifications, offline, administration on the desktop, emergency actions on mobile | macOS, Windows, later iOS and Android | Flutter (ADR 0027) | M4-18 onward |
| [console](console/README.md) | Administration and statistics in the browser, no chat | Web | Flutter Web (ADR 0028) | M6-01 onward |

Both apps use the packages of [`packages/`](../packages/README.md). Architecture:
[docs/spec/15-client-architektur.md](../docs/spec/15-client-architektur.md). Usage and screens:
[docs/spec/01-produkt.md](../docs/spec/01-produkt.md). Design: [docs/design/](../docs/design/README.md).

Shared rules (checked by `make arch`, `tools/arch-check/rules.yaml`):

- No permission logic in the apps; the server decides (15 1 principle 1).
- Platform code only in `client/{ios,macos,android,windows}` and `packages/device_key` (15 3.1
  rule 4). The platform folders do not exist yet: M4-18 creates macOS and Windows, M5 iOS and
  Android, M6-01 the web folder of the console.
- The console never depends on `sync_engine` or `local_store`, also not through another
  package (15 3.1 rule 3).
- The branding of an instance comes from the server at run time and from the build
  configuration of the instance at build time (ADR 0044); the repository holds only an example
  with `example.com`.

Platform tests (native code, desktop integration tests) are listed per member in
`platform-tests.json` and run with `make apps-platform` on macOS and Windows (CI jobs
`apps-macos` and `apps-windows`); no member has them yet, M4-10 is the first.
