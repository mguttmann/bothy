// SPDX-License-Identifier: Apache-2.0

// The paste guard (15 23.4): text with a line break is shown with "Paste with line breaks?"
// and pasted only after "Paste"; text without one goes through at once; the paste keys of the
// view use the guard instead of xterm's own paste.

import 'dart:convert';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

Future<Harness> _connected(WidgetTester tester, String clipboard) async {
  final h = (await tester.runAsync(Harness.create))!;
  await pumpApp(
    tester,
    TerminalPane(
      controller: h.controller,
      fontSize: 14,
      readClipboard: () async => clipboard,
    ),
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

Future<void> _pressPaste(WidgetTester tester) async {
  final apple =
      defaultTargetPlatform == TargetPlatform.macOS ||
      defaultTargetPlatform == TargetPlatform.iOS;
  final modifier = apple ? LogicalKeyboardKey.meta : LogicalKeyboardKey.control;
  await tester.sendKeyDownEvent(modifier);
  if (!apple) await tester.sendKeyDownEvent(LogicalKeyboardKey.shift);
  await tester.sendKeyEvent(LogicalKeyboardKey.keyV);
  if (!apple) await tester.sendKeyUpEvent(LogicalKeyboardKey.shift);
  await tester.sendKeyUpEvent(modifier);
  await tester.pumpAndSettle();
}

void main() {
  test('line breaks need the confirmation; lines are counted', () {
    expect(needsPasteConfirmation('ls -la'), isFalse);
    expect(needsPasteConfirmation('ls\nrm -rf build'), isTrue);
    expect(needsPasteConfirmation('ls\r'), isTrue);
    expect(pasteLineCount('a\nb\nc'), 3);
    expect(pasteLineCount('a\r\nb\r\n'), 2);
    expect(pasteLineCount('one'), 1);
  });

  testWidgets(
    'a paste with line breaks asks first; Cancel sends nothing, Paste sends it',
    (tester) async {
      final h = await _connected(tester, 'echo one\necho two\n');
      await tester.tap(find.byType(TerminalScreenView));
      await tester.pumpAndSettle();
      await _pressPaste(tester);
      expect(find.text('Paste with line breaks?'), findsOneWidget);
      expect(
        find.text(
          'The text has 2 lines. Every line break can start a command.',
        ),
        findsOneWidget,
      );
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
      await tester.pump(const Duration(milliseconds: 20));
      expect(_sent(h), isEmpty);

      await _pressPaste(tester);
      await tester.tap(find.text('Paste'));
      await tester.pumpAndSettle();
      await pumpUntil(tester, () => _sent(h).isNotEmpty);
      expect(_sent(h), contains('echo one'));
      expect(_sent(h), contains('echo two'));
    },
  );

  testWidgets('a single line pastes without a dialog', (tester) async {
    final h = await _connected(tester, 'git status');
    await tester.tap(find.byType(TerminalScreenView));
    await tester.pumpAndSettle();
    await _pressPaste(tester);
    expect(find.text('Paste with line breaks?'), findsNothing);
    await pumpUntil(tester, () => _sent(h).isNotEmpty);
    expect(_sent(h), 'git status');
    // Let the double tap timer of the renderer's gesture detector run out.
    await tester.pump(const Duration(seconds: 1));
  });
}
