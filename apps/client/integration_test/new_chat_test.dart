// SPDX-License-Identifier: Apache-2.0

// A new chat from the sidebar against the fake server (brief M4-41 Tests; 01 6.8 item 8): "+"
// on a project forum with an operator agent opens the new chat view; sending puts exactly one
// `createThread` with an empty title (`title_source = auto`), the draft row stays at the top
// of the group and becomes the post row, and the post's history opens; the agent's title
// reaches the row. Offline the rows show "Sending" and the posts are created after the
// reconnect in the order of the outbox. A forum without an operator agent requires a title.

import 'package:design_system/design_system.dart' show dsIsolate;
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;
import 'package:sync_engine/testing.dart';

import '../test/support/fakes.dart';
import '../test/support/until.dart' show testPatience;
import 'support/new_chat.dart';
import 'support/test_app.dart';

/// A signed-in app with the project forum `server` (Robin its operator agent, the older post
/// `Gateway resume` in it) and the forum `requests` without an operator.
final class Scene {
  Scene._(this.app, this.forum, this.older, this.plain);

  /// The app.
  final TestApp app;

  /// The project forum with an operator agent.
  final String forum;

  /// The post that was in [forum] before the test.
  final String older;

  /// The forum without an operator agent.
  final String plain;

  /// The fake's world.
  FakeWorld get world => app.fake.server.world;

  /// The fake's seed.
  WorldSeed get seed => app.fake.seed;

  /// The tester.
  WidgetTester get tester => app.tester;

