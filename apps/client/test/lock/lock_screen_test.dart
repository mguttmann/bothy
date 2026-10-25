// SPDX-License-Identifier: Apache-2.0

// E6 in the phone app (brief M5-08 Tests; 01 5.6 E6; fixed requirements 12 and 13): E6
// replaces every route, no content is in the tree while locked, a deep link waits for the
// unlock, a failure keeps E6 with "Try again", "Log out" leads to E7.

import 'dart:io';

import 'package:client/lock/app_lock_controller.dart';
import 'package:client/lock/app_lock_port.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

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
    await tester.pumpAndSettle();
    expect(e6, findsOneWidget);
    expect(shell, findsNothing);
    await tester.tap(find.byKey(const ValueKey('e6.unlock')));
    await tester.pumpAndSettle();
    expect(app.location, '/mentions', reason: 'back where the person was');
    await rig.dispose();
  });
}
