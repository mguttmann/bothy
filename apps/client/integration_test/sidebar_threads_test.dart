// SPDX-License-Identifier: Apache-2.0

// The thread rows of the sidebar against the fake server (brief M4-40 Tests; 01 6.8; 04 6.3
// item 4, R-SY-28): after connect every forum shows its active posts from the snapshot without a
// `listChannelThreads` request; a post that Sam creates appears at the top of its group; a post
// closed on the server leaves after the pointer leaves the sidebar; a run starting in a post
// shows its run icon.

import 'package:client/app/providers/session.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../test/support/until.dart' show testPatience;
import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('posts in the sidebar from the snapshot and kept current', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'server', {
      seed.alex.id: 'trigger',
      seed.sam.id: 'write',
    });
    final first = world.addThread(
      forum,
      'Gateway resume',
      createdBy: seed.sam.id,
    );
    final second = world.addThread(
      forum,
      'Check migration',
      createdBy: seed.sam.id,
    );
    Finder row(String id) => find.byKey(ValueKey('sidebar.thread.$id'));
    double top(Finder f) => tester.getTopLeft(f).dy;
    Future<void> until(String what, bool Function() condition) async {
      final end = DateTime.now().add(testPatience);
      while (!condition()) {
        if (DateTime.now().isAfter(end)) fail('timed out: $what');
        await tester.pump(const Duration(milliseconds: 50));
      }
    }

    try {
      await app.connect();
      await app.waitFor(find.byKey(const ValueKey('e2.loginName')));
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();

      // From the snapshot, without one list request per channel.
      await app.waitFor(row(first));
      await app.waitFor(row(second));
      expect(
        app.fake.server.requestsOf('listChannelThreads'),
        isEmpty,
        reason: 'the sidebar fills from active_threads',
      );
      expect(
        top(row(second)),
        lessThan(top(row(first))),
        reason: 'newest first',
      );

      // Sam creates a post: it appears at the top of the group.
      final created = world.createThread(seed.sam.id, forum, {
        'title': 'Review grant wizard',
        'first_message': {
          'client_msg_id': world.nextId(),
          'content_md': 'Please have a look.',
        },
      });
      final newest = (created['thread']! as Map)['id']! as String;
      await app.waitFor(row(newest));
      expect(top(row(newest)), lessThan(top(row(second))));

      // A run starting in a post shows its run icon in front of the title.
      final trigger = world.createMessage(seed.alex.id, first, {
        'client_msg_id': world.nextId(),
        'content_md': '@robin look into it',
        'mention_ids': [seed.robin.id],
      }).message;
      final run = world.createRun(
        agentId: seed.robin.id,
        triggeringMessageId: trigger['id']! as String,
        triggeredBy: seed.alex.id,
      );
      world.setRunState(
        run['id']! as String,
        'running',
        triggerState: 'working',
      );
      final running = find.descendant(
        of: row(first),
        matching: find.byWidgetPredicate(
          (w) => w is DsIcon && w.glyph == DsIcons.runRunning,
        ),
      );
      await app.waitFor(running);

      // Closed on the server while the pointer is over the sidebar: the row stays until the
      // pointer leaves (01 6.8 item 4).
      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      await mouse.addPointer(location: tester.getCenter(row(second)));
      await tester.pump();
      world.closeThread(second, actor: seed.sam.id);
      // Wait until the app stored the close, then let the sidebar rebuild over it.
      final db = app.container.read(localDatabaseProvider)!;
      final end = DateTime.now().add(testPatience);
      while ((await tester.runAsync(
            () => db.threadsDao.byId(ThreadId.parse(second)),
          ))?.state !=
          ThreadState.closed) {
        if (DateTime.now().isAfter(end)) fail('the close was never stored');
        await tester.pump(const Duration(milliseconds: 50));
      }
      await tester.pump();
      await tester.pump();
      expect(
        row(second),
        findsOneWidget,
        reason: 'no row leaves under the pointer',
      );
      await mouse.moveTo(const Offset(1400, 980));
      await until(
        'the closed post leaves',
        () => row(second).evaluate().isEmpty,
      );
      await mouse.removePointer();
    } finally {
      await app.stop();
    }
  });
}
