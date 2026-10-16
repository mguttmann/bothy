// SPDX-License-Identifier: Apache-2.0

/// Resolves the personal settings and the system values to a [DsContext].
library;

import 'dart:ui' show Brightness;

import '../generated/design_tokens.g.dart';
import 'ds_context.dart';

/// The default accent of the instance setting `branding.accent_color` (docs/spec/01-product.md 12).
///
/// The generated `DsAccent.of` falls back to the same accent for an unknown key.
const String dsDefaultAccent = 'spruce';

// Values of the personal settings (docs/spec/01-product.md 11, `appearance.*`).
const String _light = 'light';
const String _dark = 'dark';
const String _compact = 'compact';
const String _on = 'on';
const String _off = 'off';

/// Resolves the personal settings `appearance.theme`, `appearance.density` and
/// `appearance.reduce_motion` and the instance accent into the inputs of the theme.
///
/// Parameters are the raw setting values (`null` when absent) and the system values of the
/// platform. Rules:
///
/// * [appearanceTheme] `light` or `dark` wins; `system`, absent or unknown follows
///   [platformBrightness].
/// * On a touch platform ([isTouchPlatform]) the density is always `touch`; on desktop
///   [appearanceDensity] `compact` gives `compact`, anything else `comfortable`.
/// * [appearanceReduceMotion] `on` gives `reduced`, `off` gives `full`; `system`, absent or
///   unknown follows [platformReduceMotion].
/// * [accentKey] must be a key of `DsAccent`; an unknown or absent key gives [dsDefaultAccent].
///
/// An unknown value never throws: the server rejects unknown values, but a newer server may add
/// one, and the app then shows the default of the setting (docs/spec/06-api.md 13).
///
/// Returns the resolved [DsContext]. Pure: no I/O and no widget state.
///
/// ```dart
/// final ctx = resolveDsContext(
///   appearanceTheme: 'system',
///   appearanceDensity: null,
///   appearanceReduceMotion: 'on',
///   platformBrightness: Brightness.dark,
///   platformReduceMotion: false,
///   isTouchPlatform: false,
///   accentKey: 'loch',
/// ); // dark, comfortable, reduced, loch
/// ```
DsContext resolveDsContext({
  required String? appearanceTheme,
  required String? appearanceDensity,
  required String? appearanceReduceMotion,
  required Brightness platformBrightness,
  required bool platformReduceMotion,
  required bool isTouchPlatform,
  required String? accentKey,
}) {
  final systemTheme = platformBrightness == Brightness.dark
      ? DsThemeMode.dark
      : DsThemeMode.light;
  final theme = switch (appearanceTheme) {
    _light => DsThemeMode.light,
    _dark => DsThemeMode.dark,
    _ => systemTheme, // system, absent or unknown
  };
  final density = isTouchPlatform
      ? DsDensityMode.touch
      : switch (appearanceDensity) {
          _compact => DsDensityMode.compact,
          _ => DsDensityMode.comfortable, // comfortable, absent or unknown
        };
  final systemMotion = platformReduceMotion
      ? DsMotionMode.reduced
      : DsMotionMode.full;
  final motion = switch (appearanceReduceMotion) {
    _on => DsMotionMode.reduced,
    _off => DsMotionMode.full,
    _ => systemMotion, // system, absent or unknown
  };
  return DsContext(
    theme: theme,
    density: density,
    motion: motion,
    accent: _knownAccent(accentKey),
  );
}

/// The key itself when `DsAccent` has a variant for it, else [dsDefaultAccent].
///
/// `DsAccent.of` maps an unknown key to the default variant, so a key whose light variant is
/// the default one is either the default key or unknown.
String _knownAccent(String? key) {
  if (key == null ||
      identical(DsAccent.of(key, Brightness.light), DsAccent.lightSpruce)) {
    return dsDefaultAccent;
  }
  return key;
}