  /// Starts the app, signs in as Alex and waits until both forums show in the sidebar.
  static Future<Scene> open(WidgetTester tester) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'server', {
      seed.alex.id: 'trigger',
      seed.sam.id: 'write',
      seed.robin.id: 'write',
    });
    final older = world.addThread(
      forum,
      'Gateway resume',
      createdBy: seed.sam.id,
    );
    world
      ..setChannelOperator(forum, seed.robin.id)
      ..setProjectBound(forum, bound: true);
    final plain = world.addForum(seed.space, 'requests', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    await app.connect();
    await app.waitFor(find.byKey(const ValueKey('e2.loginName')));
    await app.loginWithPassword();
    await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
    await app.waitLive();
    await app.waitFor(threadRow(older));
    await app.waitFor(find.byKey(ValueKey('sidebar.channel.$plain')));
    return Scene._(app, forum, older, plain);
  }

  /// Pumps until [condition] holds; fails with [what] after the test patience.
  Future<void> until(String what, bool Function() condition) async {
    final end = DateTime.now().add(testPatience);
    while (!condition()) {
      if (DateTime.now().isAfter(end)) fail('timed out: $what');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  /// The top edge of [finder].
  double top(Finder finder) => tester.getTopLeft(finder).dy;

  /// Selects the sidebar row of [channel] (so its "+" shows), taps "+" with the label
  /// [addLabel] and waits for the view with the focus in its composer.
  Future<void> plus(String channel, String addLabel) async {
    await tester.tap(find.byKey(ValueKey('sidebar.channel.$channel')));
    await until(
      'the overview of $channel',
      () => currentRoute(tester) == '/c/$channel',
    );
    final add = find.byTooltip(addLabel);
    await app.waitFor(add);
    await tester.tap(add);
    await app.waitFor(newChatField);
    await until(
      'the route of the view',
      () => currentRoute(tester) == '/c/$channel/new',
    );
  }

  /// Types [text] into the composer of the view and sends it with Enter.
  Future<void> typeAndEnter(String text) async {
    await tester.enterText(newChatField, text);
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pump();
  }

  /// The `createThread` requests the fake answered, in the order they arrived.
  List<JsonMap> get creates => [
    for (final r in app.fake.server.requestsOf('createThread'))
      if (r.status >= 200 && r.status < 300) r.body! as JsonMap,
  ];

  /// The ID of the post in [channel] whose first message is [text], or null.
  String? postWith(String channel, String text) {
    for (final t in world.threads.values) {
      if (t['channel_id'] != channel) continue;
      final id = t['id']! as String;
      if (world.messagesOf(id).any((m) => m['content_md'] == text)) return id;
    }
    return null;
  }

  /// Takes the app offline: no network and the fake drops its connection.
  Future<void> offline() async {
    (app.fake.ports.network as SwitchNetwork).set(false);
    app.fake.server.disconnect(deviceId: app.deviceId);
    await until(
      'offline',
      () => app.state.services!.engine.state.phase != ConnectionPhase.live,
    );
  }

  /// Brings the network back and waits until the engine is live.
  Future<void> online() async {
    (app.fake.ports.network as SwitchNetwork).set(true);
    await app.waitLive();
  }
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  final en = newChatEn;

  testWidgets('"+", type, send: one untitled post, its row and history; the '
      'agent names it', (tester) async {
    final s = await Scene.open(tester);
    try {
      await s.plus(s.forum, en.sidebarNewChatIn('server'));
      expect(
        tester.widget<Text>(find.byKey(const ValueKey('newChat.heading'))).data,
        en.sidebarNewChatIn(dsIsolate('server')),
      );
      final draft = draftRow(s.forum);
      await s.app.waitFor(draft);
      final groupTop = s.top(draft);
      expect(groupTop, lessThan(s.top(threadRow(s.older))));
      expect(newChatSendEnabled(), isFalse, reason: 'nothing typed yet');

      // With an operator agent the title is optional: Enter sends without one.
      await s.typeAndEnter('The gateway drops the stream after a resume.');
      await s.until('the post on the server', () => s.creates.isNotEmpty);
      final post = s.postWith(
        s.forum,
        'The gateway drops the stream after a resume.',
      )!;
      await s.until(
        'the history opens',
        () => currentRoute(tester) == '/t/$post',
      );
      final row = threadRow(post);
      await s.app.waitFor(row);
      await s.until('the draft row is gone', () => draft.evaluate().isEmpty);
      expect(sentRows(s.forum), findsNothing);
      expect(s.top(row), groupTop, reason: 'the row stays at the top');
      expect(s.top(row), lessThan(s.top(threadRow(s.older))));
      expect(s.creates, hasLength(1));
      expect(s.creates.single['title'] ?? '', isEmpty);
      expect(
        (s.creates.single['first_message']! as JsonMap)['clear_draft'],
        isTrue,
      );
      expect(s.world.threads[post]!['title_source'], 'auto');

      // The agent names the post after its first answer (09 12).
      s.world.setTitle(
        s.seed.robin.id,
        post,
        s.world.threads[post]!['version']! as int,
        'Gateway drops resumed streams',
      );
      await s.app.waitFor(
        find.descendant(
          of: row,
          // The row isolates the title as every foreign name (dsIsolate).
          matching: find.text(dsIsolate('Gateway drops resumed streams')),
        ),
      );
      expect(s.app.fake.server.violations, isEmpty);
    } finally {
      await s.app.stop();
    }
  });

  testWidgets('offline the rows show Sending; after the reconnect the posts '
      'are created in outbox order', (tester) async {
    final s = await Scene.open(tester);
    try {
      await s.offline();
      await s.plus(s.forum, en.sidebarNewChatIn('server'));
      await s.typeAndEnter('First while offline');
      await s.until(
        'the view of the sent post',
        () => currentRoute(tester).startsWith('/c/${s.forum}/new?entry='),
      );
      await s.plus(s.forum, en.sidebarNewChatIn('server'));
      await s.typeAndEnter('Second while offline');
      await s.until(
        'two rows show Sending',
        () =>
            find
                .descendant(
                  of: sentRows(s.forum),
                  matching: find.text(en.messageSending),
                )
                .evaluate()
                .length ==
            2,
      );
      final sent = sentRows(s.forum);
      expect(s.top(sent.first), lessThan(s.top(threadRow(s.older))));
      expect(s.creates, isEmpty);

      await s.online();
      await s.until(
        'both posts on the server',
        () =>
            s.postWith(s.forum, 'First while offline') != null &&
            s.postWith(s.forum, 'Second while offline') != null,
      );
      final first = s.postWith(s.forum, 'First while offline')!;
      final second = s.postWith(s.forum, 'Second while offline')!;
      await s.app.waitFor(threadRow(first));
      await s.app.waitFor(threadRow(second));
      await s.until('no row waits', () => sent.evaluate().isEmpty);
      expect(
        [
          for (final c in s.creates)
            (c['first_message']! as JsonMap)['content_md'],
        ],
        ['First while offline', 'Second while offline'],
      );
      expect(s.top(threadRow(second)), lessThan(s.top(threadRow(first))));
      expect(s.top(threadRow(first)), lessThan(s.top(threadRow(s.older))));
      expect(s.app.fake.server.violations, isEmpty);
    } finally {
      await s.app.stop();
    }
  });

  testWidgets('a forum without an operator agent requires a title', (
    tester,
  ) async {
    final s = await Scene.open(tester);
    try {
      await s.plus(s.plain, en.sidebarNewPostIn('requests'));
      expect(find.byKey(const ValueKey('newChat.operator')), findsOneWidget);
      // Enter without a title sends nothing; the note says why.
      await s.typeAndEnter('Please order a second monitor.');
      await s.app.waitFor(find.byKey(const ValueKey('newChat.titleMissing')));
      expect(newChatSendEnabled(), isFalse);
      expect(currentRoute(tester), '/c/${s.plain}/new');

      await tester.enterText(newChatTitle, 'Second monitor');
      await s.until('send enabled', newChatSendEnabled);
      expect(find.byKey(const ValueKey('newChat.titleMissing')), findsNothing);
      await tester.tap(newChatSend);
      await s.until('the post on the server', () => s.creates.isNotEmpty);
      final post = s.postWith(s.plain, 'Please order a second monitor.')!;
      await s.app.waitFor(threadRow(post));
      // The Enter above sent nothing: the one request is the titled post.
      expect(s.creates, hasLength(1));
      expect(s.creates.single['title'], 'Second monitor');
      expect(s.world.threads[post]!['title_source'], 'manual');
      expect(s.app.fake.server.violations, isEmpty);
    } finally {
      await s.app.stop();
    }
  });
}
