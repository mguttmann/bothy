// SPDX-License-Identifier: Apache-2.0

// On Windows the screen reads the hardware keyboard only (brief M4-36, D-M4-36-10): the Windows
// embedder refuses the text input client of xterm 4.0.0, which sets no view, so the screen
// attaches none there and still sends typed characters to the host. macOS, Linux and the phones
// keep xterm's text input path (dead keys, input methods, the soft keyboard); the test checks it
// on a phone, where the client attaches on focus.

import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

Future<Harness> _connected(WidgetTester tester) async {
  final h = (await tester.runAsync(Harness.create))!;
  await pumpApp(tester, TerminalPane(controller: h.controller, fontSize: 14));
  final opening = h.open();
  await pumpUntil(
    tester,
    () => h.controller.state.phase == TerminalPhase.connected,
  );
  await opening;
  return h;
}

void main() {
  test('only Windows reads the hardware keyboard only', () {
    expect(
      [
        for (final p in TargetPlatform.values)
          if (hardwareKeyboardOnlyOn(p)) p,
      ],
      [TargetPlatform.windows],
    );
  });

  for (final platform in [TargetPlatform.windows]) {
    testWidgets(
      '${platform.name}: no text input client, typed keys reach the host',
      (tester) async {
        debugDefaultTargetPlatformOverride = platform;
        try {
          final h = await _connected(tester);
          await tester.tap(find.byType(TerminalScreenView));
          await tester.pump();
          expect(tester.testTextInput.hasAnyClients, isFalse);
          await tester.sendKeyEvent(LogicalKeyboardKey.keyA, character: 'a');
          await pumpUntil(tester, () => h.host.input.isNotEmpty);
          expect(utf8.decode(h.host.input), 'a');
          await h.controller.detach();
          await tester.pump(const Duration(seconds: 1));
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );
  }

  // macOS and Linux keep xterm's own behavior, unchanged by this rule.
  for (final platform in [TargetPlatform.android]) {
    testWidgets('${platform.name} keeps the text input client', (tester) async {
      debugDefaultTargetPlatformOverride = platform;
      try {
        final h = await _connected(tester);
        await tester.tap(find.byType(TerminalScreenView));
        await tester.pump();
        expect(tester.testTextInput.hasAnyClients, isTrue);
        await h.controller.detach();
        await tester.pump(const Duration(seconds: 1));
      } finally {
        debugDefaultTargetPlatformOverride = null;
      }
    });
  }
}
