// SPDX-License-Identifier: Apache-2.0

// Editing the title of an active agent chat (brief FIX-TITLE-EDIT-412). Owner report: the edit
// always failed on the first try with "Not saved: somebody else changed this in the meantime",
// and "Send again" then went through. The server raises the version of a thread with every
// message (03 6.2) but writes only `message.created` (no thread event), so the thread the app
// holds keeps the version of its last thread event, and `setThreadTitle` sent that version as
// `If-Match` against a server that asked for the newest one. The server now accepts every
// version since the last title change (06 8 item 2); the fake follows both rules. The app
// against the fake server, headless: the edit is saved on the first try, and an edit that is
// based on a title another person replaced in between still fails and is sent again by hand.

import 'dart:io';

import 'package:client/app/providers/session.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/sidebar/sidebar_threads.dart';
import 'package:client/sync/message_ops_app.dart';
import 'package:client/thread/thread_actions.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

void main() {
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer app;
  late String forum;
  late String chat;

  /// The chat as the sidebar group of its forum shows it, or null while it is not stored.
  Thread? shown() {
    final rows = app.read(channelThreadsProvider(forum))?.rows;
    for (final r in rows ?? const <SidebarThread>[]) {
      if (r.thread.id.value == chat) return r.thread;
    }
    return null;
  }

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    f = await FakeInstance.start();
    app = appContainer(f);
    // A chat with an agent as operator, still named by the placeholder of a new chat.
    final seed = f.seed;
    final world = f.server.world;
    forum = world.addForum(seed.space, 'help', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
      seed.robin.id: 'write',
    });
    world.setOperator(forum, seed.robin.id);
    chat = world.addThread(forum, 'Neuer Chat', createdBy: seed.alex.id);
    world.threads[chat]!['title_source'] = 'auto';
    app.listen(channelThreadsProvider(forum), (_, _) {});
    final control = await connected(app, f);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => app.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
    );
    await eventually('the chat is in the sidebar', () => shown() != null);
  });

  tearDown(() async {
    app.dispose();
    await f.stop();
    HttpOverrides.global = saved;
  });

  int serverVersion() => f.server.world.threads[chat]!['version']! as int;
  String serverTitle() => f.server.world.threads[chat]!['title']! as String;
  List<int> titleAnswers() => [
    for (final r in f.server.requestsOf('setThreadTitle')) r.status,
  ];
  Future<List<OutboxEntry>> titleEntries() async => [
    for (final e
        in await app.read(localDatabaseProvider)!.outboxDao.allEntries())
      if (e.kind == AppMessageOps.setThreadTitle) e,
  ];

  /// Alex asks and the agent answers; returns when the app stored the answer.
  Future<void> agentAnswers(String question, String answer) async {
    final engine = app.read(syncEngineProvider)!;
    final db = app.read(localDatabaseProvider)!;
    final before = f.server.world.messagesOf(chat).length;
    await ThreadActions(engine).send(ThreadId.parse(chat), question);
    await eventually(
      'the question reached the server',
      () => f.server.world.messagesOf(chat).length == before + 1,
    );
    final sent = f.seed.post(f.seed.robin, chat, answer);
    await eventually(
      'the answer reached the app',
      () async =>
          await db.messagesDao.byId(MessageId.parse(sent['id']! as String)) !=
          null,
    );
  }

  test(
    'a title edit in an active agent chat is saved on the first try',
    () async {
      await agentAnswers('The export fails', 'The March filter is the cause');
      // What the title dialog starts from: the thread the app shows. Its version is the one of
      // the last thread object the app received; the two messages moved the server past it.
      final held = shown()!;
      expect(held.title, 'Neuer Chat');
      expect(held.titleSource, TitleSource.auto);
      expect(
        held.version.value,
        lessThan(serverVersion()),
        reason: 'messages raise the thread version without a thread event',
      );

      final engine = app.read(syncEngineProvider)!;
      await ThreadActions(engine).setTitle(held, 'Export fails for March');
      await eventually(
        'the title is saved',
        () => serverTitle() == 'Export fails for March',
      );
      await eventually(
        'the app shows the title',
        () => shown()?.title == 'Export fails for March',
      );
      expect(titleAnswers(), [200], reason: 'one request, accepted');
      expect(shown()!.titleSource, TitleSource.manual);
      expect([
        for (final e in await titleEntries()) e.state,
      ], isNot(contains(OutboxState.failed)));

      // The next edit starts from the version of that answer, behind again after the next turn.
      await agentAnswers('And for April?', 'April is fine');
      await ThreadActions(engine)
          .setTitle(shown()!, 'Export fails only for March');
      await eventually(
        'the second title is saved',
        () => serverTitle() == 'Export fails only for March',
      );
      expect(titleAnswers(), [200, 200]);
      expect(f.server.violations, isEmpty);
    },
  );

  test('an edit of a title another person replaced in between fails and is '
      'sent again by hand', () async {
    await agentAnswers('The export fails', 'The March filter is the cause');
    // The dialog opens on this thread; while it is open Sam sets a title and the agent writes.
    final held = shown()!;
    final world = f.server.world;
    world.setTitle(f.seed.sam.id, chat, serverVersion(), 'Export problem');
    f.seed.post(f.seed.robin, chat, 'Anything else?');

    final engine = app.read(syncEngineProvider)!;
    await ThreadActions(engine).setTitle(held, 'Export fails for March');
    OutboxEntry? failed;
    await eventually('the edit failed', () async {
      for (final e in await titleEntries()) {
        if (e.state == OutboxState.failed) failed = e;
      }
      return failed != null;
    });
    expect(failed!.lastErrorCode, 'version_conflict');
    expect(serverTitle(), 'Export problem', reason: "Sam's title stays");
    expect(titleAnswers(), [412]);

    // "Send again" is the person's decision to replace the title they now see.
    await ThreadActions(engine).retry(failed!.id);
    await eventually(
      'the title is saved',
      () => serverTitle() == 'Export fails for March',
    );
    expect(titleAnswers(), [412, 200]);
    expect(f.server.violations, isEmpty);
  });
}
