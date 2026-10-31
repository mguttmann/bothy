// SPDX-License-Identifier: Apache-2.0

/// The menu of a channel in the sidebar and the thread header of its main history (brief M4-33
/// Scope): its notification level, "Mute until" and "Export history", which appears only where
/// the server gives the capability `export` (07 2.4; fixed requirement 9); in the sidebar a
/// forum's menu starts with "New chat in" or "New post in" (01 6.8 item 8, brief M4-41), and a
/// forum with a project binding has "Rename project" for a person with `manage` (01 6.11,
/// M4-44).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../export/channel_export.dart';
import '../new_project/rename_project_dialog.dart';
import '../notifications_settings/providers.dart';
import '../sidebar/sidebar_drag.dart' show SidebarMoves, sidebarMoveMenuItems;
import '../thread/providers.dart';
import 'level_menu.dart';

/// The menu of [channel] around [child].
class ChannelMenu extends StatelessWidget {
  /// Creates the menu of [channel].
  const ChannelMenu({
    super.key,
    required this.channel,
    required this.child,
    this.newChatLabel,
    this.onNewChat,
    this.moves,
  });

  /// "Move up" and "Move down" of the channel in the sidebar (01 6.16, M4-51), or null for none.
  final SidebarMoves? moves;

  /// The channel.
  final Channel channel;

  /// The label of the new chat entry ("New chat in `forum`" or "New post in `forum`"), or null
  /// for none.
  final String? newChatLabel;

  /// Starts a new chat in the forum (the "+" of its row), or null for no entry.
  final VoidCallback? onNewChat;

  /// The row the menu belongs to.
  final Widget child;

  @override
  Widget build(BuildContext context) => TargetMenuRegion(
    label: AppLocalizations.of(context).notifyMenuOf(channel.name),
    menuChildren: (context, ref) {
      // Watched, so the open menu follows a capability the server reports later.
      final access = ref.watch(channelAccessProvider(channel.id.value)).value;
      final label = newChatLabel;
      final start = onNewChat;
      final rename =
          channel.kind == ChannelKind.forum &&
          channel.projectBound &&
          (access?.capabilities.contains(Capability.manage) ?? false);
      return [
        if (label != null && start != null)
          DsMenuItem(
            key: const ValueKey('menu.newChat'),
            onPressed: start,
            child: Text(dsIsolate(label)),
          ),
        if (rename)
          DsMenuItem(
            key: const ValueKey('menu.renameProject'),
            onPressed: () =>
                unawaited(showRenameProjectDialog(context, ref, channel)),
            child: Text(AppLocalizations.of(context).channelMenuRenameProject),
          ),
        ...channelMenuItems(context, ref, channel),
        if (moves case final moves?) ...[
          const Divider(),
          ...sidebarMoveMenuItems(context, moves),
        ],
      ];
    },
    child: child,
  );
}

/// The entries of the menu of [channel]: the level submenus and, with the capability
/// `export` in the channel view, "Export history".
List<Widget> channelMenuItems(
  BuildContext context,
  WidgetRef ref,
  Channel channel,
) {
  final levels = ref.read(levelContextProvider);
  final access = ref.read(channelAccessProvider(channel.id.value)).value;
  final export = access?.capabilities.contains(Capability.export) ?? false;
  return [
    ...levelMenuItems(
      context,
      ref,
      LevelMenuTarget(
        kind: NotificationTargetKind.channel,
        id: channel.id.value,
        own: levels.own(NotificationTargetKind.channel, channel.id.value),
        inherited: levels.inherited(
          NotificationTargetKind.channel,
          channel: channel,
        ),
        now: levels.now,
      ),
    ),
    if (export)
      DsMenuItem(
        key: const ValueKey('menu.export'),
        onPressed: () => unawaited(showChannelExport(context, channel)),
        child: Text(AppLocalizations.of(context).channelMenuExport),
      ),
  ];
}
