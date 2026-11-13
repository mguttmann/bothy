// SPDX-License-Identifier: Apache-2.0

// A commit that fails no longer leaves the sync standing still on an open connection
// (AUD-pkg-sync-engine-01, findings sync-engine-store-1 and fn-sync-offline-2): the listener
// closes the connection and the next session repeats the page from the stored cursor; a page
// that fails again at the same cursor is passed by a snapshot; a snapshot of `ready` or of a
// resync that cannot be applied reconnects as well.

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

/// `thread.created` of thread [n] whose payload fails the mapping of its applier: a thread ID
/// that is not a UUID passes the generated decoder, which does not check formats.
Map<String, Object?> poisoned(int seq, int n) {
  final bad = threadCreated(seq, n);
  ((bad['payload']! as Map)['thread']! as Map)['id'] = 'not-a-uuid';
  return bad;
}

/// A snapshot whose first channel fails the mapping of the snapshot.
Map<String, Object?> brokenSnapshot() {
  final broken = snapshotJson(headSeq: 40);
  ((broken['channels']! as List).first as Map)['channel'] = {
    ...channelJson(1),
    'id': 'not-a-uuid',
  };
  return broken;
}

/// The upper end of every jitter, so a wait equals the bound of its attempt.
final class _Longest implements RandomSource {
  const _Longest();

  @override
  double nextDouble() => 1;
}

