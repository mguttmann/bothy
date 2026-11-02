// SPDX-License-Identifier: Apache-2.0

// The focused composer (FIX-keyboard-dismiss-focus-ring): a text field with the focus shows the
// caret and a hairline in the theme's focus color around the field, not the thick ring of
// `border-width.focus` that the input decoration theme drew inside it (yellow in dark). The
// follow-up composer of a thread with a draft, on the phone (touch) and on the desktop
// (comfortable), in light and dark.

import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'sheets.dart';

Widget _focusedSheet() => Sheet([
  DsComposer(
    targetName: 'general',
    followupAgentName: 'Robin',
    maxLines: 4,
    enterSends: true,
    onSend: (_) {},
    onAttach: () {},
    onVoice: () {},
    controller: TextEditingController(text: 'Also check the logs'),
  ),
]);

Future<AsyncCallback?> _focus(WidgetTester tester) async {
  await tester.tap(find.byType(TextField));
  await tester.pump();
  return null;
}

void main() {
  for (final density in const [
    DsDensityMode.touch,
    DsDensityMode.comfortable,
  ]) {
    for (final theme in DsThemeMode.values) {
      goldenTest(
        'the focused composer shows no thick focus ring, ${theme.name}, ${density.name}',
        fileName: 'composer_focused_${density.name}_${theme.name}',
        pumpBeforeTest: dsPumpIcons,
        whilePerforming: _focus,
        builder: () => dsGoldenMatrix(
          name: 'composer focused',
          builder: _focusedSheet,
          themes: [theme],
          densities: [density],
        ),
      );
    }
  }
}
