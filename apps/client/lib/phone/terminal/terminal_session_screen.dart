// SPDX-License-Identifier: Apache-2.0

/// Screen M8 of the phone, the terminal of one host (docs/spec/01-product.md 5.3 M8 and 5.4;
/// docs/spec/15-client-architecture.md 23; brief M5-12 Design notes 1 to 5): the header of
/// components.md 12, a session switcher with "Detach" and "End", the screen of `terminal_ui`
/// and the extra keys bar above the soft keyboard, hidden while a hardware keyboard is
/// connected.
///
/// Leaving the screen (the back button, the edge swipe, Android's system back) detaches the
/// sessions of the host: they keep running and M9 offers them again; only "End" or the server
/// ends one (01 5.4, D-M5-12-5). While the screen shows, Android keeps the window out of
/// screenshots, recordings and the recent apps (`FLAG_SECURE`), and iOS hides the terminal while
/// the screen is recorded or mirrored (D-M5-12-3). Terminal content stays in memory (R-CL-17).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' show Host, HostProfile;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfigValues;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../../app/providers/ports.dart';
import '../../platform/terminal_guard.dart';
import '../../terminal/providers.dart';
import 'providers.dart';

/// M8 of [hostId].
class TerminalSessionScreen extends ConsumerStatefulWidget {
  /// Creates the screen.
  const TerminalSessionScreen({super.key, required this.hostId});

  /// The host of the route.
  final String hostId;

  @override
  ConsumerState<TerminalSessionScreen> createState() =>
      _TerminalSessionScreenState();
}

class _TerminalSessionScreenState extends ConsumerState<TerminalSessionScreen>
    with WidgetsBindingObserver {
  // The M8 screens in the tree. A link to another host's M8 builds the new screen before the
  // old one is disposed, so only the last one to go gives the secure window back (D-M5-12-3).
  static int _screens = 0;

  late final TerminalPlatformGuard _guard;
  late final HardwareKeyboardWatch _keyboard;
  TerminalTabs? _registered;

  @override
  void initState() {
    super.initState();
    _guard = ref.read(appPortsProvider).terminalGuard;
    _keyboard = HardwareKeyboardWatch(_guard);
    _guard.keyboardChanges.addListener(_refreshKeyboard);
    WidgetsBinding.instance.addObserver(this);
    _screens++;
    unawaited(_guard.setSecure(true));
    _refreshKeyboard();
  }

  @override
  void dispose() {
    // M8 also leaves without a pop (a link replaces the stack): its sessions detach then too.
    _detachAll();
    WidgetsBinding.instance.removeObserver(this);
    _guard.keyboardChanges.removeListener(_refreshKeyboard);
    if (--_screens == 0) unawaited(_guard.setSecure(false));
    _keyboard.dispose();
    super.dispose();
  }

  void _refreshKeyboard() => unawaited(_keyboard.refresh());

  // A keyboard that connects or disconnects changes the soft keyboard's inset.
  @override
  void didChangeMetrics() => _refreshKeyboard();

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) _refreshKeyboard();
  }

  void _register(TerminalTabs tabs) {
    if (identical(_registered, tabs)) return;
    _registered = tabs;
    ref.read(phoneTerminalCoordinatorProvider)?.register(tabs);
    unawaited(tabs.refresh());
  }

  /// Leaving M8 detaches the live sessions of the host (01 5.4).
  void _detachAll() {
    for (final c in _registered?.tabs ?? const <TerminalController>[]) {
      final phase = c.state.phase;
      if (phase == TerminalPhase.connected || phase == TerminalPhase.exited) {
        unawaited(c.detach());
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final overview = ref.watch(hostsListProvider);
    final host = overview.value?.host(widget.hostId);
    final tabs = ref.watch(terminalTabsProvider(widget.hostId));
    Widget body;
    if (overview.value != null && host == null) {
      body = _Note(noteKey: 'phone.terminal.gone', text: l.terminalHostGone);
    } else if (tabs == null || host == null) {
      body = overview.hasError && !overview.hasValue
          ? _Note(noteKey: 'phone.terminal.failed', text: l.hostsLoadFailed)
          : const Center(child: CircularProgressIndicator());
    } else {
      _register(tabs);
      body = ValueListenableBuilder<bool>(
        valueListenable: _guard.captured,
        builder: (context, captured, _) => captured
            ? _Note(
                noteKey: 'phone.terminal.captured',
                text: l.terminalCaptureHidden,
              )
            : ListenableBuilder(
                listenable: tabs,
                builder: (context, _) => _view(context, tabs, host),
              ),
      );
    }
    return PopScope(
      onPopInvokedWithResult: (didPop, _) {
        if (didPop) _detachAll();
      },
      child: ColoredBox(
        key: ValueKey('phone.terminal.${widget.hostId}'),
        color: c.surfaceCanvas,
        child: body,
      ),
    );
  }

  Widget _view(BuildContext context, TerminalTabs tabs, Host host) {
    final l = AppLocalizations.of(context);
    final current = tabs.current;
    if (current == null) {
      return Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          _Note(noteKey: 'phone.terminal.empty', text: l.terminalNoSessions),
          if (host.profiles.isNotEmpty)
            _NewSession(
              host: host,
              onNew: (p) => unawaited(tabs.newSession(p.name)),
            ),
        ],
      );
    }
    return TerminalPane(
      key: ValueKey('phone.terminal.pane.${widget.hostId}'),
      controller: current,
      fontSize: tabs.fontSize,
      tabs: [
        for (final (profile, n) in tabs.labels)
          SessionTabData(label: l.terminalTabLabel(profile, n)),
      ],
      selected: tabs.selected,
      onSelect: tabs.select,
      maxSessionDuration: tabs.services.config.maxSessionDuration,
      now: tabs.services.clock.now,
      phone: true,
      showExtraKeys: true,
      hardwareKeyboard: _keyboard,
      autofocus: true,
    );
  }
}

class _NewSession extends StatelessWidget {
  const _NewSession({required this.host, required this.onNew});

  final Host host;
  final ValueChanged<HostProfile> onNew;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    // A node of its own: the menu button forms none, so its name and tap would go to the
    // screen around it.
    return Semantics(
      container: true,
      button: true,
      child: PopupMenuButton<HostProfile>(
        key: const ValueKey('phone.terminal.new'),
        tooltip: l.terminalNewSession,
        onSelected: onNew,
        itemBuilder: (context) => [
          for (final p in host.profiles)
            PopupMenuItem(
              key: ValueKey('phone.terminal.new.${p.name}'),
              value: p,
              child: Text(p.displayName),
            ),
        ],
        // The touch target of the density (density.target-min), as every control of M8.
        child: ConstrainedBox(
          constraints: BoxConstraints(minHeight: context.dsDensity.targetMin),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: DsSpace.s12),
            child: Center(
              widthFactor: 1,
              child: Text(
                l.terminalNewSession,
                style: dsTextStyle(DsType.label)
                    .copyWith(color: context.dsColors.textPrimary),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _Note extends StatelessWidget {
  const _Note({required this.noteKey, required this.text});

  final String noteKey;
  final String text;

  @override
  Widget build(BuildContext context) => Center(
    key: ValueKey(noteKey),
    child: Padding(
      padding: const EdgeInsets.all(DsSpace.s24),
      child: Text(
        text,
        textAlign: TextAlign.center,
        style: dsTextStyle(DsType.body)
            .copyWith(color: context.dsColors.textSecondary),
      ),
    ),
  );
}
