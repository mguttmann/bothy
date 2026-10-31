// SPDX-License-Identifier: Apache-2.0

// The status dots of the sidebar over the local database (brief M4-52; 01 6.17): each state on a
// chat row instead of the unread bar, a chat without runs keeping the bar, the mark leaving when
// the chat opens and staying away after it closes, a new change bringing it back, a read on
// another device clearing it, the most urgent state on the forum row and the setting that turns
// it off, the main history of a text channel on its own row, and the history column writing the
// seen mark. Test data: Alex, Sam, Robin.

import 'package:client/agents/providers.dart';
import 'package:client/sidebar/chat_seen_marker.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

/// The sidebar, and next to it the history of the chat [open] names while it names one (a
/// stand-in for the history column: it writes the same seen mark).
Widget screen(ValueNotifier<ThreadId?> open) => Row(
  crossAxisAlignment: CrossAxisAlignment.start,
  children: [
    const SizedBox(width: 280, child: SidebarColumn()),
    Expanded(
      child: ValueListenableBuilder<ThreadId?>(
        valueListenable: open,
        builder: (_, id, _) => id == null
            ? const SizedBox.shrink()
            : ChatSeenMarker(
                key: ValueKey(id),
                threadId: id,
                child: const SizedBox.shrink(),
              ),
      ),
    ),
  ],
);

Finder postRow(int n) =>
    find.byKey(ValueKey('sidebar.thread.${threadId(n).value}'));
Finder channelRow(int n) =>
    find.byKey(ValueKey('sidebar.channel.${channelId(n).value}'));

/// The status the mark of [row] shows, or null without a mark.
DsChatStatus? markOf(WidgetTester tester, Finder row) {
  final mark = find.descendant(
    of: row,
    matching: find.byType(DsChatStatusMark),
  );
  if (mark.evaluate().isEmpty) return null;
  return tester.widget<DsChatStatusMark>(mark).status;
}

bool barOf(WidgetTester tester, Finder row) =>
    tester.state<DsSidebarThreadRowState>(row).unreadBar;

/// Run [n] of post [threadN] (forum 1) in [state] with version [version]; an ended run finished
/// two minutes after it was queued.
RunSummary runOf(
  int n,
  RunState state, {
  int threadN = 1,
  int channelN = 1,
  int version = 1,
}) {
  final r = run(n, state, threadN: threadN, channelN: channelN);
  final ended = {
    RunState.completed,
    RunState.failed,
    RunState.cancelled,
  }.contains(state);
  return r.copyWith(
    version: ResourceVersion(version),
    startedAt: r.queuedAt,
    finishedAt: ended ? r.queuedAt.add(const Duration(minutes: 2)) : null,
  );
}

/// The read state of post [threadN] with [unread] messages, written at [written] when given.
ReadState readOf(
  int threadN, {
  int channelN = 1,
  int unread = 1,
  int version = 1,
  DateTime? written,
}) => ReadState(
  threadId: threadId(threadN),
  channelId: channelId(channelN),
  lastReadSeq: const ThreadSeq(0),
  mentionCount: 0,
  unreadCount: unread,
  version: ResourceVersion(version),
  updatedAt: written,
);

/// Space 1 with forum 1 (posts 1 to 5) and text channel 2 with its main history; every post
/// has one unread message.
Future<void> fill(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertChannelIfNewer(
    channel(1, kind: ChannelKind.forum, name: 'server'),
  );
  await db.structureDao.upsertChannelIfNewer(channel(2, name: 'general'));
  for (final n in [1, 2, 3, 4, 5]) {
    await db.threadsDao.upsertIfNewer(thread(n, activeMinutes: n, lastSeq: 1));
    await db.personalDao.applyReadState(readOf(n));
  }
}

Future<void> putRuns(LocalDatabase db, List<RunSummary> runs) async {
  for (final r in runs) {
    await db.runsDao.upsertIfNewer(r);
  }
}

