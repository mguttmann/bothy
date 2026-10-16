// SPDX-License-Identifier: Apache-2.0

// Duplicates (04 9 item 2, brief M4-14 Tests): every event with `seq <= cursor` is discarded,
// so a page delivered twice, live events below the cursor and a live event repeated after a
// reconnect change nothing.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  test(
    'the same page delivered twice changes nothing the second time',
    () async {
      await w.connect(cursor: 30);
      final page = [threadCreated(31, 1), messageCreated(32, 1)];
      await w.dispatch(page);
      await w.push('cursor', {'seq': 33});
      expect(await w.cursor(), 33);
      expect(await w.storedSeqs(1), [1]);

      // The edit applies; the repeated page below the cursor must not undo it.
      await w.dispatch([messageUpdated(34, 1, version: 2, content: 'edited')]);
      await w.push('cursor', {'seq': 34});
      await w.dispatch(page);
      await w.push('cursor', {'seq': 33});
      final m = await w.db.messagesDao.byId(MessageId.parse(messageId(1, 1)));
      expect(m!.contentMd, 'edited');
      expect(await w.cursor(), 34, reason: 'the cursor never moves back');
    },
  );

  test('live events at or below the cursor are discarded', () async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 40});
    expect(w.h.phase, ConnectionPhase.live);
    expect(w.liveCalls, 1);
    await w.dispatch([threadCreated(39, 1), messageCreated(40, 1)]);
    expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(1))), isNull);
    expect(await w.storedSeqs(1), isEmpty);
    await w.dispatch([threadCreated(41, 1)]);
    expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(1))), isNotNull);
    expect(await w.cursor(), 41);
  });

  test('a live event repeated after a reconnect is applied once', () async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    await w.dispatch([
      threadCreated(31, 1),
      messageCreated(32, 1, mentions: [alexId]),
    ]);
    expect(await w.cursor(), 32);
    expect(w.catchUp.countsOf(ThreadId.parse(threadId(1))).unread, 1);

    w.ch.serverClose(4000);
    await w.idle();
    await w.h.clock.elapse(const Duration(minutes: 2));
    await w.idle();
    w.ch.push('hello', {
      'heartbeat_interval_ms': 10000,
      'resume_window_ms': 60000,
      'protocol_versions': [1],
      'dpop_nonce': 'nonce-9',
      'server_time': at,
      'sync_epoch': 'epoch-1',
    });
    await w.idle();
    w.ch.push('resumed', {
      'gateway_session_id': 'gs-1',
      'head_seq': 32,
      'sync_epoch': 'epoch-1',
    });
    await w.idle();
    // The server repeats the last event; the client already stored its cursor.
    await w.dispatch([
      messageCreated(32, 1, mentions: [alexId]),
    ]);
    await w.push('catchup.complete', {'head_seq': 32});
    expect(await w.storedSeqs(1), [1]);
    expect(
      w.catchUp.countsOf(ThreadId.parse(threadId(1))).unread,
      1,
      reason: 'counted once',
    );
    expect(await w.cursor(), 32);
  });
}
