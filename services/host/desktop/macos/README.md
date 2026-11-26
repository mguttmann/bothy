# Bothy Computer Helper for macOS

The Bothy Computer Helper is the only process with screen capture and input rights on a Mac
that a Bothy agent may control (docs/spec/27-computer-use.md section 9.1). It runs in the
desktop session of the person who owns the account, as an accessory app with a menu bar item,
and talks only to the local host service `bothy-host` over the channel of 27 section 9.4. It is
built and signed on the owner's own Mac and never published (D-CU-01-29).

Names: the app bundle is `bothy-computer-helper.app` (bundle ID `com.example.bothy-computer-helper`,
executable `bothy-computer-helper`), installed to
`/Library/Application Support/bothy-host/bin/bothy-computer-helper.app`; its display name and
menu bar title are "Computer Helper", because code and configuration carry the working name only
in its prefix forms (docs/spec/02-architecture.md "6. Code layout").

What it does:

- **Enablement.** Off after installation. Only a click on "Allow computer control" in the menu
  bar switches it on; every switch-on raises the generation by one. Switch and generation live
  in `~/Library/Application Support/bothy-computer-helper/state.json` (mode `0600`, folder
  `0700`, D-CU-02-4). The file is read at start only; changing it is no order. The host service
  can switch off (`disable`), never on.
- **Session.** `show` draws a border in `#FF6A00` around every screen (D-CU-02-9) with a stop
  button, sets the menu text "Agent <name> controls this computer, Esc to stop", starts the Esc
  tap and keeps the display awake. `hide`, a broken channel, the helper's start, a real Esc, the
  stop button and a local switch-off release every key and button and undo all three before
  anything is reported (D-CU-02-6).
- **Esc.** A real Esc ends the session with `local_escape` and is swallowed; an Esc the helper
  sends for the agent passes to the app and stops nothing (27 8.4 item 4).
- **Capture.** The screen with the frontmost window, else the main screen, in native resolution
  scaled into the request's `max_edge` (from `computer.image_max_edge`), JPEG or PNG; `region`
  for `zoom`. The helper's own windows, the indicator included, are excluded from every image.
  Images stay in memory and are never written to disk.
- **Input.** Clicks with modifiers, double and triple click, move, scroll, Unicode text
  independent of the keyboard layout, and every key or combination of D-CU-02-7. The helper
  refuses no key and no app.
- **Stopping a running action.** The Esc tap, the stop button, the lock monitor and the channel
  reader (`hide`, `disable`, a broken channel) set a shared flag (`ActionInterrupt`) on their own
  threads; an action runs in units (a press and release per `repeat`, a character, a line of
  scroll, a click, a move) and checks it before every unit, releases every key and button and
  ends (27 9.6). One held Esc stops once; its auto-repeat and key up are swallowed; the Esc
  state (`EscFilter`) starts fresh with every show. `StopVectorTests` runs the shared vectors
  `services/host/testdata/computer/stop-vectors.json`.
  A stop of the person that is still pending when the next `show` arrives refuses that session
  with a `stop` for it; a pending `hide`, `disable` or break is cleared by the `show`.
  Every start of the Esc tap is a new generation (`TapGeneration`); a late callback or
  teardown of an older tap never acts on a newer one.
  `repeat` above the default of `computer.key_repeat_max`, `amount` above the default of
  `computer.scroll_amount_max` and a `text` longer than one frame of
  `host_service.computer_frame_max_bytes` are `invalid_action` before anything is injected.
- **Lock.** Lock, screen saver, user switch and display sleep count as locked: the helper
  reports `state` with `locked`, and every `capture` and `do` answers `screen_locked` and
  injects nothing.
- **Permissions.** Screen Recording and Accessibility are only checked
  (`CGPreflightScreenCaptureAccess`, `AXIsProcessTrusted`) and reported in `hello` and `state`;
  the menu opens the matching pane of the system settings. The helper never asks for or grants
  itself a permission.

## Layout

