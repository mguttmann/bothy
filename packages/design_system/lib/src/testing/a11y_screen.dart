// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../../design_system.dart';
import 'semantics_audit.dart';

/// The desktop window of the accessibility tests (logical pixels at one device pixel per
/// logical pixel).
const Size dsA11yWindow = Size(1280, 900);

/// [child] in a desktop app for an accessibility test: the delegates of `package:l10n` in
/// [locale], a [MediaQuery] with `TextScaler.linear(textScale)` and the design theme of
/// [theme] and [density] with reduced motion, so no frame of a transition is measured.
///
/// [scroll] puts the child into a vertical scroll view, for scenes that are as tall as their
/// content; pass false for a screen that scrolls itself and needs a bounded height.
Widget dsA11yHost(
  Widget child, {
  Locale locale = const Locale('en'),
  double textScale = 1,
  DsThemeMode theme = DsThemeMode.light,
  DsDensityMode density = DsDensityMode.comfortable,
  bool scroll = true,
}) {
  final ds = DsContext(
    theme: theme,
    density: density,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  return MaterialApp(
    locale: locale,
    theme: buildDsTheme(ds),
    debugShowCheckedModeBanner: false,
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    builder: (context, app) => MediaQuery(
      data: MediaQuery.of(context)
          .copyWith(textScaler: TextScaler.linear(textScale)),
      child: DsThemeScope(dsContext: ds, child: app!),
    ),
    home: Scaffold(body: scroll ? SingleChildScrollView(child: child) : child),
  );
}

/// Registers the accessibility tests of one desktop screen (01 8; brief M4-32 Design note 1).
///
/// Three tests, named for the checklist `docs/quality/accessibility-checklist.md`:
///
/// - `<screen>: roles, labels and states for screen readers`: the screen in light and English
///   has no finding of [dsSemanticsProblems], then [semantics] asserts what the screen itself
///   must say (labels, role flags, a live box container with its summary, state labels).
/// - `<screen>: contrast, labelled targets and target size in light and dark`:
///   [dsExpectAccessible] in both themes (text contrast on macOS only, D-M4-07-10).
/// - `<screen> at text scale 2 in de and en: no overflow, every control reachable`: an
///   overflow fails the test (Flutter reports it as an exception), and every finder of
///   [reachable] still finds a widget that a pointer can hit (WCAG 1.4.4).
///
/// [build] creates the screen with its fixtures; [open] runs after the first settled frame (to
/// open a menu, for example); [window] is the size of the test window; [scroll] is passed to
/// [dsA11yHost].
///
/// ```dart
/// dsAccessibilityTests(
///   'the sidebar',
///   build: sidebarScene,
///   semantics: (tester, l) => expect(find.bySemanticsLabel('general'), findsOneWidget),
///   reachable: (l) => [find.byKey(const ValueKey('sidebar.settings'))],
/// );
/// ```
void dsAccessibilityTests(
  String screen, {
  required Widget Function() build,
  Future<void> Function(WidgetTester tester)? open,
  FutureOr<void> Function(WidgetTester tester, AppLocalizations l)? semantics,
  List<Finder> Function(AppLocalizations l)? reachable,
  Size window = dsA11yWindow,
  bool scroll = true,
}) {
  Future<void> pump(
    WidgetTester tester, {
    Locale locale = const Locale('en'),
    double textScale = 1,
    DsThemeMode theme = DsThemeMode.light,
  }) async {
    tester.view
      ..physicalSize = window
      ..devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      dsA11yHost(
        build(),
        locale: locale,
        textScale: textScale,
        theme: theme,
        scroll: scroll,
      ),
    );
    await _settleIcons(tester);
    if (open != null) await open(tester);
  }

  testWidgets('$screen: roles, labels and states for screen readers', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    await pump(tester);
    final problems = dsSemanticsProblems(tester);
    expect(problems, isEmpty, reason: problems.join('\n'));
    await semantics?.call(tester, lookupAppLocalizations(const Locale('en')));
    handle.dispose();
  });

  testWidgets(
    '$screen: contrast, labelled targets and target size in light and dark',
    (tester) async {
      final handle = tester.ensureSemantics();
      for (final theme in DsThemeMode.values) {
        await pump(tester, theme: theme);
        await dsExpectAccessible(tester);
      }
      handle.dispose();
    },
  );

  testWidgets(
    '$screen at text scale 2 in de and en: no overflow, every control reachable',
    (tester) async {
      for (final locale in const [Locale('de'), Locale('en')]) {
        await pump(tester, locale: locale, textScale: 2);
        final error = tester.takeException();
        expect(
          error,
          isNull,
          reason: error is FlutterError ? error.toStringDeep() : '$error',
        );
        for (final finder
            in reachable?.call(lookupAppLocalizations(locale)) ??
                const <Finder>[]) {
          expect(finder, findsWidgets, reason: '$finder in $locale');
          await tester.ensureVisible(finder.first);
          await tester.pump();
          expect(
            finder.first.hitTestable(),
            findsOneWidget,
            reason: '$finder in $locale',
          );
        }
      }
    },
  );
}

/// Pumps until every icon asset has loaded (the loads need real time, as in `dsPumpIcons`) and
/// the frame has settled; the patience of the repository's test kits is the only bound.
Future<void> _settleIcons(WidgetTester tester) async {
  await tester.pump();
  final watch = Stopwatch()..start();
  while (find.byType(DsIconPlaceholder).evaluate().isNotEmpty) {
    if (watch.elapsed > const Duration(minutes: 3)) {
      throw StateError('icon assets still loading');
    }
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump();
  }
  await tester.pumpAndSettle();
}
