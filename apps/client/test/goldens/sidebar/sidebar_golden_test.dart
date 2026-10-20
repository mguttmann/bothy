// SPDX-License-Identifier: Apache-2.0

// The goldens of the sidebar (brief M4-19 Tests; components.md 7): three spaces with every
// channel state (read, unread, mentions, active run, muted, selected; a collapsed space with
// unread items), light and dark, densities comfortable and compact, text scaling 1 and 2,
// German and English. One file per theme keeps each file below repo.max_file_kb.

import 'package:alchemist/alchemist.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/sidebar/sidebar_model.dart';
import 'package:client/sidebar/sidebar_order.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';

import '../../support/structure.dart';

/// The size of the sidebar column in a cell.
const Size column = Size(280, 720);

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

UserSpaceState state(int n, {bool collapsed = false}) => UserSpaceState(
  spaceId: spaceId(n),
  collapsed: collapsed,
  version: const ResourceVersion(1),
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

List<SidebarSpaceNode> nodes() => [
  SidebarSpaceNode(
    entry: SidebarSpace(
      space: space(1, name: 'Product', color: 'spruce'),
    ),
    channels: [
      entry(channel(1, name: 'general')),
      entry(channel(2, name: 'releases'), unread: 4),
      entry(channel(3, name: 'support'), unread: 2, mentions: 1),
      entry(channel(4, name: 'ideas', kind: ChannelKind.forum), run: true),
      entry(channel(5, name: 'random'), unread: 7, muted: true),
      entry(channel(6, name: 'design')),
    ],
  ),
  SidebarSpaceNode(
    entry: SidebarSpace(
      space: space(2, name: 'Operations', color: 'loch'),
      state: state(2, collapsed: true),
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
                  selectedChannel: channelId(6),
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
