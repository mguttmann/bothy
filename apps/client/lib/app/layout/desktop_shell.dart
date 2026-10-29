// SPDX-License-Identifier: Apache-2.0

/// The main view of the desktop after the login: the connection banner and the column frames
/// that M4-19 to M4-25 fill (docs/spec/01-product.md 5.1, 6.6, 6.7); M4-19 fills the sidebar
/// and the post list of a forum, M4-20 the history and the frame of the session column, M4-40
/// the thread groups of the sidebar and the forum overview D15 in the content column or pinned
/// as its own column.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../forum/new_chat_view.dart';
import '../../forum/post_list_column.dart';
import '../../forum/providers.dart'
    show
        channelColumnHiddenProvider,
        overviewPinnedProvider,
        toggleChannelColumn;
import '../../keyboard/app_shortcuts.dart';
import '../../keyboard/intents.dart';
import '../../session_column/session_column.dart';
import '../../settings/settings_providers.dart';
import '../../sidebar/channel_entry.dart';
import '../../sidebar/providers.dart';
import '../../sidebar/sidebar_column.dart';
import '../../sidebar/sidebar_model.dart'
    show SidebarNavigation, SidebarNewChatSelection, SidebarTarget;
import '../../switcher/providers.dart';
import '../../switcher/quick_switcher.dart' show showQuickSwitcher;
import '../../terminal/host_terminal_page.dart';
import '../../terminal/providers.dart' show hostsListProvider;
import '../../thread/history_area.dart';
import '../../thread/providers.dart';
import '../providers/ports.dart';
import '../providers/session.dart';
import '../providers/settings.dart';
import '../router.dart';
import '../update/update_notice.dart';
import 'column_layout.dart';
import 'column_widths.dart';

/// The desktop main view for [location] (path and query of the router).
///
/// Reads only providers (R-CL-03): the connection state of the engine, the stored column
/// widths, the collapse widths of `AppConfig` and the status of the target. The banner
/// "Offline" sits at the top of the content while the engine has no connection (01 6.6),
/// "Update required" after `4007` (15 15). A location whose target is invisible shows "Not
/// found or no access" in the history column (01 6.7). The shortcut layer of M4-24
/// ([AppShortcuts]) sits above the columns with the actions of the shell: quick switcher,
/// search, Running now, mentions, settings, the channel navigation of the sidebar, the
/// session column and "Open hosts and terminal" (01 7). `/hosts/<id>` shows the terminal view
/// of the host in place of the history (01 D11, M4-36).
class DesktopShell extends ConsumerWidget {
  /// Creates the shell.
  const DesktopShell({super.key, required this.location});

