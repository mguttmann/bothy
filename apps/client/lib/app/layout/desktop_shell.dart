// SPDX-License-Identifier: Apache-2.0

/// The main view of the desktop after the login: the connection banner and the four column
/// frames that M4-19 to M4-25 fill (docs/spec/01-product.md 5.1, 6.6, 6.7); M4-19 fills the
/// sidebar and the post list of a forum, M4-20 the history and the frame of the session
/// column.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../forum/post_list_column.dart';
import '../../session_column/session_column.dart';
import '../../settings/settings_providers.dart';
import '../../sidebar/providers.dart';
import '../../sidebar/sidebar_column.dart';
import '../../thread/history_area.dart';
import '../../thread/providers.dart';
import '../providers/ports.dart';
import '../providers/session.dart';
import '../providers/settings.dart';
import '../router.dart';
import 'column_layout.dart';
import 'column_widths.dart';

/// The desktop main view for [location] (path and query of the router).
///
/// Reads only providers (R-CL-03): the connection state of the engine, the stored column
/// widths, the collapse widths of `AppConfig` and the status of the target. The banner
/// "Offline" sits at the top of the content while the engine has no connection (01 6.6),
/// "Update required" after `4007` (15 15). A location whose target is invisible shows "Not
/// found or no access" in the history column (01 6.7).
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
    // D2: with a thread of a text channel open, the session column is collapsed.
    final textThread =
        selection.kind == ChannelKind.text && selection.thread != null;
    final banner = switch (phase) {
      ConnectionPhase.updateRequired => DsBanner(
        key: const ValueKey('shell.updateRequired'),
        tone: DsTone.danger,
        text: l.shellUpdateRequired,
        liveRegion: true,
      ),
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
    return Material(
      color: context.dsColors.surfaceCanvas,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          ?banner,
          Expanded(
            child: ColumnLayout(
              widths: widths,
              minWidthFourColumns: config.desktopMinWidthFourColumns.toDouble(),
              minWidthThreeColumns: config.desktopMinWidthThreeColumns
                  .toDouble(),
              stackShowsHistory: parsed is ThreadLocation,
              showPostList: forum != null,
              showSession: ref.watch(sessionColumnShownProvider) && !textThread,
              onWidthsChanged: (next) {
                final settings = ref.read(settingsRepositoryProvider);
                if (settings != null) ColumnWidthsWriter(settings).save(next);
              },
              sidebar: SidebarColumn(selectedChannel: selection.channel),
              postList: forum == null
                  ? const SizedBox.expand()
                  : PostListColumn(
                      key: ValueKey('postList.${forum.value}'),
                      channelId: forum,
                      selectedThread: selection.thread,
                    ),
              history: HistoryArea(location: location),
              session: SessionColumn(threadId: selection.thread),
            ),
          ),
        ],
      ),
    );
  }
}
