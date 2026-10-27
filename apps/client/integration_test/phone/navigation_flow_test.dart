// SPDX-License-Identifier: Apache-2.0

// Navigation on the phone (briefs M5-02 and M5-03 Tests; 01 5.3, 5.4; 15 11): open a space's
// forum, a post and its thread, send a message, go back (the edge swipe on iOS, system back on
// Android), and open a deep link at a message, which builds the stack Spaces, channel, thread.

import 'package:client/phone/phone_navigation.dart';
import 'package:client/phone/phone_shell.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../../test/support/fakes.dart' show ManualDeepLinks;
import 'support/phone_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('forum, post, message, back, deep link', (tester) async {
    await asPhone(() async {
      final app = await startPhone(tester);
      final world = app.fake.server.world;
      final seed = app.fake.seed;
      final forum = world.addForum(seed.space, 'requests', {
        seed.alex.id: 'write',
        seed.sam.id: 'write',
      });
      final thread = world.addThread(
        forum,
        'Gateway: resume after network change',
        createdBy: seed.sam.id,
      );
      final first = seed.post(seed.sam, thread, 'The gateway drops.');
      final other = world.addThread(
        forum,
        'Costs per space',
        createdBy: seed.sam.id,
      );
      final linked = seed.post(seed.sam, other, 'See the table.');
      PhoneShellState shell() =>
          tester.state<PhoneShellState>(find.byType(PhoneShell));
      try {
        await signIn(app);
        await openThread(app, forum, thread, first['id']!);
        expect(shell().pages.map((p) => p.kind), [
          PhonePageKind.spaces,
          PhonePageKind.channel,
          PhonePageKind.thread,
        ]);

        await sendText(tester, 'Please include expired threads.');
        await until(
          tester,
          'message on the server',
          () => world
              .messagesOf(thread)
              .any((m) => m['content_md'] == 'Please include expired threads.'),
        );

        // Back goes to the post list (01 5.3, 5.4): on iOS a finger from the left edge, moved
        // in steps as a person moves it (one instant drag is too fast for the route's gesture
        // on a device); on Android system back.
        await goBack(tester);
        await until(
          tester,
          'post list on top',
          () => shell().pages.length == 2,
        );
        await app.waitFor(find.byKey(ValueKey('postList.post.$thread')));

        // A link from outside opens the thread at the message, with the whole stack.
        (app.fake.ports.deepLinks as ManualDeepLinks).send(
          Uri.parse('chat-example://t/$other?m=${linked['id']}'),
        );
        await app.waitFor(find.byKey(ValueKey('message.${linked['id']}')));
        expect(shell().tab, PhoneTab.spaces);
        expect(shell().pages.map((p) => p.kind), [
          PhonePageKind.spaces,
          PhonePageKind.channel,
          PhonePageKind.thread,
        ]);
        expect(
          find.byKey(ValueKey('historyColumn.$other.${linked['id']}')),
          findsOneWidget,
        );
        expect(app.fake.server.violations, isEmpty);
      } finally {
        await app.stop();
      }
    });
  });
}