void main() {
  late World w;
  setUp(() => w = World(random: const _Longest()));
  tearDown(() => w.dispose());

  /// Lets the wait of the reconnect pass (the last one the connection asked for) and answers
  /// `hello` and `ready` of the next connection.
  Future<void> reconnect({
    int headSeq = 40,
    Map<String, Object?>? snapshot,
  }) async {
    final before = w.h.transport.channels.length;
    await w.h.clock.elapse(w.h.clock.waits.last);
    expect(w.h.transport.channels, hasLength(before + 1));
    w.ch.push('hello', helloData(nonce: 'nonce-$before'));
    await w.idle();
    final ready = readyData(headSeq: headSeq);
    ready['snapshot'] = snapshot ?? snapshotJson(headSeq: headSeq);
    w.ch.push('ready', ready);
    await w.idle();
  }

  test('a failed page closes the connection, and the next session repeats it '
      'from the stored cursor', () async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    expect(w.h.phase, ConnectionPhase.live);
    final first = w.ch;

    await w.dispatch([poisoned(31, 2)]);

    expect(await w.cursor(), 30);
    expect(first.clientCloseCode, 1000, reason: 'the listener closed it');
    expect(
      w.h.phase,
      ConnectionPhase.backoff,
      reason: 'the app sees a reconnect, not a live connection',
    );
    expect(w.log.any((l) => l.contains('page failed')), isTrue);

    await reconnect();
    expect(w.ch, isNot(same(first)));
    final start = w.ch.lastData('resume') ?? w.ch.lastData('identify');
    expect(
      start!['cursor'],
      30,
      reason: 'the next session starts at the stored cursor',
    );
    // The page arrives again; this time the server sends an event the applier takes.
    await w.dispatch([threadCreated(31, 1)]);
    await w.push('catchup.complete', {'head_seq': 31});
    expect(await w.cursor(), 31);
    expect(w.h.phase, ConnectionPhase.live);
    expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(1))), isNotNull);
  });

  test('a page that fails again at the same cursor is passed by a '
      'snapshot', () async {
    w.rest.routes['GET /sync/snapshot'] = (_) =>
        FakeReply(200, snapshotJson(headSeq: 45));
    await w.connect(cursor: 30);
    await w.dispatch([poisoned(31, 2)]);
    await w.push('cursor', {'seq': 33});
    expect(await w.cursor(), 30);
    expect(w.notices.whereType<Resynced>(), isEmpty);

    await reconnect(headSeq: 45);
    final second = w.ch;
    await w.dispatch([poisoned(31, 2)]);
    await w.push('cursor', {'seq': 33});

    expect(
      second.clientCloseCode,
      isNull,
      reason: 'the snapshot is loaded on the same connection',
    );
    expect(await w.cursor(), 45);
    expect(w.h.phase, ConnectionPhase.live);
    expect(w.notices.whereType<Resynced>().single.reason, Resynced.pageFailed);
    // Events after the snapshot apply again, the ones it holds are duplicates.
    await w.dispatch([threadCreated(40, 3), threadCreated(46, 1)]);
    expect(await w.cursor(), 46);
    expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(3))), isNull);
    expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(1))), isNotNull);
  });

  test('the snapshot after a failed end of the catch-up decides when the '
      'session is live', () async {
    await w.connect(cursor: 30);
    await w.dispatch([poisoned(31, 2)]);
    await w.push('catchup.complete', {'head_seq': 31});
    await reconnect(headSeq: 45);
    final snapshot = Completer<void>();
    w.rest.gates['GET /sync/snapshot'] = snapshot.future;
    w.rest.routes['GET /sync/snapshot'] = (_) =>
        FakeReply(200, snapshotJson(headSeq: 45));

    await w.dispatch([poisoned(31, 2)]);
    await w.push('catchup.complete', {'head_seq': 31});

    expect(
      w.h.phase,
      ConnectionPhase.resyncing,
      reason: 'not live before the mirror is replaced',
    );
    expect(w.liveCalls, 0);

    snapshot.complete();
    await w.idle();
    expect(w.h.phase, ConnectionPhase.live);
    expect(w.liveCalls, 1);
    expect(await w.cursor(), 45);
  });

  test('a failure at another cursor reconnects again, without a '
      'snapshot', () async {
    await w.connect(cursor: 30);
    await w.dispatch([poisoned(31, 2)]);
    await w.push('cursor', {'seq': 31});
    await reconnect();
    await w.dispatch([threadCreated(31, 1)]);
    await w.push('cursor', {'seq': 32});
    expect(await w.cursor(), 32);

    final second = w.ch;
    await w.dispatch([poisoned(33, 2)]);
    await w.push('cursor', {'seq': 33});

    expect(await w.cursor(), 32);
    expect(second.clientCloseCode, 1000);
    expect(w.notices.whereType<Resynced>(), isEmpty);
  });

  test('a snapshot of ready that cannot be applied reconnects', () async {
    await w.connect(snapshot: brokenSnapshot());

    expect(w.ch.clientCloseCode, 1000);
    expect(w.h.phase, ConnectionPhase.backoff);
    expect(await w.cursor(), isNull);
    expect(w.log.any((l) => l.contains('snapshot of ready failed')), isTrue);

    await reconnect();
    expect(await w.cursor(), 40);
    expect(w.h.phase, ConnectionPhase.live);
  });

  test('the waits after failed commits follow the reconnect backoff and start '
      'again after a commit', () async {
    final base = w.h.config.reconnectBackoffBase;
    await w.connect(snapshot: brokenSnapshot());
    expect(w.h.clock.waits.last, base);

    await reconnect(snapshot: brokenSnapshot());
    expect(w.h.clock.waits.last, base * 2, reason: 'no commit in between');

    await reconnect();
    expect(await w.cursor(), 40);
    await w.dispatch([poisoned(41, 2)]);
    expect(w.h.clock.waits.last, base, reason: 'the snapshot was committed');
  });

  test('a resync snapshot that cannot be stored reconnects', () async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    final first = w.ch;
    // The database fails from here on, as a full disk would.
    await w.db.close();
    await w.push('resync_required', {
      'reason': 'retention',
      'head_seq': 90,
      'sync_epoch': 'epoch-1',
    });

    expect(first.clientCloseCode, 1000);
    expect(w.log.any((l) => l.contains('resync snapshot not stored')), isTrue);
    expect(w.notices.whereType<Resynced>(), isEmpty);
  });
}
