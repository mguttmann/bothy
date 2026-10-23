// SPDX-License-Identifier: Apache-2.0

// The import page of the console (brief M8-08 Scope): it hosts the import list of admin_ui with
// the browser's file input for the mapping file, opens a source at /import/<id>, and the picker
// asks only for YAML files.

import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';
import 'admin_pages_test.dart' show browserWindow, signedIn;

const String _source = '0192f4a0-7b1c-78d2-8a6a-ec465435b8a0';

Map<String, Object?> _sourceJson() => {
  'id': _source,
  'kind': 'discord',
  'name': 'Old community server',
  'config': {'version': 1, 'guild_id': '100', 'channels': <Object>[]},
  'state': 'draft',
  'stats': <String, Object?>{},
  'created_at': '2026-10-17T08:00:00.000Z',
};

void main() {
  testWidgets('the import page lists the sources and opens one', (
    tester,
  ) async {
    browserWindow(tester);
    final b = signedIn();
    b.server.routes['GET /api/v1/imports'] = (_) => Answer(200, {
      'items': [_sourceJson()],
    });
    b.server.routes['GET /api/v1/imports/$_source'] = (_) =>
        Answer(200, _sourceJson());
    b.server.routes['GET /api/v1/imports/$_source/items'] = (_) =>
        const Answer(200, {'items': <Object>[]});
    b.server.routes['GET /api/v1/agents'] = (_) =>
        const Answer(200, {'items': <Object>[]});

    final c = await pumpConsole(tester, b, location: '/import');
    expect(c.page, ConsolePage.dataImport);
    expect(find.byType(ImportListPage), findsOneWidget);
    expect(find.text('Old community server'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('import.row.$_source')));
    await tester.pumpAndSettle();
    expect(c.path, '/import/$_source');
    expect(find.byType(ImportSourcePage), findsOneWidget);
    expect(find.byKey(const ValueKey('import.action.start')), findsOneWidget);
  });

  test(
    'the mapping file comes from the browser file input with YAML types',
    () async {
      final files = FakeFileChooser()
        ..next = ChosenFile('mapping.yaml', Uint8List.fromList([0x61]));
      final picked = await mappingPickerOf(files)();
      expect(picked!.name, 'mapping.yaml');
      expect(files.choices.single, (mappingAccept, 'mapping'));
      expect(mappingAccept, contains('.yaml'));
    },
  );
}
