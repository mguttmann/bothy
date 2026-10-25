// SPDX-License-Identifier: Apache-2.0

// The class guard of two gaps the desktop checklist found (brief M4-32; docs/decisions.d/
// M4-32.md): Flutter's `MenuItemButton` is no button for desktop screen readers and
// `DropdownMenu` puts an unreachable button below an unnamed text field. The apps use
// `DsMenuItem` and `DsDropdownField` instead; this test fails on a new direct use in the code of
// the client, the administration or the design system.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('menus and drop-down fields come from the design system', () {
    final allowed = {
      'design_system/lib/src/components/ds_menu_item.dart': 'MenuItemButton(',
      'design_system/lib/src/components/ds_dropdown_field.dart':
          'DropdownMenu<',
    };
    final found = <String>[];
    for (final root in const [
      'lib',
      '../../packages/admin_ui/lib',
      '../../packages/design_system/lib',
    ]) {
      for (final file in Directory(root).listSync(recursive: true)) {
        if (file is! File || !file.path.endsWith('.dart')) continue;
        final path = file.uri.path;
        final own = allowed.entries
            .where((e) => path.endsWith(e.key))
            .map((e) => e.value)
            .firstOrNull;
        final lines = file.readAsLinesSync();
        for (var i = 0; i < lines.length; i++) {
          final line = lines[i];
          if (line.trimLeft().startsWith('//')) continue;
          for (final pattern in const [
            'MenuItemButton(',
            'DropdownMenu<',
            'DropdownMenu(',
          ]) {
            if (line.contains(pattern) && own != pattern) {
              found.add('${file.path}:${i + 1}: $pattern');
            }
          }
        }
      }
    }
    expect(found, isEmpty, reason: found.join('\n'));
  });
}
