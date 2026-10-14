# apps/client

Status: skeleton (M4-01): `lib/main.dart` starts an empty `MaterialApp` without text, and
`test/app_test.dart` pumps it. M4-18 builds the app shell on it.

Flutter app for macOS and Windows, later iOS and Android. One code base for interface and
logic; native code only where the system requires it.

## Purpose

- Spaces, channels, forums and threads; agent sessions with live box, follow-ups, components
  and slash commands ([01-produkt.md](../../docs/spec/01-produkt.md)).
- Offline reading and writing with an outbox; catch-up according to
  [04-sync-protokoll.md](../../docs/spec/04-sync-protokoll.md).
- Push without content, the app fetches the text itself; sharing from other apps; voice
  messages.
- Administration on the desktop (from `packages/admin_ui`), emergency actions on the phone.

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`):
`admin_ui`, `sync_engine`, `design_system`, `device_key`, `terminal_ui` and every member they
reach; `make arch` checks the manifest and every import.

## Layout

| Path | Content | Ticket |
|---|---|---|
| `lib/` | app shell, router (`go_router`), layouts per form factor, chat features | M4-18 onward |
| `test/` | widget tests, run by `make test` | M4-01 onward |
| `macos/`, `windows/` | runners, notification area, MSIX manifest, Share Target, Sparkle 2 | M4-18 (`flutter create`) |
| `ios/`, `android/` | runners, Notification Service Extension, Share Extension, push service | M5 |
| `integration_test/` | tests against the fake server; device E2E with `patrol` | M4 and M5 |

## Platforms

| Target | Minimum version | Distribution |
|---|---|---|
| macOS | 12 | Developer ID, notarization, Sparkle 2 (stable, beta) |
| Windows | 10 version 2004 | MSIX with `.appinstaller` |
| iOS | 15 | TestFlight, App Store |
| Android | API 26 | Play Internal Testing or Firebase App Distribution |

Reasons: [15-client-architektur.md](../../docs/spec/15-client-architektur.md), section 2.
Release and signing: [18-release.md](../../docs/spec/18-release.md).

## Build and test

From the repository root: `make lint test coverage arch COMPONENTS=apps`. Platform tests,
once the app has them, are listed in `platform-tests.json` and run with `make apps-platform`
on macOS or Windows. The build configuration of the instance (`branding.yaml`, ADR 0044)
supplies app name, IDs, push topics, update feeds and signing identities; without it a neutral
build with placeholders under `example.com` is produced.
