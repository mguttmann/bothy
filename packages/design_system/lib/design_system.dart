// SPDX-License-Identifier: Apache-2.0

/// Design tokens (generated), theme, components, icons and fonts of the apps.
///
/// Colors, type, spacing and motion come only from the generated token classes
/// (ADR 0030): `DsColors`, `DsAccent`, `DsElevation`, `DsDensity` and `DsMotion` as theme
/// extensions, `DsSpace`, `DsRadius`, `DsBorderWidth`, `DsLayout`, `DsSize`, `DsIconSize`,
/// `DsFonts` and `DsType` as constants. [buildDsTheme] turns a [DsContext] into a
/// `ThemeData`, [resolveDsContext] derives the context from the personal settings, and
/// [DsThemeScope] applies it below a widget.
library;

export 'src/generated/design_tokens.g.dart';
export 'src/theme/ds_context.dart';
export 'src/theme/ds_settings_resolver.dart';
export 'src/theme/ds_theme.dart';
export 'src/theme/ds_theme_scope.dart';
