// SPDX-License-Identifier: Apache-2.0

// The handlers M4-20 added to the fake (brief M4-20 Scope): revisions, mark unread with
// If-Match, pins with `manage`, close and reopen with the system line, the title, moving to a
// channel of the same operator, templates and a thread from a message. Every request and
// response passes the contract check of the fake.

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late TestDevice alex;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    alex = h.device();
    await signedIn(h, alex);
  });
  tearDown(() => h.stop());

  Future<int?> status(Future<Object?> call) => call.then(
    (_) => null,
    onError: (Object e) => (e as DioException).response?.statusCode,
  );

  test('revisions list the earlier texts of an edited message', () async {
    final m = h.seed.post(h.seed.alex, h.seed.releaseNotes, 'first text');
    final messages = alex.api.getMessagesApi();
    await messages.updateMessage(
      messageId: m['id']! as String,
      ifMatch: '"1"',
      messageUpdate: MessageUpdate((b) => b..contentMd = 'second text'),
    );
    final list = (await messages.listMessageRevisions(
      messageId: m['id']! as String,
    )).data!.items;
    expect(list.single.contentMd, 'first text');
    expect(list.single.revision, 1);
    expect(h.server.violations, isEmpty);
  });

  test('mark unread needs the version of the read state', () async {
    final thread = h.seed.releaseNotes;
    for (var i = 0; i < 3; i++) {
      h.seed.post(h.seed.sam, thread, 'line $i');
    }
    final prefs = alex.api.getPreferencesApi();
    final read = (await prefs.putReadState(
      threadId: thread,
      readStatePut: ReadStatePut((b) => b..lastReadSeq = 3),
    )).data!;
    expect(
      await status(
        prefs.markThreadUnread(
          threadId: thread,
          ifMatch: '"${read.version + 1}"',
          readStatePut: ReadStatePut((b) => b..lastReadSeq = 1),
        ),
      ),
      412,
    );
    final reset = (await prefs.markThreadUnread(
      threadId: thread,
      ifMatch: '"${read.version}"',
      readStatePut: ReadStatePut((b) => b..lastReadSeq = 1),
    )).data!;
    expect(reset.lastReadSeq, 1);
    expect(h.server.violations, isEmpty);
  });

  test('pins need manage and appear in the list and the events', () async {
    final m = h.seed.post(h.seed.sam, h.seed.releaseNotes, 'pin me');
    final id = m['id']! as String;
    final messages = alex.api.getMessagesApi();
    expect(await status(messages.pinMessage(messageId: id)), 403);
    world.grantCapability(h.seed.alex.id, h.seed.general, 'manage');
    await messages.pinMessage(messageId: id);
    final pins = (await messages.listPins(threadId: h.seed.releaseNotes))
        .data!
        .items;
    expect(pins.single.messageId, id);
    expect(world.log.last.type, 'pin.added');
    await messages.unpinMessage(messageId: id);
    expect(world.log.last.type, 'pin.removed');
    expect(world.messages[id]!['pinned'], isFalse);
    expect(h.server.violations, isEmpty);
  });

  test('close, reopen by a message with the system line, title', () async {
    final threads = alex.api.getThreadsApi();
    final thread = h.seed.releaseNotes;
    final closed = (await threads.closeThread(threadId: thread)).data!;
    expect(closed.state, ThreadState.closed);
    final reopened = (await threads.reopenThread(threadId: thread)).data!;
    expect(reopened.state, ThreadState.open);
    await threads.closeThread(threadId: thread);
    h.seed.post(h.seed.sam, thread, 'again');
    final last = world.messagesOf(thread);
    expect(last[last.length - 2]['system_code'], 'thread.reopened');
    expect(world.threads[thread]!['state'], 'open');

    final version = world.threads[thread]!['version']! as int;
    final titled = (await threads.setThreadTitle(
      threadId: thread,
      ifMatch: '"$version"',
      threadTitlePut: ThreadTitlePut((b) => b..title = 'Release plan'),
    )).data!;
    expect(titled.title, 'Release plan');
    expect(titled.titleSource, ThreadTitleSourceEnum.manual);
    expect(h.server.violations, isEmpty);
  });

  test('move only to a channel with the same operator', () async {
    final threads = alex.api.getThreadsApi();
    final other = world.addChannel(h.seed.space, 'ops', {
      h.seed.alex.id: 'write',
    });
    world
      ..grantCapability(h.seed.alex.id, h.seed.general, 'manage')
      ..grantCapability(h.seed.alex.id, other, 'manage')
      ..setOperator(other, h.seed.robin.id);
    ThreadMove to(String channel) => ThreadMove(
      (b) => b
        ..targetChannelId = channel
        ..reason = 'Belongs to ops',
    );
    expect(
      await status(
        threads.moveThread(
          threadId: h.seed.releaseNotes,
          threadMove: to(other),
        ),
      ),
      422,
    );
    world.setOperator(h.seed.general, h.seed.robin.id);
    final moved = (await threads.moveThread(
      threadId: h.seed.releaseNotes,
      threadMove: to(other),
    )).data!;
    expect(moved.id, h.seed.releaseNotes);
    expect(moved.channelId, other);
    expect(world.log.last.type, 'thread.moved');
    expect(h.server.violations, isEmpty);
  });

  test('personal and channel templates', () async {
    world.addTemplate(
      scope: 'channel',
      channelId: h.seed.general,
      name: 'Escalation',
      contentMd: 'Please escalate.',
    );
    final me = alex.api.getMeApi();
    final created = (await me.createPersonalTemplate(
      templateCreate: TemplateCreate(
        (b) => b
          ..name = 'Greeting'
          ..contentMd = 'Hello Sam',
      ),
    )).data!;
    expect(created.scope, TemplateScopeEnum.personal);
    expect((await me.listMyTemplates()).data!.items.map((t) => t.name), [
      'Greeting',
    ]);
    expect(
      (await alex.api.getChannelsApi().listTemplates(channelId: h.seed.general))
          .data!
          .items
          .map((t) => t.name),
      ['Escalation'],
    );
    expect(h.server.violations, isEmpty);
  });

  test('a thread from a message of the main thread', () async {
    final main = world.addMainThread(h.seed.general, createdBy: h.seed.sam.id);
    final parent = h.seed.post(h.seed.sam, main, 'Export format question');
    final created = (await alex.api.getThreadsApi().createThread(
      channelId: h.seed.general,
      threadCreate: ThreadCreate(
        (b) => b..parentMessageId = parent['id']! as String,
      ),
    )).data!;
    expect(created.thread.parentMessageId, parent['id']);
    expect(created.thread.title, 'Export format question');
    expect(created.thread.titleSource, ThreadTitleSourceEnum.auto);
    expect(h.server.violations, isEmpty);
  });
}
