// SPDX-License-Identifier: Apache-2.0

// Test support of the phone terminal (brief M5-12 Tests): screen M8 over the fakes of the
// desktop terminal (TerminalWorld), with a life cycle and an app lock the test switches, a
// platform guard in memory, and a page below M8 that system back returns to.

import 'dart:async';
import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/lock/app_lock_controller.dart' show AppLockEvent;
import 'package:client/phone/terminal/providers.dart';
import 'package:client/phone/terminal/terminal_lifecycle.dart';
import 'package:client/phone/terminal/terminal_session_screen.dart';
import 'package:client/platform/terminal_guard.dart';
import 'package:client/terminal/providers.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../../support/fakes.dart';
import '../../terminal/support.dart';

/// A platform guard in memory: the secure window as a log, the capture state and the keyboard
/// as values the test sets.
final class FakeGuard implements TerminalPlatformGuard {
  /// Every `setSecure` call, in order.
  final List<bool> secure = [];

  /// Whether a hardware keyboard is connected.
  bool keyboard = false;

  final ValueNotifier<bool> _captured = ValueNotifier(false);
  final _Pulse _changes = _Pulse();

  @override
  ValueListenable<bool> get captured => _captured;

  /// Sets the capture state.
  set capturedNow(bool value) => _captured.value = value;

  @override
  Listenable get keyboardChanges => _changes;

  /// Reports a keyboard change of the platform.
  void keyboardChanged() => _changes.fire();

  @override
  Future<void> setSecure(bool on) async => secure.add(on);

  @override
  Future<bool> connected() async => keyboard;
}

final class _Pulse extends ChangeNotifier {
  void fire() => notifyListeners();
}

/// An app lock gate the test opens and closes.
final class FakeLockGate extends ChangeNotifier implements TerminalLockGate {
  final StreamController<AppLockEvent> _events =
      StreamController<AppLockEvent>.broadcast(sync: true);
  bool _holds = false;

  @override
  bool get holdsContent => _holds;

  @override
  Stream<AppLockEvent> get events => _events.stream;

  /// Closes the lock (E6 shows).
  void lock() {
    _holds = true;
    _events.add(AppLockEvent.locked);
    notifyListeners();
  }

  /// Holds the content without an event: the lock decides on a return.
  void deciding() {
    _holds = true;
    notifyListeners();
  }

  /// Opens the lock after the person passed it.
  void unlock() {
    _holds = false;
    _events.add(AppLockEvent.unlocked);
    notifyListeners();
  }

  /// Ends the decision without locking.
  void decided() {
    _holds = false;
    notifyListeners();
  }
}

/// M8 of the host of [world] pumped over a page "below", with the switched [lifecycle], the
/// [gate] and the [guard].
final class PhoneTerminalHarness {
  PhoneTerminalHarness._(
    this.world,
    this.container,
    this.lifecycle,
    this.gate,
    this.guard,
    this.coordinator,
  );

  final TerminalWorld world;
  final ProviderContainer container;
  final SwitchLifecycle lifecycle;
  final FakeLockGate gate;
  final FakeGuard guard;
  final PhoneTerminalCoordinator coordinator;

  /// Pumps the page below M8 and pushes M8 on it.
  static Future<PhoneTerminalHarness> pump(
    WidgetTester tester, {
    TerminalWorld? world,
    Locale locale = const Locale('en'),
    TerminalServices Function(TerminalWorld world)? services,
  }) async {
    tester.view
      ..physicalSize = const Size(390, 844)
      ..devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final w = world ?? await TerminalWorld.create(tester);
    final lifecycle = SwitchLifecycle();
    final gate = FakeLockGate();
    final guard = FakeGuard();
    final coordinator = PhoneTerminalCoordinator(
      lifecycle: lifecycle,
      lock: gate,
    );
    addTearDown(coordinator.dispose);
    final overview = HostsOverview(
      hosts: [w.api.current],
      ownSessions: const {},
      connected: const {hostIdText: true},
    );
    final container = ProviderContainer(
      overrides: [
        terminalServicesProvider.overrideWithValue(
          services == null ? w.services() : services(w),
        ),
        terminalSelfUserIdProvider.overrideWithValue(alexId),
        hostsListProvider.overrideWith(() => FixedHosts(overview)),
        appPortsProvider.overrideWithValue(
          testPorts(
            dataRoot: Directory.systemTemp,
            platform: 'android',
            lifecycle: lifecycle,
            terminalGuard: guard,
          ),
        ),
        phoneTerminalCoordinatorProvider.overrideWithValue(coordinator),
      ],
    );
    addTearDown(container.dispose);
    const ds = DsContext(
      theme: DsThemeMode.light,
      density: DsDensityMode.touch,
      motion: DsMotionMode.reduced,
      accent: dsDefaultAccent,
    );
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          navigatorKey: w.navigatorKey,
          theme: buildDsTheme(ds),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: locale,
          builder: (context, child) =>
              DsThemeScope(dsContext: ds, child: child!),
          home: const Scaffold(body: Text('below', key: ValueKey('below'))),
        ),
      ),
    );
    unawaited(
      w.navigatorKey.currentState!.push(
        MaterialPageRoute<void>(
          builder: (_) =>
              const Scaffold(body: TerminalSessionScreen(hostId: hostIdText)),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return PhoneTerminalHarness._(
      w,
      container,
      lifecycle,
      gate,
      guard,
      coordinator,
    );
  }

  /// The tabs of the host.
  TerminalTabs get tabs => container.read(terminalTabsProvider(hostIdText))!;

  /// Opens a shell through "New session", the dialog and the step-up, until it is connected.
  Future<TerminalController> openShell(WidgetTester tester) async {
    await tester.tap(find.byKey(const ValueKey('phone.terminal.new')));
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(const ValueKey('phone.terminal.new.shell')).last,
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text(en.terminalOpenConfirm));
    await tester.pump();
    await pumpUntil(
      tester,
      () => tabs.current?.state.phase == TerminalPhase.connected,
    );
    return tabs.current!;
  }

  /// Ends every session and lets the streams close, so no timer outlives the test.
  Future<void> close(WidgetTester tester) async {
    for (final c in tabs.tabs) {
      await c.detach();
    }
    await tester.pump(const Duration(seconds: 1));
  }
}
