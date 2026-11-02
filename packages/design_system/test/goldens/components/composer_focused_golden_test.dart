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

/// Largest share of differing pixels a CI golden of this file may show (one pixel in 1,000).
///
/// The scenario label of alchemist above the dark touch sheet ends one pixel column apart on
/// macOS and on the Linux runner (22 pixels of 47,840), more than [dsCiGoldenDiffThreshold]
/// allows on a sheet this small. A thick focus ring would change far more pixels. Platform
/// goldens stay exact (FIX-KD/4).
const double _ciTolerance = 0.001;

/// Passes a CI golden whose difference stays within [_ciTolerance]; every other comparison
/// is left to the package's [DsGoldenFileComparator].
class _TolerantComparator extends DsGoldenFileComparator {
  _TolerantComparator(super.testFile);

  @override
  Future<bool> compare(Uint8List imageBytes, Uri golden) async {
    if (golden.pathSegments.first == 'ci') {
      final result = await GoldenFileComparator.compareLists(
        imageBytes,
        await getGoldenBytes(golden),
      );
      final passed = result.passed || result.diffPercent <= _ciTolerance;
      result.dispose();
      if (passed) {
        return true;
      }
    }
    return super.compare(imageBytes, golden);
  }
}

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
  setUpAll(() {
    final previous = goldenFileComparator;
    if (previous is LocalFileComparator) {
      goldenFileComparator = _TolerantComparator(
        previous.basedir.resolve('composer_focused_golden_test.dart'),
      );
    }
  });
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
