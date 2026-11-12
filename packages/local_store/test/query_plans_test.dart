// SPDX-License-Identifier: Apache-2.0

// The query plans of the lookups that used to read a whole table
// (docs/decisions.d/AUD-pkg-local-store-01.md): every write of the search index finds its rows
// through `search_rows` and reaches `search_index` by `rowid`, and the lookups by
// `client_msg_id`, thread, channel and file scope use their index. The statements are the ones
// the package sends (recorded while the real call runs), read with `EXPLAIN QUERY PLAN`.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';
import 'support/statement_recorder.dart';

/// FTS5 reports the lookup of a row by its `rowid` as index 0 with the constraint `=`; index 0
/// without a constraint reads every row of the index table.
const String _byRowid = 'SCAN search_index VIRTUAL TABLE INDEX 0:=';
const String _wholeIndex = 'SCAN search_index VIRTUAL TABLE INDEX 0:';

void main() {
  late StatementRecorder recorder;
  late FakeClock clock;
  late LocalDatabase db;
  setUp(() async {
    recorder = StatementRecorder();
    clock = FakeClock();
    db = openRecorded(recorder, clock: clock.call);
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await db.structureDao.upsertChannelIfNewer(channel(1));
  });
  tearDown(() => db.close());

  /// Thread [n] with one message, its attachment and file, a run and their search rows.
  Future<void> addThread(int n, {bool open = true}) async {
    final t = thread(
      n,
      lastSeq: 1,
      title: 'Plan $n',
    ).copyWith(state: open ? ThreadState.open : ThreadState.closed);
    await db.threadsDao.upsertIfNewer(t);
    await db.searchDao.indexThreadTitle(t, spaceId: space(1).id);
    final m = message(n, 1, text: 'Note $n from Sam');
    await db.messagesDao.upsertIfNewer(m);
    await db.searchDao.indexMessage(m, spaceId: space(1).id);
    await db.messagesDao.putFile(file(n));
    await db.messagesDao.putAttachments(m.id, [
      MessageAttachment(
        messageId: m.id,
        fileId: file(n).id,
        position: 1,
        threadId: t.id,
        channelId: t.channelId,
      ),
    ]);
    await db.searchDao.indexTranscript(
      fileId: file(n).id,
      threadId: t.id,
      channelId: t.channelId,
      spaceId: space(1).id,
      text: 'Sam on the phone about plan $n',
    );
  }

  /// The plans of the statements of [action] that name [table], by statement text.
  Future<Map<String, List<String>>> plansOn(
    String table,
    Future<void> Function() action,
  ) async {
    final plans = <String, List<String>>{};
    for (final s in await statementsOf(recorder, action)) {
      if (!s.sql.contains(table)) continue;
      plans[s.sql] = await planOf(db, s);
    }
    return plans;
  }

  group(
    'the search index is never read as a whole to replace or delete rows',
    () {
      /// Every statement of [action] on `search_index` except its inserts: none reads the whole
      /// index table, and at least one reaches it by rowid.
      Future<void> expectByRowid(Future<void> Function() action) async {
        final plans = await plansOn('search_index', action);
        plans.removeWhere((sql, _) => sql.startsWith('INSERT'));
        expect(plans, isNotEmpty, reason: 'the call writes the index');
        for (final MapEntry(key: sql, value: plan) in plans.entries) {
          expect(plan, isNot(contains(_wholeIndex)), reason: sql);
          expect(plan, contains(_byRowid), reason: sql);
        }
      }

      test('storing a message, a title and a transcript again', () async {
        await addThread(1);
        await expectByRowid(
          () => db.searchDao.indexMessage(
            message(1, 1, text: 'Edited by Sam'),
            spaceId: space(1).id,
          ),
        );
        await expectByRowid(
          () => db.searchDao.indexThreadTitle(thread(1, title: 'Renamed')),
        );
        await expectByRowid(
          () => db.searchDao.indexTranscript(
            fileId: file(1).id,
            threadId: threadId(1),
            channelId: channel(1).id,
            text: 'A new transcript',
          ),
        );
        expect(
          (await db.searchDao.search('edited renamed transcript', limit: 10)),
          isEmpty,
        );
        expect(await db.searchDao.search('edited', limit: 10), hasLength(1));
        expect(await db.searchDao.search('note', limit: 10), isEmpty);
      });

      test('a tombstone', () async {
        await addThread(1);
        await expectByRowid(
          () => db.messagesDao.tombstoneWithFiles(
            messageId(1, 1),
            deletedAt: at(9),
            version: const ResourceVersion(2),
          ),
        );
      });

      test('the cleanup of a done and of an open thread', () async {
        await addThread(1, open: false);
        await addThread(2);
        clock.advance(const Duration(days: 40));
        await expectByRowid(
          () => CacheCleaner(
            db,
            maxAge: const Duration(days: 30),
            maxBytes: 1 << 40,
            now: clock.call,
          ).run(),
        );
      });

      test('the removal of a thread, a channel and a space', () async {
        await addThread(1);
        await addThread(2);
        await expectByRowid(
          () => db.removeTargets([TargetRef.thread(threadId(1))]),
        );
        await expectByRowid(
          () => db.removeTargets([TargetRef.channel(channel(1).id)]),
        );
        await expectByRowid(
          () => db.removeTargets([TargetRef.space(space(1).id)]),
        );
        expect(await db.searchDao.search('plan', limit: 10), isEmpty);
      });

      test('pruning transcripts', () async {
        await addThread(1);
        await db.messagesDao.putAttachments(messageId(1, 1), const []);
        await expectByRowid(() => db.searchDao.pruneTranscripts());
      });

      test(
        'the transcripts to index are found without the index table',
        () async {
          await addThread(1);
          final plans = await plansOn(
            'search_',
            () => db.searchDao.watchTranscriptsToIndex().first,
          );
          expect(plans, hasLength(1));
          expect(plans.keys.single, isNot(contains('search_index')));
          expect(
            plans.values.single,
            contains(
              matches(
                r'^SEARCH s USING (COVERING )?INDEX search_rows_target '
                r'\(kind=\? AND target_id=\?\)$',
              ),
            ),
          );
        },
      );
    },
  );

  group('lookups use their index', () {
    /// No step of [plan] reads one of [tables] as a whole.
    void expectNoScanOf(List<String> plan, List<String> tables, String sql) {
      for (final table in tables) {
        expect(
          plan,
          isNot(contains(matches('^SCAN $table\\b'))),
          reason: '$table in: $sql',
        );
      }
    }

    test('the message an outbox entry sent, by client_msg_id', () async {
      await addThread(1);
      final plans = await plansOn(
        'messages',
        () => db.messagesDao.threadOfClientMsg(messageId(1, 1)),
      );
      expect(plans, hasLength(1));
      expect(
        plans.values.single,
        contains(
          'SEARCH messages USING INDEX messages_client_msg (client_msg_id=?)',
        ),
      );
    });

    test('the attachments of a thread when it is deleted', () async {
      await addThread(1, open: false);
      clock.advance(const Duration(days: 40));
      final plans = await plansOn(
        'message_attachments WHERE thread_id',
        () => CacheCleaner(
          db,
          maxAge: const Duration(days: 30),
          maxBytes: 1 << 40,
          now: clock.call,
        ).run(),
      );
      // The files of the thread are read, then its attachments deleted.
      expect(plans, hasLength(2));
      for (final MapEntry(key: sql, value: plan) in plans.entries) {
        expect(
          plan,
          contains(
            matches(
              r'^SEARCH message_attachments USING (COVERING )?INDEX '
              r'message_attachments_thread \(thread_id=\?\)$',
            ),
          ),
          reason: sql,
        );
      }
    });

    test('the files of a thread in the session column', () async {
      await addThread(1);
      final plans = await plansOn(
        'message_attachments',
        () => db.messagesDao.watchThreadFiles(threadId(1)).first,
      );
      expect(plans, hasLength(1));
      expectNoScanOf(plans.values.single, [
        'message_attachments',
        'messages',
        'files',
      ], plans.keys.single);
    });

    test('the removal of a channel: threads, messages, attachments, runs and '
        'read states by channel, files by scope', () async {
      await addThread(1);
      final statements = await statementsOf(
        recorder,
        () => db.removeTargets([TargetRef.channel(channel(1).id)]),
      );
      final threads = statements.singleWhere(
        (s) => s.sql.contains('UNION SELECT thread_id FROM messages'),
      );
      final plan = await planOf(db, threads);
      expectNoScanOf(plan, [
        'threads',
        'messages',
        'message_attachments',
        'runs',
        'read_states',
      ], threads.sql);
      for (final index in [
        'threads_channel',
        'messages_channel',
        'message_attachments_channel',
        'runs_channel',
        'read_states_channel',
      ]) {
        expect(plan, contains(contains(index)), reason: index);
      }
      final scoped = statements.singleWhere(
        (s) => s.sql.contains('FROM files WHERE scope_channel_id'),
      );
      expect(
        await planOf(db, scoped),
        contains(
          matches(
            r'^SEARCH files USING (COVERING )?INDEX files_scope_channel '
            r'\(scope_channel_id=\?\)$',
          ),
        ),
      );
    });

    test('the runs of a channel for its post list', () async {
      final plans = await plansOn(
        'runs',
        () => db.runsDao.watchForChannel(channel(1).id).first,
      );
      expect(plans, hasLength(1));
      expect(
        plans.values.single,
        contains('SEARCH runs USING INDEX runs_channel (channel_id=?)'),
      );
    });
  });
}
