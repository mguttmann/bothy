# device_key

Device keys in the secure storage of each platform, ES256 (P-256): a Flutter plugin with a typed
Pigeon interface. Two keys per device and server (docs/spec/08-identity.md 6.1, D-L10):

| Purpose | Signs | Asks the user |
|---|---|---|
| `dpop` | every DPoP proof | never; the app extensions use it too |
| `step_up` | step-up confirmations | on every signature (biometrics or device password) |

Status: iOS and macOS (M4-10), Android (M5-01). Windows follows in M4-11; on other platforms
every call fails with `DeviceKeyError.notAvailable`. Kind: Flutter plugin, a member of the Dart
workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## API

```dart
import 'package:device_key/device_key.dart';

final keys = DeviceKeyClient();
const scope = 'https://chat.example.com'; // separates the keys per server
final info = await keys.ensure(scope, KeyPurpose.dpop); // creates the key if missing
info.publicJwk; // {"crv":"P-256","kty":"EC","x":...,"y":...}
info.thumbprint; // RFC 7638, base64url
info.hardwareBound; // false: report the key to the server as not hardware-bound
final signature = await keys.sign(scope, KeyPurpose.dpop, signingInput); // R || S, 64 bytes
final confirmation = await keys.sign(
  scope,
  KeyPurpose.stepUp,
  challengeInput,
  prompt: StepUpPrompt(title: l10n.stepUpTitle, negativeButton: l10n.cancel), // Android only
);
await keys.delete(scope); // removes both keys; there is no key change (08 5.4)
```

| Member | Purpose |
|---|---|
| `DeviceKeyClient` | `ensure`, `keyInfo`, `sign`, `attestation`, `delete`; `api:` takes a fake host API in tests |
| `DeviceKeyInfo` | `x`, `y`, `hardwareBound`, `requiresUserPresence`; `publicJwk` and `thumbprint` computed in Dart for every platform |
| `KeyPurpose` | `dpop`, `stepUp` |
| `StepUpPrompt` | `title`, `subtitle`, `negativeButton` of the Android prompt, from `packages/l10n`; iOS and macOS ignore it |
| `DeviceKeyException`, `DeviceKeyError` | `userCancelled`, `keyInvalidated` (register as a new device), `notAvailable`, `internal`; never a platform message or key material |

`attestation` returns null on iOS and macOS (App Attest is planned with
`auth.require_key_attestation`). On Android it returns the key's certificate chain (DER
certificates, leaf first, concatenated) only when the call creates the key, because Android binds
the challenge at creation; call it before `ensure` when registering (docs/DECISIONS.md D-M5-01-2).

## Storage per platform

| Platform | Secure Enclave available | Without an enclave (older Macs, simulator) |
|---|---|---|
| iOS, macOS: `dpop` | enclave key, `[.privateKeyUsage]`, `AfterFirstUnlockThisDeviceOnly`, shared access group | software P-256 key in the keychain item, same accessibility and group, `hardwareBound = false` |
| iOS, macOS: `step_up` | enclave key, `[.privateKeyUsage, .biometryCurrentSet]` with enrolled biometrics, otherwise `[.privateKeyUsage, .userPresence]`; `WhenPasscodeSetThisDeviceOnly` | software key, the keychain item carries `.userPresence`, `WhenPasscodeSetThisDeviceOnly`, `hardwareBound = false` |

Every key is a generic password item in the Data Protection keychain (`kSecUseDataProtectionKeychain`,
also on macOS), `kSecAttrSynchronizable = false`, so no key reaches a backup or the iCloud keychain.
The item holds the enclave-wrapped blob of the key (useless on another device) or the software key;
the public key sits in `kSecAttrGeneric`, so it is readable without a prompt. Service is the label
prefix (`device_key`), account `<sha256(scope)>.<purpose>`, label `<prefix>.<account>`.

| Android: `dpop` | Android Keystore, P-256, StrongBox where the device has it, otherwise the trusted execution environment; no user authentication | (emulators, devices without secure hardware) software Keystore key, `hardwareBound = false` |
| Android: `step_up` | as `dpop`, plus authentication for every signature through `BiometricPrompt`: from API 30 strong biometrics or the device credential, on API 26 to 29 strong biometrics only; invalidated by new biometrics | same rule; on API 26 to 29 without enrolled biometrics there is no step-up key (`notAvailable`) |

On Android the key never leaves the Keystore and the system excludes the Keystore from backups.
StrongBox is tried first from API 28; on `StrongBoxUnavailableException` the key is generated
again without it. `hardwareBound` reports the real level: `KeyInfo.securityLevel` from API 31,
`isInsideSecureHardware` before it. A new fingerprint or face makes the Keystore drop the step-up
key's authorisation for good; the next `sign` fails with `DeviceKeyError.keyInvalidated`, and the
app registers as a new device. The Android alias is `<prefix>.<sha256(scope)>.<purpose>`.

The step-up prompt needs the app's activity to be a `FragmentActivity` (`FlutterFragmentActivity`
in `MainActivity`) and the texts of `StepUpPrompt`; without either `sign` fails with
`notAvailable` or `internal`. The app needs `minSdk` 26.

The access group of the `dpop` key comes from the app's build configuration: set the Info.plist key
`DeviceKeyAccessGroup` to the group of the entitlements (for example
`$(AppIdentifierPrefix)com.example.shared`). Without it the key stays in the app's default group.

## Layout

