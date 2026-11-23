// SPDX-License-Identifier: Apache-2.0

/// The page "Hosts and terminal" of the console (docs/spec/01-product.md 5.5; ADR 0070; brief
/// M6-18 Design note 4): screen A8 of `admin_ui` and, from a host, the terminal view of
/// `terminal_ui`.
library;

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfigValues;
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../admin/console_admin.dart';
import '../admin/console_route.dart';
import '../session/console_controller.dart';
import 'console_terminals.dart';

/// Stable keys of the page, for the widget tests and the `patrol` web test.
abstract final class ConsoleTerminalKeys {
  /// "Open terminal" on the page of a host.
  static const open = ValueKey('console.terminal.open');

  /// The terminal view (`TerminalPane`).
  static const pane = ValueKey('console.terminal.pane');

  /// "Detach": closes the stream, the session keeps running on the host.
  static const detach = ValueKey('console.terminal.detach');

  /// "Reattach": a new passkey step-up and a new stream (no quick reattach in the console).
  static const reattach = ValueKey('console.terminal.reattach');

  /// "End": ends the session (`closeTerminalSession`).
  static const end = ValueKey('console.terminal.end');

  /// "Back to host": closes the view of a session that no longer runs.
  static const close = ValueKey('console.terminal.close');

  /// The note that names the keys which leave the terminal.
  static const leaveHint = ValueKey('console.terminal.leaveHint');
}

/// The keys that move the focus from the terminal to its buttons (WCAG 2.1.2): the terminal
/// takes Tab, Shift+Tab and F6 for the shell, so the way out needs the platform modifier of the
/// terminal's app shortcuts (01 7, R-PR-10): Cmd+Shift+F6 on Apple systems, Ctrl+Shift+F6
/// elsewhere.
SingleActivator consoleLeaveTerminalKeys(TargetPlatform platform) =>
    _apple(platform)
    ? const SingleActivator(LogicalKeyboardKey.f6, meta: true, shift: true)
    : const SingleActivator(LogicalKeyboardKey.f6, control: true, shift: true);

/// The label of [consoleLeaveTerminalKeys] for [platform].
String consoleLeaveTerminalLabel(TargetPlatform platform) =>
    _apple(platform) ? 'Cmd+Shift+F6' : 'Ctrl+Shift+F6';

bool _apple(TargetPlatform platform) =>
    platform == TargetPlatform.macOS || platform == TargetPlatform.iOS;

/// Moves the focus from the terminal to the buttons above it.
class _LeaveTerminalIntent extends Intent {
  const _LeaveTerminalIntent();
}

/// The page of [route] on "Hosts and terminal".
///
/// The list and the page of a host are A8 of `admin_ui` ([AdminBody]) over the console's
/// repository, so every change of a host passes the console's step-up adapter of M6-04. The
/// page of a host adds "Open terminal"; an open terminal of that host replaces the page with
/// [TerminalPane] and the controls "Detach", "Reattach", "End" and "Back to host". The view
/// has no live update: A8 reads the host through REST when it opens and after each own change
/// (15 23.1).
///
/// Example: `ConsoleHostsPage(controller: c, admin: admin, terminals: terminals, route: r)`.
class ConsoleHostsPage extends StatelessWidget {
  /// Creates the page.
  const ConsoleHostsPage({
    super.key,
    required this.controller,
    required this.admin,
    required this.terminals,
    required this.route,
  });

  /// The console's state; its places change through it.
  final ConsoleController controller;

  /// The administration state (repository, time zone, clock).
  final ConsoleAdmin admin;

  /// The open terminals of the tab.
  final ConsoleTerminals terminals;

  /// The place to show.
  final ConsoleRoute route;

  @override
  Widget build(BuildContext context) {
    final at = route.admin ?? const AdminLocation(AdminSection.hosts);
    final hostId = at.section == AdminSection.hosts ? at.id : null;
    // The rows of A8 paint their ink on the nearest Material; the console's shell paints its
    // canvas with a ColoredBox, which would hide it.
    return Material(
      type: MaterialType.transparency,
      child: ListenableBuilder(
        listenable: terminals,
        builder: (context, _) {
          final terminal = hostId == null ? null : terminals.of(hostId);
          if (hostId != null && terminal != null) {
            return _TerminalView(
              terminal: terminal,
              fontSize: terminals.config.terminalFontSizeDefault.toDouble(),
              onClose: () => terminals.close(hostId),
            );
          }
          final body = AdminBody(
            key: ValueKey('console.admin.${route.path}.${at.id}'),
            repository: admin.repository,
            location: at,
            maxWidth: null,
            timeZone: admin.timeZone,
            clock: admin.clock,
            onNavigate: controller.openAdmin,
          );
          if (hostId == null) return body;
          final l = AppLocalizations.of(context);
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(
                  DsSpace.s24,
                  DsSpace.s8,
                  DsSpace.s24,
                  0,
                ),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: FilledButton.icon(
                    key: ConsoleTerminalKeys.open,
                    icon: DsIcon(
                      DsIcons.securityLock,
                      size: DsIconSizeRole.sm,
                      decorative: true,
                    ),
                    label: Text(l.consoleTerminalOpen),
                    onPressed: () => unawaited(terminals.open(hostId)),
                  ),
                ),
              ),
              Expanded(child: body),
            ],
          );
        },
      ),
    );
  }
}

