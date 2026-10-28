// SPDX-License-Identifier: Apache-2.0

// The goldens of M1 with the thread groups (brief M5-15 Tests; 01 6.8; D-SB-13): a text channel
// with a thread row, a forum with the draft row, posts in the row states and "All posts", a
// collapsed project forum group, and "+" on every forum row in the density `touch`; light and
// dark, text scaling 1 and 2, German and English, with the iOS and the Android target platform.
// One file per platform and theme keeps each file below repo.max_file_kb.

import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../../phone/support/phone_scenes.dart';

/// [child] with the target platform [platform], whatever the host of the test.
Widget on(TargetPlatform platform, Widget child) => Builder(
  builder: (context) => Theme(
    data: Theme.of(context).copyWith(platform: platform),
    child: child,
  ),
);

void main() {
  for (final (name, platform) in const [
    ('ios', TargetPlatform.iOS),
    ('android', TargetPlatform.android),
  ]) {
    for (final theme in DsThemeMode.values) {
      goldenTest(
        'M1 with threads on $name in ${theme.name}, text scale 1 and 2, de and en',
        fileName: 'm1_threads_${name}_${theme.name}',
        pumpBeforeTest: dsPumpIcons,
        builder: () => dsGoldenMatrix(
          name: 'm1_threads',
          themes: [theme],
          densities: const [DsDensityMode.touch],
          textScales: const [1.0, 2.0],
          locales: const [Locale('de'), Locale('en')],
          builder: () => on(platform, spacesThreadsScene()),
        ),
      );
    }
  }
}
