// SPDX-License-Identifier: Apache-2.0

/// Appliers of the personal data of 04 14 (brief M4-14 Design note 5): the echo of this device
/// (`updated_by_device_id`) is ignored, the read state only increases (`mergeReadState`) and
/// sets the counters from the payload, a draft being typed on this device keeps its local text,
/// settings, sidebar, notification preferences and quiet hours apply per row by version.
library;

import 'package:domain/domain.dart';

import '../dispatcher.dart';
import '../sync_events.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> personalAppliers = {
  'read_state.updated': _readState,
  'draft.updated': _draftUpdated,
  'draft.deleted': _draftDeleted,
  'settings.updated': _settings,
  'sidebar.updated': _sidebar,
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

/// Stores the answer of `putThreadDraft` (API `DraftPutResult`) of the app's write path and
/// emits [DraftOverwritten] when the server reports `overwrote_newer` (04 14).
Future<void> storeDraftPutResult(SyncContext sync, Json result) async {
  final draft = draftFromJson(obj(result, 'draft'));
  await sync.db.personalDao.upsertDraftIfNewer(draft);
  if (flag(result, 'overwrote_newer')) sync.notify(DraftOverwritten(draft));
}
