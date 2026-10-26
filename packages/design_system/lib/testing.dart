// SPDX-License-Identifier: Apache-2.0

/// Golden test helpers shared by every member that renders design-system widgets.
///
/// Import it only from tests: `import 'package:design_system/testing.dart';`. The golden
/// setup itself (fonts, `AlchemistConfig`) lives in each member's `test/flutter_test_config.dart`
/// (docs/DECISIONS.md D-M4-04-5).
library;

import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:alchemist/alchemist.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'design_system.dart';

export 'src/testing/a11y_screen.dart';
export 'src/testing/phone_a11y_screen.dart';
export 'src/testing/semantics_audit.dart';

/// Largest fraction of differing pixels a CI golden still passes with (0.02 %).
///
/// CI goldens recorded on macOS differ on the Linux runner in a few anti-aliased edge pixels
/// (170 of 2.28 million in the sample page), so an exact match fails there. The bound stays
/// below one full-width line of one pixel in that image, so a missing border or divider still
/// fails (docs/DECISIONS.md D-M4-04-6). [DsGoldenFileComparator] applies it, counting only
/// pixels beyond [dsCiGoldenChannelTolerance] (PL/5).
const double dsCiGoldenDiffThreshold = 0.0002;

/// Largest difference of one color channel (0 to 255) at which two pixels still count as
/// equal in [DsGoldenFileComparator].
///
/// Hosts rasterize anti-aliased edges, icons and borders slightly differently: CI goldens
/// recorded on macOS differ on the Linux runner in many pixels, nearly all by a few steps of
/// one channel. The bound stays below the closest token pairs of the dark theme (a subtle
/// border on a raised surface differs by 9 steps); a background that differs from its
/// neighbour by less still fails the exact platform golden on macOS (docs/DECISIONS.md PL/5).
const int dsCiGoldenChannelTolerance = 8;

/// Golden comparator of the members with design-system goldens. In a CI golden (a file under
/// `ci/`, the path of D-M4-04-5) a pixel differs only when one channel differs by more than
/// [dsCiGoldenChannelTolerance], and the image passes while at most [dsCiGoldenDiffThreshold]
/// of its pixels differ; a platform golden must match exactly, as it is recorded and compared
/// on macOS only. A different size always fails.
///
/// Install it in `test/flutter_test_config.dart` and pass no `diffThreshold` to alchemist,
/// which then keeps this comparator (docs/DECISIONS.md PL/5).
///
/// ```dart
/// goldenFileComparator = DsGoldenFileComparator(
///     (goldenFileComparator as LocalFileComparator).basedir.resolve('test.dart'));
/// ```
class DsGoldenFileComparator extends LocalFileComparator {
  /// Compares against the golden files next to [testFile].
  DsGoldenFileComparator(super.testFile);

  @override
  Future<bool> compare(Uint8List imageBytes, Uri golden) async {
    final goldenBytes = await getGoldenBytes(golden);
    final ci = golden.pathSegments.first == 'ci';
    if (ci && await _withinTolerance(imageBytes, goldenBytes)) {
      return true;
    }
    final result = await GoldenFileComparator.compareLists(
      imageBytes,
      goldenBytes,
    );
    if (result.passed) {
      return true;
    }
    await generateFailureOutput(result, golden, basedir);
    return false;
  }

  static Future<bool> _withinTolerance(Uint8List a, List<int> b) async {
    final first = await _decode(a);
    final second = await _decode(Uint8List.fromList(b));
    try {
      if (first.width != second.width || first.height != second.height) {
        return false;
      }
      final pa = (await first.toByteData())!.buffer.asUint8List();
      final pb = (await second.toByteData())!.buffer.asUint8List();
      var differing = 0;
      for (var i = 0; i < pa.length; i += 4) {
        for (var c = 0; c < 4; c++) {
          if ((pa[i + c] - pb[i + c]).abs() > dsCiGoldenChannelTolerance) {
            differing++;
            break;
          }
        }
      }
      return differing <= dsCiGoldenDiffThreshold * first.width * first.height;
    } finally {
      first.dispose();
      second.dispose();
    }
  }

  static Future<ui.Image> _decode(Uint8List bytes) async {
    final codec = await ui.instantiateImageCodec(bytes);
    try {
      return (await codec.getNextFrame()).image;
    } finally {
      codec.dispose();
    }
  }
}

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

/// Pumps until every [DsIcon] below the root has loaded its asset, then settles.
///
/// Icon assets load asynchronously from the asset bundle, which a widget test only serves
/// outside the fake async zone; the loop gives the loads real time ([WidgetTester.runAsync])
/// and stops when no [DsIconPlaceholder] is left. Pass it as `pumpBeforeTest` of a golden
/// that shows icons. Throws a [StateError] when an icon is still pending after [timeout].
///
/// ```dart
/// goldenTest('icons', fileName: 'icons', pumpBeforeTest: dsPumpIcons, builder: ...);
/// ```
Future<void> dsPumpIcons(
  WidgetTester tester, {
  Duration timeout = const Duration(seconds: 10),
}) async {
  final watch = Stopwatch()..start();
  await tester.pump();
  while (find.byType(DsIconPlaceholder).evaluate().isNotEmpty) {
    if (watch.elapsed > timeout) {
      throw StateError('icon assets still loading after $timeout');
    }
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump();
  }
  await tester.pumpAndSettle();
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
    // The components read their texts from package:l10n; the golden app of alchemist has no
    // delegates of its own, so the cell brings them with the locale.
    return Localizations.override(
      context: context,
      locale: locale,
      delegates: AppLocalizations.localizationsDelegates,
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
