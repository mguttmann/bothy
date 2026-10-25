// SPDX-License-Identifier: Apache-2.0

// The events of the lock (brief M5-08 Design note 5; fixed requirement 15): `locked` before E6
// appears, `unlocked` after a successful unlock only; the lock never reattaches anything.

import 'dart:io';

import 'package:client/lock/app_lock_controller.dart';
import 'package:client/lock/app_lock_port.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

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

  testWidgets('locked comes before E6, unlocked after the success', (
    tester,
  ) async {
    rig = LockRig(lockAtStart: false);
    await startLocked(tester, rig, root: root);
    final seen = <(AppLockEvent, bool)>[];
    final sub = rig.controller.events.listen(
      (e) => seen.add((e, e6.evaluate().isNotEmpty)),
    );
    addTearDown(sub.cancel);

    rig.advance(const Duration(minutes: 5));
    await rig.controller.check();
    await tester.pumpAndSettle();
    expect(seen, [(AppLockEvent.locked, false)], reason: 'E6 not yet shown');
    expect(e6, findsOneWidget);

    rig.port.results.add(AppLockAuthResult.failed);
    await tester.tap(find.byKey(const ValueKey('e6.unlock')));
    await tester.pumpAndSettle();
    expect(seen, hasLength(1), reason: 'a failure emits nothing');

    await tester.tap(find.byKey(const ValueKey('e6.unlock')));
    await tester.pumpAndSettle();
    expect(seen.last.$1, AppLockEvent.unlocked);
    expect(seen, hasLength(2));
    await rig.dispose();
  });

  testWidgets('a second check while locked emits no second locked', (
    tester,
  ) async {
    rig = LockRig();
    final events = <AppLockEvent>[];
    final sub = rig.controller.events.listen(events.add);
    addTearDown(sub.cancel);
    await startLocked(tester, rig, root: root);
    await rig.controller.check();
    expect(events, [AppLockEvent.locked]);
    await rig.dispose();
  });
}
