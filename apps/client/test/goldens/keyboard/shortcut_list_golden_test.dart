// SPDX-License-Identifier: Apache-2.0

// The goldens of the list of shortcuts (brief M4-24 Tests; 01 7): the list on macOS and on
// Windows, German and English, light and dark.

import 'package:alchemist/alchemist.dart';
import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/shortcut_list.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

Widget framed(ShortcutPlatform platform) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 640,
    height: 1200,
    child: ShortcutListView(platform: platform, onClose: () {}),
  ),
);

void main() {
  for (final platform in ShortcutPlatform.values) {
    goldenTest(
      'the list of shortcuts on ${platform.name}, light and dark, de and en',
      fileName: 'shortcut_list_${platform.name}',
      builder: () => dsGoldenMatrix(
        name: 'shortcut_list_${platform.name}',
        builder: () => framed(platform),
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
