// SPDX-License-Identifier: Apache-2.0

// Touch in the terminal of the phone (01 screen M8; 15 23.4; brief M5-12): a two-finger swipe
// scrolls the scrollback, a long press selects text, and copying and pasting are explicit
// actions of the person: "Copy" puts the selection on the clipboard, "Paste" goes through the
// paste guard, so more than one line asks first.

import 'dart:convert';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

Future<Harness> _connected(WidgetTester tester, {String? clipboard}) async {
  final h = (await tester.runAsync(Harness.create))!;
  await pumpApp(
    tester,
    TerminalPane(
      controller: h.controller,
      fontSize: 14,
      phone: true,
      readClipboard: () async => clipboard,
    ),
    size: const Size(400, 700),
  );
  final opening = h.open();
  await pumpUntil(
    tester,
    () => h.controller.state.phase == TerminalPhase.connected,
  );
  await opening;
  return h;
}

String _sent(Harness h) => utf8.decode(
  h.host.frames.whereType<DataFrame>().expand((f) => f.bytes).toList(),
);

ScrollPosition _scroll(WidgetTester tester) => tester
    .state<ScrollableState>(
      find.descendant(
        of: find.byType(TerminalScreenView),
        matching: find.byType(Scrollable),
      ),
    )
    .position;

void main() {
  late List<String> copied;

  setUp(() {
    copied = [];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, (call) async {
          if (call.method == 'Clipboard.setData') {
            copied.add((call.arguments as Map)['text'] as String);
          }
          return null;
        });
  });

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, null);
  });

  testWidgets('a two-finger swipe scrolls the scrollback', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    try {
      final h = await _connected(tester);
      await h.host.sendData(
        utf8.encode(List.generate(200, (i) => 'line $i').join('\r\n')),
      );
      await pumpUntil(
        tester,
        () => h.controller.screen.cursorLine().contains('line 199'),
      );
      await tester.pump();
      final bottom = _scroll(tester).pixels;
      expect(bottom, greaterThan(0));
      final center = tester.getCenter(find.byType(TerminalScreenView));
      final a = await tester.startGesture(center - const Offset(30, 0));
      final b = await tester.startGesture(
        center + const Offset(30, 0),
        pointer: 7,
      );
      for (var i = 0; i < 10; i++) {
        await a.moveBy(const Offset(0, 20));
        await b.moveBy(const Offset(0, 20));
        await tester.pump();
      }
      await a.up();
      await b.up();
      await tester.pumpAndSettle();
      expect(_scroll(tester).pixels, lessThan(bottom));
      await h.controller.detach();
      await tester.pump(const Duration(seconds: 1));
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('a long press selects a word and Copy puts it on the clipboard', (
    tester,
  ) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    try {
      final h = await _connected(tester);
      await h.host.sendData(utf8.encode('hello world'));
      await pumpUntil(
        tester,
        () => h.controller.screen.cursorLine().contains('hello'),
      );
      await tester.pump();
      expect(find.text('Copy'), findsNothing);
      final topLeft = tester.getTopLeft(find.byType(TerminalScreenView));
      await tester.longPressAt(topLeft + const Offset(12, 8));
      await tester.pumpAndSettle();
      expect(find.text('Copy'), findsOneWidget);
      await tester.tap(find.text('Copy'));
      await tester.pumpAndSettle();
      expect(copied, ['hello']);
      // The selection is gone after the copy; nothing reached the host.
      expect(find.text('Copy'), findsNothing);
      expect(_sent(h), isEmpty);
      await h.controller.detach();
      await tester.pump(const Duration(seconds: 1));
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('Paste with more than one line asks first', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    try {
      final h = await _connected(tester, clipboard: 'ls\nrm -rf build');
      await h.host.sendData(utf8.encode('prompt'));
      await pumpUntil(
        tester,
        () => h.controller.screen.cursorLine().contains('prompt'),
      );
      await tester.pump();
      final topLeft = tester.getTopLeft(find.byType(TerminalScreenView));
      await tester.longPressAt(topLeft + const Offset(12, 8));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Paste'));
      await tester.pumpAndSettle();
      expect(find.byType(PasteConfirmDialog), findsOneWidget);
      expect(_sent(h), isEmpty);
      await tester.tap(find.widgetWithText(FilledButton, 'Paste'));
      await tester.pumpAndSettle();
      await pumpUntil(tester, () => _sent(h).contains('rm -rf build'));
      await h.controller.detach();
      await tester.pump(const Duration(seconds: 1));
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });
}
