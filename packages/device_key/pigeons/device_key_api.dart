// SPDX-License-Identifier: Apache-2.0

/// The Pigeon source of the channel between Dart and the native device key code
/// (brief M4-10, Design note 1).
///
/// Regenerate with `dart run pigeon --input pigeons/device_key_api.dart` in
/// `packages/device_key`; `make codegen-check` regenerates and compares. The
/// generated files are `lib/src/generated/device_key_api.g.dart`,
/// `darwin/device_key/Sources/device_key/DeviceKeyApi.g.swift` and
/// `android/src/main/kotlin/devicekey/flutter/DeviceKeyApi.g.kt`.
library;

import 'package:pigeon/pigeon.dart';

/// The purpose of a device key on the channel.
enum KeyPurposeMessage {
  /// Signs DPoP proofs without asking the user.
  dpop,

  /// Signs step-up confirmations; every signature asks the user.
  stepUp,
}

/// The public part of a device key as the native side reports it.
class DeviceKeyInfoMessage {
  /// Creates a message; Pigeon fills every field.
  DeviceKeyInfoMessage({
    required this.x,
    required this.y,
    required this.hardwareBound,
    required this.requiresUserPresence,
  });

  /// The x coordinate of the P-256 public key, 32 bytes big-endian.
  Uint8List x;

  /// The y coordinate of the P-256 public key, 32 bytes big-endian.
  Uint8List y;

  /// Whether the key lives in secure hardware (Secure Enclave, TPM, StrongBox).
  bool hardwareBound;

  /// Whether every signature asks the user.
  bool requiresUserPresence;
}

/// The texts of the step-up prompt on Android (brief M5-01, Design note 1).
///
/// They come from `packages/l10n`; iOS and macOS ignore them, because the
/// system shows its own text there.
class PromptTextsMessage {
  /// Creates a message; Pigeon fills every field.
  PromptTextsMessage({
    required this.title,
    required this.subtitle,
    required this.negativeButton,
  });

  /// The prompt's title.
  String title;

  /// An optional second line.
  String? subtitle;

  /// The cancel button, shown where the prompt accepts biometrics only.
  String negativeButton;
}

/// The native side of the device key. Errors are `PlatformException`s with the
/// codes `user_cancelled`, `key_invalidated`, `not_available` and `internal`.
@ConfigurePigeon(
  PigeonOptions(
    dartOut: 'lib/src/generated/device_key_api.g.dart',
    dartPackageName: 'device_key',
    swiftOut: 'darwin/device_key/Sources/device_key/DeviceKeyApi.g.swift',
    kotlinOut: 'android/src/main/kotlin/devicekey/flutter/DeviceKeyApi.g.kt',
    kotlinOptions: KotlinOptions(package: 'devicekey.flutter'),
    copyrightHeader: 'pigeons/copyright.txt',
  ),
)
@HostApi()
abstract class DeviceKeyHostApi {
  /// Returns the key of [purpose] for [scope], creating it if it is missing.
  DeviceKeyInfoMessage ensureKey(String scope, KeyPurposeMessage purpose);

  /// Returns the key of [purpose] for [scope], or null; never creates one.
  DeviceKeyInfoMessage? keyInfo(String scope, KeyPurposeMessage purpose);

  /// Signs [data] with ES256 and returns `R || S` (64 bytes); asynchronous,
  /// because the step-up key shows the system prompt. [prompt] carries the
  /// prompt texts for Android; the step-up key there fails with `internal`
  /// without them.
  @async
  Uint8List sign(
    String scope,
    KeyPurposeMessage purpose,
    Uint8List data,
    PromptTextsMessage? prompt,
  );

  /// Returns an attestation of the key over [challenge], or null where the
  /// platform has none.
  Uint8List? attestation(
    String scope,
    KeyPurposeMessage purpose,
    Uint8List challenge,
  );

  /// Deletes both keys of [scope].
  void deleteKeys(String scope);
}
