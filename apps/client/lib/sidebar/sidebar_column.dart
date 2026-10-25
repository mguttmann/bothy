// SPDX-License-Identifier: Apache-2.0

/// The sidebar column of the desktop main view (01 5.1 D1; components.md 7; brief M4-19
/// Goal): head, tree of spaces and channels, foot.
library;

import 'dart:async';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart' show AdminLocation, AdminSection;
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../admin/admin_route.dart' show adminPathOf;
import '../app/providers/session.dart';
import '../onboarding/onboarding_controller.dart';
import '../running/providers.dart' show activeRunCountProvider;
import '../switcher/quick_switcher.dart' show showQuickSwitcher;
import '../sync/sidebar_ops.dart';
import '../telemetry/telemetry_providers.dart';
import '../terminal/hosts_group.dart';
import '../viz/live_page.dart' show livePath;
import 'providers.dart';
import 'sidebar_footer.dart';
import 'sidebar_header.dart';
import 'sidebar_live_group.dart';
import 'sidebar_model.dart';
import 'sidebar_order.dart';
import 'space_tree.dart';

/// The largest share of the window height the group "Hosts" takes; it scrolls beyond it.
const double hostsHeightShare = 0.3;

/// The sidebar as it looks for given data, without providers (widget and golden tests use it
/// directly).
class SidebarView extends StatelessWidget {
  /// Creates the view.
  const SidebarView({
    super.key,
    required this.productName,
    required this.nodes,
    required this.onToggle,
    required this.onMove,
    required this.onOpenChannel,
    required this.onSettings,
    required this.onLogout,
    this.logo,
    this.ownName,
    this.selectedChannel,
    this.onJumpTo,
    this.onRunningNow,
    this.runningCount = 0,
    this.onLive,
    this.onStatistics,
    this.showFooter = true,
    this.hosts,
  });

  /// `branding.product_name` of the instance.
  final String productName;

  /// The logo bytes, or null.
  final Uint8List? logo;

  /// The tree.
  final List<SidebarSpaceNode> nodes;

  /// The display name of the signed-in person, or null.
  final String? ownName;

  /// The selected channel, or null.
  final ChannelId? selectedChannel;

  /// Collapses or expands a space.
  final void Function(SidebarSpaceNode node, bool expanded) onToggle;

  /// Moves a space to a new index.
  final void Function(SpaceId space, int to) onMove;

  /// Opens a channel.
  final ValueChanged<ChannelId> onOpenChannel;

  /// Opens the settings.
  final VoidCallback onSettings;

  /// Logs out.
  final VoidCallback onLogout;

  /// Opens the quick switcher (01 D6), or null.
  final VoidCallback? onJumpTo;

  /// Opens "Running now" (01 D10), or null.
  final VoidCallback? onRunningNow;

  /// The count of active runs of the entry "Running now".
  final int runningCount;

  /// Opens "Live" (D12); with [onStatistics] it shows the group "Live and statistics", null
  /// hides it (the phone).
  final VoidCallback? onLive;

  /// Opens "Statistics" (A7).
  final VoidCallback? onStatistics;

  /// Whether the foot with the person, Settings and Log out shows; the phone has them in its
  /// Me tab (brief M5-02 Design note 2).
  final bool showFooter;

  /// The group "Hosts" below the spaces (01 D11, M4-36), or null (the phone has no terminal
  /// yet).
  final Widget? hosts;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      SidebarHeader(
        productName: productName,
        logo: logo,
        onJumpTo: onJumpTo,
        onRunningNow: onRunningNow,
        runningCount: runningCount,
      ),
      if (onLive != null && onStatistics != null)
        SidebarLiveGroup(onLive: onLive!, onStatistics: onStatistics!),
      Expanded(
        child: SpaceTree(
          nodes: nodes,
          selectedChannel: selectedChannel,
          onToggle: onToggle,
          onMove: onMove,
          onOpenChannel: onOpenChannel,
        ),
      ),
      // The group scrolls within a share of the column, so the spaces keep their room.
      if (hosts case final group?)
        ConstrainedBox(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.sizeOf(context).height * hostsHeightShare,
          ),
          child: SingleChildScrollView(child: group),
        ),
      if (showFooter)
        SidebarFooter(
          name: ownName,
          onSettings: onSettings,
          onLogout: onLogout,
        ),
    ],
  );
}

/// The sidebar of the signed-in instance over the providers (R-CL-03).
///
/// The desktop shows the group "Live and statistics" (D12) below the head. A click or Enter on
/// a channel goes to `/c/<id>`; collapse and drag write the personal
/// sidebar state through the outbox ([SidebarWriter]); [selectedChannel] follows the route.
/// As M1 of the phone ([phone], 01 5.3) it has no foot (the Me tab has it) and the search
/// field opens Search instead of the quick switcher.
class SidebarColumn extends ConsumerWidget {
  /// Creates the column.
  const SidebarColumn({
    super.key,
    this.selectedChannel,
    this.selectedHost,
    this.phone = false,
  });

  /// The channel of the route, or null.
  final ChannelId? selectedChannel;

  /// The host of the route (`/hosts/<id>`), or null.
  final String? selectedHost;

  /// Whether the column is the Spaces page of the phone.
  final bool phone;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final instance = ref.watch(currentInstanceProvider);
    final nodes = ref.watch(sidebarTreeProvider);
    // `client.cold_start.duration` ends with the first sidebar that shows spaces from the
    // local database (15 19, brief M4-31); later calls do nothing.
    if (nodes.isNotEmpty) ref.read(appMeasurementsProvider).sidebarShown();
    SidebarWriter? writer() {
      final engine = ref.read(syncEngineProvider);
      final db = ref.read(localDatabaseProvider);
      return engine == null || db == null ? null : SidebarWriter(engine, db);
    }

    return SidebarView(
      productName: instance?.productName ?? '',
      logo: instance?.logo,
      nodes: nodes,
      ownName: ref.watch(ownPrincipalProvider).value?.displayName,
      selectedChannel: selectedChannel,
      onToggle: (node, expanded) =>
          unawaited(writer()?.setCollapsed(node.entry, collapsed: !expanded)),
      onMove: (space, to) => unawaited(
        writer()?.move(moveSpace([for (final n in nodes) n.entry], space, to)),
      ),
      onOpenChannel: (channel) => context.go('/c/${channel.value}'),
      onSettings: () => context.go('/settings'),
      onJumpTo: phone
          ? () => context.go('/search')
          : () => unawaited(showQuickSwitcher(context)),
      showFooter: !phone,
      // The terminal of the phone is M5's; the desktop shows the group below the spaces.
      hosts: phone ? null : HostsGroup(selectedHost: selectedHost),
      onRunningNow: () => context.go('/running'),
      runningCount: ref.watch(activeRunCountProvider),
      onLive: phone ? null : () => context.go(livePath),
      onStatistics: phone
          ? null
          : () => context.go(
              adminPathOf(const AdminLocation(AdminSection.statistics)),
            ),
      onLogout: () =>
          unawaited(ref.read(onboardingControllerProvider.notifier).logout()),
    );
  }
}
