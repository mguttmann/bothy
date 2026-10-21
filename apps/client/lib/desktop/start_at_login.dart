// SPDX-License-Identifier: Apache-2.0

/// "Start at login", a device setting kept by the platform (brief M4-30 Design note 4; docs/
/// spec/15-client-architektur.md 10.3): the run key of the current user on Windows, a login
/// item on macOS; off by default and never in `user_settings`.
library;

import 'dart:io' show Platform;

import 'package:flutter/services.dart';
import 'package:win32_registry/win32_registry.dart';

/// Whether the app starts when the person signs in to this computer.
///
/// The value lives only in the system, so there is no second copy that could disagree with
/// it (D-M4-30-4); nothing is written until the person turns it on.
///
/// Example: `if (login.supported) await login.setEnabled(true);`
abstract interface class StartAtLogin {
  /// Whether this system offers a login start for the app.
  bool get supported;

  /// Whether the login start is on now; false where it is not [supported].
  Future<bool> isEnabled();

  /// Turns the login start on or off.
  ///
  /// Throws a [StateError] where it is not [supported], and what the platform throws.
  Future<void> setEnabled(bool on);
}

/// No login start: tests and systems without one.
final class NoStartAtLogin implements StartAtLogin {
  /// Creates the value.
  const NoStartAtLogin();

  @override
  bool get supported => false;

  @override
  Future<bool> isEnabled() async => false;

  @override
  Future<void> setEnabled(bool on) async =>
      throw StateError('no login start on this system');
}

/// The value `<name>` under `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`
/// with the quoted path of the executable; no administrator rights are needed.
///
/// Example: `WindowsRunKey(name: branding.appName, executable: Platform.resolvedExecutable)`.
final class WindowsRunKey implements StartAtLogin {
  /// Creates the value [name] that starts [executable].
  WindowsRunKey({required this.name, required this.executable});

  /// The value name, the app name of the build.
  final String name;

  /// The path of the executable.
  final String executable;

  static const String _path = r'Software\Microsoft\Windows\CurrentVersion\Run';

  @override
  bool get supported => true;

  String get _command => '"$executable"';

  @override
  Future<bool> isEnabled() async {
    final key = CURRENT_USER.create(_path);
    try {
      return key.getString(name) == _command;
    } finally {
      key.close();
    }
  }

  @override
  Future<void> setEnabled(bool on) async {
    final key = CURRENT_USER.create(_path);
    try {
      if (on) {
        key.setValue(name, StringValue(_command));
      } else if (key.getString(name) != null) {
        key.removeValue(name);
      }
    } finally {
      key.close();
    }
  }
}

/// The login item of the app on macOS 13 and later (`SMAppService.mainApp`), over the method
/// channel `bothy/start_at_login` of the runner (`AppDelegate.swift`).
final class MacLoginItem implements StartAtLogin {
  /// Creates the value; [supported] is the answer of `isSupported` at the start.
  MacLoginItem._(this._channel, this.supported);

  /// Asks the runner whether the system offers login items; an older runner or system gives
  /// an unsupported value.
  static Future<MacLoginItem> open({MethodChannel? channel}) async {
    final c = channel ?? const MethodChannel('bothy/start_at_login');
    try {
      return MacLoginItem._(
        c,
        await c.invokeMethod<bool>('isSupported') ?? false,
      );
    } on MissingPluginException {
      return MacLoginItem._(c, false);
    } on PlatformException {
      return MacLoginItem._(c, false);
    }
  }

  final MethodChannel _channel;

  @override
  final bool supported;

  @override
  Future<bool> isEnabled() async =>
      supported && (await _channel.invokeMethod<bool>('isEnabled') ?? false);

  @override
  Future<void> setEnabled(bool on) async {
    if (!supported) throw StateError('no login item on this system');
    await _channel.invokeMethod<void>('setEnabled', on);
  }
}

/// The login start of the running platform: [WindowsRunKey] on Windows, [MacLoginItem] on
/// macOS, [NoStartAtLogin] elsewhere.
Future<StartAtLogin> platformStartAtLogin({required String appName}) async {
  if (Platform.isWindows) {
    return WindowsRunKey(
      name: appName,
      executable: Platform.resolvedExecutable,
    );
  }
  if (Platform.isMacOS) return MacLoginItem.open();
  return const NoStartAtLogin();
}
