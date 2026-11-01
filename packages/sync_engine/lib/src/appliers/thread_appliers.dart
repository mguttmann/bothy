// SPDX-License-Identifier: Apache-2.0

/// Appliers of `thread.*` (04 9 item 3): threads by version with their title in the search
/// index; `thread.moved` changes the channel of the thread, `thread.moved_out` deletes it when
/// it moved to a channel this device does not see (brief M4-14 Design note 2d). A thread whose
/// object carries `deleted_at` goes as a removed target, and a later version without it brings
/// it back (01 6.18 item 8, D-DEL-5).
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import '../dispatcher.dart';
import '../sync_events.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> threadAppliers = {
  'thread.created': _thread,
  'thread.updated': _thread,
  'thread.closed': _thread,
  'thread.reopened': _thread,
  'thread.moved': _thread,
  'thread.moved_out': _movedOut,
};

/// Stores [thread] by the version rule and indexes its title (14 6); runs inside the caller's
/// transaction.
Future<void> storeThread(LocalDatabase db, Thread thread) async {
  if (!await db.threadsDao.upsertIfNewer(thread)) return;
  final channel = await db.structureDao.channelById(thread.channelId);
  await db.searchDao.indexThreadTitle(thread, spaceId: channel?.spaceId);
}

Future<void> _thread(ApplyContext c) async {
  final json = obj(c.payload, 'thread');
  final thread = threadFromJson(json);
  if (isDeleted(json)) {
    final stored = await c.db.threadsDao.byId(thread.id);
    return removeDeletedInPage(
      c,
      TargetRef.thread(thread.id),
      thread.version,
      stored: stored?.version,
    );
  }
  // The undo of a deletion: only a version above the deletion's (D-DEL-5); the history loads
  // again from the server when the thread opens.
  final deleted = c.sync.deletedVersions[thread.id.value];
  if (deleted != null) {
    if (thread.version <= deleted) return;
    c.sync.deletedVersions.remove(thread.id.value);
  }
  // A thread that moves into a visible channel is visible again after a `thread.moved_out`.
  c.sync.removedTargets.remove(thread.id.value);
  return storeThread(c.db, thread);
}

/// `thread.moved_out` names only the source channel (API `ThreadMovedOutPayload`). When the
/// thread is still stored in that channel, no `thread.moved` into a visible channel came before
/// it, so the target is not visible here and the thread with its content goes (04 8.3 item 3).
Future<void> _movedOut(ApplyContext c) async {
  final p = c.payload;
  final threadId = ThreadId.parse(str(p, 'thread_id'));
  final source = ChannelId.parse(str(p, 'channel_id'));
  final stored = await c.db.threadsDao.byId(threadId);
  if (stored != null && stored.channelId != source) return;
  await removeTargetsInPage(c, [TargetRef.thread(threadId)]);
}

/// Deletes [targets] inside the page transaction of [c] and, after the commit, deletes the
/// cached file bytes, drops the unread counters of their threads and tells the app to close
/// their views (04 8.3 item 3). [deleted] marks a deletion (D-DEL-5): the person's order rows
/// stay, so an undo restores the place (D-DEL-8), and the notice says so.
Future<void> removeTargetsInPage(
  ApplyContext c,
  List<TargetRef> targets, {
  bool deleted = false,
}) async {
  final sync = c.sync;
  sync.removedTargets.addAll([for (final t in targets) t.id]);
  final paths = await c.db.removeTargets(targets, keepOrder: deleted);
  // The status dots and unread marks go with the content (D-DEL-8): the counters of removed
  // threads and of the threads of removed channels, whose rows are gone now.
  final gone = {
    for (final t in targets)
      if (t.kind == RemovedTargetKind.thread) ThreadId.parse(t.id),
  };
  final channels = {...sync.counterChannels.values};
  for (final channel in channels) {
    if (await c.db.structureDao.channelById(channel) != null) continue;
    gone.addAll([
      for (final MapEntry(:key, :value) in sync.counterChannels.entries)
        if (value == channel) key,
    ]);
  }
  c.after.steps.add(() async {
    if (paths.isNotEmpty) await sync.eraser.erase(paths);
    for (final t in gone) {
      sync.counterChannels.remove(t);
      if (sync.countedThreads.remove(t)) sync.counterChanges.add(t);
    }
    sync.notify(TargetsRemoved(List.unmodifiable(targets), deleted: deleted));
  });
}

/// The object of [target] in the event of [c] carries `deleted_at` (D-DEL-5): the item and its
/// content go as a removed target (04 8.3 item 3) with the person's order rows kept (D-DEL-8).
/// By the version rule a deletion not newer than the [stored] row, or older than the last
/// deletion of [target] this process saw, is stale; the same version again (a page that rolled
/// back and comes again) removes again.
Future<void> removeDeletedInPage(
  ApplyContext c,
  TargetRef target,
  ResourceVersion version, {
  ResourceVersion? stored,
}) async {
  final seen = c.sync.deletedVersions[target.id];
  if (stored != null && version <= stored) return;
  if (seen != null && version < seen) return;
  c.sync.deletedVersions[target.id] = version;
  await removeTargetsInPage(c, [target], deleted: true);
}
