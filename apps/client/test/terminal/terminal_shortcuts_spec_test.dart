// SPDX-License-Identifier: Apache-2.0

// The table "In the terminal" as data equals the second table of docs/spec/01-product.md 7 on
// both platforms, and every stroke of it carries the platform modifier: Cmd on macOS,
// Ctrl+Shift on Windows (R-PR-10; brief M4-36 Design note 4).

import 'dart:io';

import 'package:client/keyboard/bindings.dart';
import 'package:client/terminal/terminal_shortcuts.dart';
import 'package:flutter_test/flutter_test.dart';

/// The rows of the table "In the terminal" of 01 7, read by walking up to the repository root.
List<List<String>> specRows() {
  var dir = Directory.current.absolute;
  while (!File('${dir.path}/docs/spec/01-product.md').existsSync()) {
    final parent = dir.parent;
    if (parent.path == dir.path) fail('docs/spec/01-product.md not found');
    dir = parent;
  }
  final text = File('${dir.path}/docs/spec/01-product.md').readAsStringSync();
  final start = text.indexOf('| Action in the terminal |');
  final end = text.indexOf('\n## 8.', start);
  expect(start, isNonNegative);
  expect(end, greaterThan(start));
  return [
    for (final line in text.substring(start, end).split('\n'))
      if (line.startsWith('|'))
        [for (final c in line.split('|')) c.trim()]
            .sublist(1, line.split('|').length - 1),
  ].sublist(2);
}

void main() {
  test('every row equals the table "In the terminal" of 01 7', () {
    final spec = specRows();
    expect(spec, hasLength(terminalShortcutRows.length));
    for (var i = 0; i < spec.length; i++) {
      final row = terminalShortcutRows[i];
      expect(
        [
          row.specLabel,
          describeTerminalRow(row, ShortcutPlatform.mac, specTerminalKeyNames),
          describeTerminalRow(
            row,
            ShortcutPlatform.windows,
            specTerminalKeyNames,
          ),
        ],
        spec[i],
        reason: 'row ${i + 1} of the table',
      );
    }
  });

  test('every stroke carries the platform modifier, so plain keys stay with the shell', () {
    for (final row in terminalShortcutRows) {
      for (final b in row.bindings) {
        expect(b.mac.meta, isTrue, reason: '${b.action} on macOS');
        expect(b.mac.control || b.mac.alt, isFalse, reason: '${b.action}');
        expect(
          b.windows.control && b.windows.shift,
          isTrue,
          reason: '${b.action} on Windows',
        );
        expect(b.windows.alt || b.windows.meta, isFalse, reason: '${b.action}');
      }
    }
  });

  test('copy and paste stay with the renderer and its paste guard', () {
    for (final platform in ShortcutPlatform.values) {
      final intents = terminalActivators(platform).values
          .whereType<TerminalShortcutIntent>();
      expect(
        intents.map((i) => i.action),
        isNot(
          anyOf(
            contains(TerminalShortcutAction.copy),
            contains(TerminalShortcutAction.paste),
          ),
        ),
      );
    }
  });
}
