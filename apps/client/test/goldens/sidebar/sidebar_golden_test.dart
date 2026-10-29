// SPDX-License-Identifier: Apache-2.0

// The goldens of the sidebar (briefs M4-19 and M4-40 Tests; components.md 7; 01 6.8): three
// spaces with every channel state (read, unread, mentions, active run, muted, selected; a
// collapsed space with unread items), a text channel with a thread row, a project forum with an
// expanded group of thread rows in every state (running, waiting for me, failed, mention,
// unread, an untitled "New chat", selected) and "All posts", a collapsed project forum group;
// light and dark, densities comfortable and compact, text scaling 1 and 2, German and English.
// One file per theme keeps each file below repo.max_file_kb.

import 'package:alchemist/alchemist.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/sidebar/sidebar_model.dart';
import 'package:client/sidebar/sidebar_order.dart';
import 'package:client/sidebar/sidebar_threads.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';

import '../../support/structure.dart';

/// The size of the sidebar column in a cell.
const Size column = Size(280, 1100);

/// The width of a cell: wider than the longest scenario label of alchemist, so the matrix has
/// a whole number of pixels on every host (docs/DECISIONS.md D-M4-07-10).
const double cellWidth = 460;

/// [child] at the top left of a cell of [cellWidth].
Widget cell(Widget child) => SizedBox(
  width: cellWidth,
  child: Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    mainAxisSize: MainAxisSize.min,
    children: [child],
  ),
);

SidebarChannel entry(
  Channel c, {
  int unread = 0,
  int mentions = 0,
  bool run = false,
  bool muted = false,
}) => SidebarChannel(
  channel: c,
  counts: ThreadCounts(unread: unread, mentions: mentions),
  runActive: run,
  muted: muted,
);

const SidebarThreadLimits limits = SidebarThreadLimits(
  window: Duration(hours: 72),
  perChannel: 8,
  forumPostsVisible: 15,
);

/// [c] with the thread group [threads] (selected by the rules of 01 6.8), its counters and its
/// collapse state.
SidebarChannel grouped(
  Channel c,
  List<Thread> threads, {
  Map<ThreadId, ThreadCounts> counts = const {},
  List<RunSummary> runs = const [],
  ThreadId? selected,
  bool expanded = true,
}) {
  final group = selectSidebarThreads(
    channel: c,
    threads: threads,
    counts: counts,
    runs: runs,
    selected: selected,
    me: alex,
    now: t0.add(const Duration(hours: 1)),
    limits: limits,
    expanded: expanded,
  );
  var total = ThreadCounts.zero;
  for (final v in counts.values) {
    total = ThreadCounts(
      unread: total.unread + v.unread,
      mentions: total.mentions + v.mentions,
    );
  }
  return SidebarChannel(
    channel: c,
    counts: total,
    runActive: runs.any((r) => r.channelId == c.id),
    muted: false,
    group: group,
    expanded: expanded,
  );
}

/// The project forum `server` (channel 9) with posts in every row state.
SidebarChannel server() {
  final c = channel(
    9,
    name: 'server',
    kind: ChannelKind.forum,
  ).copyWith(projectBound: true);
  Thread post(int n, String title, int created) => thread(
    n,
    channelN: 9,
    title: title,
    createdMinutes: created,
    activeMinutes: 30,
  );
  return grouped(
    c,
    [
      post(1, 'Gateway: resume after network change', 60),
      post(2, 'Check migration 0012', 50),
      post(3, 'Review: grant wizard', 40),
      post(4, '', 35),
      post(5, 'Upload limit', 30),
      post(6, 'Search: hits in transcripts', 20),
    ],
    selected: threadId(1),
    runs: [
      run(1, RunState.running, threadN: 1, channelN: 9),
      run(2, RunState.waitingInput, threadN: 2, channelN: 9),
      run(3, RunState.failed, threadN: 5, channelN: 9),
    ],
    counts: {
      threadId(3): const ThreadCounts(unread: 2, mentions: 1),
      threadId(6): const ThreadCounts(unread: 1, mentions: 0),
      // A post not stored here: only "All posts" counts it.
      threadId(40): const ThreadCounts(unread: 1, mentions: 1),
    },
  );
}

/// The project forum `apps` (channel 10) with a collapsed group.
SidebarChannel apps() => grouped(
  channel(
    10,
    name: 'apps',
    kind: ChannelKind.forum,
  ).copyWith(projectBound: true),
  [thread(11, channelN: 10, title: 'Release 1.2', activeMinutes: 10)],
  counts: {threadId(11): const ThreadCounts(unread: 3, mentions: 0)},
  expanded: false,
);

/// The text channel `general` with one thread row.
SidebarChannel general() => grouped(
  channel(1, name: 'general'),
  [thread(12, title: 'Export format', activeMinutes: 20)],
  counts: {threadId(12): const ThreadCounts(unread: 1, mentions: 0)},
);

List<SidebarSpaceNode> nodes() => [
  SidebarSpaceNode(
    entry: SidebarSpace(
      space: space(1, name: 'Product', color: 'spruce'),
    ),
    channels: [
      general(),
      entry(channel(2, name: 'releases'), unread: 4),
      entry(channel(3, name: 'support'), unread: 2, mentions: 1),
      entry(channel(4, name: 'ideas', kind: ChannelKind.forum), run: true),
      entry(channel(5, name: 'random'), unread: 7, muted: true),
      entry(channel(6, name: 'design')),
      server(),
      apps(),
    ],
  ),
  SidebarSpaceNode(
    entry: SidebarSpace(
      space: space(2, name: 'Operations', color: 'loch'),
      collapsed: true,
    ),
    channels: [entry(channel(7, spaceN: 2, name: 'alerts'), unread: 3)],
  ),
  SidebarSpaceNode(
    entry: SidebarSpace(
      space: space(3, name: 'Research', color: 'heather'),
    ),
    channels: [
      entry(
        channel(8, spaceN: 3, name: 'papers', kind: ChannelKind.forum),
        mentions: 2,
        unread: 2,
        run: true,
      ),
    ],
  ),
];

void main() {
  for (final theme in DsThemeMode.values) {
    goldenTest(
      'the sidebar in ${theme.name}: every channel state, two densities, '
      'text scale 1 and 2, de and en',
      fileName: 'sidebar_${theme.name}',
      builder: () => dsGoldenMatrix(
        name: 'sidebar',
        themes: [theme],
        densities: DsDensityMode.values,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
        builder: () => cell(
          Builder(
            builder: (context) => ColoredBox(
              color: context.dsColors.surfaceSidebar,
              child: SizedBox.fromSize(
                size: column,
                child: SidebarView(
                  productName: 'Team Chat',
                  nodes: nodes(),
                  ownName: 'Alex',
                  selectedChannel: channelId(9),
                  selectedThread: threadId(1),
                  onOpenThread: (_) {},
                  onToggleGroup: (_, _) {},
                  onAdd: (_) {},
                  onToggle: (_, _) {},
                  onMove: (_, _) {},
                  onOpenChannel: (_) {},
                  onSettings: () {},
                  onLogout: () {},
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
