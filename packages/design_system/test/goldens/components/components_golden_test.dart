// SPDX-License-Identifier: Apache-2.0

// The golden matrix of the core components (docs/spec/15-client-architektur.md 20, brief
// M4-07 Design note 8): one sheet per component with every state of its table, in light and
// dark, in comfortable, compact and touch, at text scale 1 and 2, in German and English. One
// file per sheet, density and theme keeps each file below repo.max_file_kb while no dimension
// is dropped (docs/DECISIONS.md D-M4-07-5).

import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import 'sheets.dart';

void main() {
  for (final MapEntry(key: name, value: sheet) in sheets.entries) {
    for (final density in DsDensityMode.values) {
      for (final theme in DsThemeMode.values) {
        goldenTest(
          '$name, ${theme.name}, ${density.name}, text scale 1 and 2, de and en',
          fileName: '${name}_${density.name}_${theme.name}',
          pumpBeforeTest: dsPumpIcons,
          builder: () => dsGoldenMatrix(
            name: name,
            builder: sheet,
            themes: [theme],
            densities: [density],
            textScales: const [1.0, 2.0],
            locales: const [Locale('de'), Locale('en')],
          ),
        );
      }
    }
  }
}
