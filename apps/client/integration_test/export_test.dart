// SPDX-License-Identifier: Apache-2.0

// The channel export (brief M4-33 Tests; 07 2.4; 05 `export.channel`): with the capability
// `export` the channel menu offers "Export history", the step-up dialog appears, and the
// chosen file holds the server's stream; without the capability the menu has no entry.

import 'dart:convert';
import 'dart:io';

import 'package:client/files/file_ports.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import '../test/support/file_fakes.dart';
import 'support/test_app.dart';

/// The patience of the repository's test kits: the only upper bound of a wait.
const Duration patience = Duration(minutes: 3);

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  Future<void> openMenu(
    TestApp app,
    WidgetTester tester,
    String channel,
  ) async {
    await app.waitFor(
      find.byKey(ValueKey('sidebar.channel.$channel')),
      timeout: patience,
    );
    await tester.tap(
      find.byKey(ValueKey('sidebar.channel.$channel')),
      buttons: kSecondaryMouseButton,
    );
    await tester.pumpAndSettle();
  }

  testWidgets('export with capability and step-up; none without', (
    tester,
  ) async {
    final picker = FakePicker();
    final out = Directory.systemTemp.createTempSync('export');
    picker.saveTo = '${out.path}/general.ndjson';
    final app = await TestApp.start(tester, files: FilePorts(picker: picker));
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    try {
      final message = seed.post(seed.sam, seed.releaseNotes, 'Hello');
      world.grantCapability(seed.alex.id, seed.general, 'export');
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(
        find.byKey(const ValueKey('column.sidebar')),
        timeout: patience,
      );
      await app.waitLive();

      // `support` has no capability export: no entry.
      await openMenu(app, tester, seed.support);
      expect(find.byKey(const ValueKey('menu.export')), findsNothing);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();

      await openMenu(app, tester, seed.general);
      await tester.tap(find.byKey(const ValueKey('menu.export')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('export.start')));
      await app.waitFor(
        find.byKey(const ValueKey('stepUp.dialog')),
        timeout: patience,
      );
      await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
      await app.waitFor(
        find.byKey(const ValueKey('export.close')),
        timeout: patience,
      );
      expect(picker.asked.single, startsWith('general-'));
      final lines = const LineSplitter().convert(
        File(picker.saveTo!).readAsStringSync(),
      );
      expect(
        [for (final l in lines) (jsonDecode(l) as Map)['id']],
        [message['id']],
      );
      expect(app.fake.server.requestsOf('exportChannel').last.status, 200);
      await tester.tap(find.byKey(const ValueKey('export.close')));
      await tester.pumpAndSettle();
    } finally {
      await app.stop();
      out.deleteSync(recursive: true);
    }
  });
}
