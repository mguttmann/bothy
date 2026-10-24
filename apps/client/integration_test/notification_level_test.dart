// SPDX-License-Identifier: Apache-2.0

// The level takes effect according to 13 3.1 (brief M4-33 Tests): Alex mutes the forum `ideas`
// from its menu in the sidebar, the fake's decision of M4-30 (`FakeWorld.notify`, which
// follows 13 3.1) sends no `notification.created` for a message of Sam there; then Alex sets
// the post to All from its menu in the post list and the next message notifies.
// The changes reach a second simulated device of Alex (04 14).

import 'package:client/sidebar/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

/// The patience of the repository's test kits: the only upper bound of a wait.
const Duration patience = Duration(minutes: 3);

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  Future<void> until(
    WidgetTester tester,
    String what,
    bool Function() condition,
  ) async {
    final watch = Stopwatch()..start();
    while (!condition()) {
      if (watch.elapsed > patience) fail('timed out: $what');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  testWidgets('mute a channel, then follow a thread at All; both devices', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    final second = await app.secondApp();
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'ideas', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    final post = world.addThread(forum, 'Printer', createdBy: seed.sam.id);
    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(
        find.byKey(const ValueKey('column.sidebar')),
        timeout: patience,
      );
      await app.waitLive();
      second.container.listen(notificationPreferencesProvider, (_, _) {});
      await second.signIn();
      world.notifyOnMessages = true;

      NotificationPreference? onSecond(String kind, String id) => second
          .container
          .read(notificationPreferencesProvider)
          .value
          ?.where((p) => p.targetKind.code == kind && p.targetId == id)
          .firstOrNull;

      // Mute the forum from the menu of its sidebar row.
      await app.waitFor(
        find.byKey(ValueKey('sidebar.channel.$forum')),
        timeout: patience,
      );
      await tester.tap(
        find.byKey(ValueKey('sidebar.channel.$forum')),
        buttons: kSecondaryMouseButton,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('menu.notifications')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('menu.level.mute')));
      await tester.pumpAndSettle();
      await until(
        tester,
        'the server has the channel muted',
        () => world.effectiveLevel(seed.alex.id, post) == 'mute',
      );
      await until(
        tester,
        'the second device has the row',
        () => onSecond('channel', forum)?.level == NotificationLevel.mute,
      );

      final created = world.notifications.length;
      seed.post(seed.sam, post, 'Muted here');
      expect(world.notifications, hasLength(created));
      expect(
        world.notificationSkips.lastWhere((s) => s.userId == seed.alex.id),
        (userId: seed.alex.id, reason: 'level'),
      );

      // The post at All from its menu in the post list (13 3.1 step 1 wins over the channel).
      await tester.tap(find.byKey(ValueKey('sidebar.channel.$forum')));
      await app.waitFor(
        find.byKey(ValueKey('postList.post.$post')),
        timeout: patience,
      );
      await tester.tap(
        find.byKey(ValueKey('postList.post.$post')),
        buttons: kSecondaryMouseButton,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('menu.notifications')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('menu.level.all')));
      await tester.pumpAndSettle();
      await until(
        tester,
        'the server has the thread at all',
        () => world.effectiveLevel(seed.alex.id, post) == 'all',
      );
      await until(
        tester,
        'the second device has the thread row',
        () => onSecond('thread', post)?.level == NotificationLevel.all,
      );

      seed.post(seed.sam, post, 'Now it notifies');
      expect(world.notifications, hasLength(created + 1));
      expect(world.notifications.last.recipient, seed.alex.id);
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await second.stop();
      await app.stop();
    }
  });
}
