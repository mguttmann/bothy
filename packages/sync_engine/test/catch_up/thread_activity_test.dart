// SPDX-License-Identifier: Apache-2.0

// A new message advances the stored `last_activity_at` of its thread (brief
// FIX-autoarrange-delay). The server moves `threads.last_activity_at` with every message (04 3.2)
// but writes only `message.created`, no `thread.updated` (04 6.3 item 4, R-SY-28: the client keeps
// its threads current from `thread.*` and `message.created`). The sidebar orders forum posts by
// that time, so without this a post moved up only when a later REST read of the thread replaced
// the row, minutes after the message.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final t1 = ThreadId.parse(threadId(1));
  final t2 = ThreadId.parse(threadId(2));
  const later = '2026-10-15T11:00:00.000Z';
  const latest = '2026-10-15T12:00:00.000Z';

  Map<String, Object?> withCreatedAt(Map<String, Object?> e, String time) {
    final payload = (e['payload']! as Map).cast<String, Object?>();
    final message = (payload['message']! as Map).cast<String, Object?>();
    message['created_at'] = time;
    return e;
  }

  Future<DateTime> activity(ThreadId id) async =>
      (await w.db.threadsDao.byId(id))!.lastActivityAt;

  Future<List<ThreadId>> byActivity() async => [
    for (final t
        in await w.db.threadsDao
            .watchForChannel(ChannelId.parse(channelId(1)))
            .first)
      t.id,
  ];

  Future<void> connectWithTwoThreads() async {
    final second = threadJson(2)..['last_activity_at'] = later;
    await w.connect(
      snapshot: snapshotJson(activeThreads: [threadJson(1), second]),
    );
    expect(await byActivity(), [t2, t1]);
  }

  test('message.created moves its thread to the top at once, without a '
      'thread event and without touching the thread version', () async {
    await connectWithTwoThreads();
    await w.dispatch([withCreatedAt(messageCreated(41, 1), latest)]);
    expect(await activity(t1), DateTime.parse(latest));
    expect(await byActivity(), [t1, t2]);
    expect((await w.db.threadsDao.byId(t1))!.version.value, 1);

    // A thread event with a newer version still applies (the version rule is untouched).
    await w.dispatch([
      envelope(
        'thread.updated',
        seq: 42,
        payload: {'thread': threadJson(1, version: 2, title: 'Renamed')},
        resourceKind: 'thread',
        resourceId: threadId(1),
        version: 2,
        scope: {'channel_id': channelId(1), 'thread_id': threadId(1)},
      ),
    ]);
    expect((await w.db.threadsDao.byId(t1))!.title, 'Renamed');
  });

  test('an older message (an edit, a loaded page) never moves the activity '
      'back', () async {
    await connectWithTwoThreads();
    await w.dispatch([
      withCreatedAt(messageCreated(41, 1, threadN: 2), latest),
    ]);
    expect(await activity(t2), DateTime.parse(latest));
    await w.dispatch([
      withCreatedAt(messageCreated(42, 2, threadN: 2), later),
      messageUpdated(43, 1, threadN: 2, version: 2, content: 'edited'),
    ]);
    expect(await activity(t2), DateTime.parse(latest));
  });

  test('the 201 of an own message moves the thread before its echo', () async {
    await connectWithTwoThreads();
    final own = messageJson(1, 1, author: alexId)..['created_at'] = latest;
    await w.catchUp.storeConfirmedMessage(own, created: true);
    expect(await activity(t1), DateTime.parse(latest));
    expect(await byActivity(), [t1, t2]);
  });
}
