// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'generated/device_key_api.g.dart';

/// The purpose of a device key (docs/spec/08-identity.md 6.1, R-CL-05).
enum KeyPurpose {
  /// Signs every DPoP proof; never asks the user, the extensions use it too.
  dpop,

  /// Signs step-up confirmations; every signature asks for biometrics or the
  /// device password.
  stepUp;

  KeyPurposeMessage get _message => switch (this) {
    KeyPurpose.dpop => KeyPurposeMessage.dpop,
    KeyPurpose.stepUp => KeyPurposeMessage.stepUp,
  };
}

/// Why a device key operation failed; the codes are stable across platforms.
enum DeviceKeyError {
  /// The user cancelled the biometrics or password prompt.
  userCancelled('user_cancelled'),

  /// The key can no longer be used, for example after new biometrics were
  /// enrolled; the app registers as a new device (08 5.4).
  keyInvalidated('key_invalidated'),

  /// There is no key, or the device cannot hold or use one right now.
  notAvailable('not_available'),

  /// Any other failure.
  internal('internal');

  const DeviceKeyError(this.wire);

  /// The code the native side sends (`PlatformException.code`).
  final String wire;

  static DeviceKeyError _fromWire(String code) => values.firstWhere(
    (e) => e.wire == code,
    orElse: () => DeviceKeyError.internal,
  );
}

/// A failed device key operation.
///
/// It carries only the [code]: the platform's message is dropped, so no key
/// material and no server address reach a log.
class DeviceKeyException implements Exception {
  /// Creates an exception with [code].
  const DeviceKeyException(this.code);

  /// Why the operation failed.
  final DeviceKeyError code;

  @override
  String toString() => 'DeviceKeyException(${code.wire})';
}

/// The public part of a device key.
@immutable
class DeviceKeyInfo {
  /// Creates the description of a key.
  ///
  /// [x] and [y] are the coordinates of the P-256 public key, 32 bytes each,
  /// big-endian.
  const DeviceKeyInfo({
    required this.x,
    required this.y,
    required this.hardwareBound,
    required this.requiresUserPresence,
  });

  /// The x coordinate, 32 bytes big-endian.
  final Uint8List x;

  /// The y coordinate, 32 bytes big-endian.
  final Uint8List y;

  /// Whether the private key lives in secure hardware; false for a software
  /// key, which the app reports to the server as not hardware-bound.
  final bool hardwareBound;

  /// Whether every signature asks the user (the step-up key).
  final bool requiresUserPresence;

  /// The public key as a JWK (RFC 7517) with exactly the members RFC 7638
  /// requires for an EC key, in lexicographic order; base64url without
  /// padding.
  Map<String, String> get publicJwk => {
    'crv': 'P-256',
    'kty': 'EC',
    'x': _base64Url(x),
    'y': _base64Url(y),
  };

  /// The RFC 7638 thumbprint: base64url of SHA-256 over the JSON of
  /// [publicJwk] without whitespace.
  String get thumbprint =>
      _base64Url(sha256.convert(utf8.encode(jsonEncode(publicJwk))).bytes);

  @override
  bool operator ==(Object other) =>
      other is DeviceKeyInfo &&
      listEquals(other.x, x) &&
      listEquals(other.y, y) &&
      other.hardwareBound == hardwareBound &&
      other.requiresUserPresence == requiresUserPresence;

  @override
  int get hashCode => Object.hash(
    Object.hashAll(x),
    Object.hashAll(y),
    hardwareBound,
    requiresUserPresence,
  );

  @override
  String toString() =>
      'DeviceKeyInfo(thumbprint: $thumbprint, hardwareBound: $hardwareBound, '
      'requiresUserPresence: $requiresUserPresence)';
}

String _base64Url(List<int> bytes) =>
    base64Url.encode(bytes).replaceAll('=', '');

/// The texts of the step-up prompt on Android (brief M5-01, Design note 1).
///
/// They come from `packages/l10n`, never from native code. iOS and macOS
/// ignore them, because the system shows its own text there.
@immutable
class StepUpPrompt {
  /// Creates the texts; [subtitle] is optional.
  const StepUpPrompt({
    required this.title,
    this.subtitle,
    required this.negativeButton,
  });

  /// The prompt's title.
  final String title;

  /// An optional second line.
  final String? subtitle;

