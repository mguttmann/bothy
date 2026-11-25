// SPDX-License-Identifier: Apache-2.0

// The thread list of a channel carries the tags of each thread (15 6.1 `thread_tags`), read
// with one query for the whole channel.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  ForumTagId tag(int n) => ForumTagId.parse(uuid(0x900 + n));

  test('watchForChannel hands out the tags of every thread of the channel', () async {
    await db.threadsDao.upsertIfNewer(
      thread(1).copyWith(tagIds: [tag(2), tag(1)]),
    );
    await db.threadsDao.upsertIfNewer(thread(2));
    await db.threadsDao.upsertIfNewer(thread(3).copyWith(tagIds: [tag(3)]));
    // A thread of another channel with a tag of its own stays out of the list.
    await db.threadsDao.upsertIfNewer(
      thread(4, channelN: 2).copyWith(tagIds: [tag(4)]),
    );
    final list = await db.threadsDao.watchForChannel(thread(1).channelId).first;
    final byId = {for (final t in list) t.id: t.tagIds};
    expect(byId.keys, unorderedEquals([threadId(1), threadId(2), threadId(3)]));
    expect(byId[threadId(1)], [tag(1), tag(2)]);
    expect(byId[threadId(2)], isEmpty);
    expect(byId[threadId(3)], [tag(3)]);
  });
}
