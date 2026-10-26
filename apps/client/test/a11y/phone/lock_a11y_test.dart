// SPDX-License-Identifier: Apache-2.0

// Accessibility of the app lock of M5-08 (brief M5-10 Design note 2; 01 5.6 E6, 8): E6 with
// Unlock and Log out, after a failed attempt with the reason in words, and the notice for a
// device without a screen lock. E6 is also part of onboarding_phone_a11y_test.dart.

import 'package:client/lock/lock_screen.dart';
import 'package:client/onboarding/e6_app_lock.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void _noop() {}

void main() {
  for (final failed in [false, true]) {
    phoneScreenTests(
      failed ? 'the app lock after a failed unlock' : 'the app lock',
      build: () => AppLockScreen(
        productName: 'Team Chat',
        failed: failed,
        onUnlock: _noop,
        onLogout: _noop,
      ),
      reachable: (l) => [
        find.byKey(const ValueKey('e6.unlock')),
        find.text(l.onboardingLogOut),
      ],
    );
  }
  phoneScreenTests(
    'the app lock without a screen lock of the device',
    build: () => const AppLockUnavailableScreen(
      productName: 'Team Chat',
      onCheckAgain: _noop,
      onLogout: _noop,
    ),
    reachable: (l) => [
      find.byKey(const ValueKey('e6.checkAgain')),
      find.byKey(const ValueKey('e6.logout')),
    ],
  );
}
