// SPDX-License-Identifier: Apache-2.0

// The goldens of the draft row "New chat" in the sidebar (brief M4-41 Tests; components.md 7;
// 01 6.6, 6.8 item 8): a project forum whose group starts with the selected draft row, a sent
// post with "Sending" and a refused one with "Not sent" above its thread rows and "All posts";
// a forum without a project binding with a draft row of its own; a collapsed forum whose draft
// row does not show. Light and dark, densities comfortable and compact, text scaling 1 and 2,
// German and English. One file per theme keeps each file below repo.max_file_kb.

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
const Size column = Size(280, 860);

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

const SidebarThreadLimits limits = SidebarThreadLimits(
  window: Duration(hours: 72),
  perChannel: 8,
);

/// The outbox entry of the sent post that waits.
final String sending = uuid(0x801);

/// The outbox entry of the refused post.
final String refused = uuid(0x802);

/// [c] with the thread group [threads], the new chat rows [newChat] and its collapse state.
SidebarChannel grouped(
  Channel c,
  List<Thread> threads, {
  List<SidebarNewChatRow> newChat = const [],
  Map<ThreadId, ThreadCounts> counts = const {},
  bool expanded = true,
}) {
  final group = selectSidebarThreads(
    channel: c,
    threads: threads,
    counts: counts,
    runs: const [],
    selected: null,
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
    runActive: false,
    muted: false,
    group: group,
    expanded: expanded,
    newChatRows: newChat,
  );
}

/// The project forum `server` (channel 9): the draft row, a sent and a refused post above two
/// thread rows.
SidebarChannel server() => grouped(
  channel(
    9,
    name: 'server',
    kind: ChannelKind.forum,
  ).copyWith(projectBound: true),
  [
    thread(
      1,
      channelN: 9,
      title: 'Gateway: resume after network change',
      activeMinutes: 30,
      createdMinutes: 20,
    ),
    thread(
      2,
      channelN: 9,
      title: 'Check migration 0012',
      activeMinutes: 20,
      createdMinutes: 10,
    ),
  ],
  newChat: [
    const SidebarNewChatRow.draft(),
    SidebarNewChatRow.sent(entryId: sending, title: ''),
    SidebarNewChatRow.sent(
      entryId: refused,
      title: 'Upload limit for the release builds',
      failed: true,
    ),
  ],
  counts: {threadId(2): const ThreadCounts(unread: 2, mentions: 1)},
);

/// The forum `ideas` (channel 4) without a project binding: only a draft row.
SidebarChannel ideas() => grouped(
  channel(4, name: 'ideas', kind: ChannelKind.forum),
  const [],
  newChat: const [SidebarNewChatRow.draft()],
);

/// The project forum `apps` (channel 10), collapsed: its draft row does not show.
SidebarChannel apps() => grouped(
  channel(
    10,
    name: 'apps',
    kind: ChannelKind.forum,
  ).copyWith(projectBound: true),
  [thread(11, channelN: 10, title: 'Release 1.2', activeMinutes: 10)],
  newChat: const [SidebarNewChatRow.draft()],
  expanded: false,
);

List<SidebarSpaceNode> nodes() => [
  SidebarSpaceNode(
    entry: SidebarSpace(
      space: space(1, name: 'Product', color: 'spruce'),
    ),
    channels: [
      SidebarChannel(
        channel: channel(1, name: 'general'),
        counts: ThreadCounts.zero,
        runActive: false,
        muted: false,
      ),
      ideas(),
      server(),
      apps(),
    ],
  ),
];

void main() {
  for (final theme in DsThemeMode.values) {
    goldenTest(
      'the draft row in ${theme.name}: draft, sending and not sent, two densities, '
      'text scale 1 and 2, de and en',
      fileName: 'draft_row_${theme.name}',
      builder: () => dsGoldenMatrix(
        name: 'draft row',
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
                  selectedNewChat: SidebarNewChatSelection(channelId(9)),
                  onOpenNewChat: (_, _) {},
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
