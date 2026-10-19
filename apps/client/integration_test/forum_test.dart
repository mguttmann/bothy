// SPDX-License-Identifier: Apache-2.0

// A forum post from "New post" to Done and back (brief M4-19 Tests; 01 6.1): the post shows
// at the top, closing it on the server moves it into the collapsed Done section, and a new
// message opens it again.

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  Future<void> until(
    WidgetTester tester,
    String what,
    bool Function() condition,
  ) async {
    final end = DateTime.now().add(const Duration(seconds: 20));
    while (!condition()) {
      if (DateTime.now().isAfter(end)) fail('timed out: $what');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  testWidgets('create a post, close it to Done, reopen it by a message', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'ideas', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    final older = world.addThread(forum, 'Older post', createdBy: seed.sam.id);
    world.addForumTag(forum, 'idea', color: 'spruce');
    try {
      await app.connect();
      await app.waitFor(find.byKey(const ValueKey('e2.loginName')));
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();

      await app.waitFor(find.byKey(ValueKey('sidebar.channel.$forum')));
      await tester.tap(find.byKey(ValueKey('sidebar.channel.$forum')));
      await app.waitFor(find.byKey(ValueKey('postList.post.$older')));

      await tester.tap(find.byKey(const ValueKey('postList.newPost')));
      await app.waitFor(find.byKey(const ValueKey('newPost.dialog')));
      await tester.enterText(
        find.byKey(const ValueKey('newPost.title')),
        'Printer jam on floor 2',
      );
      await tester.enterText(
        find.byKey(const ValueKey('newPost.message')),
        'The printer stops after one page.',
      );
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('newPost.create')));
      await until(
        tester,
        'the post on the server',
        () => world.threads.values.any(
          (t) => t['title'] == 'Printer jam on floor 2',
        ),
      );
      final created =
          world.threads.values.singleWhere(
                (t) => t['title'] == 'Printer jam on floor 2',
              )['id']!
              as String;
      final row = find.byKey(ValueKey('postList.post.$created'));
      final olderRow = find.byKey(ValueKey('postList.post.$older'));
      await app.waitFor(row);
      expect(
        tester.getTopLeft(row).dy,
        lessThan(tester.getTopLeft(olderRow).dy),
        reason: 'the new post at the top',
      );

      // Closed on the server: it moves into the Done section, collapsed by default.
      world.closeThread(created, actor: seed.sam.id);
      await until(tester, 'moved to Done', () => row.evaluate().isEmpty);
      await tester.tap(find.byKey(const ValueKey('postList.done')));
      await app.waitFor(row);
      expect(
        tester.getTopLeft(row).dy,
        greaterThan(
          tester.getTopLeft(find.byKey(const ValueKey('postList.done'))).dy,
        ),
      );

      // A new message opens it again (01 6.1).
      seed.post(seed.sam, created, 'It happens again.');
      await until(
        tester,
        'open again',
        () =>
            row.evaluate().isNotEmpty &&
            tester.getTopLeft(row).dy <
                tester
                    .getTopLeft(find.byKey(const ValueKey('postList.done')))
                    .dy,
      );
    } finally {
      await app.stop();
    }
  });
}
