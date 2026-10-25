// SPDX-License-Identifier: Apache-2.0

// E6 in the phone app (brief M5-08 Tests; 01 5.6 E6; fixed requirements 12 and 13): E6
// replaces every route, no content is in the tree while locked, a deep link waits for the
// unlock, a failure keeps E6 with "Try again", "Log out" leads to E7.

import 'dart:async';
import 'dart:io';

import 'package:client/lock/app_lock_controller.dart';
import 'package:client/lock/app_lock_port.dart';
import 'package:client/lock/providers.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity;

import '../support/widgets.dart' show en;
import 'support/lock_app.dart';

void main() {
  late Directory root;
  late LockRig rig;
  // The rig is built inside each test body, in the fake async zone of the widget test.
  setUp(() => root = Directory.systemTemp.createTempSync('lock'));
  tearDown(() async {
    await rig.dispose();
    root.deleteSync(recursive: true);
  });

  testWidgets('a cold start shows E6 and no content, for every route', (
    tester,
  ) async {
    rig = LockRig();
    final (app, _) = await startLocked(tester, rig, root: root);
    expect(rig.controller.status, AppLockStatus.locked);
    expect(e6, findsOneWidget);
    expect(shell, findsNothing, reason: 'no content in the tree');
    expect(find.text(en.onboardingLockedTitle('ExampleChat')), findsOneWidget);
    for (final path in ['/', '/mentions', '/running', '/settings/about']) {
      await app.open(Uri.parse('chat-example:$path'));
      expect(e6, findsOneWidget, reason: path);
      expect(shell, findsNothing, reason: path);
    }
    await rig.dispose();
  });

  testWidgets('a deep link that arrives while locked opens after the unlock', (
    tester,
  ) async {
    rig = LockRig();
    final (app, _) = await startLocked(tester, rig, root: root);
    await app.open(Uri.parse('chat-example://mentions'));
    expect(e6, findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('e6.unlock')));
    await tester.pumpAndSettle();
    expect(rig.port.prompts, [en.appLockReason('ExampleChat')]);
    expect(e6, findsNothing);
    expect(shell, findsOneWidget);
    expect(app.location, '/mentions');
    await rig.dispose();
  });

  testWidgets('a failed unlock keeps E6 with "Try again"', (tester) async {
    rig = LockRig();
    rig.port.results.add(AppLockAuthResult.failed);
    await startLocked(tester, rig, root: root);
    await tester.tap(find.byKey(const ValueKey('e6.unlock')));
    await tester.pumpAndSettle();
    expect(e6, findsOneWidget);
    expect(shell, findsNothing);
    expect(find.byKey(const ValueKey('e6.failed')), findsOneWidget);
    expect(find.text(en.onboardingTryAgain), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('e6.unlock')));
    await tester.pumpAndSettle();
    expect(shell, findsOneWidget);
    await rig.dispose();
  });

  testWidgets('"Log out" on E6 ends the session and shows E7', (tester) async {
    rig = LockRig();
    final (_, onboarding) = await startLocked(tester, rig, root: root);
    await tester.tap(find.byKey(const ValueKey('e6.logout')));
    await tester.pumpAndSettle();
    expect(onboarding.logouts, 1);
    expect(find.byKey(const ValueKey('screen.e7')), findsOneWidget);
    expect(rig.port.prompts, isEmpty, reason: 'no unlock needed to log out');
    await rig.dispose();
  });

  testWidgets('the lock closes over open content and hides it', (tester) async {
    rig = LockRig(lockAtStart: false);
    final (app, _) = await startLocked(tester, rig, root: root);
    expect(shell, findsOneWidget);
    await app.go('/mentions');
    rig.advance(const Duration(minutes: 5));
    await rig.controller.check();
    // One frame: E6 replaces the content at once, no page transition paints it meanwhile.
    await tester.pump();
    expect(e6, findsOneWidget);
    expect(find.byKey(const ValueKey('phone.shell')), findsNothing);
    await tester.pumpAndSettle();
    expect(shell, findsNothing);
    await tester.tap(find.byKey(const ValueKey('e6.unlock')));
    await tester.pumpAndSettle();
    expect(app.location, '/mentions', reason: 'back where the person was');
    await rig.dispose();
  });

  testWidgets('while the lock decides at a cold start only the cover shows', (
    tester,
  ) async {
    // A lock whose cached policy has not been read yet stays in `checking`.
    rig = LockRig();
    final undecided = AppLockController(
      port: rig.port,
      clock: rig.clock,
      lifecycle: rig.lifecycle,
      store: rig.store,
      policy: const Stream.empty(),
      lockAtStart: true,
    );
    final onboarding = ScriptedOnboarding(
      const OnboardingState(step: OnboardingStep.server),
    );
    await PhoneApp.start(
      tester,
      root: root,
      onboarding: onboarding,
      overrides: [
        appLockProvider.overrideWith(
          (ref) =>
              ref.watch(onboardingControllerProvider).step ==
                  OnboardingStep.signedIn
              ? undecided
              : null,
        ),
      ],
    );
    onboarding.show(
      const OnboardingState(step: OnboardingStep.signedIn, restored: true),
    );
    await tester.pump();
    await tester.pump();
    expect(undecided.status, AppLockStatus.checking);
    expect(find.byKey(const ValueKey('privacy.cover')), findsOneWidget);
    expect(find.byKey(const ValueKey('phone.shell')), findsNothing);
    undecided.dispose();
    await rig.dispose();
  });

  group('the return from the background', () {
    Future<void> leave(WidgetTester tester) async {
      for (final state in [
        AppLifecycleState.inactive,
        AppLifecycleState.hidden,
        AppLifecycleState.paused,
      ]) {
        tester.binding.handleAppLifecycleStateChanged(state);
      }
      rig.lifecycle.set(AppActivity.background);
    }

    void come(WidgetTester tester) {
      for (final state in [
        AppLifecycleState.hidden,
        AppLifecycleState.inactive,
        AppLifecycleState.resumed,
      ]) {
        tester.binding.handleAppLifecycleStateChanged(state);
      }
      rig.lifecycle.set(AppActivity.foreground);
    }

    final onstage = find.byKey(const ValueKey('phone.shell'));

    testWidgets(
      'after the time E6 is in the first frame, before the boot clock answers',
      (tester) async {
        rig = LockRig(lockAtStart: false);
        await startLocked(tester, rig, root: root);
        await leave(tester);
        rig.advance(const Duration(minutes: 10));
        rig.monotonic.hold = Completer<void>();
        come(tester);
        await tester.pump();
        expect(onstage, findsNothing);
        expect(e6, findsOneWidget);
        rig.monotonic.hold!.complete();
        await tester.pumpAndSettle();
        expect(e6, findsOneWidget);
        await rig.dispose();
      },
    );

    testWidgets(
      'while only the boot clock knows the time, the cover waits for its answer',
      (tester) async {
        rig = LockRig(lockAtStart: false);
        await startLocked(tester, rig, root: root);
        await leave(tester);
        rig.sleep(const Duration(minutes: 10));
        rig.setWallBack(
          const Duration(minutes: 10) - const Duration(seconds: 1),
        );
        rig.monotonic.hold = Completer<void>();
        come(tester);
        await tester.pump();
        expect(onstage, findsNothing, reason: 'covered while the lock decides');
        expect(find.byKey(const ValueKey('privacy.cover')), findsOneWidget);
        rig.monotonic.hold!.complete();
        await tester.pumpAndSettle();
        expect(e6, findsOneWidget);
        expect(onstage, findsNothing);
        await rig.dispose();
      },
    );

    testWidgets(
      'a short trip uncovers the content once the boot clock answered',
      (tester) async {
        rig = LockRig(lockAtStart: false);
        await startLocked(tester, rig, root: root);
        await leave(tester);
        rig.advance(const Duration(seconds: 10));
        come(tester);
        await tester.pumpAndSettle();
        expect(onstage, findsOneWidget);
        expect(e6, findsNothing);
        await rig.dispose();
      },
    );
  });
}
