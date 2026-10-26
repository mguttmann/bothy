// SPDX-License-Identifier: Apache-2.0

// The phone app with a lock the test drives (brief M5-08 Tests): the phone harness of M5-02
// with the lock provider replaced by a controller over fakes.

import 'dart:io';

import 'package:client/app/account_wiper.dart' show SignOutCause;
import 'package:client/lock/providers.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';

import '../../phone/support/phone_app.dart';
import 'lock_fakes.dart';

export '../../phone/support/phone_app.dart';
export 'lock_fakes.dart';

/// An onboarding whose "Log out" shows E7 and counts the calls.
class LoggingOutOnboarding extends ScriptedOnboarding {
  LoggingOutOnboarding()
    : super(const OnboardingState(step: OnboardingStep.signedIn));

  /// How often "Log out" ran.
  int logouts = 0;

  @override
  Future<void> logout() async {
    logouts++;
    show(
      const OnboardingState(
        step: OnboardingStep.signedOut,
        cause: SignOutCause.loggedOut,
      ),
    );
  }
}

/// Starts the phone app signed in with [rig]'s lock and waits for the lock's first decision.
Future<(PhoneApp, LoggingOutOnboarding)> startLocked(
  WidgetTester tester,
  LockRig rig, {
  required Directory root,
}) async {
  final onboarding = LoggingOutOnboarding();
  final app = await PhoneApp.start(
    tester,
    root: root,
    onboarding: onboarding,
    overrides: [appLockProviderOverride(rig)],
  );
  await rig.settle();
  await tester.pumpAndSettle();
  return (app, onboarding);
}

/// The lock screen E6.
final Finder e6 = find.byKey(const ValueKey('screen.e6'));

/// The phone shell with content, also when offstage.
final Finder shell = find.byKey(
  const ValueKey('phone.shell'),
  skipOffstage: false,
);

/// The override of the lock provider with [rig]'s controller.
Override appLockProviderOverride(LockRig rig) =>
    appLockProvider.overrideWithValue(rig.controller);
