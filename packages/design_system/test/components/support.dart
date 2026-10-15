// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

/// The theme inputs of a test.
DsContext testContext({
  DsThemeMode theme = DsThemeMode.light,
  DsDensityMode density = DsDensityMode.comfortable,
}) => DsContext(
  theme: theme,
  density: density,
  motion: DsMotionMode.reduced,
  accent: dsDefaultAccent,
);

/// Pumps [child] in an app with both locales, a [DsThemeScope] and a canvas background.
Future<void> pumpDs(
  WidgetTester tester,
  Widget child, {
  DsThemeMode theme = DsThemeMode.light,
  DsDensityMode density = DsDensityMode.comfortable,
  Locale locale = const Locale('en'),
  double width = 600,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: DsThemeScope(
        dsContext: testContext(theme: theme, density: density),
        child: Builder(
          builder: (context) => Material(
            color: context.dsColors.surfaceCanvas,
            child: Align(
              alignment: Alignment.topLeft,
              child: SizedBox(
                width: width,
                child: SingleChildScrollView(child: child),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

/// The English texts, for expectations.
Future<AppLocalizations> en() =>
    AppLocalizations.delegate.load(const Locale('en'));

/// The German texts, for expectations.
Future<AppLocalizations> de() =>
    AppLocalizations.delegate.load(const Locale('de'));

/// Every text of the [RichText] widgets below the root, joined.
String allText(WidgetTester tester) {
  return tester
      .widgetList<RichText>(find.byType(RichText))
      .map((t) => t.text.toPlainText())
      .join('\n');
}
