// SPDX-License-Identifier: Apache-2.0

// The keys in the terminal (01 7 "In the terminal", R-PR-10; brief M4-36 Design note 4):
// Ctrl+C, Ctrl+D, Ctrl+K, Ctrl+L, Alt+F and Esc reach the host as `data` frames, and the
// shortcuts of the table work only with the platform modifier: Ctrl+Shift on Windows, Cmd on
// macOS.

import 'dart:convert';

import 'package:client/keyboard/bindings.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support.dart';

Future<void> _press(
  WidgetTester tester,
  LogicalKeyboardKey key, {
  bool control = false,
  bool shift = false,
  bool alt = false,
  bool meta = false,
}) async {
  final mods = [
    if (control) LogicalKeyboardKey.controlLeft,
    if (shift) LogicalKeyboardKey.shiftLeft,
    if (alt) LogicalKeyboardKey.altLeft,
    if (meta) LogicalKeyboardKey.metaLeft,
  ];
  for (final m in mods) {
    await tester.sendKeyDownEvent(m);
  }
  await tester.sendKeyEvent(key);
  for (final m in mods.reversed) {
    await tester.sendKeyUpEvent(m);
  }
  await tester.pump();
}

void main() {
  testWidgets('Ctrl+C, Ctrl+D, Ctrl+K, Ctrl+L, Alt+F and Esc reach the shell', (
    tester,
  ) async {
    final w = await TerminalWorld.create(tester);
    final c = await w.pump(tester);
    final tabs = await openShell(tester, c);
    await tester.tap(find.byType(TerminalScreenView));
    await tester.pump();
    await _press(tester, LogicalKeyboardKey.keyC, control: true);
    await _press(tester, LogicalKeyboardKey.keyD, control: true);
    await _press(tester, LogicalKeyboardKey.keyK, control: true);
    await _press(tester, LogicalKeyboardKey.keyL, control: true);
    await _press(tester, LogicalKeyboardKey.keyF, alt: true);
    await _press(tester, LogicalKeyboardKey.escape);
    await pumpUntil(tester, () => w.host.input.length >= 7);
    // xterm 4.0.0 sends Alt+F as ESC with the capital letter; either way it reaches the shell.
    expect(
      utf8.decode(w.host.input).toLowerCase(),
      '\x03\x04\x0b\x0c\x1bf\x1b',
    );
    // `input` holds the bytes of the host's `data` frames only.
    expect(tabs.current!.state.phase, TerminalPhase.connected);
    await tabs.current!.detach();
    await tester.pump(const Duration(seconds: 1));
  });

  testWidgets(
    'on Windows Ctrl+Shift detaches, sizes the font and opens a session',
    (tester) async {
      final w = await TerminalWorld.create(tester);
      final c = await w.pump(tester);
      final tabs = await openShell(tester, c);
      await tester.tap(find.byType(TerminalScreenView));
      await tester.pump();
      final size = tabs.fontSize;
      await _press(
        tester,
        LogicalKeyboardKey.equal,
        control: true,
        shift: true,
      );
      expect(tabs.fontSize, size + 1);
      await _press(
        tester,
        LogicalKeyboardKey.minus,
        control: true,
        shift: true,
      );
      expect(tabs.fontSize, size);
      await _press(tester, LogicalKeyboardKey.keyF, control: true, shift: true);
      expect(find.byKey(const ValueKey('terminal.search')), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('terminal.search.close')));
      await tester.pump();
      await tester.tap(find.byType(TerminalScreenView));
      await tester.pump();
      await _press(tester, LogicalKeyboardKey.keyD, control: true, shift: true);
      expect(tabs.current!.state.phase, TerminalPhase.detached);
      expect(w.host.input, isEmpty, reason: 'no shortcut reached the shell');
      await _press(tester, LogicalKeyboardKey.keyN, control: true, shift: true);
      await tester.pumpAndSettle();
      expect(find.text(en.terminalOpenTitle), findsOneWidget);
      await tester.tap(find.text(en.terminalCancel));
      await tester.pump(const Duration(seconds: 1));
    },
  );

  testWidgets(
    'on macOS Cmd+Shift+D detaches and Ctrl+C still reaches the shell',
    (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
      addTearDown(() => debugDefaultTargetPlatformOverride = null);
      final w = await TerminalWorld.create(tester);
      final c = await w.pump(tester, platform: ShortcutPlatform.mac);
      final tabs = await openShell(tester, c);
      await tester.tap(find.byType(TerminalScreenView));
      await tester.pump();
      await _press(tester, LogicalKeyboardKey.keyC, control: true);
      await pumpUntil(tester, () => w.host.input.isNotEmpty);
      expect(utf8.decode(w.host.input), '\x03');
      await _press(tester, LogicalKeyboardKey.keyD, meta: true, shift: true);
      expect(tabs.current!.state.phase, TerminalPhase.detached);
      expect(utf8.decode(w.host.input), '\x03');
      await tester.pump(const Duration(seconds: 1));
      debugDefaultTargetPlatformOverride = null;
    },
  );
}
