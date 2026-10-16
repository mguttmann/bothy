// SPDX-License-Identifier: Apache-2.0

/// The snapshot of `ready` and `GET /api/v1/sync/snapshot` (04 6.3 item 4, 04 7.3; brief
/// M4-14 Design note 1): the whole snapshot on the first connection and after a resync, only
/// the parts that are not events (counters, presence, feature flags, active run summaries) on a
/// connection that continues from a cursor.
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'appliers/structure_appliers.dart';
import 'dispatcher.dart';
import 'wire.dart';

/// Writes a [SnapshotData] into the local database and the in-memory stores.
///
/// Example:
/// ```dart
/// await db.syncStateDao.commitPage((txn) async {
///   await db.syncStateDao.clearMirror(txn);
///   await snapshots.applyFull(txn, snapshot, after);
/// }, cursor: snapshot.headSeq, syncEpoch: snapshot.syncEpoch);
/// ```
final class SnapshotApplier {
  /// Creates the applier over [sync].
  SnapshotApplier(this.sync);

  /// The shared context.
  final SyncContext sync;

  /// Applies the whole snapshot inside [txn] (the caller empties the mirror first and stores
  /// cursor and epoch in the same transaction): spaces and channels with access, sidebar,
  /// the person, settings and active runs; after the commit the counters, presence and flags,
  /// and in the background the drafts newer than the local ones, the read states and the
  /// instance information, which the snapshot names only by version or not at all.
  Future<void> applyFull(
    LocalTxn txn,
    SnapshotData s,
    AfterCommit after,
  ) async {
    final db = txn.db;
    // What the snapshot lists is visible again.
    sync.removedTargets.removeAll([
      for (final v in s.spaces) v.space.id.value,
      for (final v in s.channels) v.channel.id.value,
    ]);
    for (final v in s.spaces) {
      await storeSpaceView(db, v);
    }
    for (final v in s.channels) {
      await storeChannelView(db, v);
    }
    for (final state in s.sidebar) {
      await db.structureDao.upsertUserSpaceStateIfNewer(state);
    }
    await db.principalsDao.put(s.principal);
    for (final setting in s.settings) {
      await db.personalDao.upsertSettingIfNewer(setting);
    }
    await applyNonEvent(txn, s, after);
    final drafts = await _newerDrafts(db, s.drafts);
    if (drafts) after.reads.add(_loadDrafts);
    after.reads
      ..add(() async {
        // `listMyReadStates` reads one channel; the channels with unread threads are the ones
        // whose read position the interface shows (D-M4-14-2).
        final channels = {for (final u in s.unreadThreads) u.channelId};
        for (final channel in channels) {
          final states = await sync.fetcher.myReadStates(channel);
          await sync.db.transaction(() async {
            for (final r in states) {
              if (await sync.holdsChannel(sync.db, r.channelId)) {
                await sync.db.personalDao.applyReadState(r);
              }
            }
          });
        }
      })
      ..add(() async {
        final info = await sync.fetcher.instance();
        await sync.db.instanceDao.put(info.copyWith(features: s.features));
      });
  }

  /// Applies the parts of the snapshot that are not events (Design note 1): the run summaries
  /// by version inside [txn]; after the commit the counters, the agent presence and the
  /// feature flags.
  Future<void> applyNonEvent(
    LocalTxn txn,
    SnapshotData s,
    AfterCommit after,
  ) async {
    final db = txn.db;
    for (final run in s.activeRuns) {
      await db.runsDao.upsertIfNewer(run);
    }
    final instance = await db.instanceDao.get();
    if (instance != null) {
      await db.instanceDao.put(instance.copyWith(features: s.features));
    }
    final floors = <ThreadId, ThreadSeq>{};
    for (final u in s.unreadThreads) {
      final thread = await db.threadsDao.byId(u.threadId);
      if (thread != null) floors[u.threadId] = thread.unreadFloorSeq;
    }
    after.steps.add(() {
      sync.me = s.principal.id;
      sync.ephemeral.seedPresence(s.agents);
      final previous = sync.countedThreads.toSet();
      sync.counter = UnreadCounter();
      sync.countedThreads.clear();
      for (final u in s.unreadThreads) {
        sync.counter.resetFrom(
          ReadState(
            threadId: u.threadId,
            channelId: u.channelId,
            lastReadSeq: u.lastReadSeq,
            mentionCount: u.mentionCount,
            unreadCount: u.unreadCount,
            // The counters of the snapshot carry no version; this state only seeds the
            // counter and is never stored.
            version: const ResourceVersion(0),
          ),
          lastMessageSeq: u.lastMessageSeq,
          unreadFloorSeq: floors[u.threadId] ?? const ThreadSeq(0),
        );
        sync.countedThreads.add(u.threadId);
      }
      for (final t in {...previous, ...sync.countedThreads}) {
        sync.counterChanges.add(t);
      }
    });
  }

  Future<bool> _newerDrafts(LocalDatabase db, List<DraftRef> refs) async {
    if (refs.isEmpty) return false;
    final local = {
      for (final d in await db.personalDao.allDrafts()) d.id: d.version,
    };
    return refs.any((r) {
      final v = local[r.id];
      return v == null || r.version > v;
    });
  }

  /// `listMyDrafts`: drafts by version; a draft being typed here keeps its text (04 14).
  Future<void> _loadDrafts() async {
    final drafts = await sync.fetcher.myDrafts();
    await sync.db.transaction(() async {
      for (final d in drafts) {
        if (sync.draftEditing.isEditing(
          channelId: d.channelId,
          threadId: d.threadId,
        )) {
          continue;
        }
        if (!await sync.holdsChannel(sync.db, d.channelId)) continue;
        await sync.db.personalDao.upsertDraftIfNewer(d);
      }
    });
  }
}
