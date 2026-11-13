// SPDX-License-Identifier: Apache-2.0

/// Appliers of the personal data of 04 14 (brief M4-14 Design note 5): the echo of this device
/// (`updated_by_device_id`) is ignored, the read state only increases (`mergeReadState`) and
/// sets the counters from the payload, a draft being typed on this device keeps its local text,
/// settings, sidebar (spaces and channels), notification preferences and quiet hours apply per row by version.
/// The answer to an empty draft write stores no row: the server deleted the draft. Neither does
/// the answer to a draft that a send of this device has overtaken: the server deletes it with
/// that send (docs/decisions.d/FIX-COMPOSER-DRAFT-LOST.md).
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import '../dispatcher.dart';
import '../outbox/executors.dart' show decodePayload;
import '../sync_events.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> personalAppliers = {
  'read_state.updated': _readState,
  'draft.updated': _draftUpdated,
  'draft.deleted': _draftDeleted,
  'settings.updated': _settings,
  'sidebar.updated': _sidebar,
  'sidebar_channel.updated': _sidebarChannel,
  'notification_pref.updated': _notificationPref,
  'quiet_hours.updated': _quietHours,
};

bool _ownEcho(ApplyContext c, DeviceId? device) =>
    device != null && device == c.sync.identity.deviceId;

/// The read state is merged also when it is this device's echo: the merge is monotonic and
/// versioned, so the echo changes nothing the write path did not store, and the counters come
/// only from the server (D-M4-14-7).
Future<void> _readState(ApplyContext c) async {
  final incoming = readStateFromJson(obj(c.payload, 'read_state'));
  final merged = await c.db.personalDao.applyReadState(incoming);
  if (merged.version != incoming.version) return; // stale: an older version
  final thread = await c.db.threadsDao.byId(incoming.threadId);
  final lastMessageSeq = thread?.lastMessageSeq ?? merged.lastReadSeq;
  final floor = thread?.unreadFloorSeq ?? const ThreadSeq(0);
  final sync = c.sync;
  final seq = c.event.seq;
  c.after.steps.add(() {
    sync.counter.resetFrom(
      ReadState(
        threadId: incoming.threadId,
        channelId: incoming.channelId,
        lastReadSeq: merged.lastReadSeq,
        mentionCount: incoming.mentionCount,
        unreadCount: incoming.unreadCount,
        version: merged.version,
      ),
      lastMessageSeq: lastMessageSeq,
      unreadFloorSeq: floor,
      at: seq == null ? null : GlobalSeq(seq),
    );
    sync.countedThreads.add(incoming.threadId);
    sync.counterChannels[incoming.threadId] = incoming.channelId;
    sync.counterChanges.add(incoming.threadId);
  });
}

Future<void> _draftUpdated(ApplyContext c) async {
  final draft = draftFromJson(obj(c.payload, 'draft'));
  if (_ownEcho(c, draft.updatedByDeviceId)) return;
  // 04 14: a device takes over a change of another device only when the person is not typing
  // in this draft on it; otherwise the next local write wins.
  if (c.sync.draftEditing.isEditing(
    channelId: draft.channelId,
    threadId: draft.threadId,
  )) {
    return;
  }
  await c.db.personalDao.upsertDraftIfNewer(draft);
}

Future<void> _draftDeleted(ApplyContext c) async {
  final p = c.payload;
  final threadId = optStr(p, 'thread_id');
  if (c.sync.draftEditing.isEditing(
    channelId: ChannelId.parse(str(p, 'channel_id')),
    threadId: threadId == null ? null : ThreadId.parse(threadId),
  )) {
    return;
  }
  await c.db.personalDao.deleteDraft(DraftId.parse(str(p, 'draft_id')));
}

Future<void> _settings(ApplyContext c) async {
  final setting = settingFromJson(obj(c.payload, 'setting'));
  if (_ownEcho(c, setting.updatedByDeviceId)) return;
  if (flag(c.payload, 'deleted')) {
    await c.db.personalDao.deleteSettingIfNewer(setting.key, setting.version);
  } else {
    await c.db.personalDao.upsertSettingIfNewer(setting);
  }
}

