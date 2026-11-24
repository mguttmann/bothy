// SPDX-License-Identifier: Apache-2.0

// Semantics of the screen (01 8, components.md 12): the line at the cursor is readable, the
// visible screen is read on command, and running output is no live region.

import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';
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

  // a11y-3: macOS, Windows and the browser ignore custom semantics actions, so the command
  // also has keys, which the screen names in its hint.
  for (final (platform, keys, meta) in [
    (TargetPlatform.macOS, 'Cmd+Shift+S', true),
    (TargetPlatform.windows, 'Ctrl+Shift+S', false),
  ]) {
    testWidgets('${platform.name}: $keys reads the screen', (tester) async {
      debugDefaultTargetPlatformOverride = platform;
      final handle = tester.ensureSemantics();
      try {
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
        await h.host.sendData(utf8.encode('first line\r\n~/src \$ '));
        await pumpUntil(
          tester,
          () => h.controller.screen.visibleText().contains('~/src'),
        );
        await tester.tap(find.byType(TerminalScreenView));
        await tester.pump();
        expect(terminalReadScreenKeys(platform), keys);
        final node = find.bySemanticsLabel(RegExp('^Terminal, line'));
        expect(
          tester.getSemantics(node).getSemanticsData().hint,
          '$keys reads the screen',
        );
        tester.takeAnnouncements();
        await tester.sendKeyDownEvent(
          meta ? LogicalKeyboardKey.metaLeft : LogicalKeyboardKey.controlLeft,
        );
        await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
        await tester.sendKeyEvent(LogicalKeyboardKey.keyS);
        await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
        await tester.sendKeyUpEvent(
          meta ? LogicalKeyboardKey.metaLeft : LogicalKeyboardKey.controlLeft,
        );
        await tester.pump();
        final said = tester.takeAnnouncements().single.message;
        expect(said, contains('first line'));
        expect(h.host.input, isEmpty, reason: 'the keys stay with the app');
        await h.controller.detach();
        await tester.pump(const Duration(seconds: 1));
      } finally {
        handle.dispose();
        debugDefaultTargetPlatformOverride = null;
      }
    });
  }
}
