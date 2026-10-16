// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

const _base = DsContext(
  theme: DsThemeMode.light,
  density: DsDensityMode.comfortable,
  motion: DsMotionMode.full,
  accent: 'spruce',
);

/// Records what a widget below the scope reads from the theme.
class _Probe extends StatelessWidget {
  const _Probe(this.onBuild);

  final void Function(BuildContext context) onBuild;

  @override
  Widget build(BuildContext context) {
    onBuild(context);
    return const SizedBox.shrink();
  }
}

Future<BuildContext> _pump(WidgetTester tester, DsContext ctx) async {
  late BuildContext captured;
  await tester.pumpWidget(
    DsThemeScope(dsContext: ctx, child: _Probe((c) => captured = c)),
  );
  await tester.pumpAndSettle();
  return captured;
}

void main() {
  testWidgets(
    'light then dark changes the text color to the token value of each theme',
    (tester) async {
      var c = await _pump(tester, _base);
      expect(c.dsColors.textPrimary, DsColors.light.textPrimary);
      expect(Theme.of(c).brightness, Brightness.light);
      c = await _pump(tester, _base.copyWith(theme: DsThemeMode.dark));
      expect(c.dsColors.textPrimary, DsColors.dark.textPrimary);
      expect(Theme.of(c).brightness, Brightness.dark);
      expect(c.dsElevation, DsElevation.dark);
    },
  );

  testWidgets('the three densities give the three token heights', (
    tester,
  ) async {
    for (final (mode, tokens) in [
      (DsDensityMode.comfortable, DsDensity.comfortable),
      (DsDensityMode.compact, DsDensity.compact),
      (DsDensityMode.touch, DsDensity.touch),
    ]) {
      final c = await _pump(tester, _base.copyWith(density: mode));
      expect(
        c.dsDensity.controlHeight,
        tokens.controlHeight,
        reason: mode.name,
      );
    }
  });

  testWidgets(
    'reduced motion gives a zero AnimatedTheme duration, full the fade token',
    (tester) async {
      await _pump(tester, _base.copyWith(motion: DsMotionMode.reduced));
      expect(
        tester.widget<AnimatedTheme>(find.byType(AnimatedTheme)).duration,
        Duration.zero,
      );
      expect(tester.element(find.byType(_Probe)).dsMotion, DsMotion.reduced);
      await _pump(tester, _base);
      expect(
        tester.widget<AnimatedTheme>(find.byType(AnimatedTheme)).duration,
        DsMotion.full.durationFade,
      );
      expect(tester.element(find.byType(_Probe)).dsMotion, DsMotion.full);
    },
  );

  testWidgets('reduced motion switches the theme without an animation frame', (
    tester,
  ) async {
    await _pump(tester, _base.copyWith(motion: DsMotionMode.reduced));
    await tester.pumpWidget(
      DsThemeScope(
        dsContext: _base.copyWith(
          motion: DsMotionMode.reduced,
          theme: DsThemeMode.dark,
        ),
        child: const _Probe(_ignore),
      ),
    );
    expect(
      tester.element(find.byType(_Probe)).dsColors.textPrimary,
      DsColors.dark.textPrimary,
    );
    expect(tester.hasRunningAnimations, isFalse);
  });

  testWidgets('reduced motion removes page transitions', (tester) async {
    final reduced = buildDsTheme(_base.copyWith(motion: DsMotionMode.reduced));
    final full = buildDsTheme(_base);
    for (final platform in TargetPlatform.values) {
      expect(
        reduced.pageTransitionsTheme.builders[platform],
        isA<DsNoTransitionsBuilder>(),
      );
      expect(
        full.pageTransitionsTheme.builders[platform],
        isNot(isA<DsNoTransitionsBuilder>()),
      );
    }
  });

  testWidgets('the loch accent changes dsAccent to the loch variant', (
    tester,
  ) async {
    var c = await _pump(tester, _base.copyWith(accent: 'loch'));
    expect(c.dsAccent, DsAccent.lightLoch);
    expect(Theme.of(c).colorScheme.primary, DsAccent.lightLoch.accentBg);
    c = await _pump(
      tester,
      _base.copyWith(accent: 'loch', theme: DsThemeMode.dark),
    );
    expect(c.dsAccent, DsAccent.darkLoch);
  });

  testWidgets(
    'a widget under a plain ThemeData gets a StateError naming the extension',
    (tester) async {
      late BuildContext c;
      await tester.pumpWidget(
        Theme(data: ThemeData(), child: _Probe((ctx) => c = ctx)),
      );
      for (final (name, read) in <(String, Object Function())>[
        ('DsColors', () => c.dsColors),
        ('DsAccent', () => c.dsAccent),
        ('DsDensity', () => c.dsDensity),
        ('DsMotion', () => c.dsMotion),
        ('DsElevation', () => c.dsElevation),
      ]) {
        expect(
          read,
          throwsA(
            isA<StateError>().having(
              (e) => e.message,
              'message',
              contains(name),
            ),
          ),
        );
      }
    },
  );

  group('buildDsTheme', () {
    test('maps the color scheme from the tokens', () {
      final theme = buildDsTheme(_base);
      final scheme = theme.colorScheme;
      expect(theme.useMaterial3, isTrue);
      expect(scheme.brightness, Brightness.light);
      expect(scheme.primary, DsAccent.lightSpruce.accentBg);
      expect(scheme.onPrimary, DsColors.light.textOnAccent);
      expect(scheme.surface, DsColors.light.surfaceCanvas);
      expect(scheme.onSurface, DsColors.light.textPrimary);
      expect(scheme.error, DsColors.light.dangerBg);
      expect(scheme.outline, DsColors.light.borderDefault);
      expect(theme.scaffoldBackgroundColor, DsColors.light.surfaceCanvas);
    });

    test('carries the five extensions of the context', () {
      final theme = buildDsTheme(
        _base.copyWith(
          theme: DsThemeMode.dark,
          density: DsDensityMode.touch,
          motion: DsMotionMode.reduced,
        ),
      );
      expect(theme.extension<DsColors>(), DsColors.dark);
      expect(theme.extension<DsAccent>(), DsAccent.darkSpruce);
      expect(theme.extension<DsElevation>(), DsElevation.dark);
      expect(theme.extension<DsDensity>(), DsDensity.touch);
      expect(theme.extension<DsMotion>(), DsMotion.reduced);
    });

    test('takes the text theme from the type roles', () {
      final text = buildDsTheme(_base).textTheme;
      expect(text.bodyMedium!.fontFamily, DsType.body.fontFamily);
      expect(text.bodyMedium!.fontSize, DsType.body.fontSize);
      expect(text.titleLarge!.fontSize, DsType.title.fontSize);
      expect(text.displayLarge!.fontSize, DsType.display.fontSize);
      expect(text.bodyMedium!.color, DsColors.light.textPrimary);
    });

    test('density sets the visual density', () {
      expect(
        buildDsTheme(_base.copyWith(density: DsDensityMode.compact))
            .visualDensity,
        VisualDensity.compact,
      );
      expect(buildDsTheme(_base).visualDensity, VisualDensity.comfortable);
      expect(
        buildDsTheme(_base.copyWith(density: DsDensityMode.touch))
            .visualDensity,
        VisualDensity.standard,
      );
    });

    test('a focused control shows the focus ring token', () {
      final theme = buildDsTheme(_base);
      final side = theme.filledButtonTheme.style!.side!.resolve({
        WidgetState.focused,
      });
      expect(side!.color, DsAccent.lightSpruce.focusRing);
      expect(side.width, DsBorderWidth.focus);
      expect(theme.filledButtonTheme.style!.side!.resolve({}), isNull);
      final outlined = theme.outlinedButtonTheme.style!.side!.resolve({});
      expect(outlined!.color, DsColors.light.borderControl);
      final input = theme.inputDecorationTheme.focusedBorder!.borderSide;
      expect(input.color, DsAccent.lightSpruce.focusRing);
      expect(input.width, DsBorderWidth.focus);
      expect(theme.focusColor, DsAccent.lightSpruce.focusRing);
    });
  });
}

void _ignore(BuildContext context) {}
