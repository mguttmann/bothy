// SPDX-License-Identifier: Apache-2.0

// What the phone app counts as activity of the person for the app lock (brief M5-08 Design
// note 2; docs/decisions.d/AUD-client-app-01.md AUD-CA1/3): a touch and a key, and also what a
// person does through a screen reader or a switch control, which arrives without any pointer
// event: an action for a node of the semantics tree and the focus of the screen reader moving
// on.

import 'dart:io';

import 'package:client/lock/app_lock_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/lock_app.dart';

void main() {
  late Directory root;
  late LockRig rig;
  // The rig is built inside each test body, in the fake async zone of the widget test.
  setUp(() => root = Directory.systemTemp.createTempSync('lock-activity'));
  tearDown(() async {
    await rig.dispose();
    root.deleteSync(recursive: true);
  });

  /// Delivers [action] for the node of [finder] the way the engine does for VoiceOver,
  /// TalkBack and switch control: through the platform dispatcher, without a pointer event.
  void fromAssistiveTechnology(
    WidgetTester tester,
    Finder finder,
    SemanticsAction action,
  ) {
    final node = tester.getSemantics(finder);
    tester.binding.platformDispatcher.onSemanticsActionEvent!(
      SemanticsActionEvent(
        type: action,
        viewId: tester.view.viewId,
        nodeId: node.id,
      ),
    );
  }

  /// Moves the focus of the screen reader to the node of [finder] the way the runners of iOS
  /// and Android report it: the message `didGainFocus` on the accessibility channel, which
  /// they send for every node, also one without any action (plain text).
  Future<void> focusOfScreenReader(WidgetTester tester, Finder finder) =>
      tester.binding.defaultBinaryMessenger.handlePlatformMessage(
        SystemChannels.accessibility.name,
        SystemChannels.accessibility.codec.encodeMessage({
          'type': 'didGainFocus',
          'nodeId': tester.getSemantics(finder).id,
        }),
        (_) {},
      );

  final mentions = find.byKey(const ValueKey('phone.tab.mentions'));

  for (final (name, act) in <(String, Future<void> Function(WidgetTester))>[
    (
      'the focus of a screen reader moving to a node',
      (tester) => focusOfScreenReader(tester, mentions),
    ),
    (
      'the focus action TalkBack sends with its focus',
      (tester) async => fromAssistiveTechnology(
        tester,
        mentions,
        SemanticsAction.didGainAccessibilityFocus,
      ),
    ),
    (
      'a double tap of a screen reader',
      (tester) async =>
          fromAssistiveTechnology(tester, mentions, SemanticsAction.tap),
    ),
  ]) {
    testWidgets('$name is activity: the lock stays open', (tester) async {
      rig = LockRig(lockAtStart: false);
      final semantics = tester.ensureSemantics();
      await startLocked(tester, rig, root: root);
      expect(shell, findsOneWidget);

      // Four of the five minutes pass, then the person acts through the screen reader.
      rig.advance(const Duration(minutes: 4));
      await act(tester);
      await tester.pumpAndSettle();

      // Four more minutes: eight since the start, four since the action.
      rig.advance(const Duration(minutes: 4));
      await rig.controller.check();
      await tester.pump();
      expect(rig.controller.status, AppLockStatus.unlocked);
      expect(e6, findsNothing);
      expect(shell, findsOneWidget);

      // Without any further action the time closes the lock as before.
      rig.advance(const Duration(minutes: 5));
      await rig.controller.check();
      await tester.pump();
      expect(e6, findsOneWidget);
      semantics.dispose();
      await rig.dispose();
    });
  }
}
