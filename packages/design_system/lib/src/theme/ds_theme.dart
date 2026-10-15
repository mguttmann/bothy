// SPDX-License-Identifier: Apache-2.0

/// The theme builder and the token accessors on [BuildContext].
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import 'ds_context.dart';

/// The package that bundles the fonts (`pubspec.yaml`, section `flutter.fonts`).
///
/// Flutter registers the fonts of a dependency as `packages/<package>/<family>`; inside this
/// package (its own tests) they keep the plain family name.
const String _fontPackage = 'design_system';

/// A type role ready for rendering: [role] with the bundled font reachable from every app.
///
/// `DsType` names the plain family (`Atkinson Hyperlegible Next`), which resolves only in this
/// package. An app that depends on the package reaches the same font as
/// `packages/design_system/<family>`, so that name joins the front of the fallback list. The
/// weight also drives the `wght` axis of the variable font files.
///
/// Returns a copy of [role]; size and color stay as they are, so `MediaQuery.textScaler`
/// scales the result as usual.
///
/// ```dart
/// Text('Alex', style: dsTextStyle(DsType.label));
/// ```
TextStyle dsTextStyle(TextStyle role) {
  final family = role.fontFamily;
  final weight = role.fontWeight;
  return role.copyWith(
    fontFamilyFallback: [
      if (family != null) 'packages/$_fontPackage/$family',
      ...?role.fontFamilyFallback,
    ],
    fontVariations: [
      if (weight != null) FontVariation.weight(weight.value.toDouble()),
    ],
  );
}

/// Builds the Material theme of a [DsContext].
///
/// * `useMaterial3`, `brightness` and a [ColorScheme] mapped from `DsColors` and `DsAccent`
///   (the full mapping is in the package README).
/// * `extensions`: `DsColors`, `DsAccent` (accent and brightness), `DsElevation`,
///   `DsDensity` and `DsMotion` of the context; widgets read them through
///   [DsThemeContext].
/// * `textTheme` from the `DsType` roles through [dsTextStyle], colored with
///   `color.text.primary`; `visualDensity` from the density; no page transition under
///   reduced motion ([DsNoTransitionsBuilder]); the focus ring `color.focus.ring` with
///   `border-width.focus` on buttons and inputs (components: focus is always visible).
///
/// An unknown [DsContext.accent] renders as the default accent.
///
/// ```dart
/// MaterialApp(theme: buildDsTheme(ctx), home: const Placeholder());
/// ```
ThemeData buildDsTheme(DsContext context) {
  final dark = context.theme == DsThemeMode.dark;
  final brightness = dark ? Brightness.dark : Brightness.light;
  final colors = dark ? DsColors.dark : DsColors.light;
  final accent = DsAccent.of(context.accent, brightness);
  final elevation = dark ? DsElevation.dark : DsElevation.light;
  final (density, visualDensity) = switch (context.density) {
    DsDensityMode.comfortable => (
      DsDensity.comfortable,
      VisualDensity.comfortable,
    ),
    DsDensityMode.compact => (DsDensity.compact, VisualDensity.compact),
    DsDensityMode.touch => (DsDensity.touch, VisualDensity.standard),
  };
  final motion = context.motion == DsMotionMode.reduced
      ? DsMotion.reduced
      : DsMotion.full;

  final scheme = ColorScheme(
    brightness: brightness,
    primary: accent.accentBg,
    onPrimary: colors.textOnAccent,
    primaryContainer: accent.accentSubtle,
    onPrimaryContainer: accent.accentFg,
    secondary: accent.accentFg,
    onSecondary: colors.surfaceCanvas,
    secondaryContainer: colors.surfaceSelected,
    onSecondaryContainer: colors.textPrimary,
    tertiary: colors.infoBg,
    onTertiary: colors.infoOnBg,
    error: colors.dangerBg,
    onError: colors.dangerOnBg,
    errorContainer: colors.dangerSubtle,
    onErrorContainer: colors.dangerFg,
    surface: colors.surfaceCanvas,
    onSurface: colors.textPrimary,
    onSurfaceVariant: colors.textSecondary,
    surfaceContainerLowest: colors.surfaceSunken,
    surfaceContainerLow: colors.surfaceList,
    surfaceContainer: colors.surfaceRaised,
    surfaceContainerHigh: colors.surfaceRaised,
    surfaceContainerHighest: colors.surfaceOverlay,
    outline: colors.borderDefault,
    outlineVariant: colors.borderSubtle,
    shadow: colors.shadow,
    scrim: colors.scrim,
    inverseSurface: colors.textPrimary,
    onInverseSurface: colors.surfaceCanvas,
    inversePrimary: accent.accentSubtle,
    surfaceTint: colors.surfaceCanvas,
  );

  final focusSide = BorderSide(
    color: accent.focusRing,
    width: DsBorderWidth.focus,
  );
  BorderSide? focusOnly(Set<WidgetState> states) =>
      states.contains(WidgetState.focused) ? focusSide : null;
  BorderSide outlinedSide(Set<WidgetState> states) =>
      focusOnly(states) ??
      BorderSide(color: colors.borderControl, width: DsBorderWidth.hairline);
  final focusStyle = ButtonStyle(
    side: WidgetStateProperty.resolveWith(focusOnly),
  );

  final text = TextTheme(
    displayLarge: dsTextStyle(DsType.display),
    displayMedium: dsTextStyle(DsType.display),
    displaySmall: dsTextStyle(DsType.display),
    headlineLarge: dsTextStyle(DsType.title),
    headlineMedium: dsTextStyle(DsType.title),
    headlineSmall: dsTextStyle(DsType.title),
    titleLarge: dsTextStyle(DsType.title),
    titleMedium: dsTextStyle(DsType.heading),
    titleSmall: dsTextStyle(DsType.label),
    bodyLarge: dsTextStyle(DsType.body),
    bodyMedium: dsTextStyle(DsType.body),
    bodySmall: dsTextStyle(DsType.bodySm),
    labelLarge: dsTextStyle(DsType.label),
    labelMedium: dsTextStyle(DsType.label),
    labelSmall: dsTextStyle(DsType.caption),
  ).apply(bodyColor: colors.textPrimary, displayColor: colors.textPrimary);

  final transitions = context.motion == DsMotionMode.reduced
      ? PageTransitionsTheme(
          builders: {
            for (final platform in TargetPlatform.values)
              platform: const DsNoTransitionsBuilder(),
          },
        )
      : const PageTransitionsTheme();

  return ThemeData(
    useMaterial3: true,
    brightness: brightness,
    colorScheme: scheme,
    scaffoldBackgroundColor: colors.surfaceCanvas,
    canvasColor: colors.surfaceCanvas,
    dividerColor: colors.borderSubtle,
    focusColor: accent.focusRing,
    hoverColor: colors.surfaceHover,
    fontFamily: DsFonts.familySans.first,
    fontFamilyFallback: DsFonts.familySans.skip(1).toList(),
    textTheme: text,
    visualDensity: visualDensity,
    pageTransitionsTheme: transitions,
    filledButtonTheme: FilledButtonThemeData(style: focusStyle),
    elevatedButtonTheme: ElevatedButtonThemeData(style: focusStyle),
    textButtonTheme: TextButtonThemeData(style: focusStyle),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: ButtonStyle(side: WidgetStateProperty.resolveWith(outlinedSide)),
    ),
    inputDecorationTheme: InputDecorationThemeData(
      focusedBorder: OutlineInputBorder(borderSide: focusSide),
    ),
    extensions: [colors, accent, elevation, density, motion],
  );
}

