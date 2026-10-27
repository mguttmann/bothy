// SPDX-License-Identifier: Apache-2.0

/// Whether a physical keyboard is connected, so the phone hides the extra keys bar
/// (docs/spec/01-product.md screen M8: "With a hardware keyboard the app hides the bar";
/// brief M5-12 Design note 2; docs/decisions.d/M5-12.md D-M5-12-6).
library;

import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

/// Asks the platform whether a physical keyboard is connected (Android: the configuration's
/// keyboard; iOS: `GCKeyboard`). The app passes the adapter over its runner; tests pass a fake.
abstract interface class HardwareKeyboardProbe {
  /// Whether a physical keyboard is connected now; throws when the platform cannot tell.
  Future<bool> connected();
}

/// The connected state of a physical keyboard, asked from a [HardwareKeyboardProbe].
///
/// Flutter itself has no "keyboard connected" signal, and a soft keyboard of Android may send
/// key events too, so a key event alone proves nothing: it only makes the watch ask the probe
/// again (while it believes no keyboard is there). The app also calls [refresh] when the
/// terminal opens, when the app returns to the foreground and when the window metrics change
/// (a keyboard that connects or disconnects changes the soft keyboard's inset). A failed probe
/// keeps the last answer.
///
/// ```dart
/// final watch = HardwareKeyboardWatch(probe);
/// await watch.refresh();
/// TerminalPane(..., showExtraKeys: true, hardwareKeyboard: watch);
/// ```
final class HardwareKeyboardWatch extends ChangeNotifier {
  /// Creates the watch over [probe] and starts listening to key events of [keyboard]
  /// (`HardwareKeyboard.instance` by default).
  HardwareKeyboardWatch(this._probe, {HardwareKeyboard? keyboard})
    : _keyboard = keyboard ?? HardwareKeyboard.instance {
    _keyboard.addHandler(_onKey);
  }

  final HardwareKeyboardProbe _probe;
  final HardwareKeyboard _keyboard;
  bool _connected = false;
  bool _disposed = false;
  int _asked = 0;

  /// Whether a physical keyboard is connected, as the probe last answered.
  bool get connected => _connected;

  /// Asks the probe again; notifies when the answer changed. The answer of the latest call
  /// wins over an older one that arrives later.
  Future<void> refresh() async {
    final ask = ++_asked;
    final bool value;
    try {
      value = await _probe.connected();
    } on Object {
      return;
    }
    if (_disposed || ask != _asked || value == _connected) return;
    _connected = value;
    notifyListeners();
  }

  // Never consumes the event: the key goes on to the renderer.
  bool _onKey(KeyEvent event) {
    if (event is KeyDownEvent && !_connected) unawaited(refresh());
    return false;
  }

  @override
  void dispose() {
    _disposed = true;
    _keyboard.removeHandler(_onKey);
    super.dispose();
  }
}
