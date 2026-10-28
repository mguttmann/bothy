// SPDX-License-Identifier: Apache-2.0

// The selection of the thread rows of a sidebar group (01 6.8 items 2 to 6; brief M4-40 Design
// notes 1 and 3; D-SB-2 to D-SB-5): which threads qualify, the bound and its priority, the order
// by creation, the rows kept while the person points at the sidebar, the counters of the channel
// row and of "All posts", and property tests over random channels.

import 'dart:math';

import 'package:client/sidebar/sidebar_threads.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';

/// Now in the tests: one day after [t0].
final DateTime now = t0.add(const Duration(days: 1));

const SidebarThreadLimits limits = SidebarThreadLimits(
  window: Duration(hours: 72),
  perChannel: 3,
);

/// Thread [n] of the forum (channel 1), created [created] minutes after [t0] and last active
/// [ageHours] hours before [now].
Thread post(
  int n, {
  int created = 0,
  int ageHours = 1,
  ThreadState state = ThreadState.open,
  String? title,
  int channelN = 1,
}) => Thread(
  id: threadId(n),
  channelId: channelId(channelN),
  kind: ThreadKind.thread,
  title: title ?? 'Post $n',
  titleSource: TitleSource.manual,
  state: state,
  lastMessageSeq: const ThreadSeq(3),
  unreadFloorSeq: const ThreadSeq(0),
  lastActivityAt: now.subtract(Duration(hours: ageHours)),
  messageCount: 3,
  createdBy: sam,
  createdAt: t0.add(Duration(minutes: created)),
  version: const ResourceVersion(1),
  tagIds: const [],
);

final Channel forum = channel(1, kind: ChannelKind.forum, name: 'server');
final Channel text = channel(2, name: 'general');

SidebarThreadGroup select(
  List<Thread> threads, {
  Channel? on,
  Map<ThreadId, ThreadCounts> counts = const {},
  List<RunSummary> runs = const [],
  ThreadId? selected,
  bool expanded = true,
  Set<ThreadId> keep = const {},
  Set<ThreadId> muted = const {},
  SidebarThreadLimits l = limits,
}) => selectSidebarThreads(
  channel: on ?? forum,
  threads: threads,
  counts: counts,
  runs: runs,
  selected: selected,
  me: alex,
  now: now,
  limits: l,
  expanded: expanded,
  keep: keep,
  mutedThreads: muted,
);

List<ThreadId> ids(SidebarThreadGroup g) => [
  for (final r in g.rows) r.thread.id,
];

ThreadCounts c(int unread, [int mentions = 0]) =>
    ThreadCounts(unread: unread, mentions: mentions);

