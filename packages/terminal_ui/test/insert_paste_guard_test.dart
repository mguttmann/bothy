// SPDX-License-Identifier: Apache-2.0

// Red team round 3: text that the soft keyboard commits through the text input connection
// must not bypass the paste guard of 15 23.4.

import 'dart:convert';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

Future<Harness> _connected(WidgetTester tester) async {
  final h = (await tester.runAsync(Harness.create))!;
  await pumpApp(
    tester,
    TerminalPane(
      controller: h.controller,
      fontSize: 14,
      readClipboard: () async => '',
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

Future<void> _commitThroughIme(WidgetTester tester, String text) async {
  await tester.tap(find.byType(TerminalScreenView));
  await tester.pumpAndSettle();
  expect(tester.testTextInput.hasAnyClients, isTrue);
  tester.testTextInput.updateEditingValue(
    TextEditingValue(
      text: text,
      selection: TextSelection.collapsed(offset: text.length),
    ),
  );
  for (var i = 0; i < 50; i++) {
    await tester.pump(const Duration(milliseconds: 1));
  }
}

void main() {
  testWidgets(
    'plain text and a single Enter of the soft keyboard go straight to the host',
    (tester) async {
      final h = await _connected(tester);
      await _commitThroughIme(tester, 'ls');
      tester.testTextInput.updateEditingValue(
        const TextEditingValue(
          text: '\n',
          selection: TextSelection.collapsed(offset: 1),
        ),
      );
      for (var i = 0; i < 50; i++) {
        await tester.pump(const Duration(milliseconds: 1));
      }
      expect(_sent(h), contains('ls'));
      expect(find.text('Paste with line breaks?'), findsNothing);
      await tester.pump(const Duration(seconds: 1));
    },
  );

  testWidgets(
    'a multi-line insert of the soft keyboard asks first and sends nothing before Paste',
    (tester) async {
      final h = await _connected(tester);
      await _commitThroughIme(tester, 'echo one\nrm x\n');
      expect(_sent(h), isNot(contains('\n')));
      expect(_sent(h), isNot(contains('rm x')));
      expect(find.text('Paste with line breaks?'), findsOneWidget);
      // xterm's double-tap timer of the tap that focused the view.
      await tester.pump(const Duration(seconds: 1));
    },
  );

  testWidgets(
    'an insert of the soft keyboard never sends ESC (no early end of a bracketed paste)',
    (tester) async {
      final h = await _connected(tester);
      await _commitThroughIme(tester, 'echo hi\x1b[201~');
      expect(_sent(h), isNot(contains('\x1b')));
      await tester.pump(const Duration(seconds: 1));
    },
  );
}
