// SPDX-License-Identifier: Apache-2.0

/// The group "Hosts" of the sidebar below the spaces (docs/spec/01-product.md 5.1 D11; brief
/// M4-36 fixed requirement 12): every host the person sees, its connection and the count of
/// the person's own sessions; a disabled host shows "disabled". A click opens the terminal view
/// of the host (`/hosts/<id>`).
library;

import 'package:api_client/api_client.dart' show Host;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import 'providers.dart';

/// One row of the group: the host, its connection and the count of own sessions.
@immutable
final class HostRowData {
  /// Creates the row data.
  const HostRowData({
    required this.id,
    required this.name,
    required this.connected,
    required this.disabled,
    required this.ownSessions,
  });

  /// Builds the rows of [overview], in its order.
  static List<HostRowData> of(HostsOverview overview) => [
    for (final h in overview.hosts) HostRowData.from(h, overview),
  ];

  /// The row of [host] in [overview].
  factory HostRowData.from(Host host, HostsOverview overview) => HostRowData(
    id: host.id,
    name: host.name,
    connected: overview.isConnected(host.id),
    disabled: hostDisabled(host),
    ownSessions: overview.sessionCount(host.id),
  );

  /// The host ID.
  final String id;

  /// The host name.
  final String name;

  /// Whether the host service is connected.
  final bool connected;

  /// Whether the host is disabled.
  final bool disabled;

  /// The count of the person's own sessions that are not closed.
  final int ownSessions;
}

/// The group as it looks for given rows, without providers (widget and golden tests use it).
///
/// Shows nothing when [rows] is empty and nothing failed: a person without hosts has no group.
class HostsGroupView extends StatelessWidget {
  /// Creates the view.
  const HostsGroupView({
    super.key,
    required this.rows,
    required this.onOpen,
    this.selectedHost,
    this.failed = false,
  });

  /// The rows.
  final List<HostRowData> rows;

  /// The host the route shows, or null.
  final String? selectedHost;

  /// Opens the terminal view of a host.
  final ValueChanged<String> onOpen;

  /// Whether listing the hosts failed.
  final bool failed;

  @override
  Widget build(BuildContext context) {
    if (rows.isEmpty && !failed) return const SizedBox.shrink();
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    return Semantics(
      key: const ValueKey('hosts.group'),
      container: true,
      explicitChildNodes: true,
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s8,
          vertical: DsSpace.s4,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Semantics(
              header: true,
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: DsSpace.s8,
                  vertical: DsSpace.s4,
                ),
                child: Text(
                  l.hostsGroupTitle,
                  style: dsTextStyle(DsType.label)
                      .copyWith(color: c.textOnSidebarSecondary),
                ),
              ),
            ),
            if (failed)
              Padding(
                key: const ValueKey('hosts.failed'),
                padding: const EdgeInsets.all(DsSpace.s8),
                child: Text(
                  l.hostsLoadFailed,
                  style: dsTextStyle(DsType.caption)
                      .copyWith(color: c.textOnSidebarSecondary),
                ),
              ),
            for (final row in rows)
              _HostRow(
                row: row,
                selected: row.id == selectedHost,
                onOpen: () => onOpen(row.id),
              ),
          ],
        ),
      ),
    );
  }
}

class _HostRow extends StatelessWidget {
  const _HostRow({
    required this.row,
    required this.selected,
    required this.onOpen,
  });

  final HostRowData row;
  final bool selected;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final state = row.disabled
        ? l.hostStateDisabled
        : row.connected
        ? l.hostConnected
        : l.hostDisconnected;
    final color = row.disabled
        ? c.textOnSidebarMuted
        : c.textOnSidebarSecondary;
    return DsTappable(
      key: ValueKey('hosts.row.${row.id}'),
      onTap: onOpen,
      label: l.hostRowLabel(row.name, state, row.ownSessions),
      selected: selected,
      button: false,
      onSidebar: true,
      hoverColor: c.surfaceSidebarHover,
      excludeChildSemantics: true,
      child: Container(
        constraints: BoxConstraints(minHeight: context.dsDensity.rowChannel),
        decoration: BoxDecoration(
          color: selected ? c.surfaceSidebarSelected : null,
          borderRadius: BorderRadius.circular(DsRadius.sm),
        ),
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
        child: Row(
          children: [
            Container(
              key: ValueKey('hosts.dot.${row.id}'),
              width: DsSize.statusDot,
              height: DsSize.statusDot,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: row.connected && !row.disabled
                    ? c.categorySpruceOnSidebar
                    : null,
                border: Border.all(color: color),
              ),
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    dsIsolate(row.name),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.bodySm).copyWith(color: color),
                  ),
                  if (row.disabled)
                    Text(
                      l.hostStateDisabled,
                      key: ValueKey('hosts.disabled.${row.id}'),
                      style: dsTextStyle(DsType.caption).copyWith(color: color),
                    ),
                ],
              ),
            ),
            if (row.ownSessions > 0) ...[
              const SizedBox(width: DsSpace.s6),
              Text(
                dsFormat(context).count(row.ownSessions),
                key: ValueKey('hosts.count.${row.id}'),
                style: dsTextStyle(DsType.caption).copyWith(color: color),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// The group of the signed-in person over the providers (R-CL-03); [selectedHost] follows the
/// route.
class HostsGroup extends ConsumerWidget {
  /// Creates the group.
  const HostsGroup({super.key, this.selectedHost});

  /// The host the route shows, or null.
  final String? selectedHost;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final hosts = ref.watch(hostsListProvider);
    final overview = hosts.value;
    return HostsGroupView(
      rows: overview == null ? const [] : HostRowData.of(overview),
      selectedHost: selectedHost,
      failed: hosts.hasError && overview == null,
      onOpen: (id) => context.go('/hosts/$id'),
    );
  }
}
