// SPDX-License-Identifier: Apache-2.0

// The extra keys bar (01 screen M8, R-PR-11, 24 12, components.md 12; brief M5-12 Design note 2):
// every key, Ctrl and Alt as one-shot (one tap) and latched (a second tap within the double-tap
// interval, or a long press) modifiers, the arrows in normal and application cursor mode, the
// labels for screen readers, and a hardware keyboard that hides the bar.

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:xterm/xterm.dart' show Terminal;

import '../support/fakes.dart';
import '../support/widgets.dart';

void main() {
  late List<String> out;
  late Terminal terminal;
  late FakeClock clock;
  late ExtraKeysController keys;

  setUp(() {
    out = [];
    clock = FakeClock();
    keys = ExtraKeysController(clock: clock);
    terminal = Terminal(onOutput: (d) => out.add(keys.transform(d)));
  });

  Future<void> pumpBar(WidgetTester tester) => pumpApp(
    tester,
    Align(
      alignment: Alignment.bottomCenter,
      child: ExtraKeysBar(keys: keys, terminal: () => terminal),
    ),
  );

  // A tap later than the double-tap interval after the one before.
  void later() => clock.current = clock.current.add(
    ExtraKeysController.doubleTapInterval + const Duration(milliseconds: 1),
  );

  group('key mapping', () {
    test('Ctrl turns a letter or one of @[\\]^_ into its control byte', () {
      expect(controlByte('c'), '\x03');
      expect(controlByte('C'), '\x03');
      expect(controlByte('d'), '\x04');
      expect(controlByte('@'), '\x00');
      expect(controlByte('['), '\x1b');
      expect(controlByte('\\'), '\x1c');
      expect(controlByte(']'), '\x1d');
      expect(controlByte('^'), '\x1e');
      expect(controlByte('_'), '\x1f');
    });

    test('Ctrl leaves every other character unmodified', () {
      for (final c in ['1', '-', '/', '|', '~', ' ', '?', 'é', '\r', '\x7f']) {
        expect(controlByte(c), c, reason: c);
      }
    });

    test('the arrows follow the cursor key mode of the renderer', () {
      final normal = [
        for (final k in [
          ExtraKey.up,
          ExtraKey.down,
          ExtraKey.right,
          ExtraKey.left,
        ])
          extraKeySequence(k, applicationCursor: false),
      ];
      expect(normal, ['\x1b[A', '\x1b[B', '\x1b[C', '\x1b[D']);
      final application = [
        for (final k in [
          ExtraKey.up,
          ExtraKey.down,
          ExtraKey.right,
          ExtraKey.left,
        ])
          extraKeySequence(k, applicationCursor: true),
      ];
      expect(application, ['\x1bOA', '\x1bOB', '\x1bOC', '\x1bOD']);
    });

    test('Alt puts ESC in front of every key of the bar', () {
      expect(
        extraKeySequence(ExtraKey.up, applicationCursor: false, alt: true),
        '\x1b\x1b[A',
      );
      expect(
        extraKeySequence(ExtraKey.pipe, applicationCursor: false, alt: true),
        '\x1b|',
      );
    });
  });

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

  testWidgets('the arrows send ESC O in application cursor mode', (
    tester,
  ) async {
    // DECCKM on, as a full-screen program sets it.
    terminal.write('\x1b[?1h');
    await pumpBar(tester);
    await tester.tap(find.text('↑'));
    await tester.tap(find.text('←'));
    expect(out, ['\x1bOA', '\x1bOD']);
  });

  testWidgets('Ctrl one-shot then c sends 0x03 and clears', (tester) async {
    await pumpBar(tester);
    await tester.tap(find.text('Ctrl'));
    await tester.pump();
    expect(keys.ctrl, ModifierLatch.oneShot);
    expect(find.bySemanticsLabel('Control, for the next key'), findsOneWidget);
    terminal.textInput('c');
    terminal.textInput('c');
    expect(out, ['\x03', 'c']);
    await tester.pump();
    expect(keys.ctrl, ModifierLatch.off);
    expect(find.bySemanticsLabel('Control'), findsOneWidget);
  });

  testWidgets(
    'Ctrl latched by a second tap: c, d send 0x03, 0x04 and it stays latched until tapped',
    (tester) async {
      await pumpBar(tester);
      await tester.tap(find.text('Ctrl'));
      await tester.tap(find.text('Ctrl'));
      await tester.pump();
      expect(keys.ctrl, ModifierLatch.latched);
      expect(find.bySemanticsLabel('Control, latched'), findsOneWidget);
      terminal.textInput('c');
      terminal.textInput('d');
      expect(out, ['\x03', '\x04']);
      expect(keys.ctrl, ModifierLatch.latched);
      later();
      await tester.tap(find.text('Ctrl'));
      await tester.pump();
      expect(keys.ctrl, ModifierLatch.off);
      terminal.textInput('a');
      expect(out.last, 'a');
    },
  );

  testWidgets('a second tap after the double-tap interval turns Ctrl off', (
    tester,
  ) async {
    await pumpBar(tester);
    await tester.tap(find.text('Ctrl'));
    later();
    await tester.tap(find.text('Ctrl'));
    await tester.pump();
    expect(keys.ctrl, ModifierLatch.off);
  });

  testWidgets('a long press latches Ctrl as well', (tester) async {
    await pumpBar(tester);
    await tester.longPress(find.text('Ctrl'));
    await tester.pump();
    expect(keys.ctrl, ModifierLatch.latched);
    expect(find.bySemanticsLabel('Control, latched'), findsOneWidget);
  });

  testWidgets(
    'Ctrl with a key of the bar sends the key unmodified and clears one-shot',
    (tester) async {
      await pumpBar(tester);
      await tester.tap(find.text('Ctrl'));
      await tester.tap(find.text('←'));
      await tester.pump();
      expect(out, ['\x1b[D']);
      expect(keys.ctrl, ModifierLatch.off);
      // Latched Ctrl stays through a key of the bar.
      later();
      await tester.tap(find.text('Ctrl'));
      await tester.tap(find.text('Ctrl'));
      await tester.tap(find.text('-'));
      expect(out.last, '-');
      expect(keys.ctrl, ModifierLatch.latched);
    },
  );

  testWidgets('Alt then b sends ESC b; a paste is never changed', (
    tester,
  ) async {
    await pumpBar(tester);
    await tester.tap(find.text('Alt'));
    await tester.pump();
    expect(find.bySemanticsLabel('Alt, for the next key'), findsOneWidget);
    terminal.textInput('b');
    expect(out.single, '\x1bb');
    later();
    await tester.tap(find.text('Alt'));
    terminal.textInput('two words');
    expect(out.last, 'two words');
    expect(keys.alt, ModifierLatch.oneShot);
  });

  testWidgets('Alt with an arrow of the bar puts ESC in front once', (
    tester,
  ) async {
    await pumpBar(tester);
    await tester.tap(find.text('Alt'));
    await tester.tap(find.text('↑'));
    expect(out, ['\x1b\x1b[A']);
    expect(keys.alt, ModifierLatch.off);
  });

  testWidgets('the characters of the bar send themselves', (tester) async {
    await pumpBar(tester);
    for (final c in ['|', '~', '/', '-']) {
      await tester.tap(find.text(c));
    }
    expect(out, ['|', '~', '/', '-']);
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

  group('hardware keyboard', () {
    testWidgets('a hardware keyboard event hides the bar of the pane', (
      tester,
    ) async {
      final probe = _Probe();
      final watch = HardwareKeyboardWatch(probe);
      addTearDown(watch.dispose);
      await watch.refresh();
      final h = (await tester.runAsync(Harness.create))!;
      await pumpApp(
        tester,
        TerminalPane(
          controller: h.controller,
          fontSize: 14,
          showExtraKeys: true,
          hardwareKeyboard: watch,
        ),
      );
      expect(find.byType(ExtraKeysBar), findsOneWidget);
      probe.value = true;
      await tester.sendKeyEvent(LogicalKeyboardKey.keyA);
      await tester.pump();
      expect(watch.connected, isTrue);
      expect(find.byType(ExtraKeysBar), findsNothing);
      // Disconnected: the next refresh (a return to the app, a change of the window) shows
      // the bar again.
      probe.value = false;
      await watch.refresh();
      await tester.pump();
      expect(find.byType(ExtraKeysBar), findsOneWidget);
      h.controller.dispose();
    });

    test('a failed probe keeps the last answer', () async {
      final probe = _Probe()..value = true;
      final watch = HardwareKeyboardWatch(probe);
      addTearDown(watch.dispose);
      await watch.refresh();
      probe.fail = true;
      await watch.refresh();
      expect(watch.connected, isTrue);
    });
  });
}

final class _Probe implements HardwareKeyboardProbe {
  bool value = false;
  bool fail = false;

  @override
  Future<bool> connected() async {
    if (fail) throw StateError('no runner');
    return value;
  }
}
