// SPDX-License-Identifier: Apache-2.0

/// Appliers of `thread.*` (04 9 item 3): threads by version with their title in the search
/// index; `thread.moved` changes the channel of the thread, `thread.moved_out` deletes it when
/// it moved to a channel this device does not see (brief M4-14 Design note 2d).
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

Future<void> _thread(ApplyContext c) {
  final thread = threadFromJson(obj(c.payload, 'thread'));
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
/// cached file bytes and tells the app to close their views (04 8.3 item 3).
Future<void> removeTargetsInPage(
  ApplyContext c,
  List<TargetRef> targets,
) async {
  c.sync.removedTargets.addAll([for (final t in targets) t.id]);
  final paths = await c.db.removeTargets(targets);
  c.after.steps.add(() async {
    if (paths.isNotEmpty) await c.sync.eraser.erase(paths);
    c.sync.notify(TargetsRemoved(List.unmodifiable(targets)));
  });
}
