---
status: proposed
date: 2026-10-10
decision-makers: Owner (accounts and costs), Planning lead, Team F (operations)
consulted: Team E (apps and design), Team G (platform services, push)
informed: all teams
---

# App distribution and auto-update channels

## Context and Problem Statement

The Flutter apps run on iOS, Android, macOS and Windows (B5). Every platform has its own ways
for signing, distribution and updates. Push keys and the Firebase project belong to the
instance configuration (R-Push), and the apps are white label. Who builds and signs the apps, and
through which channels do updates arrive?

Status "proposed": the technical paths are set; accounts and costs (Apple Developer Program,
Windows signing, Play account) and the Android path are decisions of the owner.

## Decision Drivers

* Updates without manual work by people, with channels stable and beta
* Signing that Gatekeeper, SmartScreen and Play accept
* Push needs the same app identity as the signature (bundle ID, team, Firebase project)
* White label: no central infrastructure of the project in the operation of other instances
* low running costs for a small organization

## Considered Options

* Publisher model: every organization builds and signs itself with `distribution.yaml`;
  macOS Developer ID with notarization and Sparkle 2; Windows MSIX with `.appinstaller`; iOS
  TestFlight; Android Play Internal Testing
* Central builds of the project with its own push forwarding for all instances
* macOS through TestFlight or the Mac App Store instead of Developer ID
* Windows through Velopack or WinSparkle instead of MSIX

## Decision Outcome

Chosen option (proposed): "Publisher model with Sparkle, MSIX and TestFlight", because it works
without central push forwarding and on every platform uses the path with update channels that
the baseline names (R-Desktop-Verteilung).

1. Publisher model and `distribution.yaml` ([18-release.md](../spec/18-release.md) 8).
2. macOS: Developer ID, notarization, Sparkle 2 with EdDSA, channel `beta` in the appcast, phased
   rollout (https://sparkle-project.org/documentation/publishing/, retrieved 2026-10-10).
3. Windows: MSIX with `stable.appinstaller` and `beta.appinstaller`; signing through Azure
   Artifact Signing (EU organization possible) or an OV certificate with HSM; plan for SmartScreen
   warnings at the start (https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation,
   retrieved 2026-10-10). MSIX gives the app the package identity for local notifications.
4. iOS: TestFlight groups beta and stable; App Store later.
5. Android: Play Internal Testing (recommendation); Firebase App Distribution as an alternative.

### Consequences

* Good, because every instance runs push with its own keys and nobody sees content or tokens
  of another instance.
* Good, because every platform has a channel beta with the same build for stable.
* Bad, because every organization needs its own accounts with Apple, Microsoft or a CA, and
  Google.
* Bad, because an app build chain with macOS and Windows runners is needed.
* Neutral, because a central distribution remains possible later as an additional publisher.

### Confirmation

* The release chain checks per platform: signature valid (`codesign --verify`, `spctl --assess`,
  `signtool verify`), notarization stapled, appcast entry with an EdDSA signature.
* Test of the update chain per platform with a test publisher (a beta build replaces an older
  build).

## Pros and Cons of the Options

### Publisher model, Sparkle, MSIX, TestFlight

* Good, because push identity and signing are in one hand.
* Good, because Sparkle and MSIX bring channels and signed updates.
* Bad, because the effort per organization rises.

### Central builds with push forwarding

* Good, because organizations need no accounts.
* Bad, because the project would have to run a service that sees the push data of all instances.
* Bad, because that works against white label and single tenant (B13).

### macOS through TestFlight or the Mac App Store

* Good, because Apple takes over the distribution.
* Bad, because TestFlight for macOS requires an App Sandbox [assumed], and a second edition
  would result from that.

### Windows through Velopack or WinSparkle

* Good, because Velopack can do deltas and several platforms.
* Bad, because Velopack has no Dart SDK (https://github.com/velopack/velopack) and the
  Flutter package `auto_updater` uses WinSparkle with DSA on Windows and has had no new version for
  about two years (https://pub.dev/packages/auto_updater); both retrieved 2026-10-10
  [verified].
* Bad, because without MSIX no package identity for notifications is available.

## More Information

* Owner decisions: Apple Developer Program; path of the Windows signing (Artifact Signing or
  OV with HSM); Android path; time of the App Store.
* Push details: chapter 13 (Team G).
