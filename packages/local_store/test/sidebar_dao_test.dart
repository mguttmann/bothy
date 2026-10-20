// SPDX-License-Identifier: Apache-2.0

// The DAO methods M4-19 added for the sidebar and the post list (brief M4-19 Design notes 1, 2
// and 4): the local write of a sidebar state, every read state and notification preference,
// the active runs, and the Done collapse state per forum on this device (`forum_view_states`).

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

RunSummary run(int n, RunState state, {int threadN = 1, int channelN = 1}) =>
    RunSummary(
      id: RunId.parse(uuid(0x700 + n)),
      threadId: threadId(threadN),
      channelId: ChannelId.parse(uuid(0x200 + channelN)),
      agentId: robin,
      triggeredBy: alex,
      state: state,
      version: const ResourceVersion(1),
      billingModeExpected: BillingMode.organization,
      cost: const Money(amountMicros: 0, currency: 'USD', isEstimate: true),
      stepCount: 0,
      queuedAt: at(n),
    );

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  final spaceA = SpaceId.parse(uuid(0x101));

  test(
    'a local sidebar write keeps the version and creates a missing row',
    () async {
      final dao = db.structureDao;
      await dao.putUserSpaceStateLocally(spaceA, collapsed: true);
      var states = await dao.watchUserSpaceStates().first;
      expect(states.single.collapsed, isTrue);
      expect(states.single.sortKey, isNull);
      expect(states.single.version, const ResourceVersion(0));

      await dao.upsertUserSpaceStateIfNewer(
        UserSpaceState(
          spaceId: spaceA,
          collapsed: true,
          sortKey: 'a1',
          version: const ResourceVersion(4),
        ),
      );
      await dao.putUserSpaceStateLocally(spaceA, collapsed: false);
      states = await dao.watchUserSpaceStates().first;
      expect(states.single.collapsed, isFalse);
      expect(states.single.sortKey, 'a1', reason: 'collapse keeps the order');
      expect(states.single.version, const ResourceVersion(4));

      await dao.putUserSpaceStateLocally(
        spaceA,
        collapsed: false,
        sortKey: 'a0V',
      );
      states = await dao.watchUserSpaceStates().first;
      expect(states.single.sortKey, 'a0V');
      expect(states.single.version, const ResourceVersion(4));
    },
  );

  test('every read state and every notification preference', () async {
    final dao = db.personalDao;
    for (final (n, channelN) in [(1, 1), (2, 2)]) {
      await dao.applyReadState(
        ReadState(
          threadId: threadId(n),
          channelId: ChannelId.parse(uuid(0x200 + channelN)),
          lastReadSeq: const ThreadSeq(1),
          mentionCount: n,
          version: const ResourceVersion(1),
        ),
      );
    }
    expect(await dao.watchAllReadStates().first, hasLength(2));
    await dao.upsertNotificationPreferenceIfNewer(
      NotificationPreference(
        targetKind: NotificationTargetKind.channel,
        targetId: uuid(0x201),
        level: NotificationLevel.mute,
        version: const ResourceVersion(1),
      ),
    );
    final prefs = await dao.watchNotificationPreferences().first;
    expect(prefs.single.level, NotificationLevel.mute);
  });

  test('the active runs are those not ended', () async {
    final dao = db.runsDao;
    await dao.upsertIfNewer(run(1, RunState.running));
    await dao.upsertIfNewer(run(2, RunState.waitingInput, threadN: 2));
    await dao.upsertIfNewer(run(3, RunState.completed, threadN: 3));
    await dao.upsertIfNewer(run(4, RunState.queued, threadN: 4, channelN: 2));
    final active = await dao.watchActive().first;
    expect(
      [for (final r in active) r.id],
      [
        for (final n in [1, 2, 4]) RunId.parse(uuid(0x700 + n)),
      ],
    );
  });

  test(
    'the Done section starts collapsed and keeps its state per forum',
    () async {
      final dao = db.forumViewDao;
      final forum = ChannelId.parse(uuid(0x201));
      final other = ChannelId.parse(uuid(0x202));
      expect(await dao.watchDoneCollapsed(forum).first, isTrue);
      await dao.setDoneCollapsed(forum, collapsed: false);
      expect(await dao.watchDoneCollapsed(forum).first, isFalse);
      expect(await dao.watchDoneCollapsed(other).first, isTrue);
      await dao.setDoneCollapsed(forum, collapsed: true);
      expect(await dao.watchDoneCollapsed(forum).first, isTrue);
    },
  );

  test('a resync keeps the Done state: it is no mirror table', () {
    expect([
      for (final t in db.mirrorTables) t.actualTableName,
    ], isNot(contains('forum_view_states')));
  });
}
