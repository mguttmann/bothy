// SPDX-License-Identifier: Apache-2.0

/// The open terminals of the console: one `TerminalController` per host over the console's
/// ports (docs/spec/15-client-architecture.md 23.1; brief M6-18 Design note 3).
library;

import 'dart:async';
import 'dart:math';

import 'package:auth/auth.dart' show SystemClock;
import 'package:domain/domain.dart' show AppConfig, HostId, Uuid7Generator;
import 'package:flutter/foundation.dart';
import 'package:flutter/scheduler.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../session/console_controller.dart';
import 'ports.dart';
import 'step_up_prompt.dart';
import 'terminal_api.dart';
import 'transport.dart';

/// The profile the dialog "Open terminal" preselects; every host reports it (24 11.1).
const String consoleDefaultTerminalProfile = 'shell';

/// The terminals of the console that outlive a page: the person can leave "Hosts and terminal"
/// and come back to the same tab, while the stream keeps running.
///
/// Every controller gets the console's ports: [api] ([ConsoleTerminalApi]), its own
/// [ConsoleTerminalTransport] (one transport per tab, because `identify` reads the device of
/// the tab's last stream), the pins of the console ([ConsoleController.terminalPins]), the
/// passkey step-up ([stepUp]) and no host events: the console reloads through REST when the
/// view opens and after each own action, never live (15 23.1). The ephemeral Noise key comes
/// from the controller's default factory (`bothy_noise` with `cryptography`) and lives only in
/// the memory of the tab (R-CL-15).
///
/// When the session ends or another person signs in, every controller is disposed, which
/// closes its stream and destroys its key; the server ends the sessions of a logout itself.
///
/// Example:
/// ```dart
/// final terminals = ConsoleTerminals(console: c, ports: ports,
///     prompts: (me) => ConsoleTerminalPrompts(context, selfUserId: me), stepUp: stepUp);
/// await terminals.open(hostId);
/// ```
class ConsoleTerminals extends ChangeNotifier {
  /// Creates the terminals of [console]. [prompts] answers the dialogs, [stepUp] confirms with
  /// the passkey, [config] gives the client settings of the view (defaults of the
  /// configuration when null), [random] draws the idempotency keys.
  ConsoleTerminals({
    required this.console,
    required this.ports,
    required this.prompts,
    required this.stepUp,
    AppConfig? config,
    Random? random,
  }) : config = config ?? AppConfig.defaults(),
       api = ConsoleTerminalApi(
         console.api,
         newIdempotencyKey: _ids(random ?? Random.secure()).next,
       ) {
    console.addListener(_onConsole);
    _user = console.session?.userId;
  }

  /// The console's state.
  final ConsoleController console;

  /// The browser seams of the terminal.
  final ConsoleTerminalPorts ports;

  /// The dialogs of the terminal for the signed-in person (its user ID finds the person's
  /// working folders and the warning of 24 8.4).
  final TerminalPrompts Function(String selfUserId) prompts;

  /// The passkey step-up of `terminal.open` and `terminal.attach`.
  final PasskeyStepUpPrompt stepUp;

  /// The client settings of the view (`client.terminal.*`).
  final AppConfig config;

  /// The REST operations of the terminal over the console's client.
  final ConsoleTerminalApi api;

  final Map<String, TerminalController> _open = {};
  String? _user;

  /// The terminal of [hostId], or null when none is open in this tab.
  TerminalController? of(String hostId) => _open[hostId];

  /// Opens a terminal on [hostId]: a new controller runs the flow of 15 23.2 (the host from
  /// REST, the pin check, the dialog, the passkey step-up, the stream). A cancelled or refused
  /// opening leaves no tab behind. Does nothing without a session or while the host has one.
  Future<void> open(String hostId) async {
    final self = console.session?.userId;
    if (self == null || _open.containsKey(hostId)) return;
    final controller = TerminalController(
      api: api,
      transport: ConsoleTerminalTransport(
        origin: console.environment.origin,
        openSocket: ports.connect,
        deviceIdFor: api.deviceIdFor,
      ),
      pins: console.terminalPins,
      stepUp: stepUp.confirm,
      links: _TabLinks(ports.openLink),
      clock: const SystemClock(),
      prompts: prompts(self),
      selfUserId: self,
      config: config,
    );
    _open[hostId] = controller;
    notifyListeners();
    await controller.open(
      host: HostId.parse(hostId),
      profile: consoleDefaultTerminalProfile,
      cols: 80,
      rows: 24,
    );
    // The view stays only for an opening that reached the server; a cancel shows the host.
    if (identical(_open[hostId], controller) &&
        controller.state.sessionId == null &&
        controller.state.phase != TerminalPhase.failed) {
      close(hostId);
    }
  }

  /// Closes the view of [hostId] and disposes its controller (the stream closes and the key is
  /// destroyed); a session that still runs stays detached on the host until it ends there.
  void close(String hostId) {
    final controller = _open.remove(hostId);
    if (controller == null) return;
    notifyListeners();
    _disposeAfterFrame([controller]);
  }

  void _onConsole() {
    final user = console.session?.userId;
    if (console.state is ConsoleSignedIn && user == _user) return;
    _user = user;
    if (_open.isEmpty) return;
    final gone = _open.values.toList();
    _open.clear();
    notifyListeners();
    _disposeAfterFrame(gone);
  }

  /// Disposes [controllers] once the frame that removes their panes is done, so no pane
  /// touches a disposed controller while it unmounts.
  void _disposeAfterFrame(List<TerminalController> controllers) {
    SchedulerBinding.instance
      ..addPostFrameCallback((_) {
        for (final c in controllers) {
          c.dispose();
        }
      })
      ..ensureVisualUpdate();
  }

  @override
  void dispose() {
    console.removeListener(_onConsole);
    for (final c in _open.values) {
      c.dispose();
    }
    _open.clear();
    super.dispose();
  }

  static Uuid7Generator _ids(Random random) => Uuid7Generator(
    nowMillis: () => DateTime.now().millisecondsSinceEpoch,
    // Not `1 << bits`: in the web build a shift works on 32 bits (as in ConsoleAdmin).
    random: (bits) => random.nextInt(pow(2, bits).toInt()),
  );
}

/// Opens a confirmed link in a new tab of the browser (24 11.2).
final class _TabLinks implements LinkOpener {
  _TabLinks(this._open);

  final void Function(Uri target) _open;

  @override
  Future<void> open(Uri target) async => _open(target);
}
