// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late UnreadCounter counter;
  setUp(() => counter = UnreadCounter());

  void apply(MessageEvent e, {int read = 0, int floor = 0}) => counter.apply(
    e,
    me: alex,
    lastReadSeq: ThreadSeq(read),
    unreadFloorSeq: ThreadSeq(floor),
  );

  test('a message of someone else above the read state counts', () {
    apply(MessageCreated(message(5)));
    expect(
      counter.countsOf(thread1),
      const ThreadCounts(unread: 1, mentions: 0),
    );
  });

  test('an own message is not counted', () {
    apply(MessageCreated(message(5, author: alex)));
    expect(counter.countsOf(thread1), ThreadCounts.zero);
  });

  test('a message at or below the read state is not counted', () {
    apply(MessageCreated(message(5)), read: 5);
    apply(MessageCreated(message(4)), read: 5);
    expect(counter.countsOf(thread1), ThreadCounts.zero);
  });

  test('a mention of me counts as mention', () {
    apply(MessageCreated(message(5, mentions: [alex, robin])));
    expect(
      counter.countsOf(thread1),
      const ThreadCounts(unread: 1, mentions: 1),
    );
  });

  test('a mention added by an edit counts, a removed one is taken back', () {
    apply(MessageCreated(message(5)));
    apply(
      MessageUpdated(
        message(5, mentions: [alex], version: 2),
        previous: message(5),
      ),
    );
    expect(counter.countsOf(thread1).mentions, 1);
    apply(
      MessageUpdated(
        message(5, version: 3),
        previous: message(5, mentions: [alex], version: 2),
      ),
    );
    expect(counter.countsOf(thread1).mentions, 0);
  });

  test('an edit that keeps a mention counted by the server does not count it again', () {
    counter.resetFrom(
      readState(4, mentions: 1, unread: 1),
      lastMessageSeq: const ThreadSeq(5),
    );
    final before = message(5, mentions: [alex]);
    apply(
      MessageUpdated(
        message(5, mentions: [alex], version: 2),
        previous: before,
      ),
      read: 4,
    );
    expect(
      counter.countsOf(thread1),
      const ThreadCounts(unread: 1, mentions: 1),
    );
    apply(
      MessageUpdated(
        message(5, version: 3),
        previous: message(5, mentions: [alex], version: 2),
      ),
      read: 4,
    );
    expect(
      counter.countsOf(thread1),
      const ThreadCounts(unread: 1, mentions: 0),
    );
  });

  test(
    'an edit of an unknown message without its previous state changes nothing',
    () {
      apply(MessageUpdated(message(5, mentions: [alex], version: 2)));
      expect(counter.countsOf(thread1), ThreadCounts.zero);
    },
  );

  test('an edit at or below the read state changes nothing', () {
    apply(MessageUpdated(message(5, mentions: [alex])), read: 5);
    apply(MessageUpdated(message(6, author: alex, mentions: [alex])));
    expect(counter.countsOf(thread1), ThreadCounts.zero);
  });

  test('a deleted message decrements', () {
    apply(MessageCreated(message(5, mentions: [alex])));
    apply(MessageCreated(message(6)));
    apply(
      MessageDeleted(
        messageId: message(5).id,
        threadId: thread1,
        threadSeq: const ThreadSeq(5),
      ),
    );
    expect(
      counter.countsOf(thread1),
      const ThreadCounts(unread: 1, mentions: 0),
    );
  });

  test('a tombstone in message.updated decrements, a created tombstone never counts', () {
    apply(MessageCreated(message(5, mentions: [alex])));
    apply(MessageUpdated(message(5, deletedAt: t0, version: 2)));
    apply(MessageCreated(message(7, deletedAt: t0)));
    expect(counter.countsOf(thread1), ThreadCounts.zero);
  });

  test('an imported message at or below unread_floor_seq is not counted', () {
    apply(MessageCreated(message(5, mentions: [alex])), floor: 5);
    expect(counter.countsOf(thread1), ThreadCounts.zero);
    apply(MessageCreated(message(6)), floor: 5);
    expect(counter.countsOf(thread1).unread, 1);
  });

  test('resetFrom takes the server values', () {
    apply(MessageCreated(message(5, mentions: [alex])));
    counter.resetFrom(
      readState(3, mentions: 4, unread: 7),
      lastMessageSeq: const ThreadSeq(8),
    );
    expect(
      counter.countsOf(thread1),
      const ThreadCounts(unread: 7, mentions: 4),
    );
    apply(MessageCreated(message(9)), read: 3);
    expect(
      counter.countsOf(thread1),
      const ThreadCounts(unread: 8, mentions: 4),
    );
  });

  test('resetFrom drops buffered entries the new read boundary covers', () {
    // Seen live above the stored last_message_seq, then read elsewhere: the server's
    // read state (last_read_seq 15) covers both, so neither may stay counted on top.
    apply(MessageCreated(message(12, mentions: [alex])));
    apply(MessageCreated(message(13)));
    counter.resetFrom(
      readState(15, mentions: 0, unread: 0),
      lastMessageSeq: const ThreadSeq(10),
    );
    expect(counter.countsOf(thread1), ThreadCounts.zero);
  });

  test('resetFrom drops buffered entries below a raised unread floor', () {
    apply(MessageCreated(message(12, mentions: [alex])));
    apply(MessageCreated(message(13)));
    counter.resetFrom(
      readState(3, mentions: 0, unread: 0),
      lastMessageSeq: const ThreadSeq(10),
      unreadFloorSeq: const ThreadSeq(14),
    );
    expect(counter.countsOf(thread1), ThreadCounts.zero);
  });

  test(
    'resetFrom without an unread count computes it as 03 read_states does',
    () {
      counter.resetFrom(
        readState(3, unread: 7),
        lastMessageSeq: const ThreadSeq(10),
      );
      counter.resetFrom(
        readState(5, version: 2),
        lastMessageSeq: const ThreadSeq(10),
      );
      expect(counter.countsOf(thread1).unread, 5);
      counter.resetFrom(
        readState(5, version: 3),
        lastMessageSeq: const ThreadSeq(10),
        unreadFloorSeq: const ThreadSeq(8),
      );
      expect(counter.countsOf(thread1).unread, 2);
      counter.resetFrom(
        readState(12, version: 4),
        lastMessageSeq: const ThreadSeq(10),
      );
      expect(counter.countsOf(thread1).unread, 0);
    },
  );

  test('deleting a message whose mention the server counted takes the mention back', () {
    counter.resetFrom(
      readState(4, mentions: 2, unread: 2),
      lastMessageSeq: const ThreadSeq(6),
    );
    apply(
      MessageDeleted(
        messageId: message(5).id,
        threadId: thread1,
        threadSeq: const ThreadSeq(5),
        previous: message(5, mentions: [alex]),
      ),
      read: 4,
    );
    expect(counter.countsOf(thread1).mentions, 1);
    apply(
      MessageUpdated(
        message(6, deletedAt: t0, version: 2),
        previous: message(6, mentions: [alex]),
      ),
      read: 4,
    );
    expect(counter.countsOf(thread1).mentions, 0);
  });

  test('deleting a message that never counted leaves the server mentions', () {
    counter.resetFrom(
      readState(4, mentions: 1, unread: 1),
      lastMessageSeq: const ThreadSeq(5),
    );
    apply(
      MessageDeleted(
        messageId: message(3).id,
        threadId: thread1,
        threadSeq: const ThreadSeq(3),
        previous: message(3, mentions: [alex]),
      ),
      read: 4,
    );
    apply(
      MessageDeleted(
        messageId: message(5).id,
        threadId: thread1,
        threadSeq: const ThreadSeq(5),
        previous: message(5),
      ),
      read: 4,
    );
    expect(counter.countsOf(thread1).mentions, 1);
  });

  test('threads count separately', () {
    final other = ThreadId.parse(uuid(11));
    apply(MessageCreated(message(5, thread: other)));
    expect(counter.countsOf(thread1), ThreadCounts.zero);
    expect(counter.countsOf(other).unread, 1);
    expect(
      counter.countsOf(other).toString(),
      'ThreadCounts(unread: 1, mentions: 0)',
    );
    expect(
      ThreadCounts.zero.hashCode,
      const ThreadCounts(unread: 0, mentions: 0).hashCode,
    );
  });
}
