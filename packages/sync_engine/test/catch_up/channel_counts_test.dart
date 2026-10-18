// SPDX-License-Identifier: Apache-2.0

// The counters per channel the sidebar shows (brief M4-19 Design note 1, 04 14): the engine
// knows the channel of every thread it counts (snapshot, read state, message events), and a
// page of `listChannelThreads` lands in the local store with its next cursor.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final ch1 = ChannelId.parse(channelId(1));
  final ch2 = ChannelId.parse(channelId(2));
  final t1 = ThreadId.parse(threadId(1));
  final t2 = ThreadId.parse(threadId(2));
  final t3 = ThreadId.parse(threadId(3));

  Map<String, Object?> unread(int threadN, int channelN, int n, int m) => {
    'thread_id': threadId(threadN),
    'channel_id': channelId(channelN),
    'unread_count': n,
    'mention_count': m,
    'last_message_seq': 5,
    'last_read_seq': 5 - n,
  };

  test(
    'the counts of a channel cover every thread the engine counts there',
    () async {
      await w.connect(
        snapshot: snapshotJson(
          unread: [unread(1, 1, 2, 1), unread(2, 2, 1, 0)],
        ),
      );
      w.rest.routes['GET /me/read-states'] = (_) =>
          const FakeReply(200, {'items': <Object?>[]});
      await w.idle();
      expect(w.catchUp.me, PrincipalId.parse(alexId));
      expect(w.catchUp.countsOfChannel(ch1), {
        t1: const ThreadCounts(unread: 2, mentions: 1),
      });
      expect(w.catchUp.countsOfChannel(ch2), {
        t2: const ThreadCounts(unread: 1, mentions: 0),
      });

      await w.push('catchup.complete', {'head_seq': 40});
      // A thread the device never stored: the message names its channel.
      await w.dispatch([
        messageCreated(41, 1, threadN: 3, mentions: [alexId]),
      ]);
      expect(
        w.catchUp.countsOfChannel(ch1)[t3],
        const ThreadCounts(unread: 1, mentions: 1),
      );
      expect(w.catchUp.countsOfChannel(ch2).keys, [t2]);
    },
  );

  test('a page of threads is stored and its next cursor returned', () async {
    await w.connect();
    w.rest.routes['GET /me/read-states'] = (_) =>
        const FakeReply(200, {'items': <Object?>[]});
    await w.idle();
    final queries = <Map<String, String>>[];
    w.rest.routes['GET /channels/${channelId(1)}/threads'] = (r) {
      queries.add(r.query);
      return FakeReply(200, {
        'items': [threadJson(7, title: 'Printer'), threadJson(8)],
        'next_cursor': 'page-2',
      });
    };
    final next = await w.catchUp.loadThreadPage(
      ch1,
      sort: ForumSort.created,
      cursor: 'page-1',
      limit: 2,
    );
    expect(next, 'page-2');
    expect(queries.single, {
      'sort': 'created',
      'cursor': 'page-1',
      'limit': '2',
    });
    expect(
      [for (final t in await w.db.threadsDao.watchForChannel(ch1).first) t.id],
      unorderedEquals([
        ThreadId.parse(threadId(7)),
        ThreadId.parse(threadId(8)),
      ]),
    );

    // The last page has no cursor; a channel this device does not hold stores nothing.
    w.rest.routes['GET /channels/${channelId(9)}/threads'] = (_) =>
        FakeReply(200, {
          'items': [threadJson(9, channelN: 9)],
        });
    expect(
      await w.catchUp.loadThreadPage(ChannelId.parse(channelId(9))),
      isNull,
    );
    expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(9))), isNull);
  });

  test('a confirmed new thread is stored by the version rule and indexed, '
      'never for a channel this device does not hold', () async {
    await w.connect();
    w.rest.routes['GET /me/read-states'] = (_) =>
        const FakeReply(200, {'items': <Object?>[]});
    await w.idle();
    await w.catchUp.storeConfirmedThread(
      threadJson(7, title: 'Printer jam', version: 2),
    );
    expect(
      (await w.db.threadsDao.byId(ThreadId.parse(threadId(7))))?.title,
      'Printer jam',
    );
    expect(await w.db.searchDao.search('printer', limit: 10), isNotEmpty);
    // The older `thread.created` that follows changes nothing.
    await w.catchUp.storeConfirmedThread(
      threadJson(7, title: 'Old', version: 1),
    );
    expect(
      (await w.db.threadsDao.byId(ThreadId.parse(threadId(7))))?.title,
      'Printer jam',
    );
    await w.catchUp.storeConfirmedThread(threadJson(8, channelN: 9));
    expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(8))), isNull);
  });

  test('after a reconnect the catch-up does not count again what the snapshot holds', () async {
    // Offline, Sam wrote 31 and 32 in thread 1 (32 mentions Alex); the snapshot holds both.
    await w.connect(
      cursor: 30,
      headSeq: 42,
      snapshot: snapshotJson(
        headSeq: 42,
        unread: [
          {
            'thread_id': threadId(1),
            'channel_id': channelId(1),
            'unread_count': 2,
            'mention_count': 1,
            'last_message_seq': 2,
            'last_read_seq': 0,
          },
        ],
      ),
    );
    w.rest.routes['GET /me/read-states'] = (_) =>
        const FakeReply(200, {'items': <Object?>[]});
    await w.dispatch([
      messageCreated(41, 1),
      messageCreated(42, 2, mentions: [alexId]),
    ]);
    await w.push('catchup.complete', {'head_seq': 42});
    expect(
      w.catchUp.countsOfChannel(ch1)[t1],
      const ThreadCounts(unread: 2, mentions: 1),
    );
    // A live message after the snapshot counts.
    await w.dispatch([
      messageCreated(43, 3, mentions: [alexId]),
    ]);
    expect(
      w.catchUp.countsOfChannel(ch1)[t1],
      const ThreadCounts(unread: 3, mentions: 2),
    );
  });

  // Known repros of the replay class (rt-M4-19, D-M4-19-10).
  Map<String, Object?> unreadAt(int unread, int mentions, {int lastSeq = 2}) =>
      {
        'thread_id': threadId(1),
        'channel_id': channelId(1),
        'unread_count': unread,
        'mention_count': mentions,
        'last_message_seq': lastSeq,
        'last_read_seq': 0,
      };

  Map<String, Object?> readStateUpdated(
    int seq,
    int lastRead,
    int version,
    int mentions,
  ) => envelope(
    'read_state.updated',
    seq: seq,
    payload: {
      'read_state': readStateJson(
        1,
        lastRead: lastRead,
        version: version,
        mentions: mentions,
        device: otherDevice,
      ),
    },
    resourceKind: 'thread',
    resourceId: threadId(1),
    personal: true,
  );

  Map<String, Object?> deleted(int seq, int threadSeq) => envelope(
    'message.deleted',
    seq: seq,
    payload: {
      'message_id': messageId(1, threadSeq),
      'thread_id': threadId(1),
      'thread_seq': threadSeq,
      'deleted_at': at,
      'deleted_by': samId,
    },
    resourceKind: 'message',
    resourceId: messageId(1, threadSeq),
    version: 2,
    scope: {'channel_id': channelId(1), 'thread_id': threadId(1)},
  );

  Future<void> reconnect(Map<String, Object?> unread, int head) async {
    w.rest.routes['GET /me/read-states'] = (_) =>
        const FakeReply(200, {'items': <Object?>[]});
    await w.connect(
      cursor: 30,
      headSeq: head,
      snapshot: snapshotJson(headSeq: head, unread: [unread]),
    );
  }

  test(
    'a replayed read state does not reset the counter below the snapshot',
    () async {
      // Delta red team: read on the phone at 35, Sam mentions Alex at 40, head 42.
      await reconnect(unreadAt(1, 1), 42);
      await w.dispatch([
        messageCreated(33, 1),
        readStateUpdated(35, 1, 2, 0),
        messageCreated(40, 2, mentions: [alexId]),
      ]);
      await w.push('catchup.complete', {'head_seq': 42});
      expect(
        w.catchUp.countsOf(t1),
        const ThreadCounts(unread: 1, mentions: 1),
      );
    },
  );

  test(
    'a replayed deletion and edit do not take a mention back twice',
    () async {
      // Red team round 1: Sam deletes the stored mention 1 and edits 2 offline, then mentions
      // Alex again in 3; the snapshot holds all of it.
      await w.connect(
        cursor: 0,
        headSeq: 0,
        snapshot: snapshotJson(headSeq: 0),
      );
      await w.push('catchup.complete', {'head_seq': 0});
      await w.dispatch([
        threadCreated(1, 1),
        messageCreated(2, 1, mentions: [alexId]),
        messageCreated(3, 2, mentions: [alexId]),
      ]);
      expect(
        w.catchUp.countsOf(t1),
        const ThreadCounts(unread: 2, mentions: 2),
      );
      w.ch.serverClose(4000);
      await w.idle();
      await w.h.clock.elapse(const Duration(minutes: 2));
      await w.idle();
      w.ch.push('hello', helloData(nonce: 'nonce-9'));
      await w.idle();
      w.rest.routes['GET /me/read-states'] = (_) =>
          const FakeReply(200, {'items': <Object?>[]});
      w.ch.push(
        'ready',
        readyData(headSeq: 6, gatewaySessionId: 'gs-2')
          ..['snapshot'] = snapshotJson(
            headSeq: 6,
            unread: [unreadAt(2, 1, lastSeq: 3)],
          ),
      );
      await w.idle();
      await w.dispatch([
        deleted(4, 1),
        envelope(
          'message.updated',
          seq: 5,
          payload: {
            'message': messageJson(1, 2, version: 2, content: 'no mention'),
          },
          resourceKind: 'message',
          resourceId: messageId(1, 2),
          version: 2,
          scope: {'channel_id': channelId(1), 'thread_id': threadId(1)},
        ),
        messageCreated(6, 3, mentions: [alexId]),
      ]);
      await w.push('catchup.complete', {'head_seq': 6});
      expect(
        w.catchUp.countsOf(t1),
        const ThreadCounts(unread: 2, mentions: 1),
      );
    },
  );

  test(
    'a live read on another device drops the mentions counted before it',
    () async {
      await reconnect(unreadAt(1, 1), 30);
      await w.push('catchup.complete', {'head_seq': 30});
      await w.dispatch([
        messageCreated(31, 3, mentions: [alexId]),
      ]);
      expect(
        w.catchUp.countsOf(t1),
        const ThreadCounts(unread: 2, mentions: 2),
      );
      await w.dispatch([readStateUpdated(32, 3, 2, 0)]);
      expect(w.catchUp.countsOf(t1), ThreadCounts.zero);
    },
  );

  test(
    'a live partial read keeps the unread messages above it counted',
    () async {
      // Class-fix red team: a read_state.updated carries no unread_count (openapi ReadState),
      // so its unread value is computed from the stored last_message_seq, which live messages
      // do not raise; the messages counted above the new read position stay counted.
      await w.connect();
      w.rest.routes['GET /me/read-states'] = (_) =>
          const FakeReply(200, {'items': <Object?>[]});
      await w.idle();
      await w.push('catchup.complete', {'head_seq': 40});
      await w.dispatch([messageCreated(41, 1), messageCreated(42, 2)]);
      expect(
        w.catchUp.countsOf(t1),
        const ThreadCounts(unread: 2, mentions: 0),
      );
      await w.dispatch([readStateUpdated(43, 1, 2, 0)]);
      expect(
        w.catchUp.countsOf(t1),
        const ThreadCounts(unread: 1, mentions: 0),
      );
    },
  );
}
