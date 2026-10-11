# apps/client

Status: planned (M0). No code yet.

Flutter app for iOS, Android, macOS and Windows. One codebase for interface and logic; native
code only where the system requires it.

## Purpose

- Spaces, channels, forums and threads; agent sessions with live box, follow-ups, components and
  slash commands ([01-product.md](../../docs/spec/01-product.md)).
- Reading and writing offline with an outbox; catch-up according to [04-sync-protocol.md](../../docs/spec/04-sync-protocol.md).
- Push without content, the app fetches the text itself; sharing from other apps; voice messages.
- Administration on desktop (from `packages/admin_ui`), emergency actions on the phone.

## Structure (planned)

| Path | Content |
|---|---|
| `lib/` | App shell, router (`go_router`), layouts per form factor (desktop four columns, mobile stack), chat features |
| `ios/` | Runner, Notification Service Extension, Share Extension, `ClientKit` (Swift package, shared with macOS) |
| `macos/` | Runner, Notification Service Extension, Share Extension, Sparkle 2 |
| `android/` | Activity, own FCM service with WorkManager, share activity |
| `windows/` | Runner, notification area, MSIX manifest with Share Target, toast activation |
| `integration_test/` | Tests against the fake server; device E2E with `patrol` |

## Platforms

| Target | Minimum version | Distribution |
|---|---|---|
| iOS | 15 | TestFlight, App Store |
| Android | API 26 | Play Internal Testing or Firebase App Distribution |
| macOS | 12 | Developer ID, notarization, Sparkle 2 (stable, beta) |
| Windows | 10 version 2004 | MSIX with `.appinstaller` |

Reasons and evidence: [15-client-architecture.md](../../docs/spec/15-client-architecture.md), section 2.
Release and signing: [18-release.md](../../docs/spec/18-release.md).

## Building (planned)

One make target per platform from the repository root (quality team). The build configuration of
the instance (`branding.yaml`, ADR 0044) supplies the app name, IDs, push topics, update feeds and
signing identities; without it, a neutral build with placeholders under `example.com` is produced.
