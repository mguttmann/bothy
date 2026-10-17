// SPDX-License-Identifier: Apache-2.0

// The goldens of E1 to E7 (brief M4-18 Design note 8; 15 20 row "Golden"): the desktop card of
// each screen state in light and dark, text scaling 1 and 2, German and English, density
// comfortable. One file per screen state keeps each file below repo.max_file_kb.

import 'package:alchemist/alchemist.dart';
import 'package:client/app/account_wiper.dart';
import 'package:client/app/discovery.dart';
import 'package:client/onboarding/e1_server_address.dart';
import 'package:client/onboarding/e2_login.dart';
import 'package:client/onboarding/e3_enrollment_code.dart';
import 'package:client/onboarding/e4_pending_device.dart';
import 'package:client/onboarding/e5_pairing.dart';
import 'package:client/onboarding/e6_app_lock.dart';
import 'package:client/onboarding/e7_signed_out.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../../support/instances.dart';

/// The window the card sits in: a desktop window, smaller than any real one.
const Size window = Size(640, 900);

final DateTime now = DateTime.utc(2026, 10, 17, 9);
DateTime clock() => now;
const Duration interval = Duration(seconds: 10);

void noop() {}

Map<String, Widget Function()> screens() => {
  for (final status in ServerStatus.values)
    if (status != ServerStatus.connecting)
      'e1_${status.name}': () => ServerAddressScreen(
        address: 'https://chat.example.com',
        status: status,
        minVersion: '2.0.0',
        onConnect: (_) {},
      ),
  'e1_connecting': () => const ServerAddressScreen(
    address: 'https://chat.example.com',
    status: ServerStatus.connecting,
    onConnect: _ignore,
  ),
  for (final MapEntry(key: name, value: instance) in {
    'oidc_only': oidcOnly,
    'local_passkey_password': localWithPassword,
    'nothing': nothing,
  }.entries)
    'e2_$name': () => LoginScreen(
      instance: instance,
      passkeysAvailable: true,
      onProvider: (_) {},
      onPasskey: (_) {},
      onPassword: (_, _, _) {},
      onEnroll: noop,
      onPair: noop,
      onChangeServer: noop,
    ),
  'e3': () => EnrollmentCodeScreen(
    instance: oidcOnly,
    passkeysAvailable: true,
    onRedeem: (_, _) {},
    onBack: noop,
  ),
  'e4': () => PendingDeviceScreen(
    code: '481207',
    deadline: now.add(const Duration(minutes: 23, seconds: 30)),
    clock: clock,
    announceInterval: interval,
    announce: (_) {},
    onCancel: noop,
  ),
  'e5': () => PairingScreen(
    code: 'HQ7P3MXR',
    deadline: now.add(const Duration(minutes: 4, seconds: 30)),
    clock: clock,
    announceInterval: interval,
    announce: (_) {},
    onNewCode: noop,
    onBack: noop,
  ),
  'e5_expired': () => PairingScreen(
    expired: true,
    clock: clock,
    announceInterval: interval,
    onNewCode: noop,
    onBack: noop,
  ),
  'e6': () => const AppLockScreen(
    productName: 'Team Chat',
    onUnlock: noop,
    onLogout: noop,
  ),
  'e6_failed': () => const AppLockScreen(
    productName: 'Team Chat',
    failed: true,
    onUnlock: noop,
    onLogout: noop,
  ),
  'e7_revoked_by_other_device': () => const SignedOutScreen(
    cause: SignOutCause.revokedBySelf,
    onLoginAgain: noop,
  ),
  'e7_disabled': () => const SignedOutScreen(
    cause: SignOutCause.userDisabled,
    onLoginAgain: noop,
  ),
};

void _ignore(String _) {}

void main() {
  for (final MapEntry(key: name, value: screen) in screens().entries) {
    goldenTest(
      '$name in light and dark, text scale 1 and 2, de and en',
      fileName: 'onboarding_$name',
      builder: () => dsGoldenMatrix(
        name: name,
        builder: () => SizedBox.fromSize(size: window, child: screen()),
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
  // Keeps the analyzer from flagging the fixture type as unused in this file.
  assert(nothing is InstanceInfo);
}
