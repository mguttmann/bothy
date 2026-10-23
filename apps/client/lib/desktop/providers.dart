// SPDX-License-Identifier: Apache-2.0

/// The providers of the desktop window, the tray and "Start at login" (brief M4-30; docs/spec/
/// 15-client-architecture.md 10.3).
library;

import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../onboarding/onboarding_controller.dart';
import 'start_at_login.dart';
import 'tray.dart';
import 'window_behavior.dart';

part 'providers.g.dart';

/// The behavior of the app window: on Windows a close hides it to the notification area
/// (15 10.3), and Quit closes the services of the instance (engine and database) before the
/// window ends, so the device has no gateway connection any more.
@Riverpod(keepAlive: true)
WindowBehavior windowBehavior(Ref ref) {
  final ports = ref.watch(appPortsProvider);
  final behavior = WindowBehavior(
    window: ports.window,
    hideOnClose: ports.platform == 'windows',
    shutdown: () async =>
        ref.read(onboardingControllerProvider).services?.close(),
  );
  ref.onDispose(behavior.dispose);
  return behavior;
}

/// Starts the window behavior and, on Windows, the tray icon with Open and Quit; the app
/// widget watches it once.
@Riverpod(keepAlive: true)
Future<void> desktopShell(Ref ref) async {
  final ports = ref.watch(appPortsProvider);
  final behavior = ref.watch(windowBehaviorProvider);
  // D-M4-30-5: the notification area is the Windows path; macOS keeps its window behavior.
  if (ports.platform != 'windows') {
    await behavior.start();
    return;
  }
  final tray = DesktopTray(port: ports.tray, behavior: behavior);
  ref.onDispose(tray.dispose);
  final l = lookupAppLocalizations(
    resolveAppLocale(
      systemLocales: WidgetsBinding.instance.platformDispatcher.locales,
    ),
  );
  await tray.start(
    tooltip: ports.branding.appName,
    openLabel: l.trayOpen,
    quitLabel: l.trayQuit,
  );
  // Hide on close only once the tray icon exists: a failed tray start leaves the window
  // closing normally instead of hiding it where nothing can bring it back.
  await behavior.start();
}

/// The state of "Start at login": whether the system offers it and whether it is on.
typedef StartAtLoginState = ({bool supported, bool enabled});

/// "Start at login", read from and written to the system (D-M4-30-4); off until the person
/// turns it on.
@Riverpod(keepAlive: true)
class StartAtLoginSetting extends _$StartAtLoginSetting {
  StartAtLogin? _login;

  @override
  Future<StartAtLoginState> build() async {
    final login = await ref.watch(appPortsProvider).startAtLogin();
    _login = login;
    return (supported: login.supported, enabled: await login.isEnabled());
  }

  /// Turns the login start [on] or off and reads the system's value back; an error of the
  /// system becomes the error state.
  Future<void> set(bool on) async {
    final login = _login;
    if (login == null || !login.supported) return;
    state = await AsyncValue.guard(() async {
      await login.setEnabled(on);
      return (supported: true, enabled: await login.isEnabled());
    });
  }
}
