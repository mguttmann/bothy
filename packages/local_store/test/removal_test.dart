// SPDX-License-Identifier: Apache-2.0

// Removal of targets (04 8.3, 15 6.3): everything below a removed target goes in one
// transaction, the file cache paths come back, other targets stay.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  /// Channel [c] in space [s] with thread [c] and two messages, a file attached to the first,
  /// a cached original, a draft, a read state, a run, a mention, a component and search rows.
  Future<void> fill(int c, {int s = 1}) async {
    await db.structureDao.upsertSpaceIfNewer(space(s));
    await db.structureDao.upsertChannelIfNewer(channel(c, spaceN: s));
    await db.structureDao.putForumTags(channel(c).id, [
      ForumTag(
        id: ForumTagId.parse(uuid(0x600 + c)),
        channelId: channel(c).id,
        name: 'bug',
        sortKey: 'a',
        restricted: false,
      ),
    ]);
    final t = thread(c, channelN: c, lastSeq: 2);
    await db.threadsDao.upsertIfNewer(t);
    for (final seq in [1, 2]) {
      final m = message(c, seq, channelN: c, text: 'Hello from channel $c');
      await db.messagesDao.upsertIfNewer(m);
      await db.searchDao.indexMessage(m, spaceId: space(s).id);
    }
    await db.searchDao.indexThreadTitle(t, spaceId: space(s).id);
    await db.messagesDao.addLoadedRange(
      t.id,
      const ThreadSeq(1),
      const ThreadSeq(2),
    );
    await db.messagesDao.putFile(file(c, channelN: c));
    await db.messagesDao.putAttachments(messageId(c, 1), [
      MessageAttachment(
        messageId: messageId(c, 1),
        fileId: file(c).id,
        position: 0,
        threadId: t.id,
        channelId: channel(c).id,
      ),
    ]);
    await db.fileCacheDao.put(
      FileCacheEntry(
        fileId: file(c).id,
        variant: 'original',
        path: 'cache/file-$c',
        sizeBytes: 100,
        lastAccessedAt: at(0),
      ),
    );
    await db.messagesDao.applyReactionSummary(
      ReactionSummary(
        messageId: messageId(c, 1),
        emoji: '👍',
        count: 1,
        me: true,
      ),
    );
    await db.messagesDao.putMention(
      Mention(messageId: messageId(c, 2), principalId: sam, threadId: t.id),
    );
    await db.messagesDao.upsertComponentIfNewer(
      MessageComponent(
        id: ComponentId.parse(uuid(0x800 + c)),
        messageId: messageId(c, 2),
        customId: 'approve',
        type: ComponentType.button,
        definition: const {'label': 'Approve'},
        position: 0,
        singleUse: true,
        restrictedTo: ComponentRestriction.any,
        state: ComponentState.active,
        version: const ResourceVersion(1),
      ),
    );
    await db.runsDao.upsertIfNewer(
      RunSummary(
        id: RunId.parse(uuid(0x700 + c)),
        threadId: t.id,
        channelId: channel(c).id,
        agentId: robin,
        triggeredBy: alex,
        state: RunState.completed,
        version: const ResourceVersion(1),
        billingModeExpected: BillingMode.personal,
        cost: const Money(amountMicros: 0, currency: 'EUR', isEstimate: false),
        stepCount: 1,
        queuedAt: at(1),
      ),
    );
    await db.personalDao.upsertDraftIfNewer(draft(c, threadN: c, channelN: c));
    await db.personalDao.upsertDraftIfNewer(
      draft(c + 50, threadN: null, channelN: c),
    );
    await db.personalDao.applyReadState(
      ReadState(
        threadId: t.id,
        channelId: channel(c).id,
        lastReadSeq: const ThreadSeq(1),
        mentionCount: 0,
        version: const ResourceVersion(1),
      ),
    );
    await db.outboxDao.enqueue(
      id: uuid(0x900 + c),
      kind: 'postMessage',
      payload: '{}',
      threadId: t.id,
    );
  }

  Future<int> count(String table) async =>
      (await db.customSelect('SELECT count(*) AS n FROM $table').getSingle())
          .read<int>('n');

  test(
    'removing a channel deletes its content and returns the file paths',
    () async {
      await fill(1);
      await fill(2);
      final paths = await db.removeTargets([TargetRef.channel(channel(1).id)]);
      expect(paths, ['cache/file-1']);

      expect(await db.structureDao.channelById(channel(1).id), isNull);
      expect(await db.threadsDao.byId(threadId(1)), isNull);
      expect(await db.messagesDao.byId(messageId(1, 1)), isNull);
      expect(await db.messagesDao.fileById(file(1).id), isNull);
      expect(await db.fileCacheDao.entry(file(1).id, 'original'), isNull);
      expect(await db.messagesDao.componentsOf(messageId(1, 2)), isEmpty);
      expect(await db.messagesDao.mentionsIn(threadId(1)), isEmpty);
      expect(await db.personalDao.readState(threadId(1)), isNull);
      expect((await db.messagesDao.loadedRanges(threadId(1))).ranges, isEmpty);
      final hits = await db.searchDao.search('channel', limit: 10);
      expect({for (final h in hits) h.channelId}, {channel(2).id});
      expect([
        for (final d in await db.personalDao.allDrafts()) d.channelId,
      ], everyElement(channel(2).id));
      // Outbox entries stay: they fail with 404 and keep their text (04 8.3 item 4).
      expect(await db.outboxDao.byId(uuid(0x901)), isNotNull);

      // The other channel is untouched.
      expect(await db.structureDao.channelById(channel(2).id), isNotNull);
      expect(await db.messagesDao.byId(messageId(2, 2)), isNotNull);
      expect(await db.fileCacheDao.entry(file(2).id, 'original'), isNotNull);
      expect(await count('runs'), 1);
      expect(await count('reactions_summary'), 1);
      expect(await count('forum_tags'), 1);
      expect(await count('channel_operators'), 1);
    },
  );

  test(
    'removing a space deletes its channels; removing a thread only the thread',
    () async {
      await fill(1, s: 1);
      await fill(2, s: 2);
      await fill(3, s: 2);
      final paths = await db.removeTargets([
        TargetRef.space(space(2).id),
        const TargetRef(
          RemovedTargetKind.host,
          '0192f4a0-7b1c-78f9-8074-000000000001',
        ),
      ]);
      expect(paths, unorderedEquals(['cache/file-2', 'cache/file-3']));
      expect(await db.structureDao.spaceById(space(2).id), isNull);
      expect(await count('channels'), 1);
      expect(await db.structureDao.spaceById(space(1).id), isNotNull);

      expect(await db.removeTargets([TargetRef.thread(threadId(1))]), [
        'cache/file-1',
      ]);
      expect(await db.threadsDao.byId(threadId(1)), isNull);
      expect(await db.structureDao.channelById(channel(1).id), isNotNull);
      expect(await count('messages'), 0);
    },
  );

  test('removing a space deletes its collapse state on this device', () async {
    await fill(1, s: 1);
    await fill(2, s: 2);
    for (final n in [1, 2]) {
      await db.forumViewDao.setSpaceCollapsed(space(n).id, collapsed: true);
    }
    await db.removeTargets([TargetRef.space(space(2).id)]);
    expect(await db.forumViewDao.watchCollapsedSpaces().first, {space(1).id});
  });

  test(
    'removing a channel or its space deletes the state of its thread group',
    () async {
      await fill(1, s: 1);
      await fill(2, s: 2);
      await fill(3, s: 1);
      for (final c in [1, 2, 3]) {
        await db.forumViewDao.setGroupCollapsed(channel(c).id, collapsed: true);
      }
      await db.forumViewDao.setOverviewPinned(pinned: true);

      await db.removeTargets([TargetRef.channel(channel(1).id)]);
      expect(await db.forumViewDao.watchCollapsedGroups().first, {
        channel(2).id,
        channel(3).id,
      });

      await db.removeTargets([TargetRef.space(space(2).id)]);
      expect(await db.forumViewDao.watchCollapsedGroups().first, {
        channel(3).id,
      });
      // The pinned overview belongs to the device, not to a target.
      expect(await db.forumViewDao.overviewPinned(), isTrue);
    },
  );

  test('a removal that fails midway changes nothing', () async {
    await fill(1);
    await db.customStatement(
      "CREATE TRIGGER fail_threads BEFORE DELETE ON threads BEGIN SELECT RAISE(ABORT, 'x'); END",
    );
    await expectLater(
      db.removeTargets([TargetRef.channel(channel(1).id)]),
      throwsA(anything),
    );
    expect(await db.messagesDao.byId(messageId(1, 1)), isNotNull);
    expect(await db.fileCacheDao.entry(file(1).id, 'original'), isNotNull);
    expect(await count('search_index'), 3);
  });

  test(
    'a file attached in another thread keeps its metadata and cache row',
    () async {
      await fill(1);
      await fill(2);
      await db.messagesDao.putAttachments(messageId(2, 1), [
        MessageAttachment(
          messageId: messageId(2, 1),
          fileId: file(1).id,
          position: 0,
          threadId: threadId(2),
          channelId: channel(2).id,
        ),
      ]);
      expect(
        await db.removeTargets([TargetRef.channel(channel(1).id)]),
        isEmpty,
      );
      expect(await db.messagesDao.fileById(file(1).id), isNotNull);
    },
  );

  test(
    'removing a channel deletes messages whose thread row is not stored',
    () async {
      await fill(1);
      await fill(2);
      // The cleanup dropped the thread row; a later message.created stored a message again.
      await db.transaction(
        () => db.customStatement(
          "DELETE FROM threads WHERE id = '${threadId(1).value}'",
        ),
      );
      await db.messagesDao.upsertIfNewer(message(1, 3, text: 'secret three'));
      await db.removeTargets([TargetRef.space(space(1).id)]);
      for (final table in [
        'messages',
        'message_attachments',
        'runs',
        'read_states',
        'reactions_summary',
        'my_mentions',
        'message_components',
      ]) {
        expect(await count(table), 0, reason: table);
      }
      expect(await db.searchDao.search('secret', limit: 10), isEmpty);
    },
  );

  test('removing a channel deletes its files when no message attaches them any more', () async {
    await fill(1);
    await fill(2);
    // An edit dropped the attachment; metadata and cached bytes of the file stay.
    await db.messagesDao.putAttachments(messageId(1, 1), const []);
    final paths = await db.removeTargets([TargetRef.channel(channel(1).id)]);
    expect(paths, ['cache/file-1']);
    expect(await db.messagesDao.fileById(file(1).id), isNull);
    expect(await db.fileCacheDao.entry(file(1).id, 'original'), isNull);
    expect(await db.messagesDao.fileById(file(2).id), isNotNull);
  });
}
