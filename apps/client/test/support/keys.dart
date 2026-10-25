// SPDX-License-Identifier: Apache-2.0

// Presses the strokes of the shortcut table with their modifiers (brief M4-24 Tests).

import 'package:client/keyboard/bindings.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

/// Holds the modifiers of [stroke], presses its key and releases everything; returns whether
/// the framework handled the key press.
Future<bool> pressStroke(WidgetTester tester, KeyStroke stroke) async {
  final modifiers = [
    if (stroke.meta) LogicalKeyboardKey.metaLeft,
    if (stroke.control) LogicalKeyboardKey.controlLeft,
    if (stroke.alt) LogicalKeyboardKey.altLeft,
    if (stroke.shift) LogicalKeyboardKey.shiftLeft,
  ];
  for (final m in modifiers) {
    await tester.sendKeyDownEvent(m);
  }
  final handled = await tester.sendKeyDownEvent(stroke.key!);
  await tester.sendKeyUpEvent(stroke.key!);
  for (final m in modifiers.reversed) {
    await tester.sendKeyUpEvent(m);
  }
  await tester.pump();
  return handled;
}

/// The single-stroke app bindings of [platform] with their actions.
List<(ShortcutBinding, KeyStroke)> appStrokes(ShortcutPlatform platform) => [
  for (final b in allShortcutBindings)
    if (b.kind == ShortcutKind.app)
      for (final strokes in b.on(platform))
        if (strokes.length == 1) (b, strokes.single),
];
