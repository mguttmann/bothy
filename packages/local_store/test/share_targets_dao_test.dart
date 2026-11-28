// SPDX-License-Identifier: Apache-2.0

// The reads of the share sheet (brief M5-06 Design notes 1 and 7): the threads the person last
// wrote into and the writable channels, against the real schema with bound values, so a renamed
// column fails here and not at run time in the app.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

Channel _channel(int n, {ChannelKind kind = ChannelKind.text}) {
  final c = channel(n);
  return Channel(
    id: c.id,
    spaceId: c.spaceId,
    kind: kind,
    name: 'Channel $n',
    topic: c.topic,
    sortKey: c.sortKey,
    operator: c.operator,
    readOnly: false,
    mainThreadId: c.mainThreadId,
    dmParticipantIds: c.dmParticipantIds,
    createdAt: c.createdAt,
    version: c.version,
    settingsVersion: c.settingsVersion,
  );
}

void main() {
  late LocalDatabase db;

  Future<void> put(int n, AccessLevel level, {ChannelKind? kind}) async {
    final c = _channel(n, kind: kind ?? ChannelKind.text);
    await db.structureDao.upsertChannelIfNewer(c);
    await db.structureDao.putChannelAccess(
      c.id,
      AccessView(level: level, capabilities: const []),
    );
  }

  setUp(() async {
    db = openInMemoryForTests();
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await put(1, AccessLevel.trigger);
    await put(2, AccessLevel.write, kind: ChannelKind.forum);
    await put(3, AccessLevel.read);
    await db.threadsDao.upsertIfNewer(thread(1, channelN: 1, title: 'One'));
    await db.threadsDao.upsertIfNewer(thread(2, channelN: 2, title: 'Two'));
    await db.threadsDao.upsertIfNewer(thread(3, channelN: 3, title: 'Three'));
    await db.messagesDao.upsertIfNewer(message(1, 1));
    await db.messagesDao.upsertIfNewer(message(2, 2, channelN: 2));
    await db.messagesDao.upsertIfNewer(message(3, 3, channelN: 3));
  });
  tearDown(() => db.close());

  test('recentWrites gives the writable threads of the author, newest first, '
      'at most max', () async {
    final rows = await db.shareTargetsDao.recentWrites(
      me: alex.value,
      max: 8,
    );
    expect([for (final r in rows) r.threadId], [
      threadId(2).value,
      threadId(1).value,
    ]);
    expect(rows.first.channelName, 'Channel 2');
    expect(rows.first.title, 'Two');
    expect(
      await db.shareTargetsDao.recentWrites(me: alex.value, max: 1),
      hasLength(1),
    );
    expect(
      await db.shareTargetsDao.recentWrites(me: alex.value, max: 0),
      isEmpty,
    );
    expect(
      await db.shareTargetsDao.recentWrites(me: sam.value, max: 8),
      isEmpty,
    );
  });

  test('the author is a bound value, never SQL text', () async {
    final rows = await db.shareTargetsDao.recentWrites(
      me: "x' OR '1'='1",
      max: 8,
    );
    expect(rows, isEmpty);
  });

  test('writableChannels leaves out a channel without the write level', () async {
    final rows = await db.shareTargetsDao.writableChannels();
    expect(
      [for (final r in rows) (r.name, r.kind)],
      [('Channel 1', 'text'), ('Channel 2', 'forum')],
    );
  });

  test('watchChannels reports a change of the stored channels', () async {
    final seen = <void>[];
    final sub = db.shareTargetsDao.watchChannels().listen(seen.add);
    await pumpEventQueue();
    expect(seen, hasLength(1));
    await put(3, AccessLevel.write);
    await pumpEventQueue();
    expect(seen.length, greaterThan(1));
    await sub.cancel();
  });
}
