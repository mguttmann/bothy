// SPDX-License-Identifier: Apache-2.0

// The Flutter accessibility guidelines for every core component in light and dark
// (docs/spec/15-client-architektur.md 20): text contrast, the Android and iOS tap targets with
// the touch density and labeled tap targets.

import 'package:design_system/design_system.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/components/sheets.dart';
import 'support.dart';

void main() {
  for (final MapEntry(key: name, value: sheet) in sheets.entries) {
    for (final theme in DsThemeMode.values) {
      testWidgets('$name meets the guidelines in ${theme.name}', (
        tester,
      ) async {
        final handle = tester.ensureSemantics();
        await pumpDs(
          tester,
          sheet(),
          theme: theme,
          density: DsDensityMode.touch,
          width: sheetWidth,
        );
        await tester.pumpAndSettle();
        await expectLater(tester, meetsGuideline(textContrastGuideline));
        await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
        await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
        await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
        handle.dispose();
      });
    }
  }
}
