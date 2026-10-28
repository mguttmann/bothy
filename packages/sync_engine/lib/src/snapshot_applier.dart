// SPDX-License-Identifier: Apache-2.0

/// The snapshot of `ready` and `GET /api/v1/sync/snapshot` (04 6.3 item 4, 04 7.3; brief
/// M4-14 Design note 1): the whole snapshot on the first connection and after a resync, the
/// active threads of the sidebar included (R-SY-28, brief M4-40 Design note 5), only the parts
/// that are not events (counters, presence, feature flags, active run summaries) on a connection
/// that continues from a cursor.
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'appliers/principal_appliers.dart';
import 'appliers/structure_appliers.dart';
import 'appliers/thread_appliers.dart';
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
  /// cursor and epoch in the same transaction): spaces and channels with access, the active
  /// threads of the held channels, sidebar, the person, settings and active runs; after the
  /// commit the counters, presence and flags,
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
      for (final t in s.activeThreads) t.id.value,
    ]);
    for (final v in s.spaces) {
      await storeSpaceView(db, v);
    }
    for (final v in s.channels) {
      await storeChannelView(db, v);
    }
    // R-SY-28, D-SB-11: the sidebar lists the open threads of every channel without one list
    // request per channel. Only here, on the first connection and a resync: a connection that
    // continues from a cursor replays the `thread.*` events after it, which keep the threads
    // current, and the snapshot's state (as of `head_seq`) stored before those older events
    // could be undone by one of them, for example a replayed `thread.moved_out`.
    for (final thread in s.activeThreads) {
      if (await sync.holdsChannel(db, thread.channelId)) {
        await storeThread(db, thread);
      }
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
    // The snapshot names its agents only by ID, and a new device never received their
    // `agent.updated`: one read per agent this database does not name yet.
    final agents = {...s.agents.keys, for (final r in s.activeRuns) r.agentId};
    for (final agent in agents) {
      after.reads.add(() => _nameAgent(agent));
    }
  }

  /// Reads [agent] (`getAgent`) and stores its card and principal row, unless the database
  /// names it already.
  Future<void> _nameAgent(PrincipalId agent) async {
    if (await sync.db.principalsDao.byId(agent) != null) return;
    final card = await sync.fetcher.agent(agent);
    await sync.db.transaction(() => storeAgentCard(sync.db, card));
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
      // The counters of the snapshot hold every event up to its head (04 14).
      sync.counter = UnreadCounter(heldUpTo: s.headSeq);
      sync.countedThreads.clear();
      sync.counterChannels.clear();
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
        sync.counterChannels[u.threadId] = u.channelId;
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
