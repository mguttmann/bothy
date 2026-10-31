// SPDX-License-Identifier: Apache-2.0

// The manual order of the posts of a forum group (brief M4-51; 01 6.15; D-M4-51-3, D-M4-51-9):
// with "Arrange chats automatically" off the rows follow the person's list, posts not in it come
// first by activity, the limit of "Show more" takes the first posts of that order, and text
// channels keep their rule.

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
  List<ThreadId>? manualOrder,
  bool showAll = false,
  ThreadId? selected,
  List<ThreadId> heldOrder = const [],
}) => selectSidebarThreads(
  channel: forum,
  threads: threads,
  counts: const {},
  runs: const [],
  selected: selected,
  me: alex,
  now: t0.add(const Duration(days: 1)),
  limits: limits,
  showAllPosts: showAll,
  manualOrder: manualOrder,
  heldOrder: heldOrder,
);

void main() {
  // Post n was last active n minutes after t0: by activity 5, 4, 3, 2, 1.
  final posts = [for (var n = 1; n <= 5; n++) thread(n, activeMinutes: n)];

  test('without a manual order the forum is ordered by activity (chats '
      'automatic, M4-46)', () {
    expect(ids(select(posts, showAll: true).rows), [5, 4, 3, 2, 1]);
  });

  test('a manual order puts the posts in its order; posts not in it come '
      'first by activity', () {
    final g = select(
      posts,
      showAll: true,
      manualOrder: [threadId(1), threadId(2), threadId(3)],
    );
    expect(ids(g.rows), [5, 4, 1, 2, 3]);
  });

  test('the limit takes the first posts of the manual order; the rest is '
      'behind "Show more"', () {
    final g = select(
      posts,
      manualOrder: [for (var n = 1; n <= 5; n++) threadId(n)],
    );
    expect(ids(g.rows), [1, 2, 3]);
    expect(g.more?.hidden, 2);
    expect(ids(g.candidates), [1, 2, 3, 4, 5]);
  });

  test('activity never reorders a manual group, and no held order applies '
      '(a drag must show at once)', () {
    final order = [threadId(3), threadId(1), threadId(2)];
    final g = select(
      [thread(1, activeMinutes: 99), thread(2), thread(3)],
      manualOrder: order,
      heldOrder: [threadId(1), threadId(2), threadId(3)],
    );
    expect(ids(g.rows), [3, 1, 2]);
  });

  test('a done post leaves a manual group unless selected', () {
    final g = select(
      [thread(1), thread(2, state: ThreadState.closed), thread(3)],
      showAll: true,
      manualOrder: [threadId(2), threadId(1), threadId(3)],
    );
    expect(ids(g.rows), [1, 3]);
  });
}
