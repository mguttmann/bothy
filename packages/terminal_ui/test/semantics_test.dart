// SPDX-License-Identifier: Apache-2.0

// Semantics of the screen (01 8, components.md 12): the line at the cursor is readable, the
// visible screen is read on command, and running output is no live region.

import 'dart:convert';

import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

void main() {
  testWidgets(
    'the line at the cursor is the label; Read screen announces the screen; no '
    'live region',
    (tester) async {
      final handle = tester.ensureSemantics();
      final h = (await tester.runAsync(Harness.create))!;
      await pumpApp(
        tester,
        TerminalPane(controller: h.controller, fontSize: 14),
      );
      final opening = h.open();
      await pumpUntil(
        tester,
        () => h.controller.state.phase == TerminalPhase.connected,
      );
      await opening;
      await h.host.sendData(
        utf8.encode('first line\r\nsecond line\r\n~/src \$ '),
      );
      await pumpUntil(
        tester,
        () => h.controller.screen.visibleText().contains('~/src'),
      );
      await tester.pump();

      final node = find.bySemanticsLabel(
        RegExp(r'^Terminal, line at the cursor: ~/src \$'),
      );
      expect(node, findsOneWidget);
      final data = tester.getSemantics(node).getSemanticsData();
      expect(data.flagsCollection.isLiveRegion, isFalse);
      expect(
        tester.takeAnnouncements(),
        isEmpty,
        reason: 'output is not read aloud',
      );

      final action = data.customSemanticsActionIds!.single;
      final semanticsNode = tester.getSemantics(node);
      semanticsNode.owner!.performAction(
        semanticsNode.id,
        SemanticsAction.customAction,
        action,
      );
      await tester.pump();
      final said = tester.takeAnnouncements().single.message;
      expect(said, contains('first line'));
      expect(said, contains('second line'));
      handle.dispose();
    },
  );
}
