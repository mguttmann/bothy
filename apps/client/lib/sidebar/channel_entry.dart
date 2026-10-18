// SPDX-License-Identifier: Apache-2.0

/// A channel of the sidebar tree (components.md 7): `DsChannelRow` of M4-07 below its space.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import 'sidebar_model.dart';

/// The row of [entry], indented below its space.
///
/// The row shows the states of components.md 7 (read, unread, mentions, active run, muted,
/// selected) and the label `<Name>, <n> unread, <m> mentions, run active`; a click or Enter
/// calls [onOpen] (the router path `/c/<id>`, brief M4-19 Design note 5).
class ChannelEntry extends StatelessWidget {
  /// Creates the entry.
  const ChannelEntry({
    super.key,
    required this.entry,
    required this.selected,
    required this.onOpen,
  });

  /// The channel with its counters and states.
  final SidebarChannel entry;

  /// Whether the route shows this channel or one of its threads.
  final bool selected;

  /// Opens the channel.
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsetsDirectional.only(
      start: DsSpace.s16,
      end: DsSpace.s4,
    ),
    child: DsChannelRow(
      entry.rowData,
      key: ValueKey('sidebar.channel.${entry.channel.id.value}'),
      selected: selected,
      onOpen: onOpen,
    ),
  );
}
