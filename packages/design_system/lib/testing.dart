// SPDX-License-Identifier: Apache-2.0

/// Golden test helpers shared by every member that renders design-system widgets.
///
/// Import it only from tests: `import 'package:design_system/testing.dart';`. The golden
/// setup itself (fonts, `AlchemistConfig`) lives in each member's `test/flutter_test_config.dart`
/// (docs/DECISIONS.md D-M4-04-5).
library;

import 'package:alchemist/alchemist.dart';
import 'package:flutter/material.dart';

import 'design_system.dart';

/// The font family alchemist sets on the ambient theme for CI goldens (obscured text).
const String _obscuredFontFamily = 'Ahem';

/// Builds one alchemist [GoldenTestGroup] with a scenario per combination of [themes],
/// [densities], [textScales] and [locales].
///
/// Each scenario wraps `builder()` in the given [Locale], a [MediaQuery] with
/// `TextScaler.linear(scale)` and a [DsThemeScope] (reduced motion, so a golden never
/// captures a frame of a transition; default accent) on a `color.surface.canvas` background.
/// The scenario name is `<theme> <density> x<scale> <locale>`.
///
/// Parameters: [name] is the group's label in failure messages; [builder] creates a fresh
/// widget per scenario. Returns the group to pass to `goldenTest(builder: ...)`.
///
/// For CI goldens alchemist renders text as blocks with the Ahem font on its own theme; the
/// scope then keeps Ahem for the text theme and the default text style, so the CI image does
/// not depend on font rendering of the host.
///
/// ```dart
/// goldenTest('sample page', fileName: 'sample_page',
///     builder: () => dsGoldenMatrix(name: 'sample page', builder: SamplePage.new));
/// ```
GoldenTestGroup dsGoldenMatrix({
  required String name,
  required Widget Function() builder,
  Iterable<DsThemeMode> themes = DsThemeMode.values,
  Iterable<DsDensityMode> densities = const [DsDensityMode.comfortable],
  Iterable<double> textScales = const [1.0],
  Iterable<Locale> locales = const [Locale('en')],
}) {
  return GoldenTestGroup(
    columns: textScales.length,
    children: [
      for (final theme in themes)
        for (final density in densities)
          for (final locale in locales)
            for (final scale in textScales)
              GoldenTestScenario(
                name:
                    '${theme.name} ${density.name} x$scale ${locale.toLanguageTag()}',
                child: _DsGoldenCell(
                  dsContext: DsContext(
                    theme: theme,
                    density: density,
                    motion: DsMotionMode.reduced,
                    accent: dsDefaultAccent,
                  ),
                  locale: locale,
                  textScale: scale,
                  child: builder(),
                ),
              ),
    ],
  );
}

/// One scenario of [dsGoldenMatrix]: locale, text scale, theme scope and background.
class _DsGoldenCell extends StatelessWidget {
  const _DsGoldenCell({
    required this.dsContext,
    required this.locale,
    required this.textScale,
    required this.child,
  });

  final DsContext dsContext;
  final Locale locale;
  final double textScale;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final obscured =
        Theme.of(context).textTheme.bodyMedium?.fontFamily ==
        _obscuredFontFamily;
    return Localizations.override(
      context: context,
      locale: locale,
      child: MediaQuery(
        data: MediaQuery.of(context)
            .copyWith(textScaler: TextScaler.linear(textScale)),
        child: DsThemeScope(
          dsContext: dsContext,
          child: Builder(
            builder: (context) {
              Widget body = ColoredBox(
                color: context.dsColors.surfaceCanvas,
                child: child,
              );
              if (obscured) {
                final theme = Theme.of(context);
                body = Theme(
                  data: theme.copyWith(
                    textTheme: theme.textTheme.apply(
                      fontFamily: _obscuredFontFamily,
                    ),
                  ),
                  child: DefaultTextStyle.merge(
                    style: const TextStyle(fontFamily: _obscuredFontFamily),
                    child: body,
                  ),
                );
              }
              return body;
            },
          ),
        ),
      ),
    );
  }
}
