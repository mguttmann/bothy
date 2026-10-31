// SPDX-License-Identifier: Apache-2.0

/// The one routing rule of a start or "Connect" (docs/DECISIONS.md D-M4-18-15): which screen
/// follows from what the app holds for the instance, what discovery answered and what the
/// server says about this device.
library;

import 'package:api_client/api_client.dart' show DeviceStateEnum;
import 'package:device_key/device_key.dart' show DeviceKeyException;
import 'package:flutter/services.dart' show PlatformException;

import '../app/discovery.dart';
import '../app/router.dart' show OnboardingStep;

/// What the app holds for the instance at the address.
enum StoredAccount {
  /// No refresh token: never signed in there, or the account ended.
  none,

  /// A refresh token of an active device.
  signedIn,

  /// A refresh token of a device that waited for its confirmation when the app last knew
  /// (E4; 08 5.2).
  devicePending,
}

/// What E1 shows below the address field.
enum ServerStatus {
  /// Nothing yet.
  idle,

  /// `getDiscovery` runs.
  connecting,

  /// The text is no address.
  invalidAddress,

  /// No instance at this address.
  noInstance,

  /// The device keys or the keychain of this device refused the app (for example a macOS
  /// build without the keychain entitlements whose login keychain is locked); the cause goes
  /// to the developer log, which a release build does not keep.
  keychainUnavailable,

  /// The local data of the instance could not be opened on this device
  /// (folder or database); the cause goes to the developer log, which a release build does
  /// not keep.
  openFailed,

  /// The TLS handshake failed; no way to continue (R-PR-14).
  certificateInvalid,

  /// `setup_required`.
  notSetUp,

  /// Below `min_client_versions`.
  appTooOld,
}

/// The E1 status of a failure to open the services of an instance: the device keys
/// ([DeviceKeyException]) and the keychain of the secrets (a [PlatformException] of
/// `flutter_secure_storage`, code [keychainErrorCode]) are [ServerStatus.keychainUnavailable],
/// everything else (the instance folder, its file protection, the database)
/// [ServerStatus.openFailed]. Never [ServerStatus.noInstance]: the address was not the problem.
ServerStatus openFailureStatus(Object error) =>
    error is DeviceKeyException ||
        (error is PlatformException && error.code == keychainErrorCode)
    ? ServerStatus.keychainUnavailable
    : ServerStatus.openFailed;

/// The code of every keychain failure `flutter_secure_storage` reports on iOS and macOS.
const String keychainErrorCode = 'Unexpected security result code';

/// The screen a start leads to: [step], and on E1 its [serverStatus] and [minVersion].
typedef StartRoute = ({
  OnboardingStep step,
  ServerStatus serverStatus,
  String? minVersion,
});

/// The screen for [stored], the [discovery] result and the [device] state `getDevice` reported
/// (null without an answer).
///
/// A certificate error, `setup_required` and a too old app always show E1 with their text
/// (R-PR-14, 16 3.1, 18). A stored session otherwise opens its local data, whatever else the
/// network answered (no answer, a timeout, an error page of a proxy, a captive portal): the
/// main view, or E4 while the device waits; the server's answer about the device wins over
/// what was stored (01 3.6, 08 5.2). Without a session a found instance shows E2 and every
/// other answer "No instance at this address".
///
/// Example: `routeStart(stored: StoredAccount.devicePending, discovery: const
/// DiscoveryUnreachable())` is E4.
StartRoute routeStart({
  required StoredAccount stored,
  required DiscoveryOutcome discovery,
  DeviceStateEnum? device,
}) {
  StartRoute server(ServerStatus status, [String? minVersion]) => (
    step: OnboardingStep.server,
    serverStatus: status,
    minVersion: minVersion,
  );
  StartRoute screen(OnboardingStep step) =>
      (step: step, serverStatus: ServerStatus.idle, minVersion: null);
  switch (discovery) {
    case DiscoveryCertificateInvalid():
      return server(ServerStatus.certificateInvalid);
    case DiscoverySetupRequired():
      return server(ServerStatus.notSetUp);
    case DiscoveryAppTooOld(:final minVersion):
      return server(ServerStatus.appTooOld, minVersion);
    case DiscoveryFound() || DiscoveryNoInstance() || DiscoveryUnreachable():
      break;
  }
  if (stored == StoredAccount.none) {
    return discovery is DiscoveryFound
        ? screen(OnboardingStep.login)
        : server(ServerStatus.noInstance);
  }
  final pending =
      device == DeviceStateEnum.pending ||
      (device != DeviceStateEnum.active &&
          stored == StoredAccount.devicePending);
  return screen(pending ? OnboardingStep.pending : OnboardingStep.signedIn);
}
