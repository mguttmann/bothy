// SPDX-License-Identifier: Apache-2.0

// The personal channel and thread orders of the sidebar (M4-51, 04 14): `sidebar.updated` and
// the snapshot member `sidebar` carry `channel_order`; `sidebar_channel.updated` and the
// snapshot member `sidebar_channels` store the thread order of a channel by version, the echo
// of this device is ignored, and a whole snapshot replaces the rows it does not list.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final ch1 = ChannelId.parse(channelId(1));
  final ch2 = ChannelId.parse(channelId(2));
  final t1 = ThreadId.parse(threadId(1));
  final t2 = ThreadId.parse(threadId(2));

  Map<String, Object?> spaceState({
    required int version,
    List<String>? channelOrder,
    String? device,
  }) => {
    'space_id': spaceId(1),
    'collapsed': false,
    'channel_order': ?channelOrder,
    'version': version,
    'updated_by_device_id': ?device,
  };

  Map<String, Object?> channelState(
    String channel, {
    required int version,
    required List<String> threadOrder,
    String? device,
  }) => {
    'channel_id': channel,
    'thread_order': threadOrder,
    'version': version,
    'updated_by_device_id': ?device,
  };

  Map<String, Object?> sidebarUpdated(int seq, Map<String, Object?> state) =>
      envelope(
        'sidebar.updated',
        seq: seq,
        payload: {'state': state},
        resourceKind: 'space',
        resourceId: state['space_id']! as String,
        personal: true,
      );

  Map<String, Object?> sidebarChannelUpdated(
    int seq,
    Map<String, Object?> state,
  ) => envelope(
    'sidebar_channel.updated',
    seq: seq,
    payload: {'state': state},
    resourceKind: 'channel',
    resourceId: state['channel_id']! as String,
    personal: true,
  );

  Future<List<UserSpaceState>> spaceRows() =>
      w.db.structureDao.watchUserSpaceStates().first;

  Future<List<UserChannelState>> channelRows() =>
      w.db.structureDao.watchUserChannelStates().first;

  test('the wire maps channel_order and SidebarChannelState', () {
    expect(sidebarFromJson(spaceState(version: 1)).channelOrder, isNull);
    expect(
      sidebarFromJson(
        spaceState(version: 1, channelOrder: [channelId(2), channelId(1)]),
      ).channelOrder,
      [ch2, ch1],
    );
    expect(
      sidebarChannelFromJson(
        channelState(
          channelId(1),
          version: 4,
          threadOrder: [threadId(2), threadId(1)],
          device: otherDevice,
        ),
      ),
      UserChannelState(
        channelId: ch1,
        threadOrder: [t2, t1],
        version: const ResourceVersion(4),
        updatedByDeviceId: DeviceId.parse(otherDevice),
      ),
    );
  });

  test('sidebar.updated stores the channel order by version', () async {
    await w.connect(cursor: 30);
    await w.dispatch([
      sidebarUpdated(
        31,
        spaceState(
          version: 2,
          channelOrder: [channelId(2), channelId(1)],
          device: otherDevice,
        ),
      ),
    ]);
    await w.push('cursor', {'seq': 31});
    expect((await spaceRows()).single.channelOrder, [ch2, ch1]);

    // The default order again: the member is absent.
    await w.dispatch([
      sidebarUpdated(32, spaceState(version: 3, device: otherDevice)),
    ]);
    await w.push('cursor', {'seq': 32});
    expect((await spaceRows()).single.channelOrder, isNull);
  });

  test(
    'sidebar_channel.updated stores the thread order by version, not the echo',
    () async {
      await w.connect(cursor: 30);
      await w.dispatch([
        sidebarChannelUpdated(
          31,
          channelState(
            channelId(1),
            version: 2,
            threadOrder: [threadId(2), threadId(1)],
            device: otherDevice,
          ),
        ),
        // Older: ignored.
        sidebarChannelUpdated(
          32,
          channelState(
            channelId(1),
            version: 1,
            threadOrder: [threadId(1)],
            device: otherDevice,
          ),
        ),
        // The echo of this device: ignored.
        sidebarChannelUpdated(
          33,
          channelState(
            channelId(2),
            version: 5,
            threadOrder: [threadId(1)],
            device: deviceId,
          ),
        ),
      ]);
      await w.push('cursor', {'seq': 33});
      final rows = await channelRows();
      expect(rows.single.channelId, ch1);
      expect(rows.single.threadOrder, [t2, t1]);
      expect(rows.single.version, const ResourceVersion(2));
    },
  );

  test(
    'a whole snapshot stores sidebar and sidebar_channels and drops other rows',
    () async {
      // Rows of an earlier session that the new snapshot does not list.
      await w.db.structureDao.upsertUserChannelStateIfNewer(
        UserChannelState(
          channelId: ch2,
          threadOrder: [t1],
          version: const ResourceVersion(9),
        ),
      );
      final snapshot = snapshotJson()
        ..['sidebar'] = [
          spaceState(version: 3, channelOrder: [channelId(2), channelId(1)]),
        ]
        ..['sidebar_channels'] = [
          channelState(
            channelId(1),
            version: 2,
            threadOrder: [threadId(2), threadId(1)],
          ),
        ];
      await w.connect(snapshot: snapshot);
      expect((await spaceRows()).single.channelOrder, [ch2, ch1]);
      final rows = await channelRows();
      expect(rows.single.channelId, ch1);
      expect(rows.single.threadOrder, [t2, t1]);
    },
  );

  test('a snapshot of an older server without sidebar_channels', () {
    final s = snapshotFromJson(snapshotJson()..remove('sidebar_channels'));
    expect(s.sidebarChannels, isEmpty);
  });
}
