// SPDX-License-Identifier: Apache-2.0

// A host for phone screens in widget tests: the design theme in the density `touch` (01 11:
// fixed on the phone), the languages and a text scale.

import 'package:client/app/form_factor.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// [child] on the phone with [textScale], [locale] and [theme].
Widget hostPhone(
  Widget child, {
  double textScale = 1,
  Locale locale = const Locale('en'),
  DsThemeMode theme = DsThemeMode.light,
}) {
  final ds = DsContext(
    theme: theme,
    density: DsDensityMode.touch,
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
      child: FormFactorScope(
        formFactor: FormFactor.phone,
        child: DsThemeScope(dsContext: ds, child: app!),
      ),
    ),
    home: child,
  );
}
