// SPDX-License-Identifier: Apache-2.0

/// The menu of a thread in the post list and the thread header (brief M4-33 Scope): its
/// notification level with "Following" for a thread the server follows for the person (13 3.2)
/// and "Mute until", ending with "Delete chat..." for its creator or a person with `manage`
/// (01 6.18 items 1 and 2, D-DEL-14).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../delete/delete_flow.dart';
import '../delete/delete_port.dart';
import '../notifications_settings/providers.dart';
import '../sidebar/providers.dart' show storedThreadProvider;
import '../sidebar/sidebar_drag.dart' show SidebarMoves, sidebarMoveMenuItems;
import '../thread/providers.dart'
    show myPrincipalIdProvider, threadPermissionsProvider;
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
      ...threadDeleteItems(context, ref, thread, channel, title: title),
    ],
    child: child,
  );
}

/// The end of the menu of [thread] in [channel]: a divider and "Delete chat..." for its creator
/// or a person with `manage`, both with `write` (01 6.18 item 2, D-DEL-4); none for a main
/// thread and none in an archived or read only channel. [title] names it in the dialog; a
/// blank title reads "New chat".
List<Widget> threadDeleteItems(
  BuildContext context,
  WidgetRef ref,
  ThreadId thread,
  Channel channel, {
  required String title,
}) {
  final session = DeleteSession.watch(ref);
  if (session == null ||
      thread == channel.mainThreadId ||
      channel.archivedAt != null ||
      channel.readOnly) {
    return const [];
  }
  final stored = ref.watch(storedThreadProvider(thread.value)).value;
  final permissions = ref.watch(threadPermissionsProvider(channel.id.value));
  final me = ref.watch(myPrincipalIdProvider);
  if (stored == null || stored.kind == ThreadKind.main) return const [];
  final own = me != null && stored.createdBy == me;
  if (!permissions.canWrite || !(own || permissions.canManage)) {
    return const [];
  }
  final l = AppLocalizations.of(context);
  final name = title.trim().isEmpty ? l.sidebarNewChat : title;
  return [
    const Divider(),
    DsMenuItem(
      key: const ValueKey('menu.delete'),
      onPressed: () => unawaited(
        startDelete(
          context,
          session,
          DeleteTarget(kind: DeleteKind.chat, id: thread.value, name: name),
        ),
      ),
      child: Text(l.menuDeleteChat),
    ),
  ];
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
        ? [
            ...channelMenuItems(context, ref, channel),
            ...channelDeleteItems(context, ref, channel),
          ]
        : [
            ...levelMenuItems(context, ref, threadMenuTarget(ref, id, channel)),
            ...threadDeleteItems(
              context,
              ref,
              id,
              channel,
              title:
                  ref.watch(storedThreadProvider(id.value)).value?.title ?? '',
            ),
          ];
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
