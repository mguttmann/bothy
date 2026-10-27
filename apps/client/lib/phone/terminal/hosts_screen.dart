// SPDX-License-Identifier: Apache-2.0

/// Screen M9 of the phone, hosts and sessions (docs/spec/01-product.md 5.3 M9; brief M5-12
/// Design note 1): the hosts the person may see with their connection, the badge of the LLM
/// login and the person's own sessions; a tap on a session attaches it in M8, "New session"
/// opens one.
///
/// It uses the controllers of the desktop (M4-36): `hostsListProvider` (REST, reloaded after
/// the host events of `sync_engine`, and once when the screen opens) and `terminalTabsProvider`.
/// No permission logic: the hosts and profiles are what the server returns (15 1 principle 1).
library;

import 'dart:async';

import 'package:api_client/api_client.dart'
    show Host, HostProfile, TerminalSession;
import 'package:api_client/api_client.dart' as rest show TerminalSessionState;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart' show TerminalPhase, llmLoginBadge;

import '../../terminal/providers.dart';

/// The path of M8 for [hostId].
String terminalPathOf(String hostId) => '/hosts/$hostId';

/// M9: the list of hosts and own sessions.
class HostsScreen extends ConsumerStatefulWidget {
  /// Creates the screen.
  const HostsScreen({super.key});

  @override
  ConsumerState<HostsScreen> createState() => _HostsScreenState();
}

class _HostsScreenState extends ConsumerState<HostsScreen> {
  @override
  void initState() {
    super.initState();
    // A refresh on opening (Design note 1); the host events keep it current afterwards.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) unawaited(ref.read(hostsListProvider.notifier).reload());
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final overview = ref.watch(hostsListProvider);
    final value = overview.value;
    if (value == null) {
      return Center(
        key: const ValueKey('phone.hosts.state'),
        child: overview.hasError
            ? Padding(
                padding: const EdgeInsets.all(DsSpace.s24),
                child: Text(l.hostsLoadFailed, textAlign: TextAlign.center),
              )
            : const CircularProgressIndicator(),
      );
    }
    if (value.hosts.isEmpty) {
      return Center(
        key: const ValueKey('phone.hosts.empty'),
        child: Padding(
          padding: const EdgeInsets.all(DsSpace.s24),
          child: Text(
            l.terminalNoHosts,
            textAlign: TextAlign.center,
            style: dsTextStyle(DsType.body).copyWith(color: c.textSecondary),
          ),
        ),
      );
    }
    return ListView(
      key: const ValueKey('phone.hosts'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      children: [
        for (final host in value.hosts)
          _HostCard(
            host: host,
            connected: value.isConnected(host.id),
            sessions: value.ownSessions[host.id] ?? const [],
            onOpen: () => context.go(terminalPathOf(host.id)),
            onSession: (s) => unawaited(_attach(host, s)),
            onNew: (p) => _newSession(host, p),
          ),
      ],
    );
  }

  /// Goes to M8 with [session] selected and attaches it when it is detached; the controller
  /// checks the pin, the warning and the step-up as on the desktop.
  Future<void> _attach(Host host, TerminalSession session) async {
    final tabs = ref.read(terminalTabsProvider(host.id));
    context.go(terminalPathOf(host.id));
    if (tabs == null) return;
    await tabs.refresh();
    final index = tabs.tabs.indexWhere((t) => t.state.sessionId == session.id);
    if (index < 0) return;
    tabs.select(index);
    final controller = tabs.tabs[index];
    if (controller.state.phase == TerminalPhase.detached) {
      await controller.attach();
    }
  }

  void _newSession(Host host, HostProfile profile) {
    final tabs = ref.read(terminalTabsProvider(host.id));
    context.go(terminalPathOf(host.id));
    if (tabs != null) unawaited(tabs.newSession(profile.name));
  }
}

/// One host: name, connection, the LLM login badge, its own sessions and "New session".
class _HostCard extends StatelessWidget {
  const _HostCard({
    required this.host,
    required this.connected,
    required this.sessions,
    required this.onOpen,
    required this.onSession,
    required this.onNew,
  });

