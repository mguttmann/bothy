// SPDX-License-Identifier: Apache-2.0

// The pure rules of the sidebar (brief M4-19 Design notes 1, 2 and 5): the personal order with
// its fallback, the key of a move, the counters of a channel, the muted state, the mapping to
// the row data of M4-07 and the navigation controller for M4-24.

import 'package:client/sidebar/sidebar_model.dart';
import 'package:client/sidebar/sidebar_order.dart';
import 'package:client/sidebar/sidebar_threads.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';

UserSpaceState state(int n, {String? key, bool collapsed = false}) =>
    UserSpaceState(
      spaceId: spaceId(n),
      collapsed: collapsed,
      sortKey: key,
      version: const ResourceVersion(1),
    );

void main() {
  group('personal order', () {
    test('the personal key wins, the global order is the fallback', () {
      final order = orderSpaces(
        [space(1, key: 'a1'), space(2, key: 'a2'), space(3, key: 'a3')],
        [state(3, key: 'a0V'), state(2, collapsed: true)],
      );
      expect(
        [for (final s in order) s.space.id],
        [spaceId(3), spaceId(1), spaceId(2)],
      );
      expect(order.last.collapsed, isTrue);
      expect(order.first.collapsed, isFalse);
    });

    test('archived spaces are not shown', () {
      final archived = Space(
        id: spaceId(9),
        name: 'Old',
        sortKey: 'a0',
        createdAt: t0,
        archivedAt: t0,
        version: const ResourceVersion(1),
      );
      expect(orderSpaces([space(1), archived], const []), hasLength(1));
    });

    test('a move writes one key between the new neighbours', () {
      final order = orderSpaces([space(1), space(2), space(3)], const []);
      final writes = moveSpace(order, spaceId(3), 0);
      expect(writes, hasLength(1));
      expect(writes.single.spaceId, spaceId(3));
      expect(writes.single.sortKey.compareTo('a1'), lessThan(0));
      expect(FractionalIndex.isValid(writes.single.sortKey), isTrue);
      final between = moveSpace(order, spaceId(1), 1);
      expect(between.single.sortKey, FractionalIndex.between('a2', 'a3'));
      expect(moveSpace(order, spaceId(2), 1), isEmpty, reason: 'no change');
    });

    test('neighbours without room get fresh keys, collapse states kept', () {
      // Two spaces with the same global key, and one whose key is no valid key.
      final order = orderSpaces(
        [space(1, key: 'a1'), space(2, key: 'a1'), space(3, key: 'a10')],
        [state(1, collapsed: true)],
      );
      final writes = moveSpace(order, spaceId(3), 1);
      final keys = {for (final w in writes) w.spaceId: w.sortKey};
      expect(keys.keys, containsAll([spaceId(1), spaceId(3)]));
      final after = [spaceId(1), spaceId(3), spaceId(2)];
      for (var i = 1; i < after.length; i++) {
        final a = keys[after[i - 1]] ?? order[1].orderKey;
        final b = keys[after[i]] ?? order[1].orderKey;
        expect(a.compareTo(b), lessThan(0));
      }
      expect(
        writes.firstWhere((w) => w.spaceId == spaceId(1)).collapsed,
        isTrue,
      );
      for (final w in writes) {
        expect(FractionalIndex.isValid(w.sortKey), isTrue);
      }
    });
  });

  group('channels', () {
    test('counters: the engine first, the stored read state otherwise', () {
      final counts = channelCounts(
        engine: {threadId(1): const ThreadCounts(unread: 3, mentions: 1)},
        readStates: [
          readState(1, mentions: 5, unread: 5),
          readState(2, mentions: 2),
          readState(3, unread: 4),
          readState(4, channelN: 2, unread: 9),
        ],
        channel: channelId(1),
      );
      expect(counts, const ThreadCounts(unread: 7, mentions: 3));
    });

    test('muted by the own row whose level after 13 3.1 is mute (M4-33)', () {
      NotificationPreference pref(
        NotificationLevel level, {
        DateTime? until,
        NotificationTargetKind kind = NotificationTargetKind.channel,
      }) => NotificationPreference(
        targetKind: kind,
        targetId: channelId(1).value,
        level: level,
        mutedUntil: until,
        version: const ResourceVersion(1),
      );
      final now = t0;
      final later = now.add(const Duration(hours: 1));
      final earlier = now.subtract(const Duration(minutes: 1));
      bool muted(List<NotificationPreference> prefs) =>
          isMuted(channelId(1), prefs, now: now);
      expect(muted([pref(NotificationLevel.mute)]), isTrue);
      expect(muted([pref(NotificationLevel.mute, until: later)]), isTrue);
      // A passed muted_until has no effect; the stored level applies (D-M3-18-3).
      expect(muted([pref(NotificationLevel.mute, until: earlier)]), isTrue);
      // "Mute until" keeps the level that applies afterwards (D-M4-33-2).
      expect(muted([pref(NotificationLevel.all, until: later)]), isTrue);
      expect(muted([pref(NotificationLevel.all, until: earlier)]), isFalse);
      expect(muted([pref(NotificationLevel.mentions)]), isFalse);
      expect(
        muted([
          pref(NotificationLevel.mute, kind: NotificationTargetKind.space),
        ]),
        isFalse,
      );
    });

    test('the row data of a channel', () {
      final entry = SidebarChannel(
        channel: channel(1, kind: ChannelKind.forum, name: 'questions'),
        counts: const ThreadCounts(unread: 3, mentions: 2),
        runActive: true,
        muted: false,
      );
      final row = entry.rowData;
      expect(row.id, channelId(1).value);
      expect(row.name, 'questions');
      expect(row.kind, DsChannelKind.forum);
      expect(row.unreadCount, 3);
      expect(row.mentionCount, 2);
      expect(row.runActive, isTrue);
      expect(row.muted, isFalse);
    });

    test('the row data of a space: the dot needs unread items', () {
      final quiet = SidebarChannel(
        channel: channel(1),
        counts: ThreadCounts.zero,
        runActive: false,
        muted: false,
      );
      final busy = SidebarChannel(
        channel: channel(2),
        counts: const ThreadCounts(unread: 0, mentions: 1),
        runActive: false,
        muted: false,
      );
      final node = SidebarSpaceNode(
        entry: SidebarSpace(
          space: space(1, icon: 'rocket', color: 'rowan'),
        ),
        channels: [quiet, busy],
      );
      expect(node.rowData.hasUnread, isTrue);
      expect(node.rowData.expanded, isTrue);
      expect(node.rowData.category, DsCategory.rowan);
      expect(
        SidebarSpaceNode(
          entry: SidebarSpace(space: space(1)),
          channels: [quiet],
        ).rowData.hasUnread,
        isFalse,
      );
    });
  });

  group('navigation', () {
    SidebarChannel ch(int n, {int unread = 0}) => SidebarChannel(
      channel: channel(n),
      counts: ThreadCounts(unread: unread, mentions: 0),
      runActive: false,
      muted: false,
    );
    final tree = [
      SidebarSpaceNode(
        entry: SidebarSpace(space: space(1)),
        channels: [ch(1), ch(2, unread: 1)],
      ),
      SidebarSpaceNode(
        entry: SidebarSpace(space: space(2), state: state(2, collapsed: true)),
        channels: [ch(3, unread: 2)],
      ),
      SidebarSpaceNode(
        entry: SidebarSpace(space: space(3)),
        channels: [ch(4)],
      ),
    ];

    SidebarTarget at(int n, [int? t]) =>
        SidebarTarget(channelId(n), t == null ? null : threadId(t));

    test('next and previous channel skip collapsed spaces and wrap', () {
      expect(neighbourTarget(tree, at(2), forward: true), at(4));
      expect(neighbourTarget(tree, at(4), forward: true), at(1));
      expect(neighbourTarget(tree, at(1), forward: false), at(4));
      expect(neighbourTarget(tree, null, forward: true), at(1));
    });

    test('next and previous unread channel include collapsed spaces', () {
      expect(
        neighbourTarget(tree, at(1), forward: true, unreadOnly: true),
        at(2),
      );
      expect(
        neighbourTarget(tree, at(2), forward: true, unreadOnly: true),
        at(3),
      );
      expect(
        neighbourTarget(tree, at(2), forward: false, unreadOnly: true),
        at(3),
      );
      expect(
        neighbourTarget(
          [
            tree.first.copyWithChannels([ch(1)]),
          ],
          at(1),
          forward: true,
          unreadOnly: true,
        ),
        isNull,
      );
    });

    // M4-40: thread rows of expanded groups are rows of the tree (01 7, 6.8; D-SB-8).
    SidebarChannel grouped(
      int n,
      List<(int, int)> threads, {
      bool expanded = true,
    }) {
      final rows = [
        for (final (t, unread) in threads)
          SidebarThread(
            thread: thread(t, channelN: n, createdMinutes: -t),
            counts: ThreadCounts(unread: unread, mentions: 0),
          ),
      ];
      return SidebarChannel(
        channel: channel(n, kind: ChannelKind.forum),
        counts: ThreadCounts(
          unread: threads.fold(0, (a, t) => a + t.$2),
          mentions: 0,
        ),
        runActive: false,
        muted: false,
        expanded: expanded,
        group: SidebarThreadGroup(
          rows: rows,
          candidates: rows,
          channelCounts: expanded
              ? ThreadCounts.zero
              : ThreadCounts(
                  unread: threads.fold(0, (a, t) => a + t.$2),
                  mentions: 0,
                ),
          allPostsCounts: expanded ? ThreadCounts.zero : null,
        ),
      );
    }

    final threaded = [
      SidebarSpaceNode(
        entry: SidebarSpace(space: space(1)),
        channels: [
          grouped(1, [(1, 0), (2, 3)]),
          grouped(2, [(3, 1)], expanded: false),
          ch(5),
        ],
      ),
    ];

    test('the channel navigation stops at thread rows', () {
      expect(neighbourTarget(threaded, at(1), forward: true), at(1, 1));
      expect(neighbourTarget(threaded, at(1, 1), forward: true), at(1, 2));
      expect(neighbourTarget(threaded, at(1, 2), forward: true), at(2));
      // A collapsed group shows no rows.
      expect(neighbourTarget(threaded, at(2), forward: true), at(5));
      expect(neighbourTarget(threaded, at(2), forward: false), at(1, 2));
    });

    test('next unread stops at an unread thread row and a collapsed group', () {
      expect(
        neighbourTarget(threaded, at(1), forward: true, unreadOnly: true),
        at(1, 2),
      );
      expect(
        neighbourTarget(threaded, at(1, 2), forward: true, unreadOnly: true),
        at(2),
        reason: 'the collapsed group counts its thread on the channel row',
      );
    });

    test('a selected thread that is no row stands for its channel', () {
      expect(neighbourTarget(threaded, at(5, 9), forward: true), at(1));
    });
  });
}
