// SPDX-License-Identifier: Apache-2.0

// A host for screen widgets: languages, the design theme and a wide desktop window.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

/// [child] with the delegates of `package:l10n` in [locale] and the light design theme.
Widget hostScreen(Widget child, {Locale locale = const Locale('en')}) {
  const ds = DsContext(
    theme: DsThemeMode.light,
    density: DsDensityMode.comfortable,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  return MaterialApp(
    locale: locale,
    theme: buildDsTheme(ds),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: DsThemeScope(dsContext: ds, child: child),
  );
}

/// [router] with the delegates of `package:l10n` in [locale] and the light design theme.
Widget hostRouter(
  RouterConfig<Object> router, {
  Locale locale = const Locale('en'),
  DsDensityMode density = DsDensityMode.comfortable,
}) {
  final ds = DsContext(
    theme: DsThemeMode.light,
    density: density,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  return MaterialApp.router(
    locale: locale,
    theme: buildDsTheme(ds),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    routerConfig: router,
    builder: (context, child) =>
        DsThemeScope(dsContext: ds, child: child ?? const SizedBox()),
  );
}

/// A desktop-sized window for the test.
void desktopWindow(WidgetTester tester) {
  tester.view
    ..physicalSize = const Size(1440, 1000)
    ..devicePixelRatio = 1;
  addTearDown(tester.view.reset);
}

/// The English texts.
final AppLocalizations en = lookupAppLocalizations(const Locale('en'));