  final Host host;
  final bool connected;
  final List<TerminalSession> sessions;
  final VoidCallback onOpen;
  final ValueChanged<TerminalSession> onSession;
  final ValueChanged<HostProfile> onNew;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final disabled = hostDisabled(host);
    final state = disabled
        ? l.hostStateDisabled
        : connected
        ? l.hostConnected
        : l.hostDisconnected;
    final caption = dsTextStyle(DsType.caption)
        .copyWith(color: c.textSecondary);
    // The label of each session as the tabs name it: profile and running number per profile.
    final counts = <String, int>{};
    final labels = [
      for (final s in sessions)
        l.terminalTabLabel(
          s.profile,
          counts[s.profile] = (counts[s.profile] ?? 0) + 1,
        ),
    ];
    return Padding(
      key: ValueKey('phone.hosts.${host.id}'),
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s8,
        vertical: DsSpace.s4,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          DsTappable(
            key: ValueKey('phone.hosts.open.${host.id}'),
            onTap: onOpen,
            label: l.hostRowLabel(host.name, state, sessions.length),
            excludeChildSemantics: true,
            child: ConstrainedBox(
              constraints: BoxConstraints(
                minHeight: context.dsDensity.controlHeight,
              ),
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
                child: Row(
                  children: [
                    Container(
                      width: DsSize.statusDot,
                      height: DsSize.statusDot,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: connected && !disabled ? c.successFg : null,
                        border: Border.all(color: c.textSecondary),
                      ),
                    ),
                    const SizedBox(width: DsSpace.s8),
                    // The badge sits at the end of the row (01 M9) while both fit; at a large
                    // text size it moves below the name instead of squeezing it (WCAG 1.4.4).
                    Expanded(
                      child: Wrap(
                        alignment: WrapAlignment.spaceBetween,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        spacing: DsSpace.s8,
                        runSpacing: DsSpace.s4,
                        children: [
                          Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Text(
                                dsIsolate(host.name),
                                style: dsTextStyle(DsType.label)
                                    .copyWith(color: c.textPrimary),
                              ),
                              Text(state, style: caption),
                            ],
                          ),
                          llmLoginBadge(context, host),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          for (final (i, s) in sessions.indexed)
            DsTappable(
              key: ValueKey('phone.hosts.session.${s.id}'),
              onTap: () => onSession(s),
              label: l.hostSessionRowLabel(labels[i], _stateOf(l, s.state)),
              excludeChildSemantics: true,
              child: ConstrainedBox(
                constraints: BoxConstraints(
                  minHeight: context.dsDensity.controlHeight,
                ),
                child: Padding(
                  padding: const EdgeInsets.only(
                    left: DsSpace.s24,
                    right: DsSpace.s8,
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          labels[i],
                          style: dsTextStyle(DsType.bodySm)
                              .copyWith(color: c.textPrimary),
                        ),
                      ),
                      Text(_stateOf(l, s.state), style: caption),
                    ],
                  ),
                ),
              ),
            ),
          if (!disabled && host.profiles.isNotEmpty)
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: Padding(
                padding: const EdgeInsets.only(left: DsSpace.s16),
                // A node of its own: the menu button forms none, so its name and tap would go
                // to the whole card around the host and session rows.
                child: Semantics(
                  container: true,
                  button: true,
                  child: PopupMenuButton<HostProfile>(
                    key: ValueKey('phone.hosts.new.${host.id}'),
                    tooltip: l.terminalNewSession,
                    onSelected: onNew,
                    itemBuilder: (context) => [
                      for (final p in host.profiles)
                        PopupMenuItem(
                          key: ValueKey('phone.hosts.new.${host.id}.${p.name}'),
                          value: p,
                          child: Text(p.displayName),
                        ),
                    ],
                    child: ConstrainedBox(
                      constraints: BoxConstraints(
                        // The touch target of the density (density.target-min): a
                        // control height of `touch` is below it.
                        minHeight: context.dsDensity.targetMin,
                      ),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: DsSpace.s8,
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            DsIcon(
                              DsIcons.actionNew,
                              size: DsIconSizeRole.sm,
                              color: c.textPrimary,
                              decorative: true,
                            ),
                            const SizedBox(width: DsSpace.s4),
                            Text(
                              l.terminalNewSession,
                              style: dsTextStyle(DsType.label)
                                  .copyWith(color: c.textPrimary),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }

  static String _stateOf(AppLocalizations l, rest.TerminalSessionState state) =>
      switch (state) {
        rest.TerminalSessionState.active => l.hostSessionStateActive,
        rest.TerminalSessionState.opening => l.hostSessionStateOpening,
        _ => l.hostSessionStateDetached,
      };
}
