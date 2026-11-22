// SPDX-License-Identifier: Apache-2.0

// The pinned posts of a forum group (01 6.24, FEAT-PIN-CHANNELS): they come first in the order
// of their pins, before the order by activity, the held order and the manual order; they never
// hide behind "Show more"; their rows say they are pinned; text channels ignore pins.

import 'package:client/sidebar/sidebar_threads.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';

const SidebarThreadLimits limits = SidebarThreadLimits(
  window: Duration(hours: 72),
  perChannel: 3,
  forumPostsVisible: 3,
);

final Channel forum = channel(1, kind: ChannelKind.forum, name: 'server');

List<int> ids(List<SidebarThread> rows) => [
  for (final r in rows)
    [for (var n = 0; n < 50; n++) n]
        .firstWhere((n) => threadId(n) == r.thread.id),
];

SidebarThreadGroup select(
  List<Thread> threads, {
  Channel? of,
  List<ThreadId> pinned = const [],
  List<ThreadId>? manualOrder,
  bool showAll = false,
  List<ThreadId> heldOrder = const [],
  Set<ThreadId> keep = const {},
}) => selectSidebarThreads(
  channel: of ?? forum,
  threads: threads,
  counts: const {},
  runs: const [],
  selected: null,
  me: alex,
  now: t0.add(const Duration(minutes: 30)),
  limits: limits,
  showAllPosts: showAll,
  manualOrder: manualOrder,
  heldOrder: heldOrder,
  keep: keep,
  pinned: pinned,
);

void main() {
  // Post n was last active n minutes after t0: by activity 5, 4, 3, 2, 1.
  final posts = [for (var n = 1; n <= 5; n++) thread(n, activeMinutes: n)];

  test('pinned posts come first in the order of their pins, the rest stays '
      'by activity, and the rows say they are pinned', () {
    final g = select(posts, showAll: true, pinned: [threadId(1), threadId(3)]);
    expect(ids(g.rows), [1, 3, 5, 4, 2]);
    expect(
      [for (final r in g.rows) r.rowData.pinned],
      [true, true, false, false, false],
    );
  });

  test('order and heldOrder leave the pins out, so a move and the hold keep '
      'the place a post returns to after its unpin', () {
    final g = select(posts, showAll: true, pinned: [threadId(2)]);
    expect(ids(g.order), [5, 4, 3, 2, 1]);
    expect(g.heldOrder, [
      for (final n in [5, 4, 3, 2, 1]) threadId(n),
    ]);
    expect(ids(g.rows), [2, 5, 4, 3, 1]);
  });

  test('a pinned post never hides behind "Show more"', () {
    final g = select(posts, pinned: [threadId(1)]);
    expect(ids(g.rows), [1, 5, 4]);
    expect(g.more?.hidden, 2);
  });

  test('pins come before the manual order and before the held order', () {
    expect(
      ids(
        select(
          posts,
          showAll: true,
          pinned: [threadId(2)],
          manualOrder: [for (var n = 1; n <= 5; n++) threadId(n)],
        ).rows,
      ),
      [2, 1, 3, 4, 5],
    );
    final held = [
      for (final n in [5, 4, 3, 2, 1]) threadId(n),
    ];
    expect(
      ids(
        select(
          posts,
          showAll: true,
          pinned: [threadId(1)],
          heldOrder: held,
          keep: held.toSet(),
        ).rows,
      ),
      [1, 5, 4, 3, 2],
      reason: 'a pin applies at once, also while the sidebar holds its rows',
    );
  });

  test('pins of posts that are gone or done are ignored, and a text channel '
      'ignores pins', () {
    final done = thread(6, activeMinutes: 9, state: ThreadState.closed);
    expect(
      ids(
        select(
          [...posts, done],
          showAll: true,
          pinned: [threadId(40), threadId(6)],
        ).rows,
      ),
      [5, 4, 3, 2, 1],
    );
    final text = channel(1, name: 'general');
    final g = select(posts, of: text, pinned: [threadId(1)]);
    expect(g.rows.any((r) => r.rowData.pinned), isFalse);
  });
}
