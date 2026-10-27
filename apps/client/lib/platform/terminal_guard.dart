// SPDX-License-Identifier: Apache-2.0

/// What the phone terminal needs from the platform (brief M5-12 Design notes 2 and 4;
/// docs/decisions.d/M5-12.md D-M5-12-3 and D-M5-12-6): the secure window of Android while a
/// terminal is open, the screen capture of iOS, and whether a physical keyboard is connected.
library;

import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:terminal_ui/terminal_ui.dart' show HardwareKeyboardProbe;

import '../src/generated/android_platform_api.g.dart';
import '../src/generated/ios_platform_api.g.dart';

/// The platform side of the phone terminal.
abstract interface class TerminalPlatformGuard
    implements HardwareKeyboardProbe {
  /// Asks for a window that the system keeps out of screenshots, recordings and the recent
  /// apps screen while [on] (Android `FLAG_SECURE`); a no-op where the platform cannot.
  Future<void> setSecure(bool on);

  /// Whether the screen is recorded or mirrored now (iOS); always false elsewhere.
  ValueListenable<bool> get captured;

  /// Notifies when the platform reports a keyboard that connected or disconnected, so the
  /// terminal asks [connected] again.
  Listenable get keyboardChanges;
}

/// The guard of the desktop and of tests: nothing to protect, never captured, no keyboard
/// reported.
final class NoTerminalGuard implements TerminalPlatformGuard {
  /// Creates the guard.
  NoTerminalGuard();

  @override
  final ValueListenable<bool> captured = ValueNotifier(false);

  @override
  final Listenable keyboardChanges = ChangeNotifier();

  @override
  Future<void> setSecure(bool on) async {}

  @override
  Future<bool> connected() async => false;
}

/// The guard over the Android runner: `FLAG_SECURE` through `setSecureWindow` and the
/// configuration's keyboard.
final class AndroidTerminalGuard implements TerminalPlatformGuard {
  /// Creates the guard over [api].
  AndroidTerminalGuard(this._api);

  final AndroidPlatformApi _api;

  @override
  final ValueListenable<bool> captured = ValueNotifier(false);

  @override
  final Listenable keyboardChanges = ChangeNotifier();

  @override
  Future<void> setSecure(bool on) => _api.setSecureWindow(on);

  @override
  Future<bool> connected() => _api.hardwareKeyboardConnected();
}

/// The guard over the iOS runner: the capture state and the keyboard from `IosPlatformApi`, and
/// their changes from `IosScreenEvents`. iOS has no secure window; the terminal hides itself
/// while [captured] instead (D-M5-12-3).
final class IosTerminalGuard implements TerminalPlatformGuard, IosScreenEvents {
  /// Creates the guard over [api], receives the events of the runner and reads the capture
  /// state once.
  IosTerminalGuard(this._api) {
    IosScreenEvents.setUp(this);
    unawaited(_readCapture());
  }

  final IosPlatformApi _api;
  final ValueNotifier<bool> _captured = ValueNotifier(false);
  final _Pulse _keyboard = _Pulse();

  @override
  ValueListenable<bool> get captured => _captured;

  @override
  Listenable get keyboardChanges => _keyboard;

  @override
  Future<void> setSecure(bool on) async {}

  @override
  Future<bool> connected() => _api.hardwareKeyboardConnected();

  @override
  void captureChanged(bool captured) => _captured.value = captured;

  @override
  void hardwareKeyboardChanged(bool connected) => _keyboard.fire();

  Future<void> _readCapture() async {
    try {
      _captured.value = await _api.screenCaptured();
    } on Object {
      // The runner answers the next change through captureChanged.
    }
  }
}

final class _Pulse extends ChangeNotifier {
  void fire() => notifyListeners();
}
