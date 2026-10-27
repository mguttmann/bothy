// SPDX-License-Identifier: Apache-2.0

// Every page of the phone router has a row in the checklist (brief M5-10 Tests): the table
// "Routes of the phone" of docs/quality/accessibility-mobile.md names every value of
// PhonePageKind and a section of the checklist that exists, so a new kind of page without a row
// fails here.

import 'dart:io';

import 'package:client/phone/phone_navigation.dart';
import 'package:flutter_test/flutter_test.dart';

/// The checklist, from the package folder `apps/client` where `flutter test` runs.
final File checklist = File('../../docs/quality/accessibility-mobile.md');

/// The rows of the table under the heading "Routes of the phone": route kind to section text.
Map<String, String> routeRows(String markdown) {
  final lines = markdown.split('\n');
  final start = lines.indexOf('## Routes of the phone');
  expect(start, isNot(-1), reason: 'no section "Routes of the phone"');
  final rows = <String, String>{};
  for (final line in lines.skip(start + 1)) {
    if (line.startsWith('## ')) break;
    final match = RegExp(r'^\| `PhonePageKind\.(\w+)` \|[^|]*\| (.+) \|$')
        .firstMatch(line);
    if (match != null) rows[match.group(1)!] = match.group(2)!;
  }
  return rows;
}

void main() {
  test('every kind of page of the phone router has a row in the checklist', () {
    final markdown = checklist.readAsStringSync();
    final rows = routeRows(markdown);
    final headings = {
      for (final m in RegExp(
        r'^## (.+)$',
        multiLine: true,
      ).allMatches(markdown))
        m.group(1)!,
    };
    for (final kind in PhonePageKind.values) {
      expect(rows.keys, contains(kind.name), reason: 'no row for $kind');
      // A row may name more than one section, separated by ";", each with a note after a
      // parenthesis.
      for (final section in rows[kind.name]!.split(';')) {
        final name = section.split('(').first.trim();
        expect(
          headings,
          contains(name),
          reason: '$kind names the section "$name", which the checklist lacks',
        );
      }
    }
    expect(
      rows.keys.toSet().difference({
        for (final k in PhonePageKind.values) k.name,
      }),
      isEmpty,
      reason: 'a row names a kind the router does not have',
    );
  });

  test('the parser reads the route rows of a table', () {
    const markdown = '''
## Routes of the phone

| Route kind | Path | Section |
|---|---|---|
| `PhonePageKind.spaces` | `/` | M1 Spaces |
| `PhonePageKind.channel` | `/c/<id>` | M2 Post list (forum); M3 History (text channel) |

## M1 Spaces
''';
    expect(routeRows(markdown), {
      'spaces': 'M1 Spaces',
      'channel': 'M2 Post list (forum); M3 History (text channel)',
    });
  });
}