/// The terminal of one host with the console's controls above it, and the keys that leave the
/// terminal for those controls ([consoleLeaveTerminalKeys]).
class _TerminalView extends StatefulWidget {
  const _TerminalView({
    required this.terminal,
    required this.fontSize,
    required this.onClose,
  });

  final TerminalController terminal;
  final double fontSize;
  final VoidCallback onClose;

  @override
  State<_TerminalView> createState() => _TerminalViewState();
}

class _TerminalViewState extends State<_TerminalView> {
  final _back = FocusNode(debugLabel: 'console.terminal.close');
  final _detach = FocusNode(debugLabel: 'console.terminal.detach');
  final _reattach = FocusNode(debugLabel: 'console.terminal.reattach');
  final _end = FocusNode(debugLabel: 'console.terminal.end');

  @override
  void dispose() {
    for (final node in [_back, _detach, _reattach, _end]) {
      node.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final terminal = widget.terminal;
    final platform = defaultTargetPlatform;
    return ListenableBuilder(
      listenable: terminal,
      builder: (context, _) {
        final state = terminal.state;
        final phase = state.phase;
        final live =
            phase == TerminalPhase.connected || phase == TerminalPhase.exited;
        final over =
            state.sessionId == null ||
            phase == TerminalPhase.ended ||
            phase == TerminalPhase.blocked ||
            phase == TerminalPhase.failed;
        final detached = phase == TerminalPhase.detached;
        final endable = state.sessionId != null && !over;
        // The first button that can act takes the focus when the person leaves the terminal.
        final target = [
          if (over) _back,
          if (live) _detach,
          if (detached) _reattach,
          if (endable) _end,
        ].firstOrNull;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: DsSpace.s24,
                vertical: DsSpace.s8,
              ),
              child: Wrap(
                spacing: DsSpace.s8,
                runSpacing: DsSpace.s8,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  OutlinedButton(
                    key: ConsoleTerminalKeys.close,
                    focusNode: _back,
                    onPressed: over ? widget.onClose : null,
                    child: Text(l.consoleTerminalBack),
                  ),
                  OutlinedButton(
                    key: ConsoleTerminalKeys.detach,
                    focusNode: _detach,
                    onPressed: live ? () => unawaited(terminal.detach()) : null,
                    child: Text(l.terminalDetach),
                  ),
                  OutlinedButton(
                    key: ConsoleTerminalKeys.reattach,
                    focusNode: _reattach,
                    onPressed: detached
                        ? () => unawaited(terminal.attach())
                        : null,
                    child: Text(l.terminalReattach),
                  ),
                  OutlinedButton(
                    key: ConsoleTerminalKeys.end,
                    focusNode: _end,
                    onPressed: endable ? () => unawaited(terminal.end()) : null,
                    child: Text(l.terminalEnd),
                  ),
                  Text(
                    l.consoleTerminalLeaveHint(
                      consoleLeaveTerminalLabel(platform),
                    ),
                    key: ConsoleTerminalKeys.leaveHint,
                    style: dsTextStyle(DsType.caption),
                  ),
                ],
              ),
            ),
            Expanded(
              child: Actions(
                actions: {
                  _LeaveTerminalIntent: CallbackAction<_LeaveTerminalIntent>(
                    onInvoke: (_) {
                      target?.requestFocus();
                      return null;
                    },
                  ),
                },
                child: TerminalPane(
                  key: ConsoleTerminalKeys.pane,
                  controller: terminal,
                  fontSize: widget.fontSize,
                  shortcuts: {
                    consoleLeaveTerminalKeys(platform):
                        const _LeaveTerminalIntent(),
                  },
                ),
              ),
            ),
          ],
        );
      },
    );
  }
}
