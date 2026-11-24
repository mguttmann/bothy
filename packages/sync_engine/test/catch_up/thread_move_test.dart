// SPDX-License-Identifier: Apache-2.0

// A thread move as the server writes it (04 8.1, ruling AUD-PKG-P04/E1): `thread.moved` to the
// target channel, then `thread.moved_out` to the source with the next `seq`. A reader of both
// channels keeps the thread with its messages in the target; a reader of only the source gets
// only `thread.moved_out` and loses the thread with its content (04 8.3 item 3); a reader of
// only the target gets only `thread.moved` and stores the thread.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final t1 = ThreadId.parse(threadId(1));

  Map<String, Object?> movedOut(int seq) => envelope(
    'thread.moved_out',
    seq: seq,
    payload: {'thread_id': threadId(1), 'channel_id': channelId(1)},
    resourceKind: 'channel',
    resourceId: channelId(1),
    scope: {'channel_id': channelId(1)},
  );

  Map<String, Object?> moved(int seq) => envelope(
    'thread.moved',
    seq: seq,
    payload: {
      'thread': threadJson(1, channelN: 2, version: 2),
      'from_channel_id': channelId(1),
    },
    resourceKind: 'thread',
    resourceId: threadId(1),
    version: 2,
    scope: {'channel_id': channelId(2), 'thread_id': threadId(1)},
  );

  Future<void> threadWithMessage() async {
    await w.connect(cursor: 30);
    await w.dispatch([
      threadCreated(31, 1),
      messageCreated(32, 1, content: 'hello'),
    ]);
    await w.push('cursor', {'seq': 32});
  }

  for (final split in [false, true]) {
    test('a reader of both channels keeps the thread and its messages, '
        'split pages: $split', () async {
      await threadWithMessage();
      if (split) {
        await w.dispatch([moved(33)]);
        await w.push('cursor', {'seq': 33});
        await w.dispatch([movedOut(34)]);
      } else {
        await w.dispatch([moved(33), movedOut(34)]);
      }
      await w.push('cursor', {'seq': 34});
      final stored = await w.db.threadsDao.byId(t1);
      expect(stored?.channelId.value, channelId(2));
      expect(await w.storedSeqs(1), [1]);
      expect(w.notices.whereType<TargetsRemoved>(), isEmpty);
      // The content moved with the thread: losing the source channel later leaves it alone.
      await w.dispatch([accessRemoved(35, 1)]);
      await w.push('cursor', {'seq': 35});
      expect((await w.db.threadsDao.byId(t1))?.channelId.value, channelId(2));
      expect(await w.storedSeqs(1), [1]);
      final hits = await w.db.searchDao.search('hello', limit: 5);
      expect([for (final h in hits) h.channelId?.value], [channelId(2)]);
    });
  }

  test(
    'a reader of only the source loses the thread and its messages',
    () async {
      await threadWithMessage();
      await w.dispatch([movedOut(34)]);
      await w.push('cursor', {'seq': 34});
      expect(await w.db.threadsDao.byId(t1), isNull);
      expect(await w.storedSeqs(1), isEmpty);
      expect(w.notices.whereType<TargetsRemoved>().single.targets, [
        TargetRef.thread(t1),
      ]);
    },
  );

  test('a reader of only the target stores the thread', () async {
    await w.connect(cursor: 30);
    await w.dispatch([moved(33)]);
    await w.push('cursor', {'seq': 33});
    expect((await w.db.threadsDao.byId(t1))?.channelId.value, channelId(2));
    expect(w.notices.whereType<TargetsRemoved>(), isEmpty);
  });
}
