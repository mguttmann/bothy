// SPDX-License-Identifier: Apache-2.0

// The app on a phone for widget tests (brief M5-02 Tests): the ports of an iOS build, a phone
// window and an onboarding controller whose state the test sets.

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/phone/phone_shell.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../support/fakes.dart';

/// The logical size of the phone window of the tests (a 6.1 inch iPhone).
const Size phoneSize = Size(390, 844);

/// A phone window: [phoneSize] at three device pixels per logical pixel.
void phoneWindow(WidgetTester tester, {Size size = phoneSize}) {
  tester.view
    ..physicalSize = size * 3
    ..devicePixelRatio = 3;
  addTearDown(tester.view.reset);
}

/// A controller whose state the test sets.
class ScriptedOnboarding extends OnboardingController {
  ScriptedOnboarding(this.initial);

  final OnboardingState initial;

  @override
  OnboardingState build() => initial;

  /// Shows [next].
  void show(OnboardingState next) => state = next;

  @override
  Future<void> logout() async {}
}

/// The running phone app.
class PhoneApp {
  PhoneApp._(this.tester, this.links, this.control);

  final WidgetTester tester;

  /// The deep links of the runner.
  final ManualDeepLinks links;

  /// The onboarding state.
  final ScriptedOnboarding control;

  /// Pumps the app of an iOS build in [step]; [root] is the data folder.
  static Future<PhoneApp> start(
    WidgetTester tester, {
    required Directory root,
    OnboardingStep step = OnboardingStep.signedIn,
    List<Override> overrides = const [],
  }) async {
    phoneWindow(tester);
    final links = ManualDeepLinks();
    final control = ScriptedOnboarding(OnboardingState(step: step));
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appPortsProvider.overrideWithValue(
            testPorts(dataRoot: root, deepLinks: links, platform: 'ios'),
          ),
          onboardingControllerProvider.overrideWith(() => control),
          ...overrides,
        ],
        child: const ClientApp(),
      ),
    );
    await tester.pumpAndSettle();
    return PhoneApp._(tester, links, control);
  }

  /// The shell's state.
  PhoneShellState get shell =>
      tester.state<PhoneShellState>(find.byType(PhoneShell));

  /// Goes to [path] as a tap in the app does.
  Future<void> go(String path) async {
    GoRouter.of(tester.element(find.byType(PhoneShell))).go(path);
    await tester.pumpAndSettle();
  }

  /// Delivers [link] from the runner and waits for the app to show it.
  Future<void> open(Uri link) async {
    links.send(link);
    await tester.pumpAndSettle();
  }

  /// The path the router shows.
  String get location =>
      GoRouter.of(tester.element(find.byType(PhoneShell)))
          .routeInformationProvider
          .value
          .uri
          .toString();
}
