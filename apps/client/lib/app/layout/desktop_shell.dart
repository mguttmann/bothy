// SPDX-License-Identifier: Apache-2.0

/// The main view of the desktop after the login: the connection banner and the four column
/// frames that M4-19 to M4-25 fill (docs/spec/01-produkt.md 5.1, 6.6, 6.7).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../onboarding/onboarding_controller.dart';
import '../providers/ports.dart';
import '../providers/session.dart';
import '../providers/settings.dart';
import '../providers/target.dart';
import '../router.dart';
import '../target_resolver.dart';
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
              minWidthFourColumns: config.desktopMinWidthFourColumns
                  .toDouble(),
              minWidthThreeColumns: config.desktopMinWidthThreeColumns
                  .toDouble(),
              stackShowsHistory: parsed is ThreadLocation,
              onWidthsChanged: (next) {
                final engine = ref.read(syncEngineProvider);
                if (engine != null) ColumnWidthsWriter(engine).save(next);
              },
              sidebar: Align(
                alignment: Alignment.bottomLeft,
                child: Padding(
                  padding: const EdgeInsets.all(DsSpace.s8),
                  child: TextButton(
                    key: const ValueKey('shell.logout'),
                    onPressed: () => ref
                        .read(onboardingControllerProvider.notifier)
                        .logout(),
                    child: Text(
                      l.onboardingLogOut,
                      style: TextStyle(color: context.dsColors.textOnSidebar),
                    ),
                  ),
                ),
              ),
              postList: const SizedBox.expand(),
              history: _TargetView(location: location),
              session: const SizedBox.expand(),
            ),
          ),
        ],
      ),
    );
  }
}

/// The history column's placeholder for the target: empty while it is visible or loading,
/// "Not found or no access" when it is not (M4-20 shows the thread here).
class _TargetView extends ConsumerWidget {
  const _TargetView({required this.location});

  final Uri location;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref.watch(targetStatusProvider(location.toString())).value;
    if (status != TargetStatus.notFound) return const SizedBox.expand();
    return Center(
      child: Text(
        AppLocalizations.of(context).accessNotFoundOrNoAccess,
        key: const ValueKey('target.notFound'),
        style: dsTextStyle(
          DsType.body,
        ).copyWith(color: context.dsColors.textSecondary),
      ),
    );
  }
}