| Path | Content |
|---|---|
| `Package.swift` | Swift package, tools version 5.9, macOS 14 (D-CU-02-1) |
| `Sources/HelperCore/` | The logic, Foundation only: channel codec (`ChannelCodec.swift`), JSON model (`JSONValue.swift`, `Model.swift`), key names (`KeyNames.swift`), the platform interfaces (`Interfaces.swift`), the controller (`HelperController.swift`, `ActionInterrupt.swift`), the peer rule (`PeerRule.swift`), the connection loop (`ChannelRunner.swift`), the state file (`FileEnablementStore.swift`) and the frame limit from the host configuration (`HostConfig.swift`) |
| `Sources/HelperMac/` | The macOS implementations of the interfaces and the app wiring (`HelperApp.swift`) |
| `Sources/bothy-computer-helper/` | The executable; `--version` prints the version and exits |
| `Tests/HelperCoreTests/` | XCTest unit tests of `HelperCore` against fakes and the shared channel vectors |
| `Resources/Info.plist` | Bundle metadata and usage texts; `@VERSION@` is filled in by `bundle.sh` |
| `Resources/helper.entitlements` | Hardened runtime without sandbox; no entitlement is needed |
| `bundle.sh` | Builds the release and assembles the app bundle |

## Interfaces

Every platform call sits behind one of these protocols of `HelperCore` (brief item 13), so the
controller runs against fakes in the unit tests:

| Protocol | macOS implementation | Platform API |
|---|---|---|
| `Capturer` | `MacCapturer` | ScreenCaptureKit `SCScreenshotManager` with an `SCContentFilter` that excludes the helper's windows; ImageIO for JPEG and PNG |
| `Injector` | `MacInjector` | `CGEvent` from a private event source; every event carries a marker in `eventSourceUserData` |
| `AppInspector` | `MacAppInspector` | `NSWorkspace` frontmost app; team ID from the running code's signature |
| `LockMonitor` | `MacLockMonitor` | `CGSessionCopyCurrentDictionary`, screen lock and screen saver notifications, `NSWorkspace` session and display sleep notifications |
| `Waker` | `MacWaker` | `IOPMAssertionCreateWithName` with `kIOPMAssertionTypePreventUserIdleDisplaySleep` |
| `EscTap` | `MacEscTap` | an active `CGEventTap` on the session; an Esc with the helper's marker or source state passes |
| `Indicator` | `MacIndicator` | borderless click-through border windows, a stop button panel and an `NSStatusItem` menu |
| `EnablementStore` | `FileEnablementStore` (in `HelperCore`) | the state file of D-CU-02-4 |
| `HostChannel` | `UnixSocketChannel` | Unix socket client to `/Library/Application Support/bothy-host/computer/computer.sock` |

The channel: frames of a 4-byte big-endian length, a type byte (`0x01` JSON object, `0x02` image
bytes) and the content; the longest frame is `host_service.computer_frame_max_bytes`, read from
the host service's `config.yaml` when readable, else the default of 27 section 18 (D-CU-02-8).
Before `hello` the helper checks the host side (D-CU-02-5, rule in `PeerRule.swift`): the peer
(`getpeereid`) is the service user `bothy-host` or root and never the helper's own user; every
folder from `/` to `bothy-host/` is owned by root, the endpoint folder `computer/` by root or the
service user, none writable by group or others; and the socket (read with `lstat`) is owned by
the peer. A development build accepts another endpoint in the environment variable
`BOTHY_COMPUTER_HELPER_CHANNEL` and then skips that check; a verified build ignores the variable.

## Build and test

```sh
swift build
swift test
./bundle.sh <out_dir> <version>
```

`swift test` needs XCTest, which comes with Xcode; the Command Line Tools alone do not ship it.
The tests read `services/host/testdata/computer/channel-vectors.json`, so run them from a full
checkout. They use fakes only: no test captures the screen, sends input or opens a window. The
native behaviour is tested by the CI jobs of `host-platform.yml` on hosted runners.

`bundle.sh` writes `<out_dir>/bothy-computer-helper.app` with the executable in
`Contents/MacOS/bothy-computer-helper` and the version in `Info.plist`, and signs it ad hoc with
the hardened runtime. Such a bundle reports `signature: development`.

## Signing and packages

Signing follows D-CU-01-29 and D-CU-02-10: the helper is signed on the owner's Mac, with the
owner's Developer ID Application identity and the hardened runtime, without notarization, and
no signing secret goes anywhere else. The make targets of `services/host/Makefile` drive it:

- `make -C services/host helper-macos` builds the app bundle (a development build);
- `make -C services/host helper-sign-macos` signs it with the identity named by `SIGN_IDENTITY`,
  verifies it and compares its designated requirement with the previous signed build, because a
  changed requirement silently voids the Screen Recording and Accessibility grants;
- `make -C services/host helper-package-macos` builds the host `.pkg` with the helper as an
  optional component, which sets `host_service.computer_use` when selected.

At run time the helper checks its own signature against the Developer ID requirement
(`SecCodeCopySelf`, `SecCodeCheckValidity`) and reports `verified` or `development`.
