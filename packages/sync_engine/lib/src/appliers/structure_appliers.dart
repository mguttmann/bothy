// SPDX-License-Identifier: Apache-2.0

/// Appliers of `space.*` and `channel.*` (04 9 item 3, 04 8.1): spaces and channels by version;
/// archiving and a new `read_only` or operator come with the object (brief M4-14 Design note
/// 2d). The caller's access comes from `SpaceView` and `ChannelView` of REST and the snapshot.
///
/// A space or channel whose object carries `deleted_at` goes as a removed target with its
/// content, and a later version without it brings it back with its structure loaded through
/// REST (01 6.18 item 8, D-DEL-5).
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import '../dispatcher.dart';
import '../wire.dart';
import 'thread_appliers.dart';

/// The appliers of this group by event type.
final Map<String, Applier> structureAppliers = {
  'space.created': _space,
  'space.updated': _space,
  'space.archived': _space,
  'channel.created': _channel,
  'channel.updated': _channel,
  'channel.archived': _channel,
};

Future<void> _space(ApplyContext c) async {
  final json = obj(c.payload, 'space');
  final space = spaceFromJson(json);
  final target = TargetRef.space(space.id);
  final stored = await c.db.structureDao.spaceById(space.id);
  if (isDeleted(json)) {
    return removeDeletedInPage(
      c,
      target,
      space.version,
      stored: stored?.version,
    );
  }
  final undo = _undoOf(c, target, space.version, stored: stored != null);
  if (undo == _Undo.stale) return;
  await c.db.structureDao.upsertSpaceIfNewer(space);
  if (undo == _Undo.restore) _reload(c, target);
}

Future<void> _channel(ApplyContext c) async {
  final json = obj(c.payload, 'channel');
  final channel = channelFromJson(json);
  final target = TargetRef.channel(channel.id);
  final stored = await c.db.structureDao.channelById(channel.id);
  if (isDeleted(json)) {
    return removeDeletedInPage(
      c,
      target,
      channel.version,
      stored: stored?.version,
    );
  }
  final undo = _undoOf(c, target, channel.version, stored: stored != null);
  if (undo == _Undo.stale) return;
  await c.db.structureDao.upsertChannelIfNewer(channel);
  if (undo == _Undo.restore) _reload(c, target);
}

/// What an object of [target] without `deleted_at` means after a deletion (D-DEL-5).
enum _Undo {
  /// No deletion of the target is known: the version rule of the DAO applies.
  none,

  /// Not newer than the deletion this process saw: changes nothing.
  stale,

  /// The undo: the item comes back and its structure loads through REST.
  restore,
}

/// Decides [_Undo] for [target] at [version]; [stored] tells whether its row is held here. A
/// version above the deletion seen is the undo, and the target leaves
/// [SyncContext.deletedVersions] and [SyncContext.removedTargets]. A process that started after
/// the deletion knows no version: an `*.updated` of a space or channel this device does not
/// hold and did not remove for access loss is taken as the undo as well, so the sidebar gets
/// the item with its channels and access instead of a bare row.
_Undo _undoOf(
  ApplyContext c,
  TargetRef target,
  ResourceVersion version, {
  required bool stored,
}) {
  final sync = c.sync;
  final deleted = sync.deletedVersions[target.id];
  if (deleted != null) {
    if (version <= deleted) return _Undo.stale;
    sync.deletedVersions.remove(target.id);
    sync.removedTargets.remove(target.id);
    return _Undo.restore;
  }
  if (!stored &&
      c.event.type.endsWith('.updated') &&
      !sync.removedTargets.contains(target.id)) {
    return _Undo.restore;
  }
  return _Undo.none;
}

/// Loads the structure of the restored [target] after the commit, as `access.changed` with
/// `added` does (04 8.4).
void _reload(ApplyContext c, TargetRef target) {
  final sync = c.sync;
  c.after.reads.add(() => loadStructureOf(sync, [target]));
}

/// Loads the structure of [targets] through REST (`Fetcher.loadStructure`) and stores the
/// spaces and channels with the caller's access and the first page of threads of each channel
/// (04 8.4: no catch-up of older events). A removal while the reads run wins (04 8.3), and a
/// deleted item is not stored (D-DEL-5).
Future<void> loadStructureOf(SyncContext sync, List<TargetRef> targets) async {
  final s = await sync.fetcher.loadStructure(targets);
  await sync.db.transaction(() async {
    for (final v in s.spaces) {
      if (!sync.removedTargets.contains(v.space.id.value)) {
        await storeSpaceView(sync.db, v);
      }
    }
    for (final v in s.channels) {
      if (sync.mayStoreChannel(v.channel.id, v.channel.spaceId)) {
        await storeChannelView(sync.db, v);
      }
    }
    for (final t in s.threads) {
      // The server lists the thread in a channel the person sees now: a moved-out thread that
      // moved into it is visible again.
      if (sync.mayStoreChannel(t.channelId, null)) {
        sync.removedTargets.remove(t.id.value);
      }
      if (await sync.holdsContent(sync.db, t.channelId, t.id)) {
        await storeThread(sync.db, t);
      }
    }
  });
}

/// Stores a space with the caller's access as the server reported it (snapshot, `getSpace`);
/// a deleted space is never stored back (D-DEL-5).
Future<void> storeSpaceView(LocalDatabase db, SpaceWithAccess view) async {
  if (view.deletedAt != null) return;
  await db.structureDao.upsertSpaceIfNewer(view.space);
  await db.structureDao.putSpaceAccess(view.space.id, view.access);
}

/// Stores a channel with the caller's access as the server reported it (snapshot,
/// `getChannel`); a deleted channel is never stored back (D-DEL-5).
Future<void> storeChannelView(LocalDatabase db, ChannelWithAccess view) async {
  if (view.deletedAt != null) return;
  await db.structureDao.upsertChannelIfNewer(view.channel);
  await db.structureDao.putChannelAccess(view.channel.id, view.access);
}