Future<void> _sidebar(ApplyContext c) async {
  final state = sidebarFromJson(obj(c.payload, 'state'));
  if (_ownEcho(c, state.updatedByDeviceId)) return;
  await c.db.structureDao.upsertUserSpaceStateIfNewer(state);
}

/// `sidebar_channel.updated` (M4-51): the own thread order of a channel by version; the echo of
/// this device is ignored as for the space rows.
Future<void> _sidebarChannel(ApplyContext c) async {
  final state = sidebarChannelFromJson(obj(c.payload, 'state'));
  if (_ownEcho(c, state.updatedByDeviceId)) return;
  await c.db.structureDao.upsertUserChannelStateIfNewer(state);
}

Future<void> _notificationPref(ApplyContext c) async {
  final pref = preferenceFromJson(obj(c.payload, 'preference'));
  if (flag(c.payload, 'deleted')) {
    await c.db.personalDao.deleteNotificationPreferenceIfNewer(
      pref.targetKind,
      pref.targetId,
      pref.version,
    );
  } else {
    await c.db.personalDao.upsertNotificationPreferenceIfNewer(pref);
  }
}

/// `quiet_hours.updated` carries every rule of the person: rules not in it were deleted.
Future<void> _quietHours(ApplyContext c) async {
  final rules = [
    for (final r in objList(c.payload, 'rules')) quietHoursFromJson(r),
  ];
  final ids = {for (final r in rules) r.id};
  for (final stored in await c.db.personalDao.quietHours()) {
    if (!ids.contains(stored.id)) {
      await c.db.personalDao.deleteQuietHours(stored.id);
    }
  }
  for (final rule in rules) {
    await c.db.personalDao.upsertQuietHoursIfNewer(rule);
  }
}

/// Stores the answer of `putThreadDraft` or `putChannelDraft` (API `DraftPutResult`) of the
/// app's write path and emits [DraftOverwritten] when the server reports `overwrote_newer`
/// (04 14).
///
/// The answer to an empty draft is the empty final state of a draft the server deleted, so it
/// is no row: the stored draft of the target ends, up to the time of the answer, and a draft
/// another device wrote after it stays. The answer to a draft that a send of this device has
/// overtaken ([_sendWaits]) is no row either: the draft ended on this device with that send,
/// and the server deletes it when the send arrives (FIX-CDL/9 in
/// docs/decisions.d/FIX-COMPOSER-DRAFT-LOST.md).
Future<void> storeDraftPutResult(SyncContext sync, Json result) async {
  final draft = draftFromJson(obj(result, 'draft'));
  if (draft.contentMd.trim().isEmpty && draft.fileIds.isEmpty) {
    await sync.db.personalDao.endDraft(
      channelId: draft.channelId,
      threadId: draft.threadId,
      until: draft.updatedAt,
    );
  } else {
    // One transaction: a send queued meanwhile is either seen here, or it ends the stored
    // draft after this row was written.
    await sync.db.transaction(() async {
      if (await _sendWaits(sync.db, draft)) return;
      await sync.db.personalDao.upsertDraftIfNewer(draft);
    });
  }
  if (flag(result, 'overwrote_newer')) sync.notify(DraftOverwritten(draft));
}

/// Whether a send of this device that ends the draft of [draft]'s target waits in the outbox:
/// an entry without a final state whose payload carries `clear_draft`, which is a message of
/// the draft's thread or, for the draft of a new post, a new post in its forum (`channel_id`).
///
/// A draft write and the sends of its target share one lane, so such a send was queued after
/// the write whose answer [draft] is.
Future<bool> _sendWaits(LocalDatabase db, Draft draft) async {
  final thread = draft.threadId;
  for (final entry in await db.outboxDao.entriesFor(thread)) {
    if (entry.state.isFinal) continue;
    final payload = decodePayload(entry.payload);
    if (payload['clear_draft'] != true) continue;
    if (thread != null || payload['channel_id'] == draft.channelId.value) {
      return true;
    }
  }
  return false;
}
