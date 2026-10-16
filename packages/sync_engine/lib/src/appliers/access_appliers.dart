// SPDX-License-Identifier: Apache-2.0

/// Appliers of `access.changed` (04 8.3, 8.4) and `session.revoked` (04 8.5) (brief M4-14
/// Design note 3).
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import '../dispatcher.dart';
import '../wire.dart';
import 'structure_appliers.dart';
import 'thread_appliers.dart';

/// The appliers of this group by event type.
final Map<String, Applier> accessAppliers = {
  'access.changed': _accessChanged,
  'session.revoked': _sessionRevoked,
};

List<TargetRef> _targets(Json payload, String key) => [
  for (final t in objList(payload, key))
    switch (str(t, 'kind')) {
      'space' => TargetRef.space(SpaceId.parse(str(t, 'id'))),
      'channel' => TargetRef.channel(ChannelId.parse(str(t, 'id'))),
      // Hosts are the terminal's (M4-36); other kinds are of a newer server.
      _ => TargetRef(RemovedTargetKind.host, str(t, 'id')),
    },
];

bool _structural(TargetRef t) =>
    t.kind == RemovedTargetKind.space || t.kind == RemovedTargetKind.channel;

Future<void> _accessChanged(ApplyContext c) async {
  final p = c.payload;
  final state = await c.db.syncStateDao.read();
  final session = state.gatewaySessionId;
  final head = state.headSeq;
  if (session != null && head != null) {
    await c.db.syncStateDao.saveSession(
      gatewaySessionId: session,
      permissionVersion: PermissionVersion(integer(p, 'permission_version')),
      headSeq: head,
    );
  }
  final removed = _targets(p, 'removed').where(_structural).toList();
  if (removed.isNotEmpty) await removeTargetsInPage(c, removed);
  final added = _targets(p, 'added').where(_structural).toList();
  final changed = _targets(p, 'changed').where(_structural).toList();
  final sync = c.sync;
  if (added.isNotEmpty) {
    sync.removedTargets.removeAll([for (final t in added) t.id]);
    // 04 8.4: the structure of new targets through REST, no catch-up of older events. A
    // removal while the reads run wins (04 8.3).
    c.after.reads.add(() async {
      final s = await sync.fetcher.loadStructure(added);
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
          if (await sync.holdsContent(sync.db, t.channelId, t.id)) {
            await storeThread(sync.db, t);
          }
        }
      });
    });
  }
  for (final t in changed) {
    // The new level and capabilities come with the view; controls follow them (15 1). Only a
    // target still stored here is updated, so a later removal is not undone.
    c.after.reads.add(() async {
      if (t.kind == RemovedTargetKind.space) {
        final v = await sync.fetcher.space(SpaceId.parse(t.id));
        await sync.db.transaction(() async {
          final known = await sync.db.structureDao.spaceById(v.space.id);
          if (known != null && !sync.removedTargets.contains(t.id)) {
            await storeSpaceView(sync.db, v);
          }
        });
      } else {
        final v = await sync.fetcher.channel(ChannelId.parse(t.id));
        await sync.db.transaction(() async {
          if (await sync.holdsChannel(sync.db, v.channel.id)) {
            await storeChannelView(sync.db, v);
          }
        });
      }
    });
  }
}

/// `session.revoked` names one session of the person; only this device's own session ends
/// here (another device's logout reaches every device of the person, 05 6). An unknown own
/// session counts as this one, so a revocation never leaves data behind (D-M4-14-5).
Future<void> _sessionRevoked(ApplyContext c) async {
  final revoked = str(c.payload, 'session_id');
  final own = c.sync.identity.sessionId;
  if (own != null && own != revoked) return;
  c.stop = true;
}
