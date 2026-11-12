// SPDX-License-Identifier: Apache-2.0

// A thread on screen is the access the cache cleanup counts (04 13.1, 15 6.3;
// docs/decisions.d/AUD-pkg-local-store-01.md): the history column records it in the local
// database when it opens and again each time the window gets the focus back, so the cleanup
// that runs at every start of the app keeps what the person uses and removes only what nobody
// opened for `client.cache_max_age`. The real history column, the real database and the real
// cleanup run here, with the clock in the hand of the test.

import 'dart:io';

import 'package:client/files/file_cache.dart';
import 'package:client/thread/history_column.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity;

import '../support/column_host.dart';
import '../support/fakes.dart' show SwitchLifecycle;
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

void main() {
  final maxAge = AppConfig.defaults().cacheMaxAge;

  /// The cleanup of a start of the app (`InstanceServices` in bootstrap.dart): the file cache
  /// runs the cleaner with the limits of the configuration.
  Future<void> startCleanup(
    WidgetTester tester,
    ColumnHost host,
    DateTime Function() now,
  ) => host.run(
    tester,
    () => FileCache(
      db: host.db,
      root: Directory('${host.root.path}/files'),
    ).clean(CacheCleaner.fromConfig(host.db, AppConfig.defaults(), now: now)),
  );

  /// Stores thread [n] with two loaded messages next to the seeded thread 1.
  Future<void> addThread(
    LocalDatabase db,
    int n, {
    ThreadState state = ThreadState.open,
  }) async {
    await db.threadsDao.upsertIfNewer(thread(n, state: state, lastSeq: 2));
    for (final seq in [1, 2]) {
      await db.messagesDao.upsertIfNewer(msg(seq, threadN: n));
    }
    await db.messagesDao.addLoadedRange(
      threadId(n),
      const ThreadSeq(1),
      const ThreadSeq(2),
    );
  }

  Future<List<int>> storedSeqs(WidgetTester tester, ColumnHost host, int n) =>
      host.run(tester, () async {
        final window = await host.db.messagesDao.contiguous(
          threadId(n),
          limit: 50,
        );
        return [for (final m in window.messages) m.threadSeq.value];
      });

  Future<DateTime> accessedAt(WidgetTester tester, ColumnHost host, int n) =>
      host.run(tester, () async {
        final row = await host.db
            .customSelect(
              'SELECT last_accessed_at AS t FROM threads '
              "WHERE id = '${threadId(n).value}'",
            )
            .getSingle();
        return DateTime.fromMillisecondsSinceEpoch(
          row.read<int>('t'),
          isUtc: true,
        );
      });

  testWidgets('an old thread the person opened yesterday keeps its messages '
      'over the start cleanup, threads nobody opened lose theirs', (
    tester,
  ) async {
    var now = t0;
    final db = openInMemoryForTests(clock: () => now);
    final host = await ColumnHost.open(tester, database: db);
    // Three chats reach the device on the same day: thread 1, an open thread 2 and a done
    // thread 3, each with its messages.
    await host.run(tester, () async {
      await seedThread(host.db, messages: [msg(1), msg(2, minutes: 5)]);
      await addThread(host.db, 2);
      await addThread(host.db, 3, state: ThreadState.closed);
    });

    // Long after `client.cache_max_age` the person opens thread 1 ...
    now = t0.add(maxAge + const Duration(days: 9));
    await pumpThread(tester, host, FakeThreadReads(host.db));
    expect(
      find.byKey(ValueKey('message.${messageId(2).value}')),
      findsOneWidget,
    );
    await host.settle(tester);
    await tester.pumpWidget(const SizedBox());

    // ... and the next day the app starts again and cleans its cache.
    now = now.add(const Duration(days: 1));
    await startCleanup(tester, host, () => now);

    expect(await storedSeqs(tester, host, 1), [
      1,
      2,
    ], reason: 'opened yesterday: the thread and its messages stay');
    expect(
      await storedSeqs(tester, host, 2),
      isEmpty,
      reason:
          'the cleanup ran: nobody opened thread 2 for client.cache_max_age',
    );
    final threads = await host.run(tester, () async {
      return [
        for (final n in [1, 2, 3]) await host.db.threadsDao.byId(threadId(n)),
      ];
    });
    expect(threads[0], isNotNull);
    expect(threads[1], isNotNull, reason: 'the sidebar lists an open thread');
    expect(threads[2], isNull, reason: 'a done thread nobody opened goes');
    await host.dispose(tester);
    await db.close();
  });

  testWidgets('a thread that stays on screen is recorded again when the window '
      'gets the focus back', (tester) async {
    var now = t0;
    final db = openInMemoryForTests(clock: () => now);
    final host = await ColumnHost.open(tester, database: db);
    await host.run(
      tester,
      () => seedThread(host.db, messages: [msg(1), msg(2, minutes: 5)]),
    );
    final lifecycle = SwitchLifecycle();
    now = t0.add(const Duration(days: 1));
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: threadOverrides(FakeThreadReads(host.db)),
      lifecycle: lifecycle,
    );
    await host.settle(tester);
    expect(await accessedAt(tester, host, 1), now);

    // The app stays open with this thread for longer than `client.cache_max_age`; the person
    // comes back to its window now and then.
    lifecycle.set(AppActivity.background);
    now = t0.add(maxAge + const Duration(days: 3));
    await host.settle(tester);
    expect(
      await accessedAt(tester, host, 1),
      t0.add(const Duration(days: 1)),
      reason: 'a window in the background is no access',
    );
    lifecycle.set(AppActivity.foreground);
    await host.settle(tester);
    expect(await accessedAt(tester, host, 1), now);

    // The next start, a day later, keeps the thread the person had on screen.
    await tester.pumpWidget(const SizedBox());
    now = now.add(const Duration(days: 1));
    await startCleanup(tester, host, () => now);
    expect(await storedSeqs(tester, host, 1), [1, 2]);
    await host.dispose(tester);
    await db.close();
  });
}
