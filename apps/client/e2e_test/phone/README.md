<!-- SPDX-License-Identifier: Apache-2.0 -->

# Mobile E2E suites

The `patrol` suites of brief M5-11 (Design note 2; docs/spec/15-client-architecture.md 20, row
"E2E on devices"): the phone app on an iOS simulator or an Android emulator against the real
server of the Compose stack of `infra/compose/e2e.sh`. They live outside `integration_test/`, so
the platform jobs of `platform-tests.json`, which run without a server, never pick them up.

| File | What it proves |
|---|---|
| `login_e2e_test.dart` | E1 with the stack's address, E2 with Alex's password and a TOTP code of a step after the world's `last_step`, E4 waiting until Alex's other device approves it (with the code E4 shows and the approval's step-up), then the phone shell, live |
| `push_e2e_test.dart` | the notification permission dialog accepted through patrol's native automation; Sam's message in a post Alex follows becomes a push to the fake provider, which the runner's relay hands to the device; the notification shows the text the native part fetched from the server; tapping it opens the thread at the message |
| `share_e2e_test.dart` | the sample image from Photos (iOS, through the Share Extension) or from the file app (Android, through the share chooser and the app's sheet) lands in general for Sam |
| `offline_e2e_test.dart` | without its connections to the server the app keeps Alex's message "Sending" under the offline banner; after the reconnect Sam has it exactly once |
| `support/phone_run.dart` | the shared set-up: the world from the defines `E2E_CONFIG` and `E2E_CA`, the network, Alex's other device and Sam as API actors, the app with the ports of a phone build, the clean-up |

## Running them

`make e2e-mobile` (`tools/e2e/mobile_e2e.py`) starts the stack with the mobile overlay, boots the
simulator or emulator, starts the push relay and runs `tool/mobile_e2e_suite.sh ios|android
[suite...]` inside `infra/compose/e2e.sh`. The suite script prepares the device (iOS: the test CA
as a trusted root, the sample photo in Photos, the simulator groups of the build; Android: `adb
reverse` of the server's port, the sample image in the downloads, the Gradle properties
`e2ePush` and `e2eCa` of the debug build) and runs one `patrol test` per suite file. A
suite never starts a stack, a device or the relay itself, and without the defines it fails at once
(P-09: never skipped). The push suite needs Android 13 (API 33) or later: only there the app asks
for the notification permission.

## How the suites work

- **One process per suite.** Each suite is its own install and app process against one stack. An
  enrollment code works once (08 3.2), so Alex's other device ("Alex desk") is one device per stack
  with keys and a device ID derived from the instance ID: the first suite of a stack redeems the
  world's code, every later one signs the same device in again with Alex's password and TOTP, which
  the server accepts for a known active device (08 5.2). A TOTP code is for a step after the step
  current when the process started, because the server accepts only steps above the last one it
  accepted (08 2.5). Sam gets a new device per suite with a code Alex issues.
- **The network of the app.** Every connection of the app to the server goes through the tunnel
  of `../support/e2e_network.dart` on the device's loopback (`E2eNetwork.installForServer`); every
  other connection, patrol's own to its automation server among them, connects directly. The
  offline suite cuts the tunnel: the app's connections end and new ones are refused while the
  server, the actors and patrol stay connected.
- **The ports of a phone build.** The app runs with Flutter's lifecycle (the background closes
  the gateway connection, so the server pushes) and the network status of a release build. The
  push suite adds the platform's push and keeps the `dpop` key in the platform's keystore, where
  the Android preview worker and the iOS Notification Service Extension find it; the iOS share
  suite does the same for the Share Extension. On the iOS simulator an unsigned build gets no APNs
  token, so the push suite registers a random one; `xcrun simctl push` needs none.
- **Waits.** Every wait ends at its condition and fails only after the repository's patience. In
  the background, where the app draws no frames, the waits poll without pumping. The push suite has
  Sam write again when no notification came within a while, because the server may still route a
  message right after the app went to the background to the closed connection (13 4.6).
- **Clean-up.** Every suite removes the app and its data, its push registration and platform key,
  and revokes the devices it created except Alex's other device, which later suites of the stack
  use; the stack itself is removed by `infra/compose/e2e.sh`. The suites print no message
  content, code or token; patrol's native automation logs the notification texts it reads (on
  iOS), which are only the made-up texts of the push suite.
