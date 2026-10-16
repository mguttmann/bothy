// SPDX-License-Identifier: Apache-2.0

// resync_required (04 7.3, 15 7.3 item 2, brief M4-14 Tests): for every reason the mirror is
// emptied, outbox, drafts and share inbox stay, the snapshot is loaded, cursor and epoch are
// its values, events up to the new head_seq are discarded and the connection stays (no new
// identify).

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  for (final reason in [
    'retention',
    'too_far_behind',
    'epoch_changed',
    'cursor_ahead',
  ]) {
    test(
      'resync_required with $reason replaces the mirror on the same connection',
      () async {
        final w = World();
        addTearDown(w.dispose);
        await w.connect(cursor: 30);
        await w.dispatch([threadCreated(31, 1), messageCreated(32, 1)]);
        await w.push('cursor', {'seq': 32});
        final t1 = ThreadId.parse(threadId(1));
        await w.db.outboxDao.enqueue(
          id: id(0x801),
          kind: 'sendMessage',
          threadId: t1,
          payload: '{"content_md":"queued"}',
        );
        await w.db.personalDao.upsertDraftIfNewer(
          Draft(
            id: DraftId.parse(draftId(1)),
            threadId: t1,
            channelId: ChannelId.parse(channelId(1)),
            contentMd: 'half a sentence',
            fileIds: const [],
            version: const ResourceVersion(2),
          ),
        );
        await w.db.shareInboxDao.add(
          ShareInboxEntry(
            id: id(0x802),
            payload: '{"text":"shared"}',
            filePaths: const [],
            createdAt: DateTime.utc(2026, 10, 15, 10),
          ),
        );
        w.rest.routes['GET /sync/snapshot'] = (_) => FakeReply(
          200,
          snapshotJson(
            headSeq: 90,
            epoch: 'epoch-2',
            channels: [channelView(2)],
          ),
        );
        final opsBefore = w.ch.sentOps.where((o) => o == 'identify').length;
        final channelsBefore = w.h.transport.channels.length;

        await w.push('resync_required', {
          'reason': reason,
          'head_seq': 90,
          'sync_epoch': 'epoch-2',
        });

        expect(w.h.phase, ConnectionPhase.live);
        expect(w.notices.whereType<Resynced>().single.reason, reason);
        expect(w.rest.count('/sync/snapshot'), 1);
        final s = await w.db.syncStateDao.read();
        expect(s.cursor, const GlobalSeq(90));
        expect(s.syncEpoch, 'epoch-2');
        expect(
          await w.db.threadsDao.byId(t1),
          isNull,
          reason: 'mirror emptied',
        );
        expect(await w.storedSeqs(1), isEmpty);
        expect(
          await w.db.structureDao.channelById(ChannelId.parse(channelId(1))),
          isNull,
        );
        expect(
          await w.db.structureDao.channelById(ChannelId.parse(channelId(2))),
          isNotNull,
        );
        expect(await w.db.outboxDao.byId(id(0x801)), isNotNull);
        expect(await w.db.personalDao.allDrafts(), hasLength(1));
        expect(await w.db.shareInboxDao.all(), hasLength(1));

        // Events up to the new head_seq are discarded, later ones apply.
        await w.dispatch([threadCreated(90, 3, channelN: 2)]);
        expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(3))), isNull);
        await w.dispatch([threadCreated(91, 4, channelN: 2)]);
        expect(
          await w.db.threadsDao.byId(ThreadId.parse(threadId(4))),
          isNotNull,
        );
        expect(await w.cursor(), 91);

        expect(
          w.h.transport.channels.length,
          channelsBefore,
          reason: 'no reconnect',
        );
        expect(
          w.ch.sentOps.where((o) => o == 'identify').length,
          opsBefore,
          reason: 'no new identify',
        );
      },
    );
  }

  test(
    'frames during the resync wait and apply after it with the duplicate rule',
    () async {
      final w = World();
      addTearDown(w.dispose);
      await w.connect(cursor: 30);
      w.rest.routes['GET /sync/snapshot'] = (_) =>
          FakeReply(200, snapshotJson(headSeq: 60, channels: [channelView(1)]));
      w.ch
        ..push('resync_required', {
          'reason': 'too_far_behind',
          'head_seq': 60,
          'sync_epoch': 'epoch-1',
        })
        ..push('dispatch', threadCreated(59, 5))
        ..push('dispatch', threadCreated(61, 6));
      await w.idle();
      expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(5))), isNull);
      expect(
        await w.db.threadsDao.byId(ThreadId.parse(threadId(6))),
        isNotNull,
      );
    },
  );

  test(
    'a failed snapshot is retried after the backoff on the same connection',
    () async {
      final w = World();
      addTearDown(w.dispose);
      await w.connect(cursor: 30);
      await w.dispatch([threadCreated(31, 1)]);
      await w.push('cursor', {'seq': 31});
      var fail = true;
      w.rest.routes['GET /sync/snapshot'] = (_) => fail
          ? FakeReply.problem(503, 'unavailable')
          : FakeReply(200, snapshotJson(headSeq: 90, epoch: 'epoch-2'));
      await w.push('resync_required', {
        'reason': 'epoch_changed',
        'head_seq': 90,
        'sync_epoch': 'epoch-2',
      });
      expect(w.h.phase, ConnectionPhase.resyncing);
      expect(await w.cursor(), 31, reason: 'kept until a snapshot arrives');
      // Live frames wait in memory meanwhile.
      await w.dispatch([threadCreated(90, 3), threadCreated(91, 4)]);
      expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(4))), isNull);
      fail = false;
      await w.h.clock.elapse(const Duration(seconds: 2));
      await w.idle();
      expect(w.h.phase, ConnectionPhase.live);
      expect(await w.cursor(), 91);
      expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(1))), isNull);
      expect(await w.db.threadsDao.byId(ThreadId.parse(threadId(3))), isNull);
      expect(
        await w.db.threadsDao.byId(ThreadId.parse(threadId(4))),
        isNotNull,
      );
      expect(w.notices.whereType<Resynced>().single.reason, 'epoch_changed');
      expect(w.h.transport.channels, hasLength(1), reason: 'no reconnect');
    },
  );
}