  /// The location the router shows.
  final Uri location;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final config = ref.watch(appConfigProvider);
    final phase = ref.watch(connectionStateProvider).value?.phase;
    final widths = ref.watch(columnWidthsProvider);
    final parsed = parseLocation(location);
    final resolving = ref.watch(routeSelectionProvider(location.toString()));
    // Every space, channel or thread the shell shows becomes a recent target of the quick
    // switcher (01 D6); the provider records once per route while it is shown.
    ref.watch(recordVisitProvider(location.toString()));
    ref.listen(routeSelectionProvider(location.toString()), (_, next) {
      if (next.value case final value?) {
        ref.read(lastRouteSelectionProvider.notifier).keep(value);
      }
    });
    // While a new route resolves, the last selection stays (no column leaves for a frame).
    final RouteSelection selection =
        resolving.value ??
        (resolving.isLoading
            ? ref.watch(lastRouteSelectionProvider)
            : const RouteSelection());
    final forum = selection.forum;
    // D2: with a thread of a text channel open the session column is collapsed until the
    // person shows it.
    final textChannel =
        selection.kind == ChannelKind.text && selection.thread != null;
    // M4-45: whether the person hid (or, beside a text channel's thread, showed) the session
    // column on this device; the default of the layout until the database answers.
    final sessionHidden =
        ref.watch(sessionColumnHiddenProvider(textThread: textChannel)).value ??
        textChannel;
    final channelOfRoute = selection.channel == null
        ? null
        : ref
              .watch(storedChannelOfRouteProvider(selection.channel!.value))
              .value;
    // The session of the open thread, else of the main history a channel route shows.
    final sessionThread = selection.thread ?? channelOfRoute?.mainThreadId;
    final banner = switch (phase) {
      ConnectionPhase.updateRequired => const UpdateRequiredBanner(),
      ConnectionPhase.offline ||
      ConnectionPhase.backoff ||
      ConnectionPhase.suspended => DsBanner(
        key: const ValueKey('shell.offline'),
        tone: DsTone.info,
        text: l.connectionOffline,
        liveRegion: true,
      ),
      _ => null,
    };
    // M4-40: the navigation stops at thread rows too (01 7, D-SB-8).
    final nav = SidebarNavigation(
      tree: () => ref.read(sidebarTreeProvider),
      selected: () => selection.channel == null
          ? null
          : SidebarTarget(selection.channel!, selection.thread),
      open: (target) => context.go(target.path),
    );
    final pinned = ref.watch(overviewPinnedProvider).value ?? false;
    // The cross-cutting views, the run graph and the terminal of a host have no session
    // column.
    final host = parsed is HostLocation ? parsed : null;
    final newChat = parsed is NewChatLocation ? parsed : null;
    final isView =
        parsed is ViewLocation || parsed is RunGraphLocation || host != null;
    final hasSession = !isView && sessionThread != null;
    // M4-42 (01 D1, D2): an open thread has a channel column beside it, the main history of a
    // text channel or the pinned overview of a forum, which the person can hide.
    final channelHidden = ref.watch(channelColumnHiddenProvider).value ?? false;
    final thread = parsed is ThreadLocation;
    final hasChannelColumn =
        thread && (textChannel || (forum != null && host == null && pinned));
    return AppShortcuts(
      handlers: {
        ShortcutAction.quickSwitcher: KeyboardHandler(
          () => unawaited(showQuickSwitcher(context)),
        ),
        ShortcutAction.search: KeyboardHandler(() => context.go('/search')),
        ShortcutAction.runningNow: KeyboardHandler(
          () => context.go('/running'),
        ),
        ShortcutAction.mentions: KeyboardHandler(() => context.go('/mentions')),
        ShortcutAction.settings: KeyboardHandler(() => context.go('/settings')),
        // 01 7: the first host the person sees, or the hint to choose one (M4-36).
        ShortcutAction.hostsTerminal: KeyboardHandler(() {
          final first = ref.read(hostsListProvider).value?.hosts.firstOrNull;
          context.go(first == null ? '/hosts' : '/hosts/${first.id}');
        }),
        // 01 7: Cmd+Shift+O or Ctrl+Shift+O pins or unpins the forum overview (D15); the choice
        // is kept on this device (D-SB-6).
        ShortcutAction.pinOverview: KeyboardHandler(
          () => unawaited(
            ref
                .read(localDatabaseProvider)
                ?.forumViewDao
                .setOverviewPinned(pinned: !pinned),
          ),
        ),
        // 01 7, 6.8 item 8: Cmd+N or Ctrl+N starts a new chat in the forum of the focused
        // sidebar row, else of the selected one; on a text channel it does nothing (M4-41).
        ShortcutAction.newPost: KeyboardHandler(() {
          final forum = newChatForum(
            WidgetsBinding.instance.focusManager.primaryFocus?.context,
            selected: selection.forum,
          );
          if (forum != null) openNewChat(context, ref, forum);
        }),
        ShortcutAction.nextChannel: KeyboardHandler(nav.next),
        ShortcutAction.previousChannel: KeyboardHandler(nav.previous),
        ShortcutAction.nextUnreadChannel: KeyboardHandler(nav.nextUnread),
        ShortcutAction.previousUnreadChannel: KeyboardHandler(
          nav.previousUnread,
        ),
        // D1, D2: the session column of the open thread or main history; the views have
        // none (the same rule as `showSession` below).
        ShortcutAction.toggleSessionColumn: KeyboardHandler(
          () => unawaited(
            toggleSessionColumn(
              ref.read(localDatabaseProvider),
              textThread: textChannel,
            ),
          ),
          enabled: hasSession,
        ),
        // 01 7: Cmd+Shift+H or Ctrl+Shift+H hides or shows the channel next to an open thread;
        // the choice is kept on this device (M4-42).
        ShortcutAction.toggleChannelColumn: KeyboardHandler(
          () => unawaited(toggleChannelColumn(ref.read(localDatabaseProvider))),
          enabled: hasChannelColumn,
        ),
      },
      child: Material(
        color: context.dsColors.surfaceCanvas,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            ?banner,
            const RecommendedUpdateBanner(),
            Expanded(
              child: ColumnLayout(
                widths: widths,
                minWidthFourColumns: config.desktopMinWidthFourColumns
                    .toDouble(),
                minWidthThreeColumns: config.desktopMinWidthThreeColumns
                    .toDouble(),
                stackShowsHistory:
                    parsed is ThreadLocation || host != null || newChat != null,
                showPostList: forum != null && host == null,
                // A hidden channel column gives the open post the overview's room (M4-42).
                listPinned: pinned && !(thread && channelHidden),
                // The cross-cutting views (search, mentions, Running now) have no session.
                showSession: !isView && !sessionHidden,
                // M4-45: the slim rail that brings a hidden session column back.
                onShowSession: hasSession
                    ? () => unawaited(
                        showSessionColumn(
                          ref.read(localDatabaseProvider),
                          textThread: textChannel,
                        ),
                      )
                    : null,
                onWidthsChanged: (next) {
                  final settings = ref.read(settingsRepositoryProvider);
                  if (settings != null) ColumnWidthsWriter(settings).save(next);
                },
                sidebar: SidebarColumn(
                  selectedChannel: host == null ? selection.channel : null,
                  selectedThread: host == null ? selection.thread : null,
                  selectedHost: host?.hostId?.value,
                  newChat: newChat == null
                      ? null
                      : SidebarNewChatSelection(
                          newChat.channelId,
                          entry: newChat.sentEntry,
                        ),
                ),
                postList: forum == null
                    ? const SizedBox.expand()
                    : PostListColumn(
                        key: ValueKey('postList.${forum.value}'),
                        channelId: forum,
                        selectedThread: selection.thread,
                      ),
                history: host != null
                    ? HostTerminalPage(hostId: host.hostId?.value)
                    : newChat != null
                    // 01 6.8 item 8: the new chat view in the content column (M4-41).
                    ? NewChatView(
                        key: ValueKey('newChat.${location.toString()}'),
                        forum: newChat.channelId,
                        sentEntry: newChat.sentEntry,
                      )
                    : HistoryArea(location: location),
                session: SessionColumn(threadId: sessionThread),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// The forum Cmd+N or Ctrl+N starts a new chat in (01 7, 6.8 item 8; brief M4-41): the forum of
/// the sidebar row that has the focus ([focused] is the context of the focused node), else the
/// [selected] forum of the route. A focused row of a text channel gives null, because threads
/// there come from messages (01 3.1).
ChannelId? newChatForum(BuildContext? focused, {required ChannelId? selected}) {
  final row = focused?.findAncestorWidgetOfExactType<ChannelEntry>();
  if (row == null) return selected;
  final channel = row.entry.channel;
  return channel.kind == ChannelKind.forum ? channel.id : null;
}
