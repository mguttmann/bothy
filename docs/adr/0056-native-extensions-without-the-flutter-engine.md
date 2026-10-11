---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team E (apps and design), Team G (platform services), Team C (identity)
informed: Team F (operations)
---

# Native extensions without the Flutter engine (NSE, share, Android push)

## Context and Problem Statement

The apps are Flutter (ADR 0027). Three entry points, however, run outside the app interface: the
Notification Service Extension (NSE) on iOS and macOS, which fetches the text of a push; the Share
Extension on iOS and macOS, through which another app shares content; push reception on Android.
All three have tight limits: the NSE has about 30 seconds, `onMessageReceived` under FCM only
a few seconds, and a Share Extension cannot open its own app
([15-client-architecture.md](../spec/15-client-architecture.md), sections 10 and 12, with evidence).
Do these entry points run with a Flutter engine or natively?

## Decision Drivers

- Time and memory limits of the extensions and background services.
- Push carries only IDs; the device fetches the text itself (ADR 0031).
- Sharing must not depend on the person opening the app later.
- One codebase, native only where the system requires it (15, section 1, principle 4).
- No second process may rotate the app's refresh family (reuse detection,
  [08-identity.md](../spec/08-identity.md), section 7.4).

## Considered Options

- Native extensions in Swift and Kotlin with a shared Swift library `ClientKit`;
  no Flutter engine in the extension and the push service
- Flutter engine in the background (Dart handler of `firebase_messaging`, Flutter in the extension)
- No extensions: push only with a placeholder, sharing only through the app
- Push with content, so that nothing has to be fetched afterwards
- Share Extension only stores, the app sends later

## Decision Outcome

Chosen option: "Native extensions with `ClientKit`, without the Flutter engine", because only they
reliably finish within the limits of the systems. Decision R-CL-06 in
[15-client-architecture.md](../spec/15-client-architecture.md).

1. **NSE (iOS, macOS).** Swift without the Flutter engine. It fetches the preview through
   `GET /api/v1/push/preview` with the preview token of the push registration and a DPoP proof
   of the device key without user confirmation
   ([13-push-notifications.md](../spec/13-push-notifications.md), sections 5.1 and 6.4).
   It never renews tokens. If the fetch fails within `client.push.fetch_timeout`, the
   placeholder stays. With `notify.preview_mode = none` it fetches nothing.
2. **Share Extension (iOS, macOS).** Swift and SwiftUI with its own sheet for the destination. It
   sends by itself through `ClientKit` (upload, then message with its own `client_msg_id`) and uses
   its own child session with its own refresh family (08, section 7.4). Without network it puts the
   entry into `share_inbox`; the app takes it over with the same `client_msg_id`, and on a
   duplicate the server returns the original.
3. **Android push.** A custom `FirebaseMessagingService` in Kotlin immediately shows a notification
   with a placeholder and starts an expedited WorkManager job that fetches the preview and
   replaces the same notification. It passes the FCM token to Dart through Pigeon (`push.register`).
4. **Android sharing and Windows** need no extension: the share target on Android is an
   Activity of the app with a Flutter sheet, the Windows app keeps the connection in the
   notification area (15, sections 10.3 and 12).
5. `ClientKit` (Swift Package in `apps/client`) contains shared keychain access, DPoP,
   preview fetch, upload and sending. The plugin `device_key` uses the same implementation
   (15, section 9).

### Consequences

- Good, because no engine starts in the background; the scarce time belongs to the network fetch.
- Good, because shared content is sent immediately and is not lost without network.
- Good, because the NSE touches no refresh family and the Share Extension has its own; a
  simultaneous renewal does not trigger theft detection.
- Bad, because logic is duplicated: DPoP proofs, preview fetch, upload and sending exist in
  Dart, Swift and, for the fetch, also in Kotlin. Mitigation: small scope of the native parts and
  contract tests against the same sample responses as the Dart client.
- Bad, because Swift and Kotlin are further toolchains in the repo and in the release chain
  (Xcode builds need macOS).
- Bad, because the memory limit of the extensions is not verified; `ClientKit` therefore streams
  large files from disk, and device tests measure the rest.
- Neutral, because an NSE reaches a server that is only reachable in a private network only with an
  active VPN on the device; otherwise the placeholder stays (15, section 10.1).

### Confirmation

- Build ticket M5-04 in [PLAN.md](../PLAN.md): XCTest of `ClientKit`; device test "text appears
  when the server is reachable, otherwise placeholder"; with `notify.preview_mode = none` nothing
  is fetched.
- Test ID-T16 from [08-identity.md](../spec/08-identity.md), section 15, with the real Share
  Extension: a simultaneous renewal by app and extension does not trigger reuse detection.
- Build ticket M5-05: instrumented Android test with a simulated FCM payload; placeholder without
  network.
- Build ticket M5-06: device test with `patrol`, an image from another app lands in the chosen
  thread.
- Build check in M5-04: the targets of the extensions link no Flutter framework.

## Pros and Cons of the Options

### Native extensions with `ClientKit`

- Good, because they keep within the system limits and work without a detour through Dart.
- Bad, because two more languages are maintained.

### Flutter engine in the background

- Good, because the logic from the Dart packages would be reused.
- Bad, because starting an engine costs a large part of the scarce time and two services
  (Flutter plugin and own service) would receive the same message (15, section 10.2).

### No extensions

- Good, because nothing native is needed.
- Bad, because notifications show only a neutral placeholder and sharing from other apps is
  missing; the feature list requires both ([01-product.md](../spec/01-product.md)).

### Push with content

- Good, because nothing has to be fetched afterwards.
- Bad, because content would go through Apple and Google; ADR 0031 forbids that (R-Push).

### Share Extension only stores

- Good, because the extension stays small.
- Bad, because sending would depend on the person opening the app later; the extension
  cannot open the app itself (15, section 12). Remains as a fallback without network.

## More Information

- Evidence on time limits, `NSExtensionContext.open` and FCM is in
  [15-client-architecture.md](../spec/15-client-architecture.md), sections 10 and 12, and in
  [13-push-notifications.md](../spec/13-push-notifications.md), section 18.
- Related: ADR 0020 (device binding, two device keys per ruling L10), ADR 0027 (Flutter),
  ADR 0029 (client state and outbox), ADR 0031 (push without content).
