// SPDX-License-Identifier: Apache-2.0

import 'dart:io';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The fixtures of the package, read from `fixtures/` (tests run in the package root).
final fixtures = VizFixtures((name) => File('fixtures/$name').readAsString());

/// Every fixture file name.
List<String> fixtureNames() =>
    (Directory('fixtures')
            .listSync()
            .whereType<File>()
            .map((f) => f.uri.pathSegments.last))
        .where((n) => n.endsWith('.json'))
        .toList()
      ..sort();

/// The names of the test world (07 10) for the IDs of the fixtures.
const names = VizNames(
  agents: {
    '0192f4a0-7b1c-7000-8a00-000000000201': 'support-agent',
    '0192f4a0-7b1c-7000-8a00-000000000202': 'dev-runner',
    '0192f4a0-7b1c-7000-8a00-000000000203': 'Texter',
    '0192f4a0-7b1c-7000-8a00-000000000204': 'Assistent',
  },
  spaces: {
    '0192f4a0-7b1c-7000-8a00-000000000001': 'Support',
    '0192f4a0-7b1c-7000-8a00-000000000002': 'Engineering',
  },
  channels: {
    '0192f4a0-7b1c-7000-8a00-000000000101': '#tickets',
    '0192f4a0-7b1c-7000-8a00-000000000102': '#backend',
    '0192f4a0-7b1c-7000-8a00-000000000103': '#release',
  },
);

/// [child] in a themed, localized app of [locale].
Widget app(
  Widget child, {
  Locale locale = const Locale('en'),
  DsThemeMode theme = DsThemeMode.light,
}) => MaterialApp(
  locale: locale,
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  theme: buildDsTheme(
    DsContext(
      theme: theme,
      density: DsDensityMode.comfortable,
      motion: DsMotionMode.reduced,
      accent: dsDefaultAccent,
    ),
  ),
  home: Scaffold(body: SingleChildScrollView(child: child)),
);
