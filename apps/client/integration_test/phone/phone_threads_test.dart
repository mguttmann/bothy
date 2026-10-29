// SPDX-License-Identifier: Apache-2.0

// The thread groups of M1 and a new chat on the phone against the fake server (brief M5-15
// Tests; 01 6.8; D-SB-13; 04 6.3 item 4): after connect M1 shows the active posts of every forum
// from the snapshot without a `listChannelThreads` request; a post that Sam creates appears at
// the top of its group; the tree reads with its levels and labels for VoiceOver and TalkBack; a
// tap on a thread row opens its history M3; "+", typing and sending create the post through the
// outbox and its history opens. The same file runs on the iPhone layout (iOS simulator, the
// Flutter tester) and on the Android layout (an Android device or emulator).

import 'package:client/phone/phone_navigation.dart';
import 'package:client/phone/phone_shell.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../support/new_chat.dart';
import 'support/phone_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('threads in M1, a thread row, "+" and send', (tester) async {
    await asPhone(() async {
      final semantics = tester.ensureSemantics();
      final app = await startPhone(tester);
      final world = app.fake.server.world;
      final seed = app.fake.seed;
      final forum = world.addForum(seed.space, 'server', {
        seed.alex.id: 'trigger',
        seed.sam.id: 'write',
        seed.robin.id: 'write',
      });
      world
        ..setChannelOperator(forum, seed.robin.id)
        ..setProjectBound(forum, bound: true);
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
      PhoneShellState shell() =>
          tester.state<PhoneShellState>(find.byType(PhoneShell));
      List<PhonePageKind> kinds() => [for (final p in shell().pages) p.kind];
      double top(Finder f) => tester.getTopLeft(f).dy;
      try {
        await signIn(app);

        // From the snapshot, without one list request per channel: after the full snapshot
        // only the open posts of the forum are listed (D-M4-46-5), no text channel.
        await app.waitFor(threadRow(first));
        await app.waitFor(threadRow(second));
        expect(
          [
            for (final r in app.fake.server.requestsOf('listChannelThreads'))
              r.path,
          ],
          allOf(
            hasLength(lessThanOrEqualTo(1)),
            everyElement(endsWith('/channels/$forum/threads')),
          ),
          reason: 'M1 fills from active_threads',
        );
        expect(top(threadRow(second)), lessThan(top(threadRow(first))));

        // Sam creates a post: it appears at the top of the group.
        final created = world.createThread(seed.sam.id, forum, {
          'title': 'Review grant wizard',
          'first_message': {
            'client_msg_id': world.nextId(),
            'content_md': 'Please have a look.',
          },
        });
        final newest = (created['thread']! as Map)['id']! as String;
        await app.waitFor(threadRow(newest));
        expect(top(threadRow(newest)), lessThan(top(threadRow(second))));

        // The tree for VoiceOver and TalkBack: the row's label, and the row one level below
        // its channel row inside the channel's group (components.md 7).
        expect(
          find.bySemanticsLabel(RegExp('^Review grant wizard, ')),
          findsOneWidget,
        );
        final channelNode = tester.getSemantics(
          find.byKey(ValueKey('sidebar.channel.$forum')),
        );
        final rowNode = tester.getSemantics(threadRow(newest));
        bool within(SemanticsNode node, SemanticsNode? outer) {
          for (var n = node.parent; n != null; n = n.parent) {
            if (n == outer) return true;
          }
          return false;
        }

        expect(within(rowNode, channelNode.parent), isTrue);
        expect(within(channelNode, rowNode.parent), isFalse);
        expect(
          find.bySemanticsLabel(newChatEn.sidebarNewChatIn('server')),
          findsOneWidget,
          reason: '"+" is a labelled button on touch',
        );

        // A tap on a thread row opens its history M3.
        await untilStill(tester, threadRow(first));
        await tester.tap(threadRow(first));
        await until(
          tester,
          'the history of the post',
          () => currentRoute(tester) == '/t/$first',
        );
        await until(tester, 'M3 on top', () => kinds().length == 3);
        expect(kinds(), [
          PhonePageKind.spaces,
          PhonePageKind.channel,
          PhonePageKind.thread,
        ]);
        await goBack(tester);
        await until(tester, 'M2 on top', () => kinds().length == 2);
        await goBack(tester);
        await until(tester, 'M1 again', () => kinds().length == 1);

        // "+", type, send: the post is created and its history opens.
        final add = find.byTooltip(newChatEn.sidebarNewChatIn('server'));
        await app.waitFor(add);
        await untilStill(tester, add);
        await tester.tap(add);
        await app.waitFor(newChatField);
        expect(kinds(), [PhonePageKind.spaces, PhonePageKind.newChat]);
        await tester.enterText(newChatField, 'Restart the gateway');
        await tester.pump();
        await until(tester, 'send enabled', newChatSendEnabled);
        await untilStill(tester, newChatSend);
        await tester.tap(newChatSend);
        String? post;
        await until(tester, 'the post on the server', () {
          for (final t in world.threads.values) {
            if (t['channel_id'] != forum) continue;
            final id = t['id']! as String;
            if (world
                .messagesOf(id)
                .any((m) => m['content_md'] == 'Restart the gateway')) {
              post = id;
              return true;
            }
          }
          return false;
        });
        await until(
          tester,
          'the history of the new post',
          () => currentRoute(tester) == '/t/$post',
        );
        await until(tester, 'M3 of the new post', () => kinds().length == 3);
        expect(kinds().last, PhonePageKind.thread);
        expect(
          app.fake.server.requestsOf('createThread'),
          hasLength(1),
          reason: 'one post, created through the outbox',
        );
      } finally {
        semantics.dispose();
        await app.stop();
      }
    });
  });
}
