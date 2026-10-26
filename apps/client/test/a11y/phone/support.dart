// SPDX-License-Identifier: Apache-2.0

// The phone accessibility tests of the app's screens (brief M5-10 Design note 2): the harness
// of package:design_system/testing.dart with the app's form factor phone around each screen.

import 'dart:async';

import 'package:client/app/form_factor.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

/// Registers the tests of [dsPhoneAccessibilityTests] for one phone screen of the app; every
/// parameter is passed on, and the screen is built inside a [FormFactorScope] of
/// [FormFactor.phone] as in the phone app.
void phoneScreenTests(
  String screen, {
  required Widget Function() build,
  Future<void> Function(WidgetTester tester)? open,
  FutureOr<void> Function(WidgetTester tester, AppLocalizations l)? semantics,
  List<Finder> Function(AppLocalizations l)? reachable,
  TargetPlatform platform = TargetPlatform.iOS,
}) => dsPhoneAccessibilityTests(
  screen,
  build: build,
  open: open,
  semantics: semantics,
  reachable: reachable,
  platform: platform,
  wrap: (child) => FormFactorScope(formFactor: FormFactor.phone, child: child),
);
