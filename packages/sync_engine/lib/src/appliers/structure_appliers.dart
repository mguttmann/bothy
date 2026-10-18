// SPDX-License-Identifier: Apache-2.0

/// Appliers of `space.*` and `channel.*` (04 9 item 3, 04 8.1): spaces and channels by version;
/// archiving and a new `read_only` or operator come with the object (brief M4-14 Design note
/// 2d). The caller's access comes from `SpaceView` and `ChannelView` of REST and the snapshot.
library;

import 'package:local_store/local_store.dart';

import '../dispatcher.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> structureAppliers = {
  'space.created': _space,
  'space.updated': _space,
  'space.archived': _space,
  'channel.created': _channel,
  'channel.updated': _channel,
  'channel.archived': _channel,
};

Future<void> _space(ApplyContext c) => c.db.structureDao.upsertSpaceIfNewer(
  spaceFromJson(obj(c.payload, 'space')),
);

Future<void> _channel(ApplyContext c) => c.db.structureDao.upsertChannelIfNewer(
  channelFromJson(obj(c.payload, 'channel')),
);

/// Stores a space with the caller's access as the server reported it (snapshot, `getSpace`).
Future<void> storeSpaceView(LocalDatabase db, SpaceWithAccess view) async {
  await db.structureDao.upsertSpaceIfNewer(view.space);
  await db.structureDao.putSpaceAccess(view.space.id, view.access);
}

/// Stores a channel with the caller's access as the server reported it (snapshot,
/// `getChannel`).
Future<void> storeChannelView(LocalDatabase db, ChannelWithAccess view) async {
  await db.structureDao.upsertChannelIfNewer(view.channel);
  await db.structureDao.putChannelAccess(view.channel.id, view.access);
}
