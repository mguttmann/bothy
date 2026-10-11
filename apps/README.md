# apps

Status: planned (M0). No code yet.

| App | Purpose | Platforms | Technology |
|---|---|---|---|
| [client](client/README.md) | Chat with agents, live box, notifications, offline, administration on desktop, emergency actions on mobile | iOS, Android, macOS, Windows | Flutter (ADR 0027) |
| [console](console/README.md) | Administration and statistics in the browser, no chat | Web | Flutter Web (ADR 0028) |

Both apps use the packages from [`packages/`](../packages/README.md). Architecture:
[docs/spec/15-client-architecture.md](../docs/spec/15-client-architecture.md). Usage and screens:
[docs/spec/01-product.md](../docs/spec/01-product.md). Visual design: [docs/design/](../docs/design/README.md).

Shared rules:

- No permission logic in the apps; the server decides (B7).
- Platform code only in `client/{ios,macos,android,windows}` and `packages/device_key`.
- The branding of an instance comes from the server at runtime and from the build configuration
  of the instance at build time (ADR 0044); the repository holds only an example with `example.com`.