void main() {
  group('which threads show (01 6.8 item 2)', () {
    test('a thread active within the window shows, an older one does not', () {
      final g = select([post(1, ageHours: 71), post(2, ageHours: 73)]);
      expect(ids(g), [threadId(1)]);
    });

    test('the selected thread shows, also when old or done', () {
      expect(ids(select([post(1, ageHours: 500)], selected: threadId(1))), [
        threadId(1),
      ]);
      expect(
        ids(
          select([post(1, state: ThreadState.closed)], selected: threadId(1)),
        ),
        [threadId(1)],
      );
    });

    test('a done thread never shows unless selected', () {
      final g = select(
        [post(1, state: ThreadState.closed)],
        counts: {threadId(1): c(4, 1)},
        runs: [run(1, RunState.running)],
      );
      expect(ids(g), isEmpty);
    });

    test(
      'an active run, a waiting run of mine, unread and mentions qualify',
      () {
        final g = select(
          [
            post(1, ageHours: 500, created: 1),
            post(2, ageHours: 500, created: 2),
            post(3, ageHours: 500, created: 3),
            post(4, ageHours: 500, created: 4),
            post(5, ageHours: 500, created: 5),
          ],
          l: const SidebarThreadLimits(
            window: Duration(hours: 72),
            perChannel: 8,
          ),
          runs: [
            run(1, RunState.running, threadN: 1),
            run(2, RunState.waitingInput, threadN: 2),
          ],
          counts: {threadId(3): c(2), threadId(4): c(1, 1)},
        );
        expect(ids(g), [threadId(4), threadId(3), threadId(2), threadId(1)]);
      },
    );

    test(
      'a waiting run that someone else triggered does not make it wait for me',
      () {
        final g = select(
          [post(1, ageHours: 500)],
          runs: [run(1, RunState.waitingInput, triggeredBy: sam)],
        );
        // The run is still active, so the thread shows; it is not "waiting for me".
        expect(ids(g), [threadId(1)]);
        expect(g.rows.single.waitingForMe, isFalse);
      },
    );

    test('the main thread of a text channel is the channel row, never a thread row', () {
      final main = Thread(
        id: text.mainThreadId!,
        channelId: text.id,
        kind: ThreadKind.main,
        title: '',
        titleSource: TitleSource.manual,
        state: ThreadState.open,
        lastMessageSeq: const ThreadSeq(9),
        unreadFloorSeq: const ThreadSeq(0),
        lastActivityAt: now,
        messageCount: 9,
        createdBy: sam,
        createdAt: t0,
        version: const ResourceVersion(1),
        tagIds: const [],
      );
      final g = select([main, post(7, channelN: 2)], on: text);
      expect(ids(g), [threadId(7)]);
    });

    test('threads of another channel are ignored', () {
      expect(ids(select([post(1, channelN: 9)])), isEmpty);
    });
  });

  group('bound and priority (01 6.8 item 3)', () {
    test(
      'selected, waiting, run, mentions, unread, then newest activity win',
      () {
        final threads = [
          for (var i = 1; i <= 7; i++) post(i, created: i, ageHours: 10 + i),
        ];
        // Thread 7 is the oldest activity; 6 is unread; 5 has a mention; 4 runs; 3 waits; 2 is
        // selected; 1 is only new.
        final g = select(
          threads,
          selected: threadId(2),
          runs: [
            run(3, RunState.waitingInput, threadN: 3),
            run(4, RunState.running, threadN: 4),
          ],
          counts: {threadId(5): c(1, 1), threadId(6): c(3)},
          l: const SidebarThreadLimits(
            window: Duration(hours: 72),
            perChannel: 5,
          ),
        );
        expect(ids(g).toSet(), {
          threadId(2),
          threadId(3),
          threadId(4),
          threadId(5),
          threadId(6),
        });
      },
    );

    test('at the bound the newest activity wins among equals', () {
      final g = select([
        post(1, created: 1, ageHours: 5),
        post(2, created: 2, ageHours: 1),
        post(3, created: 3, ageHours: 3),
        post(4, created: 4, ageHours: 2),
      ]);
      expect(ids(g).toSet(), {threadId(2), threadId(3), threadId(4)});
    });
  });

  group('order and kept rows (01 6.8 item 4)', () {
    test('rows are ordered by creation, newest first, never by activity', () {
      final g = select([
        post(1, created: 30, ageHours: 9),
        post(2, created: 10, ageHours: 1),
        post(3, created: 20, ageHours: 5),
      ]);
      expect(ids(g), [threadId(1), threadId(3), threadId(2)]);
    });

    test('a kept row stays in place although it no longer qualifies', () {
      final g = select(
        [post(1, created: 1, ageHours: 500), post(2, created: 2, ageHours: 1)],
        keep: {threadId(1)},
      );
      expect(ids(g), [threadId(2), threadId(1)]);
    });

    test('a kept row that is no longer stored goes', () {
      final g = select([post(2, created: 2)], keep: {threadId(1)});
      expect(ids(g), [threadId(2)]);
    });
  });

  group('counters (01 6.8 items 5 and 6, D-SB-4, D-SB-5)', () {
    test('an expanded forum: the rows show their own, All posts the rest', () {
      final g = select(
        [post(1, created: 1), post(2, created: 2, ageHours: 500)],
        counts: {
          threadId(1): c(2, 1),
          // Thread 2 is unread, so it shows; thread 9 is not stored.
          threadId(2): c(1),
          threadId(9): c(5, 2),
        },
      );
      expect(ids(g), [threadId(2), threadId(1)]);
      expect(g.allPostsCounts, c(5, 2));
      // No number twice on screen: the channel row of an expanded forum carries nothing that
      // a row or All posts below it shows.
      expect(g.channelCounts, ThreadCounts.zero);
      expect(g.rows.first.counts, c(1));
    });

    test('a collapsed group counts every thread on the channel row', () {
      final g = select(
        [post(1)],
        counts: {threadId(1): c(2, 1), threadId(9): c(5, 2)},
        expanded: false,
      );
      expect(g.channelCounts, c(7, 3));
      expect(g.allPostsCounts, isNull);
    });

    test('an expanded text channel counts its main thread and the threads not shown', () {
      final g = select(
        [post(7, channelN: 2)],
        on: text,
        counts: {
          threadId(7): c(1, 1),
          text.mainThreadId!: c(4),
          threadId(8): c(2, 1),
        },
      );
      expect(ids(g), [threadId(7)]);
      expect(g.channelCounts, c(6, 1));
      expect(
        g.allPostsCounts,
        isNull,
        reason: 'text channels have no All posts',
      );
    });

    test(
      'a forum without rows has no group and counts everything on its row',
      () {
        final g = select(
          [post(1, ageHours: 500)],
          counts: {threadId(9): c(3, 1)},
        );
        expect(g.hasRows, isFalse);
        expect(g.channelCounts, c(3, 1));
        expect(g.allPostsCounts, isNull);
      },
    );
  });

  group('markers (01 6.8 item 6)', () {
    test('the run state icon: running, waiting, failed of the last run', () {
      final g = select(
        [post(1, created: 1), post(2, created: 2), post(3, created: 3)],
        runs: [
          run(1, RunState.queued, threadN: 1),
          run(2, RunState.waitingInput, threadN: 2),
          run(3, RunState.failed, threadN: 3),
        ],
      );
      final states = {for (final r in g.rows) r.thread.id: r.rowData.runState};
      expect(states[threadId(1)], DsRunState.queued);
      expect(states[threadId(2)], DsRunState.waitingInput);
      expect(states[threadId(3)], DsRunState.failed);
    });

    test('a finished run shows no icon', () {
      final g = select([post(1)], runs: [run(1, RunState.completed)]);
      expect(g.rows.single.rowData.runState, isNull);
    });

    test('row data: title, counters, muted, done', () {
      final g = select(
        [post(1, title: '', state: ThreadState.closed)],
        selected: threadId(1),
        counts: {threadId(1): c(3, 2)},
        muted: {threadId(1)},
      );
      final d = g.rows.single.rowData;
      expect(d.id, threadId(1).value);
      expect(d.title, '');
      expect(d.unreadCount, 3);
      expect(d.mentionCount, 2);
      expect(d.muted, isTrue);
      expect(d.done, isTrue);
    });
  });

  group('properties (brief M4-40 Design note 1)', () {
    for (var seed = 0; seed < 200; seed++) {
      test('seed $seed', () {
        final random = Random(seed);
        final n = random.nextInt(14);
        final threads = [
          for (var i = 1; i <= n; i++)
            post(
              i,
              created: random.nextInt(1000),
              ageHours: random.nextInt(200),
              state: random.nextInt(5) == 0
                  ? ThreadState.closed
                  : ThreadState.open,
            ),
        ];
        final counts = <ThreadId, ThreadCounts>{
          for (final t in threads)
            if (random.nextBool())
              t.id: c(random.nextInt(4), random.nextInt(2)),
        };
        // Active and ended runs: queued, running, waiting, completed, failed.
        final runs = [
          for (var i = 1; i <= n; i++)
            if (random.nextInt(3) == 0)
              run(
                i,
                RunState.values[random.nextInt(5)],
                threadN: i,
                triggeredBy: random.nextBool() ? alex : sam,
              ),
        ];
        final selected = n > 0 && random.nextBool()
            ? threads[random.nextInt(n)].id
            : null;
        final bound = 1 + random.nextInt(4);
        final l = SidebarThreadLimits(
          window: const Duration(hours: 72),
          perChannel: bound,
        );
        final g = select(
          threads,
          counts: counts,
          runs: runs,
          selected: selected,
          l: l,
        );
        final shown = ids(g).toSet();
        expect(shown.length, lessThanOrEqualTo(bound), reason: 'seed $seed');
        if (selected != null) {
          expect(shown, contains(selected), reason: 'seed $seed');
        }
        // Never a waiting thread dropped while a lower one shows.
        final waiting = {
          for (final r in g.candidates)
            if (r.waitingForMe) r.thread.id,
        };
        final lower = {
          for (final r in g.candidates)
            if (!r.waitingForMe && r.thread.id != selected) r.thread.id,
        };
        if (waiting.difference(shown).isNotEmpty) {
          expect(
            shown.intersection(lower),
            isEmpty,
            reason: 'seed $seed: a waiting thread was dropped',
          );
        }
        // Order by creation, newest first.
        final created = [for (final r in g.rows) r.thread.createdAt];
        for (var i = 1; i < created.length; i++) {
          expect(
            created[i - 1].isBefore(created[i]),
            isFalse,
            reason: 'seed $seed',
          );
        }
        // Counters: the channel row and All posts plus the rows add up to the total.
        final total = counts.values.fold(
          ThreadCounts.zero,
          (a, b) => c(a.unread + b.unread, a.mentions + b.mentions),
        );
        final rows = g.rows.fold(
          ThreadCounts.zero,
          (a, r) =>
              c(a.unread + r.counts.unread, a.mentions + r.counts.mentions),
        );
        final rest = g.allPostsCounts ?? ThreadCounts.zero;
        expect(
          c(
            g.channelCounts.unread + rows.unread + rest.unread,
            g.channelCounts.mentions + rows.mentions + rest.mentions,
          ),
          total,
          reason: 'seed $seed',
        );
      });
    }
  });
}
