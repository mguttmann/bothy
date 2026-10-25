// SPDX-License-Identifier: Apache-2.0

// The goldens of the phone screens (brief M5-02 Tests; 01 5.3): M1 Spaces, M2 Post list, M3
// History with the compact live box running, waiting for input and collapsed, M4 Large
// components and M7 Offline, in the density `touch`, light and dark, text scaling 1 and 2,
// German and English. One file per screen and theme keeps each file below repo.max_file_kb.

import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../../phone/support/phone_scenes.dart';

void main() {
  for (final MapEntry(key: name, value: scene) in phoneScenes.entries) {
    for (final theme in DsThemeMode.values) {
      goldenTest(
        '$name in ${theme.name}, text scale 1 and 2, de and en',
        fileName: 'phone_${name}_${theme.name}',
        pumpBeforeTest: dsPumpIcons,
        builder: () => dsGoldenMatrix(
          name: name,
          themes: [theme],
          densities: const [DsDensityMode.touch],
          textScales: const [1.0, 2.0],
          locales: const [Locale('de'), Locale('en')],
          builder: scene,
        ),
      );
    }
  }
}