| Path | Content |
|---|---|
| `lib/device_key.dart`, `lib/src/device_key.dart` | the Dart API |
| `pigeons/device_key_api.dart` | the Pigeon source of the channel |
| `lib/src/generated/`, `darwin/device_key/Sources/device_key/DeviceKeyApi.g.swift`, `android/src/main/kotlin/devicekey/flutter/DeviceKeyApi.g.kt` | generated by Pigeon, checked in |
| `darwin/device_key/` | the Swift package of the plugin (Swift Package Manager; `sharedDarwinSource` for iOS and macOS), `DeviceKeyPlugin.swift` connects the channel to the core |
| `darwin/device_key/DeviceKeyCore/` | the Swift package `DeviceKeyCore` with the key code and its tests |
| `android/` | the Gradle project of the plugin (`build.gradle.kts`, Gradle wrapper), `DeviceKeyPlugin.kt` connects the channel to the core |
| `android/devicekeycore/` | the Kotlin core module `DeviceKeyCore` with its JVM tests (`src/test`) and instrumented tests (`src/androidTest`) |
| `darwin/run_core_tests.sh`, `darwin/run_ios_sim_tests.sh`, `android/run_instrumented_tests.sh`, `platform-tests.json` | the native tests (`make apps-platform`) |
| `test/` | Dart tests against a fake host API |

## Reuse in the app extensions

The Notification Service Extension and the Share Extension (M5-04, M5-06) add the local package
`packages/device_key/darwin/device_key/DeviceKeyCore` to their targets and sign with the app's DPoP
key without the Flutter engine:

```swift
import DeviceKeyCore

let group = Bundle.main.object(forInfoDictionaryKey: "DeviceKeyAccessGroup") as? String
let keys = DeviceKeyManager(store: KeychainKeyStore(), accessGroup: group)
let signature = try keys.sign(scope: "https://chat.example.com", purpose: .dpop, data: signingInput)
```

The extension needs the same keychain access group in its entitlements as the app.

## Reuse in the Android push service

The push service (M5-05) and the share activity (M5-06) live in the app's Gradle project, which
depends on the plugin project `:device_key`. The plugin library compiles the sources of
`android/devicekeycore` into itself, so the app uses the same classes without including the module
as a second project (that would duplicate them):

```kotlin
import devicekey.core.DeviceKeyCore
import devicekey.core.forContext

val keys = DeviceKeyCore.forContext(applicationContext) { null } // no activity: DPoP only
val signature = keys.signDpop("https://chat.example.com", signingInput) // R || S
```

## Generated code

After a change of `pigeons/device_key_api.dart`, from this folder:

```sh
dart run pigeon --input pigeons/device_key_api.dart && dart format lib/src/generated
```

`make codegen-check` runs the same and fails when the checked-in files differ. Pigeon also writes
`analyzer: exclude: [android/**]` into `analysis_options.yaml`; there is no Dart code there.

## Native tests

`make apps-platform` on a Mac runs `platform-tests.json`:

- `darwin/run_core_tests.sh`: `swift test` of `DeviceKeyCore` (Swift Testing). With only the
  command line tools installed, the script passes their Swift Testing paths, so a Mac with an Apple
  chip proves the Secure Enclave path without Xcode. `testHardwarePathOnAppleChip` reports a skip
  with `secure enclave not available on this host` where there is no enclave (CI virtual machines),
  `testKeychainRoundTrip` a skip when the test process lacks the keychain entitlement.
- `darwin/run_ios_sim_tests.sh`: the same tests in the iOS simulator (the first available iPhone),
  which proves the fallback without an enclave; without Xcode it prints a SKIP line.

The CI job `apps-macos` (`macos-15`) runs both.

Android, with a JDK 17 and the Android SDK (`ANDROID_HOME`), from `android/`:

- `./gradlew -p devicekeycore testDebugUnitTest`: the JVM tests with the in-memory
  `KeyStoreAccess` (DER conversion, alias, error mapping, the core's rules); `make test` runs them.
- `sh run_instrumented_tests.sh`: the instrumented tests against the real Keystore on the one
  attached device or emulator (`make apps-platform` runs it when `adb devices` lists exactly one,
  prints `SKIP ... android: no device` without one and fails with two). On an emulator without a
  screen lock it sets a temporary PIN, because the step-up key needs a secure lock screen, and
  clears it afterwards. `strongBoxWhenAvailable` skips with `strongbox not available on this
  device` on emulators; the step-up tests skip where the device has no authenticator for the key
  (API 26 to 29 without enrolled biometrics), and `stepUpNotAvailableWithoutAnAuthenticator` runs
  there instead. The CI job `apps-android` runs them on emulators with API 26 and 36.

Every test key has a unique `test.device_key.` prefix and is deleted by the test, also after a
failure (on Android in an `@After` method).

## Dependencies

Allowed members (docs/spec/15-client-architecture.md, 3.1; `tools/arch-check/rules.yaml`): none: no
arrow of the diagram leaves this package. Runtime dependency: `crypto` (SHA-256 of the thumbprint);
dev dependency: `pigeon`. Android: `androidx.biometric` and `kotlinx-coroutines-android`. Native sources only in `darwin/`, `ios/`, `macos/`, `android/`,
`windows/`, `linux/`, `web/` (rule 4, checked by `make arch`).

Run from the repository root: `make lint test coverage arch codegen-check COMPONENTS=apps`. The
API pages come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
