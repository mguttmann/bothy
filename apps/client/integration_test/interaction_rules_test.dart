// SPDX-License-Identifier: Apache-2.0

// The interaction rules of 01 6 in the thread view against the fake server (brief M4-20
// Tests): send online; send offline with a queue and reconnect (exactly one message, 04 16.2
// through the interface); a message in a done thread reopens it; an edit conflict; delete;
// reply with quote; reactions; pin; move a thread; a template; mark as unread; the read state
// and a draft across two devices.

import 'package:client/app/providers/session.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;
import 'package:sync_engine/testing.dart';

import '../test/support/fakes.dart';
import 'support/test_app.dart';

final Finder composerField = find.descendant(
  of: find.byKey(const ValueKey('composer')),
  matching: find.byType(TextField),
);

/// A signed-in app with the forum `ideas` (Alex and Sam write, Alex manages) and its post
/// `Printer jam` with Sam's first message, opened in the history.
final class Scene {
  Scene._(this.app, this.forum, this.thread, this.first);

  final TestApp app;
  final String forum;
  final String thread;
  final String first;

  FakeWorld get world => app.fake.server.world;
  WorldSeed get seed => app.fake.seed;
  WidgetTester get tester => app.tester;

  static Future<Scene> open(WidgetTester tester) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'ideas', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    world.grantCapability(seed.alex.id, forum, 'manage');
    final thread = world.addThread(
      forum,
      'Printer jam',
      createdBy: seed.sam.id,
    );
    final first = seed.post(
      seed.sam,
      thread,
      'The printer stops after one page.',
    );
    await app.connect();
    await app.loginWithPassword();
    await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
    await app.waitLive();
    await app.waitFor(find.byKey(ValueKey('sidebar.channel.$forum')));
    await tester.tap(find.byKey(ValueKey('sidebar.channel.$forum')));
    await app.waitFor(find.byKey(ValueKey('postList.post.$thread')));
    await tester.tap(find.byKey(ValueKey('postList.post.$thread')));
    final scene = Scene._(app, forum, thread, first['id']! as String);
    await app.waitFor(scene.message(scene.first));
    return scene;
  }

  Finder message(String id) => find.byKey(ValueKey('message.$id'));

  Future<void> until(String what, bool Function() condition) async {
    final end = DateTime.now().add(const Duration(seconds: 20));
    while (!condition()) {
      if (DateTime.now().isAfter(end)) fail('timed out: $what');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  Future<void> wait([int ms = 400]) async {
    for (var i = 0; i < ms ~/ 50; i++) {
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  Future<void> send(String text) async {
    await tester.enterText(composerField, text);
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await wait(100);
  }

  Future<void> act(String id, String action) async {
    final item = find.byKey(ValueKey('action.$action'));
    FocusManager.instance.primaryFocus?.unfocus();
    await tester.pump();
    for (var i = 0; i < 5 && item.evaluate().isEmpty; i++) {
      await tester.tap(
        message(id),
        buttons: kSecondaryButton,
        kind: PointerDeviceKind.mouse,
      );
      await wait();
    }
    await tester.ensureVisible(find.byKey(ValueKey('action.$action')));
    await wait(100);
    await tester.tap(find.byKey(ValueKey('action.$action')));
    await wait();
  }

  Future<void> tapKey(String key) async {
    await tester.tap(find.byKey(ValueKey(key)));
    await wait();
  }

  List<JsonMap> get serverMessages => world.messagesOf(thread);

  JsonMap? serverText(String text) {
    for (final m in serverMessages) {
      if (m['content_md'] == text) return m;
    }
    return null;
  }

  Future<void> offline() async {
    (app.fake.ports.network as SwitchNetwork).set(false);
    app.fake.server.disconnect(deviceId: app.deviceId);
    await until(
      'offline',
      () => app.state.services!.engine.state.phase != ConnectionPhase.live,
    );
  }

  Future<void> online() async {
    (app.fake.ports.network as SwitchNetwork).set(true);
    await app.waitLive();
  }
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('send online and offline with a queue; a done thread reopens', (
    tester,
  ) async {
    final s = await Scene.open(tester);
    try {
      await s.send('Online message');
      await s.until(
        'on the server',
        () => s.serverText('Online message') != null,
      );
      await s.until(
        'shown confirmed',
        () => s
            .message(s.serverText('Online message')!['id']! as String)
            .evaluate()
            .isNotEmpty,
      );

      // 04 16.2: offline the message waits with "Sending", after the reconnect it is sent
      // exactly once.
      await s.offline();
      await s.send('Queued message');
      await s.until(
        'shown as sending',
        () => find.text('Sending').evaluate().isNotEmpty,
      );
      await s.online();
      await s.until('sent', () => s.serverText('Queued message') != null);
      await s.wait(1000);
      expect(
        s.serverMessages.where((m) => m['content_md'] == 'Queued message'),
        hasLength(1),
      );

      // 01 6.1: a new message in a done thread reopens it with a system line.
      s.world.closeThread(s.thread, actor: s.seed.sam.id);
      await s.until(
        'Reopen offered',
        () => find.byKey(const ValueKey('thread.reopen')).evaluate().isNotEmpty,
      );
      await s.send('Back again');
      await s.until(
        'open again',
        () => s.world.threads[s.thread]!['state'] == 'open',
      );
      await s.until(
        'system line',
        () => find
            .textContaining('Reopened by message from')
            .evaluate()
            .isNotEmpty,
      );
      expect(s.app.fake.server.violations, isEmpty);
    } finally {
      await s.app.stop();
    }
  });

  testWidgets('reply, reactions, pin, delete, mark unread, template, move', (
    tester,
  ) async {
    final s = await Scene.open(tester);
    // A second forum reaches the device through access.changed (04 8.4).
    final target = s.world.addForum(s.seed.space, 'support-ideas', {});
    s.world
      ..grantCapability(s.seed.alex.id, target, 'manage')
      ..setAccess(s.seed.alex.id, target, 'write');
    s.world.addTemplate(
      scope: 'channel',
      channelId: s.forum,
      name: 'Escalation',
      contentMd: 'Please escalate this.',
    );
    try {
      // Reply with quote.
      await s.act(s.first, 'reply');
      await tester.enterText(
        composerField,
        '${tester.widget<TextField>(composerField).controller!.text}On it',
      );
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await s.until(
        'reply on the server',
        () => s.serverMessages.any((m) => m['reply_to_message_id'] == s.first),
      );
      final reply = s.serverMessages.firstWhere(
        (m) => m['reply_to_message_id'] == s.first,
      );
      expect(reply['content_md'], startsWith('> The printer stops'));

      // Reactions: add and remove.
      await s.act(s.first, 'react');
      await tester.enterText(
        find.byKey(const ValueKey('reaction.input')),
        '+1',
      );
      await s.tapKey('reaction.add');
      await s.until(
        'reacted',
        () => (s.world.messages[s.first]!['reactions']! as List).isNotEmpty,
      );
      await s.wait(500);
      await s.act(s.first, 'react');
      await s.tapKey('reaction.chip.+1');
      await s.until(
        'reaction removed',
        () => (s.world.messages[s.first]!['reactions']! as List).isEmpty,
      );

      // Pin.
      await s.act(s.first, 'pin');
      await s.until(
        'pinned',
        () => s.world.messages[s.first]!['pinned'] == true,
      );

      // Delete the own reply after one confirmation.
      final replyId = reply['id']! as String;
      await s.until(
        'reply shown',
        () => s.message(replyId).evaluate().isNotEmpty,
      );
      await s.act(replyId, 'delete');
      await s.tapKey('delete.confirm');
      await s.until(
        'deleted',
        () => s.world.messages[replyId]!['deleted_at'] != null,
      );

      // Mark as unread from the first message.
      await s.until(
        'read on the server',
        () => s.world.readStates['${s.seed.alex.id}|${s.thread}'] != null,
      );
      await s.wait(500);
      await s.act(s.first, 'markUnread');
      await s.until(
        'unread',
        () =>
            s
                .world
                .readStates['${s.seed.alex.id}|${s.thread}']!['last_read_seq'] ==
            0,
      );

      // A channel template into the field.
      await s.tapKey('composer.templates');
      await s.until(
        'templates',
        () => find.text('\u2068Escalation\u2069').evaluate().isNotEmpty,
      );
      await tester.tap(find.text('\u2068Escalation\u2069'));
      await s.wait();
      expect(
        tester.widget<TextField>(composerField).controller!.text,
        'Please escalate this.',
      );

      // Move to the other forum with the same (no) operator; the ID stays.
      await s.tapKey('thread.move');
      await tester.tap(find.byKey(ValueKey('move.channel.$target')));
      await tester.enterText(
        find.byKey(const ValueKey('move.reason')),
        'Belongs there',
      );
      await tester.pump();
      await s.tapKey('move.confirm');
      await s.until(
        'moved',
        () => s.world.threads[s.thread]!['channel_id'] == target,
      );
      expect(s.app.fake.server.violations, isEmpty);
    } finally {
      await s.app.stop();
    }
  });

  testWidgets('an edit conflict shows both versions', (tester) async {
    final s = await Scene.open(tester);
    try {
      await s.send('My first text');
      await s.until('sent', () => s.serverText('My first text') != null);
      final id = s.serverText('My first text')!['id']! as String;
      await s.until('confirmed', () => s.message(id).evaluate().isNotEmpty);
      await s.offline();
      await s.act(id, 'edit');
      await tester.enterText(
        find.byKey(const ValueKey('edit.field')),
        'My edit',
      );
      await s.tapKey('edit.save');
      // Another device of Alex edits meanwhile.
      s.world.updateMessage(s.seed.alex.id, id, 1, {
        'content_md': 'Their edit',
      });
      await s.online();
      await s.until(
        'conflict row',
        () =>
            find.byKey(const ValueKey('outbox.compare')).evaluate().isNotEmpty,
      );
      await s.tapKey('outbox.compare');
      expect(find.byKey(const ValueKey('conflict.dialog')), findsOneWidget);
      await s.tapKey('conflict.keepMine');
      await s.until(
        'mine kept',
        () => s.world.messages[id]!['content_md'] == 'My edit',
      );
      expect(s.app.fake.server.violations, isEmpty);
    } finally {
      await s.app.stop();
    }
  });

  testWidgets('read state and draft across two devices', (tester) async {
    final s = await Scene.open(tester);
    final second = await s.app.secondApp();
    try {
      second.container.listen(localDatabaseProvider, (_, _) {});
      await second.signIn();
      final thread = ThreadId.parse(s.thread);
      await eventually('second device live', () {
        final engine = second.state.services?.engine;
        return engine?.state.phase == ConnectionPhase.live;
      });
      // A new message while both devices run: this device reads it, the other one follows.
      s.seed.post(s.seed.sam, s.thread, 'Any news?');
      await s.until(
        'read here',
        () =>
            s
                .world
                .readStates['${s.seed.alex.id}|${s.thread}']?['last_read_seq'] ==
            2,
      );
      await eventually('read on the second device', () async {
        final db = second.container.read(localDatabaseProvider);
        final read = await db?.personalDao.readState(thread);
        return read != null && read.lastReadSeq.value == 2;
      });

      await tester.enterText(composerField, 'Draft from the laptop');
      await s.wait();
      await eventually('the draft on the second device', () async {
        final db = second.container.read(localDatabaseProvider);
        final drafts = await db?.personalDao.allDrafts() ?? const [];
        return drafts.any((d) => d.contentMd == 'Draft from the laptop');
      });
      expect(s.app.fake.server.violations, isEmpty);
    } finally {
      await second.stop();
      await s.app.stop();
    }
  });
}
