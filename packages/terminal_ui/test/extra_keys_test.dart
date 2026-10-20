// SPDX-License-Identifier: Apache-2.0

// The extra keys bar (01 screen M8, 24 12, components.md 12): every key, Ctrl and Alt as
// one-shot (tap) and held (long press) modifiers, and the labels for screen readers.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:xterm/xterm.dart' show Terminal;

import 'support/widgets.dart';

void main() {
  late List<String> out;
  late Terminal terminal;
  late ExtraKeysController keys;

  setUp(() {
    out = [];
    keys = ExtraKeysController();
    terminal = Terminal(onOutput: (d) => out.add(keys.transform(d)));
  });

  Future<void> pumpBar(WidgetTester tester) => pumpApp(
    tester,
    Align(
      alignment: Alignment.bottomCenter,
      child: ExtraKeysBar(keys: keys, terminal: () => terminal),
    ),
  );

  testWidgets('each key sends its sequence', (tester) async {
    await pumpBar(tester);
    for (final label in [
      'Esc',
      'Tab',
      '↑',
      '↓',
      '→',
      '←',
      '|',
      '~',
      '/',
      '-',
    ]) {
      await tester.tap(find.text(label));
    }
    expect(out, [
      '\x1b',
      '\t',
      '\x1b[A',
      '\x1b[B',
      '\x1b[C',
      '\x1b[D',
      '|',
      '~',
      '/',
      '-',
    ]);
  });

  testWidgets('Ctrl one-shot applies to the next key only', (tester) async {
    await pumpBar(tester);
    await tester.tap(find.text('Ctrl'));
    await tester.pump();
    expect(keys.ctrl, ModifierLatch.oneShot);
    expect(find.bySemanticsLabel('Control, latched'), findsOneWidget);
    terminal.textInput('c');
    terminal.textInput('c');
    expect(out, ['\x03', 'c']);
    await tester.pump();
    expect(keys.ctrl, ModifierLatch.off);
    expect(find.bySemanticsLabel('Control'), findsOneWidget);
  });

  testWidgets(
    'Ctrl held stays until tapped again; it also applies to the arrows',
    (tester) async {
      await pumpBar(tester);
      await tester.longPress(find.text('Ctrl'));
      await tester.pump();
      expect(keys.ctrl, ModifierLatch.held);
      expect(find.bySemanticsLabel('Control, held'), findsOneWidget);
      terminal.textInput('a');
      terminal.textInput('d');
      await tester.tap(find.text('←'));
      expect(out.take(2), ['\x01', '\x04']);
      expect(out.last, '\x1b[1;5D');
      await tester.tap(find.text('Ctrl'));
      await tester.pump();
      expect(keys.ctrl, ModifierLatch.off);
      terminal.textInput('a');
      expect(out.last, 'a');
    },
  );

  testWidgets('Alt one-shot puts ESC in front; a paste is never changed', (
    tester,
  ) async {
    await pumpBar(tester);
    await tester.tap(find.text('Alt'));
    await tester.pump();
    expect(find.bySemanticsLabel('Alt, latched'), findsOneWidget);
    terminal.textInput('b');
    expect(out.single, '\x1bb');
    await tester.tap(find.text('Alt'));
    terminal.textInput('two words');
    expect(out.last, 'two words');
    expect(keys.alt, ModifierLatch.oneShot);
  });

  testWidgets('Esc with Alt latched is sent once with Alt, not twice', (
    tester,
  ) async {
    await pumpBar(tester);
    await tester.tap(find.text('Alt'));
    await tester.tap(find.text('Esc'));
    expect(out, hasLength(1));
    expect(keys.alt, ModifierLatch.off);
  });

  testWidgets('the keys carry their names for screen readers', (tester) async {
    await pumpBar(tester);
    for (final name in [
      'Escape',
      'Tab',
      'Control',
      'Alt',
      'Arrow up',
      'Arrow down',
      'Arrow left',
      'Arrow right',
      'Vertical bar',
      'Tilde',
      'Slash',
      'Minus',
    ]) {
      expect(find.bySemanticsLabel(name), findsOneWidget, reason: name);
    }
    expect(find.bySemanticsLabel('Extra keys bar'), findsOneWidget);
  });
}
