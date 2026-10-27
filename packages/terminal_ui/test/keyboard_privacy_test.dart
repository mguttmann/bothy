// SPDX-License-Identifier: Apache-2.0

// Keyboard privacy of the terminal on the phones (brief M5-12 fixed requirement 15, Design
// note 3; PLAN 11 red team "keyboard suggestions"): the text input client that feeds the
// terminal on iOS and Android asks the system keyboard for no suggestions, no autocorrection, no
// smart punctuation and no personalized learning, so no terminal input enters the keyboard's
// dictionary; typed text, a deletion and the return key still reach the host.

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
  test('only the phones use the private text input client', () {
    expect(
      [
        for (final p in TargetPlatform.values)
          if (privateTextInputOn(p)) p,
      ],
      [TargetPlatform.android, TargetPlatform.iOS],
    );
  });

  for (final platform in [TargetPlatform.android, TargetPlatform.iOS]) {
    testWidgets(
      '${platform.name}: suggestions, autocorrection, smart punctuation and learning are off',
      (tester) async {
        debugDefaultTargetPlatformOverride = platform;
        try {
          final h = await _connected(tester);
          await tester.tap(find.byType(TerminalScreenView));
          await tester.pump();
          expect(tester.testTextInput.hasAnyClients, isTrue);
          final config = tester.testTextInput.setClientArgs!;
          expect(config['autocorrect'], isFalse);
          expect(config['enableSuggestions'], isFalse);
          expect(config['enableIMEPersonalizedLearning'], isFalse);
          // SmartDashesType.disabled and SmartQuotesType.disabled.
          expect(config['smartDashesType'], '0');
          expect(config['smartQuotesType'], '0');
          expect(config['enableDeltaModel'], isFalse);
          expect(config['autofill'], isNull);
          final type = (config['inputType'] as Map)['name'];
          expect(
            type,
            platform == TargetPlatform.android
                ? 'TextInputType.visiblePassword'
                : 'TextInputType.text',
          );
          await h.controller.detach();
          await tester.pump(const Duration(seconds: 1));
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );
  }

  testWidgets(
    'typed text, a deletion and return reach the host through the client',
    (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
      try {
        final h = await _connected(tester);
        await tester.tap(find.byType(TerminalScreenView));
        await tester.pump();
        final start = tester.testTextInput.editingState!;
        final base = start['text'] as String;
        tester.testTextInput.updateEditingValue(
          TextEditingValue(
            text: '${base}l',
            selection: TextSelection.collapsed(offset: base.length + 1),
          ),
        );
        // The client puts the field back to its sentinel after every change.
        tester.testTextInput.updateEditingValue(
          TextEditingValue(
            text: base.substring(0, base.length - 1),
            selection: TextSelection.collapsed(offset: base.length - 1),
          ),
        );
        tester.testTextInput.updateEditingValue(
          TextEditingValue(
            text: '$base\n',
            selection: TextSelection.collapsed(offset: base.length + 1),
          ),
        );
        await pumpUntil(tester, () => h.host.input.length >= 3);
        expect(utf8.decode(h.host.input), 'l\x7f\r');
        await h.controller.detach();
        await tester.pump(const Duration(seconds: 1));
      } finally {
        debugDefaultTargetPlatformOverride = null;
      }
    },
  );

  testWidgets('a composition is sent only when it ends', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    try {
      final h = await _connected(tester);
      await tester.tap(find.byType(TerminalScreenView));
      await tester.pump();
      final base = tester.testTextInput.editingState!['text'] as String;
      tester.testTextInput.updateEditingValue(
        TextEditingValue(
          text: '${base}ka',
          selection: TextSelection.collapsed(offset: base.length + 2),
          composing: TextRange(start: base.length, end: base.length + 2),
        ),
      );
      for (var i = 0; i < 20; i++) {
        await tester.pump(const Duration(milliseconds: 1));
      }
      expect(h.host.input, isEmpty);
      tester.testTextInput.updateEditingValue(
        TextEditingValue(
          text: '$baseか',
          selection: TextSelection.collapsed(offset: base.length + 1),
        ),
      );
      await pumpUntil(tester, () => h.host.input.isNotEmpty);
      expect(utf8.decode(h.host.input), 'か');
      await h.controller.detach();
      await tester.pump(const Duration(seconds: 1));
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('the return action of the keyboard sends return', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    try {
      final h = await _connected(tester);
      await tester.tap(find.byType(TerminalScreenView));
      await tester.pump();
      await tester.testTextInput.receiveAction(TextInputAction.newline);
      await pumpUntil(tester, () => h.host.input.isNotEmpty);
      expect(utf8.decode(h.host.input), '\r');
      await h.controller.detach();
      await tester.pump(const Duration(seconds: 1));
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('a hardware key still reaches the host on a phone', (
    tester,
  ) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    try {
      final h = await _connected(tester);
      await tester.tap(find.byType(TerminalScreenView));
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.keyA, character: 'a');
      await pumpUntil(tester, () => h.host.input.isNotEmpty);
      expect(utf8.decode(h.host.input), 'a');
      await h.controller.detach();
      await tester.pump(const Duration(seconds: 1));
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });
}
