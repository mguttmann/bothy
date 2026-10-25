// SPDX-License-Identifier: Apache-2.0

// The shortcut table of apps/client/README.md equals `bindings.dart` (brief M4-24 Scope).

import 'dart:io';

import 'package:client/keyboard/bindings.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('the README table is the table of bindings.dart', () {
    final readme = File('README.md').readAsStringSync();
    final start = readme.indexOf('## Keyboard shortcuts and focus');
    final end = readme.indexOf('\n## ', start + 1);
    expect(start, isNonNegative);
    final lines = readme
        .substring(start, end)
        .split('\n')
        .where((l) => l.startsWith('|'))
        .toList();
    expect(lines.sublist(2), [
      for (final row in shortcutRows)
        '| ${row.specLabel} | '
            '${describeRow(row, ShortcutPlatform.mac, specKeyNames)} | '
            '${describeRow(row, ShortcutPlatform.windows, specKeyNames)} |',
    ]);
  });
}
