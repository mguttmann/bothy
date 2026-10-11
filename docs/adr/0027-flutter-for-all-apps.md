---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team apps and design
consulted: Team architecture, Team platform services
informed: all teams
---

# Flutter for all apps

## Context and Problem Statement

The product needs apps for iOS, Android, macOS and Windows as well as a web console for
administration and statistics (Baseline B5). A small team builds and maintains them; features,
density and design should be the same on all devices. At the same time, system features require
native code: Notification Service Extension (iOS, macOS), Share Extensions, push service on
Android, notification area and MSIX on Windows, keys in secure storage. Which technology carries
all user interfaces?

## Decision Drivers

- four target platforms plus a web console with one codebase for user interface and logic
- an own, distinctive design system instead of the platform's default look
- dense lists and long, live-updated content (live box) without stutter
- native code where the system requires it, with typed channels
- testability (widget, golden, integration tests) without a device farm for every run
- license and supply chain compatible with Apache-2.0

## Considered Options

1. Flutter for all user interfaces, native code only for extensions and system services
2. Native per platform (SwiftUI, Jetpack Compose, WinUI 3) plus a separate web console
3. React Native (with react-native-windows and react-native-macos)
4. Kotlin Multiplatform with Compose Multiplatform
5. Web technology in a desktop shell (Tauri or Electron) plus mobile separately

## Decision Outcome

Chosen option: "Flutter for all user interfaces", because it is the only option that serves iOS,
Android, macOS, Windows and web from one codebase with its own rendering (supported platforms
according to https://docs.flutter.dev/reference/supported-platforms, retrieved 2026-10-10
[verified]), allows the own design system without fighting platform components and permits typed
channels (Pigeon) and extensions in Swift, Kotlin and C++ for system features. The preliminary
study came to the same result; Android and the web console strengthen it.

### Consequences

- Good, because user interface, state, sync and design system are built and tested once.
- Good, because golden tests pin the design across all platforms.
- Good, because pure Dart packages (domain, sync, login) are testable without a device.
- Bad, because extensions (NSE, Share Extension, Android push service) are still built natively
  and need a small Swift or Kotlin library (`ClientKit`).
- Bad, because multi-window on desktop is still experimental in Flutter (a thread as its own
  window is therefore "later").
- Bad, because accessibility on macOS must be checked separately before shipping.
- Neutral, because the Windows build needs Windows with Visual Studio and the macOS and iOS build
  needs a Mac with Xcode; the runner strategy handles that (ADR 0039).

### Confirmation

- The gate builds `apps/client` for iOS, Android, macOS and Windows and `apps/console` for web.
- An architecture test checks that platform code lives only in `packages/device_key` and
  `apps/client/{ios,macos,android,windows}` and that pure packages do not import `package:flutter`
  (15-client-architecture.md, section 3.1).

## Pros and Cons of the Options

### Flutter

- Good, because one codebase for all targets including web.
- Good, because its own rendering keeps the design system pixel-identical.
- Good, because mature packages for the core tasks (drift, Riverpod, Sentry, record) exist.
- Bad, because some desktop features (multi-window) are young.
- Bad, because extensions stay native.

### Native per platform

- Good, because the best system integration and accessibility per platform.
- Bad, because three to four user interface codebases; that contradicts the team size.
- Bad, because the design system is created anew per platform and drifts apart.

### React Native

- Good, because a large community, close to the web.
- Bad, because Windows and macOS depend on separate projects outside the core.
- Bad, because platform components make an own, dense design harder.

### Kotlin Multiplatform with Compose Multiplatform

- Good, because the logic is shared, and Compose is native on Android.
- Bad, because the iOS and web targets of Compose Multiplatform are younger; Windows only via the
  desktop JVM.
- Bad, because a second language next to Go and Python without added value for the web target.

### Web technology in a desktop shell

- Good, because one web codebase for desktop and console.
- Bad, because mobile would need a second technology.
- Bad, because push, Share Extension and secure storage on mobile would be native anyway.

## More Information

- Platform details, packages and minimum versions: [15-client-architecture.md](../spec/15-client-architecture.md).
- Flutter 3.47 is the current stable version (https://docs.flutter.dev/release/release-notes,
  retrieved 2026-10-10 [verified]).
- Related: ADR 0028 (web only for the console), ADR 0029 (state and local DB), ADR 0030 (design
  tokens).