void main() {
  for (final platform in ['macos', 'windows', 'ios', 'android']) {
    testWidgets('$platform: each state shows its mark on its chat row instead '
        'of the unread bar; a chat without runs keeps the bar', (tester) async {
      final host = await ColumnHost.open(tester);
      await host.run(tester, () async {
        await fill(host.db);
        await putRuns(host.db, [
          runOf(1, RunState.completed, threadN: 1),
          runOf(2, RunState.running, threadN: 2),
          runOf(3, RunState.waitingInput, threadN: 3),
          runOf(4, RunState.failed, threadN: 4),
        ]);
      });
      await host.pump(tester, screen(ValueNotifier(null)), platform: platform);
      expect(markOf(tester, postRow(1)), DsChatStatus.done);
      expect(markOf(tester, postRow(2)), DsChatStatus.working);
      expect(markOf(tester, postRow(3)), DsChatStatus.waiting);
      expect(markOf(tester, postRow(4)), DsChatStatus.failed);
      for (final n in [1, 2, 3, 4]) {
        expect(barOf(tester, postRow(n)), isFalse, reason: 'post $n');
      }
      expect(markOf(tester, postRow(5)), isNull);
      expect(barOf(tester, postRow(5)), isTrue);
      await host.dispose(tester);
    });
  }

  testWidgets('the mark leaves when the chat opens, whatever its colour, stays '
      'away after it closes and comes back with a new change', (tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () async {
      await fill(host.db);
      await putRuns(host.db, [
        runOf(1, RunState.running, threadN: 1),
        runOf(4, RunState.failed, threadN: 4),
      ]);
    });
    final open = ValueNotifier<ThreadId?>(null);
    await host.pump(tester, screen(open));
    expect(markOf(tester, postRow(1)), DsChatStatus.working);
    expect(markOf(tester, postRow(4)), DsChatStatus.failed);

    for (final n in [1, 4]) {
      open.value = threadId(n);
      await host.settle(tester);
      expect(markOf(tester, postRow(n)), isNull, reason: 'post $n open');
    }
    open.value = null;
    await host.settle(tester);
    expect(markOf(tester, postRow(1)), isNull, reason: 'seen stays seen');
    expect(markOf(tester, postRow(4)), isNull, reason: 'seen stays seen');

    // The run the person watched finishes while they are elsewhere: green.
    await host.run(
      tester,
      () => putRuns(host.db, [
        runOf(1, RunState.completed, threadN: 1, version: 2),
      ]),
    );
    await host.settle(tester);
    expect(markOf(tester, postRow(1)), DsChatStatus.done);
    // A new run of the failed chat asks something: blue.
    await host.run(
      tester,
      () => putRuns(host.db, [runOf(9, RunState.waitingInput, threadN: 4)]),
    );
    await host.settle(tester);
    expect(markOf(tester, postRow(4)), DsChatStatus.waiting);
    await host.dispose(tester);
  });

  testWidgets('a read on another device after the change clears the mark', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    final done = runOf(1, RunState.completed, threadN: 1);
    await host.run(tester, () async {
      await fill(host.db);
      await putRuns(host.db, [done]);
    });
    await host.pump(tester, screen(ValueNotifier(null)));
    expect(markOf(tester, postRow(1)), DsChatStatus.done);
    await host.run(
      tester,
      () => host.db.personalDao.applyReadState(
        readOf(
          1,
          unread: 0,
          version: 2,
          written: done.finishedAt!.add(const Duration(seconds: 3)),
        ),
      ),
    );
    await host.settle(tester);
    expect(markOf(tester, postRow(1)), isNull);
    await host.dispose(tester);
  });

  testWidgets('the forum row shows the most urgent unseen state of its chats; '
      'the setting turns it off, the main history keeps its own', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () async {
      await fill(host.db);
      await putRuns(host.db, [
        runOf(1, RunState.completed, threadN: 1),
        runOf(2, RunState.running, threadN: 2),
        runOf(3, RunState.waitingInput, threadN: 3),
        // The main history of the text channel waits for the person.
        runOf(5, RunState.waitingInput, threadN: 0x82, channelN: 2),
      ]);
      await host.db.personalDao.applyReadState(readOf(0x82, channelN: 2));
    });
    final open = ValueNotifier<ThreadId?>(null);
    await host.pump(tester, screen(open));
    expect(markOf(tester, channelRow(1)), DsChatStatus.waiting);
    expect(markOf(tester, channelRow(2)), DsChatStatus.waiting);

    await host.run(
      tester,
      () => putRuns(host.db, [runOf(4, RunState.failed, threadN: 4)]),
    );
    await host.settle(tester);
    expect(markOf(tester, channelRow(1)), DsChatStatus.failed, reason: 'red');

    // Seen chats leave the aggregate.
    open.value = threadId(4);
    await host.settle(tester);
    open.value = threadId(3);
    await host.settle(tester);
    expect(markOf(tester, channelRow(1)), DsChatStatus.working);

    await host.run(
      tester,
      () => host.db.personalDao.upsertSettingIfNewer(
        const UserSetting(
          key: 'sidebar.show_project_status',
          value: false,
          version: ResourceVersion(1),
        ),
      ),
    );
    await host.settle(tester);
    expect(markOf(tester, channelRow(1)), isNull);
    expect(markOf(tester, postRow(2)), DsChatStatus.working, reason: 'chats');
    expect(markOf(tester, channelRow(2)), DsChatStatus.waiting, reason: 'own');
    await host.dispose(tester);
  });

  testWidgets('the history column writes the seen mark of its chat', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    final failed = runOf(1, RunState.failed, threadN: 1);
    await host.run(tester, () async {
      await seedThread(host.db);
      await putRuns(host.db, [failed]);
    });
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        runSourceProvider.overrideWithValue(StoreRunSource()),
        agentReadsProvider.overrideWithValue(FakeAgentReads()),
      ],
    );
    final marks = await host.run(
      tester,
      () => host.db.forumViewDao.watchSeenStates().first,
    );
    expect(
      marks[threadId(1)],
      ThreadSeenState(
        threadId: threadId(1),
        runId: failed.id,
        runState: RunState.failed,
      ),
    );
    await host.dispose(tester);
  });
}
