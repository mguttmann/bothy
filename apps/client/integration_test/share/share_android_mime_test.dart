// SPDX-License-Identifier: Apache-2.0

// The Android share chooser offers the app only for its concrete types (brief M5-06 Tests and
// fixed requirement 17; 15 12): a text/plain and an application/pdf file from the file app
// reach the app's share sheet, a file of a type the app does not list is not offered. The
// installed app's filters themselves are checked by ShareManifestTest (androidTest), which
// asserts that no `*/*` filter exists.

import 'dart:io';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support/share_flow.dart';

void main() {
  patrolTest(
    'text and PDF reach the sheet, an unsupported type is not offered',
    skip: !Platform.isAndroid,
    platformAutomatorConfig: shareAutomatorConfig,
    ($) async {
      final app = await startShareApp($.tester);
      try {
        await signIn(app);
        for (final name in [sampleText, samplePdf]) {
          final chooser = await openAndroidChooser($, name, offered: true);
          expect(
            chooserOffersApp(chooser),
            isTrue,
            reason: '$name: ${chooserTexts(chooser)}',
          );
          await pickAppInChooser($);
          // The sheet of this share (the sheet of the share before may still be closing).
          await app.waitFor(
            find.descendant(
              of: find.byKey(const ValueKey('share.sheet')),
              matching: find.textContaining(name),
            ),
          );
          await app.tap('share.cancel');
        }
        final chooser = await openAndroidChooser($, sampleUnsupported);
        expect(
          chooserOffersApp(chooser),
          isFalse,
          reason: chooserTexts(chooser),
        );
        await $.platform.android.pressBack();
      } finally {
        await app.stop();
      }
    },
  );
}
