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
class ThreadMenu extends ConsumerWidget {
  /// Creates the menu.
  const ThreadMenu({
    super.key,
    required this.thread,
    required this.title,
    required this.channel,
    required this.child,
  });

  /// The thread.
  final ThreadId thread;

  /// Its title, for the screen reader label.
  final String title;

  /// Its channel.
  final Channel channel;

  /// The row the menu belongs to.
  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.watch(levelContextProvider);
    return TargetMenuRegion(
      label: AppLocalizations.of(context).notifyMenuOf(title),
      menuChildren: (context) =>
          levelMenuItems(context, ref, threadMenuTarget(ref, thread, channel)),
      child: child,
    );
  }
}

/// The button "More" of the thread header: opens the menu of [thread] in [channel], or the
/// channel's menu for the main history ([thread] null).
class TargetMenuButton extends ConsumerWidget {
  /// Creates the button.
  const TargetMenuButton({super.key, required this.channel, this.thread});

  /// The channel of the history.
  final Channel channel;

  /// The thread, or null for the main history of a text channel.
  final ThreadId? thread;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    ref.watch(levelContextProvider);
    final id = thread;
    return MenuAnchor(
      menuChildren: id == null
          ? channelMenuItems(context, ref, channel)
          : levelMenuItems(context, ref, threadMenuTarget(ref, id, channel)),
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
