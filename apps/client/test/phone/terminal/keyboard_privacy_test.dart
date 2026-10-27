// SPDX-License-Identifier: Apache-2.0

// Keyboard privacy of screen M8 (brief M5-12 fixed requirement 15, Design note 3; PLAN 11 red
// team "keyboard suggestions"): the input field that feeds the terminal on the phone has
// suggestions, autocorrection, smart punctuation and personalized learning off, on iOS and on
// Android.

import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support.dart';

void main() {
  for (final platform in [TargetPlatform.iOS, TargetPlatform.android]) {
    testWidgets(
      '${platform.name}: the terminal field of M8 has suggestions, autocorrect and learning off',
      (tester) async {
        debugDefaultTargetPlatformOverride = platform;
        try {
          final h = await PhoneTerminalHarness.pump(tester);
          await h.openShell(tester);
          await tester.tap(find.byType(TerminalScreenView));
          await tester.pump();
          final config = tester.testTextInput.setClientArgs!;
          expect(config['enableSuggestions'], isFalse);
          expect(config['autocorrect'], isFalse);
          expect(config['enableIMEPersonalizedLearning'], isFalse);
          expect(config['smartDashesType'], '0');
          expect(config['smartQuotesType'], '0');
          await h.close(tester);
        } finally {
          debugDefaultTargetPlatformOverride = null;
        }
      },
    );
  }
}
