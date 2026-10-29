// SPDX-License-Identifier: Apache-2.0

// The handlers M4-41 added to the fake (new chat from the sidebar): the draft of a new forum
// post (`putChannelDraft`, `deleteChannelDraft`, 04 14, 03 table drafts with `thread_id` NULL),
// forum posts without a title (`title_source = auto`) and `first_message.clear_draft`, which
// deletes the channel draft of the forum as the server's `OnMessagePosted` does.

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late String forum;

  setUp(() async {
    h = await FakeWorldHarness.start();
    final w = h.seed;
    forum = h.server.world.addForum(w.space, 'questions', {
      w.alex.id: 'write',
      w.sam.id: 'read',
    });
  });
  tearDown(() => h.stop());

  Future<DioException?> failure(Future<Object?> call) =>
      call.then((_) => null, onError: (Object e) => e as DioException);

  Map<String, Object?> payloadOf(FakeEvent e) =>
      e.envelope['payload']! as Map<String, Object?>;

  Map<String, Object?> scopeOf(FakeEvent e) =>
      e.envelope['scope']! as Map<String, Object?>;

  // FIX-draft-conflict-banner: as the server, overwrote_newer only for different text of
  // another device that this device had not seen; its own writes with a lagging base and an
  // empty or identical draft never report it.
  test(
    'overwrote_newer only for another device\'s newer different text',
    () async {
      final phone = h.device();
      await signedIn(h, phone);
      final laptop = h.device(keySeed: 2);
      await signedIn(h, laptop);
      Future<bool> put(
        TestDevice d,
        String text, {
        int? base,
        String? thread,
      }) async {
        final draft = DraftPut(
          (b) => b
            ..contentMd = text
            ..baseVersion = base,
        );
        final prefs = d.api.getPreferencesApi();
        final result = thread == null
            ? await prefs.putChannelDraft(channelId: forum, draftPut: draft)
            : await prefs.putThreadDraft(threadId: thread, draftPut: draft);
        return result.data!.overwroteNewer;
      }

      for (final thread in [null, h.seed.releaseNotes]) {
        expect(await put(phone, 'H', thread: thread), isFalse);
        expect(await put(phone, 'He', thread: thread), isFalse);
        expect(await put(phone, 'Hel', base: 1, thread: thread), isFalse);
        expect(await put(laptop, 'Hel', base: 1, thread: thread), isFalse);
        expect(await put(laptop, 'Laptop', base: 4, thread: thread), isFalse);
        expect(await put(phone, 'Hello', base: 4, thread: thread), isTrue);
        expect(await put(laptop, 'Mine', thread: thread), isTrue);
        expect(await put(phone, '', base: 1, thread: thread), isFalse);
      }
      expect(h.server.violations, isEmpty);
    },
  );

  test('put, overwrite and list the draft of a new forum post', () async {
    final alex = h.device();
    final session = await signedIn(h, alex);
    final prefs = alex.api.getPreferencesApi();

    final first = (await prefs.putChannelDraft(
      channelId: forum,
      draftPut: DraftPut((b) => b..contentMd = 'Printer jams'),
    )).data!;
    expect(first.overwroteNewer, isFalse);
    expect(first.draft.threadId, isNull);
    expect(first.draft.channelId, forum);
    expect(first.draft.contentMd, 'Printer jams');
    expect(first.draft.version, 1);
    expect(first.draft.updatedByDeviceId, session.deviceId);

    final updated = h.server.world.log.last;
    expect(updated.type, 'draft.updated');
    expect(updated.audience, h.seed.alex.id);
    expect(scopeOf(updated), isNot(contains('thread_id')));
    expect(scopeOf(updated)['channel_id'], forum);
    expect(
      (payloadOf(updated)['draft']! as Map<String, Object?>)['id'],
      first.draft.id,
    );

    final second = (await prefs.putChannelDraft(
      channelId: forum,
      draftPut: DraftPut(
        (b) => b
          ..contentMd = 'Printer jams since Monday'
          ..baseVersion = 1,
      ),
    )).data!;
    expect(second.overwroteNewer, isFalse);
    expect(second.draft.id, first.draft.id);
    expect(second.draft.version, 2);

    // Another device edited version 1 while this one already wrote version 2.
    final laptop = h.device(keySeed: 2);
    await signedIn(h, laptop);
    final stale = (await laptop.api.getPreferencesApi().putChannelDraft(
      channelId: forum,
      draftPut: DraftPut(
        (b) => b
          ..contentMd = 'Printer jams on floor 2'
          ..baseVersion = 1,
      ),
    )).data!;
    expect(stale.overwroteNewer, isTrue);
    expect(stale.draft.version, 3);

    final listed = (await prefs.listMyDrafts()).data!.items;
    final mine = listed.singleWhere((d) => d.channelId == forum);
    expect(mine.threadId, isNull);
    expect(mine.contentMd, 'Printer jams on floor 2');
    final snapshotDrafts =
        h.server.world.snapshot(h.seed.alex.id)['drafts']! as List;
    expect(
      snapshotDrafts.cast<Map<String, Object?>>().single,
      isNot(contains('thread_id')),
    );
    expect(h.server.violations, isEmpty);
  });

  test('the channel draft and a thread draft are separate rows', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final prefs = alex.api.getPreferencesApi();
    final post = h.server.world.addThread(
      forum,
      'Login loop',
      createdBy: h.seed.alex.id,
    );
    await prefs.putThreadDraft(
      threadId: post,
      draftPut: DraftPut((b) => b..contentMd = 'Reply to the post'),
    );
    final channel = (await prefs.putChannelDraft(
      channelId: forum,
      draftPut: DraftPut((b) => b..contentMd = 'A new post'),
    )).data!;
    expect(channel.draft.version, 1);
    final listed = (await prefs.listMyDrafts()).data!.items;
    expect(listed, hasLength(2));
    expect(
      listed.singleWhere((d) => d.threadId == post).contentMd,
      'Reply to the post',
    );
    expect(h.server.violations, isEmpty);
  });

  test('an empty put deletes the draft and answers the final state', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final prefs = alex.api.getPreferencesApi();

    // Without a row: version 1 and no event.
    final headBefore = h.server.world.headSeq;
    final none = (await prefs.putChannelDraft(
      channelId: forum,
      draftPut: DraftPut((b) => b..contentMd = '   '),
    )).data!;
    expect(none.draft.version, 1);
    expect(none.draft.contentMd, '');
    expect(none.overwroteNewer, isFalse);
    expect(h.server.world.headSeq, headBefore);

    final stored = (await prefs.putChannelDraft(
      channelId: forum,
      draftPut: DraftPut((b) => b..contentMd = 'Printer jams'),
    )).data!;
    final cleared = (await prefs.putChannelDraft(
      channelId: forum,
      draftPut: DraftPut(
        (b) => b
          ..contentMd = ' \n'
          ..baseVersion = stored.draft.version,
      ),
    )).data!;
    expect(cleared.draft.id, stored.draft.id);
    expect(cleared.draft.version, stored.draft.version + 1);
    expect(cleared.draft.contentMd, '');
    expect(cleared.draft.fileIds, isEmpty);
    expect(cleared.overwroteNewer, isFalse);

    final deleted = h.server.world.log.last;
    expect(deleted.type, 'draft.deleted');
    expect(deleted.audience, h.seed.alex.id);
    expect(payloadOf(deleted), {
      'draft_id': stored.draft.id,
      'channel_id': forum,
    });
    expect((await prefs.listMyDrafts()).data!.items, isEmpty);
    expect(h.server.violations, isEmpty);
  });

  test(
    'delete removes the draft with draft.deleted; twice is no error',
    () async {
      final alex = h.device();
      await signedIn(h, alex);
      final prefs = alex.api.getPreferencesApi();
      final stored = (await prefs.putChannelDraft(
        channelId: forum,
        draftPut: DraftPut((b) => b..contentMd = 'Printer jams'),
      )).data!;

      final first = await prefs.deleteChannelDraft(channelId: forum);
      expect(first.statusCode, 204);
      final deleted = h.server.world.log.last;
      expect(deleted.type, 'draft.deleted');
      expect(deleted.audience, h.seed.alex.id);
      expect(scopeOf(deleted), isNot(contains('thread_id')));
      expect(payloadOf(deleted), {
        'draft_id': stored.draft.id,
        'channel_id': forum,
      });

      final headBefore = h.server.world.headSeq;
      final second = await prefs.deleteChannelDraft(channelId: forum);
      expect(second.statusCode, 204);
      expect(h.server.world.headSeq, headBefore);
      expect((await prefs.listMyDrafts()).data!.items, isEmpty);
      expect(h.server.violations, isEmpty);
    },
  );

  test('only a visible forum with write takes a channel draft', () async {
    final w = h.seed;
    final hidden = h.server.world.addForum(w.space, 'hidden', {
      w.sam.id: 'write',
    });
    final alex = h.device();
    await signedIn(h, alex);
    final prefs = alex.api.getPreferencesApi();
    final draft = DraftPut((b) => b..contentMd = 'Text');

    final unseen = await failure(
      prefs.putChannelDraft(channelId: hidden, draftPut: draft),
    );
    expect(unseen?.response?.statusCode, 404);
    final unseenDelete = await failure(
      prefs.deleteChannelDraft(channelId: hidden),
    );
    expect(unseenDelete?.response?.statusCode, 404);

    // A text channel drafts on its main thread (the server's NotAForum).
    final text = await failure(
      prefs.putChannelDraft(channelId: w.general, draftPut: draft),
    );
    expect(text?.response?.statusCode, 400);

    // A new forum post has no reply target.
    final reply = await failure(
      prefs.putChannelDraft(
        channelId: forum,
        draftPut: DraftPut(
          (b) => b
            ..contentMd = 'Text'
            ..replyToMessageId = h.server.world.nextId(),
        ),
      ),
    );
    expect(reply?.response?.statusCode, 400);

    final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
    await signedIn(h, sam);
    final readOnly = await failure(
      sam.api.getPreferencesApi().putChannelDraft(
        channelId: forum,
        draftPut: draft,
      ),
    );
    expect(readOnly?.response?.statusCode, 403);
    expect(h.server.world.drafts, isEmpty);
    expect(h.server.violations, isEmpty);
  });

  test('a forum post without a title gets title_source auto', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final threads = alex.api.getThreadsApi();

    final untitled = (await threads.createThread(
      channelId: forum,
      threadCreate: ThreadCreate(
        (b) => b
          ..firstMessage.clientMsgId = h.server.world.nextId()
          ..firstMessage.contentMd = 'Printer jams',
      ),
    )).data!;
    expect(untitled.thread.title, '');
    expect(untitled.thread.titleSource, ThreadTitleSourceEnum.auto);

    final blank = (await threads.createThread(
      channelId: forum,
      threadCreate: ThreadCreate(
        (b) => b
          ..title = ''
          ..firstMessage.clientMsgId = h.server.world.nextId()
          ..firstMessage.contentMd = 'Scanner jams',
      ),
    )).data!;
    expect(blank.thread.titleSource, ThreadTitleSourceEnum.auto);

    final titled = (await threads.createThread(
      channelId: forum,
      threadCreate: ThreadCreate(
        (b) => b
          ..title = 'Login loop'
          ..firstMessage.clientMsgId = h.server.world.nextId()
          ..firstMessage.contentMd = 'Since Monday',
      ),
    )).data!;
    expect(titled.thread.title, 'Login loop');
    expect(titled.thread.titleSource, ThreadTitleSourceEnum.manual);
    expect(h.server.violations, isEmpty);
  });

  test('clear_draft of a new post deletes the channel draft only', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final prefs = alex.api.getPreferencesApi();
    final other = h.server.world.addThread(
      forum,
      'Login loop',
      createdBy: h.seed.alex.id,
    );
    final threadDraft = (await prefs.putThreadDraft(
      threadId: other,
      draftPut: DraftPut((b) => b..contentMd = 'Reply to the post'),
    )).data!;
    final channelDraft = (await prefs.putChannelDraft(
      channelId: forum,
      draftPut: DraftPut((b) => b..contentMd = 'Printer jams'),
    )).data!;

    final created = (await alex.api.getThreadsApi().createThread(
      channelId: forum,
      threadCreate: ThreadCreate(
        (b) => b
          ..firstMessage.clientMsgId = h.server.world.nextId()
          ..firstMessage.contentMd = 'Printer jams'
          ..firstMessage.clearDraft = true,
      ),
    )).data!;

    // Following the new post (notification_pref.updated) may come in between.
    expect(
      h.server.world.log.map((e) => e.type),
      containsAllInOrder([
        'thread.created',
        'message.created',
        'draft.deleted',
      ]),
    );
    expect(
      h.server.world.log.where((e) => e.type == 'draft.deleted'),
      hasLength(1),
    );
    final deleted = h.server.world.log.last;
    expect(deleted.audience, h.seed.alex.id);
    expect(payloadOf(deleted), {
      'draft_id': channelDraft.draft.id,
      'channel_id': forum,
    });
    final listed = (await prefs.listMyDrafts()).data!.items;
    expect(listed.single.id, threadDraft.draft.id);
    expect(listed.single.threadId, other);
    expect(created.thread.titleSource, ThreadTitleSourceEnum.auto);
    expect(h.server.violations, isEmpty);
  });

  test('a post without clear_draft keeps the channel draft', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final prefs = alex.api.getPreferencesApi();
    await prefs.putChannelDraft(
      channelId: forum,
      draftPut: DraftPut((b) => b..contentMd = 'Printer jams'),
    );
    final headBefore = h.server.world.headSeq;
    await alex.api.getThreadsApi().createThread(
      channelId: forum,
      threadCreate: ThreadCreate(
        (b) => b
          ..title = 'Scanner'
          ..firstMessage.clientMsgId = h.server.world.nextId()
          ..firstMessage.contentMd = 'Scanner jams',
      ),
    );
    expect((await prefs.listMyDrafts()).data!.items, hasLength(1));
    expect(
      h.server.world.log.where(
        (e) => e.seq! > headBefore && e.type == 'draft.deleted',
      ),
      isEmpty,
    );
    expect(h.server.violations, isEmpty);
  });
}
