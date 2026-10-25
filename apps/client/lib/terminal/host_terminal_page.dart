// SPDX-License-Identifier: Apache-2.0

/// The terminal view of one host on the right of the desktop shell (docs/spec/01-product.md
/// 5.1 D11; docs/spec/15-client-architecture.md 23; brief M4-36 Scope): the view of
/// `terminal_ui` with one tab per own session, "New session" with the profiles of the host,
/// the header with the time left until `terminal.max_session_duration`, the shortcuts of the
/// table "In the terminal" inside the pass-through region of M4-24, and the search in the
/// scrollback.
///
/// Terminal content stays in memory: the search reads the scrollback of the renderer and keeps
/// nothing (R-CL-17).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' show Host, HostProfile;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfigValues;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../keyboard/bindings.dart' show ShortcutPlatform;
import '../switcher/quick_switcher.dart' show showQuickSwitcher;
import 'providers.dart';
import 'terminal_shortcuts.dart';

/// The lines of the scrollback of [controller] that contain [query] (case-insensitive), in
/// order, at most [limit]; empty for an empty query.
List<String> scrollbackMatches(
  TerminalController controller,
  String query, {
  int limit = 200,
}) {
  final q = query.trim().toLowerCase();
  if (q.isEmpty) return const [];
  final lines = controller.screen.terminal.buffer.lines;
  final out = <String>[];
  for (var i = 0; i < lines.length && out.length < limit; i++) {
    final text = lines[i].getText().trimRight();
    if (text.toLowerCase().contains(q)) out.add(text);
  }
  return out;
}

/// The view of [hostId], or the hint to choose a host when it is null (the shortcut "Open
/// hosts and terminal" without a host).
class HostTerminalPage extends ConsumerStatefulWidget {
  /// Creates the page.
  const HostTerminalPage({super.key, this.hostId, this.platform});

  /// The host of the route, or null.
  final String? hostId;

  /// The keyboard family of the shortcuts; null uses the running platform.
  final ShortcutPlatform? platform;

  @override
  ConsumerState<HostTerminalPage> createState() => _HostTerminalPageState();
}

