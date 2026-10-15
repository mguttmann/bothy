// SPDX-License-Identifier: Apache-2.0

/// The four inputs of the theme: theme, density, motion and accent.
library;

import 'package:flutter/foundation.dart';

/// Color theme of the apps (`appearance.theme` after resolving `system`).
enum DsThemeMode {
  /// Light theme (`DsColors.light`).
  light,

  /// Dark theme (`DsColors.dark`).
  dark,
}

/// Density context of the tokens (`DsDensity`).
///
/// `comfortable` and `compact` are the desktop choices of `appearance.density`;
/// `touch` is fixed on touch platforms (docs/spec/01-produkt.md 11).
enum DsDensityMode {
  /// Desktop default.
  comfortable,

  /// Desktop, selectable.
  compact,

  /// Mobile, fixed.
  touch,
}

/// Motion context of the tokens (`DsMotion`).
enum DsMotionMode {
  /// Full motion.
  full,

  /// Reduced motion: the system setting or `appearance.reduce_motion` asks for it.
  reduced,
}

/// The resolved inputs of [buildDsTheme]: one value per token context plus the accent.
///
/// A value type: two contexts with the same fields are equal, so a widget can compare
/// the previous and the next context and rebuild the theme only on a change.
///
/// ```dart
/// const ctx = DsContext(
///   theme: DsThemeMode.dark,
///   density: DsDensityMode.comfortable,
///   motion: DsMotionMode.full,
///   accent: 'loch',
/// );
/// ```
@immutable
final class DsContext {
  /// Creates a context; [accent] is a key of `DsAccent` (`branding.accent_color`).
  const DsContext({
    required this.theme,
    required this.density,
    required this.motion,
    required this.accent,
  });

  /// Light or dark.
  final DsThemeMode theme;

  /// Density context.
  final DsDensityMode density;

  /// Motion context.
  final DsMotionMode motion;

  /// Accent key, for example `spruce` or `loch`; an unknown key renders as the default accent.
  final String accent;

  /// A copy with the given fields replaced.
  DsContext copyWith({
    DsThemeMode? theme,
    DsDensityMode? density,
    DsMotionMode? motion,
    String? accent,
  }) {
    return DsContext(
      theme: theme ?? this.theme,
      density: density ?? this.density,
      motion: motion ?? this.motion,
      accent: accent ?? this.accent,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is DsContext &&
        other.theme == theme &&
        other.density == density &&
        other.motion == motion &&
        other.accent == accent;
  }

  @override
  int get hashCode => Object.hash(theme, density, motion, accent);

  @override
  String toString() =>
      'DsContext(${theme.name}, ${density.name}, ${motion.name}, $accent)';
}