/// A page transition without motion, for reduced motion: the new route appears at once.
class DsNoTransitionsBuilder extends PageTransitionsBuilder {
  /// Creates the builder.
  const DsNoTransitionsBuilder();

  /// Returns [child] unchanged; the route's animations are ignored.
  @override
  Widget buildTransitions<T>(
    PageRoute<T> route,
    BuildContext context,
    Animation<double> animation,
    Animation<double> secondaryAnimation,
    Widget child,
  ) {
    return child;
  }
}

/// Token accessors on [BuildContext]: the theme extensions of [buildDsTheme].
///
/// Each getter throws a [StateError] naming the missing extension when the ambient theme was
/// not built with [buildDsTheme] (for example a plain `ThemeData()`), so a missing scope fails
/// loudly instead of rendering default Material colors.
///
/// ```dart
/// final color = context.dsColors.textPrimary;
/// ```
extension DsThemeContext on BuildContext {
  /// Semantic colors of the current theme.
  DsColors get dsColors => _extension<DsColors>();

  /// Accent roles of the instance accent in the current theme.
  DsAccent get dsAccent => _extension<DsAccent>();

  /// Density values of the current density.
  DsDensity get dsDensity => _extension<DsDensity>();

  /// Durations and curves of the current motion context.
  DsMotion get dsMotion => _extension<DsMotion>();

  /// Shadows of the current theme.
  DsElevation get dsElevation => _extension<DsElevation>();

  T _extension<T extends ThemeExtension<T>>() {
    final value = Theme.of(this).extension<T>();
    if (value == null) {
      throw StateError(
        '$T missing from the theme: build it with buildDsTheme (or use DsThemeScope)',
      );
    }
    return value;
  }
}
