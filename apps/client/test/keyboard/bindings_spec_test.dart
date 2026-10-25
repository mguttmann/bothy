// SPDX-License-Identifier: Apache-2.0

// The shortcut table as data equals the first table of docs/spec/01-product.md 7 on both
// platforms, and no activator uses a system assignment named above the table (brief M4-24
// Tests; Design note 1; fixed requirement 11).

import 'dart:io';

import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart' show SingleActivator;
import 'package:flutter_test/flutter_test.dart';

/// The rows of the first table of 01 7: the Markdown table between the heading
/// "## 7. Keyboard shortcuts" and the paragraph "**In the terminal**", read by walking up to
/// the repository root.
List<List<String>> specRows() {
  var dir = Directory.current.absolute;
  while (!File('${dir.path}/docs/spec/01-product.md').existsSync()) {
    final parent = dir.parent;
    if (parent.path == dir.path) fail('docs/spec/01-product.md not found');
    dir = parent;
  }
  final text = File('${dir.path}/docs/spec/01-product.md').readAsStringSync();
  final start = text.indexOf('## 7. Keyboard shortcuts');
  final end = text.indexOf('**In the terminal**', start);
  expect(start, isNonNegative);
  expect(end, greaterThan(start));
  final rows = <List<String>>[];
  for (final line in text.substring(start, end).split('\n')) {
    if (!line.startsWith('|')) continue;
    final cells = line
        .split('|')
        .map((c) => c.trim())
        .toList()
        .sublist(1, line.split('|').length - 1);
    rows.add(cells);
  }
  // Header and separator row.
  return rows.sublist(2);
}

void main() {
  test('every row equals the table of 01 7 on macOS and Windows', () {
    final spec = specRows();
    expect(spec, hasLength(shortcutRows.length));
    for (var i = 0; i < spec.length; i++) {
      final row = shortcutRows[i];
      expect(
        [
          row.specLabel,
          describeRow(row, ShortcutPlatform.mac, specKeyNames),
          describeRow(row, ShortcutPlatform.windows, specKeyNames),
        ],
        spec[i],
        reason: 'row ${i + 1} of the table',
      );
    }
  });

  test('every action of the table has a binding and an intent', () {
    final bound = {for (final b in allShortcutBindings) b.action};
    expect(bound, ShortcutAction.values.toSet());
    for (final action in ShortcutAction.values) {
      expect(intentOf(action).action, action);
    }
  });

  test(
    'no activator uses a system assignment (Cmd+H, Cmd+M, Cmd+Q, Alt+F4, the '
    'Windows key)',
    () {
      final mac = [
        for (final b in allShortcutBindings)
          for (final strokes in b.mac) ...strokes,
      ];
      final windows = [
        for (final b in allShortcutBindings)
          for (final strokes in b.windows) ...strokes,
      ];
      final forbidden = {
        const KeyStroke(LogicalKeyboardKey.keyH, meta: true),
        const KeyStroke(LogicalKeyboardKey.keyM, meta: true),
        const KeyStroke(LogicalKeyboardKey.keyQ, meta: true),
      };
      for (final s in mac) {
        expect(
          forbidden,
          isNot(contains(s)),
          reason: '$s is a macOS system key',
        );
        expect(s.control, isFalse, reason: '$s: macOS bindings use Cmd');
      }
      for (final s in windows) {
        expect(s.meta, isFalse, reason: '$s uses the Windows key');
        expect(
          s.alt && s.key == LogicalKeyboardKey.f4,
          isFalse,
          reason: '$s is Alt+F4',
        );
      }
    },
  );

  test('the app layer binds no plain key a text field needs', () {
    for (final platform in ShortcutPlatform.values) {
      for (final activator in appActivators(platform).keys) {
        final a = activator as SingleActivator;
        final plain = !a.meta && !a.control && !a.alt;
        if (!plain) continue;
        // Only F6 without a platform modifier: Enter, Shift+Enter, the arrows, Tab and Esc
        // stay with the focused input or the history (01 7, Design note 2).
        expect(a.trigger, LogicalKeyboardKey.f6, reason: '$a on $platform');
      }
    }
  });
}
