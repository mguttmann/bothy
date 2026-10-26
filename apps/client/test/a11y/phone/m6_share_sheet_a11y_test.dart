// SPDX-License-Identifier: Apache-2.0

// Accessibility of M6 Share from other apps, the target sheet of Android (brief M5-10 Design
// note 2; 01 5.3 M6, 5.4, 8): Cancel, Send, the preview, the comment and the targets; a file
// over the limit says why in words. The sheet of the iOS Share Extension is native and checked
// on the device (docs/quality/accessibility-mobile.md).

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../goldens/share/share_sheet_golden_test.dart' as share;
import 'support.dart';

void main() {
  for (final refused in [false, true]) {
    phoneScreenTests(
      refused
          ? 'M6 Share sheet with a file over the limit'
          : 'M6 Share sheet with a chosen thread',
      build: () => share.shareSheetScene(refused: refused),
      platform: TargetPlatform.android,
      reachable: (l) => [find.byType(TextField)],
    );
  }
}
