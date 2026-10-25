// SPDX-License-Identifier: Apache-2.0

/// The system authentication of the app lock (docs/spec/08-identity.md 9; brief M5-08 Design
/// note 3; docs/decisions.d/M5-08.md D-M5-08-3).
library;

import 'package:flutter/services.dart';
import 'package:local_auth/local_auth.dart';

/// The outcome of one system authentication.
enum AppLockAuthResult {
  /// The person confirmed with biometrics or the device credential.
  success,

  /// The person cancelled, did not match, or the system refused this time; the lock stays.
  failed,

  /// The device has no device credential (PIN, pattern, password or passcode) to unlock with.
  noCredential,
}

/// The system authentication the app lock unlocks with.
///
/// The app counts no attempts of its own: the system limits them (Design note 3).
abstract interface class AppLockPort {
  /// Whether the device can authenticate the person: a device credential or enrolled
  /// biometrics with the device credential as fallback.
  Future<bool> hasDeviceCredential();

  /// Shows the system prompt with [reason] and waits for its outcome; never throws.
  Future<AppLockAuthResult> authenticate(String reason);
}

/// A port that never authenticates: builds without a lock (the desktop, the tests that do not
/// test the lock). [hasDeviceCredential] is false, so an enforced lock there shows the notice.
final class NoAppLockPort implements AppLockPort {
  /// Creates the port.
  const NoAppLockPort();

  @override
  Future<bool> hasDeviceCredential() async => false;

  @override
  Future<AppLockAuthResult> authenticate(String reason) async =>
      AppLockAuthResult.noCredential;
}

/// [AppLockPort] over `local_auth` (flutter.dev, BSD-3-Clause; D-M5-08-3): iOS
/// `LAContext` with `LAPolicy.deviceOwnerAuthentication`, Android `BiometricPrompt` with
/// biometrics or the device credential.
///
/// Example: `await LocalAuthAppLockPort().authenticate('Unlock Team Chat')`.
final class LocalAuthAppLockPort implements AppLockPort {
  /// Creates the port; [auth] replaces the plugin in tests.
  LocalAuthAppLockPort({LocalAuthentication? auth})
    : _auth = auth ?? LocalAuthentication();

  final LocalAuthentication _auth;

  @override
  Future<bool> hasDeviceCredential() async {
    try {
      return await _auth.isDeviceSupported();
    } on PlatformException {
      return false;
    }
  }

  @override
  Future<AppLockAuthResult> authenticate(String reason) async {
    try {
      // biometricOnly false: the device credential is always a way in (Design note 3).
      final ok = await _auth.authenticate(localizedReason: reason);
      return ok ? AppLockAuthResult.success : AppLockAuthResult.failed;
    } on LocalAuthException catch (e) {
      return e.code == LocalAuthExceptionCode.noCredentialsSet
          ? AppLockAuthResult.noCredential
          : AppLockAuthResult.failed;
    } on PlatformException {
      return AppLockAuthResult.failed;
    }
  }
}
