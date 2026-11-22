// SPDX-License-Identifier: Apache-2.0

/// The pin entries of the menus of a channel and a forum post (01 6.24, FEAT-PIN-CHANNELS):
/// "Pin to top" pins the channel to the top of its space or the post to the top of its forum in
/// the person's own sidebar, "Unpin" returns it to its place. The entries stand in the sidebar
/// menu (right click, the menu key and Shift+F10 on macOS and Windows, a long press on iOS and
/// Android) and in the More menu of the header. A pin is personal and follows the person to
/// every device through the outbox (`putMySidebarChannel`).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/session.dart';
import '../sidebar/providers.dart'
    show
        pinnedChannelsProvider,
        storedThreadProvider,
        userChannelStatesProvider;
import '../sync/sidebar_ops.dart';

/// The writer of the personal sidebar of [container], or null while signed out.
SidebarWriter? _writer(ProviderContainer container) {
  final engine = container.read(syncEngineProvider);
  final db = container.read(localDatabaseProvider);
  return engine == null || db == null ? null : SidebarWriter(engine, db);
}

/// The pin entry of [channel]: "Unpin" while the person pinned it, else "Pin to top"; none for a
/// direct message or a channel archived for everyone, which are no rows of a space.
List<Widget> channelPinMenuItems(
  BuildContext context,
  WidgetRef ref,
  Channel channel,
) {
  if (channel.kind == ChannelKind.dm || channel.archivedAt != null) {
    return const [];
  }
  final pinned = ref.watch(pinnedChannelsProvider).contains(channel.id);
  // Taken now: an entry runs after the menu closed and its ref is gone (D-TEAM-APP-6).
  final container = ProviderScope.containerOf(context, listen: false);
  final l = AppLocalizations.of(context);
  return [
    DsMenuItem(
      key: ValueKey(pinned ? 'menu.unpin' : 'menu.pin'),
      onPressed: () =>
          unawaited(_writer(container)?.setPinned(channel.id, pinned: !pinned)),
      child: Text(pinned ? l.sidebarUnpin : l.sidebarPin),
    ),
  ];
}

/// The pin entry of the post [thread] of the forum [channel]: "Unpin" while the person pinned
/// it, else "Pin to top" for an open post; none in a text channel, for the main thread and for
/// a done post that is not pinned (the sidebar shows no done post).
List<Widget> postPinMenuItems(
  BuildContext context,
  WidgetRef ref,
  ThreadId thread,
  Channel channel,
) {
  if (channel.kind != ChannelKind.forum ||
      channel.archivedAt != null ||
      thread == channel.mainThreadId) {
    return const [];
  }
  final pinned =
      ref
          .watch(userChannelStatesProvider)
          .value?[channel.id]
          ?.pinnedThreads
          .contains(thread) ??
      false;
  final stored = ref.watch(storedThreadProvider(thread.value)).value;
  if (!pinned && stored?.state == ThreadState.closed) return const [];
  final container = ProviderScope.containerOf(context, listen: false);
  final l = AppLocalizations.of(context);
  return [
    DsMenuItem(
      key: ValueKey(pinned ? 'menu.unpin' : 'menu.pin'),
      onPressed: () =>
          unawaited(_writer(container)?.togglePinnedThread(channel.id, thread)),
      child: Text(pinned ? l.sidebarUnpin : l.sidebarPin),
    ),
  ];
}