class _HostTerminalPageState extends ConsumerState<HostTerminalPage> {
  TerminalTabs? _refreshed;
  bool _searching = false;
  final TextEditingController _search = TextEditingController();

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  void _refreshOnce(TerminalTabs tabs) {
    if (identical(_refreshed, tabs)) return;
    _refreshed = tabs;
    unawaited(tabs.refresh());
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final hostId = widget.hostId;
    final overview = ref.watch(hostsListProvider);
    Widget note(String key, String text) => Center(
      key: ValueKey(key),
      child: Padding(
        padding: const EdgeInsets.all(DsSpace.s24),
        child: Text(
          text,
          textAlign: TextAlign.center,
          style: dsTextStyle(DsType.body).copyWith(color: c.textSecondary),
        ),
      ),
    );
    if (hostId == null) {
      final hosts = overview.value?.hosts ?? const <Host>[];
      return Material(
        color: c.surfaceCanvas,
        child: hosts.isEmpty && overview.hasValue
            ? note('terminal.noHosts', l.terminalNoHosts)
            : note('terminal.chooseHost', l.terminalChooseHost),
      );
    }
    final host = overview.value?.host(hostId);
    // A failed list without any earlier value would otherwise wait forever.
    if (overview.hasError && !overview.hasValue) {
      return Material(
        color: c.surfaceCanvas,
        child: note('terminal.loadFailed', l.hostsLoadFailed),
      );
    }
    if (overview.hasValue && overview.value != null && host == null) {
      return Material(
        color: c.surfaceCanvas,
        child: note('terminal.hostGone', l.terminalHostGone),
      );
    }
    final tabs = ref.watch(terminalTabsProvider(hostId));
    if (tabs == null || host == null) {
      return Material(
        color: c.surfaceCanvas,
        child: const Center(child: CircularProgressIndicator()),
      );
    }
    _refreshOnce(tabs);
    final config = tabs.services.config;
    final platform = widget.platform ?? ShortcutPlatform.current;
    return Material(
      key: ValueKey('terminal.page.$hostId'),
      color: c.surfaceCanvas,
      child: ListenableBuilder(
        listenable: tabs,
        builder: (context, _) {
          final current = tabs.current;
          final labels = tabs.labels;
          final handlers = <TerminalShortcutAction, VoidCallback?>{
            TerminalShortcutAction.quickSwitcher: () =>
                unawaited(showQuickSwitcher(context)),
            TerminalShortcutAction.newSession: host.profiles.isEmpty
                ? null
                : () => _newSession(tabs, host.profiles.first),
            TerminalShortcutAction.nextSession: tabs.tabs.length > 1
                ? tabs.next
                : null,
            TerminalShortcutAction.previousSession: tabs.tabs.length > 1
                ? tabs.previous
                : null,
            TerminalShortcutAction.detach: current == null
                ? null
                : () => unawaited(current.detach()),
            TerminalShortcutAction.search: current == null
                ? null
                : () => setState(() => _searching = true),
            TerminalShortcutAction.fontLarger: () => tabs.changeFont(1),
            TerminalShortcutAction.fontSmaller: () => tabs.changeFont(-1),
          };
          return TerminalShortcutScope(
            handlers: handlers,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _Toolbar(host: host, onNew: (p) => _newSession(tabs, p)),
                if (_searching && current != null)
                  _SearchPanel(
                    controller: current,
                    query: _search,
                    onClose: () => setState(() {
                      _searching = false;
                      _search.clear();
                    }),
                  ),
                Expanded(
                  child: current == null
                      ? note('terminal.empty', l.terminalNoSessions)
                      : TerminalPane(
                          key: ValueKey('terminal.pane.$hostId'),
                          controller: current,
                          fontSize: tabs.fontSize,
                          tabs: [
                            for (final (profile, n) in labels)
                              SessionTabData(
                                label: l.terminalTabLabel(profile, n),
                              ),
                          ],
                          selected: tabs.selected,
                          onSelect: tabs.select,
                          maxSessionDuration: config.maxSessionDuration,
                          shortcuts: terminalActivators(platform),
                          autofocus: true,
                        ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }

  void _newSession(TerminalTabs tabs, HostProfile profile) =>
      unawaited(tabs.newSession(profile.name));
}

/// The row above the view: the host's name and "New session" with its profiles.
class _Toolbar extends StatelessWidget {
  const _Toolbar({required this.host, required this.onNew});

  final Host host;
  final ValueChanged<HostProfile> onNew;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s12,
        vertical: DsSpace.s4,
      ),
      child: Row(
        children: [
          Expanded(
            child: Text(
              host.name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: dsTextStyle(DsType.label).copyWith(color: c.textSecondary),
            ),
          ),
          PopupMenuButton<HostProfile>(
            key: const ValueKey('terminal.new'),
            tooltip: l.terminalNewSession,
            enabled: host.profiles.isNotEmpty,
            onSelected: onNew,
            itemBuilder: (context) => [
              for (final p in host.profiles)
                PopupMenuItem(
                  key: ValueKey('terminal.new.${p.name}'),
                  value: p,
                  child: Text(p.displayName),
                ),
            ],
            child: Padding(
              padding: const EdgeInsets.all(DsSpace.s4),
              child: Text(
                l.terminalNewSession,
                style: dsTextStyle(DsType.label).copyWith(color: c.textPrimary),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// The search in the scrollback (01 7, "In the terminal"): a field, the count of matching
/// lines and the lines themselves, read from memory and kept nowhere.
class _SearchPanel extends StatefulWidget {
  const _SearchPanel({
    required this.controller,
    required this.query,
    required this.onClose,
  });

  final TerminalController controller;
  final TextEditingController query;
  final VoidCallback onClose;

  @override
  State<_SearchPanel> createState() => _SearchPanelState();
}

class _SearchPanelState extends State<_SearchPanel> {
  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final matches = scrollbackMatches(widget.controller, widget.query.text);
    return Container(
      key: const ValueKey('terminal.search'),
      color: c.surfaceRaised,
      constraints: const BoxConstraints(maxHeight: DsLayout.columnMin),
      padding: const EdgeInsets.all(DsSpace.s8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: TextField(
                  key: const ValueKey('terminal.search.field'),
                  controller: widget.query,
                  autofocus: true,
                  decoration: InputDecoration(
                    hintText: l.terminalSearchHint,
                    labelText: l.terminalSearchHint,
                  ),
                  onChanged: (_) => setState(() {}),
                ),
              ),
              IconButton(
                key: const ValueKey('terminal.search.close'),
                tooltip: l.terminalSearchClose,
                onPressed: widget.onClose,
                icon: DsIcon(DsIcons.actionDismiss, decorative: true),
              ),
            ],
          ),
          if (widget.query.text.trim().isNotEmpty) ...[
            const SizedBox(height: DsSpace.s4),
            Semantics(
              liveRegion: true,
              child: Text(
                l.terminalSearchMatches(matches.length),
                key: const ValueKey('terminal.search.count'),
                style: dsTextStyle(DsType.caption)
                    .copyWith(color: c.textSecondary),
              ),
            ),
            Flexible(
              child: ListView(
                shrinkWrap: true,
                children: [
                  for (final line in matches)
                    Text(
                      line,
                      style: dsTextStyle(DsType.codeSm)
                          .copyWith(color: c.textPrimary),
                    ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}
