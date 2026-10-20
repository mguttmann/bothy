// SPDX-License-Identifier: Apache-2.0

// Widget test helpers of terminal_ui: the app frame with the design-system theme and the
// localizations, and a pump loop for the asynchronous crypto of the controller.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

/// Pumps [child] in a MaterialApp with the theme of the design system and English texts.
Future<void> pumpApp(
  WidgetTester tester,
  Widget child, {
  GlobalKey<NavigatorState>? navigatorKey,
  Locale locale = const Locale('en'),
  Size size = const Size(1000, 700),
}) async {
  tester.view
    ..physicalSize = size
    ..devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  const ds = DsContext(
    theme: DsThemeMode.light,
    density: DsDensityMode.comfortable,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  await tester.pumpWidget(
    MaterialApp(
      navigatorKey: navigatorKey,
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      theme: buildDsTheme(ds),
      builder: (context, app) => DsThemeScope(dsContext: ds, child: app!),
      home: Scaffold(body: child),
    ),
  );
}

/// Pumps frames until [condition] holds; the controller's crypto completes between pumps.
Future<void> pumpUntil(
  WidgetTester tester,
  bool Function() condition, {
  int maxPumps = 500,
}) async {
  for (var i = 0; i < maxPumps; i++) {
    if (condition()) return;
    await tester.pump(const Duration(milliseconds: 1));
  }
  throw StateError('condition never held');
}
