// SPDX-License-Identifier: Apache-2.0

/// The modifiers of the extra keys bar (docs/spec/01-product.md screen M8: "Ctrl and Alt act
/// once for the next key; tapping twice holds them until the next tap"; brief M5-12 Design
/// note 2; docs/decisions.d/M5-12.md D-M5-12-1).
library;

import 'package:auth/auth.dart' show Clock, SystemClock;
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart' show kDoubleTapTimeout;
import 'package:terminal_ui/src/extra_keys/key_mapping.dart';

/// The state of one modifier of the bar.
enum ModifierLatch {
  /// Off.
  off,

  /// On for the next key only (one tap).
  oneShot,

  /// On until the next tap on it (a second tap within the double-tap interval, or a long
  /// press).
  latched,
}

/// Ctrl and Alt of the bar and how they change input.
///
/// A tap on an off modifier makes it one-shot; a second tap within [doubleTapInterval] latches
/// it; any other tap turns it off. A long press latches it at once (screen readers and
/// switch access cannot tap twice in time). [transform] applies the modifiers to text the
/// person typed, so a letter of the soft keyboard after Ctrl becomes its control byte; one-shot
/// modifiers clear after the next key from the bar or the keyboard, latched ones stay.
///
/// [clock] gives the time of each tap; tests pass a fake one.
///
/// ```dart
/// final keys = ExtraKeysController();
/// controller.inputTransform = keys.transform;
/// ```
final class ExtraKeysController extends ChangeNotifier {
  /// Creates the modifiers, both off.
  ExtraKeysController({this._clock = const SystemClock()});

  /// The time within which a second tap latches a modifier: the platform's double-tap
  /// interval of Flutter's gesture system.
  static const Duration doubleTapInterval = kDoubleTapTimeout;

  final Clock _clock;
  ModifierLatch _ctrl = ModifierLatch.off;
  ModifierLatch _alt = ModifierLatch.off;
  DateTime? _ctrlTap;
  DateTime? _altTap;
  bool _bypass = false;

  /// The state of Ctrl.
  ModifierLatch get ctrl => _ctrl;

  /// The state of Alt.
  ModifierLatch get alt => _alt;

  /// A tap on Ctrl: off becomes one-shot, one-shot becomes latched within
  /// [doubleTapInterval] and off after it, latched becomes off.
  void tapCtrl() {
    final now = _clock.now();
    _ctrl = _next(_ctrl, _ctrlTap, now);
    _ctrlTap = now;
    notifyListeners();
  }

  /// A tap on Alt, as [tapCtrl].
  void tapAlt() {
    final now = _clock.now();
    _alt = _next(_alt, _altTap, now);
    _altTap = now;
    notifyListeners();
  }

  /// A long press on Ctrl: latched until the next tap.
  void latchCtrl() {
    _ctrl = ModifierLatch.latched;
    _ctrlTap = null;
    notifyListeners();
  }

  /// A long press on Alt, as [latchCtrl].
  void latchAlt() {
    _alt = ModifierLatch.latched;
    _altTap = null;
    notifyListeners();
  }

  /// Applies the modifiers to [text] typed by the person and clears one-shot ones.
  ///
  /// Ctrl turns one letter or one of `@[\]^_` into its control byte ([controlByte]) and leaves
  /// any other character unmodified; Alt puts ESC in front. Text of more than one character (a
  /// paste, an answer of the renderer) passes unchanged and clears nothing.
  String transform(String text) {
    if (_bypass ||
        text.runes.length != 1 ||
        (_ctrl == ModifierLatch.off && _alt == ModifierLatch.off)) {
      return text;
    }
    var out = text;
    if (_ctrl != ModifierLatch.off) out = controlByte(out);
    if (_alt != ModifierLatch.off) out = '\x1b$out';
    _release();
    return out;
  }

  /// Sends the sequence of [key] through [send] with Alt applied (Ctrl does not change a key
  /// of the bar), past [transform], and clears one-shot modifiers.
  void sendKey(
    ExtraKey key, {
    required bool applicationCursor,
    required void Function(String data) send,
  }) {
    final data = extraKeySequence(
      key,
      applicationCursor: applicationCursor,
      alt: _alt != ModifierLatch.off,
    );
    _bypass = true;
    try {
      send(data);
    } finally {
      _bypass = false;
    }
    _release();
  }

  static ModifierLatch _next(
    ModifierLatch state,
    DateTime? lastTap,
    DateTime now,
  ) => switch (state) {
    ModifierLatch.off => ModifierLatch.oneShot,
    ModifierLatch.oneShot
        when lastTap != null &&
            !now.isBefore(lastTap) &&
            now.difference(lastTap) <= doubleTapInterval =>
      ModifierLatch.latched,
    _ => ModifierLatch.off,
  };

  void _release() {
    var changed = false;
    if (_ctrl == ModifierLatch.oneShot) {
      _ctrl = ModifierLatch.off;
      changed = true;
    }
    if (_alt == ModifierLatch.oneShot) {
      _alt = ModifierLatch.off;
      changed = true;
    }
    if (changed) notifyListeners();
  }
}
