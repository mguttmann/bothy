// SPDX-License-Identifier: Apache-2.0

// The version rule (04 9 item 3, 15 5): a durable change applies only with a greater version;
// reactions and pins carry absolute values and leave `messages.version` alone.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  test(
    'an older or equal version does not overwrite, a newer one does',
    () async {
      final dao = db.messagesDao;
      expect(
        await dao.upsertIfNewer(message(1, 1, version: 2, text: 'v2')),
        isTrue,
      );
      expect(
        await dao.upsertIfNewer(message(1, 1, version: 1, text: 'v1')),
        isFalse,
      );
      expect(
        await dao.upsertIfNewer(message(1, 1, version: 2, text: 'again')),
        isFalse,
      );
      expect((await dao.byId(messageId(1, 1)))!.contentMd, 'v2');
      expect(
        await dao.upsertIfNewer(message(1, 1, version: 3, text: 'v3')),
        isTrue,
      );
      expect(
        await dao.byId(messageId(1, 1)),
        message(1, 1, version: 3, text: 'v3'),
      );
    },
  );

  test('every versioned mirror table follows the rule', () async {
    final s = db.structureDao;
    expect(await s.upsertSpaceIfNewer(space(1, version: 2)), isTrue);
    expect(await s.upsertSpaceIfNewer(space(1).copyWith(name: 'old')), isFalse);
    expect((await s.spaceById(space(1).id))!.name, 'Space 1');

    expect(await s.upsertChannelIfNewer(channel(1, version: 2)), isTrue);
    final older = channel(1)
        .copyWith(operator: const ChannelOperator(mode: OperatorMode.none));
    expect(await s.upsertChannelIfNewer(older), isFalse);
    expect(await s.channelById(channel(1).id), channel(1, version: 2));

    final t = db.threadsDao;
    final tagged = thread(
      1,
      version: 2,
    ).copyWith(tagIds: [ForumTagId.parse(uuid(0x601))]);
    expect(await t.upsertIfNewer(tagged), isTrue);
    expect(await t.upsertIfNewer(thread(1, title: 'stale')), isFalse);
    expect(await t.byId(threadId(1)), tagged);

    final r = db.runsDao;
    final run = RunSummary(
      id: RunId.parse(uuid(0x700)),
      threadId: threadId(1),
      channelId: channel(1).id,
      agentId: robin,
      triggeredBy: alex,
      state: RunState.running,
      version: const ResourceVersion(2),
      billingModeExpected: BillingMode.organization,
      cost: const Money(amountMicros: 1200, currency: 'EUR', isEstimate: true),
      stepCount: 3,
      queuedAt: at(1),
    );
    expect(await r.upsertIfNewer(run), isTrue);
    expect(
      await r.upsertIfNewer(
        run.copyWith(state: RunState.queued, version: const ResourceVersion(1)),
      ),
      isFalse,
    );
    expect(await r.byId(run.id), run);
  });

  test(
    'reactions apply absolute values without touching messages.version',
    () async {
      final dao = db.messagesDao;
      await dao.upsertIfNewer(message(1, 1, version: 5));
      final id = messageId(1, 1);
      await dao.applyReactionSummary(
        ReactionSummary(messageId: id, emoji: '👍', count: 2, me: true),
      );
      await dao.applyReactionSummary(
        ReactionSummary(messageId: id, emoji: '👍', count: 1, me: false),
      );
      expect(await dao.watchReactions(id).first, [
        ReactionSummary(messageId: id, emoji: '👍', count: 1, me: false),
      ]);
      await dao.applyReactionSummary(
        ReactionSummary(messageId: id, emoji: '👍', count: 0, me: false),
      );
      expect(await dao.watchReactions(id).first, isEmpty);
      expect((await dao.byId(id))!.version, const ResourceVersion(5));
    },
  );

  test(
    'pins apply absolute values without touching messages.version',
    () async {
      final dao = db.messagesDao;
      await dao.upsertIfNewer(message(1, 1, version: 5));
      await dao.applyPin(messageId(1, 1), pinned: true);
      var stored = await dao.byId(messageId(1, 1));
      expect(stored!.pinned, isTrue);
      expect(stored.version, const ResourceVersion(5));
      await dao.applyPin(messageId(1, 1), pinned: false);
      stored = await dao.byId(messageId(1, 1));
      expect(stored!.pinned, isFalse);
      expect(stored.version, const ResourceVersion(5));
    },
  );

  test('a tombstone keeps thread_seq, clears the content and needs a newer version', () async {
    final dao = db.messagesDao;
    await dao.upsertIfNewer(message(1, 4, version: 2));
    expect(
      await dao.tombstone(
        messageId(1, 4),
        deletedAt: at(9),
        version: const ResourceVersion(2),
      ),
      isFalse,
    );
    expect(
      await dao.tombstone(
        messageId(1, 4),
        deletedAt: at(9),
        version: const ResourceVersion(3),
      ),
      isTrue,
    );
    final stored = (await dao.byId(messageId(1, 4)))!;
    expect(stored.threadSeq, const ThreadSeq(4));
    expect(stored.contentMd, isEmpty);
    expect(stored.mentionIds, isEmpty);
    expect(stored.deletedAt, at(9));
    expect(stored.version, const ResourceVersion(3));
  });
}
