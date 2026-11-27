// SPDX-License-Identifier: Apache-2.0

/// The providers of the desktop window, the tray and "Start at login" (brief M4-30; docs/spec/
/// 15-client-architecture.md 10.3).
library;

import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../computer/computer_tray.dart';
import '../onboarding/onboarding_controller.dart';
import '../settings/settings_providers.dart';
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

/// The tray of the desktop app, or null where the app has none: on Windows the icon with Open
/// and Quit stays (15 10.3); on macOS the icon shows only while a computer session of the owner
/// is live (D-CU-07-4, which narrows D-M4-30-5).
@Riverpod(keepAlive: true)
DesktopTray? desktopTray(Ref ref) {
  final ports = ref.watch(appPortsProvider);
  final windows = ports.platform == 'windows';
  if (!windows && ports.platform != 'macos') return null;
  final tray = DesktopTray(
    port: ports.tray,
    behavior: ref.watch(windowBehaviorProvider),
    persistent: windows,
  );
  ref.onDispose(tray.dispose);
  return tray;
}

/// The texts of the app in the language of `users.locale` once the person chose one, the
/// system language before (01 9; brief M4-32).
AppLocalizations _trayTexts(String? userLocale) => lookupAppLocalizations(
  resolveAppLocale(
    userLocale: userLocale,
    systemLocales: WidgetsBinding.instance.platformDispatcher.locales,
  ),
);

/// Starts the window behavior and the tray: on Windows the tray icon with Open and Quit, on
/// macOS the texts of the icon that shows during computer use; the app widget watches it once.
@Riverpod(keepAlive: true)
Future<void> desktopShell(Ref ref) async {
  final ports = ref.watch(appPortsProvider);
  final behavior = ref.watch(windowBehaviorProvider);
  final tray = ref.watch(desktopTrayProvider);
  if (tray == null) {
    await behavior.start();
    return;
  }
  // 01 9: the menu speaks the language of the app and follows a later choice (01 8).
  Future<void> label(String? userLocale) {
    final l = _trayTexts(userLocale);
    return tray.start(
      tooltip: ports.branding.appName,
      openLabel: l.trayOpen,
      quitLabel: l.trayQuit,
    );
  }

  ref.listen(profileControllerProvider, (before, now) {
    final userLocale = now.value?.locale;
    if (userLocale != before?.value?.locale) unawaited(label(userLocale));
  });
  await label(ref.read(profileControllerProvider).value?.locale);
  // Hide on close only once the tray icon exists: a failed tray start leaves the window
  // closing normally instead of hiding it where nothing can bring it back.
  await behavior.start();
}

/// The computer entry of the tray (27 11 item 4; brief CU-07): follows the owner's live
/// computer sessions; null where the app has no tray.
@Riverpod(keepAlive: true)
ComputerTray? computerTray(Ref ref) {
  final tray = ref.watch(desktopTrayProvider);
  if (tray == null) return null;
  return ComputerTray(
    tray: tray,
    ref: ref,
    label: (desktops) =>
        _trayTexts(ref.read(profileControllerProvider).value?.locale)
            .trayComputerStop(desktops),
  );
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
