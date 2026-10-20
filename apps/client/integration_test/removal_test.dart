// SPDX-License-Identifier: Apache-2.0

// Access removal while a thread is open (brief M4-20 Tests; 01 6.6; 04 8.3, 16.4 through the
// interface): the fake removes Alex's access to the forum, the view closes with "No access
// anymore" and the waiting message remains as text to copy.

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../test/support/fakes.dart';
import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('removal closes the view and keeps the waiting text', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'ideas', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    final thread = world.addThread(
      forum,
      'Printer jam',
      createdBy: seed.sam.id,
    );
    final first = seed.post(seed.sam, thread, 'The printer stops.');
    Future<void> until(String what, bool Function() condition) async {
      final end = DateTime.now().add(const Duration(seconds: 20));
      while (!condition()) {
        if (DateTime.now().isAfter(end)) fail('timed out: $what');
        await tester.pump(const Duration(milliseconds: 50));
      }
    }

    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      await app.waitFor(find.byKey(ValueKey('sidebar.channel.$forum')));
      await tester.tap(find.byKey(ValueKey('sidebar.channel.$forum')));
      await app.waitFor(find.byKey(ValueKey('postList.post.$thread')));
      await tester.tap(find.byKey(ValueKey('postList.post.$thread')));
      await app.waitFor(find.byKey(ValueKey('message.${first['id']}')));

      // The message waits: the device is offline.
      (app.fake.ports.network as SwitchNetwork).set(false);
      app.fake.server.disconnect(deviceId: app.deviceId);
      final field = find.descendant(
        of: find.byKey(const ValueKey('composer')),
        matching: find.byType(TextField),
      );
      await tester.enterText(field, 'Please keep this text');
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await until('waiting', () => find.text('Sending').evaluate().isNotEmpty);

      // The access goes; the device learns it on the reconnect.
      world.setAccess(seed.alex.id, forum, null);
      (app.fake.ports.network as SwitchNetwork).set(true);
      await until(
        'No access anymore',
        () =>
            find.byKey(const ValueKey('thread.noAccess')).evaluate().isNotEmpty,
      );
      expect(find.byKey(ValueKey('message.${first['id']}')), findsNothing);
      expect(
        find.widgetWithText(SelectableText, 'Please keep this text'),
        findsOneWidget,
      );
      expect(
        world
            .messagesOf(thread)
            .where((m) => m['content_md'] == 'Please keep this text'),
        isEmpty,
      );
    } finally {
      await app.stop();
    }
  });
}
