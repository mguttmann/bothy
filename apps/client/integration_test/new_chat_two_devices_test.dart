// SPDX-License-Identifier: Apache-2.0

// The draft of a new post across two simulated devices of Alex (brief M4-41 Tests; 04 14; 03
// `drafts` with `thread_id` NULL): a draft written on the second device appears as the draft
// row of the forum in the app and opens with its text; what the app types there reaches the
// second device, which shows its own draft row.

import 'package:client/forum/providers.dart' show channelDraftProvider;
import 'package:client/sidebar/providers.dart' show newChatRowsProvider;
import 'package:client/sync/thread_ops.dart';
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../test/support/until.dart' show testPatience;
import 'support/new_chat.dart';
import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pumpUntil(
    WidgetTester tester,
    String what,
    bool Function() condition,
  ) async {
    final end = DateTime.now().add(testPatience);
    while (!condition()) {
      if (DateTime.now().isAfter(end)) fail('timed out: $what');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  testWidgets('a draft typed on one device is a draft row on the other', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'server', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    world.setChannelOperator(forum, seed.robin.id);
    final second = await app.secondApp();
    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      await app.waitFor(find.byKey(ValueKey('sidebar.channel.$forum')));
      second.container
        ..listen(channelDraftProvider(forum), (_, _) {})
        ..listen(newChatRowsProvider(forum), (_, _) {});
      await second.signIn();
      expect(draftRow(forum), findsNothing);

      // The second device writes a draft of a new post through its outbox.
      await tester.runAsync(
        () => second.state.services!.engine.outbox.enqueue(
          ThreadOps.channelDraftRequest(
            forum: ChannelId.parse(forum),
            contentMd: 'Check the gateway logs',
          ),
        ),
      );
      await app.waitFor(draftRow(forum));
      expect(
        world.drafts.values.where((d) => d['channel_id'] == forum),
        hasLength(1),
      );

      // The row opens the new chat view with that text.
      await tester.tap(draftRow(forum));
      await app.waitFor(newChatField);
      await pumpUntil(
        tester,
        'the text of the other device in the composer',
        () =>
            tester.widget<EditableText>(newChatField).controller.text ==
            'Check the gateway logs',
      );

      // What the app types reaches the second device, which shows the draft row too.
      await tester.enterText(newChatField, 'Check the gateway logs of Tuesday');
      await tester.pump();
      await eventually('the second device has the new text', () async {
        await tester.pump(const Duration(milliseconds: 50));
        return second.container
                .read(channelDraftProvider(forum))
                .value
                ?.contentMd ==
            'Check the gateway logs of Tuesday';
      });
      expect(
        second.container
            .read(newChatRowsProvider(forum))
            .where((r) => r.isDraft),
        hasLength(1),
      );
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await second.stop();
      await app.stop();
    }
  });
}