  /// The cancel button; Android shows it where the prompt accepts biometrics
  /// only (API 26 to 29), because a prompt that also accepts the device
  /// credential offers that instead.
  final String negativeButton;
}

/// Creates, uses and deletes the device keys through the native plugin.
///
/// `scope` separates the keys per server (for example the server's origin);
/// the native side hashes it into the key label. There is no operation that
/// replaces a key of an existing registration (08 5.4): [delete] removes both
/// keys, and the app registers as a new device.
class DeviceKeyClient {
  /// Creates a client; [api] replaces the generated host API in tests.
  DeviceKeyClient({DeviceKeyHostApi? api}) : _api = api ?? DeviceKeyHostApi();

  final DeviceKeyHostApi _api;

  /// Returns the key of [purpose] for [scope], creating it if it is missing.
  ///
  /// Throws a [DeviceKeyException] on failure.
  Future<DeviceKeyInfo> ensure(String scope, KeyPurpose purpose) =>
      _guard(() async => _info(await _api.ensureKey(scope, purpose._message)));

  /// Returns the key of [purpose] for [scope], or null; never creates one.
  ///
  /// Throws a [DeviceKeyException] on failure.
  Future<DeviceKeyInfo?> keyInfo(String scope, KeyPurpose purpose) =>
      _guard(() async {
        final message = await _api.keyInfo(scope, purpose._message);
        return message == null ? null : _info(message);
      });

  /// Signs [data] with ES256 and returns the JOSE form `R || S` (64 bytes,
  /// RFC 7518 3.4).
  ///
  /// The step-up key shows the system prompt first; on Android it needs
  /// [prompt] (without it the call fails with [DeviceKeyError.internal]).
  /// Throws a [DeviceKeyException], with [DeviceKeyError.userCancelled] when
  /// the user cancels and [DeviceKeyError.keyInvalidated] when the key is gone
  /// for good.
  Future<Uint8List> sign(
    String scope,
    KeyPurpose purpose,
    Uint8List data, {
    StepUpPrompt? prompt,
  }) => _guard(() async {
    final signature = await _api.sign(
      scope,
      purpose._message,
      data,
      prompt == null
          ? null
          : PromptTextsMessage(
              title: prompt.title,
              subtitle: prompt.subtitle,
              negativeButton: prompt.negativeButton,
            ),
    );
    if (signature.length != 64) {
      throw const DeviceKeyException(DeviceKeyError.internal);
    }
    return signature;
  });

  /// Returns an attestation of the key over [challenge], or null where the
  /// platform has none (iOS and macOS in this version).
  ///
  /// On Android the attestation is the certificate chain of the key (DER
  /// certificates, leaf first, concatenated). Android binds the challenge when
  /// it creates the key, so this call returns one only when it creates the
  /// key; call it before [ensure] when registering the device.
  ///
  /// Throws a [DeviceKeyException] on failure.
  Future<Uint8List?> attestation(
    String scope,
    KeyPurpose purpose,
    Uint8List challenge,
  ) => _guard(() => _api.attestation(scope, purpose._message, challenge));

  /// Deletes both keys of [scope]; missing keys are not an error.
  ///
  /// Throws a [DeviceKeyException] on failure.
  Future<void> delete(String scope) => _guard(() => _api.deleteKeys(scope));

  static DeviceKeyInfo _info(DeviceKeyInfoMessage message) {
    if (message.x.length != 32 || message.y.length != 32) {
      throw const DeviceKeyException(DeviceKeyError.internal);
    }
    return DeviceKeyInfo(
      x: message.x,
      y: message.y,
      hardwareBound: message.hardwareBound,
      requiresUserPresence: message.requiresUserPresence,
    );
  }

  /// Turns platform errors into [DeviceKeyException]s without their messages.
  static Future<T> _guard<T>(Future<T> Function() body) async {
    try {
      return await body();
    } on PlatformException catch (e) {
      // Pigeon reports `channel-error` when no native handler answers: a
      // platform without the plugin.
      if (e.code == 'channel-error') {
        throw const DeviceKeyException(DeviceKeyError.notAvailable);
      }
      throw DeviceKeyException(DeviceKeyError._fromWire(e.code));
    } on MissingPluginException {
      throw const DeviceKeyException(DeviceKeyError.notAvailable);
    }
  }
}
