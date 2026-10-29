// SPDX-License-Identifier: Apache-2.0

// The DAO methods M4-19 added for the sidebar and the post list (brief M4-19 Design notes 1, 2
// and 4): the local write of a sidebar state, every read state and notification preference,
// the active runs, and the Done collapse state per forum on this device (`forum_view_states`);
// and those of M4-40 (01 6.8 item 7, D-SB-6): the collapsed thread groups per channel
// (`channel_view_states`) and the pinned forum overview (`shell_view_state`); and that of M4-42
// (01 D1, D2): the hidden channel column next to an open thread (`shell_view_state`); and those
// of M4-45 (01 D1, D2): the hidden session column per layout (`shell_view_state`); and that of
// M4-46 (01 6.8 item 3): the forum groups expanded to every open post (`channel_view_states`);
// and those of FIX-device-mirroring-2 (01 6.7, R-PR-24): the collapsed spaces of this device
// (`space_view_states`).

import 'dart:async';

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

  test(
    'thread groups start expanded and keep their state per channel',
    () async {
      final dao = db.forumViewDao;
      final forum = ChannelId.parse(uuid(0x201));
      final text = ChannelId.parse(uuid(0x202));
      final seen = <Set<ChannelId>>[];
      final watching = dao.watchCollapsedGroups().listen(seen.add);
      addTearDown(watching.cancel);

      Future<void> until(Set<ChannelId> expected) async {
        while (seen.isEmpty || !_sameSet(seen.last, expected)) {
          await Future<void>.delayed(Duration.zero);
        }
      }

      await until({});
      await dao.setGroupCollapsed(forum, collapsed: true);
      await until({forum});
      await dao.setGroupCollapsed(text, collapsed: true);
      await until({forum, text});
      await dao.setGroupCollapsed(forum, collapsed: false);
      await until({text});
      expect(await dao.watchCollapsedGroups().first, {text});
    },
  );

  test('forum groups show the limit until Show more is stored per channel, and '
      'the collapse state and Show more keep each other', () async {
    final dao = db.forumViewDao;
    final forum = ChannelId.parse(uuid(0x201));
    final other = ChannelId.parse(uuid(0x202));
    expect(await dao.watchPostListsShownInFull().first, <ChannelId>{});

    await dao.setPostListShownInFull(forum, inFull: true);
    expect(await dao.watchPostListsShownInFull().first, {forum});
    expect(
      await dao.watchCollapsedGroups().first,
      <ChannelId>{},
      reason: 'a new row keeps the group expanded',
    );

    await dao.setGroupCollapsed(forum, collapsed: true);
    expect(await dao.watchCollapsedGroups().first, {forum});
    expect(await dao.watchPostListsShownInFull().first, {
      forum,
    }, reason: 'collapsing the group keeps Show more');

    await dao.setPostListShownInFull(other, inFull: true);
    await dao.setPostListShownInFull(forum, inFull: false);
    expect(await dao.watchPostListsShownInFull().first, {other});
    expect(await dao.watchCollapsedGroups().first, {
      forum,
    }, reason: 'Show less keeps the collapse state');
  });

  test(
    'the overview is not pinned until it is set, read once or watched',
    () async {
      final dao = db.forumViewDao;
      expect(await dao.overviewPinned(), isFalse);
      expect(await dao.watchOverviewPinned().first, isFalse);
      final seen = <bool>[];
      final watching = dao.watchOverviewPinned().listen(seen.add);
      addTearDown(watching.cancel);
      while (seen.isEmpty) {
        await Future<void>.delayed(Duration.zero);
      }

      await dao.setOverviewPinned(pinned: true);
      expect(await dao.overviewPinned(), isTrue);
      while (seen.isEmpty || !seen.last) {
        await Future<void>.delayed(Duration.zero);
      }
      await dao.setOverviewPinned(pinned: false);
      expect(await dao.overviewPinned(), isFalse);
      while (seen.last) {
        await Future<void>.delayed(Duration.zero);
      }
      // drift may emit an unchanged value again; the changes come in order.
      final changes = [
        for (final (i, v) in seen.indexed)
          if (i == 0 || seen[i - 1] != v) v,
      ];
      expect(changes, [false, true, false]);
      // One row, like sync_state.
      final rows = await db
          .customSelect('SELECT count(*) AS n FROM shell_view_state')
          .getSingle();
      expect(rows.read<int>('n'), 1);
    },
  );

  test('the channel column is shown until it is hidden; hiding keeps the pin and back', () async {
    final dao = db.forumViewDao;
    expect(await dao.channelColumnHidden(), isFalse);
    expect(await dao.watchChannelColumnHidden().first, isFalse);

    await dao.setOverviewPinned(pinned: true);
    await dao.setChannelColumnHidden(hidden: true);
    expect(await dao.channelColumnHidden(), isTrue);
    expect(await dao.watchChannelColumnHidden().first, isTrue);
    expect(await dao.overviewPinned(), isTrue, reason: 'the pin stays');

    await dao.setOverviewPinned(pinned: false);
    expect(await dao.channelColumnHidden(), isTrue, reason: 'stays hidden');
    await dao.setChannelColumnHidden(hidden: false);
    expect(await dao.watchChannelColumnHidden().first, isFalse);
    expect(await dao.overviewPinned(), isFalse);
    final rows = await db
        .customSelect('SELECT count(*) AS n FROM shell_view_state')
        .getSingle();
    expect(rows.read<int>('n'), 1);
  });

  test(
    'the session column shows beside a forum post and is collapsed beside a '
    'thread of a text channel until stored otherwise; both are kept apart',
    () async {
      final dao = db.forumViewDao;
      // 01 D1, D2: shown in the forum layout, collapsed beside a text channel's thread.
      expect(await dao.sessionColumnHidden(textThread: false), isFalse);
      expect(await dao.sessionColumnHidden(textThread: true), isTrue);
      expect(
        await dao.watchSessionColumnHidden(textThread: false).first,
        isFalse,
      );
      expect(
        await dao.watchSessionColumnHidden(textThread: true).first,
        isTrue,
      );

      await dao.setOverviewPinned(pinned: true);
      await dao.setChannelColumnHidden(hidden: true);
      await dao.setSessionColumnHidden(textThread: false, hidden: true);
      expect(await dao.sessionColumnHidden(textThread: false), isTrue);
      expect(
        await dao.watchSessionColumnHidden(textThread: false).first,
        isTrue,
      );
      expect(
        await dao.sessionColumnHidden(textThread: true),
        isTrue,
        reason: 'the other layout keeps its own value',
      );
      expect(await dao.overviewPinned(), isTrue, reason: 'the pin stays');
      expect(
        await dao.channelColumnHidden(),
        isTrue,
        reason: 'the channel stays',
      );

      await dao.setSessionColumnHidden(textThread: true, hidden: false);
      expect(
        await dao.watchSessionColumnHidden(textThread: true).first,
        isFalse,
      );
      expect(await dao.sessionColumnHidden(textThread: false), isTrue);
      await dao.setSessionColumnHidden(textThread: false, hidden: false);
      expect(await dao.sessionColumnHidden(textThread: false), isFalse);
      expect(await dao.sessionColumnHidden(textThread: true), isFalse);
      final rows = await db
          .customSelect('SELECT count(*) AS n FROM shell_view_state')
          .getSingle();
      expect(rows.read<int>('n'), 1);
    },
  );

  test('the collapse of a space is kept on this device and is independent of '
      'the synchronized sidebar state', () async {
    final spaceB = SpaceId.parse(uuid(0x102));
    final dao = db.forumViewDao;
    expect(await dao.watchCollapsedSpaces().first, isEmpty);
    await dao.setSpaceCollapsed(spaceA, collapsed: true);
    await dao.setSpaceCollapsed(spaceB, collapsed: true);
    expect(await dao.watchCollapsedSpaces().first, {spaceA, spaceB});
    await dao.setSpaceCollapsed(spaceB, collapsed: false);
    expect(await dao.watchCollapsedSpaces().first, {spaceA});

    // A `sidebar.updated` of another device writes `user_space_states` only.
    await db.structureDao.upsertUserSpaceStateIfNewer(
      UserSpaceState(
        spaceId: spaceA,
        collapsed: false,
        version: const ResourceVersion(5),
      ),
    );
    expect(await dao.watchCollapsedSpaces().first, {spaceA});
    expect([
      for (final t in db.mirrorTables) t.actualTableName,
    ], isNot(contains('space_view_states')));
  });

  test('a resync keeps the group and overview state: they are no mirror '
      'tables', () {
    expect(
      [for (final t in db.mirrorTables) t.actualTableName],
      isNot(
        anyOf(contains('channel_view_states'), contains('shell_view_state')),
      ),
    );
  });
}

bool _sameSet(Set<ChannelId> a, Set<ChannelId> b) =>
    a.length == b.length && a.containsAll(b);
