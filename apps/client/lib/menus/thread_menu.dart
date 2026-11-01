// SPDX-License-Identifier: Apache-2.0

/// The menu of a thread in the post list and the thread header (brief M4-33 Scope): its
/// notification level with "Following" for a thread the server follows for the person (13 3.2)
/// and "Mute until".
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../notifications_settings/providers.dart';
import '../sidebar/sidebar_drag.dart' show SidebarMoves, sidebarMoveMenuItems;
import '../thread/thread_header.dart' show ThreadHeaderAction, ThreadHeaderInfo;
import 'channel_menu.dart';
import 'level_menu.dart';

/// The level target of [thread] in [channel] from the current [LevelContext].
LevelMenuTarget threadMenuTarget(
  WidgetRef ref,
  ThreadId thread,
  Channel channel,
) {
  final levels = ref.read(levelContextProvider);
  return LevelMenuTarget(
    kind: NotificationTargetKind.thread,
    id: thread.value,
    own: levels.own(NotificationTargetKind.thread, thread.value),
    inherited: levels.inherited(
      NotificationTargetKind.thread,
      channel: channel,
    ),
    now: levels.now,
  );
}

/// The menu of [thread] (titled [title]) in [channel] around [child].
class ThreadMenu extends StatelessWidget {
  /// Creates the menu.
  const ThreadMenu({
    super.key,
    required this.thread,
    required this.title,
    required this.channel,
    required this.child,
    this.moves,
  });

  /// "Move up" and "Move down" of the post in the sidebar (01 6.16, M4-51), or null for none.
  final SidebarMoves? moves;

  /// The thread.
  final ThreadId thread;

  /// Its title, for the screen reader label.
  final String title;

  /// Its channel.
  final Channel channel;

  /// The row the menu belongs to.
  final Widget child;

  @override
  Widget build(BuildContext context) => TargetMenuRegion(
    label: AppLocalizations.of(context).notifyMenuOf(title),
    menuChildren: (context, ref) => [
      ...levelMenuItems(context, ref, threadMenuTarget(ref, thread, channel)),
      if (moves case final moves?) ...[
        const Divider(),
        ...sidebarMoveMenuItems(context, moves),
      ],
    ],
    child: child,
  );
}

/// The button "More" of the thread header: opens the menu of [thread] in [channel], or the
/// channel's menu for the main history ([thread] null).
///
/// With [actions] (the phone header, FIX-draft-conflict-banner) it is the header's one overflow
/// button `thread.more`: an icon whose menu lists the actions first and then the entries of the
/// thread or channel menu.
class TargetMenuButton extends ConsumerWidget {
  /// Creates the button.
  const TargetMenuButton({
    super.key,
    required this.channel,
    this.thread,
    this.actions,
    this.operator,
  });

  /// The channel of the history.
  final Channel channel;

  /// The thread, or null for the main history of a text channel.
  final ThreadId? thread;

  /// The actions of the phone header, or null for the labelled button of the wide header.
  final List<ThreadHeaderAction>? actions;

  /// The operator line at the top of the phone header's menu (brief M5-17), or null.
  final ThreadHeaderInfo? operator;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    ref.watch(levelContextProvider);
    final id = thread;
    final entries = id == null
        ? channelMenuItems(context, ref, channel)
        : levelMenuItems(context, ref, threadMenuTarget(ref, id, channel));
    final compact = actions;
    if (compact != null) {
      return MenuAnchor(
        menuChildren: [
          if (operator case final info?)
            // Text, not an action: the operator moved here from the header (M5-17).
            Padding(
              key: ValueKey(info.key),
              padding: const EdgeInsets.symmetric(
                horizontal: DsSpace.s12,
                vertical: DsSpace.s8,
              ),
              child: Text(
                info.text,
                style: dsTextStyle(DsType.caption)
                    .copyWith(color: colors.textSecondary),
              ),
            ),
          for (final a in compact)
            DsMenuItem(
              key: ValueKey(a.key),
              onPressed: a.onTap,
              child: Text(a.label),
            ),
          if (compact.isNotEmpty && entries.isNotEmpty) const Divider(),
          ...entries,
        ],
        builder: (context, menu, _) => IconButton(
          key: const ValueKey('thread.more'),
          tooltip: l.phoneMoreActions,
          onPressed: () => menu.isOpen ? menu.close() : menu.open(),
          icon: DsIcon(
            DsIcons.actionMore,
            size: DsIconSizeRole.md,
            color: colors.textSecondary,
            decorative: true,
          ),
        ),
      );
    }
    return MenuAnchor(
      menuChildren: entries,
      builder: (context, menu, _) => Tooltip(
        message: l.notifyMenuOpen,
        excludeFromSemantics: true,
        child: DsTappable(
          key: const ValueKey('thread.menu'),
          onTap: () => menu.isOpen ? menu.close() : menu.open(),
          label: l.notifyMenuOpen,
          excludeChildSemantics: true,
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: DsSpace.s6,
              vertical: DsSpace.s4,
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                DsIcon(
                  DsIcons.actionMore,
                  size: DsIconSizeRole.sm,
                  color: colors.textSecondary,
                  decorative: true,
                ),
                const SizedBox(width: DsSpace.s4),
                Text(
                  l.notifyMenuOpen,
                  style: dsTextStyle(DsType.label)
                      .copyWith(color: colors.textSecondary),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
