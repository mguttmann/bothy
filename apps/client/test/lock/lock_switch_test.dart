// SPDX-License-Identifier: Apache-2.0

// The switch in Settings, Security and the rule without a device credential (brief M5-08
// Design note 4; fixed requirement 14): `required` shows the switch on and disabled, turning
// it on asks for the system authentication once, a device without a credential keeps it off,
// and with `required` such a device shows only the notice and "Log out".

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
  setUp(() => root = Directory.systemTemp.createTempSync('lock'));
  tearDown(() async {
    await rig.dispose();
    root.deleteSync(recursive: true);
  });

  final toggle = find.byKey(const ValueKey('settings.appLock'));
  Switch switchOf(WidgetTester tester) => tester.widget<Switch>(toggle);

  testWidgets('required shows the switch on and disabled with the reason', (
    tester,
  ) async {
    rig = LockRig(
      lockAtStart: false,
      on: false,
      initial: policy(required: true),
    );
    final (app, _) = await startLocked(tester, rig, root: root);
    await app.go('/settings/security');
    expect(switchOf(tester).value, isTrue);
    expect(switchOf(tester).onChanged, isNull);
    expect(find.text(en.settingsAppLockRequired), findsOneWidget);
    expect(
      await rig.controller.setEnabled(false, reason: ''),
      AppLockSwitchResult.required,
    );
    await rig.dispose();
  });

  testWidgets('turning the lock on asks for the authentication once', (
    tester,
  ) async {
    rig = LockRig(lockAtStart: false, on: false);
    final (app, _) = await startLocked(tester, rig, root: root);
    await app.go('/settings/security');
    expect(switchOf(tester).value, isFalse);
    rig.port.results.add(AppLockAuthResult.failed);
    await tester.tap(toggle);
    await tester.pumpAndSettle();
    expect(switchOf(tester).value, isFalse, reason: 'a failure keeps it off');
    expect(find.text(en.settingsAppLockFailed), findsOneWidget);
    expect(rig.store.on, isFalse);

    await tester.tap(toggle);
    await tester.pumpAndSettle();
    expect(switchOf(tester).value, isTrue);
    expect(rig.store.on, isTrue, reason: 'stored on this device');
    expect(rig.port.prompts, hasLength(2));

    await tester.tap(toggle);
    await tester.pumpAndSettle();
    expect(switchOf(tester).value, isFalse);
    expect(rig.port.prompts, hasLength(2), reason: 'turning off asks nothing');
    await rig.dispose();
  });

  testWidgets('without a device credential the switch stays off with the way', (
    tester,
  ) async {
    rig = LockRig(lockAtStart: false, on: false, credential: false);
    final (app, _) = await startLocked(tester, rig, root: root);
    await app.go('/settings/security');
    await tester.tap(toggle);
    await tester.pumpAndSettle();
    expect(switchOf(tester).value, isFalse);
    expect(find.text(en.settingsAppLockNoCredential), findsOneWidget);
    expect(rig.port.prompts, isEmpty);
    await rig.dispose();
  });

  testWidgets(
    'required without a credential shows only the notice and Log out',
    (tester) async {
      rig = LockRig(
        on: false,
        initial: policy(required: true),
        credential: false,
      );
      final (_, onboarding) = await startLocked(tester, rig, root: root);
      expect(rig.controller.status, AppLockStatus.unavailable);
      expect(
        find.byKey(const ValueKey('screen.e6.unavailable')),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('e6.unlock')), findsNothing);
      expect(shell, findsNothing);
      expect(
        find.text(en.appLockNoCredentialTitle('ExampleChat')),
        findsOneWidget,
      );

      // A credential set in the system settings: "Check again" leads to E6.
      rig.port.credential = true;
      await tester.tap(find.byKey(const ValueKey('e6.checkAgain')));
      await tester.pumpAndSettle();
      expect(e6, findsOneWidget);

      rig.port.credential = false;
      await tester.tap(find.byKey(const ValueKey('e6.unlock')));
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('screen.e6.unavailable')),
        findsOneWidget,
      );
      await tester.tap(find.byKey(const ValueKey('e6.logout')));
      await tester.pumpAndSettle();
      expect(onboarding.logouts, 1);
      await rig.dispose();
    },
  );

  testWidgets(
    'an owner who starts requiring the lock hides the content of a device '
    'without a credential at once',
    (tester) async {
      rig = LockRig(lockAtStart: false, on: false, credential: false);
      await startLocked(tester, rig, root: root);
      expect(shell, findsOneWidget);
      rig.deliver(policy(required: true));
      await tester.pumpAndSettle();
      expect(rig.controller.status, AppLockStatus.unavailable);
      expect(shell, findsNothing);
      await rig.dispose();
    },
  );
}
